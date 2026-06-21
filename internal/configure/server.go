package configure

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Server struct {
	StateDir          string
	CoordinatorConfig string
	LogDir            string
	Progress          Progress
	HealthTimeout     time.Duration
	mu                sync.Mutex
}

func NewServer() *Server {
	lib := os.Getenv("COFISWARM_VAR_LIB")
	if lib == "" {
		lib = "/var/lib/cofiswarm"
	}
	state := filepath.Join(lib, "cofiswarm", "launcher")
	if v := os.Getenv("COFISWARM_LAUNCHER_STATE"); v != "" {
		state = v
	}
	coord := os.Getenv("COFISWARM_COORDINATOR_CONFIG")
	if coord == "" {
		coord = "/etc/cofiswarm/config/coordinator.json"
	}
	logDir := os.Getenv("COFISWARM_VAR_LOG")
	if logDir == "" {
		logDir = "/var/log/cofiswarm"
	}
	logDir = filepath.Join(logDir, "launcher")
	timeout := 120 * time.Second
	if v := os.Getenv("COFISWARM_CONFIGURE_HEALTH_SECS"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	return &Server{
		StateDir:          state,
		CoordinatorConfig: coord,
		LogDir:            logDir,
		HealthTimeout:     timeout,
	}
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/configure/status", s.handleStatus)
	mux.HandleFunc("/api/configure", s.handleConfigure)
	return mux
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	active, ports := s.Progress.Snapshot()
	_ = json.NewEncoder(w).Encode(map[string]any{"active": active, "running": active, "ports": ports})
}

func (s *Server) handleConfigure(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	agents, err := ParseAgents(body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	groups, err := BuildPortGroups(agents)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if len(groups) > 0 && llamaBin() == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "llama-server not found — set MATRIX_LLAMA_SERVER in .env or PATH",
		})
		return
	}
	if err := WriteActiveConfig(s.StateDir, s.CoordinatorConfig, agents); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	s.mu.Lock()
	if active, _ := s.Progress.Snapshot(); active {
		s.mu.Unlock()
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "configure already in progress"})
		return
	}
	s.mu.Unlock()

	ports := make([]int, 0, len(groups))
	for _, g := range groups {
		ports = append(ports, g.Port)
	}
	s.Progress.Reset(ports)

	type result struct {
		failed  []int
		servers []map[string]any
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("configure panic: %v", rec)
				s.Progress.Finish()
				done <- result{failed: ports}
			}
		}()
		servers, failed := s.RunGroups(groups)
		done <- result{failed: failed, servers: servers}
	}()

	select {
	case res := <-done:
		s.writeConfigureResult(w, groups, res.failed, res.servers)
	case <-r.Context().Done():
		// Client went away — keep spawning; UI polls /api/configure/status.
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "active": true})
		go func() { <-done }()
	}
}

// RunGroups spawns each port group and waits for health, updating Progress. Returns the
// ready servers and any failed ports. Shared by the HTTP and bus (-bus) configure paths.
func (s *Server) RunGroups(groups []PortGroup) ([]map[string]any, []int) {
	var failed []int
	var servers []map[string]any
	for _, g := range groups {
		if err := spawnGroup(g, s.LogDir); err != nil {
			log.Printf("configure spawn port %d: %v", g.Port, err)
			s.Progress.Set(g.Port, "error")
			failed = append(failed, g.Port)
			continue
		}
		if err := WaitHealthy(g.Port, s.HealthTimeout); err != nil {
			log.Printf("configure health port %d: %v", g.Port, err)
			s.Progress.Set(g.Port, "error")
			failed = append(failed, g.Port)
			continue
		}
		s.Progress.Set(g.Port, "ready")
		servers = append(servers, map[string]any{
			"port": g.Port, "model": g.Model, "agents": g.Names, "status": "ready",
		})
	}
	s.Progress.Finish()
	return servers, failed
}

func (s *Server) writeConfigureResult(w http.ResponseWriter, groups []PortGroup, failed []int, servers []map[string]any) {
	if len(failed) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "health check failed", "failedPorts": failed, "servers": servers,
		})
		return
	}
	if len(groups) == 0 {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "note": "no llama agents to spawn", "servers": []any{},
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "servers": servers})
}
