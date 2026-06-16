#include "proxy_configure.h"
#include "proxy_configure_internal.h"
#include "proxy_configure_spawn_args.h"
#include "proxy_configure_spawn_detach.h"
#include "matrix_env.h"
#include <iostream>
#include <map>
#include <thread>
#include <chrono>

void spawn_detached(const std::string& bin,
                    const std::vector<std::string>& args,
                    const std::string& log_path,
                    bool use_path_search)
{
    spawn_detach::spawn_detached(bin, args, log_path, use_path_search);
}

void spawn_inference_servers(const std::map<int, PortGroup>& pgs,
                              const std::string& proj)
{
    int hf_n = 0;
    int llama_n = 0;
    for (const auto& [port, g] : pgs) {
        if (g.backend == "llama") {
            if (llama_n++ > 0) std::this_thread::sleep_for(std::chrono::seconds(5));
        }
        std::string log = proj + "/agent_logs/" + std::to_string(port) + ".log";
        std::string ps  = std::to_string(port);

        if (g.backend == "docker") {
            std::cout << "[Configure] DOCKER :" << port << " model=" << g.model
                      << " [" << spawn_detach::join_names(g.names) << "]\n";
        } else if (g.backend == "mlx") {
            if (hf_n++ > 0) std::this_thread::sleep_for(std::chrono::seconds(5));
            spawn_detach::spawn_detached(g_env.mlx_python,
                {"-m","mlx_lm","server","--model",g.model,"--port",ps,"--host","127.0.0.1"}, log);
            std::cout << "[Configure] MLX :" << port << " [" << spawn_detach::join_names(g.names) << "]\n";
        } else if (g.backend == "vllm") {
            if (hf_n++ > 0) std::this_thread::sleep_for(std::chrono::seconds(5));
            char gmu_buf[16];
            snprintf(gmu_buf, sizeof(gmu_buf), "%.2f", g.gpu_mem_util);
            spawn_detach::spawn_detached(g_env.vllm_python,
                {"-m","vllm.entrypoints.openai.api_server","--model",g.model,
                 "--port",ps,"--host","127.0.0.1","--max-model-len",std::to_string(g.context),
                 "--gpu-memory-utilization",std::string(gmu_buf)},
                log, /*use_path_search=*/false);
            std::cout << "[Configure] vLLM :" << port << " gpu_mem=" << gmu_buf
                      << " python=" << g_env.vllm_python
                      << " [" << spawn_detach::join_names(g.names) << "]\n";
        } else if (g.backend == "docker-vllm") {
            char gmu_buf[16];
            snprintf(gmu_buf, sizeof(gmu_buf), "%.2f", g.gpu_mem_util);
            spawn_detach::spawn_detached("docker",
                {"model","run",g.model,"--backend","vllm","--port",ps,
                 "--gpu-memory-utilization",std::string(gmu_buf),
                 "--max-model-len",std::to_string(g.context)},
                log, /*use_path_search=*/true);
            std::cout << "[Configure] DOCKER-vLLM :" << port << " gpu_mem=" << gmu_buf
                      << " [" << spawn_detach::join_names(g.names) << "]\n";
        } else {
            int ctx = 0;
            auto args = spawn_args::build_llama_args(g, ps, ctx);
            spawn_detach::spawn_detached(g_env.llama_server_bin, args, log);
            std::cout << "[Configure] LLAMA :" << port << " ctx=" << ctx
                      << (g.flash_attn ? " flash_attn+kv_q8" : "")
                      << " x" << g.names.size()
                      << " [" << spawn_detach::join_names(g.names) << "]"
                      << (g.draft_model.empty() ? ""
                          : " spec=" + g.draft_model
                            + (g.draft_max > 0 ? "/" + std::to_string(g.draft_max) : ""))
                      << "\n";
        }
    }
}
