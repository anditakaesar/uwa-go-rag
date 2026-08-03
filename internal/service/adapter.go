package service

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
)

type IUserRepository interface {
	CreateUser(ctx context.Context, newUser domain.User) (*domain.User, error)
	CreateUserWithRole(ctx context.Context, newUser domain.User, role string) (*domain.User, error)
	FetchUserByParam(ctx context.Context, param domain.FetchUserParam) (*domain.User, error)
	Update(ctx context.Context, id int64, param domain.UpdateUserParam) (*domain.User, error)
	FindAll(ctx context.Context, param *domain.FindAllUsersParam) ([]domain.UserEnriched, error)
	Delete(ctx context.Context, id int64) (*domain.User, error)
}

type IRagRepository interface {
	//CreateRagFile(ctx context.Context, ragFile domain.RagFile) (*domain.RagFile, error)
}

type IRoleRepository interface {
	FetchRoleByParam(ctx context.Context, param domain.FetchRoleParam) (*domain.Role, error)
	FetchAll(ctx context.Context, param *domain.FetchAllRoleParam) ([]domain.Role, error)
}

type IUnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type IPasswordChecker interface {
	HashPassword(password string) (string, error)
	CheckPassword(password string, hash string) (bool, error)
}

type AIClient interface {
	SendPrompt(ctx context.Context, prompt string) (string, error)
	SendTextForEmbedding(ctx context.Context, text string) ([]float64, error)
}

type IJobQueue interface {
	EnqueueChat(ctx context.Context, words []string) error
	EnqueueRagFile(ctx context.Context, ragFileID int64) error
}

type IStorageClient interface {
	ListFiles(ctx context.Context) ([]string, error)
	GetPresignURL(ctx context.Context, key string) (string, error)
}

type IFileRepository interface {
	Insert(ctx context.Context, newFile domain.File) (*domain.File, error)
	Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
}
