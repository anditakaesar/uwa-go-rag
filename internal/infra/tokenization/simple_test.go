package tokenization_test

import (
	"errors"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/infra/tokenization"
	"github.com/stretchr/testify/assert"
)

func TestSimpleTokenizer_CountTokens(t *testing.T) {
	t.Parallel()

	tk := tokenization.NewSimpleTokenizer()

	testCases := []struct {
		name string
		text string
		want int
	}{
		{name: "empty", text: "", want: 0},
		{name: "single word", text: "hello", want: 1},
		{name: "two words", text: "hello world", want: 2},
		{name: "punctuation", text: "hello, world!", want: 4},
		{name: "unicode", text: "halo dunia", want: 2},
		{name: "mixed symbols", text: "a+b=c", want: 5},
		{name: "heading context", text: "# API Reference > ## Authentication\n\nBearer token required.", want: 13},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tk.CountTokens(tc.text))
		})
	}
}

func TestSimpleTokenizer_EncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	tk := tokenization.NewSimpleTokenizer()

	samples := []string{
		"",
		"hello world",
		"# API Reference > ## Authentication\n\nBearer token required.",
		"halo dunia, apa kabar?",
		"code: `x := 1`",
		"tabs\tand\nnewlines",
	}

	for _, sample := range samples {
		encoded, err := tk.Encode(sample)
		assert.NoError(t, err)

		decoded, err := tk.Decode(encoded)
		assert.NoError(t, err)
		assert.Equal(t, sample, decoded, "round trip mismatch")
	}
}

func TestSimpleTokenizer_EncodeDeterministic(t *testing.T) {
	t.Parallel()

	tk := tokenization.NewSimpleTokenizer()

	first, err := tk.Encode("repeat repeat token")
	assert.NoError(t, err)

	second, err := tk.Encode("repeat repeat token")
	assert.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestSimpleTokenizer_DecodeUnknownID(t *testing.T) {
	t.Parallel()

	tk := tokenization.NewSimpleTokenizer()

	_, err := tk.Decode([]int{99})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, tokenization.ErrUnknownTokenID))
}
