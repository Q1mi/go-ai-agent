CREATE EXTENSION IF NOT EXISTS vector;

-- The code creates this table dynamically because pgvector dimensions are part of the column type.
-- Default local embeddings use vector(384).
CREATE TABLE IF NOT EXISTS kb_chunks (
    id          BIGSERIAL PRIMARY KEY,
    doc_id      TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content     TEXT NOT NULL,
    embedding   vector(384) NOT NULL,
    search_text tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (doc_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS kb_chunks_search_idx ON kb_chunks USING GIN (search_text);
CREATE INDEX IF NOT EXISTS kb_chunks_embedding_hnsw_idx ON kb_chunks USING hnsw (embedding vector_cosine_ops);
