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
	textChatModel  string
	embeddingDims  int64
}

// EmbeddingDimensions is the fixed vector dimension for this MVP. It must match
// the configured embedding model's output size and the chunks.embedding
// VECTOR(n) column size.
const EmbeddingDimensions int64 = 1024

// Chat generation settings shared by SendPrompt and SendContextPrompt.
const (
	chatMaxTokens   = 1024
	chatTemperature = 0.5
)

// baseInstructions is the shared system prompt for every chat call: plain
// text, no markdown, always Bahasa Indonesia.
const baseInstructions = `response in plain text;
do not response in markdown;
emojis are ok;
always answer in Bahasa Indonesia`

// groundedInstructions enforces the RAG grounding rules: no tools/internet and
// answer strictly from the injected context, otherwise reply with the canonical
// "I don't know" message.
const groundedInstructions = `Anda TIDAK memiliki akses internet, mesin pencari, atau alat apa pun selain
konteks di bawah ini. DILARANG menjawab dari pengetahuan umum atau menebak.

Gunakan hanya konteks di bawah ini untuk menjawab pertanyaan pengguna.
Jika jawaban tidak ada dalam konteks, balas PERSIS dengan kalimat:
"Maaf, saya tidak tahu. Silakan coba lagi dengan pertanyaan yang lebih spesifik."
Rujuk sumber sesuai heading yang tersedia.`

type ClientDependency struct {
	BaseURL        string
	ApiKey         string
	EmbeddingModel string
	TextChatModel  string
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

	textModel := dep.TextChatModel
	if textModel == "" {
		textModel = "qwen/qwen3-8b"
	}

	return &AIClient{
		client:         client,
		embeddingModel: model,
		textChatModel:  textModel,
		embeddingDims:  EmbeddingDimensions,
	}
}

func (b *AIClient) SendPrompt(ctx context.Context, prompt string) (string, error) {
	resp, err := b.client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		Model:           b.textChatModel,
		Instructions:    openai.String(baseInstructions),
		MaxOutputTokens: openai.Int(chatMaxTokens),
		Temperature:     openai.Float(chatTemperature),
	})

	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}

// SendContextPrompt answers `question` grounded in the provided context. The
// retrieved context is appended to the system instructions and the question is
// passed as the user input. No tool or function definitions are registered on
// this call, so the model cannot invoke external tools.
func (b *AIClient) SendContextPrompt(ctx context.Context, contextText string, question string) (string, error) {
	instructions := baseInstructions + "\n\n" + groundedInstructions + "\n\n" +
		"===== KONTEKS =====\n" + contextText + "\n===== AKHIR KONTEKS ====="

	resp, err := b.client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(question),
		},
		Model:           b.textChatModel,
		Instructions:    openai.String(instructions),
		MaxOutputTokens: openai.Int(chatMaxTokens),
		Temperature:     openai.Float(chatTemperature),
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
