package file

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type FileService interface {
	Save(filename string, content io.Reader) (string, error)
	FetchAll(ctx context.Context, param *domain.FindAllFilesParam) ([]domain.File, error)
	GeneratePresignURL(ctx context.Context, param domain.GeneratePresignURLParam) (*domain.GeneratePresignURLReturn, error)
	GeneratePresignDownloadURL(ctx context.Context, fileID uuid.UUID) (string, error)
	Update(ctx context.Context, id uuid.UUID, param domain.UpdateFileParam) (*domain.File, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// dto
type SingleFileResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       int64     `json:"userID"`
	OriginalName string    `json:"originalName"`
	MimeType     string    `json:"mimeType"`
	SizeBytes    int64     `json:"sizeBytes"`
	SizeHumanize string    `json:"sizeHumanize"`
	Status       string    `json:"status"`
	ThumbnailURL string    `json:"thumbnailURL"`
	CreatedAt    time.Time `json:"createdAt"`
}

func FileDomainToResponse(data *domain.File) SingleFileResponse {
	return SingleFileResponse{
		ID:           data.ID,
		UserID:       data.UserID,
		OriginalName: data.OriginalName,
		MimeType:     data.MimeType,
		SizeBytes:    data.SizeBytes,
		SizeHumanize: data.SizeHumanize(),
		ThumbnailURL: fmt.Sprintf("%s/%s/%s", env.Get().S3Config.S3Endpoint, env.Get().S3Config.S3Bucket, data.GeneratePublicThumbnailKey()),
		Status:       string(data.Status),
		CreatedAt:    data.CreatedAt,
	}
}

func ListToResponse(data []domain.File) []SingleFileResponse {
	list := make([]SingleFileResponse, 0)
	for _, d := range data {
		r := FileDomainToResponse(&d)
		list = append(list, r)
	}

	return list
}

type UpdateStatusFileRequest struct {
	Status string `json:"status"`
}

func (r *UpdateStatusFileRequest) ToDomainParam() domain.UpdateFileParam {
	return domain.UpdateFileParam{
		Status: (*domain.UploadStatus)(&r.Status),
	}
}

func (r *UpdateStatusFileRequest) Validate() error {
	validStatus := []string{
		string(domain.UPLOAD_STATUS_COMPLETED),
		string(domain.UPLOAD_STATUS_FAILED),
	}

	if !slices.Contains(validStatus, r.Status) {
		return &xerror.ErrorValidation{Message: "status to update is not valid"}
	}

	return nil
}

// routers
func SetupFileApiRoutes(router chi.Router, h *FileApi) {
	endpoints := []handler.EndpointWithMiddleware{
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/files",
				Handler:    handler.MakeHandler(h.FetchFiles),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPatch,
				Path:       "/files/{uuid}/status",
				Handler:    handler.MakeHandler(h.UpdateStatus),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/files/{uuid}/download",
				Handler:    handler.MakeHandler(h.GetDownloadURL),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/files/generate-presign-url",
				Handler:    handler.MakeHandler(h.GeneratePresignURL),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/files/{uuid}/enqueue-thumbnail",
				Handler:    handler.MakeHandler(h.RegisterThumbnailGen),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodDelete,
				Path:       "/files/{uuid}",
				Handler:    handler.MakeHandler(h.DeleteFile),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
			},
		},
	}

	for _, e := range endpoints {
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}

// dto
type GeneratePresignURLRequest struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	MimeType  string `json:"mimeType"`
}

func (req *GeneratePresignURLRequest) Validate() error {
	if strings.TrimSpace(req.Name) == "" {
		return &xerror.ErrorValidation{Message: "name parameter required"}
	}

	if strings.TrimSpace(req.MimeType) == "" {
		return &xerror.ErrorValidation{Message: "mimeType parameter required"}
	}

	if req.SizeBytes < 0 {
		return &xerror.ErrorValidation{Message: "sizeBytes parameter required"}
	}

	return nil
}

func (req *GeneratePresignURLRequest) ToDomainParam() domain.GeneratePresignURLParam {
	return domain.GeneratePresignURLParam{
		Name:      req.Name,
		SizeBytes: req.SizeBytes,
		MimeType:  req.MimeType,
	}
}

type GeneratePresignURLResponse struct {
	FileID     string `json:"fileID"`
	PresignURL string `json:"presignURL"`
}

func PresignURLReturnToResult(res *domain.GeneratePresignURLReturn) GeneratePresignURLResponse {
	return GeneratePresignURLResponse{
		FileID:     res.ID.String(),
		PresignURL: res.PresignURL,
	}
}

type FetchFilesRequest struct {
	MimeTypes []string
}

func (req *FetchFilesRequest) ParseParams(r *http.Request) {
	q := r.URL.Query()
	mTypes := q["mimeTypes[]"]
	if len(mTypes) > 0 {
		if strings.TrimSpace(mTypes[0]) != "" {
			req.MimeTypes = mTypes
		}
	}
}

// handler
type FileApi struct {
	FileService FileService
	JobQueue    JobQueue
}

type FileApiDependency struct {
	FileService FileService
	JobQueue    JobQueue
}

func NewFileApi(dep FileApiDependency) *FileApi {
	return &FileApi{
		FileService: dep.FileService,
		JobQueue:    dep.JobQueue,
	}
}

func (h *FileApi) FetchFiles(w http.ResponseWriter, r *http.Request) error {
	var req FetchFilesRequest
	req.ParseParams(r)
	pagination := handler.ParsePagination(r)
	param := &domain.FindAllFilesParam{
		Pagination: pagination,
		MimeTypes:  req.MimeTypes,
	}

	result, err := h.FileService.FetchAll(r.Context(), param)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, ListToResponse(result), transport.WithMeta(*param))
	return nil
}

func (h *FileApi) GeneratePresignURL(w http.ResponseWriter, r *http.Request) error {
	var req GeneratePresignURLRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	err = req.Validate()
	if err != nil {
		return err
	}

	presignUrlReturn, err := h.FileService.GeneratePresignURL(r.Context(), req.ToDomainParam())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusCreated, PresignURLReturnToResult(presignUrlReturn), transport.WithMeta(req))
	return nil
}

func (h *FileApi) UpdateStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParsePathParam[uuid.UUID](r, "uuid")
	if err != nil {
		return err
	}

	var req UpdateStatusFileRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	err = req.Validate()
	if err != nil {
		return err
	}

	updatedFile, err := h.FileService.Update(r.Context(), id, req.ToDomainParam())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, FileDomainToResponse(updatedFile))
	return nil
}

func (h *FileApi) GetDownloadURL(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParsePathParam[uuid.UUID](r, "uuid")
	if err != nil {
		return err
	}

	downloadURL, err := h.FileService.GeneratePresignDownloadURL(r.Context(), id)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, downloadURL)
	return nil
}

func (h *FileApi) RegisterThumbnailGen(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParsePathParam[uuid.UUID](r, "uuid")
	if err != nil {
		return err
	}

	err = h.JobQueue.EnqueueThumbnailGen(r.Context(), id)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, handler.DefaultSuccessResponse)
	return nil
}

func (h *FileApi) DeleteFile(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParsePathParam[uuid.UUID](r, "uuid")
	if err != nil {
		return err
	}

	err = h.FileService.Delete(r.Context(), id)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, handler.DefaultSuccessResponse)
	return nil
}
