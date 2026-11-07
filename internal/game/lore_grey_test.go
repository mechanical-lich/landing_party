package game

import "testing"

var loreContacts = []string{"greyish", "whitish", "greenish", "pinkish"}

// The salvage party is aboard around the earlier wreck or station core.
func TestLoreGreyContactAtTheOrigin(t *testing.T) {
	ms, ox, oy, _ := loreBuild(t, loreSite{Map: "abandoned_station", Scenario: "grey_contact", Seed: 59, Chapter: "salvage_expedition",
		Flags: map[string]any{"origin_civ": "Vael"}, RolledX: 0.5, RolledY: 0.5})
	near, far := loreNear(ms.level, loreContacts, ox, oy, 16)
	t.Logf("%d contacts at the origin (%d,%d)", near, ox, oy)
	if near < 6 || float64(near) != toSetupFloat(ms.level.Flags["grey_origin_contacts"]) || far != 0 {
		t.Errorf("%d contacts within 16 tiles of the origin, %d elsewhere, script says %v", near, far, ms.level.Flags["grey_origin_contacts"])
	}
}

func TestLoreGreyContactUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "abandoned_station", "grey_contact", "grey_contact_setup", 59, loreContacts)
}
