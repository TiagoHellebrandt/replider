package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the application configuration
type Config struct {
	Environment      string
	Port             string
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
	DatabaseSSLMode  string
	JWTSecret        string
	// JWTSecretPrevious is the prior signing key kept valid for VERIFICATION
	// only during a JWT secret rotation (set via JWT_SECRET_PREVIOUS). Empty
	// when no rotation is in flight. See docs/secret-rotation.md.
	JWTSecretPrevious string
	AIProvider        string
	GeminiAPIKey      string
	OpenAIAPIKey      string
	// AIRequireBYOK makes EVERY account (not just contract plans) bring its
	// own AI key: with it on, an account without a stored key gets a clear
	// BYOK_KEY_MISSING refusal instead of silently spending the platform's
	// env key. The platform account itself is exempt (its own tools). This
	// is the sales-led cost model: the customer pays the provider directly.
	AIRequireBYOK bool
	// MCPLegacyTools re-registers the archived v1 MCP tools (agents, knowledge,
	// calendars, Telegram, commissions, contracts, signatures, Places
	// prospecting, logs, sequences). Off by default since the SDR pivot.
	MCPLegacyTools bool
	// LegacyWorkers starts the background workers of the self-serve
	// customer-service SaaS this product used to be (CSAT asker,
	// conversation auto-resume, personality learning, weekly digest,
	// reactivation / win-back / pre-billing emails, Stripe overage and
	// WhatsApp metering, PIX renewal ladder). Off by default since the SDR
	// pivot: they ran in prod for nothing — or against the new rules (the
	// PIX ladder SUSPENDS accounts while AbacatePay is off). Hidden, not
	// deleted (docs/foco-produto.md: "esconder, não apagar"). The same flag
	// gates the surfaces those workers back: sequence enrollment and the
	// autopilot settings answer 503 *_UNAVAILABLE while it is off.
	LegacyWorkers bool
	// AIBatchDrafts routes the overnight draft pass through the provider's
	// Batch API (50% of the token price, answers collected before morning).
	// Any submit failure falls back to the synchronous pass.
	AIBatchDrafts       bool
	StripeSecretKey     string
	StripeWebhookSecret string
	// Metered (usage-based) Stripe price IDs for overage billing
	// (US-168). Each is the id of a `metered` recurring Price attached
	// to the subscription as an extra line item; empty disables that
	// meter (the overage reporter no-ops for it). See docs/billing.md.
	StripeMeteredPriceExtraConversation string
	StripeMeteredPriceExtraToken        string
	StripeMeteredPriceExtraAgent        string
	// AbacatePayKey authorizes the AbacatePay adapter (Brazilian PIX +
	// card gateway). Empty disables the whole AbacatePay path — BRL
	// accounts then fall back to the Stripe checkout. The transparent
	// checkout routes BRL→AbacatePay (PIX embedded + card hosted, Elo
	// support) and non-BRL→Stripe. See docs/abacatepay.md.
	AbacatePayKey string
	// AbacatePayWebhookSecret is the shared secret used to HMAC-verify
	// inbound AbacatePay webhooks (`X-Abacate-Signature: t=…,v1=…`).
	// Empty fails webhook verification closed.
	AbacatePayWebhookSecret string
	GCSBucketName           string
	GoogleCredentialsBase64 string // single service account for GCS + Calendar (and future Google APIs)
	// GooglePlacesAPIKey authorizes the outbound-prospecting adapter against the
	// official Google Places API (New). Empty disables prospecting search — the
	// adapter returns a clear PROSPECTING_PROVIDER_UNCONFIGURED error (US-021).
	GooglePlacesAPIKey string
	// PlacesDailyCapPerAccount bounds Places searches per account per UTC day
	// (PLACES_DAILY_CAP_PER_ACCOUNT, default 50; <= 0 disables). Platform
	// cost rail — Places has no BYOK.
	PlacesDailyCapPerAccount int
	// AIDefaultMonthlyBudgetCents is the ceiling applied to every account
	// whose ai_monthly_budget_cents is NULL (AI_DEFAULT_MONTHLY_BUDGET_CENTS,
	// BRL cents, default 5000 = R$50; 0 = no default ceiling).
	AIDefaultMonthlyBudgetCents int64
	// MetricsToken protects GET /health/details (METRICS_TOKEN, bearer or
	// ?token=). Empty = the detailed endpoint refuses every caller. Since
	// 2026-09-01 the same token, when set, is also required as a bearer on
	// GET /metrics (middleware.MetricsAuth); empty leaves /metrics to the
	// nginx network restriction alone.
	MetricsToken string
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SalesEmail   string // recipient for Enterprise sales-qualification leads (US-157)
	// BookingURL is the calendar page (Google Calendar appointment schedule)
	// where a prospect books a call. Sales-led: the product shows no price
	// list. Empty = the clients fall back to WhatsApp/e-mail.
	BookingURL string
	AppBaseURL string
	// APIPublicURL is the public origin of THIS API (e.g.
	// https://api.replider.com). Used as the OAuth 2.1 issuer for the
	// remote MCP integration — the well-known/authorize/token routes
	// live here. Falls back to AppBaseURL (correct in local dev where
	// both are localhost:8080).
	APIPublicURL string
	// DashboardURL is the Flutter dashboard origin (e.g.
	// https://app.replider.com). Used for MCP checkout redirect links.
	// Falls back to AppBaseURL.
	DashboardURL        string
	WebBaseURL          string // marketing-site origin; canonical seller prospecting link host (US-003)
	PrivacyPolicyURL    string // public URL of the LGPD/GDPR privacy policy (required by Meta + most BSP audits)
	TermsOfServiceURL   string // public URL of the terms of service (required by Meta App Live mode)
	WhatsAppAppID       string
	WhatsAppAppSecret   string
	WhatsAppVerifyToken string
	// MetaAppID / MetaAppSecret identify the Meta app behind Facebook Login
	// (POST /auth/meta). They prove an inbound access_token was minted for us
	// via /debug_token — without them the login is a confused deputy. Default
	// to the WhatsApp app credentials, since a single Meta app usually hosts
	// both products; set META_APP_ID / META_APP_SECRET only when Facebook
	// Login lives on a different app.
	MetaAppID     string
	MetaAppSecret string
	// InstagramAppSecret validates X-Hub-Signature-256 on inbound
	// Instagram DM webhooks (same Meta HMAC scheme as WhatsApp). Empty
	// means signature validation is skipped with a WARN — the manual-
	// token MVP lets tenants use their own Meta app whose secret we
	// don't hold; a per-channel `app_secret` config key overrides this
	// global. Usually equals WHATSAPP_APP_SECRET when both products
	// live on the same Meta app.
	InstagramAppSecret string
	// EmailWebhookSecret validates the Standard Webhooks signature on inbound
	// mail (EMAIL_WEBHOOK_SECRET, the `whsec_…` the provider shows once).
	// Fail-closed like the Instagram one: empty here AND no per-channel
	// `webhook_secret` means every inbound-mail delivery is refused. Until
	// 15/09/2026 that endpoint verified nothing at all.
	EmailWebhookSecret string
	// ClamdAddr points at a ClamAV daemon ("clamav:3310"). Empty means no
	// scanner: inbound attachments are then recorded as "skipped" and are
	// NOT downloadable, so an unscanned file can never be served as a
	// checked one. Optional on purpose — clamd costs ~1GB of RAM for its
	// signature database, and that is an infrastructure decision, not a
	// code one.
	ClamdAddr string
	// MediaCacheDir is where the media proxy keeps a local copy of served
	// objects (and where email ingest pre-warms attachments). The VPS
	// reaches the bucket at ~4Mbps, so without it a 4MB photo costs ~9s on
	// every view. Empty disables. The default is container-local and
	// ephemeral — a compose volume makes it survive restarts, but a cold
	// cache is only a slow first view, never an error.
	MediaCacheDir string
	// MediaCacheMaxMB bounds the cache's total size (LRU eviction).
	MediaCacheMaxMB int
	// ProductPDFMaxBytes / ProductPDFMaxPages bound the product-brochure PDF
	// import (PRODUCT_PDF_IMPORT_MAX_MB / PRODUCT_PDF_IMPORT_MAX_PAGES).
	// Both configurable: brochures vary wildly by brand, and the ceiling is a
	// cost control (every page reaches a multimodal model), not a format rule.
	ProductPDFMaxBytes int64
	ProductPDFMaxPages int
	// EmailWarmupRampDays / EmailWarmupDailyCap tune the mailbox warmup
	// (EMAIL_WARMUP_RAMP_DAYS / EMAIL_WARMUP_DAILY_CAP). Zero keeps the
	// domain defaults (21 days, 25/day) — misconfiguration never disables
	// the guard.
	EmailWarmupRampDays int
	EmailWarmupDailyCap int
	// DomainAgeLookupEnabled turns on the RDAP registry query used to flag a
	// sender whose domain was registered days ago. Off by default: it is an
	// external call on the inbound hot path, and losing it costs one signal
	// out of six rather than the whole check.
	DomainAgeLookupEnabled bool
	// EmailTrackingBaseURL is the base for tracked links and the open pixel.
	// Empty disables tracking entirely: links go out untouched and no pixel
	// is inserted.
	//
	// Should be a CUSTOMER domain CNAME'd here. Tracking from our own domain
	// works and is worse twice over: the link does not look like it came from
	// the sender, and the tracking domain's reputation becomes shared across
	// every customer.
	EmailTrackingBaseURL string
	WhatsAppAccessToken  string // System User token for BSP / tech provider mode
	GoogleClientID       string
	// GoogleOAuthClientSecret pairs with GoogleClientID for the
	// server-side "Conectar Google Calendar" OAuth flow (authorization
	// code exchange — the id_token login flow never needs it). Read from
	// GOOGLE_CLIENT_SECRET, the name already provisioned in the
	// production-api environment since 2026-04. Empty disables the
	// connect feature (503 GCAL_NOT_CONFIGURED); the platform service
	// account remains the booking fallback.
	GoogleOAuthClientSecret string
	CORSAllowedOrigins      string // comma-separated origins, e.g. "https://app.replider.com,https://replider.com"
	RedisURL                string // e.g. "redis://localhost:6379" or "redis://redis:6379" in Docker
	SentryDSN               string // when set, unhandled errors + manual captures flow to Sentry
	OTelExporterEndpoint    string // OTLP/HTTP collector endpoint (e.g. "otel-collector:4318"); empty disables distributed tracing
	OTelServiceName         string // service.name reported on every span (default "replider-api")
	AlertWebhookURL         string // Slack/Discord incoming webhook for AI-failure alerts (optional)
	WebPushVAPIDPublicKey   string // URL-safe base64 VAPID public key (shared with the web client)
	WebPushVAPIDPrivateKey  string // URL-safe base64 VAPID private key (server-side only)
	WebPushSubject          string // "mailto:…" or "https://…" ownership URL embedded in the VAPID JWT
	// BYOKEncryptionKey is the 32-byte (64 hex chars) AES-256-GCM master
	// key used to seal per-account BYOK API keys at rest. Required in
	// production; missing in dev maps to "BYOK temporarily disabled" so
	// other features keep booting. Generate with:
	//   openssl rand -hex 32
	BYOKEncryptionKey string
	// TOTPKEK is the 32-byte (64 hex chars) AES-256-GCM key-encryption key
	// used to seal users' TOTP shared secrets at rest (US-076). Kept
	// separate from BYOKEncryptionKey so the 2FA seeds and customer BYOK
	// keys have independent blast radii on a key leak. Required to enroll
	// 2FA; when empty (dev / unconfigured) TOTP enrollment is rejected at
	// Setup rather than persisting a plaintext seed. Generate with:
	//   openssl rand -hex 32
	TOTPKEK string
	// SuperAdminEmailsRaw is the comma-separated allowlist of operator
	// emails granted access to the /v1/owner/* platform-admin routes.
	// Resolved at request time by middleware.RequireSuperAdmin (no DB
	// column today — keeps the privilege out of band so a compromised
	// admin login can't grant peers without a deploy).
	SuperAdminEmailsRaw string
	// SuperAdminRequire2FA, when true, gates the owner panel on TOTP
	// 2FA enrollment + a fresh proof (12h window). Default false during
	// the rollout so super-admins can enroll before enforcement turns
	// on. Flip to true once every operator on SUPER_ADMIN_EMAILS is
	// enrolled.
	SuperAdminRequire2FA bool
	// TurnstileSecretKey is the Cloudflare Turnstile secret used to verify
	// captcha tokens on the public signup endpoint (US-054). Empty disables
	// the captcha challenge entirely (local dev / unconfigured) — signup
	// then relies solely on the US-053 rate limit. Obtain from the
	// Cloudflare Turnstile dashboard; the matching site key lives in the
	// web client.
	TurnstileSecretKey string
	// MaxRequestBodyBytes caps the size of an inbound request body (US-192).
	// Requests whose body exceeds this are rejected with 413. Configurable via
	// MAX_REQUEST_BODY_BYTES (in bytes); defaults to 10 MiB. A non-positive
	// value disables the limit.
	MaxRequestBodyBytes int64
	// PlatformAccountID is the id of the Replider-owned account whose
	// attendants are the commissioned sellers (vendedores PRD v3). The
	// B/C-layer seller features (carteira, comissão, lead conversion)
	// activate ONLY for this account. Empty (default) disables all of
	// them — safe for self-hosted / dev environments.
	PlatformAccountID string
	// SupportUnreadEmailAfter is the canal-interno US-004 window: an
	// internal-CS-thread reply unread for this long triggers the single
	// fallback email. Configurable via SUPPORT_UNREAD_EMAIL_AFTER
	// (Go duration, e.g. "4h"); defaults to 4h; unparseable → default + WARN.
	SupportUnreadEmailAfter time.Duration
	// RegistrationEnabled gates every account-creating path — password
	// signup AND social auth's new-account branch (US-F001). Set
	// REGISTRATION_ENABLED=false to close the door during the SDR pivot;
	// existing users keep logging in. Defaults to true; only the exact
	// strings accepted by strconv.ParseBool flip it (garbage → default
	// + WARN).
	RegistrationEnabled bool
	// AllowPrivateWebhookTargets decides whether customer-supplied webhook
	// URLs may point at loopback/RFC1918/metadata addresses (US-F011).
	// Defaults to "yes outside production" — dev and ngrok setups depend on
	// it — and is overridable via WEBHOOK_ALLOW_PRIVATE_TARGETS so staging
	// can run the production policy (and so the guard is demonstrable
	// locally without pretending to be production).
	AllowPrivateWebhookTargets bool
	// TrustedProxies is the comma-separated list of CIDRs/IPs whose
	// X-Forwarded-For / X-Real-IP headers Gin may believe (TRUSTED_PROXIES).
	// Everything else is treated as the client itself, so a caller cannot
	// forge ClientIP() — which keys every per-IP rate limiter and the audit
	// log. Default: loopback + the Docker bridge range the nginx container
	// lives in (172.16.0.0/12; compose's `proxy` network takes a subnet from
	// it). Set it to the exact nginx address when the network is pinned.
	TrustedProxies string
	// SpeedioAPIKey authorizes the Brazilian company-data provider (US-M002).
	// Empty disables CNAE/UF prospecting with a clear
	// DATA_PROVIDER_UNCONFIGURED — Google Places prospecting keeps working.
	SpeedioAPIKey string

	// Email authentication setup (US-M006). These describe the PLATFORM
	// side of a customer's sending domain: what their SPF must include and
	// which hosts their DKIM/tracking CNAMEs point at. They change only
	// when the sending provider changes, so they are env, not per-account.
	//
	// All empty is a valid state: BuildEmailDNSSetup then emits only the
	// records it can (DMARC), and the setup screen says what is missing
	// instead of inventing a hostname the customer would publish for
	// nothing.
	EmailSPFInclude     string
	EmailDKIMSelector   string
	EmailDKIMTarget     string
	EmailTrackingTarget string
	EmailDMARCReportTo  string

	// ResendAPIKey is the platform's bulk email provider key (hybrid sending:
	// batch approvals only). Empty = volume sending unavailable, fails closed
	// with BULK_SENDING_UNAVAILABLE — individual approvals are unaffected.
	ResendAPIKey string
}

// IsPlatformAccount reports whether accountID is the Replider platform
// account (the account whose attendants are commissioned sellers).
// Always false when PLATFORM_ACCOUNT_ID is unset, so seller features
// stay dormant unless explicitly configured.
func (c *Config) IsPlatformAccount(accountID string) bool {
	return c.PlatformAccountID != "" && accountID == c.PlatformAccountID
}

// SuperAdminEmails returns the parsed allowlist (lowercased, trimmed,
// blanks removed). Empty when the env var is unset — the middleware
// fails closed in that case.
func (c *Config) SuperAdminEmails() []string {
	if c.SuperAdminEmailsRaw == "" {
		return nil
	}
	parts := strings.Split(c.SuperAdminEmailsRaw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.ToLower(strings.TrimSpace(p))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// DBString formats the individual connection parameters into a PostgreSQL URL.
// Uses url.URL so that special characters in the password are percent-encoded correctly.
func (c *Config) DBString() string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DatabaseUser, c.DatabasePassword),
		Host:     fmt.Sprintf("%s:%s", c.DatabaseHost, c.DatabasePort),
		Path:     c.DatabaseName,
		RawQuery: "sslmode=" + c.DatabaseSSLMode,
	}
	return u.String()
}

// Load loads the configuration from environment variables or .env file
func Load() *Config {
	// Attempt to load .env file, but ignore the error if it doesn't exist (e.g., in a Docker container)
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, relying on environment variables only")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	envIntDefault := func(key string, def int) int {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
		slog.Warn("invalid int env, using default", "key", key, "value", raw, "default", def)
		return def
	}
	getEnvOrDefault := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}

	env := getEnvOrDefault("APP_ENV", "development")

	// MAX_REQUEST_BODY_BYTES caps inbound payload size (US-192). Default 10 MiB;
	// an unparseable value falls back to the default with a WARN.
	const defaultMaxRequestBodyBytes int64 = 10 << 20 // 10 MiB
	maxRequestBodyBytes := defaultMaxRequestBodyBytes
	if raw := os.Getenv("MAX_REQUEST_BODY_BYTES"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			maxRequestBodyBytes = parsed
		} else {
			slog.Warn("invalid MAX_REQUEST_BODY_BYTES, using default", "value", raw, "default", defaultMaxRequestBodyBytes)
		}
	}

	// Product-brochure PDF import limits. Unparseable values fall back to the
	// default with a WARN, mirroring MAX_REQUEST_BODY_BYTES above.
	const defaultProductPDFMaxMB int64 = 10
	productPDFMaxBytes := defaultProductPDFMaxMB << 20
	if raw := os.Getenv("PRODUCT_PDF_IMPORT_MAX_MB"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			productPDFMaxBytes = parsed << 20
		} else {
			slog.Warn("invalid PRODUCT_PDF_IMPORT_MAX_MB, using default", "value", raw, "default_mb", defaultProductPDFMaxMB)
		}
	}
	// Email warmup policy. Zero = domain defaults; only positive values are
	// passed through, so garbage can loosen nothing.
	emailWarmupRampDays := 0
	if raw := os.Getenv("EMAIL_WARMUP_RAMP_DAYS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			emailWarmupRampDays = parsed
		} else {
			slog.Warn("invalid EMAIL_WARMUP_RAMP_DAYS, using default", "value", raw)
		}
	}
	emailWarmupDailyCap := 0
	if raw := os.Getenv("EMAIL_WARMUP_DAILY_CAP"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			emailWarmupDailyCap = parsed
		} else {
			slog.Warn("invalid EMAIL_WARMUP_DAILY_CAP, using default", "value", raw)
		}
	}

	const defaultProductPDFMaxPages = 40
	productPDFMaxPages := defaultProductPDFMaxPages
	if raw := os.Getenv("PRODUCT_PDF_IMPORT_MAX_PAGES"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			productPDFMaxPages = parsed
		} else {
			slog.Warn("invalid PRODUCT_PDF_IMPORT_MAX_PAGES, using default", "value", raw, "default", defaultProductPDFMaxPages)
		}
	}

	// SUPPORT_UNREAD_EMAIL_AFTER: unread window before the internal-CS
	// email fallback fires (US-004). Go duration string; default 4h.
	supportUnreadEmailAfter := 4 * time.Hour
	if raw := os.Getenv("SUPPORT_UNREAD_EMAIL_AFTER"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			supportUnreadEmailAfter = parsed
		} else {
			slog.Warn("invalid SUPPORT_UNREAD_EMAIL_AFTER, using default", "value", raw, "default", supportUnreadEmailAfter)
		}
	}

	// REGISTRATION_ENABLED closes public signup during the pivot (US-F001).
	// Default true so an unset var never locks a fresh deploy out of signup.
	registrationEnabled := true
	if raw := os.Getenv("REGISTRATION_ENABLED"); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			registrationEnabled = parsed
		} else {
			slog.Warn("invalid REGISTRATION_ENABLED, using default", "value", raw, "default", true)
		}
	}

	// WEBHOOK_ALLOW_PRIVATE_TARGETS overrides the SSRF policy (US-F011).
	// Unset → allowed everywhere except production.
	allowPrivateWebhookTargets := env != "production"
	if raw := os.Getenv("WEBHOOK_ALLOW_PRIVATE_TARGETS"); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			allowPrivateWebhookTargets = parsed
		} else {
			slog.Warn("invalid WEBHOOK_ALLOW_PRIVATE_TARGETS, using environment default",
				"value", raw, "default", allowPrivateWebhookTargets)
		}
	}

	// Default DB SSL mode depends on environment
	dbSSLDefault := "disable"
	if env == "production" {
		dbSSLDefault = "require"
	}

	cfg := &Config{
		Environment:      env,
		Port:             port,
		DatabaseHost:     getEnvOrDefault("DB_HOST", "localhost"),
		DatabasePort:     getEnvOrDefault("DB_PORT", "5433"),
		DatabaseUser:     getEnvOrDefault("DB_USER", "replider"),
		DatabasePassword: getEnvOrDefault("DB_PASSWORD", "secret"),
		DatabaseName:     getEnvOrDefault("DB_NAME", "replider_db"),
		DatabaseSSLMode:  getEnvOrDefault("DB_SSLMODE", dbSSLDefault),
		// JWT_SECRET_CURRENT is the canonical name for the active signing key;
		// JWT_SECRET is the legacy alias kept for backward compatibility.
		JWTSecret:                           getEnvOrDefault("JWT_SECRET_CURRENT", getEnvOrDefault("JWT_SECRET", "super-secret-key-change-me")),
		JWTSecretPrevious:                   getEnvOrDefault("JWT_SECRET_PREVIOUS", ""),
		AIProvider:                          getEnvOrDefault("AI_PROVIDER", "gemini"),
		GeminiAPIKey:                        getEnvOrDefault("GEMINI_API_KEY", ""),
		OpenAIAPIKey:                        getEnvOrDefault("OPENAI_API_KEY", ""),
		AIRequireBYOK:                       getEnvOrDefault("AI_REQUIRE_BYOK", "") == "true",
		AIBatchDrafts:                       getEnvOrDefault("AI_BATCH_DRAFTS", "") == "true",
		MCPLegacyTools:                      getEnvOrDefault("MCP_LEGACY_TOOLS", "") == "true",
		LegacyWorkers:                       getEnvOrDefault("LEGACY_WORKERS", "") == "true",
		StripeSecretKey:                     getEnvOrDefault("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:                 getEnvOrDefault("STRIPE_WEBHOOK_SECRET", ""),
		StripeMeteredPriceExtraConversation: getEnvOrDefault("STRIPE_METERED_PRICE_EXTRA_CONVERSATION", ""),
		StripeMeteredPriceExtraToken:        getEnvOrDefault("STRIPE_METERED_PRICE_EXTRA_TOKEN", ""),
		StripeMeteredPriceExtraAgent:        getEnvOrDefault("STRIPE_METERED_PRICE_EXTRA_AGENT", ""),
		AbacatePayKey:                       getEnvOrDefault("ABACATEPAY_KEY", ""),
		AbacatePayWebhookSecret:             getEnvOrDefault("ABACATEPAY_WEBHOOK_SECRET", ""),
		GCSBucketName:                       getEnvOrDefault("GCS_BUCKET_NAME", ""),
		GoogleCredentialsBase64:             getEnvOrDefault("GOOGLE_CREDENTIALS_BASE64", ""),
		GooglePlacesAPIKey:                  getEnvOrDefault("GOOGLE_PLACES_API_KEY", ""),
		PlacesDailyCapPerAccount:            envIntDefault("PLACES_DAILY_CAP_PER_ACCOUNT", 50),
		AIDefaultMonthlyBudgetCents:         int64(envIntDefault("AI_DEFAULT_MONTHLY_BUDGET_CENTS", 5000)),
		MetricsToken:                        getEnvOrDefault("METRICS_TOKEN", ""),
		SMTPHost:                            getEnvOrDefault("SMTP_HOST", ""),
		SMTPPort:                            getEnvOrDefault("SMTP_PORT", "587"),
		SMTPUsername:                        getEnvOrDefault("SMTP_USERNAME", ""),
		SMTPPassword:                        getEnvOrDefault("SMTP_PASSWORD", ""),
		SMTPFrom:                            getEnvOrDefault("SMTP_FROM", "noreply@replider.com"),
		SalesEmail:                          getEnvOrDefault("SALES_EMAIL", "sales@replider.com"),
		BookingURL:                          getEnvOrDefault("BOOKING_URL", ""),
		AppBaseURL:                          getEnvOrDefault("APP_BASE_URL", "http://localhost:8080"),
		APIPublicURL:                        getEnvOrDefault("API_PUBLIC_URL", getEnvOrDefault("APP_BASE_URL", "http://localhost:8080")),
		DashboardURL:                        getEnvOrDefault("DASHBOARD_URL", getEnvOrDefault("APP_BASE_URL", "http://localhost:8080")),
		WebBaseURL:                          getEnvOrDefault("WEB_BASE_URL", "https://replider.com"),
		PrivacyPolicyURL:                    os.Getenv("PRIVACY_POLICY_URL"),
		TermsOfServiceURL:                   os.Getenv("TERMS_OF_SERVICE_URL"),
		WhatsAppAppID:                       getEnvOrDefault("WHATSAPP_APP_ID", ""),
		WhatsAppAppSecret:                   getEnvOrDefault("WHATSAPP_APP_SECRET", ""),
		WhatsAppVerifyToken:                 getEnvOrDefault("WHATSAPP_VERIFY_TOKEN", "replider-whatsapp-verify"),
		MetaAppID:                           getEnvOrDefault("META_APP_ID", getEnvOrDefault("WHATSAPP_APP_ID", "")),
		MetaAppSecret:                       getEnvOrDefault("META_APP_SECRET", getEnvOrDefault("WHATSAPP_APP_SECRET", "")),
		InstagramAppSecret:                  getEnvOrDefault("INSTAGRAM_APP_SECRET", ""),
		EmailWebhookSecret:                  getEnvOrDefault("EMAIL_WEBHOOK_SECRET", ""),
		ClamdAddr:                           getEnvOrDefault("CLAMD_ADDR", ""),
		MediaCacheDir:                       getEnvOrDefault("MEDIA_CACHE_DIR", "/var/cache/replider-media"),
		MediaCacheMaxMB:                     envIntDefault("MEDIA_CACHE_MAX_MB", 2048),
		ProductPDFMaxBytes:                  productPDFMaxBytes,
		ProductPDFMaxPages:                  productPDFMaxPages,
		EmailWarmupRampDays:                 emailWarmupRampDays,
		EmailWarmupDailyCap:                 emailWarmupDailyCap,
		DomainAgeLookupEnabled:              getEnvOrDefault("DOMAIN_AGE_LOOKUP", "") == "true",
		EmailTrackingBaseURL:                getEnvOrDefault("EMAIL_TRACKING_BASE_URL", ""),
		WhatsAppAccessToken:                 getEnvOrDefault("WHATSAPP_ACCESS_TOKEN", ""),
		GoogleClientID:                      getEnvOrDefault("GOOGLE_CLIENT_ID", ""),
		GoogleOAuthClientSecret:             getEnvOrDefault("GOOGLE_CLIENT_SECRET", ""),
		CORSAllowedOrigins:                  getEnvOrDefault("CORS_ALLOWED_ORIGINS", ""),
		RedisURL:                            getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),
		SentryDSN:                           getEnvOrDefault("SENTRY_DSN", ""),
		OTelExporterEndpoint:                getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTelServiceName:                     getEnvOrDefault("OTEL_SERVICE_NAME", "replider-api"),
		AlertWebhookURL:                     getEnvOrDefault("ALERT_WEBHOOK_URL", ""),
		WebPushVAPIDPublicKey:               getEnvOrDefault("WEBPUSH_VAPID_PUBLIC_KEY", ""),
		WebPushVAPIDPrivateKey:              getEnvOrDefault("WEBPUSH_VAPID_PRIVATE_KEY", ""),
		WebPushSubject:                      getEnvOrDefault("WEBPUSH_SUBJECT", "mailto:noreply@replider.com"),
		BYOKEncryptionKey:                   getEnvOrDefault("BYOK_ENCRYPTION_KEY", ""),
		TOTPKEK:                             getEnvOrDefault("TOTP_KEK", ""),
		SuperAdminEmailsRaw:                 os.Getenv("SUPER_ADMIN_EMAILS"),
		SuperAdminRequire2FA:                strings.EqualFold(os.Getenv("SUPER_ADMIN_REQUIRE_2FA"), "true"),
		TurnstileSecretKey:                  getEnvOrDefault("TURNSTILE_SECRET_KEY", ""),
		MaxRequestBodyBytes:                 maxRequestBodyBytes,
		PlatformAccountID:                   getEnvOrDefault("PLATFORM_ACCOUNT_ID", ""),
		SupportUnreadEmailAfter:             supportUnreadEmailAfter,
		RegistrationEnabled:                 registrationEnabled,
		SpeedioAPIKey:                       getEnvOrDefault("SPEEDIO_API_KEY", ""),
		EmailSPFInclude:                     getEnvOrDefault("EMAIL_SPF_INCLUDE", ""),
		EmailDKIMSelector:                   getEnvOrDefault("EMAIL_DKIM_SELECTOR", ""),
		EmailDKIMTarget:                     getEnvOrDefault("EMAIL_DKIM_TARGET", ""),
		EmailTrackingTarget:                 getEnvOrDefault("EMAIL_TRACKING_TARGET", ""),
		EmailDMARCReportTo:                  getEnvOrDefault("EMAIL_DMARC_REPORT_TO", ""),
		ResendAPIKey:                        getEnvOrDefault("RESEND_API_KEY", ""),
		AllowPrivateWebhookTargets:          allowPrivateWebhookTargets,
		TrustedProxies:                      getEnvOrDefault("TRUSTED_PROXIES", "127.0.0.1/32,::1/128,172.16.0.0/12"),
	}

	cfg.Validate()

	return cfg
}

// Validate checks that security-critical configuration is properly set for the current environment.
// It panics on missing or insecure required config so the container fails fast with a clear message
// instead of starting up and failing later with a cryptic error.
func (c *Config) Validate() {
	if c.Environment != "production" {
		return
	}

	type check struct {
		value   string
		varName string
		reason  string
	}

	// Required: app will not function correctly without these.
	required := []check{
		{c.DatabaseUser, "DB_USER", ""},
		{c.DatabasePassword, "DB_PASSWORD", ""},
		{c.DatabaseName, "DB_NAME", ""},
		{c.JWTSecret, "JWT_SECRET", ""},
		{c.CORSAllowedOrigins, "CORS_ALLOWED_ORIGINS", ""},
		{c.StripeSecretKey, "STRIPE_SECRET_KEY", ""},
		{c.StripeWebhookSecret, "STRIPE_WEBHOOK_SECRET", ""},
		{c.GCSBucketName, "GCS_BUCKET_NAME", ""},
		{c.GoogleCredentialsBase64, "GOOGLE_CREDENTIALS_BASE64", ""},
		// WhatsApp Cloud API: APP_ID drives Embedded Signup OAuth and
		// APP_SECRET signs inbound webhooks. Without APP_SECRET an
		// attacker could forge inbound messages and trigger AI runs on
		// behalf of any tenant — never run prod without it.
		{c.WhatsAppAppID, "WHATSAPP_APP_ID", ""},
		{c.WhatsAppAppSecret, "WHATSAPP_APP_SECRET", ""},
	}

	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.varName)
		}
	}
	if len(missing) > 0 {
		panic("FATAL: missing required environment variables in production: " + strings.Join(missing, ", "))
	}

	// Insecure defaults: values that ship as placeholders and must be changed.
	if c.JWTSecret == "super-secret-key-change-me" {
		panic("FATAL: JWT_SECRET must be changed from the default value in production")
	}
	if c.DatabasePassword == "secret" {
		panic("FATAL: DB_PASSWORD must be changed from the default value in production")
	}

	// Optional but recommended — warn instead of panic.
	if c.SMTPHost == "" {
		slog.Warn("SMTP_HOST not configured — transactional emails will not be sent")
	}
}

// TrustedProxyList splits TrustedProxies for gin.Engine.SetTrustedProxies.
// An empty value means "trust no proxy": ClientIP() is then always the TCP
// peer, which is the safe answer when the API is exposed without nginx.
func (c *Config) TrustedProxyList() []string {
	var out []string
	for _, p := range strings.Split(c.TrustedProxies, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
