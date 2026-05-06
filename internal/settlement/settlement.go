package settlement

import (
	"image/color"

	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/scifi_settlements/internal/lore"
)

var Settlements map[string]*Settlement = make(map[string]*Settlement)

type Settlement struct {
	Name      string
	Color     color.Color
	CenterX   int
	CenterY   int
	CenterZ   int
	Tasks     task.TaskScheduler
	KnownTechs []string
}

func NewSettlement(centerX, centerY, centerZ int) *Settlement {
	name := ""
	for _, ok := Settlements[name]; ok || name == ""; {
		name = lore.RandomSettlementName()
		_, ok = Settlements[name]
	}
	s := &Settlement{
		Name:    name,
		CenterX: centerX,
		CenterY: centerY,
		CenterZ: centerZ,
	}
	Settlements[name] = s
	return s
}

func (s *Settlement) HasTech(key string) bool {
	for _, t := range s.KnownTechs {
		if t == key {
			return true
		}
	}
	return false
}

func (s *Settlement) UnlockTech(key string) {
	if !s.HasTech(key) {
		s.KnownTechs = append(s.KnownTechs, key)
	}
}
