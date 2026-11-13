package game

import (
	"fmt"
	"log"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// Ships landing and lifting off on a loaded level (phase D;
// docs/developer/factions.md, "Ships"). Tier 1 never steps a ship whose
// site is loaded: until phase D one bound for a loaded site waited in orbit
// for as long as the player stayed, and one landed there never left. Now,
// once a star-date day, the world manager resolves them live: an arrival
// comes down and its camp and crew are built onto the level at once, and a
// ship whose stay is over (or whose hold is full) gathers its crew and its
// hold and lifts off, leaving its camp (as a ruin, phase D4).

// stepLiveShips lands and lifts off ships on loaded levels, in fleet order.
// It reports the alerts it raised (for tests).
func (wm *WorldManager) stepLiveShips() []string {
	c := wm.Campaign
	if c == nil {
		return nil
	}
	day := c.FactionDay
	fd := campaign.LoadFactionData()
	var alerts []string
	var dead []string // landed ships whose crew all died here (phase E)
	for i := range c.Fleets {
		f := &c.Fleets[i]
		if f.Kind != campaign.FleetShip || f.Mothership {
			continue
		}
		d := fd.Doctrine(f.Presence.Doctrine)
		if d == nil || d.Settlement == nil {
			continue
		}
		switch {
		case !f.Landed() && !f.Returning && f.ArriveDay <= day && wm.IsLoaded(f.ToID):
			loc := c.LandShip(f, day)
			ms := wm.loaded[f.ToID]
			if loc == nil || ms == nil || ms.level == nil {
				continue
			}
			hostile := c.Stance(f.Presence.Faction) == campaign.StanceHostile
			if isGreyishDoctrine(d) {
				materialiseGreyish(c, ms.level, loc, d, &f.Presence, hostile)
			} else {
				materialiseSettlement(ms.level, loc, d, &f.Presence, hostile)
			}
			c.MakeFleetKnown(f, loc)
			log.Printf("[faction] %s landed live on %s", f.ID, loc.Name)
			alerts = append(alerts, wm.shipAlert(loc, shipLandAlert(d, loc)))
		case f.Landed() && wm.IsLoaded(f.LandedID) && liveHold(wm, f) && f.Presence.Materialized && f.Presence.Population <= 0:
			dead = append(dead, f.ID)
		case f.Landed() && wm.IsLoaded(f.LandedID) && c.ShipLeaving(f, day):
			loc := c.Locations[f.LandedID]
			ms := wm.loaded[f.LandedID]
			if loc == nil || ms == nil || ms.level == nil {
				continue
			}
			// The record first: crew, hold and techs leave with the ship.
			// Nothing comes off the level until it is going: one that
			// cannot leave (no site, no fuel) stays exactly as it is.
			reconcileSettlementOf(c, ms.level, loc, &f.Presence)
			to := c.DepartShip(f, day, wm.IsLoaded)
			if to == nil {
				continue
			}
			liftOffLive(ms, loc, &f.Presence)
			stampRuins(c, loc, ms.level) // its camp, ruined as the player watches
			log.Printf("[faction] %s lifted off live from %s for %s", f.ID, loc.Name, to.Name)
			alerts = append(alerts, wm.shipAlert(loc, shipLeaveAlert(d, loc)))
		}
	}
	// The greys' mothership beams materials down to its camps (phase E4).
	for i := range c.Fleets {
		f := &c.Fleets[i]
		if f.Kind != campaign.FleetShip || f.Mothership || !f.Landed() || !wm.IsLoaded(f.LandedID) || !f.Presence.Materialized {
			continue
		}
		if d := fd.Doctrine(f.Presence.Doctrine); d != nil && d.Settlement != nil {
			wm.beamDownToCamp(f, d)
		}
	}
	for _, id := range dead {
		if a := wm.wipeShipLive(id); a != "" {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// wipeShipLive drops landed ship id whose crew is all dead on its loaded
// level at once: the record wiped (a Legends line), its settlement and
// director gone, its camp left standing as it is. It returns the alert.
func (wm *WorldManager) wipeShipLive(id string) string {
	c := wm.Campaign
	var f *campaign.Fleet
	for i := range c.Fleets {
		if c.Fleets[i].ID == id {
			f = &c.Fleets[i]
		}
	}
	if f == nil {
		return ""
	}
	loc := c.Locations[f.LandedID]
	d := campaign.LoadFactionData().Doctrine(f.Presence.Doctrine)
	if loc == nil || d == nil {
		return ""
	}
	name := factionSettlementName(&f.Presence, loc.ID)
	if ms := wm.loaded[loc.ID]; ms != nil {
		if st := settlement.Settlements[name]; st != nil {
			for _, t := range st.Tasks.GetTasks() {
				t.Complete()
			}
		}
		delete(ms.factionDirectors, name)
	}
	delete(settlement.Settlements, name)
	if !c.WipeShip(f) {
		return ""
	}
	log.Printf("[faction] %s wiped out live on %s", id, loc.Name)
	return wm.shipAlert(loc, fmt.Sprintf("The %s ship's crew on %s is dead.", d.MorphAt(loc).Singular, loc.Name))
}

// liveHold brings a landed ship's record up to date from its level (crew,
// hold, techs), so the hold-full rule reads what its crew has hauled. It
// always reports true (a switch guard).
func liveHold(wm *WorldManager, f *campaign.Fleet) bool {
	loc, ms := wm.Campaign.Locations[f.LandedID], wm.loaded[f.LandedID]
	if loc != nil && ms != nil && ms.level != nil && f.Presence.Materialized {
		reconcileSettlementOf(wm.Campaign, ms.level, loc, &f.Presence)
	}
	return true
}

// shipAlert posts text when loc is the planet in orbit, and returns it.
func (wm *WorldManager) shipAlert(loc *campaign.Location, text string) string {
	if loc.ID == wm.currentID {
		message.PostMessage("Alert", text)
	}
	return text
}

func shipLandAlert(d *campaign.Doctrine, loc *campaign.Location) string {
	return fmt.Sprintf("A %s ship has come down on %s.", d.MorphAt(loc).Singular, loc.Name)
}

func shipLeaveAlert(d *campaign.Doctrine, loc *campaign.Location) string {
	return fmt.Sprintf("The %s ship has lifted off from %s.", d.MorphAt(loc).Singular, loc.Name)
}

// liftOffLive takes a departing ship's settlement off its level, once
// DepartShip has sent it (the record reconciled before, the camp recorded as
// a ruin): the crew goes aboard, the hold is emptied (the stockpile left
// with the ship), its orders are dropped and the settlement leaves the
// global map. The camp stays, and is ruined by stampRuins (D4).
func liftOffLive(ms *MainState, loc *campaign.Location, p *campaign.FactionPresence) {
	level := ms.level
	name := factionSettlementName(p, loc.ID)
	for _, e := range settlementWorkers(level, name) {
		level.RemoveEntity(e)
	}
	emptySettlementStorage(level, name)
	if st := settlement.Settlements[name]; st != nil {
		for _, t := range st.Tasks.GetTasks() {
			t.Complete()
		}
	}
	delete(settlement.Settlements, name)
	delete(ms.factionDirectors, name)
	p.Materialized = false
}

// emptySettlementStorage takes everything out of the settlement's containers
// (it is aboard the ship now).
func emptySettlementStorage(level *world.Level, name string) {
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			if sc := e.GetComponent(components.Storage).(*components.StorageComponent); sc.OwnedBy == name {
				sc.Items = nil
			}
		}
	}
}

// liveShipsByLevel is the ships landed on each loaded level, for the tests
// and the sidecar: location ID to fleet IDs, sorted.
func (wm *WorldManager) liveShipsByLevel() map[string][]string {
	out := map[string][]string{}
	for i := range wm.Campaign.Fleets {
		f := &wm.Campaign.Fleets[i]
		if f.Landed() && wm.IsLoaded(f.LandedID) {
			out[f.LandedID] = append(out[f.LandedID], f.ID)
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// beamDownToCamp tops a landed ship's camp on a loaded level up to its
// mothership's beam floor (DoctrineBeam: the greys, phase E4), from the
// mothership's beam stock, then its hoard, into the camp's storage. It
// reports what came down.
func (wm *WorldManager) beamDownToCamp(f *campaign.Fleet, d *campaign.Doctrine) map[string]int {
	if d.Ships == nil || d.Ships.Mothership == nil || d.Ships.Mothership.Beam == nil {
		return nil
	}
	loc, ms := wm.Campaign.Locations[f.LandedID], wm.loaded[f.LandedID]
	if loc == nil || ms == nil || ms.level == nil {
		return nil
	}
	name := factionSettlementName(&f.Presence, loc.ID)
	st := settlement.Settlements[name]
	if st == nil {
		return nil
	}
	have := storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{name})
	hold := 0
	for _, v := range have {
		hold += v
	}
	want := map[string]int{}
	for k, v := range d.Ships.Mothership.Beam.Floor {
		if n := v - have[k]; n > 0 {
			want[k] = n
		}
	}
	if len(want) == 0 {
		f.NoteBeamed(hold, 0) // what it spent comes off the beamed units
		return nil
	}
	got := wm.Campaign.BeamFromMothership(f.Presence.Faction, want)
	n := 0
	for _, v := range got {
		n += v
	}
	f.NoteBeamed(hold, n) // left out of its hold target
	if len(got) > 0 {
		fillFactionStock(ms.level, name, got, st.CenterX, st.CenterY, st.CenterZ)
		log.Printf("[faction] %s: the mothership beamed down %v", f.ID, got)
	}
	return got
}
