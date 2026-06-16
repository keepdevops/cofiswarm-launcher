#include "proxy_configure.h"
#include "proxy_configure_internal.h"
#include "proxy_configure_ports.h"
#include "proxy_configure_health.h"
#include "proxy_configure_kill_prepare.h"
#include "proxy_configure_coordinator_startup.h"
#include "proxy_configure_health_docker.h"
#include "proxy_validate.h"
#include "proxy_validate_vllm.h"
#include "matrix_env.h"

#include <iostream>
#include <string>

namespace {

bool ends_with_gguf(const std::string& s) {
    return s.size() > 5 && s.compare(s.size() - 5, 5, ".gguf") == 0;
}

}  // namespace

ConfigureResult handle_configure(const json& request_body, const std::string& proj) {
    if (!request_body.contains("agents") || !request_body["agents"].is_array()
        || request_body["agents"].empty())
        return {false, 400, {{"error", "agents array required"}}};

    auto built = proxy_configure_build_port_groups(request_body["agents"]);
    if (!built.ok)
        return {false, built.status, built.body};

    std::string config_err;
    if (!proxy_configure_write_active_config(built.agents, built.pgs, proj, config_err))
        return {false, 500, {{"error", config_err}}};

    g_configure_progress.reset(built.pgs);
    proxy_configure_kill_old_and_prepare_dirs(proj);

    for (const auto& [port, g] : built.pgs) {
        std::string err;
        if (g.backend == "llama")       err = validate_llama_model(g.model);
        else if (g.backend == "mlx")    err = validate_mlx_model(g.model, g_env.mlx_python);
        else if (g.backend == "vllm")   err = validate_vllm_model(g.model, g_env.vllm_python, g.context);
        else if (g.backend == "docker-vllm") err = validate_docker_vllm_model(g.model);
        else if (g.backend == "docker") err = proxy_configure_check_docker_model_runner(g.model);
        if (!err.empty()) {
            std::cerr << "[Configure] Pre-flight failed port " << port << ": " << err << "\n";
            g_configure_progress.active.store(false);
            return {false, 400, {{"error", err}, {"port", port}, {"model", g.model}}};
        }
    }

    spawn_inference_servers(built.pgs, proj);

    int health_timeout = 240;
    for (const auto& kv : built.pgs) {
        if (kv.second.backend == "docker-vllm") { health_timeout = 600; break; }
    }
    auto failed = proxy_configure_wait_for_health(built.pgs, health_timeout);
    if (!failed.empty()) {
        json fa = json::array();
        std::string fl;
        for (int p : failed) { fa.push_back(p); if (!fl.empty()) fl += ", "; fl += std::to_string(p); }
        std::cerr << "[Configure] Health timeout. Ports not ready: " << fl << "\n";
        g_configure_progress.active.store(false);
        std::string hint = ". MLX can take 1-2 min per model on first load.";
        for (int p : failed) {
            auto it = built.pgs.find(p);
            if (it != built.pgs.end() && it->second.backend == "llama") {
                hint = ". Llama ports must pass a /completion smoke test — see agent_logs/"
                       + std::to_string(p)
                       + ".log for 'Insufficient Memory' or 'Compute error' "
                         "(duplicate large models or too many agents for unified memory).";
                break;
            }
        }
        return {false, 503, {
            {"error", "Servers failed to become healthy within several minutes. Check agent_logs/"
                      + std::to_string(failed[0]) + ".log. Ports not ready: " + fl + hint},
            {"failedPorts", fa}
        }};
    }

    proxy_configure_spawn_coordinator(proj);

    json servers = json::array();
    for (const auto& [port, g] : built.pgs) {
        std::string mn = g.model;
        if (auto sl = mn.rfind('/'); sl != std::string::npos) mn = mn.substr(sl + 1);
        if (ends_with_gguf(mn)) mn = mn.substr(0, mn.size() - 5);
        json na = json::array(); for (auto& n : g.names) na.push_back(n);
        servers.push_back({{"port",port},{"model",mn},{"agents",na},{"parallel",(int)g.names.size()}});
    }
    std::cout << "[Configure] Swarm online: " << servers.size() << " server(s)\n";
    g_configure_progress.active.store(false);
    return {true, 200, {{"status","ok"},{"servers",servers}}};
}
