package rag

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
)

// ChunkRepository persists generated chunks. Consumed by the chunking pipeline
// and implemented in internal/infra/db/postgres.
type ChunkRepository interface {
	StoreBatch(ctx context.Context, chunks []domain.Chunk) error
	GetByFileID(ctx context.Context, fileID uuid.UUID) ([]domain.Chunk, error)
	DeleteByFileID(ctx context.Context, fileID uuid.UUID) error
	// SearchSimilar returns top-k chunks ordered by cosine similarity against
	// embedding, optionally filtered by a minimum similarity threshold.
	SearchSimilar(ctx context.Context, embedding []float32, limit int, threshold float64) ([]domain.Chunk, error)
}

type JobQueue interface {
	EnqueueRagFile(ctx context.Context, fileID uuid.UUID, objectKey string) error
	EnqueueAuditLog(ctx context.Context, auditLog domain.AuditLog) error
}

type FileRepository interface {
	Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
}
