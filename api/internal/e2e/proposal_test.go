package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// Priced offers, against real Postgres. The SQL is what carries the guarantees
// here: that lines and their proposal commit together, that a sent offer stops
// being editable, and that a price snapshot survives the catalogue changing.

func seedPricedProduct(t *testing.T, accountID, name string, cents int64, currency string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO products (id, account_id, name, description, price_cents,
		    currency, attributes, active, created_at, updated_at)
		 VALUES ($1,$2,$3,'',$4,$5,'{}',true,NOW(),NOW())`,
		id, accountID, name, cents, currency)
	require.NoError(t, err)
	return id
}

func TestProposal_LinesAndProposalCommitTogether(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	repo := postgres.NewProposalRepository(testPool)

	p := &domain.Proposal{
		ID:        ulid.Make().String(),
		AccountID: accountID,
		LeadID:    leadID,
		Status:    domain.ProposalDraft,
		Currency:  "BRL",
		Body:      "Segue a proposta.",
		Items: []domain.ProposalItem{
			{Name: "Gaze estéril", Quantity: 3, UnitPriceCents: 4990},
			{Name: "Luva cirúrgica", Quantity: 2, UnitPriceCents: 12550},
		},
	}
	require.NoError(t, repo.Create(ctx, p))

	got, err := repo.GetByID(ctx, accountID, p.ID)
	require.NoError(t, err)
	assert.Len(t, got.Items, 2)
	assert.Equal(t, int64(40070), got.TotalCents())
	assert.Equal(t, "BRL", got.Currency)
	assert.Equal(t, domain.ProposalDraft, got.Status)
	assert.Nil(t, got.SentAt)
}

// An offer with no lines is a total of zero presented as an offer.
func TestProposal_RefusesToStoreAnOfferWithNoLines(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	repo := postgres.NewProposalRepository(testPool)

	err := repo.Create(ctx, &domain.Proposal{
		ID: ulid.Make().String(), AccountID: accountID, LeadID: leadID,
		Status: domain.ProposalDraft, Currency: "BRL",
	})
	assert.Equal(t, domain.ErrProposalNoItems, err)
}

// The snapshot is the point: repricing the catalogue must not restate an offer.
func TestProposal_SnapshotSurvivesACatalogueReprice(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	productID := seedPricedProduct(t, accountID, "Gaze estéril", 4990, "brl")
	repo := postgres.NewProposalRepository(testPool)

	p := &domain.Proposal{
		ID: ulid.Make().String(), AccountID: accountID, LeadID: leadID,
		Status: domain.ProposalDraft, Currency: "BRL",
		Items: []domain.ProposalItem{
			{ProductID: productID, Name: "Gaze estéril", Quantity: 3, UnitPriceCents: 4990},
		},
	}
	require.NoError(t, repo.Create(ctx, p))

	_, err := testPool.Exec(ctx,
		`UPDATE products SET name = 'Gaze (novo fornecedor)', price_cents = 9990 WHERE id = $1`,
		productID)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, accountID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Gaze estéril", got.Items[0].Name)
	assert.Equal(t, int64(4990), got.Items[0].UnitPriceCents)
	assert.Equal(t, int64(14970), got.TotalCents())
}

// Discontinuing a product must not erase the line from an offer already made.
func TestProposal_DeletingAProductKeepsTheLine(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	productID := seedPricedProduct(t, accountID, "Gaze estéril", 4990, "brl")
	repo := postgres.NewProposalRepository(testPool)

	p := &domain.Proposal{
		ID: ulid.Make().String(), AccountID: accountID, LeadID: leadID,
		Status: domain.ProposalDraft, Currency: "BRL",
		Items: []domain.ProposalItem{
			{ProductID: productID, Name: "Gaze estéril", Quantity: 1, UnitPriceCents: 4990},
		},
	}
	require.NoError(t, repo.Create(ctx, p))

	_, err := testPool.Exec(ctx, `DELETE FROM products WHERE id = $1`, productID)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, accountID, p.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "Gaze estéril", got.Items[0].Name)
	assert.Equal(t, int64(4990), got.Items[0].UnitPriceCents)
	// The reference is gone; the record is not.
	assert.Empty(t, got.Items[0].ProductID)
}

// A sent proposal is a statement somebody is holding.
func TestProposal_SentOfferStopsBeingEditable(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	repo := postgres.NewProposalRepository(testPool)

	p := &domain.Proposal{
		ID: ulid.Make().String(), AccountID: accountID, LeadID: leadID,
		Status: domain.ProposalDraft, Currency: "BRL",
		Items: []domain.ProposalItem{{Name: "Gaze", Quantity: 1, UnitPriceCents: 4990}},
	}
	require.NoError(t, repo.Create(ctx, p))

	sentAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.MarkSent(ctx, accountID, p.ID, sentAt))

	// Both edit paths report WHY, not "not found" — the record is right there.
	assert.Equal(t, domain.ErrProposalNotEditable,
		repo.UpdateBody(ctx, accountID, p.ID, "outra coisa"))
	assert.Equal(t, domain.ErrProposalNotEditable,
		repo.ReplaceItems(ctx, accountID, p.ID,
			[]domain.ProposalItem{{Name: "Luva", Quantity: 1, UnitPriceCents: 100}}, "BRL"))

	got, err := repo.GetByID(ctx, accountID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProposalSent, got.Status)
	require.NotNil(t, got.SentAt)
}

// MarkSent is forward-only: a second call must not restate when the offer left.
func TestProposal_MarkSentDoesNotRestamp(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	repo := postgres.NewProposalRepository(testPool)

	p := &domain.Proposal{
		ID: ulid.Make().String(), AccountID: accountID, LeadID: leadID,
		Status: domain.ProposalDraft, Currency: "BRL",
		Items: []domain.ProposalItem{{Name: "Gaze", Quantity: 1, UnitPriceCents: 4990}},
	}
	require.NoError(t, repo.Create(ctx, p))

	first := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	require.NoError(t, repo.MarkSent(ctx, accountID, p.ID, first))
	require.NoError(t, repo.MarkSent(ctx, accountID, p.ID, time.Now().UTC()))

	got, err := repo.GetByID(ctx, accountID, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got.SentAt)
	assert.WithinDuration(t, first, got.SentAt.UTC(), time.Second)
}

// Replacing lines swaps the whole set, and the old rows go with it.
func TestProposal_ReplaceLinesLeavesNoOrphanRows(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	repo := postgres.NewProposalRepository(testPool)

	p := &domain.Proposal{
		ID: ulid.Make().String(), AccountID: accountID, LeadID: leadID,
		Status: domain.ProposalDraft, Currency: "BRL",
		Items: []domain.ProposalItem{
			{Name: "Gaze", Quantity: 1, UnitPriceCents: 4990},
			{Name: "Luva", Quantity: 1, UnitPriceCents: 12550},
		},
	}
	require.NoError(t, repo.Create(ctx, p))

	require.NoError(t, repo.ReplaceItems(ctx, accountID, p.ID,
		[]domain.ProposalItem{{Name: "Swab", Quantity: 4, UnitPriceCents: 500}}, "USD"))

	got, err := repo.GetByID(ctx, accountID, p.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "Swab", got.Items[0].Name)
	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, int64(2000), got.TotalCents())

	var rows int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM proposal_items WHERE proposal_id = $1`, p.ID).Scan(&rows))
	assert.Equal(t, 1, rows)
}

// Every read is account-scoped: an offer is somebody's commercial terms.
func TestProposal_AnotherAccountCannotReadOrEditIt(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	owner := seedIdentityAccount(t)
	stranger := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, owner)
	repo := postgres.NewProposalRepository(testPool)

	p := &domain.Proposal{
		ID: ulid.Make().String(), AccountID: owner, LeadID: leadID,
		Status: domain.ProposalDraft, Currency: "BRL",
		Items: []domain.ProposalItem{{Name: "Gaze", Quantity: 1, UnitPriceCents: 4990}},
	}
	require.NoError(t, repo.Create(ctx, p))

	_, err := repo.GetByID(ctx, stranger, p.ID)
	assert.Equal(t, domain.ErrProposalNotFound, err)

	// And the edit paths answer "not found" rather than "not editable" — a
	// stranger must not learn that the id exists.
	assert.Equal(t, domain.ErrProposalNotFound,
		repo.UpdateBody(ctx, stranger, p.ID, "meu texto"))

	list, err := repo.ListByLead(ctx, stranger, leadID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

// The history is newest first: the last offer shapes the next one.
func TestProposal_HistoryIsNewestFirst(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	leadID := seedLeadForSequence(t, accountID)
	repo := postgres.NewProposalRepository(testPool)

	for _, name := range []string{"primeira", "segunda"} {
		p := &domain.Proposal{
			ID: ulid.Make().String(), AccountID: accountID, LeadID: leadID,
			Status: domain.ProposalDraft, Currency: "BRL", Body: name,
			Items: []domain.ProposalItem{{Name: "Gaze", Quantity: 1, UnitPriceCents: 4990}},
		}
		require.NoError(t, repo.Create(ctx, p))
		// created_at defaults to NOW(); without a gap the order is by tie-break.
		_, err := testPool.Exec(ctx,
			`UPDATE proposals SET created_at = created_at + (CASE WHEN body = 'segunda' THEN interval '1 hour' ELSE interval '0' END) WHERE id = $1`,
			p.ID)
		require.NoError(t, err)
	}

	list, err := repo.ListByLead(ctx, accountID, leadID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "segunda", list[0].Body)
	assert.Equal(t, "primeira", list[1].Body)
	// Lines travel with each proposal — a list that renders totals cannot get
	// them from half the rows.
	assert.Len(t, list[0].Items, 1)
	assert.Len(t, list[1].Items, 1)
}
