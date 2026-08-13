# Product Requirements Document (PRD)

## MVP: Embedding Generation for the Ingestion Pipeline

---

## Executive Summary

The chunking pipeline (`ProcessDocWorker` → `ChunkGeneratorWorker`) currently persists structural chunks **without vector embeddings**. This MVP extends the existing job pipeline so that, before a chunk is persisted, its context-prepended content (`chunk.Content`) is sent to an OpenAI-API-compatible embeddings endpoint via the existing `internal/infra/ai` client, and the resulting vector is stored in the `chunks` table using **pgvector**.

This MVP also introduces a **per-file ingestion flag** — `files.embedding_status` — that durably records whether a file's chunks have been fully ingested (embedded) into the vector store. The existing `files.status` (`upload_status`) only tracks the object-upload lifecycle and says nothing about embedding progress; without a dedicated flag the system cannot distinguish *"uploaded but not yet embedded"* from *"fully embedded and retrievable"*, cannot tell a partially ingested file from a complete one, and cannot discover files that predate this rollout (the backfill backlog).

The embedding client stays swappable by **configuration only**: the `openai-go` SDK already accepts a custom `BaseURL`, so switching between LM Studio, OpenAI, DeepSeek, or any OpenAI-compatible `/v1/embeddings` service is a config change (`AI_BASE_URL` env var), not a code change. The **model name is parameterized** via an `AI_EMBEDDING_MODEL` env var so the operator can point the pipeline at whatever embedding model the active provider exposes (e.g. a model served locally by LM Studio). Only the embedding dimension (`1024`) remains hardcoded for this MVP and must match the provider's output.

This document also analyzes the schema impact and concludes that a dedicated `embedding VECTOR` column on the `chunks` table **is required** for the retrieval pipeline that will build on top.

---

## 1. Scope & MVP Goals

### In-Scope (MVP)

* **Per-chunk embedding generation** inside the existing Job 2 worker (`ChunkGeneratorWorker`) before `StoreBatch`, so each persisted chunk carries a vector from day one.
* **OpenAI-compatible endpoint abstraction**: the embedder is consumed through a narrow interface; the concrete client is the existing `AIClient` (`internal/infra/ai/client.go`), configured via `AIBaseURL` / `AIAPIKey` / `AIEmbeddingModel` env vars. The model name is configurable; the embedding dimension (`1024`) is a hardcoded constant for this MVP.
* **pgvector schema change**: `embedding VECTOR(1024) NOT NULL` column on `chunks`, `CREATE EXTENSION vector`, and an ANN (HNSW) cosine index.
* **Per-file ingestion flag**: `files.embedding_status` (`pending` → `processing` → `completed` / `failed`) tracking whether a file's chunks have been fully embedded, plus a completion job so the flag only turns `completed` once every chunk of the file carries a non-null vector.
* **pgx integration**: register the pgvector Go type so `ChunkRepository` can read/write vectors with pgx v5.
* **Retrieval-ready search contract**: add `SearchSimilar` to `ChunkRepository` (cosine distance, optional threshold + limit) so retrieval can be built without further repo work.

### Out-of-Scope (Deferred)

* Retrieval pipeline / RAG query flow, reranking, hybrid (BM25 + vector) search.
* Configurable embedding **dimension** (dimensions remain hardcoded; model name is parameterized in this MVP).
* Embedding versioning, re-embedding on model change, backfill jobs (the `embedding_status` flag makes the backfill backlog *discoverable* but does not implement backfill).
* Batching multiple chunks in a single embeddings API call for throughput.
* Any change to `SendPrompt` / chat completion behavior.

---

## 2. System Architecture & Job Flow

```
[Object Storage (RustFS)]
       │ (files.s3_key)
       ▼
┌─────────────────────────────────────────┐
│ Job 1: ProcessDocWorker                 │
│ (GetObjectIntoBuffer + BuildChunks)     │
│ 1. files.embedding_status → 'processing'│
└────────────────────┬────────────────────┘
                     │ Emits GenerateChunksArgs × N,
                     │ then Mark-File-Embedded {ExpectedChunks=N}
                     ▼
        [River Queue (Postgres-backed)]
                     │
                     ▼
┌──────────────────────────────────────────────┐
│ Job 2: ChunkGeneratorWorker (UPDATED)        │
│ 1. build domain.Chunk                        │
│ 2. Embed(content) via Embedder  ──────────┐  │
│ 3. Attach embedding to chunk              │  │
│ 4. StoreBatch                             │  │
└───────────────┬──────────────────────────────┘
                │ Writes Chunks (+ embedding)
                ▼
   [chunks table via ChunkRepository / pgvector]
                │
                ▼
┌──────────────────────────────────────────────┐
│ Job 3: MarkFileEmbeddedWorker (NEW)          │
│  embedding_status → 'completed' only when    │
│  ALL chunks of the file carry non-null       │
│  embeddings (else retries via River)         │
└──────────────────────────────────────────────┘
```

### Flow Breakdown

1. **Unchanged:** Job 1 parses and sizes the document into finalized chunks and emits one `GenerateChunksArgs` per chunk (existing behavior).
2. **New — Flag armed:** `ProcessDocWorker` marks the file `files.embedding_status = 'processing'` (via the file service) and records how many chunk jobs it enqueues.
3. **New — Embedding:** Job 2 (`ChunkGeneratorWorker`) builds the `domain.Chunk`, calls the `Embedder` with `chunk.Content` (the heading-context-prepended text), and assigns the returned vector to `chunk.Embedding`.
4. **Persist:** `StoreBatch` writes the chunk including the `embedding` column.
5. **New — Completion:** after enqueueing all `GenerateChunksArgs`, Job 1 enqueues a single `Mark-File-Embedded` job carrying `ExpectedChunks = N`. The job flips `embedding_status` to `completed` **only** when every chunk of the file is stored with a non-null `embedding`; otherwise it returns a "not yet" error and River retries. `failed` is the terminal state when retries are exhausted.
6. **Failure handling:** if the embeddings call fails, the chunk job returns an error and River retries it (existing per-job retry semantics). No chunk is persisted half-embedded because the embed happens before the write, and the flag only turns `completed` after *all* chunks are embedded — so a partially ingested file is never marked ready.

### The `embedding_status` Data Flag

The pipeline needs a durable, per-file record of *"has this file been ingested into the embedded store?"* — neither `files.status` nor the chunk rows answer that question today:

| Question | `files.status` (`upload_status`) | `chunks` rows |
| --- | --- | --- |
| Was the object uploaded? | Yes (`pending` → `completed` / `failed`) | No |
| Have the chunks been embedded? | No — the client flips this on upload completion, *before* the pipeline runs | Partially — each stored row is embedded, but row count alone cannot tell "0 of N done" from "all N done" |
| Is the file safe to serve via retrieval? | No | No |

**Flag:** `files.embedding_status`, an enum column defaulting to `pending`:

```sql
CREATE TYPE embedding_status AS ENUM ('pending', 'processing', 'completed', 'failed');
```

| Value | Meaning |
| --- | --- |
| `pending` | Default on file creation; embedding has not started. |
| `processing` | `ProcessDocWorker` has started ingesting this file; chunk jobs are in flight. |
| `completed` | Every chunk of the file is persisted with a non-null `embedding`; the file is retrievable. |
| `failed` | The pipeline exhausted retries (a chunk job or the completion job gave up). |

**Completion semantics (why a dedicated job):** `GenerateChunksArgs` jobs run in parallel (`MaxWorkers=5`), so there is no FIFO guarantee that the last-enqueued chunk is the last to finish — a naive "mark completed after enqueue" is wrong. Instead `ProcessDocWorker` enqueues exactly one `Mark-File-Embedded` job *after* all chunk jobs; that job flips the flag to `completed` only when `COUNT(chunks WHERE file_id = $1 AND embedding IS NOT NULL) = ExpectedChunks`, returning an error to retry otherwise. This is idempotent and race-free: a file is only ever marked `completed` once its full set of vectors exists.

> **Cross-PRD note:** the FAQ virtual file (`mvp-internal-faq-pipeline.md`, §3.2) is seeded with `files.status = 'completed'`. When that pipeline lands it must also seed `embedding_status = 'completed'`, otherwise FAQ chunks would be flagged as un-ingested.

> **Design rationale:** embedding inside Job 2 reuses the existing per-chunk job parallelism (one River job per chunk, `MaxWorkers=5`) and requires **no changes to `GenerateChunksArgs`** (its `Content` field is sufficient). A dedicated Job 3 (`EmbeddingWorker`) is deferred; it adds queue decoupling but not correctness for the MVP.

---

## 3. Core Go Contracts

### 3.1 Embedder Interface (consumer-side)

Defined in the consuming layer (`internal/worker`), implemented by the existing `AIClient`, keeping the swappability documented in the executive summary.

```go
// internal/worker/adapter.go
type Embedder interface {
    // Embed returns the vector representation of text via the configured
    // OpenAI-compatible embeddings endpoint. The model comes from the client
    // config (AI_EMBEDDING_MODEL); dimension (1024) is fixed at the client
    // layer and must match the configured model's output.
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

> **Note:** `AIClient.SendTextForEmbedding` currently returns `[]float64`. Either add a dedicated `Embed(ctx, text) ([]float32, error)` method or widen the existing one; pgvector stores `float32`. Pick one and keep it consistent with the search contract.

### 3.2 Embedding Client — Configuration-Only Switch

The `openai-go` SDK already supports custom endpoints, so no new client abstraction is required:

```go
// internal/infra/ai/client.go (wiring with model param)
type ClientDependency struct {
    BaseURL        string // AI_BASE_URL env → LM Studio / OpenAI / DeepSeek / etc.
    ApiKey         string // AI_API_KEY env
    EmbeddingModel string // AI_EMBEDDING_MODEL env → model served by the active provider
}
```

* **Model (configurable):** `AI_EMBEDDING_MODEL` env var (e.g. `text-embedding-bge-m3`, `text-embedding-3-large`, or a locally served model name in LM Studio), defaulting to `text-embedding-bge-m3` when unset. Threaded through `ClientDependency.EmbeddingModel` → `openai.EmbeddingNewParams.Model`.
* **Dimension (hardcoded for MVP):** `1024` — must equal the configured model's output dimension and the `VECTOR(n)` column size. If the chosen model outputs a different dimension, the column and client constant must be aligned together.
* **Provider compatibility:** any service exposing `POST /v1/embeddings` with the OpenAI request/response shape (LM Studio: `http://localhost:1234/v1`, OpenAI, DeepSeek, vLLM, Ollama, etc.). Retry/timeout policy on the embeddings call is inherited from River job retry + the SDK's default transport.

### 3.3 domain.Chunk + ChunkRepository

```go
// internal/domain/chunk.go (extended)
type Chunk struct {
    ID          uuid.UUID
    FileID      uuid.UUID
    Index       int
    Content     string
    RawText     string
    TokenCount  int
    HeadingPath []string
    ContentHash string
    Metadata    map[string]any
    Embedding   []float32 `json:"embedding,omitempty"` // pgvector, 1024-d, absent until Job 2 embeds
    CreatedAt   time.Time
}
```

```go
// internal/rag/adapters.go (extended)
type ChunkRepository interface {
    StoreBatch(ctx context.Context, chunks []domain.Chunk) error
    GetByFileID(ctx context.Context, fileID uuid.UUID) ([]domain.Chunk, error)
    DeleteByFileID(ctx context.Context, fileID uuid.UUID) error
    // SearchSimilar returns top-k chunks ordered by cosine similarity against
    // embedding, optionally filtered by a minimum similarity threshold.
    SearchSimilar(ctx context.Context, embedding []float32, limit int, threshold float64) ([]domain.Chunk, error)
}
```

`ChunkGeneratorWorker` gains the `Embedder` dependency:

```go
// internal/worker/generate_chunks.go (updated)
type ChunkGeneratorWorker struct {
    river.WorkerDefaults[GenerateChunksArgs]
    chunkRepository ChunkRepository
    embedder        Embedder
}

func (w *ChunkGeneratorWorker) Work(ctx context.Context, job *river.Job[GenerateChunksArgs]) error {
    chunk, err := buildChunk(job.Args)
    if err != nil {
        return err
    }
    vec, err := w.embedder.Embed(ctx, chunk.Content)
    if err != nil {
        return err
    }
    chunk.Embedding = vec
    return w.chunkRepository.StoreBatch(ctx, []domain.Chunk{*chunk})
}
```

### 3.4 Worker Wiring

`RegisterWorkerDep` (`internal/worker/setup.go`) gains `Embedder Embedder`. The concrete `ai.AIClient` is passed in from `newClientSet`/`service_manager.go`; it is already instantiated from env and now also receives `EmbeddingModel` (`env.Object.AIEmbeddingModel`), added alongside `AIBaseURL`/`AIAPIKey` in `internal/env/env.go`.

`RegisterWorkerDep` also gains the ingestion-flag dependencies: the existing `worker.FileService` is extended with `SetEmbeddingStatus`, `worker.ChunkRepository` gains `CountEmbeddedByFileID`, `worker.JobQueue` gains `EnqueueMarkFileEmbedded`, and `MarkFileEmbeddedWorker` is registered in `RegisterWorkers`.

### 3.5 File Ingestion Flag — Domain, Repo & Completion Worker

```go
// internal/domain/file.go (extended)
type EmbeddingStatus string

const (
    EMBEDDING_STATUS_PENDING    EmbeddingStatus = "pending"
    EMBEDDING_STATUS_PROCESSING EmbeddingStatus = "processing"
    EMBEDDING_STATUS_COMPLETED  EmbeddingStatus = "completed"
    EMBEDDING_STATUS_FAILED     EmbeddingStatus = "failed"
)

type File struct {
    // ...existing fields...
    EmbeddingStatus EmbeddingStatus // files.embedding_status
}

type UpdateFileParam struct {
    Status          *UploadStatus
    EmbeddingStatus *EmbeddingStatus // NEW
}
```

`FileRepository` (`internal/infra/db/postgres/file_repository.go`) adds `embedding_status` to `fileColumns`/`scanFileRow`, includes it in `insertColumns` (defaults to `pending`), and handles the new field in `Update`. `SingleFileResponse` (`internal/file/handler.go`) exposes `embeddingStatus` so clients can surface ingestion progress.

```go
// internal/worker/process_rag.go (updated) — Job 1: arm the flag, then hand off
func (w *ProcessDocWorker) Work(ctx context.Context, job *river.Job[ProcessDocArgs]) error {
    fileID, _ := uuid.Parse(job.Args.FileID)
    if err := w.fileService.SetEmbeddingStatus(ctx, fileID, domain.EMBEDDING_STATUS_PROCESSING); err != nil {
        return err
    }
    source, err := w.storageClient.GetObjectIntoBuffer(ctx, job.Args.ObjectKey)
    if err != nil {
        return err
    }
    chunks, err := w.ragService.BuildChunks(ctx, source)
    if err != nil {
        return err
    }
    for i, chunk := range chunks {
        if err := w.jobQueue.EnqueueGenerateChunks(ctx, GenerateChunksArgs{ /* unchanged */ }); err != nil {
            return err
        }
    }
    return w.jobQueue.EnqueueMarkFileEmbedded(ctx, MarkFileEmbeddedArgs{
        FileID:        job.Args.FileID,
        ExpectedChunks: len(chunks),
    })
}
```

```go
// internal/worker/mark_file_embedded.go (new) — Job 3: flip the flag when all vectors exist
type MarkFileEmbeddedArgs struct {
    FileID         string `json:"fileID"`
    ExpectedChunks int    `json:"expectedChunks"`
}

func (MarkFileEmbeddedArgs) Kind() string { return "Mark-File-Embedded" }

type MarkFileEmbeddedWorker struct {
    river.WorkerDefaults[MarkFileEmbeddedArgs]
    fileService    FileService // SetEmbeddingStatus
    chunkRepository ChunkRepository // CountEmbeddedByFileID
}

func (w *MarkFileEmbeddedWorker) Work(ctx context.Context, job *river.Job[MarkFileEmbeddedArgs]) error {
    fileID, err := uuid.Parse(job.Args.FileID)
    if err != nil {
        return err
    }
    count, err := w.chunkRepository.CountEmbeddedByFileID(ctx, fileID)
    if err != nil {
        return err
    }
    if count < job.Args.ExpectedChunks {
        return ErrNotAllChunksEmbedded // "not yet" → River retries; never marks complete early
    }
    return w.fileService.SetEmbeddingStatus(ctx, fileID, domain.EMBEDDING_STATUS_COMPLETED)
}
```

---

## 4. Schema Change Analysis — Do We Need an Embedding Column?

**Verdict: Yes.** Add a dedicated `embedding` column to the existing `chunks` table — and, alongside it, the per-file `embedding_status` ingestion flag described in §2.

| Consideration | Analysis |
| --- | --- |
| **Retrieval requirement** | A vector index needs the embedding stored server-side. Without a column there is nowhere to persist the vector and no index to query. |
| **1:1 cardinality** | One embedding per chunk. A separate `chunk_embeddings` table adds a join for zero benefit; a column matches `domain.Chunk` 1:1 and the existing repo contract. |
| **pgvector support** | Assumed available (postgres running the `vector` extension). Column type `VECTOR(n)` with an HNSW index gives ANN search without external infra. |
| **Consistency with query** | Embedding the context-prepended `chunk.Content` at write time lets the future query path embed the same shape of text and compare apples-to-apples. |
| **Dimension coupling** | `VECTOR(1024)` must equal the hardcoded `Dimensions: 1024` client constant, which in turn must match the `AI_EMBEDDING_MODEL` model's output. If the model is swapped for one with different dims, the column and the client constant must be recreated + backfilled together — acceptable for MVP, tracked as a deferred re-embedding concern. |
| **NOT NULL timing** | Column is added `NOT NULL` and populated in the same write (`StoreBatch`) so no null state exists in normal flow. |

### 4.1 Migration — `db/migrations/000006_add_embedding_ingestion.{up,down}.sql`

The embedding feature ships **one** migration (renamed from `000006_add_chunk_embeddings` to reflect that it also carries the per-file ingestion flag). The chunk `embedding` column and the `files.embedding_status` flag must land together so the flag can never claim `completed` for a file whose chunks have no vector column:

```sql
-- up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TYPE embedding_status AS ENUM ('pending', 'processing', 'completed', 'failed');

ALTER TABLE "public"."chunks"
    ADD COLUMN embedding VECTOR(1024) NOT NULL;

CREATE INDEX idx_chunks_embedding_hnsw
    ON "public"."chunks"
    USING hnsw (embedding vector_cosine_ops);

ALTER TABLE "public"."files"
    ADD COLUMN embedding_status embedding_status NOT NULL DEFAULT 'pending';
```

`000006_add_embedding_ingestion.down.sql`:

```sql
DROP INDEX IF EXISTS idx_chunks_embedding_hnsw;
ALTER TABLE "public"."chunks" DROP COLUMN IF EXISTS embedding;
ALTER TABLE "public"."files" DROP COLUMN IF EXISTS embedding_status;
DROP TYPE IF EXISTS embedding_status;
```

> Existing files default to `pending` on migration; they are the backfill backlog (identified by `embedding_status = 'pending' AND status = 'completed'`) and are not re-ingested by this MVP.

> HNSW is chosen over IVFFlat because it requires no training pass and behaves well from small to large corpora — a good default for this MVP's data volume.

### 4.2 pgx + pgvector Integration

pgx v5 does not natively scan `vector` types. Add `github.com/pgvector/pgvector-go` and register it on the pool connection (e.g. in `postgres.New`, `pgxpool.NewWithConfig` → `config.AfterConnect`/`pgtype.Map`). `ChunkRepository` then marshals via `pgvector.Vector` for writes and scans `[]float32` back on reads.

---

## 5. Persistence & Search Contract

* `StoreBatch` serializes `chunk.Embedding` as `pgvector.Vector` into the `embedding` column; `scanChunkRow` reads it back.
* `SearchSimilar` uses cosine distance over the HNSW index:

```sql
SELECT id, file_id, chunk_index, content, raw_text, token_count, heading_path,
       content_hash, metadata, created_at,
       1 - (embedding <=> $1) AS similarity
FROM chunks
WHERE embedding <=> $1 < 1 - $2
ORDER BY embedding <=> $1
LIMIT $3;
```

> `<=>` is the cosine distance operator; threshold filters `distance < 1 - threshold`. This keeps retrieval out of the MVP's scope while making the repo interface ready for it.

> **Flag vs search:** `SearchSimilar` does **not** filter on `files.embedding_status` in this MVP — it searches all embedded chunks. The flag exists so a per-file filter ("only serve files with `embedding_status = 'completed'`") and backfill tooling can be built later without another schema change.

---

## 6. Implementation Phases

```
Phase 1: Embedding Client Surface ────────► Embedder interface + AIClient.Embed([]float32)
Phase 2: Schema & pgx Integration ────────► 000006 migration (embedding col + embedding_status flag), pgvector-go registration
Phase 3: Job 2 Embedding Wiring ──────────► ChunkGeneratorWorker + Embedder, StoreBatch writes vector
Phase 4: File Ingestion Flag ─────────────► embedding_status transitions + MarkFileEmbeddedWorker
Phase 5: Search Contract + Tests ─────────► SearchSimilar + repo/worker unit & integration tests
```

* **Week 1:** Add `Embedder` interface; extend `AIClient` with an `Embed` method returning `[]float32`, taking the model from `ClientDependency.EmbeddingModel`; add `AI_EMBEDDING_MODEL` env var (`env.Object.AIEmbeddingModel`, default `text-embedding-bge-m3`); keep dims `1024` constant.
* **Week 2:** Add `000006_add_embedding_ingestion` migration (`extension vector`, `VECTOR(1024)` column + HNSW index, `files.embedding_status` enum column defaulting to `pending`) and the `down` file; add `github.com/pgvector/pgvector-go` and register it in `postgres.New`; extend `domain.File`/`UpdateFileParam` and `FileRepository` (columns, scan, update).
* **Week 3:** Extend `domain.Chunk` with `Embedding`; inject `Embedder` into `ChunkGeneratorWorker` and call it before `StoreBatch`; update `StoreBatch`/`scanChunkRow` for the vector column; `ProcessDocWorker` sets `embedding_status='processing'` and enqueues `Mark-File-Embedded{ExpectedChunks}`; add `MarkFileEmbeddedWorker` (flips to `completed` only when all chunks are embedded, retries otherwise); wire `JobQueue.EnqueueMarkFileEmbedded`, `FileService.SetEmbeddingStatus`, `ChunkRepository.CountEmbeddedByFileID` in `worker/setup.go`, `worker/adapter.go` and `providers.go`.
* **Week 4:** Add `SearchSimilar` to `ChunkRepository` (cosine distance, threshold, limit); update mockery mocks; add tests (worker embeds-then-stores; flag transitions `pending→processing→completed` and never `completed` before all chunks embedded; repo round-trips and orders by similarity; retry on embed failure) matching existing mockery + testify patterns.

---

## 7. Success Criteria

1. **Vector persistence:** Every chunk stored by the ingestion pipeline has a non-null `embedding` of dimension 1024 (`SELECT count(*) FROM chunks WHERE embedding IS NULL;` returns 0).
2. **Swappable endpoint & model:** Pointing `AI_BASE_URL` at LM Studio, OpenAI, or DeepSeek (OpenAI-compatible `/v1/embeddings`) and setting `AI_EMBEDDING_MODEL` to a model available on that provider produces the same pipeline behavior with no code changes; only the model's dimension consistency (vs the `1024` constant) needs to hold.
3. **Index readiness:** `idx_chunks_embedding_hnsw` exists and is used (`EXPLAIN` on a `SearchSimilar` query shows an index/ANN scan).
4. **Search contract:** `SearchSimilar` returns chunks ranked by cosine similarity, respects `limit` and `threshold`, and is covered by unit tests.
5. **Failure recovery:** A transient embeddings API error causes the chunk job to retry (River) and eventually persists the chunk with a valid vector.
6. **Ingestion flag:** a file reaches `embedding_status='completed'` only after **all** of its chunks are stored with non-null `embedding`, and never earlier; the completion job is idempotent under retries and a file whose chunk jobs are still in flight is never marked `completed`. Pre-existing files stay `pending` and are identifiable as the backfill backlog.

---
