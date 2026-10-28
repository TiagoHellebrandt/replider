package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDetectNgrokURLFrom(t *testing.T) {
	type tunnelEntry struct {
		PublicURL string `json:"public_url"`
	}
	type tunnelsResponse struct {
		Tunnels []tunnelEntry `json:"tunnels"`
	}

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantResult string
	}{
		{
			name: "returns HTTPS tunnel URL",
			handler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(tunnelsResponse{
					Tunnels: []tunnelEntry{
						{PublicURL: "http://abc123.ngrok.io"},
						{PublicURL: "https://abc123.ngrok.io"},
					},
				})
			},
			wantResult: "https://abc123.ngrok.io",
		},
		{
			name: "picks first HTTPS tunnel when multiple exist",
			handler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(tunnelsResponse{
					Tunnels: []tunnelEntry{
						{PublicURL: "https://first.ngrok.io"},
						{PublicURL: "https://second.ngrok.io"},
					},
				})
			},
			wantResult: "https://first.ngrok.io",
		},
		{
			name: "returns empty when only HTTP tunnels exist",
			handler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(tunnelsResponse{
					Tunnels: []tunnelEntry{
						{PublicURL: "http://abc123.ngrok.io"},
					},
				})
			},
			wantResult: "",
		},
		{
			name: "returns empty when tunnels list is empty",
			handler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(tunnelsResponse{Tunnels: nil})
			},
			wantResult: "",
		},
		{
			name: "returns empty on invalid JSON response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("not json"))
			},
			wantResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			// Single retry with no sleep to keep tests fast.
			got := detectNgrokURLFrom(srv.URL+"/api/tunnels", 1, time.Millisecond)
			if got != tt.wantResult {
				t.Errorf("detectNgrokURLFrom() = %q, want %q", got, tt.wantResult)
			}
		})
	}
}

func TestDetectNgrokURLFrom_NoServer(t *testing.T) {
	// Point at an address that is not listening.
	got := detectNgrokURLFrom("http://127.0.0.1:0/api/tunnels", 1, time.Millisecond)
	if got != "" {
		t.Errorf("detectNgrokURLFrom() with no server = %q, want empty string", got)
	}
}

func TestDetectNgrokURLFrom_RetriesUntilAvailable(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			// First two attempts: return empty tunnels (simulating ngrok not ready).
			json.NewEncoder(w).Encode(map[string]interface{}{"tunnels": []interface{}{}})
			return
		}
		// Third attempt: return a valid HTTPS tunnel.
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tunnels": []map[string]string{
				{"public_url": "https://ready.ngrok.io"},
			},
		})
	}))
	defer srv.Close()

	got := detectNgrokURLFrom(srv.URL+"/api/tunnels", 5, time.Millisecond)
	if got != "https://ready.ngrok.io" {
		t.Errorf("detectNgrokURLFrom() = %q, want %q", got, "https://ready.ngrok.io")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

// TestParseLogLevel locks in US-029: LOG_LEVEL maps to the right slog level
// and any unset/unknown value defaults to info (prod-safe).
func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"  warn ": slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"bogus":   slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLogLevel(in); got != want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
