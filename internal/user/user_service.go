package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/anditakaesar/uwa-go-rag/internal/application"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

type Service struct {
	userRepo    UserRepository
	passChecker PasswordChecker
	uow         application.UnitOfWork
}

type ServiceDependency struct {
	UserRepo    UserRepository
	PassChecker PasswordChecker
	UOW         application.UnitOfWork
}

func NewUserService(dep ServiceDependency) *Service {
	return &Service{
		userRepo:    dep.UserRepo,
		passChecker: dep.PassChecker,
		uow:         dep.UOW,
	}
}

func (s *Service) CreateUser(ctx context.Context, user domain.User) (*domain.User, error) {
	hash, err := s.passChecker.HashPassword(user.Password)
	if err != nil {
		return nil, err
	}

	user.Password = hash
	return s.userRepo.CreateUser(ctx, user)
}

func (s *Service) CreateUserWithRole(ctx context.Context, user domain.User, role string) (*domain.User, error) {
	hash, err := s.passChecker.HashPassword(user.Password)
	if err != nil {
		return nil, err
	}

	user.Password = hash
	return s.userRepo.CreateUserWithRole(ctx, user, role)
}

func (s *Service) AuthenticateUser(ctx context.Context, username string, password string) (*domain.User, error) {
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

func (s *Service) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	return s.userRepo.FetchUserByParam(ctx, domain.FetchUserParam{
		ID: &id,
	})
}

func (s *Service) UpdatePassword(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error) {
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

func (s *Service) Update(ctx context.Context, id int64, update *domain.UpdateUserParam) (*domain.User, error) {
	var result *domain.User
	updateErr := s.uow.Do(ctx, func(txCtx context.Context) error {
		user, err := s.userRepo.FetchUserByParam(txCtx, domain.FetchUserParam{
			ID:        &id,
			ForUpdate: true,
		})
		if err != nil {
			return err
		}

		hash, err := s.passChecker.HashPassword(*update.Password)
		if err != nil {
			return err
		}

		result, err = s.userRepo.Update(txCtx, user.ID, domain.UpdateUserParam{
			RoleID:   update.RoleID,
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

func (s *Service) FindAll(ctx context.Context, param domain.FindAllUsersParam) ([]domain.UserEnriched, *domain.FindAllUsersParam, error) {
	param.Normalize()
	users, err := s.userRepo.FindAll(ctx, &param)
	if err != nil {
		return nil, nil, err
	}
	return users, &param, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	delErr := s.uow.Do(ctx, func(txCtx context.Context) error {
		userToDelete, err := s.userRepo.FetchUserByParam(txCtx, domain.FetchUserParam{
			ID:        &id,
			ForUpdate: true,
		})
		if err != nil {
			return err
		}

		_, err = s.userRepo.Delete(txCtx, userToDelete.ID)
		return err
	})

	return delErr
}
