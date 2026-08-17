package chat

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/go-chi/chi/v5"
)

// routes
func SetupRoutes(router chi.Router, h *Api) {
	protectedEndpoints := []handler.EndpointWithMiddleware{
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/chat/raw",
				Handler:    handler.MakeHandler(h.SendMessage),
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

// handler
type Api struct {
	ChatService ChatService
}

type ApiDependency struct {
	ChatService ChatService
}

func NewChatApi(dep ApiDependency) *Api {
	return &Api{
		ChatService: dep.ChatService,
	}
}

type ChatReq struct {
	Prompt string `json:"prompt"`
}

func (h *Api) SendMessage(w http.ResponseWriter, r *http.Request) error {
	var req ChatReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		return &xerror.ErrorBadRequest{Message: "prompt required"}
	}

	resp, err := h.ChatService.Chat(r.Context(), req.Prompt)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, resp, transport.WithMeta(req))
	return nil
}
