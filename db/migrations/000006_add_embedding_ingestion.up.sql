CREATE EXTENSION IF NOT EXISTS vector;

CREATE TYPE embedding_status AS ENUM ('pending', 'processing', 'completed', 'failed');

ALTER TABLE "public"."chunks"
    ADD COLUMN embedding VECTOR(1024) NOT NULL;

CREATE INDEX idx_chunks_embedding_hnsw
    ON "public"."chunks"
    USING hnsw (embedding vector_cosine_ops);

ALTER TABLE "public"."files"
    ADD COLUMN embedding_status embedding_status NOT NULL DEFAULT 'pending';
