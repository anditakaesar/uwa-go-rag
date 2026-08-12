package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

func main() {
	ragSvc := rag.NewRagService()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	file, err := os.ReadFile("testfile.md")
	if err != nil {
		xlog.Logger.Error("error open file", "err", err.Error())
		os.Exit(1)
	}

	result, err := ragSvc.BuildChunks(ctx, file)
	if err != nil {
		xlog.Logger.Error("error build chunk", "err", err.Error())
		os.Exit(1)
	}

	for i, res := range result {
		fmt.Printf("i: %d, %v\n------\n", i, res)
	}
}
