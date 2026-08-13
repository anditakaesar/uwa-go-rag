# Product Requirements Document (PRD)

## MVP: Internal FAQ — Capturing Unanswered Questions & Auto-Indexing Answers

---

## Executive Summary

The RAG chat pipeline returns a fixed *"I don't know"* answer when no grounded context is found or the model cannot answer from context (`mvp-retrieval-rag-chat-pipeline.md` §4.3). This MVP closes that gap by turning every unanswered question into an **internal FAQ item**: unanswered questions are captured into a `faqs` table, internal users write the canonical answer, and — the core of this PRD — the saved Q&A is **synthesized into markdown, chunked by the existing deterministic chunker, embedded, and persisted as retrievable chunks** so the next identical or similar question is answered from ground truth.

The pipeline after an internal user saves an answer reuses the existing ingestion machinery end-to-end:

```
internal user saves answer  →  enqueue FaqIndexWorker  →  rag.BuildChunks("# {question}\n\n{answer}")
    →  GenerateChunksArgs (metadata: source=faq, faq_id)  →  ChunkGeneratorWorker embeds & stores
    →  chunks table (pgvector)  →  SearchSimilar now returns FAQ chunks for future chats
```

No new embedding client, model, or storage layer is needed — FAQ answers flow through the **same** `rag.BuildChunks` + `ChunkGeneratorWorker` pipeline that ingests uploaded documents, reusing the exact same `AI_EMBEDDING_MODEL`, dimension, and pgvector index.

---

## 1. Scope & MVP Goals

### In-Scope (MVP)

* **Unanswered-question capture** from the chat flow (`UnansweredRecorder` implemented by the FAQ service, wired in the retrieval PRD) with deduplication.
* **Internal curation API**: internal users list unanswered questions and write answers; answering moves the FAQ to `answered`.
* **Auto-indexing pipeline**: saving an answer enqueues a River job that synthesizes markdown, chunk sizes it, and reuses `ChunkGeneratorWorker` to embed + store the Q&A as a pgvector chunk.
* **Retrieval integration**: FAQ chunks live in the same `chunks` table, so `SearchSimilar` picks them up automatically; citations surface the question as the heading path.
* **Re-indexing on answer edits** (delete + regenerate the FAQ's chunks).

### Out-of-Scope (Deferred)

* FAQ UI polish, moderation workflow, bulk import/export.
* Multi-language FAQ variants and answer versioning.
* Hybrid/reranking of FAQ vs document chunks; FAQ-only source filtering.
* Per-question attachments or links.
* Job-based (async) unanswered capture — the MVP records in-process during chat.

---

## 2. System Architecture & Job Flow

```
[Chat Pipeline (retrieval PRD)]
   "I don't know" (no context / model fallback)
       │  UnansweredRecorder.RecordUnanswered(question)
       ▼
┌──────────────────────────────────────────────┐
│ faqs table  (status = 'unanswered')          │
└────────────────────┬─────────────────────────┘
                     │  Internal user lists & answers
                     ▼
┌──────────────────────────────────────────────┐
│ FAQ Service.Answer(id, answer)               │
│  status → 'answered', answered_by, answered_at│
│  enqueue IndexFaqArgs{FAQID}                 │
└────────────────────┬─────────────────────────┘
                     ▼
         [River Queue (Postgres-backed)]
                     ▼
┌──────────────────────────────────────────────┐
│ FaqIndexWorker (NEW)                         │
│ 1. Load FAQ                                   │
│ 2. markdown = "# {question}\n\n{answer}"      │
│ 3. rag.BuildChunks(markdown) → FinalChunk(s) │
│ 4. Emit GenerateChunksArgs (metadata faq_id)  │
└────────────────────┬─────────────────────────┘
                     │  one GenerateChunksArgs per FinalChunk
                     ▼
┌──────────────────────────────────────────────┐
│ ChunkGeneratorWorker (EXISTING, reused)      │
│  embed(Content) → StoreBatch                  │
└────────────────────┬─────────────────────────┘
                     ▼
   [chunks table] embedding VECTOR(1536), file_id = FAQ virtual file,
                  metadata = {"source":"faq","faq_id":<id>}
                     │
                     ▼
   [Retrieval] SearchSimilar now retrieves FAQ chunks for future chats
```

### Flow Breakdown

1. **Capture:** during a chat request, when grounding fails (retrieval PRD §4.6), `FAQService.RecordUnanswered` inserts a `faqs` row with `status = 'unanswered'`, deduplicated by `lower(question)`.
2. **Curate:** an internal user lists unanswered FAQs and writes the canonical answer.
3. **Answer:** `FAQService.Answer` validates the answer, flips status to `answered`, records `answered_by`/`answered_at`, then enqueues `IndexFaqArgs{FAQID}`.
4. **Synthesize & chunk:** `FaqIndexWorker` loads the FAQ, builds `# {question}\n\n{answer}`, and runs the existing deterministic `rag.BuildChunks` (question becomes the heading — the chunker's heading-context prepending is free).
5. **Embed & store:** the worker emits one `GenerateChunksArgs` per finalized chunk with `FileID` = the FAQ virtual file and `Metadata = {"source":"faq","faq_id":<id>}`; the existing `ChunkGeneratorWorker` embeds `Content` and persists the chunk.
6. **Retrieve:** future queries embed against the same model and `SearchSimilar` now includes the FAQ chunk; the answer is grounded and cited.

---

## 3. Data Model

### 3.1 `faqs` table — migration `db/migrations/000007_add_faqs.up.sql`

```sql
CREATE TYPE faq_status AS ENUM ('unanswered', 'answered', 'dismissed');

CREATE TABLE "public"."faqs" (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    question            TEXT NOT NULL,
    answer              TEXT,                       -- NULL until answered
    status              faq_status NOT NULL DEFAULT 'unanswered',
    asked_by            BIGINT,                     -- end-user who asked (nullable)
    answered_by         BIGINT,                     -- internal user (nullable)
    file_id             UUID NOT NULL REFERENCES files(id),  -- FAQ virtual file (shared)
    answer_content_hash VARCHAR(64),                -- for idempotent re-indexing
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    answered_at         TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_faqs_status ON faqs(status);
-- Dedupe: only one open 'unanswered' row per normalized question.
CREATE UNIQUE INDEX uq_faqs_unanswered_question
    ON faqs (lower(question)) WHERE status = 'unanswered';
```

`000007_add_faqs.down.sql` drops the table, type, and indexes.

### 3.2 FAQ Virtual File

`chunks.file_id` is `NOT NULL REFERENCES files(id)` (ingestion contract), but FAQ chunks do not originate from an uploaded document. The MVP seeds a single **virtual FAQ file** row so FAQ chunks satisfy the FK without a schema change:

```sql
-- 000007_add_faqs.up.sql (same migration)
INSERT INTO "public"."files" (
    id, user_id, original_name, mime_type, size_bytes,
    s3_bucket, s3_key, status, metadata
) VALUES (
    '00000000-0000-0000-0000-0000000000fa', -- fixed, well-known constant
    0, 'Internal FAQ', 'text/markdown', 0,
    '', 'faq/internal-faq', 'completed', '{"source":"faq-virtual"}'
);
```

* The `id` is a **fixed constant** (`faqVirtualFileID`) shared between the migration and Go code, so `FaqIndexWorker` can reference it without lookups.
* `user_id = 0` marks a system-owned row; normal file listings **must filter out** the virtual file (e.g. `metadata->>'source' != 'faq-virtual'` or `s3_key != 'faq/internal-faq'`).
* Trade-off note: relaxing `file_id` to nullable would avoid the virtual file but changes the ingestion contract; deferred.

### 3.3 `chunks` metadata

FAQ chunks reuse the existing `chunks` table. The `metadata` JSONB tags them as FAQ-owned so re-indexing and future filtering can target them:

```json
{ "source": "faq", "faq_id": "<faq uuid>", "answered_at": "..." }
```

---

## 4. Core Go Contracts

### 4.1 domain.FAQ

```go
// internal/domain/faq.go
type FAQStatus string

const (
    FAQStatusUnanswered FAQStatus = "unanswered"
    FAQStatusAnswered   FAQStatus = "answered"
    FAQStatusDismissed  FAQStatus = "dismissed"
)

type FAQ struct {
    ID                uuid.UUID
    Question          string
    Answer            string
    Status            FAQStatus
    AskedBy           *int64
    AnsweredBy        *int64
    FileID            uuid.UUID
    AnswerContentHash string
    CreatedAt         time.Time
    AnsweredAt        *time.Time
    UpdatedAt         time.Time
}
```

### 4.2 FAQ Repository (consumer-side, `internal/faq`)

```go
// internal/faq/repository.go (implemented in internal/infra/db/postgres)
type Repository interface {
    RecordUnanswered(ctx context.Context, question string, askedBy *int64) error
    ListByStatus(ctx context.Context, status domain.FAQStatus, limit, offset int) ([]domain.FAQ, error)
    Get(ctx context.Context, id uuid.UUID) (*domain.FAQ, error)
    Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64, now time.Time) (*domain.FAQ, error)
}
```

### 4.3 FAQ Service

```go
// internal/faq/service.go
type Service struct {
    repo  Repository
    queue JobQueue
}

// Implements chat.UnansweredRecorder (retrieval PRD §3.4).
func (s *Service) RecordUnanswered(ctx context.Context, question string) error

// ListUnanswered returns open questions for internal curation.
func (s *Service) ListUnanswered(ctx context.Context, limit, offset int) ([]domain.FAQ, error)

// Answer validates + persists the answer, then enqueues IndexFaqArgs.
func (s *Service) Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64) (*domain.FAQ, error) {
    faq, err := s.repo.Answer(ctx, id, answer, answeredBy, time.Now().UTC())
    if err != nil {
        return nil, err
    }
    if err := s.queue.EnqueueIndexFaq(ctx, IndexFaqArgs{FAQID: faq.ID.String()}); err != nil {
        return nil, err
    }
    return faq, nil
}
```

### 4.4 GenerateChunksArgs — metadata extension

The existing `GenerateChunksArgs` (ingestion PRD §5) gains a backward-compatible `Metadata` field so FAQ chunks are tagged. `ChunkGeneratorWorker` merges it into `domain.Chunk.Metadata`:

```go
// internal/worker/generate_chunks.go (updated)
type GenerateChunksArgs struct {
    FileID      string         `json:"fileID"`
    Index       int            `json:"index"`
    HeadingPath []string       `json:"headingPath"`
    Content     string         `json:"content"`
    RawText     string         `json:"rawText"`
    TokenCount  int            `json:"tokenCount"`
    Metadata    map[string]any `json:"metadata,omitempty"` // NEW: {"source":"faq","faq_id":...}
}
```

Existing document workers pass `nil`; behavior is unchanged for them.

### 4.5 FaqIndexWorker (new job)

```go
// internal/worker/index_faq.go
type IndexFaqArgs struct {
    FAQID string `json:"faqID"`
}

func (IndexFaqArgs) Kind() string { return "Index-FAQ" }

type FaqIndexWorker struct {
    river.WorkerDefaults[IndexFaqArgs]
    faqRepo      FaqRepository       // Load(ctx, id)
    ragService   RagService          // BuildChunks (existing)
    jobQueue     JobQueue            // EnqueueGenerateChunks (existing)
    virtualFileID uuid.UUID           // faqVirtualFileID constant
}

func (w *FaqIndexWorker) Work(ctx context.Context, job *river.Job[IndexFaqArgs]) error {
    faq, err := w.faqRepo.Get(ctx, idFrom(job.Args.FAQID))
    if err != nil { return err }
    if faq.Status != domain.FAQStatusAnswered { return nil } // skip stale

    source := []byte("# " + faq.Question + "\n\n" + faq.Answer)
    chunks, err := w.ragService.BuildChunks(ctx, source)
    if err != nil { return err }

    for i, chunk := range chunks {
        if err := w.jobQueue.EnqueueGenerateChunks(ctx, GenerateChunksArgs{
            FileID:      w.virtualFileID.String(),
            Index:       i,
            HeadingPath: chunk.HeadingPath,
            Content:     chunk.Content,
            RawText:     chunk.RawText,
            TokenCount:  chunk.TokenCount,
            Metadata:    map[string]any{"source": "faq", "faq_id": faq.ID.String()},
        }); err != nil { return err }
    }
    return nil
}
```

Registered in `RegisterWorkers` (`internal/worker/setup.go`), which gains `FaqRepository` in `RegisterWorkerDep`.

---

## 5. Indexing Pipeline — After the Internal User Saves an Answer

This section directly answers *"how does the pipeline work after the internal user stores the question and answer?"*

1. **Trigger:** `FAQService.Answer` persists the answer (`status='answered'`) and enqueues `IndexFaqArgs{FAQID}` onto the River queue. The enqueue is **after** the DB commit so the worker never sees a half-written answer.
2. **Synthesize:** `FaqIndexWorker` loads the FAQ and renders a markdown document `# {question}\n\n{answer}`. Using the question as an H1 heading is deliberate — the existing chunker prepends heading context, so the embedded content already carries the question, and the heading path doubles as the citation label.
3. **Chunk:** `rag.BuildChunks` (the exact same deterministic parser used for uploaded documents) applies the 128–512 token rules. A typical Q&A yields a single chunk; long answers split at paragraph/sentence boundaries with overlap, producing multiple chunks that all share the same `faq_id`.
4. **Hand off to the existing embedder:** one `GenerateChunksArgs` per finalized chunk is emitted with `Metadata = {"source":"faq","faq_id":<id>}`. The existing `ChunkGeneratorWorker` takes over — it embeds `Content` with `AI_EMBEDDING_MODEL` and writes the row (embedding + all columns) via `StoreBatch`.
5. **Result:** FAQ chunks are ordinary `chunks` rows referencing the virtual FAQ file. `SearchSimilar` needs **zero changes** to return them; a future identical/similar question retrieves the FAQ chunk at high similarity and is answered from ground truth with the question as its citation heading.
6. **Failure/retry:** any failure (embed API, store) retries via River's per-job semantics. Because the worker only enqueues `GenerateChunksArgs` and never writes chunks directly, partial FAQ indexing is impossible.

**Why reuse instead of a new path?** The embedding client, model, dimension, pgvector index, and chunk store are already in production for documents. Channeling FAQ through `rag.BuildChunks` + `ChunkGeneratorWorker` means FAQ retrieval quality, indexing, and search infrastructure are identical to documents, with no parallel embedding pipeline to maintain.

---

## 6. HTTP Contract (Internal Users)

Routes registered behind an internal/admin role using the existing `middlewares.RequireRole` pattern. Placeholder request/response shown; final shapes follow `transport.SendJSON` conventions.

**List unanswered questions** — `GET /api/faqs?status=unanswered&page=1&pageSize=20`

```json
{
  "data": [
    { "id": "…", "question": "Bagaimana cara reset password?", "askedBy": null,
      "createdAt": "…" }
  ],
  "pagination": { "page": 1, "pageSize": 20, "total": 1 }
}
```

**Answer a question** — `PUT /api/faqs/{id}/answer`

```json
{ "answer": "Buka halaman Login → klik Lupa Password → ikuti email reset." }
```

Response `200 OK`:

```json
{
  "id": "…",
  "question": "Bagaimana cara reset password?",
  "answer": "Buka halaman Login → klik Lupa Password → ikuti email reset.",
  "status": "answered",
  "answeredBy": 42,
  "answeredAt": "…"
}
```

Error responses follow the existing `xerror` / `GlobalErrorMiddleware` patterns (not found, empty answer, not in unanswered status).

---

## 7. Implementation Phases

```
Phase 1: FAQ Domain & Storage ──────────► 000007 migration (faqs + virtual file), postgres repo
Phase 2: Capture & Curation API ────────► RecordUnanswered + ListUnanswered + Answer endpoints
Phase 3: Indexing Worker ───────────────► IndexFaqArgs + FaqIndexWorker + GenerateChunksArgs.Metadata
Phase 4: Re-index & Tests ──────────────► delete-and-regenerate on edit + e2e tests
```

* **Week 1:** Add `000007` migration (`faqs` table, `faq_status` enum, partial unique index, virtual FAQ file seed) + `down` file; `domain.FAQ`; postgres `FaqRepository` (mockery mocks).
* **Week 2:** `FAQService` (`RecordUnanswered` implementing `chat.UnansweredRecorder`, `ListUnanswered`, `Answer`); internal HTTP routes; wire into `service_manager.go`/`providers.go` and the chat `ChatService`.
* **Week 3:** Extend `GenerateChunksArgs` with `Metadata`; `FaqIndexWorker` + `IndexFaqArgs`; register in `RegisterWorkers`; verify an answered FAQ lands as an embedded chunk with `{"source":"faq"}` metadata.
* **Week 4:** Re-index on answer edit (delete chunks by `metadata->>'faq_id'` then re-enqueue); tests (dedupe capture, answer → job enqueue, worker chunk + metadata, retrieval round-trip retrieves the FAQ chunk, re-index replaces old chunks) matching existing mockery + testify patterns.

---

## 8. Success Criteria

1. **Capture:** every grounded failure in chat creates a deduplicated `faqs` row with `status='unanswered'`; repeated identical questions do not duplicate open rows.
2. **Curation:** internal users can list unanswered FAQs and answer them; answering sets `answered` and enqueues exactly one `Index-FAQ` job.
3. **Auto-indexing:** within one job cycle of saving an answer, the FAQ exists as ≥1 `chunks` row with a non-null `embedding`, `metadata.source='faq'`, and the correct `file_id` (virtual FAQ file).
4. **Retrieval closure:** asking the same question via `POST /chat/raw` after indexing returns the curated answer with a citation whose heading path is the FAQ question, instead of "I don't know".
5. **Edit correctness:** updating an answered FAQ removes its old chunks and indexes the new answer; no orphaned chunks carry a stale `faq_id`.
6. **No retrieval regression:** `SearchSimilar` behavior and latency are unchanged for document chunks (FAQ chunks simply add rows to the same index).

---
