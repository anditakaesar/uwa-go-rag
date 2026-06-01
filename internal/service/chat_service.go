package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

type ChatService struct {
	RagRepo   IRagRepository
	AIClient  AIClient
	JobQueue  IJobQueue
	UploadDir string
}

type IChatService interface {
	SendPrompt(ctx context.Context, prompt string) (string, error)
	SendSortJob(ctx context.Context, words []string) error
	SendProcessDocJob(ctx context.Context, ragFileID int64) error
	SendTextIntoEmbedding(ctx context.Context, text string) error
	DoSort(ctx context.Context, words []string) ([]string, error)
	//CreateRagFile(ctx context.Context, ragFile domain.RagFile) (*domain.RagFile, error)
}

type ChatServiceDep struct {
	RagRepo   IRagRepository
	AIClient  AIClient
	JobQueue  IJobQueue
	UploadDir string
}

func NewChatService(dep ChatServiceDep) *ChatService {
	return &ChatService{
		RagRepo:   dep.RagRepo,
		AIClient:  dep.AIClient,
		JobQueue:  dep.JobQueue,
		UploadDir: dep.UploadDir,
	}
}

func (s *ChatService) SendPrompt(ctx context.Context, prompt string) (string, error) {
	return s.AIClient.SendPrompt(ctx, prompt)
}

func (s *ChatService) SendSortJob(ctx context.Context, words []string) error {
	return s.JobQueue.EnqueueChat(ctx, words)
}

func (s *ChatService) SendProcessDocJob(ctx context.Context, ragFileID int64) error {
	return s.JobQueue.EnqueueRagFile(ctx, ragFileID)
}

func (s *ChatService) SendTextIntoEmbedding(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}

	newCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := s.AIClient.SendTextForEmbedding(newCtx, text)

	return err
}

func (s *ChatService) DoSort(ctx context.Context, words []string) ([]string, error) {
	sort.Strings(words)
	xlog.Logger.Info(fmt.Sprintf("chat service sort method called: %v", words))
	return words, nil
}
