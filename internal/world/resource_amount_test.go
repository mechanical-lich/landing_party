package world

import "testing"

func TestRollDepositRichness_StaysInRange(t *testing.T) {
	ranges := map[string][2]int{
		"ore_deposit":     {25, 100},
		"crystal_vein":    {15, 60},
		"radioactive_ore": {10, 40},
	}
	for name, r := range ranges {
		for i := 0; i < 1000; i++ {
			v := RollDepositRichness(name)
			if v < r[0] || v > r[1] {
				t.Fatalf("%s: roll %d out of range [%d,%d]", name, v, r[0], r[1])
			}
		}
	}
	if RollDepositRichness("rock") != 0 {
		t.Errorf("non-deposit tile should roll 0")
	}
}

func TestConsumeResource_DecrementsAndClamps(t *testing.T) {
	l := NewLevel(4, 4, 2)
	l.SetResourceAmount(1, 1, 1, 25)

	if got := l.ConsumeResource(1, 1, 1, 10); got != 10 {
		t.Fatalf("first chunk: got %d, want 10", got)
	}
	if rem := l.ResourceAmountAt(1, 1, 1); rem != 15 {
		t.Fatalf("remaining: got %d, want 15", rem)
	}
	// Final chunk clamps to what's left (5, not the requested 10).
	l.ConsumeResource(1, 1, 1, 10)
	if got := l.ConsumeResource(1, 1, 1, 10); got != 5 {
		t.Fatalf("final chunk should clamp to 5, got %d", got)
	}
	// Depleted: entry removed, further consumption yields nothing.
	if rem := l.ResourceAmountAt(1, 1, 1); rem != 0 {
		t.Fatalf("depleted deposit should read 0, got %d", rem)
	}
	if got := l.ConsumeResource(1, 1, 1, 10); got != 0 {
		t.Fatalf("empty deposit should yield 0, got %d", got)
	}
}

// The cancel/re-issue exploit is closed because the deposit amount — not the
// task's progress — is the source of truth. Re-mining a partially-mined deposit
// only ever draws down the remaining amount; the total extracted can never
// exceed what was rolled.
func TestConsumeResource_NoReRollExploit(t *testing.T) {
	l := NewLevel(4, 4, 2)
	l.SetResourceAmount(2, 2, 1, 30)

	total := 0
	// Simulate many "start mining, cancel" cycles, each pulling one chunk.
	for i := 0; i < 100; i++ {
		total += l.ConsumeResource(2, 2, 1, 10)
	}
	if total != 30 {
		t.Fatalf("total extracted %d should equal the rolled 30 regardless of restarts", total)
	}
}
