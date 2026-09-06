// Package storage abstracts S3-compatible object storage (TECH_SPEC §4.3).
//
// Security invariants (execution plan B06):
//   - All URLs are short-lived presigned URLs against a PRIVATE bucket; the
//     service never produces public or permanent links.
//   - Object keys are generated server-side from random UUIDs; client-supplied
//     keys are never trusted for put or get.
//   - The interface exists so verification (and the future production cloud
//     provider) can be swapped without touching domain code (docs/03 §5.2).
package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

// ObjectInfo is what validation needs to know about an uploaded object.
type ObjectInfo struct {
	Size        int64
	ContentType string
	FirstBytes  []byte // leading bytes for magic-number sniffing
}

// PresignedUpload is returned by NewUploadURL.
type PresignedUpload struct {
	ObjectKey string
	URL       string
	ExpiresAt time.Time
}

// Store is the object-storage port. Implementations must scope every call by
// the object key namespace derived from the actor's organization.
type Store interface {
	// NewUploadURL presigns a PUT for a fresh, server-generated object key.
	NewUploadURL(ctx context.Context, actor auth.Actor, projectID uuid.UUID, contentType string, sizeBytes int64, ttl time.Duration) (PresignedUpload, error)
	// Stat returns size, content type and the leading bytes of an object.
	Stat(ctx context.Context, objectKey string) (ObjectInfo, error)
	// Read returns up to limit bytes (used for server-side hash verification;
	// oversized content must error, never be truncated silently).
	Read(ctx context.Context, objectKey string, limit int64) ([]byte, error)
	// PresignGet issues a short-lived download URL (private reads only).
	PresignGet(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
}

// MinIOStore is the local/dev implementation (TECH_SPEC §4.3: MinIO locally,
// cloud S3 in production behind the same interface).
type MinIOStore struct {
	client *minio.Client
	bucket string
}

// NewMinIOStore connects to an S3-compatible endpoint.
func NewMinIOStore(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	return &MinIOStore{client: client, bucket: bucket}, nil
}

// NewUploadURL generates `<org>/<project>/<uuid>` keys. The random middle
// segment makes keys unguessable; the org prefix keeps the namespace tidy and
// lets lifecycle rules act per tenant later.
func (s *MinIOStore) NewUploadURL(ctx context.Context, actor auth.Actor, projectID uuid.UUID, contentType string, sizeBytes int64, ttl time.Duration) (PresignedUpload, error) {
	oid, err := uuid.NewV7()
	if err != nil {
		return PresignedUpload{}, err
	}
	key := fmt.Sprintf("%s/%s/%s", actor.OrganizationID, projectID, oid)

	// Content-Type must be pinned in the signature so the stored object's
	// metadata matches what validation expects later.
	reqParams := url.Values{"Content-Type": {contentType}}
	presigned, err := s.client.Presign(ctx, http.MethodPut, s.bucket, key, ttl, reqParams)
	if err != nil {
		return PresignedUpload{}, fmt.Errorf("presign put: %w", err)
	}
	return PresignedUpload{
		ObjectKey: key,
		URL:       presigned.String(),
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

// EnsureBucket creates the configured bucket when missing. Dev/test
// convenience only: production buckets are provisioned by deployment, not
// application code (least privilege).
func (s *MinIOStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("bucket probe: %w", err)
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

// Stat fetches object metadata and the leading 512 bytes for magic sniffing.
func (s *MinIOStore) Stat(ctx context.Context, objectKey string) (ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat object: %w", err)
	}
	head := make([]byte, 0, 512)
	obj, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("open object: %w", err)
	}
	defer obj.Close()
	buf := make([]byte, 512)
	n, _ := obj.Read(buf)
	head = append(head, buf[:n]...)

	return ObjectInfo{Size: info.Size, ContentType: info.ContentType, FirstBytes: head}, nil
}

// Read downloads up to limit bytes; a larger object is a validation failure
// (the declared size gate should catch this earlier — this is defense in depth).
func (s *MinIOStore) Read(ctx context.Context, objectKey string, limit int64) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}
	defer obj.Close()

	content := make([]byte, 0, limit)
	buf := make([]byte, 64*1024)
	var total int64
	for {
		if total > limit {
			return nil, fmt.Errorf("object exceeds %d bytes", limit)
		}
		n, err := obj.Read(buf)
		content = append(content, buf[:n]...)
		total += int64(n)
		if err != nil {
			if err.Error() == "EOF" {
				return content, nil
			}
			return nil, err
		}
	}
}

func (s *MinIOStore) PresignGet(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	presigned, err := s.client.Presign(ctx, http.MethodGet, s.bucket, objectKey, ttl, url.Values{})
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return presigned.String(), nil
}
