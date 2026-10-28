package e2e

import (
	"context"
	"errors"
	"sync"
	"time"
)

// testAttachmentStore stands in for GCS so the suite needs no credentials and
// no network. It is a package-level singleton because the router is built once
// per suite, and tests that care about the bytes read them back through it.
var testAttachmentStore = &memAttachmentStore{objects: map[string][]byte{}}

type memAttachmentStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memAttachmentStore) UploadBytes(_ context.Context, path string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Copy: the caller owns the slice it passed and may reuse it.
	stored := make([]byte, len(data))
	copy(stored, data)
	m.objects[path] = stored
	return nil
}

func (m *memAttachmentStore) GenerateReadSignedURL(_ context.Context, path string, _ time.Duration) (string, error) {
	return "https://signed.test/" + path, nil
}

func (m *memAttachmentStore) DownloadObject(_ context.Context, path string, _ int64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[path]
	if !ok {
		return nil, errors.New("object not found")
	}
	return data, nil
}

// count reports how many objects live under a prefix, for assertions.
func (m *memAttachmentStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}
