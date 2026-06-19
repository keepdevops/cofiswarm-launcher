package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempComposeDir(t *testing.T, withProfile string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("name: t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withProfile != "" {
		if err := os.MkdirAll(filepath.Join(dir, "profiles"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "profiles", withProfile+".yml"), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestArgsUpWithProfileAndDetach(t *testing.T) {
	dir := tempComposeDir(t, "16gb")
	got := strings.Join(Args(Options{ComposeDir: dir, Profile: "16gb", Detach: true}, "up"), " ")
	for _, want := range []string{"compose", "docker-compose.yml", "profiles/16gb.yml", "--profile 16gb", "up", "-d"} {
		if !strings.Contains(got, want) {
			t.Errorf("args %q missing %q", got, want)
		}
	}
}

func TestArgsDownNoProfileFile(t *testing.T) {
	dir := tempComposeDir(t, "") // no profile overlay on disk
	got := Args(Options{ComposeDir: dir, Profile: "8gb"}, "down")
	j := strings.Join(got, " ")
	if strings.Contains(j, "profiles/8gb.yml") {
		t.Errorf("overlay should be absent: %q", j)
	}
	if !strings.Contains(j, "--profile 8gb") || !strings.Contains(j, "down") {
		t.Errorf("expected --profile + down: %q", j)
	}
	if strings.Contains(j, "-d") {
		t.Errorf("down must not have -d: %q", j)
	}
}

func TestArgsConfig(t *testing.T) {
	dir := tempComposeDir(t, "32gb")
	got := strings.Join(Args(Options{ComposeDir: dir, Profile: "32gb"}, "config"), " ")
	if !strings.HasSuffix(got, "config") || !strings.Contains(got, "profiles/32gb.yml") {
		t.Errorf("config args wrong: %q", got)
	}
}
