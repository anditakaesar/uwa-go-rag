package postgres

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/jackc/pgx/v5"
)

// Base Repository pattern
type Repository struct {
	DB DBExecutor
}

func NewRepository(db DBExecutor) Repository {
	return Repository{
		DB: db,
	}
}

func (r Repository) Executor(ctx context.Context) DBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.DB
}

// usage
// type MyNewRepo struct {
// 		db.Repository // the base repository
// }
//
// func NewRepo(pool repo.IDBExecutor) *MyNewRepo {
// 		return &MyNewRepo{
// 			DB: db.NewRepository(pool)
// 		}
// }
