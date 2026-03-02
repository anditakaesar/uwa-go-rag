package infra

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type ChatBot struct {
	client openai.Client
}

type ChatBotDeps struct {
	BaseURL string
	ApiKey  string
}

func NewChatBot(dep ChatBotDeps) *ChatBot {
	client := openai.NewClient(
		option.WithBaseURL(dep.BaseURL),
		option.WithAPIKey(dep.ApiKey),
	)

	return &ChatBot{
		client: client,
	}
}

func (b *ChatBot) SendPrompt(ctx context.Context, prompt string) (string, error) {
	resp, err := b.client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		// Model: "deepseek/deepseek-r1-0528-qwen3-8b",
		Model: "google/gemma-3-12b",
		// Reasoning: shared.ReasoningParam{ // not compatible with local llm
		// 	Effort: openai.ReasoningEffortLow,
		// },
		Instructions: openai.String(`
			response in plain text;
			do not response in markdown;
			emojis are ok;
			always answer in Bahasa Indonesia
		`),
		MaxOutputTokens: openai.Int(1024),  // set this to limit the generation response
		Temperature:     openai.Float(0.5), // 0 - 0.1: focused, predictable, literal; 1.0+: diverse, creative
	})

	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}
