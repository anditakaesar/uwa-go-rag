package handler

import (
	"context"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/gorilla/csrf"
)

const (
	loginPage  string = "login.html"
	sessionKey string = "auth_session"
)

type MainHandler struct {
	UserService   service.IUserService
	JWTService    infra.IJWTService
	CookieService infra.ICookieService
	FileService   service.IFileService
	Render        func(context.Context, http.ResponseWriter, string, map[string]any)
}

type MainHandlerDeps struct {
	UserService   service.IUserService
	JWTService    infra.IJWTService
	CookieService infra.ICookieService
	FileService   service.IFileService
	WebRenderer   IWebRenderer
}

func NewMainHandler(dep MainHandlerDeps) *MainHandler {
	return &MainHandler{
		UserService:   dep.UserService,
		JWTService:    dep.JWTService,
		CookieService: dep.CookieService,
		FileService:   dep.FileService,
		Render:        dep.WebRenderer.Render2,
	}
}

func (h *MainHandler) Index(w http.ResponseWriter, r *http.Request) {
	session, err := h.CookieService.Get(r, sessionKey)
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
	session, err := h.CookieService.Get(r, sessionKey)
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

	session, err := h.CookieService.Get(r, sessionKey)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}
	session.Values["user_id"] = user.ID
	session.Values["username"] = user.Username

	jwtToken, err := h.JWTService.IssueJWT(user.ID, []byte(env.Values.JWTSecret))
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
			"Error": "error while performing save file request",
		})
		return
	}

	h.Render(r.Context(), w, uploadHTML, map[string]any{
		"CSRF":     csrf.Token(r),
		"Uploaded": "uploads/" + newName,
	})
}
