package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
)

// adapters
type UserService interface {
	CreateUser(ctx context.Context, user domain.User) (*domain.User, error)
	AuthenticateUser(ctx context.Context, username string, password string) (*domain.User, error)
}

type AuditRecorder interface {
	Record(ctx context.Context, auditlog domain.AuditLog) error
}

type CookieService interface {
	Get(r *http.Request, name string) (*sessions.Session, error)
	Save(ses *sessions.Session, r *http.Request, w http.ResponseWriter) error
}

type JWTService interface {
	Verify(token string) (domain.UserClaims, error)
	IssueJWT(ctx context.Context, userID int64, secret []byte) (string, error)

	VerifyRefreshToken(ctx context.Context, token string) (domain.RefreshTokenClaims, error)
	IssueRefreshToken(ctx context.Context, param common.RefreshTokenParam) (string, error)
}

type AuthApi struct {
	UserService   UserService
	JWTService    JWTService
	jwtSecret     []byte
	CookieService CookieService
	AuditService  AuditRecorder
}

type LoginApiDeps struct {
	UserService   UserService
	JWTService    JWTService
	JWTSecret     string
	CookieService CookieService
	AuditService  AuditRecorder
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
	endpoints := []handler.Endpoint{
		{
			HttpMethod: http.MethodPost,
			Path:       "/auth/login",
			Handler:    handler.MakeHandler(h.Login),
		},
		{
			HttpMethod: http.MethodPost,
			Path:       "/auth/refresh",
			Handler:    handler.MakeHandler(h.RefreshToken),
		},
		{
			HttpMethod: http.MethodPost,
			Path:       "/auth/logout",
			Handler:    handler.MakeHandler(h.Logout),
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

	session, err := h.CookieService.Get(r, env.SESSION_KEY)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}

	samesiteMode := http.SameSiteLaxMode

	if env.Values.IsDevelopment() {
		samesiteMode = http.SameSiteNoneMode
	}

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

	go func(recorder AuditRecorder) {
		errAudit := recorder.Record(context.Background(), domain.AuditLog{
			ResourceName: "users",
			ResourceID:   fmt.Sprint(user.ID),
			Action:       domain.USER_LOGIN,
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
	session, err := h.CookieService.Get(r, env.SESSION_KEY)
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
	session, err := h.CookieService.Get(r, env.SESSION_KEY)
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
