package generation

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

func conversionLevel() *world.Level {
	lvl := world.NewLevel(30, 30, 1)
	lvl.AllocTerrain()
	for x := 0; x < 30; x++ {
		for y := 0; y < 30; y++ {
			lvl.SetFloor(x, y, 0, "hull_floor", 0)
		}
	}
	lvl.SetMiddle(15, 12, 0, "hull_wall", 0) // solid: never converted
	return lvl
}

func floorsOf(lvl *world.Level, tile string) map[[2]int]bool {
	out := map[[2]int]bool{}
	for x := 0; x < 30; x++ {
		for y := 0; y < 30; y++ {
			if t := lvl.GetTileAt(x, y, 0).(*world.Tile); world.TileDefinitions[t.Floor.Type].Name == tile {
				out[[2]int{x, y}] = true
			}
		}
	}
	return out
}

// A presence's Conversion share is a patch of the doctrine's tile around its
// centre: the same tiles for the same seed, nearest the centre first, the
// share read back, a second application adding nothing, and solid ground
// left alone.
func TestConversionPatchDeterministic(t *testing.T) {
	loadPlacementData(t)
	a, b := conversionLevel(), conversionLevel()
	na := ApplyConversion(a, 15, 15, 0, 6, "blue_wires", 0.4, 77)
	nb := ApplyConversion(b, 15, 15, 0, 6, "blue_wires", 0.4, 77)
	fa, fb := floorsOf(a, "blue_wires"), floorsOf(b, "blue_wires")
	if na == 0 || na != nb || len(fa) != na {
		t.Fatalf("converted %d and %d (%d tiles)", na, nb, len(fa))
	}
	for k := range fa {
		if !fb[k] {
			t.Fatalf("same seed, different patch at %v", k)
		}
	}
	if !fa[[2]int{15, 15}] || fa[[2]int{15, 12}] {
		t.Error("the centre should be converted first and the wall never")
	}
	if s := ConversionShare(a, 15, 15, 0, 6, "blue_wires"); s < 0.38 || s > 0.42 {
		t.Errorf("share read back %.3f, want 0.4", s)
	}
	if n := ApplyConversion(a, 15, 15, 0, 6, "blue_wires", 0.4, 77); n != 0 {
		t.Errorf("second application converted %d more", n)
	}
	c := conversionLevel()
	ApplyConversion(c, 15, 15, 0, 6, "blue_wires", 0.4, 78)
	diff := 0
	for k := range floorsOf(c, "blue_wires") {
		if !fa[k] {
			diff++
		}
	}
	if diff == 0 {
		t.Log("another seed gave the same edge (possible, but unlikely)")
	}
}

// Ground converted elsewhere in the radius (a live director lays by reach,
// not nearest first) counts toward the share: applying the share it already
// has adds nothing, so a thaw does not ratchet it up (review finding 4).
func TestConversionCountsWhatIsAlreadyLaid(t *testing.T) {
	loadPlacementData(t)
	a := conversionLevel()
	// Lay the east half of the disc, not the nearest-first ring.
	for dy := -6; dy <= 6; dy++ {
		for dx := 1; dx <= 6; dx++ {
			if dx*dx+dy*dy <= 36 {
				a.SetFloor(15+dx, 15+dy, 0, "blue_wires", 0)
			}
		}
	}
	before := ConversionShare(a, 15, 15, 0, 6, "blue_wires")
	if n := ApplyConversion(a, 15, 15, 0, 6, "blue_wires", before, 77); n != 0 {
		t.Errorf("applying the share already laid converted %d more", n)
	}
	if after := ConversionShare(a, 15, 15, 0, 6, "blue_wires"); after != before {
		t.Errorf("share %.3f became %.3f with no conversion", before, after)
	}
	// A larger share lays only the shortfall.
	ApplyConversion(a, 15, 15, 0, 6, "blue_wires", 0.8, 77)
	if s := ConversionShare(a, 15, 15, 0, 6, "blue_wires"); s < 0.78 || s > 0.82 {
		t.Errorf("share %.3f after applying 0.8", s)
	}
}
