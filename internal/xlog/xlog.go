package xlog

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	Logger = slog.New(handler)
}
