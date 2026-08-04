package user

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/server/handler"
	"github.com/anditakaesar/uwa-go-rag/internal/server/transport"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

type IUserService interface {
	CreateUser(ctx context.Context, user domain.User) (*domain.User, error)
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
	UpdatePassword(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error)
	Update(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error)
	FindAll(ctx context.Context, param domain.FindAllUsersParam) ([]domain.UserEnriched, *domain.FindAllUsersParam, error)
	Delete(ctx context.Context, id int64) error
}

type UserApi struct {
	UserService IUserService
}

type UserApiDeps struct {
	UserService IUserService
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
		RoleID:   req.RoleID,
	})
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusCreated, UserDomainToResponse(user))
	return nil
}

func (h *UserApi) UpdateUserPassword(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParseIDParam(r)
	if err != nil {
		return &xerror.ErrorNotFound{Message: err.Error()}
	}

	var req UpdateUserPasswordReq
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
		return &xerror.ErrorPermission{Message: "unauthorized"}
	}

	if requesterUser.ID != id {
		return &xerror.ErrorPermission{Message: "unauthorized"}
	}

	_, err = h.UserService.UpdatePassword(r.Context(), id, req.ToDomainParam())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, handler.DefaultSuccessResponse)
	return nil
}

func (h *UserApi) FetchUsers(w http.ResponseWriter, r *http.Request) error {
	pagination := handler.ParsePagination(r)

	var req FindUserRequest
	req.parseParam(r)

	users, param, err := h.UserService.FindAll(r.Context(), domain.FindAllUsersParam{
		UsernameLike: req.UsernameLike,
		Pagination:   pagination,
	})
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, UserEnrichedListToResponse(users), transport.WithMeta(*param))
	return nil
}

func (h *UserApi) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParseIDParam(r)
	if err != nil {
		return err
	}

	err = h.UserService.Delete(r.Context(), id)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, handler.DefaultSuccessResponse)
	return nil
}

func (h *UserApi) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := handler.ParseIDParam(r)
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

	_, err = h.UserService.Update(r.Context(), id, req.ToDomainParam())
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, handler.DefaultSuccessResponse)
	return nil
}

func (h *UserApi) FetchMe(w http.ResponseWriter, r *http.Request) error {
	identity := r.Context().Value(domain.IdentityKey).(domain.Identity)

	user, err := h.UserService.GetUserByID(r.Context(), identity.UserID)
	if err != nil {
		return err
	}

	transport.SendJSON(w, http.StatusOK, UserDomainToResponse(user))
	return nil
}
