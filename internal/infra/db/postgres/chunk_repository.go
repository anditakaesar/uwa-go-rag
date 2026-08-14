package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/google/uuid"
	"github.com/henvic/pgq"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

type ChunkRepository struct {
	db DBExecutor
}

func NewChunkRepository(db DBExecutor) *ChunkRepository {
	return &ChunkRepository{
		db: db,
	}
}

const chunkColumns = "id, file_id, chunk_index, content, raw_text, token_count, heading_path, content_hash, metadata, embedding, created_at"

type rowScanner interface {
	Scan(dest ...any) error
}

func scanChunkRow(row pgx.Row) (*domain.Chunk, error) {
	return scanChunkColumns(row, false)
}

func scanSimilarChunkRow(row pgx.Row) (*domain.Chunk, error) {
	return scanChunkColumns(row, true)
}

func scanChunkColumns(row rowScanner, withSimilarity bool) (*domain.Chunk, error) {
	var (
		chunk      domain.Chunk
		headingRaw []byte
		metaRaw    []byte
		embedding  pgvector.Vector
	)

	dests := []any{
		&chunk.ID,
		&chunk.FileID,
		&chunk.Index,
		&chunk.Content,
		&chunk.RawText,
		&chunk.TokenCount,
		&headingRaw,
		&chunk.ContentHash,
		&metaRaw,
		&embedding,
		&chunk.CreatedAt,
	}
	if withSimilarity {
		dests = append(dests, &chunk.Similarity)
	}

	err := row.Scan(dests...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &xerror.ErrorResourceNotFound{Message: "chunks not found"}
		}
		return nil, err
	}

	if len(headingRaw) > 0 {
		if err := json.Unmarshal(headingRaw, &chunk.HeadingPath); err != nil {
			return nil, err
		}
	}

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &chunk.Metadata); err != nil {
			return nil, err
		}
	}

	chunk.Embedding = embedding.Slice()

	return &chunk, nil
}

func (r *ChunkRepository) StoreBatch(ctx context.Context, chunks []domain.Chunk) error {
	for _, chunk := range chunks {
		headingPath, err := json.Marshal(chunk.HeadingPath)
		if err != nil {
			return err
		}

		metadata, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return err
		}

		insertQuery := pgq.Insert("chunks").Columns(
			"id", "file_id", "chunk_index", "content", "raw_text", "token_count", "heading_path", "content_hash", "metadata", "embedding",
		).Values(
			chunk.ID,
			chunk.FileID,
			chunk.Index,
			chunk.Content,
			chunk.RawText,
			chunk.TokenCount,
			headingPath,
			chunk.ContentHash,
			metadata,
			pgvector.NewVector(chunk.Embedding),
		).Returning(chunkColumns)

		sql, args, err := insertQuery.SQL()
		if err != nil {
			return err
		}

		row := Executor(ctx, r.db).QueryRow(ctx, sql, args...)
		if _, err := scanChunkRow(row); err != nil {
			return err
		}
	}

	return nil
}

func (r *ChunkRepository) GetByFileID(ctx context.Context, fileID uuid.UUID) ([]domain.Chunk, error) {
	selectQuery := pgq.Select(chunkColumns).
		From("chunks").
		Where("file_id = ?", fileID).
		OrderBy("chunk_index ASC")

	sql, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	rows, err := Executor(ctx, r.db).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := []domain.Chunk{}

	for rows.Next() {
		c, err := scanChunkRow(rows)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, *c)
	}

	return chunks, rows.Err()
}

func (r *ChunkRepository) DeleteByFileID(ctx context.Context, fileID uuid.UUID) error {
	deleteQuery := pgq.Delete("chunks").Where("file_id = ?", fileID)

	sql, args, err := deleteQuery.SQL()
	if err != nil {
		return err
	}

	_, err = Executor(ctx, r.db).Exec(ctx, sql, args...)
	return err
}

func (r *ChunkRepository) CountEmbeddedByFileID(ctx context.Context, fileID uuid.UUID) (int, error) {
	selectQuery := pgq.Select(COUNT_AS_TOTAL).
		From("chunks").
		Where("file_id = ? AND embedding IS NOT NULL", fileID)

	sql, args, err := selectQuery.SQL()
	if err != nil {
		return 0, err
	}

	var count int
	err = Executor(ctx, r.db).QueryRow(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// SearchSimilar returns top-k chunks ordered by cosine similarity against
// embedding, optionally filtered by a minimum similarity threshold.
func (r *ChunkRepository) SearchSimilar(ctx context.Context, embedding []float32, limit int, threshold float64) ([]domain.Chunk, error) {
	queryVec := pgvector.NewVector(embedding)

	selectQuery := pgq.Select(chunkColumns).
		Column("1 - (embedding <=> ?) AS similarity", queryVec).
		From("chunks").
		Where("embedding <=> ? < 1 - ?", queryVec, threshold).
		OrderByClause("embedding <=> ?", queryVec).
		Limit(uint64(limit))

	sql, args, err := selectQuery.SQL()
	if err != nil {
		return nil, err
	}

	rows, err := Executor(ctx, r.db).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := []domain.Chunk{}

	for rows.Next() {
		c, err := scanSimilarChunkRow(rows)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, *c)
	}

	return chunks, rows.Err()
}
