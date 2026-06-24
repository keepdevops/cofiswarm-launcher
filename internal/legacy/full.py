"""brewctl up / down — orchestrate the full stack (RAG sidecar + UI).

RAG is serverless now: the store is a local sqlite-vec .db file the ingest
sidecar writes directly, so there is no database container to boot/stop.

up:    spawn rag ingest sidecar → auto-index → run_launch
down:  run_shutdown → kill sidecar :8001
"""
from __future__ import annotations

import json
import logging
import os
import subprocess
import sys
import time
from pathlib import Path

from ._proc import kill_pids, lsof_pids_on_port
from .launch import run_launch
from .shutdown import run_shutdown

logger = logging.getLogger(__name__)

REPO = Path(__file__).resolve().parents[2]
SIDECAR_PORT = 8001
SIDECAR_SCRIPT = REPO / "scripts" / "rag-ingest-server.py"
COORD_CONFIG = REPO / "config" / "coordinator.json"
VALID_EMBEDDERS = {"hash", "mlx", "nomic"}


def rag_embedder() -> str:
    """Single source of truth for the RAG embedder.

    The ingest sidecar (which also serves /embed for query-time embedding), the
    background auto-index, and the coordinator's query path must all use the same
    embedder — otherwise ingested documents and queries land in different 768-d
    vector spaces and retrieval silently returns irrelevant chunks (no dimension
    error is raised). Resolution order:
        RAG_INGEST_EMBEDDER env → config/coordinator.json rag.embedder → 'mlx'.
    """
    env = os.environ.get("RAG_INGEST_EMBEDDER")
    if env in VALID_EMBEDDERS:
        return env
    try:
        cfg = json.loads(COORD_CONFIG.read_text())
        emb = (cfg.get("rag") or {}).get("embedder")
        if emb in VALID_EMBEDDERS:
            return emb
        logger.error("rag: %s rag.embedder=%r invalid; falling back to 'mlx'",
                     COORD_CONFIG, emb)
    except Exception as exc:
        logger.error("rag: could not read embedder from %s: %s; using 'mlx'",
                     COORD_CONFIG, exc)
    return "mlx"


def _wait_port(port: int, timeout: float = 15.0) -> bool:
    import socket
    end = time.time() + timeout
    while time.time() < end:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(0.3)
            try:
                s.connect(("127.0.0.1", port))
                return True
            except OSError:
                time.sleep(0.2)
    return False


def _sidecar_up() -> int:
    if not SIDECAR_SCRIPT.is_file():
        logger.error("sidecar script missing at %s", SIDECAR_SCRIPT)
        print(f"FATAL: {SIDECAR_SCRIPT} not found")
        return 2
    if lsof_pids_on_port(SIDECAR_PORT):
        print(f"Sidecar already running on :{SIDECAR_PORT}, skipping spawn")
        return 0
    embedder = rag_embedder()
    print(f"Starting RAG ingest sidecar on :{SIDECAR_PORT} (embedder={embedder}) ...")
    logs = REPO / "logs"
    logs.mkdir(parents=True, exist_ok=True)
    log_fp = open(logs / "rag-sidecar.log", "ab")
    # Pin the embedder explicitly (flag is authoritative; env is belt-and-suspenders
    # for any child that reads RAG_INGEST_EMBEDDER) so the sidecar's /embed matches
    # the embedder used to index documents.
    env = os.environ.copy()
    env["RAG_INGEST_EMBEDDER"] = embedder
    proc = subprocess.Popen(
        [sys.executable, str(SIDECAR_SCRIPT), "--embedder", embedder],
        cwd=REPO, env=env,
        stdout=log_fp, stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    if not _wait_port(SIDECAR_PORT, timeout=15.0):
        logger.error("sidecar did not bind :%d (pid=%d)", SIDECAR_PORT, proc.pid)
        print(f"FATAL: sidecar pid={proc.pid} never bound :{SIDECAR_PORT}")
        print(f"  See logs/rag-sidecar.log")
        return 3
    print(f"  sidecar pid={proc.pid} ready")
    return 0


def _auto_index(index_path: Path) -> None:
    """Kick off background re-index of index_path using the configured embedder.

    Uses the same embedder as the sidecar/coordinator (rag_embedder()) so indexed
    documents and query embeddings share one vector space.
    """
    brewctl = REPO / "scripts" / "brewctl"
    logs = REPO / "logs"
    logs.mkdir(parents=True, exist_ok=True)
    log_fp = open(logs / "rag-autoindex.log", "ab")
    cmd = [sys.executable, str(brewctl), "rag", "index", str(index_path),
           "--embedder", rag_embedder()]
    proc = subprocess.Popen(
        cmd, cwd=REPO, env=os.environ.copy(),
        stdout=log_fp, stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    print(f"  auto-index pid={proc.pid} indexing {index_path} → logs/rag-autoindex.log")


def run_up(no_rag: bool = False, no_index: bool = False, index_path: Path | None = None) -> int:
    print("=" * 60)
    print("SWARM MATRIX up" + (" (no-rag)" if no_rag else ""))
    if not no_rag:
        rc = _sidecar_up()
        if rc != 0:
            return rc
        if not no_index:
            target = index_path or REPO
            print(f"Auto-indexing {target} in background ...")
            _auto_index(target)
    return run_launch()


def _sidecar_down() -> None:
    pids = lsof_pids_on_port(SIDECAR_PORT)
    if not pids:
        print(f"  no sidecar on :{SIDECAR_PORT}")
        return
    print(f"  stopping sidecar :{SIDECAR_PORT} pids={pids}")
    survivors = kill_pids(pids)
    if survivors:
        logger.error("sidecar pids still alive: %s", survivors)


def run_down(full: bool = False) -> int:
    # `full` retained for CLI compatibility; RAG has no container to stop now
    # (the sqlite-vec .db file persists on disk between runs).
    _ = full
    rc = run_shutdown()
    print("-" * 60)
    _sidecar_down()
    return rc
