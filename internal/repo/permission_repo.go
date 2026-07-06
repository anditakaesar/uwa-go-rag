package repo

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/jackc/pgx/v5"
)

type PermissionRepo struct {
	db IDBExecutor
}

func NewPermissionRepo(db IDBExecutor) *PermissionRepo {
	return &PermissionRepo{
		db: db,
	}
}

func (r *PermissionRepo) GetExecutor(ctx context.Context) IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}
