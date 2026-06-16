#pragma once

#include "proxy_configure_internal.h"
#include "json.hpp"

#include <atomic>
#include <map>
#include <mutex>
#include <string>
#include <vector>

using json = nlohmann::json;

// Thread-safe per-port launch progress shared between the health-wait loop
// and the GET /api/configure/status route.
struct ConfigureProgress {
    mutable std::mutex mu;
    std::map<int, std::string> ports;  // port → "pending" | "ready" | "error"
    std::atomic<bool> active{false};

    void reset(const std::map<int, PortGroup>& pgs) {
        std::lock_guard<std::mutex> lk(mu);
        ports.clear();
        for (const auto& [port, _] : pgs) ports[port] = "pending";
        active.store(true);
    }

    void set(int port, const std::string& state) {
        std::lock_guard<std::mutex> lk(mu);
        ports[port] = state;
    }

    json to_json() const {
        std::lock_guard<std::mutex> lk(mu);
        json p = json::object();
        for (const auto& [port, state] : ports)
            p[std::to_string(port)] = state;
        return {{"active", active.load()}, {"ports", p}};
    }
};

extern ConfigureProgress g_configure_progress;

std::vector<int> proxy_configure_wait_for_health(
    const std::map<int, PortGroup>& pgs, int timeout_secs);

std::string proxy_configure_check_docker_model_runner(const std::string& model);
