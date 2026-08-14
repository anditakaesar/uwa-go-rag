package worker

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/google/uuid"
)

type RagService interface {
	BuildChunks(ctx context.Context, source []byte) ([]rag.FinalChunk, error)
}

// Embedder returns the vector representation of text via the configured
// OpenAI-compatible embeddings endpoint. Implemented by the ai.AIClient; the
// model comes from the client config (AI_EMBEDDING_MODEL) and the dimension
// (1024) is fixed at the client layer and must match the configured model.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type ChunkRepository interface {
	StoreBatch(ctx context.Context, chunks []domain.Chunk) error
	CountEmbeddedByFileID(ctx context.Context, fileID uuid.UUID) (int, error)
}

type JobQueue interface {
	EnqueueGenerateChunks(ctx context.Context, args GenerateChunksArgs) error
	EnqueueMarkFileEmbedded(ctx context.Context, args MarkFileEmbeddedArgs) error
}

type Recorder interface {
	Record(ctx context.Context, auditlog domain.AuditLog) error
}

type ChatService interface {
	DoSort(ctx context.Context, words []string) ([]string, error)
}

type StorageClient interface {
	GetPresignPutURL(ctx context.Context, key string) (string, error)
	GetPresignGetURL(ctx context.Context, key string) (string, error)
	GetObjectIntoBuffer(ctx context.Context, key string) ([]byte, error)
	UploadObject(ctx context.Context, key string, mimeType string, buff []byte) error
	DeleteObject(ctx context.Context, key string) error
}

type FileService interface {
	Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
	SetEmbeddingStatus(ctx context.Context, fileID uuid.UUID, status domain.EmbeddingStatus) error
}
