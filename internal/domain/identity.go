package domain

import (
	"context"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
)

type Identity struct {
	UserID int64
	Method string // "session" | "token" | "jwt"
}

type UserClaims struct {
	UserID int64
	Exp    time.Time
}

type ctxKey string

const IdentityKey ctxKey = env.IDENTITY_KEY

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(IdentityKey).(Identity)
	return id, ok
}
