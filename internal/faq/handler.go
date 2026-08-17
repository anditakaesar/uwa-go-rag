package faq

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type FAQService interface {
	List(ctx context.Context, status domain.FAQStatus, limit, offset int) ([]domain.FAQ, error)
	Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64) (*domain.FAQ, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// dto
type FAQListResponse struct {
	ID        uuid.UUID `json:"id"`
	Question  string    `json:"question"`
	AskedBy   *int64    `json:"askedBy"`
	CreatedAt time.Time `json:"createdAt"`
}

func FAQToListResponse(f domain.FAQ) FAQListResponse {
	return FAQListResponse{
		ID:        f.ID,
		Question:  f.Question,
		AskedBy:   f.AskedBy,
		CreatedAt: f.CreatedAt,
	}
}

func FAQListToResponse(faqs []domain.FAQ) []FAQListResponse {
	list := make([]FAQListResponse, 0, len(faqs))
	for _, f := range faqs {
		list = append(list, FAQToListResponse(f))
	}
	return list
}

type FAQAnswerResponse struct {
	ID         uuid.UUID  `json:"id"`
	Question   string     `json:"question"`
	Answer     string     `json:"answer"`
	Status     string     `json:"status"`
	AnsweredBy *int64     `json:"answeredBy"`
	AnsweredAt *time.Time `json:"answeredAt"`
}

func FAQToAnswerResponse(f *domain.FAQ) FAQAnswerResponse {
	return FAQAnswerResponse{
		ID:         f.ID,
		Question:   f.Question,
		Answer:     f.Answer,
		Status:     string(f.Status),
		AnsweredBy: f.AnsweredBy,
		AnsweredAt: f.AnsweredAt,
	}
}

type AnswerFAQRequest struct {
	Answer string `json:"answer"`
}

// routes
func SetupFAQApiRoutes(router chi.Router, h *Api) {
	endpoints := []handler.EndpointWithMiddleware{
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/faqs",
				Handler:    handler.MakeHandler(h.List),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("faqs.read"),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPut,
				Path:       "/faqs/{uuid}/answer",
				Handler:    handler.MakeHandler(h.Answer),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("faqs.update"),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodDelete,
				Path:       "/faqs/{uuid}",
				Handler:    handler.MakeHandler(h.Delete),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("faqs.delete"),
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
	FAQService FAQService
}

type ApiDependency struct {
	FAQService FAQService
}

func NewFAQApi(dep ApiDependency) *Api {
	return &Api{
		FAQService: dep.FAQService,
	}
}

func (h *Api) List(w http.ResponseWriter, r *http.Request) error {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = string(domain.FAQStatusUnanswered)
	}
	switch domain.FAQStatus(status) {
	case domain.FAQStatusUnanswered, domain.FAQStatusAnswered, domain.FAQStatusDismissed:
	default:
		return &xerror.ErrorValidation{Message: "status is not supported"}
	}

	pagination := handler.ParsePagination(r)
	pagination.Normalize()

	result, err := h.FAQService.List(r.Context(), domain.FAQStatus(status), pagination.Size, pagination.GetOffset())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, FAQListToResponse(result), transport.WithMeta(pagination))
	return nil
}

func (h *Api) Answer(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParsePathParam[uuid.UUID](r, "uuid")
	if err != nil {
		return err
	}

	identity, ok := domain.IdentityFromContext(r.Context())
	if !ok {
		return &xerror.ErrorPermission{Message: "permission required"}
	}

	var req AnswerFAQRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	answered, err := h.FAQService.Answer(r.Context(), id, req.Answer, identity.UserID)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, FAQToAnswerResponse(answered))
	return nil
}

func (h *Api) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParsePathParam[uuid.UUID](r, "uuid")
	if err != nil {
		return err
	}

	err = h.FAQService.Delete(r.Context(), id)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, handler.DefaultSuccessResponse)
	return nil
}
