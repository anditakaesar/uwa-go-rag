package faq_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/application/mocks/custom"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/faq"
	"github.com/anditakaesar/uwa-go-rag/internal/faq/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFaqService_RecordUnanswered(test *testing.T) {
	test.Parallel()

	ctx := context.Background()
	question := "Bagaimana cara reset password?"
	fileID := uuid.Must(uuid.NewV7())

	test.Run("success - creates file row and faq row in one transaction", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockUow := new(custom.IMockUow)

		mockUow.On("Do", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("CreateFile", ctx, mock.Anything).Return(fileID, nil).Once()
		mockRepo.On("CreateUnanswered", ctx, mock.MatchedBy(func(f domain.FAQ) bool {
			return f.ID != uuid.Nil && f.FileID == fileID && f.Question == question
		})).Return(nil).Once()

		svc := faq.NewService(faq.ServiceDependency{
			Repo: mockRepo,
			UOW:  mockUow,
		})
		err := svc.RecordUnanswered(ctx, question)

		assert.NoError(t, err)
		mockUow.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	test.Run("duplicate question - silent no-op", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockUow := new(custom.IMockUow)

		mockUow.On("Do", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("CreateFile", ctx, mock.Anything).Return(fileID, nil).Once()
		mockRepo.On("CreateUnanswered", ctx, mock.MatchedBy(func(f domain.FAQ) bool {
			return f.Question == question
		})).Return(faq.ErrAlreadyCaptured).Once()

		svc := faq.NewService(faq.ServiceDependency{
			Repo: mockRepo,
			UOW:  mockUow,
		})
		err := svc.RecordUnanswered(ctx, question)

		assert.NoError(t, err)
		mockUow.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	test.Run("create file failure - propagates", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockUow := new(custom.IMockUow)

		mockUow.On("Do", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("CreateFile", ctx, mock.Anything).Return(uuid.Nil, errors.New("insert_error")).Once()

		svc := faq.NewService(faq.ServiceDependency{
			Repo: mockRepo,
			UOW:  mockUow,
		})
		err := svc.RecordUnanswered(ctx, question)

		assert.ErrorContains(t, err, "insert_error")
		mockUow.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	test.Run("create unanswered failure - propagates", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockUow := new(custom.IMockUow)

		mockUow.On("Do", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("CreateFile", ctx, mock.Anything).Return(fileID, nil).Once()
		mockRepo.On("CreateUnanswered", ctx, mock.MatchedBy(func(f domain.FAQ) bool {
			return f.Question == question
		})).Return(errors.New("insert_error")).Once()

		svc := faq.NewService(faq.ServiceDependency{
			Repo: mockRepo,
			UOW:  mockUow,
		})
		err := svc.RecordUnanswered(ctx, question)

		assert.ErrorContains(t, err, "insert_error")
		mockUow.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})
}

func TestFaqService_List(test *testing.T) {
	test.Parallel()

	ctx := context.Background()

	test.Run("success - unanswered", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)

		faqs := []domain.FAQ{
			{ID: uuid.Must(uuid.NewV7()), Question: "Bagaimana cara reset password?", Status: domain.FAQStatusUnanswered},
		}
		mockRepo.On("ListByStatus", ctx, domain.FAQStatusUnanswered, 20, 0).Return(faqs, nil).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo})
		got, err := svc.List(ctx, domain.FAQStatusUnanswered, 20, 0)

		assert.NoError(t, err)
		assert.Len(t, got, 1)
		mockRepo.AssertExpectations(t)
	})

	test.Run("success - answered status passed through", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)

		faqs := []domain.FAQ{
			{ID: uuid.Must(uuid.NewV7()), Question: "q?", Status: domain.FAQStatusAnswered},
		}
		mockRepo.On("ListByStatus", ctx, domain.FAQStatusAnswered, 20, 0).Return(faqs, nil).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo})
		got, err := svc.List(ctx, domain.FAQStatusAnswered, 20, 0)

		assert.NoError(t, err)
		assert.Len(t, got, 1)
		mockRepo.AssertExpectations(t)
	})

	test.Run("repository failure - propagates", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)

		mockRepo.On("ListByStatus", ctx, domain.FAQStatusUnanswered, 20, 0).Return([]domain.FAQ{}, errors.New("query_error")).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo})
		got, err := svc.List(ctx, domain.FAQStatusUnanswered, 20, 0)

		assert.ErrorContains(t, err, "query_error")
		assert.Empty(t, got)
		mockRepo.AssertExpectations(t)
	})
}

func TestFaqService_Answer(test *testing.T) {
	test.Parallel()

	ctx := context.Background()
	faqID := uuid.Must(uuid.NewV7())
	answeredAt := time.Now().UTC()
	answeredByID := int64(42)

	test.Run("success - persists answer and enqueues Index-FAQ", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockQueue := new(mocks.MockJobQueue)

		answered := &domain.FAQ{
			ID:         faqID,
			Question:   "Bagaimana cara reset password?",
			Answer:     "Buka halaman Login, klik Lupa Password.",
			Status:     domain.FAQStatusAnswered,
			AnsweredBy: &answeredByID,
			AnsweredAt: &answeredAt,
		}
		mockRepo.On("Answer", ctx, faqID, answered.Answer, int64(42), mock.AnythingOfType("time.Time")).Return(answered, nil).Once()
		mockQueue.On("EnqueueIndexFaq", ctx, worker.IndexFaqArgs{FAQID: faqID.String()}).Return(nil).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo, JobQueue: mockQueue})
		got, err := svc.Answer(ctx, faqID, answered.Answer, 42)

		assert.NoError(t, err)
		assert.Equal(t, domain.FAQStatusAnswered, got.Status)
		mockRepo.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	test.Run("enqueue failure - propagates", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockQueue := new(mocks.MockJobQueue)

		mockRepo.On("Answer", ctx, faqID, "answer", int64(42), mock.AnythingOfType("time.Time")).Return(&domain.FAQ{ID: faqID}, nil).Once()
		mockQueue.On("EnqueueIndexFaq", ctx, worker.IndexFaqArgs{FAQID: faqID.String()}).Return(errors.New("enqueue_error")).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo, JobQueue: mockQueue})
		got, err := svc.Answer(ctx, faqID, "answer", 42)

		assert.ErrorContains(t, err, "enqueue_error")
		assert.Nil(t, got)
		mockRepo.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	test.Run("empty answer - validation error", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockQueue := new(mocks.MockJobQueue)

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo, JobQueue: mockQueue})
		got, err := svc.Answer(ctx, faqID, "   ", 42)

		assert.ErrorContains(t, err, "answer is required")
		assert.Nil(t, got)
		mockRepo.AssertNotCalled(t, "Answer", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		mockQueue.AssertNotCalled(t, "EnqueueIndexFaq", mock.Anything, mock.Anything)
	})

	test.Run("repository failure - propagates", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockQueue := new(mocks.MockJobQueue)

		mockRepo.On("Answer", ctx, faqID, "answer", int64(42), mock.AnythingOfType("time.Time")).Return(nil, errors.New("update_error")).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo, JobQueue: mockQueue})
		got, err := svc.Answer(ctx, faqID, "answer", 42)

		assert.ErrorContains(t, err, "update_error")
		assert.Nil(t, got)
		mockRepo.AssertExpectations(t)
	})
}

func TestFaqService_Delete(test *testing.T) {
	test.Parallel()

	ctx := context.Background()
	faqID := uuid.Must(uuid.NewV7())

	test.Run("success - deletes rows in tx and enqueues S3 cleanup", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockUow := new(custom.IMockUow)
		mockQueue := new(mocks.MockJobQueue)

		mockUow.On("Do", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("Delete", ctx, faqID).Return(nil).Once()
		mockQueue.On("EnqueueDeleteFile", ctx, domain.FAQS3Key(faqID)).Return(nil).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo, UOW: mockUow, JobQueue: mockQueue})
		err := svc.Delete(ctx, faqID)

		assert.NoError(t, err)
		mockUow.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})

	test.Run("repo not found - propagates", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockUow := new(custom.IMockUow)

		mockUow.On("Do", ctx, mock.Anything).Return(errors.New("faq not found")).Once()
		mockRepo.On("Delete", ctx, faqID).Return(errors.New("faq not found")).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo, UOW: mockUow})
		err := svc.Delete(ctx, faqID)

		assert.ErrorContains(t, err, "faq not found")
		mockRepo.AssertExpectations(t)
	})

	test.Run("enqueue failure - propagates", func(t *testing.T) {
		mockRepo := new(mocks.MockRepository)
		mockUow := new(custom.IMockUow)
		mockQueue := new(mocks.MockJobQueue)

		mockUow.On("Do", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("Delete", ctx, faqID).Return(nil).Once()
		mockQueue.On("EnqueueDeleteFile", ctx, domain.FAQS3Key(faqID)).Return(errors.New("enqueue_error")).Once()

		svc := faq.NewService(faq.ServiceDependency{Repo: mockRepo, UOW: mockUow, JobQueue: mockQueue})
		err := svc.Delete(ctx, faqID)

		assert.ErrorContains(t, err, "enqueue_error")
		mockUow.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockQueue.AssertExpectations(t)
	})
}
