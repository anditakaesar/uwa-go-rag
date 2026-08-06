package user

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(ctx context.Context, newUser domain.User) (*domain.User, error)
	CreateUserWithRole(ctx context.Context, newUser domain.User, role string) (*domain.User, error)
	FetchUserByParam(ctx context.Context, param domain.FetchUserParam) (*domain.User, error)
	Update(ctx context.Context, id int64, param domain.UpdateUserParam) (*domain.User, error)
	FindAll(ctx context.Context, param *domain.FindAllUsersParam) ([]domain.UserEnriched, error)
	Delete(ctx context.Context, id int64) (*domain.User, error)
}

type PasswordChecker interface {
	HashPassword(password string) (string, error)
	CheckPassword(password string, hash string) (bool, error)
}

type FileRepository interface {
	Insert(ctx context.Context, newFile domain.File) (*domain.File, error)
	Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
}

type UserService interface {
	CreateUser(ctx context.Context, user domain.User) (*domain.User, error)
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
	UpdatePassword(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error)
	Update(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error)
	FindAll(ctx context.Context, param domain.FindAllUsersParam) ([]domain.UserEnriched, *domain.FindAllUsersParam, error)
	Delete(ctx context.Context, id int64) error
}
