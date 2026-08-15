package file_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/file"
	"github.com/anditakaesar/uwa-go-rag/internal/file/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var allowedTypesTest = map[string]bool{
	"image/png": true,
}

func TestNewFileService(test *testing.T) {
	test.Run("success", func(t *testing.T) {
		got := file.NewService(file.ServiceDependency{
			DirName:      "uploadDir",
			AllowedTypes: allowedTypesTest,
		})
		assert.Equal(t, reflect.TypeFor[*file.Service](), reflect.TypeOf(got))
	})
}

func TestFileService_Save(test *testing.T) {
	tmpDir := test.TempDir()
	svc := file.NewService(file.ServiceDependency{
		DirName:      tmpDir,
		AllowedTypes: allowedTypesTest,
	})

	test.Run("success", func(t *testing.T) {
		// Create dummy PNG data (PNG header + padding)
		content := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("a"), 600)...)
		reader := bytes.NewReader(content)
		filename := "my photo!!.png"

		newName, err := svc.Save(filename, reader)

		// Assertions
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Check if file actually exists
		path := filepath.Join(tmpDir, newName)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("file was not actually created on disk")
		}

		// Check if name was sanitized (no spaces or !!)
		if strings.Contains(newName, " ") || strings.Contains(newName, "!!") {
			t.Errorf("filename was not sanitized: %s", newName)
		}

		// Verify content integrity (The MultiReader check)
		savedContent, _ := os.ReadFile(path)
		if !bytes.Equal(content, savedContent) {
			t.Error("saved content does not match original; check MultiReader logic")
		}
	})

	test.Run("success save invalid name", func(t *testing.T) {
		// Create dummy PNG data (PNG header + padding)
		content := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("a"), 600)...)
		reader := bytes.NewReader(content)
		filename := "!!!.png"

		newName, err := svc.Save(filename, reader)

		// Assertions
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Check if file actually exists
		path := filepath.Join(tmpDir, newName)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("file was not actually created on disk")
		}

		// Check if name was sanitized (no spaces or !!)
		if strings.Contains(newName, " ") || strings.Contains(newName, "!!") {
			t.Errorf("filename was not sanitized: %s", newName)
		}

		// Verify content integrity (The MultiReader check)
		savedContent, _ := os.ReadFile(path)
		if !bytes.Equal(content, savedContent) {
			t.Error("saved content does not match original; check MultiReader logic")
		}
	})

	test.Run("success save too long name", func(t *testing.T) {
		content := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("a"), 600)...)
		reader := bytes.NewReader(content)
		filename := "abcdefghijklmnopqrstuvwxyz1234abcdefghijklmnopqrstuvwxyz1234abcdefghijklmnopqrstuvwxyz1234abcdefghijklmnopqrstuvwxyz1234.png"

		newName, err := svc.Save(filename, reader)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		path := filepath.Join(tmpDir, newName)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("file was not actually created on disk")
		}

		if strings.Contains(newName, " ") || strings.Contains(newName, "!!") {
			t.Errorf("filename was not sanitized: %s", newName)
		}

		savedContent, _ := os.ReadFile(path)
		if !bytes.Equal(content, savedContent) {
			t.Error("saved content does not match original; check MultiReader logic")
		}
	})

	test.Run("disallowed file type", func(t *testing.T) {
		content := []byte("<?xml version=\"1.0\"?><svg></svg>") // Detects as image/svg+xml or text/xml
		reader := bytes.NewReader(content)

		_, err := svc.Save("malicious.svg", reader)
		if err == nil || !strings.Contains(err.Error(), "file type not allowed") {
			t.Errorf("expected 'file type not allowed' error, got %v", err)
		}
	})

}

func TestFileService_SetEmbeddingStatus(test *testing.T) {
	fileID := uuid.Must(uuid.NewV7())

	status := domain.EMBEDDING_STATUS_COMPLETED

	test.Run("success", func(t *testing.T) {
		fileRepo := mocks.NewMockFileRepository(t)
		fileRepo.EXPECT().Update(mock.Anything, fileID, domain.UpdateFileParam{
			EmbeddingStatus: &status,
		}).Return(&domain.File{}, nil)

		svc := file.NewService(file.ServiceDependency{
			DirName:      "uploadDir",
			AllowedTypes: allowedTypesTest,
			FileRepo:     fileRepo,
		})

		err := svc.SetEmbeddingStatus(context.Background(), fileID, status)

		assert.NoError(t, err)
	})

	test.Run("error", func(t *testing.T) {
		fileRepo := mocks.NewMockFileRepository(t)
		fileRepo.EXPECT().Update(mock.Anything, fileID, domain.UpdateFileParam{
			EmbeddingStatus: &status,
		}).Return(nil, errors.New("update_error"))

		svc := file.NewService(file.ServiceDependency{
			DirName:      "uploadDir",
			AllowedTypes: allowedTypesTest,
			FileRepo:     fileRepo,
		})

		err := svc.SetEmbeddingStatus(context.Background(), fileID, status)

		assert.Error(t, err)
	})
}

func TestFileService_SetStatus(test *testing.T) {
	fileID := uuid.Must(uuid.NewV7())

	status := domain.UPLOAD_STATUS_COMPLETED

	test.Run("success", func(t *testing.T) {
		fileRepo := mocks.NewMockFileRepository(t)
		fileRepo.EXPECT().Update(mock.Anything, fileID, domain.UpdateFileParam{
			Status: &status,
		}).Return(&domain.File{}, nil)

		svc := file.NewService(file.ServiceDependency{
			DirName:      "uploadDir",
			AllowedTypes: allowedTypesTest,
			FileRepo:     fileRepo,
		})

		err := svc.SetStatus(context.Background(), fileID, status)

		assert.NoError(t, err)
	})

	test.Run("error", func(t *testing.T) {
		fileRepo := mocks.NewMockFileRepository(t)
		fileRepo.EXPECT().Update(mock.Anything, fileID, domain.UpdateFileParam{
			Status: &status,
		}).Return(nil, errors.New("update_error"))

		svc := file.NewService(file.ServiceDependency{
			DirName:      "uploadDir",
			AllowedTypes: allowedTypesTest,
			FileRepo:     fileRepo,
		})

		err := svc.SetStatus(context.Background(), fileID, status)

		assert.Error(t, err)
	})
}
