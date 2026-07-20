package handler

import (
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/go-chi/chi/v5"
)

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
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/users/me",
				Handler:    MakeHandler(h.FetchMe),
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
