package worker

import (
	"context"

	"github.com/riverqueue/river"
)

// ProcessDocArgs is the Job 1 payload, triggered when a document is ready for
// processing. FileID references files.id; ObjectKey is the object storage key.
type ProcessDocArgs struct {
	FileID    string `json:"fileID"`
	ObjectKey string `json:"objectKey"`
}

func (ProcessDocArgs) Kind() string { return "Process-RAG-File" }

// ProcessDocWorker pulls the markdown payload from object storage, parses and
// sizes it into finalized chunks, then emits one GenerateChunksArgs event per
// chunk so storage is decoupled and can run in parallel.
type ProcessDocWorker struct {
	river.WorkerDefaults[ProcessDocArgs]
	ragService    RagService
	storageClient StorageClient
	jobQueue      JobQueue
}

type ProcessDocWorkerDep struct {
	RagService    RagService
	StorageClient StorageClient
	JobQueue      JobQueue
}

func NewProcessDocWorker(dep ProcessDocWorkerDep) *ProcessDocWorker {
	return &ProcessDocWorker{
		ragService:    dep.RagService,
		storageClient: dep.StorageClient,
		jobQueue:      dep.JobQueue,
	}
}

func (w *ProcessDocWorker) Work(ctx context.Context, job *river.Job[ProcessDocArgs]) error {
	source, err := w.storageClient.GetObjectIntoBuffer(ctx, job.Args.ObjectKey)
	if err != nil {
		return err
	}

	chunks, err := w.ragService.BuildChunks(ctx, source)
	if err != nil {
		return err
	}

	for i, chunk := range chunks {
		err = w.jobQueue.EnqueueGenerateChunks(ctx, GenerateChunksArgs{
			FileID:      job.Args.FileID,
			Index:       i,
			HeadingPath: chunk.HeadingPath,
			Content:     chunk.Content,
			RawText:     chunk.RawText,
			TokenCount:  chunk.TokenCount,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
