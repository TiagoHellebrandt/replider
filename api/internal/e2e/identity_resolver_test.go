package e2e

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-F007: the inbound resolver is what makes email-first outbound possible.
// The point of these tests is the CROSS-CHANNEL case — a person we met by
// email who later replies on WhatsApp must be ONE contact carrying one lead
// history, not two strangers.

func newResolverUC(t *testing.T) *usecases.InboundWebhookUseCase {
	t.Helper()
	uc := usecases.NewInboundWebhookUseCase(
		postgres.NewChannelRepository(testPool, nil),
		postgres.NewConversationRepository(testPool),
		postgres.NewMessageRepository(testPool),
		nil, nil,
	)
	uc.SetContactRepo(postgres.NewContactRepository(testPool))
	uc.SetIdentityRepo(postgres.NewContactIdentityRepository(testPool))
	return uc
}

// seedResolverChannel creates an account + a channel of the given type.
func seedResolverChannel(t *testing.T, channelType string) *domain.Channel {
	t.Helper()
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	ch := &domain.Channel{
		ID:           ulid.Make().String(),
		AccountID:    accountID,
		Name:         channelType + " test",
		Type:         channelType,
		WebhookToken: ulid.Make().String(),
		IsActive:     true,
	}
	_, err := testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, name, provider, webhook_token, is_active, credentials, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, true, '{}', NOW(), NOW())`,
		ch.ID, ch.AccountID, ch.Name, ch.Type, ch.WebhookToken)
	require.NoError(t, err)
	return ch
}

func TestInboundResolver_EmailSenderCreatesContactAndIdentity(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	uc := newResolverUC(t)
	identities := postgres.NewContactIdentityRepository(testPool)
	contacts := postgres.NewContactRepository(testPool)

	ch := seedResolverChannel(t, "email")

	_, err := uc.SaveInbound(ctx, ch, usecases.InboundMessageDTO{
		SenderPhone:       "Compras@Hospital.com", // the sender identifier, whatever its kind
		SenderName:        "Setor de Compras",
		Content:           "Bom dia, gostaria de um orçamento",
		ExternalMessageID: ulid.Make().String(),
	})
	require.NoError(t, err)

	contactID, err := identities.ContactIDByValue(ctx, ch.AccountID, domain.IdentityKindEmail, "compras@hospital.com")
	require.NoError(t, err)
	require.NotEmpty(t, contactID, "an email-first inbound must register an email identity")
	assert.Len(t, contactID, 26, "the contact id is an opaque ULID, not the address")

	contact, err := contacts.GetByID(ctx, contactID, ch.AccountID)
	require.NoError(t, err)
	require.NotNil(t, contact)
	require.NotNil(t, contact.Email)
	assert.Equal(t, "compras@hospital.com", *contact.Email)

	// An inbound message is proof the address reaches them.
	list, err := identities.ListByContact(ctx, ch.AccountID, contactID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.NotNil(t, list[0].VerifiedAt)
}

func TestInboundResolver_SecondInboundReusesContact(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	uc := newResolverUC(t)
	identities := postgres.NewContactIdentityRepository(testPool)

	ch := seedResolverChannel(t, "email")
	dto := func() usecases.InboundMessageDTO {
		return usecases.InboundMessageDTO{
			SenderPhone:       "compras@hospital.com",
			SenderName:        "Setor de Compras",
			Content:           "mensagem",
			ExternalMessageID: ulid.Make().String(),
		}
	}

	_, err := uc.SaveInbound(ctx, ch, dto())
	require.NoError(t, err)
	first, err := identities.ContactIDByValue(ctx, ch.AccountID, domain.IdentityKindEmail, "compras@hospital.com")
	require.NoError(t, err)

	_, err = uc.SaveInbound(ctx, ch, dto())
	require.NoError(t, err)
	second, err := identities.ContactIDByValue(ctx, ch.AccountID, domain.IdentityKindEmail, "compras@hospital.com")
	require.NoError(t, err)

	assert.Equal(t, first, second, "a repeat inbound must reuse the contact, not mint a new one")

	var contactCount int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, ch.AccountID).Scan(&contactCount))
	assert.Equal(t, 1, contactCount)
}

// The pre-pivot behaviour must be unchanged for telephony channels: the
// sender is a phone, and the contact carries it as such.
func TestInboundResolver_WhatsAppSenderKeysOnPhone(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	uc := newResolverUC(t)
	identities := postgres.NewContactIdentityRepository(testPool)
	contacts := postgres.NewContactRepository(testPool)

	ch := seedResolverChannel(t, "whatsapp")
	_, err := uc.SaveInbound(ctx, ch, usecases.InboundMessageDTO{
		SenderPhone:       "+5511988887777",
		SenderName:        "Dr. Almeida",
		Content:           "oi",
		ExternalMessageID: ulid.Make().String(),
	})
	require.NoError(t, err)

	contactID, err := identities.ContactIDByValue(ctx, ch.AccountID, domain.IdentityKindPhone, "+5511988887777")
	require.NoError(t, err)
	require.NotEmpty(t, contactID)

	contact, err := contacts.GetByID(ctx, contactID, ch.AccountID)
	require.NoError(t, err)
	require.NotNil(t, contact)
	require.NotNil(t, contact.Phone)
	assert.Equal(t, "+5511988887777", *contact.Phone)
}

// The payoff: two channels, two identifiers, ONE person. Manually linking the
// second identity to the contact found by email is exactly what the email
// channel (US-M005) will do on a reply, and what a lead's history depends on.
func TestInboundResolver_SameContactAcrossChannels(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	uc := newResolverUC(t)
	identities := postgres.NewContactIdentityRepository(testPool)

	emailCh := seedResolverChannel(t, "email")
	// Same account, second channel.
	waCh := &domain.Channel{
		ID:           ulid.Make().String(),
		AccountID:    emailCh.AccountID,
		Name:         "wa",
		Type:         "whatsapp",
		WebhookToken: ulid.Make().String(),
	}
	_, err := testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, name, provider, webhook_token, is_active, credentials, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, true, '{}', NOW(), NOW())`,
		waCh.ID, waCh.AccountID, waCh.Name, waCh.Type, waCh.WebhookToken)
	require.NoError(t, err)

	_, err = uc.SaveInbound(ctx, emailCh, usecases.InboundMessageDTO{
		SenderPhone:       "diretor@clinica.com",
		Content:           "quero saber mais",
		ExternalMessageID: ulid.Make().String(),
	})
	require.NoError(t, err)

	contactID, err := identities.ContactIDByValue(ctx, emailCh.AccountID, domain.IdentityKindEmail, "diretor@clinica.com")
	require.NoError(t, err)
	require.NotEmpty(t, contactID)

	// Enrichment/user action attaches the phone to the SAME contact...
	_, err = identities.Upsert(ctx, &domain.ContactIdentity{
		AccountID: emailCh.AccountID,
		ContactID: contactID,
		Kind:      domain.IdentityKindPhone,
		Value:     "+5511977776666",
	})
	require.NoError(t, err)

	// ...so when they reply on WhatsApp, the resolver finds the person we
	// already know instead of creating a stranger.
	_, err = uc.SaveInbound(ctx, waCh, usecases.InboundMessageDTO{
		SenderPhone:       "+5511977776666",
		Content:           "oi, vi seu email",
		ExternalMessageID: ulid.Make().String(),
	})
	require.NoError(t, err)

	resolved, err := identities.ContactIDByValue(ctx, waCh.AccountID, domain.IdentityKindPhone, "+5511977776666")
	require.NoError(t, err)
	assert.Equal(t, contactID, resolved, "the WhatsApp reply must resolve to the email-first contact")

	var contactCount int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, emailCh.AccountID).Scan(&contactCount))
	assert.Equal(t, 1, contactCount, "one person, one contact row")
}
