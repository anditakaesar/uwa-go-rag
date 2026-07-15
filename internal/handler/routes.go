package handler

import (
	"net/http"

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
				//middlewares.RequireRole([]domain.Role{domain.RoleAdmin}),
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
				middlewares.RequirePermission("users.create"),
			},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/users",
				Handler:    MakeHandler(h.FetchUsers),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("users.read"),
			},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/users/{id}/password",
				Handler:    MakeHandler(h.UpdateUserPassword),
			},
			Middlewares: []func(http.Handler) http.Handler{},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodPatch,
				Path:       "/users/{id}",
				Handler:    MakeHandler(h.Update),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("users.update"),
			},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodDelete,
				Path:       "/users/{id}",
				Handler:    MakeHandler(h.Delete),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("users.delete"),
			},
		},
	}

	for _, e := range protectedEndpoints {
		requiredMiddlewares := []func(http.Handler) http.Handler{
			middlewares.RequireAuth(),
		}
		e.Middlewares = append(requiredMiddlewares, e.Middlewares...)
		router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
	}
}

func SetupRoleApiRoutes(router chi.Router, h *RoleApi) {
	protectedEndpoints := []EndpointWithMiddleware{
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/roles",
				Handler:    MakeHandler(h.FetchRoles),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.RequirePermission("roles.read")},
		},
	}

	for _, e := range protectedEndpoints {
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}

func SetupLoginApiRoutes(router chi.Router, h *LoginApi) {
	endpoints := []Endpoint{
		{
			HttpMethod: http.MethodPost,
			Path:       "/login",
			Handler:    MakeHandler(h.ApiLogin),
		},
	}

	router.Group(func(r chi.Router) {
		for _, endpoint := range endpoints {
			r.MethodFunc(endpoint.HttpMethod, endpoint.Path, endpoint.Handler)
		}
	})
}
