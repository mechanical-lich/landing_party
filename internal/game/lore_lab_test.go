package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// loreLevelFacts are what the parity test compares: every resolved origin and
// the lore datapads' events.
func loreLevelFacts(lvl *world.Level, events []string) (map[string][3]int, map[string]int) {
	origins := map[string][3]int{}
	for _, id := range append([]string{generation.ChapterOrigin}, events...) {
		if x, y, z, ok := generation.Origin(lvl, id); ok {
			origins[id] = [3]int{x, y, z}
		}
	}
	pads := map[string]int{}
	for _, e := range lvl.Entities {
		if e == nil || !e.HasComponent(components.Datapad) {
			continue
		}
		if dp := e.GetComponent(components.Datapad).(*components.DatapadComponent); dp.QuestID == "" && dp.EventID != "" {
			pads[dp.EventID]++
		}
	}
	return origins, pads
}

// The planet generator's lab builds a charted site the way the game does: the
// same resolved origins and the same lore datapads.
func TestLoreLabBuildsWhatTheGameBuilds(t *testing.T) {
	loreLoadData(t)
	seed, locID := loreZombieSite(t)
	c, wm, err := newExpedition("Lore", seed, false)
	if err != nil {
		t.Fatal(err)
	}
	loc := c.Locations[locID]
	site := c.SiteHistory(loc)
	if err := wm.Travel(locID); err != nil {
		t.Fatal(err)
	}
	gameOrigins, gamePads := loreLevelFacts(wm.Current().level, site.Events)

	md := mapdef.ByID(loc.MapID)
	w, h, d := md.Size.Roll(loc.Seed)
	lab, err := NewPlanetLab(PlanetLabSpec{
		Map: *md, Scenario: scenario.ByID(loc.ScenarioID),
		Width: w, Height: h, Depth: d, Seed: loc.Seed,
		History: c.History, Site: site,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lab.Close)
	labOrigins, labPads := loreLevelFacts(lab.Level(), site.Events)

	if len(gameOrigins) == 0 {
		t.Fatal("the game resolved no origins")
	}
	for id, g := range gameOrigins {
		if labOrigins[id] != g {
			t.Errorf("origin %s: game %v, lab %v", id, g, labOrigins[id])
		}
	}
	if len(labOrigins) != len(gameOrigins) {
		t.Errorf("lab resolved %d origins, game %d", len(labOrigins), len(gameOrigins))
	}
	want := 0
	for _, f := range loc.Fixtures {
		if f.Kind == campaign.LoreDatapadKind {
			want++
		}
	}
	gameN, labN := 0, 0
	for id, n := range gamePads {
		gameN += n
		if labPads[id] != n {
			t.Errorf("datapads for %s: game %d, lab %d", id, n, labPads[id])
		}
	}
	for _, n := range labPads {
		labN += n
	}
	if gameN != want || labN != want {
		t.Errorf("lore datapads: game %d, lab %d, fixtures %d", gameN, labN, want)
	}
}
