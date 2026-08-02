package xlog

import (
	"log/slog"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
)

var Logger *slog.Logger

// func init() {
func InitializeLogger() {
	lvl := env.GetLogLevel()
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
	})

	Logger = slog.New(handler)
}

func InitializeLoggerFile(file *os.File) {
	lvl := env.GetLogLevel()
	handler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
	})

	Logger = slog.New(handler)
}
