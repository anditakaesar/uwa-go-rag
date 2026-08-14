DROP INDEX IF EXISTS idx_chunks_embedding_hnsw;
ALTER TABLE "public"."chunks" DROP COLUMN IF EXISTS embedding;
ALTER TABLE "public"."files" DROP COLUMN IF EXISTS embedding_status;
DROP TYPE IF EXISTS embedding_status;
