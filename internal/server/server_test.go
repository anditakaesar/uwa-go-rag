package server_test

import (
	"reflect"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/server"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestSetupServer(test *testing.T) {
	test.Run("success generate ", func(t *testing.T) {

		mockDB := new(mocks.MockIDatabase)

		mockDB.On("Get").Return(nil).Once()

		got := server.SetupServer(&server.ServerDependency{
			DB: mockDB,
		})

		assert.Equal(t, reflect.TypeOf(&chi.Mux{}), reflect.TypeOf(got))
		mockDB.AssertExpectations(t)
	})
}
