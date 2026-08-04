package cookie

import (
	"net/http"

	"github.com/gorilla/sessions"
)

type Service struct {
	cookieStore *sessions.CookieStore
}

func NewService(isDev bool, secret string) *Service {
	cookieStore := sessions.NewCookieStore(
		[]byte(secret),
	)

	return &Service{
		cookieStore: cookieStore,
	}
}

func (s *Service) Get(r *http.Request, name string) (*sessions.Session, error) {
	return s.cookieStore.Get(r, name)
}

func (s *Service) Save(ses *sessions.Session, r *http.Request, w http.ResponseWriter) error {
	return ses.Save(r, w)
}
