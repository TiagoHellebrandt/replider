package router

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/adapters/captcha"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/adapters/observability"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
)

// Setup configures all the application routes
func Setup(
	r *gin.Engine,
	jwtService domain.JWTService,
	accountHandler *handlers.AccountHandler,
	authHandler *handlers.AuthHandler,
	agentHandler *handlers.AgentHandler,
	agentTemplatesHandler *handlers.AgentTemplatesHandler,
	newsletterHandler *handlers.NewsletterHandler,
	salesHandler *handlers.SalesHandler,
	roiHandler *handlers.ROIHandler,
	attachmentHandler *handlers.AttachmentHandler,
	emailOriginalHandler *handlers.EmailOriginalHandler,
	emailRiskHandler *handlers.EmailRiskHandler,
	brandKitHandler *handlers.BrandKitHandler,
	emailTemplateHandler *handlers.EmailTemplateHandler,
	emailTrackingHandler *handlers.EmailTrackingHandler,
	emailSuppressionHandler *handlers.EmailSuppressionHandler,
	signalHandler *handlers.SignalHandler,
	signalIngestHandler *handlers.SignalIngestHandler,
	autopilotHandler *handlers.AutopilotHandler,
	audienceHandler *handlers.AudienceHandler,
	sequenceHandler *handlers.SequenceHandler,
	strategyHandler *handlers.StrategyHandler,
	saleHandler *handlers.SaleHandler,
	portfolioHandler *handlers.PortfolioHandler,
	// discoveredHandler is "traz os clientes que você já tem" (Sent-folder
	// discovery). Nil is a valid wiring — the routes simply do not mount, and
	// nothing else on the account changes.
	discoveredHandler *handlers.DiscoveredContactHandler,
	// proposalHandler is pillar 3 — priced offers. No send route by design.
	proposalHandler *handlers.ProposalHandler,
	// productImportHandler brings in the price list she already has (pillar 2).
	productImportHandler *handlers.ProductImportHandler,
	// customerImportHandler brings in the customer list she already has — the
	// other half of pillar 2. Until now nothing created a contact at all.
	customerImportHandler *handlers.CustomerImportHandler,
	// googleMailboxHandler connects the mailbox she already has (pillar 1).
	googleMailboxHandler *handlers.GoogleMailboxHandler,
	relationshipHandler *handlers.RelationshipHandler,
	commissionRuleHandler *handlers.CommissionRuleHandler,
	commissionStatementHandler *handlers.CommissionStatementHandler,
	contractHandler *handlers.ContractHandler,
	contractAlertHandler *handlers.ContractAlertHandler,
	signatureHandler *handlers.SignatureHandler,

	calendarHandler *handlers.CalendarHandler,
	knowledgeHandler *handlers.KnowledgeHandler,
	ingestionHandler *handlers.IngestionHandler,
	usageHandler *handlers.UsageHandler,
	meHandler *handlers.MeHandler,
	wsHandler *handlers.WSHandler,
	paymentHandler *handlers.PaymentHandler,
	plansHandler *handlers.PlansHandler,
	conversationHandler *handlers.ConversationHandler,
	mediaHandler *handlers.MediaHandler,
	inboundHandler *handlers.InboundWebhookHandler,
	waWebhookHandler *handlers.WhatsAppWebhookHandler,
	channelHandler *handlers.ChannelHandler,
	waSignupHandler *handlers.WhatsAppSignupHandler,
	waConfigHandler *handlers.WhatsAppConfigHandler,
	webChatHandler *handlers.WebChatHandler,
	campaignHandler *handlers.CampaignHandler,
	productHandler *handlers.ProductHandler,
	funnelHandler *handlers.FunnelHandler,
	leadHandler *handlers.LeadHandler,
	caseMetricsHandler *handlers.CaseMetricsHandler,
	copilotHandler *handlers.CopilotHandler,
	approachExperimentHandler *handlers.ApproachExperimentHandler,
	lgpdHandler *handlers.LGPDHandler,
	auditLogHandler *handlers.AuditLogHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	templatesHandler *handlers.TemplatesHandler,
	transcribeHandler *handlers.TranscribeHandler,
	teamHandler *handlers.TeamHandler,
	cannedResponseHandler *handlers.CannedResponseHandler,
	tagHandler *handlers.TagHandler,
	contactHandler *handlers.ContactHandler,
	exportHandler *handlers.ExportHandler,
	importHandler *handlers.ImportHandler,
	logoutHandler *handlers.LogoutHandler,
	adminHandler *handlers.AdminHandler,
	webhookHandler *handlers.AccountWebhookHandler,
	pushHandler *handlers.PushHandler,
	tgFileHandler *handlers.TelegramFileHandler,
	mediaFileHandler *handlers.MediaFileHandler,
	scheduledHandler *handlers.ScheduledMessageHandler,
	promptHandler *handlers.PromptHandler,
	experimentHandler *handlers.ExperimentHandler,
	ownerHandler *handlers.OwnerHandler,
	sellerHandler *handlers.SellerHandler,
	twoFactorHandler *handlers.TwoFactorHandler,
	couponHandler *handlers.CouponHandler,
	oauthMetadataHandler *handlers.OAuthMetadataHandler,
	oauthAuthorizeHandler *handlers.OAuthAuthorizeHandler,
	oauthTokenHandler *handlers.OAuthTokenHandler,
	mcpTokenHandler *handlers.MCPTokenHandler,
	gcalIntegrationHandler *handlers.GoogleCalendarIntegrationHandler,
	tokenBlacklist domain.TokenBlacklist,
	accountRepo domain.AccountRepository,
	userRepo domain.UserRepository,
	sellerFlagChecker middleware.SellerFlagChecker,
	dbPool *pgxpool.Pool,
	cfg *config.Config,
) {
	// Request size limits
	r.MaxMultipartMemory = 10 << 20 // 10 MB

	// Request ID for tracing
	r.Use(middleware.RequestID())

	// Reject oversized request bodies with 413 (US-192). Runs early — before
	// any handler buffers the body — so a giant payload can't exhaust memory.
	r.Use(middleware.MaxRequestBodySize(cfg.MaxRequestBodyBytes))

	// OpenTelemetry HTTP server spans (US-066). Draws off the global
	// TracerProvider, so it's a no-op until OTEL_EXPORTER_OTLP_ENDPOINT is
	// set. Runs after RequestID so the request_id is already on the context
	// and before Sentry/logging so those nest inside the server span.
	// Health/metrics endpoints are excluded to keep trace volume meaningful.
	r.Use(otelgin.Middleware(
		cfg.OTelServiceName,
		otelgin.WithGinFilter(func(c *gin.Context) bool {
			p := c.Request.URL.Path
			return p != "/health" && p != "/healthz" && p != "/ready" && p != "/metrics"
		}),
	))

	// Sentry capture (no-op when DSN unset). Runs early so any panic in
	// later middleware is reported with a proper stack.
	r.Use(observability.GinMiddleware())

	// Security headers
	r.Use(middleware.SecurityHeaders())

	// CORS — restrict to allowed origins. The dev fallback (allow-all)
	// is gated on Environment != production so a missing env var on a
	// production deploy fails closed rather than silently exposing the
	// API to any origin.
	allowedOrigins := buildAllowedOrigins(cfg.CORSAllowedOrigins)
	if len(allowedOrigins) == 0 {
		if cfg.Environment == "production" {
			slog.Error("CORS_ALLOWED_ORIGINS is empty in production — rejecting all cross-origin requests")
		} else {
			slog.Warn("CORS_ALLOWED_ORIGINS is empty, allowing all origins (dev mode)")
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return originAllowed(allowedOrigins, origin)
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Mcp-Session-Id", "MCP-Protocol-Version"},
		ExposeHeaders:    []string{"Content-Length", "Mcp-Session-Id", "WWW-Authenticate"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Structured request logging (JSON via slog)
	r.Use(middleware.StructuredLogger())

	// Prometheus request metrics (US-067): http_requests_total counter +
	// http_request_duration_seconds histogram, labelled by the route
	// PATTERN (c.FullPath()) so cardinality stays bounded. Always on — the
	// series are cheap and the /metrics scrape below reads the same default
	// registry the promauto collectors register into.
	r.Use(middleware.PrometheusMetrics())

	// Per-request 30s deadline (US-034). Bounds every handler's work and,
	// with the server's ReadHeaderTimeout, defeats slowloris connection
	// holding. WebSocket upgrades are exempt inside the middleware.
	r.Use(middleware.Timeout(30 * time.Second))

	// Stamp the request origin (client IP + User-Agent) onto the context
	// so the audit log writer records "from where" for every admin
	// mutation (US-073). Cheap value-on-context; read back in the audit
	// repository via domain.GetAuditMeta.
	r.Use(middleware.AuditMeta())

	// Deep Health Check route
	healthHandler := handlers.NewHealthHandler(dbPool, cfg)
	// US-F015: probe the media bucket for real (a config check never catches
	// revoked credentials or a deleted bucket). The adapter is stateless —
	// it builds a client per call — so a second instance here is free.
	healthHandler.SetStorageProbe(storage.NewGCSAdapter(cfg.GCSBucketName, cfg.GoogleCredentialsBase64))
	r.GET("/health", healthHandler.HandleHealth)
	// Operator breakdown (vendors, hosts, probe errors) — METRICS_TOKEN only.
	r.GET("/health/details", healthHandler.HandleHealthDetails)
	// Readiness probe (US-068): checks critical dependencies (DB, AI, GCS,
	// Stripe) are reachable before the pod is sent traffic.
	r.GET("/ready", healthHandler.HandleReady)

	// Prometheus scrape endpoint (US-067). Serves the default registry
	// (HTTP, LLM and webhook collectors all register into it via promauto)
	// in the text exposition format. Excluded from tracing above; intended
	// to be locked down at the reverse-proxy / network layer rather than in
	// app auth so the scraper needs no JWT.
	// METRICS_TOKEN, when set, additionally demands a bearer token so a
	// misconfigured proxy never exposes the scrape (2026-09-01 hardening).
	r.GET("/metrics", middleware.MetricsAuth(cfg.MetricsToken), gin.WrapH(promhttp.Handler()))

	// Public legal links endpoint. Required by Meta App review (Live
	// mode), most BSP audits, and the WhatsApp Cloud API onboarding
	// flow which surfaces a "Privacy Policy" + "Terms of Service"
	// link to merchants. Empty values render as null so the
	// dashboard can hide / disable the link rather than send to a
	// 404. Returns 200 always so frontends can branch on emptiness
	// without an extra status check.
	r.GET("/legal", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"privacy_policy_url":   cfg.PrivacyPolicyURL,
			"terms_of_service_url": cfg.TermsOfServiceURL,
		})
	})

	// Swagger route
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API Routes Group
	v1 := r.Group("/v1")

	// Create middleware instance. Wrap the JWT validator with the
	// last-seen tracker so every authenticated request updates
	// users.last_seen_at (throttled at the SQL layer to once per 5m).
	var rawAuthMw gin.HandlerFunc
	if tokenBlacklist != nil {
		rawAuthMw = middleware.AuthMiddleware(jwtService, tokenBlacklist)
	} else {
		rawAuthMw = middleware.AuthMiddleware(jwtService)
	}
	trackMw := middleware.TrackActivity(userRepo)
	impersonationAuditMw := middleware.ImpersonationAudit()
	// A token minted before users.session_invalidated_at (member removed)
	// is refused even though it is unexpired and not on the blacklist.
	// The postgres repository implements the reader; a test double that
	// does not simply gets a pass-through.
	sessionReader, _ := userRepo.(domain.SessionInvalidationReader)
	sessionGuardMw := middleware.SessionInvalidationGuard(sessionReader)

	// US-051: default token-bucket limiter keyed per (account_id, ip).
	// Runs as the last step of authMw so accountID is already populated.
	// 100 req/min sustained per account+ip. Disabled in test env to avoid
	// false failures in handler tests. Abuse-sensitive public routes keep
	// their own stricter per-IP limits below.
	var defaultRL gin.HandlerFunc
	if cfg.Environment == "test" {
		defaultRL = func(c *gin.Context) { c.Next() }
	} else {
		defaultRL = middleware.PerAccountIPRateLimiter(middleware.RateLimitConfig{
			RequestsPerMinute: middleware.DefaultRateLimitPerMinute,
		})
	}

	authMw := func(c *gin.Context) {
		rawAuthMw(c)
		if c.IsAborted() {
			return
		}
		sessionGuardMw(c)
		if c.IsAborted() {
			return
		}
		trackMw(c)
		// Promote impersonation actor onto request context + slog
		// forensic line on every write done under the session.
		impersonationAuditMw(c)
		if c.IsAborted() {
			return
		}
		defaultRL(c)
	}

	// Rate limiter factory: disabled in test environment to avoid false failures
	rl := func(format string) gin.HandlerFunc {
		if cfg.Environment == "test" {
			return func(c *gin.Context) { c.Next() }
		}
		return middleware.RateLimiter(format)
	}

	// Rate limiters for abuse-sensitive routes
	chatRateLimit := rl("30-M")    // 30 req/min per IP for chat
	checkoutRateLimit := rl("5-M") // 5 req/min per IP for checkout

	// Per-account rate limiter for AI-heavy endpoints. Protects shared
	// provider quota from a single customer with a runaway integration
	// or compromised key. Disabled in test env like other limits.
	accountRL := func(format string) gin.HandlerFunc {
		if cfg.Environment == "test" {
			return func(c *gin.Context) { c.Next() }
		}
		return middleware.RateLimiterByAccount(format)
	}
	aiAccountLimit := accountRL("120-M") // 120 AI-bound req/min per account

	// US-F012: per-(IP, email) credential-stuffing cap on /auth/login,
	// layered under the per-IP limiter. 5/min matches the audit's number;
	// burst 5 so a human retrying a typo is never throttled but a script is.
	var loginRL gin.HandlerFunc
	if cfg.Environment == "test" {
		loginRL = func(c *gin.Context) { c.Next() }
	} else {
		loginRL = middleware.LoginRateLimiter(middleware.RateLimitConfig{
			RequestsPerMinute: 5,
			Burst:             5,
		})
	}

	// US-052: per-conversation message rate limit. Caps how fast messages can
	// be pushed into a single conversation thread (keyed by the `:id` path
	// param), independent of the per-account/per-IP budget. Disabled in test
	// env like the other limiters so handler tests never hit 429.
	var convMsgRateLimit gin.HandlerFunc
	if cfg.Environment == "test" {
		convMsgRateLimit = func(c *gin.Context) { c.Next() }
	} else {
		convMsgRateLimit = middleware.PerConversationRateLimiter(middleware.RateLimitConfig{
			RequestsPerMinute: 60,
		})
	}

	// Paid-feature guard: returns 402 when trial expired or subscription is
	// past_due/canceled. Applied after auth on resource routes (agents,
	// channels, knowledge, calendars, campaigns, conversations). Deliberately
	// NOT applied to /auth, /me, /billing, /accounts/usage and webhooks —
	// those must remain reachable so the user can log in, see their billing
	// state, and pay to restore access.
	paidMw := middleware.RequireActiveSubscription(accountRepo)

	// authPaidMw stacks auth + subscription check into a single handler so
	// existing setupXxxRoutes signatures (which take one gin.HandlerFunc) can
	// opt into paid-tier gating without changing their shape.
	authPaidMw := func(c *gin.Context) {
		authMw(c)
		if c.IsAborted() {
			return
		}
		paidMw(c)
	}

	// US-054: escalating captcha on signup. After 3 attempts/hour from one
	// IP a valid Cloudflare Turnstile token becomes mandatory. Disabled in
	// the test env (so handler tests never need a token) and when no secret
	// is configured (local dev / unconfigured) — NewTurnstileVerifier
	// returns nil for an empty secret and SignupCaptchaGuard no-ops on a nil
	// verifier.
	var signupCaptcha gin.HandlerFunc
	if verifier := captcha.NewTurnstileVerifier(cfg.TurnstileSecretKey, nil); cfg.Environment == "test" || verifier == nil {
		// Explicit nil-pointer check avoids the typed-nil interface trap:
		// passing a nil *TurnstileVerifier as a CaptchaVerifier yields a
		// non-nil interface, so we must branch on the concrete pointer here.
		signupCaptcha = func(c *gin.Context) { c.Next() }
	} else {
		signupCaptcha = middleware.SignupCaptchaGuard(verifier, middleware.CaptchaConfig{})
	}

	// Setup modular routes
	setupAccountRoutes(v1, accountHandler, usageHandler, authMw, rl, signupCaptcha)
	setupAuthRoutes(v1, authHandler, rl, loginRL)
	// OAuth 2.1 AS for the remote MCP integration (root-level, RFC 8414)
	// + dashboard-facing PAT/connection management under /v1/me.
	if oauthMetadataHandler != nil && oauthAuthorizeHandler != nil && oauthTokenHandler != nil {
		setupOAuthRoutes(r, oauthMetadataHandler, oauthAuthorizeHandler, oauthTokenHandler, rl)
	}
	if mcpTokenHandler != nil {
		setupMCPTokenRoutes(v1, mcpTokenHandler, authMw)
	}

	// Logout (requires auth — revokes the current token)
	if logoutHandler != nil {
		v1.POST("/auth/logout", authMw, logoutHandler.HandleLogout)
	}
	setupNewsletterRoutes(v1, newsletterHandler, rl)
	setupSalesRoutes(v1, salesHandler, rl)
	setupROILeadRoutes(v1, roiHandler, rl)
	setupAgentRoutes(v1, agentHandler, agentTemplatesHandler, calendarHandler, authPaidMw, rl)
	if gcalIntegrationHandler != nil {
		setupIntegrationRoutes(v1, gcalIntegrationHandler, authMw, rl)
	}
	// Pillar 1. Nil when the platform has no Google client configured, which
	// leaves the SMTP path exactly as it was.
	setupGoogleMailboxRoutes(v1, googleMailboxHandler, authMw, rl)
	setupKnowledgeRoutes(v1, knowledgeHandler, ingestionHandler, authPaidMw)

	// Setup user & ws routes inline or modularly
	// 6/h per ACCOUNT for the catalogue hunt: it writes leads, and the honest
	// reason to press it twice in a minute does not exist — the second press
	// would find the ground the first one just took.
	setupUserRoutes(v1, meHandler, twoFactorHandler, authMw, rl("10-M"), accountRL("6-H"))
	setupWSRoutes(v1, wsHandler, authMw, chatRateLimit)

	// Seller (vendedores) panel — platform account only, 404 elsewhere;
	// explicit seller flag required inside it (US-014).
	if sellerHandler != nil {
		setupSellerRoutes(v1, sellerHandler, authMw, cfg.PlatformAccountID, sellerFlagChecker)
	}

	// Public: Plan listing for Flutter pricing page. Auth is optional —
	// an authenticated call is pinned to the account's billing currency
	// (US-002); an anonymous one filters via ?currency= (default brl).
	v1.GET("/plans", middleware.OptionalAuthMiddleware(authMw), plansHandler.HandleListPlans)
	v1.GET("/plans/contact-channels", plansHandler.HandleContactChannels)

	// Public: client bootstrap flags (US-F001). Built from cfg here rather
	// than threaded through Setup's already-long handler list — it carries
	// no state beyond the config value.
	v1.GET("/config", handlers.NewPublicConfigHandler(cfg.RegistrationEnabled).HandleGetPublicConfig)

	// Public payment webhooks
	v1.POST("/payments/webhook", paymentHandler.Webhook)
	// AbacatePay (BRL PIX + card) webhook — separate path from the
	// inbound-message `/webhooks/:channel_token` tree to avoid the gin
	// static-vs-param collision.
	v1.POST("/payments/abacatepay-webhook", paymentHandler.AbacatePayWebhook)

	// Public: list addon credit packs
	v1.GET("/payments/addon-packs", paymentHandler.ListAddonPacks)

	// Protected Payment routes (rate limited)
	setupPaymentRoutes(v1, paymentHandler, authMw, checkoutRateLimit)

	// Protected Analytics routes
	setupAnalyticsRoutes(v1, analyticsHandler, authMw)

	// Public: Prompt templates
	v1.GET("/templates/prompts", templatesHandler.HandleListPromptTemplates)

	// Protected Conversation routes
	setupConversationRoutes(v1, conversationHandler, authMw, convMsgRateLimit)
	setupAttachmentRoutes(v1, attachmentHandler, emailOriginalHandler, emailRiskHandler, authMw)
	setupBrandRoutes(v1, brandKitHandler, authMw)
	setupEmailTemplateRoutes(v1, emailTemplateHandler, authMw)
	setupEmailSuppressionRoutes(v1, emailSuppressionHandler, authMw)
	setupSignalRoutes(v1, signalHandler, authMw)
	setupAutopilotRoutes(v1, autopilotHandler, authMw)
	setupAudienceRoutes(v1, audienceHandler, authMw)
	setupSequenceRoutes(v1, sequenceHandler, authMw)
	setupStrategyRoutes(v1, strategyHandler, authMw)
	setupSaleRoutes(v1, saleHandler, authMw)
	setupPortfolioRoutes(v1, portfolioHandler, authMw)
	setupRelationshipRoutes(v1, relationshipHandler, authMw)
	setupCommissionRuleRoutes(v1, commissionRuleHandler, authMw)
	setupCommissionStatementRoutes(v1, commissionStatementHandler, authMw)
	setupContractRoutes(v1, contractHandler, authMw)
	setupContractAlertRoutes(v1, contractAlertHandler, authMw)
	setupSignatureRoutes(v1, signatureHandler, authMw)
	// PUBLIC: the provider cannot hold a JWT. The token in the path is the
	// credential, and the handler re-reads the truth from the provider before it
	// writes anything — a forged callback causes a read, not a sale.
	if signatureHandler != nil {
		v1.POST("/webhooks/signature/:token", signatureHandler.HandleCallback)
	}
	if signalIngestHandler != nil {
		// Machine-to-machine: no session, authenticated by an HMAC over the
		// body. Rate limited because it is unauthenticated at the JWT layer
		// and a wrong signature is cheap for the caller to retry.
		v1.POST("/webhooks/signals/:account_id",
			rl("120-M"), signalIngestHandler.HandleIngest)
	}
	setupTrackingRoutes(v1, emailTrackingHandler)

	// Protected Media routes
	setupMediaRoutes(v1, mediaHandler, authMw)

	// Protected Team routes
	setupTeamRoutes(v1, teamHandler, authMw)

	// Protected Channel routes (gated on paid tier — main product feature)
	setupChannelRoutes(v1, channelHandler, waSignupHandler, waConfigHandler, authPaidMw)
	setupWebhookRoutes(v1, webhookHandler, authMw)

	// Scheduled outbound messages: dashboard form posts here, the
	// dispatcher worker drains rows when send_at falls due.
	if scheduledHandler != nil {
		sched := v1.Group("/scheduled-messages", authPaidMw, middleware.RequireRole("admin", "attendant"))
		sched.POST("", scheduledHandler.HandleSchedule)
		sched.GET("", scheduledHandler.HandleList)
		sched.DELETE("/:id", scheduledHandler.HandleCancel)
	}

	// Protected Campaign routes
	if campaignHandler != nil {
		setupCampaignRoutes(v1, campaignHandler, authPaidMw)
	}

	// Protected Product routes (outbound sales catalog — US-004)
	if productHandler != nil {
		setupProductRoutes(v1, productHandler, authPaidMw)
	}
	// Deliberately NOT inside the block above: see setupProductImportRoutes.
	setupProductImportRoutes(v1, productImportHandler, authPaidMw)

	// Protected Funnel routes (outbound sales — AI persona gen US-005)
	if funnelHandler != nil {
		setupFunnelRoutes(v1, funnelHandler, authPaidMw, aiAccountLimit)
	}

	// Protected Lead/CRM routes (outbound sales — leads + metadata US-015).
	// aiAccountLimit guards ONLY the AI-spending subset (draft, draft-batch,
	// enrich) — plain CRM reads/edits keep the default budget.
	if leadHandler != nil {
		setupLeadRoutes(v1, leadHandler, caseMetricsHandler, copilotHandler, approachExperimentHandler, authPaidMw, aiAccountLimit)
	}

	// Admin-only LGPD titular-rights endpoints
	if lgpdHandler != nil {
		setupLGPDRoutes(v1, lgpdHandler, authMw)
	}

	// Admin-only server infra canaries (SMTP test, future: webhook ping,
	// queue depth). Group is admin-gated so attendants can't spam email.
	if adminHandler != nil {
		admin := v1.Group("/admin")
		admin.Use(authMw)
		admin.Use(middleware.RequireRole("admin"))
		admin.POST("/smtp/test", adminHandler.HandleSendSMTPTest)
		admin.GET("/wa-webhook-events", adminHandler.HandleListWAWebhookEvents)
		admin.POST("/wa-webhook-events/:id/retry", adminHandler.HandleRetryWAWebhookEvent)
		admin.GET("/wa-meter", adminHandler.HandleListWABillingMeter)
		admin.GET("/agents/invalid-timezones", adminHandler.HandleListInvalidTimezoneAgents)
		admin.POST("/accounts/:id/trial-bypass", adminHandler.HandleSetTrialBypass)
		admin.GET("/billing/metrics", adminHandler.HandleGetBillingMetrics)
		admin.GET("/accounts/health", adminHandler.HandleListAccountHealth)
		admin.GET("/nps", adminHandler.HandleGetNPSMetrics)
		admin.GET("/cancellation-survey", adminHandler.HandleGetCancellationSurvey)
	}

	// Admin-only audit log listing (separate route group so it can be
	// disabled independently of LGPD).
	if auditLogHandler != nil {
		setupAuditLogRoutes(v1, auditLogHandler, authMw)
	}

	// Protected Canned Response routes
	setupCannedResponseRoutes(v1, cannedResponseHandler, authMw)

	// Prompt builder + library + versioning. Synthesis is a paid
	// feature (consumes tokens via the same usage meter); the apply
	// path mutates an agent so the per-agent gating must already be
	// satisfied via the Agent admin role enforced inside the route
	// group.
	if promptHandler != nil {
		// Refinement is rate-limited at 30/H per account; same bucket as
		// synthesis. Plan gating (Pro+) enforced inside the usecase.
		refineRL := accountRL("30-H")
		// Test scenarios fan out to ~10 LLM calls per run — cap at 5/day
		// per account. Plan gating (Business+) enforced inside the
		// usecase.
		testScenariosRL := accountRL("5-D")
		setupPromptRoutes(v1, promptHandler, authPaidMw, refineRL, testScenariosRL)
	}

	// Owner / super-admin panel. Allowlist comes from
	// SUPER_ADMIN_EMAILS (CSV). Empty allowlist fails closed inside
	// the middleware, so a misconfigured deploy can't accidentally
	// expose the routes.
	if ownerHandler != nil {
		superAdminMw := middleware.RequireSuperAdmin(userRepo, cfg.SuperAdminEmails(), cfg.SuperAdminRequire2FA)
		setupOwnerRoutes(v1, ownerHandler, authMw, superAdminMw)
		if couponHandler != nil {
			coupons := v1.Group("/owner/coupons", authMw, superAdminMw)
			coupons.POST("", couponHandler.HandleCreate)
			coupons.GET("", couponHandler.HandleList)
			coupons.POST("/:id/deactivate", couponHandler.HandleDeactivate)
		}
	}
	if couponHandler != nil {
		// Public lookup so the register / billing page can render
		// "X% off applied" before the user is authenticated. No PII;
		// invalid codes return {valid:false}.
		v1.GET("/coupons/:code", couponHandler.HandlePublicLookup)
	}

	// A/B testing (MVP-2). Pro+ gate enforced inside the usecase so
	// 403 PLAN_FEATURE_LOCKED carries an upsell payload to the UI.
	if experimentHandler != nil {
		// Auto-suggester (MVP-3) is expensive — cap at 5/day per
		// account. Plan gating (Business+) is enforced inside the
		// use case so the rate limiter doesn't have to know about
		// plan tiers.
		suggestRL := accountRL("5-D")
		setupExperimentRoutes(v1, experimentHandler, authPaidMw, suggestRL)
	}

	// Protected Tag routes
	setupTagRoutes(v1, tagHandler, authMw)

	// Protected Contact routes
	setupContactRoutes(v1, contactHandler, authMw)
	setupDiscoveredContactRoutes(v1, discoveredHandler, authMw)
	// Deliberately NOT inside setupContactRoutes: see setupCustomerImportRoutes.
	setupCustomerImportRoutes(v1, customerImportHandler, authMw)
	setupProposalRoutes(v1, proposalHandler, authMw)

	// Protected Push routes (web + future mobile)
	if pushHandler != nil {
		setupPushRoutes(v1, pushHandler, authMw)
	}

	// Import routes
	importGroup := v1.Group("/import", authMw, middleware.RequireRole("admin"))
	{
		importGroup.POST("/conversations", importHandler.HandleImportConversation)
	}

	// Export routes (rate limited: 1/min)
	// J1: machine keys and the public API surface they authenticate. Wired
	// through package-level vars set by main so router.Setup keeps its
	// signature — the same reason every other optional dependency uses a
	// setter.
	setupAPIKeyRoutes(v1, apiKeyHandler, authMw)
	setupPublicAPIRoutes(v1, exportHandler, analyticsHandler, webhookHandler,
		apiKeyAuthMiddleware, apiKeyRateLimitMiddleware)

	exportGroup := v1.Group("/export", authMw, middleware.RequireRole("admin"))
	{
		exportGroup.GET("/conversations", exportHandler.HandleExportConversations)
		exportGroup.GET("/messages", exportHandler.HandleExportMessages)
		// The export trail (I3). Registered BEFORE /:entity so "audits" is a
		// route and not an entity name — gin matches static segments first, but
		// stating the order makes the dependency visible.
		exportGroup.GET("/audits", exportHandler.HandleListExportAudits)
		// Universal export: same filters as the screen, CSV or XLSX, every
		// download recorded.
		exportGroup.GET("/:entity", exportHandler.HandleUniversalExport)
	}

	// Per-message on-demand transcription (audio/voice bubbles).
	if transcribeHandler != nil {
		msgGroup := v1.Group("/messages", authMw, middleware.RequireRole("admin", "attendant"))
		msgGroup.POST("/:id/transcribe", transcribeHandler.HandleTranscribe)
	}

	// Protected conversation notes + search routes
	convNotesGroup := v1.Group("/conversations", authMw, middleware.RequireRole("admin", "attendant"))
	{
		convNotesGroup.POST("/:id/notes", conversationHandler.HandleCreateNote)
		convNotesGroup.GET("/:id/notes", conversationHandler.HandleListNotes)
		convNotesGroup.DELETE("/:id/notes/:noteId", conversationHandler.HandleDeleteNote)
	}
	searchGroup := v1.Group("/conversations", authMw, middleware.RequireRole("admin", "attendant"))
	{
		searchGroup.GET("/search", conversationHandler.HandleSearchMessages)
	}

	// Public Telegram file proxy — HMAC-signed URLs keep the bot token
	// server-side while letting <img>/NetworkImage load avatars without
	// auth headers. Rate-limited same as widget/webhook to bound abuse.
	if tgFileHandler != nil {
		tgFileRateLimit := rl("120-M")
		v1.GET("/telegram/file", tgFileRateLimit, tgFileHandler.Proxy)
	}

	// Conversation media, streamed from GCS through the API. Same shape and
	// same reasoning as the Telegram proxy above: unauthenticated because an
	// <img> cannot attach an Authorization header, with an HMAC signature as
	// the auth. It exists because a GCS bucket sends no CORS headers, so the
	// signed URLs this replaced failed in the dashboard while opening fine in
	// a browser tab.
	if mediaFileHandler != nil {
		v1.GET("/media/file", rl("240-M"), mediaFileHandler.Proxy)
	}

	// Public Inbound Webhook routes (no auth, rate limited)
	webhookRateLimit := rl("60-M") // 60 req/min per IP for inbound webhooks
	// WhatsApp BSP: single global endpoint, routes internally by phone_number_id
	v1.GET("/webhooks/whatsapp", waWebhookHandler.HandleVerify)
	v1.POST("/webhooks/whatsapp", webhookRateLimit, waWebhookHandler.HandleInbound)
	// Telegram + others: per-channel token
	v1.POST("/webhooks/:channel_token", webhookRateLimit, inboundHandler.HandleInboundWebhook)
	// Meta hub.challenge verification for per-channel subscriptions
	// (Instagram DM). The operator sets the channel token as the Verify
	// Token in the Meta App Dashboard.
	v1.GET("/webhooks/:channel_token", webhookRateLimit, inboundHandler.HandleWebhookVerify)

	// Public webchat widget endpoints (per-channel CORS, stricter rate-limit)
	if webChatHandler != nil {
		widgetRateLimit := rl("60-M") // 60 req/min per IP per channel; AI cost is bounded
		widget := v1.Group("/widget/:channel_token", widgetRateLimit)
		{
			widget.OPTIONS("/messages", webChatHandler.HandlePreflight)
			widget.OPTIONS("/bootstrap", webChatHandler.HandlePreflight)
			widget.OPTIONS("/csat", webChatHandler.HandlePreflight)
			widget.GET("/bootstrap", webChatHandler.HandleBootstrap)
			widget.POST("/messages", webChatHandler.HandleSendMessage)
			widget.GET("/messages", webChatHandler.HandlePollMessages)
			// Anonymous CSAT submission — tied to channel_token + visitor_id.
			widget.POST("/csat", webChatHandler.HandleRecordCSAT)
		}
	}
}

func setupKnowledgeRoutes(v1 *gin.RouterGroup, h *handlers.KnowledgeHandler, ingestion *handlers.IngestionHandler, authMw gin.HandlerFunc) {
	knowledge := v1.Group("/knowledge")
	knowledge.Use(authMw, middleware.RequireRole("admin"))
	{
		knowledge.POST("/sources", h.HandleCreateSource)
		knowledge.GET("/sources", h.HandleListSources)
		knowledge.POST("/sources/upload", h.HandleUploadFile)
		knowledge.POST("/sources/url", h.HandleCrawlURL)
		knowledge.POST("/sources/:id/chunks", h.HandleIngestKnowledge)
		knowledge.GET("/sources/:id/chunks", h.HandleListChunks)
		knowledge.DELETE("/sources/:id", h.HandleDeleteSource)
		knowledge.DELETE("/chunks/:id", h.HandleDeleteKnowledge)
		// Mission 4: async single-URL ingestion + job polling.
		knowledge.POST("/sources/:id/ingest-url", ingestion.HandleIngestURL)
		knowledge.GET("/sources/:id/jobs", ingestion.HandleListIngestionJobs)
		knowledge.GET("/ingestion-jobs/:id", ingestion.HandleGetIngestionJob)
		// Mission 5: async PDF ingestion — mint signed PUT URL,
		// then confirm + enqueue once the app has uploaded.
		knowledge.POST("/sources/:id/upload-url", ingestion.HandleUploadURL)
		knowledge.POST("/sources/:id/ingest-pdf", ingestion.HandleIngestPDF)
	}
}

func setupPaymentRoutes(v1 *gin.RouterGroup, paymentHandler *handlers.PaymentHandler, authMw gin.HandlerFunc, rateLimit gin.HandlerFunc) {
	protected := v1.Group("/payments")
	protected.Use(authMw, middleware.RequireRole("admin"), rateLimit)
	{
		protected.POST("/checkout", paymentHandler.Checkout)
		protected.POST("/addon-checkout", paymentHandler.AddonCheckout)
		protected.POST("/billing-portal", paymentHandler.BillingPortal)
		protected.POST("/retry-payment", paymentHandler.RetryPayment)
		// Country-routed transparent checkout (BRL→AbacatePay PIX/card,
		// non-BRL→Stripe) + PIX charge-status poll.
		protected.POST("/transparent-checkout", paymentHandler.TransparentCheckout)
		protected.GET("/transparent-checkout/:id/status", paymentHandler.ChargeStatus)
	}

	// US-175: canonical self-serve subscription-management endpoint. Same
	// handler as /payments/billing-portal (kept for back-compat), exposed
	// under the /billing namespace the app's "Manage subscription" button
	// now targets.
	billing := v1.Group("/billing")
	billing.Use(authMw, middleware.RequireRole("admin"), rateLimit)
	{
		billing.POST("/portal-session", paymentHandler.BillingPortal)
	}
}

func setupUserRoutes(v1 *gin.RouterGroup, meHandler *handlers.MeHandler, twoFactorHandler *handlers.TwoFactorHandler, authMw gin.HandlerFunc, stepUpRateLimit gin.HandlerFunc, autoHuntRateLimit gin.HandlerFunc) {
	protected := v1.Group("", authMw)
	{
		protected.GET("/me", meHandler.HandleGetMe)
		protected.GET("/me/dashboard", meHandler.HandleGetDashboard)
		// The home command centre (I2). Role is enforced in the use case
		// (managers see the floor, salespeople see their own work), so the
		// route stays open to the dashboard roles.
		protected.GET("/me/command-center", meHandler.HandleGetCommandCenter)
		protected.POST("/me/pending-actions/dismiss", meHandler.HandleDismissPendingAction)
		// The approval queue's weekly feedback line (Ofertas → Hoje).
		protected.GET("/me/offers/outcome", meHandler.HandleOffersOutcome)
		// The opportunity radar (Ofertas phase 2) and its park-offer action.
		protected.GET("/me/radar", meHandler.HandleGetRadar)
		protected.POST("/me/radar/park-offer", meHandler.HandleRadarParkOffer)
		protected.POST("/me/territory/hunt", meHandler.HandleTerritoryHunt)
		// The same sweep the nightly worker runs, on demand. Rate-limited per
		// account rather than per IP: it writes leads, and the honest reason
		// to press it twice in a minute does not exist — the second press
		// would find the ground the first one just took.
		protected.POST("/me/territory/auto-hunt",
			autoHuntRateLimit, meHandler.HandleAutoHunt)
		// K2: the AI fills the approval queue overnight. Admin-only — it
		// spends the account's AI allowance on a schedule.
		// K5: the onboarding conversation. Any role — the person who signs up
		// is whoever signed up, and gating it on admin would strand a team
		// whose first login is not the owner's.
		protected.GET("/me/onboarding-chat", meHandler.HandleOnboardingChat)
		protected.POST("/me/onboarding-chat", meHandler.HandleOnboardingAnswer)
		protected.GET("/me/overnight-drafting", meHandler.HandleGetOvernightSettings)
		protected.PUT("/me/overnight-drafting", middleware.RequireRole("admin"), meHandler.HandlePutOvernightSettings)
		protected.POST("/me/overnight-drafting/:id/seen", meHandler.HandleSeenBriefing)
		protected.PUT("/me", meHandler.HandleUpdateProfile)
		protected.PATCH("/me/password", meHandler.HandleChangePassword)
		protected.GET("/me/plan-limits", meHandler.HandleGetPlanLimits)
		protected.GET("/me/referral", meHandler.HandleGetReferral)
		protected.GET("/me/onboarding-progress", meHandler.HandleGetOnboardingProgress)
		protected.POST("/me/onboarding-checklist/dismiss", meHandler.HandleDismissOnboardingChecklist)
		protected.POST("/me/onboarding/first-offer", meHandler.HandleFirstOffer)
		protected.PUT("/me/onboarding-draft", meHandler.HandleUpdateOnboardingDraft)
		protected.POST("/me/nps", meHandler.HandleSubmitNPS)
		protected.GET("/me/nps-prompt", meHandler.HandleNPSPrompt)
		protected.POST("/me/nps/dismiss", meHandler.HandleDismissNPS)
		protected.POST("/me/cancellation-survey", meHandler.HandleSubmitCancellationSurvey)
		// Internal CS channel (canal-interno US-002): the user's own
		// support thread with the platform. Any authenticated role —
		// admin and attendant each get their own thread.
		protected.GET("/me/support/messages", meHandler.HandleListSupportMessages)
		protected.POST("/me/support/messages", meHandler.HandleSendSupportMessage)
		if twoFactorHandler != nil {
			protected.GET("/me/2fa/status", twoFactorHandler.HandleStatus)
			protected.POST("/me/2fa/setup", twoFactorHandler.HandleSetup)
			protected.POST("/me/2fa/verify", twoFactorHandler.HandleVerify)
			// Step-up: refreshes an expired 2FA proof in place so the owner
			// panel never has to send the operator through a full re-login.
			// Same rate limit as the login verify — a code guess costs the
			// same wherever it is tried.
			protected.POST("/me/2fa/reverify", stepUpRateLimit, twoFactorHandler.HandleReverify)
			protected.POST("/me/2fa/disable", twoFactorHandler.HandleDisable)
		}
		// BYOK keys: scoped to admins because these are billable
		// credentials. The use-case also enforces "contract plans only"
		// so a stale client on a self-serve account can't accidentally
		// store a key.
		adminOnly := protected.Group("", middleware.RequireRole("admin"))
		adminOnly.PATCH("/me/api-keys", meHandler.HandleSetAPIKeys)
		adminOnly.GET("/me/ai-costs", meHandler.HandleGetAICosts)
		adminOnly.PUT("/me/ai-budget", meHandler.HandleSetAIBudget)
	}
}

// setupAttachmentRoutes exposes inbound message attachments. Dashboard roles
// (admin + attendant) — an attachment is part of a conversation, so anyone who
// may read the conversation may read what came with it.
// setupBrandRoutes exposes the company voice layer and the personal overlay.
//
// Two permission shapes on purpose: the company layer decides what the
// BUSINESS may claim, which is an admin decision, while the overlay is how
// one person signs their own messages.
// setupTrackingRoutes exposes the two endpoints a mail client hits.
//
// PUBLIC on purpose: the caller is Gmail fetching an image, or a customer
// clicking a link in their inbox. Neither has a session, and the signed token
// is the authorisation — it can increment a counter and redirect to a
// destination that was signed at send time, and nothing else.
func setupTrackingRoutes(v1 *gin.RouterGroup, h *handlers.EmailTrackingHandler) {
	if h == nil {
		return
	}
	v1.GET("/t/o/:token", h.HandleOpen)
	v1.GET("/t/c/:token", h.HandleClick)
}

func setupBrandRoutes(v1 *gin.RouterGroup, h *handlers.BrandKitHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	admin := v1.Group("", authMw, middleware.RequireRole("admin"))
	{
		admin.PUT("/brand-kit", h.HandleSave)
		admin.PUT("/objections", h.HandleSaveObjections)
		// The "why ours" library: written by an admin, because it is what the
		// company commits to saying.
		admin.PUT("/differentiators", h.HandleSaveDifferentiators)
	}
	// Reading it is not an admin act: a salesperson needs to know what they
	// may and may not claim.
	dash := v1.Group("", authMw, middleware.RequireRole("admin", "attendant"))
	{
		dash.GET("/brand-kit", h.HandleGet)
		// The library is read by whoever hears the pushback.
		dash.GET("/objections", h.HandleListObjections)
		// Read by anyone on the floor: a seller needs to know the argument.
		dash.GET("/differentiators", h.HandleListDifferentiators)
		dash.GET("/me/seller-overlay", h.HandleGetOverlay)
		dash.PUT("/me/seller-overlay", h.HandleSaveOverlay)
	}
}

// setupEmailTemplateRoutes exposes the template library. Dashboard roles: a
// salesperson writes the messages, so they write the templates.
func setupEmailTemplateRoutes(v1 *gin.RouterGroup, h *handlers.EmailTemplateHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/email-templates", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("", h.HandleList)
		g.GET("/:id", h.HandleGet)
		g.PUT("/:id", h.HandleUpsert)
		g.DELETE("/:id", h.HandleDelete)
	}
}

// setupSignalRoutes exposes public buying signals (§2.1).
//
// Dashboard roles: a salesperson is who acts on a tender, and the watches
// that select them are part of doing that job, not an admin setting.
func setupSignalRoutes(v1 *gin.RouterGroup, h *handlers.SignalHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("/signals", h.HandleList)
		g.GET("/signal-sources", h.HandleListSources)
		// The buying calendar: what runs out, and when. Same dashboard roles as
		// the radar — it is the same job, earlier.
		g.GET("/contract-calendar", h.HandleContractCalendar)
		// A calendar you can only read is a calendar read once. This is the
		// door from a date to a card in the funnel.
		g.POST("/contract-calendar/:id/work", h.HandleWorkContractWindow)
		g.POST("/signals/:id/dismiss", h.HandleDismiss)
		// "I will chase this one myself": the tender becomes a card in the
		// funnel with no AI draft and nothing sent.
		g.POST("/signals/:id/work", h.HandleWorkSignal)
		g.GET("/signal-watches", h.HandleListWatches)
		g.PUT("/signal-watches/:id", h.HandleUpsertWatch)
		g.DELETE("/signal-watches/:id", h.HandleDeleteWatch)
	}
}

// setupAutopilotRoutes exposes progressive autopilot (§3.4).
//
// Admin-only to change: this decides whether cold email leaves without anyone
// reading it, which is an account-level risk rather than a personal
// preference. Reading it — including the audit of what it sent — is open to
// the dashboard roles, because the salesperson whose queue it drains has
// every right to see what went out in their name.
func setupAutopilotRoutes(v1 *gin.RouterGroup, h *handlers.AutopilotHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/autopilot", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("", h.HandleGet)
		g.GET("/sends", h.HandleListSends)
		g.POST("/dismiss-offer", h.HandleDismissOffer)
		g.PUT("", middleware.RequireRole("admin"), h.HandleUpdate)
	}
}

// setupAudienceRoutes exposes target audiences (§2 D3).
//
// Dashboard roles: deciding who to prospect is the job, not an admin setting.
func setupAudienceRoutes(v1 *gin.RouterGroup, h *handlers.AudienceHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/audiences", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("", h.HandleList)
		g.PUT("/:id", h.HandleUpsert)
		g.DELETE("/:id", h.HandleDelete)
		// Costs a model call. Bounded by the per-account limiter the whole
		// authenticated surface already runs under; a tighter one here would
		// need the limiter factory threaded in for a button nobody presses
		// twice in a row.
		g.POST("/suggest", h.HandleSuggest)
	}
}

// setupSequenceRoutes exposes cadences (§1.5 E1).
//
// Dashboard roles: designing the follow-up is the job.
func setupSequenceRoutes(v1 *gin.RouterGroup, h *handlers.SequenceHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("/sequences", h.HandleList)
		g.GET("/sequences/:id", h.HandleGet)
		g.PUT("/sequences/:id", h.HandleUpsert)
		g.DELETE("/sequences/:id", h.HandleDelete)
		g.POST("/sequences/:id/enroll", h.HandleEnroll)
		g.POST("/leads/:id/sequence/pause", h.HandlePause)
	}
}

// setupStrategyRoutes exposes the strategy bank (§4 E2/E3).
//
// Writing is open to attendants on purpose: a strategy is a salesperson's
// method, authorship is visible, and gating it behind an admin is how a
// strategy bank ends up containing only what management wrote.
func setupStrategyRoutes(v1 *gin.RouterGroup, h *handlers.StrategyHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/strategies", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("", h.HandleList)
		g.GET("/:id", h.HandleGet)
		g.PUT("/:id", h.HandleUpsert)
		// Promoting one is a decision about the whole team's method.
		g.PUT("/:id/status", middleware.RequireRole("admin"), h.HandleSetStatus)
	}
}

// setupSaleRoutes exposes sales (§5 F1).
//
// Dashboard roles: recording what they sold is the salesperson's own job, and
// gating it behind an admin is how a portfolio ends up empty.
func setupSaleRoutes(v1 *gin.RouterGroup, h *handlers.SaleHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("/sales", h.HandleList)
		g.POST("/sales", h.HandleCreate)
		g.GET("/contacts/:id/sales", h.HandleListByContact)
		g.POST("/conversations/:id/extract-sale", h.HandleExtract)
		// Deleting a sale moves money in a commission statement, so it is an
		// admin call even though recording one is not.
		g.DELETE("/sales/:id", middleware.RequireRole("admin"), h.HandleDelete)

		// The sales-history import (carteira backfill). Admin-only like the
		// customer import: it writes the account's portfolio in bulk.
		// "/sales/import" is unambiguous against "/sales/:id" because gin
		// matches a static segment ahead of a param.
		if imp := h.Import(); imp != nil {
			g.POST("/sales/import/preview", middleware.RequireRole("admin"), imp.HandlePreview)
			g.POST("/sales/import", middleware.RequireRole("admin"), imp.HandleImport)
		}
	}
}

// setupPortfolioRoutes exposes the customer view of sales (§6.5 F2/F3).
func setupPortfolioRoutes(v1 *gin.RouterGroup, h *handlers.PortfolioHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("", authMw, middleware.RequireRole("admin", "attendant"))
	{
		// Registered BEFORE /portfolio/:id so "reorders" is not swallowed as
		// a contact id.
		g.GET("/portfolio/reorders", h.HandleUpcomingReorders)
		g.GET("/portfolio", h.HandleList)
		g.GET("/portfolio/:id", h.HandleGet)
		g.PUT("/products/:id/cycles", h.HandleSetProductCycles)
	}
}

// setupRelationshipRoutes exposes the relationship routines and cross-sell
// (§6.5 F4/F5).
//
// Dashboard roles, like the portfolio it sits next to: remembering a
// customer's birthday is the salesperson's own job.
func setupRelationshipRoutes(v1 *gin.RouterGroup, h *handlers.RelationshipHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("", authMw, middleware.RequireRole("admin", "attendant"))
	{
		// Registered BEFORE /relationship-dates/:id so "sectors" is not
		// swallowed as an id — the same trap as /portfolio/reorders.
		g.GET("/relationship-dates/sectors", h.HandleSectors)
		g.GET("/relationship-dates", h.HandleList)
		g.POST("/relationship-dates", h.HandleSave)
		g.DELETE("/relationship-dates/:id", h.HandleDelete)

		g.GET("/cross-sell/patterns", h.HandleCrossSellPatterns)
		g.GET("/portfolio/:id/cross-sell", h.HandleCrossSell)
	}
}

// setupCommissionRuleRoutes exposes the client-facing commission rules
// (§6.6 G1).
//
// Admin-only, unlike the portfolio next door: a commission rule is a pay
// policy. Reading a colleague's rate is reading their salary, and the seller's
// own view is a different, narrower screen (G3).
func setupCommissionRuleRoutes(v1 *gin.RouterGroup, h *handlers.CommissionRuleHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/commission-rules", authMw, middleware.RequireRole("admin"))
	{
		// Registered BEFORE /:id so "simulate" is not swallowed as a rule id.
		g.POST("/simulate", h.HandleSimulate)
		g.GET("", h.HandleList)
		g.POST("", h.HandleSave)
		g.GET("/:id", h.HandleGet)
		g.PUT("/:id", h.HandleSave)
		g.DELETE("/:id", h.HandleDelete)
	}
}

// setupCommissionStatementRoutes exposes the monthly close (§6.6 G2/G3).
//
// Two role tiers, deliberately: the statements are everybody's pay in one
// screen and stay admin-only, while /mine and /estimate carry ONE number — the
// caller's own — and are what makes the rule visible to the person it pays.
func setupCommissionStatementRoutes(v1 *gin.RouterGroup, h *handlers.CommissionStatementHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	own := v1.Group("/commissions", authMw, middleware.RequireRole("admin", "attendant"))
	{
		own.GET("/mine", h.HandleMine)
		own.GET("/estimate", h.HandleEstimate)
	}
	g := v1.Group("/commissions", authMw, middleware.RequireRole("admin"))
	{
		g.GET("/statements", h.HandleList)
		// Registered as its own path, not a query flag: recomputing writes, and
		// a write hidden inside a GET is the kind of thing a browser prefetch
		// triggers by accident.
		g.POST("/statements/recompute", h.HandleRecompute)
		g.POST("/statements/adjust", h.HandleAdjust)
		g.POST("/statements/pay", h.HandleMarkPaid)
		g.GET("/statements/export.csv", h.HandleExportCSV)
	}
}

// setupContractRoutes exposes the contract repository (§6.11 G4).
//
// Two tiers. Reading the list and one contract is open to the dashboard roles —
// somebody serving the customer needs the term and the SLA — but the response is
// REDACTED for anyone who is not owner/admin/gestor, in the domain, on the way
// out. Writing, deleting, re-reading the document and downloading the PDF are
// admin-only: a signed URL IS the document, with every clause the redaction just
// removed.
func setupContractRoutes(v1 *gin.RouterGroup, h *handlers.ContractHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	read := v1.Group("/contracts", authMw, middleware.RequireRole("admin", "attendant"))
	{
		read.GET("", h.HandleList)
		read.GET("/:id", h.HandleGet)
	}
	write := v1.Group("/contracts", authMw, middleware.RequireRole("admin"))
	{
		// Registered BEFORE /:id so "upload-url" is not swallowed as an id.
		write.POST("/upload-url", h.HandleUploadURL)
		write.POST("", h.HandleCreate)
		write.PUT("/:id", h.HandleUpdate)
		write.DELETE("/:id", h.HandleDelete)
		write.POST("/:id/reextract", h.HandleReextract)
		write.GET("/:id/download", h.HandleDownload)
	}
}

// setupContractAlertRoutes exposes the clause deadlines and the recovered-revenue
// ledger (§6.11 G5/G6).
//
// Admin-only, all of it: these payloads carry the value, the penalties and what
// the company went back and collected. The redaction that lets an attendant read
// a contract's term does not extend to the money it recovered.
func setupContractAlertRoutes(v1 *gin.RouterGroup, h *handlers.ContractAlertHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("", authMw, middleware.RequireRole("admin"))
	{
		g.GET("/contracts/:id/attention", h.HandleAttention)
		g.POST("/contracts/:id/alerts", h.HandleSchedule)
		g.GET("/contract-alerts", h.HandleDue)
		g.POST("/contract-alerts/:id/complete", h.HandleComplete)
		g.GET("/contract-recoveries", h.HandleRecovered)
		g.POST("/contract-recoveries", h.HandleRecordRecovery)
	}
}

// setupSignatureRoutes exposes electronic signature (§6.11 G7).
//
// Admin-only for the operator surface: sending a document for signature spends
// the customer's own signature quota and can register a Sale, which lands in
// somebody's commission.
//
// The callback is mounted PUBLICLY by the caller (see the webhook group): the
// provider has no way to hold a JWT, so the unguessable token in the path is the
// credential — and the handler trusts nothing in the body, re-reading the truth
// from the provider before writing anything.
func setupSignatureRoutes(v1 *gin.RouterGroup, h *handlers.SignatureHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("", authMw, middleware.RequireRole("admin"))
	{
		g.GET("/signature/settings", h.HandleGetSettings)
		g.PUT("/signature/settings", h.HandleSaveSettings)
		g.GET("/signature/requests", h.HandleList)
		g.POST("/signature/requests", h.HandleSend)
		g.GET("/signature/requests/:id", h.HandleGet)
		g.POST("/signature/requests/:id/refresh", h.HandleRefresh)
		g.GET("/contracts/:id/signatures", h.HandleListForContract)
	}
}

// setupEmailSuppressionRoutes exposes the do-not-send list.
//
// Same roles as the template library: the person who writes the messages is
// the person who needs to see why one did not go out.
func setupEmailSuppressionRoutes(v1 *gin.RouterGroup, h *handlers.EmailSuppressionHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/email-suppressions", authMw, middleware.RequireRole("admin", "attendant"))
	{
		g.GET("", h.HandleList)
		// Releasing is an admin call: it puts an address that a mail server
		// refused, or that reported us as spam, back into circulation.
		g.DELETE("/:address", middleware.RequireRole("admin"), h.HandleRelease)
	}
}

func setupAttachmentRoutes(
	v1 *gin.RouterGroup,
	h *handlers.AttachmentHandler,
	originalH *handlers.EmailOriginalHandler,
	riskH *handlers.EmailRiskHandler,
	authMw gin.HandlerFunc,
) {
	g := v1.Group("", authMw, middleware.RequireRole("admin", "attendant"))
	if h != nil {
		g.GET("/messages/:id/attachments", h.HandleListForMessage)
		g.GET("/attachments/:id/download", h.HandleDownload)
	}
	if riskH != nil {
		g.POST("/messages/:id/legitimate", riskH.HandleMarkLegitimate)
		g.POST("/messages/:id/report-scam", riskH.HandleReportScam)
	}
	if originalH != nil {
		g.POST("/messages/:id/original-token", originalH.HandleMintToken)
		// Deliberately OUTSIDE the authenticated group: a browser tab carries
		// no Authorization header, and the token in the query IS the
		// authorisation — scoped to one message, expiring in two minutes.
		v1.GET("/messages/original", originalH.HandleOriginal)
	}
}

func setupConversationRoutes(v1 *gin.RouterGroup, h *handlers.ConversationHandler, authMw, convMsgRateLimit gin.HandlerFunc) {
	protected := v1.Group("/conversations")
	protected.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		protected.GET("", h.HandleListConversations)
		protected.GET("/waiting-count", h.HandleWaitingCount)
		protected.GET("/unread-count", h.HandleUnreadCount)
		protected.GET("/:id/messages", h.HandleListMessages)
		protected.GET("/:id/media", h.HandleListConversationMedia)
		protected.GET("/:id/contact-stats", h.HandleGetContactStats)
		protected.GET("/:id/summary", h.HandleConversationBrief)
		// US-052: message-creating endpoints are rate limited to 60/min per
		// conversation. The AC names `/conversations/{id}/messages`; in this
		// codebase a message enters a conversation via the reply/send-message
		// routes (there is no POST `/:id/messages`), so the per-conversation
		// cap is applied to those.
		protected.POST("/:id/reply", convMsgRateLimit, h.HandleHumanReply)
		protected.PATCH("/:id/resume-bot", h.HandleResumeBot)
		protected.PATCH("/:id/assign", h.HandleAssignConversation)
		// Human Takeover (Mission 3): explicit "assume" / "release" /
		// "send-message" trio. Kept alongside the legacy
		// reply/resume-bot/assign routes so existing clients don't
		// break while the Flutter dashboard migrates to the new flow.
		protected.POST("/:id/assign", h.HandleAssumeConversation)
		protected.POST("/:id/release", h.HandleReleaseConversation)
		protected.POST("/:id/send-message", convMsgRateLimit, h.HandleSendHumanMessage)
		protected.PATCH("/:id/priority", h.HandleSetPriority)
		protected.PATCH("/:id/resolve", h.HandleResolveConversation)
		protected.PATCH("/:id/reopen", h.HandleReopenConversation)
		protected.POST("/:id/csat", h.HandleRecordCSAT)
		protected.POST("/:id/read", h.HandleMarkAsRead)
		// K1: stop waiting for the customer to finish and answer now.
		protected.POST("/:id/ai-now", h.HandleAnswerNow)
		protected.POST("/bulk", h.HandleBulkAction)
		protected.POST("/:id/suggest-replies", h.HandleSuggestReplies)
		protected.POST("/:id/summary", h.HandleSummarize)
	}
}

func setupMediaRoutes(v1 *gin.RouterGroup, h *handlers.MediaHandler, authMw gin.HandlerFunc) {
	protected := v1.Group("/media")
	protected.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		protected.POST("/upload", h.HandleUpload)
	}
}

func setupAnalyticsRoutes(v1 *gin.RouterGroup, h *handlers.AnalyticsHandler, authMw gin.HandlerFunc) {
	protected := v1.Group("/analytics")
	protected.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		protected.GET("", h.HandleGetAnalytics)
		// Mission 6: operational MVP — admin-only. Wrapped in its own
		// sub-group so we can tighten the role separately from the
		// existing /analytics route (still admin+attendant for now,
		// per mission CSAT-carries-sensitive-data caveat).
		protected.GET("/overview", middleware.RequireRole("admin"), h.HandleGetOverview)
		// US-030: cost-attribution dashboard — admin-only since spend
		// data is commercially sensitive.
		protected.GET("/expensive-conversations", middleware.RequireRole("admin"), h.HandleListExpensiveConversations)
		// Per-human-attendant metrics. Role enforcement lives in the use
		// case (attendants scoped to themselves), so we expose the
		// endpoint to both roles here.
		protected.GET("/attendants", h.HandleListAttendantPerformance)
		protected.GET("/attendants/export.csv", h.HandleExportAttendantPerformanceCSV)
		// Live supervisor snapshot: per-attendant load + unassigned
		// queue, polled every ~10s by the team panel.
		protected.GET("/attendants/live", h.HandleLiveSupervision)
		// SDR analytics (I1) — admin-only: the report carries per-seller
		// revenue and commission, which an attendant must not read about
		// their colleagues.
		sdr := protected.Group("/sdr", middleware.RequireRole("admin"))
		{
			sdr.GET("", h.HandleGetSDRAnalytics)
			// The drill-down behind every number on the report.
			sdr.GET("/leads", h.HandleDrillSDRAnalytics)
		}
	}
}

// originAllowed reports whether origin should pass the CORS gate given the
// configured allow-list. An empty allow-list is the dev-only fallback that
// permits every origin (production boots refuse to start with an empty
// CORS_ALLOWED_ORIGINS — see config.Validate). When the list is non-empty the
// match is exact (case-insensitive scheme/host); any other origin is blocked.
func originAllowed(allowedOrigins []string, origin string) bool {
	if len(allowedOrigins) == 0 {
		return true // dev fallback when no origins configured
	}
	for _, o := range allowedOrigins {
		if strings.EqualFold(origin, o) {
			return true
		}
	}
	return false
}

// buildAllowedOrigins splits a comma-separated string into trimmed, non-empty origins.
func buildAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ── Machine API keys (J1) ────────────────────────────────────────────────────
//
// Package-level rather than parameters on Setup, which already takes seventy
// handlers: adding three more would make every test that builds a router change
// for a feature it does not use. Set by main before Setup runs; unset leaves the
// key routes and the public surface unmounted, which is the correct state for a
// deployment that has not enabled them.
var (
	apiKeyHandler             *handlers.APIKeyHandler
	apiKeyAuthMiddleware      gin.HandlerFunc
	apiKeyRateLimitMiddleware gin.HandlerFunc
)

// SetAPIKeySupport wires the machine-key surface.
func SetAPIKeySupport(h *handlers.APIKeyHandler, auth, rateLimit gin.HandlerFunc) {
	apiKeyHandler = h
	apiKeyAuthMiddleware = auth
	apiKeyRateLimitMiddleware = rateLimit
}
