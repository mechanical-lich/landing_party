package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// New Expedition's rival box names each choice as every other screen does:
// the colony's rival view is "Rival Colony", not the colony's own name.
func TestRivalChoiceNames(t *testing.T) {
	seen := false
	for _, k := range campaign.RivalChoices() {
		name := rivalChoiceName(k)
		if k == campaign.RivalColonyDoctrine {
			seen = true
			if want := campaign.LoadFactionData().Doctrine(k).Name; name != want || name == "Colony" {
				t.Errorf("rival colony box reads %q, want %q", name, want)
			}
		}
		if name == "" || name == k && campaign.LoadFactionData().Doctrine(campaign.BaseDoctrine(k)) != nil {
			t.Errorf("choice %q named %q", k, name)
		}
	}
	if !seen {
		t.Errorf("no rival colony among the choices %v", campaign.RivalChoices())
	}
}
