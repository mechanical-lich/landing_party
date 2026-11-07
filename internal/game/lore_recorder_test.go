package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// loreLive returns the live events of a type, oldest first.
func loreLive(h *history.History, typ string) []history.Event {
	var out []history.Event
	for _, e := range h.Events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// A colonist killed by a zombie is recorded with the killer, dated in the
// star date and known at once; as the site's last colonist, the colony's wipe
// follows.
func TestLoreRecorderColonistDeathAndWipe(t *testing.T) {
	wm, _ := bgAway(t, "a")
	c := wm.Campaign
	c.History = history.New(1)
	c.StarDate = 42
	bgTravel(t, wm, "a")
	ms := wm.Current()
	var worker *ecs.Entity
	for _, e := range ms.level.Entities {
		if e.Blueprint == "colonist" {
			worker = e
		}
	}
	if worker == nil {
		t.Fatal("no colonist on the site")
	}
	zombie := &ecs.Entity{Blueprint: "zombie"}
	ms.level.QueueEvent(eventsystem.EntityDamagedEvent{Victim: worker, Attacker: zombie, Amount: 5, Level: ms.level})
	ms.level.Events.HandleQueue()
	worker.AddComponent(&rlcomponents.DeadComponent{})
	ms.cleanUpSystem.Update(ms.level)

	died := loreLive(c.History, "colonist_died")
	if len(died) != 1 {
		t.Fatalf("%d colonist deaths recorded", len(died))
	}
	d := died[0]
	if !strings.Contains(d.Vars["cause"], "zombie") || d.Vars["colonist"] == "" || d.Date != 42 || !c.History.Known[d.ID] {
		t.Errorf("death recorded as %+v (known %v)", d, c.History.Known[d.ID])
	}
	if len(d.Figures) != 1 || c.History.Figures[d.Figures[0]].Fate != "died" {
		t.Errorf("no dead figure for the colonist: %v", d.Figures)
	}
	if len(loreLive(c.History, "colony_wiped")) != 1 {
		t.Error("the site's wipe was not recorded")
	}
	lib, _ := history.DefaultLibrary()
	if text := c.History.Render(lib, &d, ""); !strings.Contains(text, "killed by zombie") {
		t.Errorf("rendered %q", text)
	}
}

// Beaming the first colonist down founds a colony; beaming the last one up
// leaves it.
func TestLoreRecorderColonyFoundedAndLeft(t *testing.T) {
	wm, _ := bgAway(t, "a")
	c := wm.Campaign
	c.History = history.New(1)
	bgTravel(t, wm, "a")
	ms := wm.Current()
	var worker *ecs.Entity
	for _, e := range ms.level.Entities {
		if e.Blueprint == "colonist" {
			worker = e
		}
	}
	if err := wm.BeamUp(worker); err != nil {
		t.Fatal(err)
	}
	if len(loreLive(c.History, "colony_left")) != 1 {
		t.Error("leaving the site was not recorded")
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	if len(loreLive(c.History, "colony_founded")) != 1 {
		t.Error("founding the colony was not recorded")
	}
}

// A colonist standing by an event's origin finds it: the event becomes known.
func TestLoreReachingAnOriginRevealsIt(t *testing.T) {
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
	site := c.SiteHistory(c.Locations[locID])
	before := c.KnownEventCount()
	if c.History.Known[site.Chapter] {
		t.Fatal("chapter known before anyone went there")
	}
	ox, oy, oz := ms.level.Regions[LoreOriginRegion][0][0], ms.level.Regions[LoreOriginRegion][0][1], ms.level.Regions[LoreOriginRegion][0][2]
	x, y, z := freeLandingTile(ms.level, ox, oy, oz)
	if _, err := spawnColonist(ms.level, ms.colonyName(), x, y, z); err != nil {
		t.Fatal(err)
	}
	ms.loreUpkeep()
	if !c.History.Known[site.Chapter] || c.KnownEventCount() <= before {
		t.Errorf("standing at the origin (%d,%d) did not reveal %s", x, y, site.Chapter)
	}
}

// Live history speaks in display names: no internal settlement key
// ("... Colony @ Site") and no quest-list suffix in any rendered live event.
func TestLoreLiveEventsUseDisplayNames(t *testing.T) {
	wm, _ := bgAway(t, "a")
	c := wm.Campaign
	c.Name = "Lore Legends"
	c.History = history.New(1)
	bgTravel(t, wm, "a")
	ms := wm.Current()
	var worker *ecs.Entity
	for _, e := range ms.level.Entities {
		if e.Blueprint == "colonist" {
			worker = e
		}
	}
	if err := wm.BeamUp(worker); err != nil {
		t.Fatal(err)
	}
	colonist := wm.ShipColonists()[0]
	if err := wm.BeamDown(colonist); err != nil {
		t.Fatal(err)
	}
	colonist.AddComponent(&rlcomponents.DeadComponent{})
	ms.cleanUpSystem.Update(ms.level)
	loc := c.Locations["a"]
	c.QuestDefs = append(c.QuestDefs, campaign.Quest{ID: "q1", Name: "Ore Run — " + loc.Name, Location: "a"})
	c.BindQuests()
	c.RecordLive("quest_completed", loc, map[string]string{"quest": c.QuestTitle(&c.QuestDefs[len(c.QuestDefs)-1])})

	lib, _ := history.DefaultLibrary()
	seen := map[string]bool{}
	for i := range c.History.Events {
		ev := &c.History.Events[i]
		text := c.History.Render(lib, ev, "")
		seen[ev.Type] = true
		if strings.Contains(text, " @ ") || strings.Contains(text, " — ") {
			t.Errorf("%s leaks an internal name: %q", ev.Type, text)
		}
		if strings.HasPrefix(ev.Type, "colony_") && !strings.Contains(strings.ToLower(text), "the lore legends colony") {
			t.Errorf("%s does not name the colony: %q", ev.Type, text)
		}
	}
	for _, typ := range []string{"colony_left", "colony_founded", "colonist_died", "colony_wiped", "quest_completed"} {
		if !seen[typ] {
			t.Errorf("no %s recorded", typ)
		}
	}
}
