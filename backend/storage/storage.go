package storage

import "time"

// Storage defines the interface for object storage operations.
type Storage interface {
	Health() bool
	Put(bucket, fnm string, binary []byte, tenantID ...string) error
	Get(bucket, fnm string, tenantID ...string) ([]byte, error)
	Remove(bucket, fnm string, tenantID ...string) error
	ObjExist(bucket, fnm string, tenantID ...string) bool
	GetPresignedURL(bucket, fnm string, expires time.Duration, tenantID ...string) (string, error)
	BucketExists(bucket string) bool
	RemoveBucket(bucket string) error
	Copy(srcBucket, srcPath, destBucket, destPath string) bool
	Move(srcBucket, srcPath, destBucket, destPath string) bool
}
