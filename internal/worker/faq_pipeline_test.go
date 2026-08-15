package worker_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestFaqPipeline_RoundTrip(test *testing.T) {
	test.Parallel()

	ctx := context.Background()
	faqID := uuid.Must(uuid.NewV7())
	fileID := uuid.Must(uuid.NewV7())
	question := "Bagaimana cara reset password?"
	answer := "Buka halaman Login, klik Lupa Password."
	contentHash := sha256hex(answer)
	source := []byte("# " + question + "\n\n" + answer)
	objectKey := domain.FAQS3Key(faqID)

	ragSvc := rag.NewRagService(rag.ServiceDependency{
		Tokenizer: tokenization.NewSimpleTokenizer(),
	})
	expected, err := ragSvc.BuildChunks(ctx, source)
	require.NoError(test, err)
	require.NotEmpty(test, expected)

	faq := &domain.FAQ{
		ID:                faqID,
		Question:          question,
		Answer:            answer,
		Status:            domain.FAQStatusAnswered,
		FileID:            fileID,
		AnswerContentHash: contentHash,
	}

	faqRepo := mocks.NewMockFaqRepository(test)
	faqRepo.EXPECT().Get(mock.Anything, faqID).Return(faq, nil).Once()
	faqRepo.EXPECT().SetLastIndexedHash(mock.Anything, faqID, contentHash).Return(nil).Once()

	storage := mocks.NewMockStorageClient(test)
	storage.EXPECT().UploadObject(mock.Anything, objectKey, "text/markdown", source).Return(nil).Once()
	storage.EXPECT().GetObjectIntoBuffer(mock.Anything, objectKey).Return(source, nil).Once()

	chunkRepo := mocks.NewMockChunkRepository(test)
	chunkRepo.EXPECT().DeleteByFileID(mock.Anything, fileID).Return(nil).Once()
	chunkRepo.EXPECT().CountEmbeddedByFileID(mock.Anything, fileID).Return(len(expected), nil).Once()

	fileSvc := mocks.NewMockFileService(test)
	fileSvc.EXPECT().SetStatus(mock.Anything, fileID, domain.UPLOAD_STATUS_COMPLETED).Return(nil).Once()
	fileSvc.EXPECT().SetEmbeddingStatus(mock.Anything, fileID, domain.EMBEDDING_STATUS_PROCESSING).Return(nil).Once()
	fileSvc.EXPECT().SetEmbeddingStatus(mock.Anything, fileID, domain.EMBEDDING_STATUS_COMPLETED).Return(nil).Once()

	var tasks []worker.GenerateChunksArgs
	queue := mocks.NewMockJobQueue(test)
	queue.EXPECT().EnqueueRagFile(mock.Anything, fileID, objectKey).Return(nil).Once()
	queue.EXPECT().EnqueueGenerateChunks(mock.Anything, mock.Anything).
		Run(func(_ context.Context, args worker.GenerateChunksArgs) {
			tasks = append(tasks, args)
		}).Return(nil).Times(len(expected))
	queue.EXPECT().EnqueueMarkFileEmbedded(mock.Anything, worker.MarkFileEmbeddedArgs{
		FileID:         fileID.String(),
		ExpectedChunks: len(expected),
	}).Return(nil).Once()

	embedder := mocks.NewMockEmbedder(test)
	embedder.EXPECT().Embed(mock.Anything, mock.Anything).Return([]float32{0.1, 0.2, 0.3}, nil).Times(len(expected))

	var stored []domain.Chunk
	chunkRepo.EXPECT().StoreBatch(mock.Anything, mock.Anything).
		Run(func(_ context.Context, chunks []domain.Chunk) {
			stored = append(stored, chunks...)
		}).Return(nil).Times(len(expected))

	// Index-FAQ: synthesize + hand off.
	indexWorker := worker.NewFaqIndexWorker(worker.FaqIndexWorkerDep{
		FaqRepository:   faqRepo,
		StorageClient:   storage,
		ChunkRepository: chunkRepo,
		JobQueue:        queue,
		FileService:     fileSvc,
	})
	err = indexWorker.Work(ctx, &river.Job[worker.IndexFaqArgs]{Args: worker.IndexFaqArgs{FAQID: faqID.String()}})
	require.NoError(test, err)

	// Process-RAG-File: fetch + parse + emit.
	processWorker := worker.NewProcessDocWorker(worker.ProcessDocWorkerDep{
		RagService:    ragSvc,
		StorageClient: storage,
		JobQueue:      queue,
		FileService:   fileSvc,
	})
	err = processWorker.Work(ctx, &river.Job[worker.ProcessDocArgs]{Args: worker.ProcessDocArgs{
		FileID:    fileID.String(),
		ObjectKey: objectKey,
	}})
	require.NoError(test, err)
	require.Len(test, tasks, len(expected))

	// Generate-Chunks: embed + persist each task.
	genWorker := worker.NewChunkGeneratorWorker(chunkRepo, embedder)
	for _, task := range tasks {
		err = genWorker.Work(ctx, &river.Job[worker.GenerateChunksArgs]{Args: task})
		require.NoError(test, err)
	}

	// Mark-File-Embedded: flip flag when all chunks carry vectors.
	markWorker := worker.NewMarkFileEmbeddedWorker(fileSvc, chunkRepo)
	err = markWorker.Work(ctx, &river.Job[worker.MarkFileEmbeddedArgs]{Args: worker.MarkFileEmbeddedArgs{
		FileID:         fileID.String(),
		ExpectedChunks: len(expected),
	}})
	require.NoError(test, err)

	// The FAQ chunk is an ordinary chunk: heading path is the question, the
	// content carries the curated answer, and the vector is set.
	require.Len(test, stored, len(expected))
	for i, chunk := range stored {
		assert.Equal(test, fileID, chunk.FileID)
		assert.Equal(test, expected[i].HeadingPath, chunk.HeadingPath)
		assert.Equal(test, expected[i].Content, chunk.Content)
		assert.Equal(test, []string{"# " + question}, chunk.HeadingPath)
		assert.Contains(test, chunk.Content, answer)
		assert.Equal(test, []float32{0.1, 0.2, 0.3}, chunk.Embedding)
		assert.Equal(test, sha256hex(chunk.Content), chunk.ContentHash)
	}
}

func TestFaqIndexWorker_ReindexReplacesOldChunks(test *testing.T) {
	test.Parallel()

	ctx := context.Background()
	faqID := uuid.Must(uuid.NewV7())
	fileID := uuid.Must(uuid.NewV7())
	question := "Bagaimana cara reset password?"

	answerV1 := "Buka halaman Login, klik Lupa Password."
	answerV2 := "Buka halaman Login, klik Lupa Password, lalu ikuti email reset."

	sourceV1 := []byte("# " + question + "\n\n" + answerV1)
	sourceV2 := []byte("# " + question + "\n\n" + answerV2)
	objectKey := domain.FAQS3Key(faqID)

	faqV1 := &domain.FAQ{
		ID:                faqID,
		Question:          question,
		Answer:            answerV1,
		Status:            domain.FAQStatusAnswered,
		FileID:            fileID,
		AnswerContentHash: sha256hex(answerV1),
	}
	faqV2 := &domain.FAQ{
		ID:                faqID,
		Question:          question,
		Answer:            answerV2,
		Status:            domain.FAQStatusAnswered,
		FileID:            fileID,
		AnswerContentHash: sha256hex(answerV2),
	}

	faqRepo := mocks.NewMockFaqRepository(test)
	faqRepo.EXPECT().Get(mock.Anything, faqID).Return(faqV1, nil).Once()
	faqRepo.EXPECT().SetLastIndexedHash(mock.Anything, faqID, faqV1.AnswerContentHash).Return(nil).Once()
	faqRepo.EXPECT().Get(mock.Anything, faqID).Return(faqV2, nil).Once()
	faqRepo.EXPECT().SetLastIndexedHash(mock.Anything, faqID, faqV2.AnswerContentHash).Return(nil).Once()

	// Cycle 3: the FAQ now carries the same content hash it was indexed with.
	indexedV2 := *faqV2
	indexedV2.LastIndexedHash = faqV2.AnswerContentHash
	faqRepo.EXPECT().Get(mock.Anything, faqID).Return(&indexedV2, nil).Once()

	chunkRepo := mocks.NewMockChunkRepository(test)
	chunkRepo.EXPECT().DeleteByFileID(mock.Anything, fileID).Return(nil).Twice()

	storage := mocks.NewMockStorageClient(test)
	storage.EXPECT().UploadObject(mock.Anything, objectKey, "text/markdown", sourceV1).Return(nil).Once()
	storage.EXPECT().UploadObject(mock.Anything, objectKey, "text/markdown", sourceV2).Return(nil).Once()

	fileSvc := mocks.NewMockFileService(test)
	fileSvc.EXPECT().SetStatus(mock.Anything, fileID, domain.UPLOAD_STATUS_COMPLETED).Return(nil).Twice()

	queue := mocks.NewMockJobQueue(test)
	queue.EXPECT().EnqueueRagFile(mock.Anything, fileID, objectKey).Return(nil).Twice()

	w := worker.NewFaqIndexWorker(worker.FaqIndexWorkerDep{
		FaqRepository:   faqRepo,
		StorageClient:   storage,
		ChunkRepository: chunkRepo,
		JobQueue:        queue,
		FileService:     fileSvc,
	})

	// Cycle 1: index the first answer.
	err := w.Work(ctx, &river.Job[worker.IndexFaqArgs]{Args: worker.IndexFaqArgs{FAQID: faqID.String()}})
	require.NoError(test, err)

	// Cycle 2: the answer is edited (new content hash) - stale chunks are
	// deleted before regeneration, so old and new chunks never coexist.
	err = w.Work(ctx, &river.Job[worker.IndexFaqArgs]{Args: worker.IndexFaqArgs{FAQID: faqID.String()}})
	require.NoError(test, err)

	// Cycle 3: unchanged answer - idempotent no-op, nothing deleted/uploaded.
	err = w.Work(ctx, &river.Job[worker.IndexFaqArgs]{Args: worker.IndexFaqArgs{FAQID: faqID.String()}})
	require.NoError(test, err)
}
