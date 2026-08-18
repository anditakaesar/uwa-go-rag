package faq

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type FAQService interface {
	List(ctx context.Context, param *domain.FetchFAQParam) ([]domain.FAQ, error)
	Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64) (*domain.FAQ, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, param *domain.UpdateFAQParam) (*domain.FAQ, error)
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
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPatch,
				Path:       "/faqs/{uuid}",
				Handler:    handler.MakeHandler(h.Update),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("faqs.update"),
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
	req, err := parseParam(r)
	if err != nil {
		return err
	}

	pagination := handler.ParsePagination(r)
	pagination.Normalize()

	param := &domain.FetchFAQParam{
		Status:     req.Status,
		Pagination: pagination,
	}
	result, err := h.FAQService.List(r.Context(), param)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, FAQListToResponse(result), transport.WithMeta(param))
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

func (h *Api) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParsePathParam[uuid.UUID](r, "uuid")
	if err != nil {
		return err
	}

	var req UpdateFAQRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	updatedFaq, err := h.FAQService.Update(r.Context(), id, req.ToDomainParam())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, FAQToAnswerResponse(updatedFaq), transport.WithMeta(req))
	return nil
}
