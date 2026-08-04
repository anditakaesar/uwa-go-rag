package file

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/go-chi/chi/v5"
)

type FileService interface {
	Save(filename string, content io.Reader) (string, error)
	ListFiles(ctx context.Context) ([]string, error)
	GeneratePresignURL(ctx context.Context, param domain.GeneratePresignURLParam) (*domain.GeneratePresignURLReturn, error)
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
				HttpMethod: http.MethodPost,
				Path:       "/files/generate-presign-url",
				Handler:    handler.MakeHandler(h.GeneratePresignURL),
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

// handler
type FileApi struct {
	FileService FileService
}

type FileApiDependency struct {
	FileService FileService
}

func NewFileApi(dep FileApiDependency) *FileApi {
	return &FileApi{
		FileService: dep.FileService,
	}
}

func (h *FileApi) FetchFiles(w http.ResponseWriter, r *http.Request) error {
	result, err := h.FileService.ListFiles(r.Context())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, result)
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

func (h *FileApi) UpdateFileStatus(w http.ResponseWriter, r *http.Request) error {
	// call fileservice api to set the status into completed or failed
	return nil
}
