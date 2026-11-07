package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
)

// loreZombieSite finds a campaign seed whose charted systems include a
// zombie_plague site with a chapter origin, preferring the smaller maps.
func loreZombieSite(t *testing.T) (int64, string) {
	t.Helper()
	for _, maps := range [][]string{{"moon", "asteroid"}, {"desert_planet", "earth_like", "frozen_planet", "alien", "ocean_planet"}} {
		for seed := int64(1); seed <= 400; seed++ {
			c, err := campaign.GenerateCampaign("Lore", seed)
			if err != nil {
				t.Fatal(err)
			}
			for _, loc := range c.Locations {
				site := c.SiteHistory(loc)
				if site == nil || loc.ScenarioID != "zombie_plague" || site.Chapter == "" || c.History.Event(site.Chapter).Origin == nil {
					continue
				}
				for _, m := range maps {
					if loc.MapID == m {
						return seed, loc.ID
					}
				}
			}
		}
	}
	t.Fatal("no zombie_plague site with an origin in 400 seeds")
	return 0, ""
}

// A zombie_plague site grows its threat from the history's origin: the first
// dead rise within reach of the resolved origin tile, and the lore datapad
// lying there reveals the chapter.
func TestLoreZombiePlagueRisesAtItsOrigin(t *testing.T) {
	loreLoadData(t)
	seed, locID := loreZombieSite(t)
	c, wm, err := newExpedition("Lore", seed, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := wm.Travel(locID); err != nil {
		t.Fatal(err)
	}
	ms := wm.Current()
	if ms == nil {
		t.Fatal("no level built")
	}
	lvl := ms.level
	site := c.SiteHistory(c.Locations[locID])
	ox, oy, oz, ok := generation.Origin(lvl, generation.ChapterOrigin)
	if !ok {
		t.Fatal("chapter origin not resolved on the level")
	}
	if lvl.Flags["origin_x"] != float64(ox) || lvl.Flags["origin_event"] != c.History.Event(site.Chapter).Type {
		t.Errorf("lore flags %v/%v, origin (%d,%d) chapter %s", lvl.Flags["origin_x"], lvl.Flags["origin_event"], ox, oy, c.History.Event(site.Chapter).Type)
	}

	const reach = 13 // find_open_tile_near's radius 12, plus a tile
	near, far := 0, 0
	for _, e := range lvl.Entities {
		if e == nil || e.Blueprint != "zombie" || !e.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if dx, dy := pc.GetX()-ox, pc.GetY()-oy; dx*dx+dy*dy <= reach*reach && pc.GetZ() == oz {
			near++
		} else {
			far++
		}
	}
	spawned := toSetupFloat(lvl.Flags["zombie_origin_spawned"])
	if near == 0 || float64(near) < spawned {
		t.Errorf("%d zombies within %d tiles of the origin (%d,%d), script spawned %v, %d elsewhere", near, reach, ox, oy, spawned, far)
	}

	// The chapter's datapad lies at the origin.
	var pad *rlcomponents.PositionComponent
	for _, e := range lvl.Entities {
		if e == nil || !e.HasComponent(components.Datapad) {
			continue
		}
		if dp := e.GetComponent(components.Datapad).(*components.DatapadComponent); dp.EventID == site.Chapter && dp.QuestID == "" {
			pad = e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		}
	}
	if pad == nil {
		t.Fatal("no lore datapad for the chapter")
	}
	if dx, dy := pad.GetX()-ox, pad.GetY()-oy; dx*dx+dy*dy > 6*6 {
		t.Errorf("chapter datapad at (%d,%d), origin (%d,%d)", pad.GetX(), pad.GetY(), ox, oy)
	}

	// A colonist standing on it reads it: the chapter becomes known and its
	// text is posted.
	message.MessageLog = nil
	if _, err := spawnColonist(lvl, ms.colonyName(), pad.GetX(), pad.GetY(), pad.GetZ()); err != nil {
		t.Fatal(err)
	}
	before := c.KnownEventCount()
	ms.collectDatapads()
	if !c.History.Known[site.Chapter] || c.KnownEventCount() != before+1 {
		t.Errorf("chapter %s not known after reading its datapad", site.Chapter)
	}
	event.GetQueuedInstance().HandleQueue()
	want, _ := c.RevealEvent(site.Chapter)
	found := false
	for _, line := range message.MessageLog {
		if strings.Contains(line, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("datapad text %q not posted; log %v", want, message.MessageLog)
	}
}

// Without a history (no origin) the script behaves as it always did: the
// first wave stands 20-50 tiles from the landing.
func TestLoreZombiePlagueWithoutHistoryKeepsItsOldShape(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	const colony = "Plain Colony"
	ms, err := NewMainState(SettlementConfig{Name: colony, ColonyName: colony, MapID: "moon", ScenarioID: "zombie_plague", Seed: 77, CampaignMode: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	lvl := ms.level
	if lvl.Flags["origin_x"] != -1.0 {
		t.Fatalf("origin_x = %v without a history", lvl.Flags["origin_x"])
	}
	sx, sy := lvl.Flags["start_x"].(float64), lvl.Flags["start_y"].(float64)
	n := 0
	for _, e := range lvl.Entities {
		if e == nil || e.Blueprint != "zombie" {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		dx, dy := float64(pc.GetX())-sx, float64(pc.GetY())-sy
		if d2 := dx*dx + dy*dy; d2 <= 400 || d2 >= 2500 {
			t.Errorf("zombie at distance² %.0f from the landing", d2)
		}
		n++
	}
	if n == 0 {
		t.Error("no zombies rose")
	}
}

func TestLoreZombiePlagueUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "moon", "zombie_plague", "zombie_plague_setup", 77, []string{"zombie"})
}
