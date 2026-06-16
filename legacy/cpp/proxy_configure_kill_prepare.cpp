#include "proxy_configure_kill_prepare.h"

#include "httplib.h"
#include "matrix_env.h"

#include <chrono>
#include <cstdlib>
#include <iostream>
#include <sys/stat.h>
#include <thread>
#include <unistd.h>

void proxy_configure_kill_old_and_prepare_dirs(const std::string& proj) {
    // Match coordinator by argv suffix (`coordinator --config`), not absolute
    // path — instances launched from the project dir won't match a pattern
    // containing the full project path.
    system("pkill -f llama-server 2>/dev/null");
    system("pkill -f 'llama_cpp.server' 2>/dev/null");
    system("pkill -f 'mlx_lm.server' 2>/dev/null");
    system("pkill -f 'vllm.entrypoints' 2>/dev/null");
    system("pkill -f 'docker model run' 2>/dev/null");
    system("pkill -f 'coordinator --config' 2>/dev/null");
    system("lsof -ti:8080,8081,8082,8083,8084,8085,8086,8087,8088,8089,8090 | xargs kill -9 2>/dev/null");

    // Wait for all inference ports to be free before spawning new servers.
    // A fixed sleep is not sufficient — stale processes can linger past it and
    // answer health checks with the wrong model.
    const int ports[] = {8080,8081,8082,8083,8084,8085,8086,8087,8088,8089,8090};
    auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(15);
    while (std::chrono::steady_clock::now() < deadline) {
        bool any_busy = false;
        for (int p : ports) {
            httplib::Client cli("127.0.0.1", p);
            cli.set_connection_timeout(1);
            cli.set_read_timeout(1);
            if (cli.Get("/health")) { any_busy = true; break; }
        }
        if (!any_busy) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(300));
    }
    mkdir(g_env.matrix_slots_dir.c_str(), 0755);
    mkdir((proj + "/logs").c_str(), 0755);
    mkdir((proj + "/agent_logs").c_str(), 0755);
}
