package rag_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/infra/tokenization"
	"github.com/anditakaesar/uwa-go-rag/internal/rag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildChunks(t *testing.T, md string) []rag.FinalChunk {
	t.Helper()

	tk := tokenization.NewSimpleTokenizer()
	svc := rag.NewRagService(rag.ServiceDependency{Tokenizer: tk})

	chunks, err := svc.BuildChunks(context.Background(), []byte(md))
	require.NoError(t, err)

	return chunks
}

func parseSections(t *testing.T, md string) []rag.RawChunk {
	t.Helper()

	tk := tokenization.NewSimpleTokenizer()
	svc := rag.NewRagService(rag.ServiceDependency{Tokenizer: tk})

	sections, err := svc.ParseSections(context.Background(), []byte(md))
	require.NoError(t, err)

	return sections
}

func countTokens(text string) int {
	return tokenization.NewSimpleTokenizer().CountTokens(text)
}

func TestBuildChunks_EmptySource(t *testing.T) {
	t.Parallel()

	assert.Empty(t, buildChunks(t, ""))
}

func TestParseSections_SectionsPerHeading(t *testing.T) {
	t.Parallel()

	md := "# H1\n\nbody one\n\n## H2\n\nbody two\n\n# H3\n\nbody three"

	sections := parseSections(t, md)

	require.Len(t, sections, 3)
	assert.Equal(t, []rag.RawChunkHeading{{Text: "H1", Level: 1}}, sections[0].Headings)
	assert.Equal(t, []rag.RawChunkHeading{{Text: "H1", Level: 1}, {Text: "H2", Level: 2}}, sections[1].Headings)
	assert.Equal(t, []rag.RawChunkHeading{{Text: "H3", Level: 1}}, sections[2].Headings)
	assert.Len(t, sections[0].Blocks, 1)
	assert.Equal(t, "body one", sections[0].Blocks[0].Text)
	assert.Equal(t, countTokens("body one"), sections[0].TokenCount)
}

func TestBuildChunks_HeadingContext(t *testing.T) {
	t.Parallel()

	// Each section body exceeds TargetMinChunkTokens so sections are not merged.
	overview := strings.TrimSpace(strings.Repeat("overview content ", 150))
	deepDive := strings.TrimSpace(strings.Repeat("deep dive content ", 150))
	anotherTopic := strings.TrimSpace(strings.Repeat("another topic content ", 150))

	md := "# Overview\n" + overview + "\n\n## Deep Dive\n" + deepDive + "\n\n# Another Topic\n" + anotherTopic

	chunks := buildChunks(t, md)

	require.Len(t, chunks, 3)

	assert.Equal(t, []string{"# Overview"}, chunks[0].HeadingPath)
	assert.Equal(t, []string{"# Overview", "## Deep Dive"}, chunks[1].HeadingPath)
	assert.Equal(t, []string{"# Another Topic"}, chunks[2].HeadingPath)

	// Heading context is prepended to the chunk content.
	assert.True(t, strings.HasPrefix(chunks[1].Content, "# Overview > ## Deep Dive\n\n"), chunks[1].Content)
	assert.Equal(t, chunks[1].RawText, strings.TrimPrefix(chunks[1].Content, "# Overview > ## Deep Dive\n\n"))
}

func TestBuildChunks_TokenBasedSizing(t *testing.T) {
	t.Parallel()

	// Three paragraphs of 200 tokens each under one heading. The section totals
	// 600 tokens (> 512) so it splits at paragraph boundaries into two chunks:
	// [p1, p2] and [p3].
	paragraph := strings.TrimSpace(strings.Repeat("word ", 200))
	md := "# Title\n\n" + paragraph + "\n\n" + paragraph + "\n\n" + paragraph

	chunks := buildChunks(t, md)

	require.Len(t, chunks, 2)
	assert.Equal(t, 2, countParagraphs(chunks[0].RawText))
	assert.Equal(t, 1, countParagraphs(chunks[1].RawText))

	for _, c := range chunks {
		assert.LessOrEqual(t, c.TokenCount, rag.TargetMaxChunkTokens)
		assert.GreaterOrEqual(t, c.TokenCount, rag.TargetMinChunkTokens)
		assert.Equal(t, countTokens(c.Content), c.TokenCount)
	}
}

func TestChunkSections_SplitOversizedParagraphOverlap(t *testing.T) {
	t.Parallel()

	// 20 sentences of ~59 tokens each (> 512 total). The paragraph must be
	// split into overlapping token windows.
	sentences := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		sentences = append(sentences, fmt.Sprintf("s%d %s", i, strings.TrimSpace(strings.Repeat("word ", 58))))
	}
	md := strings.Join(sentences, ". ")

	chunks := buildChunks(t, md)

	require.GreaterOrEqual(t, len(chunks), 2)

	for _, c := range chunks {
		assert.LessOrEqual(t, c.TokenCount, rag.TargetMaxChunkTokens)
	}

	// Consecutive windows overlap by at least one sentence.
	markerRe := regexp.MustCompile(`s\d+`)
	markers := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, tok := range markerRe.FindAllString(s, -1) {
			m[tok] = true
		}
		return m
	}

	for i := 0; i < len(chunks)-1; i++ {
		a := markers(chunks[i].RawText)
		b := markers(chunks[i+1].RawText)
		shared := false
		for k := range a {
			if b[k] {
				shared = true
				break
			}
		}
		assert.True(t, shared, "window %d and %d do not overlap", i, i+1)
	}
}

func TestChunkSections_MergeSmallSiblings(t *testing.T) {
	t.Parallel()

	// Two sibling H2 sections, each well under 128 tokens, must merge into a
	// single chunk under the common parent heading.
	md := "# H1\n\n## A\n\nalpha tiny section\n\n## B\n\nbeta tiny section"

	chunks := buildChunks(t, md)

	require.Len(t, chunks, 1)
	assert.Equal(t, []string{"# H1"}, chunks[0].HeadingPath)
	assert.Contains(t, chunks[0].Content, "alpha tiny section")
	assert.Contains(t, chunks[0].Content, "beta tiny section")
}

func TestChunkSections_UnmergeableSmallSection(t *testing.T) {
	t.Parallel()

	// A small section with no adjacent sibling is kept as its own chunk.
	md := "# Solo\n\njust a tiny bit of content"

	chunks := buildChunks(t, md)

	require.Len(t, chunks, 1)
	assert.Equal(t, []string{"# Solo"}, chunks[0].HeadingPath)
	assert.Contains(t, chunks[0].Content, "just a tiny bit of content")
}

func TestBuildChunks_CodeBlockAtomic(t *testing.T) {
	t.Parallel()

	md := `
# Code

` + "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```" + `

after the code block
`

	chunks := buildChunks(t, md)

	require.Len(t, chunks, 1)
	assert.Contains(t, chunks[0].Content, "func main()")
	assert.Contains(t, chunks[0].Content, "fmt.Println")
	assert.Contains(t, chunks[0].Content, "after the code block")
}

func TestBuildChunks_TableAtomic(t *testing.T) {
	t.Parallel()

	md := `
# Table Test

| Name | Value |
| ---- | ----- |
| alpha | 1 |
| beta | 2 |
`

	chunks := buildChunks(t, md)

	require.Len(t, chunks, 1)
	for _, cell := range []string{"Name", "Value", "alpha", "1", "beta", "2"} {
		assert.Contains(t, chunks[0].Content, cell)
	}
}

func TestSplitSentences(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  []string
	}{
		{"Hello world. Next one. Final", []string{"Hello world.", "Next one.", "Final"}},
		{"no punctuation here", []string{"no punctuation here"}},
		{"", nil},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.want, rag.SplitSentences(tc.input))
	}
}

func countParagraphs(text string) int {
	return strings.Count(text, "\n\n") + 1
}
