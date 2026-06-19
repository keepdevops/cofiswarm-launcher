package configure

import "sync"

type PortGroup struct {
	Port       int
	Model      string
	Backend    string
	Context    int
	CtxCap     int
	GPULayers  int
	NBatch     int
	Parallel   int
	MaxTokens  int
	DraftModel string
	Names      []string
	// Long-context + KV-memory tuning, propagated from the group's first agent.
	FlashAttn     bool
	ExtraArgs     []string
	KVCacheType   string
	RopeScaling   string
	RopeFreqBase  float64
	RopeFreqScale float64
	YarnOrigCtx   int
	YarnExtFactor float64
	TurboQuant    bool
}

type Progress struct {
	mu     sync.RWMutex
	Active bool
	Ports  map[string]string
}

func (p *Progress) Reset(ports []int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Active = true
	p.Ports = make(map[string]string, len(ports))
	for _, port := range ports {
		p.Ports[itoa(port)] = "starting"
	}
}

func (p *Progress) Set(port int, state string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Ports == nil {
		p.Ports = map[string]string{}
	}
	p.Ports[itoa(port)] = state
}

func (p *Progress) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Active = false
}

func (p *Progress) Snapshot() (bool, map[string]string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]string, len(p.Ports))
	for k, v := range p.Ports {
		out[k] = v
	}
	return p.Active, out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
