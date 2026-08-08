package file

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

	"github.com/anditakaesar/uwa-go-rag/internal/application"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/google/uuid"
)

type PasswordChecker interface {
	HashPassword(password string) (string, error)
	CheckPassword(password string, hash string) (bool, error)
}

type StorageClient interface {
	GetPresignPutURL(ctx context.Context, key string) (string, error)
	GetPresignGetURL(ctx context.Context, key string) (string, error)
}

type FileRepository interface {
	Insert(ctx context.Context, newFile domain.File) (*domain.File, error)
	Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
	FindAll(ctx context.Context, param *domain.FindAllFilesParam) ([]domain.File, error)
	Update(ctx context.Context, id uuid.UUID, updateParam domain.UpdateFileParam) (*domain.File, error)
}

type Service struct {
	uploadDir     string
	allowedTypes  map[string]bool
	storageClient StorageClient
	fileRepo      FileRepository
	uow           application.UnitOfWork
}

type ServiceDependency struct {
	DirName       string
	AllowedTypes  map[string]bool
	StorageClient StorageClient
	FileRepo      FileRepository
	UOW           application.UnitOfWork
}

func NewService(dep ServiceDependency) *Service {
	return &Service{
		uploadDir:     dep.DirName,
		allowedTypes:  dep.AllowedTypes,
		storageClient: dep.StorageClient,
		fileRepo:      dep.FileRepo,
		uow:           dep.UOW,
	}
}

func (svc *Service) Save(filename string, file io.Reader) (string, error) {
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

func (svc *Service) GeneratePresignURL(ctx context.Context, param domain.GeneratePresignURLParam) (*domain.GeneratePresignURLReturn, error) {
	identity := ctx.Value(domain.IdentityKey).(domain.Identity)
	cleanFilename := sanitizeFilename(param.Name)
	extensionFile := filepath.Ext(param.Name)
	newID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	var result domain.GeneratePresignURLReturn

	presignUrlErr := svc.uow.Do(ctx, func(ctx context.Context) error {
		newFile, err := svc.fileRepo.Insert(ctx, domain.File{
			ID:           newID,
			UserID:       identity.UserID,
			OriginalName: cleanFilename,
			MimeType:     param.MimeType,
			SizeBytes:    param.SizeBytes,
			S3Bucket:     env.Get().S3Config.S3Bucket,
			S3Key:        path.Join(env.Get().S3Config.S3Prefix, fmt.Sprintf("%v%v", newID.String(), extensionFile)),
			Status:       domain.UPLOAD_STATUS_PENDING,
		})
		if err != nil {
			return err
		}

		presignUrl, err := svc.storageClient.GetPresignPutURL(ctx, newFile.S3Key)
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

func (svc *Service) GeneratePresignDownloadURL(ctx context.Context, fileID uuid.UUID) (string, error) {
	file, err := svc.fileRepo.Get(ctx, fileID)
	if err != nil {
		return "", err
	}

	presignGetURL, err := svc.storageClient.GetPresignGetURL(ctx, file.S3Key)
	if err != nil {
		return "", err
	}

	return presignGetURL, nil
}

func (svc *Service) FetchAll(ctx context.Context, param *domain.FindAllFilesParam) ([]domain.File, error) {
	param.Normalize()
	return svc.fileRepo.FindAll(ctx, param)
}

func (svc *Service) Update(ctx context.Context, id uuid.UUID, param domain.UpdateFileParam) (*domain.File, error) {
	return svc.fileRepo.Update(ctx, id, param)
}
