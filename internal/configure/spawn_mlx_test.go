package configure

import (
	"strings"
	"testing"
)

func TestBuildMLXArgs_Basic(t *testing.T) {
	args := buildMLXArgs(PortGroup{Port: 8083, Model: "/m/Llama-3.2-1B", Backend: "mlx"})
	if argValue(args, "-m") != "mlx_lm.server" {
		t.Errorf("module = %q, want mlx_lm.server", argValue(args, "-m"))
	}
	if argValue(args, "--model") != "/m/Llama-3.2-1B" {
		t.Errorf("--model = %q", argValue(args, "--model"))
	}
	if argValue(args, "--port") != "8083" || argValue(args, "--host") != "127.0.0.1" {
		t.Errorf("host/port wrong: %v", args)
	}
	if hasFlag(args, "--prompt-cache-bytes") {
		t.Errorf("non-turbo model should not cap prompt cache: %v", args)
	}
}

func TestBuildMLXArgs_TurboQuantCapsCache(t *testing.T) {
	// Explicit turbo flag.
	a1 := buildMLXArgs(PortGroup{Port: 8083, Model: "/m/Llama-3.2-1B", TurboQuant: true})
	if argValue(a1, "--prompt-cache-bytes") == "" {
		t.Errorf("turbo_quant should cap prompt cache: %v", a1)
	}
	// 4bit model-name convention.
	a2 := buildMLXArgs(PortGroup{Port: 8083, Model: "/m/Llama-3.2-1B-Instruct-4bit"})
	if argValue(a2, "--prompt-cache-bytes") == "" {
		t.Errorf("4bit model should cap prompt cache: %v", a2)
	}
}

func TestBuildMLXArgs_MaxTokensAndDraft(t *testing.T) {
	args := buildMLXArgs(PortGroup{
		Port: 8083, Model: "/m/x", MaxTokens: 512, DraftModel: "/m/draft",
	})
	if argValue(args, "--max-tokens") != "512" {
		t.Errorf("--max-tokens = %q", argValue(args, "--max-tokens"))
	}
	if argValue(args, "--draft-model") != "/m/draft" {
		t.Errorf("--draft-model = %q", argValue(args, "--draft-model"))
	}
}

func TestBuildMLXArgs_ExtraArgsLast(t *testing.T) {
	args := buildMLXArgs(PortGroup{
		Port: 8083, Model: "/m/x", ExtraArgs: []string{"--trust-remote-code"},
	})
	if !strings.HasSuffix(strings.Join(args, " "), "--trust-remote-code") {
		t.Errorf("extra_args not last: %v", args)
	}
}

func TestBuildPortGroups_IncludesMLX(t *testing.T) {
	groups, err := BuildPortGroups([]Agent{
		{Name: "mlx-scout", Model: "/m/Llama-4bit", Port: 8083, Engine: "mlx", MaxTokens: 512, TurboQuant: true},
		{Name: "architect", Model: "/m/coder", Port: 8086, Engine: "llama"},
		{Name: "docker-x", Model: "/m/d", Port: 9000, Engine: "docker"}, // skipped
	})
	if err != nil {
		t.Fatal(err)
	}
	var mlx, llama, other int
	for _, g := range groups {
		switch g.Backend {
		case "mlx":
			mlx++
			if !g.TurboQuant || g.MaxTokens != 512 {
				t.Errorf("mlx group missing fields: %+v", g)
			}
		case "llama":
			llama++
		default:
			other++
		}
	}
	if mlx != 1 || llama != 1 || other != 0 {
		t.Errorf("groups: mlx=%d llama=%d other=%d (docker should be skipped)", mlx, llama, other)
	}
}
