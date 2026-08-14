package worker

import (
	"github.com/riverqueue/river"
)

type RegisterWorkerDep struct {
	RagService      RagService
	ChunkRepository ChunkRepository
	Embedder        Embedder
	Recorder        Recorder
	FileService     FileService
	StorageClient   StorageClient
	JobQueue        JobQueue
}

func RegisterWorkers(dep RegisterWorkerDep) (*river.Workers, error) {
	workers := river.NewWorkers()

	registrations := []func() error{
		func() error {
			return river.AddWorkerSafely(workers, NewProcessDocWorker(ProcessDocWorkerDep{
				RagService:    dep.RagService,
				StorageClient: dep.StorageClient,
				JobQueue:      dep.JobQueue,
				FileService:   dep.FileService,
			}))
		},
		func() error {
			return river.AddWorkerSafely(workers, NewChunkGeneratorWorker(dep.ChunkRepository, dep.Embedder))
		},
		func() error {
			return river.AddWorkerSafely(workers, NewMarkFileEmbeddedWorker(dep.FileService, dep.ChunkRepository))
		},
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
