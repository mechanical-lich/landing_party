package campaign

import (
	"container/heap"
	"math"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// The gate and the end of a run (pregenerated_galaxy_plan.md, plan step 4).
// The gate is a site in space beside the controls world; its controls sit
// deep in a guarded vault on that world (a structure fixture, GateVault, so
// no lore budget can drop it). Repairing the controls, an ordinary repair
// order, sets the level flag GateFlag; the world manager then switches the
// gate on for everyone, for good (SwitchGateOn). With the gate on, a ship in
// orbit of it may leave the galaxy, paying GateConfig.LeaveFuel: the run
// ends as a win. A rival colony's default objective is to leave: it heads
// for the controls once it knows where the gate is, switches it on and
// leaves through it, ship by ship.

const (
	// GateControlsKind is the fixture kind of the controls vault.
	GateControlsKind = "gate_controls"
	// GateVaultStructure is the structure script that builds it.
	GateVaultStructure = "gate_vault"
	// GateVaultW, GateVaultH is the vault's footprint.
	GateVaultW, GateVaultH = 27, 23
	// GateFlag is the level flag the controls set when repaired; GateFlagBy
	// names the settlement that repaired them.
	GateFlag   = "gate:on"
	GateFlagBy = "gate:on:by"
	// GateControlsDormant is the controls' blueprint before the repair.
	GateControlsDormant = "gate_controls_dormant"
	// ObjectiveLeave is a doctrine objective: seek the gate and leave.
	ObjectiveLeave = "leave"
	// GateByPlayer is GateOnBy when the player's people switched it on.
	GateByPlayer = "player"
)

// GateConfig is generation.json's gate block.
type GateConfig struct {
	// LeaveFuel is the fuel leaving the galaxy burns.
	LeaveFuel int `json:"leave_fuel"`
}

// DoctrineGate is how a faction with the leave objective uses the gate.
type DoctrineGate struct {
	// SenseRange: a ship landed within this of the gate learns where it is.
	SenseRange float64 `json:"sense_range"`
	// SwitchDays: a ship landed on the controls world off any loaded level
	// switches the gate on after this many days there, losing SwitchLoss
	// crew (never its last).
	SwitchDays int `json:"switch_days"`
	SwitchLoss int `json:"switch_loss"`
	// ExploreStay: before it knows the gate, a ship moves on outward after
	// this many days on a site (its table's stay_days otherwise).
	ExploreStay int `json:"explore_stay"`
}

// ControlsLocation is the world holding the gate's controls, or nil.
func (c *Campaign) ControlsLocation() *Location {
	if c == nil || c.Galaxy == nil {
		return nil
	}
	return c.Locations[c.Galaxy.ControlsID]
}

// GateIsOn reports whether the gate has been switched on.
func (c *Campaign) GateIsOn() bool { return c != nil && c.Galaxy != nil && c.Galaxy.GateOn }

// SwitchGateOn switches the gate on, for good, by (a faction key, or
// GateByPlayer). It reports whether that was news. The gate and its
// controls world become identified, a Legends line records it (beside the
// controls world's history), and when anyone but the player did it an alert
// waits in News.
func (c *Campaign) SwitchGateOn(by string) bool {
	if c == nil || c.Galaxy == nil || c.Galaxy.GateOn {
		return false
	}
	g := c.Galaxy
	g.GateOn, g.GateOnBy, g.GateOnDay = true, by, c.StarDate
	gate, ctl := c.GateLocation(), c.ControlsLocation()
	c.Identify(gate)
	c.Identify(ctl)
	if gate != nil {
		gate.Summary = gateOnSummary
	}
	who := c.gateWho(by)
	c.RecordLive("gate_switched_on", ctl, map[string]string{"who": who})
	if by != GateByPlayer {
		c.News = append(c.News, "The gate is on. "+capitalize(who)+" switched it on.") // DRAFT
	}
	return true
}

// gateOnSummary is the gate's summary once it is on.
const gateOnSummary = "The ring is lit. Beyond it lies the way out of the galaxy." // DRAFT

// gateWho names whoever switched the gate on, for lore text.
func (c *Campaign) gateWho(by string) string {
	if by == GateByPlayer || by == "" {
		return "the " + c.ColonyDisplayName() + " colonists" // DRAFT
	}
	if d := LoadFactionData().Doctrine(c.DoctrineOf(by)); d != nil {
		return "the " + d.MorphAt(nil).Name + c.FactionSuffix(by) // DRAFT
	}
	return by
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}

// GateSummary is the gate's line for the Star Map once identified: dark,
// with its controls on the world beside it, or lit.
func (c *Campaign) GateSummary() string {
	if c.GateIsOn() {
		return gateOnSummary
	}
	name := "a world beside it"
	if ctl := c.ControlsLocation(); ctl != nil {
		name = ctl.Name
	}
	return "A ring of old machinery the size of a moon, adrift and dark. Its controls are on " + name + ", in a vault deep under the surface." // DRAFT
}

// LeaveFuel is the fuel leaving through the gate burns.
func LeaveFuel() int { return genConfig().Gate.LeaveFuel }

// CanLeave reports whether the ship may leave the galaxy now: in orbit of
// the gate, the gate on, fuel covering LeaveFuel. The string says why not.
func (c *Campaign) CanLeave(fuel int) (bool, string) {
	cur := c.CurrentLocation()
	switch {
	case cur == nil || cur.Kind != GateKind:
		return false, "Only from beside the gate." // DRAFT
	case !c.GateIsOn():
		return false, "The gate is dark. Its controls are on " + c.ControlsLocation().Name + "." // DRAFT
	case fuel < LeaveFuel():
		return false, "Leaving needs a full reserve of fuel." // DRAFT
	}
	return true, ""
}

// LeaveGalaxy ends the run as a win: the expedition goes through the gate.
// The caller has charged the fuel.
func (c *Campaign) LeaveGalaxy() {
	c.Won = true
	c.RecordLive("gate_left", c.GateLocation(), map[string]string{"who": c.gateWho(GateByPlayer)})
}

// ---- rival colonies seeking the gate ----

// seeksGate reports whether ship f flies for the gate: its doctrine's
// objective is to leave (a rival colony), and it is an ordinary ship.
func seeksGate(d *Doctrine, f *Fleet) bool {
	return d != nil && d.Objective == ObjectiveLeave && d.Gate != nil && !f.Mothership && !f.IsExpedition() && f.Kind == FleetShip
}

// seeksGateIn is seeksGate in a campaign with a gate (one built by hand has
// none: its rival colony flies by its table as before).
func (c *Campaign) seeksGateIn(d *Doctrine, f *Fleet) bool {
	return c.GateLocation() != nil && seeksGate(d, f)
}

// ShipSeeksGate reports whether f flies for the gate (a rival colony's ship).
func (c *Campaign) ShipSeeksGate(f *Fleet) bool {
	return c.seeksGateIn(LoadFactionData().Doctrine(f.Presence.Doctrine), f)
}

// gateKnown reports whether faction knows where the gate is: it has sensed
// it, or the gate is on (lit, it is seen across the galaxy).
func (c *Campaign) gateKnown(faction string) bool {
	if c.GateIsOn() {
		return true
	}
	r := c.FactionOf(faction)
	return r != nil && r.GateKnown
}

// learnGate makes the gate known to f's faction when loc lies within its
// doctrine's sense range of the gate.
func (c *Campaign) learnGate(d *Doctrine, f *Fleet, loc *Location) {
	gate := c.GateLocation()
	if !c.seeksGateIn(d, f) || loc == nil {
		return
	}
	if math.Hypot(loc.X-gate.X, loc.Y-gate.Y) > d.Gate.SenseRange {
		return
	}
	c.EnsureFactionTable()
	if r := c.FactionOf(f.Presence.Faction); r != nil && !r.GateKnown {
		r.GateKnown = true
		c.recordFaction("faction_found_gate", c.ControlsLocation(), &f.Presence, d, nil)
	}
}

// gateGoal is where a gate-seeking ship heads: the gate once it is on, the
// controls world while it is off, nil while its faction does not know it.
func (c *Campaign) gateGoal(f *Fleet) *Location {
	if c.Galaxy == nil || !c.gateKnown(f.Presence.Faction) {
		return nil
	}
	if c.GateIsOn() {
		return c.GateLocation()
	}
	return c.ControlsLocation()
}

// shipReach is how far a gate-seeking ship may jump at once: its table's
// tank less the reserve (seeking the gate, it jumps as far as its fuel
// carries it, past its mobility's everyday base_range).
func shipReach(d *Doctrine, t *DoctrineExpedition) float64 {
	if t != nil && t.FleetFuel > 0 {
		return float64(t.FleetFuel - max(t.FuelReserve, 0))
	}
	if d.Mobility.BaseRange > 0 {
		return d.Mobility.BaseRange
	}
	return math.Inf(1)
}

// gateMove is a gate-seeking ship's move (fleetMove's, for such a ship):
// the next hop on the shortest route of stops within reach to where it is
// heading, "gate" once its faction knows the gate (to the controls world
// while it is off, the gate once it is on), "explore" before (to the system
// deepest in the galaxy, farthest from every landfall, where a gate is
// likeliest, then new ground farthest from where it has been, until the gate is
// in its sense range). "" to stay: on the controls world with the gate off,
// before its explore stay is up, while the tank refills, or with nowhere
// to go. to, when not nil, gets the hop.
func (c *Campaign) gateMove(d *Doctrine, t *DoctrineExpedition, f *Fleet, day history.StarDate, to **Location) string {
	here := c.Locations[f.LandedID]
	if here == nil {
		return ""
	}
	why := "gate"
	goal := c.gateGoal(f)
	if goal == nil {
		if here.HasSite() && int(day-f.LandedDay) < max(d.Gate.ExploreStay, 1) {
			return ""
		}
		why, goal = "explore", c.exploreTarget(d, t, f, here)
		if goal == nil {
			return ""
		}
	}
	if goal.ID == here.ID {
		return "" // on the controls world: switching the gate on
	}
	hop := c.routeHop(d, t, f, here, goal)
	if hop == nil {
		return ""
	}
	cost := int(math.Round(math.Hypot(hop.X-here.X, hop.Y-here.Y)))
	if cost > fleetFuel(d, f)-max(t.FuelReserve, 0) {
		return "" // the tank refills first
	}
	if to != nil {
		*to = hop
	}
	return why
}

// visitedBy reports whether f has landed on loc (a ruin of its camp there).
func visitedBy(f *Fleet, loc *Location) bool {
	for _, r := range loc.Ruins {
		if r.Presence == f.Presence.ID {
			return true
		}
	}
	return false
}

// exploreTarget is where a ship that does not know the gate heads: the
// deepest system (farthest from every landfall) while it has not been there,
// then the unvisited system it can route to farthest from everywhere it has
// been. nil when it has seen them all.
func (c *Campaign) exploreTarget(d *Doctrine, t *DoctrineExpedition, f *Fleet, here *Location) *Location {
	if c.Galaxy == nil {
		return nil
	}
	if f.ExploreTo == here.ID {
		f.ExploreTo = "" // arrived
	}
	if l := c.Locations[f.ExploreTo]; l != nil && !visitedBy(f, l) && c.gateStop(f, l) && c.routeHop(d, t, f, here, l) != nil {
		return l // still on its way there
	}
	// The deepest system: the one farthest from every landfall point on the
	// rim (every star and the rim are in sight; a gate is hidden deep).
	deep := func(l *Location) float64 {
		v := math.Inf(1)
		for _, lf := range c.Galaxy.Landfalls {
			v = math.Min(v, math.Hypot(l.X-lf.X, l.Y-lf.Y))
		}
		return v
	}
	var core *Location
	for _, l := range c.sortedLocations() {
		if !c.gateStop(f, l) && l.ID != here.ID {
			continue
		}
		if !l.HasSite() || l.Tutorial || l.Kind == EncounterKind {
			continue
		}
		if core == nil || deep(l) > deep(core) {
			core = l
		}
	}
	if core != nil && core.ID != here.ID && !visitedBy(f, core) && c.routeHop(d, t, f, here, core) != nil {
		f.ExploreTo = core.ID
		return core
	}
	// Then new ground: the unvisited stop farthest from everywhere it has
	// been (its sense range covers what lies near those).
	seen := []*Location{here}
	for _, l := range c.sortedLocations() {
		if visitedBy(f, l) {
			seen = append(seen, l)
		}
	}
	var best *Location
	bv := -1.0
	for _, l := range c.sortedLocations() {
		if l.ID == here.ID || visitedBy(f, l) || !c.gateStop(f, l) {
			continue
		}
		v := math.Inf(1)
		for _, s := range seen {
			v = math.Min(v, math.Hypot(l.X-s.X, l.Y-s.Y))
		}
		if v > bv && c.routeHop(d, t, f, here, l) != nil {
			best, bv = l, v
		}
	}
	if best != nil {
		f.ExploreTo = best.ID
	}
	return best
}

// gateStop reports whether a gate-seeking ship may land on loc on its way:
// a free site a fleet may take, or the controls world (whoever holds it).
func (c *Campaign) gateStop(f *Fleet, loc *Location) bool {
	if loc == nil || !loc.HasSite() || loc.Tutorial || loc.Kind == EncounterKind {
		return false
	}
	if c.Galaxy != nil && loc.ID == c.Galaxy.ControlsID {
		return true
	}
	if len(loc.Factions) > 0 || !c.fleetSiteOK(f, loc) {
		return false
	}
	for i := range c.Fleets {
		g := &c.Fleets[i]
		if g.ID != f.ID && (g.LandedID == loc.ID || g.Kind == FleetShip && !g.Landed() && g.ToID == loc.ID) {
			return false
		}
	}
	return true
}

// routeHop is the first hop of the shortest route from here to goal over
// stops a gate-seeking ship may land on, each hop within its reach (the
// goal may be the gate, which is no stop). nil when no route exists.
func (c *Campaign) routeHop(d *Doctrine, t *DoctrineExpedition, f *Fleet, here, goal *Location) *Location {
	reach := shipReach(d, t)
	nodes := []*Location{here}
	for _, l := range c.sortedLocations() {
		if l.ID != here.ID && l.ID != goal.ID && c.gateStop(f, l) {
			nodes = append(nodes, l)
		}
	}
	nodes = append(nodes, goal)
	gi := len(nodes) - 1
	dist := make([]float64, len(nodes))
	prev := make([]int, len(nodes))
	for i := range dist {
		dist[i], prev[i] = math.Inf(1), -1
	}
	dist[0] = 0
	pq := &routeQueue{{0, 0}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(routeItem)
		if cur.d > dist[cur.i] {
			continue
		}
		if cur.i == gi {
			break
		}
		a := nodes[cur.i]
		for j, b := range nodes {
			if j == cur.i {
				continue
			}
			e := math.Hypot(a.X-b.X, a.Y-b.Y)
			if e > reach {
				continue
			}
			if nd := cur.d + e; nd < dist[j] {
				dist[j], prev[j] = nd, cur.i
				heap.Push(pq, routeItem{j, nd})
			}
		}
	}
	if math.IsInf(dist[gi], 1) {
		return nil
	}
	i := gi
	for prev[i] != 0 {
		i = prev[i]
	}
	return nodes[i]
}

type routeItem struct {
	i int
	d float64
}

type routeQueue []routeItem

func (q routeQueue) Len() int { return len(q) }
func (q routeQueue) Less(i, j int) bool {
	return q[i].d < q[j].d || q[i].d == q[j].d && q[i].i < q[j].i
}
func (q routeQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *routeQueue) Push(x interface{}) { *q = append(*q, x.(routeItem)) }
func (q *routeQueue) Pop() interface{} {
	old := *q
	n := len(old)
	it := old[n-1]
	*q = old[:n-1]
	return it
}

// stepGateShip is a landed gate-seeking ship's day: it may learn where the
// gate is, and on the controls world with the gate off (its level not
// loaded) it works at the controls, switching the gate on after its
// doctrine's switch_days, at a cost of switch_loss crew.
func (c *Campaign) stepGateShip(d *Doctrine, f *Fleet, loc *Location, isLoaded func(string) bool) {
	if !c.seeksGateIn(d, f) {
		return
	}
	c.learnGate(d, f, loc)
	if c.GateIsOn() || c.Galaxy == nil || loc.ID != c.Galaxy.ControlsID || isLoaded(loc.ID) || !c.gateKnown(f.Presence.Faction) {
		return
	}
	f.GateWork++
	if f.GateWork < max(d.Gate.SwitchDays, 1) {
		return
	}
	f.Presence.Population = max(f.Presence.Population-max(d.Gate.SwitchLoss, 0), 1)
	c.SwitchGateOn(f.Presence.Faction)
}

// leaveThroughGate is a gate-seeking ship arriving at the lit gate: it
// leaves the galaxy, its crew and hold with it. A Legends line records it;
// the player hears (News) only of a faction they have met. It reports
// whether f left (the caller drops the fleet).
func (c *Campaign) leaveThroughGate(d *Doctrine, f *Fleet) bool {
	gate := c.GateLocation()
	if !c.seeksGateIn(d, f) || f.ToID != gate.ID || !c.GateIsOn() {
		return false
	}
	met := f.Known || f.Presence.Known || c.Observer
	c.recordFaction("faction_left_galaxy", c.ControlsLocation(), &f.Presence, d, nil)
	if met {
		c.News = append(c.News, "A "+d.MorphAt(nil).Singular+" ship has gone through the gate and left the galaxy.") // DRAFT
	}
	return true
}
