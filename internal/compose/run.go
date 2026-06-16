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

func Up(opts Options) error {
	if opts.ComposeDir == "" {
		return fmt.Errorf("compose dir required")
	}
	base := filepath.Join(opts.ComposeDir, "docker-compose.yml")
	args := []string{"compose", "-f", base}
	if opts.Profile != "" {
		prof := filepath.Join(opts.ComposeDir, "profiles", opts.Profile+".yml")
		if _, err := os.Stat(prof); err == nil {
			args = append(args, "-f", prof)
		}
		args = append(args, "--profile", opts.Profile)
	}
	args = append(args, "up")
	if opts.Detach {
		args = append(args, "-d")
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = opts.ComposeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if opts.DryRun {
		fmt.Println("docker", joinArgs(args))
		return nil
	}
	return cmd.Run()
}

func Down(opts Options) error {
	base := filepath.Join(opts.ComposeDir, "docker-compose.yml")
	args := []string{"compose", "-f", base, "down"}
	if opts.Profile != "" {
		args = append(args, "--profile", opts.Profile)
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = opts.ComposeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if opts.DryRun {
		fmt.Println("docker", joinArgs(args))
		return nil
	}
	return cmd.Run()
}

func ConfigValidate(opts Options) error {
	base := filepath.Join(opts.ComposeDir, "docker-compose.yml")
	args := []string{"compose", "-f", base}
	if opts.Profile != "" {
		prof := filepath.Join(opts.ComposeDir, "profiles", opts.Profile+".yml")
		if _, err := os.Stat(prof); err == nil {
			args = append(args, "-f", prof)
		}
		args = append(args, "--profile", opts.Profile)
	}
	args = append(args, "config")
	cmd := exec.Command("docker", args...)
	cmd.Dir = opts.ComposeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

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
