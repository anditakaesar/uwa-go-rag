package worker

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/riverqueue/river"
)

type InsertAuditLogArgs struct {
	domain.AuditLog
}

func (InsertAuditLogArgs) Kind() string { return "Insert-Audit-Log" }

type InsertAuditLogWorker struct {
	auditRecorder Recorder
	river.WorkerDefaults[InsertAuditLogArgs]
}

func NewInsertAuditLogWorker(recorder Recorder) *InsertAuditLogWorker {
	return &InsertAuditLogWorker{
		auditRecorder: recorder,
	}
}

func (w *InsertAuditLogWorker) Work(ctx context.Context, job *river.Job[InsertAuditLogArgs]) error {
	err := w.auditRecorder.Record(ctx, job.Args.AuditLog)
	if err != nil {
		return err
	}
	return nil
}

func (w *InsertAuditLogWorker) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5}
}
