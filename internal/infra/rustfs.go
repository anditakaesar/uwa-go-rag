package infra

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client Abstraction
type S3Client struct {
	client *s3.Client
}

func NewS3Client(ctx context.Context, dep S3ClientDependency) (*S3Client, error) {
	client := s3.New(s3.Options{
		Region:       dep.Region,
		BaseEndpoint: aws.String(dep.EndpointURL),
		Credentials:  credentials.NewStaticCredentialsProvider(dep.AccessKey, dep.SecretKey, ""),
		UsePathStyle: true,
	})

	// test connection
	_, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		xlog.Logger.Error(err.Error())
		return nil, err
	}

	return &S3Client{
		client: client,
	}, nil
}

func (impl *S3Client) Get() *s3.Client {
	return impl.client
}

// Implementation
type RustFS struct {
	client *s3.Client
}

type S3ClientDependency struct {
	EndpointURL string
	AccessKey   string
	SecretKey   string
	Region      string
}

func NewRustFs(client *s3.Client) *RustFS {
	return &RustFS{
		client: client,
	}
}

func (r *RustFS) ListFiles(ctx context.Context, bucketName string, prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
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
