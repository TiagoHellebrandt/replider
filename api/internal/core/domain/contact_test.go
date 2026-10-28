package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func TestContact_Validate_Phone(t *testing.T) {
	cases := []struct {
		name    string
		phone   *string
		wantErr bool
	}{
		// Optional: nil / empty / whitespace phones are allowed.
		{"nil phone", nil, false},
		{"empty phone", strPtr(""), false},
		{"whitespace phone", strPtr("   "), false},

		// Valid E.164.
		{"br mobile", strPtr("+5511999999999"), false},
		{"us number", strPtr("+14155552671"), false},
		{"short valid", strPtr("+12"), false},
		{"max 15 digits", strPtr("+123456789012345"), false},

		// Invalid.
		{"missing plus", strPtr("5511999999999"), true},
		{"leading zero", strPtr("+0511999999999"), true},
		{"letters", strPtr("+55ABC999"), true},
		{"spaces inside", strPtr("+55 11 99999"), true},
		{"dashes", strPtr("+55-11-99999"), true},
		{"too long 16 digits", strPtr("+1234567890123456"), true},
		{"only plus", strPtr("+"), true},
		{"plus single digit", strPtr("+5"), true},
		{"garbage", strPtr("not-a-phone"), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Contact{Phone: tc.phone}
			err := c.Validate()
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrInvalidPhone)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
