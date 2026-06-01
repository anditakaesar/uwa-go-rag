package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

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
