package server_test

import (
	"reflect"
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/mocks"
	"github.com/anditakaesar/uwa-go-rag/internal/server"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
)

func TestSetupServer(test *testing.T) {
	env.Values = &env.Object{}
	env.CorsOpts = &env.CorsOptions{}
	env.S3Conf = &env.S3Config{}
	test.Run("success generate setup server", func(t *testing.T) {

		mockDB := new(mocks.MockIDatabase)
		mockS3Client := new(mocks.MockIStorageClient)

		mockDB.On("Get").Return(nil).Once()
		mockS3Client.On("Get").Return(&infra.InfraStorageClient{
			StorageClient: &s3.Client{},
			PresignClient: &s3.PresignClient{},
		}).Once()

		got := server.SetupServer(&server.ServerDependency{
			DB:            mockDB,
			StorageClient: mockS3Client,
		})

		assert.Equal(t, reflect.TypeOf(&server.Executor{}), reflect.TypeOf(got))
		mockDB.AssertExpectations(t)
	})
}
