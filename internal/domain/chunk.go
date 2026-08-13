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
	CreatedAt   time.Time
}
