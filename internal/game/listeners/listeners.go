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

type KillListener struct{}

func (l *KillListener) HandleEvent(e event.EventData) error {
	if ke, ok := e.(eventsystem.EntityKilledEvent); ok {
		message.AddMessage(fmt.Sprintf("%s eliminated %s", ke.KillerName, ke.TargetName))
	}
	return nil
}

type TaskListener struct{}

func (l *TaskListener) HandleEvent(e event.EventData) error {
	if te, ok := e.(eventsystem.TaskCompletedEvent); ok {
		message.AddMessage(te.Message)
	}
	return nil
}

type StructureListener struct{}

func (l *StructureListener) HandleEvent(e event.EventData) error {
	if se, ok := e.(eventsystem.StructureBuiltEvent); ok {
		message.AddMessage(fmt.Sprintf("Built %s at [%d,%d,%d]", se.Type, se.X, se.Y, se.Z))
	}
	return nil
}

type ResearchListener struct{}

func (l *ResearchListener) HandleEvent(e event.EventData) error {
	if re, ok := e.(eventsystem.ResearchDoneEvent); ok {
		message.AddMessage(fmt.Sprintf("Research complete: %s", re.TechKey))
	}
	return nil
}
