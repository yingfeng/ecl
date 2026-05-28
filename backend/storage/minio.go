package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioStorage struct {
	client *minio.Client
}

func NewMinioStorage(endpoint, user, password string, secure bool) (*MinioStorage, error) {
	var transport http.RoundTripper
	if secure {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		}
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(user, password, ""),
		Secure:    secure,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MinIO: %w", err)
	}
	return &MinioStorage{client: client}, nil
}

func (m *MinioStorage) ensureBucket(ctx context.Context, bucket string) error {
	exists, err := m.client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		return m.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}
	return nil
}

func (m *MinioStorage) Health() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := m.client.ListBuckets(ctx)
	return err == nil
}

func (m *MinioStorage) Put(bucket, fnm string, binary []byte, tenantID ...string) error {
	ctx := context.Background()
	if err := m.ensureBucket(ctx, bucket); err != nil {
		return fmt.Errorf("ensure bucket %s: %w", bucket, err)
	}
	_, err := m.client.PutObject(ctx, bucket, fnm, bytes.NewReader(binary), int64(len(binary)), minio.PutObjectOptions{})
	return err
}

func (m *MinioStorage) Get(bucket, fnm string, tenantID ...string) ([]byte, error) {
	ctx := context.Background()
	obj, err := m.client.GetObject(ctx, bucket, fnm, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(obj); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *MinioStorage) Remove(bucket, fnm string, tenantID ...string) error {
	ctx := context.Background()
	return m.client.RemoveObject(ctx, bucket, fnm, minio.RemoveObjectOptions{})
}

func (m *MinioStorage) ObjExist(bucket, fnm string, tenantID ...string) bool {
	ctx := context.Background()
	_, err := m.client.StatObject(ctx, bucket, fnm, minio.StatObjectOptions{})
	return err == nil
}

func (m *MinioStorage) GetPresignedURL(bucket, fnm string, expires time.Duration, tenantID ...string) (string, error) {
	ctx := context.Background()
	url, err := m.client.PresignedGetObject(ctx, bucket, fnm, expires, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

func (m *MinioStorage) BucketExists(bucket string) bool {
	ctx := context.Background()
	exists, err := m.client.BucketExists(ctx, bucket)
	return err == nil && exists
}

func (m *MinioStorage) RemoveBucket(bucket string) error {
	ctx := context.Background()
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for obj := range m.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if obj.Err != nil {
				return
			}
			objectsCh <- obj
		}
	}()
	for err := range m.client.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return err.Err
		}
	}
	return m.client.RemoveBucket(ctx, bucket)
}

func (m *MinioStorage) Copy(srcBucket, srcPath, destBucket, destPath string) bool {
	ctx := context.Background()
	if err := m.ensureBucket(ctx, destBucket); err != nil {
		return false
	}
	srcOpts := minio.CopySrcOptions{Bucket: srcBucket, Object: srcPath}
	destOpts := minio.CopyDestOptions{Bucket: destBucket, Object: destPath}
	_, err := m.client.CopyObject(ctx, destOpts, srcOpts)
	return err == nil
}

func (m *MinioStorage) Move(srcBucket, srcPath, destBucket, destPath string) bool {
	if m.Copy(srcBucket, srcPath, destBucket, destPath) {
		_ = m.Remove(srcBucket, srcPath)
		return true
	}
	return false
}
