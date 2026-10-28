package handlers

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/stretchr/testify/assert"
)

// captureLogs swaps the default slog handler for a buffer-backed JSON
// handler so tests can assert on the structured fields logMetaError
// emits. Restored on cleanup.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	buf := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// Catalog hit must produce one structured log line containing the full
// Meta envelope so on-call can triage straight from the log.
func TestMapMetaErrorAction_LogsStructuredFieldsOnCatalogHit(t *testing.T) {
	buf := captureLogs(t)
	err := &waadapter.MetaError{
		Code: 100, Subcode: 2388001,
		UserMsg:    "Please verify ownership of this phone number.",
		FBTraceID:  "AbCdEf123",
		HTTPStatus: 400,
	}
	status, appErr := mapMetaErrorAction(err, waadapter.ActionRegisterPhone, "WA_REGISTER_FAILED", "fallback")

	assert.Equal(t, "WA_OWNERSHIP_NOT_VERIFIED", appErr.Code)
	assert.NotZero(t, status)

	logs := buf.String()
	for _, want := range []string{
		`"action":"register_phone"`,
		`"kind":"WA_OWNERSHIP_NOT_VERIFIED"`,
		`"catalog_hit":true`,
		`"meta_code":100`,
		`"meta_subcode":2388001`,
		`"fb_trace_id":"AbCdEf123"`,
	} {
		assert.True(t, strings.Contains(logs, want), "expected log to contain %q, got %s", want, logs)
	}
}

// Catalog miss must log at ERROR level + carry catalog_hit:false so
// log shippers can filter unknown codes for catalog additions.
func TestMapMetaErrorAction_LogsErrorOnCatalogMiss(t *testing.T) {
	buf := captureLogs(t)
	err := &waadapter.MetaError{Code: 9999999, FBTraceID: "miss-trace"}
	mapMetaErrorAction(err, waadapter.ActionVerifyCode, "WA_VERIFY_CODE_FAILED", "fallback")

	logs := buf.String()
	assert.True(t, strings.Contains(logs, `"catalog_hit":false`), "miss should log catalog_hit:false; got %s", logs)
	assert.True(t, strings.Contains(logs, `"level":"ERROR"`), "miss should log at ERROR level; got %s", logs)
	assert.True(t, strings.Contains(logs, `"action":"verify_code"`), "miss should log action; got %s", logs)
}

// Non-Meta transport errors (timeouts, network failures) still get a
// log line so we don't lose any failure path.
func TestMapMetaErrorAction_LogsTransportError(t *testing.T) {
	buf := captureLogs(t)
	mapMetaErrorAction(errors.New("connection refused"), waadapter.ActionSendTemplate, "WA_TEMPLATE_SEND_FAILED", "boom")

	logs := buf.String()
	assert.True(t, strings.Contains(logs, `"transport_error":"connection refused"`), "transport log missing; got %s", logs)
	assert.True(t, strings.Contains(logs, `"action":"send_template"`), "missing action; got %s", logs)
}
