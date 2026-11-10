package research

import (
	"encoding/json"
	"os"
	"testing"
)

// The hive's evolve tree: every entry is a hive_* key or a launcher range
// entry carrying tree "hive" and an evolve block, researched at the evolution
// chamber; the doctrine's walk lists every one of them, each after what it
// requires; no other entry carries a tree (the colony's tree is unchanged).
func TestHiveTree(t *testing.T) {
	if err := Load("../../data/research.json"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Load("") })
	var fd struct {
		Doctrines map[string]struct {
			Research []struct {
				ID string `json:"id"`
			} `json:"research"`
		} `json:"doctrines"`
	}
	b, err := os.ReadFile("../../data/factions.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fd); err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, r := range fd.Doctrines["hive"].Research {
		pos[r.ID] = i + 1
	}
	hive := 0
	for k, tech := range AllTechs() {
		if tech.Tree == "" {
			if tech.Evolve != nil || tech.Assimilate != nil {
				t.Errorf("%s: an evolve or assimilate block outside a tree", k)
			}
			continue
		}
		if tech.Tree == "greyish" {
			// The greyish' Tier 1 walk: ids only, no effect block.
			if tech.Evolve != nil || tech.Assimilate != nil {
				t.Errorf("%s: %+v", k, tech)
			}
			continue
		}
		if tech.Tree == "autonz" {
			// The autonz' assimilation tree (Phase E3): its own block,
			// at the cogitator; the campaign's tree test walks it.
			if tech.Assimilate == nil || tech.Evolve != nil || tech.RequiredBuilding != "cogitator" {
				t.Errorf("%s: %+v", k, tech)
			}
			continue
		}
		hive++
		if tech.Tree != "hive" || tech.Evolve == nil || tech.RequiredBuilding != "evolution_chamber" || tech.RequiredInt != 0 {
			t.Errorf("%s: %+v", k, tech)
		}
		if tech.Cost["biomass"] <= 0 || len(tech.Cost) != 1 {
			t.Errorf("%s costs %v, want biomass", k, tech.Cost)
		}
		if tech.RequiresTech != "" && pos[tech.RequiresTech] == 0 {
			t.Errorf("%s requires %s, not in the hive's walk", k, tech.RequiresTech)
		}
		if p := pos[k]; p > 0 && tech.RequiresTech != "" && pos[tech.RequiresTech] >= p {
			t.Errorf("%s (walk %d) before what it requires, %s (walk %d)", k, p, tech.RequiresTech, pos[tech.RequiresTech])
		}
	}
	if hive < 14 {
		t.Errorf("%d hive entries", hive)
	}
	for _, k := range []string{"hive_drone_vigour", "hive_warrior_carapace_2", "hive_hunter_talons_2"} {
		if tech, _ := GetTech(k); tech.Evolve == nil || !tech.Evolve.Evolved {
			t.Errorf("%s: the type's second upgrade switches the frame", k)
		}
	}
}
