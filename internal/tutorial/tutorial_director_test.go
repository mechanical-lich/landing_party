package tutorial

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

func flagRule(flag string) objective.Rule {
	return objective.Rule{Trigger: objective.TriggerFlagSet, Flag: flag}
}

func testDef() Definition {
	return Definition{Steps: []Step{
		{ID: "a", Context: "dash:starmap", Done: flagRule("ui.a")},
		{ID: "b", Context: ContextSite, Done: flagRule("ui.b")},
		{ID: "c", Context: ContextSite, Done: flagRule("ui.c")},
		{ID: "d", At: AtTutorial, Context: ContextSite, Done: objective.Rule{Trigger: objective.TriggerStructureBuilt, Structure: "storage_locker"}},
		{ID: "e", Context: ContextAny, UnlockNext: true, Done: flagRule("ui.e")},
		{ID: "f", At: AtNext, Context: ContextSite, Done: flagRule("ui.f")},
	}}
}

func snap(at string, flags map[string]any) Snapshot {
	return Snapshot{Context: ContextSite, At: at, Eval: objective.EvalContext{Flags: flags}}
}

func TestDirectorAdvancesOnRuleOnly(t *testing.T) {
	d := New(testDef(), "")
	d.Observe(snap(AtTutorial, map[string]any{"ui.b": true}))
	if d.Current().ID != "a" {
		t.Fatalf("advanced on another step's flag: at %s", d.Current().ID)
	}
	if d.StepChanged() {
		t.Fatal("step change reported with no change")
	}
	d.Observe(snap(AtTutorial, map[string]any{"ui.a": true}))
	if d.Current().ID != "b" || !d.StepChanged() || d.StepChanged() {
		t.Fatalf("after ui.a: at %s", d.Current().ID)
	}
}

// Several already-satisfied steps pass in one Observe.
func TestDirectorCatchesUp(t *testing.T) {
	d := New(testDef(), "")
	d.Observe(snap(AtTutorial, map[string]any{"ui.a": true, "ui.b": true}))
	if d.Current().ID != "c" || d.Index() != 2 {
		t.Fatalf("at %s (%d), want c", d.Current().ID, d.Index())
	}
}

func TestDirectorSkip(t *testing.T) {
	d := New(testDef(), "b")
	d.Skip()
	if !d.Skipped() || d.Current() != nil || d.Done() {
		t.Fatalf("after Skip: skipped=%v current=%v done=%v", d.Skipped(), d.Current(), d.Done())
	}
	if d.StepID() != "b" {
		t.Errorf("StepID after skip = %q, want the step it stopped on", d.StepID())
	}
	d.Observe(snap(AtTutorial, map[string]any{"ui.b": true}))
	if d.Current() != nil {
		t.Fatal("skipped director came back to life")
	}
}

func TestDirectorResumesFromSavedStep(t *testing.T) {
	d := New(testDef(), "d")
	if d.Current().ID != "d" {
		t.Fatalf("resumed at %s", d.Current().ID)
	}
	u := New(testDef(), "no_such_step")
	if u.Current().ID != "a" {
		t.Fatalf("unknown id started at %s", u.Current().ID)
	}
	u.Observe(snap(AtTutorial, map[string]any{"ui.a": true, "ui.b": true}))
	if u.Current().ID != "c" {
		t.Fatalf("unknown id did not catch up: at %s", u.Current().ID)
	}
}

func TestDirectorInContext(t *testing.T) {
	d := New(testDef(), "")
	if !d.InContext("dash:starmap") || d.InContext(ContextSite) || d.InContext("dash:crew") {
		t.Error("dash:starmap step context wrong")
	}
	d = New(testDef(), "b")
	if !d.InContext(ContextSite) || d.InContext(ContextShip) {
		t.Error("site step context wrong")
	}
	d = New(testDef(), "e")
	if !d.InContext(ContextSite) || !d.InContext("dash:quests") || !d.InContext(ContextShip) {
		t.Error("any step context wrong")
	}
	d.Skip()
	if d.InContext(ContextSite) {
		t.Error("skipped director in context")
	}
}

// A step with At does not advance from another location, even satisfied, and
// nothing after it is reached either.
func TestDirectorAtGatesEvaluation(t *testing.T) {
	d := New(testDef(), "d")
	built := objective.EvalContext{StructuresBuilt: map[string]int{"storage_locker": 1}, Flags: map[string]any{"ui.e": true}}
	d.Observe(Snapshot{Context: ContextSite, At: AtNext, Eval: built})
	if d.Current().ID != "d" {
		t.Fatalf("locker at the wrong planet passed step d: at %s", d.Current().ID)
	}
	d.Observe(Snapshot{Context: ContextSite, At: "", Eval: built})
	if d.Current().ID != "d" {
		t.Fatalf("locker nowhere in particular passed step d: at %s", d.Current().ID)
	}
	d.Observe(Snapshot{Context: ContextSite, At: AtTutorial, Eval: built})
	if d.Current().ID != "f" {
		t.Fatalf("at the tutorial planet: at %s, want f (d and e pass)", d.Current().ID)
	}
	d.Observe(snap(AtTutorial, map[string]any{"ui.f": true}))
	if d.Current().ID != "f" {
		t.Fatal("step f passed away from the next planet")
	}
	d.Observe(snap(AtNext, map[string]any{"ui.f": true}))
	if d.Current() != nil || !d.Done() || d.StepID() != "f" {
		t.Fatalf("not finished: current=%v done=%v id=%q", d.Current(), d.Done(), d.StepID())
	}
}

func TestDirectorNextUnlocked(t *testing.T) {
	d := New(testDef(), "")
	if d.NextUnlocked() {
		t.Fatal("unlocked before the unlock_next step")
	}
	d = New(testDef(), "d")
	if d.NextUnlocked() {
		t.Fatal("unlocked at step d")
	}
	// d and e pass in one Observe; the unlock must not be missed.
	d.Observe(Snapshot{Context: ContextSite, At: AtTutorial, Eval: objective.EvalContext{
		StructuresBuilt: map[string]int{"storage_locker": 1}, Flags: map[string]any{"ui.e": true}}})
	if d.Current().ID != "f" || !d.NextUnlocked() {
		t.Fatalf("at %s unlocked=%v", d.Current().ID, d.NextUnlocked())
	}
	e := New(testDef(), "e")
	if !e.NextUnlocked() {
		t.Fatal("not unlocked at the unlock_next step itself")
	}
}

func TestValidateRejectsBadData(t *testing.T) {
	good := testDef()
	if err := good.Validate(); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
	cases := map[string]func(*Definition){
		"duplicate id":  func(d *Definition) { d.Steps[1].ID = "a" },
		"bad context":   func(d *Definition) { d.Steps[0].Context = "nowhere" },
		"bad at":        func(d *Definition) { d.Steps[0].At = "elsewhere" },
		"missing topic": func(d *Definition) { d.Steps[0].Topic = "ghost" },
		"no rule":       func(d *Definition) { d.Steps[0].Done = objective.Rule{} },
		"gathered no op": func(d *Definition) {
			d.Steps[0].Done = objective.Rule{Trigger: objective.TriggerResourceGathered, Resource: "food", Threshold: 3}
		},
		"flag no name": func(d *Definition) { d.Steps[0].Done = objective.Rule{Trigger: objective.TriggerFlagSet} },
	}
	for name, mutate := range cases {
		d := testDef()
		mutate(&d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
