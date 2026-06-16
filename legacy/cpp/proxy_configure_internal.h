#pragma once

#include <string>
#include <vector>

struct PortGroup {
    std::string model, backend;
    int context = 0, gpu_layers = 0, n_batch = 0;
    float gpu_mem_util = 0.75f;
    std::vector<std::string> names;
    std::string draft_model;
    int draft_max = 0;
    bool flash_attn = false; // --flash-attn + KV q8_0 quantization
    int ctx_cap = 65536;     // max total context across all slots (replaces hardcoded 16384)
    std::vector<std::string> extra_args; // appended verbatim to llama-server command line
    int parallel = 0;        // per-server --parallel override (0 = auto = agent count)
};

inline constexpr int PROXY_CONFIGURE_DOCKER_PORT = 12434;

#include <map>

// Spawn all inference server processes for the given port groups.
// Defined in proxy_configure_spawn.cpp.
void spawn_inference_servers(const std::map<int, PortGroup>& pgs,
                              const std::string& proj);
