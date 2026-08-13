package worker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/worker/mocks"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessDocWorker_Work(test *testing.T) {
	test.Parallel()

	args := worker.ProcessDocArgs{FileID: "file-123", ObjectKey: "docs/readme.md"}
	source := []byte("# Title\n\nsome body")
	finalChunks := []rag.FinalChunk{
		{HeadingPath: []string{"# Title"}, Content: "# Title\n\nsome body", RawText: "some body", TokenCount: 5},
		{HeadingPath: []string{"# Title"}, Content: "# Title\n\nmore body", RawText: "more body", TokenCount: 6},
	}

	test.Run("success", func(t *testing.T) {
		storage := mocks.NewMockStorageClient(t)
		storage.EXPECT().GetObjectIntoBuffer(mock.Anything, args.ObjectKey).Return(source, nil)

		ragSvc := mocks.NewMockRagService(t)
		ragSvc.EXPECT().BuildChunks(mock.Anything, source).Return(finalChunks, nil)

		queue := mocks.NewMockJobQueue(t)
		for i, c := range finalChunks {
			queue.EXPECT().EnqueueGenerateChunks(mock.Anything, worker.GenerateChunksArgs{
				FileID:      args.FileID,
				Index:       i,
				HeadingPath: c.HeadingPath,
				Content:     c.Content,
				RawText:     c.RawText,
				TokenCount:  c.TokenCount,
			}).Return(nil)
		}

		w := worker.NewProcessDocWorker(worker.ProcessDocWorkerDep{
			RagService:    ragSvc,
			StorageClient: storage,
			JobQueue:      queue,
		})

		err := w.Work(context.Background(), &river.Job[worker.ProcessDocArgs]{Args: args})

		assert.NoError(t, err)
	})

	test.Run("error fetching from storage", func(t *testing.T) {
		storage := mocks.NewMockStorageClient(t)
		storage.EXPECT().GetObjectIntoBuffer(mock.Anything, args.ObjectKey).Return(nil, errors.New("storage_error"))

		ragSvc := mocks.NewMockRagService(t)
		queue := mocks.NewMockJobQueue(t)

		w := worker.NewProcessDocWorker(worker.ProcessDocWorkerDep{
			RagService:    ragSvc,
			StorageClient: storage,
			JobQueue:      queue,
		})

		err := w.Work(context.Background(), &river.Job[worker.ProcessDocArgs]{Args: args})

		assert.Error(t, err)
	})

	test.Run("error building chunks", func(t *testing.T) {
		storage := mocks.NewMockStorageClient(t)
		storage.EXPECT().GetObjectIntoBuffer(mock.Anything, args.ObjectKey).Return(source, nil)

		ragSvc := mocks.NewMockRagService(t)
		ragSvc.EXPECT().BuildChunks(mock.Anything, source).Return(nil, errors.New("build_error"))

		queue := mocks.NewMockJobQueue(t)

		w := worker.NewProcessDocWorker(worker.ProcessDocWorkerDep{
			RagService:    ragSvc,
			StorageClient: storage,
			JobQueue:      queue,
		})

		err := w.Work(context.Background(), &river.Job[worker.ProcessDocArgs]{Args: args})

		assert.Error(t, err)
	})

	test.Run("error enqueueing", func(t *testing.T) {
		storage := mocks.NewMockStorageClient(t)
		storage.EXPECT().GetObjectIntoBuffer(mock.Anything, args.ObjectKey).Return(source, nil)

		ragSvc := mocks.NewMockRagService(t)
		ragSvc.EXPECT().BuildChunks(mock.Anything, source).Return(finalChunks, nil)

		queue := mocks.NewMockJobQueue(t)
		queue.EXPECT().EnqueueGenerateChunks(mock.Anything, mock.Anything).Return(errors.New("enqueue_error"))

		w := worker.NewProcessDocWorker(worker.ProcessDocWorkerDep{
			RagService:    ragSvc,
			StorageClient: storage,
			JobQueue:      queue,
		})

		err := w.Work(context.Background(), &river.Job[worker.ProcessDocArgs]{Args: args})

		assert.Error(t, err)
	})
}
