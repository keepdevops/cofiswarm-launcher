#pragma once
#include "proxy_configure_ports.h"
#include <algorithm>
#include <iostream>
#include <set>
#include <sys/socket.h>
#include <netinet/in.h>
#include <unistd.h>
#include <string>

namespace ports_assign {

inline bool is_port_available(int port) {
    int fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) return false;
    int opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
    struct sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(port));
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    bool ok = bind(fd, reinterpret_cast<struct sockaddr*>(&addr), sizeof(addr)) == 0;
    close(fd);
    return ok;
}

inline bool ends_with_gguf(const std::string& s) {
    return s.size() > 5 && s.compare(s.size() - 5, 5, ".gguf") == 0;
}

inline std::string assign_key(const std::string& bk, const std::string& model,
                               const std::string& sg, int fixed_port) {
    if (bk == "docker") return "docker:shared";
    if ((bk == "docker-vllm") && fixed_port > 0) return "docker-vllm:" + std::to_string(fixed_port);
    if ((bk == "mlx" || bk == "vllm") && fixed_port > 0) return bk + ":" + std::to_string(fixed_port);
    // Llama: group by model path only. server_group is a UI label; splitting the
    // same GGUF across groups duplicates ~18GB+ GPU loads and causes Metal OOM.
    // (merge_group only checks backend+model, so model-only keying is consistent.)
    if (bk == "llama") return bk + ":" + model;
    return bk + ":" + model + ":" + sg;
}

inline bool merge_group(PortGroup& g, const nlohmann::json& a,
                         const std::string& bk, const std::string& model,
                         int port, PortBuildResult& result) {
    int agent_n_batch = a.value("n_batch", 0);
    if (g.model.empty()) {
        float gmu = a.value("gpu_mem_util", 0.75f);
        int default_gpu_layers = (bk == "llama") ? 99 : 0;
        g = {model, bk, a["context"].get<int>(), a.value("gpu_layers", default_gpu_layers), 0, gmu, {}, "", 0};
        g.n_batch = agent_n_batch;
    } else {
        if (g.backend != bk || g.model != model) {
            result.ok = false;
            result.status = 400;
            result.body = {
                {"error", "Port " + std::to_string(port)
                    + " is assigned to incompatible servers. Put agents that use different backends or models on different ports."},
                {"port", port},
                {"existing_backend", g.backend}, {"existing_model", g.model},
                {"agent", a["name"].get<std::string>()},
                {"agent_backend", bk}, {"agent_model", model}
            };
            return false;
        }
        g.context = std::max(g.context, a["context"].get<int>());
        if (agent_n_batch > 0)
            g.n_batch = (g.n_batch == 0) ? agent_n_batch : std::min(g.n_batch, agent_n_batch);
    }
    g.names.push_back(a["name"].get<std::string>());
    if (a.value("flash_attn", false)) g.flash_attn = true;
    if (a.contains("extra_args") && a["extra_args"].is_array()) {
        for (const auto& arg : a["extra_args"]) {
            if (arg.is_string()) {
                const std::string s = arg.get<std::string>();
                if (std::find(g.extra_args.begin(), g.extra_args.end(), s) == g.extra_args.end())
                    g.extra_args.push_back(s);
            }
        }
    }
    if (a.contains("ctx_cap") && a["ctx_cap"].is_number_integer()) {
        int agent_cap = a["ctx_cap"].get<int>();
        if (agent_cap > 0) g.ctx_cap = std::min(g.ctx_cap, agent_cap);
    }
    // Per-server --parallel override (0/absent = auto = agent count). Agents
    // sharing a model should carry the same value; take the max if they differ.
    if (a.contains("parallel") && a["parallel"].is_number_integer()) {
        int p = a["parallel"].get<int>();
        if (p > 0) g.parallel = std::max(g.parallel, p);
    }
    if (bk == "llama") {
        std::string dm = a.value("draft_model", std::string(""));
        int dmax = a.value("draft_max", 0);
        if (!dm.empty()) {
            if (g.draft_model.empty()) {
                g.draft_model = dm;
                g.draft_max = dmax;
            } else if (g.draft_model != dm) {
                std::cerr << "[Configure] WARNING: agent '" << a["name"].get<std::string>()
                          << "' on port " << port << " requested draft_model='" << dm
                          << "' but port already uses '" << g.draft_model << "'; ignoring." << std::endl;
            }
        }
    }
    return true;
}

}  // namespace ports_assign
