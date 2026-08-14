package rag

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/tokenization"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

type ServiceDependency struct {
	Tokenizer tokenization.Tokenizer
	JobQueue  JobQueue
	FileRepo  FileRepository
}

type Service struct {
	tokenizer tokenization.Tokenizer
	queue     JobQueue
	fileRepo  FileRepository
}

func NewRagService(dep ServiceDependency) *Service {
	return &Service{
		tokenizer: dep.Tokenizer,
		queue:     dep.JobQueue,
		fileRepo:  dep.FileRepo,
	}
}

const (
	TargetMinChunkTokens = 128
	TargetMaxChunkTokens = 512
	ChunkOverlapTokens   = 32
)

type RawChunkHeading struct {
	Text  string
	Level int
}

// RawChunk is a structural section from the AST parse: the blocks following a
// heading, together with the heading hierarchy they belong to.
type RawChunk struct {
	Headings   []RawChunkHeading
	Blocks     []Block
	TokenCount int
}

// BuildChunks parses markdown into heading sections and sizes them into final
// chunks following the token boundary rules. Convenience wrapper around
// ParseSections + ChunkSections.
func (s *Service) BuildChunks(ctx context.Context, source []byte) ([]FinalChunk, error) {
	sections, err := s.ParseSections(ctx, source)
	if err != nil {
		return nil, err
	}
	return s.ChunkSections(ctx, sections)
}

// ParseSections walks the markdown AST and groups content into one section per
// heading. Content before the first heading forms a section with an empty
// heading path. Token counts are computed per block.
func (s *Service) ParseSections(ctx context.Context, source []byte) ([]RawChunk, error) {
	mdParser := goldmark.New(goldmark.WithExtensions(extension.Table)).Parser()
	reader := text.NewReader(source)
	doc := mdParser.Parse(reader)

	result := make([]RawChunk, 0)

	var (
		headingStack  []RawChunkHeading
		currentBlocks []Block
		currentLen    int
	)

	flushSection := func() {
		if len(currentBlocks) == 0 {
			return
		}

		headingsCopy := make([]RawChunkHeading, len(headingStack))
		copy(headingsCopy, headingStack)

		result = append(result, RawChunk{
			Headings:   headingsCopy,
			Blocks:     currentBlocks,
			TokenCount: currentLen,
		})

		currentBlocks = nil
		currentLen = 0
	}

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			headingText := strings.TrimSpace(extractText(node, source))
			if headingText == "" {
				return ast.WalkSkipChildren, nil
			}

			// Start a new section: flush content accumulated under the previous
			// heading BEFORE updating the stack, so each section keeps its own
			// heading context.
			flushSection()

			level := node.Level

			if level <= len(headingStack) {
				headingStack = headingStack[:level-1]
			}

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

			return ast.WalkSkipChildren, nil

		case *ast.Paragraph, *ast.FencedCodeBlock, *ast.CodeBlock, *ast.List, *gast.Table:
			blockText := strings.TrimSpace(extractText(node, source))
			if len(blockText) == 0 {
				return ast.WalkSkipChildren, nil
			}

			currentBlocks = append(currentBlocks, Block{
				Text:       blockText,
				Kind:       blockKind(node),
				TokenCount: s.tokenizer.CountTokens(blockText),
			})
			currentLen += s.tokenizer.CountTokens(blockText)

			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	flushSection()

	return result, nil
}

func blockKind(n ast.Node) BlockKind {
	switch n.(type) {
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		return BlockKindCode
	case *gast.Table:
		return BlockKindTable
	default:
		return BlockKindParagraph
	}
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

func (s *Service) ProcessDocForEmbedding(ctx context.Context, fileID uuid.UUID) error {
	user, ok := domain.UserFromContext(ctx)
	if !ok {
		return &xerror.ErrorPermission{Message: "user not found in context"}
	}

	file, err := s.fileRepo.Get(ctx, fileID)
	if err != nil {
		return err
	}

	err = s.queue.EnqueueRagFile(ctx, file.ID, file.S3Key)
	if err != nil {
		return err
	}

	queueErr := s.queue.EnqueueAuditLog(ctx, domain.AuditLog{
		ResourceName: "files",
		ResourceID:   fmt.Sprint(file.ID),
		Action:       domain.FILE_PROCESS_RAG,
		ActorName:    user.Username,
		ActorID:      &user.ID,
		CreatedAt:    time.Now(),
	})
	if queueErr != nil {
		xlog.Logger.Error("error while enqueue job", "queueErr", queueErr)
	}

	return nil
}
