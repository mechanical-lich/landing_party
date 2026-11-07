package game

import (
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
)

// loreRecorder writes live history for one level: who killed whom, colonies
// founded, left and wiped, threats cleared, origins reached. It listens to the
// level's own event bus for hits (EntityDamagedEvent carries the attacker;
// EntityKilledEvent only fires for kills by colonists) and is called from the
// cleanup system's death hook, beam-down/up and the level's 30-tick upkeep.
type loreRecorder struct {
	s       *MainState
	lastHit map[*ecs.Entity]string
}

func newLoreRecorder(s *MainState) *loreRecorder {
	return &loreRecorder{s: s, lastHit: map[*ecs.Entity]string{}}
}

// HandleEvent remembers the last attacker of every victim, for death causes.
func (r *loreRecorder) HandleEvent(e event.EventData) error {
	if d, ok := e.(eventsystem.EntityDamagedEvent); ok && d.Victim != nil && d.Attacker != nil {
		r.lastHit[d.Victim] = loreEntityName(d.Attacker)
	}
	return nil
}

// loreEntityName is how the chronicle names an entity: its own name, else its
// blueprint in words.
func loreEntityName(e *ecs.Entity) string {
	if n := rlentity.GetName(e); n != "" && n != "Unknown" {
		return n
	}
	return strings.ReplaceAll(e.Blueprint, "_", " ")
}

// location is the campaign location this level belongs to, or nil (the ship,
// a level outside a campaign).
func (s *MainState) location() *campaign.Location {
	if s.wm == nil || s.campaign == nil || s == s.wm.shipLevel {
		return nil
	}
	for id, ms := range s.wm.loaded {
		if ms == s {
			return s.campaign.Locations[id]
		}
	}
	return nil
}

// livingColonyName is the settlement this level's colonists belong to.
func (s *MainState) livingColonyName() string {
	if s.wm != nil && s == s.wm.shipLevel {
		return s.wm.shipSettlementName()
	}
	return s.colonyName()
}

// recordDeath logs a colonist's or a boss's death, with who killed them, and a
// site's wipe when its last colonist falls. Called from the death hook.
func (r *loreRecorder) recordDeath(e *ecs.Entity) {
	s := r.s
	defer delete(r.lastHit, e)
	if s.campaign == nil || e == nil {
		return
	}
	cause := ""
	if k := r.lastHit[e]; k != "" {
		cause = ", killed by " + k
	}
	loc := s.location()
	vars := map[string]string{"cause": cause}
	if loc == nil && s.wm != nil && s == s.wm.shipLevel {
		vars["place"] = "the ship"
	}
	colony := s.livingColonyName()
	switch {
	case e.HasComponents(components.Worker, components.Settlement) &&
		e.GetComponent(components.Settlement).(*components.SettlementComponent).Name == colony:
		name := loreEntityName(e)
		vars["colonist"] = name
		s.campaign.RecordLive("colonist_died", loc, vars, history.Figure{Name: name, Role: "colonist", Fate: "died"})
		if loc != nil && countLevelColonists(s.level, colony) == 0 {
			s.campaign.RecordLive("colony_wiped", loc, map[string]string{"colony": s.campaign.ColonyDisplayName()})
		}
	case e.HasComponent(components.QuestTarget):
		name := loreEntityName(e)
		vars["boss"] = name
		s.campaign.RecordLive("boss_slain", loc, vars, history.Figure{Name: name, Role: "boss", Fate: "died"})
	}
}

// loreUpkeep runs every 30 ticks: a threat cleared, origins reached.
func (s *MainState) loreUpkeep() {
	if s.campaign == nil || s.level == nil {
		return
	}
	s.checkThreatCleared()
	s.revealReachedOrigins()
}

// Level flags marking a site's threat: seen alive, then cleared.
const (
	loreThreatSeenFlag    = "lore_threat_seen"
	loreThreatClearedFlag = "lore_threat_cleared"
)

// checkThreatCleared records the moment the last of a scenario's finite
// threat (its threat_blueprints: every xeno, every troid) is gone.
func (s *MainState) checkThreatCleared() {
	sc := s.scenario
	if sc == nil || len(sc.ThreatBlueprints) == 0 || s.level.Flags[loreThreatClearedFlag] != nil {
		return
	}
	want := map[string]bool{}
	for _, b := range sc.ThreatBlueprints {
		want[b] = true
	}
	alive := 0
	for _, e := range s.level.Entities {
		if e != nil && want[e.Blueprint] && !e.HasComponent(rlcomponents.Dead) {
			alive++
		}
	}
	switch {
	case alive > 0:
		s.level.Flags[loreThreatSeenFlag] = 1.0
	case s.level.Flags[loreThreatSeenFlag] != nil:
		s.level.Flags[loreThreatClearedFlag] = 1.0
		name := sc.ThreatName
		if name == "" {
			name = "threat"
		}
		s.campaign.RecordLive("threat_cleared", s.location(), map[string]string{"threat": name})
	}
}

// originReach is how close a colonist must stand to an event's origin to find
// where it happened.
const originReach = 2

// revealReachedOrigins makes known every site event whose origin a colonist
// stands next to.
func (s *MainState) revealReachedOrigins() {
	loc := s.location()
	site := s.campaign.SiteHistory(loc)
	if site == nil {
		return
	}
	colony := s.livingColonyName()
	for _, id := range site.Events {
		if s.campaign.History.Known[id] {
			continue
		}
		ox, oy, oz, ok := generation.Origin(s.level, id)
		if !ok {
			continue
		}
		for _, e := range s.level.Entities {
			if !components.IsColonyWorker(e, colony) || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if pc.GetZ() == oz && abs(pc.GetX()-ox) <= originReach && abs(pc.GetY()-oy) <= originReach {
				if text, newly := s.campaign.RevealEvent(id); newly && text != "" {
					s.present("Found where it happened: " + text)
				}
				break
			}
		}
	}
}

