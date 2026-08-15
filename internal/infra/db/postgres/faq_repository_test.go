package postgres_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/faq"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/db/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func timePtr(v time.Time) *time.Time {
	return &v
}

type anyUUID struct{}

func (anyUUID) Match(v any) bool {
	_, ok := v.(uuid.UUID)
	return ok
}

type s3KeyMatcher struct {
	prefix string
}

func (m s3KeyMatcher) Match(v any) bool {
	key, ok := v.(string)
	return ok && regexp.MustCompile(`^`+regexp.QuoteMeta(m.prefix)+`[0-9a-f-]{36}\.md$`).MatchString(key)
}

type nilInt64Matcher struct{}

func (nilInt64Matcher) Match(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}

const faqColumns = "id, question, answer, status, asked_by, answered_by, file_id, answer_content_hash, last_indexed_hash, created_at, answered_at, updated_at"

var faqRowDefs = []string{
	"id", "question", "answer", "status", "asked_by", "answered_by", "file_id", "answer_content_hash", "last_indexed_hash", "created_at", "answered_at", "updated_at",
}

func faqFixture() (domain.FAQ, time.Time, uuid.UUID, uuid.UUID) {
	now := time.Now().UTC()
	faqID := uuid.Must(uuid.NewV7())
	fileID := uuid.Must(uuid.NewV7())

	return domain.FAQ{
		ID:         faqID,
		Question:   "Bagaimana cara reset password?",
		Answer:     "Buka halaman Login, klik Lupa Password.",
		Status:     domain.FAQStatusAnswered,
		AskedBy:    nil,
		AnsweredBy: int64Ptr(42),
		FileID:     fileID,
		CreatedAt:  now,
		AnsweredAt: timePtr(now),
		UpdatedAt:  now,
	}, now, faqID, fileID
}

func faqRowValues(f domain.FAQ, answerHash string) []any {
	return []any{
		f.ID, f.Question, f.Answer, f.Status, f.AskedBy, f.AnsweredBy, f.FileID,
		answerHash, nil, f.CreatedAt, f.AnsweredAt, f.UpdatedAt,
	}
}

func newTestFaqRepository(mockDB pgxmock.PgxPoolIface) *postgres.FaqRepository {
	return postgres.NewFaqRepository(mockDB, postgres.NewFileRepository(mockDB))
}

func TestFaqRepository_CreateFile(test *testing.T) {
	test.Parallel()

	fileInsert := "INSERT INTO files (id,user_id,original_name,mime_type,size_bytes,s3_bucket,s3_key,status,embedding_status,metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id, user_id, original_name, mime_type, size_bytes, s3_bucket, s3_key, status, embedding_status, metadata, created_at, updated_at"
	fileInsertArgs := []any{
		anyUUID{}, int64(0), "Internal FAQ", "text/markdown", int64(0), "",
		s3KeyMatcher{prefix: "faq/"}, domain.UPLOAD_STATUS_PENDING, domain.EMBEDDING_STATUS_PENDING, map[string]any{"source": "faq"},
	}

	test.Run("success - returns generated fileID", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		now := time.Now().UTC()
		row := mockDB.NewRows([]string{
			"id", "user_id", "original_name", "mime_type", "size_bytes", "s3_bucket", "s3_key", "status", "embedding_status", "metadata", "created_at", "updated_at",
		}).AddRow(uuid.Nil, 0, "Internal FAQ", "text/markdown", 0, "", "faq/x.md", "pending", "pending", map[string]any{"source": "faq"}, now, now)

		mockDB.ExpectQuery(regexp.QuoteMeta(fileInsert)).WithArgs(fileInsertArgs...).WillReturnRows(row)

		r := newTestFaqRepository(mockDB)
		fileID, err := r.CreateFile(context.Background(), uuid.Must(uuid.NewV7()))

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, fileID)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("insert failure", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		mockDB.ExpectQuery(regexp.QuoteMeta(fileInsert)).WithArgs(fileInsertArgs...).WillReturnError(errors.New("insert_error"))

		r := newTestFaqRepository(mockDB)
		fileID, err := r.CreateFile(context.Background(), uuid.Must(uuid.NewV7()))

		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, fileID)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestFaqRepository_CreateUnanswered(test *testing.T) {
	test.Parallel()

	faqInsert := "INSERT INTO faqs (id,question,asked_by,file_id) VALUES ($1,$2,$3,$4) RETURNING " + faqColumns

	test.Run("success", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faqID := uuid.Must(uuid.NewV7())
		fileID := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()

		row := mockDB.NewRows(faqRowDefs).AddRow(faqID, "Bagaimana cara reset password?", nil, "unanswered", nil, nil, fileID, nil, nil, now, nil, now)
		mockDB.ExpectQuery(regexp.QuoteMeta(faqInsert)).
			WithArgs(faqID, "Bagaimana cara reset password?", nilInt64Matcher{}, fileID).
			WillReturnRows(row)

		r := newTestFaqRepository(mockDB)
		err = r.CreateUnanswered(context.Background(), domain.FAQ{
			ID:       faqID,
			FileID:   fileID,
			Question: "Bagaimana cara reset password?",
		})

		assert.NoError(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("duplicate question - returns ErrAlreadyCaptured", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faqID := uuid.Must(uuid.NewV7())
		fileID := uuid.Must(uuid.NewV7())

		mockDB.ExpectQuery(regexp.QuoteMeta(faqInsert)).
			WithArgs(faqID, "Bagaimana cara reset password?", nilInt64Matcher{}, fileID).
			WillReturnError(&pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"})

		r := newTestFaqRepository(mockDB)
		err = r.CreateUnanswered(context.Background(), domain.FAQ{
			ID:       faqID,
			FileID:   fileID,
			Question: "Bagaimana cara reset password?",
		})

		assert.ErrorIs(t, err, faq.ErrAlreadyCaptured)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("insert failure", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		mockDB.ExpectQuery(regexp.QuoteMeta(faqInsert)).
			WithArgs(anyUUID{}, "q?", nilInt64Matcher{}, anyUUID{}).
			WillReturnError(errors.New("insert_error"))

		r := newTestFaqRepository(mockDB)
		err = r.CreateUnanswered(context.Background(), domain.FAQ{
			ID:       uuid.Must(uuid.NewV7()),
			FileID:   uuid.Must(uuid.NewV7()),
			Question: "q?",
		})

		assert.Error(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestFaqRepository_ListByStatus(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faq, _, _, _ := faqFixture()
		rows := mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(faq, "hash")...)

		query := "SELECT " + faqColumns + " FROM faqs WHERE status = $1 ORDER BY created_at ASC LIMIT 20"
		mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(domain.FAQStatusAnswered).
			WillReturnRows(rows)

		r := newTestFaqRepository(mockDB)
		got, err := r.ListByStatus(context.Background(), domain.FAQStatusAnswered, 20, 0)

		assert.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, faq.Question, got[0].Question)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("no rows", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		query := "SELECT " + faqColumns + " FROM faqs WHERE status = $1 ORDER BY created_at ASC"
		mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(domain.FAQStatusUnanswered).
			WillReturnRows(mockDB.NewRows(faqRowDefs))

		r := newTestFaqRepository(mockDB)
		got, err := r.ListByStatus(context.Background(), domain.FAQStatusUnanswered, 0, 0)

		assert.NoError(t, err)
		assert.Empty(t, got)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestFaqRepository_Get(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faq, _, faqID, _ := faqFixture()
		rows := mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(faq, "hash")...)

		mockDB.ExpectQuery(regexp.QuoteMeta("SELECT " + faqColumns + " FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnRows(rows)

		r := newTestFaqRepository(mockDB)
		got, err := r.Get(context.Background(), faqID)

		require.NoError(t, err)
		assert.Equal(t, faqID, got.ID)
		assert.Equal(t, faq.Question, got.Question)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("not found", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faqID := uuid.Must(uuid.NewV7())
		mockDB.ExpectQuery(regexp.QuoteMeta("SELECT " + faqColumns + " FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs))

		r := newTestFaqRepository(mockDB)
		got, err := r.Get(context.Background(), faqID)

		assert.ErrorContains(t, err, "faq not found")
		assert.Nil(t, got)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestFaqRepository_Answer(test *testing.T) {
	test.Parallel()

	test.Run("success - flips status and sets hash", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faq, now, faqID, _ := faqFixture()
		unanswered := faq
		unanswered.Status = domain.FAQStatusUnanswered
		unanswered.Answer = ""
		unanswered.AnsweredBy = nil
		unanswered.AnsweredAt = nil

		contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(faq.Answer)))

		mockDB.ExpectQuery(regexp.QuoteMeta("SELECT " + faqColumns + " FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(unanswered, "")...))

		updateQuery := "UPDATE faqs SET status = $1, answer = $2, answered_by = $3, answered_at = $4, answer_content_hash = $5, updated_at = $6 WHERE id = $7 RETURNING " + faqColumns
		mockDB.ExpectQuery(regexp.QuoteMeta(updateQuery)).
			WithArgs(domain.FAQStatusAnswered, faq.Answer, int64(42), now, contentHash, "NOW()", faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(faq, contentHash)...))

		r := newTestFaqRepository(mockDB)
		got, err := r.Answer(context.Background(), faqID, faq.Answer, 42, now)

		require.NoError(t, err)
		assert.Equal(t, domain.FAQStatusAnswered, got.Status)
		assert.Equal(t, int64(42), *got.AnsweredBy)
		assert.Equal(t, contentHash, got.AnswerContentHash)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("edit - re-answering an answered FAQ updates hash", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faq, now, faqID, _ := faqFixture()
		editedAnswer := "Buka halaman Login, klik Lupa Password, lalu ikuti email reset."
		edited := faq
		edited.Answer = editedAnswer

		newHash := fmt.Sprintf("%x", sha256.Sum256([]byte(editedAnswer)))

		mockDB.ExpectQuery(regexp.QuoteMeta("SELECT " + faqColumns + " FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(faq, "oldhash")...))

		updateQuery := "UPDATE faqs SET status = $1, answer = $2, answered_by = $3, answered_at = $4, answer_content_hash = $5, updated_at = $6 WHERE id = $7 RETURNING " + faqColumns
		mockDB.ExpectQuery(regexp.QuoteMeta(updateQuery)).
			WithArgs(domain.FAQStatusAnswered, editedAnswer, int64(42), now, newHash, "NOW()", faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(edited, newHash)...))

		r := newTestFaqRepository(mockDB)
		got, err := r.Answer(context.Background(), faqID, editedAnswer, 42, now)

		require.NoError(t, err)
		assert.Equal(t, newHash, got.AnswerContentHash)
		assert.Equal(t, editedAnswer, got.Answer)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("dismissed status - rejected", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faq, _, faqID, _ := faqFixture()
		dismissed := faq
		dismissed.Status = domain.FAQStatusDismissed

		mockDB.ExpectQuery(regexp.QuoteMeta("SELECT " + faqColumns + " FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(dismissed, "hash")...))

		r := newTestFaqRepository(mockDB)
		got, err := r.Answer(context.Background(), faqID, "answer", 42, time.Now())

		assert.ErrorContains(t, err, "faq cannot be answered in its current status")
		assert.Nil(t, got)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestFaqRepository_SetLastIndexedHash(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faqID := uuid.Must(uuid.NewV7())
		mockDB.ExpectExec(regexp.QuoteMeta("UPDATE faqs SET last_indexed_hash = $1, updated_at = $2 WHERE id = $3")).
			WithArgs("hash", "NOW()", faqID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		r := newTestFaqRepository(mockDB)
		err = r.SetLastIndexedHash(context.Background(), faqID, "hash")

		assert.NoError(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestFaqRepository_Delete(test *testing.T) {
	test.Parallel()

	test.Run("success - removes faqs row then files row", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faq, _, faqID, _ := faqFixture()
		now := time.Now().UTC()

		mockDB.ExpectQuery(regexp.QuoteMeta("SELECT " + faqColumns + " FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs).AddRow(faqRowValues(faq, "hash")...))

		mockDB.ExpectExec(regexp.QuoteMeta("DELETE FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		mockDB.ExpectQuery(regexp.QuoteMeta("DELETE FROM files WHERE id = $1 RETURNING id, user_id, original_name, mime_type, size_bytes, s3_bucket, s3_key, status, embedding_status, metadata, created_at, updated_at")).
			WithArgs(faq.FileID).
			WillReturnRows(mockDB.NewRows([]string{
				"id", "user_id", "original_name", "mime_type", "size_bytes", "s3_bucket", "s3_key", "status", "embedding_status", "metadata", "created_at", "updated_at",
			}).AddRow(faq.FileID, 0, "Internal FAQ", "text/markdown", 0, "", "faq/x.md", "pending", "pending", map[string]any{"source": "faq"}, now, now))

		r := newTestFaqRepository(mockDB)
		err = r.Delete(context.Background(), faqID)

		assert.NoError(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("not found", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		faqID := uuid.Must(uuid.NewV7())
		mockDB.ExpectQuery(regexp.QuoteMeta("SELECT " + faqColumns + " FROM faqs WHERE id = $1")).
			WithArgs(faqID).
			WillReturnRows(mockDB.NewRows(faqRowDefs))

		r := newTestFaqRepository(mockDB)
		err = r.Delete(context.Background(), faqID)

		assert.ErrorContains(t, err, "faq not found")
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}
