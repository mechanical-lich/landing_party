package campaign

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// Ships (phase C2; docs/developer/factions.md, "The mirror colony"). A fleet
// faction's presence rides a ship, not a planet (plan, "Where a presence
// lives"): population, stockpile and techs travel with it. A ship is a Fleet
// of kind "ship" that is either in flight (FromID -> ToID, as an egg) or
// landed (LandedID): a landed ship's presence is a settlement on that site,
// built onto its level like a planet-carried one. A ship lands, stays its
// doctrine's stay_days, and lifts off for another site within its range,
// taking everyone and the stockpile; the camp it built stays on the level.
// Tier 1 steps a landed ship only while its site is not loaded (a loaded
// level simulates it, and reconcile writes back), and a ship bound for a
// loaded site waits in orbit, as an egg does.

const (
	// FleetShip is a Fleet that is a faction's ship.
	FleetShip = "ship"
	// MobilityFleet is the mobility kind of a faction that travels by ship.
	MobilityFleet = "fleet"
)

// Landed reports whether f is a ship on the ground.
func (f *Fleet) Landed() bool { return f.Kind == FleetShip && f.LandedID != "" }

// LandedFleets are the ships landed on locID, in fleet order.
func (c *Campaign) LandedFleets(locID string) []*Fleet {
	var out []*Fleet
	for i := range c.Fleets {
		if c.Fleets[i].Landed() && c.Fleets[i].LandedID == locID {
			out = append(out, &c.Fleets[i])
		}
	}
	return out
}

// FleetByPresence is the fleet carrying presence id, or nil.
func (c *Campaign) FleetByPresence(id string) *Fleet {
	for i := range c.Fleets {
		if c.Fleets[i].Presence.ID == id {
			return &c.Fleets[i]
		}
	}
	return nil
}

// newShip puts a fresh ship of doctrine key landed on loc, its presence pop
// strong with the doctrine's kit in the hold.
func (c *Campaign) newShip(key string, d *Doctrine, loc *Location, pop int, record bool) *Fleet {
	c.FactionSeq++
	c.FleetSeq++
	kit := map[string]int{}
	for k, v := range d.Kit {
		kit[k] = v
	}
	if len(kit) == 0 {
		kit = nil
	}
	c.Fleets = append(c.Fleets, Fleet{
		ID: fmt.Sprintf("fl%d", c.FleetSeq), Kind: FleetShip,
		FromID: loc.ID, ToID: loc.ID, FromX: loc.X, FromY: loc.Y, ToX: loc.X, ToY: loc.Y,
		LaunchDay: c.FactionDay, ArriveDay: c.FactionDay,
		LandedID: loc.ID, LandedDay: c.FactionDay,
		Presence: FactionPresence{
			ID:          fmt.Sprintf("fp%d", c.FactionSeq),
			Faction:     key,
			Doctrine:    c.DoctrineOf(key),
			Population:  pop,
			Stockpile:   kit,
			ArrivedDay:  c.FactionDay,
			LastStepDay: c.FactionDay,
			LastShotDay: NeverFired,
			Seed:        c.Seed + int64(c.FactionSeq)*6271,
		},
	})
	f := &c.Fleets[len(c.Fleets)-1]
	if d.Expedition != nil {
		f.Presence.Fuel = d.Expedition.FleetFuel
	}
	if record {
		c.recordFaction("faction_landed", loc, &f.Presence, d, nil)
	}
	return f
}

// stepShip advances one ship one day: an arrival lands (unless its target is
// loaded: the world manager lands it live), a ship bound home docks at its
// mothership, a landed ship off a loaded site grows (to its site's capacity
// under its tier's crew cap), digs into its hold and researches, and lifts
// off once its stay is over or its hold is full (a loaded site's ship lifts
// off live). A mothership hoards and builds ships. It reports whether the
// ship survives (a crew of none is gone).
func (c *Campaign) stepShip(rng *rand.Rand, fd *FactionData, f *Fleet, day history.StarDate, isLoaded func(string) bool) bool {
	d := fd.Doctrine(f.Presence.Doctrine)
	if d == nil {
		return true
	}
	p := &f.Presence
	if f.BoardedAt != "" {
		return true // held while the player is aboard
	}
	if f.Mothership {
		c.stepMothership(d, f, day)
		return true
	}
	if !f.Landed() {
		if f.ArriveDay > day {
			return true
		}
		if f.Returning {
			c.dock(d, f, day, isLoaded)
			return true
		}
		if isLoaded(f.ToID) {
			return true // the world manager lands it live
		}
		c.LandShip(f, day)
		return true
	}
	if isLoaded(f.LandedID) {
		return true // the level simulates it, and lifts it off live
	}
	loc := c.Locations[f.LandedID]
	if loc == nil {
		return true
	}
	if p.Population <= 0 {
		c.recordFaction("faction_wiped", loc, p, d, nil)
		return false
	}
	p.LastStepDay = day
	// Grow toward the cap (the bio-printer, abstractly).
	if cap := d.shipCap(f, loc); p.Population < cap {
		p.Growth += d.Growth.PerDay * float64(p.Population)
		for p.Growth >= 1 && p.Population < cap {
			p.Population++
			p.Growth--
		}
	}
	// A typed crew's bodies by type (the greyish, phase E4): the growth
	// rule's bookkeeping, read by materialise; nothing reads it back here.
	if d.HasTypes() {
		d.NormalizeUnits(p, loc)
	}
	// Dig into the hold, by the site's tags.
	if d.Ships != nil && len(d.Ships.Yield) > 0 {
		c.shipYield(d, f, loc)
	}
	// A table-flown ship refuels at its camp.
	if d.Expedition != nil {
		c.refuelShip(d.Expedition, f)
	}
	// Research walks the doctrine's list: the first entry it does not know
	// (a live colony's techs are merged in on reconcile, in any order, so
	// the count is no index into the walk).
	if next := nextUnknown(d.Research, p.KnownTechs); next != nil && d.ResearchPerDay > 0 {
		p.ResearchPts += d.ResearchPerDay
		if p.ResearchPts >= next.Cost {
			p.ResearchPts -= next.Cost
			p.KnownTechs = append(p.KnownTechs, next.ID)
		}
	}
	// The second walk (phase E4, the greys): on the same landed days, its
	// own points per day by hull tier. Nothing it learns moves a Tier 1
	// number, so the pace is the spine's alone.
	if next := nextUnknown(d.Assimilation, p.KnownTechs); next != nil && len(d.AssimilationPerDay) > 0 {
		p.AssimPoints += d.AssimilationPerDay[min(max(f.ShipTier, 0), len(d.AssimilationPerDay)-1)]
		if p.AssimPoints >= next.Cost {
			p.AssimPoints -= next.Cost
			p.KnownTechs = append(p.KnownTechs, next.ID)
		}
	}
	if c.ShipLeaving(f, day) {
		c.DepartShip(f, day, isLoaded)
	}
	_ = rng
	return true
}

// shipYield adds a landed ship's day of digging to its hold: per crew
// member, each tag's yield, the fraction carried in Growth's spare digits
// (whole units only; a small site is slow).
func (c *Campaign) shipYield(d *Doctrine, f *Fleet, loc *Location) {
	p := &f.Presence
	add := map[string]float64{}
	for _, tag := range loc.CapacityTags() {
		for res, v := range d.Ships.Yield[tag] {
			add[res] += v * float64(p.Population)
		}
	}
	for res, v := range d.Ships.Yield["base"] {
		add[res] += v * float64(p.Population)
	}
	if len(add) == 0 {
		return
	}
	if p.Stockpile == nil {
		p.Stockpile = map[string]int{}
	}
	keys := make([]string, 0, len(add))
	for k := range add {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, res := range keys {
		p.Stockpile[res] += int(add[res])
		if frac := add[res] - float64(int(add[res])); frac > 0 {
			// Carry the fraction, per resource.
			k := res
			if p.YieldFrac == nil {
				p.YieldFrac = map[string]float64{}
			}
			p.YieldFrac[k] += frac
			for p.YieldFrac[k] >= 1 {
				p.Stockpile[res]++
				p.YieldFrac[k]--
			}
		}
	}
}

// dock brings a returning ship's hold and techs to its mothership, raises
// its tier when the mothership can pay for the next, and sends it out again
// from there. With no mothership left (destroyed), it lands as any ship.
func (c *Campaign) dock(d *Doctrine, f *Fleet, day history.StarDate, isLoaded func(string) bool) {
	ms := c.Mothership(f.Presence.Faction)
	if ms == nil {
		f.Returning = false
		if !isLoaded(f.ToID) {
			c.LandShip(f, day)
		}
		return
	}
	mp, p := &ms.Presence, &f.Presence
	if mp.Stockpile == nil {
		mp.Stockpile = map[string]int{}
	}
	for k, v := range p.Stockpile {
		mp.Stockpile[k] += v
	}
	p.Stockpile = nil
	for _, k := range p.KnownTechs {
		if !hasStr(mp.KnownTechs, k) {
			mp.KnownTechs = append(mp.KnownTechs, k)
		}
	}
	p.KnownTechs = append([]string(nil), mp.KnownTechs...)
	// Tiers rise only here, paid from the mothership's hoard.
	if ts := d.ShipTiers(); f.ShipTier+1 < len(ts) && covers(mp.Stockpile, ts[f.ShipTier+1].Cost) {
		spend(mp.Stockpile, ts[f.ShipTier+1].Cost)
		f.ShipTier++
		if loc := c.Locations[ms.ToID]; loc != nil {
			c.recordFaction("faction_upgraded", loc, p, d, map[string]string{"tier": d.TierName(f.ShipTier)})
		}
	}
	f.Returning = false
	// Out again, from the mothership's location: a free site in range.
	from := c.Locations[ms.ToID]
	if from == nil {
		return
	}
	f.LandedID = from.ID // as if landed there, for shipTarget's origin
	rng := rand.New(rand.NewSource(c.Seed*137 + int64(day)*7919 + fleetNum(f)))
	if to := c.shipTarget(rng, d, f, isLoaded); to != nil {
		c.setOut(d, f, from, to, day) // from orbit: no camp, no ruin
		return
	}
	// Nowhere to go: it waits docked (in orbit of the mothership).
	f.LandedID = ""
	f.FromID, f.FromX, f.FromY = from.ID, from.X, from.Y
	f.ToID, f.ToX, f.ToY = from.ID, from.X, from.Y
	f.Returning = true
	f.ArriveDay = day + 1
}

// stepMothership is a mothership's day: when its hoard covers a new ship
// and it has room for one, it orders one (built after the day's fleet pass,
// buildShip).
func (c *Campaign) stepMothership(d *Doctrine, f *Fleet, day history.StarDate) {
	if d.Ships == nil || d.Ships.Mothership == nil {
		return
	}
	m := d.Ships.Mothership
	if m.MaxShips <= 0 || len(m.ShipCost) == 0 || !covers(f.Presence.Stockpile, m.ShipCost) {
		return
	}
	if c.shipCount(f.Presence.Faction) >= m.MaxShips {
		return
	}
	c.shipOrders = append(c.shipOrders, f.Presence.Faction)
}

// shipCount is faction's ships, its mothership aside.
func (c *Campaign) shipCount(faction string) int {
	n := 0
	for i := range c.Fleets {
		if g := &c.Fleets[i]; g.Kind == FleetShip && !g.Mothership && g.Presence.Faction == faction {
			n++
		}
	}
	return n
}

// buildShip has faction's mothership build a tier-0 ship from its hoard,
// with its techs, setting out at once for a free site in range (or waiting
// docked).
func (c *Campaign) buildShip(faction string, day history.StarDate, isLoaded func(string) bool) {
	ms := c.Mothership(faction)
	if ms == nil {
		return
	}
	d := LoadFactionData().Doctrine(ms.Presence.Doctrine)
	if d == nil || d.Ships == nil || d.Ships.Mothership == nil {
		return
	}
	m := d.Ships.Mothership
	loc := c.Locations[ms.ToID]
	if loc == nil || !covers(ms.Presence.Stockpile, m.ShipCost) || c.shipCount(faction) >= m.MaxShips {
		return
	}
	spend(ms.Presence.Stockpile, m.ShipCost)
	techs := append([]string(nil), ms.Presence.KnownTechs...)
	pop := max(d.SeedPopulation, 1)
	if cc := d.CrewCap(0); cc > 0 {
		pop = min(pop, cc)
	}
	ship := c.newShip(faction, d, loc, pop, false) // ms is stale from here
	ship.Presence.KnownTechs = techs
	ship.Presence.Stockpile = nil
	// Built in orbit: it sets out at once.
	rng := rand.New(rand.NewSource(c.Seed*139 + int64(day)*7919 + fleetNum(ship)))
	if to := c.shipTarget(rng, d, ship, isLoaded); to != nil {
		c.setOut(d, ship, loc, to, day) // from orbit: no camp, no ruin
	} else {
		ship.LandedID = ""
		ship.Returning = true
		ship.ArriveDay = day + 1
	}
}

// covers reports whether stock has cost.
func covers(stock, cost map[string]int) bool {
	for k, v := range cost {
		if stock[k] < v {
			return false
		}
	}
	return true
}

func spend(stock, cost map[string]int) {
	for k, v := range cost {
		stock[k] -= v
		if stock[k] <= 0 {
			delete(stock, k)
		}
	}
}

// shipTarget is where a ship goes next: a site a presence may be on, not the
// one it is leaving, holding no presence and no other ship, not loaded, in
// range, weighted toward the doctrine's economy tag. nil: it stays.
func (c *Campaign) shipTarget(rng *rand.Rand, d *Doctrine, f *Fleet, isLoaded func(string) bool) *Location {
	return pickWeighted(rng, c.shipCandidates(d, f, isLoaded), func(l *Location) float64 { return economyWeight(d, l) })
}

// shipCandidates are the sites a landed ship may go to next: a site a
// presence may be on, not the one it is leaving, holding no presence and
// no other ship, not loaded, in range; in location order.
func (c *Campaign) shipCandidates(d *Doctrine, f *Fleet, isLoaded func(string) bool) []*Location {
	if isLoaded == nil {
		isLoaded = func(string) bool { return false }
	}
	from := c.Locations[f.LandedID]
	if from == nil {
		return nil
	}
	rng2 := d.Mobility.BaseRange
	var cands []*Location
	for _, loc := range c.sortedLocations() {
		if loc.ID == from.ID || isLoaded(loc.ID) || len(loc.Factions) > 0 || !c.fleetSiteOK(f, loc) {
			continue
		}
		if rng2 > 0 && math.Hypot(loc.X-from.X, loc.Y-from.Y) > rng2 {
			continue
		}
		busy := false
		for i := range c.Fleets {
			g := &c.Fleets[i]
			if g.ID != f.ID && (g.LandedID == loc.ID || g.Kind == FleetShip && !g.Landed() && g.ToID == loc.ID || g.Mothership && g.ToID == loc.ID) {
				busy = true
			}
		}
		if !busy {
			cands = append(cands, loc)
		}
	}
	return cands
}

// liftOff sends a landed ship to to: everyone and the stockpile go with it,
// and its camp stays behind as a ruin.
func (c *Campaign) liftOff(d *Doctrine, f *Fleet, to *Location, day history.StarDate) {
	from := c.Locations[f.LandedID]
	if from == nil {
		return
	}
	ev := c.recordFaction("faction_departed", from, &f.Presence, d, map[string]string{"to": to.Name})
	// Its camp stays behind as a ruin (phase D4): ruined live by the world
	// manager on a loaded level, else on the level's next build.
	r := FactionRuin{
		Presence: f.Presence.ID, Faction: f.Presence.Faction, Doctrine: f.Presence.Doctrine,
		Day: day, Seed: f.Presence.Seed, EventID: ev, CampBuilt: f.Presence.Materialized || f.Presence.Camped,
	}
	if d.ShipSettles() {
		// The scar's wrecks (phase E4): what stood, or what would have.
		r.Tier, r.DaysLanded = f.ShipTier, int(day-f.LandedDay)
		r.Techs = append([]string(nil), f.Presence.KnownTechs...)
		if f.Presence.Built != nil {
			r.Built = map[string]int{}
			for k, v := range f.Presence.Built {
				r.Built[k] = v
			}
		}
		f.Presence.Built = nil
	}
	from.addRuin(r)
	f.Presence.Camped = false
	c.setOut(d, f, from, to, day)
}

// addRuin records r on loc, folded into an unstamped ruin of the same
// presence there: a presence's camp is always sited on the same spot of a
// level (its seed), so a ship that lands and leaves again before the level is
// built leaves one ruin there, not one per visit.
func (loc *Location) addRuin(r FactionRuin) {
	for i := range loc.Ruins {
		if o := &loc.Ruins[i]; !o.Stamped && o.Presence == r.Presence {
			o.Day, o.EventID, o.Seed = r.Day, r.EventID, r.Seed
			o.CampBuilt = o.CampBuilt || r.CampBuilt
			o.Built, o.Tier, o.Techs, o.DaysLanded = r.Built, r.Tier, r.Techs, r.DaysLanded
			return
		}
	}
	loc.Ruins = append(loc.Ruins, r)
}

// setOut puts f in flight from from to to, leaving day: a lift-off's travel,
// or a ship leaving the mothership's orbit (docked or just built), which
// landed nowhere and leaves nothing behind.
func (c *Campaign) setOut(d *Doctrine, f *Fleet, from, to *Location, day history.StarDate) {
	speed := 1.0
	if d.Mobility.ShipSpeed > 0 {
		speed = d.Mobility.ShipSpeed
	}
	days := int(math.Ceil(math.Hypot(to.X-from.X, to.Y-from.Y) / speed))
	if days < 1 {
		days = 1
	}
	f.LandedID = ""
	f.FromID, f.FromX, f.FromY = from.ID, from.X, from.Y
	f.ToID, f.ToX, f.ToY = to.ID, to.X, to.Y
	f.LaunchDay, f.ArriveDay = day, day+history.StarDate(days)
}

func hasStr(list []string, k string) bool {
	for _, v := range list {
		if v == k {
			return true
		}
	}
	return false
}

// ---- diplomacy ----

// Stances toward the player.
const (
	StanceNeutral = "neutral"
	StanceHostile = "hostile"
)

// Stance is faction's stance toward the player: neutral for a faction of
// the player's own group (rival_selection_plan.md, "Groups"), else what the
// campaign recorded, else its doctrine's starting diplomacy, else hostile.
func (c *Campaign) Stance(faction string) string {
	if c != nil {
		if c.Player != nil && faction != c.Player.Faction && c.SameGroup(faction, c.Player.Faction) {
			return StanceNeutral
		}
		if s, ok := c.Diplomacy[faction]; ok {
			return s
		}
	}
	if d := LoadFactionData().Doctrine(c.DoctrineOf(faction)); d != nil && d.Diplomacy != "" {
		return d.Diplomacy
	}
	return StanceHostile
}

// SetStance records faction's stance toward the player; it reports whether
// it changed. Turning hostile is a Legends event at loc.
func (c *Campaign) SetStance(faction, stance string, loc *Location) bool {
	if c.Stance(faction) == stance {
		return false
	}
	if stance == StanceHostile && c.Player != nil && c.SameGroup(faction, c.Player.Faction) {
		return false // a faction of the player's group never turns on the player
	}
	if c.Diplomacy == nil {
		c.Diplomacy = map[string]string{}
	}
	c.Diplomacy[faction] = stance
	if d := LoadFactionData().Doctrine(c.DoctrineOf(faction)); d != nil && stance == StanceHostile {
		c.recordFaction("faction_hostile", loc, nil, d, nil)
	}
	return true
}

// HeldBy is the factions holding loc with a settlement: a landed ship's or a
// planet-carried settlement presence.
func (c *Campaign) HeldBy(loc *Location) []string {
	if loc == nil {
		return nil
	}
	fd := LoadFactionData()
	var out []string
	for _, p := range loc.Factions {
		if d := fd.Doctrine(p.Doctrine); d != nil && d.Settlement != nil {
			out = append(out, p.Faction)
		}
	}
	for _, f := range c.LandedFleets(loc.ID) {
		out = append(out, f.Presence.Faction)
	}
	return out
}

// fleetSiteOK is FactionSiteOK for a ship's next stop; the expedition may
// also take the first-jump sites (they are kept for the player, and in an
// observer run it is the player's).
func (c *Campaign) fleetSiteOK(f *Fleet, loc *Location) bool {
	if f.IsExpedition() {
		return loc != nil && loc.HasSite() && !loc.Tutorial && loc.Kind != EncounterKind
	}
	return c.FactionSiteOK(loc, false)
}
