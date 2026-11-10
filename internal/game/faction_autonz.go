package game

import (
	"fmt"
	"log"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
)

// The autonz on a level (phase E; docs/developer/factions.md, "The
// autonz"). An autonz presence is a settlement presence whose camp is its
// stage's stamp (the node, then the hub stamped over it), with its
// conversion share laid around it as blue wire and its crew by the stage's
// mix. A dormant presence (founded by a signal) is a single sleeper node and
// nothing else until its wake day; destroyed before then, the presence is
// wiped. Live, once a day, a dormant presence wakes on its day and a hub
// signals by the Tier 1 rule from the level's own count and conversion.

// sleeperOf is presence id's sleeper node on level, or nil.
func sleeperOf(level *world.Level, d *campaign.Doctrine, id string) *ecs.Entity {
	if d == nil || d.Level.Sleeper == "" {
		return nil
	}
	for _, e := range presenceMembers(level, id) {
		if e.Blueprint == d.Level.Sleeper {
			return e
		}
	}
	return nil
}

// placeSleeper stands a dormant presence's sleeper node at its centre, once
// (idempotent: a standing node adds nothing).
func placeSleeper(level *world.Level, loc *campaign.Location, d *campaign.Doctrine, p *campaign.FactionPresence) {
	if d.Level.Sleeper == "" {
		return
	}
	if e := sleeperOf(level, d, p.ID); e != nil {
		// An older save's node keeps its blueprint's look (Phase E3: the
		// dark core block, once the dormant enforcer).
		if base, err := factory.Create(d.Level.Sleeper, 0, 0, 0); err == nil && base.HasComponent(components.Appearance) && e.HasComponent(components.Appearance) {
			*e.GetComponent(components.Appearance).(*components.AppearanceComponent) = *base.GetComponent(components.Appearance).(*components.AppearanceComponent)
		}
		return
	}
	if _, _, _, placed := generation.Origin(level, factionOriginKey(p.ID)); placed {
		return // it stood here and was destroyed: reconcile wipes it
	}
	cx, cy, cz, ok := presenceCentre(level, loc, d, p)
	if !ok {
		log.Printf("[faction] %s on %s: no room for a sleeper", p.ID, loc.Name)
		return
	}
	x, y, z := freeLandingTile(level, cx, cy, cz)
	e, err := factory.Create(d.Level.Sleeper, x, y, z)
	if err != nil {
		log.Printf("[faction] sleeper %s: %v", d.Level.Sleeper, err)
		return
	}
	components.KinOf(e).Presence = p.ID
	level.AddEntity(e)
}

// removeSleeper takes a woken presence's sleeper node off the level.
func removeSleeper(level *world.Level, d *campaign.Doctrine, id string) {
	if e := sleeperOf(level, d, id); e != nil {
		level.RemoveEntity(e)
	}
}

// reconcileDormant wipes every dormant presence on loc whose sleeper node
// stood on level and is gone (the player destroyed it before it woke).
func reconcileDormant(c *campaign.Campaign, loc *campaign.Location, level *world.Level) {
	fd := campaign.LoadFactionData()
	var gone []string
	for _, p := range loc.Factions {
		if !p.Dormant {
			continue
		}
		d := fd.Doctrine(p.Doctrine)
		if d == nil || d.Level.Sleeper == "" {
			continue
		}
		if _, _, _, placed := generation.Origin(level, factionOriginKey(p.ID)); placed && sleeperOf(level, d, p.ID) == nil {
			gone = append(gone, p.ID)
		}
	}
	for _, id := range gone {
		if c.WipeDormant(loc, id) {
			log.Printf("[faction] dormant %s on %s destroyed before it woke", id, loc.Name)
		}
	}
}

// buildStagedCamp is a staged camp (the autonz): the stage's stamp at the
// centre (stamped over the last stage's, the stockpile lifted out first and
// put back), the settlement's storage inside claimed (a locker placed when
// there is none) and the doctrine's station placed when none stands.
func buildStagedCamp(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, stage, cx, cy, cz int, first bool) {
	st := d.Stages[stage]
	if !first {
		// The re-stamp clears the footprint's furniture: lift the hold out.
		p.Stockpile = storage.Summarize(storage.LevelProvider{Level: level}, []string{name})
	}
	if st.Stamp != "" {
		stampAt(level, st.Stamp, cx, cy, st.W, st.H, nil, p.Seed+int64(stage)*104729)
	}
	furnishStagedCamp(level, d, p, name, stage, cx, cy, cz)
	if !first {
		fillFactionStock(level, name, p.Stockpile, cx, cy, cz)
	}
	p.LevelStage = stage
}

// furnishStagedCamp claims the settlement's storage inside stage's camp (a
// locker placed when there is none) and places the doctrine's station when
// none stands.
func furnishStagedCamp(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence, name string, stage, cx, cy, cz int) {
	st := d.Stages[stage]
	camp := d.Settlement.Camp
	w, h := max(st.W, camp.W, 5), max(st.H, camp.H, 5)
	x0, y0 := cx-w/2, cy-h/2
	claimed, station := false, false
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			pc, ok := entityPos(e)
			if !ok || pc.GetZ() != cz || pc.GetX() < x0 || pc.GetX() >= x0+w || pc.GetY() < y0 || pc.GetY() >= y0+h {
				continue
			}
			if e.HasComponent(components.Storage) {
				e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = name
				claimed = true
			}
			if e.Blueprint == camp.Station {
				station = true
			}
		}
	}
	place := func(bp string, dx, dy int, own func(*ecs.Entity)) {
		x, y, z := freeLandingTile(level, cx+dx, cy+dy, cz)
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			log.Printf("[faction] camp %s: %v", bp, err)
			return
		}
		if own != nil {
			own(e)
		}
		level.AddEntity(e)
	}
	if !claimed {
		place("storage_locker", -2, -2, func(e *ecs.Entity) {
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = name
		})
	}
	if camp.Station != "" && !station {
		place(camp.Station, 2, 2, nil)
	}
}

// stepLiveAutonz is the signal doctrines' daily live rule on a loaded level:
// a dormant presence whose day has come wakes (its sleeper gone, its node
// built at once), and an awake one takes the level's workers and converted
// share into its record, stamps a stage it has reached (the hub) and
// signals by the Tier 1 rule, its signal's cost taken off the level. It
// returns the alerts for the planet in orbit.
func stepLiveAutonz(c *campaign.Campaign, loc *campaign.Location, level *world.Level, loaded func(string) bool) []string {
	if c == nil || loc == nil || level == nil {
		return nil
	}
	fd := campaign.LoadFactionData()
	var alerts []string
	ids := make([]string, 0, len(loc.Factions))
	for _, p := range loc.Factions {
		if d := fd.Doctrine(p.Doctrine); d != nil && d.Mobility.Kind == campaign.MobilitySignal {
			ids = append(ids, p.ID)
		}
	}
	for _, id := range ids {
		p := loc.PresenceByID(id)
		if p == nil {
			continue
		}
		d := fd.Doctrine(p.Doctrine)
		hostile := c.Stance(p.Faction) == campaign.StanceHostile
		if p.Dormant {
			if !c.WakePresence(loc, id) {
				continue
			}
			removeSleeper(level, d, id)
			materialiseSettlement(level, loc, d, p, hostile)
			applyPresenceConversion(level, loc, d, p)
			log.Printf("[faction] %s on %s woke live", id, loc.Name)
			alerts = append(alerts, wakeAlert(d, loc))
			continue
		}
		if !p.Materialized || d.Settlement == nil {
			continue
		}
		if isAutonzDoctrine(d) {
			if stepLiveAutonzSettlement(c, loc, level, d, p, loaded) {
				alerts = append(alerts, signalAlert(d, loc))
			}
			continue
		}
		name := factionSettlementName(p, loc.ID)
		workers := settlementWorkers(level, name)
		reconcileConversion(level, d, p)
		target := c.LiveSignal(loc, id, len(workers), p.Conversion, -1, true, loaded)
		if target != nil {
			// The signal's cost leaves the level, the newest crew first.
			cost := len(workers) - p.Population
			for i := 0; i < cost && i < len(workers); i++ {
				level.RemoveEntity(workers[len(workers)-1-i])
			}
			log.Printf("[faction] %s on %s signalled %s", id, loc.Name, target.Name)
			alerts = append(alerts, signalAlert(d, loc))
		}
		if camp := d.Settlement.Camp; camp != nil && camp.Staged && p.LevelStage < p.Stage {
			materialiseSettlement(level, loc, d, p, hostile)
		}
	}
	return alerts
}

func wakeAlert(d *campaign.Doctrine, loc *campaign.Location) string {
	return fmt.Sprintf("Something powered down on %s has woken. %s wire is spreading.", loc.Name, d.MorphAt(loc).Name)
}

func signalAlert(d *campaign.Doctrine, loc *campaign.Location) string {
	return fmt.Sprintf("The %s hub on %s pulsed a signal into space.", d.MorphAt(loc).Name, loc.Name)
}
