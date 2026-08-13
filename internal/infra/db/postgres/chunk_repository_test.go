package postgres_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/db/postgres"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

const chunkColumns = "id, file_id, chunk_index, content, raw_text, token_count, heading_path, content_hash, metadata, created_at"

func newChunkFixture() (domain.Chunk, time.Time) {
	now := time.Now().UTC()
	fileID := uuid.Must(uuid.NewV7())
	id := uuid.Must(uuid.NewV7())

	return domain.Chunk{
		ID:          id,
		FileID:      fileID,
		Index:       0,
		Content:     "# API Reference > ## Authentication\n\nBearer token required.",
		RawText:     "Bearer token required.",
		TokenCount:  10,
		HeadingPath: []string{"# API Reference", "## Authentication"},
		ContentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Metadata:    map[string]any{"source": "md"},
		CreatedAt:   now,
	}, now
}

func TestChunkRepository_StoreBatch(test *testing.T) {
	test.Parallel()

	chunk, now := newChunkFixture()

	const query = "INSERT INTO chunks (id,file_id,chunk_index,content,raw_text,token_count,heading_path,content_hash,metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, file_id, chunk_index, content, raw_text, token_count, heading_path, content_hash, metadata, created_at"

	test.Run("success", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mockDB.Close()

		headingRaw := []byte(`["# API Reference","## Authentication"]`)
		metaRaw := []byte(`{"source":"md"}`)

		rows := mockDB.NewRows([]string{
			"id", "file_id", "chunk_index", "content", "raw_text", "token_count", "heading_path", "content_hash", "metadata", "created_at",
		}).AddRow(chunk.ID, chunk.FileID, chunk.Index, chunk.Content, chunk.RawText, chunk.TokenCount, headingRaw, chunk.ContentHash, metaRaw, now)

		mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(chunk.ID, chunk.FileID, chunk.Index, chunk.Content, chunk.RawText, chunk.TokenCount, headingRaw, chunk.ContentHash, metaRaw).
			WillReturnRows(rows)

		r := postgres.NewChunkRepository(mockDB)
		err = r.StoreBatch(context.Background(), []domain.Chunk{chunk})

		assert.NoError(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("error", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mockDB.Close()

		mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(chunk.ID, chunk.FileID, chunk.Index, chunk.Content, chunk.RawText, chunk.TokenCount, []byte("null"), chunk.ContentHash, []byte("null")).
			WillReturnError(errors.New("query_error"))

		empty := chunk
		empty.HeadingPath = nil
		empty.Metadata = nil

		r := postgres.NewChunkRepository(mockDB)
		err = r.StoreBatch(context.Background(), []domain.Chunk{empty})

		assert.Error(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestChunkRepository_GetByFileID(test *testing.T) {
	test.Parallel()

	chunk, now := newChunkFixture()

	const query = "SELECT id, file_id, chunk_index, content, raw_text, token_count, heading_path, content_hash, metadata, created_at FROM chunks WHERE file_id = $1 ORDER BY chunk_index ASC"

	test.Run("success", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mockDB.Close()

		headingRaw := []byte(`["# API Reference","## Authentication"]`)
		metaRaw := []byte(`{"source":"md"}`)

		rows := mockDB.NewRows([]string{
			"id", "file_id", "chunk_index", "content", "raw_text", "token_count", "heading_path", "content_hash", "metadata", "created_at",
		}).AddRow(chunk.ID, chunk.FileID, chunk.Index, chunk.Content, chunk.RawText, chunk.TokenCount, headingRaw, chunk.ContentHash, metaRaw, now)

		mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(chunk.FileID).
			WillReturnRows(rows)

		r := postgres.NewChunkRepository(mockDB)
		got, err := r.GetByFileID(context.Background(), chunk.FileID)

		assert.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, chunk.ID, got[0].ID)
		assert.Equal(t, chunk.HeadingPath, got[0].HeadingPath)
		assert.Equal(t, chunk.Metadata, got[0].Metadata)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("error", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mockDB.Close()

		mockDB.ExpectQuery(regexp.QuoteMeta(query)).
			WithArgs(chunk.FileID).
			WillReturnError(errors.New("query_error"))

		r := postgres.NewChunkRepository(mockDB)
		got, err := r.GetByFileID(context.Background(), chunk.FileID)

		assert.Error(t, err)
		assert.Nil(t, got)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestChunkRepository_DeleteByFileID(test *testing.T) {
	test.Parallel()

	_, _ = newChunkFixture()
	fileID := uuid.Must(uuid.NewV7())

	const query = "DELETE FROM chunks WHERE file_id = $1"

	test.Run("success", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mockDB.Close()

		mockDB.ExpectExec(regexp.QuoteMeta(query)).
			WithArgs(fileID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		r := postgres.NewChunkRepository(mockDB)
		err = r.DeleteByFileID(context.Background(), fileID)

		assert.NoError(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})

	test.Run("error", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mockDB.Close()

		mockDB.ExpectExec(regexp.QuoteMeta(query)).
			WithArgs(fileID).
			WillReturnError(errors.New("query_error"))

		r := postgres.NewChunkRepository(mockDB)
		err = r.DeleteByFileID(context.Background(), fileID)

		assert.Error(t, err)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}
