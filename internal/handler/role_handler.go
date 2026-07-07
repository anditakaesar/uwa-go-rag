package handler

import (
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
)

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
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func RolesToListResponse(roles []domain.Role) []RoleResponse {
	results := make([]RoleResponse, 0, len(roles))
	for _, role := range roles {
		r := RoleResponse{
			ID:   role.ID,
			Name: role.Name,
		}
		results = append(results, r)
	}

	return results
}

// handler
type RoleApi struct {
	RoleService service.IRoleService
}

type RoleApiDeps struct {
	RoleService service.IRoleService
}

func NewRoleApi(dep RoleApiDeps) *RoleApi {
	return &RoleApi{
		RoleService: dep.RoleService,
	}
}

func (h *RoleApi) FetchRoles(w http.ResponseWriter, r *http.Request) error {
	pagination := parsePagination(r)

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
