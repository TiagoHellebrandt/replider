package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// envIntDefault reads an integer env var, falling back to def when unset or
// unparseable (logged, never fatal — a typo in an ops knob must not stop the
// API from booting).
func envIntDefault(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		slog.Warn("config: invalid integer, using default", "key", key, "value", raw, "default", def)
		return def
	}
	return n
}
