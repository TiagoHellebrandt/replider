package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// clearEnvVars unsets all config-related env vars to ensure a clean slate.
func clearEnvVars(t *testing.T) {
	t.Helper()
	vars := []string{
		"PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"DB_SSLMODE", "JWT_SECRET", "JWT_SECRET_CURRENT", "JWT_SECRET_PREVIOUS",
		"AI_PROVIDER", "GEMINI_API_KEY", "OPENAI_API_KEY",
		"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "GCS_BUCKET_NAME",
		"GOOGLE_CREDENTIALS_BASE64", "SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME",
		"SMTP_PASSWORD", "SMTP_FROM", "APP_BASE_URL", "WHATSAPP_APP_ID",
		"WHATSAPP_APP_SECRET", "WHATSAPP_VERIFY_TOKEN", "GOOGLE_CLIENT_ID",
		"CORS_ALLOWED_ORIGINS",
	}
	for _, v := range vars {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnvVars(t)

	cfg := Load()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "localhost", cfg.DatabaseHost)
	assert.Equal(t, "5433", cfg.DatabasePort)
	assert.Equal(t, "replider", cfg.DatabaseUser)
	assert.Equal(t, "secret", cfg.DatabasePassword)
	assert.Equal(t, "replider_db", cfg.DatabaseName)
	assert.Equal(t, "disable", cfg.DatabaseSSLMode)
	assert.Equal(t, "super-secret-key-change-me", cfg.JWTSecret)
	assert.Equal(t, "gemini", cfg.AIProvider)
	assert.Equal(t, "", cfg.GeminiAPIKey)
	assert.Equal(t, "", cfg.OpenAIAPIKey)
	assert.Equal(t, "", cfg.StripeSecretKey)
	assert.Equal(t, "", cfg.StripeWebhookSecret)
	assert.Equal(t, "", cfg.GCSBucketName)
	assert.Equal(t, "", cfg.GoogleCredentialsBase64)
	assert.Equal(t, "", cfg.SMTPHost)
	assert.Equal(t, "587", cfg.SMTPPort)
	assert.Equal(t, "", cfg.SMTPUsername)
	assert.Equal(t, "", cfg.SMTPPassword)
	assert.Equal(t, "noreply@replider.com", cfg.SMTPFrom)
	assert.Equal(t, "http://localhost:8080", cfg.AppBaseURL)
	assert.Equal(t, "", cfg.WhatsAppAppID)
	assert.Equal(t, "", cfg.WhatsAppAppSecret)
	assert.Equal(t, "replider-whatsapp-verify", cfg.WhatsAppVerifyToken)
	assert.Equal(t, "", cfg.GoogleClientID)
	assert.Equal(t, "", cfg.CORSAllowedOrigins)
}

func TestLoad_WithEnvVars(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("PORT", "9090")
	t.Setenv("DB_HOST", "db.prod.internal")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "prod_user")
	t.Setenv("DB_PASSWORD", "prod_pass")
	t.Setenv("DB_NAME", "prod_db")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("JWT_SECRET", "prod-jwt-secret")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("GEMINI_API_KEY", "gemini-key-123")
	t.Setenv("OPENAI_API_KEY", "openai-key-456")
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_xxx")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_xxx")
	t.Setenv("GCS_BUCKET_NAME", "my-bucket")
	t.Setenv("GOOGLE_CREDENTIALS_BASE64", "base64creds")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "smtp_user")
	t.Setenv("SMTP_PASSWORD", "smtp_pass")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("APP_BASE_URL", "https://app.example.com")
	t.Setenv("WHATSAPP_APP_ID", "wa_app_id")
	t.Setenv("WHATSAPP_APP_SECRET", "wa_secret")
	t.Setenv("WHATSAPP_VERIFY_TOKEN", "wa_verify")
	t.Setenv("GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com,https://example.com")

	cfg := Load()

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "db.prod.internal", cfg.DatabaseHost)
	assert.Equal(t, "5432", cfg.DatabasePort)
	assert.Equal(t, "prod_user", cfg.DatabaseUser)
	assert.Equal(t, "prod_pass", cfg.DatabasePassword)
	assert.Equal(t, "prod_db", cfg.DatabaseName)
	assert.Equal(t, "require", cfg.DatabaseSSLMode)
	assert.Equal(t, "prod-jwt-secret", cfg.JWTSecret)
	assert.Equal(t, "openai", cfg.AIProvider)
	assert.Equal(t, "gemini-key-123", cfg.GeminiAPIKey)
	assert.Equal(t, "openai-key-456", cfg.OpenAIAPIKey)
	assert.Equal(t, "sk_live_xxx", cfg.StripeSecretKey)
	assert.Equal(t, "whsec_xxx", cfg.StripeWebhookSecret)
	assert.Equal(t, "my-bucket", cfg.GCSBucketName)
	assert.Equal(t, "base64creds", cfg.GoogleCredentialsBase64)
	assert.Equal(t, "smtp.example.com", cfg.SMTPHost)
	assert.Equal(t, "465", cfg.SMTPPort)
	assert.Equal(t, "smtp_user", cfg.SMTPUsername)
	assert.Equal(t, "smtp_pass", cfg.SMTPPassword)
	assert.Equal(t, "no-reply@example.com", cfg.SMTPFrom)
	assert.Equal(t, "https://app.example.com", cfg.AppBaseURL)
	assert.Equal(t, "wa_app_id", cfg.WhatsAppAppID)
	assert.Equal(t, "wa_secret", cfg.WhatsAppAppSecret)
	assert.Equal(t, "wa_verify", cfg.WhatsAppVerifyToken)
	assert.Equal(t, "google-client-id", cfg.GoogleClientID)
	assert.Equal(t, "https://app.example.com,https://example.com", cfg.CORSAllowedOrigins)
}

// productionConfig returns a Config with every required production var
// populated, so a single field can be blanked to exercise one Validate
// failure path in isolation.
func productionConfig() *Config {
	return &Config{
		Environment:             "production",
		DatabaseUser:            "prod_user",
		DatabasePassword:        "prod_pass",
		DatabaseName:            "prod_db",
		JWTSecret:               "prod-jwt-secret",
		CORSAllowedOrigins:      "https://app.example.com",
		StripeSecretKey:         "sk_live_xxx",
		StripeWebhookSecret:     "whsec_xxx",
		GCSBucketName:           "my-bucket",
		GoogleCredentialsBase64: "base64creds",
		WhatsAppAppID:           "wa_app_id",
		WhatsAppAppSecret:       "wa_secret",
	}
}

// TestValidate_PanicsWithoutWhatsAppAppSecret guards the startup gate:
// a production boot with WHATSAPP_APP_SECRET unset must fail fast (panic)
// rather than starting up and silently accepting forged inbound webhooks.
// This is the boot-time complement to the request-time 500 returned by
// the webhook handler when the secret is empty.
func TestValidate_PanicsWithoutWhatsAppAppSecret(t *testing.T) {
	c := productionConfig()
	c.WhatsAppAppSecret = ""

	assert.PanicsWithValue(t,
		"FATAL: missing required environment variables in production: WHATSAPP_APP_SECRET",
		func() { c.Validate() },
		"production boot without WHATSAPP_APP_SECRET must panic")
}

// TestValidate_PanicsWithoutCORSAllowedOrigins is the AC#1 guard for US-080:
// a production boot with CORS_ALLOWED_ORIGINS unset must fail fast (panic)
// rather than starting up and falling back to the allow-all dev behaviour,
// which would let any origin call the API with credentials.
func TestValidate_PanicsWithoutCORSAllowedOrigins(t *testing.T) {
	c := productionConfig()
	c.CORSAllowedOrigins = ""

	assert.PanicsWithValue(t,
		"FATAL: missing required environment variables in production: CORS_ALLOWED_ORIGINS",
		func() { c.Validate() },
		"production boot without CORS_ALLOWED_ORIGINS must panic")
}

// TestValidate_PassesWithWhatsAppAppSecret is the positive control: a
// fully-configured production config must not panic.
func TestValidate_PassesWithWhatsAppAppSecret(t *testing.T) {
	c := productionConfig()
	assert.NotPanics(t, func() { c.Validate() })
}

// TestValidate_SkipsChecksOutsideProduction confirms the gate only
// applies in production — local/dev boots with no secret must not panic.
func TestValidate_SkipsChecksOutsideProduction(t *testing.T) {
	c := productionConfig()
	c.Environment = "development"
	c.WhatsAppAppSecret = ""
	assert.NotPanics(t, func() { c.Validate() })
}

func TestDBString_FormatsCorrectly(t *testing.T) {
	cfg := &Config{
		DatabaseUser:     "myuser",
		DatabasePassword: "mypass",
		DatabaseHost:     "db.example.com",
		DatabasePort:     "5432",
		DatabaseName:     "mydb",
		DatabaseSSLMode:  "require",
	}

	expected := "postgres://myuser:mypass@db.example.com:5432/mydb?sslmode=require"
	assert.Equal(t, expected, cfg.DBString())
}

func TestDBString_DefaultValues(t *testing.T) {
	clearEnvVars(t)
	cfg := Load()

	expected := "postgres://replider:secret@localhost:5433/replider_db?sslmode=disable"
	assert.Equal(t, expected, cfg.DBString())
}

// IsPlatformAccount gates the vendedores B/C-layer features to the
// Replider platform account (US-001). Unset id ⇒ always false.
func TestIsPlatformAccount(t *testing.T) {
	c := &Config{PlatformAccountID: "acc-platform"}
	assert.True(t, c.IsPlatformAccount("acc-platform"))
	assert.False(t, c.IsPlatformAccount("acc-other"))
	assert.False(t, c.IsPlatformAccount(""))

	unset := &Config{}
	assert.False(t, unset.IsPlatformAccount("acc-platform"))
	assert.False(t, unset.IsPlatformAccount(""))
}

// LEGACY_WORKERS defaults off: the self-serve SaaS workers (CSAT asker,
// digests, win-back, PIX renewal ladder, …) must not run unless prod opts in.
func TestConfig_LegacyWorkersDefaultsOff(t *testing.T) {
	clearEnvVars(t)
	os.Unsetenv("LEGACY_WORKERS")

	assert.False(t, Load().LegacyWorkers)

	t.Setenv("LEGACY_WORKERS", "true")
	assert.True(t, Load().LegacyWorkers)

	t.Setenv("LEGACY_WORKERS", "1")
	assert.False(t, Load().LegacyWorkers, "only the literal \"true\" opts in")
}
