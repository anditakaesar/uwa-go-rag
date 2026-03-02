package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

func main() {
	db, err := infra.NewDatabase(context.Background(), env.Values.DBUrl)
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("unable to connect to database: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	svr := server.SetupServer(&server.ServerDependency{
		DB: db,
	})
	xlog.Logger.Info(fmt.Sprintf("server listened at port: %s", env.Values.Port))
	err = http.ListenAndServe(env.Values.Port, svr)
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("failed to start server: %v", err))
		os.Exit(1)
	}
}
