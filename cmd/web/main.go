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

	env.Load()
	manager := &ServiceManager{}

	db, err := infra.NewDatabase(ctx, env.Values.DBUrl)
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("unable to connect to database: %v", err))
		os.Exit(1)
	}

	client, err := infra.NewS3Client(ctx, infra.S3ClientDependency{
		EndpointURL: env.S3Conf.S3Endpoint,
		AccessKey:   env.S3Conf.S3AccessKey,
		SecretKey:   env.S3Conf.S3SecretKey,
		Region:      env.S3Conf.S3Region,
	})
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("unable to connect to storage service: %v", err))
		os.Exit(1)
	}

	executor := server.SetupServer(&server.ServerDependency{
		DB: db, StorageClient: client,
	})
	manager.Register(NewDBServer(db))
	manager.Register(NewWebServer("web-server", executor.Mux, env.Values.Port))
	manager.Register(NewWorkerServer("river-worker", executor.RiverClient))

	if err := manager.Start(ctx); err != nil {
		xlog.Logger.Error(fmt.Sprintf("error starting services: %v", err))
		os.Exit(1)
	}
}
