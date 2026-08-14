package server

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

// noopUnansweredRecorder is a placeholder for the FAQ service (see the FAQ
// PRD). It logs captured questions until the FAQ curation pipeline is wired in.
type noopUnansweredRecorder struct{}

func (noopUnansweredRecorder) RecordUnanswered(_ context.Context, question string) error {
	xlog.Logger.Info("unanswered question captured", "question", question)
	return nil
}
