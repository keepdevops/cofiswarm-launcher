#pragma once
#include <iostream>
#include <string>
#include <vector>
#include <cstring>
#include <fcntl.h>
#include <unistd.h>
#include <spawn.h>
#if defined(__APPLE__)
#include <crt_externs.h>
#else
// POSIX global, defined by libc. Declared at global scope (not inside the
// namespace, where `extern char** environ;` would bind to spawn_detach::environ
// and fail to link) so it resolves to ::environ.
extern char** environ;
#endif

namespace spawn_detach {

inline char** environ_ptr() {
#if defined(__APPLE__)
    return *_NSGetEnviron();
#else
    return environ;
#endif
}

inline std::string join_names(const std::vector<std::string>& v) {
    std::string r;
    for (size_t i = 0; i < v.size(); ++i) { if (i) r += ", "; r += v[i]; }
    return r;
}

inline void spawn_detached(const std::string& bin,
                            const std::vector<std::string>& args,
                            const std::string& log_path,
                            bool use_path_search = false)
{
    int fd = open(log_path.c_str(), O_WRONLY | O_CREAT | O_APPEND, 0644);
    if (fd < 0) fd = open("/dev/null", O_WRONLY);

    posix_spawn_file_actions_t fa;
    posix_spawn_file_actions_init(&fa);
    posix_spawn_file_actions_addclose(&fa, STDIN_FILENO);
    posix_spawn_file_actions_adddup2(&fa, fd, STDOUT_FILENO);
    posix_spawn_file_actions_adddup2(&fa, fd, STDERR_FILENO);

    posix_spawnattr_t attr;
    posix_spawnattr_init(&attr);
    posix_spawnattr_setflags(&attr, POSIX_SPAWN_SETSID);

    std::vector<char*> argv_ptrs;
    argv_ptrs.push_back(const_cast<char*>(bin.c_str()));
    for (const auto& a : args) argv_ptrs.push_back(const_cast<char*>(a.c_str()));
    argv_ptrs.push_back(nullptr);

    pid_t pid = -1;
    int rc = use_path_search
        ? posix_spawnp(&pid, bin.c_str(), &fa, &attr, argv_ptrs.data(), environ_ptr())
        : posix_spawn (&pid, bin.c_str(), &fa, &attr, argv_ptrs.data(), environ_ptr());
    if (rc != 0) std::cerr << "[spawn] " << bin << ": " << strerror(rc) << "\n";

    posix_spawn_file_actions_destroy(&fa);
    posix_spawnattr_destroy(&attr);
    close(fd);
}

} // namespace spawn_detach
