package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

type LoginApi struct {
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

func NewLoginApi(dep LoginApiDeps) *LoginApi {
	return &LoginApi{
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

func (h *LoginApi) ApiLogin(w http.ResponseWriter, r *http.Request) error {
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

	refreshToken, err := h.JWTService.IssueRefreshToken(r.Context(), user.ID, h.jwtSecret)
	if err != nil {
		return &xerror.ErrorToken{Message: err.Error()}
	}

	session, err := h.CookieService.Get(r, sessionKey)
	if err != nil {
		return &xerror.ErrorSession{Message: err.Error()}
	}
	session.Values["user_id"] = user.ID
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

func (h *LoginApi) RefreshToken(w http.ResponseWriter, r *http.Request) error {
	session, err := h.CookieService.Get(r, sessionKey)
	if err != nil {
		return &xerror.ErrorSession{Message: "invalid or expired session cookie"}
	}

	// 2. Extract the values stored during login
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
