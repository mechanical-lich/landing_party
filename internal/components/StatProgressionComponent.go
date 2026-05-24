package components

import "github.com/mechanical-lich/mlge/ecs"

// StatLevel tracks progression for a single stat.
type StatLevel struct {
	Level int
	XP    int
}

// StatProgressionComponent tracks per-stat XP and level for a colonist.
// Each stat can gain at most 10 levels (+10 to the base stat value).
// XP threshold to reach the next level: (currentLevel+1)^2 * 100.
type StatProgressionComponent struct {
	Str StatLevel
	Int StatLevel
	Dex StatLevel
	Con StatLevel
}

func (s *StatProgressionComponent) GetType() ecs.ComponentType { return StatProgression }
