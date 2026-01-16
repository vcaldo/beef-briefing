package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/newrelic/go-agent/v3/newrelic"
)

type MinIOClient struct {
	client *minio.Client
	bucket string
	nrApp  *newrelic.Application
}

func NewMinIOClient(endpoint, accessKey, secretKey, bucket, region string, useSSL bool, nrApp *newrelic.Application) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	mc := &MinIOClient{
		client: client,
		bucket: bucket,
		nrApp:  nrApp,
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

// ComputeFileHash computes the SHA256 hash of file data and returns it as a hex string.
// This can be used for content-addressable storage and deduplication.
func ComputeFileHash(data []byte) string {
	hashBytes := sha256.Sum256(data)
	return hex.EncodeToString(hashBytes[:])
}

// GenerateObjectKey generates a MinIO object key from media type and file hash.
// Format: {mediaType}/{hash[:2]}/{hash}
func GenerateObjectKey(mediaType, fileHash string) string {
	return fmt.Sprintf("%s/%s/%s", mediaType, fileHash[:2], fileHash)
}

// UploadMedia uploads media file to MinIO using content-addressable storage.
// Returns the object key and file hash. Deduplicates files with same hash.
func (mc *MinIOClient) UploadMedia(ctx context.Context, fileID string, data []byte, mimeType string, mediaType string) (string, string, error) {
	defer nrutil.StartSegment(ctx, "storage:upload-media")()

	// Compute SHA256 hash of file content
	fileHash := ComputeFileHash(data)

	// Generate object key: {mediaType}/{hash[:2]}/{hash}
	objectKey := GenerateObjectKey(mediaType, fileHash)

	// Check if object already exists (deduplication)
	_, err := mc.client.StatObject(ctx, mc.bucket, objectKey, minio.StatObjectOptions{})
	if err == nil {
		// Object already exists, skip upload
		slog.Debug("media already exists in MinIO, skipping upload",
			"object_key", objectKey,
			"file_hash", fileHash,
		)
		return objectKey, fileHash, nil
	}

	// Upload new object
	reader := bytes.NewReader(data)

	_, err = mc.client.PutObject(
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
		nrutil.NoticeError(ctx, err)
		return "", "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	slog.Debug("uploaded media to MinIO",
		"object_key", objectKey,
		"file_hash", fileHash,
		"size", len(data),
		"mime_type", mimeType,
	)

	return objectKey, fileHash, nil
}

// GetObjectURL returns the URL to access an object
func (mc *MinIOClient) GetObjectURL(objectKey string) string {
	return fmt.Sprintf("%s/%s/%s", mc.client.EndpointURL(), mc.bucket, objectKey)
}

// GetObject retrieves an object from MinIO by its key.
// Returns the object reader, content length, content type, and error.
// The caller is responsible for closing the returned reader.
func (mc *MinIOClient) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, int64, string, error) {
	defer nrutil.StartSegment(ctx, "storage:get-object")()

	obj, err := mc.client.GetObject(ctx, mc.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		nrutil.NoticeError(ctx, err)
		return nil, 0, "", fmt.Errorf("failed to get object from MinIO: %w", err)
	}

	// Get object info for content-length and content-type
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		nrutil.NoticeError(ctx, err)
		return nil, 0, "", fmt.Errorf("failed to stat object: %w", err)
	}

	slog.Debug("retrieved object from MinIO",
		"object_key", objectKey,
		"size", info.Size,
		"content_type", info.ContentType,
	)

	return obj, info.Size, info.ContentType, nil
}

// GetPresignedURL generates a time-limited URL for accessing an object.
func (mc *MinIOClient) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	defer nrutil.StartSegment(ctx, "storage:get-presigned-url")()

	url, err := mc.client.PresignedGetObject(ctx, mc.bucket, objectKey, expiry, nil)
	if err != nil {
		nrutil.NoticeError(ctx, err)
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	slog.Debug("generated presigned URL",
		"object_key", objectKey,
		"expiry", expiry,
	)

	return url.String(), nil
}

// GetPresignedURLSeconds is a convenience wrapper that takes expiry in seconds.
// Implements the services.MinIOClient interface.
func (mc *MinIOClient) GetPresignedURLSeconds(ctx context.Context, objectKey string, expirySeconds int) (string, error) {
	expiry := time.Duration(expirySeconds) * time.Second
	return mc.GetPresignedURL(ctx, objectKey, expiry)
}
