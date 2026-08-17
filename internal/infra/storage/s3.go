package storage

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Implementation
type RustFS struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
	bucketPrefix  string
}

type RustFSDependency struct {
	StorageClient *S3Client
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

	files := make([]string, 0, *output.KeyCount)
	for _, obj := range output.Contents {
		files = append(files, *obj.Key) // key is name
	}

	return files, nil
}

func (r *RustFS) GetPresignPutURL(ctx context.Context, key string) (string, error) {
	objectInput := &s3.PutObjectInput{
		Bucket: aws.String(env.Get().S3Config.S3Bucket),
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

func (r *RustFS) GetPresignGetURL(ctx context.Context, key string) (string, error) {
	objectInput := &s3.GetObjectInput{
		Bucket: aws.String(env.Get().S3Config.S3Bucket),
		Key:    aws.String(key),
	}

	req, err := r.presignClient.PresignGetObject(ctx, objectInput, func(opts *s3.PresignOptions) {
		opts.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (r *RustFS) GetObjectIntoBuffer(ctx context.Context, key string) ([]byte, error) {
	object, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Get().S3Config.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	defer object.Body.Close()

	buff, err := io.ReadAll(object.Body)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func (r *RustFS) UploadObject(ctx context.Context, key string, mimeType string, buff []byte) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(env.Get().S3Config.S3Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buff),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *RustFS) DeleteObject(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Get().S3Config.S3Bucket),
		Key:    aws.String(key),
	})

	return err
}
