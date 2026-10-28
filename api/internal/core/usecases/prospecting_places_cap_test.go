package usecases

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The per-account daily cap on Places searches (a platform cost rail).

func TestPlacesDailyCounter_RefusesPastTheCapForThatAccountOnly(t *testing.T) {
	c := newPlacesDailyCounter()
	c.now = func() time.Time { return time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC) }

	for i := 0; i < 3; i++ {
		require.NoError(t, c.take("acc-a", 3))
	}
	assert.ErrorIs(t, c.take("acc-a", 3), ErrPlacesDailyCapExceeded)
	assert.NoError(t, c.take("acc-b", 3), "another tenant's day is untouched")
}

func TestPlacesDailyCounter_ResetsAtTheUTCDayBoundary(t *testing.T) {
	c := newPlacesDailyCounter()
	now := time.Date(2026, 9, 1, 23, 59, 0, 0, time.UTC)
	c.now = func() time.Time { return now }

	require.NoError(t, c.take("acc", 1))
	assert.ErrorIs(t, c.take("acc", 1), ErrPlacesDailyCapExceeded)

	now = now.Add(2 * time.Minute)
	assert.NoError(t, c.take("acc", 1), "a new UTC day starts a new allowance")
}

func TestPlacesDailyCounter_ZeroCapAndNilCounterMeanNoCap(t *testing.T) {
	c := newPlacesDailyCounter()
	for i := 0; i < 100; i++ {
		require.NoError(t, c.take("acc", 0))
	}
	var none *placesDailyCounter
	assert.NoError(t, none.take("acc", 1), "an unwired counter never blocks a search")
}

func TestProspectingUseCase_SearchIsCountedBeforeTheProviderIsCalled(t *testing.T) {
	uc := &ProspectingUseCase{}
	uc.SetPlacesDailyCap(1)
	uc.placesQuota.now = func() time.Time { return time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC) }

	require.NoError(t, uc.placesQuota.take("acc", uc.placesDailyCap))
	// The second search of the day is refused with no provider involved:
	// uc.provider is nil and search() would panic if it got that far.
	_, err := uc.search(t.Context(), ProspectingSearchDTO{AccountID: "acc"}, "hospital", 5)
	assert.ErrorIs(t, err, ErrPlacesDailyCapExceeded)
}
