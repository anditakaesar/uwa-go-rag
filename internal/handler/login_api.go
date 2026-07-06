package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anditakaesar/uwa-go-rag/internal/audit"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
)

type LoginApi struct {
	UserService  service.IUserService
	JWTService   infra.IJWTService
	AuditService audit.Recorder
}

type LoginApiDeps struct {
	UserService  service.IUserService
	JWTService   infra.IJWTService
	AuditService audit.Recorder
}

func NewLoginApi(dep LoginApiDeps) *LoginApi {
	return &LoginApi{
		UserService:  dep.UserService,
		JWTService:   dep.JWTService,
		AuditService: dep.AuditService,
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

	jwtToken, err := h.JWTService.IssueJWT(r.Context(), user.ID, []byte(env.Values.JWTSecret))
	if err != nil {
		return &xerror.ErrorToken{Message: err.Error()}
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
