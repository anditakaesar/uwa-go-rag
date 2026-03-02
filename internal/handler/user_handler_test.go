package handler_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/handler"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestUserApi_CreateUser(test *testing.T) {
	test.Parallel()

	test.Run("success create user", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		m.userSvc.On("CreateUser", m.anything, domain.User{
			Username: "newuser",
			Password: "newpassword",
		}).Return(&domain.User{
			Base: domain.Base{
				ID: 1,
			},
			Username: "newuser",
			Role:     domain.RoleUser,
		}, nil).Once()

		userReq := `{"username":"newuser","password":"newpassword"}`

		req, err := http.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(userReq))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		gotErr := h.CreateUser(rr, req)

		assert.NoError(t, gotErr)
		assert.Equal(t, http.StatusCreated, rr.Code)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when create user", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		m.userSvc.On("CreateUser", m.anything, domain.User{
			Username: "newuser",
			Password: "newpassword",
		}).Return(nil, errors.New("error_CreateUser")).Once()

		userReq := `{"username":"newuser","password":"newpassword"}`

		req, err := http.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(userReq))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		gotErr := h.CreateUser(rr, req)
		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when validate request", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"username":"","password":"password"}`

		req, err := http.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(userReq))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		gotErr := h.CreateUser(rr, req)
		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when validate request", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"username":"username","password":""}`

		req, err := http.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(userReq))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		gotErr := h.CreateUser(rr, req)
		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when decoding request", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"username":"","password":"x}`

		req, err := http.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(userReq))
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		gotErr := h.CreateUser(rr, req)
		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})
}

func TestUserApi_UpdateUser(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		newPass := "new-pass"
		m.userSvc.On("Update", m.anything, int64(1), &domain.UpdateUserParam{
			OldPassword: "old-pass",
			Password:    &newPass,
		}).Return(&domain.User{
			Base: domain.Base{
				ID: 1,
			},
			Username: "user1",
			Role:     domain.RoleUser,
		}, nil).Once()

		userReq := `{"oldPassword":"old-pass","password":"new-pass"}`
		// 1. Create a new chi route context
		rctx := chi.NewRouteContext()

		// 2. Add the "id" parameter to that context
		rctx.URLParams.Add("id", "1")

		// 3. Inject the context into your request
		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))

		ctxUser := context.WithValue(req.Context(), domain.UserCtxKey, &domain.User{
			Base: domain.Base{
				ID: 1,
			},
		})

		req = req.WithContext(context.WithValue(ctxUser, chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.NoError(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when update", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		newPass := "new-pass"
		m.userSvc.On("Update", m.anything, int64(1), &domain.UpdateUserParam{
			OldPassword: "old-pass",
			Password:    &newPass,
		}).Return(nil, errors.New("error_Update")).Once()

		userReq := `{"oldPassword":"old-pass","password":"new-pass"}`
		// 1. Create a new chi route context
		rctx := chi.NewRouteContext()

		// 2. Add the "id" parameter to that context
		rctx.URLParams.Add("id", "1")

		// 3. Inject the context into your request
		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))

		ctxUser := context.WithValue(req.Context(), domain.UserCtxKey, &domain.User{
			Base: domain.Base{
				ID: 1,
			},
		})

		req = req.WithContext(context.WithValue(ctxUser, chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when verifying user", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"oldPassword":"old-pass","password":"new-pass"}`
		// 1. Create a new chi route context
		rctx := chi.NewRouteContext()

		// 2. Add the "id" parameter to that context
		rctx.URLParams.Add("id", "2")

		// 3. Inject the context into your request
		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))

		ctxUser := context.WithValue(req.Context(), domain.UserCtxKey, &domain.User{
			Base: domain.Base{
				ID: 1,
			},
		})

		req = req.WithContext(context.WithValue(ctxUser, chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when authorizing", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"oldPassword":"old-pass","password":"new-pass"}`
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "2")

		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when validating request", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"oldPassword":"old-pass","password":""}`
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")

		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when validating request 2", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"oldPassword":"","password":""}`
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")

		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when parsing request body", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"oldPassword":"old-pass,"password":""}`
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")

		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when parsing id not a number", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"oldPassword":"old-pass","password":"new-pass"}`
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "not-anumber")

		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error when parsing id empty", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		userReq := `{"oldPassword":"old-pass","password":"new-pass"}`
		rctx := chi.NewRouteContext()

		req, err := http.NewRequest(http.MethodPost, "/api/users/1", bytes.NewBufferString(userReq))
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.UpdateUser(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})
}

func TestUserApi_FetchUsers(test *testing.T) {

	test.Run("success", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		m.userSvc.On("FindAll", m.anything, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 15,
			},
		}).Return([]domain.User{
			{
				Base: domain.Base{
					ID: 1,
				},
				Username: "user1",
				Password: "pass",
				Role:     domain.RoleUser,
			},
			{
				Base: domain.Base{
					ID: 1,
				},
				Username: "user1",
				Password: "pass",
				Role:     domain.RoleUser,
			},
		}, &domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 15,
			},
		}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, "/api/users?page=1&size=15", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.FetchUsers(rr, req)

		assert.NoError(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("success using default pagination", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		m.userSvc.On("FindAll", m.anything, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 10,
			},
		}).Return([]domain.User{
			{
				Base: domain.Base{
					ID: 1,
				},
				Username: "user1",
				Password: "pass",
				Role:     domain.RoleUser,
			},
			{
				Base: domain.Base{
					ID: 1,
				},
				Username: "user1",
				Password: "pass",
				Role:     domain.RoleUser,
			},
		}, &domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 10,
			},
		}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, "/api/users?page=0&size=0", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.FetchUsers(rr, req)

		assert.NoError(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("success when page and size are not number", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		m.userSvc.On("FindAll", m.anything, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 10,
			},
		}).Return([]domain.User{
			{
				Base: domain.Base{
					ID: 1,
				},
				Username: "user1",
				Password: "pass",
				Role:     domain.RoleUser,
			},
			{
				Base: domain.Base{
					ID: 1,
				},
				Username: "user1",
				Password: "pass",
				Role:     domain.RoleUser,
			},
		}, &domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 10,
			},
		}, nil).Once()

		req, err := http.NewRequest(http.MethodGet, "/api/users?page=x&size=x", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.FetchUsers(rr, req)

		assert.NoError(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

	test.Run("error", func(t *testing.T) {
		m, d := setupMocks()

		h := handler.NewUserApi(handler.UserApiDeps{
			UserService: d.UserService,
		})

		m.userSvc.On("FindAll", m.anything, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 15,
			},
		}).Return(nil, nil, errors.New("error_FindAll")).Once()

		req, err := http.NewRequest(http.MethodGet, "/api/users?page=1&size=15", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		gotErr := h.FetchUsers(rr, req)

		assert.Error(t, gotErr)
		m.userSvc.AssertExpectations(t)
	})

}
