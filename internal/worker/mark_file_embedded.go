package worker

import (
	"context"
	"errors"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ErrNotAllChunksEmbedded signals the completion job that not every chunk of
// the file carries an embedding yet; returning it makes River retry the job
// so the flag is never flipped to 'completed' early.
var ErrNotAllChunksEmbedded = errors.New("not all chunks embedded yet")

// MarkFileEmbeddedArgs is the Job 3 payload carrying how many chunks the file
// is expected to have once every GenerateChunksArgs job has been persisted.
type MarkFileEmbeddedArgs struct {
	FileID         string `json:"fileID"`
	ExpectedChunks int    `json:"expectedChunks"`
}

func (MarkFileEmbeddedArgs) Kind() string { return "Mark-File-Embedded" }

// MarkFileEmbeddedWorker flips files.embedding_status to 'completed' only when
// every chunk of the file is stored with a non-null embedding; otherwise it
// returns ErrNotAllChunksEmbedded and River retries.
type MarkFileEmbeddedWorker struct {
	river.WorkerDefaults[MarkFileEmbeddedArgs]
	fileService     FileService
	chunkRepository ChunkRepository
}

func NewMarkFileEmbeddedWorker(fileService FileService, chunkRepository ChunkRepository) *MarkFileEmbeddedWorker {
	return &MarkFileEmbeddedWorker{
		fileService:     fileService,
		chunkRepository: chunkRepository,
	}
}

func (w *MarkFileEmbeddedWorker) Work(ctx context.Context, job *river.Job[MarkFileEmbeddedArgs]) error {
	fileID, err := uuid.Parse(job.Args.FileID)
	if err != nil {
		return err
	}

	count, err := w.chunkRepository.CountEmbeddedByFileID(ctx, fileID)
	if err != nil {
		return err
	}

	if count < job.Args.ExpectedChunks {
		return ErrNotAllChunksEmbedded
	}

	return w.fileService.SetEmbeddingStatus(ctx, fileID, domain.EMBEDDING_STATUS_COMPLETED)
}
