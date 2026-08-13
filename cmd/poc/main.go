package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/infra/tokenization"
	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

func main() {
	ragSvc := rag.NewRagService(rag.ServiceDependency{
		Tokenizer: tokenization.NewSimpleTokenizer(),
	})

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
		fmt.Printf("i: %d, tokens: %d, path: %v\n%s\n------\n", i, res.TokenCount, res.HeadingPath, res.Content)
	}
}
