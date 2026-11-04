package gui

import "testing"

// Cost lines list materials alphabetically, the same on every call.
func TestCostTextSorted(t *testing.T) {
	cost := map[string]int{"metal_ore": 6, "crystal": 2, "biomass": 4, "stone": 1}
	want := "biomass×4, crystal×2, metal_ore×6, stone×1"
	for i := 0; i < 50; i++ {
		if got := costText(cost); got != want {
			t.Fatalf("costText = %q, want %q", got, want)
		}
	}
	if got := costText(nil); got != "" {
		t.Errorf("empty cost = %q, want empty", got)
	}
}
