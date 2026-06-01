package worker

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/riverqueue/river"
)

type ProccessDocArgs struct {
	RagFileID int64
}

func (ProccessDocArgs) Kind() string { return "Process-RAG-File" }

type ProcessDocWorker struct {
	river.WorkerDefaults[ProccessDocArgs]
	RagService service.IRagService
}

func NewProcessDocWorker(ragService service.IRagService) *ProcessDocWorker {
	return &ProcessDocWorker{RagService: ragService}
}

func (w *ProcessDocWorker) Work(ctx context.Context, job *river.Job[ProccessDocArgs]) error {
	return w.RagService.ProcessDocument(ctx, job.Args.RagFileID)
}
