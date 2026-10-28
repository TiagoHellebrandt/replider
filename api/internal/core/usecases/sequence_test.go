package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// LEGACY_WORKERS off: Enroll refuses before reading anything (nil repo), so
// the REST route and the MCP tool both stop promising a cadence that has no
// runner.
func TestSequenceUseCase_EnrollUnavailableWhenLegacyWorkersOff(t *testing.T) {
	uc := NewSequenceUseCase(nil)
	uc.SetEnrollmentEnabled(false)

	_, err := uc.Enroll(context.Background(), "acc-1", "seq-1", "lead-1")

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "SEQUENCES_UNAVAILABLE", appErr.Code)
}
