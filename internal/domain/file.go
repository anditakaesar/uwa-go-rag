package domain

import (
	"time"

	"github.com/google/uuid"
)

type UploadStatus string

const (
	UPLOAD_STATUS_PENDING   UploadStatus = "pending"
	UPLOAD_STATUS_COMPLETED UploadStatus = "completed"
	UPLOAD_STATUS_FAILED    UploadStatus = "failed"
)

type File struct {
	ID           uuid.UUID
	UserID       int64
	OriginalName string
	MimeType     string
	SizeBytes    int64
	S3Bucket     string
	S3Key        string
	Status       UploadStatus
	Metadata     any // jsonb
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
