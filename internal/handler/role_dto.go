package handler

import (
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
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
