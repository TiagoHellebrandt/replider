package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// --- ConversationRepository ---

type ConversationRepository struct {
	queries *Queries
	// db is kept for the handful of statements that are not worth a sqlc
	// round trip — see SetInboxCategory.
	db DBTX
}

func NewConversationRepository(db DBTX) *ConversationRepository {
	return &ConversationRepository{queries: New(db), db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, conv *domain.Conversation) error {
	q := getQueries(ctx, r.queries)

	var lastMsg pgtype.Timestamptz
	if conv.LastCustomerMessageAt != nil {
		lastMsg = pgtype.Timestamptz{Time: *conv.LastCustomerMessageAt, Valid: true}
	}
	var waitSince pgtype.Timestamptz
	if conv.WaitingSince != nil {
		waitSince = pgtype.Timestamptz{Time: *conv.WaitingSince, Valid: true}
	}

	err := q.CreateConversation(ctx, CreateConversationParams{
		ID:                    conv.ID,
		AccountID:             conv.AccountID,
		ChannelID:             conv.ChannelID,
		AgentID:               conv.AgentID,
		ContactID:             conv.ContactID,
		ContactRefID:          conv.ContactRefID,
		ContactName:           conv.ContactName,
		ContactUsername:       conv.ContactUsername,
		ContactAvatarUrl:      conv.ContactAvatarURL,
		ContactBio:            conv.ContactBio,
		Status:                string(conv.Status),
		IsBotPaused:           conv.IsBotPaused,
		LastCustomerMessageAt: lastMsg,
		WaitingSince:          waitSince,
	})
	// Translate the partial-unique-index violation into a domain
	// sentinel so SaveInbound can recover (re-fetch the winning row)
	// instead of bubbling a generic 5xx.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return domain.ErrConversationAlreadyOpen
	}
	return err
}

func (r *ConversationRepository) GetByID(ctx context.Context, id string, accountID string) (*domain.Conversation, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetConversationByID(ctx, GetConversationByIDParams{ID: id, AccountID: accountID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapConversationToDomain(row), nil
}

func (r *ConversationRepository) Update(ctx context.Context, conv *domain.Conversation) error {
	q := getQueries(ctx, r.queries)

	var firstResp pgtype.Timestamptz
	if conv.FirstResponseAt != nil {
		firstResp = pgtype.Timestamptz{Time: *conv.FirstResponseAt, Valid: true}
	}
	var resolved pgtype.Timestamptz
	if conv.ResolvedAt != nil {
		resolved = pgtype.Timestamptz{Time: *conv.ResolvedAt, Valid: true}
	}

	return q.UpdateConversation(ctx, UpdateConversationParams{
		ID:              conv.ID,
		Status:          string(conv.Status),
		IsBotPaused:     conv.IsBotPaused,
		AgentID:         conv.AgentID,
		AssignedTo:      conv.AssignedTo,
		Priority:        &conv.Priority,
		FirstResponseAt: firstResp,
		ResolvedAt:      resolved,
		AccountID:       conv.AccountID,
	})
}

// UpdateSummary persists the rolling digest of aged-out turns (US-024).
func (r *ConversationRepository) UpdateSummary(ctx context.Context, conversationID, accountID, summary string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateConversationSummary(ctx, UpdateConversationSummaryParams{
		ID:        conversationID,
		AccountID: accountID,
		Summary:   &summary,
	})
}

func (r *ConversationRepository) IncrementCost(ctx context.Context, conversationID, accountID string, costUSD float64) error {
	q := getQueries(ctx, r.queries)
	return q.IncrementConversationCost(ctx, IncrementConversationCostParams{
		ID:           conversationID,
		AccountID:    accountID,
		TotalCostUsd: costUSD,
	})
}

func (r *ConversationRepository) AssignConversation(ctx context.Context, conversationID, accountID string, assignedTo *string) error {
	q := getQueries(ctx, r.queries)
	return q.AssignConversation(ctx, AssignConversationParams{
		ID:         conversationID,
		AssignedTo: assignedTo,
		AccountID:  accountID,
	})
}

func (r *ConversationRepository) SetFirstInboundAt(ctx context.Context, conversationID, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.SetFirstInboundAt(ctx, SetFirstInboundAtParams{
		ID:        conversationID,
		AccountID: accountID,
	})
}

func (r *ConversationRepository) SetLanguage(ctx context.Context, conversationID, lang string) error {
	q := getQueries(ctx, r.queries)
	return q.SetConversationLanguage(ctx, SetConversationLanguageParams{
		ID:       conversationID,
		Language: &lang,
	})
}

func (r *ConversationRepository) SetSentiment(ctx context.Context, conversationID, accountID, sentiment string, score float32) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateConversationSentiment(ctx, UpdateConversationSentimentParams{
		ID:             conversationID,
		AccountID:      accountID,
		Sentiment:      sentiment,
		SentimentScore: score,
	})
}

func (r *ConversationRepository) SetFirstResponse(ctx context.Context, conversationID, accountID, by string) error {
	q := getQueries(ctx, r.queries)
	return q.SetFirstResponse(ctx, SetFirstResponseParams{
		ID:              conversationID,
		AccountID:       accountID,
		FirstResponseBy: &by,
	})
}

func (r *ConversationRepository) AssumeConversation(ctx context.Context, conversationID, accountID, userID string) error {
	q := getQueries(ctx, r.queries)
	uid := userID
	return q.AssumeConversation(ctx, AssumeConversationParams{
		ID:         conversationID,
		AssignedTo: &uid,
		AccountID:  accountID,
	})
}

func (r *ConversationRepository) ReleaseConversation(ctx context.Context, conversationID, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.ReleaseConversation(ctx, ReleaseConversationParams{
		ID:        conversationID,
		AccountID: accountID,
	})
}

func (r *ConversationRepository) GetAssignment(ctx context.Context, conversationID, accountID string) (*domain.ConversationAssignment, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetConversationAssignment(ctx, GetConversationAssignmentParams{
		ID:        conversationID,
		AccountID: accountID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a := &domain.ConversationAssignment{
		ConversationID: row.ConversationID,
		AssignedTo:     row.AssignedTo,
		AssignedToName: row.AssignedUserName,
		IsBotPaused:    row.IsBotPaused,
	}
	if row.AssignedAt.Valid {
		t := row.AssignedAt.Time
		a.AssignedAt = &t
	}
	return a, nil
}

func (r *ConversationRepository) GetActiveAttendants(ctx context.Context, accountID string) ([]string, error) {
	q := getQueries(ctx, r.queries)
	return q.GetActiveAttendants(ctx, accountID)
}

func (r *ConversationRepository) ListByAccountID(ctx context.Context, accountID string, status string) ([]*domain.Conversation, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListConversationsByAccountAndStatus(ctx, ListConversationsByAccountAndStatusParams{
		AccountID: accountID,
		Status:    status,
	})
	if err != nil {
		return nil, err
	}
	convs := make([]*domain.Conversation, len(rows))
	for i, row := range rows {
		convs[i] = mapConversationToDomain(row)
	}
	return convs, nil
}

func (r *ConversationRepository) ListEnriched(ctx context.Context, accountID string, status string, limit, offset int32, sort string, assignedTo string, channelType string) ([]*domain.ConversationSummary, error) {
	return r.listEnriched(ctx, accountID, status, limit, offset, sort, assignedTo, channelType, "", "", "")
}

// ListEnrichedFiltered adds the inbox category (§6.10 camada 3).
//
// A separate entry point rather than an eighth positional parameter on the
// interface: ConversationRepository is implemented by dozens of test mocks,
// and widening it for one screen's filter would rewrite all of them.
func (r *ConversationRepository) ListEnrichedFiltered(ctx context.Context, accountID string, status string, limit, offset int32, sort string, assignedTo string, channelType string, category string) ([]*domain.ConversationSummary, error) {
	return r.ListEnrichedQuery(ctx, domain.ConversationListFilter{
		AccountID: accountID, Status: status, Limit: limit, Offset: offset,
		Sort: sort, AssignedTo: assignedTo, ChannelType: channelType, Category: category,
	})
}

// escapeLike neutralises the ILIKE metacharacters in a user-typed search
// so "100%" looks for a literal percent sign. Backslash is Postgres's
// default ESCAPE character, so it has to be doubled first.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// ListEnrichedQuery is the full inbox filter set in one call (the
// domain.ConversationLister port). Search and priority join the other
// filters in SQL, before LIMIT/OFFSET — the handler used to apply them
// over the already-paginated page, which answered "nada encontrado" for
// a name that sat on page two.
func (r *ConversationRepository) ListEnrichedQuery(ctx context.Context, f domain.ConversationListFilter) ([]*domain.ConversationSummary, error) {
	return r.listEnriched(ctx, f.AccountID, f.Status, f.Limit, f.Offset, f.Sort, f.AssignedTo, f.ChannelType, f.Category, f.Search, f.Priority)
}

func (r *ConversationRepository) listEnriched(ctx context.Context, accountID string, status string, limit, offset int32, sort string, assignedTo string, channelType string, category string, search string, priority string) ([]*domain.ConversationSummary, error) {
	q := getQueries(ctx, r.queries)
	var searchFilter *string
	if s := strings.TrimSpace(search); s != "" {
		escaped := escapeLike(s)
		searchFilter = &escaped
	}
	var priorityFilter *string
	if priority != "" {
		priorityFilter = &priority
	}
	// Optional assignment filter — applied in SQL BEFORE LIMIT/OFFSET so
	// an attendant's conversations are never hidden behind pagination
	// (empty string = no filter).
	var assignedFilter *string
	if assignedTo != "" {
		assignedFilter = &assignedTo
	}
	// Same rule for the channel: a Go-side filter over an already-paginated
	// page would hide every email thread ranking past the limit, and the
	// inbox would report "no email" for an account full of it.
	var channelFilter *string
	if channelType != "" {
		channelFilter = &channelType
	}
	// Likewise the triage category: a page filtered in Go would answer
	// "nenhum lead quente" for an inbox that has several, just further down.
	var categoryFilter *string
	if category != "" {
		categoryFilter = &category
	}
	// 3 sqlc-generated row types share identical fields; Go's
	// struct-conversion rules let us cast the alternates into the
	// canonical ListConversationsEnrichedRow so the summary builder
	// below stays a single loop.
	var rows []ListConversationsEnrichedRow
	var err error
	switch sort {
	case "first_in_conv":
		alt, e := q.ListConversationsEnrichedByFirstInConv(ctx, ListConversationsEnrichedByFirstInConvParams{
			AccountID:     accountID,
			Status:        status,
			Limit:         limit,
			Offset:        offset,
			AssignedTo:    assignedFilter,
			ChannelType:   channelFilter,
			InboxCategory: categoryFilter,
			Q:             searchFilter,
			Priority:      priorityFilter,
		})
		err = e
		rows = make([]ListConversationsEnrichedRow, len(alt))
		for i, r := range alt {
			rows[i] = ListConversationsEnrichedRow(r)
		}
	case "oldest":
		alt, e := q.ListConversationsEnrichedByOldest(ctx, ListConversationsEnrichedByOldestParams{
			AccountID:     accountID,
			Status:        status,
			Limit:         limit,
			Offset:        offset,
			AssignedTo:    assignedFilter,
			ChannelType:   channelFilter,
			InboxCategory: categoryFilter,
			Q:             searchFilter,
			Priority:      priorityFilter,
		})
		err = e
		rows = make([]ListConversationsEnrichedRow, len(alt))
		for i, r := range alt {
			rows[i] = ListConversationsEnrichedRow(r)
		}
	case "importance":
		// Triage order: a lead who answered outranks a newsletter that
		// arrived a minute ago.
		alt, e := q.ListConversationsEnrichedByImportance(ctx, ListConversationsEnrichedByImportanceParams{
			AccountID:     accountID,
			Status:        status,
			Limit:         limit,
			Offset:        offset,
			AssignedTo:    assignedFilter,
			ChannelType:   channelFilter,
			InboxCategory: categoryFilter,
			Q:             searchFilter,
			Priority:      priorityFilter,
		})
		err = e
		rows = make([]ListConversationsEnrichedRow, len(alt))
		for i, r := range alt {
			rows[i] = ListConversationsEnrichedRow(r)
		}
	default:
		rows, err = q.ListConversationsEnriched(ctx, ListConversationsEnrichedParams{
			AccountID:     accountID,
			Status:        status,
			Limit:         limit,
			Offset:        offset,
			AssignedTo:    assignedFilter,
			ChannelType:   channelFilter,
			InboxCategory: categoryFilter,
			Q:             searchFilter,
			Priority:      priorityFilter,
		})
	}
	if err != nil {
		return nil, err
	}
	summaries := make([]*domain.ConversationSummary, len(rows))
	for i, row := range rows {
		s := &domain.ConversationSummary{
			ID:               row.ID,
			AccountID:        row.AccountID,
			ChannelID:        row.ChannelID,
			AgentID:          row.AgentID,
			ContactID:        row.ContactID,
			ContactName:      row.ContactName,
			ContactUsername:  row.ContactUsername,
			ContactAvatarURL: row.ContactAvatarUrl,
			ContactBio:       row.ContactBio,
			Status:           domain.ConversationStatus(row.Status),
			IsBotPaused:      row.IsBotPaused,
			AssignedTo:       row.AssignedTo,
			AssignedToName:   row.AssignedUserName,
			CreatedAt:        row.CreatedAt.Time,
			UpdatedAt:        row.UpdatedAt.Time,
			ChannelType:      row.ChannelType,
			LastMessage:      row.LastMessage,
			LastMessageType:  row.LastMessageType,
			LeadID:           row.LeadID,
			LeadStageName:    row.LeadStageName,
			LeadStatus:       row.LeadStatus,
			Sentiment:        row.Sentiment,
			SentimentScore:   row.SentimentScore,
			InboxImportance:  int(row.InboxImportance),
			UnreadCount:      int(row.UnreadCount),
		}
		if row.InboxCategory != nil {
			s.InboxCategory = *row.InboxCategory
		}
		if row.Priority != nil {
			s.Priority = *row.Priority
		}
		if row.AssignedAt.Valid {
			t := row.AssignedAt.Time
			s.AssignedAt = &t
		}
		if row.LastCustomerMessageAt.Valid {
			t := row.LastCustomerMessageAt.Time
			s.LastCustomerMessageAt = &t
		}
		if row.WaitingSince.Valid {
			t := row.WaitingSince.Time
			s.WaitingSince = &t
		}
		summaries[i] = s
	}

	// Enrich with tags in a single round-trip. The dashboard list is
	// paginated so the IN-list stays small; a batch SELECT beats
	// per-row ListTagsByConversation by orders of magnitude.
	if len(summaries) > 0 {
		ids := make([]string, len(summaries))
		for i, s := range summaries {
			ids[i] = s.ID
		}
		tagRows, err := q.ListTagsByConversationIDs(ctx, ids)
		if err == nil {
			byConv := make(map[string][]*domain.Tag, len(summaries))
			for _, row := range tagRows {
				byConv[row.ConversationID] = append(byConv[row.ConversationID], &domain.Tag{
					ID:        row.ID,
					AccountID: row.AccountID,
					Name:      row.Name,
					Color:     row.Color,
					CreatedAt: row.CreatedAt.Time,
				})
			}
			for _, s := range summaries {
				if tags, ok := byConv[s.ID]; ok {
					s.Tags = tags
				}
			}
		}
	}
	return summaries, nil
}

// RefreshContactAvatar stores a freshly-fetched avatar URL and bio
// for the conversation and bumps contact_avatar_fetched_at to NOW()
// so the next staleness check waits the full window. Best-effort:
// callers log-and-continue on error since the inbound message
// already saved.
func (r *ConversationRepository) RefreshContactAvatar(ctx context.Context, conversationID, accountID, url, bio string) error {
	q := getQueries(ctx, r.queries)
	var u *string
	if url != "" {
		u = &url
	}
	var b *string
	if bio != "" {
		b = &bio
	}
	return q.RefreshContactAvatar(ctx, RefreshContactAvatarParams{
		ID:               conversationID,
		AccountID:        accountID,
		ContactAvatarUrl: u,
		ContactBio:       b,
	})
}

func (r *ConversationRepository) SetBotPaused(ctx context.Context, conversationID string, accountID string, paused bool) error {
	q := getQueries(ctx, r.queries)
	return q.SetBotPaused(ctx, SetBotPausedParams{
		ID:          conversationID,
		IsBotPaused: paused,
		AccountID:   accountID,
	})
}

// AutoResumeStuckConversations flips is_bot_paused back to FALSE on
// conversations whose human takeover is older than `idleMinutes` and
// has had no activity since. Returns the count of rows touched.
func (r *ConversationRepository) AutoResumeStuckConversations(ctx context.Context, idleMinutes int) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.AutoResumeStuckConversations(ctx, int32(idleMinutes))
}

func (r *ConversationRepository) UpdateLastCustomerMessageAt(ctx context.Context, conversationID string, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateLastCustomerMessageAt(ctx, UpdateLastCustomerMessageAtParams{ID: conversationID, AccountID: accountID})
}

// CountWaiting backs the shell badge: open conversations whose customer is
// still waiting on a human. Concrete-only (not on the domain interface —
// its mocks are legion); the handler consumes it through a narrow port.
func (r *ConversationRepository) CountWaiting(ctx context.Context, accountID string) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.CountWaitingConversations(ctx, accountID)
}

func (r *ConversationRepository) ClearWaitingSince(ctx context.Context, conversationID string, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.ClearWaitingSince(ctx, ClearWaitingSinceParams{ID: conversationID, AccountID: accountID})
}

func mapConversationToDomain(row Conversation) *domain.Conversation {
	conv := &domain.Conversation{
		ID:               row.ID,
		AccountID:        row.AccountID,
		ChannelID:        row.ChannelID,
		AgentID:          row.AgentID,
		ContactID:        row.ContactID,
		ContactRefID:     row.ContactRefID,
		ContactName:      row.ContactName,
		ContactUsername:  row.ContactUsername,
		ContactAvatarURL: row.ContactAvatarUrl,
		ContactBio:       row.ContactBio,
		Status:           domain.ConversationStatus(row.Status),
		IsBotPaused:      row.IsBotPaused,
		AssignedTo:       row.AssignedTo,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
	if row.ContactAvatarFetchedAt.Valid {
		t := row.ContactAvatarFetchedAt.Time
		conv.ContactAvatarFetchedAt = &t
	}
	if row.Priority != nil {
		conv.Priority = *row.Priority
	}
	if row.FirstResponseAt.Valid {
		t := row.FirstResponseAt.Time
		conv.FirstResponseAt = &t
	}
	if row.ResolvedAt.Valid {
		t := row.ResolvedAt.Time
		conv.ResolvedAt = &t
	}
	if row.AssignedAt.Valid {
		t := row.AssignedAt.Time
		conv.AssignedAt = &t
	}
	if row.LastCustomerMessageAt.Valid {
		t := row.LastCustomerMessageAt.Time
		conv.LastCustomerMessageAt = &t
	}
	if row.WaitingSince.Valid {
		t := row.WaitingSince.Time
		conv.WaitingSince = &t
	}
	conv.Language = row.Language
	conv.Summary = row.Summary
	conv.CustomerAccountID = row.CustomerAccountID
	if row.SummaryUpdatedAt.Valid {
		t := row.SummaryUpdatedAt.Time
		conv.SummaryUpdatedAt = &t
	}
	return conv
}

// SetCustomerAccount stamps the customer account on an internal CS thread
// (canal-interno US-002). Write-once at the SQL level — a row that already
// carries a customer_account_id is left untouched.
func (r *ConversationRepository) SetCustomerAccount(ctx context.Context, conversationID, customerAccountID string) error {
	q := getQueries(ctx, r.queries)
	return q.SetConversationCustomerAccount(ctx, SetConversationCustomerAccountParams{
		ID:                conversationID,
		CustomerAccountID: &customerAccountID,
	})
}

func (r *ConversationRepository) FindByContact(ctx context.Context, channelID string, contactID string) (*domain.Conversation, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.FindConversationByContact(ctx, FindConversationByContactParams{
		ChannelID: channelID,
		ContactID: contactID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapConversationToDomain(row), nil
}

// FindLatestByContact is the domain.ConversationReopener lookup: the newest
// thread for the (channel, raw sender) pair regardless of status.
func (r *ConversationRepository) FindLatestByContact(ctx context.Context, channelID string, contactID string) (*domain.Conversation, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.FindLatestConversationByContact(ctx, FindLatestConversationByContactParams{
		ChannelID: channelID,
		ContactID: contactID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapConversationToDomain(row), nil
}

// Reopen flips a closed conversation back to open. false when nothing was
// closed to reopen; ErrConversationAlreadyOpen when the partial unique
// index says another thread for the same sender is already open.
func (r *ConversationRepository) Reopen(ctx context.Context, conversationID, accountID string) (bool, error) {
	q := getQueries(ctx, r.queries)
	n, err := q.ReopenConversation(ctx, ReopenConversationParams{ID: conversationID, AccountID: accountID})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return false, domain.ErrConversationAlreadyOpen
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CountUnread backs GET /conversations/unread-count (domain.UnreadCounter):
// open threads holding a customer message nobody opened, and how many such
// messages there are.
func (r *ConversationRepository) CountUnread(ctx context.Context, accountID string) (int64, int64, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.CountUnreadConversations(ctx, accountID)
	if err != nil {
		return 0, 0, err
	}
	return row.Conversations, row.Messages, nil
}

func (r *ConversationRepository) GetContactStats(ctx context.Context, accountID, contactID string) (*domain.ContactStats, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetContactStats(ctx, GetContactStatsParams{
		AccountID: accountID,
		ContactID: contactID,
	})
	if err != nil {
		return nil, err
	}
	stats := &domain.ContactStats{
		TotalConversations: row.TotalConversations,
		TotalMessages:      row.TotalMessages,
	}
	if row.FirstContactAt.Valid {
		t := row.FirstContactAt.Time
		stats.FirstContactAt = &t
	}
	if row.LastMessageAt.Valid {
		t := row.LastMessageAt.Time
		stats.LastMessageAt = &t
	}
	return stats, nil
}

func (r *ConversationRepository) ListPendingReminders(ctx context.Context, limit int) ([]*domain.ReminderCandidate, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	q := getQueries(ctx, r.queries)
	rows, err := q.ListPendingReminders(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]*domain.ReminderCandidate, 0, len(rows))
	for _, row := range rows {
		rc := &domain.ReminderCandidate{
			AccountID:        row.AccountID,
			ConversationID:   row.ConversationID,
			ChannelID:        row.ChannelID,
			ContactName:      row.ContactName,
			AssignedTo:       row.AssignedTo,
			ThresholdMinutes: int(row.ThresholdMinutes),
			SentCount:        int(row.SentCount),
		}
		if row.WaitingSince.Valid {
			rc.WaitingSince = row.WaitingSince.Time
		}
		out = append(out, rc)
	}
	return out, nil
}

func (r *ConversationRepository) MarkReminderSent(ctx context.Context, conversationID string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkReminderSent(ctx, conversationID)
}

// FindByAccountContact answers "which thread is this contact in", scoped by
// account rather than by channel.
//
// The lead panel needs it: a lead knows its contact but not which channel the
// conversation happened on, and asking it to know would make the panel depend
// on how the customer was first reached.
func (r *ConversationRepository) FindByAccountContact(ctx context.Context, accountID, contactID string) (*domain.Conversation, error) {
	// Reuses the sqlc-owned lookup rather than hand-rolling SQL: the query
	// this needs already exists per channel, so the account-wide form goes
	// through the same generated layer.
	rows, err := getQueries(ctx, r.queries).FindConversationByAccountContact(ctx,
		FindConversationByAccountContactParams{AccountID: accountID, ContactRefID: &contactID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &domain.Conversation{ID: rows}, nil
}

// SetInboxCategory files a conversation into its triage bucket (§6.10).
//
// Raw SQL on the sqlc repo, deliberately: the column is written by one
// caller and read by the list queries, and a sqlc round trip would mean
// regenerating for a two-column UPDATE.
func (r *ConversationRepository) SetInboxCategory(ctx context.Context, conversationID, category string, importance int) error {
	//nolint:gosec // G115: importance comes from a fixed table of constants (10..100), never from input.
	_, err := r.db.Exec(ctx,
		`UPDATE conversations
		    SET inbox_category = $2, inbox_importance = $3, updated_at = NOW()
		  WHERE id = $1`, conversationID, category, int16(importance))
	return err
}
