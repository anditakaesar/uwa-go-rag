package repo

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/google/uuid"
	"github.com/henvic/pgq"
	"github.com/jackc/pgx/v5"
)

type FileRepo struct {
	db IDBExecutor
}

func NewFileRepo(db IDBExecutor) *FileRepo {
	return &FileRepo{
		db: db,
	}
}

func (r *FileRepo) GetExecutor(ctx context.Context) IDBExecutor {
	tx, ok := ctx.Value(common.TxKey).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

const fileColumns = "id, user_id, original_name, mime_type, size_bytes, s3_bucket, s3_key, status, metadata, created_at, updated_at"

func scanFileRow(row pgx.Row) (*domain.File, error) {
	var f domain.File
	err := row.Scan(
		&f.ID,
		&f.UserID,
		&f.OriginalName,
		&f.MimeType,
		&f.SizeBytes,
		&f.S3Bucket,
		&f.S3Key,
		&f.Status,
		&f.Metadata,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		if err.Error() == pgx.ErrNoRows.Error() {
			return nil, &xerror.ErrorResourceNotFound{Message: "resource file not found"}
		}
		return nil, err
	}

	return &f, nil
}

var insertColumns = []string{
	"id", "user_id", "original_name", "mime_type", "size_bytes", "s3_bucket", "s3_key", "status", "metadata",
}

func (r *FileRepo) Insert(ctx context.Context, newFile domain.File) (*domain.File, error) {
	insertQuery := pgq.Insert("files").Columns(insertColumns...).
		Values(
			newFile.ID,
			newFile.UserID,
			newFile.OriginalName,
			newFile.MimeType,
			newFile.SizeBytes,
			newFile.S3Bucket,
			newFile.S3Key,
			newFile.Status,
			newFile.Metadata).
		Returning(fileColumns)
	sql, args, err := insertQuery.SQL()
	if err != nil {
		return nil, err
	}

	row := r.GetExecutor(ctx).QueryRow(ctx, sql, args...)
	return scanFileRow(row)
}

func (r *FileRepo) Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error) {
	selectQuery := pgq.Select(fileColumns).From("files").Where("id = ?", fileID)

	sql, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	row := r.GetExecutor(ctx).QueryRow(ctx, sql, args...)
	return scanFileRow(row)
}
