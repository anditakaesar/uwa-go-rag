package handler_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/mocks"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockItems struct {
	cookieSvc   *mocks.MockICookieService
	userSvc     *mocks.MockIUserService
	jwtSvc      *mocks.MockIJWTService
	fileSvc     *mocks.MockIFileService
	webRenderer handler.IWebRenderer
	anything    string
}

type mockWebRenderer struct {
	render2Fn func(ctx context.Context, w http.ResponseWriter, s string, m map[string]any)
}

func (r *mockWebRenderer) Render(w http.ResponseWriter, name string, data any) {}

func (r *mockWebRenderer) Render2(ctx context.Context, w http.ResponseWriter, name string, data map[string]any) {
	r.render2Fn(ctx, w, name, data)
}

func newMockWebRenderer(render2Fn func(ctx context.Context, w http.ResponseWriter, s string, m map[string]any)) handler.IWebRenderer {
	return &mockWebRenderer{
		render2Fn: render2Fn,
	}
}

func setupMocks() (*mockItems, handler.MainHandlerDeps) {
	cookieSvc := new(mocks.MockICookieService)
	userSvc := new(mocks.MockIUserService)
	jwtSvc := new(mocks.MockIJWTService)
	fileSvc := new(mocks.MockIFileService)
	renderFn := func(ctx context.Context, w http.ResponseWriter, s string, m map[string]any) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "rendered")
	}
	webRenderer := newMockWebRenderer(renderFn)

	return &mockItems{
			cookieSvc:   cookieSvc,
			userSvc:     userSvc,
			jwtSvc:      jwtSvc,
			webRenderer: webRenderer,
			fileSvc:     fileSvc,
			anything:    mock.Anything,
		}, handler.MainHandlerDeps{
			UserService:   userSvc,
			JWTService:    jwtSvc,
			CookieService: cookieSvc,
			FileService:   fileSvc,
			WebRenderer:   webRenderer,
		}
}

func TestNewMainHandler(test *testing.T) {
	test.Run("success", func(t *testing.T) {
		got := handler.NewMainHandler(handler.MainHandlerDeps{
			UserService:   new(mocks.MockIUserService),
			JWTService:    new(mocks.MockIJWTService),
			CookieService: new(mocks.MockICookieService),
			WebRenderer:   new(mocks.MockIWebRenderer),
		})
		assert.NotNil(t, got)
		assert.Equal(t, got.CookieService, new(mocks.MockICookieService))
		assert.Equal(t, got.JWTService, new(mocks.MockIJWTService))
		assert.Equal(t, got.UserService, new(mocks.MockIUserService))
	})
}

func TestMainHandler_Index(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewMainHandler(d)

		sessionMock := &sessions.Session{
			Values: map[any]any{
				"user_id":  "1",
				"username": "user1",
			},
		}

		m.cookieSvc.On("Get", mock.Anything, "auth_session").Return(sessionMock, nil).Once()

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		h.Index(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "rendered")
		m.cookieSvc.AssertExpectations(t)
	})

	test.Run("success and token found", func(t *testing.T) {
		m, _ := setupMocks()

		h := &handler.MainHandler{
			CookieService: m.cookieSvc,
			Render: func(ctx context.Context, w http.ResponseWriter, s string, m map[string]any) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "rendered")
			},
		}

		sessionMock := &sessions.Session{
			Values: map[any]any{
				"user_id":  "1",
				"username": "user1",
				"token":    "some-token-here",
			},
		}

		m.cookieSvc.On("Get", mock.Anything, "auth_session").Return(sessionMock, nil).Once()

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		h.Index(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "rendered")
		m.cookieSvc.AssertExpectations(t)
	})

	test.Run("error when get session", func(t *testing.T) {
		m, _ := setupMocks()

		h := &handler.MainHandler{
			CookieService: m.cookieSvc,
			Render: func(ctx context.Context, w http.ResponseWriter, s string, m map[string]any) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "rendered")
			},
		}

		m.cookieSvc.On("Get", mock.Anything, "auth_session").Return(nil, errors.New("error_GetSession")).Once()

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		h.Index(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		m.cookieSvc.AssertExpectations(t)
	})
}

func TestMainHandler_GetLogin(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		_, d := setupMocks()
		h := handler.NewMainHandler(d)

		req, err := http.NewRequest(http.MethodGet, "/login", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		h.GetLogin(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "rendered", rr.Body.String())
	})
}

func TestMainHandler_DoLogout(test *testing.T) {
	test.Parallel()

	test.Run("success logout", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewMainHandler(d)

		session := &sessions.Session{
			ID: "1",
			Values: map[any]any{
				"user_id": 1,
				"token":   "some-token",
			},
		}

		m.cookieSvc.On("Get", m.anything, "auth_session").Return(session, nil).Once()
		m.cookieSvc.On("Save", session, m.anything, m.anything).Return(nil).Once()

		req, err := http.NewRequest(http.MethodGet, "/logout", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()

		gotErr := h.DoLogout(rr, req)
		assert.NoError(t, gotErr)
		assert.Empty(t, session.Values["user_id"])
		assert.Equal(t, http.StatusSeeOther, rr.Code)
		m.cookieSvc.AssertExpectations(t)
	})

	test.Run("error when save cookies", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewMainHandler(d)

		session := &sessions.Session{
			ID: "1",
			Values: map[any]any{
				"user_id": 1,
				"token":   "some-token",
			},
		}

		m.cookieSvc.On("Get", m.anything, "auth_session").Return(session, nil).Once()
		m.cookieSvc.On("Save", session, m.anything, m.anything).Return(errors.New("error_Save")).Once()

		req, err := http.NewRequest(http.MethodGet, "/logout", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()

		gotErr := h.DoLogout(rr, req)
		assert.Error(t, gotErr)
		assert.Empty(t, session.Values["user_id"])
		m.cookieSvc.AssertExpectations(t)
	})

	test.Run("error when get cookies", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewMainHandler(d)

		session := &sessions.Session{
			ID: "1",
			Values: map[any]any{
				"user_id": 1,
				"token":   "some-token",
			},
		}

		m.cookieSvc.On("Get", m.anything, "auth_session").Return(session, errors.New("error_Get")).Once()

		req, err := http.NewRequest(http.MethodGet, "/logout", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()

		gotErr := h.DoLogout(rr, req)
		assert.Error(t, gotErr)
		m.cookieSvc.AssertExpectations(t)
	})
}

func TestMainHandler_DoLogin(test *testing.T) {
	test.Run("success", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		m.userSvc.On("AuthenticateUser", m.anything, "registereduser", "password123").Return(&domain.User{
			Base: domain.Base{
				ID: 1,
			},
			Username: "registereduser",
			Role:     domain.RoleUser,
		}, nil).Once()

		session := &sessions.Session{
			ID:     "1",
			Values: make(map[interface{}]interface{}),
		}
		m.cookieSvc.On("Get", m.anything, "auth_session").Return(session, nil).Once()

		m.jwtSvc.On("IssueJWT", int64(1), m.anything).Return("fake-jwt", nil).Once()

		session.Values["user_id"] = 1
		session.Values["username"] = "registereduser"
		m.cookieSvc.On("Save", session, m.anything, m.anything).Return(nil).Once()

		form := url.Values{"username": {"registereduser"}, "password": {"password123"}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		gotErr := h.DoLogin(rr, req)
		assert.NoError(t, gotErr)
		assert.Equal(t, http.StatusSeeOther, rr.Code)
		assert.Equal(t, "fake-jwt", session.Values["token"])

		m.userSvc.AssertExpectations(t)
		m.cookieSvc.AssertExpectations(t)
		m.jwtSvc.AssertExpectations(t)
	})

	test.Run("error when cookie save", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		m.userSvc.On("AuthenticateUser", m.anything, "registereduser", "password123").Return(&domain.User{
			Base: domain.Base{
				ID: 1,
			},
			Username: "registereduser",
			Role:     domain.RoleUser,
		}, nil).Once()

		session := &sessions.Session{
			ID:     "1",
			Values: make(map[interface{}]interface{}),
		}
		m.cookieSvc.On("Get", m.anything, "auth_session").Return(session, nil).Once()

		m.jwtSvc.On("IssueJWT", int64(1), m.anything).Return("fake-jwt", nil).Once()

		session.Values["user_id"] = 1
		session.Values["username"] = "registereduser"
		m.cookieSvc.On("Save", session, m.anything, m.anything).Return(errors.New("error_SaveCookie")).Once()

		form := url.Values{"username": {"registereduser"}, "password": {"password123"}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		gotErr := h.DoLogin(rr, req)
		assert.Error(t, gotErr)

		m.userSvc.AssertExpectations(t)
		m.cookieSvc.AssertExpectations(t)
		m.jwtSvc.AssertExpectations(t)
	})

	test.Run("error when issue jwt", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		m.userSvc.On("AuthenticateUser", m.anything, "registereduser", "password123").Return(&domain.User{
			Base: domain.Base{
				ID: 1,
			},
			Username: "registereduser",
			Role:     domain.RoleUser,
		}, nil).Once()

		session := &sessions.Session{
			ID:     "1",
			Values: make(map[interface{}]interface{}),
		}
		m.cookieSvc.On("Get", m.anything, "auth_session").Return(session, nil).Once()

		m.jwtSvc.On("IssueJWT", int64(1), m.anything).Return("fake-jwt", errors.New("error_IssueJWt")).Once()

		form := url.Values{"username": {"registereduser"}, "password": {"password123"}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		gotErr := h.DoLogin(rr, req)
		assert.Error(t, gotErr)

		m.userSvc.AssertExpectations(t)
		m.cookieSvc.AssertExpectations(t)
		m.jwtSvc.AssertExpectations(t)
	})

	test.Run("error when get session", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		m.userSvc.On("AuthenticateUser", m.anything, "registereduser", "password123").Return(&domain.User{
			Base: domain.Base{
				ID: 1,
			},
			Username: "registereduser",
			Role:     domain.RoleUser,
		}, nil).Once()

		m.cookieSvc.On("Get", m.anything, "auth_session").Return(&sessions.Session{}, errors.New("error_CookieGet")).Once()

		form := url.Values{"username": {"registereduser"}, "password": {"password123"}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		gotErr := h.DoLogin(rr, req)
		assert.Error(t, gotErr)

		m.userSvc.AssertExpectations(t)
		m.cookieSvc.AssertExpectations(t)
		m.jwtSvc.AssertExpectations(t)
	})

	test.Run("error when get authenticate user", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		m.userSvc.On("AuthenticateUser", m.anything, "registereduser", "password123").Return(&domain.User{}, errors.New("error_AuthenticateUser")).Once()

		form := url.Values{"username": {"registereduser"}, "password": {"password123"}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		gotErr := h.DoLogin(rr, req)
		assert.NoError(t, gotErr)
		assert.Equal(t, http.StatusOK, rr.Code)

		m.userSvc.AssertExpectations(t)
		m.cookieSvc.AssertExpectations(t)
		m.jwtSvc.AssertExpectations(t)
	})

	test.Run("error validating request", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		form := url.Values{"username": {""}, "password": {""}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		gotErr := h.DoLogin(rr, req)
		assert.NoError(t, gotErr)
		assert.Equal(t, http.StatusOK, rr.Code)

		m.userSvc.AssertExpectations(t)
		m.cookieSvc.AssertExpectations(t)
		m.jwtSvc.AssertExpectations(t)
	})

	test.Run("error parsing request", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		req := httptest.NewRequest("POST", "/login", strings.NewReader("username=andi&password=foo%ZZ"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		gotErr := h.DoLogin(rr, req)
		assert.Error(t, gotErr)

		m.userSvc.AssertExpectations(t)
		m.cookieSvc.AssertExpectations(t)
		m.jwtSvc.AssertExpectations(t)
	})
}

func TestMainHandler_GetUploadPage(test *testing.T) {
	test.Run("success", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewMainHandler(d)
		req, err := http.NewRequest(http.MethodGet, "/upload", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		h.GetUploadPage(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "rendered")
		m.cookieSvc.AssertExpectations(t)
	})

}

func TestMainHandler_PostUpload(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		m.fileSvc.On("Save", m.anything, m.anything).Return("test.png", nil).Once()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.png")
		part.Write([]byte("fake-file-content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.PostUpload(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		m.fileSvc.AssertExpectations(t)
	})

	test.Run("error on save", func(t *testing.T) {
		m, d := setupMocks()
		h := handler.NewMainHandler(d)

		var capturedData map[string]any
		d.WebRenderer.(*mockWebRenderer).render2Fn = func(ctx context.Context, w http.ResponseWriter, s string, data map[string]any) {
			capturedData = data
			w.WriteHeader(http.StatusOK)
		}

		m.fileSvc.On("Save", m.anything, m.anything).Return("", errors.New("error_Save")).Once()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.png")
		part.Write([]byte("fake-file-content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		h.PostUpload(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "error while performing save file request", capturedData["Error"])

		m.fileSvc.AssertExpectations(t)
	})

	test.Run("error parsemultipart form", func(t *testing.T) {
		m, d := setupMocks()

		var capturedData map[string]any
		d.WebRenderer.(*mockWebRenderer).render2Fn = func(ctx context.Context, w http.ResponseWriter, s string, data map[string]any) {
			capturedData = data
			w.WriteHeader(http.StatusOK)
		}

		h := handler.NewMainHandler(d)

		// Send a Content-Type that claims to be multipart but has no boundary
		req := httptest.NewRequest("POST", "/upload", strings.NewReader("not a multipart body"))
		req.Header.Set("Content-Type", "multipart/form-data") // Missing boundary parameter

		rr := httptest.NewRecorder()
		h.PostUpload(rr, req)

		assert.Equal(t, "error when parsing file", capturedData["Error"])
		m.fileSvc.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	test.Run("error file handler", func(t *testing.T) {
		m, d := setupMocks()

		var capturedData map[string]any
		d.WebRenderer.(*mockWebRenderer).render2Fn = func(ctx context.Context, w http.ResponseWriter, s string, data map[string]any) {
			capturedData = data
			w.WriteHeader(http.StatusOK)
		}

		h := handler.NewMainHandler(d)

		// Create a valid multipart body but use the wrong field name
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("wrong_field_name", "test.png") // Not "file"
		part.Write([]byte("some data"))
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		h.PostUpload(rr, req)

		assert.Equal(t, "bad request", capturedData["Error"])
		m.fileSvc.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})
}
