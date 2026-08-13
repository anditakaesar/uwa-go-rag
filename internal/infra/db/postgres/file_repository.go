package postgres

import (
	"context"
	"errors"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/google/uuid"
	"github.com/henvic/pgq"
	"github.com/jackc/pgx/v5"
)

type FileRepository struct {
	db DBExecutor
}

func NewFileRepository(db DBExecutor) *FileRepository {
	return &FileRepository{
		db: db,
	}
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &xerror.ErrorResourceNotFound{Message: "resource file not found"}
		}
		return nil, err
	}

	return &f, nil
}

var insertColumns = []string{
	"id", "user_id", "original_name", "mime_type", "size_bytes", "s3_bucket", "s3_key", "status", "metadata",
}

func (r *FileRepository) Insert(ctx context.Context, newFile domain.File) (*domain.File, error) {
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

	row := Executor(ctx, r.db).QueryRow(ctx, sql, args...)
	return scanFileRow(row)
}

func (r *FileRepository) Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error) {
	selectQuery := pgq.Select(fileColumns).From("files").Where("id = ?", fileID)

	sql, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	row := Executor(ctx, r.db).QueryRow(ctx, sql, args...)
	return scanFileRow(row)
}

func (r *FileRepository) FindAll(ctx context.Context, param *domain.FindAllFilesParam) ([]domain.File, error) {
	selectQuery := pgq.Select(fileColumns).
		From("files").OrderBy("created_at DESC")

	if len(param.MimeTypes) > 0 {
		selectQuery = selectQuery.Where(pgq.Eq{"mime_type": param.MimeTypes})
	}

	countQuery, countArgs, err := pgq.Select(COUNT_AS_TOTAL).FromSelect(selectQuery, "u").SQL()
	if err != nil {
		return nil, err
	}

	err = Executor(ctx, r.db).QueryRow(ctx, countQuery, countArgs...).Scan(&param.Pagination.Total)
	if err != nil {
		return nil, err
	}

	param.Pagination.WrapPaging(&selectQuery)
	query, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	rows, err := Executor(ctx, r.db).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []domain.File{}

	for rows.Next() {
		f, err := scanFileRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *f)
	}

	return list, rows.Err()
}

func (r *FileRepository) Update(ctx context.Context, id uuid.UUID, updateParam domain.UpdateFileParam) (*domain.File, error) {
	updateQuery := pgq.Update("files").
		Where("id = ?", id).Returning(fileColumns)
	argCount := 0

	if updateParam.Status != nil {
		updateQuery = updateQuery.Set("status", *updateParam.Status)
		argCount++
	}

	if argCount == 0 {
		return nil, errors.New("nothing to update")
	}

	updateQuery = updateQuery.Set("updated_at", "NOW()")

	query, args, err := updateQuery.SQL()
	if err != nil {
		return nil, err
	}

	row := Executor(ctx, r.db).QueryRow(ctx, query, args...)
	return scanFileRow(row)
}

func (r *FileRepository) Delete(ctx context.Context, id uuid.UUID) (*domain.File, error) {
	deleteQ := pgq.Delete("files").
		Where("id = ?", id).Returning(fileColumns)

	query, args, err := deleteQ.SQL()
	if err != nil {
		return nil, err
	}

	row := Executor(ctx, r.db).QueryRow(ctx, query, args...)
	return scanFileRow(row)
}
