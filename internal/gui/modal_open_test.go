package gui

import (
	"testing"

	"github.com/mechanical-lich/mlge/ui/minui"
)

// ModalOpen knows every named HUD modal, and AnyModalOpen follows them all.
func TestModalOpenKnowsEveryHUDModal(t *testing.T) {
	h := &HUDScreen{
		mainMenuModal: minui.NewModal("mainMenu", "", 10, 10),
		saveModal:     minui.NewModal("saveModal", "", 10, 10),
		loadModal:     minui.NewModal("loadModal", "", 10, 10),
		colonistModal: minui.NewModal("colonistModal", "", 10, 10),
		craftingModal: minui.NewModal("craftingModal", "", 10, 10),
		researchModal: minui.NewModal("researchModal", "", 10, 10),
	}
	byName := map[string]*minui.Modal{
		"mainMenu": h.mainMenuModal, "saveModal": h.saveModal, "loadModal": h.loadModal,
		"colonistModal": h.colonistModal, "craftingModal": h.craftingModal, "researchModal": h.researchModal,
	}
	for _, m := range byName {
		m.SetVisible(false)
	}
	if h.AnyModalOpen() {
		t.Fatal("AnyModalOpen with every modal closed")
	}
	for name, m := range byName {
		if h.ModalOpen(name) {
			t.Errorf("%s reported open while closed", name)
		}
		m.SetVisible(true)
		if !h.ModalOpen(name) {
			t.Errorf("%s reported closed while open", name)
		}
		if !h.AnyModalOpen() {
			t.Errorf("AnyModalOpen false with %s open", name)
		}
		m.SetVisible(false)
	}
	if h.ModalOpen("no_such_modal") {
		t.Error("an unknown modal reported open")
	}
	// A HUD built without one of its modals answers false, not a panic.
	if (&HUDScreen{}).ModalOpen("craftingModal") {
		t.Error("nil crafting modal reported open")
	}
}
