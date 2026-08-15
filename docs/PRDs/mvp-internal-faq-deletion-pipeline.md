# Product Requirements Document (PRD)

## MVP: Internal FAQ Deletion — Hard-Delete a FAQ (rows + chunks + S3 object)

---

## Executive Summary

The FAQ pipeline (`mvp-internal-faq-pipeline.md`) lets internal users capture unanswered questions, curate canonical answers, and auto-index them into the retrieval store. This MVP closes the lifecycle gap: internal users must be able to **delete a FAQ** when it is obsolete, wrong, or was captured by mistake.

Deletion is a **hard delete** — the `faqs` row, its derived `files` row, and every `chunks` row are removed permanently; the S3 markdown object (`faq/<faq_id>.md`) is removed asynchronously. No `dismissed` soft-delete state is used; the existing `faq_status` enum keeps `dismissed` reserved for future use.

```
DELETE /api/faqs/{id}  →  FAQService.Delete(id):
    uow.Do: 1. FaqRepository.Get(id)            → 404 if missing
            2. DELETE faqs row
            3. FileRepository.Delete(file_id)   → chunks removed by ON DELETE CASCADE
    (after commit) enqueue Delete-File-From-Storage('faq/<id>.md') → DeleteFileWorker → S3
```

**Why cascade for chunks?** `chunks.file_id REFERENCES files(id) ON DELETE CASCADE` (migration `000005`) already guarantees chunk rows can never outlive their file row. Deleting the FAQ's `files` row therefore removes its chunks **atomically inside the same transaction** — no explicit chunk call, no window where chunks exist without their file, and no violation of `UNIQUE (file_id, chunk_index)`.

**Why delete `faqs` before `files`?** `faqs.file_id REFERENCES files(id)` has no cascade; the `faqs` row must be removed first or the FK is violated.

**Why reuse `Delete-File-From-Storage`?** The existing `RiverQueue.EnqueueDeleteFile` + `DeleteFileWorker` already performs idempotent S3 object deletion (S3 `DELETE` is a no-op on missing keys, so it is safe for FAQs that were never indexed). The FAQ service only enqueues the markdown key — no thumbnail keys exist for FAQ file rows.

---

## 1. Scope & MVP Goals

### In-Scope (MVP)

* **Hard delete of a FAQ** in any status (`unanswered`, `answered`): `faqs` row, derived `files` row, and all `chunks` rows (via FK cascade) removed in one transaction.
* **S3 cleanup**: the FAQ's markdown object (`faq/<faq_id>.md`) is deleted asynchronously through the existing `Delete-File-From-Storage` job.
* **Internal API**: `DELETE /api/faqs/{id}` behind `RequireAuth` + `RequirePermission("faqs.delete")`.
* **Idempotent-in-intent semantics**: deleting an already-deleted FAQ returns `404` (the resource no longer exists).

### Out-of-Scope (Deferred)

* Soft delete via the `dismissed` status (enum exists, unused).
* Deleting *single chunks* or re-answering a deleted FAQ (deletion is terminal).
* Cancelling or purging in-flight `Index-FAQ` / `Process-RAG-File` River jobs on delete.
* Bulk deletion, undo/restore, audit of deletions.

---

## 2. System Architecture & Job Flow

```
[Internal user / curation UI]
        │  DELETE /api/faqs/{id}   (RequireAuth + RequirePermission("faqs.delete"))
        ▼
┌──────────────────────────────────────────────┐
│ FAQService.Delete(id)                        │
│  1. FaqRepository.Get(id)  → 404 if missing  │
│  2. uow.Do:                                  │
│       repo.Delete(id)                        │
│         DELETE FROM faqs WHERE id = $1       │
│         fileRepo.Delete(file_id)             │
│           └─ chunks rows removed by          │
│              ON DELETE CASCADE (same tx)     │
│  3. queue.EnqueueDeleteFile('faq/<id>.md')   │  (after commit)
└────────────────────┬─────────────────────────┘
                     ▼
         [River Queue (Postgres-backed)]
                     ▼
┌──────────────────────────────────────────────┐
│ DeleteFileWorker (EXISTING, reused)          │
│  storage.DeleteObject('faq/<id>.md')         │
└────────────────────┬─────────────────────────┘
                     ▼
        [S3] object removed (idempotent no-op if absent)
```

### Flow Breakdown

1. **Resolve:** `FAQService.Delete` calls `repo.Get`; a missing FAQ surfaces as `xerror.ErrorResourceNotFound` (HTTP 404) and nothing is touched.
2. **Delete (one transaction):** the service wraps `repo.Delete` in `application.UnitOfWork` (the same pattern `user`/`file`/`faq` services use). Inside the transaction the repo deletes the `faqs` row first, then the FAQ's `files` row — the `chunks` rows cascade with the file row. All-or-nothing: any failure rolls back, leaving the FAQ fully intact.
3. **S3 cleanup (async):** after commit, `EnqueueDeleteFile(domain.FAQS3Key(faq.ID))` inserts a `Delete-File-From-Storage` job; the existing `DeleteFileWorker` calls `DeleteObject`. S3 `DELETE` is idempotent, so never-uploaded (unanswered) FAQs are safe.
4. **Result:** the FAQ is gone from curation lists (`GET /api/faqs`), from retrieval (`SearchSimilar` finds no chunks — the FAQ's chunks are gone), and its file row no longer exists in `files` (it was already hidden from `GET /api/files` by the `s3_key NOT LIKE 'faq/%'` filter while it lived).

---

## 3. Data Model

No schema changes. Existing constraints do the work:

| Constraint | Behavior on FAQ delete |
|---|---|
| `faqs.file_id UUID NOT NULL REFERENCES files(id)` | `faqs` row must be deleted before the `files` row (no cascade). |
| `chunks.file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE` (migration `000005`) | Deleting the FAQ's `files` row removes every chunk for that file atomically. |
| `UNIQUE (file_id, chunk_index)` | Never violated: chunk rows cannot coexist with a deleted file row. |
| `files.s3_key VARCHAR(500) NOT NULL UNIQUE` | The derived key `faq/<faq_id>.md` is freed for reuse after the row is gone. |

* `domain.FAQS3Key(faqID)` remains the single source of truth for the object key (shared by `CreateFile`, `FaqIndexWorker`, and now `FAQService.Delete`).
* Deletion is **terminal**: an `Index-FAQ` job enqueued later for a deleted FAQ fails its `Get` and is discarded by River after retries; `Answer` on a deleted FAQ returns 404.

---

## 4. Core Go Contracts

### 4.1 FAQ Repository (consumer-side, `internal/faq`)

```go
// internal/faq/repository.go — added to the existing interface
type Repository interface {
    // ...existing methods (CreateFile, CreateUnanswered, ListByStatus, Get, Answer, SetLastIndexedHash)

    // Delete removes the faqs row and the FAQ's derived files row. The
    // chunks rows cascade with the files row. Returns ErrorResourceNotFound
    // when the FAQ does not exist.
    Delete(ctx context.Context, id uuid.UUID) error
}
```

Notes:
* `Delete` is **transaction-agnostic** — the service wraps it in `application.UnitOfWork`; the repo uses `Executor(ctx, r.db)`, so it participates in whatever transaction the context carries.
* The postgres implementation resolves the FAQ's `file_id` internally (via `Get`) so callers only ever pass the FAQ id.

### 4.2 FAQ Service

```go
// internal/faq/service.go
type JobQueue interface {
    EnqueueIndexFaq(ctx context.Context, args worker.IndexFaqArgs) error
    EnqueueDeleteFile(ctx context.Context, key string) error   // added
}

// Delete hard-deletes the FAQ and its derived state. The faqs row and the
// files row (chunks cascade) are removed in one transaction; the S3 markdown
// object is deleted asynchronously after commit.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
    err := s.uow.Do(ctx, func(txCtx context.Context) error {
        return s.repo.Delete(txCtx, id)
    })
    if err != nil {
        return err
    }
    return s.queue.EnqueueDeleteFile(ctx, domain.FAQS3Key(id))
}
```

* The transaction lives in the **service** (consistent with `RecordUnanswered`/`Answer`).
* The enqueue happens **after commit**, so a crash between commit and enqueue leaves only an S3 object behind — harmless, idempotently deletable.

### 4.3 HTTP Handler

```go
// internal/faq/handler.go
type FAQService interface {
    // ...existing methods
    Delete(ctx context.Context, id uuid.UUID) error   // added
}

// routes
{
    HttpMethod: http.MethodDelete,
    Path:       "/faqs/{uuid}",
    Handler:    handler.MakeHandler(h.Delete),
    Middlewares: []func(http.Handler) http.Handler{
        middlewares.RequirePermission("faqs.delete"),
    },
}
```

* `Delete` handler parses the path param (`handler.ParsePathParam[uuid.UUID]`), calls the service, and responds `200 OK` with `handler.DefaultSuccessResponse` (`{"message":"success"}`) — the same shape as `DELETE /api/files/{uuid}`.
* Errors flow through `GlobalErrorMiddleware`: `404` (not found, invalid uuid), `401` (missing/invalid auth or permission).

### 4.4 Permissions

`db/seed/roles_resource_users.sql` gains `('faqs', 'delete', 'faqs.delete')`; the existing `resource = 'faqs'` role grant insert covers `superadmin` with no further change.

---

## 5. HTTP Contract (Internal Users)

**Delete a FAQ** — `DELETE /api/faqs/{id}`

| | |
|---|---|
| Auth | `RequireAuth` + `RequirePermission("faqs.delete")` |
| Path | `/api/faqs/{id}` (uuid) |
| Body | none |
| Success | `200 OK` `{"data":{"message":"success"}}` |
| Not found | `404` `{"error":{"title":"server error","message":"faq not found"}}` |
| Bad uuid | `404` (existing `ErrorPathParamValue` mapping) |
| Unauthorized / no permission | `401` |

---

## 6. Tests

Matching the existing mockery + testify patterns:

* **Repository** (`faq_repository_test.go`): `Delete` success (Get → `DELETE FROM faqs` → file delete, order asserted), `Delete` not-found.
* **Service** (`service_test.go`): `Delete` success (UoW → `repo.Delete` → `EnqueueDeleteFile` with `faq/<id>.md`), 404 propagation, transaction failure propagation, enqueue failure propagation.
* **Handler** (`handler_test.go`): `Delete` success (200), invalid uuid (404), service error (500).
* **E2E** (worker + mock AI, as in the FAQ pipeline PRD): answer+index a FAQ → `DELETE /api/faqs/{id}` → assert `faqs`/`files`/`chunks` rows gone, S3 object gone, `GET /api/faqs` and `GET /api/files` unaffected, second `DELETE` → 404.

---

## 7. Known Limitations (MVP)

* **In-flight jobs are not cancelled.** An `Index-FAQ` or `Process-RAG-File` job already running at delete time fails its lookup (rows are gone) and is discarded by River after retry backoff. Narrow, admin-triggered window — accepted for the MVP.
* **No restore.** Deletion is permanent; no audit trail of deletions is recorded.

---

## 8. Success Criteria

1. **Atomic removal:** `DELETE /api/faqs/{id}` removes the `faqs` row, the derived `files` row, and every `chunks` row for that file — all-or-nothing.
2. **Retrieval closure:** after deletion, the FAQ's question no longer returns its curated chunk via `SearchSimilar` (no chunks remain).
3. **Listing consistency:** the FAQ disappears from `GET /api/faqs`; `GET /api/files` never shows the FAQ's file row (pre- and post-delete).
4. **S3 cleanup:** the `faq/<id>.md` object is deleted by the existing `Delete-File-From-Storage` worker; deleting a never-indexed FAQ is a no-op.
5. **Errors:** deleting a non-existent FAQ returns `404`; unauthenticated or unauthorized calls return `401`.
6. **No regression:** `UNIQUE (file_id, chunk_index)` is never violated; the re-index flow (edit → `Index-FAQ`) is unaffected.
