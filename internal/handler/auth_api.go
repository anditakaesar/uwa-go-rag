package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
)

type AuthApi struct {
	UserService   service.IUserService
	JWTService    infra.IJWTService
	jwtSecret     []byte
	CookieService infra.ICookieService
	AuditService  audit.Recorder
}

type LoginApiDeps struct {
	UserService   service.IUserService
	JWTService    infra.IJWTService
	JWTSecret     string
	CookieService infra.ICookieService
	AuditService  audit.Recorder
}

func NewLoginApi(dep LoginApiDeps) *AuthApi {
	return &AuthApi{
		UserService:   dep.UserService,
		JWTService:    dep.JWTService,
		jwtSecret:     []byte(dep.JWTSecret),
		CookieService: dep.CookieService,
		AuditService:  dep.AuditService,
	}
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func SetupLoginApiRoutes(router chi.Router, h *AuthApi) {
	endpoints := []Endpoint{
		{
			HttpMethod: http.MethodPost,
			Path:       "/auth/login",
			Handler:    MakeHandler(h.Login),
		},
		{
			HttpMethod: http.MethodPost,
			Path:       "/auth/refresh",
			Handler:    MakeHandler(h.RefreshToken),
		},
		{
			HttpMethod: http.MethodPost,
			Path:       "/auth/logout",
			Handler:    MakeHandler(h.Logout),
		},
	}

	router.Group(func(r chi.Router) {
		for _, endpoint := range endpoints {
			r.MethodFunc(endpoint.HttpMethod, endpoint.Path, endpoint.Handler)
		}
	})
}

func (h *AuthApi) Login(w http.ResponseWriter, r *http.Request) error {
	var loginReq LoginReq

	err := json.NewDecoder(r.Body).Decode(&loginReq)

	if loginReq.Username == "" || loginReq.Password == "" {
		return &xerror.ErrorValidation{Message: "username and password required"}
	}

	user, err := h.UserService.AuthenticateUser(r.Context(), loginReq.Username, loginReq.Password)
	if err != nil {
		return &xerror.ErrorSession{Message: "username and password didn't match"}
	}

	jwtToken, err := h.JWTService.IssueJWT(r.Context(), user.ID, h.jwtSecret)
	if err != nil {
		return &xerror.ErrorToken{Message: err.Error()}
	}

	maxAge := 7 * 86400
	refreshToken, err := h.JWTService.IssueRefreshToken(r.Context(), common.RefreshTokenParam{
		UserID:           user.ID,
		Secret:           h.jwtSecret,
		MaxAgeExpiration: maxAge,
	})
	if err != nil {
		return &xerror.ErrorToken{Message: err.Error()}
	}

	session, err := h.CookieService.Get(r, sessionKey)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	samesiteMode := http.SameSiteLaxMode

	session.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: samesiteMode,
		Secure:   true,
		MaxAge:   maxAge,
	}

	session.Values["user_id"] = user.ID
	session.Values["username"] = user.Username
	session.Values["refreshToken"] = refreshToken

	err = h.CookieService.Save(session, r, w)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	go func(recorder audit.Recorder) {
		errAudit := recorder.Record(context.Background(), audit.AuditLog{
			ResourceName: "users",
			ResourceID:   fmt.Sprint(user.ID),
			Action:       audit.USER_LOGIN,
			ActorName:    user.Username,
			ActorID:      &user.ID,
		})
		if errAudit != nil {
			xlog.Logger.Error(fmt.Sprintf("error when audit logging: %v", errAudit))
		}
	}(h.AuditService)

	transport.SendJSON(w, http.StatusOK, map[string]string{
		"token": jwtToken,
	})
	return nil
}

func (h *AuthApi) RefreshToken(w http.ResponseWriter, r *http.Request) error {
	session, err := h.CookieService.Get(r, sessionKey)
	if err != nil {
		return &xerror.ErrorSession{Message: "invalid or expired session cookie"}
	}

	refreshTokenVal, ok := session.Values["refreshToken"]
	if !ok || refreshTokenVal == "" {
		return &xerror.ErrorSession{Message: "refresh token missing from session"}
	}

	_, err = h.JWTService.VerifyRefreshToken(r.Context(), refreshTokenVal.(string))
	if err != nil {
		return &xerror.ErrorPermission{Message: "refresh token invalid or expired"}
	}

	userIDVal, ok := session.Values["user_id"]
	if !ok {
		return &xerror.ErrorSession{Message: "user identity missing from session"}
	}

	jwtToken, err := h.JWTService.IssueJWT(r.Context(), userIDVal.(int64), h.jwtSecret)
	if err != nil {
		return &xerror.ErrorSession{Message: "generate new token failed"}
	}

	transport.SendJSON(w, http.StatusOK, map[string]string{
		"token": jwtToken,
	})
	return nil
}

func (h *AuthApi) Logout(w http.ResponseWriter, r *http.Request) error {
	session, err := h.CookieService.Get(r, sessionKey)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	session.Values = make(map[any]any)
	session.Options.MaxAge = -1

	err = h.CookieService.Save(session, r, w)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	return nil
}
