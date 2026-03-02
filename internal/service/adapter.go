package service

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

type IUserRepository interface {
	CreateUser(ctx context.Context, newUser domain.User) (*domain.User, error)
	CreateUserAdmin(ctx context.Context, newUser domain.User) (*domain.User, error)
	FetchUserByParam(ctx context.Context, param domain.FetchUserParam) (*domain.User, error)
	Update(ctx context.Context, id int64, param domain.UpdateUserParam) (*domain.User, error)
	FindAll(ctx context.Context, param domain.FindAllUsersParam) ([]domain.User, error)
}

type IUnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type IPasswordChecker interface {
	HashPassword(password string) (string, error)
	CheckPassword(password string, hash string) (bool, error)
}
