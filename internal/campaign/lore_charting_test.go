package campaign

import (
	"encoding/json"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// loreCampaign is a generated campaign grown by expansion and travel rolls.
func loreCampaign(t *testing.T, seed int64) *Campaign {
	t.Helper()
	c := genCampaign(t, seed)
	c.Expand(6)
	for i := 0; i < 12; i++ {
		c.MaybeTravelQuests()
	}
	return c
}

// Every charted site gets a history whose chapter (or quiet base) is the
// scenario it plays and whose summary replaces the archetype sentence; Home,
// the Wreck Site and the tutorial get none.
func TestLoreChartingBindsHistories(t *testing.T) {
	useRepoData(t)
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range []int64{1, 7, 42} {
		c := loreCampaign(t, seed)
		for _, loc := range c.Locations {
			site := c.SiteHistory(loc)
			if !loc.HasSite() || loc.Tutorial {
				if loc.History != "" {
					t.Errorf("seed %d: %s (%s) has a history", seed, loc.ID, loc.Kind)
				}
				continue
			}
			if site == nil {
				t.Errorf("seed %d: %s has no history", seed, loc.ID)
				continue
			}
			if site.Location != loc.ID || site.Map != loc.MapID || site.Seed != loc.Seed {
				t.Errorf("seed %d: %s bound to %+v", seed, loc.ID, site)
			}
			if loc.ScenarioID != site.Scenario {
				t.Errorf("seed %d: %s plays %s, history says %s", seed, loc.ID, loc.ScenarioID, site.Scenario)
			}
			if want := c.History.Summary(lib, site); want == "" || loc.Summary != want {
				t.Errorf("seed %d: %s summary %q, history gives %q", seed, loc.ID, loc.Summary, want)
			}
		}
	}
}

// The same seed charts the same sites and histories, and a saved campaign keeps
// charting exactly as the original does.
func TestLoreSameSeedSameSitesAcrossSaveLoad(t *testing.T) {
	useRepoData(t)
	a, b := loreCampaign(t, 99), loreCampaign(t, 99)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("two runs of one seed differ")
	}
	var loaded Campaign
	if err := json.Unmarshal(ja, &loaded); err != nil {
		t.Fatal(err)
	}
	loaded.BindQuests()
	a.Expand(3)
	loaded.Expand(3)
	for i := 0; i < 6; i++ {
		a.MaybeTravelQuests()
		loaded.MaybeTravelQuests()
	}
	ja, _ = json.Marshal(a)
	jl, _ := json.Marshal(&loaded)
	if string(ja) != string(jl) {
		t.Error("campaign diverged after save and load")
	}
}

// Every spawn_event a contract names is a chapter that can end a history on
// the contract's archetype's maps, and every binding plays its scenario (an
// exact ending or another chapter of the same scenario).
func TestLoreSpawnEventsExistAndBind(t *testing.T) {
	useRepoData(t)
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	lt, qt, err := loadGenTemplates()
	if err != nil {
		t.Fatal(err)
	}
	named := 0
	for _, tpl := range contractTemplates(qt) {
		if tpl.SpawnEvent == "" {
			continue
		}
		named++
		def := lib.Events[tpl.SpawnEvent]
		if def == nil || !def.IsChapter() {
			t.Errorf("contract %s: spawn_event %q is not a chapter event", tpl.ID, tpl.SpawnEvent)
			continue
		}
		a, ok := archetypeByID(lt, tpl.SpawnArchetype)
		if !ok {
			t.Errorf("contract %s: unknown archetype %q", tpl.ID, tpl.SpawnArchetype)
			continue
		}
		exact := false
		for _, m := range a.Maps {
			for seed := int64(1); seed <= 20; seed++ {
				h := history.New(seed)
				site, err := h.BindSiteEnding(lib, m, seed*131, tpl.SpawnEvent)
				if err != nil {
					t.Errorf("contract %s on %s (seed %d): %v", tpl.ID, m, seed, err)
					break
				}
				if site.Scenario != def.Scenario {
					t.Errorf("contract %s on %s plays %s, want %s", tpl.ID, m, site.Scenario, def.Scenario)
				}
				if h.Event(site.Chapter).Type == tpl.SpawnEvent {
					exact = true
				}
			}
		}
		if !exact {
			t.Errorf("contract %s: %s never ends a history on its archetype's maps", tpl.ID, tpl.SpawnEvent)
		}
	}
	if named == 0 {
		t.Fatal("no contract names a spawn_event")
	}
}

// An archetype's spawn_event holds for every site of it: hive worlds still
// play exterminate_xenos, as their archetype list used to force.
func TestLoreArchetypeSpawnEventHolds(t *testing.T) {
	useRepoData(t)
	lt, _, err := loadGenTemplates()
	if err != nil {
		t.Fatal(err)
	}
	a, ok := archetypeByID(lt, "hive")
	if !ok || a.SpawnEvent == "" {
		t.Fatal("hive archetype has no spawn_event")
	}
	lib, _ := history.DefaultLibrary()
	want := lib.Events[a.SpawnEvent].Scenario
	for seed := int64(1); seed <= 30; seed++ {
		c := genCampaign(t, seed)
		loc := c.newLocation(rand.New(rand.NewSource(seed)), a, 50, 50, "", "")
		if loc.ScenarioID != want {
			t.Errorf("seed %d: hive site plays %s, want %s", seed, loc.ScenarioID, want)
		}
	}
}

// The first jump is safe: across 300 seeds the start system never plays a
// scenario with hostiles unless its chapter is flagged safe_start or it is a
// quiet_safe map's quiet outcome.
func TestLoreStartSystemIsSafe(t *testing.T) {
	useRepoData(t)
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	hostile := map[string]int{}
	for _, sc := range loreScenarioHostiles(t) {
		hostile[sc.id] = sc.max
	}
	for seed := int64(1); seed <= 300; seed++ {
		c := genCampaign(t, seed)
		loc := c.Locations[StartLocationID]
		site := c.SiteHistory(loc)
		if site == nil {
			t.Fatalf("seed %d: start has no history", seed)
		}
		if hostile[loc.ScenarioID] == 0 {
			continue
		}
		if site.Chapter == "" {
			if !lib.Maps[loc.MapID].Present.QuietSafe {
				t.Errorf("seed %d: start is a quiet %s, not quiet_safe", seed, loc.MapID)
			}
			continue
		}
		if def := lib.Events[c.History.Event(site.Chapter).Type]; !def.SafeStart {
			t.Errorf("seed %d: start plays %s (chapter %s), not safe_start", seed, loc.ScenarioID, def.ID)
		}
	}
}

type loreHostile struct {
	id  string
	max int
}

// loreScenarioHostiles reads each scenario's hostile_max from data/scenarios.
func loreScenarioHostiles(t *testing.T) []loreHostile {
	t.Helper()
	paths, _ := filepath.Glob("../../data/scenarios/*.json")
	var out []loreHostile
	for _, p := range paths {
		var sc struct {
			ID         string `json:"id"`
			HostileMax int    `json:"hostile_max"`
		}
		if err := loadJSON(p, &sc); err != nil {
			t.Fatal(err)
		}
		out = append(out, loreHostile{sc.ID, sc.HostileMax})
	}
	if len(out) == 0 {
		t.Fatal("no scenarios")
	}
	return out
}
