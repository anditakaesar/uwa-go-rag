package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/common"
	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/mocks/custom"
	"github.com/anditakaesar/uwa-go-rag/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockItems struct {
	ctx         context.Context
	userRepo    *mocks.MockIUserRepository
	passChecker *mocks.MockIPasswordChecker
	uow         *custom.IMockUow
	anything    string
	now         time.Time
}

func setupMocks() *mockItems {
	mockUserRepo := new(mocks.MockIUserRepository)
	mockPassChecker := new(mocks.MockIPasswordChecker)
	mockUOW := new(custom.IMockUow)
	return &mockItems{
		ctx:         context.Background(),
		userRepo:    mockUserRepo,
		passChecker: mockPassChecker,
		uow:         mockUOW,
		anything:    mock.Anything,
		now:         time.Now(),
	}
}

func TestNewUserService(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m := setupMocks()

		got := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})
		assert.NotNil(t, got)
	})

}

func TestUserService_CreateUser(test *testing.T) {
	test.Parallel()

	userParam := domain.User{
		Username: "newusernonadmin",
		Password: "Some Pass",
	}

	test.Run("success", func(t *testing.T) {
		m := setupMocks()
		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})

		userReponse := domain.User{
			Base: domain.Base{
				ID:        1,
				CreatedAt: m.now,
			},
			Username: "John Doe",
			Role:     domain.RoleUser,
			Password: "somestring",
		}

		m.passChecker.On("HashPassword", userParam.Password).Return("somestring", nil).Once()
		updatedParam := userParam
		updatedParam.Password = "somestring"
		m.userRepo.On("CreateUser", m.ctx, updatedParam).Return(&userReponse, nil).Once()

		got, gotErr := s.CreateUser(m.ctx, userParam)
		assert.NoError(t, gotErr)

		assert.Equal(t, userReponse.Username, got.Username)
		m.passChecker.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
	})

	test.Run("error when hashing password", func(t *testing.T) {
		m := setupMocks()
		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})

		m.passChecker.On("HashPassword", userParam.Password).Return("", errors.New("error_HashPassword")).Once()

		got, gotErr := s.CreateUser(m.ctx, userParam)
		assert.Error(t, gotErr)
		assert.Nil(t, got)
		m.passChecker.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
	})

}

func TestUserService_CreateUserAdmin(test *testing.T) {
	test.Parallel()

	userParam := domain.User{
		Username: "newuseradmin",
		Password: "Some Pass",
	}

	test.Run("success", func(t *testing.T) {
		m := setupMocks()
		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})

		userReponse := domain.User{
			Base: domain.Base{
				ID:        1,
				CreatedAt: m.now,
			},
			Username: "John Doe",
			Role:     domain.RoleAdmin,
			Password: "somestring",
		}

		m.passChecker.On("HashPassword", userParam.Password).Return("somestring", nil).Once()
		updatedParam := userParam
		updatedParam.Password = "somestring"
		m.userRepo.On("CreateUserAdmin", m.ctx, updatedParam).Return(&userReponse, nil).Once()

		got, gotErr := s.CreateUserAdmin(m.ctx, userParam)
		assert.NoError(t, gotErr)

		assert.Equal(t, userReponse.Username, got.Username)
		m.passChecker.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
	})

	test.Run("error when hashing password", func(t *testing.T) {
		m := setupMocks()
		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})

		m.passChecker.On("HashPassword", userParam.Password).Return("", errors.New("error_HashPassword")).Once()

		got, gotErr := s.CreateUserAdmin(m.ctx, userParam)
		assert.Error(t, gotErr)
		assert.Nil(t, got)
		m.passChecker.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
	})

}

func TestUserService_AuthenticateUser(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m := setupMocks()

		userResponse := domain.User{
			Username: "testuser",
			Password: "testpassword",
			Role:     domain.RoleAdmin,
			Base: domain.Base{
				ID: 1,
			},
		}

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			Username: &userResponse.Username,
		}).Return(&userResponse, nil).Once()
		m.passChecker.On("CheckPassword", userResponse.Password, userResponse.Password).Return(true, nil).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})
		got, gotErr := s.AuthenticateUser(m.ctx, userResponse.Username, userResponse.Password)

		assert.NoError(t, gotErr)
		assert.NotNil(t, got)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})

	test.Run("password check failed", func(t *testing.T) {
		m := setupMocks()

		userResponse := domain.User{
			Username: "testuser",
			Password: "testpassword",
			Role:     domain.RoleAdmin,
			Base: domain.Base{
				ID: 1,
			},
		}

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			Username: &userResponse.Username,
		}).Return(&userResponse, nil).Once()
		m.passChecker.On("CheckPassword", userResponse.Password, userResponse.Password).Return(false, nil).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})
		got, gotErr := s.AuthenticateUser(m.ctx, userResponse.Username, userResponse.Password)

		assert.Error(t, gotErr)
		assert.Nil(t, got)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})

	test.Run("error when get user", func(t *testing.T) {
		m := setupMocks()

		userResponse := domain.User{
			Username: "testuser",
			Password: "testpassword",
			Base: domain.Base{
				ID: 1,
			},
		}

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			Username: &userResponse.Username,
		}).Return(nil, errors.New("error_FetchUserByParam")).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})
		got, gotErr := s.AuthenticateUser(m.ctx, userResponse.Username, userResponse.Password)

		assert.Error(t, gotErr)
		assert.Nil(t, got)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})
}

func TestUserService_GetUserByID(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m := setupMocks()
		userResponse := domain.User{
			Username: "testuser",
			Password: "testpassword",
			Role:     domain.RoleAdmin,
			Base: domain.Base{
				ID: 1,
			},
		}

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			ID: &userResponse.ID,
		}).Return(&userResponse, nil).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			PassChecker: m.passChecker,
			UOW:         m.uow,
		})
		got, gotErr := s.GetUserByID(context.Background(), int64(1))

		assert.NoError(t, gotErr)
		assert.NotNil(t, got)
		assert.Equal(t, userResponse.Username, got.Username)
		m.userRepo.AssertExpectations(t)
	})

}

func TestUserService_Update(test *testing.T) {
	test.Parallel()

	oldUser := domain.User{
		Base: domain.Base{
			ID: 1,
		},
		Username: "user1",
		Password: "hashedpassword",
		Role:     domain.RoleUser,
	}

	newPass := "newpass"
	updateParam := domain.UpdateUserParam{
		OldPassword: "oldpass",
		Password:    &newPass,
	}

	test.Run("success", func(t *testing.T) {
		m := setupMocks()

		m.uow.On("Do", m.ctx, m.anything).Return(nil)

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			ID:        &oldUser.ID,
			ForUpdate: true,
		}).Return(&oldUser, nil).Once()

		m.passChecker.On("CheckPassword", "oldpass", "hashedpassword").Return(true, nil).Once()

		m.passChecker.On("HashPassword", *updateParam.Password).Return("newhashedpass", nil).Once()

		newUser := oldUser
		newUser.Password = "newhashedpass"
		m.userRepo.On("Update", m.ctx, oldUser.ID, domain.UpdateUserParam{
			Password: &newUser.Password,
		}).Return(&newUser, nil).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			UOW:         m.uow,
			PassChecker: m.passChecker,
		})

		got, gotErr := s.Update(m.ctx, oldUser.ID, &updateParam)
		assert.NoError(t, gotErr)
		assert.Equal(t, oldUser.ID, got.ID)
		assert.Equal(t, "newhashedpass", got.Password)

		m.uow.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})

	test.Run("error when update user", func(t *testing.T) {
		m := setupMocks()

		m.uow.On("Do", m.ctx, m.anything).Return(nil)

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			ID:        &oldUser.ID,
			ForUpdate: true,
		}).Return(&oldUser, nil).Once()

		m.passChecker.On("CheckPassword", "oldpass", "hashedpassword").Return(true, nil).Once()

		m.passChecker.On("HashPassword", *updateParam.Password).Return("newhashedpass", nil).Once()

		newUser := oldUser
		newUser.Password = "newhashedpass"
		m.userRepo.On("Update", m.ctx, oldUser.ID, domain.UpdateUserParam{
			Password: &newUser.Password,
		}).Return(nil, errors.New("error_Update")).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			UOW:         m.uow,
			PassChecker: m.passChecker,
		})

		got, gotErr := s.Update(m.ctx, oldUser.ID, &updateParam)
		assert.Error(t, gotErr)
		assert.Nil(t, got)

		m.uow.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})

	test.Run("error when hash new password", func(t *testing.T) {
		m := setupMocks()

		m.uow.On("Do", m.ctx, m.anything).Return(nil)

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			ID:        &oldUser.ID,
			ForUpdate: true,
		}).Return(&oldUser, nil).Once()

		m.passChecker.On("CheckPassword", "oldpass", "hashedpassword").Return(true, nil).Once()

		m.passChecker.On("HashPassword", *updateParam.Password).Return("", errors.New("error_HashPassword")).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			UOW:         m.uow,
			PassChecker: m.passChecker,
		})

		got, gotErr := s.Update(m.ctx, oldUser.ID, &updateParam)
		assert.Error(t, gotErr)
		assert.Nil(t, got)

		m.uow.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})

	test.Run("error when verify old password", func(t *testing.T) {
		m := setupMocks()

		m.uow.On("Do", m.ctx, m.anything).Return(nil)

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			ID:        &oldUser.ID,
			ForUpdate: true,
		}).Return(&oldUser, nil).Once()

		m.passChecker.On("CheckPassword", "oldpass", "hashedpassword").Return(true, errors.New("error_CheckPassword")).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			UOW:         m.uow,
			PassChecker: m.passChecker,
		})

		got, gotErr := s.Update(m.ctx, oldUser.ID, &updateParam)
		assert.Error(t, gotErr)
		assert.Nil(t, got)

		m.uow.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})

	test.Run("error when fetch current user", func(t *testing.T) {
		m := setupMocks()

		m.uow.On("Do", m.ctx, m.anything).Return(nil)

		m.userRepo.On("FetchUserByParam", m.ctx, domain.FetchUserParam{
			ID:        &oldUser.ID,
			ForUpdate: true,
		}).Return(nil, errors.New("error_FetchUserByParam")).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo:    m.userRepo,
			UOW:         m.uow,
			PassChecker: m.passChecker,
		})

		got, gotErr := s.Update(m.ctx, oldUser.ID, &updateParam)
		assert.Error(t, gotErr)
		assert.Nil(t, got)

		m.uow.AssertExpectations(t)
		m.userRepo.AssertExpectations(t)
		m.passChecker.AssertExpectations(t)
	})
}

func TestUserService_FindAll(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		m := setupMocks()

		m.userRepo.On("FindAll", m.ctx, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 1,
			},
		}).Return([]domain.User{
			{
				Base: domain.Base{
					ID: 1,
				},
				Username: "user1",
				Password: "pass",
				Role:     domain.RoleAdmin,
			},
		}, nil).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo: m.userRepo,
		})

		got, param, gotErr := s.FindAll(m.ctx, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 1,
			},
		})
		assert.NoError(t, gotErr)
		assert.NotNil(t, got)
		assert.Equal(t, 1, len(got))
		assert.NotNil(t, param)

		m.userRepo.AssertExpectations(t)
	})

	test.Run("error", func(t *testing.T) {
		m := setupMocks()

		m.userRepo.On("FindAll", m.ctx, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 1,
			},
		}).Return([]domain.User{}, errors.New("error_FindAll")).Once()

		s := service.NewUserService(service.UserServiceDeps{
			UserRepo: m.userRepo,
		})

		got, param, gotErr := s.FindAll(m.ctx, domain.FindAllUsersParam{
			Pagination: common.Pagination{
				Page: 1,
				Size: 1,
			},
		})
		assert.Error(t, gotErr)
		assert.Nil(t, got)
		assert.Nil(t, param)

		m.userRepo.AssertExpectations(t)
	})
}
