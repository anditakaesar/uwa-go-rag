package ai

import (
	"context"
	"errors"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type AIClient struct {
	client         openai.Client
	embeddingModel string
	embeddingDims  int64
}

// EmbeddingDimensions is the fixed vector dimension for this MVP. It must match
// the configured embedding model's output size and the chunks.embedding
// VECTOR(n) column size.
const EmbeddingDimensions int64 = 1024

type ClientDependency struct {
	BaseURL        string
	ApiKey         string
	EmbeddingModel string
}

func NewClient(dep ClientDependency) *AIClient {
	client := openai.NewClient(
		option.WithBaseURL(dep.BaseURL),
		option.WithAPIKey(dep.ApiKey),
	)

	model := dep.EmbeddingModel
	if model == "" {
		model = "text-embedding-bge-m3"
	}

	return &AIClient{
		client:         client,
		embeddingModel: model,
		embeddingDims:  EmbeddingDimensions,
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
	vec, err := b.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	res := make([]float64, len(vec))
	for i, v := range vec {
		res[i] = float64(v)
	}
	return res, nil
}

// Embed returns the vector representation of text via the configured
// OpenAI-compatible embeddings endpoint. The model comes from the client
// config (AI_EMBEDDING_MODEL); the dimension (1024) is fixed and must match
// the configured model's output. pgvector stores float32.
func (b *AIClient) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := b.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model:          b.embeddingModel,
		Dimensions:     openai.Int(b.embeddingDims),
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Data) > 0 {
		emb := resp.Data[0].Embedding
		vec := make([]float32, len(emb))
		for i, v := range emb {
			vec[i] = float32(v)
		}
		return vec, nil
	}

	return nil, errors.New("no response from embedding endpoint")
}
