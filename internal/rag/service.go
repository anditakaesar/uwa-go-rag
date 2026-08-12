package rag

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Service struct{}

func NewRagService() *Service {
	return &Service{}
}

func (s *Service) ProcessDocument(ctx context.Context, ragFileID int64) error {
	xlog.Logger.Info(fmt.Sprintf("processing file with id: %d", ragFileID))
	return nil
}

type RawChunkHeading struct {
	Text  string
	Level int
}

type RawChunk struct {
	Headings []RawChunkHeading
	Texts    []string
}

const (
	TargetMinChunkSize = 200
	TargetMaxChunkSize = 1200
)

func (s *Service) BuildChunks(ctx context.Context, source []byte) ([]RawChunk, error) {
	mdParser := goldmark.DefaultParser()
	reader := text.NewReader(source)
	doc := mdParser.Parse(reader)

	result := make([]RawChunk, 0)

	var (
		headingStack []RawChunkHeading
		currentTexts []string
		currentLen   int
	)

	// Flushes the current accumulated text into a RawChunk
	flushChunk := func() {
		if len(currentTexts) == 0 {
			return
		}

		// Preserve current headings context by making a deep copy
		headingsCopy := make([]RawChunkHeading, len(headingStack))
		copy(headingsCopy, headingStack)

		result = append(result, RawChunk{
			Headings: headingsCopy,
			Texts:    currentTexts,
		})

		currentTexts = make([]string, 0)
		currentLen = 0
	}

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			headingText := strings.TrimSpace(extractText(node, source))

			// Skip empty headings
			if headingText == "" {
				return ast.WalkSkipChildren, nil
			}

			level := node.Level

			// Trim heading stack to contain only parent headings
			// Example: current stack [H1, H2, H3], new heading H2
			// Keep H1 (level-1=1 elements), trim rest -> [H1], then append H2 -> [H1, H2]
			if level <= len(headingStack) {
				headingStack = headingStack[:level-1]
			}

			// Fill gaps for skipped heading levels
			// Example: H1 exists, suddenly H4 appears -> need placeholders for H2, H3
			for len(headingStack) < level-1 {
				placeholderLevel := len(headingStack) + 1
				headingStack = append(headingStack, RawChunkHeading{
					Text:  "",
					Level: placeholderLevel,
				})
			}

			headingStack = append(headingStack, RawChunkHeading{
				Text:  headingText,
				Level: level,
			})

			// Flush current chunk on heading change to maintain section boundaries
			if currentLen >= TargetMinChunkSize {
				flushChunk()
			}

			return ast.WalkSkipChildren, nil

		case *ast.Paragraph, *ast.FencedCodeBlock, *ast.CodeBlock, *ast.List:
			blockText := strings.TrimSpace(extractText(node, source))
			if len(blockText) == 0 {
				return ast.WalkSkipChildren, nil
			}

			blockSize := len(blockText)

			// If adding this block exceeds target length and we met minimum chunk size, flush first
			if currentLen+blockSize > TargetMaxChunkSize && currentLen >= TargetMinChunkSize {
				flushChunk()
			}

			currentTexts = append(currentTexts, blockText)
			currentLen += blockSize

			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, err
	}

	// Flush any remaining accumulated text
	flushChunk()

	return result, nil
}

// Helper to extract clean text from AST nodes and their lines
func extractText(n ast.Node, source []byte) string {
	var buf bytes.Buffer

	// Extract inline children text (e.g., formatted text inside paragraphs/headings)
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if textNode, ok := child.(*ast.Text); ok {
			buf.Write(textNode.Segment.Value(source))
		} else if child.HasChildren() {
			buf.WriteString(extractText(child, source))
		}
	}

	// Fallback to raw node lines if inline parsing yields nothing (e.g., Code Blocks)
	if buf.Len() == 0 && n.Lines().Len() > 0 {
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			buf.Write(line.Value(source))
		}
	}

	return buf.String()
}

func (rc RawChunk) String() string {
	var buf bytes.Buffer

	// Write heading hierarchy with proper levels
	for _, heading := range rc.Headings {
		if heading.Text != "" {
			// Create markdown heading with appropriate number of # symbols
			prefix := strings.Repeat("#", heading.Level)
			buf.WriteString(prefix)
			buf.WriteString(" ")
			buf.WriteString(heading.Text)
			buf.WriteString("\n")
		}
	}

	// Add blank line between headings and content if there are headings
	hasHeadings := false
	for _, h := range rc.Headings {
		if h.Text != "" {
			hasHeadings = true
			break
		}
	}
	if hasHeadings {
		buf.WriteString("\n")
	}

	// Write text blocks
	for i, text := range rc.Texts {
		buf.WriteString(text)
		// Add spacing between blocks
		if i < len(rc.Texts)-1 {
			buf.WriteString("\n\n")
		}
	}

	return buf.String()
}
