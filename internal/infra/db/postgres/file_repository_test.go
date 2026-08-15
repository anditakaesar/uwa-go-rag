package postgres_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/db/postgres"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFileColumns = "id, user_id, original_name, mime_type, size_bytes, s3_bucket, s3_key, status, embedding_status, metadata, created_at, updated_at"

func TestFileRepository_FindAll_ExcludesFaqRows(test *testing.T) {
	test.Parallel()

	now := time.Now().UTC()
	param := &domain.FindAllFilesParam{Pagination: common.Pagination{Page: 1, Size: 10}}

	countQuery := "SELECT count(*) as total FROM (SELECT " + testFileColumns + " FROM files WHERE s3_key NOT LIKE 'faq/%' ORDER BY created_at DESC) AS u"
	selectQuery := "SELECT " + testFileColumns + " FROM files WHERE s3_key NOT LIKE 'faq/%' ORDER BY created_at DESC LIMIT 10"

	test.Run("count and listing both exclude faq rows", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockDB.Close()

		fileRow := mockDB.NewRows([]string{
			"id", "user_id", "original_name", "mime_type", "size_bytes", "s3_bucket", "s3_key", "status", "embedding_status", "metadata", "created_at", "updated_at",
		}).AddRow(domain.File{}.ID, 0, "a.md", "text/markdown", 10, "bucket", "docs/a.md", "completed", "completed", map[string]any{}, now, now)

		mockDB.ExpectQuery(regexp.QuoteMeta(countQuery)).WillReturnRows(mockDB.NewRows([]string{"total"}).AddRow(1))
		mockDB.ExpectQuery(regexp.QuoteMeta(selectQuery)).WillReturnRows(fileRow)

		r := postgres.NewFileRepository(mockDB)
		got, err := r.FindAll(context.Background(), param)

		assert.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "docs/a.md", got[0].S3Key)
		assert.Equal(t, int64(1), param.Pagination.Total)
		assert.NoError(t, mockDB.ExpectationsWereMet())
	})
}
