# Launch matrix (from bin/matrix.mjs)

Sprint 7 replaces `posix_spawn` process trees with:

1. **Host infer** — llama-server / mlx via FHS profiles (`cofiswarm-infer-*`)
2. **Docker compose** — `cofiswarm-launcher profile up 8gb` (nats bus + future control-plane; RAG is serverless sqlite-vec)
3. **brewctl** — `scripts/brewctl` delegates to `internal/legacy/` until full port

Reference: `spec/launch-matrix.mjs.reference`
