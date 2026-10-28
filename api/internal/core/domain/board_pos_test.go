package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func fp(v float64) *float64 { return &v }

// ComputeBoardPos is the whole ranking algebra: every drop slot must map to
// a number that sorts the card exactly where the finger let go — and a
// never-placed neighbor must behave like an absent one, because the placed
// set is the only order the server actually keeps.
func TestComputeBoardPos(t *testing.T) {
	cases := []struct {
		name               string
		before, after, max *float64
		want               float64
	}{
		{"between two placed cards → midpoint", fp(1024), fp(2048), fp(4096), 1536},
		{"midpoint keeps halving", fp(1024), fp(1536), fp(4096), 1280},
		{"below the last placed card", fp(4096), nil, fp(4096), 4096 + BoardPositionStep},
		{"above the first placed card", nil, fp(1024), fp(4096), 1024 - BoardPositionStep},
		{"negative positions stay legal at the top", nil, fp(-2048), fp(0), -2048 - BoardPositionStep},
		{"among never-placed cards → bottom of the placed set", nil, nil, fp(512), 512 + BoardPositionStep},
		{"first manual position in an unplaced column", nil, nil, nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.InDelta(t, tc.want, ComputeBoardPos(tc.before, tc.after, tc.max), 1e-9)
		})
	}
}
