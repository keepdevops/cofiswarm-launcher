package compose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Options struct {
	ComposeDir string
	Profile    string
	DryRun     bool
	Detach     bool
}

// Args builds the `docker compose ...` argument list for an action ("up"/"down"/"config").
// It includes the base file, the profile overlay (when present on disk), and --profile.
// Pure + filesystem-only, so it is unit-testable.
func Args(opts Options, action string) []string {
	args := []string{"compose", "-f", filepath.Join(opts.ComposeDir, "docker-compose.yml")}
	if opts.Profile != "" {
		prof := filepath.Join(opts.ComposeDir, "profiles", opts.Profile+".yml")
		if _, err := os.Stat(prof); err == nil {
			args = append(args, "-f", prof)
		}
		args = append(args, "--profile", opts.Profile)
	}
	args = append(args, action)
	if action == "up" && opts.Detach {
		args = append(args, "-d")
	}
	return args
}

func run(opts Options, action string) error {
	if opts.ComposeDir == "" {
		return fmt.Errorf("compose dir required")
	}
	args := Args(opts, action)
	if opts.DryRun {
		fmt.Println("docker", joinArgs(args))
		return nil
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = opts.ComposeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func Up(opts Options) error             { return run(opts, "up") }
func Down(opts Options) error           { return run(opts, "down") }
func ConfigValidate(opts Options) error { return run(opts, "config") }

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
