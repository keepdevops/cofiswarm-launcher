package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/keepdevops/cofiswarm-launcher/internal/compose"
)

func main() {
	composeDir := flag.String("compose-dir", "", "directory with docker-compose.yml")
	dryRun := flag.Bool("dry-run", false, "print docker command only")
	flag.Parse()
	if *composeDir == "" {
		if v := os.Getenv("COFISWARM_LAUNCHER_COMPOSE_DIR"); v != "" {
			*composeDir = v
		} else {
			exe, _ := os.Executable()
			*composeDir = filepath.Join(filepath.Dir(exe), "..", "compose")
		}
	}
	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: cofiswarm-launcher profile up|down|config <8gb|16gb|32gb>\n")
		os.Exit(2)
	}
	if args[0] != "profile" {
		fmt.Fprintf(os.Stderr, "only 'profile' subcommand implemented\n")
		os.Exit(2)
	}
	profile := "8gb"
	if len(args) >= 3 {
		profile = args[2]
	}
	opts := compose.Options{ComposeDir: *composeDir, Profile: profile, DryRun: *dryRun, Detach: true}
	var err error
	switch args[1] {
	case "up":
		err = compose.Up(opts)
	case "down":
		err = compose.Down(opts)
	case "config":
		err = compose.ConfigValidate(opts)
	default:
		fmt.Fprintf(os.Stderr, "unknown action %q\n", args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "launcher: %v\n", err)
		os.Exit(1)
	}
}
