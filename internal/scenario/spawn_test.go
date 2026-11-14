package scenario

import (
	"math/rand"
	"testing"
)

// The same roll always picks the same creature: PickRandom walks the rules
// in key order, not the map's (random) order (lore_ufo's crew came out a
// worker in one run and a heavy in the next).
func TestPickRandomIsRepeatable(t *testing.T) {
	rules := map[string]SpawnRule{}
	for _, bp := range []string{"greyish", "greenish", "whitish", "pinkish", "skimmer", "lancer", "xeno", "yeti"} {
		rules[bp] = SpawnRule{SpawnRate: 10}
	}
	var first string
	for i := 0; i < 50; i++ {
		rand.Seed(42) //nolint:staticcheck // the sim's global source, as the game seeds it
		got := PickRandom(rules, "grass", 50, 3, 3)
		if got == "" {
			t.Fatal("nothing picked")
		}
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("pick %d is %q, the first was %q", i, got, first)
		}
	}
}
