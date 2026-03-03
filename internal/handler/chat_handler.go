package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

type ChatApi struct {
	ChatService service.IChatService
}

type ChatApiDeps struct {
	ChatService service.IChatService
}

func NewChatApi(dep ChatApiDeps) *ChatApi {
	return &ChatApi{
		ChatService: dep.ChatService,
	}
}

type ChatReq struct {
	Propmt string `json:"prompt"`
}

func (h *ChatApi) SendMessage(w http.ResponseWriter, r *http.Request) error {
	var req ChatReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	if strings.TrimSpace(req.Propmt) == "" {
		return &xerror.ErrorBadRequest{Message: "prompt required"}
	}

	resp, err := h.ChatService.SendPrompt(r.Context(), req.Propmt)
	if err != nil {
		return err
	}
	data := map[string]string{
		"message": resp,
	}

	// err = h.ChatService.SendSortJob(r.Context(), []string{"Cobra", "Bear", "Anchovie"})
	// if err != nil {
	// 	return err
	// }

	// data := map[string]string{
	// 	"message": "sending job",
	// }

	transport.SendJSON(w, http.StatusOK, data, transport.WithMeta(req))
	return nil
}
