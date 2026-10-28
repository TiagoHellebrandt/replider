package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DiskCache is a local byte cache for immutable GCS objects.
//
// It exists because of geography: the VPS reaches the bucket at ~4Mbps, so a
// 4.3MB inline photo cost ~9 seconds EVERY time someone opened the email that
// carries it. The objects are immutable (every writer mints a fresh ULID per
// upload), so a local copy is never stale — the only questions are disk space
// and eviction, both answered here.
//
// Two write paths fill it: the media proxy tees whatever it streams (first
// reader pays the GCS fetch, everyone after reads local disk) and the email
// ingest pre-warms attachments at the moment they arrive, when the bytes are
// already in memory — so even the FIRST open of an email serves its images
// from local disk.
//
// The content type rides in a sidecar file (<key>.ct): the proxy must answer
// with the stored type, and encoding it into the blob would make the blob no
// longer the object's bytes.
type DiskCache struct {
	dir      string
	maxBytes int64
	mu       sync.Mutex
}

// NewDiskCache prepares the cache directory. maxBytes bounds the TOTAL cache
// size; eviction is oldest-mtime-first (Get touches mtime, making it LRU).
func NewDiskCache(dir string, maxBytes int64) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &DiskCache{dir: dir, maxBytes: maxBytes}, nil
}

func (c *DiskCache) keyPath(objectPath string) string {
	sum := sha256.Sum256([]byte(objectPath))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:]))
}

// Get opens a cached object. The caller owns the returned file. A hit
// refreshes the entry's mtime so eviction is least-recently-USED, not
// least-recently-written.
func (c *DiskCache) Get(objectPath string) (f *os.File, size int64, contentType string, ok bool) {
	p := c.keyPath(objectPath)
	// The path is derived from a sha256 of the object key inside our own
	// cache dir — no user-controlled traversal is possible.
	file, err := os.Open(p) //nolint:gosec // G304: path is dir + hex(sha256)
	if err != nil {
		return nil, 0, "", false
	}
	st, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, "", false
	}
	_ = os.Chtimes(p, time.Now(), time.Now())
	if ct, ctErr := os.ReadFile(p + ".ct"); ctErr == nil { //nolint:gosec // G304: same derived path
		contentType = string(ct)
	}
	return file, st.Size(), contentType, true
}

// PutBytes stores an object it already holds in memory (the ingest path).
// Best-effort: a failure logs and the cache simply stays cold for this object.
func (c *DiskCache) PutBytes(objectPath string, data []byte, contentType string) {
	tmp, err := os.CreateTemp(c.dir, "put-*")
	if err != nil {
		slog.Warn("media cache: temp create failed", "error", err)
		return
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		slog.Warn("media cache: write failed", "error", err)
		return
	}
	_ = tmp.Close()
	c.commit(tmp.Name(), objectPath, contentType)
}

// NewWriter opens a temp file the streaming path tees into. Call Commit on a
// complete copy, Discard on an aborted one — a half-written blob must never
// become a cache entry.
func (c *DiskCache) NewWriter() (*CacheWriter, error) {
	tmp, err := os.CreateTemp(c.dir, "tee-*")
	if err != nil {
		return nil, err
	}
	return &CacheWriter{cache: c, tmp: tmp}, nil
}

type CacheWriter struct {
	cache *DiskCache
	tmp   *os.File
}

func (w *CacheWriter) Write(p []byte) (int, error) { return w.tmp.Write(p) }

func (w *CacheWriter) Commit(objectPath, contentType string) {
	_ = w.tmp.Close()
	w.cache.commit(w.tmp.Name(), objectPath, contentType)
}

func (w *CacheWriter) Discard() {
	_ = w.tmp.Close()
	_ = os.Remove(w.tmp.Name())
}

// commit renames the finished temp file into place and evicts if over budget.
func (c *DiskCache) commit(tmpName, objectPath, contentType string) {
	p := c.keyPath(objectPath)
	if contentType != "" {
		if err := os.WriteFile(p+".ct", []byte(contentType), 0o600); err != nil {
			slog.Warn("media cache: content-type sidecar failed", "error", err)
		}
	}
	if err := os.Rename(tmpName, p); err != nil {
		_ = os.Remove(tmpName)
		slog.Warn("media cache: rename failed", "error", err)
		return
	}
	c.evictIfOver()
}

// evictIfOver removes oldest-mtime entries until the cache fits its budget.
// Coarse and rare by design — one pass per commit, never per read.
func (c *DiskCache) evictIfOver() {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type item struct {
		name  string
		size  int64
		mtime int64
	}
	var items []item
	var total int64
	for _, e := range entries {
		info, ierr := e.Info()
		if ierr != nil || e.IsDir() {
			continue
		}
		total += info.Size()
		if filepath.Ext(e.Name()) == ".ct" {
			continue // evicted alongside its blob
		}
		items = append(items, item{e.Name(), info.Size(), info.ModTime().UnixNano()})
	}
	if total <= c.maxBytes {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mtime < items[j].mtime })
	for _, it := range items {
		if total <= c.maxBytes {
			break
		}
		_ = os.Remove(filepath.Join(c.dir, it.name))
		_ = os.Remove(filepath.Join(c.dir, it.name+".ct"))
		total -= it.size
	}
}
