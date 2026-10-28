package antivirus

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClamd speaks just enough of the protocol to drive the adapter: it
// drains a zINSTREAM upload and answers with a canned reply.
func fakeClamd(t *testing.T, reply string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		cmd := make([]byte, len("zINSTREAM\x00"))
		if _, err := io.ReadFull(conn, cmd); err != nil {
			return
		}
		for {
			var size [4]byte
			if _, err := io.ReadFull(conn, size[:]); err != nil {
				return
			}
			n := binary.BigEndian.Uint32(size[:])
			if n == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, conn, int64(n)); err != nil {
				return
			}
		}
		_, _ = conn.Write([]byte(reply))
	}()
	return ln.Addr().String()
}

func TestScan_CleanFile(t *testing.T) {
	s := New(fakeClamd(t, "stream: OK\x00"), 5*time.Second)
	require.NotNil(t, s)

	v, err := s.Scan(context.Background(), []byte("um pdf inofensivo"))
	require.NoError(t, err)
	assert.Equal(t, domain.ScanClean, v.Status)
	assert.True(t, v.Status.Downloadable())
}

func TestScan_InfectedFileNamesTheSignature(t *testing.T) {
	s := New(fakeClamd(t, "stream: Eicar-Test-Signature FOUND\x00"), 5*time.Second)

	v, err := s.Scan(context.Background(), []byte("payload"))
	require.NoError(t, err)
	assert.Equal(t, domain.ScanInfected, v.Status)
	// "infectado" with no name is impossible to argue with when it is a
	// false positive.
	assert.Equal(t, "Eicar-Test-Signature", v.Signature)
	assert.False(t, v.Status.Downloadable())
}

// The one outcome that must never come from confusion is "clean".
func TestScan_UnrecognisedReplyIsFailedNotClean(t *testing.T) {
	s := New(fakeClamd(t, "wat\x00"), 5*time.Second)

	v, err := s.Scan(context.Background(), []byte("x"))
	require.Error(t, err)
	assert.Equal(t, domain.ScanFailed, v.Status)
	assert.False(t, v.Status.Downloadable())
}

func TestScan_DaemonDownIsFailedNotClean(t *testing.T) {
	// Port 1 on loopback: nothing listens, connection refused immediately.
	s := New("127.0.0.1:1", 2*time.Second)

	v, err := s.Scan(context.Background(), []byte("x"))
	require.Error(t, err)
	assert.Equal(t, domain.ScanFailed, v.Status)
	assert.False(t, v.Status.Downloadable())
}

// A file past the daemon's stream limit is skipped, not failed: clamd would
// hang up mid-transfer and the truth is "we did not look", not "the scanner
// broke".
func TestScan_OversizedFileIsSkipped(t *testing.T) {
	s := New(fakeClamd(t, "stream: OK\x00"), 5*time.Second).(*ClamdScanner)
	s.maxBytes = 8

	v, err := s.Scan(context.Background(), []byte("muito maior que oito bytes"))
	require.NoError(t, err)
	assert.Equal(t, domain.ScanSkipped, v.Status)
	assert.False(t, v.Status.Downloadable(),
		"skipped means nobody looked — it must not be served as safe")
}

func TestScan_EmptyPayloadIsSkipped(t *testing.T) {
	s := New(fakeClamd(t, "stream: OK\x00"), 5*time.Second)

	v, err := s.Scan(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, domain.ScanSkipped, v.Status)
}

// No address configured means no scanner — a typed nil would satisfy the
// interface and then fail on every call.
func TestNew_EmptyAddressReturnsNilInterface(t *testing.T) {
	assert.Nil(t, New("", time.Second))
	assert.Nil(t, New("   ", time.Second))
}

// Only a real clean verdict may be served.
func TestScanStatus_OnlyCleanIsDownloadable(t *testing.T) {
	assert.True(t, domain.ScanClean.Downloadable())
	for _, s := range []domain.ScanStatus{
		domain.ScanPending, domain.ScanInfected, domain.ScanSkipped, domain.ScanFailed,
	} {
		assert.False(t, s.Downloadable(), "status %q must not be downloadable", s)
	}
}

// Chunking is invisible to the caller but a payload larger than one chunk is
// exactly where a length-prefix bug would show up.
func TestScan_MultiChunkPayload(t *testing.T) {
	s := New(fakeClamd(t, "stream: OK\x00"), 10*time.Second)

	big := make([]byte, 200*1024)
	for i := range big {
		big[i] = byte(i % 251)
	}
	v, err := s.Scan(context.Background(), big)
	require.NoError(t, err)
	assert.Equal(t, domain.ScanClean, v.Status)
}
