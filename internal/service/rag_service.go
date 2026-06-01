package service

import (
	"context"
	"fmt"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

type IRagService interface {
	ProcessDocument(ctx context.Context, ragFileID int64) error
}

type RagService struct{}

func NewRagService() *RagService {
	return &RagService{}
}

func (s *RagService) ProcessDocument(ctx context.Context, ragFileID int64) error {
	xlog.Logger.Info(fmt.Sprintf("processing file with id: %d", ragFileID))
	return nil
}
