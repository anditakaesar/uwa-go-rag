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

	err := river.AddWorkerSafely(workers, NewSortWorker(dep.ChatService))
	if err != nil {
		return nil, err
	}

	err = river.AddWorkerSafely(workers, NewProcessDocWorker(dep.RagService))
	if err != nil {
		return nil, err
	}

	err = river.AddWorkerSafely(workers, NewInsertAuditLogWorker(dep.Recorder))
	if err != nil {
		return nil, err
	}

	err = river.AddWorkerSafely(workers, NewThumbnailWorker(ThumbnailWorkerDep{
		FileService:   dep.FileService,
		StorageClient: dep.StorageClient,
	}))
	if err != nil {
		return nil, err
	}

	return workers, nil
}
