# Product Requirements Document (PRD)

## MVP: Internal FAQ — Capturing Unanswered Questions & Auto-Indexing Answers

---

## Executive Summary

The RAG chat pipeline returns a fixed *"I don't know"* answer when no grounded context is found or the model cannot answer from context (`mvp-retrieval-rag-chat-pipeline.md` §4.3). This MVP closes that gap by turning every unanswered question into an **internal FAQ item**: unanswered questions are captured into a `faqs` table, internal users write the canonical answer, and — the core of this PRD — the saved Q&A is **synthesized into markdown, chunked by the existing deterministic chunker, embedded, and persisted as retrievable chunks** so the next identical or similar question is answered from ground truth.

The pipeline after an internal user saves an answer reuses the existing ingestion machinery end-to-end:

```
internal user saves answer  →  enqueue Index-FAQ job  →  FaqIndexWorker:
    render "# {question}\n\n{answer}" → delete stale chunks → upload markdown to S3
    → enqueue Process-RAG-File (existing)  →  ProcessDocWorker: rag.BuildChunks
    → GenerateChunksArgs per chunk  →  ChunkGeneratorWorker embeds & stores
    → chunks table (pgvector)  →  SearchSimilar now returns FAQ chunks for future chats
```

No new embedding client, model, or storage layer is needed — FAQ answers flow through the **same** `rag.BuildChunks` + `ProcessDocWorker` + `ChunkGeneratorWorker` pipeline that ingests uploaded documents, reusing the exact same `AI_EMBEDDING_MODEL`, dimension, and pgvector index.

### Key design decision: one `files` row per FAQ (derived state)

FAQ chunks must satisfy `chunks.file_id NOT NULL REFERENCES files(id)` (ingestion contract). Instead of a single shared "virtual file" with a magic constant UUID — which cannot satisfy the `UNIQUE (file_id, chunk_index)` constraint across multiple FAQs and would leak a phantom row into the storage-facing `files` table — **each FAQ owns its own `files` row**:

* `s3_key = 'faq/<faq_id>.md'` is **derived state**: it is reproducible from the `faqs` row at any time, so it can never be orphaned or "lost in the storage" — an audit can always tell which FAQ owns which file row, and re-indexing is idempotent.
* The `faqs` table remains the curation **source of truth**; the `files` row exists purely as the ingestion-contract artifact (FK target + `embedding_status` lifecycle).
* `UNIQUE (file_id, chunk_index)` keeps working exactly as designed (chunk position is unique *per FAQ*).
* `SearchSimilar` needs **zero changes**: FAQ chunks are ordinary `chunks` rows.

---

## 1. Scope & MVP Goals

### In-Scope (MVP)

* **Unanswered-question capture** from the chat flow (`UnansweredRecorder` implemented by the FAQ service, wired in the retrieval PRD) with deduplication.
* **Internal curation API**: internal users list unanswered questions and write answers; answering moves the FAQ to `answered`.
* **Auto-indexing pipeline**: saving an answer enqueues a River job that synthesizes markdown, (re)uploads it as the FAQ's file, and reuses the existing `Process-RAG-File` chain to chunk, embed, and store the Q&A as pgvector chunks.
* **Retrieval integration**: FAQ chunks live in the same `chunks` table, so `SearchSimilar` picks them up automatically; citations surface the question as the heading path.
* **Idempotent re-indexing on answer edits** (content-hash skip + delete-and-regenerate).
* **File-listing isolation**: FAQ file rows are excluded from normal file listings.

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
│ faqs table  (status = 'unanswered')          │  ← files row created here too
│ files table  (s3_key = 'faq/<faq_id>.md')    │      (same transaction)
└────────────────────┬─────────────────────────┘
                     │  Internal user lists & answers
                     ▼
┌──────────────────────────────────────────────┐
│ FAQ Service.Answer(id, answer)               │
│  status → 'answered', answered_by, answered_at│
│  answer_content_hash = sha256(answer)        │
│  enqueue IndexFaqArgs{FAQID}                 │  (after commit)
└────────────────────┬─────────────────────────┘
                     ▼
         [River Queue (Postgres-backed)]
                     ▼
┌──────────────────────────────────────────────┐
│ FaqIndexWorker (NEW, thin)                   │
│ 1. Load FAQ; skip if not answered            │
│ 2. Skip if last_indexed_hash == hash         │  (idempotent)
│ 3. Delete existing chunks (DeleteByFileID)   │
│ 4. Upload "# {question}\n\n{answer}" → S3    │
│ 5. files.status → 'completed'                │
│ 6. EnqueueRagFile(file_id, s3_key)           │
│ 7. faqs.last_indexed_hash = hash             │
└────────────────────┬─────────────────────────┘
                     ▼
┌──────────────────────────────────────────────┐
│ ProcessDocWorker (EXISTING, reused)          │
│ 1. files.embedding_status → 'processing'     │
│ 2. GetObjectIntoBuffer(s3_key)               │
│ 3. rag.BuildChunks(markdown) → FinalChunk(s) │
│ 4. Emit one GenerateChunksArgs per chunk     │
│ 5. Enqueue Mark-File-Embedded                │
└────────────────────┬─────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────┐
│ ChunkGeneratorWorker (EXISTING, reused)      │
│  embed(Content) → StoreBatch                 │
└────────────────────┬─────────────────────────┘
                     ▼
┌──────────────────────────────────────────────┐
│ MarkFileEmbeddedWorker (EXISTING, reused)    │
│  embedding_status → 'completed' when all     │
│  ExpectedChunks carry vectors                │
└────────────────────┬─────────────────────────┘
                     ▼
   [chunks table] embedding VECTOR(1024), file_id = the FAQ's file row
                     │
                     ▼
   [Retrieval] SearchSimilar now retrieves FAQ chunks for future chats
```

### Flow Breakdown

1. **Capture:** during a chat request, when grounding fails (retrieval PRD §4.6), `FAQService.RecordUnanswered` inserts — in one transaction — a `files` row (`s3_key = 'faq/<faq_id>.md'`, `status = 'pending'`, `metadata = {"source":"faq"}`) **and** a `faqs` row with `status = 'unanswered'`, deduplicated by `lower(question)`.
2. **Curate:** an internal user lists unanswered FAQs and writes the canonical answer.
3. **Answer:** `FAQService.Answer` validates the answer, flips status to `answered`, records `answered_by`/`answered_at`, stores `answer_content_hash = sha256(answer)`, then enqueues `IndexFaqArgs{FAQID}`.
4. **Synthesize:** `FaqIndexWorker` loads the FAQ, renders `# {question}\n\n{answer}` (question as H1 heading — the chunker's heading-context prepending is free, and the heading path doubles as the citation label), deletes the FAQ's stale chunks, and uploads the markdown to `faq/<faq_id>.md`.
5. **Chunk, embed, store (reused chain):** the worker enqueues the existing `Process-RAG-File` job; `ProcessDocWorker` runs `rag.BuildChunks`, emits one `GenerateChunksArgs` per finalized chunk, and `ChunkGeneratorWorker` embeds `Content` and persists the chunk. `MarkFileEmbeddedWorker` flips `embedding_status` to `completed` once every expected chunk carries a vector — the free "indexed" signal.
6. **Retrieve:** future queries embed against the same model and `SearchSimilar` now includes the FAQ chunk; the answer is grounded and cited.

---

## 3. Data Model

### 3.1 `faqs` table — migration `db/migrations/000008_add_faqs.up.sql`

> Note: `000007` is already taken by `000007_add_audit_logs_created_at_index` — the FAQ migration is `000008`.

```sql
CREATE TYPE faq_status AS ENUM ('unanswered', 'answered', 'dismissed');

CREATE TABLE "public"."faqs" (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    question            TEXT NOT NULL,
    answer              TEXT,                       -- NULL until answered
    status              faq_status NOT NULL DEFAULT 'unanswered',
    asked_by            BIGINT,                     -- end-user who asked (nullable)
    answered_by         BIGINT,                     -- internal user (nullable)
    file_id             UUID NOT NULL REFERENCES files(id),  -- this FAQ's file row
    answer_content_hash VARCHAR(64),                -- sha256 of the current answer
    last_indexed_hash   VARCHAR(64),                -- sha256 of the answer actually embedded
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    answered_at         TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_faqs_status ON faqs(status);
-- Dedupe: only one open 'unanswered' row per normalized question.
CREATE UNIQUE INDEX uq_faqs_unanswered_question
    ON faqs (lower(question)) WHERE status = 'unanswered';
```

`000008_add_faqs.down.sql` drops the table, type, and indexes.

No changes to `chunks` or `files` — both tables and all their constraints (including `UNIQUE (file_id, chunk_index)`) work as-is.

### 3.2 FAQ File Row (derived state, not a shared virtual file)

`chunks.file_id` is `NOT NULL REFERENCES files(id)` (ingestion contract). Each FAQ gets its **own** `files` row, created inside the same transaction as the `faqs` row at capture time (`RecordUnanswered`), so `faqs.file_id NOT NULL` always holds:

```sql
-- Created by FAQService.RecordUnanswered via FaqRepository.CreateFile
-- within a UnitOfWork transaction; CreateUnanswered inserts the faqs row
-- in the same transaction. FaqID below is the faqs.id generated first.
INSERT INTO "public"."files" (
    id, user_id, original_name, mime_type, size_bytes,
    s3_bucket, s3_key, status, metadata
) VALUES (
    uuidv7(), 0, 'Internal FAQ', 'text/markdown', 0,
    '', 'faq/<faq_id>.md', 'pending', '{"source":"faq"}'
);
```

* `s3_key = 'faq/<faq_id>.md'` is **derived from the FAQ** — reproducible at any time, never a magic constant. An audit, cleanup job, or re-index can always reconstruct it. The derivation lives in `domain.FAQS3Key(faqID)` (shared by the postgres repo and `FaqIndexWorker`).
* `user_id = 0` marks a system-owned row.
* `status` starts `'pending'` and flips to `'completed'` when `FaqIndexWorker` uploads the markdown (via `file.Service.SetStatus`).
* `embedding_status` starts `'pending'` (column default) and flows `'processing' → 'completed'` through the existing pipeline, driven by `MarkFileEmbeddedWorker` — the FAQ's "indexed" signal with zero new tracking code.
* **File listings must exclude FAQ rows**: `FileRepository.FindAll` gains `s3_key NOT LIKE 'faq/%'` so the system-owned rows never surface (this also covers the listing count subquery, which is derived from the same select).
* No S3 object exists until the first answer is saved — `RecordUnanswered` creates the row only; uploads happen in `FaqIndexWorker`.

### 3.3 `chunks` rows

FAQ chunks reuse the existing `chunks` table unchanged: `file_id` = the FAQ's file row, `chunk_index` unique per FAQ, `heading_path` = `["# <question>"]`, plain `metadata = {}`. The `{"source":"faq"}` tag lives on the `files` row, not the chunks. Chunk-level tagging (and therefore FAQ-only source filtering) is deferred; re-indexing targets chunks by `file_id` via the existing `DeleteByFileID`.

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

// FAQS3Key is the derived object storage key for a FAQ's markdown file.
func FAQS3Key(faqID uuid.UUID) string {
    return fmt.Sprintf("faq/%s.md", faqID.String())
}

type FAQ struct {
    ID                uuid.UUID
    Question          string
    Answer            string
    Status            FAQStatus
    AskedBy           *int64
    AnsweredBy        *int64
    FileID            uuid.UUID
    AnswerContentHash string
    LastIndexedHash   string
    CreatedAt         time.Time
    AnsweredAt        *time.Time
    UpdatedAt         time.Time
}
```

### 4.2 FAQ Repository (consumer-side, `internal/faq`)

```go
// internal/faq/repository.go (implemented in internal/infra/db/postgres)
// ErrAlreadyCaptured is returned when an unanswered FAQ for the same
// normalized question already exists; callers treat it as a silent no-op.
var ErrAlreadyCaptured = errors.New("faq already captured")

type Repository interface {
    CreateFile(ctx context.Context, faqID uuid.UUID) (uuid.UUID, error)
    CreateUnanswered(ctx context.Context, newFAQ domain.FAQ) error
    ListByStatus(ctx context.Context, status domain.FAQStatus, limit, offset int) ([]domain.FAQ, error)
    Get(ctx context.Context, id uuid.UUID) (*domain.FAQ, error)
    Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64, now time.Time) (*domain.FAQ, error)
    SetLastIndexedHash(ctx context.Context, id uuid.UUID, hash string) error
}
```

Notes:
* `CreateFile` inserts the derived `files` row and returns the generated `fileID`; `CreateUnanswered` inserts the `faqs` row and maps the partial-unique-index violation (`23505`) to `ErrAlreadyCaptured`.
* The **transaction lives in the service**: `FAQService.RecordUnanswered` wraps `CreateFile` + `CreateUnanswered` in `application.UnitOfWork` (the same pattern `user`/`file` services use), so a duplicate question rolls back the just-created file row and `RecordUnanswered` returns nil.
* `Answer` sets `answer_content_hash = sha256(answer)` alongside the status flip. Re-answering an `answered` FAQ is an **answer edit** (the new hash drives re-indexing); only `dismissed` rows are rejected.

### 4.3 FAQ Service

```go
// internal/faq/service.go
type JobQueue interface {
    EnqueueIndexFaq(ctx context.Context, args worker.IndexFaqArgs) error
}

type Service struct {
    repo  Repository
    uow   application.UnitOfWork
    queue JobQueue
}

// Implements chat.UnansweredRecorder (retrieval PRD §3.4); replaces the
// noopUnansweredRecorder placeholder wired in internal/server/providers.go.
func (s *Service) RecordUnanswered(ctx context.Context, question string) error {
    err := s.uow.Do(ctx, func(txCtx context.Context) error {
        faqID, err := uuid.NewV7()
        if err != nil {
            return err
        }
        fileID, err := s.repo.CreateFile(txCtx, faqID)
        if err != nil {
            return err
        }
        return s.repo.CreateUnanswered(txCtx, domain.FAQ{
            ID:       faqID,
            FileID:   fileID,
            Question: question,
        })
    })
    if errors.Is(err, ErrAlreadyCaptured) {
        return nil // dedupe is a silent no-op
    }
    return err
}

// ListUnanswered returns open questions for internal curation.
func (s *Service) ListUnanswered(ctx context.Context, limit, offset int) ([]domain.FAQ, error)

// Answer validates + persists the answer, then enqueues IndexFaqArgs.
// The enqueue happens after the repo call returns, so the worker never sees
// a half-written answer.
func (s *Service) Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64) (*domain.FAQ, error) {
    faq, err := s.repo.Answer(ctx, id, answer, answeredBy, time.Now().UTC())
    if err != nil {
        return nil, err
    }
    if err := s.queue.EnqueueIndexFaq(ctx, worker.IndexFaqArgs{FAQID: faq.ID.String()}); err != nil {
        return nil, err
    }
    return faq, nil
}
```

### 4.4 FaqIndexWorker (new job — thin adapter into the existing pipeline)

`GenerateChunksArgs` and `ChunkGeneratorWorker` are **unchanged** — FAQ chunks never carry chunk-level metadata and are produced by the existing `Process-RAG-File` chain.

```go
// internal/worker/index_faq.go
type IndexFaqArgs struct {
    FAQID string `json:"faqID"`
}

func (IndexFaqArgs) Kind() string { return "Index-FAQ" }

type FaqIndexWorker struct {
    river.WorkerDefaults[IndexFaqArgs]
    faqRepo       FaqRepository   // Get, SetLastIndexedHash (worker adapter interface)
    storageClient StorageClient   // UploadObject
    chunkRepo     ChunkRepository // DeleteByFileID (exists on postgres repo; added to worker adapter)
    jobQueue      JobQueue        // EnqueueRagFile (added to worker adapter)
    fileService   FileService     // SetStatus (added to file.Service and the worker adapter)
}

func (w *FaqIndexWorker) Work(ctx context.Context, job *river.Job[IndexFaqArgs]) error {
    faqID, err := uuid.Parse(job.Args.FAQID)
    if err != nil { return err }

    faq, err := w.faqRepo.Get(ctx, faqID)
    if err != nil { return err }
    if faq.Status != domain.FAQStatusAnswered { return nil } // skip stale
    if faq.LastIndexedHash == faq.AnswerContentHash { return nil } // idempotent skip

    source := []byte("# " + faq.Question + "\n\n" + faq.Answer)

    // Idempotent re-index: stale chunks must not coexist with new ones.
    if err := w.chunkRepo.DeleteByFileID(ctx, faq.FileID); err != nil { return err }

    objectKey := domain.FAQS3Key(faq.ID)
    if err := w.storageClient.UploadObject(ctx, objectKey, "text/markdown", source); err != nil { return err }
    if err := w.fileService.SetStatus(ctx, faq.FileID, domain.UPLOAD_STATUS_COMPLETED); err != nil { return err }

    if err := w.jobQueue.EnqueueRagFile(ctx, faq.FileID, objectKey); err != nil { return err }

    return w.faqRepo.SetLastIndexedHash(ctx, faq.ID, faq.AnswerContentHash)
}
```

* The worker **never writes chunks directly** — it hands off to `Process-RAG-File`; partial FAQ indexing is impossible.
* Any failure (upload, embed API, store) retries via River's per-job semantics; because stale chunks were deleted up front, a retry simply regenerates them — the flow is idempotent.
* **Concurrency hardening:** enqueue `Index-FAQ` with River `InsertOpts{UniqueOpts: {ByArgs: true}}` so at most one Index-FAQ job exists per FAQ at a time (concurrent answer edits serialize). `ByState` is scoped to the in-flight states (`available`, `pending`, `running`, `retryable`, `scheduled`) — river's default includes `completed`, which would block re-enqueue after a successful index until the job cleaner purges it, breaking answer edits.
* Registered in `RegisterWorkers` (`internal/worker/setup.go`), whose `RegisterWorkerDep` gains `FaqRepository`; `FileService` gains `SetStatus`, `ChunkRepository` gains `DeleteByFileID`, and `JobQueue` gains `EnqueueRagFile` (worker adapter interfaces).

### 4.5 Queue

```go
// internal/infra/queue/river.go (new method)
func (r *RiverQueue) EnqueueIndexFaq(ctx context.Context, args worker.IndexFaqArgs) error {
    _, err := r.client.Insert(ctx, args, &river.InsertOpts{
        UniqueOpts: river.UniqueOpts{
            ByArgs: true,
            ByState: []rivertype.JobState{
                rivertype.JobStateAvailable,
                rivertype.JobStatePending,
                rivertype.JobStateRunning,
                rivertype.JobStateRetryable,
                rivertype.JobStateScheduled,
            },
        },
    })
    return err
}
```

The `ByState` list keeps the unique constraint on in-flight jobs only: a completed `Index-FAQ` must not block the next edit. River encodes the state bitmask per job; the partial unique index only conflicts while the existing job's state is inside the mask.

---

## 5. Indexing Pipeline — After the Internal User Saves an Answer

This section directly answers *"how does the pipeline work after the internal user stores the question and answer?"*

1. **Trigger:** `FAQService.Answer` persists the answer (`status='answered'`, `answer_content_hash = sha256(answer)`) and enqueues `IndexFaqArgs{FAQID}` onto the River queue. The enqueue is **after** the DB commit so the worker never sees a half-written answer.
2. **Synthesize:** `FaqIndexWorker` loads the FAQ and renders a markdown document `# {question}\n\n{answer}`. Using the question as an H1 heading is deliberate — the existing chunker prepends heading context, so the embedded content already carries the question, and the heading path doubles as the citation label.
3. **Idempotency gate:** if `last_indexed_hash == answer_content_hash`, the job returns immediately — repeated enqueues (duplicate clicks, retries) are no-ops. On the first index, and on every edit, the FAQ's stale chunks are deleted (`DeleteByFileID`).
4. **Upload + hand off to the existing pipeline:** the markdown is uploaded to `faq/<faq_id>.md` (the FAQ's derived `files` row flips to `status='completed'`), then `Process-RAG-File` is enqueued. The **existing** `ProcessDocWorker` pulls the markdown, runs `rag.BuildChunks` (the exact same deterministic parser used for uploaded documents, 128–512 token rules; a typical Q&A yields a single chunk, long answers split at paragraph/sentence boundaries with overlap), emits one `GenerateChunksArgs` per finalized chunk, and `ChunkGeneratorWorker` embeds `Content` with `AI_EMBEDDING_MODEL` and writes the row via `StoreBatch`. `MarkFileEmbeddedWorker` flips `embedding_status` to `'completed'` once every expected chunk carries a vector.
5. **Result:** FAQ chunks are ordinary `chunks` rows referencing the FAQ's file row. `SearchSimilar` needs **zero changes** to return them; a future identical/similar question retrieves the FAQ chunk at high similarity and is answered from ground truth with the question as its citation heading.
6. **Failure/retry:** any failure (upload, embed API, store) retries via River's per-job semantics. Chunks are only ever (re)written through the existing chain, so there is no parallel write path to keep consistent; a retry regenerates from the FAQ row, which is the single source of truth.

**Why reuse instead of a new path?** The embedding client, model, dimension, pgvector index, chunk store, and completion tracking are already in production for documents. Channeling FAQ through a per-FAQ `files` row + `Process-RAG-File` means FAQ retrieval quality, indexing, completion signalling, and search infrastructure are **identical** to documents — with no parallel embedding pipeline, no chunk-level metadata plumbing, and no special-case constraints to maintain.

---

## 6. HTTP Contract (Internal Users)

Routes registered behind `middlewares.RequireAuth` + `middlewares.RequirePermission` using the existing patterns: `GET /api/faqs` → `faqs.read`, `PUT /api/faqs/{id}/answer` → `faqs.update` (permissions seeded for `superadmin` in `db/seed/roles_resource_users.sql`). Handlers live in `internal/faq/handler.go` (`FAQApi` + `FAQService` interface, mockery `MockFAQService`) and follow `transport.SendJSON` conventions — every response is wrapped in a `{"data": ..., "meta": ...}` envelope.

**List unanswered questions** — `GET /api/faqs?status=unanswered&page=1&size=20`

`status` defaults to `unanswered` and only `unanswered` is supported in this MVP (any other value → 400). Pagination follows the codebase `page`/`size` convention (`common.Pagination`).

```json
{
  "data": [
    { "id": "…", "question": "Bagaimana cara reset password?", "askedBy": null,
      "createdAt": "…" }
  ],
  "meta": { "page": 1, "size": 20, "total": 1 }
}
```

**Answer (or edit) a question** — `PUT /api/faqs/{id}/answer`

`answeredBy` is taken from the JWT identity (`domain.IdentityFromContext`). Re-answering an already-`answered` FAQ is an **answer edit**: the answer is replaced, `answer_content_hash` is bumped, and a new `Index-FAQ` job re-indexes it.

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

Error responses follow the existing `xerror` / `GlobalErrorMiddleware` patterns (not found, empty answer, dismissed status, unsupported status filter).

---

## 7. Implementation Phases

All phases below are **delivered**. Actual delivery per phase:

```
Phase 1: FAQ Domain & Storage ────────► 000008 migration (faqs only), domain.FAQ, consumer
                                        Repository interface, postgres repo (+ file row in UoW)
Phase 2: Capture & Curation API ──────► RecordUnanswered + ListUnanswered + Answer endpoints
Phase 3: Indexing Worker ─────────────► IndexFaqArgs + FaqIndexWorker + queue method +
                                        Answer enqueue + listing filter
Phase 4: Re-index & Tests ────────────► answer edits, unique-by-args (in-flight states),
                                        pipeline round-trip + re-index tests, e2e verification
```

* **Phase 1:** `000008` migration (`faqs` table, `faq_status` enum, partial unique index, `answer_content_hash`/`last_indexed_hash`) + `down` file; `domain.FAQ` (+ `FAQS3Key`); consumer-side `Repository` interface (`CreateFile`, `CreateUnanswered`, `ListByStatus`, `Get`, `Answer`, `SetLastIndexedHash`, `ErrAlreadyCaptured`); postgres `FaqRepository` (mockery mocks). Deviation from the original §4.2 sketch: the transaction lives in the **service** (`FAQService.RecordUnanswered` wraps both repo calls in `application.UnitOfWork`), matching the existing `user`/`file` service pattern — the repo is transaction-agnostic.
* **Phase 2:** `FAQService` (`RecordUnanswered` implementing `chat.UnansweredRecorder`, `ListUnanswered`, `Answer`); `FAQApi` + `SetupFAQApiRoutes` (`GET /api/faqs`, `PUT /api/faqs/{id}/answer`); `faqs.read`/`faqs.update` permissions seeded; wired into `internal/server/setup.go`/`providers.go`/`apis.go`/`routes.go`; `noopUnansweredRecorder` removed and `faqSvc` injected as the chat `Recorder`.
* **Phase 3:** `FaqIndexWorker` + `IndexFaqArgs` + `RiverQueue.EnqueueIndexFaq` (unique-by-args); `FAQService.Answer` enqueues `Index-FAQ` after the commit; registered in `RegisterWorkers` (`RegisterWorkerDep` gains `FaqRepository`; worker adapters gain `FileService.SetStatus`, `ChunkRepository.DeleteByFileID`, `JobQueue.EnqueueRagFile`); `file.Service.SetStatus`; `FileRepository.FindAll` gains the `s3_key NOT LIKE 'faq/%'` filter; verified an answered FAQ lands as an embedded chunk after one pipeline cycle.
* **Phase 4:** answer edits supported (`repo.Answer` accepts re-answering `answered` rows, rejects `dismissed`); `EnqueueIndexFaq` `ByState` scoped to in-flight states (river's default includes `completed` and would block re-indexing after the first index); tests — dedupe capture, answer → job enqueue, worker hash-skip/delete/upload/enqueue, full pipeline round-trip (Index-FAQ → Process-RAG-File → Generate-Chunks → Mark-File-Embedded), re-index replaces old chunks, idempotent no-op on unchanged answer; e2e verified: index, edit re-index, idempotent re-answer, retrieval closure, listing isolation.

---

## 8. Success Criteria

1. **Capture:** every grounded failure in chat creates a deduplicated `faqs` row with `status='unanswered'` plus its `files` row; repeated identical questions do not duplicate open rows.
2. **Curation:** internal users can list unanswered FAQs and answer them; answering sets `answered` and enqueues exactly one `Index-FAQ` job.
3. **Auto-indexing:** within one pipeline cycle of saving an answer, the FAQ exists as ≥1 `chunks` row with a non-null `embedding`, referencing the FAQ's own `file_id`; `files.embedding_status` ends at `completed`; `faqs.last_indexed_hash == answer_content_hash`.
4. **Idempotency:** re-enqueuing `Index-FAQ` for an unchanged answer is a no-op (no chunk churn, no duplicate rows).
5. **Retrieval closure:** asking the same question via `POST /chat/raw` after indexing returns the curated answer with a citation whose heading path is the FAQ question, instead of "I don't know".
6. **Edit correctness:** updating an answered FAQ removes its old chunks and indexes the new answer; no orphaned chunks carry stale content; `UNIQUE (file_id, chunk_index)` is never violated across multiple FAQs.
7. **No retrieval or listing regression:** `SearchSimilar` behavior and latency are unchanged for document chunks (FAQ chunks simply add rows to the same index); FAQ file rows never appear in `GET /api/files` listings or their count.
