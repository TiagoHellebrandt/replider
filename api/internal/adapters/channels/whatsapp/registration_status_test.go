package whatsapp

import (
	"errors"
	"testing"
)

func TestClassifyRegisterError_Nil(t *testing.T) {
	out := ClassifyRegisterError(nil)
	if out.Status != RegistrationStatusRegistered {
		t.Fatalf("expected registered, got %s", out.Status)
	}
	if out.Code != 0 || out.Kind != "" || out.Message != "" {
		t.Fatalf("expected empty error fields, got %+v", out)
	}
}

func TestClassifyRegisterError_RateLimited(t *testing.T) {
	err := &MetaError{Code: 133016, Message: "rate limit"}
	out := ClassifyRegisterError(err)
	if out.Status != RegistrationStatusRateLimited {
		t.Fatalf("expected rate_limited, got %s", out.Status)
	}
	if out.Code != 133016 {
		t.Fatalf("expected code 133016, got %d", out.Code)
	}
	if out.Kind != "WA_REGISTER_RATE_LIMITED" {
		t.Fatalf("unexpected kind %q", out.Kind)
	}
}

func TestClassifyRegisterError_PinMismatch(t *testing.T) {
	err := &MetaError{Code: 133005, UserMsg: "pin incorreto"}
	out := ClassifyRegisterError(err)
	if out.Status != RegistrationStatusPinRequired {
		t.Fatalf("expected pin_required, got %s", out.Status)
	}
	if out.Message != "pin incorreto" {
		t.Fatalf("expected localized user message, got %q", out.Message)
	}
}

func TestClassifyRegisterError_PinLocked(t *testing.T) {
	out := ClassifyRegisterError(&MetaError{Code: 133008})
	if out.Status != RegistrationStatusPinLocked {
		t.Fatalf("expected pin_locked, got %s", out.Status)
	}
}

func TestClassifyRegisterError_InUseElsewhere(t *testing.T) {
	out := ClassifyRegisterError(&MetaError{Code: 133010})
	if out.Status != RegistrationStatusInUse {
		t.Fatalf("expected in_use_elsewhere, got %s", out.Status)
	}
}

func TestClassifyRegisterError_GenericMeta(t *testing.T) {
	out := ClassifyRegisterError(&MetaError{Code: 999, Message: "boom"})
	if out.Status != RegistrationStatusFailed {
		t.Fatalf("expected failed, got %s", out.Status)
	}
	if out.Code != 999 || out.Message != "boom" {
		t.Fatalf("expected code/message preserved, got %+v", out)
	}
}

func TestClassifyRegisterError_NonMeta(t *testing.T) {
	out := ClassifyRegisterError(errors.New("network down"))
	if out.Status != RegistrationStatusFailed {
		t.Fatalf("expected failed, got %s", out.Status)
	}
	if out.Code != 0 {
		t.Fatalf("expected zero code on non-meta error, got %d", out.Code)
	}
	if out.Message != "network down" {
		t.Fatalf("expected raw error message, got %q", out.Message)
	}
}

// MetaError.Kind() coverage for the codes added in round-2 — each
// distinct CTA on the dashboard depends on the Kind being stable.

func TestMetaErrorKind_NewlyMappedCodes(t *testing.T) {
	cases := map[int]string{
		131009: "INVALID_PARAM",
		131021: "WA_SELF_MESSAGE",
		131031: "WA_ACCOUNT_LOCKED",
		131045: "",
		131047: "OUTSIDE_24H_WINDOW",
		131051: "RECIPIENT_ERROR",
		131056: "WA_PAIR_RATE_LIMIT",
		132045: "WA_TEMPLATE_PAUSED",
		133006: "WA_NOT_REGISTERED",
		133008: "WA_PIN_LOCKED",
		133010: "WA_PHONE_IN_USE_ELSEWHERE",
		133015: "WA_ACCOUNT_CHECK_PENDING",
		136025: "WA_NOT_ONBOARDED",
	}
	for code, want := range cases {
		got := (&MetaError{Code: code}).Kind()
		if got != want {
			t.Errorf("Kind(%d) = %q, want %q", code, got, want)
		}
	}
	// 2388001 is a SUBCODE in production (Meta envelope is
	// {code:100, error_subcode:2388001}), so it lives in the
	// catalog as Code=100, Subcode=2388001 entries.
	if got := (&MetaError{Code: 100, Subcode: 2388001}).Kind(); got != "WA_CERT_2FA_BLOCKED" {
		t.Errorf("Kind(100/2388001) = %q, want WA_CERT_2FA_BLOCKED", got)
	}
}

// 2388001 has two flavours that share the same Meta numeric subcode;
// the only signal that distinguishes them is Meta's user_msg. Make
// sure the dispatch picks WA_OWNERSHIP_NOT_VERIFIED for the SMS-flow
// case while keeping WA_CERT_2FA_BLOCKED as the default — getting
// this wrong sends users to disable 2FA when they actually need to
// verify ownership through SMS/voice.
//
// Critical: Meta returns Code=100 + Subcode=2388001 in production.
// The Kind() dispatch must look at Subcode FIRST so this case isn't
// short-circuited by the Code=100 → INVALID_PARAM branch.
func TestMetaError_Kind_2388001_DisambiguateOwnership(t *testing.T) {
	if k := (&MetaError{Code: 100, Subcode: 2388001, UserMsg: "Please verify ownership of this phone number."}).Kind(); k != "WA_OWNERSHIP_NOT_VERIFIED" {
		t.Errorf("ownership flavour (real Meta shape): got %q, want WA_OWNERSHIP_NOT_VERIFIED", k)
	}
	if k := (&MetaError{Code: 100, Subcode: 2388001, Message: "verify ownership"}).Kind(); k != "WA_OWNERSHIP_NOT_VERIFIED" {
		t.Errorf("ownership in message: got %q, want WA_OWNERSHIP_NOT_VERIFIED", k)
	}
	if k := (&MetaError{Code: 100, Subcode: 2388001, UserMsg: "2FA still on"}).Kind(); k != "WA_CERT_2FA_BLOCKED" {
		t.Errorf("2FA flavour: got %q, want WA_CERT_2FA_BLOCKED", k)
	}
}

// ClassifyRegisterError must produce the verify_ownership status when
// the underlying Meta error carries the ownership user_msg, so the
// dashboard pre-selects the right tab on the register dialog instead
// of leaving the user clicking the (failing) "PIN conhecido" tab.
//
// Asserts the realistic Code=100 + Subcode=2388001 shape that Meta
// actually produces — the previous test caught mis-dispatch when
// Code=2388001 (which never happens in production).
func TestClassifyRegisterError_VerifyOwnership(t *testing.T) {
	err := &MetaError{Code: 100, Subcode: 2388001, UserMsg: "Please verify ownership of this phone number."}
	out := ClassifyRegisterError(err)
	if out.Status != RegistrationStatusVerifyOwnership {
		t.Errorf("status = %q, want verify_ownership", out.Status)
	}
	if out.Kind != "WA_OWNERSHIP_NOT_VERIFIED" {
		t.Errorf("kind = %q, want WA_OWNERSHIP_NOT_VERIFIED", out.Kind)
	}
}
