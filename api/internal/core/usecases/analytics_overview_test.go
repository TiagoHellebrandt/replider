package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestAnalyticsOverview_ClampsPeriodToAllowedWindows(t *testing.T) {
	ctx := context.Background()

	want := &domain.AnalyticsOverview{
		TotalConversations:      42,
		ResolvedByAIPct:         60.0,
		AvgFirstResponseSeconds: 15,
		PeriodDays:              30,
	}

	cases := []struct {
		name    string
		request int
		repoArg int
	}{
		// Accepted values pass through unchanged.
		{"seven days", 7, 7},
		{"thirty days", 30, 30},
		{"ninety days", 90, 90},
		// Anything else snaps to 30 — the default on the Flutter picker.
		{"zero", 0, 30},
		{"negative", -1, 30},
		{"arbitrary day count", 14, 30},
		{"huge window", 365, 30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockAnalyticsRepo)
			repo.On("GetOverview", ctx, "acc-1", tc.repoArg).Return(want, nil).Once()
			uc := NewGetAnalyticsOverviewUseCase(repo)

			got, err := uc.Execute(ctx, "acc-1", tc.request)
			assert.NoError(t, err)
			assert.Same(t, want, got)
			repo.AssertExpectations(t)
		})
	}
}

func TestParseCSATReply(t *testing.T) {
	cases := []struct {
		in     string
		wantN  int
		wantOK bool
	}{
		{"1", 1, true},
		{"5", 5, true},
		{" 3 ", 3, true},
		{"0", 0, false},
		{"6", 0, false},
		{"", 0, false},
		{"five", 0, false},
		// Strict: rejects messages that happen to start with a digit
		// so "5 estrelas obrigado" falls through to the AI instead of
		// being swallowed as a rating.
		{"5 estrelas", 0, false},
		{"ótimo 5", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseCSATReply(tc.in)
		assert.Equal(t, tc.wantOK, ok, "input=%q", tc.in)
		if ok {
			assert.Equal(t, tc.wantN, got, "input=%q", tc.in)
		}
	}
}
