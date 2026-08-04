package postgres

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Executor pattern, a single base function
func Executor(ctx context.Context, db DBExecutor) DBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return db
}

// Usage
// type MyNewRepo struct {
// 		db repo.IDBExecutor
// }
//
// realDB := db.Executor(ctx, r.db)
