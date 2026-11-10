package campaign

import "testing"

// Tier 1 rebuilds a razed settlement one structure a day; unknown (nil)
// takes the stage's set (review finding 3).
func TestTier1Rebuild(t *testing.T) {
	want := map[string]int{"cogitator": 1, "foundry": 1, "relay_pylon": 2}
	got := tier1Rebuild(map[string]int{}, want)
	if len(got) != 1 || got["cogitator"] != 1 {
		t.Errorf("day 1 from razed: %v, want one cogitator", got)
	}
	for day := 0; day < 10; day++ {
		got = tier1Rebuild(got, want)
	}
	if len(got) != 3 || got["relay_pylon"] != 2 {
		t.Errorf("after 11 days: %v, want %v", got, want)
	}
	if got := tier1Rebuild(nil, want); len(got) != 3 {
		t.Errorf("unknown: %v", got)
	}
	if got := tier1Rebuild(map[string]int{"cogitator": 1, "old": 3}, map[string]int{"cogitator": 1}); len(got) != 1 {
		t.Errorf("a structure the stage no longer holds stays: %v", got)
	}
}
