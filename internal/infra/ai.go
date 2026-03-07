package infra

import (
	"context"
	"errors"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type AIClient struct {
	client openai.Client
}

type AIClientDep struct {
	BaseURL string
	ApiKey  string
}

func NewAIClient(dep AIClientDep) *AIClient {
	client := openai.NewClient(
		option.WithBaseURL(dep.BaseURL),
		option.WithAPIKey(dep.ApiKey),
	)

	return &AIClient{
		client: client,
	}
}

func (b *AIClient) SendPrompt(ctx context.Context, prompt string) (string, error) {
	resp, err := b.client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		// Model: "deepseek/deepseek-r1-0528-qwen3-8b",
		// Model: "google/gemma-3-12b",
		// Model: "openrouter/free",
		Model: "openai/gpt-oss-20b:free",

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

func (b *AIClient) SendTextForEmbedding(ctx context.Context, text string) ([]float64, error) {
	resp, err := b.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model:          "text-embedding-bge-m3",
		Dimensions:     openai.Int(1536),
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return resp.Data[0].Embedding, nil
	}

	return nil, errors.New("no response from embedding endpoint")
}
