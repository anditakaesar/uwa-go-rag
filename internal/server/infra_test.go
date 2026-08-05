package server_test

import (
	"testing"

	"github.com/anditakaesar/uwa-go-rag/internal/infra/storage"
	"github.com/anditakaesar/uwa-go-rag/internal/server"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
)

func TestNewInfra(test *testing.T) {
	test.Run("success", func(t *testing.T) {
		got := server.NewInfra(nil, &storage.S3Client{
			StorageClient: &s3.Client{},
			PresignClient: &s3.PresignClient{},
		})
		assert.NotNil(t, got)
	})
}
