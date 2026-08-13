package rag

import (
	"context"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/infra/tokenization"
)

// BlockKind classifies a parsed block so the sizing engine knows whether it can
// be broken up (paragraph) or must be kept as a single unit.
type BlockKind int

const (
	// BlockKindParagraph is a regular splittable paragraph or list.
	BlockKindParagraph BlockKind = iota
	// BlockKindCode is a fenced/indented code block.
	BlockKindCode
	// BlockKindTable is a GFM table.
	BlockKindTable
)

// Block is a single content unit under a heading section.
type Block struct {
	Text       string
	Kind       BlockKind
	TokenCount int
}

// FinalChunk is the output of the sizing engine: content with heading context
// prepended, ready to be persisted as a domain.Chunk.
type FinalChunk struct {
	HeadingPath []string
	Content     string
	RawText     string
	TokenCount  int
}

// ChunkSections applies the 128-512 token boundary rules to parsed sections:
// small sections are merged with adjacent siblings, oversized sections are
// split at paragraph boundaries and, if needed, at sentence boundaries with
// token overlap. Atomic code blocks and tables are preserved.
func (s *Service) ChunkSections(ctx context.Context, sections []RawChunk) ([]FinalChunk, error) {
	merged := mergeSmallSections(sections, s.tokenizer)

	chunks := make([]FinalChunk, 0, len(merged))
	for _, section := range merged {
		headingPath := renderHeadingPath(section.Headings)

		// Reserve tokens for the heading context so the final content still
		// fits within the maximum window.
		contextTokens := s.tokenizer.CountTokens(strings.Join(headingPath, " > "))
		budget := TargetMaxChunkTokens - contextTokens
		if budget < 1 {
			budget = 1
		}

		for _, subBlocks := range splitSection(section, budget, s.tokenizer) {
			body := joinBlocks(subBlocks)
			content := prependContext(headingPath, body)
			chunks = append(chunks, FinalChunk{
				HeadingPath: headingPath,
				Content:     content,
				RawText:     body,
				TokenCount:  s.tokenizer.CountTokens(content),
			})
		}
	}

	return chunks, nil
}

// mergeSmallSections merges sections under TargetMinChunkTokens with an
// adjacent sibling section sharing the same parent heading, provided the
// combined token count stays within the maximum budget. The merged section
// keeps the common parent heading path.
func mergeSmallSections(sections []RawChunk, tk tokenization.Tokenizer) []RawChunk {
	for {
		merged := false

		for i := 0; i < len(sections); i++ {
			if sections[i].TokenCount >= TargetMinChunkTokens {
				continue
			}

			// Try merging with the next sibling.
			if i+1 < len(sections) && isSibling(sections[i], sections[i+1]) &&
				combinedFits(sections[i], sections[i+1], tk) {
				sections[i] = mergeInto(sections[i], sections[i+1])
				sections = append(sections[:i+1], sections[i+2:]...)
				merged = true
				break
			}

			// Try merging with the previous sibling.
			if i > 0 && isSibling(sections[i-1], sections[i]) &&
				combinedFits(sections[i-1], sections[i], tk) {
				sections[i-1] = mergeInto(sections[i-1], sections[i])
				sections = append(sections[:i], sections[i+1:]...)
				merged = true
				break
			}
		}

		if !merged {
			break
		}
	}

	return sections
}

// combinedFits reports whether two sibling sections can merge into one chunk
// without exceeding the maximum token budget (reserving heading-context tokens).
func combinedFits(a, b RawChunk, tk tokenization.Tokenizer) bool {
	parent := parentPath(a.Headings)
	contextTokens := tk.CountTokens(strings.Join(renderHeadingPath(parent), " > "))
	budget := TargetMaxChunkTokens - contextTokens
	return a.TokenCount+b.TokenCount <= budget
}

// mergeInto combines two sibling sections under their common parent heading.
func mergeInto(a, b RawChunk) RawChunk {
	a.Blocks = append(a.Blocks, b.Blocks...)
	a.TokenCount += b.TokenCount
	a.Headings = parentPath(a.Headings)
	return a
}

// isSibling reports whether two sections sit directly under the same parent
// heading (same depth and identical parent path).
func isSibling(a, b RawChunk) bool {
	if len(a.Headings) != len(b.Headings) {
		return false
	}
	pa, pb := parentPath(a.Headings), parentPath(b.Headings)
	if len(pa) != len(pb) {
		return false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			return false
		}
	}
	return true
}

func parentPath(headings []RawChunkHeading) []RawChunkHeading {
	if len(headings) <= 1 {
		return []RawChunkHeading{}
	}
	return headings[:len(headings)-1]
}

// splitSection splits a section's blocks into sub-groups of at most budget
// tokens. Paragraph boundaries are the primary split points; a single block
// exceeding the budget is split at sentence boundaries with token overlap.
// Tables are always kept atomic.
func splitSection(section RawChunk, budget int, tk tokenization.Tokenizer) [][]Block {
	var result [][]Block
	var current []Block
	currentTokens := 0

	flush := func() {
		if len(current) > 0 {
			result = append(result, current)
			current = nil
			currentTokens = 0
		}
	}

	for _, block := range section.Blocks {
		if block.TokenCount <= budget {
			if currentTokens > 0 && currentTokens+block.TokenCount > budget {
				flush()
			}
			current = append(current, block)
			currentTokens += block.TokenCount
			continue
		}

		// A single block exceeds the budget.
		flush()

		if block.Kind == BlockKindTable {
			// Tables stay atomic even when oversized.
			current = append(current, block)
			flush()
			continue
		}

		// Paragraphs and code blocks are split at sentence boundaries with overlap.
		for _, piece := range sentenceWindows(block.Text, budget, ChunkOverlapTokens, tk) {
			current = append(current, Block{
				Text:       piece,
				Kind:       block.Kind,
				TokenCount: tk.CountTokens(piece),
			})
			flush()
		}
	}

	flush()
	return result
}

// sentenceWindows builds token windows of at most maxTokens from the sentences
// of text, overlapping consecutive windows by roughly overlap tokens.
func sentenceWindows(text string, maxTokens, overlap int, tk tokenization.Tokenizer) []string {
	sentences := SplitSentences(text)
	if len(sentences) <= 1 {
		return []string{text}
	}

	var windows []string
	start := 0

	for start < len(sentences) {
		window := []string{}
		tokens := 0
		lastIdx := start

		for i := start; i < len(sentences); i++ {
			st := tk.CountTokens(sentences[i])
			if len(window) > 0 && tokens+st > maxTokens {
				break
			}
			window = append(window, sentences[i])
			tokens += st
			lastIdx = i
		}

		windows = append(windows, strings.Join(window, " "))

		if lastIdx >= len(sentences)-1 {
			break
		}

		// Walk back from lastIdx until ~overlap tokens are covered to start the
		// next window, guaranteeing forward progress.
		next := lastIdx + 1
		backTokens := 0
		for k := lastIdx; k >= start && backTokens < overlap; k-- {
			backTokens += tk.CountTokens(sentences[k])
			next = k
		}
		if next <= start {
			next = lastIdx + 1
		}
		start = next
	}

	return windows
}

// SplitSentences breaks text on punctuation followed by whitespace, keeping the
// punctuation with its sentence.
func SplitSentences(text string) []string {
	var sentences []string
	start := 0

	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '.' && c != '!' && c != '?' {
			continue
		}

		j := i + 1
		for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n') {
			j++
		}
		if j == i+1 {
			// No whitespace follows the punctuation: not a sentence boundary.
			continue
		}

		sentences = append(sentences, text[start:i+1])
		start = j
		i = j - 1
	}

	if start < len(text) {
		sentences = append(sentences, text[start:])
	}

	return sentences
}

func renderHeadingPath(headings []RawChunkHeading) []string {
	path := make([]string, 0, len(headings))
	for _, h := range headings {
		if h.Text == "" {
			continue
		}
		path = append(path, strings.Repeat("#", h.Level)+" "+h.Text)
	}
	return path
}

func prependContext(headingPath []string, body string) string {
	if len(headingPath) == 0 {
		return body
	}
	return strings.Join(headingPath, " > ") + "\n\n" + body
}

func joinBlocks(blocks []Block) string {
	texts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		texts = append(texts, b.Text)
	}
	return strings.Join(texts, "\n\n")
}
