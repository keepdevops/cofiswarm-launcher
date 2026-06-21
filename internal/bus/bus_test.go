package bus

import (
	"encoding/json"
	"testing"

	"github.com/keepdevops/cofiswarm-launcher/internal/configure"
)

// These exercise the validation / conflict / status paths, which never spawn a real server.

func TestConfigureRejectsNoAgents(t *testing.T) {
	out, _ := Routes(configure.NewServer())[SubjConfigure]([]byte(`{"agents":[]}`))
	if r := out.(configureReply); r.OK {
		t.Fatalf("expected ok=false, got %+v", r)
	}
}

func TestConfigureRejectsAgentMissingModel(t *testing.T) {
	out, _ := Routes(configure.NewServer())[SubjConfigure](
		[]byte(`{"agents":[{"name":"a","port":8086}]}`))
	if r := out.(configureReply); r.OK || r.Error == "" {
		t.Fatalf("expected validation error, got %+v", r)
	}
}

func TestConfigureConflictWhenActive(t *testing.T) {
	s := configure.NewServer()
	s.Progress.Reset([]int{8086}) // mark a configure in progress
	out, _ := Routes(s)[SubjConfigure](
		[]byte(`{"agents":[{"name":"a","model":"m","port":8086}]}`))
	if r := out.(configureReply); r.OK || r.Error != "configure already in progress" {
		t.Fatalf("got %+v", r)
	}
}

func TestStatusReportsProgress(t *testing.T) {
	s := configure.NewServer()
	s.Progress.Reset([]int{8086})
	s.Progress.Set(8086, "ready")
	out, _ := Routes(s)[SubjStatus](nil)
	if r := out.(statusReply); !r.Active || r.Ports["8086"] != "ready" {
		t.Fatalf("got %+v", r)
	}
}

func TestConfigureReplyFieldNames(t *testing.T) {
	s := configure.NewServer()
	s.Progress.Reset([]int{8086}) // active -> conflict path, no spawn
	out, _ := Routes(s)[SubjConfigure](
		[]byte(`{"agents":[{"name":"a","model":"m","port":8086}]}`))
	b, _ := json.Marshal(out)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"schema_version", "ok", "accepted", "servers"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing %q in %s", k, b)
		}
	}
}
