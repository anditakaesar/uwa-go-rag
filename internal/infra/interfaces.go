package infra

import (
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/gorilla/sessions"
)

type ICookieService interface {
	Get(r *http.Request, name string) (*sessions.Session, error)
	Save(ses *sessions.Session, r *http.Request, w http.ResponseWriter) error
}

type IJWTService interface {
	Verify(token string) (domain.UserClaims, error)
	IssueJWT(userID int64, secret []byte) (string, error)
}
