package objective

import "testing"

func newTestEval() *Evaluator {
	return New(RuleSet{Rules: []Rule{
		{ID: "survive", Trigger: TriggerDaysSurvived, Op: "gte", Threshold: 30},
		{ID: "wiped", Trigger: TriggerColonistEliminated, Op: "eq", Threshold: 0},
		{ID: "hoard", Trigger: TriggerResourceGathered, Resource: "metal_ore", Op: "gte", Threshold: 100},
		{ID: "purge", Trigger: TriggerEntityKilled, Blueprint: "alien_egg", Op: "lte", Threshold: 0},
	}})
}

func alive(colony int) map[string]int { return map[string]int{"colony": colony} }

func TestEvaluateDispatch(t *testing.T) {
	cases := []struct {
		name string
		ctx  EvalContext
		want string // rule ID, "" = none
	}{
		{"nothing", EvalContext{Day: 5, SettlementPop: alive(4)}, ""},
		{"survived", EvalContext{Day: 30, SettlementPop: alive(4)}, "survive"},
		{"eliminated", EvalContext{Day: 5, SettlementPop: alive(0)}, "wiped"},
		{"gathered", EvalContext{Day: 1, SettlementPop: alive(4), ResourceCounts: map[string]int{"metal_ore": 150}}, "hoard"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, ok := newTestEval().Evaluate(c.ctx)
			if c.want == "" {
				if ok {
					t.Fatalf("expected no rule, got %s", r.ID)
				}
				return
			}
			if !ok || r.ID != c.want {
				t.Fatalf("want %s, got ok=%v rule=%v", c.want, ok, r)
			}
		})
	}
}

// "Kill all of X" must not fire before X has ever existed, and must fire once
// every X is gone.
func TestEvaluateKillAllNeedsPriorExistence(t *testing.T) {
	e := newTestEval()

	if _, ok := e.Evaluate(EvalContext{Day: 1, SettlementPop: alive(4)}); ok {
		t.Fatal("kill-all fired before any alien_egg existed")
	}
	// Eggs appear.
	if _, ok := e.Evaluate(EvalContext{Day: 2, SettlementPop: alive(4), EntityCounts: map[string]int{"alien_egg": 3}}); ok {
		t.Fatal("rule fired while eggs still alive")
	}
	// Eggs cleared.
	r, ok := e.Evaluate(EvalContext{Day: 3, SettlementPop: alive(4), EntityCounts: map[string]int{"alien_egg": 0}})
	if !ok || r.ID != "purge" {
		t.Fatalf("expected purge after eggs cleared, got ok=%v rule=%v", ok, r)
	}
}
