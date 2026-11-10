package game

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// SettlementStats is how a settlement is doing (phase C1): the vmtest
// sidecar's settlement_stats and the soak test's measure.
type SettlementStats struct {
	Alive int `json:"alive"` // colony workers alive on the level
	Dead  int `json:"dead"`  // colonists who died on it
	Beds  int `json:"beds"`
	// Lockers is storage containers the colony owns (lockers, bins, pads).
	Lockers int `json:"lockers"`
	// Food is food units (rations, bars, gels) stored or carried.
	Food int `json:"food"`
	// BuildQueue is open build orders; Tasks every open order.
	BuildQueue int `json:"build_queue"`
	Tasks      int `json:"tasks"`
	Research   bool `json:"research"` // a research order is open
	Techs      int  `json:"techs"`
	Day        int  `json:"day"`
	Tick       int  `json:"tick"`
	// Built is the structures the colony finished on this level, by type
	// (StructureBuilt events); Ordered and Cancelled the director's orders.
	Built     map[string]int `json:"built"`
	Ordered   map[string]int `json:"ordered,omitempty"`
	Cancelled int            `json:"cancelled"`
}

func (s *MainState) settlementStats() SettlementStats {
	st := SettlementStats{Dead: s.colonistsDied, Techs: len(s.knownTechs()), Tick: s.tick, Built: map[string]int{}}
	for k, v := range s.builtHere {
		st.Built[k] = v
	}
	if s.campaign != nil {
		st.Day = int(s.campaign.Day())
	} else {
		st.Day = s.day
	}
	colony := s.colonyName()
	food := func(items []*ecs.Entity) {
		for _, it := range items {
			if it != nil && it.HasComponent(rlcomponents.Food) {
				st.Food++
			}
		}
	}
	for _, list := range [][]*ecs.Entity{s.level.Entities, s.level.StaticEntities} {
		for _, e := range list {
			if e == nil {
				continue
			}
			if components.IsColonyWorker(e, colony) {
				st.Alive++
				if e.HasComponent(rlcomponents.Inventory) {
					food(e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag)
				}
				continue
			}
			if e.HasComponent(components.Bed) {
				st.Beds++
			}
			if e.HasComponent(components.Storage) {
				sc := e.GetComponent(components.Storage).(*components.StorageComponent)
				if sc.OwnedBy == colony {
					st.Lockers++
					food(sc.Items)
				}
			}
		}
	}
	if s.MainSettlement != nil {
		for _, t := range s.MainSettlement.Tasks.GetTasks() {
			if t.Completed {
				continue
			}
			st.Tasks++
			switch t.Action {
			case task_requests.BuildAction:
				st.BuildQueue++
			case task_requests.ResearchAction:
				st.Research = true
			}
		}
	}
	if d := s.director; d != nil {
		st.Ordered = map[string]int{}
		for k, v := range d.Ordered {
			st.Ordered[k] = v
		}
		st.Cancelled = d.Cancelled
	}
	return st
}
