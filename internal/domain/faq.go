package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type FAQStatus string

const (
	FAQStatusUnanswered FAQStatus = "unanswered"
	FAQStatusAnswered   FAQStatus = "answered"
	FAQStatusDismissed  FAQStatus = "dismissed"
)

// FAQS3Key is the derived object storage key for a FAQ's markdown file. It is
// reproducible from the FAQ at any time, so the FAQ's files row can never be
// orphaned or lost.
func FAQS3Key(faqID uuid.UUID) string {
	return fmt.Sprintf("faq/%s.md", faqID.String())
}

type FAQ struct {
	ID                uuid.UUID
	Question          string
	Answer            string
	Status            FAQStatus
	AskedBy           *int64
	AnsweredBy        *int64
	FileID            uuid.UUID
	AnswerContentHash string
	LastIndexedHash   string
	CreatedAt         time.Time
	AnsweredAt        *time.Time
	UpdatedAt         time.Time
}
