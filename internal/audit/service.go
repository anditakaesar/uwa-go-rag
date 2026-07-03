package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

// AuditLog Carry context
type auditCtxKey string

const AuditKey auditCtxKey = "AUDIT_KEY"

type AuditAction string

// User.Create, User.Login, etc
const (
	USER_LOGIN AuditAction = "user.login"
)

type Repository interface {
	Insert(ctx context.Context, auditlog AuditLog) error
}

type Recorder interface {
	Record(ctx context.Context, auditlog AuditLog) error
}

type AuditLog struct {
	ID int64

	ResourceName string
	ResourceID   string
	ActorID      *int64
	ActorName    string
	ActorType    string
	Action       AuditAction

	Before    any
	After     any
	Metadata  any
	CreatedAt time.Time
}

func (auditlog *AuditLog) ToArgs() []any {
	return []any{
		auditlog.ResourceName,
		auditlog.ResourceID,
		auditlog.ActorID,
		auditlog.ActorName,
		auditlog.ActorType,
		auditlog.Action,
		auditlog.Before,
		auditlog.After,
		auditlog.Metadata,
	}
}

func (auditlog *AuditLog) Validate() error {
	errFields := []string{}
	if strings.TrimSpace(auditlog.ResourceName) == "" {
		errFields = append(errFields, "resource_name")
	}

	if strings.TrimSpace(auditlog.ResourceID) == "" {
		errFields = append(errFields, "resource_id")
	}

	if strings.TrimSpace(auditlog.ActorName) == "" {
		errFields = append(errFields, "actor_name")
	}

	if len(errFields) > 0 {
		return &xerror.ErrorAuditLogRecordValidation{
			Message: fmt.Sprintf("error auditlog validation: %v", errFields),
		}
	}

	return nil
}

type AuditRecorder struct {
	repo Repository
}

func NewAuditLogRecorder(repo Repository) *AuditRecorder {
	return &AuditRecorder{
		repo: repo,
	}
}

func (r *AuditRecorder) Record(ctx context.Context, auditlog AuditLog) error {
	if err := auditlog.Validate(); err != nil {
		return err
	}
	return r.repo.Insert(ctx, auditlog)
}
