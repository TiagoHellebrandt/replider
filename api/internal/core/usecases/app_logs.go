package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// AppLogUseCase serves the production log for remote diagnosis.
//
// Whoever reads this — over MCP — sees EVERY account, so the gate is the same as
// the operator panel's: the email list in SUPER_ADMIN_EMAILS. With no list
// configured, nobody reads: a deploy that forgot the variable must not become an
// open door onto everyone's log.
type AppLogUseCase struct {
	repo      domain.AppLogRepository
	users     domain.UserRepository
	allowlist []string
	now       func() time.Time
}

func NewAppLogUseCase(repo domain.AppLogRepository, users domain.UserRepository,
	allowlist []string) *AppLogUseCase {
	normalized := make([]string, 0, len(allowlist))
	for _, e := range allowlist {
		if v := strings.ToLower(strings.TrimSpace(e)); v != "" {
			normalized = append(normalized, v)
		}
	}
	return &AppLogUseCase{
		repo:      repo,
		users:     users,
		allowlist: normalized,
		now:       time.Now,
	}
}

func (uc *AppLogUseCase) WithClock(now func() time.Time) *AppLogUseCase {
	uc.now = now
	return uc
}

// ErrLogsForbidden is the refusal. 403, never 401: the credential is fine, the
// reach is not — and a 401 makes the app conclude the session died.
var ErrLogsForbidden = &domain.AppError{
	Code:    "LOGS_FORBIDDEN",
	Message: "somente operadores da plataforma leem o log de produção",
}

func (uc *AppLogUseCase) authorize(ctx context.Context, userID string) error {
	if len(uc.allowlist) == 0 || uc.users == nil {
		return ErrLogsForbidden
	}
	user, err := uc.users.GetByID(ctx, userID)
	if err != nil || user == nil {
		return ErrLogsForbidden
	}
	email := strings.ToLower(strings.TrimSpace(user.Email))
	for _, allowed := range uc.allowlist {
		if allowed == email {
			return nil
		}
	}
	return ErrLogsForbidden
}

// defaultWindow is how far back to look when nobody said. One hour answers "what
// is breaking right now", which is the question that brings anyone here.
const defaultWindow = time.Hour

func (uc *AppLogUseCase) Query(ctx context.Context, userID string,
	f domain.AppLogFilter) ([]*domain.AppLog, error) {
	if err := uc.authorize(ctx, userID); err != nil {
		return nil, err
	}
	if f.Since.IsZero() {
		f.Since = uc.now().Add(-defaultWindow)
	}
	return uc.repo.Query(ctx, f)
}

func (uc *AppLogUseCase) Summarize(ctx context.Context, userID string,
	since, until time.Time, limit int) ([]*domain.AppLogGroup, error) {
	if err := uc.authorize(ctx, userID); err != nil {
		return nil, err
	}
	if since.IsZero() {
		since = uc.now().Add(-defaultWindow)
	}
	if until.IsZero() {
		until = uc.now()
	}
	return uc.repo.Summarize(ctx, since, until, limit)
}
