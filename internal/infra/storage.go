package infra

import (
	"context"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client Function
type InfraStorageClient struct {
	StorageClient *s3.Client
	PresignClient *s3.PresignClient
}

type S3ClientDependency struct {
	EndpointURL string
	AccessKey   string
	SecretKey   string
	Region      string
}

func NewStorageClient(ctx context.Context, dep S3ClientDependency) (*InfraStorageClient, error) {
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
	return &InfraStorageClient{
		StorageClient: rustfs,
		PresignClient: presignClient,
	}, nil
}

func (i *InfraStorageClient) Get() *InfraStorageClient {
	return i
}

// Implementation
type RustFS struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
	bucketPrefix  string
}

type RustFSDependency struct {
	StorageClient *InfraStorageClient
	BucketName    string
	BucketPrefix  string
}

func NewRustFs(dep RustFSDependency) *RustFS {
	return &RustFS{
		client:        dep.StorageClient.StorageClient,
		presignClient: dep.StorageClient.PresignClient,
		bucketName:    dep.BucketName,
		bucketPrefix:  dep.BucketPrefix,
	}
}

func (r *RustFS) ListFiles(ctx context.Context) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(r.bucketName),
		Prefix: aws.String(r.bucketPrefix),
	}

	output, err := r.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}

	files := make([]string, *output.KeyCount)
	for _, obj := range output.Contents {
		files = append(files, *obj.Key) // key is name
	}

	return files, nil
}

func (r *RustFS) GetPresignURL(ctx context.Context, key string) (string, error) {
	objectInput := &s3.PutObjectInput{
		Bucket: aws.String(env.S3Conf.S3Bucket),
		Key:    aws.String(key),
	}

	req, err := r.presignClient.PresignPutObject(ctx, objectInput, func(opts *s3.PresignOptions) {
		opts.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", err
	}

	return req.URL, nil
}
