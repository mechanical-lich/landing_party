package construction

import "testing"

// A buildable's required floor must lie under its whole footprint; the
// hive's structures carry resin, the autonz' wire, nothing else a floor rule.
func TestRequiredFloor(t *testing.T) {
	if err := Load("../../data/build.json"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Load("") })
	b := GetBuildable("egg_cluster")
	if len(b.RequiredFloor) != 1 || !b.RequiredFloor.Has("hive_resin") {
		t.Fatalf("egg_cluster required floor %q", b.RequiredFloor)
	}
	tiles := [][2]int{{0, 0}, {1, 0}}
	resin := func(x, y int) string { return "hive_resin" }
	half := func(x, y int) string {
		if x == 0 {
			return "hive_resin"
		}
		return "alien_ground"
	}
	if !b.FloorOK(tiles, resin) || b.FloorOK(tiles, half) {
		t.Error("FloorOK: all resin passes, part resin is refused")
	}
	for _, k := range []string{"biomass_pool", "egg_cluster", "evolution_chamber", "egg_launcher"} {
		if !GetBuildable(k).RequiredFloor.Has("hive_resin") || GetBuildable(k).Category != "hive" {
			t.Errorf("%s: %+v", k, GetBuildable(k))
		}
	}
	autonz := map[string]bool{"cogitator": true, "foundry": true, "converter_node": true, "defence_pylon": true, "signal_core": true, "relay_pylon": true}
	for k, b := range ListBuildables() {
		switch {
		case autonz[k]:
			if !b.RequiredFloor.Has("blue_wires") || !b.RequiredFloor.Has("red_wires") {
				t.Errorf("%s needs %v", k, b.RequiredFloor)
			}
		case len(b.RequiredFloor) > 0 && b.Category != "hive":
			t.Errorf("%s needs %v", k, b.RequiredFloor)
		}
	}
	if !GetBuildable("storage_locker").FloorOK(tiles, half) {
		t.Error("a buildable without a floor rule refused")
	}
}
