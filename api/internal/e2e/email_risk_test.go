package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The dor: B2B healthcare is a constant target for invoice fraud. Someone
// impersonates a supplier, sends a boleto with new bank details, and the
// money leaves. These run the checks end to end, through the real webhook.

func riskFor(t *testing.T, messageID string) *domain.EmailRiskRecord {
	t.Helper()
	rec, err := postgres.NewEmailRiskRepository(testPool).
		GetByMessage(context.Background(), messageID)
	require.NoError(t, err)
	return rec
}

// Gives the account a history, so the lookalike check has something to
// compare against — a domain resembling nothing you know is just a domain.
func seedKnownContact(t *testing.T, accountID, email string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO contacts (id, account_id, external_id, created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $3, NOW(), NOW(), NOW(), NOW())`,
		ulid.Make().String(), accountID, email)
	require.NoError(t, err)
}

func TestEmailRisk_ImpersonatedSupplierIsQuarantined(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	seedKnownContact(t, ch.AccountID, "vendas@medline.com.br")

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":                   "financeiro@medl1ne.com.br",
		"subject":                "Atualização de dados bancários",
		"text":                   "Mudamos de banco, segue o novo boleto.",
		"authentication_results": "mx; spf=fail; dkim=fail; dmarc=fail",
	})
	require.Equal(t, http.StatusOK, w.Code)

	rec := riskFor(t, lastMessageID(t, ch.AccountID))
	require.NotNil(t, rec, "an impersonation attempt must be recorded")
	assert.True(t, rec.Assessment.Quarantined(), "score was %d", rec.Assessment.Score)
	assert.Equal(t, "medl1ne.com.br", rec.SenderDomain)

	kinds := map[domain.EmailRiskKind]string{}
	for _, s := range rec.Assessment.Signals {
		kinds[s.Kind] = s.Detail
	}
	assert.Contains(t, kinds, domain.RiskAuthFailed)
	assert.Contains(t, kinds, domain.RiskLookalikeDomain)
	// The evidence is what makes it actionable in ten seconds.
	assert.Contains(t, kinds[domain.RiskLookalikeDomain], "medline.com.br")
}

// The failure that kills a warning system: a normal cold reply must stay
// quiet. Prospecting means strangers writing back.
func TestEmailRisk_OrdinaryColdReplyIsNotQuarantined(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":                   "compras@clinicavidanova.com.br",
		"text":                   "Pode mandar a proposta.",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	})

	rec := riskFor(t, lastMessageID(t, ch.AccountID))
	if rec != nil {
		assert.False(t, rec.Assessment.Quarantined(), "score was %d", rec.Assessment.Score)
	}
}

// Nothing found is not stored: recording "tudo certo" for every inbound would
// double the write volume of the busiest table in the product.
//
// Written as two messages from the same sender because "first contact" is
// itself a signal — a contact row means the account PROSPECTED someone, not
// that they have written before, and those are different facts.
func TestEmailRisk_SecondMessageFromAKnownSenderStoresNothing(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	body := map[string]any{
		"from":                   "compras@clinicavidanova.com.br",
		"text":                   "Bom dia",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	}
	// First message: a stranger writing in, which is exactly what prospecting
	// produces. Flagged as a first contact, and that is correct.
	body["message_id"] = "<primeira@clinicavidanova.com.br>"
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, body).Code)
	first := riskFor(t, lastMessageID(t, ch.AccountID))
	require.NotNil(t, first)
	assert.False(t, first.Assessment.Quarantined())

	// Second message from the same address: verified, known, nothing to say.
	body["message_id"] = "<segunda@clinicavidanova.com.br>"
	body["text"] = "Segue nossa resposta"
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, body).Code)

	msgID := lastMessageID(t, ch.AccountID)
	var count int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM email_risk_assessments WHERE message_id = $1`, msgID).Scan(&count))
	assert.Zero(t, count, "a clean message must leave no row")
}

func TestEmailRisk_ReplyToMismatchIsRecorded(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":                   "compras@clinica.com.br",
		"reply_to":               "financeiro@dominio-estranho.com",
		"text":                   "Segue o boleto",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	})

	rec := riskFor(t, lastMessageID(t, ch.AccountID))
	require.NotNil(t, rec)

	var found bool
	for _, s := range rec.Assessment.Signals {
		if s.Kind == domain.RiskReplyToMismatch {
			found = true
			assert.Contains(t, s.Detail, "dominio-estranho.com")
		}
	}
	assert.True(t, found)
}

// The verdict travels with the message so the UI can render it without a
// second decision of its own.
func TestEmailRisk_AssessmentIsReadableAsJSON(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":                   "novo@desconhecido.com.br",
		"text":                   "Olá",
		"authentication_results": "mx; spf=fail; dmarc=fail",
	})

	rec := riskFor(t, lastMessageID(t, ch.AccountID))
	require.NotNil(t, rec)

	raw, err := json.Marshal(rec.Assessment)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"kind":"auth_failed"`)
	assert.Contains(t, string(raw), `"detail":`)
}

// The golpe do boleto, end to end. A supplier the account has paid before
// writes from their real verified address and asks for money at a new
// account. Every technical check passes — this is the one that catches it.
func TestEmailRisk_ChangedBankDetailsFromAKnownSupplier(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	invoice := map[string]any{
		"from":                   "financeiro@medline.com.br",
		"subject":                "Fatura de julho",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	}

	// Two legitimate invoices, same account, establishing the history.
	for i, body := range []string{
		"Segue a fatura. Ag. 1234 C/C 56789-0",
		"Fatura de junho. Ag. 1234 C/C 56789-0",
	} {
		invoice["text"] = body
		invoice["message_id"] = "<fatura" + string(rune('a'+i)) + "@medline.com.br>"
		require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, invoice).Code)
	}

	// The hijacked one.
	invoice["text"] = "Prezados, mudamos de banco. Ag 9999 C/C 111222-3. Segue o novo boleto."
	invoice["message_id"] = "<golpe@medline.com.br>"
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, invoice).Code)

	rec := riskFor(t, lastMessageID(t, ch.AccountID))
	require.NotNil(t, rec, "a changed payment instruction must be recorded")
	assert.True(t, rec.Assessment.Quarantined(), "score was %d", rec.Assessment.Score)

	var detail string
	for _, s := range rec.Assessment.Signals {
		if s.Kind == domain.RiskChangedPaymentDetails {
			detail = s.Detail
		}
	}
	require.NotEmpty(t, detail, "the payment change must be among the signals")
	// The evidence has to be the account itself — that is what the reader
	// checks against the invoice on their desk.
	assert.Contains(t, detail, "9999")
}

// The same supplier re-sending the account they always use must stay silent.
// An alarm on every invoice is an alarm nobody reads.
func TestEmailRisk_SameBankDetailsStaySilent(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	invoice := map[string]any{
		"from":                   "financeiro@medline.com.br",
		"subject":                "Fatura",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	}
	for i, body := range []string{
		"Segue a fatura. Ag. 1234 C/C 56789-0",
		// Same account, different punctuation: must not read as a change.
		"Fatura nova. agência: 1234, conta corrente: 567890",
	} {
		invoice["text"] = body
		invoice["message_id"] = "<f" + string(rune('a'+i)) + "@medline.com.br>"
		require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, invoice).Code)
	}

	rec := riskFor(t, lastMessageID(t, ch.AccountID))
	if rec != nil {
		for _, s := range rec.Assessment.Signals {
			assert.NotEqual(t, domain.RiskChangedPaymentDetails, s.Kind,
				"the account they always use is not a change")
		}
	}
}

// A first invoice from a new supplier is a first invoice. Flagging it would
// fire on every legitimate new relationship.
func TestEmailRisk_FirstInvoiceFromANewSupplierIsNotFraud(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":                   "financeiro@fornecedornovo.com.br",
		"subject":                "Primeira fatura",
		"text":                   "Bom dia, segue a fatura. Ag 4321 C/C 98765-4",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	}).Code)

	rec := riskFor(t, lastMessageID(t, ch.AccountID))
	if rec != nil {
		for _, s := range rec.Assessment.Signals {
			assert.NotEqual(t, domain.RiskChangedPaymentDetails, s.Kind)
		}
		assert.False(t, rec.Assessment.Quarantined())
	}
}

// A false positive that cannot be dismissed is worse than no warning: the
// reader learns to scroll past the banner, and then the real one is invisible
// too. One click has to actually settle it.
func TestEmailRisk_VouchingClearsTheWarningAndSilencesTheSender(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	adminToken := tokenForSeededAccount(t, ch.AccountID, "admin")

	suspicious := map[string]any{
		"from":                   "novo@fornecedordesconhecido.com.br",
		"subject":                "Proposta",
		"text":                   "Bom dia",
		"authentication_results": "mx; spf=fail; dmarc=fail",
	}
	suspicious["message_id"] = "<um@x.com>"
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, suspicious).Code)

	flagged := lastMessageID(t, ch.AccountID)
	require.NotNil(t, riskFor(t, flagged), "the sender must be flagged first")

	w := doRequest("POST", "/v1/messages/"+flagged+"/legitimate", nil, adminToken)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	// The banner on the message they were looking at is gone, because it was
	// answered — not hidden.
	assert.Nil(t, riskFor(t, flagged))

	// And the next message from the same sender is not flagged again. A
	// warning already dismissed that keeps coming back is the same as none.
	suspicious["message_id"] = "<dois@x.com>"
	suspicious["text"] = "Segunda mensagem"
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, suspicious).Code)
	assert.Nil(t, riskFor(t, lastMessageID(t, ch.AccountID)))
}

// Vouching is a statement about one account's relationship. It must never
// silence a warning for a different customer.
func TestEmailRisk_VouchIsScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	chA := seedEmailChannel(t)
	chB := seedEmailChannel(t)
	tokenA := tokenForSeededAccount(t, chA.AccountID, "admin")

	body := map[string]any{
		"from":                   "golpista@dominio-novo.com.br",
		"subject":                "Fatura",
		"text":                   "Bom dia",
		"authentication_results": "mx; spf=fail; dmarc=fail",
		"message_id":             "<a@x.com>",
	}
	require.Equal(t, http.StatusOK, postInboundEmail(t, chA.WebhookToken, body).Code)
	flaggedA := lastMessageID(t, chA.AccountID)
	require.Equal(t, http.StatusNoContent,
		doRequest("POST", "/v1/messages/"+flaggedA+"/legitimate", nil, tokenA).Code)

	// Account B has vouched for nothing.
	body["message_id"] = "<b@x.com>"
	require.Equal(t, http.StatusOK, postInboundEmail(t, chB.WebhookToken, body).Code)
	assert.NotNil(t, riskFor(t, lastMessageID(t, chB.AccountID)),
		"one customer's trust must not silence another's warning")
}

// The address comes from the STORED message, so a caller cannot vouch for an
// address they never received mail from.
func TestEmailRisk_CannotVouchForAnotherAccountsMessage(t *testing.T) {
	cleanDB(t)
	chA := seedEmailChannel(t)
	chB := seedEmailChannel(t)
	tokenB := tokenForSeededAccount(t, chB.AccountID, "admin")

	require.Equal(t, http.StatusOK, postInboundEmail(t, chA.WebhookToken, map[string]any{
		"from": "x@y.com", "subject": "oi", "text": "oi",
		"authentication_results": "mx; spf=fail; dmarc=fail",
	}).Code)

	w := doRequest("POST", "/v1/messages/"+lastMessageID(t, chA.AccountID)+"/legitimate", nil, tokenB)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// The flywheel: one company reports a scam, and the next company is protected
// before it reaches anyone there. This is the network effect the planning
// document calls the defensive moat.
func TestScamReport_TwoCompaniesQuarantineTheDomainForEveryoneElse(t *testing.T) {
	cleanDB(t)
	victimA := seedEmailChannel(t)
	victimB := seedEmailChannel(t)
	targetC := seedEmailChannel(t)
	tokenA := tokenForSeededAccount(t, victimA.AccountID, "admin")
	tokenB := tokenForSeededAccount(t, victimB.AccountID, "admin")

	scam := map[string]any{
		"from":                   "financeiro@golpista-novo.com.br",
		"subject":                "Fatura em aberto",
		"text":                   "Segue boleto. Ag 9999 C/C 111222-3",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	}

	// Two independent companies receive it and report it.
	for _, v := range []struct {
		ch    *domain.Channel
		token string
		mid   string
	}{{victimA, tokenA, "<a@golpe.com>"}, {victimB, tokenB, "<b@golpe.com>"}} {
		scam["message_id"] = v.mid
		require.Equal(t, http.StatusOK, postInboundEmail(t, v.ch.WebhookToken, scam).Code)

		w := doRequest("POST", "/v1/messages/"+lastMessageID(t, v.ch.AccountID)+"/report-scam",
			map[string]string{"reason": "esse fornecedor nunca manda boleto por email"}, v.token)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}

	// A third company, which has never seen this sender, is now protected
	// before anyone there has to make a judgement call.
	scam["message_id"] = "<c@golpe.com>"
	require.Equal(t, http.StatusOK, postInboundEmail(t, targetC.WebhookToken, scam).Code)

	rec := riskFor(t, lastMessageID(t, targetC.AccountID))
	require.NotNil(t, rec)
	assert.True(t, rec.Assessment.Quarantined(), "score was %d", rec.Assessment.Score)

	var networkDetail string
	for _, s := range rec.Assessment.Signals {
		if s.Kind == domain.RiskNetworkReported {
			networkDetail = s.Detail
		}
	}
	require.NotEmpty(t, networkDetail, "the network signal must be present")
	// The reader has to see WHY they are protected — that is what makes the
	// network visible, and being part of it worth paying for.
	assert.Contains(t, networkDetail, "rede")
}

// One report is a single company's judgement and can be a mistake, a grudge
// or a misunderstanding. It must not quarantine anyone else's mail.
func TestScamReport_OneReportDoesNotQuarantineOthers(t *testing.T) {
	cleanDB(t)
	victim := seedEmailChannel(t)
	other := seedEmailChannel(t)
	token := tokenForSeededAccount(t, victim.AccountID, "admin")

	msg := map[string]any{
		"from":                   "contato@empresa-disputada.com.br",
		"subject":                "Proposta",
		"text":                   "Bom dia",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
		"message_id":             "<um@x.com>",
	}
	require.Equal(t, http.StatusOK, postInboundEmail(t, victim.WebhookToken, msg).Code)
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/messages/"+lastMessageID(t, victim.AccountID)+"/report-scam",
			map[string]string{}, token).Code)

	msg["message_id"] = "<dois@x.com>"
	require.Equal(t, http.StatusOK, postInboundEmail(t, other.WebhookToken, msg).Code)

	rec := riskFor(t, lastMessageID(t, other.AccountID))
	if rec != nil {
		for _, s := range rec.Assessment.Signals {
			assert.NotEqual(t, domain.RiskNetworkReported, s.Kind,
				"one company's judgement is not a network verdict")
		}
	}
}

// A single customer must not be able to push a domain over the threshold by
// reporting it repeatedly.
func TestScamReport_RepeatedReportsFromOneAccountCountOnce(t *testing.T) {
	cleanDB(t)
	victim := seedEmailChannel(t)
	other := seedEmailChannel(t)
	token := tokenForSeededAccount(t, victim.AccountID, "admin")

	msg := map[string]any{
		"from":                   "contato@insistente.com.br",
		"subject":                "Oi",
		"text":                   "Bom dia",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
	}
	for i, mid := range []string{"<r1@x.com>", "<r2@x.com>", "<r3@x.com>"} {
		msg["message_id"] = mid
		msg["text"] = "Mensagem " + string(rune('a'+i))
		require.Equal(t, http.StatusOK, postInboundEmail(t, victim.WebhookToken, msg).Code)
		require.Equal(t, http.StatusOK,
			doRequest("POST", "/v1/messages/"+lastMessageID(t, victim.AccountID)+"/report-scam",
				map[string]string{}, token).Code)
	}

	msg["message_id"] = "<outro@x.com>"
	require.Equal(t, http.StatusOK, postInboundEmail(t, other.WebhookToken, msg).Code)

	rec := riskFor(t, lastMessageID(t, other.AccountID))
	if rec != nil {
		for _, s := range rec.Assessment.Signals {
			assert.NotEqual(t, domain.RiskNetworkReported, s.Kind,
				"three reports from one account are still one account")
		}
	}
}

// A scammer who burns a domain but keeps the bank account is the cheaper half
// of their setup to reuse — and still caught.
func TestScamReport_SameBankAccountUnderANewDomainIsCaught(t *testing.T) {
	cleanDB(t)
	a := seedEmailChannel(t)
	b := seedEmailChannel(t)
	target := seedEmailChannel(t)

	for _, v := range []struct {
		ch     *domain.Channel
		token  string
		domain string
	}{
		{a, tokenForSeededAccount(t, a.AccountID, "admin"), "golpe-um.com.br"},
		{b, tokenForSeededAccount(t, b.AccountID, "admin"), "golpe-dois.com.br"},
	} {
		require.Equal(t, http.StatusOK, postInboundEmail(t, v.ch.WebhookToken, map[string]any{
			"from":                   "financeiro@" + v.domain,
			"subject":                "Fatura",
			"text":                   "Pague em Ag 9999 C/C 111222-3",
			"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
			"message_id":             "<" + v.domain + ">",
		}).Code)
		require.Equal(t, http.StatusOK,
			doRequest("POST", "/v1/messages/"+lastMessageID(t, v.ch.AccountID)+"/report-scam",
				map[string]string{}, v.token).Code)
	}

	// Third domain, never reported — but the same account behind it.
	require.Equal(t, http.StatusOK, postInboundEmail(t, target.WebhookToken, map[string]any{
		"from":                   "cobranca@golpe-tres.com.br",
		"subject":                "Fatura",
		"text":                   "Pague em Ag 9999 C/C 111222-3",
		"authentication_results": "mx; spf=pass; dkim=pass; dmarc=pass",
		"message_id":             "<terceiro@x.com>",
	}).Code)

	rec := riskFor(t, lastMessageID(t, target.AccountID))
	require.NotNil(t, rec)
	assert.True(t, rec.Assessment.Quarantined(),
		"the bank account is the indicator that survives a domain change (score %d)",
		rec.Assessment.Score)
}
