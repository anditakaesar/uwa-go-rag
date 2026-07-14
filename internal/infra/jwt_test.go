package infra_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockItems struct {
	ctx                context.Context
	rolePermissionRepo *mocks.MockIInfraRolePermissionRepo
	anything           string
	now                time.Time
}

func setupMocks() *mockItems {
	mockRolePermissionRepo := new(mocks.MockIInfraRolePermissionRepo)
	return &mockItems{
		ctx:                context.Background(),
		rolePermissionRepo: mockRolePermissionRepo,
		anything:           mock.Anything,
		now:                time.Now(),
	}
}

func TestJWTService(test *testing.T) {
	secret := "super-secret"
	env.Values = &env.Object{
		JWTSecret: secret,
		JWTExpire: 15,
	}
	userID := int64(112)

	test.Run("issue and verify success", func(t *testing.T) {
		m := setupMocks()
		m.rolePermissionRepo.On("GetPermissionsByUser", m.anything, userID).Return(
			[]domain.Permission{
				{
					ID:   int64(1),
					Name: "resource.action",
				},
			}, nil,
		).Once()

		svc := infra.NewJWTService(infra.JWTServiceDep{
			Secret:             []byte(secret),
			JWTExpire:          15,
			RolePermissionRepo: m.rolePermissionRepo,
		})

		token, err := svc.IssueJWT(context.Background(), userID, []byte(secret))
		assert.NoError(t, err)

		claims, err := svc.Verify(token)
		assert.NoError(t, err)

		claimUserID, err := strconv.ParseInt(claims.Subject, 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, userID, claimUserID)
		assert.NotEqual(t, true, claims.ExpiresAt.Before(time.Now()))
	})

	test.Run("invalid secret failure", func(t *testing.T) {
		m := setupMocks()
		m.rolePermissionRepo.On("GetPermissionsByUser", m.anything, userID).Return(
			[]domain.Permission{
				{
					ID:   int64(1),
					Name: "resource.action",
				},
			}, nil,
		).Once()

		svc := infra.NewJWTService(infra.JWTServiceDep{
			Secret:             []byte(secret),
			RolePermissionRepo: m.rolePermissionRepo,
		})

		token, err := svc.IssueJWT(context.Background(), userID, []byte(secret))
		assert.NoError(t, err)

		wrongSecretSvc := infra.NewJWTService(infra.JWTServiceDep{
			Secret:             []byte("wrong-secret"),
			RolePermissionRepo: nil,
		})
		_, err = wrongSecretSvc.Verify(token)
		assert.Error(t, err)
	})

	test.Run("malformed token", func(t *testing.T) {
		m := setupMocks()
		svc := infra.NewJWTService(infra.JWTServiceDep{
			Secret:             []byte(secret),
			RolePermissionRepo: m.rolePermissionRepo,
		})

		_, err := svc.Verify("not-a-token")
		assert.Error(t, err)
	})
}
