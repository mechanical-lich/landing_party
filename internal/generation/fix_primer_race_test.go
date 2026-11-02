package generation

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// loadTileDefs loads the real tile definitions. Without them SetFloor and SetMiddle
// return before touching DirtyColumns, so these tests would pass on the racy code.
func loadTileDefs(t *testing.T) {
	t.Helper()
	if err := world.LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		t.Fatal(err)
	}
}

// The primer paints from several goroutines. Before the fix each paint appended
// to the level's DirtyColumns unlocked, which panicked about one run in twenty
// and, with tile definitions loaded, trips the race detector.
func TestPlanetPrimerRepeatedGenerationIsSafe(t *testing.T) {
	loadTileDefs(t)
	const runs = 60
	for seed := int64(0); seed < runs; seed++ {
		level := world.NewLevel(128, 128, 13)
		if err := (PlanetPrimer{}).Prime(level, nil, seed); err != nil {
			t.Fatal(err)
		}
		if level.GetTilePtr(5, 5, 0).Floor.IsEmpty() {
			t.Fatalf("seed %d: primer painted nothing", seed)
		}
		if got, want := len(level.DirtyColumns), 128*128; got != want {
			t.Fatalf("seed %d: %d dirty columns, want one per column (%d)", seed, got, want)
		}
	}
}

// Painting without the dirty list must not change what gets painted: the cell
// helper and the serial paintKind agree on every terrain kind, and only
// paintKind marks the column.
func TestPaintKindCellMatchesPaintKind(t *testing.T) {
	loadTileDefs(t)
	kinds := []world.TerrainKind{
		world.TKBedrock, world.TKUnderground, world.TKCavern, world.TKSubsurface,
		world.TKSurface, world.TKAtmosphere, world.TKSpace,
	}
	for _, k := range kinds {
		a, b := world.NewLevel(4, 4, 4), world.NewLevel(4, 4, 4)
		paintKind(a, 1, 2, 3, k)
		painted := paintKindCell(b, 1, 2, 3, k)
		if !painted {
			t.Errorf("kind %v: painted nothing", k)
		}

		ta, tb := a.GetTilePtr(1, 2, 3), b.GetTilePtr(1, 2, 3)
		if ta.Floor != tb.Floor || ta.Middle != tb.Middle {
			t.Errorf("kind %v: slots differ: %+v/%+v vs %+v/%+v", k, ta.Floor, ta.Middle, tb.Floor, tb.Middle)
		}
		if a.GetTerrainKind(1, 2, 3) != b.GetTerrainKind(1, 2, 3) {
			t.Errorf("kind %v: terrain kind differs", k)
		}
		if len(b.DirtyColumns) != 0 {
			t.Errorf("kind %v: paintKindCell touched DirtyColumns", k)
		}
		if painted != (len(a.DirtyColumns) > 0) {
			t.Errorf("kind %v: painted=%v but paintKind dirtied %d columns", k, painted, len(a.DirtyColumns))
		}
	}
}
