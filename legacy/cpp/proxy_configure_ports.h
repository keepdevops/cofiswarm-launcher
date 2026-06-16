#pragma once

#include "json.hpp"
#include "proxy_configure_internal.h"

#include <map>
#include <string>

struct PortBuildResult {
    bool ok = false;
    int status = 500;
    nlohmann::json body = nlohmann::json::object();
    nlohmann::json agents;
    std::map<int, PortGroup> pgs;
};

PortBuildResult proxy_configure_build_port_groups(nlohmann::json agents);

bool proxy_configure_write_active_config(const nlohmann::json& agents,
                                         const std::map<int, PortGroup>& pgs,
                                         const std::string& proj,
                                         std::string& error_out);
