package infra

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

type IInfraRolePermissionRepo interface {
	GetPermissionsByUser(ctx context.Context, userID int64) ([]domain.Permission, error)
}
