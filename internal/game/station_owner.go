package game

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// Stations carry no owner. A faction settlement's director counts the
// stations within its radius of its centre as its own (director.look), and a
// hive larva or structure names the settlement that laid it (Kin.Settlement).
// stationOwner reads ownership the same way, so a click on a rival's bench,
// bio-printer, larva or lab does not open the player's crafting or research
// modal (phase F, playable_races_plan.md: "Station clicks check the owner").

// stationOwner is the faction settlement on this level that owns ent: the one
// named by its Kin, else the nearest faction settlement whose director radius
// (plus the director's 2-tile margin) covers it on its own z. "" when none
// does: the station is the player's, or nobody's.
func (s *MainState) stationOwner(ent *ecs.Entity) string {
	if k := components.KinOf(ent); k != nil && k.Settlement != "" {
		return k.Settlement
	}
	if s.campaign == nil || s.locationID == "" {
		return ""
	}
	x, y, z, ok := entityXYZ(ent)
	if !ok {
		return ""
	}
	loc := s.campaign.Locations[s.locationID]
	ps, ds := settlementPresences(s.campaign, loc)
	best, bestDist := "", -1
	for i, p := range ps {
		name := factionSettlementName(p, loc.ID)
		st := settlement.Settlements[name]
		if st == nil || st.CenterZ != z {
			continue
		}
		r := 10
		if dir := ds[i].Settlement.Director; dir != nil && dir.Radius > 0 {
			r = dir.Radius
		}
		d := max(abs(x-st.CenterX), abs(y-st.CenterY))
		if d <= r+2 && (bestDist < 0 || d < bestDist) {
			best, bestDist = name, d
		}
	}
	return best
}

// foreignStation reports whether ent belongs to a settlement other than the
// player's own on this level.
func (s *MainState) foreignStation(ent *ecs.Entity) bool {
	o := s.stationOwner(ent)
	return o != "" && o != s.colonyName()
}

func entityXYZ(e *ecs.Entity) (int, int, int, bool) {
	pc, ok := entityPos(e)
	if !ok {
		return 0, 0, 0, false
	}
	return pc.GetX(), pc.GetY(), pc.GetZ(), true
}

// ownStructure reports whether ent is a structure of the player's own
// faction settlement on this level (a race run's Resin Spire, which carries
// a script, would otherwise read as an attack target for its own brood).
// Always false for the colony, whose settlement is no faction settlement.
func (s *MainState) ownStructure(ent *ecs.Entity) bool {
	if ent == nil || !ent.HasComponent(rlcomponents.Description) ||
		ent.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Classification != "structure" {
		return false
	}
	return s.stationOwner(ent) == s.colonyName()
}
