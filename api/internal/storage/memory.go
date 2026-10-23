package storage

import (
	"context"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Memory is a bucket in memory, with the same methods as Bucket. The tests use it, and dev uses it when R2 is not
// set up. Its presigned URLs point at PutURL, a dev-only route of the API that writes into it.
type Memory struct {
	PutURL string // the base URL of the upload route; a key and the signed size follow it

	mu      sync.Mutex
	objects map[string]Object
}

// Object is one object of a Memory bucket, with the headers that a download gets.
type Object struct {
	Body        []byte
	ContentType string
	Disposition string
}

// PresignPut gives PutURL with the key, the size and the type, so the dev upload route can check them as R2 does.
func (m *Memory) PresignPut(key, contentType string, size int64, _ time.Duration) string {
	return m.PutURL + url.PathEscape(key) + "?size=" + strconv.FormatInt(size, 10) + "&type=" + url.QueryEscape(contentType)
}

// Get reads an object, with ErrNotFound and ErrTooLarge as Bucket gives them.
func (m *Memory) Get(_ context.Context, key string, max int64) ([]byte, error) {
	o, ok := m.Object(key)
	switch {
	case !ok:
		return nil, ErrNotFound
	case int64(len(o.Body)) > max:
		return nil, ErrTooLarge
	}
	return o.Body, nil
}

// Put writes an object with its headers.
func (m *Memory) Put(_ context.Context, key string, body []byte, contentType, disposition string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.objects == nil {
		m.objects = map[string]Object{}
	}
	m.objects[key] = Object{Body: append([]byte(nil), body...), ContentType: contentType, Disposition: disposition}
	return nil
}

// Object gives one object and whether it exists. The tests and the dev download route read with it.
func (m *Memory) Object(key string) (Object, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.objects[key]
	return o, ok
}
