package worker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/worker/mocks"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMarkFileEmbeddedWorker_Work(test *testing.T) {
	test.Parallel()

	fileID := uuid.Must(uuid.NewV7())
	args := worker.MarkFileEmbeddedArgs{
		FileID:         fileID.String(),
		ExpectedChunks: 3,
	}

	test.Run("marks completed when all chunks embedded", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)
		repo.EXPECT().CountEmbeddedByFileID(mock.Anything, fileID).Return(args.ExpectedChunks, nil)

		fileSvc := mocks.NewMockFileService(t)
		fileSvc.EXPECT().SetEmbeddingStatus(mock.Anything, fileID, domain.EMBEDDING_STATUS_COMPLETED).Return(nil)

		w := worker.NewMarkFileEmbeddedWorker(fileSvc, repo)
		err := w.Work(context.Background(), &river.Job[worker.MarkFileEmbeddedArgs]{Args: args})

		assert.NoError(t, err)
	})

	test.Run("retries when not all chunks embedded", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)
		repo.EXPECT().CountEmbeddedByFileID(mock.Anything, fileID).Return(args.ExpectedChunks-1, nil)

		fileSvc := mocks.NewMockFileService(t)
		fileSvc.AssertNotCalled(t, "SetEmbeddingStatus")

		w := worker.NewMarkFileEmbeddedWorker(fileSvc, repo)
		err := w.Work(context.Background(), &river.Job[worker.MarkFileEmbeddedArgs]{Args: args})

		assert.ErrorIs(t, err, worker.ErrNotAllChunksEmbedded)
	})

	test.Run("never completes when count exceeds expectation", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)
		repo.EXPECT().CountEmbeddedByFileID(mock.Anything, fileID).Return(args.ExpectedChunks, nil)

		fileSvc := mocks.NewMockFileService(t)
		fileSvc.EXPECT().SetEmbeddingStatus(mock.Anything, fileID, domain.EMBEDDING_STATUS_COMPLETED).Return(nil)

		w := worker.NewMarkFileEmbeddedWorker(fileSvc, repo)
		err := w.Work(context.Background(), &river.Job[worker.MarkFileEmbeddedArgs]{Args: args})

		assert.NoError(t, err)
	})

	test.Run("error counting embedded chunks", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)
		repo.EXPECT().CountEmbeddedByFileID(mock.Anything, fileID).Return(0, errors.New("count_error"))

		fileSvc := mocks.NewMockFileService(t)

		w := worker.NewMarkFileEmbeddedWorker(fileSvc, repo)
		err := w.Work(context.Background(), &river.Job[worker.MarkFileEmbeddedArgs]{Args: args})

		assert.Error(t, err)
	})

	test.Run("error setting completed status", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)
		repo.EXPECT().CountEmbeddedByFileID(mock.Anything, fileID).Return(args.ExpectedChunks, nil)

		fileSvc := mocks.NewMockFileService(t)
		fileSvc.EXPECT().SetEmbeddingStatus(mock.Anything, fileID, domain.EMBEDDING_STATUS_COMPLETED).Return(errors.New("update_error"))

		w := worker.NewMarkFileEmbeddedWorker(fileSvc, repo)
		err := w.Work(context.Background(), &river.Job[worker.MarkFileEmbeddedArgs]{Args: args})

		assert.Error(t, err)
	})

	test.Run("invalid file id", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)
		fileSvc := mocks.NewMockFileService(t)

		badArgs := args
		badArgs.FileID = "not-a-uuid"

		w := worker.NewMarkFileEmbeddedWorker(fileSvc, repo)
		err := w.Work(context.Background(), &river.Job[worker.MarkFileEmbeddedArgs]{Args: badArgs})

		assert.Error(t, err)
	})
}
