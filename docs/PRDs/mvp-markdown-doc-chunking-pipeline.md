# Product Requirements Document (PRD)

## MVP: Markdown Document Chunking Pipeline

---

## Executive Summary

This MVP defines a lightweight, high-performance **Markdown-focused document chunking pipeline**. By leveraging Markdown’s native Abstract Syntax Tree (AST), the pipeline processes documents **deterministically without LLM agent overhead**, eliminating multi-agent latency and costs. The architecture is asynchronous and job-driven, pulling source files directly from object storage via the existing S3 (`RustFS`) client, tokenizing content into **128–512 token boundaries**, and storing output through an extensible repository interface ready for future format additions.

The MVP reuses the existing `rag.BuildChunks` heading-tree parser (`internal/rag/service.go`) as the foundation for Job 1, migrating its current character-based sizing (200–1200 chars) to token-based sizing, and adds the missing contracts (`Tokenizer`, `Chunk`/`ChunkRepository`) and the Job 2 chunk-generation worker.

---

## 1. Scope & MVP Goals

### In-Scope (MVP)

* **Single Format:** Native `.md` / Markdown file format processing using deterministic AST parsing (`github.com/yuin/goldmark`, already a project dependency).
* **Deterministic Chunking:** Heading-based hierarchy chunking ($H_1 \to H_6$) without LLM-agent calls. Reuse/refactor the existing `BuildChunks` implementation.
* **Token-Aware Sizing:** Strict token boundaries between **128 and 512 tokens** utilizing a dedicated `Tokenizer` interface, aligned to the `text-embedding-bge-m3` embedding model already configured in `internal/infra/ai/client.go`.
* **Context Injection:** Automatic prepending of heading paths into chunk content for optimal vector embedding context.
* **Async Job Decoupling:** Separate River jobs for **Document Processing** (`ProcessDocWorker`, fetch + parse + size) and **Chunk Persistence** (`ChunkGeneratorWorker`, storage) to protect processing nodes from memory overload.
* **Extensible Storage Layer:** Storage interfaces designed generically so future formats (PDF, DOCX, HTML) can reuse the exact same database contracts.

### Out-of-Scope (Deferred)

* AI Agent Layers (`StructureAnalyzer`, `ChunkingStrategy`, `QualityValidator`).
* Multi-format support (PDF, DOCX, HTML, OCR).
* Real-time synchronous HTTP processing.

---

## 2. System Architecture & Job Flow

```
[Object Storage (RustFS)] 
       │ (files.s3_key)
       ▼
┌─────────────────────────────────────────┐
│ Job 1: ProcessDocWorker                 │
│ (GetObjectIntoBuffer + BuildChunks)     │
└────────────────────┬────────────────────┘
                     │ Emits GenerateChunksArgs
                     ▼
        [River Queue (Postgres-backed)]
                     │
                     ▼
┌─────────────────────────────────────────┐
│ Job 2: ChunkGeneratorWorker             │
│ (build domain.Chunk, StoreBatch)        │
└────────────────────┬────────────────────┘
                     │ Writes Generated Chunks
                     ▼
       [chunks table via ChunkRepository]
```

### Flow Breakdown

1. **Trigger:** A background worker receives a `ProcessDocArgs` task containing a `FileID` (a `files.id` UUID) and an `ObjectKey` (`files.s3_key`).
2. **Fetch:** Job 1 calls `StorageClient.GetObjectIntoBuffer` to pull the raw Markdown payload from Object Storage (S3/MinIO via `RustFS`, `internal/infra/storage/s3.go`).
3. **AST Parse & Size:** Job 1 runs `rag.BuildChunks` (AST parse → heading sections → merge/split against the `Tokenizer`) and produces finalized chunks with prepended heading paths.
4. **Chunk Execution:** Job 1 emits one `GenerateChunksArgs` per finalized chunk onto the River queue; Job 2 workers consume them and batch-write `domain.Chunk` records to the `ChunkRepository` (`chunks` table).

---

## 3. Core Go Contracts

### 3.1 Object Storage Client

The project does not define a standalone `storage.ObjectStorage` interface; it exposes the concrete `RustFS` client (`internal/infra/storage/s3.go`) and defines **consumer-side interfaces** where they are used (e.g. `worker.StorageClient` in `internal/worker/adapter.go`). The chunking pipeline consumes the following methods from `RustFS`:

```go
type StorageClient interface {
	GetObjectIntoBuffer(ctx context.Context, key string) ([]byte, error)
}
```

Existing full client surface (kept unchanged):

```go
type RustFS struct { /* internal */ }

func (r *RustFS) ListFiles(ctx context.Context) ([]string, error)
func (r *RustFS) GetPresignPutURL(ctx context.Context, key string) (string, error)
func (r *RustFS) GetPresignGetURL(ctx context.Context, key string) (string, error)
func (r *RustFS) GetObjectIntoBuffer(ctx context.Context, key string) ([]byte, error)
func (r *RustFS) UploadObject(ctx context.Context, key string, mimeType string, buff []byte) error
func (r *RustFS) DeleteObject(ctx context.Context, key string) error
```

> **Note:** The PRD's original `GetObjectByKey(ctx, key) (io.ReadCloser, error)` is intentionally not added; `GetObjectIntoBuffer` returns `[]byte`, matching the project's existing read pattern.

### 3.2 Tokenizer Interface

New package following the project's `internal/infra/*` layering (e.g. `internal/infra/tokenization`). Token counting must match the `text-embedding-bge-m3` model used by `internal/infra/ai/client.go`.

```go
package tokenization

// Tokenizer ensures chunk size accuracy based on vector embedding model token counts.
type Tokenizer interface {
	CountTokens(text string) int
	Encode(text string) ([]int, error)
	Decode(tokens []int) (string, error)
}
```

### 3.3 Extensible Chunk Repository Interface

The `Chunk` struct lives in the `domain` package (following `domain.File`, `domain.AuditLog`), with IDs typed as `uuid.UUID` to match `files.id`. The repository interface is defined where it is consumed (service/worker layer, per the `file.adapters.go` pattern) and implemented in `internal/infra/db/postgres`.

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type Chunk struct {
	ID          uuid.UUID              `json:"id"`
	FileID      uuid.UUID              `json:"file_id"`  // references files.id
	Index       int                    `json:"index"`
	Content     string                 `json:"content"`      // Context-prepended text
	RawText     string                 `json:"raw_text"`     // Original text without context header
	TokenCount  int                    `json:"token_count"`
	HeadingPath []string               `json:"heading_path"` // e.g. ["# System Architecture", "## Key Components"]
	ContentHash string                 `json:"content_hash"`
	Metadata    map[string]interface{} `json:"metadata"`     // jsonb, format-specific data
	CreatedAt   time.Time              `json:"created_at"`
}
```

```go
// Defined in the consuming package (e.g. internal/rag), implemented in postgres.
type ChunkRepository interface {
	StoreBatch(ctx context.Context, chunks []domain.Chunk) error
	GetByFileID(ctx context.Context, fileID uuid.UUID) ([]domain.Chunk, error)
	DeleteByFileID(ctx context.Context, fileID uuid.UUID) error
}
```

Persistence contract — new migration `db/migrations/000005_add_chunks_table.{up,down}.sql`:

```sql
CREATE TABLE "public"."chunks" (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    raw_text TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    heading_path JSONB NOT NULL DEFAULT '[]'::jsonb,
    content_hash VARCHAR(64) NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_chunks_file_index UNIQUE (file_id, chunk_index)
);

CREATE INDEX idx_chunks_file_id ON chunks(file_id);
```

---

## 4. Markdown Processing Rules & Chunk Sizing

### 4.1 Heading Hierarchy & AST Extraction

1. **Primary Boundary:** Split content at heading markers (`#`, `##`, `###`, etc.). Reuse the existing heading-stack logic in `BuildChunks` (`internal/rag/service.go`), which walks the `goldmark` AST and preserves parent-heading context with placeholder fill for skipped levels.
2. **Context Prepending:** Each chunk's vectorized content must prepend its parent heading hierarchy:

$$\text{Final Content} = \text{Join}(\text{HeadingPath}, \text{ " > "}) + \text{"\n\n"} + \text{Section Body}$$


> **Example:**
> ```markdown
> Context: # API Reference > ## Authentication
> 
> All API requests require a Bearer token in the HTTP Authorization header.
> 
> ```


### 4.2 Token Boundary Rules

* **Minimum Size:** `128 tokens`
* **Target / Maximum Size:** `512 tokens`
* **Sub-segmenting Large Sections:** If a single section under a heading exceeds `512 tokens`:
1. Split at paragraph boundaries (`\n\n`).
2. If a single paragraph exceeds `512 tokens`, split at sentence boundaries (`. `) using sliding token windows with a `32-token` overlap.


* **Merging Small Sections:** If a section is under `128 tokens`, merge it with the adjacent sibling section under the same parent heading, provided the combined token count remains $\le 512$ tokens.
* **Code Blocks & Tables:** Treat Markdown code fences (```) and tables as **atomic units**. Never break inside a code fence or table row unless the single block itself exceeds `512 tokens`.
* **Tables in goldmark:** The current `goldmark.DefaultParser()` has no table extension, so tables are parsed as generic blocks. Enable `extension.Table` (`goldmark`'s GFM tables) so tables can be treated as atomic units during chunking.

> **Migration note:** The existing `BuildChunks` sizes by **characters** (`TargetMinChunkSize = 200`, `TargetMaxChunkSize = 1200`, `internal/rag/service.go:37-38`). Phase 3 replaces this with token-based sizing against the `Tokenizer`, while keeping the heading-stack and block-collection logic intact.

---

## 5. Job Queue Architecture

Built on the existing River queue (`internal/infra/queue/river.go`, Postgres-backed, default queue MaxWorkers=5). Job payloads are plain structs with a `Kind()` method, matching the worker pattern in `internal/worker/` (see `process_rag.go`, `sort.go`).

```go
package worker

// Job 1 Payload: Triggered when a document needs processing.
// FileID references files.id; ObjectKey references files.s3_key.
type ProcessDocArgs struct {
	FileID    string `json:"fileID"`
	ObjectKey string `json:"objectKey"`
}

func (ProcessDocArgs) Kind() string { return "Process-RAG-File" }

// Job 2 Payload: Emitted by Job 1 per finalized chunk for parallel persistence.
type GenerateChunksArgs struct {
	FileID      string   `json:"fileID"`
	Index       int      `json:"index"`
	HeadingPath []string `json:"headingPath"`
	Content     string   `json:"content"`  // Context-prepended text
	RawText     string   `json:"rawText"`  // Body without context header
	TokenCount  int      `json:"tokenCount"`
}

func (GenerateChunksArgs) Kind() string { return "Generate-Chunks" }
```

### Job Responsibilities

| Job Name | Responsibility | Output |
| --- | --- | --- |
| **`ProcessDocWorker`** (Job 1) | Pulls file via `StorageClient.GetObjectIntoBuffer`, runs `rag.BuildChunks` (AST parse, 128–512 merge/split sizing, context injection), assigns global chunk indexes. | Emits one `GenerateChunksArgs` event per finalized chunk to the River queue. |
| **`ChunkGeneratorWorker`** (Job 2) | Consumes `GenerateChunksArgs`, builds `domain.Chunk` records (content hash, metadata), batch-writes. | Stores `[]domain.Chunk` records in the `chunks` table. |

Both workers are registered in `RegisterWorkers` (`internal/worker/setup.go`). `RiverQueue` gains `EnqueueGenerateChunks`; `EnqueueRagFile` now takes `(fileID uuid.UUID, objectKey string)`. `ProcessDocArgs` no longer references `RagFileID int64` — there is no `rag_files` table; the source of truth is `files`.

---

## 6. Implementation Phases

```
Phase 1: Tokenizer & Chunk Storage  ─────────────► tokenization package, ChunkRepository + migration
Phase 2: Refactor Markdown AST Parser ───────────► migrate BuildChunks to token-based sizing, tables
Phase 3: Chunking & Sizing Engine ───────────────► 128-512 token split/merge algorithms
Phase 4: Async Job Pipeline ─────────────────────► Job 2 worker + enqueue wiring + e2e tests
```

* **Week 1:** Add `internal/infra/tokenization` (bge-m3-aligned `Tokenizer`), `domain.Chunk`, `chunks` migration, and postgres `ChunkRepository` (+ mockery mocks).
* **Week 2:** Refactor `rag.BuildChunks` to token-based sizing (replacing the 200/1200 char constants) and enable `extension.Table` so tables are atomic.
* **Week 3:** Implement token-boundary logic (merge $<128$, split $>512$, atomic code block/table preservation, 32-token overlap).
* **Week 4:** Add `ChunkGeneratorWorker` + `GenerateChunksArgs`, update `ProcessDocArgs` (`FileID`/`ObjectKey`), wire `EnqueueGenerateChunks` in `RiverQueue`, and complete end-to-end integration tests using mocked `StorageClient`/`ChunkRepository` (mockery + testify, matching existing tests).

---

## 7. Success Criteria

1. **Processing Velocity:** Process a 1MB Markdown document in **$< 300\text{ ms}$** (no LLM latency overhead).
2. **Token Compliance:** 100% of generated chunks strictly fall within the **128–512 token** window (excluding rare cases where a single unbroken code block exceeds 512 tokens).
3. **Context Integrity:** Every chunk contains an accurate, full `HeadingPath` array in metadata and prepended in `Content`.
4. **Queue Stability:** Zero worker node OOM (Out-of-Memory) crashes during parallel ingestion tests.

---
