# Product Requirements Document (PRD)

## MVP: Retrieval-Augmented Generation (RAG) Chat Pipeline

---

## Executive Summary

The ingestion pipeline now persists token-sized chunks with pgvector embeddings. This MVP closes the loop by adding the **retrieval half**: when a user sends a chat message through the existing HTTP web handler (`POST /chat/raw`), the server (1) embeds the query with the same `AI_EMBEDDING_MODEL`, (2) searches the `chunks` table for the most similar vectors via the pgvector HNSW index, (3) augments the prompt with the retrieved chunk contexts and heading paths, (4) calls the LLM via the existing `AIClient`, and (5) returns the answer **plus citations** pointing back to the source chunks.

The flow is **grounded by construction**: the model is explicitly forbidden from using external tools (no search engine, no function calling) and instructed to answer **only from the retrieved context**. When no context is found above the similarity threshold — or the model reports it cannot answer from context — the service returns a fixed "I don't know" message and **captures the unanswered question** so it can be curated into the Internal FAQ pipeline (see `mvp-internal-faq-pipeline.md`).

The flow is **synchronous** (request/response from the HTTP handler), reusing the exact same embedding client, model, and dimension as ingestion so the query vector is comparable to stored vectors. This MVP defines the retrieval service contract, the search repository method, the prompt-augmentation strategy, the grounding rules, and the HTTP response shape. Streaming, conversation memory, reranking, hybrid search, and the FAQ curation UI are deferred.

---

## 1. Scope & MVP Goals

### In-Scope (MVP)

* **Synchronous RAG query flow** triggered from the existing HTTP web handler `ChatApi.SendMessage` (`POST /chat/raw`, `internal/chat/handler.go`).
* **Query embedding** reusing the ingestion `Embedder` contract and the same `AI_EMBEDDING_MODEL` env var + hardcoded `1024` dimension, guaranteeing query/stored vector compatibility.
* **Vector search** via `ChunkRepository.SearchSimilar` (cosine distance, HNSW index, top-k + similarity threshold) — the search method already specified in the ingestion PRD.
* **Prompt augmentation**: retrieved chunks (context-prepended `Content` + `HeadingPath` + file reference) are injected as ground-truth context into the LLM call with an instruction to answer strictly from the retrieved context and cite sources.
* **Grounded answer & no-tool enforcement**: the LLM call is made with **zero tools** (no function calling, no web/search tool) and an explicit instruction to answer only from the injected context; on insufficient context the model returns the fixed "I don't know" message.
* **Unanswered-question capture**: when grounding fails (no chunk above threshold, or a "I don't know" answer), the service records the user's question as an **unanswered FAQ** for internal curation (§4.6).
* **Citations**: the HTTP response includes the answer plus source metadata (file ID, heading path, similarity, content snippet) per cited chunk.
* **Retrieval contract wiring**: replace the empty `IRagRepository` in `internal/chat/service.go` with the consumer-side `RetrievalRepository` (field renamed `ChunkRepo`) backed directly by the existing `postgres.ChunkRepository`. There is no RAG table/resource, so the redundant `RagRepository` pass-through (`internal/infra/db/postgres/rag_repo.go`) is **removed** and `ChatService` uses `ChunkRepository` directly.

### Out-of-Scope (Deferred)

* Streaming / SSE responses.
* Conversation history, session state, and multi-turn memory.
* Reranking, hybrid (BM25 + vector) search, and similarity-threshold tuning harnesses.
* Per-file permission scoping of retrieved chunks by the authenticated user.
* Chat message persistence and audit logging.
* Asynchronous retrieval (job queue) — the MVP is a blocking HTTP request.
* **Internal FAQ curation & re-indexing** (answering captured questions, embedding Q&A pairs) — covered by the dedicated PRD `mvp-internal-faq-pipeline.md`; this MVP only **captures** unanswered questions.

---

## 2. System Architecture & Query Flow

```
[HTTP Web Handler]  POST /chat/raw  { "prompt": "..." }
       │
       ▼
┌───────────────────────────────────────────────┐
│ ChatApi.SendMessage (handler)                 │
│ → ChatService.Chat(ctx, prompt)               │
└────────────────────┬──────────────────────────┘
                     │
                     ▼
┌───────────────────────────────────────────────┐
│ RetrievalService / ChatService (UPDATED)      │
│ 1. Embed(query) via Embedder                 │
│ 2. SearchSimilar(embedding, topK, threshold) │
│ 3. Build augmented prompt (context + prompt) │
│ 4. SendPrompt → LLM                          │
└────────────────────┬──────────────────────────┘
                     │
                     ▼
        [pgvector HNSW index on chunks.embedding]
                     │
                     ▼
┌───────────────────────────────────────────────┐
│ HTTP Response: { answer, citations[] }        │
└───────────────────────────────────────────────┘
```

### Flow Breakdown

1. **Request:** `ChatApi.SendMessage` decodes `{ "prompt" }`, validates non-empty (existing behavior in `internal/chat/handler.go:55`).
2. **Embed:** the service embeds the raw prompt with `Embedder.Embed`, using the same model/dims as ingestion.
3. **Search:** `ChunkRepository.SearchSimilar(ctx, embedding, topK, threshold)` returns the top-k chunks ranked by cosine similarity.
4. **Augment:** the service builds a prompt from the base instructions + the retrieved chunk contexts (`Content`, which already carries the heading path) + the user question, and calls the LLM.
5. **Respond:** the handler returns the generated answer plus a `citations` array derived from the retrieved chunks used in the prompt.
6. **Failure handling:** if embedding or search fails, the request returns the appropriate error through the existing `GlobalErrorMiddleware`. If **no chunk** passes the threshold, the service returns an honest "no relevant context" response instead of hallucinating (see §4.3).

---

## 3. Core Go Contracts

### 3.1 Embedder (reused from ingestion)

No new client. The same consumer-side interface is injected:

```go
// internal/worker/adapter.go → shared, or redefined in internal/chat
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

> **Consistency requirement:** retrieval must use the **same** `AI_EMBEDDING_MODEL` and dimension (`1024`) as ingestion, or query vectors will be incomparable to stored vectors. Both paths read from the same `AIClient` instance/config.

### 3.2 Retrieval Repository (consumer-side, `internal/chat`)

Replaces the empty `IRagRepository` in `internal/chat/service.go`:

```go
type RetrievalRepository interface {
    // SearchSimilar returns top-k chunks ordered by cosine similarity against
    // embedding, optionally filtered by a minimum similarity threshold.
    SearchSimilar(ctx context.Context, embedding []float32, limit int, threshold float64) ([]domain.Chunk, error)
}
```

Implemented directly by `postgres.ChunkRepository` (`internal/infra/db/postgres/chunk_repository.go`), which already exposes the `SearchSimilar` from the ingestion PRD (`internal/rag/adapters.go`). Since there is no separate RAG table/resource, the former `RagRepository` pass-through (`internal/infra/db/postgres/rag_repo.go`) is **removed** and `ChatService` is wired to `ChunkRepository` directly via the `ChunkRepo` field.

`SearchSimilar` also surfaces the cosine similarity per chunk so citations can report it: the query selects `1 - (embedding <=> ?) AS similarity` and populates a new `domain.Chunk.Similarity float64` field (search-only; zero elsewhere). `toCitations` copies it into `Citation.Similarity` (§4.5).

### 3.3 LLM Client — Context-Aware Prompt

`AIClient.SendPrompt` currently hardcodes system instructions and takes only a `prompt` (`internal/infra/ai/client.go:32`). Retrieval needs to inject retrieved context, so the client contract gains a context-aware variant:

```go
// internal/chat/service.go (consumer-side)
type LLMClient interface {
    // SendContextPrompt answers `question` grounded in the provided context.
    // Base instructions (plain text, Bahasa Indonesia) remain fixed at the client.
    SendContextPrompt(ctx context.Context, context string, question string) (string, error)
}
```

On the concrete `AIClient`, `SendContextPrompt` wraps the existing `Responses.New` call, appending the retrieved context to the system instructions and passing the question as user input. **No tool or function definitions are registered on this call** — the model cannot invoke a search engine or any external tool (§4.3). `SendPrompt` is kept for the legacy/raw path.

### 3.4 ChatService — Retrieval Orchestration

```go
// Grounding constants (single source of truth for the "I don't know" message).
const (
    noContextMsg  = "Maaf, saya tidak tahu. Silakan coba lagi dengan pertanyaan yang lebih spesifik."
    fallbackWords = []string{"tidak tahu", "tidak mengetahui", "tidak ada informasi", "i don't know"}
)

type ChatService struct {
    ChunkRepo  RetrievalRepository
    AIClient   LLMClient
    Embedder   Embedder
    Recorder   UnansweredRecorder // captures "I don't know" questions (see §4.6)
    JobQueue   IJobQueue
    UploadDir  string
}

// Chat runs the full RAG flow and returns the answer with citations.
func (s *ChatService) Chat(ctx context.Context, prompt string) (*ChatResponse, error) {
    queryVec, err := s.Embedder.Embed(ctx, prompt)
    if err != nil {
        return nil, err
    }

    chunks, err := s.ChunkRepo.SearchSimilar(ctx, queryVec, topK, simThreshold)
    if err != nil {
        return nil, err
    }

    // No grounded context: never call the LLM, record for FAQ curation.
    if len(chunks) == 0 {
        _ = s.Recorder.RecordUnanswered(ctx, prompt)
        return &ChatResponse{Message: noContextMsg, Citations: nil}, nil
    }

    ctxText := buildContext(chunks)
    answer, err := s.AIClient.SendContextPrompt(ctx, ctxText, prompt)
    if err != nil {
        return nil, err
    }

    // Model admits it could not answer from the provided context.
    if isFallbackAnswer(answer) {
        _ = s.Recorder.RecordUnanswered(ctx, prompt)
        return &ChatResponse{Message: noContextMsg, Citations: nil}, nil
    }

    return &ChatResponse{
        Message:    answer,
        Citations:  toCitations(chunks),
    }, nil
}
```

```go
type ChatResponse struct {
    Message   string      `json:"message"`
    Citations []Citation  `json:"citations"`
}

type Citation struct {
    ChunkID     uuid.UUID  `json:"chunkId"`
    FileID      uuid.UUID  `json:"fileId"`
    HeadingPath []string   `json:"headingPath"`
    Similarity  float64    `json:"similarity"`
    Snippet     string     `json:"snippet"` // raw_text, truncated
}

// Consumer-side contract implemented by the FAQ service (see the FAQ PRD).
type UnansweredRecorder interface {
    RecordUnanswered(ctx context.Context, question string) error
}

// isFallbackAnswer detects the model's grounded "I don't know" response.
func isFallbackAnswer(answer string) bool {
    lower := strings.ToLower(strings.TrimSpace(answer))
    for _, w := range fallbackWords {
        if strings.Contains(lower, w) {
            return true
        }
    }
    return false
}
```

`IChatService` gains `Chat(ctx, prompt) (*ChatResponse, error)`; `ChatApi.SendMessage` calls it instead of the placeholder body (`internal/chat/handler.go:92-94`). `UnansweredRecorder` is wired to the FAQ service from the FAQ PRD (an in-process call — no extra queue hop for the MVP).

---

## 4. Retrieval Strategy

### 4.1 Query Embedding

* Prompt text is embedded as-is (optionally trimmed/normalized) with `Embedder.Embed`.
* No chunking of the query — queries are short; single embedding call per request.

### 4.2 Top-k & Threshold

* Hardcoded service constants for the MVP: `topK = 5`, `simThreshold = 0.5`.
* The threshold guards against answering from irrelevant context. Values are intentionally left as constants (not env) for this MVP; a tuning harness is deferred.

### 4.3 Grounding Rules & No-Context Handling

* **Never call the LLM without grounded context.** If `SearchSimilar` returns zero chunks above `simThreshold`, return `noContextMsg` with an empty citation list and record the question as unanswered (§4.6).
* **No external tools.** The LLM call (`Responses.New`) is configured with **zero tools** — no function/`tool` params, no web-search tool. This is enforced in the client and reinforced in the system instruction (below).
* **Fixed fallback answer.** The model is instructed that when the answer is not present in the provided context it must reply with exactly `noContextMsg` ("Saya tidak tahu, silakan coba lagi…"). The service treats any answer containing a `fallbackWords` token as an unanswered question and returns the canonical `noContextMsg` to the user (§3.4 `isFallbackAnswer`).

### 4.4 Prompt Augmentation Template

```
[System instructions — existing: plain text, Bahasa Indonesia, no markdown]

Anda TIDAK memiliki akses internet, mesin pencari, atau alat apa pun selain
konteks di bawah ini. DILARANG menjawab dari pengetahuan umum atau menebak.

Gunakan hanya konteks di bawah ini untuk menjawab pertanyaan pengguna.
Jika jawaban tidak ada dalam konteks, balas PERSIS dengan kalimat:
"Maaf, saya tidak tahu. Silakan coba lagi dengan pertanyaan yang lebih spesifik."
Rujuk sumber sesuai heading yang tersedia.

===== KONTEKS =====
[1] (# System Architecture > ## Authentication)  similarity 0.92
<body dari chunk 1...>

[2] (# API Reference > ## Rate Limits)  similarity 0.87
<body dari chunk 2...>
===== AKHIR KONTEKS =====

[User prompt]
```

`buildContext(chunks)` joins each chunk's `Content` (already heading-context-prepended by ingestion) with a source index; the model is instructed to reference source numbers in its answer. The client **must not** register any tool or function definition when issuing this call.

### 4.5 Citations

* `toCitations(chunks)` maps the retrieved top-k chunks (those that made it into the prompt) to `Citation` records.
* `Snippet` is truncated `RawText` (e.g. first 200 runes) so the payload stays light.
* `FileID` is available directly on `domain.Chunk`; the file's `OriginalName` lookup via the file repository is a post-MVP enhancement (the JOIN is out of scope).

### 4.6 Unanswered-Question Capture

When grounding fails, the question is persisted as an **unanswered FAQ** so internal users can curate an answer (see `mvp-internal-faq-pipeline.md`):

* **Trigger sources:** (a) zero chunks above the threshold; (b) `isFallbackAnswer(answer)` is true.
* **Contract:** `UnansweredRecorder.RecordUnanswered(ctx, question)` — implemented by the FAQ service as an in-process insert into the `faqs` table (`status = 'unanswered'`), deduplicated by exact/normalized question text.
* **Async vs sync:** recorded in-process to avoid adding a queue hop to chat latency; a single fast INSERT. A job-based variant is deferred.
* **Failure isolation:** a recording error is logged, not propagated — the user still receives `noContextMsg`.

---

## 5. HTTP Contract

**Request** — `POST /chat/raw` (existing route, `internal/chat/handler.go`):

```json
{ "prompt": "Bagaimana cara setup autentikasi?" }
```

**Success Response** — `200 OK`:

> Responses follow the existing `transport.SendJSON` convention and are wrapped in the standard `{ "data": …, "meta": … }` envelope (`data` carries the `ChatResponse`, `meta` echoes the request).

```json
{
  "data": {
    "message": "Autentikasi dijelaskan di bagian Authentication…",
    "citations": [
      {
        "chunkId": "…",
        "fileId": "…",
        "headingPath": ["# System Architecture", "## Authentication"],
        "similarity": 0.92,
        "snippet": "All API requests require a Bearer token…"
      }
    ]
  },
  "meta": { "prompt": "Bagaimana cara setup autentikasi?" }
}
```

**Grounded "I don't know" Response** — `200 OK` (no context found or model cannot answer):

```json
{
  "data": {
    "message": "Maaf, saya tidak tahu. Silakan coba lagi dengan pertanyaan yang lebih spesifik.",
    "citations": []
  },
  "meta": { "prompt": "…" }
}
```

> The same payload shape is returned in both grounded and ungrounded cases; clients only distinguish by non-empty `citations` and the message text. The unanswered question is recorded server-side (§4.6).

**Error responses** follow the existing `transport.SendError` / `xerror` + `GlobalErrorMiddleware` patterns (decoding error, empty prompt, embed/search/LLM failures).

---

## 6. Implementation Phases

```
Phase 1: Repository Search ───────────────► SearchSimilar on chunks (pgvector) + un-stub RagRepository
Phase 2: Context-Aware LLM Client ────────► SendContextPrompt on AIClient (context + question)
Phase 3: Retrieval Service ───────────────► ChatService.Chat (embed → search → augment → respond → cite)
Phase 4: HTTP Wiring + Tests ─────────────► SendMessage → Chat, response shape, unit/integration tests
```

* **Week 1:** `ChunkRepository.SearchSimilar` (cosine distance, threshold, limit, HNSW index scan — reuses the ingestion PRD's contract) already exists; remove the redundant `RagRepository` pass-through and wire `ChatService` to `ChunkRepository` via the consumer-side `RetrievalRepository` (`ChunkRepo`); update mockery mocks.
* **Week 2:** Add `SendContextPrompt(ctx, context, question)` to `AIClient` (system instructions + injected context, **zero tools**); keep `SendPrompt` for the legacy path.
* **Week 3:** Add `Chat` to `ChatService` with `ChatResponse`/`Citation` types, grounding constants (`noContextMsg`, `fallbackWords`, `isFallbackAnswer`), and `UnansweredRecorder` capture; wire `RetrievalRepository` + `Embedder` + `LLMClient` into `ChatServiceDep` and `service_manager.go`/`providers.go`.
* **Week 4:** Point `ChatApi.SendMessage` at `ChatService.Chat`; add tests (no-context short-circuit + capture, fallback-answer detection + capture, citation mapping, prompt template with no-tool instruction, embed/search/LLM error propagation) matching existing mockery + testify patterns.

---

## 7. Success Criteria

1. **End-to-end RAG:** `POST /chat/raw` with a question about an ingested document returns an answer that is grounded in that document's content, with ≥1 citation whose heading path matches the relevant source section.
2. **Model consistency:** query embedding uses the same `AI_EMBEDDING_MODEL`/dims as ingestion; a re-embedded query correctly retrieves its own source chunk (round-trip similarity > threshold).
3. **No hallucination:** a question with no context above `simThreshold` returns `noContextMsg` with an empty citation list, does **not** call the LLM, and is recorded as an unanswered FAQ.
4. **No-tool enforcement:** the `Responses.New` call for the RAG flow carries **zero** tools/function definitions, and the system instruction explicitly forbids internet/search usage; a model fallback reply triggers `isFallbackAnswer` and the canonical `noContextMsg` is returned instead.
5. **Unanswered capture:** every grounded failure (no-context or model fallback) creates a deduplicated `faqs` row with `status = 'unanswered'`; recording errors never fail the chat response.
6. **Latency budget:** end-to-end response completes within a synchronous HTTP request timeout (embed ~100ms + HNSW search ~10ms + LLM generation) with no queue involvement.
7. **Error surfacing:** embed/search/LLM failures surface as structured errors via `GlobalErrorMiddleware`, not silent 200s or partial responses.

---
