package handler

import (
	"context"
	"net/http"
)

type IWebRenderer interface {
	Render(w http.ResponseWriter, name string, data any)
	Render2(ctx context.Context, w http.ResponseWriter, name string, data map[string]any)
}
