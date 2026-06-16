"""brewctl shutdown — stop UI, proxy, coordinator, llama/mlx workers."""
from __future__ import annotations

import logging
import os
import subprocess
from pathlib import Path

from ._proc import kill_pids, lsof_pids_on_port, pgrep_pids

logger = logging.getLogger(__name__)

PATTERNS = [
    ("npm.*start", "npm start (UI)"),
    ("serve.*3000", "static serve"),
    ("matrix-project/proxy", "proxy"),
    ("matrix-project/coordinator", "coordinator"),
    ("llama-server", "llama-server"),
    ("llama_server", "llama_server"),
    ("mlx_lm", "mlx_lm"),
    ("mlx-lm", "mlx-lm"),
    # MS-142: orchestrate sidecar replaces Python MLX coordinator on :3003
    ("orchestration.mlx_coordinator.sidecar", "orchestrate sidecar"),
]

# Port 3003 is the orchestrate sidecar (MS-142); 8001 is RAG ingest
PORTS = [3000, 3002, 3003, 8000, 8001]

LAUNCHD_PLIST = Path.home() / "Library/LaunchAgents/com.xxxxx.swarm-dashboard.plist"
DISABLED_PLIST = """<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.xxxxx.swarm-dashboard</string>
    <key>Disabled</key>
    <true/>
</dict>
</plist>
"""


def run_shutdown() -> int:
    print("=" * 60)
    for pattern, label in PATTERNS:
        pids = pgrep_pids(pattern)
        if not pids:
            print(f"  - no process matching {pattern!r}")
            continue
        print(f"  stopping {label}: pids={pids}")
        survivors = kill_pids(pids)
        if survivors:
            logger.error("could not kill %s: %s", label, survivors)
            print(f"  ! still alive: {survivors}")

    for port in PORTS:
        pids = lsof_pids_on_port(port)
        if not pids:
            continue
        print(f"  port {port} still holds pids={pids}; killing")
        kill_pids(pids)

    LAUNCHD_PLIST.parent.mkdir(parents=True, exist_ok=True)
    LAUNCHD_PLIST.write_text(DISABLED_PLIST)
    subprocess.run(["launchctl", "unload", str(LAUNCHD_PLIST)],
                   capture_output=True, check=False)

    print("-" * 60)
    blocked = lsof_pids_on_port(3000)
    if blocked:
        print(f"  ! port 3000 still blocked by {blocked}")
        return 1
    print("  port 3000 free")
    print("Shutdown complete.")
    print("=" * 60)
    return 0
