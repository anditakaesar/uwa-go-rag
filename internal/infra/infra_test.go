package infra_test

import (
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/stretchr/testify/assert"
)

func TestNewInfra(test *testing.T) {
	test.Run("success", func(t *testing.T) {
		got := infra.NewInfra(nil)
		assert.NotNil(t, got)
	})
}
