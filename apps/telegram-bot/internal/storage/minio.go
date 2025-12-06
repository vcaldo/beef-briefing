package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOClient struct {
	client     *minio.Client
	bucketName string
}

func NewMinIOClient(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	mc := &MinIOClient{
		client:     client,
		bucketName: bucketName,
	}

	// Ensure bucket exists
	if err := mc.ensureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	return mc, nil
}

func (m *MinIOClient) ensureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = m.client.MakeBucket(ctx, m.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return nil
}

// ComputeSHA256 computes the SHA256 hash of a reader
func ComputeSHA256(reader io.Reader) (string, []byte, error) {
	hasher := sha256.New()
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read data: %w", err)
	}

	hasher.Write(data)
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	return hash, data, nil
}

// UploadFile uploads a file to MinIO using SHA256 hash as the key
// Returns the SHA256 hash (object key)
func (m *MinIOClient) UploadFile(ctx context.Context, reader io.Reader, contentType string) (string, error) {
	// Compute SHA256 hash
	hash, data, err := ComputeSHA256(reader)
	if err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	// Check if file already exists (deduplication)
	_, err = m.client.StatObject(ctx, m.bucketName, hash, minio.StatObjectOptions{})
	if err == nil {
		// File already exists, return hash
		return hash, nil
	}

	// Upload file with actual data
	_, err = m.client.PutObject(ctx, m.bucketName, hash,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: contentType,
		})

	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return hash, nil
}

// FileExists checks if a file with the given hash exists in MinIO
func (m *MinIOClient) FileExists(ctx context.Context, hash string) (bool, error) {
	_, err := m.client.StatObject(ctx, m.bucketName, hash, minio.StatObjectOptions{})
	if err != nil {
		// Check if error is "object not found"
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	return true, nil
}

// UploadFileWithHash uploads a file using a pre-computed hash and data
func (m *MinIOClient) UploadFileWithHash(ctx context.Context, hash string, data []byte, contentType string) error {
	// Check if file already exists (deduplication)
	_, err := m.client.StatObject(ctx, m.bucketName, hash, minio.StatObjectOptions{})
	if err == nil {
		// File already exists, skip upload
		return nil
	}

	// Upload file
	_, err = m.client.PutObject(ctx, m.bucketName, hash,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: contentType,
		})

	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// GetFileURL returns the URL to access a file
// GetFileURL returns the URL to access a file
func (m *MinIOClient) GetFileURL(ctx context.Context, hash string) (string, error) {
	// For internal access, return the object path
	// In production, you might want to generate a presigned URL
	return fmt.Sprintf("/%s/%s", m.bucketName, hash), nil
}
