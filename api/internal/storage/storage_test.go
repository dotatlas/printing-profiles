package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeR2 stands in for the R2 endpoint: it keeps objects in memory and records each request's signed headers.
type fakeR2 struct {
	objects map[string][]byte
	types   map[string]string
	last    *http.Request
}

func (f *fakeR2) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.last = r
	if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=key/") {
		http.Error(w, "unsigned", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		body, ok := f.objects[r.URL.Path]
		if !ok {
			http.Error(w, "<Error><Code>NoSuchKey</Code></Error>", http.StatusNotFound)
			return
		}
		w.Write(body)
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("X-Amz-Content-Sha256") != sha256Hex(body) {
			http.Error(w, "bad hash", http.StatusBadRequest)
			return
		}
		f.objects[r.URL.Path] = body
		f.types[r.URL.Path] = r.Header.Get("Content-Type") + "|" + r.Header.Get("Content-Disposition")
	}
}

func newTestBucket(t *testing.T) (*Bucket, *fakeR2) {
	t.Helper()
	fake := &fakeR2{objects: map[string][]byte{}, types: map[string]string{}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	b, err := NewBucket(srv.URL, "files", "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	return b, fake
}

func TestPutThenGetAnObject(t *testing.T) {
	b, fake := newTestBucket(t)
	ctx := context.Background()
	if err := b.Put(ctx, "presets/abc.ini", []byte("layer_height = 0.2\n"), "text/plain; charset=utf-8", `attachment; filename="preset.ini"`); err != nil {
		t.Fatal(err)
	}
	if got := fake.types["/files/presets/abc.ini"]; got != `text/plain; charset=utf-8|attachment; filename="preset.ini"` {
		t.Fatalf("stored type and disposition %q", got)
	}
	body, err := b.Get(ctx, "presets/abc.ini", 1024)
	if err != nil || string(body) != "layer_height = 0.2\n" {
		t.Fatalf("get: %q, %v", body, err)
	}
}

func TestGetSaysNotFoundAndRefusesAnObjectOverTheLimit(t *testing.T) {
	b, fake := newTestBucket(t)
	ctx := context.Background()
	if _, err := b.Get(ctx, "missing", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing object: %v", err)
	}
	fake.objects["/files/big"] = make([]byte, 11)
	if _, err := b.Get(ctx, "big", 10); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("an object over the limit: %v", err)
	}
}

func TestPresignPutPointsAtTheKeyAndExpires(t *testing.T) {
	b, _ := newTestBucket(t)
	b.now = func() time.Time { return exampleTime }
	u := b.PresignPut("u/abc", "application/octet-stream", 512, 10*time.Minute)
	if !strings.Contains(u, "/files/u/abc?") || !strings.Contains(u, "X-Amz-Expires=600") || !strings.Contains(u, "content-length%3Bcontent-type%3Bhost") {
		t.Fatalf("url %s", u)
	}
}

func TestNewBucketNeedsAnHTTPEndpointAndABucketName(t *testing.T) {
	if _, err := NewBucket("ftp://example.com", "files", "key", "secret"); err == nil {
		t.Fatal("accepted an ftp endpoint")
	}
	if _, err := NewBucket("https://account.r2.cloudflarestorage.com", "", "key", "secret"); err == nil {
		t.Fatal("accepted an empty bucket name")
	}
}
