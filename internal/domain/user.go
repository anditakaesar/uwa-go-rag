package domain

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
)

type User struct {
	Base
	Username string
	Password string
	RoleID   int64
}

type ctxKeyUser string

const UserCtxKey ctxKeyUser = env.USER_CTX_KEY

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserCtxKey).(*User)
	return user, ok
}

type UpdateUserParam struct {
	OldPassword string
	Password    *string
	RoleID      *int64
}

type FetchUserParam struct {
	ID        *int64
	Username  *string
	ForUpdate bool
}

type FindAllUsersParam struct {
	UsernameLike *string           `json:"usernamelike"`
	Pagination   common.Pagination `json:"pagination"`
}

func (param *FindAllUsersParam) Normalize() {
	param.Pagination.Normalize()
}
