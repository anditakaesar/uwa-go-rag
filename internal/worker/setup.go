package worker

import (
	"github.com/riverqueue/river"
)

type RegisterWorkerDep struct {
	ChatService   ChatService
	RagService    RagService
	Recorder      Recorder
	FileService   FileService
	StorageClient StorageClient
}

func RegisterWorkers(dep RegisterWorkerDep) (*river.Workers, error) {
	workers := river.NewWorkers()

	registrations := []func() error{
		func() error { return river.AddWorkerSafely(workers, NewSortWorker(dep.ChatService)) },
		func() error { return river.AddWorkerSafely(workers, NewProcessDocWorker(dep.RagService)) },
		func() error { return river.AddWorkerSafely(workers, NewInsertAuditLogWorker(dep.Recorder)) },
		func() error {
			return river.AddWorkerSafely(workers, NewThumbnailWorker(ThumbnailWorkerDep{
				FileService:   dep.FileService,
				StorageClient: dep.StorageClient,
			}))
		},
		func() error { return river.AddWorkerSafely(workers, NewDeleteFileWorker(dep.StorageClient)) },
	}

	for _, reg := range registrations {
		if err := reg(); err != nil {
			return nil, err
		}
	}

	return workers, nil
}
