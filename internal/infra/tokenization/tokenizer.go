package tokenization

// Tokenizer ensures chunk size accuracy based on vector embedding model token counts.
type Tokenizer interface {
	CountTokens(text string) int
	Encode(text string) ([]int, error)
	Decode(tokens []int) (string, error)
}
