package repo

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/jackc/pgx/v5"
)

type AuditRepository struct {
	db IDBExecutor
}

func NewAuditRepository(db IDBExecutor) *AuditRepository {
	return &AuditRepository{
		db: db,
	}
}

func (r *AuditRepository) GetExecutor(ctx context.Context) IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *AuditRepository) Insert(ctx context.Context, auditlog audit.AuditLog) error {
	query := `
		INSERT INTO audit_logs 
		("resource_name", "resource_id", "actor_id", "actor_name", "actor_type", "action", 
		"before", "after", "metadata", "created_at") VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW());
	`

	_, err := r.GetExecutor(ctx).Query(ctx, query, auditlog.ToArgs()...)
	return err
}
