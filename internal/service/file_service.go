package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type IFileService interface {
	Save(filename string, content io.Reader) (string, error)
}

type FileService struct {
	uploadDir    string
	allowedTypes map[string]bool
}

func NewFileService(dir string, allowedTypes map[string]bool) *FileService {
	return &FileService{
		uploadDir:    dir,
		allowedTypes: allowedTypes,
	}
}

func (svc *FileService) Save(filename string, file io.Reader) (string, error) {
	buff := make([]byte, 512)
	n, err := file.Read(buff)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read header: %w", err)
	}

	contentType := http.DetectContentType(buff[:n])
	if !svc.allowedTypes[contentType] {
		return "", errors.New("file type not allowed: " + contentType)
	}

	safeBase := sanitizeFilename(filename)
	newName := fmt.Sprintf("%d_%s", time.Now().Unix(), safeBase)
	dstPath := filepath.Join(svc.uploadDir, newName)

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	fullReader := io.MultiReader(bytes.NewReader(buff[:n]), file)
	_, err = io.Copy(dst, fullReader)
	if err != nil {
		return "", fmt.Errorf("failed to save content: %w", err)
	}

	return newName, nil
}

func sanitizeFilename(filename string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	safeName := reg.ReplaceAllString(name, "_")

	safeName = strings.Trim(safeName, "_")
	if len(safeName) > 100 {
		safeName = safeName[:100]
	}

	if safeName == "" {
		safeName = "uploaded_file"
	}

	return safeName + strings.ToLower(ext)
}
