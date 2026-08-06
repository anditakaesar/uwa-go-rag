package worker

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
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
