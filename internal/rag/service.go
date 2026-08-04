package rag

import (
	"context"
	"fmt"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

type Service struct{}

func NewRagService() *Service {
	return &Service{}
}

func (s *Service) ProcessDocument(ctx context.Context, ragFileID int64) error {
	xlog.Logger.Info(fmt.Sprintf("processing file with id: %d", ragFileID))
	return nil
}
