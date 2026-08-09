package worker

import (
	"context"

	"github.com/riverqueue/river"
)

type DeleteFileArgs struct {
	Key string `json:"key"`
}

func (DeleteFileArgs) Kind() string { return "Delete-File-From-Storage" }

type DeleteFileWorker struct {
	storageClient StorageClient
	river.WorkerDefaults[DeleteFileArgs]
}

func NewDeleteFileWorker(storageClient StorageClient) *DeleteFileWorker {
	return &DeleteFileWorker{
		storageClient: storageClient,
	}
}

func (w *DeleteFileWorker) Work(ctx context.Context, job *river.Job[DeleteFileArgs]) error {
	return w.storageClient.DeleteObject(ctx, job.Args.Key)
}
