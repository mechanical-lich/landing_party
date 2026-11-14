package generation

import (
	"fmt"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// The asteroid primer tags its rocks in the same order every time: largest
// first, equal sizes by label. Features sample the "asteroid" region by index
// (ore veins, radiation pockets), so a tie that came out in map order put the
// same seed's ore on a different rock from run to run (vmtest lore_ufo).
func TestAsteroidRegionsAreRepeatable(t *testing.T) {
	loadPlacementData(t)
	for seed := int64(1); seed <= 40; seed++ {
		var first string
		for i := 0; i < 8; i++ {
			level := world.NewLevel(96, 96, 6)
			if err := (AsteroidFieldPrimer{}).Prime(level, map[string]any{"min_size": 20}, seed); err != nil {
				t.Fatal(err)
			}
			got := fmt.Sprint(level.Regions["asteroid"])
			if i == 0 {
				first = got
				continue
			}
			if got != first {
				t.Fatalf("seed %d: the asteroid regions came out in a different order on prime %d", seed, i+1)
			}
		}
	}
}
