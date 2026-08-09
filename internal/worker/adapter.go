package worker

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
)

type RagService interface {
	ProcessDocument(ctx context.Context, ragFileID int64) error
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
}
