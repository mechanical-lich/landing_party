package objective

import "testing"

// flag_set reads one caller-supplied flag: true fires, false and missing do not.
func TestEvaluateFlagSet(t *testing.T) {
	e := New(RuleSet{Rules: []Rule{
		{ID: "panned", Trigger: TriggerFlagSet, Flag: "ui.camera_panned"},
	}})
	if _, ok := e.Evaluate(EvalContext{}); ok {
		t.Fatal("fired with no flags at all")
	}
	if _, ok := e.Evaluate(EvalContext{Flags: map[string]any{"ui.camera_panned": false}}); ok {
		t.Fatal("fired on a false flag")
	}
	if _, ok := e.Evaluate(EvalContext{Flags: map[string]any{"ui.other": true}}); ok {
		t.Fatal("fired on a different flag")
	}
	r, ok := e.Evaluate(EvalContext{Flags: map[string]any{"ui.camera_panned": true}})
	if !ok || r.ID != "panned" {
		t.Fatalf("expected panned, got ok=%v rule=%v", ok, r)
	}
}
