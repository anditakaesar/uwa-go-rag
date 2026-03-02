package domain_test

import (
	"context"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestIdentityFromContext(test *testing.T) {
	test.Parallel()

	test.Run("success", func(t *testing.T) {
		ctx := context.Background()
		newCtx := context.WithValue(
			ctx,
			domain.IdentityKey,
			domain.Identity{
				UserID: 1,
				Method: "jwt",
			},
		)

		got, ok := domain.IdentityFromContext(newCtx)
		assert.Equal(t, true, ok)
		assert.Equal(t, int64(1), got.UserID)
		assert.Equal(t, "jwt", got.Method)
	})
}
