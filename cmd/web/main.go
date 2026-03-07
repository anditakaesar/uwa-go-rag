package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	manager := &ServiceManager{}

	db, err := infra.NewDatabase(ctx, env.Values.DBUrl)
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("unable to connect to database: %v", err))
		os.Exit(1)
	}

	executor := server.SetupServer(&server.ServerDependency{
		DB: db,
	})
	manager.Register(NewDBServer(db))
	manager.Register(NewWebServer("web-server", executor.Mux, env.Values.Port))
	manager.Register(NewWorkerServer("river-worker", executor.RiverClient))

	if err := manager.Start(ctx); err != nil {
		xlog.Logger.Error(fmt.Sprintf("error starting services: %v", err))
		os.Exit(1)
	}
}
