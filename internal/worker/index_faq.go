package worker

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// IndexFaqArgs triggers re-synthesis of an answered FAQ into retrievable
// chunks. FAQID references faqs.id.
type IndexFaqArgs struct {
	FAQID string `json:"faqID"`
}

func (IndexFaqArgs) Kind() string { return "Index-FAQ" }

// FaqIndexWorker is a thin adapter into the existing ingestion pipeline: it
// renders the FAQ as markdown, wipes stale chunks, uploads the document, and
// hands off to Process-RAG-File. It never writes chunks directly.
type FaqIndexWorker struct {
	river.WorkerDefaults[IndexFaqArgs]
	faqRepo       FaqRepository
	storageClient StorageClient
	chunkRepo     ChunkRepository
	jobQueue      JobQueue
	fileService   FileService
}

type FaqIndexWorkerDep struct {
	FaqRepository   FaqRepository
	StorageClient   StorageClient
	ChunkRepository ChunkRepository
	JobQueue        JobQueue
	FileService     FileService
}

func NewFaqIndexWorker(dep FaqIndexWorkerDep) *FaqIndexWorker {
	return &FaqIndexWorker{
		faqRepo:       dep.FaqRepository,
		storageClient: dep.StorageClient,
		chunkRepo:     dep.ChunkRepository,
		jobQueue:      dep.JobQueue,
		fileService:   dep.FileService,
	}
}

func (w *FaqIndexWorker) Work(ctx context.Context, job *river.Job[IndexFaqArgs]) error {
	faqID, err := uuid.Parse(job.Args.FAQID)
	if err != nil {
		return err
	}

	faq, err := w.faqRepo.Get(ctx, faqID)
	if err != nil {
		return err
	}

	if faq.Status != domain.FAQStatusAnswered {
		return nil
	}

	if faq.LastIndexedHash == faq.AnswerContentHash {
		return nil
	}

	source := []byte("# " + faq.Question + "\n\n" + faq.Answer)

	// Idempotent re-index: stale chunks must not coexist with new ones.
	if err := w.chunkRepo.DeleteByFileID(ctx, faq.FileID); err != nil {
		return err
	}

	objectKey := domain.FAQS3Key(faq.ID)
	if err := w.storageClient.UploadObject(ctx, objectKey, "text/markdown", source); err != nil {
		return err
	}

	if err := w.fileService.SetStatus(ctx, faq.FileID, domain.UPLOAD_STATUS_COMPLETED); err != nil {
		return err
	}

	if err := w.jobQueue.EnqueueRagFile(ctx, faq.FileID, objectKey); err != nil {
		return err
	}

	return w.faqRepo.SetLastIndexedHash(ctx, faq.ID, faq.AnswerContentHash)
}
