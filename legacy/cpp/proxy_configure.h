#pragma once
#include "json.hpp"
#include <string>
#include <vector>

using json = nlohmann::json;

struct ConfigureResult {
    bool ok          = false;
    int  http_status = 200;
    json body;
};

// Spawn bin with args as a detached process; stdout/stderr appended to log_path.
// Uses posix_spawn (safe in multi-threaded programs, unlike fork).
void spawn_detached(const std::string& bin,
                    const std::vector<std::string>& args,
                    const std::string& log_path,
                    bool use_path_search = false);

// Groups agents by (backend:model:server_group), kills old servers, starts
// inference servers, waits up to 240 s for health, then starts the coordinator.
// Returns {ok:true, http_status:200, body:{status:"ok",servers:[...]}} on success.
// Returns {ok:false, http_status:503, body:{error:..., failedPorts:[...]}} on timeout.
ConfigureResult handle_configure(const json& request_body,
                                 const std::string& project_root);
