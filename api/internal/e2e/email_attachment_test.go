package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The gap this closes: an attachment used to be parsed and dropped, so a
// clinic replying "segue em anexo o termo de referência" produced a message
// pointing at a file that did not exist, with nothing logged and nothing
// shown. These run the whole path — provider webhook to stored row.

func lastMessageID(t *testing.T, accountID string) string {
	t.Helper()
	var id string
	err := testPool.QueryRow(context.Background(),
		`SELECT m.id FROM messages m
		   JOIN conversations c ON c.id = m.conversation_id
		  WHERE c.account_id = $1
		  ORDER BY m.created_at DESC LIMIT 1`, accountID).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestEmailInbound_StoresAttachment(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)
	before := testAttachmentStore.count()

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":    "compras@clinicavidanova.com.br",
		"subject": "Re: monitores multiparâmetro",
		"text":    "Segue em anexo o termo de referência.",
		"attachments": []map[string]any{{
			"filename":     "termo-de-referencia.pdf",
			"content_type": "application/pdf",
			"content":      base64.StdEncoding.EncodeToString([]byte("%PDF-1.7 conteudo")),
		}},
	})
	require.Equal(t, http.StatusOK, w.Code)

	repo := postgres.NewMessageAttachmentRepository(testPool)
	items, err := repo.ListByMessage(ctx, lastMessageID(t, ch.AccountID))
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, "termo-de-referencia.pdf", items[0].Filename)
	assert.Equal(t, "application/pdf", items[0].ContentType)
	assert.Equal(t, int64(17), items[0].SizeBytes)
	assert.False(t, items[0].IsInline)
	// No scanner is wired in the suite, so the honest verdict is "nobody
	// looked" — and that must not be downloadable.
	assert.Equal(t, domain.ScanSkipped, items[0].ScanStatus)
	assert.False(t, items[0].Downloadable())

	assert.Equal(t, before+1, testAttachmentStore.count(), "the bytes must reach storage")
	// The object path is keyed by the attachment id, never by the
	// sender-supplied filename.
	assert.Contains(t, items[0].ObjectPath, "accounts/"+ch.AccountID+"/")
	assert.NotContains(t, items[0].ObjectPath, "termo-de-referencia.pdf")
}

// A signature logo is not what a person means by "anexo": it is stored (the
// original-view needs it) but flagged so the UI never counts it.
func TestEmailInbound_MarksInlineFurniture(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "dr@hospital.com.br",
		"text": "Bom dia",
		"attachments": []map[string]any{
			{"filename": "logo.png", "content_type": "image/png", "content_id": "<logo@sig>",
				"content": base64.StdEncoding.EncodeToString([]byte("PNG"))},
			{"filename": "proposta.pdf", "content_type": "application/pdf",
				"content": base64.StdEncoding.EncodeToString([]byte("%PDF"))},
		},
	})
	require.Equal(t, http.StatusOK, w.Code)

	items, err := postgres.NewMessageAttachmentRepository(testPool).
		ListByMessage(ctx, lastMessageID(t, ch.AccountID))
	require.NoError(t, err)
	require.Len(t, items, 2)

	byName := map[string]*domain.MessageAttachment{}
	for _, a := range items {
		byName[a.Filename] = a
	}
	assert.True(t, byName["logo.png"].IsInline)
	assert.False(t, byName["proposta.pdf"].IsInline)
}

// The sender picks the filename, so it is hostile input.
func TestEmailInbound_SanitisesHostileFilename(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "a@b.com",
		"text": "oi",
		"attachments": []map[string]any{{
			"filename": "../../../etc/passwd",
			"content":  base64.StdEncoding.EncodeToString([]byte("root:x:0:0")),
		}},
	})
	require.Equal(t, http.StatusOK, w.Code)

	items, err := postgres.NewMessageAttachmentRepository(testPool).
		ListByMessage(ctx, lastMessageID(t, ch.AccountID))
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "passwd", items[0].Filename)
	assert.NotContains(t, items[0].ObjectPath, "..")
}

// A verdict nobody computed must be visible as such. This is the failure the
// whole authentication pass exists to remove: an unchecked message rendering
// as a checked one.
func TestEmailInbound_RecordsSenderAuthentication(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "compras@clinica.com.br",
		"text": "Podemos conversar?",
		"headers": map[string]string{
			"Authentication-Results": "mx.google.com; spf=pass; dkim=pass; dmarc=pass",
		},
	})
	require.Equal(t, http.StatusOK, w.Code)

	var raw []byte
	err := testPool.QueryRow(ctx,
		`SELECT metadata FROM messages WHERE id = $1`, lastMessageID(t, ch.AccountID)).Scan(&raw)
	require.NoError(t, err)

	var meta struct {
		Auth struct {
			SPF         string `json:"spf"`
			DMARC       string `json:"dmarc"`
			Trustworthy bool   `json:"trustworthy"`
			Suspicious  bool   `json:"suspicious"`
		} `json:"email_auth"`
	}
	require.NoError(t, json.Unmarshal(raw, &meta))
	assert.Equal(t, "pass", meta.Auth.SPF)
	assert.Equal(t, "pass", meta.Auth.DMARC)
	assert.True(t, meta.Auth.Trustworthy)
	assert.False(t, meta.Auth.Suspicious)
}

// A forged sender is recorded as suspicious rather than dropped: the
// salesperson still sees the message, marked, and decides.
func TestEmailInbound_FlagsForgedSender(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":                   "compras@clinica.com.br",
		"text":                   "Mudamos nossa conta bancária, segue o novo boleto.",
		"authentication_results": "mx; spf=fail; dkim=fail; dmarc=fail",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var raw []byte
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT metadata FROM messages WHERE id = $1`, lastMessageID(t, ch.AccountID)).Scan(&raw))

	var meta struct {
		Auth struct {
			Trustworthy bool `json:"trustworthy"`
			Suspicious  bool `json:"suspicious"`
		} `json:"email_auth"`
	}
	require.NoError(t, json.Unmarshal(raw, &meta))
	assert.False(t, meta.Auth.Trustworthy)
	assert.True(t, meta.Auth.Suspicious)
}

// No header and no vendor fields: unknown. Not trustworthy, not suspicious —
// the three-way distinction is the point.
func TestEmailInbound_UncheckedSenderIsNeitherVerifiedNorForged(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "novo@fornecedor.com.br",
		"text": "Bom dia",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var raw []byte
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT metadata FROM messages WHERE id = $1`, lastMessageID(t, ch.AccountID)).Scan(&raw))

	var meta struct {
		Auth struct {
			SPF         string `json:"spf"`
			Trustworthy bool   `json:"trustworthy"`
			Suspicious  bool   `json:"suspicious"`
		} `json:"email_auth"`
	}
	require.NoError(t, json.Unmarshal(raw, &meta))
	assert.Equal(t, "unknown", meta.Auth.SPF)
	assert.False(t, meta.Auth.Trustworthy)
	assert.False(t, meta.Auth.Suspicious)
}

// lastConversationID is the newest conversation on an account, for tests that
// act on a thread rather than on one message.
func lastConversationID(t *testing.T, accountID string) string {
	t.Helper()
	var id string
	err := testPool.QueryRow(context.Background(),
		`SELECT id FROM conversations WHERE account_id = $1
		  ORDER BY created_at DESC LIMIT 1`, accountID).Scan(&id)
	require.NoError(t, err)
	return id
}
