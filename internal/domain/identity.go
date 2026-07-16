package domain

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/golang-jwt/jwt/v5"
)

type Identity struct {
	UserID     int64
	Permission []string
	Method     string // "session" | "token" | "jwt"
}

type UserClaims struct {
	Permissions []string
	jwt.RegisteredClaims
}

type ctxKey string

const IdentityKey ctxKey = env.IDENTITY_KEY

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(IdentityKey).(Identity)
	return id, ok
}

type RefreshTokenClaims struct {
	jwt.RegisteredClaims
}
