package e2e

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M008. The board is a second view of the same pipeline, so the tests
// that matter are the ones proving the two never disagree: a card that moved
// visually but not in the pipeline would make the AI, the approval queue and
// the salesperson each believe something different.

func newBoardFixture(t *testing.T) (rbacFixture, *postgres.FunnelStageRepository) {
	t.Helper()
	f := newRBACFixture(t)
	stages := postgres.NewFunnelStageRepository(testPool)
	f.uc.SetStageRepo(stages)
	return f, stages
}

func TestKanban_SeedsDefaultBoardOnceAndPlacesEveryLead(t *testing.T) {
	f, stages := newBoardFixture(t)
	ctx := context.Background()
	gestor := f.actor(f.gestor, domain.RoleGestor)

	board, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	require.NotEmpty(t, board.Stages)
	assert.Equal(t, "Novos", board.Stages[0].Name)

	// Every lead in the fixture predates the board (no stage_id) and must
	// still appear — a board that silently drops cards is worse than none.
	assert.Len(t, board.Cards, 4)
	for _, c := range board.Cards {
		assert.NotEmpty(t, c.StageID, "lead %s has no column", c.Lead.ID)
	}

	// Seeding is idempotent: a second load must not duplicate the columns.
	again, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	assert.Len(t, again.Stages, len(board.Stages))

	// ...and a customer who deleted a column does not get it resurrected.
	require.NoError(t, stages.Delete(ctx, f.accountID, board.Stages[0].ID))
	afterDelete, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	assert.Len(t, afterDelete.Stages, len(board.Stages)-1, "the board belongs to the customer once they touch it")
}

// The core guarantee: moving a card writes the canonical status too.
func TestKanban_MoveWritesStageAndStatusTogether(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()
	gestor := f.actor(f.gestor, domain.RoleGestor)

	board, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)

	var won *domain.FunnelStage
	for _, s := range board.Stages {
		if s.MapsToStatus == domain.LeadStatusWon {
			won = s
		}
	}
	require.NotNil(t, won)

	moved, err := f.uc.MoveLead(ctx, gestor, f.leadOfA, won.ID, "", "")
	require.NoError(t, err)
	require.NotNil(t, moved.StageID)
	assert.Equal(t, won.ID, *moved.StageID)
	assert.Equal(t, domain.LeadStatusWon, moved.Status, "the pipeline follows the board, atomically")

	// Re-read from the DB: the two columns must agree there too, not just in
	// the returned struct.
	var stageID, status string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT stage_id, status FROM leads WHERE id = $1`, f.leadOfA).Scan(&stageID, &status))
	assert.Equal(t, won.ID, stageID)
	assert.Equal(t, "won", status)
}

// A vendedor may LOOK at a teammate's team-visible lead but must not drag it
// through their pipeline — visibility governs looking, not touching.
func TestKanban_VendedorCannotMoveAColleaguesCard(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()
	actorA := f.actor(f.vendedorA, domain.RoleVendedor)

	board, err := f.uc.Board(ctx, f.actor(f.gestor, domain.RoleGestor), f.funnelID, 50, 0)
	require.NoError(t, err)
	target := board.Stages[1]

	_, err = f.uc.MoveLead(ctx, actorA, f.leadOfB, target.ID, "", "")
	assert.ErrorIs(t, err, domain.ErrForbidden)

	// ...but their own card moves fine.
	_, err = f.uc.MoveLead(ctx, actorA, f.leadOfA, target.ID, "", "")
	assert.NoError(t, err)
}

// A card dropped on another funnel's column would silently leave the
// pipeline it belongs to.
func TestKanban_CannotMoveCardIntoAnotherFunnelsColumn(t *testing.T) {
	f, stages := newBoardFixture(t)
	ctx := context.Background()
	gestor := f.actor(f.gestor, domain.RoleGestor)

	otherFunnel := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Outro Funil', NOW(), NOW())`, otherFunnel, f.accountID)
	require.NoError(t, err)

	foreign, err := stages.Create(ctx, &domain.FunnelStage{
		AccountID: f.accountID, FunnelID: otherFunnel,
		Name: "Coluna de outro funil", Position: 100, MapsToStatus: domain.LeadStatusQualified,
	})
	require.NoError(t, err)

	_, err = f.uc.MoveLead(ctx, gestor, f.leadOfA, foreign.ID, "", "")
	assert.ErrorIs(t, err, domain.ErrStageWrongFunnel)
}

// The board must not become a backdoor around US-F009: a vendedor's board
// shows their pipeline, not the floor's.
func TestKanban_BoardRespectsOwnershipScoping(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()

	board, err := f.uc.Board(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.funnelID, 50, 0)
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, c := range board.Cards {
		ids[c.Lead.ID] = true
	}
	assert.True(t, ids[f.leadOfA])
	assert.False(t, ids[f.privateB], "a colleague's private lead stays off the board")
}

// Two columns with the same name make the board ambiguous to drag onto.
func TestKanban_DuplicateColumnNameIsRejected(t *testing.T) {
	f, stages := newBoardFixture(t)
	ctx := context.Background()

	_, err := stages.EnsureDefaults(ctx, f.accountID, f.funnelID)
	require.NoError(t, err)

	_, err = stages.Create(ctx, &domain.FunnelStage{
		AccountID: f.accountID, FunnelID: f.funnelID,
		Name: "novos", Position: 999, MapsToStatus: domain.LeadStatusNew,
	})
	assert.ErrorIs(t, err, domain.ErrStageNameTaken, "comparison is case-insensitive")
}

// Deleting a column must not delete the deals inside it.
func TestKanban_DeletingAColumnKeepsItsLeads(t *testing.T) {
	f, stages := newBoardFixture(t)
	ctx := context.Background()
	gestor := f.actor(f.gestor, domain.RoleGestor)

	board, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	target := board.Stages[2]

	_, err = f.uc.MoveLead(ctx, gestor, f.leadOfA, target.ID, "", "")
	require.NoError(t, err)
	require.NoError(t, stages.Delete(ctx, f.accountID, target.ID))

	after, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)

	var found *usecases.BoardCard
	for i := range after.Cards {
		if after.Cards[i].Lead.ID == f.leadOfA {
			found = &after.Cards[i]
		}
	}
	require.NotNil(t, found, "the lead survived its column")
	assert.NotEmpty(t, found.StageID, "and was re-placed by its canonical status")
}

// The board must say WHO each card is, not which ULID it is. The operator
// approves an outbound message from this view; a recipient they cannot
// recognise makes the human-in-the-loop gate decorative.
func TestKanban_CardsCarryARecognisableRecipient(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()

	// One contact gets a real name; the rest keep only the external id they
	// were seeded with.
	_, err := testPool.Exec(ctx,
		`UPDATE contacts SET name = 'Clínica Bem Viver' WHERE account_id = $1 AND external_id = '+5511900000001'`,
		f.accountID)
	require.NoError(t, err)

	board, err := f.uc.Board(ctx, f.actor(f.gestor, domain.RoleGestor), f.funnelID, 50, 0)
	require.NoError(t, err)

	labels := map[string]string{}
	for _, c := range board.Cards {
		labels[c.Lead.ID] = c.Lead.ContactLabel
	}
	assert.Equal(t, "Clínica Bem Viver", labels[f.leadOfA], "a named contact shows its name")
	// No name on file: the phone number is still something a human reads.
	assert.Equal(t, "+5511900000002", labels[f.leadOfB])
}

// US-M020. The approval queue is the operator's whole job in one list, so it
// crosses funnels — and it must obey the same ownership rules as the
// pipeline, or it becomes a backdoor to a teammate's private leads.
func TestApprovalQueue_CrossesFunnelsOldestFirstAndRespectsOwnership(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()

	// A second funnel with its own pending draft: the queue is per operator,
	// not per funnel.
	otherFunnel := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Outro funil', NOW(), NOW())`, otherFunnel, f.accountID)
	require.NoError(t, err)
	otherLead := seedRBACLead(t, f.accountID, otherFunnel,
		seedIdentityContact(t, f.accountID, "+5511900000009"), &f.vendedorB, "team")

	// leadOfA waited longer than the one in the other funnel.
	for id, ago := range map[string]string{f.leadOfA: "2 hours", otherLead: "10 minutes"} {
		_, err = testPool.Exec(ctx,
			`UPDATE leads SET status = 'pending_approval', updated_at = NOW() - $2::interval WHERE id = $1`,
			id, ago)
		require.NoError(t, err)
	}
	// A private draft of vendedor B, which vendedor A must never approve.
	_, err = testPool.Exec(ctx,
		`UPDATE leads SET status = 'pending_approval', updated_at = NOW() - interval '1 minute' WHERE id = $1`,
		f.privateB)
	require.NoError(t, err)

	gestor, err := f.uc.ApprovalQueue(ctx, f.actor(f.gestor, domain.RoleGestor), usecases.ApprovalQueueOptions{}, 50, 0)
	require.NoError(t, err)
	require.Len(t, gestor.Items, 3)
	// Oldest first: the draft that has waited longest is the one going stale.
	assert.Equal(t,
		[]string{f.leadOfA, otherLead, f.privateB},
		[]string{gestor.Items[0].ID, gestor.Items[1].ID, gestor.Items[2].ID})

	mine, err := f.uc.ApprovalQueue(ctx, f.actor(f.vendedorA, domain.RoleVendedor), usecases.ApprovalQueueOptions{}, 50, 0)
	require.NoError(t, err)
	ids := make([]string, 0, len(mine.Items))
	for _, l := range mine.Items {
		ids = append(ids, l.ID)
	}
	assert.Contains(t, ids, f.leadOfA)
	assert.NotContains(t, ids, f.privateB, "a teammate's private draft is not approvable")

	// An attendant has no business approving outbound sales.
	_, err = f.uc.ApprovalQueue(ctx, f.actor(f.gestor, domain.RoleAttendant), usecases.ApprovalQueueOptions{}, 50, 0)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// The filter and the ordering have to reach the SQL. A funnel narrow applied in
// Go over an already-paginated page shows the operator whichever of their 40
// drafts happened to land on page one, and calls that the funnel.
func TestApprovalQueue_FiltersAndOrdersInSQLWithCountsOverTheWholeQueue(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()

	otherFunnel := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Outro funil', NOW(), NOW())`, otherFunnel, f.accountID)
	require.NoError(t, err)
	otherLead := seedRBACLead(t, f.accountID, otherFunnel,
		seedIdentityContact(t, f.accountID, "+5511900000021"), &f.vendedorB, "team")

	for id, ago := range map[string]string{f.leadOfA: "3 hours", otherLead: "10 minutes"} {
		_, err = testPool.Exec(ctx,
			`UPDATE leads SET status = 'pending_approval', updated_at = NOW() - $2::interval WHERE id = $1`,
			id, ago)
		require.NoError(t, err)
	}

	actor := f.actor(f.gestor, domain.RoleGestor)

	// Narrowing to the other funnel returns ONLY its draft — and the counts
	// still report both funnels, because they are the map of where the work is.
	narrowed, err := f.uc.ApprovalQueue(ctx, actor,
		usecases.ApprovalQueueOptions{FunnelID: otherFunnel}, 50, 0)
	require.NoError(t, err)
	require.Len(t, narrowed.Items, 1)
	assert.Equal(t, otherLead, narrowed.Items[0].ID)
	assert.Equal(t, int64(2), narrowed.Total,
		"the total counts the whole queue, not the selected funnel")

	byFunnel := map[string]int64{}
	names := map[string]string{}
	for _, c := range narrowed.ByFunnel {
		byFunnel[c.FunnelID] = c.Total
		names[c.FunnelID] = c.FunnelName
	}
	assert.Equal(t, int64(1), byFunnel[otherFunnel])
	assert.Equal(t, int64(1), byFunnel[f.funnelID],
		"the unselected funnel keeps its chip, or picking one erases the map")
	assert.Equal(t, "Outro funil", names[otherFunnel])

	// Newest first is a different order, not a different set.
	newest, err := f.uc.ApprovalQueue(ctx, actor,
		usecases.ApprovalQueueOptions{NewestFirst: true}, 50, 0)
	require.NoError(t, err)
	require.Len(t, newest.Items, 2)
	assert.Equal(t, []string{otherLead, f.leadOfA},
		[]string{newest.Items[0].ID, newest.Items[1].ID})

	// A vendedor's counts obey the same scope as their list: a chip that
	// promises a draft the board would refuse to show is worse than no chip.
	mine, err := f.uc.ApprovalQueue(ctx, f.actor(f.vendedorA, domain.RoleVendedor),
		usecases.ApprovalQueueOptions{}, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(len(mine.Items)), mine.Total)

	// The narrow reaches SQL, so it survives a page that cannot hold the queue:
	// asking for one row of the other funnel returns that funnel's row, never
	// the first row of the whole queue.
	paged, err := f.uc.ApprovalQueue(ctx, actor,
		usecases.ApprovalQueueOptions{FunnelID: otherFunnel}, 1, 0)
	require.NoError(t, err)
	require.Len(t, paged.Items, 1)
	assert.Equal(t, otherLead, paged.Items[0].ID)
	assert.Equal(t, int64(2), paged.Total)
}

// US-M023. The command centre's tiles are COUNTS, not samples: a total
// derived from the first page of a list quietly shrinks as the customer
// succeeds, which is exactly when they would stop trusting the screen.
func TestPipelineOverview_CountsWholePipelineAndRespectsScope(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx,
		`UPDATE leads SET status = 'pending_approval' WHERE id = $1`, f.leadOfA)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx,
		`UPDATE leads SET status = 'won' WHERE id = $1`, f.leadOfB)
	require.NoError(t, err)

	gestor, err := f.uc.Overview(ctx, f.actor(f.gestor, domain.RoleGestor))
	require.NoError(t, err)
	assert.Equal(t, int64(4), gestor.Total)
	// won is not work in front of anyone.
	assert.Equal(t, int64(3), gestor.Active)
	assert.Equal(t, int64(1), gestor.PendingApproval)
	require.NotNil(t, gestor.Unowned)
	// The fixture's one unassigned lead; the won/lost ones never count.
	assert.Equal(t, int64(1), *gestor.Unowned)

	// A vendedor's tiles must not total more than their board shows: the
	// colleague's PRIVATE lead is invisible to them here too.
	mine, err := f.uc.Overview(ctx, f.actor(f.vendedorA, domain.RoleVendedor))
	require.NoError(t, err)
	assert.Equal(t, int64(3), mine.Total)
	// ...and they get no unassigned nag, because they cannot assign.
	assert.Nil(t, mine.Unowned)

	_, err = f.uc.Overview(ctx, f.actor(f.gestor, domain.RoleAttendant))
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// US-M024. The inbox chip is only useful if the stage it shows belongs to
// the lead the attendant is actually talking to — and if a plain support
// conversation stays plain.
func TestInbox_ConversationCarriesItsLeadContext(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()

	board, err := f.uc.Board(ctx, f.actor(f.gestor, domain.RoleGestor), f.funnelID, 50, 0)
	require.NoError(t, err)
	require.NotEmpty(t, board.Stages)
	stage := board.Stages[0]

	// Put one lead in a known column; leave the rest alone.
	_, err = testPool.Exec(ctx,
		`UPDATE leads SET stage_id = $2 WHERE id = $1`, f.leadOfA, stage.ID)
	require.NoError(t, err)

	// A conversation with that lead's contact, plus one with a contact
	// that is nobody's lead.
	channelID := seedInboxChannel(t, f.accountID)
	leadContact := leadContactID(t, f.leadOfA)
	convWithLead := seedInboxConversation(t, f.accountID, channelID, leadContact)
	convWithout := seedInboxConversation(t, f.accountID, channelID, "+5511900009999")

	repo := postgres.NewConversationRepository(testPool)
	summaries, err := repo.ListEnriched(ctx, f.accountID, "open", 50, 0, "", "", "")
	require.NoError(t, err)

	byID := map[string]*domain.ConversationSummary{}
	for _, s := range summaries {
		byID[s.ID] = s
	}

	withLead := byID[convWithLead]
	require.NotNil(t, withLead)
	assert.Equal(t, f.leadOfA, withLead.LeadID)
	assert.Equal(t, stage.Name, withLead.LeadStageName)

	plain := byID[convWithout]
	require.NotNil(t, plain)
	assert.Empty(t, plain.LeadID, "a contact that is nobody's lead gets no chip")
	assert.Empty(t, plain.LeadStageName)
}

// A lead created before the board has no stage. The status is the
// fallback, because a chip reading the canonical status is still more
// useful than no chip at all.
func TestInbox_LeadWithoutAStageFallsBackToItsStatus(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx,
		`UPDATE leads SET stage_id = NULL, status = 'qualified' WHERE id = $1`, f.leadOfA)
	require.NoError(t, err)

	channelID := seedInboxChannel(t, f.accountID)
	convID := seedInboxConversation(t, f.accountID, channelID, leadContactID(t, f.leadOfA))

	summaries, err := postgres.NewConversationRepository(testPool).
		ListEnriched(ctx, f.accountID, "open", 50, 0, "", "", "")
	require.NoError(t, err)

	for _, s := range summaries {
		if s.ID != convID {
			continue
		}
		assert.Empty(t, s.LeadStageName)
		assert.Equal(t, "qualified", s.LeadStatus)
		return
	}
	t.Fatal("seeded conversation not returned")
}

func leadContactID(t *testing.T, leadID string) string {
	t.Helper()
	var contactID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT contact_id FROM leads WHERE id = $1`, leadID).Scan(&contactID))
	return contactID
}

func seedInboxChannel(t *testing.T, accountID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO channels (id, account_id, name, provider, credentials, webhook_token, is_active, created_at, updated_at)
		 VALUES ($1, $2, 'Inbox', 'whatsapp', '{}'::jsonb, $3, TRUE, NOW(), NOW())`, id, accountID, id)
	require.NoError(t, err)
	return id
}

func seedInboxConversation(t *testing.T, accountID, channelID, contactID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		// Production shape: contact_id is what the channel wrote; the FK to
		// the person lives in contact_ref_id (set only when the id IS a
		// contact of this account). The inbox joins on the FK.
		`INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_ref_id, status, is_bot_paused, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, (SELECT c.id FROM contacts c WHERE c.id = $5 AND c.account_id = $6), 'open', FALSE, NOW(), NOW())`,
		id, accountID, channelID, contactID, contactID, accountID)
	require.NoError(t, err)
	return id
}

// Manual ordering (Clientes → Pipeline). A drop between two neighbors must
// persist a fractional rank the next board read still agrees with — and a
// same-stage reorder must be ONLY a reorder: same status, same idle clock.
func TestKanban_ReorderPersistsBoardPosAcrossReads(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()
	gestor := f.actor(f.gestor, domain.RoleGestor)

	board, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	target := board.Stages[1] // "Prospectados" — forward from the fixture's 'new'

	// A plain drop (no neighbors) keeps NO manual position: the card stays in
	// the automatic order.
	moved, err := f.uc.MoveLead(ctx, gestor, f.leadOfA, target.ID, "", "")
	require.NoError(t, err)
	assert.Nil(t, moved.BoardPos)

	// Dropped "below leadOfA", which was never placed. Naming an unplaced
	// neighbor SEEDS the column (the bug: treated as absent, the dropped
	// card became the only placed one and the placed-first rule threw it to
	// the TOP wherever the slot was). After the seed, leadOfA carries a
	// concrete position and leadOfB lands right below it.
	movedB, err := f.uc.MoveLead(ctx, gestor, f.leadOfB, target.ID, f.leadOfA, "")
	require.NoError(t, err)
	require.NotNil(t, movedB.BoardPos)
	var posA float64
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT board_pos FROM leads WHERE id = $1`, f.leadOfA).Scan(&posA))
	assert.Greater(t, *movedB.BoardPos, posA, "below means BELOW, not the top")

	// Dropped into the slot ABOVE leadOfB (between A and B, as the client
	// names it): lands exactly between the two.
	movedU, err := f.uc.MoveLead(ctx, gestor, f.unowned, target.ID, f.leadOfA, f.leadOfB)
	require.NoError(t, err)
	require.NotNil(t, movedU.BoardPos)
	assert.Greater(t, *movedU.BoardPos, posA)
	assert.Less(t, *movedU.BoardPos, *movedB.BoardPos)

	// Ordered by the persisted board_pos (the client sorts by it), the whole
	// column is A, unowned, B — the sequence of drop slots the person saw.
	board, err = f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	type placedCard struct {
		id  string
		pos float64
	}
	var col []placedCard
	for _, c := range board.Cards {
		if c.StageID == target.ID {
			require.NotNil(t, c.Lead.BoardPos, "a manually ordered column has a position on every card")
			col = append(col, placedCard{c.Lead.ID, *c.Lead.BoardPos})
		}
	}
	require.Len(t, col, 3)
	sort.Slice(col, func(i, j int) bool { return col[i].pos < col[j].pos })
	assert.Equal(t,
		[]string{f.leadOfA, f.unowned, f.leadOfB},
		[]string{col[0].id, col[1].id, col[2].id},
		"the drop slot the person saw is the order the board keeps")

	// Same-stage reorder: leadOfA moves between unowned and leadOfB —
	// without touching status or updated_at.
	var statusBefore string
	var updatedBefore time.Time
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT status, updated_at FROM leads WHERE id = $1`, f.leadOfA).
		Scan(&statusBefore, &updatedBefore))

	movedA, err := f.uc.MoveLead(ctx, gestor, f.leadOfA, target.ID, f.unowned, f.leadOfB)
	require.NoError(t, err)
	require.NotNil(t, movedA.BoardPos)
	assert.Greater(t, *movedA.BoardPos, *movedU.BoardPos)
	assert.Less(t, *movedA.BoardPos, *movedB.BoardPos)
	assert.Equal(t, domain.LeadStatus(statusBefore), movedA.Status,
		"a reorder never rewrites the pipeline")
	assert.True(t, movedA.UpdatedAt.Equal(updatedBefore),
		"a reorder must not reset the 'parado há N dias' clock")

	// The next board read carries the positions, and sorting placed-first by
	// board_pos yields the order the finger chose.
	after, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	pos := map[string]*float64{}
	for _, c := range after.Cards {
		if c.StageID == target.ID {
			pos[c.Lead.ID] = c.Lead.BoardPos
		}
	}
	require.NotNil(t, pos[f.unowned])
	require.NotNil(t, pos[f.leadOfA])
	require.NotNil(t, pos[f.leadOfB])
	assert.Less(t, *pos[f.unowned], *pos[f.leadOfA])
	assert.Less(t, *pos[f.leadOfA], *pos[f.leadOfB])
}

// "Ordenar por parados": the reset clears ONE column's manual order and
// nothing else's.
func TestKanban_ResetBoardOrderClearsOnlyThatColumn(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()
	gestor := f.actor(f.gestor, domain.RoleGestor)

	board, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	target, other := board.Stages[1], board.Stages[2]

	_, err = f.uc.MoveLead(ctx, gestor, f.leadOfA, target.ID, "", "")
	require.NoError(t, err)
	_, err = f.uc.MoveLead(ctx, gestor, f.leadOfB, target.ID, f.leadOfA, "")
	require.NoError(t, err)
	otherMoved, err := f.uc.MoveLead(ctx, gestor, f.unowned, other.ID, f.leadOfB, "")
	require.NoError(t, err)
	require.NotNil(t, otherMoved.BoardPos)

	require.NoError(t, f.uc.ResetBoardOrder(ctx, gestor, f.funnelID, target.ID))

	var cleared *float64
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT board_pos FROM leads WHERE id = $1`, f.leadOfB).Scan(&cleared))
	assert.Nil(t, cleared, "the reset column went back to the automatic order")

	var kept *float64
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT board_pos FROM leads WHERE id = $1`, f.unowned).Scan(&kept))
	assert.NotNil(t, kept, "another column's manual order survives")
}

// A re-import must not shuffle the board: UpsertLead's conflict branch has to
// leave board_pos alone.
func TestKanban_UpsertLeadPreservesBoardPos(t *testing.T) {
	f, _ := newBoardFixture(t)
	ctx := context.Background()
	gestor := f.actor(f.gestor, domain.RoleGestor)

	board, err := f.uc.Board(ctx, gestor, f.funnelID, 50, 0)
	require.NoError(t, err)
	target := board.Stages[1]

	moved, err := f.uc.MoveLead(ctx, gestor, f.leadOfA, target.ID, "", f.leadOfB)
	require.NoError(t, err)
	require.NotNil(t, moved.BoardPos)

	var contactID string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT contact_id FROM leads WHERE id = $1`, f.leadOfA).Scan(&contactID))

	// The exact path every import takes: same (funnel, contact), fresh data.
	err = postgres.NewLeadRepository(testPool).Upsert(ctx, &domain.Lead{
		ID:        ulid.Make().String(),
		AccountID: f.accountID,
		FunnelID:  f.funnelID,
		ContactID: contactID,
		Status:    moved.Status,
		Source:    "import",
	})
	require.NoError(t, err)

	var pos *float64
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT board_pos FROM leads WHERE id = $1`, f.leadOfA).Scan(&pos))
	require.NotNil(t, pos, "an import clobbering the manual order is the bug this pins")
	assert.InDelta(t, *moved.BoardPos, *pos, 1e-9)
}
