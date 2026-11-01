package listeners

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
)

type MessageListener struct{}

func (l *MessageListener) HandleEvent(e event.EventData) error {
	if me, ok := e.(message.MessageEvent); ok {
		message.AddMessage(fmt.Sprintf("[%s] %s", me.Sender, me.Message))
	}
	return nil
}

// KillListener, StructureListener and ResearchListener write a line through Post,
// which decides whether it reaches the live log or a notification.
type KillListener struct{ Post func(string) }

func (l *KillListener) HandleEvent(e event.EventData) error {
	if ke, ok := e.(eventsystem.EntityKilledEvent); ok {
		l.Post(fmt.Sprintf("%s eliminated %s", ke.KillerName, ke.TargetName))
	}
	return nil
}

type StructureListener struct{ Post func(string) }

func (l *StructureListener) HandleEvent(e event.EventData) error {
	if se, ok := e.(eventsystem.StructureBuiltEvent); ok {
		l.Post(fmt.Sprintf("Built %s at [%d,%d,%d]", se.Type, se.X, se.Y, se.Z))
	}
	return nil
}

type ResearchListener struct{ Post func(string) }

func (l *ResearchListener) HandleEvent(e event.EventData) error {
	if re, ok := e.(eventsystem.ResearchDoneEvent); ok {
		l.Post(fmt.Sprintf("Research complete: %s", re.TechKey))
	}
	return nil
}
