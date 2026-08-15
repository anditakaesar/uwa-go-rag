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

func TestFaqIndexWorker_Work(test *testing.T) {
	test.Parallel()

	faqID := uuid.Must(uuid.NewV7())
	fileID := uuid.Must(uuid.NewV7())
	answer := "Buka halaman Login, klik Lupa Password."
	contentHash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	source := []byte("# Bagaimana cara reset password?\n\n" + answer)

	args := worker.IndexFaqArgs{FAQID: faqID.String()}

	answeredFAQ := &domain.FAQ{
		ID:                faqID,
		Question:          "Bagaimana cara reset password?",
		Answer:            answer,
		Status:            domain.FAQStatusAnswered,
		FileID:            fileID,
		AnswerContentHash: contentHash,
	}

	newWorker := func(faqRepo worker.FaqRepository, storage worker.StorageClient, chunkRepo worker.ChunkRepository, queue worker.JobQueue, fileSvc worker.FileService) *worker.FaqIndexWorker {
		return worker.NewFaqIndexWorker(worker.FaqIndexWorkerDep{
			FaqRepository:   faqRepo,
			StorageClient:   storage,
			ChunkRepository: chunkRepo,
			JobQueue:        queue,
			FileService:     fileSvc,
		})
	}

	test.Run("success - deletes stale chunks, uploads, enqueues, sets hash", func(t *testing.T) {
		faqRepo := mocks.NewMockFaqRepository(t)
		faqRepo.EXPECT().Get(mock.Anything, faqID).Return(answeredFAQ, nil)
		faqRepo.EXPECT().SetLastIndexedHash(mock.Anything, faqID, contentHash).Return(nil)

		chunkRepo := mocks.NewMockChunkRepository(t)
		chunkRepo.EXPECT().DeleteByFileID(mock.Anything, fileID).Return(nil)

		storage := mocks.NewMockStorageClient(t)
		storage.EXPECT().UploadObject(mock.Anything, domain.FAQS3Key(faqID), "text/markdown", source).Return(nil)

		fileSvc := mocks.NewMockFileService(t)
		fileSvc.EXPECT().SetStatus(mock.Anything, fileID, domain.UPLOAD_STATUS_COMPLETED).Return(nil)

		queue := mocks.NewMockJobQueue(t)
		queue.EXPECT().EnqueueRagFile(mock.Anything, fileID, domain.FAQS3Key(faqID)).Return(nil)

		w := newWorker(faqRepo, storage, chunkRepo, queue, fileSvc)

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: args})

		assert.NoError(t, err)
	})

	test.Run("unanswered FAQ - skipped", func(t *testing.T) {
		faqRepo := mocks.NewMockFaqRepository(t)
		unanswered := *answeredFAQ
		unanswered.Status = domain.FAQStatusUnanswered
		faqRepo.EXPECT().Get(mock.Anything, faqID).Return(&unanswered, nil)

		w := newWorker(faqRepo, mocks.NewMockStorageClient(t), mocks.NewMockChunkRepository(t), mocks.NewMockJobQueue(t), mocks.NewMockFileService(t))

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: args})

		assert.NoError(t, err)
	})

	test.Run("already indexed - idempotent skip", func(t *testing.T) {
		faqRepo := mocks.NewMockFaqRepository(t)
		indexed := *answeredFAQ
		indexed.LastIndexedHash = contentHash
		faqRepo.EXPECT().Get(mock.Anything, faqID).Return(&indexed, nil)

		w := newWorker(faqRepo, mocks.NewMockStorageClient(t), mocks.NewMockChunkRepository(t), mocks.NewMockJobQueue(t), mocks.NewMockFileService(t))

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: args})

		assert.NoError(t, err)
	})

	test.Run("invalid faqID - error", func(t *testing.T) {
		w := newWorker(mocks.NewMockFaqRepository(t), mocks.NewMockStorageClient(t), mocks.NewMockChunkRepository(t), mocks.NewMockJobQueue(t), mocks.NewMockFileService(t))

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: worker.IndexFaqArgs{FAQID: "not-a-uuid"}})

		assert.Error(t, err)
	})

	test.Run("faq not found - propagates", func(t *testing.T) {
		faqRepo := mocks.NewMockFaqRepository(t)
		faqRepo.EXPECT().Get(mock.Anything, faqID).Return(nil, errors.New("faq not found"))

		w := newWorker(faqRepo, mocks.NewMockStorageClient(t), mocks.NewMockChunkRepository(t), mocks.NewMockJobQueue(t), mocks.NewMockFileService(t))

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: args})

		assert.ErrorContains(t, err, "faq not found")
	})

	test.Run("delete chunks failure - propagates", func(t *testing.T) {
		faqRepo := mocks.NewMockFaqRepository(t)
		faqRepo.EXPECT().Get(mock.Anything, faqID).Return(answeredFAQ, nil)

		chunkRepo := mocks.NewMockChunkRepository(t)
		chunkRepo.EXPECT().DeleteByFileID(mock.Anything, fileID).Return(errors.New("delete_error"))

		w := newWorker(faqRepo, mocks.NewMockStorageClient(t), chunkRepo, mocks.NewMockJobQueue(t), mocks.NewMockFileService(t))

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: args})

		assert.ErrorContains(t, err, "delete_error")
	})

	test.Run("upload failure - propagates", func(t *testing.T) {
		faqRepo := mocks.NewMockFaqRepository(t)
		faqRepo.EXPECT().Get(mock.Anything, faqID).Return(answeredFAQ, nil)

		chunkRepo := mocks.NewMockChunkRepository(t)
		chunkRepo.EXPECT().DeleteByFileID(mock.Anything, fileID).Return(nil)

		storage := mocks.NewMockStorageClient(t)
		storage.EXPECT().UploadObject(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("upload_error"))

		w := newWorker(faqRepo, storage, chunkRepo, mocks.NewMockJobQueue(t), mocks.NewMockFileService(t))

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: args})

		assert.ErrorContains(t, err, "upload_error")
	})

	test.Run("enqueue failure - propagates", func(t *testing.T) {
		faqRepo := mocks.NewMockFaqRepository(t)
		faqRepo.EXPECT().Get(mock.Anything, faqID).Return(answeredFAQ, nil)

		chunkRepo := mocks.NewMockChunkRepository(t)
		chunkRepo.EXPECT().DeleteByFileID(mock.Anything, fileID).Return(nil)

		storage := mocks.NewMockStorageClient(t)
		storage.EXPECT().UploadObject(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		fileSvc := mocks.NewMockFileService(t)
		fileSvc.EXPECT().SetStatus(mock.Anything, fileID, domain.UPLOAD_STATUS_COMPLETED).Return(nil)

		queue := mocks.NewMockJobQueue(t)
		queue.EXPECT().EnqueueRagFile(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("enqueue_error"))

		w := newWorker(faqRepo, storage, chunkRepo, queue, fileSvc)

		err := w.Work(context.Background(), &river.Job[worker.IndexFaqArgs]{Args: args})

		assert.ErrorContains(t, err, "enqueue_error")
	})
}
