package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/faq"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/google/uuid"
	"github.com/henvic/pgq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type FaqRepository struct {
	db       DBExecutor
	fileRepo *FileRepository
}

func NewFaqRepository(db DBExecutor, fileRepo *FileRepository) *FaqRepository {
	return &FaqRepository{
		db:       db,
		fileRepo: fileRepo,
	}
}

const faqColumns = "id, question, answer, status, asked_by, answered_by, file_id, answer_content_hash, last_indexed_hash, created_at, answered_at, updated_at"

func scanFAQRow(row pgx.Row) (*domain.FAQ, error) {
	var (
		model             domain.FAQ
		answer            sql.NullString
		answerContentHash sql.NullString
		lastIndexedHash   sql.NullString
	)

	err := row.Scan(
		&model.ID,
		&model.Question,
		&answer,
		&model.Status,
		&model.AskedBy,
		&model.AnsweredBy,
		&model.FileID,
		&answerContentHash,
		&lastIndexedHash,
		&model.CreatedAt,
		&model.AnsweredAt,
		&model.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &xerror.ErrorResourceNotFound{Message: "faq not found"}
		}
		return nil, err
	}

	model.Answer = answer.String
	model.AnswerContentHash = answerContentHash.String
	model.LastIndexedHash = lastIndexedHash.String

	return &model, nil
}

// CreateFile inserts the FAQ's derived files row (s3_key = 'faq/<faq_id>.md')
// and returns the generated fileID. The row participates in any transaction
// supplied through the context.
func (r *FaqRepository) CreateFile(ctx context.Context, faqID uuid.UUID) (uuid.UUID, error) {
	fileID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}

	if _, err := r.fileRepo.Insert(ctx, domain.File{
		ID:           fileID,
		UserID:       0,
		OriginalName: "Internal FAQ",
		MimeType:     "text/markdown",
		SizeBytes:    0,
		S3Bucket:     "",
		S3Key:        domain.FAQS3Key(faqID),
		Status:       domain.UPLOAD_STATUS_PENDING,
		Metadata:     map[string]any{"source": "faq"},
	}); err != nil {
		return uuid.Nil, err
	}

	return fileID, nil
}

// CreateUnanswered inserts the faqs row. A duplicate open question surfaces
// as faq.ErrAlreadyCaptured (partial unique index on lower(question)).
func (r *FaqRepository) CreateUnanswered(ctx context.Context, newFAQ domain.FAQ) error {
	insertQuery := pgq.Insert("faqs").Columns(
		"id", "question", "asked_by", "file_id",
	).Values(
		newFAQ.ID,
		newFAQ.Question,
		newFAQ.AskedBy,
		newFAQ.FileID,
	).Returning(faqColumns)

	sql, args, err := insertQuery.SQL()
	if err != nil {
		return err
	}

	row := Executor(ctx, r.db).QueryRow(ctx, sql, args...)
	if _, err := scanFAQRow(row); err != nil {
		if isUniqueViolation(err) {
			return faq.ErrAlreadyCaptured
		}
		return err
	}

	return nil
}

func (r *FaqRepository) ListByStatus(ctx context.Context, status domain.FAQStatus, limit, offset int) ([]domain.FAQ, error) {
	selectQuery := pgq.Select(faqColumns).
		From("faqs").
		Where("status = ?", status).
		OrderBy("created_at ASC")

	if limit > 0 {
		selectQuery = selectQuery.Limit(uint64(limit))
	}

	if offset > 0 {
		selectQuery = selectQuery.Offset(uint64(offset))
	}

	sql, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	rows, err := Executor(ctx, r.db).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	faqs := []domain.FAQ{}

	for rows.Next() {
		f, err := scanFAQRow(rows)
		if err != nil {
			return nil, err
		}
		faqs = append(faqs, *f)
	}

	return faqs, rows.Err()
}

func (r *FaqRepository) Get(ctx context.Context, id uuid.UUID) (*domain.FAQ, error) {
	selectQuery := pgq.Select(faqColumns).
		From("faqs").
		Where("id = ?", id)

	sql, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	row := Executor(ctx, r.db).QueryRow(ctx, sql, args...)
	return scanFAQRow(row)
}

func (r *FaqRepository) Answer(ctx context.Context, id uuid.UUID, answer string, answeredBy int64, now time.Time) (*domain.FAQ, error) {
	faq, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Re-answering an answered FAQ is an answer edit: it bumps the content
	// hash so the index worker regenerates chunks.
	if faq.Status != domain.FAQStatusUnanswered && faq.Status != domain.FAQStatusAnswered {
		return nil, &xerror.ErrorValidation{Message: "faq cannot be answered in its current status"}
	}

	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(answer)))

	updateQuery := pgq.Update("faqs").
		Where("id = ?", id).
		Set("status", domain.FAQStatusAnswered).
		Set("answer", answer).
		Set("answered_by", answeredBy).
		Set("answered_at", now).
		Set("answer_content_hash", contentHash).
		Set("updated_at", "NOW()").
		Returning(faqColumns)

	sql, args, err := updateQuery.SQL()
	if err != nil {
		return nil, err
	}

	row := Executor(ctx, r.db).QueryRow(ctx, sql, args...)
	return scanFAQRow(row)
}

func (r *FaqRepository) SetLastIndexedHash(ctx context.Context, id uuid.UUID, hash string) error {
	updateQuery := pgq.Update("faqs").
		Where("id = ?", id).
		Set("last_indexed_hash", hash).
		Set("updated_at", "NOW()")

	sql, args, err := updateQuery.SQL()
	if err != nil {
		return err
	}

	_, err = Executor(ctx, r.db).Exec(ctx, sql, args...)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
