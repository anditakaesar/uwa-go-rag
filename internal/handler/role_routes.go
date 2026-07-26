package handler

import (
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/go-chi/chi/v5"
)

func SetupRoleApiRoutes(router chi.Router, h *RoleApi) {
	protectedEndpoints := []EndpointWithMiddleware{
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/roles",
				Handler:    MakeHandler(h.FetchRoles),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("roles.read")},
		},
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/roles/{id}",
				Handler:    MakeHandler(h.GetRole),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequirePermission("roles.read"),
			},
		},
	}

	for _, e := range protectedEndpoints {
		requiredMiddlewares := []func(http.Handler) http.Handler{
			middlewares.RequireAuth(),
		}
		e.Middlewares = append(requiredMiddlewares, e.Middlewares...)
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}
