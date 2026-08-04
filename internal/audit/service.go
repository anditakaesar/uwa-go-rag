package audit

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

// AuditLog Carry context
type auditCtxKey string

const AuditKey auditCtxKey = "AUDIT_KEY"

type Repository interface {
	Insert(ctx context.Context, auditlog domain.AuditLog) error
	FindAll(ctx context.Context, param *AuditLogFetchParam) ([]domain.AuditLog, error)
}

type Recorder interface {
	Record(ctx context.Context, auditlog domain.AuditLog) error
	FindAll(ctx context.Context, param AuditLogFetchParam) ([]domain.AuditLog, *AuditLogFetchParam, error)
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

type AuditLogFetchParam struct {
	ResourceNameLike *string           `json:"resourceNameLike"`
	Pagination       common.Pagination `json:"pagination"`
}

func (p *AuditLogFetchParam) Normalize() {
	p.Pagination.Normalize()
}

func (r *AuditRecorder) FindAll(ctx context.Context, param AuditLogFetchParam) ([]domain.AuditLog, *AuditLogFetchParam, error) {
	param.Normalize()
	auditlogs, err := r.repo.FindAll(ctx, &param)
	if err != nil {
		return nil, nil, err
	}
	return auditlogs, &param, nil
}
