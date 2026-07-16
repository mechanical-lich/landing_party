package world

import "testing"

func TestTileMaterialsParseAndResolve(t *testing.T) {
	if err := LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		t.Fatalf("load tile definitions: %v", err)
	}

	// The material field parses from the tiledef JSON.
	if got := TileDefinitions[TileNameToIndex["ore_deposit"]].Material; got != "metal" {
		t.Fatalf("ore_deposit material = %q, want metal", got)
	}
	if got := TileDefinitions[TileNameToIndex["grass"]].Material; got != "grass" {
		t.Fatalf("grass material = %q, want grass", got)
	}

	// Floor material feeds footsteps, Middle material feeds mining.
	lvl := NewLevel(4, 4, 1)
	lvl.SetFloor(1, 1, 0, "grass", 0)
	lvl.SetMiddle(1, 1, 0, "ore_deposit", 0)
	if got := lvl.FloorMaterialAt(1, 1, 0); got != "grass" {
		t.Fatalf("FloorMaterialAt = %q, want grass", got)
	}
	if got := lvl.MiddleMaterialAt(1, 1, 0); got != "metal" {
		t.Fatalf("MiddleMaterialAt = %q, want metal", got)
	}

	// Empty slots / out of bounds → "".
	if got := lvl.FloorMaterialAt(0, 0, 0); got != "" {
		t.Fatalf("empty floor material = %q, want empty", got)
	}
	if got := lvl.MiddleMaterialAt(-1, 0, 0); got != "" {
		t.Fatalf("out-of-bounds material = %q, want empty", got)
	}
}
