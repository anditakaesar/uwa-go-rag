package service

import (
	"context"
	"fmt"
	"sort"
)

type ChatService struct {
	Bot      IChatBot
	JobQueue IJobQueue
}

type IChatService interface {
	SendPrompt(ctx context.Context, prompt string) (string, error)
	SendSortJob(ctx context.Context, words []string) error
	DoSort(ctx context.Context, words []string) ([]string, error)
}

func NewChatService(bot IChatBot, jobQueue IJobQueue) *ChatService {
	return &ChatService{
		Bot:      bot,
		JobQueue: jobQueue,
	}
}

func (s *ChatService) SendPrompt(ctx context.Context, prompt string) (string, error) {
	return s.Bot.SendPrompt(ctx, prompt)
}

func (s *ChatService) SendSortJob(ctx context.Context, words []string) error {
	return s.JobQueue.EnqueueChat(ctx, words)
}

func (s *ChatService) DoSort(ctx context.Context, words []string) ([]string, error) {
	sort.Strings(words)
	fmt.Printf("service sorted the strings: %v", words)
	return words, nil
}
