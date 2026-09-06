package evidence

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/storage"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/testutil"
)

// Integration suite: REAL PostgreSQL + REAL MinIO (execution plan §6: 上传/权限
// 验证必须覆盖签名、对象校验与失败路径). Requires:
//
//	TEST_DATABASE_URL     throwaway PostgreSQL DSN (migrates up)
//	MINIO_TEST_ENDPOINT   host:port of a THROWAWAY MinIO (bucket auto-created)
//
// Either missing → loud skip; CI provides both.
func intEnv(t *testing.T) (*Service, *store.DB, auth.Actor) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	endpoint := os.Getenv("MINIO_TEST_ENDPOINT")
	if dsn == "" || endpoint == "" {
		t.Skip("TEST_DATABASE_URL / MINIO_TEST_ENDPOINT not set: integration suite needs throwaway PostgreSQL and MinIO")
	}

	pool := testutil.ConnectPG(t, dsn)
	db := &store.DB{Pool: pool}
	testutil.MigrateTestDB(t, dsn)

	st, err := storage.NewMinIOStore(endpoint, "arrival", "arrival_dev_password", "arrivalready-evidence", false)
	if err != nil {
		t.Fatalf("storage client: %v", err)
	}
	// CI starts the MinIO container without a healthcheck (newer images have
	// no curl/shell), so readiness is polled here: BucketExists up to 15s.
	deadline := time.Now().Add(15 * time.Second)
	var lastErr error
	for {
		if err := st.EnsureBucket(context.Background()); err == nil {
			break
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			t.Fatalf("minio not ready within 15s: %v", lastErr)
		}
		time.Sleep(500 * time.Millisecond)
	}

	svc := NewService(db, st)
	actor := testutil.SeedOrgUser(t, db.Pool, fmt.Sprintf("owner-%d@org.test", time.Now().UnixNano()))
	return svc, db, actor
}

func jpegBytes(size int) []byte {
	b := make([]byte, size)
	b[0], b[1], b[2] = 0xFF, 0xD8, 0xFF // JPEG SOI marker
	_, _ = rand.Read(b[3:])
	return b
}

func pngBytes(size int) []byte {
	b := make([]byte, size)
	copy(b, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) // PNG signature
	return b
}

func putToPresigned(t *testing.T, url string, body []byte, contentType string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT to presigned url: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyText, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT returned %d: %s", resp.StatusCode, bodyText)
	}
}

func shaOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestUploadHappyPathAndIdempotency(t *testing.T) {
	svc, db, actor := intEnv(t)
	ctx := context.Background()
	project, err := db.CreateProject(ctx, actor, store.CreateProjectInput{
		Name: "证据闭环测试", EntityType: "restaurant", TargetLocale: "en-US",
	})
	if err != nil {
		t.Fatal(err)
	}

	content := jpegBytes(2048)
	// 1. CreateUpload → presigned PUT → real upload → CompleteUpload → READY.
	ev, presigned, err := svc.CreateUpload(ctx, actor, project.ID, CreateUploadInput{
		Type: "image", ContentType: "image/jpeg", SizeBytes: int64(len(content)),
		JourneyStage: "act", SourceNote: "菜单正面",
	})
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}
	putToPresigned(t, presigned.URL, content, "image/jpeg")

	fingerprint := "fp-" + presigned.ObjectKey
	ready, err := svc.CompleteUpload(ctx, actor, ev.ID, shaOf(content), fingerprint)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if ready.ProcessingStatus != "READY" || ready.SHA256 != shaOf(content) {
		t.Fatalf("expected READY with hash, got %+v", ready)
	}

	// 2. Exact replay (same fingerprint + hash) returns the same row.
	replay, err := svc.CompleteUpload(ctx, actor, ev.ID, shaOf(content), fingerprint)
	if err != nil || replay.ID != ev.ID || replay.ProcessingStatus != "READY" {
		t.Fatalf("idempotent replay failed: %+v err=%v", replay, err)
	}

	// 3. Same evidence id with a DIFFERENT fingerprint is a conflict.
	if _, err := svc.CompleteUpload(ctx, actor, ev.ID, shaOf(content), "fp-different"); err == nil {
		t.Fatal("divergent re-completion must conflict")
	}

	// 4. Download URL only for READY.
	if url, err := svc.DownloadURL(ctx, actor, ev.ID); err != nil || url == "" {
		t.Fatalf("download url for READY: %q err=%v", url, err)
	}
}

func TestUploadQuarantinePaths(t *testing.T) {
	svc, db, actor := intEnv(t)
	ctx := context.Background()
	project, err := db.CreateProject(ctx, actor, store.CreateProjectInput{
		Name: "隔离路径测试", EntityType: "restaurant", TargetLocale: "en-US",
	})
	if err != nil {
		t.Fatal(err)
	}

	newPending := func(contentType string, size int64) (uuid.UUID, storage.PresignedUpload) {
		ev, presigned, err := svc.CreateUpload(ctx, actor, project.ID, CreateUploadInput{
			Type: "image", ContentType: contentType, SizeBytes: size, JourneyStage: "pay",
		})
		if err != nil {
			t.Fatal(err)
		}
		return ev.ID, presigned
	}

	t.Run("magic bytes mismatch", func(t *testing.T) {
		png := pngBytes(1024)
		id, presigned := newPending("image/jpeg", int64(len(png)))
		putToPresigned(t, presigned.URL, png, "image/jpeg")

		quarantined, err := svc.CompleteUpload(ctx, actor, id, shaOf(png), "fp-png-as-jpeg")
		if err != nil {
			t.Fatalf("quarantine path must not error, it records a state: %v", err)
		}
		if quarantined.ProcessingStatus != "QUARANTINED" || quarantined.QuarantineReason == "" {
			t.Fatalf("expected QUARANTINED with reason, got %+v", quarantined)
		}
		if _, err := svc.DownloadURL(ctx, actor, id); err == nil {
			t.Fatal("QUARANTINED evidence must not get download URLs")
		}
	})

	t.Run("never uploaded", func(t *testing.T) {
		id, _ := newPending("image/png", 512)
		quarantined, err := svc.CompleteUpload(ctx, actor, id, "deadbeef", "fp-never-uploaded")
		if err != nil {
			t.Fatal(err)
		}
		if quarantined.ProcessingStatus != "QUARANTINED" {
			t.Fatalf("missing object must quarantine, got %s", quarantined.ProcessingStatus)
		}
	})

	t.Run("hash mismatch", func(t *testing.T) {
		jpg := jpegBytes(1024)
		id, presigned := newPending("image/jpeg", int64(len(jpg)))
		putToPresigned(t, presigned.URL, jpg, "image/jpeg")

		quarantined, err := svc.CompleteUpload(ctx, actor, id, "not-the-real-hash", "fp-hash-mismatch")
		if err != nil {
			t.Fatal(err)
		}
		if quarantined.ProcessingStatus != "QUARANTINED" {
			t.Fatalf("hash mismatch must quarantine, got %s", quarantined.ProcessingStatus)
		}
	})

	t.Run("size mismatch", func(t *testing.T) {
		jpg := jpegBytes(512)
		id, presigned := newPending("image/jpeg", 999999) // declared ≠ actual
		putToPresigned(t, presigned.URL, jpg, "image/jpeg")

		quarantined, err := svc.CompleteUpload(ctx, actor, id, shaOf(jpg), "fp-size-mismatch")
		if err != nil {
			t.Fatal(err)
		}
		if quarantined.ProcessingStatus != "QUARANTINED" {
			t.Fatalf("size mismatch must quarantine, got %s", quarantined.ProcessingStatus)
		}
	})
}

func TestCrossOrgEvidenceIsolation(t *testing.T) {
	svc, db, actor := intEnv(t)
	ctx := context.Background()
	project, err := db.CreateProject(ctx, actor, store.CreateProjectInput{
		Name: "隔离项目", EntityType: "retail", TargetLocale: "ja-JP",
	})
	if err != nil {
		t.Fatal(err)
	}
	other := testutil.SeedOrgUser(t, db.Pool, fmt.Sprintf("outsider-%d@other.test", time.Now().UnixNano()))

	ev, _, err := svc.CreateUpload(ctx, actor, project.ID, CreateUploadInput{
		Type: "image", ContentType: "image/png", SizeBytes: 512,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Foreign actor: complete/get/list must all behave as "not found".
	if _, err := svc.CompleteUpload(ctx, other, ev.ID, "x", "fp"); err == nil {
		t.Fatal("cross-org complete must fail")
	}
	if _, err := svc.DownloadURL(ctx, other, ev.ID); err == nil {
		t.Fatal("cross-org download must fail")
	}
	if _, err := db.GetProject(ctx, other, project.ID); err == nil {
		t.Fatal("cross-org project read must fail")
	}
}
