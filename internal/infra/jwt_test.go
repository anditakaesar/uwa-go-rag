package infra_test

import (
	"testing"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/stretchr/testify/assert"
)

func TestJWTService(test *testing.T) {
	secret := "super-secret"
	svc := infra.NewJWTService(secret)
	userID := int64(112)

	test.Run("issue and verify success", func(t *testing.T) {
		token, err := svc.IssueJWT(userID, []byte(secret))
		assert.NoError(t, err)

		claims, err := svc.Verify(token)
		assert.NoError(t, err)

		assert.Equal(t, userID, claims.UserID)
		assert.NotEqual(t, true, claims.Exp.Before(time.Now()))
	})

	test.Run("invalid secret failure", func(t *testing.T) {
		token, err := svc.IssueJWT(userID, []byte(secret))
		assert.NoError(t, err)

		wrongSecretSvc := infra.NewJWTService("wrong-secret")
		_, err = wrongSecretSvc.Verify(token)
		assert.Error(t, err)
	})

	test.Run("malformed token", func(t *testing.T) {
		_, err := svc.Verify("not-a-token")
		assert.Error(t, err)
	})
}
