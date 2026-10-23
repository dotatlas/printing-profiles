// Package storage reads and writes objects in an S3-compatible bucket (Cloudflare R2 in production) with
// Signature V4 from the standard library, and makes presigned upload URLs. The upload routes use it.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrNotFound is the answer of Get for a key that the bucket does not have.
	ErrNotFound = errors.New("storage: no such object")
	// ErrTooLarge is the answer of Get for an object larger than the caller's limit.
	ErrTooLarge = errors.New("storage: object too large")
)

// Bucket is one bucket at an S3-compatible endpoint. Keys go in the path (path style), as R2 expects.
type Bucket struct {
	endpoint *url.URL
	name     string
	creds    credentials
	client   *http.Client
	now      func() time.Time
}

// NewBucket checks the endpoint and the bucket name and gives a Bucket. main calls it for the uploads and the
// files bucket when R2 is set up.
func NewBucket(endpoint, name, accessKey, secretKey string) (*Bucket, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("storage: the endpoint %q is not an http or https URL", endpoint)
	}
	if name == "" || strings.Contains(name, "/") {
		return nil, errors.New("storage: the bucket name is empty or has a slash")
	}
	return &Bucket{
		endpoint: u,
		name:     name,
		creds:    credentials{accessKey: accessKey, secretKey: secretKey, region: "auto"},
		client:   &http.Client{Timeout: 30 * time.Second},
		now:      time.Now,
	}, nil
}

// objectURL gives the URL of a key in the bucket.
func (b *Bucket) objectURL(key string) *url.URL {
	u := *b.endpoint
	u.Path = "/" + b.name + "/" + key
	return &u
}

// PresignPut gives a URL that accepts one PUT of exactly size bytes with this Content-Type until ttl ends. The
// create-upload route gives it to the browser, which never sees the bucket keys.
func (b *Bucket) PresignPut(key, contentType string, size int64, ttl time.Duration) string {
	headers := map[string]string{"Content-Length": strconv.FormatInt(size, 10), "Content-Type": contentType}
	return b.creds.presign(http.MethodPut, b.objectURL(key), headers, ttl, b.now()).String()
}

// Get reads an object, up to max bytes. It gives ErrNotFound for a missing key and ErrTooLarge for a larger
// object. The complete-upload route reads the uploaded file with it.
func (b *Bucket) Get(ctx context.Context, key string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.objectURL(key).String(), nil)
	if err != nil {
		return nil, fmt.Errorf("storage: get %s: %w", key, err)
	}
	b.creds.sign(req, emptyHash, b.now())
	res, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("storage: get %s: %w", key, err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("storage: get %s: status %d", key, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, fmt.Errorf("storage: get %s: %w", key, err)
	}
	if int64(len(body)) > max {
		return nil, ErrTooLarge
	}
	return body, nil
}

// Put writes an object with its Content-Type and Content-Disposition, which R2 sends back with every download.
// The complete-upload route writes the clean preset file with it.
func (b *Bucket) Put(ctx context.Context, key string, body []byte, contentType, disposition string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, b.objectURL(key).String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("storage: put %s: %w", key, err)
	}
	req.Header.Set("Content-Type", contentType)
	if disposition != "" {
		req.Header.Set("Content-Disposition", disposition)
	}
	b.creds.sign(req, sha256Hex(body), b.now())
	res, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("storage: put %s: %w", key, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("storage: put %s: status %d", key, res.StatusCode)
	}
	return nil
}
