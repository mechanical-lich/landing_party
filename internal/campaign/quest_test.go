package campaign

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

func techRule(key string) objective.Rule {
	return objective.Rule{Trigger: objective.TriggerTechResearched, TechKey: key}
}

// A quest persisted as "active" from before it gained a requires_quest must be
// re-gated on bind, and must never complete before its prerequisite.
func TestStalePrerequisiteReGated(t *testing.T) {
	c := &Campaign{
		Locations: map[string]*Location{},
		QuestDefs: []Quest{
			{ID: "a", Objective: techRule("t1")},
			{ID: "b", RequiresQuest: "a", AutoAccept: true, Objective: techRule("t2")},
		},
		// Stale save: b was active before "a" existed as a prerequisite.
		Quests: map[string]*QuestProgress{
			"a": {Status: QuestAvailable},
			"b": {Status: QuestActive},
		},
	}
	c.BindQuests()

	if got := c.Quests["b"].Status; got != QuestHidden {
		t.Fatalf("b should be re-hidden (prereq incomplete), got %s", got)
	}

	// Objective for b is met, but a is not complete — b must not complete.
	c.AcceptQuest("a")
	c.EvaluateQuests(objective.EvalContext{KnownTechs: []string{"t2"}})
	if c.Quests["b"].Status == QuestCompleted {
		t.Fatal("b completed before prerequisite a")
	}

	// Complete a -> b unlocks (auto_accept -> active) -> then completes.
	if done := c.EvaluateQuests(objective.EvalContext{KnownTechs: []string{"t1"}}); len(done) != 1 || done[0].ID != "a" {
		t.Fatalf("expected a to complete, got %v", done)
	}
	if got := c.Quests["b"].Status; got != QuestActive {
		t.Fatalf("b should auto-activate after a, got %s", got)
	}
	c.EvaluateQuests(objective.EvalContext{KnownTechs: []string{"t1", "t2"}})
	if got := c.Quests["b"].Status; got != QuestCompleted {
		t.Fatalf("b should complete after prerequisite, got %s", got)
	}
}

func TestPrerequisiteChainResolvesInOrder(t *testing.T) {
	c := &Campaign{
		Locations: map[string]*Location{},
		QuestDefs: []Quest{
			{ID: "q1", Objective: techRule("t1")},
			{ID: "q2", RequiresQuest: "q1", Objective: techRule("t2")},
			{ID: "q3", RequiresQuest: "q2", Objective: techRule("t3")},
		},
	}
	c.BindQuests()

	if c.Quests["q1"].Status != QuestAvailable {
		t.Fatalf("q1 should be available, got %s", c.Quests["q1"].Status)
	}
	for _, id := range []string{"q2", "q3"} {
		if c.Quests[id].Status != QuestHidden {
			t.Fatalf("%s should be hidden behind its prereq, got %s", id, c.Quests[id].Status)
		}
	}

	c.AcceptQuest("q1")
	c.EvaluateQuests(objective.EvalContext{KnownTechs: []string{"t1"}})
	if c.Quests["q2"].Status != QuestAvailable {
		t.Fatalf("q2 should unlock after q1, got %s", c.Quests["q2"].Status)
	}
	if c.Quests["q3"].Status != QuestHidden {
		t.Fatalf("q3 should still be hidden (q2 not done), got %s", c.Quests["q3"].Status)
	}

	c.AcceptQuest("q2")
	c.EvaluateQuests(objective.EvalContext{KnownTechs: []string{"t1", "t2"}})
	if c.Quests["q3"].Status != QuestAvailable {
		t.Fatalf("q3 should unlock after q2, got %s", c.Quests["q3"].Status)
	}
}

// A target_killed quest completes only after its tagged target is marked
// killed (MarkTargetKilled), and only for the matching quest id.
func TestTargetKilledQuestCompletes(t *testing.T) {
	c := &Campaign{
		Locations: map[string]*Location{},
		QuestDefs: []Quest{
			{ID: "hunt", AutoAccept: true, Objective: objective.Rule{
				Trigger: objective.TriggerTargetKilled, Target: "hunt",
			}},
		},
	}
	c.BindQuests()

	ctx := func() objective.EvalContext {
		return objective.EvalContext{KilledTargets: c.KilledTargets}
	}
	if d := c.EvaluateQuests(ctx()); len(d) != 0 {
		t.Fatal("hunt completed before target killed")
	}
	c.MarkTargetKilled("someone_else")
	if d := c.EvaluateQuests(ctx()); len(d) != 0 {
		t.Fatal("hunt completed for the wrong target")
	}
	c.MarkTargetKilled("hunt")
	if d := c.EvaluateQuests(ctx()); len(d) != 1 || d[0].ID != "hunt" {
		t.Fatalf("hunt should complete after its target died, got %v", d)
	}
	if c.Quests["hunt"].Status != QuestCompleted {
		t.Fatalf("status = %s, want completed", c.Quests["hunt"].Status)
	}
}

// A datapad-delivery quest starts hidden, is not offered/active, and only
// UnlockQuest moves it to available (never straight to active).
func TestDatapadQuestUnlock(t *testing.T) {
	c := &Campaign{
		Locations: map[string]*Location{},
		QuestDefs: []Quest{
			{ID: "dp", Delivery: "datapad", Objective: techRule("t1")},
		},
	}
	c.BindQuests()

	if got := c.Quests["dp"].Status; got != QuestHidden {
		t.Fatalf("datapad quest should start hidden, got %s", got)
	}
	if len(c.AvailableQuests()) != 0 || len(c.ActiveQuests()) != 0 {
		t.Fatal("hidden datapad quest must not be offered or active")
	}
	if c.AcceptQuest("dp") {
		t.Fatal("a hidden quest can't be accepted directly")
	}
	q := c.UnlockQuest("dp")
	if q == nil || q.ID != "dp" {
		t.Fatalf("UnlockQuest should return the quest, got %v", q)
	}
	if c.Quests["dp"].Status != QuestAvailable {
		t.Fatalf("unlocked quest should be available (not active), got %s", c.Quests["dp"].Status)
	}
	if c.UnlockQuest("dp") != nil {
		t.Fatal("re-unlocking a non-hidden quest should be a no-op")
	}
	if !c.AcceptQuest("dp") {
		t.Fatal("unlocked quest should now be acceptable")
	}
}

func TestAcceptQuestOnlyFromAvailable(t *testing.T) {
	c := &Campaign{
		Locations: map[string]*Location{},
		QuestDefs: []Quest{{ID: "x", Objective: techRule("t")}},
	}
	c.BindQuests()
	if !c.AcceptQuest("x") {
		t.Fatal("should accept an available quest")
	}
	if c.AcceptQuest("x") {
		t.Fatal("should not re-accept an active quest")
	}
	if c.AcceptQuest("missing") {
		t.Fatal("should not accept an unknown quest")
	}
}
