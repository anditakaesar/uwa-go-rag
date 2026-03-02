package handler

import (
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/go-chi/chi/v5"
)

func SetupMainRoutes(router chi.Router, h *MainHandler) {
	endpoints := []Endpoint{
		{
			HttpMethod: http.MethodGet,
			Path:       "/",
			Handler:    h.Index,
		},
		{
			HttpMethod: http.MethodGet,
			Path:       "/login",
			Handler:    h.GetLogin,
		},
		{
			HttpMethod: http.MethodPost,
			Path:       "/login",
			Handler:    MakeHandler(h.DoLogin),
		},
	}

	protectedEndpoints := []EndpointWithMiddleware{
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/logout",
				Handler:    MakeHandler(h.DoLogout),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.CSRFMiddleware(),
			},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/upload",
				Handler:    h.GetUploadPage,
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.RequireRole([]domain.Role{domain.RoleAdmin}),
				middlewares.CSRFMiddleware(),
			},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/upload",
				Handler:    h.PostUpload,
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.CSRFMiddleware(),
			},
		},
	}

	router.Group(func(r chi.Router) {
		r.Use(middlewares.CSRFMiddleware())
		for _, endpoint := range endpoints {
			r.MethodFunc(endpoint.HttpMethod, endpoint.Path, endpoint.Handler)
		}
	})

	for _, e := range protectedEndpoints {
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}

func SetupUserApiRoutes(router chi.Router, h *UserApi) {
	protectedEndpoints := []EndpointWithMiddleware{
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/users",
				Handler:    MakeHandler(h.CreateUser),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.RequireRole([]domain.Role{domain.RoleAdmin}),
			},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/users",
				Handler:    MakeHandler(h.FetchUsers),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.RequireRole([]domain.Role{domain.RoleAdmin}),
			},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/users/{id}",
				Handler:    MakeHandler(h.UpdateUser),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
			},
		},
	}

	for _, e := range protectedEndpoints {
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}

func SetupChatApiRoutes(router chi.Router, h *ChatApi) {
	protectedEndpoints := []EndpointWithMiddleware{
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/chat/raw",
				Handler:    MakeHandler(h.SendMessage),
			},
			Middlewares: []func(http.Handler) http.Handler{},
		},
	}

	for _, e := range protectedEndpoints {
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		} else {
			router.MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}
