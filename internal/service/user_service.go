package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

type IUserService interface {
	CreateUser(ctx context.Context, user domain.User) (*domain.User, error)
	AuthenticateUser(ctx context.Context, username string, password string) (*domain.User, error)
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
	Update(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error)
	FindAll(ctx context.Context, param domain.FindAllUsersParam) ([]domain.User, *domain.FindAllUsersParam, error)
}

type UserService struct {
	userRepo    IUserRepository
	passChecker IPasswordChecker
	uow         IUnitOfWork
}

type UserServiceDeps struct {
	UserRepo    IUserRepository
	PassChecker IPasswordChecker
	UOW         IUnitOfWork
}

func NewUserService(dep UserServiceDeps) *UserService {
	return &UserService{
		userRepo:    dep.UserRepo,
		passChecker: dep.PassChecker,
		uow:         dep.UOW,
	}
}

func (s *UserService) CreateUser(ctx context.Context, user domain.User) (*domain.User, error) {
	hash, err := s.passChecker.HashPassword(user.Password)
	if err != nil {
		return nil, err
	}

	user.Password = hash
	return s.userRepo.CreateUser(ctx, user)
}

func (s *UserService) CreateUserAdmin(ctx context.Context, user domain.User) (*domain.User, error) {
	hash, err := s.passChecker.HashPassword(user.Password)
	if err != nil {
		return nil, err
	}

	user.Password = hash
	return s.userRepo.CreateUserAdmin(ctx, user)
}

func (s *UserService) AuthenticateUser(ctx context.Context, username string, password string) (*domain.User, error) {
	getUser, err := s.userRepo.FetchUserByParam(ctx, domain.FetchUserParam{
		Username: &username,
	})
	if err != nil {
		return nil, fmt.Errorf("error while getting user: %v", err)
	}

	success, err := s.passChecker.CheckPassword(password, getUser.Password)
	if err != nil || !success {
		return nil, fmt.Errorf("wrong password attempt: %s", password)
	}

	return getUser, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	return s.userRepo.FetchUserByParam(ctx, domain.FetchUserParam{
		ID: &id,
	})
}

func (s *UserService) Update(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error) {
	var result *domain.User
	updateErr := s.uow.Do(ctx, func(txCtx context.Context) error {
		user, err := s.userRepo.FetchUserByParam(txCtx, domain.FetchUserParam{
			ID:        &id,
			ForUpdate: true,
		})
		if err != nil {
			return err
		}

		success, err := s.passChecker.CheckPassword(update.OldPassword, user.Password)
		if !success || err != nil {
			return errors.New("old password didn't match")
		}

		hash, err := s.passChecker.HashPassword(*update.Password)
		if err != nil {
			return err
		}

		result, err = s.userRepo.Update(txCtx, id, domain.UpdateUserParam{
			Password: &hash,
		})
		if err != nil {
			return err
		}

		return nil
	})

	if updateErr != nil {
		return nil, updateErr
	}

	return result, nil
}

func (s *UserService) FindAll(ctx context.Context, param domain.FindAllUsersParam) ([]domain.User, *domain.FindAllUsersParam, error) {
	param.Normalize()
	users, err := s.userRepo.FindAll(ctx, param)
	if err != nil {
		return nil, nil, err
	}
	return users, &param, nil
}
