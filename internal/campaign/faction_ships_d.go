package campaign

import (
	"encoding/json"
	"math"
	"math/rand"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// Ships proper (phase D; docs/developer/factions.md, "Ships"): tiers that cap
// a ship's crew, a stockpile rhythm (a ship leaves when its hold is full and
// brings it home), a mothership where tiers rise, and landings and lift-offs
// that happen live on a loaded site rather than waiting in orbit.

// DoctrineShips is a fleet doctrine's ships block.
type DoctrineShips struct {
	// Marker is how the Star Map draws its ships: "dart" (default) or
	// "saucer"; a mothership is drawn larger with a ring.
	Marker string `json:"marker,omitempty"`
	// Tiers in order: each one's crew cap, and the stock the mothership
	// spends to raise a ship to it.
	Tiers []ShipTierDef `json:"tiers,omitempty"`
	// HoldTarget: a landed ship whose stockpile holds this many units lifts
	// off for the mothership (0: it leaves only when its stay is over).
	HoldTarget int `json:"hold_target,omitempty"`
	// Yield is what a landed ship off a loaded site digs per crew member a
	// day, by the site's tags (the abstract mirror of mining and hauling).
	Yield map[string]map[string]float64 `json:"yield,omitempty"`
	// Mothership: one per faction; nil for none.
	Mothership *DoctrineMothership `json:"mothership,omitempty"`
}

// ShipTierDef is one ship tier.
type ShipTierDef struct {
	Name string         `json:"name"`
	Crew int            `json:"crew"`
	Cost map[string]int `json:"cost,omitempty"`
	// Modules are the structures a ship of the tier carries and unpacks
	// when it lands (the greys' vat and console; greyish_plan.md,
	// "Structures"): packed again on lift-off, never ruined.
	Modules []ShipModule `json:"modules,omitempty"`
}

// ShipModule is one carried structure: its buildable, carried once
// RequiresTech is known ("" always). The same key twice is two of it.
type ShipModule struct {
	Key          string `json:"key"`
	RequiresTech string `json:"requires_tech,omitempty"`
}

// UnmarshalJSON reads a bare key ("grey_vat") or an object.
func (m *ShipModule) UnmarshalJSON(b []byte) error {
	var key string
	if err := json.Unmarshal(b, &key); err == nil {
		*m = ShipModule{Key: key}
		return nil
	}
	type plain ShipModule
	return json.Unmarshal(b, (*plain)(m))
}

// ShipModules is what a ship of tier knowing techs carries, by key and
// count (nil for none).
func (d *Doctrine) ShipModules(tier int, techs []string) map[string]int {
	ts := d.ShipTiers()
	if len(ts) == 0 {
		return nil
	}
	var out map[string]int
	for _, m := range ts[min(max(tier, 0), len(ts)-1)].Modules {
		if m.RequiresTech != "" && !d.KnowsTech(techs, m.RequiresTech) {
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[m.Key]++
	}
	return out
}

// DoctrineMothership is a faction's mothership: its crew (the boarding
// party's foes) and what a new ship costs.
type DoctrineMothership struct {
	Crew int `json:"crew"`
	// ShipCost is the stock a new tier-0 ship costs; MaxShips the most
	// ships (the mothership aside) it keeps.
	ShipCost map[string]int `json:"ship_cost,omitempty"`
	MaxShips int            `json:"max_ships,omitempty"`
	// Beam: what the mothership beams down to its ships' camps on loaded
	// levels (phase E4, the greys; the owner's call, 2026-11-12: as the
	// colonists beam from their ship); nil for none.
	Beam *DoctrineBeam `json:"beam,omitempty"`
}

// DoctrineBeam is a mothership's beam-down: it starts with Stock aboard
// (apart from its hoard, which pays for hulls and ships), and once a day a
// camp of its faction on a loaded level is topped up to Floor of each
// resource, from that stock first and then the hoard. Tier 1 never beams
// (no level is loaded), so the pace does not move.
type DoctrineBeam struct {
	Stock map[string]int `json:"stock,omitempty"`
	Floor map[string]int `json:"floor,omitempty"`
}

// HoldForHome is the hold a ship's hold target is measured against: its
// stockpile less what its mothership beamed down to its camp and the camp
// has not spent (the owner's call, 2026-11-12: beamed stock does not send
// a ship home). Tier 1 never beams, so off a loaded level it is the whole
// hold.
func (f *Fleet) HoldForHome() int {
	return max(holdUnits(f.Presence.Stockpile)-f.Presence.Beamed, 0)
}

// NoteBeamed books n units beamed down to a landed ship's camp whose
// storage holds hold units now: what the camp spent since the last beam
// comes off the beamed units first (the camp spends what came down before
// what it mined), then n is added.
func (f *Fleet) NoteBeamed(hold, n int) {
	p := &f.Presence
	if p.BeamLastHold > hold {
		p.Beamed = max(p.Beamed-(p.BeamLastHold-hold), 0)
	}
	p.Beamed += n
	p.BeamLastHold = hold + n
}

// BeamFromMothership takes up to want from faction's mothership for a camp
// on a loaded level: its beam stock first, then its hoard. It returns what
// was taken (nil with no mothership: destroyed, or none).
func (c *Campaign) BeamFromMothership(faction string, want map[string]int) map[string]int {
	ms := c.Mothership(faction)
	if ms == nil {
		return nil
	}
	p := &ms.Presence
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var got map[string]int
	for _, k := range keys {
		n := want[k]
		for _, src := range []map[string]int{p.BeamStock, p.Stockpile} {
			if n <= 0 || src == nil {
				continue
			}
			take := min(n, src[k])
			if take <= 0 {
				continue
			}
			src[k] -= take
			if src[k] <= 0 {
				delete(src, k)
			}
			n -= take
			if got == nil {
				got = map[string]int{}
			}
			got[k] += take
		}
	}
	return got
}

// ShipTiers is the doctrine's tiers (none without a ships block).
func (d *Doctrine) ShipTiers() []ShipTierDef {
	if d == nil || d.Ships == nil {
		return nil
	}
	return d.Ships.Tiers
}

// CrewCap is a ship of tier's crew cap; 0 when the doctrine has no tiers.
func (d *Doctrine) CrewCap(tier int) int {
	ts := d.ShipTiers()
	if len(ts) == 0 {
		return 0
	}
	return ts[min(max(tier, 0), len(ts)-1)].Crew
}

// TierName is a ship tier's name ("" without tiers).
func (d *Doctrine) TierName(tier int) string {
	ts := d.ShipTiers()
	if len(ts) == 0 {
		return ""
	}
	return ts[min(max(tier, 0), len(ts)-1)].Name
}

// shipCap is how many a landed ship's crew grows to on loc: the site's
// capacity, under its tier's crew cap.
func (d *Doctrine) shipCap(f *Fleet, loc *Location) int {
	cap := d.CapacityAt(loc)
	if cc := d.CrewCap(f.ShipTier); cc > 0 && cc < cap {
		cap = cc
	}
	return cap
}

// holdUnits is the total units in a stockpile.
func holdUnits(stock map[string]int) int {
	n := 0
	for _, v := range stock {
		n += v
	}
	return n
}

// LandShip puts a flying ship down on its target now: the presence is built
// fresh when the level is next built (or at once on a loaded level, by the
// world manager), and a faction_landed event is recorded. It returns the
// site, nil for a ship with nowhere to land.
func (c *Campaign) LandShip(f *Fleet, day history.StarDate) *Location {
	loc := c.Locations[f.ToID]
	if loc == nil || f.Mothership {
		return nil
	}
	d := LoadFactionData().Doctrine(f.Presence.Doctrine)
	f.LandedID, f.LandedDay = loc.ID, day
	f.FromID, f.FromX, f.FromY = loc.ID, loc.X, loc.Y
	if f.IsExpedition() {
		// The expedition is the player's eyes in an observer run: its
		// landing visits the site and shows what holds it.
		loc.Visited = true
		c.RevealPresences(loc)
	}
	f.Returning = false
	p := &f.Presence
	p.Materialized, p.LevelStage = false, 0
	if d != nil && d.ShipSettles() {
		p.Built = nil // its ground structures stayed on the last site, as wrecks
	}
	p.Beamed, p.BeamLastHold = 0, 0 // a new landing's camp starts with nothing beamed
	if d != nil {
		c.recordFaction("faction_landed", loc, p, d, nil)
		c.learnGate(d, f, loc)
	}
	if c.HoldsSite(loc) {
		c.RaceContact(p.Faction, loc) // landed on a world a race run holds
	}
	return loc
}

// ShipLeaving reports whether a landed ship is done with its site on day:
// its stay is over, or its hold has reached the doctrine's target.
func (c *Campaign) ShipLeaving(f *Fleet, day history.StarDate) bool {
	d := LoadFactionData().Doctrine(f.Presence.Doctrine)
	if d == nil || !f.Landed() {
		return false
	}
	if d.Ships != nil && d.Ships.HoldTarget > 0 && f.HoldForHome() >= d.Ships.HoldTarget {
		return true
	}
	if t := d.Expedition; t != nil {
		if !c.Locations[f.LandedID].HasSite() {
			return true // the Wreck Site: nothing to stay for
		}
		if c.fleetMove(d, t, f, day, nil) != "" {
			return true
		}
		if c.seeksGateIn(d, f) {
			return false // it leaves only toward the gate (gate.go)
		}
	}
	stay := d.Mobility.StayDays
	return stay > 0 && int(day-f.LandedDay) >= stay
}

// Fleets that fly by an expedition table (the mirror's ship, the colony's
// table; docs/developer/factions.md, "How a faction moves and acts"): the
// table the player's ship flies by in observer mode decides where the ship
// goes and when it leaves, with fuel paid by the same jump cost rule. The
// colony on the ground is the whole crew, so "found" is the landing and
// "pack up" is leaving a failing site; there is no second colony to
// reinforce. The stay_days hop stays as the fallback when the table has no
// better move.

// fleetView is the expedition view from a landed table-flown ship: the site
// it is on (as visited, no colony of ours counted, so staying is scored
// fairly), and the free sites in range a ship may go to.
func (c *Campaign) fleetView(d *Doctrine, f *Fleet, day history.StarDate, isLoaded func(string) bool) ExpeditionView {
	p := &f.Presence
	v := ExpeditionView{Day: int(day), Here: f.LandedID, Fuel: fleetFuel(d, f), Crew: p.Population}
	if isLoaded == nil {
		isLoaded = func(string) bool { return false }
	}
	from := c.Locations[f.LandedID]
	if from == nil {
		return v
	}
	site := func(loc *Location) ExpeditionSite {
		visited := loc.ID == from.ID
		for _, r := range loc.Ruins {
			if r.Presence == p.ID {
				visited = true
			}
		}
		pres := len(loc.Factions)
		if loc.Colonists > 0 {
			pres++ // the player's colony
		}
		return ExpeditionSite{ID: loc.ID, Tier: c.LocationTier(loc), Tags: loc.CapacityTags(), Visited: visited,
			Presences: pres, Cost: int(math.Round(math.Hypot(loc.X-from.X, loc.Y-from.Y)))}
	}
	here := site(from)
	here.Cost = 0
	v.Sites = append(v.Sites, here)
	for _, loc := range c.shipCandidates(d, f, isLoaded) {
		v.Sites = append(v.Sites, site(loc))
	}
	return v
}

// fleetFuel is a table-flown ship's fuel for planning: what it carries,
// or a full tank when it was never flown by the table (a save or a ship
// from before it). A view reads it; only a real move writes it
// (DepartShip), so asking whether a ship would leave changes nothing.
func fleetFuel(d *Doctrine, f *Fleet) int {
	if d.Expedition != nil && f.Presence.Fuel == 0 && f.LastMove == "" {
		return d.Expedition.FleetFuel
	}
	return f.Presence.Fuel
}

// fleetMove is why a landed table-flown ship would lift off on day, ""
// to stay: "pack_up" when its crew is down to the table's pack_up_at,
// "table" when, past stay_days, a site in reach scores FleetMoveMargin
// better than staying (the jump's fuel counted). to, when not nil, gets the
// destination.
func (c *Campaign) fleetMove(d *Doctrine, t *DoctrineExpedition, f *Fleet, day history.StarDate, to **Location) string {
	if c.seeksGateIn(d, f) {
		return c.gateMove(d, t, f, day, to)
	}
	p := &f.Presence
	v := c.fleetView(d, f, day, nil)
	dest := t.Destination(v, p.Seed)
	if to != nil && dest != nil {
		*to = c.Locations[dest.ID]
	}
	if t.PackUpAt > 0 && p.Population > 0 && p.Population <= t.PackUpAt {
		return "pack_up"
	}
	if dest == nil || int(day-f.LandedDay) < t.StayDays {
		return ""
	}
	if here := v.Site(v.Here); here != nil &&
		t.SiteScore(v, dest, true, p.Seed) < t.SiteScore(v, here, false, p.Seed)+t.FleetMoveMargin {
		return ""
	}
	return "table"
}

// refuelShip is a landed table-flown ship's day at its camp: the fuel in
// its stockpile goes into its tanks, then its camp refines FleetRefuel, up
// to FleetFuel. A ship launched with more (the expedition's start fuel) is
// not cut down to it; it just refines nothing until it is under.
func (c *Campaign) refuelShip(t *DoctrineExpedition, f *Fleet) {
	p := &f.Presence
	if t.FleetFuel <= 0 || p.Fuel >= t.FleetFuel {
		return
	}
	if n := min(p.Stockpile["fuel"], t.FleetFuel-p.Fuel); n > 0 {
		p.Fuel += n
		spend(p.Stockpile, map[string]int{"fuel": n})
	}
	p.Fuel = min(p.Fuel+t.FleetRefuel, t.FleetFuel)
}

// DepartShip lifts a landed ship off on day for its next stop: the
// mothership when its hold is full (it docks there), else a free site in
// range. loaded is the levels in memory (never a target). It reports where
// it is bound, nil when there is nowhere to go (it stays). Deterministic for
// a campaign, fleet and day.
func (c *Campaign) DepartShip(f *Fleet, day history.StarDate, loaded func(string) bool) *Location {
	d := LoadFactionData().Doctrine(f.Presence.Doctrine)
	if d == nil || !f.Landed() {
		return nil
	}
	isLoaded := func(id string) bool { return loaded != nil && loaded(id) }
	if ms := c.Mothership(f.Presence.Faction); ms != nil && d.Ships != nil && d.Ships.HoldTarget > 0 &&
		f.HoldForHome() >= d.Ships.HoldTarget {
		if to := c.Locations[ms.ToID]; to != nil {
			c.liftOff(d, f, to, day)
			f.Returning = true
			return to
		}
	}
	rng := rand.New(rand.NewSource(c.Seed*131 + int64(day)*7919 + fleetNum(f)))
	if t := d.Expedition; t != nil {
		// The table's move first; the stay_days hop as the fallback, to a
		// free site the fuel reaches (reserve kept).
		var to *Location
		why := c.fleetMove(d, t, f, day, &to)
		if why == "" {
			why = "stay" // its stay is over: to the table's best site
			if !c.Locations[f.LandedID].HasSite() {
				why = "launch" // off the Wreck Site
			}
		}
		if to == nil && c.seeksGateIn(d, f) {
			return nil // nowhere toward the gate yet: it stays
		}
		if to == nil {
			why = "fallback"
			v := c.fleetView(d, f, day, isLoaded)
			var cands []*Location
			for _, s := range v.Sites {
				if s.ID != v.Here && t.Affordable(v, &s) {
					cands = append(cands, c.Locations[s.ID])
				}
			}
			to = pickWeighted(rng, cands, func(l *Location) float64 { return economyWeight(d, l) })
		}
		if to == nil || isLoaded(to.ID) {
			return nil
		}
		cost := int(math.Round(math.Hypot(to.X-c.Locations[f.LandedID].X, to.Y-c.Locations[f.LandedID].Y)))
		fuel := fleetFuel(d, f)
		if cost > fuel-max(t.FuelReserve, 0) {
			return nil
		}
		f.Presence.Fuel = fuel - cost
		f.LastMove = why
		c.liftOff(d, f, to, day)
		return to
	}
	to := c.shipTarget(rng, d, f, isLoaded)
	if to == nil {
		return nil
	}
	c.liftOff(d, f, to, day)
	return to
}

// fleetNum is a fleet's number ("fl12" -> 12), for seeding.
func fleetNum(f *Fleet) int64 {
	var n int64
	for _, ch := range f.ID {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int64(ch-'0')
		}
	}
	return n
}

// Mothership is faction's mothership, nil when it has none (or it was
// destroyed).
func (c *Campaign) Mothership(faction string) *Fleet {
	for i := range c.Fleets {
		if f := &c.Fleets[i]; f.Mothership && f.Presence.Faction == faction {
			return f
		}
	}
	return nil
}

// MakeFleetKnown marks f seen when loc is where the ship is in orbit (a
// landing or lift-off the crew watched).
func (c *Campaign) MakeFleetKnown(f *Fleet, loc *Location) {
	if loc != nil && c.CurrentLocationID == loc.ID {
		f.Known = true
	}
}

// newMothership parks faction key's mothership in orbit of at, the site its
// first ship landed on: the system of its landfall, where it arrived from
// outside the galaxy (a mothership's position is the site it orbits, so it
// sits on the landfall system, not the rim point beside it). Its crew is the
// doctrine's; it holds the kit and the techs its ships share.
func (c *Campaign) newMothership(key string, d *Doctrine, at *Location) *Fleet {
	m := d.Ships.Mothership
	if at == nil {
		return nil
	}
	f := c.newShip(key, d, at, max(m.Crew, 1), false)
	f.LandedID = "" // in orbit, never landed
	f.Mothership = true
	if m.Beam != nil && len(m.Beam.Stock) > 0 {
		f.Presence.BeamStock = map[string]int{}
		for k, v := range m.Beam.Stock {
			f.Presence.BeamStock[k] = v
		}
	}
	f.ShipTier = max(len(d.ShipTiers())-1, 0)
	return f
}

// DestroyMothership is the player's victory over faction's mothership (the
// plan's open question, answered in phase D): the mothership is gone with
// its hoard, which is returned for the boarding party's loot; no ship's tier
// rises again and no new ship is built (tiers freeze where they are); and
// every ship bound home scatters: it lands on the free site nearest where it
// was (or stays in orbit with nowhere to go). A faction_mothership_lost
// Legends line records it at loc.
func (c *Campaign) DestroyMothership(faction string, loc *Location) map[string]int {
	ms := c.Mothership(faction)
	if ms == nil {
		return nil
	}
	hoard := ms.Presence.Stockpile
	d := LoadFactionData().Doctrine(ms.Presence.Doctrine)
	kept := c.Fleets[:0]
	for _, f := range c.Fleets {
		if f.ID != ms.ID {
			kept = append(kept, f)
		}
	}
	c.Fleets = kept
	if d != nil && loc != nil {
		c.recordFaction("faction_mothership_lost", loc, nil, d, nil)
	}
	day := c.FactionDay
	for i := range c.Fleets {
		f := &c.Fleets[i]
		if f.Presence.Faction != faction || !f.Returning || d == nil {
			continue
		}
		// Scatter: from where it is now, to the nearest free site.
		x, y := f.Pos(c.StarTime())
		f.Returning = false
		if to := c.nearestFreeSite(d, f, x, y); to != nil {
			speed := math.Max(d.Mobility.ShipSpeed, 1)
			days := max(int(math.Ceil(math.Hypot(to.X-x, to.Y-y)/speed)), 1)
			f.FromID, f.FromX, f.FromY = "", x, y
			f.ToID, f.ToX, f.ToY = to.ID, to.X, to.Y
			f.LaunchDay, f.ArriveDay = day, day+history.StarDate(days)
		}
	}
	return hoard
}

// nearestFreeSite is the free site (no presence, no other ship) nearest
// (x, y) a ship of d may land on.
func (c *Campaign) nearestFreeSite(d *Doctrine, f *Fleet, x, y float64) *Location {
	var best *Location
	bd := math.MaxFloat64
	for _, l := range c.sortedLocations() {
		if len(l.Factions) > 0 || !c.FactionSiteOK(l, false) {
			continue
		}
		busy := false
		for i := range c.Fleets {
			g := &c.Fleets[i]
			if g.ID != f.ID && (g.LandedID == l.ID || g.Kind == FleetShip && !g.Landed() && g.ToID == l.ID) {
				busy = true
			}
		}
		if busy {
			continue
		}
		if dd := math.Hypot(l.X-x, l.Y-y); dd < bd {
			best, bd = l, dd
		}
	}
	return best
}

// FactionRuin is a camp a fleet faction left behind when its ship lifted
// off (phase D4): the level stamps it as a ruin once (ruin_pass over the
// camp, built first if the ship came and went while the level was frozen),
// with a datapad holding the departure's Legends line (EventID).
type FactionRuin struct {
	Presence string           `json:"presence"`
	Faction  string           `json:"faction"`
	Doctrine string           `json:"doctrine"`
	Day      history.StarDate `json:"day"`
	Seed     int64            `json:"seed"`
	EventID  string           `json:"event_id,omitempty"`
	// CampBuilt: the camp stood on the level when the ship left.
	CampBuilt bool `json:"camp_built,omitempty"`
	Stamped   bool `json:"stamped,omitempty"`
	// Built is the ground structures that stood when it left (phase E4:
	// the scar's wrecks), nil unknown (then ShipBuilt from Tier, Techs and
	// DaysLanded).
	Built      map[string]int `json:"built,omitempty"`
	Tier       int            `json:"tier,omitempty"`
	Techs      []string       `json:"techs,omitempty"`
	DaysLanded int            `json:"days_landed,omitempty"`
}

// RuinBuilt is the ground structures a ruin's scar shows: what stood when
// the ship left, else what a ship of its tier and techs would have built
// in the days it was landed.
func (d *Doctrine) RuinBuilt(r *FactionRuin) map[string]int {
	if r.Built != nil {
		return r.Built
	}
	return d.ShipBuilt(r.Tier, r.Techs, r.DaysLanded)
}

// WipeShip drops a landed ship whose whole crew died on its loaded level
// (phase E, closing D's gap: the record was wiped only once the level
// unloaded): faction_wiped on its site, made known (the player was there),
// and the fleet gone. It reports whether it did.
func (c *Campaign) WipeShip(f *Fleet) bool {
	if f == nil || !f.Landed() || f.Mothership {
		return false
	}
	loc := c.Locations[f.LandedID]
	if d := LoadFactionData().Doctrine(f.Presence.Doctrine); d != nil && loc != nil {
		f.Known = true
		f.Presence.Known = true
		c.recordFaction("faction_wiped", loc, &f.Presence, d, nil)
	}
	c.RemoveFleet(f.ID)
	return true
}
