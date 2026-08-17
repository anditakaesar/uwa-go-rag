package faq

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/application"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/worker"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/google/uuid"
)

type JobQueue interface {
	EnqueueIndexFaq(ctx context.Context, args worker.IndexFaqArgs) error
	EnqueueDeleteFile(ctx context.Context, key string) error
}

type ServiceDependency struct {
	Repo     Repository
	UOW      application.UnitOfWork
	JobQueue JobQueue
}

type Service struct {
	repo  Repository
	uow   application.UnitOfWork
	queue JobQueue
}

func NewService(dep ServiceDependency) *Service {
	return &Service{
		repo:  dep.Repo,
		uow:   dep.UOW,
		queue: dep.JobQueue,
	}
}

// RecordUnanswered creates the FAQ's files row and the faqs row in one
// transaction. Duplicate unanswered questions are a silent no-op.
func (s *Service) RecordUnanswered(ctx context.Context, question string) error {
	err := s.uow.Do(ctx, func(txCtx context.Context) error {
		faqID, err := uuid.NewV7()
		if err != nil {
			return err
		}

		fileID, err := s.repo.CreateFile(txCtx, faqID)
		if err != nil {
			return err
		}

		return s.repo.CreateUnanswered(txCtx, domain.FAQ{
			ID:       faqID,
			FileID:   fileID,
			Question: question,
		})
	})
	if errors.Is(err, ErrAlreadyCaptured) {
		return nil
	}

	return err
}

// List returns FAQs in the given status for internal curation.
func (s *Service) List(ctx context.Context, param *domain.FetchFAQParam) ([]domain.FAQ, error) {
	return s.repo.Fetch(ctx, param)
}

// Answer validates and persists the canonical answer, flipping the FAQ to
// answered, then enqueues Index-FAQ so the answer is synthesized, chunked and
// embedded. The enqueue happens after the repo call returns, so the worker
// never sees a half-written answer.
func (s *Service) Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64) (*domain.FAQ, error) {
	if strings.TrimSpace(answer) == "" {
		return nil, &xerror.ErrorValidation{Message: "answer is required"}
	}

	faq, err := s.repo.Answer(ctx, id, answer, answeredBy, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if err := s.queue.EnqueueIndexFaq(ctx, worker.IndexFaqArgs{FAQID: faq.ID.String()}); err != nil {
		return nil, err
	}

	return faq, nil
}

// Delete hard-deletes the FAQ and its derived state: the faqs row and the
// files row (chunks cascade) are removed in one transaction, then the S3
// markdown object is deleted asynchronously after commit.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.uow.Do(ctx, func(txCtx context.Context) error {
		return s.repo.Delete(txCtx, id)
	})
	if err != nil {
		return err
	}

	return s.queue.EnqueueDeleteFile(ctx, domain.FAQS3Key(id))
}
