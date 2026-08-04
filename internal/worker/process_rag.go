package worker

import (
	"context"

	"github.com/riverqueue/river"
)

// adapters
type IRagService interface {
	ProcessDocument(ctx context.Context, ragFileID int64) error
}

type ProcessDocArgs struct {
	RagFileID int64 `json:"ragFileID"`
}

func (ProcessDocArgs) Kind() string { return "Process-RAG-File" }

type ProcessDocWorker struct {
	river.WorkerDefaults[ProcessDocArgs]
	RagService IRagService
}

func NewProcessDocWorker(ragService IRagService) *ProcessDocWorker {
	return &ProcessDocWorker{RagService: ragService}
}

func (w *ProcessDocWorker) Work(ctx context.Context, job *river.Job[ProcessDocArgs]) error {
	return w.RagService.ProcessDocument(ctx, job.Args.RagFileID)
}
