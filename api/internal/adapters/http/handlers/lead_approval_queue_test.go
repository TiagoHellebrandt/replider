package handlers

import (
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The queue item must carry the lead's own fields at the TOP level. The
// dashboard reads contact_label/contact_address/approach_draft straight off
// each entry, so wrapping the lead in a sub-object would empty the approval
// screen without any error to notice.
func TestApprovalQueueItem_KeepsLeadFieldsFlat(t *testing.T) {
	item := approvalQueueItem{
		Lead: &domain.Lead{
			ID:             "lead_1",
			ContactLabel:   "Clínica Exemplo",
			ContactAddress: "+5515999990000",
			ApproachDraft:  "Bom dia, tudo bem?",
		},
		Findings: findingsOrEmpty("Bom dia, tudo bem?", false),
	}

	var out map[string]any
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &out))

	assert.Equal(t, "lead_1", out["id"])
	assert.Equal(t, "Clínica Exemplo", out["contact_label"])
	assert.Equal(t, "+5515999990000", out["contact_address"])
	assert.Equal(t, "Bom dia, tudo bem?", out["approach_draft"])
	assert.Contains(t, out, "approach_findings")
}

// A clean draft must serialize as [], never null. The client renders "no
// promise found" from an empty list; a null is indistinguishable from "the
// field is missing", which is a different statement about the same message.
func TestApprovalQueueItem_CleanDraftIsEmptyListNotNull(t *testing.T) {
	raw, err := json.Marshal(approvalQueueItem{
		Lead:     &domain.Lead{ID: "lead_1", ApproachDraft: "Bom dia, posso mandar duas opções de preço?"},
		Findings: findingsOrEmpty("Bom dia, posso mandar duas opções de preço?", false),
	})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"approach_findings":[]`)
}

// The kind travels with the quote: telling someone "você prometeu" when the
// real problem is "you asserted something you do not know" sends them to fix the wrong
// sentence.
func TestApprovalQueueItem_CarriesKindAndQuote(t *testing.T) {
	const draft = "Garantimos atendimento e vi que vocês abriram uma ala nova."

	raw, err := json.Marshal(approvalQueueItem{
		Lead:     &domain.Lead{ID: "lead_1", ApproachDraft: draft},
		Findings: findingsOrEmpty(draft, false),
	})
	require.NoError(t, err)

	var out struct {
		Findings []domain.ApproachFinding `json:"approach_findings"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))

	kinds := map[string]string{}
	for _, f := range out.Findings {
		kinds[f.Kind] = f.Quote
	}
	assert.Contains(t, kinds, "promise")
	assert.Contains(t, kinds, "invented_fact")
	assert.NotEmpty(t, kinds["promise"])
	assert.NotEmpty(t, kinds["invented_fact"])
}

// Reported from the screen: a draft that says it carries a file, with nothing
// attached, gets answered with "não veio nada" — and the salesperson looks
// careless at exactly the moment they were trying not to.
func TestApprovalQueueItem_FlagsAPromisedAttachmentThatIsNotThere(t *testing.T) {
	const draft = "Segue em anexo o termo de referência."

	withoutFile := findingsOrEmpty(draft, false)
	var found bool
	for _, f := range withoutFile {
		if f.Kind == "missing_attachment" {
			found = true
		}
	}
	assert.True(t, found, "the mismatch must be reported")

	// With a file actually attached the same text is simply true.
	for _, f := range findingsOrEmpty(draft, true) {
		assert.NotEqual(t, "missing_attachment", f.Kind)
	}
}

// An unfilled placeholder is caught by the same free pass, so it is visible
// on the card rather than only at the moment the send is refused.
func TestApprovalQueueItem_FlagsAnUnfilledPlaceholder(t *testing.T) {
	got := findingsOrEmpty("Olá {{nome}}, tudo bem?", false)

	require.Len(t, got, 1)
	assert.Equal(t, "placeholder", got[0].Kind)
	assert.Equal(t, "{{nome}}", got[0].Quote)
}

// The response says how much it left out.
//
// This endpoint used to answer with a bare array, so a 50-item page and a
// 50-item queue were the same bytes. The operator had no way to know 290 more
// drafts were waiting, and no honest way to build a funnel filter — chips
// derived from a page label the backlog with the wrong number and give no chip
// at all to the funnel holding the rest.
func TestApprovalQueueResponse_CarriesTheWholeBacklogBesideThePage(t *testing.T) {
	raw, err := json.Marshal(approvalQueueResponse{
		Items: []approvalQueueItem{{
			Lead:     &domain.Lead{ID: "lead_1", ApproachDraft: "Bom dia"},
			Findings: findingsOrEmpty("Bom dia", false),
		}},
		Total: 340,
		ByFunnel: []domain.QueueFunnelCount{
			{FunnelID: "fn_1", FunnelName: "Clínicas", Total: 290},
			{FunnelID: "fn_2", FunnelName: "Academias", Total: 50},
		},
	})
	require.NoError(t, err)

	var out struct {
		Items    []map[string]any          `json:"items"`
		Total    int64                     `json:"total"`
		ByFunnel []domain.QueueFunnelCount `json:"by_funnel"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))

	require.Len(t, out.Items, 1)
	// Still flat: the lead's fields did not move when the envelope arrived.
	assert.Equal(t, "lead_1", out.Items[0]["id"])
	assert.Equal(t, int64(340), out.Total)
	require.Len(t, out.ByFunnel, 2)
	assert.Equal(t, "Clínicas", out.ByFunnel[0].FunnelName)
	assert.Equal(t, int64(290), out.ByFunnel[0].Total)
}

// An empty queue serializes as [] for both lists. A null items would make the
// client's "empty queue" state indistinguishable from a parse failure, and a
// null by_funnel is the same trap the findings list already avoids.
func TestApprovalQueueResponse_EmptyIsListsNotNulls(t *testing.T) {
	raw, err := json.Marshal(approvalQueueResponse{
		Items:    []approvalQueueItem{},
		ByFunnel: []domain.QueueFunnelCount{},
	})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"items":[]`)
	assert.Contains(t, string(raw), `"by_funnel":[]`)
	assert.Contains(t, string(raw), `"total":0`)
}
