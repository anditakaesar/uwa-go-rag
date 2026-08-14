package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// GenerateChunksArgs is the Job 2 payload carrying a single finalized chunk
// (content already sized and context-prepended by Job 1) for persistence.
type GenerateChunksArgs struct {
	FileID      string   `json:"fileID"`
	Index       int      `json:"index"`
	HeadingPath []string `json:"headingPath"`
	Content     string   `json:"content"`
	RawText     string   `json:"rawText"`
	TokenCount  int      `json:"tokenCount"`
}

func (GenerateChunksArgs) Kind() string { return "Generate-Chunks" }

// ChunkGeneratorWorker consumes GenerateChunksArgs, embeds the finalized
// chunk content, and batch-writes domain.Chunk records (including the vector)
// to the ChunkRepository.
type ChunkGeneratorWorker struct {
	river.WorkerDefaults[GenerateChunksArgs]
	chunkRepository ChunkRepository
	embedder        Embedder
}

func NewChunkGeneratorWorker(chunkRepository ChunkRepository, embedder Embedder) *ChunkGeneratorWorker {
	return &ChunkGeneratorWorker{
		chunkRepository: chunkRepository,
		embedder:        embedder,
	}
}

func (w *ChunkGeneratorWorker) Work(ctx context.Context, job *river.Job[GenerateChunksArgs]) error {
	chunk, err := buildChunk(job.Args)
	if err != nil {
		return err
	}

	vec, err := w.embedder.Embed(ctx, chunk.Content)
	if err != nil {
		return err
	}
	chunk.Embedding = vec

	return w.chunkRepository.StoreBatch(ctx, []domain.Chunk{*chunk})
}

func buildChunk(args GenerateChunksArgs) (*domain.Chunk, error) {
	fileID, err := uuid.Parse(args.FileID)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	return &domain.Chunk{
		ID:          id,
		FileID:      fileID,
		Index:       args.Index,
		Content:     args.Content,
		RawText:     args.RawText,
		TokenCount:  args.TokenCount,
		HeadingPath: args.HeadingPath,
		ContentHash: contentHash(args.Content),
		Metadata:    map[string]any{},
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
