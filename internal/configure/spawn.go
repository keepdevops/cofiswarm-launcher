package configure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func llamaBin() string {
	if v := os.Getenv("MATRIX_LLAMA_SERVER"); v != "" {
		return v
	}
	if p, err := exec.LookPath("llama-server"); err == nil {
		return p
	}
	return ""
}

func slotsDir() string {
	if v := os.Getenv("COFISWARM_SLOTS_DIR"); v != "" {
		return v
	}
	lib := os.Getenv("COFISWARM_VAR_LIB")
	if lib == "" {
		lib = "/var/lib/cofiswarm"
	}
	return filepath.Join(lib, "cofiswarm", "models", "llama", "slots")
}

func buildLlamaArgs(g PortGroup) []string {
	slots := g.Parallel
	if slots <= 0 {
		slots = len(g.Names)
	}
	if slots <= 0 {
		slots = 1
	}
	ctx := g.Context * slots
	if ctx <= 0 {
		ctx = 8192 * slots
	}
	if g.CtxCap > 0 && ctx > g.CtxCap {
		ctx = g.CtxCap
	}
	gpu := g.GPULayers
	if gpu <= 0 {
		gpu = 99
	}
	args := []string{
		"-m", g.Model,
		"-c", strconv.Itoa(ctx),
		"--port", strconv.Itoa(g.Port),
		"--host", "127.0.0.1",
		"--n-gpu-layers", strconv.Itoa(gpu),
		"--parallel", strconv.Itoa(slots),
		"--metrics",
		"--slot-save-path", slotsDir(),
		"--fit", "off",
	}
	if g.NBatch > 0 {
		args = append(args, "--batch-size", strconv.Itoa(g.NBatch))
	}
	args = appendTuningArgs(args, g)
	return args
}

// turboCacheType is the KV cache-type used for the TurboQuant pilot lane. It's a
// generic passthrough: if the installed llama-server build supports it the lane is
// active, otherwise set kv_cache_type explicitly in config.
const turboCacheType = "q4_0"

// kvTypes splits a kv_cache_type config value ("q4_0" or "q4_0,q8_0") into the
// k and v cache types. A single value applies to both.
func kvTypes(spec string) (k, v string) {
	parts := strings.SplitN(spec, ",", 2)
	k = strings.TrimSpace(parts[0])
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return k, strings.TrimSpace(parts[1])
	}
	return k, k
}

// appendTuningArgs adds the long-context (RoPE/YaRN) and KV-memory (flash-attn,
// cache-type quant) flags to a llama-server argv. Each flag is emitted only when
// its config value is set; ExtraArgs go last so explicit config always wins.
func appendTuningArgs(args []string, g PortGroup) []string {
	// extra_args is authoritative for cache-type if it sets one — don't duplicate.
	extraHasCacheType := false
	for _, a := range g.ExtraArgs {
		if a == "--cache-type-k" || a == "--cache-type-v" {
			extraHasCacheType = true
			break
		}
	}
	// KV cache type: explicit config, else turbo lane, else flash-attn default.
	cacheSpec := g.KVCacheType
	if cacheSpec == "" && (g.TurboQuant || strings.Contains(strings.ToLower(g.Model), "4bit")) {
		cacheSpec = turboCacheType
	}
	if g.FlashAttn {
		args = append(args, "--flash-attn", "on")
		if cacheSpec == "" {
			cacheSpec = "q8_0"
		}
	}
	if cacheSpec != "" && !extraHasCacheType {
		k, v := kvTypes(cacheSpec)
		args = append(args, "--cache-type-k", k, "--cache-type-v", v)
	}
	if g.RopeScaling != "" {
		args = append(args, "--rope-scaling", g.RopeScaling)
	}
	if g.RopeFreqBase > 0 {
		args = append(args, "--rope-freq-base", strconv.FormatFloat(g.RopeFreqBase, 'g', -1, 64))
	}
	if g.RopeFreqScale > 0 {
		args = append(args, "--rope-freq-scale", strconv.FormatFloat(g.RopeFreqScale, 'g', -1, 64))
	}
	if g.RopeScaling == "linear" || g.RopeScaling == "yarn" {
		if g.YarnOrigCtx > 0 {
			args = append(args, "--yarn-orig-ctx", strconv.Itoa(g.YarnOrigCtx))
		}
		if g.YarnExtFactor > 0 {
			args = append(args, "--yarn-ext-factor", strconv.FormatFloat(g.YarnExtFactor, 'g', -1, 64))
		}
	}
	if len(g.ExtraArgs) > 0 {
		args = append(args, g.ExtraArgs...)
	}
	return args
}

func killPort(port int) {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	if err != nil || len(out) == 0 {
		return
	}
	for _, pidStr := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 1 || pid == os.Getpid() {
			continue
		}
		p := exec.Command("ps", "-p", pidStr, "-o", "comm=")
		name, _ := p.Output()
		if !strings.Contains(string(name), "llama-server") && !strings.Contains(string(name), "cofiswarm") {
			continue
		}
		if strings.Contains(string(name), "cofiswarm-configure") {
			continue
		}
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

func SpawnLlama(g PortGroup, logDir string) error {
	bin := llamaBin()
	if bin == "" {
		return fmt.Errorf("llama-server not found (set MATRIX_LLAMA_SERVER)")
	}
	if err := os.MkdirAll(slotsDir(), 0o755); err != nil {
		return fmt.Errorf("slots dir: %w", err)
	}
	killPort(g.Port)
	if g.TurboQuant || strings.Contains(strings.ToLower(g.Model), "4bit") {
		fmt.Fprintf(os.Stderr, "[configure] TurboQuant lane active on port %d (kv=%s)\n", g.Port, g.KVCacheType)
	}
	args := buildLlamaArgs(g)
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if logDir != "" {
		_ = os.MkdirAll(logDir, 0o755)
		logPath := filepath.Join(logDir, fmt.Sprintf("llama-%d.log", g.Port))
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			cmd.Stdout = f
			cmd.Stderr = f
		}
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func WaitHealthy(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	urls := []string{
		fmt.Sprintf("http://127.0.0.1:%d/health", port),
		fmt.Sprintf("http://127.0.0.1:%d/v1/models", port),
	}
	for time.Now().Before(deadline) {
		for _, u := range urls {
			resp, err := client.Get(u)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					return nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}

func WriteActiveConfig(stateDir, coordinatorPath string, agents []Agent) error {
	_ = os.MkdirAll(stateDir, 0o755)
	var coord map[string]any
	if b, err := os.ReadFile(coordinatorPath); err == nil {
		_ = json.Unmarshal(b, &coord)
	}
	active := map[string]any{"agents": agents}
	if coord != nil {
		if c, ok := coord["coordinator"]; ok {
			active["coordinator"] = c
		}
		if u, ok := coord["ui"]; ok {
			active["ui"] = u
		}
		if r, ok := coord["rag"]; ok {
			active["rag"] = r
		}
	}
	b, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, "active-config.json"), b, 0o644)
}
