#include "proxy_configure_coordinator_startup.h"

#include "matrix_env.h"
#include "proxy_configure.h"

#include "httplib.h"

#include <chrono>
#include <cstdlib>
#include <iostream>
#include <thread>

void proxy_configure_spawn_coordinator(const std::string& proj) {
    setenv("MATRIX_SOURCE_CONFIG", (proj + "/swarm-config.json").c_str(), 1);
    spawn_detached(proj + "/coordinator", {"--config", g_env.active_config_path},
                   proj + "/agent_logs/coordinator.log");

    auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(8);
    bool coord_ready = false;
    while (std::chrono::steady_clock::now() < deadline) {
        std::this_thread::sleep_for(std::chrono::milliseconds(300));
        try {
            httplib::Client cli("127.0.0.1", g_env.coordinator_port);
            cli.set_connection_timeout(1);
            cli.set_read_timeout(2);
            auto r = cli.Get("/api/health");
            if (r && r->status == 200) {
                coord_ready = true;
                break;
            }
        } catch (...) {}
    }
    if (!coord_ready) {
        std::cerr << "[Configure] Warning: coordinator did not respond within 8 s — "
                     "UI health poll may show offline briefly.\n";
    }
}
