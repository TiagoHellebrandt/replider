package whatsapp

import (
	"errors"
	"testing"
)

// Catalog must resolve the production-shape Meta envelope
// (Code=100 + Subcode=2388001) to the action-aware Kind so the
// dashboard sees WA_OWNERSHIP_NOT_VERIFIED instead of the generic
// INVALID_PARAM that Code=100 alone would yield.
func TestResolveMetaError_OwnershipFlavour(t *testing.T) {
	err := &MetaError{Code: 100, Subcode: 2388001, UserMsg: "Please verify ownership of this phone number."}
	_, entry, ok := ResolveMetaError(err, ActionRegisterPhone)
	if !ok {
		t.Fatalf("expected catalog hit, got miss")
	}
	if entry.Kind != "WA_OWNERSHIP_NOT_VERIFIED" {
		t.Errorf("kind = %q, want WA_OWNERSHIP_NOT_VERIFIED", entry.Kind)
	}
	if entry.DefaultMessage == "" {
		t.Error("expected non-empty default message")
	}
}

// Same Subcode without the user_msg signal means the 2FA flavour;
// the catalog must NOT cross-pollinate the two.
func TestResolveMetaError_2FAFlavour(t *testing.T) {
	err := &MetaError{Code: 100, Subcode: 2388001, UserMsg: "two-step verification still on"}
	_, entry, ok := ResolveMetaError(err, ActionRegisterPhone)
	if !ok || entry.Kind != "WA_CERT_2FA_BLOCKED" {
		t.Errorf("got kind=%q ok=%v, want WA_CERT_2FA_BLOCKED", entry.Kind, ok)
	}
}

// Action-scoped entries must beat Code-only entries: Code=100 alone
// resolves to INVALID_PARAM, but Code=100 with action=verify_code
// must resolve to the dedicated WA_VERIFY_CODE_INVALID so the SMS
// flow can show "código incorreto ou expirado" instead of "Invalid
// parameter".
func TestResolveMetaError_ActionScopedOverridesGeneric(t *testing.T) {
	err := &MetaError{Code: 100}
	_, generic, _ := ResolveMetaError(err, ActionUnknown)
	if generic.Kind != "INVALID_PARAM" {
		t.Errorf("generic kind = %q, want INVALID_PARAM", generic.Kind)
	}
	_, scoped, _ := ResolveMetaError(err, ActionVerifyCode)
	if scoped.Kind != "WA_VERIFY_CODE_INVALID" {
		t.Errorf("verify_code kind = %q, want WA_VERIFY_CODE_INVALID", scoped.Kind)
	}
}

// Non-Meta errors (network failures, timeouts) must return ok=false
// so callers fall back to their fallbackCode/fallbackMsg pair —
// otherwise the catalog would mask transport errors as if Meta
// returned them.
func TestResolveMetaError_NonMetaErrorReturnsMiss(t *testing.T) {
	_, _, ok := ResolveMetaError(errors.New("timeout"), ActionVerifyCode)
	if ok {
		t.Error("expected miss for non-MetaError input")
	}
}

// Unknown Meta codes are misses by design — that's the signal
// mapMetaErrorAction uses to bump the Sentry capture so a new code
// surfaces with full context instead of silently degrading.
func TestResolveMetaError_UnknownCodeIsMiss(t *testing.T) {
	err := &MetaError{Code: 999999, Subcode: 1}
	_, _, ok := ResolveMetaError(err, ActionRegisterPhone)
	if ok {
		t.Error("expected miss for unknown Meta code")
	}
}

// nil input must not panic the dispatch (callers occasionally pass
// the result of err == nil paths that already short-circuit but the
// defensive return keeps Resolve safe to call from anywhere).
func TestResolveMetaError_NilSafe(t *testing.T) {
	_, _, ok := ResolveMetaError(nil, ActionRegisterPhone)
	if ok {
		t.Error("expected miss for nil error")
	}
}

// Code=100 with a "payment method" hint inside error_data.details
// must resolve to WA_NO_PAYMENT_METHOD so the dashboard can show the
// BR chicken-and-egg explanation and the "Verificar empresa" CTA.
// Without this, register failures during BSP migration looked like a
// generic INVALID_PARAM and customers had no idea what to fix.
func TestResolveMetaError_NoPaymentMethodViaDetails(t *testing.T) {
	err := &MetaError{
		Code:    100,
		Details: "Cannot Migrate Phone Number: Your WhatsApp Business Account does not have payment method set up.",
	}
	_, entry, ok := ResolveMetaError(err, ActionRegisterPhone)
	if !ok || entry.Kind != "WA_NO_PAYMENT_METHOD" {
		t.Errorf("got kind=%q ok=%v, want WA_NO_PAYMENT_METHOD", entry.Kind, ok)
	}
}

// Same hint appearing in UserMsg (instead of Details) must also win.
// Meta is inconsistent about which field carries the substring per
// region/account state — covering both fields prevents silent regressions.
func TestResolveMetaError_NoPaymentMethodViaUserMsg(t *testing.T) {
	err := &MetaError{
		Code:    100,
		UserMsg: "Add a payment method to continue.",
	}
	_, entry, ok := ResolveMetaError(err, ActionRegisterPhone)
	if !ok || entry.Kind != "WA_NO_PAYMENT_METHOD" {
		t.Errorf("got kind=%q ok=%v, want WA_NO_PAYMENT_METHOD", entry.Kind, ok)
	}
}

// Code=100 with "business verification" / "not verified" copy must
// land on WA_BUSINESS_NOT_VERIFIED so the dashboard surfaces the
// Security Center deep link instead of the generic INVALID_PARAM.
func TestResolveMetaError_BusinessNotVerified(t *testing.T) {
	err := &MetaError{
		Code:    100,
		Details: "Action requires business verification.",
	}
	_, entry, ok := ResolveMetaError(err, ActionRegisterPhone)
	if !ok || entry.Kind != "WA_BUSINESS_NOT_VERIFIED" {
		t.Errorf("got kind=%q ok=%v, want WA_BUSINESS_NOT_VERIFIED", entry.Kind, ok)
	}
}

// Catalog entries that mention a Kind must also have a default
// message — empty messages would force every handler to ship its
// own copy and defeat the centralisation. Snapshot test, fails when
// someone adds an incomplete entry.
func TestCatalogEntries_AllHaveDefaultMessage(t *testing.T) {
	for i, e := range metaErrorCatalog {
		if e.Kind == "" {
			t.Errorf("entry[%d] has empty Kind", i)
		}
		if e.DefaultMessage == "" {
			t.Errorf("entry[%d] %q has empty DefaultMessage", i, e.Kind)
		}
	}
}
