package server_test

import (
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/server"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
)

func TestNewInfra(test *testing.T) {
	env.S3Conf = &env.S3Config{}
	env.Values = &env.Object{}
	test.Run("success", func(t *testing.T) {
		got := server.NewInfra(nil, &infra.InfraStorageClient{
			StorageClient: &s3.Client{},
			PresignClient: &s3.PresignClient{},
		})
		assert.NotNil(t, got)
	})
}
