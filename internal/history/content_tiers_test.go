package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/scenario"
)

// contentBlueprints reads every entity blueprint by ID.
func contentBlueprints(t *testing.T) map[string]map[string]json.RawMessage {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(loreDataRoot, "blueprints", "entities", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no entity blueprints (%v)", err)
	}
	out := map[string]map[string]json.RawMessage{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]map[string]json.RawMessage
		if json.Unmarshal(b, &m) != nil {
			continue // not a blueprint map
		}
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// passive reports whether a blueprint only wanders (critters).
func passive(bp map[string]json.RawMessage) bool {
	var ai struct {
		BehaviorKey string `json:"BehaviorKey"`
	}
	if raw, ok := bp["FactionAI"]; ok && json.Unmarshal(raw, &ai) == nil {
		return ai.BehaviorKey == "wander"
	}
	return false
}

// Every map's base scenario is calm: its spawn rules bring only passive
// wildlife, so the quiet tier is quiet and a hazard's danger is its own.
func TestContentCalmBasesAreCalm(t *testing.T) {
	lib := loreLibrary(t)
	bps := contentBlueprints(t)
	for _, m := range historyMaps(lib) {
		sc := scenario.ByID(lib.Maps[m].BaseScenario)
		if sc == nil {
			t.Errorf("map %s: base scenario %q missing", m, lib.Maps[m].BaseScenario)
			continue
		}
		for bp := range sc.SpawnRules {
			if b := bps[bp]; b == nil || !passive(b) {
				t.Errorf("map %s: base scenario %s spawns %s, which is not passive wildlife", m, sc.ID, bp)
			}
		}
	}
}

// The scenarios the calm bases replaced stay reachable as threats.
func TestContentOldBasesStayThreats(t *testing.T) {
	lib := loreLibrary(t)
	want := map[string]string{
		"worms":          "desert_planet",
		"yeti_territory": "frozen_planet",
		"alien_world":    "alien",
	}
	for sc, m := range want {
		found := false
		for _, e := range lib.ChapterEvents() {
			if e.Scenario == sc && e.ChapterTier() == TierThreat && e.Weight[m] > 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is no threat chapter on %s", sc, m)
		}
	}
}

// contentSite is a bound site and the history it lives in.
type contentSite struct {
	h *History
	*SiteHistory
}

// contentSites binds n seeds' worth of sites per history map the way the dump's
// frequency table does (one site per map per seed), safe or not.
func contentSites(t *testing.T, lib *Library, seeds int, safe bool) map[string][]contentSite {
	t.Helper()
	maps := historyMaps(lib)
	out := map[string][]contentSite{}
	for s := 1; s <= seeds; s++ {
		h := New(int64(s) * 7919)
		for i, m := range maps {
			bind := h.BindSite
			if safe {
				bind = h.BindSiteSafe
			}
			site, err := bind(lib, m, h.Seed+int64(i+1)*131)
			if err != nil {
				t.Fatalf("seed %d map %s: %v", s, m, err)
			}
			out[m] = append(out[m], contentSite{h, site})
		}
	}
	return out
}

// 2,000 seeds hit each map's tier shares within 5 points (the plan's sample:
// 500 seeds swing about 2 points either way as the data changes), and every
// map reaches every tier.
func TestContentTierShares(t *testing.T) {
	lib := loreLibrary(t)
	for m, sites := range contentSites(t, lib, 2000, false) {
		n := map[string]int{}
		for _, s := range sites {
			n[s.Tier]++
		}
		target, total := lib.Maps[m].Present.Tiers, 0
		for _, v := range target {
			total += v
		}
		for _, tier := range Tiers {
			if n[tier] == 0 {
				t.Errorf("%s never reaches tier %s", m, tier)
			}
			got := 100 * float64(n[tier]) / float64(len(sites))
			want := 100 * float64(target[tier]) / float64(total)
			if got < want-5 || got > want+5 {
				t.Errorf("%s: %s share %.1f%%, want %.0f%% ±5", m, tier, got, want)
			}
		}
	}
}

// A first jump is never a threat, its hazards are all safe_start, and both
// safe tiers come up.
func TestContentSafeStartTiers(t *testing.T) {
	lib := loreLibrary(t)
	tiers := map[string]int{}
	for m, sites := range contentSites(t, lib, 200, true) {
		for _, s := range sites {
			tiers[s.Tier]++
			switch s.Tier {
			case TierThreat:
				t.Errorf("%s %s: a first jump rolled a threat", m, s.ID)
			case TierHazard:
				if def := lib.Events[s.h.Event(s.Chapter).Type]; !def.SafeStart {
					t.Errorf("%s %s: a first jump plays hazard %s, not safe_start", m, s.ID, def.ID)
				}
			}
		}
	}
	if tiers[TierHazard] == 0 || tiers[TierQuiet] == 0 {
		t.Errorf("first jumps reach hazard %d and quiet %d times; want both", tiers[TierHazard], tiers[TierQuiet])
	}
}

// contentHostiles are the blueprints a hazard's evidence may spawn as enemies.
var contentHostiles = map[string]bool{
	"xeno_egg": true, "troid_egg": true, "yeti": true, "worm": true,
	"mutant_grunt": true, "mutant_scout": true, "zombie": true, "enforcer": true,
	"spore_pod": true, "old_munition": true, "sentry_turret": true,
}

// Every hazard plays the map's base scenario (or a gentle scenario of its
// own), has an origin, anchors all its evidence there, spawns at most six
// hostiles, sets its hazard lore flag and an omen; and every hazard comes up
// somewhere in the sweep.
func TestContentHazardsPlayBase(t *testing.T) {
	lib := loreLibrary(t)
	seen := map[string]bool{}
	for _, def := range lib.ChapterEvents() {
		if def.ChapterTier() != TierHazard {
			continue
		}
		if !def.Origin && len(def.OriginFrom) == 0 {
			t.Errorf("hazard %s has no origin", def.ID)
		}
		if def.Flags["hazard"] != def.ID {
			t.Errorf("hazard %s: flags.hazard = %v", def.ID, def.Flags["hazard"])
		}
		if def.Omen == "" {
			t.Errorf("hazard %s has no omen", def.ID)
		}
		max := 0
		for _, ev := range def.Evidence {
			if ev.At != "origin" {
				t.Errorf("hazard %s: %s evidence is not anchored at its origin", def.ID, ev.Kind)
			}
			if bp, _ := ev.Params["blueprint"].(string); contentHostiles[bp] {
				n := ev.Count
				if ev.CountMax > n {
					n = ev.CountMax
				}
				max += n
			} else if k, ok := ev.Params["hostiles"].(float64); ok {
				max += int(k)
			}
		}
		if max > 6 {
			t.Errorf("hazard %s spawns up to %d hostiles, want at most 6", def.ID, max)
		}
	}
	for m, sites := range contentSites(t, lib, 300, false) {
		for _, s := range sites {
			if s.Tier != TierHazard {
				continue
			}
			def := lib.Events[s.h.Event(s.Chapter).Type]
			seen[def.ID] = true
			want := def.Scenario
			if want == "" {
				want = lib.Maps[m].BaseScenario
			}
			if s.Scenario != want {
				t.Errorf("%s %s: hazard %s plays %s, want %s", m, s.ID, def.ID, s.Scenario, want)
			}
		}
	}
	for _, def := range lib.ChapterEvents() {
		if def.ChapterTier() == TierHazard && !seen[def.ID] {
			t.Errorf("hazard %s never ends a history in 300 seeds", def.ID)
		}
	}
}
