package handler

import (
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

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

func (h *RoleApi) GetRole(w http.ResponseWriter, r *http.Request) error {
	id, err := parseIDParam(r)
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
