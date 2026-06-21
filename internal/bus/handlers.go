package bus

import (
	"github.com/keepdevops/cofiswarm-launcher/internal/configure"
	"github.com/keepdevops/cofiswarm-observer-sdk/pkg/servicecomponent"
)

// Capability subjects (must match observer's bus/subjects.py).
const (
	SubjConfigure = servicecomponent.Prefix + ".launcher.configure"
	SubjStatus    = servicecomponent.Prefix + ".launcher.status"
)

// Routes wires a configure.Server to the .launcher.* subjects. configure is async (spawn +
// health can take up to HealthTimeout): the reply is `accepted` and per-port outcome is
// polled via .launcher.status. Reply field names mirror observer's bus/contracts/resource.py
// (ConfigureReply / LauncherStatusReply).
func Routes(s *configure.Server) map[string]servicecomponent.Handler {
	return map[string]servicecomponent.Handler{
		SubjConfigure: configureHandler(s),
		SubjStatus:    statusHandler(s),
	}
}

func configureHandler(s *configure.Server) servicecomponent.Handler {
	return func(data []byte) (any, error) {
		agents, err := configure.ParseAgents(data)
		if err != nil {
			return configureReply{SchemaVersion: servicecomponent.SchemaVersion, OK: false, Error: err.Error()}, nil
		}
		groups, err := configure.BuildPortGroups(agents)
		if err != nil {
			return configureReply{SchemaVersion: servicecomponent.SchemaVersion, OK: false, Error: err.Error()}, nil
		}
		if active, _ := s.Progress.Snapshot(); active {
			return configureReply{SchemaVersion: servicecomponent.SchemaVersion, OK: false,
				Error: "configure already in progress"}, nil
		}
		if err := configure.WriteActiveConfig(s.StateDir, s.CoordinatorConfig, agents); err != nil {
			return configureReply{SchemaVersion: servicecomponent.SchemaVersion, OK: false, Error: err.Error()}, nil
		}

		planned := make([]configuredServer, 0, len(groups))
		ports := make([]int, 0, len(groups))
		for _, g := range groups {
			planned = append(planned, configuredServer{
				Port: g.Port, Model: g.Model, Agents: g.Names, Status: "starting"})
			ports = append(ports, g.Port)
		}
		s.Progress.Reset(ports)
		go s.RunGroups(groups) // async; outcome observed via .launcher.status

		return configureReply{SchemaVersion: servicecomponent.SchemaVersion, OK: true, Accepted: true,
			Servers: planned}, nil
	}
}

func statusHandler(s *configure.Server) servicecomponent.Handler {
	return func([]byte) (any, error) {
		active, ports := s.Progress.Snapshot()
		if ports == nil {
			ports = map[string]string{}
		}
		return statusReply{SchemaVersion: servicecomponent.SchemaVersion, OK: true, Active: active, Ports: ports}, nil
	}
}

type configuredServer struct {
	Port   int      `json:"port"`
	Model  string   `json:"model"`
	Agents []string `json:"agents"`
	Status string   `json:"status"`
}

type configureReply struct {
	SchemaVersion string             `json:"schema_version"`
	OK            bool               `json:"ok"`
	Error         string             `json:"error,omitempty"`
	Accepted      bool               `json:"accepted"`
	Servers       []configuredServer `json:"servers"`
}

type statusReply struct {
	SchemaVersion string            `json:"schema_version"`
	OK            bool              `json:"ok"`
	Error         string            `json:"error,omitempty"`
	Active        bool              `json:"active"`
	Ports         map[string]string `json:"ports"`
}
