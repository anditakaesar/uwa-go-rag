package audit

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

// AuditLog Carry context
type auditCtxKey string

const AuditKey auditCtxKey = "AUDIT_KEY"

type Repository interface {
	Insert(ctx context.Context, auditlog domain.AuditLog) error
	FindAll(ctx context.Context, param *domain.AuditLogFetchParam) ([]domain.AuditLog, error)
}

type AuditRecorder struct {
	repo Repository
}

func NewAuditLogRecorder(repo Repository) *AuditRecorder {
	return &AuditRecorder{
		repo: repo,
	}
}

func (r *AuditRecorder) Record(ctx context.Context, auditlog domain.AuditLog) error {
	if err := auditlog.Validate(); err != nil {
		return err
	}
	return r.repo.Insert(ctx, auditlog)
}

func (r *AuditRecorder) FindAll(ctx context.Context, param domain.AuditLogFetchParam) ([]domain.AuditLog, *domain.AuditLogFetchParam, error) {
	param.Normalize()
	auditlogs, err := r.repo.FindAll(ctx, &param)
	if err != nil {
		return nil, nil, err
	}
	return auditlogs, &param, nil
}
