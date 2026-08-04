package web

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/middlewares"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
)

const (
	loginPage string = "login.html"
)

// adapter
type FileService interface {
	Save(filename string, content io.Reader) (string, error)
	ListFiles(ctx context.Context) ([]string, error)
	GeneratePresignURL(ctx context.Context, param domain.GeneratePresignURLParam) (*domain.GeneratePresignURLReturn, error)
}

type WebRenderer interface {
	Render(w http.ResponseWriter, name string, data any)
	Render2(ctx context.Context, w http.ResponseWriter, name string, data map[string]any)
}

type UserService interface {
	AuthenticateUser(ctx context.Context, username string, password string) (*domain.User, error)
}

type JWTService interface {
	Verify(token string) (domain.UserClaims, error)
	IssueJWT(ctx context.Context, userID int64, secret []byte) (string, error)

	VerifyRefreshToken(ctx context.Context, token string) (domain.RefreshTokenClaims, error)
	IssueRefreshToken(ctx context.Context, param common.RefreshTokenParam) (string, error)
}

type CookieService interface {
	Get(r *http.Request, name string) (*sessions.Session, error)
	Save(ses *sessions.Session, r *http.Request, w http.ResponseWriter) error
}

type MainHandler struct {
	UserService   UserService
	JWTService    JWTService
	jwtSecret     []byte
	CookieService CookieService
	FileService   FileService
	Render        func(context.Context, http.ResponseWriter, string, map[string]any)
}

type MainHandlerDeps struct {
	UserService   UserService
	JWTService    JWTService
	JWTSecret     string
	CookieService CookieService
	FileService   FileService
	WebRenderer   WebRenderer
}

func NewMainHandler(dep MainHandlerDeps) *MainHandler {
	return &MainHandler{
		UserService:   dep.UserService,
		JWTService:    dep.JWTService,
		jwtSecret:     []byte(dep.JWTSecret),
		CookieService: dep.CookieService,
		FileService:   dep.FileService,
		Render:        dep.WebRenderer.Render2,
	}
}

func SetupMainRoutes(router chi.Router, h *MainHandler) {
	endpoints := []handler.Endpoint{
		{
			HttpMethod: http.MethodGet,
			Path:       "/",
			Handler:    h.Index,
		},
		{
			HttpMethod: http.MethodGet,
			Path:       "/login",
			Handler:    h.GetLogin,
		},
		{
			HttpMethod: http.MethodPost,
			Path:       "/login",
			Handler:    handler.MakeHandler(h.DoLogin),
		},
	}

	protectedEndpoints := []handler.EndpointWithMiddleware{
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/logout",
				Handler:    handler.MakeHandler(h.DoLogout),
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.CSRFMiddleware(),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodGet,
				Path:       "/upload",
				Handler:    h.GetUploadPage,
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				//middlewares.RequireRole([]domain.Role{domain.RoleAdmin}),
				middlewares.CSRFMiddleware(),
			},
		},
		{
			Endpoint: handler.Endpoint{
				HttpMethod: http.MethodPost,
				Path:       "/upload",
				Handler:    h.PostUpload,
			},
			Middlewares: []func(http.Handler) http.Handler{
				middlewares.RequireAuth(),
				middlewares.CSRFMiddleware(),
			},
		},
	}

	router.Group(func(r chi.Router) {
		r.Use(middlewares.CSRFMiddleware())
		for _, endpoint := range endpoints {
			r.MethodFunc(endpoint.HttpMethod, endpoint.Path, endpoint.Handler)
		}
	})

	for _, e := range protectedEndpoints {
		if len(e.Middlewares) > 0 {
			router.With(e.Middlewares...).MethodFunc(e.HttpMethod, e.Path, e.Handler)
		}
	}
}

func (h *MainHandler) Index(w http.ResponseWriter, r *http.Request) {
	session, err := h.CookieService.Get(r, env.SESSION_KEY)
	if err != nil {
		transport.SendError(w, http.StatusInternalServerError, transport.ErrObj{
			Title:   "error when get auth_session",
			Message: err.Error(),
		})
		return
	}

	data := map[string]any{
		"Title": "Home Page",
		"Name":  "Index page html",
	}

	token, ok := session.Values["token"].(string)
	if ok {
		data["Token"] = token
	}

	h.Render(r.Context(), w, "index.html", data)
}

func (h *MainHandler) GetLogin(w http.ResponseWriter, r *http.Request) {
	h.Render(r.Context(), w, loginPage, map[string]any{
		"CSRF": csrf.Token(r),
	})
}

func (h *MainHandler) DoLogout(w http.ResponseWriter, r *http.Request) error {
	session, err := h.CookieService.Get(r, env.SESSION_KEY)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	delete(session.Values, "user_id")
	delete(session.Values, "username")
	delete(session.Values, "token")

	err = h.CookieService.Save(session, r, w)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (h *MainHandler) DoLogin(w http.ResponseWriter, r *http.Request) error {
	err := r.ParseForm()
	if err != nil {
		return &xerror.ErrorBadRequest{Message: err.Error()}
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		h.Render(r.Context(), w, loginPage, map[string]any{
			"CSRF":  csrf.Token(r),
			"Error": "username and password required",
		})
		return nil
	}

	user, err := h.UserService.AuthenticateUser(r.Context(), username, password)
	if err != nil {
		h.Render(r.Context(), w, loginPage, map[string]any{
			"CSRF":  csrf.Token(r),
			"Error": "invalid credentials",
		})
		return nil
	}

	session, err := h.CookieService.Get(r, env.SESSION_KEY)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}
	session.Values["user_id"] = user.ID
	session.Values["username"] = user.Username

	jwtToken, err := h.JWTService.IssueJWT(r.Context(), user.ID, h.jwtSecret)
	if err != nil {
		return &xerror.ErrorToken{Message: err.Error()}
	}

	session.Values["token"] = jwtToken

	err = h.CookieService.Save(session, r, w)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (h *MainHandler) GetUploadPage(w http.ResponseWriter, r *http.Request) {
	h.Render(r.Context(), w, "upload.html", map[string]any{
		"CSRF": csrf.Token(r),
	})
}

func (h *MainHandler) PostUpload(w http.ResponseWriter, r *http.Request) {
	const uploadHTML = "upload.html"
	err := r.ParseMultipartForm(env.MAX_UPLOAD_SIZE)
	if err != nil {
		h.Render(r.Context(), w, uploadHTML, map[string]any{
			"CSRF":  csrf.Token(r),
			"Error": "error when parsing file",
		})
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		h.Render(r.Context(), w, uploadHTML, map[string]any{
			"CSRF":  csrf.Token(r),
			"Error": "bad request",
		})
		return
	}
	defer file.Close()

	newName, err := h.FileService.Save(handler.Filename, file)
	if err != nil {
		h.Render(r.Context(), w, uploadHTML, map[string]any{
			"CSRF":  csrf.Token(r),
			"Error": fmt.Sprintf("error while performing save file request: %v", err),
		})
		return
	}

	h.Render(r.Context(), w, uploadHTML, map[string]any{
		"CSRF":     csrf.Token(r),
		"Uploaded": "uploads/" + newName,
	})
}
