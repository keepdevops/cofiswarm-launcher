package configure

import (
	"strings"
	"testing"
)

// argValue returns the argument following flag in args, or "" if absent.
func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func TestBuildLlamaArgs_BaselineNoTuning(t *testing.T) {
	args := buildLlamaArgs(PortGroup{Port: 8086, Model: "m.gguf", Context: 2048, Names: []string{"a"}})
	for _, unexpected := range []string{"--flash-attn", "--cache-type-k", "--rope-scaling", "--yarn-orig-ctx"} {
		if hasFlag(args, unexpected) {
			t.Errorf("baseline emitted %s unexpectedly: %v", unexpected, args)
		}
	}
	if argValue(args, "-c") != "2048" {
		t.Errorf("ctx = %q, want 2048", argValue(args, "-c"))
	}
}

func TestBuildLlamaArgs_CtxCapClamp(t *testing.T) {
	// 4096 context * 2 slots = 8192, capped to 6144.
	args := buildLlamaArgs(PortGroup{Port: 8086, Model: "m.gguf", Context: 4096, CtxCap: 6144, Parallel: 2})
	if argValue(args, "-c") != "6144" {
		t.Errorf("ctx = %q, want 6144 (clamped)", argValue(args, "-c"))
	}
}

func TestBuildLlamaArgs_FlashAttnDefaultKV(t *testing.T) {
	args := buildLlamaArgs(PortGroup{Port: 8086, Model: "m.gguf", Context: 2048, FlashAttn: true})
	if !hasFlag(args, "--flash-attn") {
		t.Fatal("flash-attn not emitted")
	}
	if argValue(args, "--cache-type-k") != "q8_0" || argValue(args, "--cache-type-v") != "q8_0" {
		t.Errorf("flash-attn default KV wrong: %v", args)
	}
}

func TestBuildLlamaArgs_KVCacheTypeSplit(t *testing.T) {
	args := buildLlamaArgs(PortGroup{Port: 8086, Model: "m.gguf", Context: 2048, KVCacheType: "q4_0,q8_0"})
	if argValue(args, "--cache-type-k") != "q4_0" || argValue(args, "--cache-type-v") != "q8_0" {
		t.Errorf("split KV wrong: %v", args)
	}
}

func TestBuildLlamaArgs_KVCacheTypeSingleAppliesBoth(t *testing.T) {
	args := buildLlamaArgs(PortGroup{Port: 8086, Model: "m.gguf", Context: 2048, KVCacheType: "q4_0"})
	if argValue(args, "--cache-type-k") != "q4_0" || argValue(args, "--cache-type-v") != "q4_0" {
		t.Errorf("single KV should apply to both: %v", args)
	}
}

func TestBuildLlamaArgs_RopeYarn(t *testing.T) {
	args := buildLlamaArgs(PortGroup{
		Port: 8086, Model: "m.gguf", Context: 2048,
		RopeScaling: "yarn", RopeFreqScale: 0.5, YarnOrigCtx: 4096, YarnExtFactor: 1,
	})
	if argValue(args, "--rope-scaling") != "yarn" {
		t.Errorf("rope-scaling = %q", argValue(args, "--rope-scaling"))
	}
	if argValue(args, "--rope-freq-scale") != "0.5" {
		t.Errorf("rope-freq-scale = %q", argValue(args, "--rope-freq-scale"))
	}
	if argValue(args, "--yarn-orig-ctx") != "4096" {
		t.Errorf("yarn-orig-ctx = %q", argValue(args, "--yarn-orig-ctx"))
	}
}

func TestBuildLlamaArgs_YarnFlagsRequireRopeScaling(t *testing.T) {
	// YaRN fields set but no rope_scaling → no yarn flags.
	args := buildLlamaArgs(PortGroup{Port: 8086, Model: "m.gguf", Context: 2048, YarnOrigCtx: 4096})
	if hasFlag(args, "--yarn-orig-ctx") {
		t.Errorf("yarn flag emitted without rope-scaling: %v", args)
	}
}

func TestBuildLlamaArgs_TurboQuantRoutesKV(t *testing.T) {
	args := buildLlamaArgs(PortGroup{Port: 8083, Model: "qwen-4bit.gguf", Context: 2048, TurboQuant: true})
	if argValue(args, "--cache-type-k") != turboCacheType {
		t.Errorf("turbo lane KV = %q, want %q", argValue(args, "--cache-type-k"), turboCacheType)
	}
}

func TestBuildLlamaArgs_ExtraArgsLast(t *testing.T) {
	args := buildLlamaArgs(PortGroup{
		Port: 8086, Model: "m.gguf", Context: 2048, KVCacheType: "q4_0",
		ExtraArgs: []string{"--mlock", "--no-warmup"},
	})
	joined := strings.Join(args, " ")
	if !strings.HasSuffix(joined, "--mlock --no-warmup") {
		t.Errorf("extra_args not last: %v", args)
	}
}

func countFlag(args []string, flag string) int {
	n := 0
	for _, a := range args {
		if a == flag {
			n++
		}
	}
	return n
}

func TestBuildLlamaArgs_NoDuplicateCacheTypeWithExtraArgs(t *testing.T) {
	// flash_attn would default a cache-type, but extra_args already sets one.
	args := buildLlamaArgs(PortGroup{
		Port: 8086, Model: "m.gguf", Context: 2048, FlashAttn: true,
		ExtraArgs: []string{"--cache-type-k", "q4_0", "--cache-type-v", "q8_0"},
	})
	if c := countFlag(args, "--cache-type-k"); c != 1 {
		t.Errorf("--cache-type-k appears %d times, want 1: %v", c, args)
	}
	if argValue(args, "--cache-type-k") != "q4_0" {
		t.Errorf("extra_args should own cache-type: %v", args)
	}
	if !hasFlag(args, "--flash-attn") {
		t.Error("flash-attn should still be emitted")
	}
}

func TestBuildPortGroups_PropagatesTuning(t *testing.T) {
	groups, err := BuildPortGroups([]Agent{{
		Name: "architect", Model: "m.gguf", Port: 8086, Engine: "llama",
		Context: 2048, FlashAttn: true, RopeScaling: "yarn", KVCacheType: "q4_0,q8_0",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups", len(groups))
	}
	g := groups[0]
	if !g.FlashAttn || g.RopeScaling != "yarn" || g.KVCacheType != "q4_0,q8_0" {
		t.Errorf("tuning not propagated to PortGroup: %+v", g)
	}
}
