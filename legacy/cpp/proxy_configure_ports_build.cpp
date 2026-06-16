#include "proxy_configure_ports.h"
#include "proxy_configure_ports_assign.h"
#include "config/path_expand.h"

#include <map>
#include <set>
#include <sys/socket.h>
#include <netinet/in.h>
#include <unistd.h>

PortBuildResult proxy_configure_build_port_groups(nlohmann::json agents) {
    PortBuildResult result;
    result.agents = std::move(agents);

    std::map<std::string, int> key_to_port;
    int next_port = 8080;
    std::set<int> fixed_ports;
    for (const auto& a : result.agents) {
        if (a.contains("port") && a["port"].is_number_integer()) {
            int p = a["port"].get<int>();
            if (p > 0) fixed_ports.insert(p);
        }
    }

    for (auto& a : result.agents) {
        if (a.contains("model") && a["model"].is_string())
            a["model"] = coordinator_config::expand_model_path(a["model"].get<std::string>());
        if (a.contains("draft_model") && a["draft_model"].is_string())
            a["draft_model"] = coordinator_config::expand_model_path(a["draft_model"].get<std::string>());
    }

    for (auto& a : result.agents) {
        if (a.value("coordinator", "") == "mlx") continue;

        std::string model = a["model"].get<std::string>();
        std::string sg    = a.value("server_group", "");
        std::string bk = a.contains("backend") && !a["backend"].get<std::string>().empty()
                         ? a["backend"].get<std::string>()
                         : a.contains("engine") && !a["engine"].get<std::string>().empty()
                           ? a["engine"].get<std::string>()
                           : std::string(ports_assign::ends_with_gguf(model) ? "llama" : "mlx");
        int fixed_port = a.contains("port") ? a["port"].get<int>() : -1;
        std::string key = ports_assign::assign_key(bk, model, sg, fixed_port);

        auto pick_port = [&](int preferred) -> int {
            if (preferred > 0 && ports_assign::is_port_available(preferred) && !fixed_ports.count(preferred))
                return preferred;
            while (fixed_ports.count(next_port) || !ports_assign::is_port_available(next_port)) ++next_port;
            return next_port++;
        };

        if (!key_to_port.count(key)) {
            if (bk == "docker") {
                key_to_port[key] = PROXY_CONFIGURE_DOCKER_PORT;
            } else if (fixed_port > 0) {
                // Explicit port in swarm config wins unconditionally for all backends.
                // Covers pre-running servers (llama/mlx/vllm) already bound to a port,
                // and lets operators pin servers to specific ports across restarts.
                key_to_port[key] = fixed_port;
            } else if (bk == "docker-vllm" || bk == "mlx" || bk == "vllm") {
                key_to_port[key] = pick_port(-1);
            } else {
                while (fixed_ports.count(next_port) || !ports_assign::is_port_available(next_port)) ++next_port;
                key_to_port[key] = next_port++;
            }
        }

        int port = key_to_port[key];
        a["port"] = port;
        if (!ports_assign::merge_group(result.pgs[port], a, bk, model, port, result))
            return result;
    }

    result.ok = true;
    result.status = 200;
    return result;
}
