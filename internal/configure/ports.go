package configure

import (
	"encoding/json"
	"fmt"
)

type Agent struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	Port       int    `json:"port"`
	Backend    string `json:"backend"`
	Engine     string `json:"engine"`
	Context    int    `json:"context"`
	CtxCap     int    `json:"ctx_cap"`
	GPULayers  int    `json:"gpu_layers"`
	NBatch     int    `json:"n_batch"`
	MaxConc    int    `json:"max_concurrency"`
	DraftModel string `json:"draft_model"`
	// Long-context + KV-memory tuning (passed through to llama-server).
	FlashAttn     bool     `json:"flash_attn"`
	ExtraArgs     []string `json:"extra_args"`
	KVCacheType   string   `json:"kv_cache_type"`   // "q4_0" or "q4_0,q8_0" (k,v)
	RopeScaling   string   `json:"rope_scaling"`    // "linear" | "yarn"
	RopeFreqBase  float64  `json:"rope_freq_base"`
	RopeFreqScale float64  `json:"rope_freq_scale"`
	YarnOrigCtx   int      `json:"yarn_orig_ctx"`
	YarnExtFactor float64  `json:"yarn_ext_factor"`
	TurboQuant    bool     `json:"turbo_quant"` // route KV to a turbo cache type
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
				// Tuning is per backing server: first agent of the group wins.
				FlashAttn: a.FlashAttn, ExtraArgs: a.ExtraArgs, KVCacheType: a.KVCacheType,
				RopeScaling: a.RopeScaling, RopeFreqBase: a.RopeFreqBase,
				RopeFreqScale: a.RopeFreqScale, YarnOrigCtx: a.YarnOrigCtx,
				YarnExtFactor: a.YarnExtFactor, TurboQuant: a.TurboQuant,
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
