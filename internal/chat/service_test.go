package chat_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/chat"
	"github.com/anditakaesar/uwa-go-rag/internal/chat/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const noContextMsg = "Maaf, saya tidak tahu. Silakan coba lagi dengan pertanyaan yang lebih spesifik."

func newChatService(t *testing.T) (*chat.Service, *mocks.MockRetrievalRepository, *mocks.MockEmbedder, *mocks.MockLLMClient, *mocks.MockUnansweredRecorder) {
	t.Helper()

	repo := mocks.NewMockRetrievalRepository(t)
	embedder := mocks.NewMockEmbedder(t)
	llm := mocks.NewMockLLMClient(t)
	recorder := mocks.NewMockUnansweredRecorder(t)

	svc := chat.NewService(chat.ServiceDependency{
		ChunkRepo: repo,
		AIClient:  llm,
		Embedder:  embedder,
		Recorder:  recorder,
	})

	return svc, repo, embedder, llm, recorder
}

func chunkFixture(similarity float64, rawText string) domain.Chunk {
	return domain.Chunk{
		ID:          uuid.Must(uuid.NewV7()),
		FileID:      uuid.Must(uuid.NewV7()),
		Content:     "# API Reference > ## Authentication\n\n" + rawText,
		RawText:     rawText,
		HeadingPath: []string{"# API Reference", "## Authentication"},
		Similarity:  similarity,
	}
}

func TestChat_GroundedAnswer(t *testing.T) {
	svc, repo, embedder, llm, _ := newChatService(t)

	queryVec := []float32{0.1, 0.2, 0.3}
	chunk := chunkFixture(0.92, "Bearer token required.")

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(queryVec, nil)
	repo.EXPECT().SearchSimilar(mock.Anything, queryVec, 5, 0.5).Return([]domain.Chunk{chunk}, nil)
	llm.EXPECT().
		SendContextPrompt(mock.Anything, mock.MatchedBy(func(ctxText string) bool {
			return strings.Contains(ctxText, "[1] (# API Reference > ## Authentication) similarity 0.92") &&
				strings.Contains(ctxText, "Bearer token required.")
		}), "pertanyaan").
		Return("Autentikasi memerlukan Bearer token.", nil)

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Autentikasi memerlukan Bearer token.", resp.Message)

	require.Len(t, resp.Citations, 1)
	c := resp.Citations[0]
	assert.Equal(t, chunk.ID, c.ChunkID)
	assert.Equal(t, chunk.FileID, c.FileID)
	assert.Equal(t, []string{"# API Reference", "## Authentication"}, c.HeadingPath)
	assert.InDelta(t, 0.92, c.Similarity, 0.0001)
	assert.Equal(t, "Bearer token required.", c.Snippet)
}

func TestChat_NoContext_ShortCircuitsAndRecords(t *testing.T) {
	svc, repo, embedder, _, recorder := newChatService(t)

	queryVec := []float32{0.1, 0.2, 0.3}

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(queryVec, nil)
	repo.EXPECT().SearchSimilar(mock.Anything, queryVec, 5, 0.5).Return([]domain.Chunk{}, nil)
	recorder.EXPECT().RecordUnanswered(mock.Anything, "pertanyaan").Return(nil)

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, noContextMsg, resp.Message)
	assert.Empty(t, resp.Citations)
}

func TestChat_FallbackAnswer_RecordsAndReturnsNoContext(t *testing.T) {
	svc, repo, embedder, llm, recorder := newChatService(t)

	queryVec := []float32{0.1, 0.2, 0.3}
	chunk := chunkFixture(0.9, "Bearer token required.")

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(queryVec, nil)
	repo.EXPECT().SearchSimilar(mock.Anything, queryVec, 5, 0.5).Return([]domain.Chunk{chunk}, nil)
	llm.EXPECT().SendContextPrompt(mock.Anything, mock.Anything, "pertanyaan").Return("Maaf, saya tidak tahu.", nil)
	recorder.EXPECT().RecordUnanswered(mock.Anything, "pertanyaan").Return(nil)

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, noContextMsg, resp.Message)
	assert.Empty(t, resp.Citations)
}

func TestChat_RecorderError_DoesNotFailResponse(t *testing.T) {
	svc, repo, embedder, _, recorder := newChatService(t)

	queryVec := []float32{0.1, 0.2, 0.3}

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(queryVec, nil)
	repo.EXPECT().SearchSimilar(mock.Anything, queryVec, 5, 0.5).Return([]domain.Chunk{}, nil)
	recorder.EXPECT().RecordUnanswered(mock.Anything, "pertanyaan").Return(errors.New("record failed"))

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, noContextMsg, resp.Message)
}

func TestChat_EmbedError(t *testing.T) {
	svc, _, embedder, _, _ := newChatService(t)

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(nil, errors.New("embed failed"))

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestChat_SearchError(t *testing.T) {
	svc, repo, embedder, _, _ := newChatService(t)

	queryVec := []float32{0.1, 0.2, 0.3}

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(queryVec, nil)
	repo.EXPECT().SearchSimilar(mock.Anything, queryVec, 5, 0.5).Return(nil, errors.New("search failed"))

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestChat_LLMError(t *testing.T) {
	svc, repo, embedder, llm, _ := newChatService(t)

	queryVec := []float32{0.1, 0.2, 0.3}
	chunk := chunkFixture(0.9, "Bearer token required.")

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(queryVec, nil)
	repo.EXPECT().SearchSimilar(mock.Anything, queryVec, 5, 0.5).Return([]domain.Chunk{chunk}, nil)
	llm.EXPECT().SendContextPrompt(mock.Anything, mock.Anything, "pertanyaan").Return("", errors.New("llm failed"))

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestChat_SnippetTruncated(t *testing.T) {
	svc, repo, embedder, llm, _ := newChatService(t)

	queryVec := []float32{0.1, 0.2, 0.3}
	longText := strings.Repeat("a", 250)
	chunk := chunkFixture(0.8, longText)

	embedder.EXPECT().Embed(mock.Anything, "pertanyaan").Return(queryVec, nil)
	repo.EXPECT().SearchSimilar(mock.Anything, queryVec, 5, 0.5).Return([]domain.Chunk{chunk}, nil)
	llm.EXPECT().SendContextPrompt(mock.Anything, mock.Anything, "pertanyaan").Return("jawaban", nil)

	resp, err := svc.Chat(context.Background(), "pertanyaan")

	require.NoError(t, err)
	require.Len(t, resp.Citations, 1)
	assert.Len(t, []rune(resp.Citations[0].Snippet), 200)
}
