package gui

import (
	"testing"

	"github.com/mechanical-lich/mlge/ui/minui"
)

// Closing a modal by any route takes its tooltips with it: the shared list
// tooltip hides, and the colonist panel's rows are unregistered while tooltips
// registered by anything else stay.
func TestOrphanTooltipsDroppedOnModalClose(t *testing.T) {
	h := &HUDScreen{
		tooltipManager: minui.NewTooltipManager(),
		listTooltip:    minui.NewTooltip("listTooltip"),
		colonistModal:  minui.NewModal("colonistModal", "", 10, 10),
		craftingModal:  minui.NewModal("craftingModal", "", 10, 10),
		researchModal:  minui.NewModal("researchModal", "", 10, 10),
	}
	h.colonistModal.SetVisible(false)
	h.craftingModal.SetVisible(false)
	h.researchModal.SetVisible(false)
	other := minui.NewButton("other", "x")
	row := minui.NewButton("row", "x")
	h.tooltipManager.Register(other, "Other", "", nil)
	h.tooltipManager.Register(row, "Row", "", nil)
	h.colonistTooltipTargets = []minui.Element{row}

	h.colonistModal.SetVisible(true)
	h.listTooltip.Show()
	h.dropOrphanTooltips()
	if !h.listTooltip.IsVisible() || h.tooltipManager.Count() != 2 {
		t.Fatalf("open modal lost tooltips: list %v, count %d", h.listTooltip.IsVisible(), h.tooltipManager.Count())
	}

	// The title bar X only clears the modal's visibility.
	h.colonistModal.SetVisible(false)
	h.dropOrphanTooltips()
	if h.listTooltip.IsVisible() {
		t.Error("list tooltip outlived the modal")
	}
	if h.tooltipManager.GetByID("row") != nil || h.tooltipManager.GetByID("other") == nil {
		t.Errorf("row %v, other %v", h.tooltipManager.GetByID("row"), h.tooltipManager.GetByID("other"))
	}

	// The research list shares the tooltip.
	h.researchModal.SetVisible(true)
	h.listTooltip.Show()
	h.dropOrphanTooltips()
	if !h.listTooltip.IsVisible() {
		t.Error("list tooltip hidden with the research modal open")
	}
	h.researchModal.SetVisible(false)
	h.dropOrphanTooltips()
	if h.listTooltip.IsVisible() {
		t.Error("list tooltip outlived the research modal")
	}
}
