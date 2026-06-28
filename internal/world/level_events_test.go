package world

import (
	"testing"

	"github.com/mechanical-lich/mlge/event"
)

const testEventType event.EventType = "test_sim_event"

type testEvent struct{ tag string }

func (testEvent) GetType() event.EventType { return testEventType }

type recordingListener struct{ got []string }

func (l *recordingListener) HandleEvent(e event.EventData) error {
	if te, ok := e.(testEvent); ok {
		l.got = append(l.got, te.tag)
	}
	return nil
}

// Each level owns an independent event bus, so a sim event queued on one planet
// dispatches only to that planet's listeners — never leaking to another planet.
// This isolation is the whole basis for background simulation.
func TestLevelEvents_AreIsolatedPerLevel(t *testing.T) {
	a := NewLevel(4, 4, 1)
	b := NewLevel(4, 4, 1)

	la, lb := &recordingListener{}, &recordingListener{}
	a.Events.RegisterListener(la, testEventType)
	b.Events.RegisterListener(lb, testEventType)

	a.QueueEvent(testEvent{tag: "from-a"})
	a.Events.HandleQueue()

	if len(la.got) != 1 || la.got[0] != "from-a" {
		t.Fatalf("planet A's listener should have received its own event, got %v", la.got)
	}
	if len(lb.got) != 0 {
		t.Fatalf("planet B's listener must NOT see planet A's event, got %v", lb.got)
	}
}

// HandleQueue dispatches queued events; before it runs nothing fires.
func TestLevelEvents_QueueDrainsOnHandle(t *testing.T) {
	l := NewLevel(4, 4, 1)
	rec := &recordingListener{}
	l.Events.RegisterListener(rec, testEventType)

	l.QueueEvent(testEvent{tag: "x"})
	if len(rec.got) != 0 {
		t.Fatalf("event should not dispatch until HandleQueue")
	}
	l.Events.HandleQueue()
	if len(rec.got) != 1 {
		t.Fatalf("HandleQueue should dispatch the queued event, got %v", rec.got)
	}
}
