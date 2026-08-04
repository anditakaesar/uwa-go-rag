package role

import (
	"context"
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/go-chi/chi/v5"
)

// adapters
type IRoleService interface {
	FetchAll(ctx context.Context, param domain.FetchAllRoleParam) ([]domain.Role, *domain.FetchAllRoleParam, error)
	Get(ctx context.Context, id int64) (*domain.Role, error)
}

// routes
func SetupRoleApiRoutes(router chi.Router, h *RoleApi) {
	protectedEndpoints := []handler.EndpointWithMiddleware{
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/roles",
				Handler:    handler.MakeHandler(h.FetchRoles),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("roles.read")},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/roles/{id}",
				Handler:    handler.MakeHandler(h.GetRole),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("roles.read"),
			},
		},
	}

	for _, e := range protectedEndpoints {
		requiredMiddlewares := []func(http.Handler) http.Handler{
			middlewares.RequireAuth(),
		}
		e.Middlewares = append(requiredMiddlewares, e.Middlewares...)
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}

// dto
type FindRoleRequest struct {
	NameLike *string `json:"name"`
}

func (req *FindRoleRequest) parseParam(r *http.Request) {
	q := r.URL.Query()
	name := q.Get("name")
	if strings.TrimSpace(name) != "" {
		req.NameLike = &name
	}
}

type RoleResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	IDName string `json:"idName"`
}

func RoleToResponse(role domain.Role) RoleResponse {
	return RoleResponse{
		ID:     role.ID,
		Name:   role.Name,
		IDName: role.IDName(),
	}
}

func RolesToListResponse(roles []domain.Role) []RoleResponse {
	results := make([]RoleResponse, 0, len(roles))
	for _, role := range roles {
		r := RoleToResponse(role)
		results = append(results, r)
	}

	return results
}

// handler
type RoleApi struct {
	RoleService IRoleService
}

type RoleApiDeps struct {
	RoleService IRoleService
}

func NewRoleApi(dep RoleApiDeps) *RoleApi {
	return &RoleApi{
		RoleService: dep.RoleService,
	}
}

func (h *RoleApi) FetchRoles(w http.ResponseWriter, r *http.Request) error {
	pagination := handler.ParsePagination(r)

	var req FindRoleRequest
	req.parseParam(r)

	roles, param, err := h.RoleService.FetchAll(r.Context(), domain.FetchAllRoleParam{
		NameLike:   req.NameLike,
		Pagination: pagination,
	})
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, RolesToListResponse(roles), transport.WithMeta(*param))
	return nil
}

func (h *RoleApi) GetRole(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParseIDParam(r)
	if err != nil {
		return &xerror.ErrorNotFound{Message: "not found"}
	}

	role, err := h.RoleService.Get(r.Context(), id)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, RoleToResponse(*role))
	return nil
}
