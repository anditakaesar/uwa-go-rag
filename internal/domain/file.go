package domain

import (
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/dustin/go-humanize"
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

func (f *File) SizeHumanize() string {
	return humanize.Bytes(uint64(f.SizeBytes))
}

type FindAllFilesParam struct {
	Pagination common.Pagination `json:"pagination"`
}

func (p *FindAllFilesParam) Normalize() {
	p.Pagination.Normalize()
}

type GeneratePresignURLParam struct {
	Name      string
	SizeBytes int64
	MimeType  string
}

type GeneratePresignURLReturn struct {
	File
	PresignURL string
}

type UpdateFileParam struct {
	Status *UploadStatus
}
