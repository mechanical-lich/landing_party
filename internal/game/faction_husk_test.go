package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
)

// An exhausted hive's husk takes its structures with its bodies: the heart,
// the pool, egg clusters and the evolution chamber are Inanimate (in
// StaticEntities, mostly without the presence's Kin), and survived the husk
// ownerless past the husk's footprint (secondary review note).
func TestHuskTakesTheHivesStructures(t *testing.T) {
	ms, loc := hiveTestSite(t, 7, 12, 1, 20)
	c := &campaign.Campaign{Locations: map[string]*campaign.Location{loc.ID: loc}}
	materialiseFactions(c, loc, ms.level, ms.scenario)
	d := campaign.LoadFactionData().Doctrine("hive")
	p := loc.Factions[0]
	cx, cy, cz, ok := originOf(ms.level, p.ID)
	if !ok {
		t.Fatal("no centre")
	}
	name := factionSettlementName(&p, loc.ID)
	standing := func() int {
		n := 0
		for _, k := range []string{"hive_heart", "biomass_pool", "egg_cluster", "evolution_chamber"} {
			n += len(hiveStructures(ms.level, k, cx, cy, cz, hiveRadius(d)+2))
		}
		return n
	}
	// One the director built out at its radius, beyond the husk's 11x11.
	far, err := factory.Create("egg_cluster", cx+9, cy, cz)
	if err != nil {
		t.Fatal(err)
	}
	ms.level.AddEntity(far)
	if standing() == 0 {
		t.Fatal("fixture: no hive structures stood")
	}
	h := &campaign.FactionHusk{Presence: p.ID, Faction: p.Faction, Doctrine: p.Doctrine, Morph: "crawler", Seed: p.Seed}
	stampHusk(ms.level, loc, d, h)
	if n := standing(); n != 0 {
		t.Errorf("%d hive structures survived the husk", n)
	}
	for _, e := range ms.level.StaticEntities {
		if e != nil && e.HasComponent(components.Storage) &&
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == name {
			t.Errorf("%s of the dead settlement survived the husk", e.Blueprint)
		}
	}
}
