package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/core/domain"
)

// Conversational onboarding (§6.20, K5). Raw SQL: two JSONB columns and an
// upsert, which sqlc would express as more code than this.
type OnboardingChatRepository struct {
	db *pgxpool.Pool
}

func NewOnboardingChatRepository(db *pgxpool.Pool) *OnboardingChatRepository {
	return &OnboardingChatRepository{db: db}
}

var _ domain.OnboardingChatRepository = (*OnboardingChatRepository)(nil)

func (r *OnboardingChatRepository) Get(ctx context.Context, accountID string) (*domain.OnboardingChat, error) {
	var chat domain.OnboardingChat
	var step string
	var answers, messages []byte
	err := r.db.QueryRow(ctx,
		`SELECT step, answers, messages, started_at, first_lead_at, completed_at
		 FROM onboarding_chats WHERE account_id = $1`,
		accountID,
	).Scan(&step, &answers, &messages, &chat.StartedAt, &chat.FirstLeadAt, &chat.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	chat.AccountID = accountID
	chat.Step = domain.OnboardingStep(step)
	// A draft that fails to parse is treated as empty rather than as an error:
	// the conversation can be re-answered, and refusing to load it would lock
	// the account out of onboarding entirely.
	_ = json.Unmarshal(answers, &chat.Answers)
	_ = json.Unmarshal(messages, &chat.Messages)
	return &chat, nil
}

func (r *OnboardingChatRepository) Save(ctx context.Context, chat *domain.OnboardingChat) error {
	answers, err := json.Marshal(chat.Answers)
	if err != nil {
		return err
	}
	messages, err := json.Marshal(chat.Messages)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO onboarding_chats
		     (account_id, step, answers, messages, started_at, completed_at)
		 VALUES ($1, $2, $3, $4, COALESCE($5, NOW()), $6)
		 ON CONFLICT (account_id) DO UPDATE SET
		     step         = EXCLUDED.step,
		     answers      = EXCLUDED.answers,
		     messages     = EXCLUDED.messages,
		     completed_at = EXCLUDED.completed_at,
		     updated_at   = NOW()`,
		chat.AccountID, string(chat.Step), answers, messages,
		nullableTime(chat.StartedAt), chat.CompletedAt)
	return err
}

// StampFirstLead records the activation moment exactly once.
//
// The conditional is the idempotency: the first lead is the first lead, and a
// second write would restart the clock and make the activation metric improve
// every time an established account creates a lead.
func (r *OnboardingChatRepository) StampFirstLead(ctx context.Context, accountID string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE onboarding_chats SET first_lead_at = $2, updated_at = NOW()
		 WHERE account_id = $1 AND first_lead_at IS NULL`,
		accountID, at)
	return err
}

// nullableTime turns the zero time into a NULL so the INSERT's COALESCE picks
// NOW() for a conversation that is starting.
func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
