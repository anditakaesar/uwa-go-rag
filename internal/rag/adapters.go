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
}
