#pragma once
#include "proxy_configure_health.h"
#include "httplib.h"
#include "json.hpp"
#include <string>

using json = nlohmann::json;

inline std::string proxy_configure_check_docker_model_runner(const std::string& model) {
    if (model.empty())
        return "docker agent requires a non-empty model field"
               "\n  (e.g. ai/meta-llama-3.2-3b-instruct:Q8_0-F32)";

    httplib::Client cli("127.0.0.1", PROXY_CONFIGURE_DOCKER_PORT);
    cli.set_connection_timeout(3);
    cli.set_read_timeout(3);
    auto r = cli.Get("/v1/models");
    if (!r || r->status != 200)
        return "Docker Model Runner is not running on port "
             + std::to_string(PROXY_CONFIGURE_DOCKER_PORT)
             + ".\n  Start it with: docker model run " + model
             + "\n  Then relaunch the swarm.";

    const std::string& body = r->body;
    if (body.find(model) != std::string::npos) return "";

    std::string loaded;
    try {
        json j = json::parse(body);
        if (j.contains("data") && j["data"].is_array()) {
            for (const auto& m : j["data"]) {
                if (m.contains("id") && m["id"].is_string()) {
                    if (!loaded.empty()) loaded += ", ";
                    loaded += m["id"].get<std::string>();
                }
            }
        }
    } catch (...) {}

    return "Model '" + model + "' is not loaded in Docker Model Runner."
         + "\n  Run: docker model run " + model
         + (loaded.empty() ? "" : "\n  Currently loaded: " + loaded);
}
