package storage

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiskCache_PutBytesRoundTrip(t *testing.T) {
	c, err := NewDiskCache(t.TempDir(), 1<<20)
	require.NoError(t, err)

	_, _, _, ok := c.Get("accounts/a/attachments/m/x")
	assert.False(t, ok, "cold cache misses")

	c.PutBytes("accounts/a/attachments/m/x", []byte("foto"), "image/jpeg")

	f, size, ct, ok := c.Get("accounts/a/attachments/m/x")
	require.True(t, ok)
	defer func() { _ = f.Close() }()
	assert.Equal(t, int64(4), size)
	assert.Equal(t, "image/jpeg", ct)
	data, _ := io.ReadAll(f)
	assert.Equal(t, "foto", string(data))
}

// The streaming path: a committed writer becomes an entry, a discarded one
// leaves nothing — a half-written blob must never be served as the object.
func TestDiskCache_WriterCommitAndDiscard(t *testing.T) {
	c, err := NewDiskCache(t.TempDir(), 1<<20)
	require.NoError(t, err)

	w, err := c.NewWriter()
	require.NoError(t, err)
	_, _ = w.Write([]byte("meio"))
	w.Discard()
	_, _, _, ok := c.Get("obj-a")
	assert.False(t, ok)

	w2, err := c.NewWriter()
	require.NoError(t, err)
	_, _ = w2.Write([]byte("inteiro"))
	w2.Commit("obj-b", "application/pdf")
	f, _, ct, ok := c.Get("obj-b")
	require.True(t, ok)
	_ = f.Close()
	assert.Equal(t, "application/pdf", ct)
}

func TestDiskCache_EvictsOldestWhenOverBudget(t *testing.T) {
	// Budget fits ~2 of the 3 entries.
	c, err := NewDiskCache(t.TempDir(), 250)
	require.NoError(t, err)

	c.PutBytes("velho", []byte(strings.Repeat("a", 100)), "")
	// Eviction is oldest-mtime-first, and three writes on a fast runner land
	// inside one filesystem timestamp tick — so without an explicit age this
	// test asks the cache to break a three-way tie and then asserts which way
	// it broke. CI failed on exactly that, keeping "velho" and dropping
	// "novo". An hour is not a wait: it is a stamp.
	hourAgo := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(c.keyPath("velho"), hourAgo, hourAgo))

	c.PutBytes("meio", []byte(strings.Repeat("b", 100)), "")
	c.PutBytes("novo", []byte(strings.Repeat("c", 100)), "")

	_, _, _, oldOK := c.Get("velho")
	f, _, _, newOK := c.Get("novo")
	if newOK {
		_ = f.Close()
	}
	assert.False(t, oldOK, "oldest entry evicted")
	assert.True(t, newOK, "newest entry kept")
}
