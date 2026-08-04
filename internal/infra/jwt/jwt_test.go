package jwt_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/jwt"
	"github.com/anditakaesar/uwa-go-rag/internal/infra/jwt/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockItems struct {
	ctx      context.Context
	repo     *mocks.MockPermissionRepo
	anything string
	now      time.Time
}

func setupMocks() *mockItems {
	mockRolePermissionRepo := new(mocks.MockPermissionRepo)
	return &mockItems{
		ctx:      context.Background(),
		repo:     mockRolePermissionRepo,
		anything: mock.Anything,
		now:      time.Now(),
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
		m.repo.On("GetPermissionsByUser", m.anything, userID).Return(
			[]domain.Permission{
				{
					ID:   int64(1),
					Name: "resource.action",
				},
			}, nil,
		).Once()

		svc := jwt.NewJWTService(jwt.ServiceDependency{
			Secret:             []byte(secret),
			JWTExpire:          15,
			RolePermissionRepo: m.repo,
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
		m.repo.On("GetPermissionsByUser", m.anything, userID).Return(
			[]domain.Permission{
				{
					ID:   int64(1),
					Name: "resource.action",
				},
			}, nil,
		).Once()

		svc := jwt.NewJWTService(jwt.ServiceDependency{
			Secret:             []byte(secret),
			RolePermissionRepo: m.repo,
		})

		token, err := svc.IssueJWT(context.Background(), userID, []byte(secret))
		assert.NoError(t, err)

		wrongSecretSvc := jwt.NewJWTService(jwt.ServiceDependency{
			Secret:             []byte("wrong-secret"),
			RolePermissionRepo: nil,
		})
		_, err = wrongSecretSvc.Verify(token)
		assert.Error(t, err)
	})

	test.Run("malformed token", func(t *testing.T) {
		m := setupMocks()
		svc := jwt.NewJWTService(jwt.ServiceDependency{
			Secret:             []byte(secret),
			RolePermissionRepo: m.repo,
		})

		_, err := svc.Verify("not-a-token")
		assert.Error(t, err)
	})
}
