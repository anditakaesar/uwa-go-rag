package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/coder/websocket"
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
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/ws/chat",
				Handler:    handler.MakeHandler(h.HandleWS),
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
	hub         *Hub
}

type ApiDependency struct {
	ChatService ChatService
	Hub         *Hub
}

func NewChatApi(dep ApiDependency) *Api {
	hub := dep.Hub
	if hub == nil {
		hub = NewHub()
	}

	return &Api{
		ChatService: dep.ChatService,
		hub:         hub,
	}
}

func (h *Api) Hub() *Hub {
	return h.hub
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

type WSAskData struct {
	MsgID string `json:"msgId"`
	Text  string `json:"text"`
}

type WSAskRequest struct {
	Type string    `json:"type"`
	Data WSAskData `json:"data"`
}

type WSTokenData struct {
	MsgID string `json:"msgId"`
	Text  string `json:"text"`
}

type WSTokenEnvelope struct {
	Type string      `json:"type"`
	Data WSTokenData `json:"data"`
}

type WsDoneData struct {
	MsgID string `json:"msgId"`
}

type WSDoneEnvelope struct {
	Type string     `json:"type"`
	Data WsDoneData `json:"data"`
}

type WSErrorData struct {
	MsgID   string `json:"msgId"`
	Message string `json:"message"`
}

type WSErrorEnvelope struct {
	Type string      `json:"type"`
	Data WSErrorData `json:"data"`
}

const (
	wsPingInterval = 30 * time.Second
	wsPongTimeout  = 60 * time.Second
)

func (h *Api) HandleWS(w http.ResponseWriter, r *http.Request) error {
	var readTimer *time.Timer
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: env.Get().CorsOptions.AllowedOrigins,
		OnPongReceived: func(_ context.Context, _ []byte) {
			if readTimer != nil {
				readTimer.Reset(wsPongTimeout)
			}
		},
	}) // upgrade connection
	if err != nil {
		return &xerror.ErrorBadRequest{Message: "websocket upgrade failed"}
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.hub.register(c)
	defer h.hub.unregister(c)

	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.Ping(ctx); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		readCtx, readCancel := context.WithCancel(ctx)
		readTimer = time.AfterFunc(wsPongTimeout, readCancel)
		_, data, err := c.Read(readCtx)
		if readTimer != nil {
			readTimer.Stop()
			readTimer = nil
		}
		readCancel()
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				return nil
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		var ask WSAskRequest
		if err := json.Unmarshal(data, &ask); err != nil || ask.Type != "ask" {
			h.sendWSError(ctx, c, ask.Data.MsgID, "expected ask type data")
			continue
		}

		token, _ := json.Marshal(
			WSTokenEnvelope{
				Type: "token",
				Data: WSTokenData{
					MsgID: ask.Data.MsgID,
					Text:  "success",
				},
			})

		time.Sleep(3000 * time.Millisecond)
		err = c.Write(ctx, websocket.MessageText, token)
		if err != nil {
			return err
		}

		done, _ := json.Marshal(
			WSDoneEnvelope{
				Type: "done",
				Data: WsDoneData{MsgID: ask.Data.MsgID},
			})
		err = c.Write(ctx, websocket.MessageText, done)
		if err != nil {
			return err
		}
	}
}

func (h *Api) sendWSError(ctx context.Context, c *websocket.Conn, msgID, message string) {
	payload, _ := json.Marshal(
		WSErrorEnvelope{
			Type: "error",
			Data: WSErrorData{
				MsgID:   msgID,
				Message: message,
			},
		})
	_ = c.Write(ctx, websocket.MessageText, payload)
}
