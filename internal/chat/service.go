package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/google/uuid"
)

// RetrievalRepository searches the persisted chunk store for the most similar
// vectors to a query embedding. Implemented by postgres.ChunkRepository.
type RetrievalRepository interface {
	SearchSimilar(ctx context.Context, embedding []float32, limit int, threshold float64) ([]domain.Chunk, error)
}

// Embedder returns the vector representation of text via the configured
// embedding model. Implemented by ai.AIClient.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// LLMClient generates text answers, optionally grounded in injected context.
// Implemented by ai.AIClient.
type LLMClient interface {
	SendPrompt(ctx context.Context, prompt string) (string, error)
	SendContextPrompt(ctx context.Context, contextText string, question string) (string, error)
}

// UnansweredRecorder captures questions the model could not answer from
// context. Implemented by the FAQ service (see the FAQ PRD).
type UnansweredRecorder interface {
	RecordUnanswered(ctx context.Context, question string) error
}

type IJobQueue interface {
	EnqueueChat(ctx context.Context, words []string) error
	EnqueueRagFile(ctx context.Context, fileID uuid.UUID, objectKey string) error
}

const (
	topK         = 5
	simThreshold = 0.5

	noContextMsg = "Maaf, saya tidak tahu. Silakan coba lagi dengan pertanyaan yang lebih spesifik."
)

var fallbackWords = []string{"tidak tahu", "tidak mengetahui", "tidak ada informasi", "i don't know"}

type ChatService struct {
	ChunkRepo RetrievalRepository
	AIClient  LLMClient
	Embedder  Embedder
	Recorder  UnansweredRecorder
	JobQueue  IJobQueue
	UploadDir string
}

type IChatService interface {
	Chat(ctx context.Context, prompt string) (*ChatResponse, error)
	SendPrompt(ctx context.Context, prompt string) (string, error)
	SendSortJob(ctx context.Context, words []string) error
	SendTextIntoEmbedding(ctx context.Context, text string) error
	DoSort(ctx context.Context, words []string) ([]string, error)
}

type ChatServiceDep struct {
	ChunkRepo RetrievalRepository
	AIClient  LLMClient
	Embedder  Embedder
	Recorder  UnansweredRecorder
	JobQueue  IJobQueue
	UploadDir string
}

func NewChatService(dep ChatServiceDep) *ChatService {
	return &ChatService{
		ChunkRepo: dep.ChunkRepo,
		AIClient:  dep.AIClient,
		Embedder:  dep.Embedder,
		Recorder:  dep.Recorder,
		JobQueue:  dep.JobQueue,
		UploadDir: dep.UploadDir,
	}
}

type ChatResponse struct {
	Message   string     `json:"message"`
	Citations []Citation `json:"citations"`
}

type Citation struct {
	ChunkID     uuid.UUID `json:"chunkId"`
	FileID      uuid.UUID `json:"fileId"`
	HeadingPath []string  `json:"headingPath"`
	Similarity  float64   `json:"similarity"`
	Snippet     string    `json:"snippet"`
}

// Chat runs the full RAG flow: embed the query, search similar chunks, augment
// the prompt, and return a grounded answer with citations.
func (s *ChatService) Chat(ctx context.Context, prompt string) (*ChatResponse, error) {
	queryVec, err := s.Embedder.Embed(ctx, prompt)
	if err != nil {
		return nil, err
	}

	chunks, err := s.ChunkRepo.SearchSimilar(ctx, queryVec, topK, simThreshold)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		s.recordUnanswered(ctx, prompt)
		return &ChatResponse{Message: noContextMsg, Citations: []Citation{}}, nil
	}

	answer, err := s.AIClient.SendContextPrompt(ctx, buildContext(chunks), prompt)
	if err != nil {
		return nil, err
	}

	if isFallbackAnswer(answer) {
		s.recordUnanswered(ctx, prompt)
		return &ChatResponse{Message: noContextMsg, Citations: []Citation{}}, nil
	}

	return &ChatResponse{
		Message:   answer,
		Citations: toCitations(chunks),
	}, nil
}

// recordUnanswered captures a grounded failure without failing the chat
// response; a recording error is only logged.
func (s *ChatService) recordUnanswered(ctx context.Context, question string) {
	if s.Recorder == nil {
		return
	}
	if err := s.Recorder.RecordUnanswered(ctx, question); err != nil {
		xlog.Logger.Error("error recording unanswered question", "err", err)
	}
}

func (s *ChatService) SendPrompt(ctx context.Context, prompt string) (string, error) {
	return s.AIClient.SendPrompt(ctx, prompt)
}

func (s *ChatService) SendSortJob(ctx context.Context, words []string) error {
	return s.JobQueue.EnqueueChat(ctx, words)
}

func (s *ChatService) SendTextIntoEmbedding(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}

	newCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := s.Embedder.Embed(newCtx, text)

	return err
}

func (s *ChatService) DoSort(ctx context.Context, words []string) ([]string, error) {
	sort.Strings(words)
	xlog.Logger.Info(fmt.Sprintf("chat service sort method called: %v", words))
	return words, nil
}

func buildContext(chunks []domain.Chunk) string {
	var b strings.Builder
	for i, c := range chunks {
		fmt.Fprintf(&b, "[%d] (%s) similarity %.2f\n%s\n\n",
			i+1, strings.Join(c.HeadingPath, " > "), c.Similarity, c.Content)
	}
	return strings.TrimSpace(b.String())
}

func toCitations(chunks []domain.Chunk) []Citation {
	citations := make([]Citation, 0, len(chunks))
	for _, c := range chunks {
		citations = append(citations, Citation{
			ChunkID:     c.ID,
			FileID:      c.FileID,
			HeadingPath: c.HeadingPath,
			Similarity:  c.Similarity,
			Snippet:     truncate(c.RawText, 200),
		})
	}
	return citations
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func isFallbackAnswer(answer string) bool {
	lower := strings.ToLower(strings.TrimSpace(answer))
	for _, w := range fallbackWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}
