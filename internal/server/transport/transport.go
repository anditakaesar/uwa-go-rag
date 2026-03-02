package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

type Response struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

type SendOption func(*Response)

func WithMeta(meta any) SendOption {
	return func(r *Response) {
		r.Meta = meta
	}
}

// ErrorResponse
type ErrObj struct {
	Title   string `json:"title,omitempty"`
	Message string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Error ErrObj `json:"error"`
}

func SendJSON(w http.ResponseWriter, status int, unwrappedData any, opts ...SendOption) {
	response := Response{Data: unwrappedData}

	for _, opt := range opts {
		opt(&response)
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(response); err != nil {
		xlog.Logger.Error(fmt.Sprintf("json encode error: %v", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

func SendError(w http.ResponseWriter, status int, errObj ErrObj) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := ErrorResponse{
		Error: errObj,
	}
	json.NewEncoder(w).Encode(resp)
}
