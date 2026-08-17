package faq

import (
	"context"
	"errors"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
)

// ErrAlreadyCaptured is returned when an unanswered FAQ for the same
// normalized question already exists; callers treat it as a silent no-op.
var ErrAlreadyCaptured = errors.New("faq already captured")

type Repository interface {
	CreateFile(ctx context.Context, faqID uuid.UUID) (uuid.UUID, error)
	CreateUnanswered(ctx context.Context, newFAQ domain.FAQ) error
	Fetch(ctx context.Context, param *domain.FetchFAQParam) ([]domain.FAQ, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.FAQ, error)
	Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64, now time.Time) (*domain.FAQ, error)
	SetLastIndexedHash(ctx context.Context, id uuid.UUID, hash string) error
	// Delete removes the faqs row and the FAQ's derived files row; the chunks
	// rows cascade with the files row. Returns ErrorResourceNotFound when the
	// FAQ does not exist.
	Delete(ctx context.Context, id uuid.UUID) error
}
