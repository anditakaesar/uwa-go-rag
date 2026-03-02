package handler

import (
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
)

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (req *CreateUserRequest) Validate() error {
	if strings.TrimSpace(req.Username) == "" {
		return &xerror.ErrorValidation{Message: "username is required"}
	}

	if strings.TrimSpace(req.Password) == "" {
		return &xerror.ErrorValidation{Message: "password is required"}
	}

	return nil
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func UserDomainToResponse(user *domain.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      string(user.Role),
		CreatedAt: user.CreatedAt,
	}
}

func UserListToResponse(users []domain.User) []UserResponse {
	results := make([]UserResponse, 0, len(users))
	for _, user := range users {
		u := UserDomainToResponse(&user)
		results = append(results, u)
	}

	return results
}

type UpdateUserRequest struct {
	OldPassword string `json:"oldPassword"`
	Password    string `json:"password"`
}

func (req *UpdateUserRequest) Validate() error {
	if strings.TrimSpace(req.OldPassword) == "" {
		return &xerror.ErrorValidation{Message: "old password is required"}
	}

	if strings.TrimSpace(req.Password) == "" {
		return &xerror.ErrorValidation{Message: "password is required"}
	}

	return nil
}

func (req *UpdateUserRequest) ToDomainParam() *domain.UpdateUserParam {
	return &domain.UpdateUserParam{
		OldPassword: req.OldPassword,
		Password:    &req.Password,
	}
}
