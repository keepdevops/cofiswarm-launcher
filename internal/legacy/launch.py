"""brewctl launch — start proxy + UI (npm start) in background.

Sources scripts/matrix-env.sh if present so model paths and conda env vars
are inherited just like the bash launcher.
"""
from __future__ import annotations

import logging
import os
import shutil
import subprocess
import time
from pathlib import Path

logger = logging.getLogger(__name__)

REPO = Path(__file__).resolve().parents[2]


def _source_env_file(path: Path) -> dict[str, str]:
    """Run `bash -c 'source <path> && env'` to capture exported vars."""
    if not path.is_file():
        return {}
    try:
        out = subprocess.run(
            ["bash", "-c", f"set -a; source {path}; env"],
            capture_output=True, text=True, check=True,
        )
    except subprocess.CalledProcessError as exc:
        logger.error("sourcing %s failed: %s", path, exc)
        return {}
    env: dict[str, str] = {}
    for line in out.stdout.splitlines():
        if "=" not in line:
            continue
        k, v = line.split("=", 1)
        env[k] = v
    return env


def _spawn(cmd: list[str], log_path: Path, env: dict[str, str]) -> int:
    log_path.parent.mkdir(parents=True, exist_ok=True)
    log_fp = open(log_path, "ab")
    proc = subprocess.Popen(
        cmd,
        stdout=log_fp,
        stderr=subprocess.STDOUT,
        cwd=REPO,
        env=env,
        start_new_session=True,  # detach from this terminal
    )
    return proc.pid


def _already_running(pid_file: Path) -> list[int]:
    """Return live PIDs from a previous launch, or empty list if none."""
    if not pid_file.is_file():
        return []
    live: list[int] = []
    for line in pid_file.read_text().splitlines():
        line = line.strip()
        if not line.isdigit():
            continue
        pid = int(line)
        try:
            os.kill(pid, 0)
            live.append(pid)
        except ProcessLookupError:
            pass
        except PermissionError:
            live.append(pid)  # process exists, owned by another user
    return live


def run_launch() -> int:
    print("=" * 60)
    print("SWARM MATRIX starting")
    print(f"ROOT = {REPO}")

    env = dict(os.environ)
    env_overlay = _source_env_file(REPO / "scripts" / "matrix-env.sh")
    env.update(env_overlay)

    # Directory holding the coordinator/proxy binaries. Defaults to the repo
    # root; MATRIX_BIN_DIR (e.g. exported from matrix-env.sh) overrides it so
    # the stack can run a parked build such as zzzzzz/. Relative paths resolve
    # against REPO.
    _bin = env.get("MATRIX_BIN_DIR", "").strip()
    if _bin:
        bin_dir = Path(_bin).expanduser()
        if not bin_dir.is_absolute():
            bin_dir = REPO / bin_dir
    else:
        bin_dir = REPO
    print(f"Binary dir: {bin_dir}")

    logs = REPO / "logs"
    logs.mkdir(parents=True, exist_ok=True)
    pid_file = logs / "matrix.pids"

    live = _already_running(pid_file)
    if live:
        print(f"FATAL: Matrix Swarm is already running (pids={live}).")
        print("       Run 'brewctl shutdown' first, then relaunch.")
        logger.error("launch aborted — existing instance pids=%s", live)
        return 1

    pid_file.write_text("")  # reset

    proxy_bin = bin_dir / "proxy"
    if not proxy_bin.is_file() or not os.access(proxy_bin, os.X_OK):
        logger.error("proxy binary missing or not executable: %s", proxy_bin)
        print(f"FATAL: {proxy_bin} not found — run scripts/build_cpp_binaries.sh")
        return 2

    # Auto-rebuild proxy if any C++ source is newer than the binary.
    cpp_src = REPO / "cpp_core" / "src"
    if cpp_src.is_dir():
        binary_mtime = proxy_bin.stat().st_mtime
        stale_sources = [
            p for p in cpp_src.rglob("*")
            if p.suffix in (".cpp", ".h") and p.stat().st_mtime > binary_mtime
        ]
        if stale_sources:
            print(f"Proxy sources changed ({len(stale_sources)} file(s)) — rebuilding ...")
            build_script = REPO / "scripts" / "build_cpp_binaries.sh"
            rc = subprocess.run(["bash", str(build_script)], cwd=REPO, env=env).returncode
            if rc != 0:
                logger.error("auto-rebuild failed rc=%d", rc)
                print("FATAL: proxy rebuild failed — check cpp_core build errors")
                return 2
            print("Rebuild complete.")

    # Kill any stale proxy on port 3002 so the freshest binary always runs.
    # This also handles the case where the binary was rebuilt after last launch.
    from ._proc import lsof_pids_on_port, kill_pids
    stale = lsof_pids_on_port(3002)
    if stale:
        print(f"Stopping stale proxy (pid={stale}) ...")
        survivors = kill_pids(stale)
        if survivors:
            logger.error("stale proxy pids still alive after kill: %s", survivors)
        else:
            time.sleep(0.5)  # let port unbind

    coord_bin = bin_dir / "coordinator"
    if coord_bin.is_file() and os.access(coord_bin, os.X_OK):
        stale_coord = lsof_pids_on_port(int(env.get("MATRIX_COORDINATOR_PORT", "8000")))
        if stale_coord:
            print(f"Stopping stale coordinator (pid={stale_coord}) ...")
            survivors = kill_pids(stale_coord)
            if not survivors:
                time.sleep(0.5)
        print("Starting coordinator ...")
        coord_pid = _spawn([str(coord_bin)], logs / "coordinator.log", env)
        with pid_file.open("a") as f:
            f.write(f"{coord_pid}\n")
        time.sleep(1.0)  # coordinator must bind :8000 before proxy starts forwarding
    else:
        logger.warning("coordinator binary missing — MLX and inference routes unavailable")

    print("Starting proxy ...")
    proxy_pid = _spawn([str(proxy_bin)], logs / "proxy.log", env)
    with pid_file.open("a") as f:
        f.write(f"{proxy_pid}\n")

    # CRA + WDS v5 patch shim — same call the bash launcher made.
    patch_script = REPO / "scripts" / "ensure-react-scripts-patch.mjs"
    if patch_script.is_file() and shutil.which("node"):
        rc = subprocess.run(["node", str(patch_script)], cwd=REPO, env=env).returncode
        if rc != 0:
            logger.error("ensure-react-scripts-patch.mjs failed rc=%d", rc)
            print("FATAL: react-scripts patch failed — run npm install")
            return 3

    # MS-144: Python mlx-coordinator (:3003) decommissioned for /api/mlx/*.
    # MS-142: thin orchestrate sidecar still needed for /api/orchestrate*.
    # Kill any stale :3003 process before starting the fresh sidecar.
    orch_port = int(env.get("ORCH_SIDECAR_PORT", "3003"))
    stale_mlx = lsof_pids_on_port(orch_port)
    if stale_mlx:
        print(f"Stopping stale process on :{orch_port} ...")
        survivors = kill_pids(stale_mlx)
        if survivors:
            logger.warning("stale :%d pids still alive: %s", orch_port, survivors)
        else:
            time.sleep(0.3)

    python = shutil.which("python3") or shutil.which("python") or "python3"
    print(f"Starting orchestrate sidecar (:{orch_port}) ...")
    sidecar_pid = _spawn(
        [python, "-m", "orchestration.mlx_coordinator.sidecar", "--port", str(orch_port)],
        logs / "orch_sidecar.log",
        env,
    )
    with pid_file.open("a") as f:
        f.write(f"{sidecar_pid}\n")
    time.sleep(0.5)  # give sidecar time to bind before proxy starts forwarding

    print("Starting UI (npm start) ...")
    npm = shutil.which("npm") or "npm"
    ui_pid = _spawn([npm, "start"], logs / "ui.log", env)
    with pid_file.open("a") as f:
        f.write(f"{ui_pid}\n")

    print(f"proxy pid={proxy_pid}  ui pid={ui_pid}")
    print("SWARM MATRIX started -> http://localhost:3000")
    print("=" * 60)
    return 0
