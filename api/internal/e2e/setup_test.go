package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/auth"
	"github.com/replider/api/internal/adapters/calendar"
	emailchannel "github.com/replider/api/internal/adapters/channels/email"
	"github.com/replider/api/internal/adapters/channels/instagram"
	tgadapter "github.com/replider/api/internal/adapters/channels/telegram"
	twadapter "github.com/replider/api/internal/adapters/channels/twilio"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/adapters/http/router"
	"github.com/replider/api/internal/adapters/payment"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/adapters/signature"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/replider/api/internal/core/usecases"
)

var (
	testPool   *pgxpool.Pool
	testRouter *gin.Engine
)

func TestMain(m *testing.M) {
	// 1. Connect to test DB
	dbURL := "postgres://replider_test:test_secret@localhost:5434/replider_test_db?sslmode=disable"
	if envURL := os.Getenv("TEST_DB_URL"); envURL != "" {
		dbURL = envURL
	}

	ctx := context.Background()
	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 30; i++ {
		pool, err = pgxpool.New(ctx, dbURL)
		if err == nil {
			if pool.Ping(ctx) == nil {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		fmt.Printf("Failed to connect to test DB: %v\n", err)
		os.Exit(1)
	}
	testPool = pool

	// 2. Run migrations (drop + re-apply for clean state)
	migrationsPath := findMigrationsPath()
	dropCmd := exec.Command("migrate", "-path", migrationsPath, "-database", dbURL, "drop", "-f")
	dropCmd.Stdout = os.Stdout
	dropCmd.Stderr = os.Stderr
	_ = dropCmd.Run() // ignore error on first run (no tables yet)

	upCmd := exec.Command("migrate", "-path", migrationsPath, "-database", dbURL, "up")
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	if err := upCmd.Run(); err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		os.Exit(1)
	}

	// 3. Setup router
	testRouter = setupFullRouter(pool)

	// 4. Run tests
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

// findMigrationsPath returns the path to db/migrations relative to the test binary.
func findMigrationsPath() string {
	// When running from api/ directory: ../../db/migrations
	candidates := []string{
		"../../db/migrations",
		"db/migrations",
		"../db/migrations",
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return "../../db/migrations"
}

func setupFullRouter(pool *pgxpool.Pool) *gin.Engine {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Environment:             "test",
		Port:                    "8080",
		JWTSecret:               "e2e-test-jwt-secret",
		AIProvider:              "gemini",
		GeminiAPIKey:            "",
		OpenAIAPIKey:            "",
		StripeSecretKey:         "",
		StripeWebhookSecret:     "",
		GCSBucketName:           "",
		GoogleCredentialsBase64: "",
		SMTPHost:                "",
		SMTPPort:                "",
		SMTPUsername:            "",
		SMTPPassword:            "",
		SMTPFrom:                "noreply@test.com",
		AppBaseURL:              "http://localhost:8080",
		WhatsAppAppID:           "",
		// A real secret so e2e can drive the signed WhatsApp webhook
		// (empty would 500 every POST with "secret not configured").
		WhatsAppAppSecret:   "e2e-wa-secret",
		WhatsAppVerifyToken: "test-verify-token",
		GoogleClientID:      "",
		CORSAllowedOrigins:  "",
		// Email sending config so the DNS setup screen produces SPF/DKIM
		// records — the journey test publishes them into the fake resolver
		// and lets the REAL verifier check them.
		EmailSPFInclude:    "spf.replider.com",
		EmailDKIMSelector:  "rp1",
		EmailDKIMTarget:    "dkim.replider.com",
		EmailDMARCReportTo: "dmarc@replider.com",
		// The owner panel is part of the journey: sales-led SDR tiers have no
		// Stripe checkout, so apply-contract is the one public door onto them.
		SuperAdminEmailsRaw: "owner@replider.com",
	}

	// 1. Transaction Manager
	txManager := postgres.NewPostgresTransactionManager(pool)

	// 2. Repositories
	accountRepo := postgres.NewAccountRepository(pool, nil)
	userRepo := postgres.NewUserRepository(pool, nil)
	usageRepo := postgres.NewUsageRepository(pool)
	agentRepo := postgres.NewAgentRepository(pool)
	planRepo := postgres.NewPlanRepository(pool)
	auditLogRepo := postgres.NewAuditLogRepository(pool)
	convRepo := postgres.NewConversationRepository(pool)
	// Channel credentials are encrypted at rest — e2e wires a fixed
	// test key so channel writes exercise the real seal/open path.
	testChannelCipher, err := secrets.NewAESGCMCipher(strings.Repeat("ab", 32))
	if err != nil {
		panic(err)
	}
	channelRepo := postgres.NewChannelRepository(pool, testChannelCipher)
	msgRepo := postgres.NewMessageRepository(pool)
	knowledgeRepo := postgres.NewKnowledgeRepository(pool)
	calendarRepo := postgres.NewCalendarRepository(pool)

	// 3. Services / Adapters
	jwtService := auth.NewJWTService(cfg.JWTSecret, 24)
	aiFactory := services.NewAIFactory(cfg)
	stripeAdpt := payment.NewStripeAdapter(cfg.StripeSecretKey, cfg.StripeWebhookSecret)
	gcsAdapter := storage.NewGCSAdapter(cfg.GCSBucketName, cfg.GoogleCredentialsBase64)
	waAdapter := waadapter.NewWhatsAppAdapter("")
	tgAdapter := tgadapter.NewTelegramAdapter(nil)
	emailService := email.NewSMTPAdapter(
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUsername,
		cfg.SMTPPassword,
		cfg.SMTPFrom,
	)
	calendarAdapter := calendar.NewAccountCalendarAdapter(calendar.NewGoogleCalendarAdapter(cfg.GoogleCredentialsBase64), nil, nil)

	// WebSocket Hub
	wsHub := handlers.NewHub()
	go wsHub.Run()

	// 4. Use Cases
	funnelRepo := postgres.NewSalesFunnelRepository(pool)
	createAccountUC := usecases.NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailService, cfg.AppBaseURL)
	createAccountUC.SetFunnelRepo(funnelRepo)
	// Sales-led signup invites: the journey's customer account is born from
	// an owner-issued invite, exactly like production with the door closed.
	accountInviteRepo := postgres.NewAccountInviteRepository(pool)
	createAccountUC.SetInviteRepo(accountInviteRepo)
	inviteUC := usecases.NewInviteUseCase(accountInviteRepo, planRepo)
	verifyEmailUC := usecases.NewVerifyEmailUseCase(userRepo, emailService, cfg.AppBaseURL)
	deleteAccountUC := usecases.NewDeleteAccountUseCase(accountRepo, userRepo)
	authenticateUC := usecases.NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
	forgotPasswordUC := usecases.NewForgotPasswordUseCase(userRepo, emailService, cfg.AppBaseURL)
	resetPasswordUC := usecases.NewResetPasswordUseCase(userRepo)
	googleAuthUC := usecases.NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, cfg.GoogleClientID)
	googleAuthUC.SetFunnelRepo(funnelRepo)
	metaAuthUC := usecases.NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
	metaAuthUC.SetFunnelRepo(funnelRepo)

	createAgentUC := usecases.NewCreateAgentUseCase(agentRepo, accountRepo, planRepo)
	listAgentsUC := usecases.NewListAgentsUseCase(agentRepo)
	updateAgentUC := usecases.NewUpdateAgentUseCase(agentRepo)
	toggleAgentUC := usecases.NewToggleAgentStatusUseCase(agentRepo)
	deleteAgentUC := usecases.NewDeleteAgentUseCase(agentRepo)

	createCalendarUC := usecases.NewCreateCalendarUseCase(agentRepo, calendarRepo)
	updateCalendarUC := usecases.NewUpdateCalendarUseCase(agentRepo, calendarRepo)
	deleteCalendarUC := usecases.NewDeleteCalendarUseCase(agentRepo, calendarRepo)
	listCalendarsUC := usecases.NewListCalendarsUseCase(agentRepo, calendarRepo)

	// For knowledge UC we need an AIProvider; pass nil-safe by using a dummy.
	// The AI factory returns an error with empty keys, so we pass nil for the provider.
	// Knowledge tests are not part of the e2e scope (they require embedding calls).
	provider, _ := aiFactory.GetProvider(context.Background(), cfg.AIProvider)
	knowledgeUC := usecases.NewKnowledgeUseCase(knowledgeRepo, agentRepo, provider)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)

	previewChatUC := usecases.NewPreviewChatUseCase(accountRepo, agentRepo, usageRepo, planRepo, auditLogRepo, aiFactory, agentRunner)
	getOnboardingProgressUC := usecases.NewGetOnboardingProgressUseCase(accountRepo)
	dismissOnboardingChecklistUC := usecases.NewDismissOnboardingChecklistUseCase(accountRepo)
	updateOnboardingDraftUC := usecases.NewUpdateOnboardingDraftUseCase(accountRepo)
	getMeUC := usecases.NewGetMeUseCase(userRepo, accountRepo, planRepo, getOnboardingProgressUC)
	updateProfileUC := usecases.NewUpdateProfileUseCase(userRepo)
	changePasswordUC := usecases.NewChangePasswordUseCase(userRepo)
	getUsageUC := usecases.NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
	usageGuard := services.NewUsageGuard(usageRepo, planRepo, accountRepo)
	processMessageUC := usecases.NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, msgRepo, calendarRepo, userRepo, aiFactory, agentRunner, calendarAdapter, emailService, cfg.AppBaseURL, usageGuard)
	checkoutUC := usecases.NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, stripeAdpt)
	purchaseAddonUC := usecases.NewPurchaseAddonUseCase(accountRepo, stripeAdpt)
	stripeEventRepo := postgres.NewStripeEventRepository(pool)
	stripeWebhookUC := usecases.NewProcessStripeWebhookUseCase(accountRepo, usageRepo, stripeEventRepo)
	inboundUC := usecases.NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, processMessageUC, agentRepo)
	// US-F007/M005: the harness must mirror production wiring, or the
	// identity resolver silently does nothing and the email inbound tests
	// would pass on a pipeline that never registers a contact.
	inboundUC.SetContactRepo(postgres.NewContactRepository(pool))
	inboundUC.SetIdentityRepo(postgres.NewContactIdentityRepository(pool))

	createChannelUC := usecases.NewCreateChannelUseCase(channelRepo, agentRepo, accountRepo, planRepo)
	// Mirrors prod: the first email channel binds the default funnel.
	createChannelUC.SetFunnelLinker(usecases.NewFunnelChannelLinker(funnelRepo, channelRepo))
	listChannelsUC := usecases.NewListChannelsUseCase(channelRepo)
	updateChannelUC := usecases.NewUpdateChannelUseCase(channelRepo, agentRepo)
	deleteChannelUC := usecases.NewDeleteChannelUseCase(channelRepo)
	toggleChannelUC := usecases.NewToggleChannelUseCase(channelRepo)

	// 5. Handlers
	updateAccountUC := usecases.NewUpdateAccountUseCase(accountRepo, userRepo)
	accountHandler := handlers.NewAccountHandler(createAccountUC, deleteAccountUC, updateAccountUC)
	accountHandler.SetInviteUC(inviteUC)
	resendVerificationUC := usecases.NewResendVerificationUseCase(userRepo, emailService, cfg.AppBaseURL)
	twoFactorUC := usecases.NewTwoFactorUseCase(userRepo, "Replider")
	verifyMfaLoginUC := usecases.NewVerifyMfaLoginUseCase(userRepo, accountRepo, jwtService, twoFactorUC)
	twoFactorHandler := handlers.NewTwoFactorHandler(twoFactorUC)
	authHandler := handlers.NewAuthHandler(
		authenticateUC,
		verifyMfaLoginUC,
		forgotPasswordUC,
		resetPasswordUC,
		googleAuthUC,
		metaAuthUC,
		verifyEmailUC,
		resendVerificationUC,
	)
	learnPersonalityUC := usecases.NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	agentHandler := handlers.NewAgentHandler(
		previewChatUC,
		createAgentUC,
		listAgentsUC,
		updateAgentUC,
		toggleAgentUC,
		deleteAgentUC,
		learnPersonalityUC,
	)
	importHandler := handlers.NewImportHandler(convRepo, msgRepo, channelRepo, agentRepo)
	calendarHandler := handlers.NewCalendarHandler(createCalendarUC, updateCalendarUC, deleteCalendarUC, listCalendarsUC, calendarRepo, nil)
	knowledgeHandler := handlers.NewKnowledgeHandler(knowledgeUC)
	meHandler := handlers.NewMeHandler(getMeUC, updateProfileUC, changePasswordUC, getOnboardingProgressUC, dismissOnboardingChecklistUC, updateOnboardingDraftUC, nil)
	// BYOK cost surface: report + customer ceiling.
	e2eAICostReportUC := usecases.NewAICostReportUseCase(accountRepo,
		postgres.NewAICostRepository(pool, usecases.DefaultUSDBRLRateCents))
	e2eAICostReportUC.SetBudgetWriter(accountRepo)
	meHandler.SetAICostsUC(e2eAICostReportUC)
	usageHandler := handlers.NewUsageHandler(getUsageUC)
	wsHandler := handlers.NewWSHandler(wsHub, processMessageUC, jwtService, cfg.CORSAllowedOrigins, cfg.Environment)
	billingPortalUC := usecases.NewCreateBillingPortalSessionUseCase(accountRepo, stripeAdpt)
	paymentHandler := handlers.NewPaymentHandler(checkoutUC, stripeWebhookUC, purchaseAddonUC, billingPortalUC, stripeAdpt, cfg.AppBaseURL)
	plansHandler := handlers.NewPlansHandler(planRepo, accountRepo)
	igAdapter := instagram.NewInstagramAdapter()
	smsAdapter := twadapter.NewTwilioAdapter()
	noteRepo := postgres.NewNoteRepository(pool)
	conversationHandler := handlers.NewConversationHandler(convRepo, msgRepo, channelRepo, noteRepo, map[string]domain.MessageProvider{
		"whatsapp":  waAdapter,
		"telegram":  tgAdapter,
		"instagram": igAdapter,
	}, wsHub, gcsAdapter)
	// The media proxy is what the dashboard's URLs point at, so the e2e wires
	// the signer too — otherwise the transcript would hand back a GCS signed
	// URL and the shape under test would never be exercised.
	conversationHandler.SetMediaSigner(e2eMediaSigner())
	conversationHandler.SetWaitingCounter(convRepo)
	// Human takeover (assume/release/send-message), as main.go wires it —
	// the journey pauses a thread through it before exercising auto-resume.
	conversationHandler.SetTakeoverUseCase(usecases.NewConversationTakeoverUseCase(
		convRepo, msgRepo, channelRepo, userRepo,
		map[string]domain.MessageProvider{
			"whatsapp":  waAdapter,
			"telegram":  tgAdapter,
			"instagram": igAdapter,
		},
		wsHub,
	))
	mediaHandler := handlers.NewMediaHandler(gcsAdapter)
	inboundHandler := handlers.NewInboundWebhookHandler(inboundUC, wsHub, channelRepo, waAdapter, tgAdapter, igAdapter, smsAdapter)
	waWebhookHandler := handlers.NewWhatsAppWebhookHandler(inboundUC, wsHub, channelRepo, nil, waAdapter, cfg.WhatsAppAppSecret, cfg.WhatsAppVerifyToken)
	channelHandler := handlers.NewChannelHandler(createChannelUC, listChannelsUC, updateChannelUC, deleteChannelUC, toggleChannelUC, channelRepo, cfg.AppBaseURL, tgAdapter)
	// Email DNS setup + verification against the harness resolver (the
	// "registrar"), stamping the same mailbox rows the send guards read.
	emailMailboxRepo := postgres.NewEmailMailboxRepository(pool)
	channelHandler.SetEmailDNSVerifier(services.NewEmailDNSVerifier(
		e2eDNSResolver,
		domain.EmailSendingConfig{
			SPFInclude:    cfg.EmailSPFInclude,
			DKIMSelector:  cfg.EmailDKIMSelector,
			DKIMTarget:    cfg.EmailDKIMTarget,
			DMARCReportTo: cfg.EmailDMARCReportTo,
		},
	))
	channelHandler.SetEmailMailboxRepo(emailMailboxRepo)

	// Analytics, Templates, Team
	analyticsRepo := postgres.NewAnalyticsRepository(pool)
	getAnalyticsUC := usecases.NewGetAnalyticsUseCase(analyticsRepo, usageRepo, planRepo, accountRepo)
	attendantPerfUC := usecases.NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	liveSupervisionUC := usecases.NewGetLiveSupervisionUseCase(analyticsRepo, userRepo)
	analyticsHandler := handlers.NewAnalyticsHandler(getAnalyticsUC, attendantPerfUC, liveSupervisionUC, nil, nil)
	// SDR analytics (I1). Wired with nil settings: the USD→BRL rate falls back
	// to the platform default, which is what the cost assertions expect.
	analyticsHandler.SetSDRAnalyticsUC(usecases.NewSDRAnalyticsUseCase(
		postgres.NewSDRAnalyticsRepository(pool),
		postgres.NewSalesEventRepository(pool),
		accountRepo,
	))
	templatesHandler := handlers.NewTemplatesHandler()

	listMembersUC := usecases.NewListTeamMembersUseCase(userRepo)
	inviteMemberUC := usecases.NewInviteTeamMemberUseCase(userRepo, accountRepo, planRepo, emailService, cfg.AppBaseURL)
	updateRoleUC := usecases.NewUpdateMemberRoleUseCase(userRepo)
	removeMemberUC := usecases.NewRemoveMemberUseCase(userRepo)
	resendInviteUC := usecases.NewResendInviteUseCase(userRepo, emailService, cfg.AppBaseURL)
	teamHandler := handlers.NewTeamHandler(listMembersUC, inviteMemberUC, updateRoleUC, removeMemberUC, resendInviteUC)

	// Canned responses & tags
	cannedResponseRepo := postgres.NewCannedResponseRepository(pool)
	tagRepo := postgres.NewTagRepository(pool)
	cannedResponseUC := usecases.NewCannedResponseUseCase(cannedResponseRepo)
	tagUC := usecases.NewTagUseCase(tagRepo, convRepo)
	cannedResponseHandler := handlers.NewCannedResponseHandler(cannedResponseUC)
	tagHandler := handlers.NewTagHandler(tagUC)

	// Contacts
	contactRepo := postgres.NewContactRepository(pool)
	listContactsUC := usecases.NewListContactsUseCase(contactRepo)
	getContactUC := usecases.NewGetContactUseCase(contactRepo)
	updateContactUC := usecases.NewUpdateContactUseCase(contactRepo)
	updateContactUC.SetIdentityRepo(postgres.NewContactIdentityRepository(pool))
	contactHandler := handlers.NewContactHandler(listContactsUC, getContactUC, updateContactUC, contactRepo)

	// Outbound sales (the SDR module). Wired so the funnel/lead HTTP
	// surface — the core of the pivot — is exercised end to end rather
	// than only through its use cases. Approaches need an AI provider and
	// a messaging channel, so the approach use case is built but its
	// sender is left nil: e2e drives the pipeline, never a real send.
	leadRepo := postgres.NewLeadRepository(pool)

	// Universal export (I3).
	exportHandler := handlers.NewExportHandler(convRepo, msgRepo)
	exportHandler.SetUniversalExportUC(usecases.NewExportUseCase(
		postgres.NewExportRepository(pool), userRepo))
	// Conversational onboarding (K5). No extractor and no audience suggester
	// wired: the e2e proves the ladder finishes with NO AI at all, which is the
	// property that keeps a provider outage from costing a signup.
	onboardingChatUC := usecases.NewOnboardingChatUseCase(
		postgres.NewOnboardingChatRepository(pool), accountRepo)
	onboardingChatUC.SetProducts(postgres.NewProductRepository(pool))
	meHandler.SetOnboardingChatUC(onboardingChatUC)

	// The home command centre (I2).
	meHandler.SetCommandCenterUC(usecases.NewCommandCenterUseCase(
		postgres.NewCommandCenterRepository(pool), leadRepo, accountRepo))
	// The opportunity radar + territory hunt + park offer (Ofertas phase 2),
	// at the same seams main.go wires.
	cnesParkRepo := postgres.NewCNESParkRepository(pool)
	meHandler.SetRadarUC(usecases.NewRadarUseCase(
		postgres.NewSignalRepository(pool), cnesParkRepo,
		postgres.NewPortfolioRepository(pool),
		postgres.NewRelationshipRepository(pool),
		postgres.NewCrossSellRepository(pool),
		postgres.NewProductRepository(pool), accountRepo))
	territoryHuntUC := usecases.NewTerritoryHuntUseCase(
		cnesParkRepo, accountRepo, contactRepo)
	territoryHuntUC.SetIdentityRepo(postgres.NewContactIdentityRepository(pool))
	meHandler.SetTerritoryHuntUC(territoryHuntUC)
	productRepo := postgres.NewProductRepository(pool)
	funnelUC := usecases.NewFunnelUseCase(funnelRepo)
	leadUC := usecases.NewLeadUseCase(leadRepo, funnelRepo)
	leadUC.SetContactRepo(contactRepo)
	leadUC.SetStageRepo(postgres.NewFunnelStageRepository(pool))
	// Owner assignment REFUSES when this is unwired (ErrLeadAssignUnavailable):
	// it is the only check that keeps a lead's owner inside its own account, and
	// assigning unchecked would be worse than not assigning. main.go has wired it
	// since the guard landed; this harness had not, so every ownership test was
	// exercising the refusal instead of the rule it meant to pin.
	leadUC.SetUserRepo(userRepo)
	// The journey test (journey_first_customer_test.go) needs the draft
	// pipeline to actually produce text, so the approach use case gets the
	// factory WRAPPED: marker accounts get a deterministic fake provider,
	// everyone else keeps the key-less refusal other tests pin.
	approachUC := usecases.NewApproachUseCase(
		leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo,
		planRepo, productRepo, usageRepo, newE2EAIFactory(aiFactory), nil,
	)
	radarParkOfferUC := usecases.NewRadarParkOfferUseCase(
		postgres.NewCNESParkRepository(pool), contactRepo, leadRepo,
		funnelRepo, approachUC)
	radarParkOfferUC.SetIdentityRepo(postgres.NewContactIdentityRepository(pool))
	meHandler.SetRadarParkOfferUC(radarParkOfferUC)
	// The email leg of approve-send, at the same seams main.go wires: the
	// outbox fake stands where the SMTP adapter stands, and the mailbox repo
	// is the real one so the deliverability guards run for real.
	approachUC.SetEmailSender(e2eEmailOutbox)
	approachUC.SetMailboxRepo(emailMailboxRepo)
	approachUC.SetOutboundRecorder(convRepo, msgRepo)
	// A/B of approach writing (US-F216) — wired so the e2e can drive it.
	approachExperimentRepo := postgres.NewApproachExperimentRepository(pool)
	approachUC.SetExperimentRepo(approachExperimentRepo)
	leadUC.SetExperimentRepo(approachExperimentRepo)
	approachExperimentHandler := handlers.NewApproachExperimentHandler(
		usecases.NewApproachExperimentUseCase(approachExperimentRepo, funnelRepo))

	brandKitHandler := handlers.NewBrandKitHandler(postgres.NewBrandKitRepository(pool))
	// Source health rides the SAME repo on both ends, exactly like main.go:
	// the poll writes it and the radar reads it, and a mismatch would leave
	// the screen dating the feed by a stamp nobody sets.
	signalRepoForE2E := postgres.NewSignalRepository(pool)
	signalHandler := handlers.NewSignalHandler(signalRepoForE2E)
	signalHandler.SetSourceHealth(signalRepoForE2E)
	brandKitHandler.SetObjectionRepo(postgres.NewObjectionRepository(pool))
	funnelHandler := handlers.NewFunnelHandler(nil, funnelUC)
	leadHandler := handlers.NewLeadHandler(leadUC, approachUC)
	// "Open the conversation" from the lead panel — the same finder main.go
	// wires. Without it the journey's last hop (lead → thread) is a dead end.
	leadHandler.SetConversationFinder(convRepo)
	// Enrichment behind the same BYOK gate production has: marker accounts
	// get the deterministic fake, everyone else the configure-your-key error.
	leadHandler.SetEnrichUseCase(usecases.NewEnrichLeadUseCase(
		leadRepo, contactRepo, accountRepo, planRepo, &e2eEnrichmentFactory{}))
	// The onboarding's first offer, without the Places fallback (no key in
	// e2e): a territory-only account gets "nothing to offer", honestly.
	meHandler.SetFirstOfferUC(usecases.NewFirstOfferUseCase(
		accountRepo, funnelRepo, contactRepo, leadRepo, leadUC, approachUC))
	// The lead's own conversation history, authorised by the lead's rules.
	leadHandler.SetTimelineUseCase(usecases.NewLeadTimelineUseCase(
		leadUC, postgres.NewLeadTimelineRepository(pool)))
	// Answering from the lead, through the same human-send path as the
	// inbox. A second takeover instance on purpose: the one above is built
	// inline for the conversation handler, and the reply only needs the
	// repos and the hub (no provider for e-mail here — the send is skipped
	// with a warning, and the message still lands in the timeline).
	leadHandler.SetReplyUC(usecases.NewLeadReplyUseCase(leadUC, convRepo,
		usecases.NewConversationTakeoverUseCase(
			convRepo, msgRepo, channelRepo, userRepo,
			map[string]domain.MessageProvider{}, wsHub)))
	// The queue's two honesty reads, wired exactly as main.go wires them:
	// "can this account send at all" and "can it reach THIS recipient". An
	// unwired reader answers the permissive way, so leaving them out here
	// would make the e2e pass over a queue that says nothing.
	leadHandler.SetSendChannels(channelRepo)
	leadHandler.SetQueueContacts(contactRepo)

	// 6. Router Setup
	r := gin.New()
	r.Use(gin.Recovery())
	// Newsletter
	newsletterRepo := postgres.NewNewsletterRepository(pool)
	subscribeNewsletterUC := usecases.NewSubscribeNewsletterUseCase(newsletterRepo)
	newsletterHandler := handlers.NewNewsletterHandler(subscribeNewsletterUC)

	// Sales lead capture (US-157) — notifier nil + empty recipient: e2e setup
	// doesn't exercise the qualify endpoint, so no email routing needed.
	salesLeadRepo := postgres.NewSalesLeadRepository(pool)
	submitSalesLeadUC := usecases.NewSubmitSalesLeadUseCase(salesLeadRepo, nil, "")
	salesHandler := handlers.NewSalesHandler(submitSalesLeadUC)

	// ROI-calculator lead magnet (US-163).
	roiLeadRepo := postgres.NewROILeadRepository(pool)
	submitROILeadUC := usecases.NewSubmitROILeadUseCase(roiLeadRepo)
	roiHandler := handlers.NewROIHandler(submitROILeadUC)

	// Inbound attachments. The store is a fake so the suite needs no GCS
	// credentials; the scanner is left nil, which is the honest default —
	// files land as "skipped" and are not downloadable.
	attachmentRepo := postgres.NewMessageAttachmentRepository(pool)
	attachmentUC := usecases.NewMessageAttachmentUseCase(attachmentRepo, testAttachmentStore)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentUC, attachmentRepo)
	emailOriginalHandler := handlers.NewEmailOriginalHandler(attachmentUC, msgRepo, cfg.JWTSecret)
	// Inbound mail is signature-gated and fails CLOSED (15/09/2026). The suite
	// signs through postInboundEmail; without a secret here every email test
	// would get a 403 that looks like a product bug.
	inboundHandler.SetEmailWebhookSecret(e2eEmailWebhookSecret)
	inboundHandler.SetAttachmentUseCase(attachmentUC)

	// Impersonation checks. No domain-age lookup here: the suite must not
	// reach an external registry, and the domain layer already treats an
	// absent registration date as "no answer" rather than as a pass.
	emailRiskUC := usecases.NewEmailRiskUseCase(postgres.NewEmailRiskRepository(pool))
	emailRiskUC.SetPaymentMemory(postgres.NewContactPaymentRepository(pool))
	emailRiskUC.SetAllowlist(postgres.NewSenderAllowlistRepository(pool))
	emailRiskUC.SetScamReports(postgres.NewScamReportRepository(pool))
	inboundHandler.SetEmailRiskUseCase(emailRiskUC)

	// Delivery failures: without this a bounce is recognised but the address
	// is never suppressed, and the e2e would pass while production kept
	// writing to dead mailboxes.
	emailBounceUC := usecases.NewEmailBounceUseCase(
		postgres.NewEmailSuppressionRepository(pool), msgRepo,
		postgres.NewBounceLookupRepository(pool))
	emailBounceUC.SetTemplateStats(postgres.NewEmailTemplateRepository(pool))
	inboundHandler.SetBounceUseCase(emailBounceUC)

	// Inbox triage: without it every conversation stays uncategorised and
	// the filter/sort tests would pass against an empty column.
	inboxTriageUC := usecases.NewInboxTriageUseCase(convRepo)
	inboxTriageUC.SetSalesLead(postgres.NewSalesFunnelRepository(pool), postgres.NewLeadRepository(pool))
	inboundHandler.SetInboxTriageUseCase(inboxTriageUC)
	// Without this the whole lead side of the inbound pipeline is mute in
	// the harness — markLeadResponded, the cadence stop and the case report
	// all hang off it.
	inboundUC.SetSalesLead(postgres.NewSalesFunnelRepository(pool), postgres.NewLeadRepository(pool))
	inboundUC.SetSequenceStopper(
		usecases.NewSequenceUseCase(postgres.NewSequenceRepository(pool)))
	emailRiskHandler := handlers.NewEmailRiskHandler(emailRiskUC, msgRepo)
	conversationHandler.SetEmailRiskRepo(postgres.NewEmailRiskRepository(pool))
	// No AI provider in the suite, so generation always fails — which is
	// exactly what makes the short-thread guard and the tenant scoping
	// testable: both must answer before a model is ever reached.
	conversationHandler.SetBriefUseCase(usecases.NewConversationBriefUseCase(
		postgres.NewConversationBriefRepository(pool),
		msgRepo, convRepo, accountRepo, planRepo, aiFactory))

	agentTemplatesHandler := handlers.NewAgentTemplatesHandler()

	// OAuth 2.1 AS + PAT layer (remote MCP integration) — real repos so
	// the e2e suite can drive the full DCR → authorize → token flow.
	oauthClientRepo := postgres.NewOAuthClientRepository(pool)
	oauthCodeRepo := postgres.NewOAuthCodeRepository(pool)
	apiTokenRepo := postgres.NewAPITokenRepository(pool)

	// Machine API keys and the public surface they authenticate (J1).
	apiKeyRepo := postgres.NewAPIKeyRepository(pool)
	router.SetAPIKeySupport(
		handlers.NewAPIKeyHandler(usecases.NewAPIKeyUseCase(apiKeyRepo, apiTokenRepo)),
		middleware.APIKeyAuth(apiKeyRepo),
		// No plan resolver in e2e: the flat default keeps the rate limit out of
		// the way of tests that are about scopes, not about throughput.
		middleware.APIKeyRateLimiter(
			middleware.RateLimitConfig{RequestsPerMinute: usecases.MaxAPIRequestsPerMinute}, nil),
	)

	oauthRegisterUC := usecases.NewOAuthRegisterClientUseCase(oauthClientRepo)
	oauthAuthorizeUC := usecases.NewOAuthAuthorizeUseCase(oauthClientRepo, oauthCodeRepo, userRepo, jwtService, twoFactorUC, cfg.JWTSecret)
	oauthTokenUC := usecases.NewOAuthTokenUseCase(oauthClientRepo, oauthCodeRepo, apiTokenRepo)
	oauthRevokeUC := usecases.NewOAuthRevokeUseCase(apiTokenRepo)
	mcpPatUC := usecases.NewMCPPatUseCase(apiTokenRepo)
	oauthMetadataHandler := handlers.NewOAuthMetadataHandler(cfg.AppBaseURL)
	oauthAuthorizeHandler := handlers.NewOAuthAuthorizeHandler(oauthAuthorizeUC)
	oauthTokenHandler := handlers.NewOAuthTokenHandler(oauthTokenUC, oauthRevokeUC, oauthRegisterUC)
	mcpTokenHandler := handlers.NewMCPTokenHandler(mcpPatUC)

	router.Setup(
		r,
		jwtService,
		accountHandler,
		authHandler,
		agentHandler,
		agentTemplatesHandler,
		newsletterHandler,
		salesHandler,
		roiHandler,
		attachmentHandler,
		emailOriginalHandler,
		emailRiskHandler,
		brandKitHandler,
		handlers.NewEmailTemplateHandler(postgres.NewEmailTemplateRepository(pool)),
		handlers.NewEmailTrackingHandler(
			postgres.NewEmailTemplateRepository(pool), "e2e-test-jwt-secret"),
		handlers.NewEmailSuppressionHandler(postgres.NewEmailSuppressionRepository(pool)),
		signalHandler,
		handlers.NewSignalIngestHandler(postgres.NewSignalRepository(pool), "e2e-signal-secret"),
		handlers.NewAutopilotHandler(postgres.NewAutopilotRepository(pool)),
		handlers.NewAudienceHandler(
			usecases.NewAudienceUseCase(postgres.NewAudienceRepository(pool))),
		handlers.NewSequenceHandler(
			usecases.NewSequenceUseCase(postgres.NewSequenceRepository(pool)),
			postgres.NewSequenceRepository(pool)),
		handlers.NewStrategyHandler(postgres.NewStrategyRepository(pool)),
		newE2ESaleHandler(pool, contactRepo, productRepo, accountRepo, leadRepo),
		newE2EPortfolioHandler(pool),
		newE2EDiscoveredContactHandler(pool, leadUC),
		newE2EProposalHandler(pool),
		newE2EProductImportHandler(pool, accountRepo, planRepo, aiFactory),
		handlers.NewCustomerImportHandler(newE2ECustomerImportUseCase(pool, contactRepo, leadUC, funnelRepo)),
		// nil: e2e has no Google client, which is the same shape production
		// takes when one is not configured — the routes simply are not mounted.
		nil,
		newE2ERelationshipHandler(pool),
		handlers.NewCommissionRuleHandler(usecases.NewCommissionRuleUseCase(
			postgres.NewCommissionRuleRepository(pool))),
		newE2ECommissionStatementHandler(pool, accountRepo, userRepo),
		newE2EContractHandler(pool, auditLogRepo),
		newE2EContractAlertHandler(pool),
		newE2ESignatureHandler(pool, testChannelCipher),
		calendarHandler,
		knowledgeHandler,
		nil, // ingestionHandler — not exercised by e2e setup yet
		usageHandler,
		meHandler,
		wsHandler,
		paymentHandler,
		plansHandler,
		conversationHandler,
		mediaHandler,
		inboundHandler,
		waWebhookHandler,
		channelHandler,
		nil, // waSignupHandler (no WhatsApp in e2e)
		nil, // waConfigHandler (no WhatsApp in e2e)
		nil, // webChatHandler (no webchat in e2e)
		nil, // campaignHandler (no campaigns in e2e)
		nil, // productHandler (no products in e2e)
		funnelHandler,
		leadHandler,
		nil, // caseMetricsHandler (US-M011 aggregation is tested at the repo)
		nil, // copilotHandler (US-M026 is tested at the use case)
		approachExperimentHandler,
		nil, // lgpdHandler (no LGPD endpoints in e2e)
		nil, // auditLogHandler (no audit listing in e2e)
		analyticsHandler,
		templatesHandler,
		nil, // transcribeHandler — not exercised in e2e suite
		teamHandler,
		cannedResponseHandler,
		tagHandler,
		contactHandler,
		exportHandler,
		importHandler,
		nil, // logoutHandler (no Redis in e2e)
		nil, // adminHandler (SMTP test not exercised in e2e)
		nil, // accountWebhookHandler (outbound webhooks not exercised in e2e)
		nil, // pushHandler
		nil, // tgFileHandler (bot-token proxy not exercised in e2e)
		newE2EMediaFileHandler(),
		nil, // scheduledHandler (outbound scheduler not exercised in e2e)
		nil, // promptHandler (prompt builder not exercised in e2e)
		nil, // experimentHandler (A/B testing not exercised in e2e)
		// Owner panel, wired as main.go wires it (accountRepo doubles as the
		// OwnerAccountRepository): the journey's customer only reaches an SDR
		// contract plan through PATCH /owner/accounts/:id/apply-contract —
		// trial deliberately cannot send a cold approach (broadcast cap 0).
		newE2EOwnerHandler(accountRepo, userRepo, planRepo, usageRepo, auditLogRepo, jwtService, inviteUC),
		nil, // sellerHandler (seller panel not exercised in e2e)
		twoFactorHandler,
		nil, // couponHandler (coupons not exercised in e2e)
		oauthMetadataHandler,
		oauthAuthorizeHandler,
		oauthTokenHandler,
		mcpTokenHandler,
		nil, // gcalIntegrationHandler (Google OAuth not configured in e2e)
		nil, // tokenBlacklist
		accountRepo,
		userRepo,
		nil, // sellerFlagChecker (seller panel not exercised in e2e)
		pool,
		cfg,
	)

	return r
}

// cleanDB truncates all tables except plans (seeded data).
func cleanDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"messages",
		"conversations",
		"channels",
		"knowledge_chunks",
		"knowledge_sources",
		"calendars",
		"audit_logs",
		"agents",
		"account_usages",
		// commission_ledger has NO FK to accounts/users since migration
		// 000136 (rows are an immutable snapshot that survives account
		// deletion) — truncating accounts no longer cascades into it, so
		// it must be truncated explicitly or rows leak across tests.
		"commission_ledger",
		// The CNES tables are SHARED public data (migration 000202): no
		// account_id, so truncating accounts never reaches them, and a park
		// seeded by one test would answer another's lookup. The TYPES table
		// is deliberately NOT truncated — the migration seeds it once and
		// the join tests depend on the real dictionary.
		"cnes_equipment",
		"cnes_establishments",
		"cnes_ingest_runs",
		// cnes_beds is truncated; cnes_bed_types deliberately is NOT — the
		// join tests hit the real migration seed, exactly like
		// cnes_equipment_types.
		"cnes_beds",
		// The dictionaries (cnes_bed_types, cnes_habilitation_types,
		// cnes_service_types) are deliberately NOT truncated — the join
		// tests hit the real migration seed, like cnes_equipment_types.
		"cnes_habilitations",
		"cnes_services",
		// Shared public record, like the CNES tables above: no account_id, so
		// truncating accounts never reaches them and rows survive into the next
		// test. That is not hypothetical — the calendar's health-gate test
		// passed alone and failed in the suite, because the previous test's
		// contracts were still sitting there matching its keywords.
		"public_contracts",
		"public_contract_scans",
		// Archived brochures and the pictures pulled out of them. Account-owned
		// but not FK'd to accounts, so the cascade misses them too.
		"product_source_images",
		"product_sources",
		// economic_index_values is a SHARED cache too (§6.11 G5/G6): public index
		// data with no account_id, so truncating accounts never reaches it and a
		// series seeded by one test would leak into the next one's accumulation.
		"economic_index_values",
		// company_enrichments is the SHARED cross-account cache (US-M001):
		// no account_id, no FK, so truncating accounts does not reach it and
		// rows leak between tests (a later test sees an earlier one's CNPJ).
		"funnel_stages",
		"company_enrichments",
		// email_imap_cursors cascades from channels, and channels is truncated
		// above — listed anyway because ListDue is account-blind: a row left by
		// one test shows up as due in the next one's poll and its assertions
		// about ordering start counting a stranger's inbox.
		"email_imap_cursors",
		// discovered_contacts cascades from accounts, but the review list is
		// read per account and a leaked candidate makes the next test's
		// ranking assertions count a stranger.
		"discovered_contacts",
		// email_mailboxes cascades from accounts, but truncating it
		// explicitly keeps the per-test state obvious.
		"email_mailboxes",
		// email_suppressions likewise. A leaked row is invisible until a
		// later test's send is refused for no reason a reader can see.
		"email_suppressions",
		// signals + signal_watches cascade from accounts, but truncating
		// them explicitly keeps a leaked tender from showing up in a later
		// test's feed.
		"signals",
		"signal_watches",
		// signal_source_health belongs to no account, so nothing cascades it
		// — a stamp left by one test would make the next one's radar claim
		// the source is down.
		"signal_source_health",
		"autopilot_sends",
		"autopilot_settings",
		"audiences",
		"sequence_step_events",
		"sequence_enrollments",
		"sequences",
		"strategies",
		"reorder_nudges",
		"check_in_nudges",
		"relationship_dates",
		"sales",
		// sales_events cascades from accounts, but truncating it first keeps
		// a case-report test from counting a previous test's pipeline.
		"sales_events",
		"ai_costs",
		"ai_batch_jobs",
		// account_invites references users (SET NULL) and plans; the users
		// CASCADE would reach it anyway, but implicit reach is how leaks
		// start — listed like everything else.
		"account_invites",
		// oauth_clients has no FK to accounts, so it must be truncated
		// explicitly (codes/tokens cascade from it and from users/accounts).
		"api_tokens",
		"oauth_authorization_codes",
		"oauth_clients",
		"users",
		"accounts",
	}
	// ONE statement, not one per table. This runs ~360 times per suite; with
	// 37 separate TRUNCATE … CASCADE each, that was ~13k truncates, and every
	// CASCADE from accounts/users re-walked ~100 dependent tables — which is
	// exactly where the goroutine sat when the suite blew its 40-minute
	// ceiling on a slow runner (2026-08-19: 42 min on main, 15 on the PR, same
	// code). A single multi-table TRUNCATE takes the locks once and resolves
	// the cascade once: 634 s → 124 s locally.
	_, err := testPool.Exec(ctx, "TRUNCATE TABLE "+strings.Join(tables, ", ")+" CASCADE")
	require.NoError(t, err, "failed to truncate e2e tables")
}

func registerUser(t *testing.T, name, email, password string) {
	t.Helper()
	body := map[string]any{
		"account_name":   name,
		"admin_name":     name,
		"admin_email":    email,
		"password":       password,
		"terms_accepted": true,
	}
	w := doRequest("POST", "/v1/accounts", body, "")
	require.Equal(t, 201, w.Code, "register failed: %s", w.Body.String())
}

func verifyEmail(t *testing.T, emailAddr string) {
	t.Helper()
	// Verification tokens rest as SHA-256 hashes (migration 000222): the
	// raw value only ever travels in the e-mail. Confirm one was issued,
	// then plant a known raw token's hash so the endpoint's hash lookup
	// is what the journey exercises.
	var storedHash string
	err := testPool.QueryRow(context.Background(),
		"SELECT verification_token FROM users WHERE email = $1", emailAddr).Scan(&storedHash)
	require.NoError(t, err, "failed to get verification token")
	require.NotEmpty(t, storedHash, "verification token is empty")
	token := "e2e-raw-verification-token"
	_, err = testPool.Exec(context.Background(),
		"UPDATE users SET verification_token = $2 WHERE email = $1", emailAddr, domain.HashToken(token))
	require.NoError(t, err)
	w := doRequest("GET", "/v1/auth/verify-email?token="+token, nil, "")
	require.Equal(t, 200, w.Code, "verify email failed: %s", w.Body.String())
}

// tokenForSeededAccount mints a session JWT for an account inserted straight
// into the database, so a test does not have to run the whole signup flow
// just to make one authenticated call.
func tokenForSeededAccount(t *testing.T, accountID, role string) string {
	t.Helper()
	userID := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO users (id, account_id, name, email, password_hash, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, 'Teste', $3, 'x', $4, true, NOW(), NOW())`,
		userID, accountID, userID+"@teste.local", role)
	require.NoError(t, err)

	token, err := auth.NewJWTService("e2e-test-jwt-secret", 24).GenerateToken(&domain.User{
		ID:        userID,
		AccountID: accountID,
		Name:      "Teste",
		Role:      &role,
	})
	require.NoError(t, err)
	return token
}

func login(t *testing.T, emailAddr, password string) string {
	t.Helper()
	w := doRequest("POST", "/v1/auth/login", map[string]string{
		"email":    emailAddr,
		"password": password,
	}, "")
	require.Equal(t, 200, w.Code, "login failed: %s", w.Body.String())
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	token, ok := resp["token"].(string)
	require.True(t, ok, "token not found in login response")
	return token
}

func doRequest(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

// parseJSON is a helper to unmarshal a response body into a map.
func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "failed to parse JSON response: %s", w.Body.String())
	return resp
}

// parseJSONArray is a helper to unmarshal a response body into a slice.
func parseJSONArray(t *testing.T, w *httptest.ResponseRecorder) []interface{} {
	t.Helper()
	var resp []interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "failed to parse JSON array: %s", w.Body.String())
	return resp
}

// newE2ERelationshipHandler wires F4/F5 the way main.go does, so the harness
// exercises the real co-purchase SQL rather than a stub.
func newE2ERelationshipHandler(pool *pgxpool.Pool) *handlers.RelationshipHandler {
	h := handlers.NewRelationshipHandler(
		usecases.NewRelationshipUseCase(postgres.NewRelationshipRepository(pool)))
	h.SetCrossSell(usecases.NewCrossSellUseCase(postgres.NewCrossSellRepository(pool)))
	return h
}

// newE2EPortfolioHandler wires the portfolio the same way main.go does, so
// the harness exercises the scoped reorder query rather than a stub.
// newE2ECommissionStatementHandler wires the monthly close (§6.6 G2) against
// the real tables — the whole point of the suite is the paid-line immutability,
// which only a database can demonstrate.
func newE2ECommissionStatementHandler(
	pool *pgxpool.Pool, accountRepo domain.AccountRepository, userRepo domain.UserRepository,
) *handlers.CommissionStatementHandler {
	uc := usecases.NewCommissionStatementUseCase(
		postgres.NewCommissionLineRepository(pool),
		postgres.NewCommissionRuleRepository(pool),
		postgres.NewSaleRepository(pool),
		accountRepo,
	)
	uc.SetUsers(userRepo)
	return handlers.NewCommissionStatementHandler(uc)
}

// newE2EContractHandler wires the contract repository (§6.11 G4) with the audit
// trail but NO storage: the suite must not reach GCS, and every upload/download
// path answering 503 is exactly the behaviour of a deployment with no bucket.
func newE2EContractHandler(
	pool *pgxpool.Pool, audit domain.AuditLogRepository,
) *handlers.ContractHandler {
	contractRepo := postgres.NewContractRepository(pool)
	h := handlers.NewContractHandler(usecases.NewContractUseCase(contractRepo))
	h.SetAudit(audit)
	// The adjustment runs off the CACHED series with no fetcher wired: the suite
	// must not reach BCB, and an empty cache is exactly the "incomplete series"
	// path that must never produce a suggested price.
	h.SetAdjustments(usecases.NewContractAdjustmentUseCase(
		contractRepo, postgres.NewEconomicIndexRepository(pool)))
	return h
}

// newE2EContractAlertHandler wires the clause alerts and the recovered-revenue
// ledger (§6.11 G5/G6) with the real tables and the CACHED index series.
func newE2EContractAlertHandler(pool *pgxpool.Pool) *handlers.ContractAlertHandler {
	contractRepo := postgres.NewContractRepository(pool)
	uc := usecases.NewContractAlertUseCase(
		contractRepo,
		postgres.NewContractAlertRepository(pool),
		postgres.NewContractRecoveryRepository(pool),
	)
	uc.SetAdjustments(usecases.NewContractAdjustmentUseCase(
		contractRepo, postgres.NewEconomicIndexRepository(pool)))
	return handlers.NewContractAlertHandler(uc)
}

// newE2ESignatureHandler wires electronic signature (§6.11 G7) against a FAKE
// provider gateway: the suite must not reach ZapSign, and the whole point under
// test is that a callback is verified against the provider rather than trusted.
//
// The fake's answers are driven by e2eSignatureGateway, which the tests set.
func newE2ESignatureHandler(
	pool *pgxpool.Pool, cipher *secrets.AESGCMCipher,
) *handlers.SignatureHandler {
	uc := usecases.NewSignatureUseCase(
		postgres.NewSignatureRepository(pool),
		postgres.NewSignatureSettingsRepository(pool, cipher),
		e2eSignatureGateway,
	)
	uc.SetStorage(testAttachmentStore, testAttachmentStore)
	uc.SetSales(usecases.NewSaleUseCase(postgres.NewSaleRepository(pool),
		postgres.NewContactRepository(pool)), postgres.NewContractRepository(pool))
	usecases.SetCallbackMapper(signature.MapEventStatus)
	return handlers.NewSignatureHandler(uc)
}

// newE2EDiscoveredContactHandler wires Sent-folder discovery the way main.go
// does, with the real IMAP fetcher.
//
// The SCAN route therefore fails in tests — there is no mail server to dial —
// which is deliberate: the interesting part over HTTP is the review and the
// import, where a confirmation turns a candidate into a contact and a lead.
// The scanner itself is tested against a real in-process IMAP server in the
// email adapter's own package.
func newE2EDiscoveredContactHandler(pool *pgxpool.Pool, leadUC *usecases.LeadUseCase) *handlers.DiscoveredContactHandler {
	uc := usecases.NewDiscoverContactsUseCase(
		emailchannel.NewIMAPFetcher(),
		postgres.NewDiscoveredContactRepository(pool),
		postgres.NewChannelRepository(pool, nil),
		postgres.NewEmailInboxCursorRepository(pool),
	)
	return handlers.NewDiscoveredContactHandler(uc, leadUC)
}

func newE2EPortfolioHandler(pool *pgxpool.Pool) *handlers.PortfolioHandler {
	repo := postgres.NewPortfolioRepository(pool)
	h := handlers.NewPortfolioHandler(repo, postgres.NewSaleRepository(pool))
	h.SetScopedReorders(repo)
	h.SetProductCycles(postgres.NewProductRepository(pool), postgres.NewProductRepository(pool))
	return h
}

// newE2EProposalHandler wires pillar 3 the way main.go does, minus the draft
// writer: the approval-queue hop belongs to the approach use case and is covered
// by its own tests, so an e2e proposal test exercises the pricing and the
// account scoping without also standing up an AI provider.
// e2eMediaSigner mints the same media-proxy URLs main.go does, off the same
// test JWT secret the rest of the suite signs tokens with.
func e2eMediaSigner() *storage.URLSigner {
	return &storage.URLSigner{
		APIBaseURL: "http://localhost:8080",
		Secret:     []byte("e2e-test-jwt-secret"),
	}
}

// newE2EMediaFileHandler serves the proxy. GCS is unconfigured in e2e, so a
// verified request reaches the storage call and fails there — which is exactly
// the boundary these tests care about: everything before the bucket.
func newE2EMediaFileHandler() *handlers.MediaFileHandler {
	return handlers.NewMediaFileHandler(nil, e2eMediaSigner())
}

// newE2EOwnerHandler wires the owner panel the way main.go does, invites
// included — apply-contract and the invite emission are both journey doors.
func newE2EOwnerHandler(
	accountRepo domain.AccountRepository,
	userRepo domain.UserRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	auditLogRepo domain.AuditLogRepository,
	jwtService domain.JWTService,
	inviteUC *usecases.InviteUseCase,
) *handlers.OwnerHandler {
	ownerRepo, _ := interface{}(accountRepo).(domain.OwnerAccountRepository)
	h := handlers.NewOwnerHandler(
		usecases.NewOwnerAdminUseCase(accountRepo, ownerRepo, userRepo, planRepo, usageRepo, auditLogRepo),
		jwtService)
	h.SetInviteUC(inviteUC)
	return h
}

// newE2EProductImportHandler wires both import doors the way main.go does:
// CSV as before, and the PDF brochure door with the harness's boundary fakes
// (clean scanner, attachment store, marker-gated fake AI). The gate ladder —
// page ceiling on the REAL fixture PDF, spend resolution, dedup preview — runs
// for real; only clamd, GCS and the model are stand-ins.
func newE2EProductImportHandler(
	pool *pgxpool.Pool,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	aiFactory domain.AIFactory,
) *handlers.ProductImportHandler {
	csvUC := usecases.NewImportProductsUseCase(postgres.NewProductRepository(pool), accountRepo)
	// Wired as main.go wires it: the brochure id the client posts back is
	// checked against the account that owns it.
	csvUC.SetSourceRepo(postgres.NewProductSourceRepository(pool))
	h := handlers.NewProductImportHandler(csvUC)
	pdfUC := usecases.NewImportProductsPDFUseCase(
		csvUC, accountRepo, planRepo, newE2EAIFactory(aiFactory),
		services.PDFPageCount, 10<<20, 40,
	)
	pdfUC.SetScanner(&e2eCleanScanner{})
	pdfUC.SetStore(testAttachmentStore)
	h.SetPDFUseCase(pdfUC, 10<<20)
	return h
}

// newE2ECustomerImportUseCase wires the BULK dedupe index, exactly as main.go
// does. Without it the e2e would exercise the per-row fallback and the raw SQL
// behind `= ANY($3)` would never run under a real Postgres.
func newE2ECustomerImportUseCase(
	pool *pgxpool.Pool, contactRepo domain.ContactRepository,
	leadUC *usecases.LeadUseCase, funnelRepo domain.SalesFunnelRepository,
) *usecases.ImportCustomersUseCase {
	uc := usecases.NewImportCustomersUseCase(
		contactRepo, postgres.NewContactIdentityRepository(pool))
	uc.SetImportIndex(postgres.NewCustomerImportIndexRepository(pool))
	uc.SetLeadMaker(leadUC, funnelRepo)
	return uc
}

func newE2EProposalHandler(pool *pgxpool.Pool) *handlers.ProposalHandler {
	return handlers.NewProposalHandler(usecases.NewProposalUseCase(
		postgres.NewProposalRepository(pool),
		postgres.NewProductRepository(pool),
		postgres.NewLeadRepository(pool),
	))
}
