package game

import "testing"

func TestReticleUnpaidRows_FillsBottomUp(t *testing.T) {
	cases := []struct {
		fill float64
		want int
	}{{0, 32}, {0.25, 24}, {0.5, 16}, {1, 0}, {1.5, 0}, {-1, 32}}
	for _, c := range cases {
		if got := reticleUnpaidRows(32, c.fill); got != c.want {
			t.Errorf("fill %v: %d unpaid rows, want %d", c.fill, got, c.want)
		}
	}
}
