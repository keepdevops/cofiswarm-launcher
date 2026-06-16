#include "proxy_configure_ports.h"
#include "matrix_env.h"

#include <fstream>
#include <map>

bool proxy_configure_write_active_config(const nlohmann::json& agents,
                                         const std::map<int, PortGroup>& pgs,
                                         const std::string& proj,
                                         std::string& error_out) {
    try {
        nlohmann::json sc;
        const std::string preferred = proj + "/config/coordinator.json";
        const std::string legacy    = proj + "/swarm-config.json";
        std::ifstream sc_in(preferred);
        if (!sc_in.is_open()) {
            sc_in.open(legacy);
            if (!sc_in.is_open())
                throw std::runtime_error("Cannot open " + preferred + " or " + legacy);
        }
        sc = nlohmann::json::parse(sc_in);
        std::ofstream sc_out(g_env.active_config_path);
        if (!sc_out.is_open()) throw std::runtime_error("Cannot write " + g_env.active_config_path);

        std::map<int, int> port_slots;
        for (const auto& kv2 : pgs)
            if (kv2.second.backend == "llama")
                port_slots[kv2.first] = (int)kv2.second.names.size();

        nlohmann::json agents_out = agents;
        for (size_t ai = 0; ai < agents_out.size(); ++ai) {
            auto& a = agents_out[ai];
            if (!a.is_object()) continue;
            std::string bk;
            if (a.contains("backend") && a["backend"].is_string() && !a["backend"].get<std::string>().empty())
                bk = a["backend"].get<std::string>();
            else if (a.contains("engine") && a["engine"].is_string())
                bk = a["engine"].get<std::string>();
            else
                bk = "llama";
            if (bk != "llama") continue;
            int p = a.contains("port") && a["port"].is_number_integer() ? a["port"].get<int>() : -1;
            auto it = port_slots.find(p);
            if (it == port_slots.end()) continue;
            int cur = a.contains("max_concurrency") && a["max_concurrency"].is_number_integer()
                      ? a["max_concurrency"].get<int>() : 0;
            if (cur == 0) a["max_concurrency"] = it->second;
        }
        nlohmann::json active = {{"agents", agents_out}, {"coordinator", sc["coordinator"]}, {"ui", sc["ui"]}};
        if (sc.contains("rag")) active["rag"] = sc["rag"];
        sc_out << active.dump(2);
        return true;
    } catch (const std::exception& e) {
        error_out = e.what();
        return false;
    }
}
