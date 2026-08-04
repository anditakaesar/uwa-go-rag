package rag

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/repo"
	"github.com/jackc/pgx/v5"
)

type RagRepository struct {
	db repo.IDBExecutor
}

func NewRagRepository(db repo.IDBExecutor) *RagRepository {
	return &RagRepository{
		db: db,
	}
}

func (r *RagRepository) GetExecutor(ctx context.Context) repo.IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}
