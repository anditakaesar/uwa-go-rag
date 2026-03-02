package infra

import (
	"net/http"

	"github.com/gorilla/sessions"
)

type CookieSvc struct {
	cookieStore *sessions.CookieStore
}

func NewCookieService(isDev bool, secret string) *CookieSvc {
	cookieStore := sessions.NewCookieStore(
		[]byte(secret),
	)

	cookieStore.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !isDev, // true in prod
	}

	return &CookieSvc{
		cookieStore: cookieStore,
	}
}

func (s *CookieSvc) Get(r *http.Request, name string) (*sessions.Session, error) {
	return s.cookieStore.Get(r, name)
}

func (s *CookieSvc) Save(ses *sessions.Session, r *http.Request, w http.ResponseWriter) error {
	return ses.Save(r, w)
}
