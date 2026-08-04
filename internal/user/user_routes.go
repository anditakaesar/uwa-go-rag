package user

import (
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/go-chi/chi/v5"
)

func SetupUserApiRoutes(router chi.Router, h *UserApi) {
	protectedEndpoints := []handler.EndpointWithMiddleware{
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/users",
				Handler:    handler.MakeHandler(h.CreateUser),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("users.create"),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/users",
				Handler:    handler.MakeHandler(h.FetchUsers),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("users.read"),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/users/{id}/password",
				Handler:    handler.MakeHandler(h.UpdateUserPassword),
			},
			Middlewares: []func(http.Handler) http.Handler{},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPatch,
				Path:       "/users/{id}",
				Handler:    handler.MakeHandler(h.Update),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("users.update"),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodDelete,
				Path:       "/users/{id}",
				Handler:    handler.MakeHandler(h.Delete),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("users.delete"),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/users/me",
				Handler:    handler.MakeHandler(h.FetchMe),
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
