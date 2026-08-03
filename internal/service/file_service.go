package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/google/uuid"
)

type IFileService interface {
	Save(filename string, content io.Reader) (string, error)
	ListFiles(ctx context.Context) ([]string, error)
	GeneratePresignURL(ctx context.Context, param domain.GeneratePresignURLParam) (*domain.GeneratePresignURLReturn, error)
}

type FileService struct {
	uploadDir     string
	allowedTypes  map[string]bool
	storageClient IStorageClient
	fileRepo      IFileRepository
	uow           IUnitOfWork
}

type FileServiceDep struct {
	DirName       string
	AllowedTypes  map[string]bool
	StorageClient IStorageClient
	FileRepo      IFileRepository
	UOW           IUnitOfWork
}

func NewFileService(dep FileServiceDep) *FileService {
	return &FileService{
		uploadDir:     dep.DirName,
		allowedTypes:  dep.AllowedTypes,
		storageClient: dep.StorageClient,
		fileRepo:      dep.FileRepo,
		uow:           dep.UOW,
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

func (svc *FileService) ListFiles(ctx context.Context) ([]string, error) {
	return svc.storageClient.ListFiles(ctx)
}

func (svc *FileService) GeneratePresignURL(ctx context.Context, param domain.GeneratePresignURLParam) (*domain.GeneratePresignURLReturn, error) {
	identity := ctx.Value(domain.IdentityKey).(domain.Identity)
	cleanFilename := sanitizeFilename(param.Name)
	extensionFile := filepath.Ext(param.Name)
	newID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	var result domain.GeneratePresignURLReturn

	presignUrlErr := svc.uow.Do(ctx, func(txCtx context.Context) error {
		newFile, err := svc.fileRepo.Insert(ctx, domain.File{
			ID:           newID,
			UserID:       identity.UserID,
			OriginalName: cleanFilename,
			MimeType:     param.MimeType,
			SizeBytes:    param.SizeBytes,
			S3Bucket:     env.S3Conf.S3Bucket,
			S3Key:        path.Join(env.S3Conf.S3Prefix, fmt.Sprintf("%v%v", newID.String(), extensionFile)),
			Status:       domain.UPLOAD_STATUS_PENDING,
		})
		if err != nil {
			return err
		}

		presignUrl, err := svc.storageClient.GetPresignURL(ctx, newFile.S3Key)
		if err != nil {
			return err
		}

		result.File = *newFile
		result.PresignURL = presignUrl

		return nil
	})

	if presignUrlErr != nil {
		return nil, presignUrlErr
	}

	return &result, nil
}
