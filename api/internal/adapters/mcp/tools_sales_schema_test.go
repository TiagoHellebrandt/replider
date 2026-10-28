package mcp

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// A tool whose OUTPUT its own schema rejects is a tool that does not exist.
//
// Every MCP result is validated against the schema inferred from the Go struct
// before it leaves the server. `json.RawMessage` is a `[]byte`, so it inferred
// `["null","array"]`, and a lead enriched by Places — an object — failed:
//
//	validating /properties/enrichment: type map[address:… place_id:…]
//	has type "object", want one of "null, array"
//
// That killed get_lead, list_funnel_leads and approve_send_approach in
// production for months. Nothing caught it because the dashboard talks to the
// REST routes; only the co-pilot comes through here, and a co-pilot that
// cannot read a lead just looks broken rather than reporting a schema fault.
//
// So the guard is not "does the struct compile" — it is the real round trip:
// infer the schema the SDK will infer, resolve it, and validate a projection
// built from a lead shaped like the ones production actually stores.
func TestLeadSummary_SurvivesItsOwnOutputSchema(t *testing.T) {
	lead := &domain.Lead{
		ID:            "01KYZW0NTS5WQP3QM7S5EZ256H",
		FunnelID:      "01KYW9HYFXQK3Y41HAJFTG9CSS",
		ContactID:     "01KYZW0NTR0VN33KK899WJ9JXR",
		Status:        domain.LeadStatusPendingApproval,
		ApproachDraft: "Olá!",
		// The exact shape that broke it: an OBJECT, from google_places.
		Enrichment: json.RawMessage(
			`{"source":"google_places","place_id":"ChIJK","address":"Praça da Sé, 20"}`),
		CollectedMetadata: json.RawMessage(`{"orcamento":{"value":"até 50 mil"}}`),
	}

	requireValidatesAgainstOwnSchema(t, toLeadSummary(lead))
}

// The funnel projection carries the same kind of blob (an ARRAY, this time),
// so it broke the same way and has to be pinned the same way.
func TestFunnelDetail_SurvivesItsOwnOutputSchema(t *testing.T) {
	funnel := &domain.SalesFunnel{
		ID:             "01KYW9HYFXQK3Y41HAJFTG9CSS",
		Name:           "Clientes",
		MetadataSchema: json.RawMessage(`[{"key":"orcamento","required":true}]`),
	}

	requireValidatesAgainstOwnSchema(t, toFunnelDetail(funnel))
}

// An unreadable blob must not take the whole read down with it: everything
// else about the lead is still true and still worth answering.
func TestLeadSummary_GarbageBlobIsOmittedNotFatal(t *testing.T) {
	lead := &domain.Lead{
		ID:         "01KYZW0NTS5WQP3QM7S5EZ256H",
		Status:     domain.LeadStatusProspected,
		Enrichment: json.RawMessage(`{not json`),
	}

	out := toLeadSummary(lead)
	require.Nil(t, out.Enrichment)
	require.Equal(t, "01KYZW0NTS5WQP3QM7S5EZ256H", out.ID)
	requireValidatesAgainstOwnSchema(t, out)
}

// Proof that the guard above has teeth: the shape it replaced still fails,
// exactly the way production failed. If this ever starts passing, the SDK
// changed how it infers []byte and the two guards above are no longer
// protecting anything.
func TestRawMessageField_IsWhyTheToolsWereBroken(t *testing.T) {
	type oldShape struct {
		Enrichment json.RawMessage `json:"enrichment,omitempty"`
	}

	schema, err := jsonschema.For[oldShape](nil)
	require.NoError(t, err)
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)

	var object any
	require.NoError(t, json.Unmarshal([]byte(`{"enrichment":{"source":"google_places"}}`), &object))

	require.Error(t, resolved.Validate(object),
		"a json.RawMessage field advertises null|array and rejects the object it carries")
}

func requireValidatesAgainstOwnSchema[T any](t *testing.T, out T) {
	t.Helper()

	schema, err := jsonschema.For[T](nil)
	require.NoError(t, err, "the SDK must be able to infer a schema for this projection")

	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)

	// Validate the marshalled form, which is what actually travels.
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	var generic any
	require.NoError(t, json.Unmarshal(raw, &generic))

	require.NoError(t, resolved.Validate(generic),
		"the tool result must satisfy the schema the SDK advertises for it")
}
