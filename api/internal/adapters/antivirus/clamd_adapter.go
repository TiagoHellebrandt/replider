// Package antivirus talks to a ClamAV daemon over its stream protocol.
//
// clamd rather than a Go library because signature databases are the product
// here, not the matching engine: freshclam updates them daily, and a bundled
// scanner is out of date the week it ships.
package antivirus

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ClamdScanner speaks clamd's INSTREAM command.
type ClamdScanner struct {
	addr    string
	timeout time.Duration
	// maxBytes mirrors clamd's StreamMaxLength. Sending more makes the
	// daemon close the connection mid-transfer, which reads as a transport
	// error and would be recorded as a failed scan; refusing up front lets
	// it be recorded as skipped-because-too-large, which is the truth.
	maxBytes int
	dial     func(ctx context.Context, addr string) (net.Conn, error)
}

// DefaultMaxScanBytes matches clamd's out-of-the-box StreamMaxLength (25MB).
const DefaultMaxScanBytes = 25 * 1024 * 1024

// New returns a scanner, or nil when addr is empty.
//
// A typed nil would satisfy the interface and then fail on every call, so
// this returns the interface type and the caller can compare against nil —
// the same trap the Turnstile verifier documents.
func New(addr string, timeout time.Duration) domain.AttachmentScanner {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &ClamdScanner{
		addr:     addr,
		timeout:  timeout,
		maxBytes: DefaultMaxScanBytes,
		dial: func(ctx context.Context, a string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", a)
		},
	}
}

// Scan streams data to clamd and reads its verdict.
func (s *ClamdScanner) Scan(ctx context.Context, data []byte) (domain.ScanVerdict, error) {
	if len(data) == 0 {
		// Nothing to scan is not a clean scan. An empty attachment should
		// never have been stored, and calling it clean would be a verdict
		// about bytes that do not exist.
		return domain.ScanVerdict{Status: domain.ScanSkipped}, nil
	}
	if len(data) > s.maxBytes {
		return domain.ScanVerdict{Status: domain.ScanSkipped}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	conn, err := s.dial(ctx, s.addr)
	if err != nil {
		return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	// zINSTREAM: the "z" prefix asks clamd to NUL-terminate its reply, which
	// removes the ambiguity of reading until newline on a protocol that does
	// not always send one.
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd write command: %w", err)
	}

	// Chunks are length-prefixed with a 4-byte big-endian size; a zero-length
	// chunk ends the stream.
	const chunkSize = 64 * 1024
	for offset := 0; offset < len(data); offset += chunkSize {
		end := min(offset+chunkSize, len(data))
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(end-offset)) //nolint:gosec // G115: end-offset is at most chunkSize (64KiB)
		if _, err := conn.Write(header[:]); err != nil {
			return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd write size: %w", err)
		}
		if _, err := conn.Write(data[offset:end]); err != nil {
			return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd write chunk: %w", err)
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd end stream: %w", err)
	}

	reply, err := bufio.NewReader(conn).ReadString(0)
	if err != nil && reply == "" {
		return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd read: %w", err)
	}
	return parseClamdReply(reply)
}

// parseClamdReply turns "stream: Eicar-Test-Signature FOUND" into a verdict.
//
// Anything unrecognised is a FAILURE, not a pass. A scanner whose answer we
// cannot read has told us nothing, and the one outcome that must never come
// from confusion is "clean".
func parseClamdReply(reply string) (domain.ScanVerdict, error) {
	r := strings.TrimSpace(strings.Trim(reply, "\x00"))
	switch {
	case strings.HasSuffix(r, "OK"):
		return domain.ScanVerdict{Status: domain.ScanClean}, nil
	case strings.HasSuffix(r, "FOUND"):
		name := strings.TrimSpace(strings.TrimSuffix(r, "FOUND"))
		if i := strings.Index(name, ":"); i >= 0 {
			name = strings.TrimSpace(name[i+1:])
		}
		if name == "" {
			name = "desconhecido"
		}
		return domain.ScanVerdict{Status: domain.ScanInfected, Signature: name}, nil
	case strings.HasSuffix(r, "ERROR"), r == "":
		return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd error reply: %q", r)
	default:
		return domain.ScanVerdict{Status: domain.ScanFailed}, fmt.Errorf("clamd unrecognised reply: %q", r)
	}
}
