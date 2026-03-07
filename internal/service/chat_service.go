package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

type ChatService struct {
	AIClient AIClient
	JobQueue IJobQueue
}

type IChatService interface {
	SendPrompt(ctx context.Context, prompt string) (string, error)
	SendSortJob(ctx context.Context, words []string) error
	SendTextIntoEmbedding(ctx context.Context, text string) error
	DoSort(ctx context.Context, words []string) ([]string, error)
}

func NewChatService(aiClient AIClient, jobQueue IJobQueue) *ChatService {
	return &ChatService{
		AIClient: aiClient,
		JobQueue: jobQueue,
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

	_, err := s.AIClient.SendTextForEmbedding(newCtx, text)

	return err
}

func (s *ChatService) DoSort(ctx context.Context, words []string) ([]string, error) {
	sort.Strings(words)
	xlog.Logger.Info(fmt.Sprintf("chat service sort method called: %v", words))
	return words, nil
}
