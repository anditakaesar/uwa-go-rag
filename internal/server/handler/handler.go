package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AppHandler func(w http.ResponseWriter, r *http.Request) error

type Endpoint struct {
	HttpMethod string
	Path       string
	Handler    func(w http.ResponseWriter, r *http.Request)
}

type EndpointWithMiddleware struct {
	Endpoint
	Middlewares []func(http.Handler) http.Handler
}

func MakeHandler(h AppHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			xlog.Logger.Error(fmt.Sprintf("Server error [%s]: %v", r.URL.Path, err))
			transport.SendError(w, xerror.DefineStatusCode(err), transport.ErrObj{
				Title:   "server error",
				Message: err.Error(),
			})
		}
	}
}

func ParseIDParam(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	if idParam == "" {
		return 0, &xerror.ErrorResourceNotFound{
			Message: "id param is not valid",
		}
	}

	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return 0, &xerror.ErrorResourceNotFound{
			Message: "id param is not valid",
		}
	}

	return id, nil
}

type ParseableID interface {
	int64 | string | uuid.UUID
}

func ParsePathParam[T ParseableID](r *http.Request, name string) (T, error) {
	var zero T
	rawVal := chi.URLParam(r, name)
	if rawVal == "" {
		return zero, &xerror.ErrorPathParamValue{
			ExpectedName: name,
		}
	}

	// Use type switch on the zero value of T to determine parsing logic
	switch any(zero).(type) {
	case string:
		return any(rawVal).(T), nil

	case int64:
		val, err := strconv.ParseInt(rawVal, 10, 64)
		if err != nil {
			return zero, &xerror.ErrorPathParamValue{
				Message:      "invalid int64 format: " + err.Error(),
				ExpectedName: name,
			}
		}
		return any(val).(T), nil

	case uuid.UUID:
		parsedUUID, err := uuid.Parse(rawVal)
		if err != nil {
			return zero, &xerror.ErrorPathParamValue{
				Message:      "invalid uuid format: " + err.Error(),
				ExpectedName: name,
			}
		}
		return any(parsedUUID).(T), nil

	default:
		return zero, &xerror.ErrorPathParamValue{
			Message:      "unsupported format",
			ExpectedName: name,
		}
	}
}

func ParsePagination(r *http.Request) common.Pagination {
	const (
		defaultPage     int = 1
		defaultPageSize int = 10
	)

	q := r.URL.Query()
	page, err := strconv.Atoi(q.Get("page"))
	if err != nil {
		page = defaultPage
	}

	size, err := strconv.Atoi(q.Get("size"))
	if err != nil {
		size = defaultPageSize
	}

	if page < 1 {
		page = defaultPage
	}

	if size < 1 {
		size = defaultPageSize
	}

	return common.Pagination{
		Page: page,
		Size: size,
	}
}

var DefaultSuccessResponse map[string]string = map[string]string{
	"message": "success",
}
