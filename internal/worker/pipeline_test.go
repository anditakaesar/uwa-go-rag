package worker_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/tokenization"
	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/worker/mocks"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPipeline_EndToEnd(test *testing.T) {
	test.Parallel()

	ragSvc := rag.NewRagService(rag.ServiceDependency{
		Tokenizer: tokenization.NewSimpleTokenizer(),
	})
	ctx := context.Background()

	fileID := uuid.Must(uuid.NewV7())
	objectKey := "docs/pipeline.md"

	overview := strings.TrimSpace(strings.Repeat("overview content ", 150))
	deepDive := strings.TrimSpace(strings.Repeat("deep dive content ", 150))
	md := "# Overview\n" + overview + "\n\n## Deep Dive\n" + deepDive

	expected, err := ragSvc.BuildChunks(ctx, []byte(md))
	require.NoError(test, err)
	require.NotEmpty(test, expected)

	// Job 1: fetch + parse + emit.
	storage := mocks.NewMockStorageClient(test)
	storage.EXPECT().GetObjectIntoBuffer(mock.Anything, objectKey).Return([]byte(md), nil)

	var tasks []worker.GenerateChunksArgs
	queue := mocks.NewMockJobQueue(test)
	queue.EXPECT().EnqueueGenerateChunks(mock.Anything, mock.Anything).
		Run(func(_ context.Context, args worker.GenerateChunksArgs) {
			tasks = append(tasks, args)
		}).Return(nil)

	processWorker := worker.NewProcessDocWorker(worker.ProcessDocWorkerDep{
		RagService:    ragSvc,
		StorageClient: storage,
		JobQueue:      queue,
	})

	err = processWorker.Work(ctx, &river.Job[worker.ProcessDocArgs]{Args: worker.ProcessDocArgs{
		FileID:    fileID.String(),
		ObjectKey: objectKey,
	}})
	require.NoError(test, err)
	require.Len(test, tasks, len(expected))

	// Job 2: persist each emitted task.
	var stored []domain.Chunk
	repo := mocks.NewMockChunkRepository(test)
	repo.EXPECT().StoreBatch(mock.Anything, mock.Anything).
		Run(func(_ context.Context, chunks []domain.Chunk) {
			stored = append(stored, chunks...)
		}).Return(nil)

	genWorker := worker.NewChunkGeneratorWorker(repo)
	for _, task := range tasks {
		err = genWorker.Work(ctx, &river.Job[worker.GenerateChunksArgs]{Args: task})
		require.NoError(test, err)
	}

	// Verify the stored chunks match the sizing engine output.
	require.Len(test, stored, len(expected))
	for i, chunk := range stored {
		assert.Equal(test, expected[i].Content, chunk.Content)
		assert.Equal(test, expected[i].RawText, chunk.RawText)
		assert.Equal(test, expected[i].TokenCount, chunk.TokenCount)
		assert.Equal(test, expected[i].HeadingPath, chunk.HeadingPath)
		assert.Equal(test, i, chunk.Index)
		assert.Equal(test, fileID, chunk.FileID)

		sum := sha256.Sum256([]byte(chunk.Content))
		assert.Equal(test, hex.EncodeToString(sum[:]), chunk.ContentHash)
	}
}
