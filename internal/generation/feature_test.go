package generation

import (
	"math/rand"
	"testing"
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
