package configure

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// mlxPython resolves the interpreter that has mlx_lm installed: explicit override,
// then the conventional mlx-env, then PATH.
func mlxPython() string {
	if v := os.Getenv("MATRIX_MLX_PYTHON"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	cand := filepath.Join(home, "miniforge3", "envs", "mlx-env", "bin", "python")
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// turboPromptCacheBytes caps the MLX prompt (KV) cache for the TurboQuant lane.
// mlx_lm.server has no launch-time --kv-bits; the quant is the 4bit model and KV
// memory is bounded with --prompt-cache-bytes. 1 GiB is a conservative default.
const turboPromptCacheBytes = 1 << 30

// buildMLXArgs builds the `python -m mlx_lm.server` argv for one MLX port group.
// mlx_lm.server serves a single model (OpenAI-compatible) and is probed at
// /v1/models. ExtraArgs go last so explicit config always wins.
func buildMLXArgs(g PortGroup) []string {
	args := []string{
		"-m", "mlx_lm.server",
		"--model", g.Model,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(g.Port),
	}
	if g.MaxTokens > 0 {
		args = append(args, "--max-tokens", strconv.Itoa(g.MaxTokens))
	}
	if g.DraftModel != "" {
		args = append(args, "--draft-model", g.DraftModel)
	}
	// TurboQuant lane: bound the KV/prompt cache memory.
	if g.TurboQuant || strings.Contains(strings.ToLower(g.Model), "4bit") {
		args = append(args, "--prompt-cache-bytes", strconv.Itoa(turboPromptCacheBytes))
	}
	if len(g.ExtraArgs) > 0 {
		args = append(args, g.ExtraArgs...)
	}
	return args
}

// SpawnMLX launches an mlx_lm.server for the group (mirrors SpawnLlama).
func SpawnMLX(g PortGroup, logDir string) error {
	py := mlxPython()
	if py == "" {
		return fmt.Errorf("mlx python not found (set MATRIX_MLX_PYTHON)")
	}
	killPort(g.Port)
	if g.TurboQuant || strings.Contains(strings.ToLower(g.Model), "4bit") {
		fmt.Fprintf(os.Stderr, "[configure] MLX TurboQuant lane on port %d (model=%s)\n", g.Port, g.Model)
	}
	args := buildMLXArgs(g)
	cmd := exec.Command(py, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if logDir != "" {
		_ = os.MkdirAll(logDir, 0o755)
		logPath := filepath.Join(logDir, fmt.Sprintf("mlx-%d.log", g.Port))
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
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

// spawnGroup routes a port group to the right backend spawner.
func spawnGroup(g PortGroup, logDir string) error {
	if g.Backend == "mlx" {
		return SpawnMLX(g, logDir)
	}
	return SpawnLlama(g, logDir)
}
