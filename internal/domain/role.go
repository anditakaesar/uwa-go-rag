package domain

import (
	"fmt"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
)

type Role struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
	IsSystem    bool
}

func (r *Role) IDName() string {
	return fmt.Sprintf("%d - %s", r.ID, r.Name)
}

type FetchRoleParam struct {
	ID   *int64
	Name *string
}

type FetchAllRoleParam struct {
	NameLike   *string           `json:"namelike"`
	Pagination common.Pagination `json:"pagination"`
}

func (param *FetchAllRoleParam) Normalize() {
	param.Pagination.Normalize()
}
