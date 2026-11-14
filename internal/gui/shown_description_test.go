package gui

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// A placeholder description ("_draft") never reaches the research list's
// tooltip: the row's tooltip text is empty, so the tooltip is not shown. A
// written one is kept.
func TestPlaceholderResearchDescriptionNotShown(t *testing.T) {
	for in, want := range map[string]string{"_draft": "", " _draft": "", "_note": "", "": "",
		"Unlocks the scanner.": "Unlocks the scanner."} {
		if got := shownDescription(in); got != want {
			t.Errorf("shownDescription(%q) = %q, want %q", in, got, want)
		}
	}
	h := &HUDScreen{researchTechList: minui.NewListBox("researchTechList", nil), listTooltip: minui.NewTooltip("listTooltip")}
	h.populateResearchTechList([]research.Tech{
		{Key: "a", Name: "Resin Flow", Description: "_draft"},
		{Key: "b", Name: "Scanner", Description: "Unlocks the scanner."},
	}, []research.Tech{{Key: "c", Name: "Resin Surge", Description: "_draft", RequiresTech: "a"}})
	want := []string{"", "Unlocks the scanner.", ""}
	if len(h.researchTooltipDescs) != len(want) {
		t.Fatalf("tooltips %q, want %q", h.researchTooltipDescs, want)
	}
	for i := range want {
		if h.researchTooltipDescs[i] != want[i] {
			t.Errorf("row %d tooltip %q, want %q", i, h.researchTooltipDescs[i], want[i])
		}
	}
}
