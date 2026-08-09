package file

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
)

type PasswordChecker interface {
	HashPassword(password string) (string, error)
	CheckPassword(password string, hash string) (bool, error)
}

type StorageClient interface {
	GetPresignPutURL(ctx context.Context, key string) (string, error)
	GetPresignGetURL(ctx context.Context, key string) (string, error)
}

type FileRepository interface {
	Insert(ctx context.Context, newFile domain.File) (*domain.File, error)
	Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
	FindAll(ctx context.Context, param *domain.FindAllFilesParam) ([]domain.File, error)
	Update(ctx context.Context, id uuid.UUID, updateParam domain.UpdateFileParam) (*domain.File, error)
	Delete(ctx context.Context, id uuid.UUID) (*domain.File, error)
}

type JobQueue interface {
	EnqueueThumbnailGen(ctx context.Context, id uuid.UUID) error
	EnqueueDeleteFile(ctx context.Context, key string) error
}
