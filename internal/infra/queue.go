package infra

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func NewRiverClient(db *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	riverClient, err := river.NewClient(riverpgxv5.New(db), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers: workers,
	})

	if err != nil {
		return nil, err
	}

	return riverClient, nil
}

// To Insert new Job queue
type RiverQueue struct {
	client *river.Client[pgx.Tx]
}

func NewRiverQueue() *RiverQueue {
	return &RiverQueue{}
}

func (r *RiverQueue) SetClient(client *river.Client[pgx.Tx]) {
	r.client = client
}

func (r *RiverQueue) EnqueueChat(ctx context.Context, words []string) error {
	_, err := r.client.Insert(ctx, worker.SortArgs{
		Strings: words,
	}, nil)
	return err
}
