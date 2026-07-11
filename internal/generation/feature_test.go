package generation

import (
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

func TestRollCount(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	// Fixed count (no CountMax) always returns Count.
	for i := 0; i < 50; i++ {
		if got := rollCount(FeatureSpec{Count: 6}, rng, 3); got != 6 {
			t.Fatalf("fixed: got %d, want 6", got)
		}
	}

	// Count unset falls back to the placer default.
	if got := rollCount(FeatureSpec{}, rng, 4); got != 4 {
		t.Fatalf("default: got %d, want 4", got)
	}

	// CountMax <= Count degrades to a fixed count.
	if got := rollCount(FeatureSpec{Count: 5, CountMax: 5}, rng, 1); got != 5 {
		t.Fatalf("equal max: got %d, want 5", got)
	}

	// CountMax > Count rolls within [Count, CountMax] and spans the range.
	sawLow, sawHigh := false, false
	for i := 0; i < 2000; i++ {
		got := rollCount(FeatureSpec{Count: 4, CountMax: 8}, rng, 1)
		if got < 4 || got > 8 {
			t.Fatalf("range: got %d out of [4,8]", got)
		}
		sawLow = sawLow || got == 4
		sawHigh = sawHigh || got == 8
	}
	if !sawLow || !sawHigh {
		t.Fatalf("range roll never hit an endpoint (low=%v high=%v)", sawLow, sawHigh)
	}
}

// Same seed must reproduce the same roll sequence — sibling locations differ
// only because their seeds differ, not because the roll is nondeterministic.
func TestRollCountDeterministic(t *testing.T) {
	s := FeatureSpec{Count: 2, CountMax: 20}
	seq := func() []int {
		rng := rand.New(rand.NewSource(42))
		out := make([]int, 10)
		for i := range out {
			out[i] = rollCount(s, rng, 1)
		}
		return out
	}
	a, b := seq(), seq()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d: %d vs %d", i, a[i], b[i])
		}
	}
}

// A biome-restricted feature must fill its count by sampling that biome's
// columns directly — even when the biome is a tiny fraction of a large map,
// which would starve a uniform sampler. This is the fix for the recurring
// "placed 1/20 ... biome too small" shortfalls.
func TestPlaceAnchorsBiomeTargeted(t *testing.T) {
	level := world.NewLevel(100, 100, 3)
	level.AllocTerrain()
	// Paint a 10×10 desert block: 100 of 10,000 columns (1%).
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			b := "plains"
			if x < 10 && y < 10 {
				b = "desert"
			}
			level.SetBiome(x, y, b)
		}
	}
	rng := rand.New(rand.NewSource(1))

	var placed [][2]int
	s := FeatureSpec{Kind: "scatter_tile", Count: 20, Biome: "desert"}
	placeAnchors(level, s, rng, 50, func(cx, cy int) bool {
		placed = append(placed, [2]int{cx, cy})
		return true
	})
	if len(placed) != 20 {
		t.Fatalf("placed %d, want 20 (biome-targeted sampling should fill the count)", len(placed))
	}
	for _, c := range placed {
		if got := level.GetBiome(c[0], c[1]); got != "desert" {
			t.Fatalf("placement at (%d,%d) is in %q, want desert", c[0], c[1], got)
		}
	}

	// An absent biome places nothing and doesn't panic.
	got := 0
	absent := FeatureSpec{Kind: "scatter_tile", Count: 5, Biome: "no_such_biome"}
	placeAnchors(level, absent, rng, 50, func(cx, cy int) bool { got++; return true })
	if got != 0 {
		t.Fatalf("absent biome should place 0, got %d", got)
	}
}
