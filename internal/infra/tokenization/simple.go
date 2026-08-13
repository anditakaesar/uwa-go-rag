package tokenization

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// ErrUnknownTokenID is returned by Decode when an id has no matching token.
var ErrUnknownTokenID = errors.New("tokenization: unknown token id")

// wordPattern matches a maximal run of word characters (unicode letters,
// digits, and underscore).
var wordPattern = regexp.MustCompile(`[\p{L}\p{N}_]+`)

// SimpleTokenizer is a deterministic, reversible rule-based tokenizer.
//
// A token is a maximal run of word characters, a punctuation character, or a
// whitespace run. Whitespace runs that directly precede a word are prefixed to
// that word (BPE-style leading-space marker, e.g. " world"), so token counts
// approximate real sentencepiece-style tokenizers instead of counting each
// space separately. Concatenating the decoded tokens always reproduces the
// original text exactly.
//
// Note: token ids are assigned in first-seen order and are scoped to the
// instance lifetime. They are NOT aligned to a specific embedding model
// vocabulary (e.g. text-embedding-bge-m3). Swap this implementation for a
// model BPE tokenizer behind the Tokenizer interface to get exact model
// token counts without changing call sites.
type SimpleTokenizer struct {
	mu     sync.RWMutex
	ids    map[string]int
	tokens []string
}

func NewSimpleTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{
		ids: make(map[string]int),
	}
}

// split breaks text into a sequence of tokens that concatenate back to text.
func (t *SimpleTokenizer) split(text string) []string {
	if text == "" {
		return nil
	}

	spans := wordPattern.FindAllStringIndex(text, -1)
	if len(spans) == 0 {
		return []string{text}
	}

	tokens := make([]string, 0, len(spans))
	prevEnd := 0

	for _, span := range spans {
		start, end := span[0], span[1]

		if run := text[prevEnd:start]; run != "" {
			pieces := splitNonWord(run)
			if len(pieces) > 0 {
				last := pieces[len(pieces)-1]
				if r, _ := utf8.DecodeRuneInString(last); unicode.IsSpace(r) {
					// Trailing whitespace run prefixes the following word.
					tokens = append(tokens, pieces[:len(pieces)-1]...)
					tokens = append(tokens, last+text[start:end])
					prevEnd = end
					continue
				}
				tokens = append(tokens, pieces...)
			}
		}

		tokens = append(tokens, text[start:end])
		prevEnd = end
	}

	if run := text[prevEnd:]; run != "" {
		if isAllWhitespace(run) {
			tokens[len(tokens)-1] += run
		} else {
			tokens = append(tokens, splitNonWord(run)...)
		}
	}

	return tokens
}

// splitNonWord breaks a non-word run into maximal whitespace runs and
// individual punctuation characters.
func splitNonWord(run string) []string {
	var pieces []string
	wsStart := -1

	appendWS := func(end int) {
		if wsStart != -1 {
			pieces = append(pieces, run[wsStart:end])
			wsStart = -1
		}
	}

	for i := 0; i < len(run); {
		r, size := utf8.DecodeRuneInString(run[i:])
		if unicode.IsSpace(r) {
			if wsStart == -1 {
				wsStart = i
			}
		} else {
			appendWS(i)
			pieces = append(pieces, run[i:i+size])
		}
		i += size
	}
	appendWS(len(run))

	return pieces
}

func isAllWhitespace(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// CountTokens returns the number of tokens in text.
func (t *SimpleTokenizer) CountTokens(text string) int {
	return len(t.split(text))
}

// Encode assigns a stable id to each distinct token (first-seen order).
func (t *SimpleTokenizer) Encode(text string) ([]int, error) {
	raw := t.split(text)
	ids := make([]int, len(raw))

	t.mu.Lock()
	defer t.mu.Unlock()

	for i, tok := range raw {
		id, ok := t.ids[tok]
		if !ok {
			id = len(t.tokens)
			t.ids[tok] = id
			t.tokens = append(t.tokens, tok)
		}
		ids[i] = id
	}

	return ids, nil
}

// Decode reconstructs the original text from previously encoded ids.
func (t *SimpleTokenizer) Decode(tokens []int) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var b strings.Builder
	for _, id := range tokens {
		if id < 0 || id >= len(t.tokens) {
			return "", ErrUnknownTokenID
		}
		b.WriteString(t.tokens[id])
	}

	return b.String(), nil
}
