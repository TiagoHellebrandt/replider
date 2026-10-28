package whatsapp

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MetaError wraps a Graph API failure so callers can react to specific
// error codes instead of parsing strings. The code field matches Meta's
// numeric error codes (190 = OAuth token expired, 100 = invalid param,
// 10 = permission denied, etc.). HTTPStatus is the response code.
type MetaError struct {
	HTTPStatus int
	Code       int
	Subcode    int
	Type       string
	Message    string
	// UserTitle and UserMsg are Meta's localized, human-facing copy
	// (error_user_title / error_user_msg). They carry the real reason
	// — e.g. "Template name already exists" — while the top-level
	// Message stays generic ("Invalid parameter"). Prefer these
	// whenever they're present.
	UserTitle string
	UserMsg   string
	// Details is Meta's error_data.details — a third human-readable
	// channel that Meta uses for register/migration failures
	// ("Cannot Migrate Phone Number: Your WhatsApp Business Account
	// does not have payment method set up."). When set, it almost
	// always carries the actionable cause and should be preferred
	// over Message but tied with UserMsg.
	Details   string
	FBTraceID string
	Raw       string
}

func (e *MetaError) Error() string {
	detail := e.Message
	if e.UserMsg != "" {
		detail = e.UserMsg
	}
	// Details overrides both because it's the only field Meta populates
	// for register/migration failures — the others stay as the generic
	// "Invalid parameter" while Details carries the real reason.
	if e.Details != "" {
		detail = e.Details
	}
	return fmt.Sprintf("meta graph api error %d/%d: %s (HTTP %d, fb_trace=%s)", e.Code, e.Subcode, detail, e.HTTPStatus, e.FBTraceID)
}

// Kind returns a short string the API can surface to clients so the UI
// can pick a CTA. Returns "" when we don't recognize the code.
//
// Delegates to the central catalog (meta_error_catalog.go) so a new
// Meta error code only needs one diff there. Falls back to the legacy
// per-Code switch for codes that haven't been migrated yet — kept
// during the transition so untouched call sites don't regress.
func (e *MetaError) Kind() string {
	return e.KindForAction(ActionUnknown)
}

// KindForAction is the catalog-aware variant. Pass the high-level
// MetaAction (e.g. ActionVerifyCode) so the catalog can disambiguate
// generic codes that mean different things per endpoint — most
// importantly the bare (#100) "Invalid parameter" returned by every
// Cloud API surface.
func (e *MetaError) KindForAction(action MetaAction) string {
	if _, entry, ok := ResolveMetaError(e, action); ok {
		return entry.Kind
	}
	return e.legacyKind()
}

// legacyKind keeps the original per-Code switch as a fallback for
// codes that haven't been migrated to the catalog yet. New entries
// should be added to the catalog instead of growing this switch.
func (e *MetaError) legacyKind() string {
	switch e.Code {
	case 190:
		return "TOKEN_EXPIRED"
	case 100:
		return "INVALID_PARAM"
	case 10, 200, 210:
		return "MISSING_PERMISSION"
	case 131000, 131005, 131016, 131051:
		return "RECIPIENT_ERROR"
	case 131009:
		// "Parameter value is not valid" — caller sent malformed phone /
		// content / template name. Distinct from 100 because it points
		// to a specific Cloud-API field rather than the OAuth envelope.
		return "INVALID_PARAM"
	case 131021:
		// "Sender and recipient are the same" — usually a misconfigured
		// test where the business number tried to message itself.
		return "WA_SELF_MESSAGE"
	case 131026:
		return "OUTSIDE_24H_WINDOW"
	case 131031:
		// Account locked by Meta integrity. Customer must appeal in
		// Business Manager; no programmatic recovery.
		return "WA_ACCOUNT_LOCKED"
	case 131047:
		return "OUTSIDE_24H_WINDOW"
	case 131056:
		// Pair rate limit hit (this business + this user). Backoff
		// is required; immediate retry just renews the cooldown.
		return "WA_PAIR_RATE_LIMIT"
	case 132000, 132001, 132005, 132007, 132012, 132015, 132016, 132068:
		return "TEMPLATE_ERROR"
	case 132045:
		// Template paused due to pacing / opt-out signal. Recoverable
		// but only after Meta auto-resumes (typically 24h) — caller
		// should fall back to a different template or queue.
		return "WA_TEMPLATE_PAUSED"
	case 133005:
		return "WA_PIN_MISMATCH"
	case 133006:
		// Phone not registered to Cloud API yet — caller must run
		// /register before sending or receiving messages.
		return "WA_NOT_REGISTERED"
	case 133008:
		// Too many incorrect 2FA PIN attempts — phone is locked for
		// ~24h. /request_code + /verify_code is the only recovery.
		return "WA_PIN_LOCKED"
	case 133010:
		// Phone already registered to a different Cloud API app
		// (legacy provider, second tenant). Caller must run
		// /deregister with the old token, or have the customer ask
		// the previous provider to release it.
		return "WA_PHONE_IN_USE_ELSEWHERE"
	case 133016:
		// Cloud API rate-limit on register/deregister attempts. Meta
		// locks the phone for ~24h after a small handful of failures
		// — retrying just extends the cooldown.
		return "WA_REGISTER_RATE_LIMITED"
	case 133015:
		// Phone is mid account-verification check by Meta. Register
		// will start working once Meta finishes; surfacing it lets
		// the UI show "Aguardando análise da Meta" instead of a
		// generic failure.
		return "WA_ACCOUNT_CHECK_PENDING"
	case 136024:
		return "WA_ALREADY_VERIFIED"
	case 136025:
		// Phone never went through onboarding (no last_onboarded_time
		// on the WABA's phone_numbers entry). Customer needs to redo
		// Embedded Signup from the start.
		return "WA_NOT_ONBOARDED"
	}
	return ""
}

// containsCaseInsensitive is a small helper so Kind() doesn't have to
// pull in the strings package just for one lookup. Inlined to keep the
// file self-contained.
func containsCaseInsensitive(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Naive scan — fine for short error strings.
	for i := 0; i+len(substr) <= len(s); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			a := s[i+j]
			b := substr[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// RegistrationStatus is the persisted view of the last /register attempt
// for a WhatsApp channel. Stored in channels.credentials JSONB so the
// dashboard can surface "registro pendente" / "rate-limited até X" /
// "PIN bloqueado" CTAs without having to re-call Meta on every page load.
type RegistrationStatus string

const (
	RegistrationStatusRegistered  RegistrationStatus = "registered"
	RegistrationStatusPending     RegistrationStatus = "pending"
	RegistrationStatusRateLimited RegistrationStatus = "rate_limited"
	RegistrationStatusPinRequired RegistrationStatus = "pin_required"
	RegistrationStatusPinLocked   RegistrationStatus = "pin_locked"
	RegistrationStatusInUse       RegistrationStatus = "in_use_elsewhere"
	// VerifyOwnership: number was never verified for this WABA. The
	// only path forward is the SMS/voice request_code → verify_code
	// dance — /register cannot mint a certificate until ownership is
	// proven. Surfaced to the dashboard so it can pre-select the
	// right tab on the register dialog.
	RegistrationStatusVerifyOwnership RegistrationStatus = "verify_ownership"
	RegistrationStatusFailed          RegistrationStatus = "failed"
)

// RegistrationOutcome summarizes a /register call. Code is Meta's numeric
// error code (0 on success or unrecognized), Kind is the short tag from
// MetaError.Kind(), Message is the localized detail when available.
type RegistrationOutcome struct {
	Status  RegistrationStatus
	Code    int
	Kind    string
	Message string
}

// ClassifyRegisterError maps a /register error into a stable status the
// UI can branch on. Returns a "registered" outcome when err is nil so
// callers can write the result unconditionally.
//
// Drives off the catalog (action = ActionRegisterPhone) so a new
// register-flow code added to the catalog automatically reflects
// here without touching this function.
func ClassifyRegisterError(err error) RegistrationOutcome {
	if err == nil {
		return RegistrationOutcome{Status: RegistrationStatusRegistered}
	}
	var meta *MetaError
	if !errors.As(err, &meta) {
		return RegistrationOutcome{
			Status:  RegistrationStatusFailed,
			Message: err.Error(),
		}
	}
	out := RegistrationOutcome{
		Code:    meta.Code,
		Kind:    meta.KindForAction(ActionRegisterPhone),
		Message: meta.Message,
	}
	if meta.UserMsg != "" {
		out.Message = meta.UserMsg
	}
	out.Status = registerStatusFromKind(out.Kind)
	return out
}

// registerStatusFromKind translates a canonical Meta error kind into
// the persisted RegistrationStatus the dashboard polls. Centralised
// here so adding a new register-flow Kind requires only this map +
// a catalog entry — handlers never branch on raw Meta codes.
func registerStatusFromKind(kind string) RegistrationStatus {
	switch kind {
	case "WA_REGISTER_RATE_LIMITED":
		return RegistrationStatusRateLimited
	case "WA_PIN_MISMATCH":
		return RegistrationStatusPinRequired
	case "WA_PIN_LOCKED":
		return RegistrationStatusPinLocked
	case "WA_PHONE_IN_USE_ELSEWHERE":
		return RegistrationStatusInUse
	case "WA_OWNERSHIP_NOT_VERIFIED":
		return RegistrationStatusVerifyOwnership
	}
	return RegistrationStatusFailed
}

// ParseMetaError decodes Meta's standard {"error": {...}} envelope.
// Falls back to a generic error if the body isn't JSON.
func ParseMetaError(httpStatus int, body []byte) error {
	var env struct {
		Error struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Code      int    `json:"code"`
			Subcode   int    `json:"error_subcode"`
			UserTitle string `json:"error_user_title"`
			UserMsg   string `json:"error_user_msg"`
			FBTraceID string `json:"fbtrace_id"`
			// error_data.details is the only field Meta populates for
			// register/migration failures (e.g. "Cannot Migrate Phone
			// Number: Your WhatsApp Business Account does not have
			// payment method set up."). Capturing it lets the catalog
			// match on the real cause instead of falling back to the
			// generic "Invalid parameter" message.
			ErrorData struct {
				Details string `json:"details"`
			} `json:"error_data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("meta graph api HTTP %d: %s", httpStatus, string(body))
	}
	// Some Meta payloads ship `error_user_msg` or `error_data.details`
	// populated and `message` empty (translated client-facing failures).
	// Treat the envelope as valid when ANY field is set so callers still
	// get a typed MetaError they can branch on instead of a string-shaped
	// fallback.
	if env.Error.Message == "" && env.Error.UserMsg == "" && env.Error.ErrorData.Details == "" && env.Error.Code == 0 {
		return fmt.Errorf("meta graph api HTTP %d: %s", httpStatus, string(body))
	}
	return &MetaError{
		HTTPStatus: httpStatus,
		Code:       env.Error.Code,
		Subcode:    env.Error.Subcode,
		Type:       env.Error.Type,
		Message:    env.Error.Message,
		UserTitle:  env.Error.UserTitle,
		UserMsg:    env.Error.UserMsg,
		Details:    env.Error.ErrorData.Details,
		FBTraceID:  env.Error.FBTraceID,
		Raw:        string(body),
	}
}
