package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/tutorial"
)

// The Field Manual leads the Encyclopedia whatever the tutorial's state, a
// topic can be selected by ID, and the selection survives a refresh.
func TestEncyclopediaFieldManual(t *testing.T) {
	w := newTutorialWorld(t, 5)
	e := w.d.encyclopedia()
	if e == nil {
		t.Fatal("no Encyclopedia tab")
	}
	e.Enter()
	if len(e.manual) == 0 || len(e.blueprints) < len(e.manual) {
		t.Fatalf("manual has %d topics, list has %d rows", len(e.manual), len(e.blueprints))
	}
	for i, tp := range e.manual {
		if e.blueprints[i] != topicKeyPrefix+tp.ID {
			t.Errorf("row %d = %q, want topic %s first", i, e.blueprints[i], tp.ID)
		}
	}
	e.selectTopic("hauling")
	if tp := e.selectedTopic(); tp == nil || tp.ID != "hauling" {
		t.Fatalf("selected topic = %v", tp)
	}
	e.populate()
	if tp := e.selectedTopic(); tp == nil || tp.ID != "hauling" {
		t.Errorf("selection lost on refresh: %v", tp)
	}
	e.selectTopic("no_such_topic")
	if tp := e.selectedTopic(); tp == nil || tp.ID != "hauling" {
		t.Errorf("unknown topic changed the selection: %v", tp)
	}
	// A catalogue entry still works as before.
	w.c.AddKnownEntity("colonist")
	e.populate()
	e.list.SelectedIndex = len(e.manual)
	if e.selectedTopic() != nil || e.selectedBlueprint() != "colonist" {
		t.Errorf("catalogue row = %q topic=%v", e.selectedBlueprint(), e.selectedTopic())
	}

	// With the tutorial off the manual is still there.
	w.c.Tutorial.Skipped = true
	fresh := NewDashboardState(w.c, w.wm)
	if fe := fresh.encyclopedia(); fe == nil || len(fe.manual) != len(e.manual) {
		t.Error("manual missing with the tutorial skipped")
	}
}

// [Manual] opens the step's topic: on the dashboard by switching tab, from a
// level by opening the dashboard on the Encyclopedia.
func TestTutorialManualButton(t *testing.T) {
	w := newTutorialWorld(t, 5)
	b := w.b
	step := b.dir.Current()
	if step.Topic == "" {
		t.Fatalf("step %s has no topic", step.ID)
	}
	w.d.selectTabByID("starmap")
	b.step("dash:starmap", nil, w.d)
	b.panel.manualBtn.OnClick()
	if w.d.tabs[w.d.active].id != "encyclopedia" {
		t.Fatalf("active tab = %s", w.d.tabs[w.d.active].id)
	}
	if tp := w.d.encyclopedia().selectedTopic(); tp == nil || tp.ID != step.Topic {
		t.Fatalf("selected topic = %v, want %s", tp, step.Topic)
	}

	w.enterSite()
	s := w.site
	b.step(tutorial.ContextSite, s, nil)
	b.panel.manualBtn.OnClick()
	d, ok := s.next.(*DashboardState)
	if !ok || !s.done {
		t.Fatalf("Manual from the site did not open the dashboard: next=%T done=%v", s.next, s.done)
	}
	if d.tabs[d.active].id != "encyclopedia" {
		t.Errorf("dashboard opened on %s", d.tabs[d.active].id)
	}
	if tp := d.encyclopedia().selectedTopic(); tp == nil || tp.ID != step.Topic {
		t.Errorf("selected topic = %v", tp)
	}
	if !strings.HasPrefix(d.encyclopedia().selectedBlueprint(), topicKeyPrefix) {
		t.Error("selected row is not a manual row")
	}
}
