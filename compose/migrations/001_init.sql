CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS chunks (
    id          BIGSERIAL PRIMARY KEY,
    source_path TEXT        NOT NULL,
    chunk_idx   INT         NOT NULL,
    content     TEXT        NOT NULL,
    embedding   vector(768) NOT NULL,
    metadata    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(source_path, chunk_idx)
);

CREATE INDEX IF NOT EXISTS chunks_embedding_hnsw
    ON chunks USING hnsw (embedding vector_cosine_ops);

CREATE INDEX IF NOT EXISTS chunks_metadata_gin
    ON chunks USING gin (metadata);
