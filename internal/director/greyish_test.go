package director

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// A rung's reserve (phase E4, the greys) keeps a share of the hold target
// unspent: afford refuses a cost that would dip into it.
func TestReserveStopsSpending(t *testing.T) {
	snap := &snapshot{free: map[string]int{"metal_ore": 14, "crystal": 6}}
	cost := map[string]int{"metal_ore": 8, "crystal": 3}
	snap.reserve = 10
	if snap.afford(cost) {
		t.Error("spent into the reserve: 20 free, 11 cost, 10 kept")
	}
	snap.reserve = 9
	if !snap.afford(cost) || snap.free["metal_ore"] != 6 {
		t.Errorf("refused a cost above the reserve (free now %v)", snap.free)
	}
}

// The arm rung ranks weapons by dice average plus attack bonus.
func TestDiceAverage(t *testing.T) {
	for in, want := range map[string]float64{"2d6": 7, "1d8+2": 6.5, "3d8+3": 16.5, "1d4": 2.5, "": 0} {
		if got := diceAverage(in); got != want {
			t.Errorf("diceAverage(%q) = %v, want %v", in, got, want)
		}
	}
}

// A ship site's stay and tier gate rungs; unless_tech retires one.
func TestShipRungGates(t *testing.T) {
	p := campaign.DirectorPriority{Build: "grey_array", Min: 1, StayLeft: 12, TierMin: 1}
	gate := func(s *Site) bool {
		return p.TierMin > s.Tier || s.Ship && p.StayLeft > 0 && s.StayLeft < p.StayLeft
	}
	if !gate(&Site{Ship: true, Tier: 0, StayLeft: 30}) || !gate(&Site{Ship: true, Tier: 1, StayLeft: 11}) || gate(&Site{Ship: true, Tier: 1, StayLeft: 12}) {
		t.Error("tier_min / stay_left gate wrong")
	}
	if gate(&Site{Tier: 1}) {
		t.Error("stay_left gated a planet settlement (no ship)")
	}
}
