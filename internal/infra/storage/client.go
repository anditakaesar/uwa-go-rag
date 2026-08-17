package storage

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client Function
type S3Client struct {
	StorageClient *s3.Client
	PresignClient *s3.PresignClient
}

type S3ClientDep struct {
	EndpointURL string
	AccessKey   string
	SecretKey   string
	Region      string
}

func NewStorageClient(ctx context.Context, dep S3ClientDep) (*S3Client, error) {
	rustfs := s3.New(s3.Options{
		Region:       dep.Region,
		BaseEndpoint: aws.String(dep.EndpointURL),
		Credentials:  credentials.NewStaticCredentialsProvider(dep.AccessKey, dep.SecretKey, ""),
		UsePathStyle: true,
	})

	presignClient := s3.NewPresignClient(rustfs)

	// test connection
	_, err := rustfs.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		xlog.Logger.Error(err.Error())
		return nil, err
	}

	xlog.Logger.Info("Storage Client Connected")
	return &S3Client{
		StorageClient: rustfs,
		PresignClient: presignClient,
	}, nil
}

func (i *S3Client) Get() *S3Client {
	return i
}
