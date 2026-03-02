package infra

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCookieService(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		svc := NewCookieService(true, "test-secret")

		assert.Equal(t, false, svc.cookieStore.Options.Secure)
	})
}

func TestCookieSvc_GetAndSave(test *testing.T) {
	secret := "very-secret-key"
	svc := NewCookieService(true, secret)
	sessionName := "auth_session"

	test.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://localhost", nil)
		rr := httptest.NewRecorder()

		session, err := svc.Get(req, sessionName)
		assert.NoError(t, err)
		session.Values["user_id"] = 123

		err = svc.Save(session, req, rr)
		assert.NoError(t, err)

		cookieHeader := rr.Result().Header.Get("Set-Cookie")
		assert.NotEqual(t, "", cookieHeader)

		req2 := httptest.NewRequest("GET", "http://localhost", nil)
		req2.Header.Set("Cookie", cookieHeader)

		session2, err := svc.Get(req2, sessionName)
		assert.NoError(t, err)
		assert.Equal(t, 123, session2.Values["user_id"])
	})
}
