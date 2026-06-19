package configure

import (
	"encoding/json"
	"fmt"
)

type Agent struct {
	Name         string `json:"name"`
	Model        string `json:"model"`
	Port         int    `json:"port"`
	Backend      string `json:"backend"`
	Engine       string `json:"engine"`
	Context      int    `json:"context"`
	CtxCap       int    `json:"ctx_cap"`
	GPULayers    int    `json:"gpu_layers"`
	NBatch       int    `json:"n_batch"`
	MaxConc      int    `json:"max_concurrency"`
	DraftModel   string `json:"draft_model"`
}

func BuildPortGroups(agents []Agent) ([]PortGroup, error) {
	type key struct {
		port    int
		model   string
		backend string
	}
	groups := map[key]*PortGroup{}
	order := []key{}
	for _, a := range agents {
		if a.Name == "" || a.Model == "" {
			return nil, fmt.Errorf("agent missing name or model")
		}
		bk := a.Backend
		if bk == "" {
			bk = a.Engine
		}
		if bk == "" {
			bk = "llama"
		}
		if bk != "llama" {
			continue
		}
		if a.Port <= 0 {
			return nil, fmt.Errorf("agent %q missing port", a.Name)
		}
		k := key{port: a.Port, model: a.Model, backend: bk}
		g, ok := groups[k]
		if !ok {
			par := a.MaxConc
			if par <= 0 {
				par = 1
			}
			g = &PortGroup{
				Port: a.Port, Model: a.Model, Backend: bk,
				Context: a.Context, CtxCap: a.CtxCap, GPULayers: a.GPULayers,
				NBatch: a.NBatch, Parallel: par,
			}
			groups[k] = g
			order = append(order, k)
		}
		g.Names = append(g.Names, a.Name)
		if a.MaxConc > g.Parallel {
			g.Parallel = a.MaxConc
		}
	}
	out := make([]PortGroup, 0, len(order))
	for _, k := range order {
		out = append(out, *groups[k])
	}
	return out, nil
}

func ParseAgents(body []byte) ([]Agent, error) {
	var req struct {
		Agents []Agent `json:"agents"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	if len(req.Agents) == 0 {
		return nil, fmt.Errorf("no agents")
	}
	return req.Agents, nil
}
