package progression

import (
	"fmt"
	"log"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

const (
	maxLevel      = 10
	xpScaleFactor = 100
	XPPerTask     = 25
)

// xpThreshold returns the XP needed to advance from the given level to the next.
func xpThreshold(level int) int {
	n := level + 1
	return n * n * xpScaleFactor
}

// AwardXP grants XP toward the named stat for the given entity.
// If the entity lacks a StatProgressionComponent it is a no-op.
// On level-up the corresponding stat in StatsComponent is incremented.
func AwardXP(entity *ecs.Entity, stat string, amount int) {
	if !entity.HasComponent(components.StatProgression) {
		return
	}
	prog := entity.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
	sl := statPtr(prog, stat)
	if sl == nil || sl.Level >= maxLevel {
		return
	}

	sl.XP += amount
	threshold := xpThreshold(sl.Level)
	if sl.XP < threshold {
		return
	}

	sl.XP -= threshold
	sl.Level++
	incrementStat(entity, stat)
	name := rlentity.GetName(entity)
	log.Printf("[PROGRESSION] %s: %s increased to level %d", name, stat, sl.Level)
	message.PostMessage(name, fmt.Sprintf("%s increased to %d!", stat, sl.Level))
}

func statPtr(prog *components.StatProgressionComponent, stat string) *components.StatLevel {
	switch stat {
	case "Str":
		return &prog.Str
	case "Int":
		return &prog.Int
	case "Dex":
		return &prog.Dex
	case "Con":
		return &prog.Con
	}
	return nil
}

func incrementStat(entity *ecs.Entity, stat string) {
	if !entity.HasComponent(rlcomponents.Stats) {
		return
	}
	sc := entity.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
	switch stat {
	case "Str":
		sc.Str++
	case "Int":
		sc.Int++
	case "Dex":
		sc.Dex++
	case "Con":
		sc.Con++
	}
}
