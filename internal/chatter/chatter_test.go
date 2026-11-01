package chatter

import (
	"testing"

	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
)

type countingListener struct{ n int }

func (l *countingListener) HandleEvent(e event.EventData) error {
	if _, ok := e.(message.MessageEvent); ok {
		l.n++
	}
	return nil
}

func TestPost_DroppedDuringBackgroundStep(t *testing.T) {
	l := &countingListener{}
	event.GetQueuedInstance().RegisterListener(l, message.MessageEventType)
	t.Cleanup(func() { SetBackground(false) })

	SetBackground(true)
	Post("worker", "Ate ration")
	event.GetQueuedInstance().HandleQueue()
	if l.n != 0 {
		t.Fatalf("background chatter reached the log: %d messages", l.n)
	}

	SetBackground(false)
	Post("worker", "Ate ration")
	event.GetQueuedInstance().HandleQueue()
	if l.n != 1 {
		t.Fatalf("live chatter should post once, got %d", l.n)
	}
}
