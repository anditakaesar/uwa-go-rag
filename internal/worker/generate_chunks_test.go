package worker_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/worker/mocks"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestChunkGeneratorWorker_Work(test *testing.T) {
	test.Parallel()

	fileID := uuid.Must(uuid.NewV7())
	args := worker.GenerateChunksArgs{
		FileID:      fileID.String(),
		Index:       2,
		HeadingPath: []string{"# Title"},
		Content:     "# Title\n\nbody content",
		RawText:     "body content",
		TokenCount:  7,
	}

	test.Run("success", func(t *testing.T) {
		var stored []domain.Chunk
		repo := mocks.NewMockChunkRepository(t)
		repo.EXPECT().StoreBatch(mock.Anything, mock.Anything).
			Run(func(_ context.Context, chunks []domain.Chunk) {
				stored = append(stored, chunks...)
			}).Return(nil)

		w := worker.NewChunkGeneratorWorker(repo)
		err := w.Work(context.Background(), &river.Job[worker.GenerateChunksArgs]{Args: args})

		assert.NoError(t, err)
		require.Len(t, stored, 1)

		chunk := stored[0]
		assert.Equal(t, fileID, chunk.FileID)
		assert.Equal(t, args.Index, chunk.Index)
		assert.Equal(t, args.HeadingPath, chunk.HeadingPath)
		assert.Equal(t, args.Content, chunk.Content)
		assert.Equal(t, args.RawText, chunk.RawText)
		assert.Equal(t, args.TokenCount, chunk.TokenCount)
		assert.NotEmpty(t, chunk.ID)

		sum := sha256.Sum256([]byte(args.Content))
		assert.Equal(t, hex.EncodeToString(sum[:]), chunk.ContentHash)
	})

	test.Run("error storing", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)
		repo.EXPECT().StoreBatch(mock.Anything, mock.Anything).Return(errors.New("store_error"))

		w := worker.NewChunkGeneratorWorker(repo)
		err := w.Work(context.Background(), &river.Job[worker.GenerateChunksArgs]{Args: args})

		assert.Error(t, err)
	})

	test.Run("invalid file id", func(t *testing.T) {
		repo := mocks.NewMockChunkRepository(t)

		badArgs := args
		badArgs.FileID = "not-a-uuid"

		w := worker.NewChunkGeneratorWorker(repo)
		err := w.Work(context.Background(), &river.Job[worker.GenerateChunksArgs]{Args: badArgs})

		assert.Error(t, err)
	})
}
