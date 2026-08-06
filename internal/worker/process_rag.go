package worker

import (
	"context"

	"github.com/riverqueue/river"
)

type ProcessDocArgs struct {
	RagFileID int64 `json:"ragFileID"`
}

func (ProcessDocArgs) Kind() string { return "Process-RAG-File" }

type ProcessDocWorker struct {
	river.WorkerDefaults[ProcessDocArgs]
	RagService RagService
}

func NewProcessDocWorker(ragService RagService) *ProcessDocWorker {
	return &ProcessDocWorker{RagService: ragService}
}

func (w *ProcessDocWorker) Work(ctx context.Context, job *river.Job[ProcessDocArgs]) error {
	return w.RagService.ProcessDocument(ctx, job.Args.RagFileID)
}
