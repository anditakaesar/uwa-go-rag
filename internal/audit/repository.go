package audit

import (
	"context"
	"fmt"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/repo"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/henvic/pgq"
	"github.com/jackc/pgx/v5"
)

type AuditRepository struct {
	db repo.IDBExecutor
}

func NewAuditRepository(db repo.IDBExecutor) *AuditRepository {
	return &AuditRepository{
		db: db,
	}
}

func (r *AuditRepository) GetExecutor(ctx context.Context) repo.IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

const auditLogColumns = "id, resource_name, resource_id, actor_id, actor_name, actor_type, action, before, after, metadata, created_at"

func scanAuditLog(row pgx.Row) (*domain.AuditLog, error) {
	var model domain.AuditLog
	err := row.Scan(
		&model.ID,
		&model.ResourceName,
		&model.ResourceID,
		&model.ActorID,
		&model.ActorName,
		&model.ActorType,
		&model.Action,
		&model.Before,
		&model.After,
		&model.Metadata,
		&model.CreatedAt,
	)
	if err != nil {
		if err.Error() == pgx.ErrNoRows.Error() {
			return nil, &xerror.ErrorResourceNotFound{Message: "audit_log not found"}
		}
		return nil, err
	}

	return &model, nil
}

func (r *AuditRepository) Insert(ctx context.Context, auditlog domain.AuditLog) error {
	query := `
		INSERT INTO audit_logs
		("resource_name", "resource_id", "actor_id", "actor_name", "actor_type", "action",
		"before", "after", "metadata", "created_at") VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW());
	`

	rows, err := r.GetExecutor(ctx).Query(ctx, query, auditlog.ToArgs()...)
	if err != nil {
		return err
	}

	defer rows.Close()
	return nil
}

func (r *AuditRepository) FindAll(ctx context.Context, param *AuditLogFetchParam) ([]domain.AuditLog, error) {
	selectQuery := pgq.Select(auditLogColumns).From("audit_logs").OrderBy("audit_logs.id DESC")

	if param.ResourceNameLike != nil {
		selectQuery = selectQuery.Where("audit_log.resource_name like ?", fmt.Sprint("%", *param.ResourceNameLike, "%"))
	}

	countQuery, countArgs, err := pgq.Select(repo.COUNT_AS_TOTAL).FromSelect(selectQuery, "al").SQL()
	if err != nil {
		return nil, err
	}

	err = r.GetExecutor(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&param.Pagination.Total)
	if err != nil {
		return nil, err
	}

	param.Pagination.WrapPaging(&selectQuery)
	query, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	rows, err := r.GetExecutor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	auditLogs := []domain.AuditLog{}

	for rows.Next() {
		al, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		auditLogs = append(auditLogs, *al)
	}

	return auditLogs, rows.Err()
}
