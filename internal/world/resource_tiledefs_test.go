package world

import "testing"

// Resource deposits must be recognized as mineable (IsDepositTileName drives
// scanner reveal + marker color) and parse a positive LightLevel from JSON
// (self-illumination in dark caves — LightingSystem skips tiles with
// LightLevel <= 0).
func TestResourceTileDefsParseDepositAndLight(t *testing.T) {
	if err := LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		t.Fatalf("load tile definitions: %v", err)
	}
	for _, name := range []string{"ore_deposit", "crystal_vein", "radioactive_ore"} {
		idx, ok := TileNameToIndex[name]
		if !ok {
			t.Fatalf("tile %q not found", name)
		}
		if !IsDepositTileName(name) {
			t.Errorf("%s: expected IsDepositTileName=true", name)
		}
		def := TileDefinitions[idx]
		if def.LightLevel <= 0 {
			t.Errorf("%s: expected positive LightLevel, got %d", name, def.LightLevel)
		}
	}
}

// Ship rubble is modeled as a scrap-metal deposit (mined, not dug), so it must
// be recognized as a deposit and drop metal_ore.
func TestRubbleTilesAreMetalDeposits(t *testing.T) {
	for _, name := range []string{"rubble_pile", "rubble_wall_h", "rubble_wall_vl", "rubble_wall_vr"} {
		if !IsDepositTileName(name) {
			t.Errorf("%s: expected IsDepositTileName=true", name)
		}
		if got := DepositDrop(name); got != "metal_ore" {
			t.Errorf("%s: expected DepositDrop=metal_ore, got %q", name, got)
		}
	}
}
