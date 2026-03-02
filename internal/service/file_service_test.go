package service_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/stretchr/testify/assert"
)

var allowedTypesTest = map[string]bool{
	"image/png": true,
}

func TestNewFileService(test *testing.T) {
	test.Run("success", func(t *testing.T) {
		got := service.NewFileService("uploadDir", allowedTypesTest)
		assert.Equal(t, reflect.TypeFor[*service.FileService](), reflect.TypeOf(got))
	})
}

func TestFileService_Save(test *testing.T) {
	tmpDir := test.TempDir()
	svc := service.NewFileService(tmpDir, allowedTypesTest)

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
