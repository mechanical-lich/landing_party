package game

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// A razed autonz settlement stays razed across a save and a thaw: an empty
// Built is "nothing standing", not "unknown" (review finding 3: every
// structure came back at once on reload or thaw).
func TestAutonzRazedStaysRazed(t *testing.T) {
	ms, loc, c, p := autonzHubSite(t, "autonz_relay_pattern", "autonz_foundry_pattern")
	d := autonzDoc(t)
	cx, cy, cz, ok := originOf(ms.level, p.ID)
	if !ok {
		t.Fatal("no centre")
	}
	for _, k := range autonzStructureKeys(d) {
		for _, e := range hiveStructures(ms.level, k, cx, cy, cz, autonzRadius(d)+2) {
			ms.level.RemoveEntity(e)
		}
	}
	reconcileFactions(c, loc, ms.level)
	p = loc.PresenceByID("fpa")
	if p.Built == nil || len(p.Built) != 0 {
		t.Fatalf("razed settlement reconciled to Built %v, want an empty map", p.Built)
	}
	// Saved and loaded.
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var back campaign.FactionPresence
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Built == nil {
		t.Fatalf("a save lost the razed state: %s", raw)
	}
	*p = back
	materialiseFactions(c, loc, ms.level, ms.scenario)
	if got := autonzBuilt(ms.level, d, cx, cy, cz); len(got) != 0 {
		t.Errorf("thaw rebuilt a razed settlement: %v", got)
	}
}
