package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

type AuditAction string

// User.Create, User.Login, etc
const (
	USER_LOGIN AuditAction = "user.login"
)

type AuditLog struct {
	ID int64 `json:"id"`

	ResourceName string      `json:"resourceName"`
	ResourceID   string      `json:"resourceID"`
	ActorID      *int64      `json:"actorID"`
	ActorName    string      `json:"actorName"`
	ActorType    string      `json:"actorType"`
	Action       AuditAction `json:"action"`

	Before    any       `json:"before"`
	After     any       `json:"after"`
	Metadata  any       `json:"metadata"`
	CreatedAt time.Time `json:"createdAt"`
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
		auditlog.CreatedAt,
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

type AuditLogFetchParam struct {
	ResourceNameLike *string           `json:"resourceNameLike"`
	Pagination       common.Pagination `json:"pagination"`
}

func (p *AuditLogFetchParam) Normalize() {
	p.Pagination.Normalize()
}
