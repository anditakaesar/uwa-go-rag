package infra

import (
	"context"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/gorilla/sessions"
)

type ICookieService interface {
	Get(r *http.Request, name string) (*sessions.Session, error)
	Save(ses *sessions.Session, r *http.Request, w http.ResponseWriter) error
}

type IJWTService interface {
	Verify(token string) (domain.UserClaims, error)
	IssueJWT(ctx context.Context, userID int64, secret []byte) (string, error)

	VerifyRefreshToken(ctx context.Context, token string) (domain.RefreshTokenClaims, error)
	IssueRefreshToken(ctx context.Context, param common.RefreshTokenParam) (string, error)
}
