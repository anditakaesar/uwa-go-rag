package middlewares_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockItems struct {
	cookieSvc *mocks.MockICookieService
	userSvc   *mocks.MockIUserService
	jwtSvc    *mocks.MockIJWTService
	anything  string
}

func setupMocks() *mockItems {
	cookieSvc := new(mocks.MockICookieService)
	userSvc := new(mocks.MockIUserService)
	jwtSvc := new(mocks.MockIJWTService)

	return &mockItems{
		cookieSvc: cookieSvc,
		userSvc:   userSvc,
		jwtSvc:    jwtSvc,
		anything:  mock.Anything,
	}
}

func TestCSRFMiddleware(test *testing.T) {
	test.Parallel()

	env.Values.CSRFSecret = "32-byte-long-auth-key-goes-here-"
	env.Values.Env = "development"

	test.Run("reject missing token", func(t *testing.T) {
		// Setup a dummy handler that should only be reached if CSRF passes
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Apply the middleware
		middleware := middlewares.CSRFMiddleware()
		handlerToTest := middleware(nextHandler)

		// Create a POST request (CSRF usually ignores GET/HEAD by default)
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		// Assert: Gorilla CSRF returns 403 Forbidden on failure
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", rr.Code)
		}
	})

	test.Run("provide token", func(t *testing.T) {
		var tokenFound bool

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// gorilla/csrf provides a TemplateField helper
			// If this doesn't panic and returns a string, the middleware is working
			token := csrf.Token(r)
			if token != "" {
				tokenFound = true
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := middlewares.CSRFMiddleware()
		handlerToTest := middleware(nextHandler)

		// GET requests usually generate the token but don't enforce it
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if !tokenFound {
			t.Error("CSRF token was not found in the request context")
		}
	})

}

func TestResolveAuth(test *testing.T) {
	test.Parallel()

	test.Run("success with cookies", func(t *testing.T) {
		m := setupMocks()

		userID := int64(1)
		sessionMock := &sessions.Session{
			Values: map[any]any{
				"user_id":  userID,
				"username": "user1",
			},
		}

		m.cookieSvc.On("Get", m.anything, "auth_session").Return(sessionMock, nil).Once()

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := r.Context().Value(domain.IdentityKey).(domain.Identity)
			if !ok {
				t.Error("Identity not found in context")
			}
			if identity.UserID != userID {
				t.Errorf("Expected UserID %d, got %d", userID, identity.UserID)
			}
			if identity.Method != "session" {
				t.Errorf("Expected method 'session', got %s", identity.Method)
			}

			w.WriteHeader(http.StatusOK)
		})
		middleware := middlewares.ResolveAuth(m.cookieSvc, m.userSvc, m.jwtSvc)

		handlerToTest := middleware(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		m.cookieSvc.AssertExpectations(t)
	})

	test.Run("success with JWT", func(t *testing.T) {
		m := setupMocks()

		// Simulate cookie failure so it falls through to JWT
		m.cookieSvc.On("Get", m.anything, "auth_session").Return(nil, errors.New("no cookie")).Once()

		// Mock JWT verification
		expectedID := int64(99)
		m.jwtSvc.On("Verify", "valid-token").Return(domain.UserClaims{UserID: expectedID}, nil).Once()

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := r.Context().Value(domain.IdentityKey).(domain.Identity)
			assert.Equal(t, "jwt", identity.Method)
			assert.Equal(t, expectedID, identity.UserID)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		middleware := middlewares.ResolveAuth(m.cookieSvc, m.userSvc, m.jwtSvc)
		middleware(nextHandler).ServeHTTP(httptest.NewRecorder(), req)

		m.jwtSvc.AssertExpectations(t)
	})

	test.Run("no credentials provided", func(t *testing.T) {
		m := setupMocks()

		// 1. Mock the cookie store to return an error (no session found)
		m.cookieSvc.On("Get", m.anything, "auth_session").
			Return(nil, errors.New("no cookie")).Once()

		// 2. The Spy handler
		nextCalled := false
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			// Verify identity is NOT in context
			id := r.Context().Value(domain.IdentityKey)
			if id != nil {
				t.Errorf("Expected nil identity in context, got %v", id)
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := middlewares.ResolveAuth(m.cookieSvc, m.userSvc, m.jwtSvc)

		// 3. Request with NO headers and NO cookies
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		// 4. Assertions
		if !nextCalled {
			t.Error("Next handler was not called")
		}
		m.cookieSvc.AssertExpectations(t)
	})
}

func TestRequireAuth(test *testing.T) {
	// 1. Setup the middleware
	middleware := middlewares.RequireAuth()

	test.Run("authorized - user in context", func(t *testing.T) {
		nextCalled := false
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		// Create a request and inject the identity manually
		req := httptest.NewRequest(http.MethodGet, "/upload", nil)
		ctx := context.WithValue(req.Context(), domain.IdentityKey, domain.Identity{UserID: 1})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		middleware(nextHandler).ServeHTTP(rr, req)

		// Assert: It should pass through
		if !nextCalled {
			t.Error("Expected next handler to be called")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	test.Run("unauthorized - no identity in context", func(t *testing.T) {
		nextCalled := false
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		// Request with NO context value
		req := httptest.NewRequest(http.MethodGet, "/upload", nil)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		// Assert: It should be blocked
		if nextCalled {
			t.Error("Expected next handler NOT to be called")
		}

		// Note: Depending on your implementation, this might be a 401
		// or a 302 Redirect to /login
		if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusFound {
			t.Errorf("Expected unauthorized or redirect, got %d", rr.Code)
		}
	})
}

func TestRequireRole(test *testing.T) {
	test.Parallel()

	middleware := middlewares.RequireRole([]domain.Role{domain.RoleAdmin})

	test.Run("no user in context", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Next handler should not have been called")
		})

		// Mock a user that is NOT an admin
		req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
		ctx := context.Background()

		rr := httptest.NewRecorder()
		middleware(nextHandler).ServeHTTP(rr, req.WithContext(ctx))

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", rr.Code)
		}
	})

	test.Run("user has wrong role", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Next handler should not have been called")
		})

		// Mock a user that is NOT an admin
		req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
		ctx := context.WithValue(req.Context(), domain.UserCtxKey, &domain.User{
			Base: domain.Base{
				ID: int64(1),
			},
			Role: domain.RoleUser,
		})

		rr := httptest.NewRecorder()
		middleware(nextHandler).ServeHTTP(rr, req.WithContext(ctx))

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", rr.Code)
		}
	})

	test.Run("success", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Mock a user that is NOT an admin
		req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
		ctx := context.WithValue(req.Context(), domain.UserCtxKey, &domain.User{
			Base: domain.Base{
				ID: int64(1),
			},
			Role: domain.RoleAdmin,
		})

		rr := httptest.NewRecorder()
		middleware(nextHandler).ServeHTTP(rr, req.WithContext(ctx))

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}
	})
}

func TestResolveUser(test *testing.T) {
	test.Run("success user exist in context", func(t *testing.T) {
		m := setupMocks()
		m.userSvc.On("GetUserByID", m.anything, int64(1)).Return(&domain.User{
			Base: domain.Base{
				ID: 1,
			},
			Role:     domain.RoleUser,
			Username: "name",
		}, nil).Once()

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// should pass this
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), domain.IdentityKey, domain.Identity{UserID: 1})
		req = req.WithContext(ctx)

		middleware := middlewares.ResolveUser(m.userSvc)
		rr := httptest.NewRecorder()
		middleware(nextHandler).ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusOK, rr.Code)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("success but user don't exist in context", func(t *testing.T) {
		m := setupMocks()

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// should pass this
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		middleware := middlewares.ResolveUser(m.userSvc)
		rr := httptest.NewRecorder()
		middleware(nextHandler).ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusOK, rr.Code)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("success user exist in database", func(t *testing.T) {
		m := setupMocks()
		m.userSvc.On("GetUserByID", m.anything, int64(1)).Return(nil, errors.New("not found")).Once()

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// should pass this
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), domain.IdentityKey, domain.Identity{UserID: 1})
		req = req.WithContext(ctx)

		middleware := middlewares.ResolveUser(m.userSvc)
		rr := httptest.NewRecorder()
		middleware(nextHandler).ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusOK, rr.Code)
		m.userSvc.AssertExpectations(t)
	})
}

func TestGlobalErrorMiddleware(t *testing.T) {
	// 1. Create a handler that panics
	bombHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went horribly wrong!")
	})

	// 2. Wrap it with the middleware
	handlerToTest := middlewares.GlobalErrorMiddleware(bombHandler)

	// 3. Execute the request
	req := httptest.NewRequest(http.MethodGet, "/any-path", nil)
	rr := httptest.NewRecorder()

	// We use a defer/recover here in the TEST itself just in case
	// the middleware FAILS to catch the panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("The middleware failed to recover the panic: %v", r)
		}
	}()

	handlerToTest.ServeHTTP(rr, req)

	// 4. Assertions
	// Your middleware calls SendError with http.StatusInternalServerError
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	// 5. Verify the body content (matching your ErrObj)
	expectedBody := "An unexpected error happened."
	if !strings.Contains(rr.Body.String(), expectedBody) {
		t.Errorf("Expected body to contain '%s', got '%s'", expectedBody, rr.Body.String())
	}
}
