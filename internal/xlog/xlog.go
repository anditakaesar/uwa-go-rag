package xlog

import (
	"log/slog"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
)

var Logger *slog.Logger

func init() {
	lvl := env.GetLogLevel()
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})

	Logger = slog.New(handler)
}
