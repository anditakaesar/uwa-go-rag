package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

type UserApi struct {
	UserService service.IUserService
}

type UserApiDeps struct {
	UserService service.IUserService
}

func NewUserApi(dep UserApiDeps) *UserApi {
	return &UserApi{
		UserService: dep.UserService,
	}
}

func (h *UserApi) CreateUser(w http.ResponseWriter, r *http.Request) error {
	var req CreateUserRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	err = req.Validate()
	if err != nil {
		return err
	}

	user, err := h.UserService.CreateUser(r.Context(), domain.User{
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
	})
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusCreated, UserDomainToResponse(user))
	return nil
}

func (h *UserApi) UpdateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := parseIDParam(r)
	if err != nil {
		return &xerror.ErrorNotFound{Message: err.Error()}
	}

	var req UpdateUserRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return &xerror.ErrorDecodingRequest{Err: err}
	}

	err = req.Validate()
	if err != nil {
		return err
	}

	requesterUser, ok := domain.UserFromContext(r.Context())
	if !ok {
		return &xerror.ErrorPermission{Message: "permission required"}
	}

	if requesterUser.ID != id {
		return &xerror.ErrorPermission{Message: "update user not allowed"}
	}

	user, err := h.UserService.Update(r.Context(), id, req.ToDomainParam())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, UserDomainToResponse(user))
	return nil
}

func (h *UserApi) FetchUsers(w http.ResponseWriter, r *http.Request) error {
	pagination := parsePagination(r)

	users, param, err := h.UserService.FindAll(r.Context(), domain.FindAllUsersParam{
		Pagination: pagination,
	})
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, UserListToResponse(users), transport.WithMeta(*param))
	return nil
}
