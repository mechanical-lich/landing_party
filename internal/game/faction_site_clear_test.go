package game

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// A presence first sited on a level already lived on keeps clear of what is
// there: the level's stamps survive a save (secondary finding: they were
// generation-only, so a thaw-sited presence skipped the overlap check), and
// the player's base is kept clear of too, so a stamp's ClearFootprint never
// takes the colony's structures.
func TestPresenceSitedAfterThawKeepsClear(t *testing.T) {
	useContentTables(t)
	ms, loc := factionTestSite(t, "moon", 7, 8, 1)
	d := campaign.LoadFactionData().Doctrine("hive")
	p := &loc.Factions[0]

	// The stamps survive a save.
	ms.level.StampFootprints = append(ms.level.StampFootprints, [6]int{3, 4, 5, 6, -1, -1})
	raw, err := json.Marshal(world.SaveLevel(ms.level))
	if err != nil {
		t.Fatal(err)
	}
	var sd world.SaveData
	if err := json.Unmarshal(raw, &sd); err != nil {
		t.Fatal(err)
	}
	if n := len(sd.StampFootprints); n == 0 || sd.StampFootprints[n-1] != [6]int{3, 4, 5, 6, -1, -1} {
		t.Fatalf("stamp footprints not saved: %v", sd.StampFootprints)
	}

	// Where it would site with nobody there.
	x, y, _, ok := presenceCentre(ms.level, loc, d, p)
	if !ok {
		t.Fatal("no site")
	}
	q := *p
	q.ID = "fp2" // same seed, same pick, no saved origin yet
	// The colony's locker stands there: it sites elsewhere, clear of it.
	locker, err := factory.Create("storage_locker", x, y, ms.level.GetSurfaceZ(x, y))
	if err != nil {
		t.Fatal(err)
	}
	locker.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = "Test Colony"
	ms.level.AddEntity(locker)
	loc.SettlementName = "Test Colony"
	x2, y2, _, ok := presenceCentre(ms.level, loc, d, &q)
	if !ok {
		t.Fatal("no site with the colony there")
	}
	w, h := 21, 21
	if x2-w/2 <= x && x <= x2+w/2 && y2-h/2 <= y && y <= y2+h/2 {
		t.Errorf("sited at %d,%d over the colony's locker at %d,%d", x2, y2, x, y)
	}
}
