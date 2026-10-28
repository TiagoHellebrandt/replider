// Package media holds adapter-side audio/video utilities shared by the
// channel integrations. Today: ffmpeg-based transcoding so a voice note
// recorded by the dashboard's MediaRecorder (webm/opus on web,
// AAC-LC/m4a on native) renders as a proper voice-note bubble on
// WhatsApp + Telegram instead of a music-player or being rejected by
// Meta on link fetch.
package media

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// TranscodeToOggOpus converts an arbitrary audio container (m4a/mp3/
// wav/webm) into OGG-Opus by piping bytes through ffmpeg. Required
// because:
//   - Telegram's sendVoice only renders a voice-note bubble for
//     OGG-Opus payloads.
//   - WhatsApp's audio link path silently fails when the actual codec
//     does not match the Content-Type the bytes are served with — the
//     browser MediaRecorder happily writes webm/opus into a file the
//     dashboard ships as audio/mp4.
//
// 32 kbps mono mirrors what mobile clients produce for voice notes;
// keeps the file tiny and well inside the 16 MB cap WhatsApp + the
// 1 MB cap Telegram enforces. Bails after 8 s so a stalled transcode
// doesn't hold up the reply pipeline.
func TranscodeToOggOpus(ctx context.Context, in []byte) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-vn",
		"-c:a", "libopus",
		"-b:a", "32k",
		"-ac", "1",
		"-ar", "48000",
		"-f", "ogg",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(in)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w (%s)", err, strings.TrimSpace(errBuf.String()))
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced empty output")
	}
	return out.Bytes(), nil
}
