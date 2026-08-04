package worker

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/riverqueue/river"
)

type SortArgs struct {
	Strings []string `json:"strings"`
}

func (SortArgs) Kind() string { return "sort" }

type SortWorker struct {
	ChatService service.IChatService
	river.WorkerDefaults[SortArgs]
}

func NewSortWorker(chatService service.IChatService) *SortWorker {
	return &SortWorker{ChatService: chatService}
}

func (w *SortWorker) Work(ctx context.Context, job *river.Job[SortArgs]) error {
	_, err := w.ChatService.DoSort(ctx, job.Args.Strings)
	// TODO: retry policy
	// if err == validation | noretry
	// return river.NoRetryableError
	return err
}

// TODO: backoff policy
// specific job timeout
// retry policy
