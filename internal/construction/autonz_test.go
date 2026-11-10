package construction

import (
	"encoding/json"
	"testing"
)

// required_floor reads a string or a list; any listed floor will do.
func TestRequiredFloorList(t *testing.T) {
	var one, many Buildable
	if err := json.Unmarshal([]byte(`{"required_floor": "hive_resin"}`), &one); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"required_floor": ["blue_wires", "red_wires"]}`), &many); err != nil {
		t.Fatal(err)
	}
	tiles := [][2]int{{0, 0}, {1, 0}}
	floor := func(names ...string) func(x, y int) string {
		return func(x, y int) string { return names[x] }
	}
	if !one.FloorOK(tiles, floor("hive_resin", "hive_resin")) || one.FloorOK(tiles, floor("hive_resin", "dirt")) {
		t.Error("a single floor")
	}
	if !many.FloorOK(tiles, floor("blue_wires", "red_wires")) || many.FloorOK(tiles, floor("blue_wires", "dirt")) {
		t.Error("a list of floors")
	}
	if !(Buildable{}).FloorOK(tiles, floor("dirt", "dirt")) {
		t.Error("no floor needed")
	}
}

// A deploy_only buildable is in no build list.
func TestDeployOnlyRefused(t *testing.T) {
	old := buildables
	t.Cleanup(func() { buildables = old })
	buildables = map[string]Buildable{
		"relay_pylon": {Name: "Relay Pylon", Category: "structures", DeployOnly: true},
		"cogitator":   {Name: "Cogitator", Category: "structures"},
	}
	if got := BuildableTypesInCategory("structures"); len(got) != 1 || got[0] != "cogitator" {
		t.Errorf("category lists %v", got)
	}
	for _, b := range ListBuildablesForTechs(nil) {
		if b.DeployOnly {
			t.Errorf("listed %s", b.Name)
		}
	}
}
