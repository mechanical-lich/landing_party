package campaign

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// The same seed gives the same galaxy with the tutorial on or off: every
// generated location and quest matches, and only the Training Ground is added.
func TestTutorialLocationLeavesGenerationAlone(t *testing.T) {
	useRepoData(t)
	for _, seed := range []int64{1, 77, 1234, 98765} {
		plain := genCampaign(t, seed)
		withTut := genCampaign(t, seed)
		loc := AddTutorialLocation(withTut)
		if loc == nil || loc.ID != TutorialLocationID || !loc.Tutorial {
			t.Fatalf("seed %d: AddTutorialLocation returned %+v", seed, loc)
		}
		if withTut.CurrentLocationID != plain.CurrentLocationID {
			t.Errorf("seed %d: current location moved to %s", seed, withTut.CurrentLocationID)
		}
		if withTut.CurrentLocationID == TutorialLocationID {
			t.Errorf("seed %d: the ship starts at the tutorial planet", seed)
		}
		if len(withTut.Locations) != len(plain.Locations)+1 {
			t.Errorf("seed %d: %d locations, want %d", seed, len(withTut.Locations), len(plain.Locations)+1)
		}
		for id, a := range plain.Locations {
			b := withTut.Locations[id]
			if b == nil || a.X != b.X || a.Y != b.Y || a.MapID != b.MapID || a.ScenarioID != b.ScenarioID || a.Seed != b.Seed || a.Name != b.Name {
				t.Errorf("seed %d: location %s differs with the tutorial on", seed, id)
			}
		}
		if len(plain.QuestDefs) != len(withTut.QuestDefs) {
			t.Fatalf("seed %d: quests %d vs %d", seed, len(plain.QuestDefs), len(withTut.QuestDefs))
		}
		for i := range plain.QuestDefs {
			if plain.QuestDefs[i].ID != withTut.QuestDefs[i].ID || plain.QuestDefs[i].Location != withTut.QuestDefs[i].Location {
				t.Errorf("seed %d: quest %d differs with the tutorial on", seed, i)
			}
		}
		if withTut.GenSeq != plain.GenSeq || withTut.TravelSeq != plain.TravelSeq {
			t.Errorf("seed %d: generation counters moved (%d/%d vs %d/%d)", seed, withTut.GenSeq, withTut.TravelSeq, plain.GenSeq, plain.TravelSeq)
		}
		// Adding it twice is a no-op.
		if again := AddTutorialLocation(withTut); again != loc || len(withTut.Locations) != len(plain.Locations)+1 {
			t.Errorf("seed %d: AddTutorialLocation is not idempotent", seed)
		}
	}
}

// The Training Ground is discovered, keeps the minimum separation, is the
// nearest destination from the start and costs 6-8 fuel, on many seeds.
func TestTutorialLocationPlacement(t *testing.T) {
	useRepoData(t)
	minSep := genConfig().MinSeparation
	for seed := int64(1); seed <= 60; seed++ {
		c := genCampaign(t, seed)
		loc := AddTutorialLocation(c)
		if !loc.Discovered {
			t.Fatalf("seed %d: tutorial planet hidden", seed)
		}
		cost := c.FuelCost(loc.ID)
		if cost < 5 || cost > 7 {
			t.Errorf("seed %d: fuel to the tutorial planet is %d, want 5-7", seed, cost)
		}
		for id, other := range c.Locations {
			if id == loc.ID {
				continue
			}
			if d := math.Hypot(other.X-loc.X, other.Y-loc.Y); d < minSep {
				t.Errorf("seed %d: tutorial planet %.1f from %s, min separation %.1f", seed, d, id, minSep)
			}
			if id != c.CurrentLocationID && other.Discovered && c.FuelCost(id) <= cost {
				t.Errorf("seed %d: %s (fuel %d) is as near as the tutorial planet (fuel %d)", seed, id, c.FuelCost(id), cost)
			}
		}
		// Away from Home: the bearing to the Training Ground is more than 60
		// degrees off the bearing to Home.
		home := c.HomeLocation()
		start := c.CurrentLocation()
		toHome := math.Atan2(home.Y-start.Y, home.X-start.X)
		toTut := math.Atan2(loc.Y-start.Y, loc.X-start.X)
		off := math.Abs(math.Mod(toTut-toHome+3*math.Pi, 2*math.Pi) - math.Pi)
		if off < math.Pi/3 {
			t.Errorf("seed %d: tutorial planet is only %.0f degrees off the Home line", seed, off*180/math.Pi)
		}
		if c.Tutorial == nil || c.Tutorial.NextLocationID != StartLocationID || !c.Tutorial.IsUnlocked(TutorialLocationID) || c.Tutorial.IsUnlocked(c.CurrentLocationID) {
			t.Errorf("seed %d: progress = %+v", seed, c.Tutorial)
		}
	}
}

// Travel quests never bind to the Training Ground, however many jumps roll.
func TestTravelQuestsSkipTutorialLocation(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 20; seed++ {
		c := genCampaign(t, seed)
		AddTutorialLocation(c)
		for i := 0; i < 60; i++ {
			c.MaybeTravelQuests()
		}
		for _, q := range c.QuestDefs {
			if q.Location == TutorialLocationID {
				t.Fatalf("seed %d: quest %s bound to the tutorial planet", seed, q.ID)
			}
		}
		if len(c.Locations[TutorialLocationID].Fixtures) != 0 {
			t.Fatalf("seed %d: fixtures placed on the tutorial planet", seed)
		}
	}
}

// Progress round-trips through the campaign JSON, including the lock's allow
// list and the baselines; an old save has no tutorial at all.
func TestTutorialProgressRoundTrip(t *testing.T) {
	c := &Campaign{Name: "RT", CurrentLocationID: "start", Locations: map[string]*Location{
		"start":    {ID: "start"},
		"tutorial": {ID: "tutorial", Tutorial: true},
	}}
	c.Tutorial = &TutorialProgress{
		StepID: "pack_cargo", NextLocationID: "start",
		Unlocked:  []string{"tutorial", "start"},
		Baselines: map[string]int{"hold.metal_ore": 3},
		PadPlaced: true,
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Campaign
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	got := back.Tutorial
	if got == nil || got.StepID != "pack_cargo" || got.NextLocationID != "start" || !got.PadPlaced {
		t.Fatalf("progress after reload = %+v", got)
	}
	if len(got.Unlocked) != 2 || got.Baselines["hold.metal_ore"] != 3 {
		t.Fatalf("unlocked/baselines lost: %+v", got)
	}
	if !back.Locations["tutorial"].Tutorial {
		t.Fatal("location Tutorial flag lost")
	}

	var old Campaign
	if err := json.Unmarshal([]byte(`{"name":"old","current_location_id":"a","locations":{}}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Tutorial != nil || old.TutorialActive() {
		t.Fatal("an old save grew a tutorial")
	}
}

// No generator archetype can roll the tutorial map or scenario.
func TestTemplatesNeverRollTutorial(t *testing.T) {
	useRepoData(t)
	raw, err := os.ReadFile(LocationTemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	var lt locTemplates
	if err := json.Unmarshal(raw, &lt); err != nil {
		t.Fatal(err)
	}
	check := func(where string, ids []string) {
		for _, id := range ids {
			if id == "tutorial" {
				t.Errorf("%s lists the tutorial map/scenario", where)
			}
		}
	}
	for _, a := range lt.Archetypes {
		check("archetype "+a.ID+" maps", a.Maps)
	}
	check("home maps", lt.Home.Maps)
	check("home scenarios", lt.Home.Scenarios)
}
