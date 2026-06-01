package repo

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/jackc/pgx/v5"
)

type RagRepository struct {
	db IDBExecutor
}

func NewRagRepository(db IDBExecutor) *RagRepository {
	return &RagRepository{
		db: db,
	}
}

func (r *RagRepository) GetExecutor(ctx context.Context) IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}
