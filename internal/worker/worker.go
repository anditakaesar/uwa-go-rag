package worker

import (
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/riverqueue/river"
)

type RegisterWorkerDep struct {
	ChatService *service.ChatService
	RagService  *service.RagService
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

	return workers, nil
}
