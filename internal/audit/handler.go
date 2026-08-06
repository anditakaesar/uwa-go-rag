package audit

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/go-chi/chi/v5"
)

// adapter
type Recorder interface {
	Record(ctx context.Context, auditlog domain.AuditLog) error
	FindAll(ctx context.Context, param domain.AuditLogFetchParam) ([]domain.AuditLog, *domain.AuditLogFetchParam, error)
}

// dto
type FetchAuditLogRequest struct {
	ResourceNameLike *string `json:"resourceName"`
}

func (req *FetchAuditLogRequest) parseParam(r *http.Request) {
	q := r.URL.Query()
	resourceName := q.Get("resourceName")
	if strings.TrimSpace(resourceName) != "" {
		req.ResourceNameLike = &resourceName
	}
}

type AuditLogResponse struct {
	ID int64 `json:"id"`

	ResourceName string `json:"resourceName"`
	ResourceID   string `json:"resourceID"`
	ActorID      *int64 `json:"actorID"`
	ActorName    string `json:"actorName"`
	ActorType    string `json:"actorType"`
	Action       string `json:"action"`

	// Before    any
	// After     any
	// Metadata  any
	CreatedAt time.Time `json:"createdAt"`
}

func AuditLogToResponse(auditlog domain.AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:           auditlog.ID,
		ResourceName: auditlog.ResourceName,
		ResourceID:   auditlog.ResourceID,
		ActorID:      auditlog.ActorID,
		ActorName:    auditlog.ActorName,
		ActorType:    auditlog.ActorType,
		Action:       string(auditlog.Action),
		CreatedAt:    auditlog.CreatedAt,
	}
}

func AuditLogsToListResponse(auditlogs []domain.AuditLog) []AuditLogResponse {
	results := make([]AuditLogResponse, 0, len(auditlogs))
	for _, al := range auditlogs {
		a := AuditLogToResponse(al)
		results = append(results, a)
	}

	return results
}

// routes
func SetupAuditLogApiRoutes(router chi.Router, h *Api) {
	endpoints := []handler.EndpointWithMiddleware{
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/auditlogs",
				Handler:    handler.MakeHandler(h.FindAll),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("audit_logs.read"),
			},
		},
	}

	for _, e := range endpoints {
		requiredMiddlewares := []func(http.Handler) http.Handler{
			middlewares.RequireAuth(),
		}
		e.Middlewares = append(requiredMiddlewares, e.Middlewares...)
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}

// handler
type Api struct {
	AuditLogService Recorder
}

type ApiDependency struct {
	AuditLogService Recorder
}

func NewAuditLogApi(dep ApiDependency) *Api {
	return &Api{
		AuditLogService: dep.AuditLogService,
	}
}

func (h *Api) FindAll(w http.ResponseWriter, r *http.Request) error {
	pagination := handler.ParsePagination(r)
	var req FetchAuditLogRequest
	req.parseParam(r)

	auditlogs, param, err := h.AuditLogService.FindAll(r.Context(), domain.AuditLogFetchParam{
		ResourceNameLike: req.ResourceNameLike,
		Pagination:       pagination,
	})
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, AuditLogsToListResponse(auditlogs), transport.WithMeta(*param))
	return nil
}
