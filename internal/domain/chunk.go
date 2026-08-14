package domain

import (
	"time"

	"github.com/google/uuid"
)

type Chunk struct {
	ID          uuid.UUID
	FileID      uuid.UUID
	Index       int
	Content     string
	RawText     string
	TokenCount  int
	HeadingPath []string
	ContentHash string
	Metadata    map[string]any
	Embedding   []float32 `json:"embedding,omitempty"` // pgvector, 1024-d, absent until Job 2 embeds
	CreatedAt   time.Time
}
