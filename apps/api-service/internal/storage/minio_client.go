package storage

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOClient struct {
	client *minio.Client
	bucket string
}

func NewMinIOClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	mc := &MinIOClient{
		client: client,
		bucket: bucket,
	}

	// Create bucket if it doesn't exist
	if err := mc.ensureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	return mc, nil
}

func (mc *MinIOClient) ensureBucket(ctx context.Context) error {
	exists, err := mc.client.BucketExists(ctx, mc.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		slog.Info("creating MinIO bucket", "bucket", mc.bucket)
		err = mc.client.MakeBucket(ctx, mc.bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		slog.Info("MinIO bucket created", "bucket", mc.bucket)
	}

	return nil
}

// UploadMedia uploads media file to MinIO and returns the object key
func (mc *MinIOClient) UploadMedia(ctx context.Context, fileID string, data []byte, mimeType string, mediaType string) (string, error) {
	// Generate object key: {mediaType}/{date}/{fileID}
	datePrefix := time.Now().Format("2006-01-02")
	objectKey := path.Join(mediaType, datePrefix, fileID)

	reader := bytes.NewReader(data)

	_, err := mc.client.PutObject(
		ctx,
		mc.bucket,
		objectKey,
		reader,
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: mimeType,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	slog.Debug("uploaded media to MinIO",
		"object_key", objectKey,
		"size", len(data),
		"mime_type", mimeType,
	)

	return objectKey, nil
}

// GetObjectURL returns the URL to access an object
func (mc *MinIOClient) GetObjectURL(objectKey string) string {
	return fmt.Sprintf("%s/%s/%s", mc.client.EndpointURL(), mc.bucket, objectKey)
}
