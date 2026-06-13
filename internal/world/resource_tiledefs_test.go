package world

import "testing"

// Resource deposits must parse the scifi-specific fields the minimap scanner
// and in-FOV glow depend on: Mineable (drives scanner reveal + marker color)
// and a positive LightLevel (self-illumination in dark caves).
func TestResourceTileDefsParseMineableAndLight(t *testing.T) {
	if err := LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		t.Fatalf("load tile definitions: %v", err)
	}
	for _, name := range []string{"ore_deposit", "crystal_vein", "radioactive_ore"} {
		idx, ok := TileNameToIndex[name]
		if !ok {
			t.Fatalf("tile %q not found", name)
		}
		def := TileDefinitions[idx]
		if !def.Mineable {
			t.Errorf("%s: expected Mineable=true", name)
		}
		if def.LightLevel <= 0 {
			t.Errorf("%s: expected positive LightLevel, got %d", name, def.LightLevel)
		}
	}
}
