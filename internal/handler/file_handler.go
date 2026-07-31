package handler

import (
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/go-chi/chi/v5"
)

// routers
func SetupFileApiRoutes(router chi.Router, h *FileApi) {
	endpoints := []EndpointWithMiddleware{
		{
			Endpoint: Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/files",
				Handler:    MakeHandler(h.FetchFiles),
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

// handler
type FileApi struct {
	FileService service.IFileService
	Bucket      string
	Prefix      string
}

type FileApiDependency struct {
	FileService service.IFileService
	Bucket      string
	Prefix      string
}

func NewFileApi(dep FileApiDependency) *FileApi {
	return &FileApi{
		FileService: dep.FileService,
		Bucket:      dep.Bucket,
		Prefix:      dep.Prefix,
	}
}

func (h *FileApi) FetchFiles(w http.ResponseWriter, r *http.Request) error {
	result, err := h.FileService.ListFiles(r.Context(), h.Bucket, h.Prefix)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, result)
	return nil
}
