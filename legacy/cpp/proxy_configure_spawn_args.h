#pragma once
// Inline argument-vector builders for spawn_inference_servers.
// Included only by proxy_configure_spawn.cpp.

#include "proxy_configure_internal.h"
#include "matrix_env.h"
#include <iostream>
#include <string>
#include <vector>

namespace spawn_args {

inline std::vector<std::string> build_llama_args(const PortGroup& g,
                                                   const std::string& ps,
                                                   int& ctx_out) {
    // Slots = explicit per-server override, else one slot per agent. Total ctx
    // scales with slots (KV is allocated per slot), so capping slots below the
    // agent count also shrinks KV memory — extra agents queue on shared slots.
    const int slots = g.parallel > 0 ? g.parallel : (int)g.names.size();
    int ctx = g.context * slots;
    if (ctx > g.ctx_cap) {
        std::cerr << "[Configure] WARNING: effective ctx "
                  << ctx << " exceeds cap " << g.ctx_cap
                  << " on port " << ps << "; truncating. "
                  << "Lower per-agent 'context' or set 'ctx_cap' "
                  << "in agent config to suppress." << std::endl;
        ctx = g.ctx_cap;
    }
    ctx_out = ctx;
    // --fit off: llama.cpp b8763 has a contiguity assert bug in the
    // automatic param-fitting path (ggml_reshape_2d); disable it.
    std::vector<std::string> args = {
        "-m", g.model, "-c", std::to_string(ctx), "--port", ps,
        "--n-gpu-layers", std::to_string(g.gpu_layers),
        "--parallel", std::to_string(slots),
        "--metrics", "--slot-save-path", g_env.matrix_slots_dir,
        "--fit", "off"
    };
    if (g.flash_attn) {
        args.push_back("--flash-attn"); args.push_back("on");
        args.push_back("--cache-type-k"); args.push_back("q8_0");
        args.push_back("--cache-type-v"); args.push_back("q8_0");
    }
    if (g.n_batch > 0) {
        args.push_back("--batch-size");
        args.push_back(std::to_string(g.n_batch));
    }
    if (!g.draft_model.empty()) {
        args.push_back("--model-draft");
        args.push_back(g.draft_model);
        if (g.draft_max > 0) {
            args.push_back("--draft-max");
            args.push_back(std::to_string(g.draft_max));
        }
    }
    for (const auto& ea : g.extra_args) args.push_back(ea);
    return args;
}

} // namespace spawn_args
