package campaign

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// Playable races (phase F; docs/brainstorm/playable_races_plan.md). A race
// whose mobility spreads (the hive's launcher, the autonz' signal) has no
// ship: the player's people are planet-carried presences of faction
// PlayerFaction running the race's doctrine, stepped by Tier 1 while frozen
// and run by their settlement director while loaded, exactly as a rival's.
// The player views a planet they hold to give orders there. Campaign.Player
// keeps the doctrine key (PlayerDoctrine) and, for a spreading race, the
// faction key; its techs are unused in step 1 (each presence knows its own).

// PlayerFaction is the faction key of a spreading race's presences, apart
// from every rival's (a rival hive stays "hive").
const PlayerFaction = "player"

// Campaign modes (Doctrine.CampaignMode).
const (
	ModeShip   = "ship"   // the colony, the greyish: hold, roster, fuel, travel
	ModeSpread = "spread" // the hive, the autonz: no ship; presences spread
)

// DoctrineRace is what a doctrine needs to be played (phase F): its blurb on
// the title screen, the player's start, and the win and loss rules of a
// spreading race. All its text is draft.
type DoctrineRace struct {
	// Order places the race on the title screen's row (after the colony;
	// ties by key).
	Order int        `json:"order,omitempty"`
	Blurb string     `json:"blurb,omitempty"`
	Start *RaceStart `json:"start,omitempty"`
	Win   *RaceWin   `json:"win,omitempty"`
	Loss  *RaceEnd   `json:"loss,omitempty"`
}

// RaceStart is a spreading race's first presence, on the start system. A
// ship race (phase F step 6: the greys) reads Units as its crew by type,
// Stockpile as its hold, Gear as each type's starting kit and Aboard as the
// structures standing on the ship from the start.
type RaceStart struct {
	Stage     int                 `json:"stage"`
	Units     map[string]int      `json:"units"`
	Larvae    int                 `json:"larvae,omitempty"`
	Stockpile map[string]int      `json:"stockpile,omitempty"`
	Gear      map[string][]string `json:"gear,omitempty"`
	Aboard    []string            `json:"aboard,omitempty"`
}

// ShipStart is a ship race's start (nil: the colony's, StartColonists by
// the crew split).
func (d *Doctrine) ShipStart() *RaceStart {
	if d == nil || d.CampaignMode() != ModeShip || d.Race == nil || d.Race.Start == nil || len(d.Race.Start.Units) == 0 {
		return nil
	}
	return d.Race.Start
}

// RaceWin is a spreading race's win: presences (awake) on Sites systems at
// once.
type RaceWin struct {
	Sites int    `json:"sites"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
}

// RaceEnd is the end screen's words.
type RaceEnd struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
}

// SpreadRun reports whether c is a spreading race's run (no ship). An
// observer run is never one.
func (c *Campaign) SpreadRun() bool {
	if c == nil || c.Observer {
		return false
	}
	d := PlayerDoctrine(c)
	return d != nil && d.CampaignMode() == ModeSpread
}

// PlayableDoctrines is the doctrines the title screen offers, the default
// (the colony) first, then by key.
func PlayableDoctrines() []string {
	fd := LoadFactionData()
	var out []string
	for k, d := range fd.Doctrines {
		if d.Playable && d.Settlement != nil && k != DefaultPlayerDoctrine {
			out = append(out, k)
		}
	}
	order := func(k string) int {
		if r := fd.Doctrine(k).Race; r != nil {
			return r.Order
		}
		return 0
	}
	sort.Slice(out, func(i, j int) bool {
		if order(out[i]) != order(out[j]) {
			return order(out[i]) < order(out[j])
		}
		return out[i] < out[j]
	})
	if d := fd.Doctrine(DefaultPlayerDoctrine); d != nil && d.Playable {
		out = append([]string{DefaultPlayerDoctrine}, out...)
	}
	return out
}

// AddPlayerBrood puts a spreading race's first presence on the start system:
// faction PlayerFaction, the doctrine's race.start, Known. The start system
// is the player's from day zero (Visited), as the expedition's first landing
// is in an observer run. The player's faction is neutral to itself (a stray
// Stance lookup).
func (c *Campaign) AddPlayerBrood() (*FactionPresence, error) {
	d := PlayerDoctrine(c)
	loc := c.Locations[StartLocationID]
	if d == nil || d.Race == nil || d.Race.Start == nil || loc == nil {
		return nil, fmt.Errorf("race: %q has no start, or no start system", PlayerDoctrineKey(c))
	}
	st := d.Race.Start
	pop := 0
	units := map[string]int{}
	for k, v := range st.Units {
		units[k] = v
		pop += v
	}
	stock := map[string]int{}
	for k, v := range st.Stockpile {
		stock[k] = v
	}
	c.FactionSeq++
	loc.Factions = append(loc.Factions, FactionPresence{
		ID: fmt.Sprintf("fp%d", c.FactionSeq), Faction: PlayerFaction, Doctrine: PlayerDoctrineKey(c),
		Population: pop, Stage: st.Stage, Units: units, Larvae: st.Larvae, Stockpile: stock,
		ArrivedDay: c.FactionDay, LastStepDay: c.FactionDay, LastShotDay: NeverFired,
		Seed: c.Seed*7919 + 23, Known: true,
	})
	loc.Visited = true
	if c.Diplomacy == nil {
		c.Diplomacy = map[string]string{}
	}
	c.Diplomacy[PlayerFaction] = StanceNeutral
	if c.Player != nil {
		c.Player.Faction = PlayerFaction
	}
	return &loc.Factions[len(loc.Factions)-1], nil
}

// PlayerSites is the locations holding an awake presence of the player's,
// by ID.
func (c *Campaign) PlayerSites() []*Location {
	var out []*Location
	for _, loc := range c.sortedLocations() {
		for i := range loc.Factions {
			if p := &loc.Factions[i]; p.Faction == PlayerFaction && !p.Dormant {
				out = append(out, loc)
				break
			}
		}
	}
	return out
}

// HoldsSite reports whether the player has a presence (awake or not) on loc.
func (c *Campaign) HoldsSite(loc *Location) bool {
	return loc != nil && loc.Presence(PlayerFaction) != nil
}

// SpreadOutcome is a spreading race's end, checked once a day: lost when no
// presence of the player's (a dormant node counts: it can still wake) and no
// egg or signal of it in flight is left; won when awake presences stand on
// race.win.sites systems at once. Neither: "".
func (c *Campaign) SpreadOutcome() string {
	if !c.SpreadRun() || c.Won || c.Lost {
		return ""
	}
	d := PlayerDoctrine(c)
	if w := d.Race.Win; w != nil && w.Sites > 0 && len(c.PlayerSites()) >= w.Sites {
		return "won"
	}
	for _, loc := range c.Locations {
		if loc.Presence(PlayerFaction) != nil {
			return ""
		}
	}
	for _, f := range c.Fleets {
		if f.Presence.Faction == PlayerFaction {
			return ""
		}
	}
	return "lost"
}

// playerSeat reports whether loc is the race run's seat: the world holding
// its largest awake presence (ties to the first by ID), the one whose Tier 1
// research walk steps the shared pool.
func (c *Campaign) playerSeat(loc *Location) bool {
	var best *Location
	bestPop := -1
	for _, l := range c.PlayerSites() {
		if p := l.Presence(PlayerFaction); p != nil && p.Population > bestPop {
			best, bestPop = l, p.Population
		}
	}
	return best == loc
}

// SyncPlayerTechs makes a race run's techs one pool (phase F, the plan's
// open question 5): every tech any of the player's presences or eggs in
// flight knows joins Campaign.Player.KnownTechs, in the order learned, and
// every one of them then knows the whole pool. The game folds its loaded
// settlements' live research into their presences first.
func (c *Campaign) SyncPlayerTechs() {
	if !c.SpreadRun() || c.Player == nil {
		return
	}
	pool := append([]string(nil), c.Player.KnownTechs...)
	add := func(techs []string) {
		for _, t := range techs {
			if !hasStr(pool, t) {
				pool = append(pool, t)
			}
		}
	}
	var mine []*FactionPresence
	for _, loc := range c.sortedLocations() {
		for i := range loc.Factions {
			if p := &loc.Factions[i]; p.Faction == PlayerFaction {
				mine = append(mine, p)
			}
		}
	}
	for i := range c.Fleets {
		if c.Fleets[i].Presence.Faction == PlayerFaction {
			mine = append(mine, &c.Fleets[i].Presence)
		}
	}
	for _, p := range mine {
		add(p.KnownTechs)
	}
	c.Player.KnownTechs = pool
	for _, p := range mine {
		p.KnownTechs = append([]string(nil), pool...)
	}
}

// Manual reports whether the player took group over on p.
func (p *FactionPresence) Manual(group string) bool { return hasStr(p.TakenOver, group) }

// SetManual takes group over on p, or gives it back.
func (p *FactionPresence) SetManual(group string, on bool) {
	if on == p.Manual(group) {
		return
	}
	if on {
		p.TakenOver = append(p.TakenOver, group)
		return
	}
	out := p.TakenOver[:0]
	for _, g := range p.TakenOver {
		if g != group {
			out = append(out, g)
		}
	}
	p.TakenOver = out
}

// GroupLaunch is the rung group of a spreading race's launch or transmit
// walk: taken over, the player loads it by hand. The aim (LaunchTarget) is
// always the player's (aimed).
const GroupLaunch = "launch"

// aimed narrows a spread's candidate sites to the player's aim: a race
// run's launcher or signal never fires on the director's choice (the owner,
// 2026-11-12: the director does not spread to other planets by itself),
// only at the armed target, and with none it holds fire, live and in Tier 1
// alike. Every rival keeps its candidates.
func aimed(p *FactionPresence, cands []*Location) []*Location {
	if p.Faction != PlayerFaction {
		return cands
	}
	for _, l := range cands {
		if l.ID == p.LaunchTarget {
			return []*Location{l}
		}
	}
	return nil
}

// SpreadStatus is a launcher's or a signal's readiness for the player's
// panel: the population against the threshold, whether the stage and the
// structure allow it, the days of cooldown left, the range, and the sites in
// range it could found on now (ignoring the aim).
type SpreadStatus struct {
	Pop, Threshold int
	StageOK        bool
	StructureLost  bool
	CooldownLeft   int
	Range          float64
	Targets        []*Location
}

// LaunchStatus is p's launcher's status on loc. It draws no random numbers
// and changes nothing.
func (c *Campaign) LaunchStatus(loc *Location, p *FactionPresence, isLoaded func(string) bool) SpreadStatus {
	fd := LoadFactionData()
	d := fd.Doctrine(p.Doctrine)
	var st SpreadStatus
	if d == nil {
		return st
	}
	m := d.Mobility
	cap := d.CapacityAt(loc)
	st.Pop, st.Threshold = p.Population, d.FireThreshold(cap)
	st.StageOK = p.Stage < len(d.Stages) && d.Stages[p.Stage].Launcher
	st.StructureLost = p.LauncherLost
	since := p.LastShotDay
	if since == NeverFired && cap < m.CrowdedCapacity {
		since = p.ArrivedDay
	}
	if since != NeverFired {
		cool := int(math.Ceil(d.CooldownAt(cap) / c.pressure(fd)))
		st.CooldownLeft = max(0, cool-int(c.FactionDay-since))
	}
	st.Range = d.Range(p.KnownTechs)
	if isLoaded == nil {
		isLoaded = func(string) bool { return false }
	}
	inbound := map[string]bool{}
	for _, f := range c.Fleets {
		if f.Presence.Faction == p.Faction {
			inbound[f.ToID] = true
		}
	}
	for _, t := range c.sortedLocations() {
		if t == loc || inbound[t.ID] || isLoaded(t.ID) || !c.FactionSiteOK(t, false) || c.HeldBySide(t, p.Faction) {
			continue
		}
		if math.Hypot(t.X-loc.X, t.Y-loc.Y) <= st.Range {
			st.Targets = append(st.Targets, t)
		}
	}
	return st
}

// SignalStatus is p's signal's status on loc, its stage's relay pylons
// counted as standing (Tier 1's count; the live rule counts the level's).
func (c *Campaign) SignalStatus(loc *Location, p *FactionPresence, isLoaded func(string) bool) SpreadStatus {
	fd := LoadFactionData()
	d := fd.Doctrine(p.Doctrine)
	var st SpreadStatus
	if d == nil {
		return st
	}
	m := d.Mobility
	cap := d.CapacityAt(loc)
	st.Pop, st.Threshold = p.Population, max(d.FireThreshold(cap), max(m.ShotPop, 1)+2)
	if m.SignalConversion > 0 && p.Conversion >= m.SignalConversion {
		st.Threshold = max(m.ShotPop, 1) + 2 // the conversion trigger
	}
	st.StageOK = !p.Dormant && p.Stage < len(d.Stages) && d.Stages[p.Stage].Signal
	if p.LastShotDay != NeverFired {
		cool := int(math.Ceil(float64(m.Cooldown) / c.pressure(fd)))
		st.CooldownLeft = max(0, cool-int(c.FactionDay-p.LastShotDay))
	}
	st.Range = d.SignalRange(p.Stage, p.KnownTechs, -1)
	if isLoaded == nil {
		isLoaded = func(string) bool { return false }
	}
	st.Targets = c.signalCands(d, loc, p, isLoaded, -1)
	return st
}

// seedRaceFactions seeds a spreading race's rivals (the plan's open question
// 8): the rival colony first, so the humans are there from Rival factions 1;
// the player's own doctrine left out of the bag; the rest by their seed_from
// order, the cycle repeating past the bag's size. Each lands on a site
// weighted toward its economy, from a stream of its own. A colony run's
// seeding is untouched (seedFactions).
func (c *Campaign) seedRaceFactions(n int) {
	fd := LoadFactionData()
	own := PlayerDoctrineKey(c)
	var order []string
	if d := fd.Doctrine(RivalColonyDoctrine); d != nil && d.Seedable && own != DefaultPlayerDoctrine {
		order = append(order, RivalColonyDoctrine) // the rival colony first
	}
	var rest []string
	for _, k := range fd.seedable() {
		if BaseDoctrine(k) != own && k != RivalColonyDoctrine {
			rest = append(rest, k)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool { return fd.Doctrine(rest[i]).SeedFrom < fd.Doctrine(rest[j]).SeedFrom })
	order = append(order, rest...)
	if len(order) == 0 {
		return
	}
	rng := rand.New(rand.NewSource(c.Seed*7919 + 15485863))
	taken := map[string]bool{}
	for i := 0; i < n; i++ {
		dkey := order[i%len(order)]
		d := fd.Doctrine(dkey)
		key := fd.FactionKeyFor(dkey)
		c.addFactionRow(key, dkey, 0, 0)
		var cands []*Location
		for _, loc := range c.sortedLocations() {
			if !taken[loc.ID] && c.FactionSiteOK(loc, true) {
				cands = append(cands, loc)
			}
		}
		loc := pickWeighted(rng, cands, func(l *Location) float64 { return economyWeight(d, l) })
		if loc == nil {
			return
		}
		taken[loc.ID] = true
		pop := max(d.SeedPopulation, 1)
		if d.Mobility.Kind == MobilityFleet {
			if cc := d.CrewCap(0); cc > 0 {
				pop = min(pop, cc)
			}
			c.newShip(key, d, loc, pop, true)
			if d.Ships != nil && d.Ships.Mothership != nil && c.Mothership(key) == nil {
				c.newMothership(key, d, taken)
			}
			continue
		}
		p := c.newPresence(key, dkey, d, loc, pop)
		c.recordFaction(d.arrivalEvent(), loc, p, d, nil)
	}
}

// RaceContact is a race run's first meeting with faction at loc (the plan's
// open question 10): a neutral faction (the rival colony) turns hostile,
// everywhere, as a colony player's beam-down onto its site does. It reports
// whether the stance changed. Not in a colony run (contact there is the
// beam-down).
func (c *Campaign) RaceContact(faction string, loc *Location) bool {
	if !c.SpreadRun() || faction == PlayerFaction || c.Stance(faction) != StanceNeutral {
		return false
	}
	return c.SetStance(faction, StanceHostile, loc)
}

// raceReach is the reach of the player's presence p on loc: its launcher's
// or signal's range once its stage carries one (the hive's spire stage, the
// autonz' hub), else 0.
func raceReach(d *Doctrine, p *FactionPresence) float64 {
	if d == nil || p.Dormant || p.Stage >= len(d.Stages) {
		return 0
	}
	st := d.Stages[p.Stage]
	switch {
	case st.Launcher:
		return d.Range(p.KnownTechs)
	case st.Signal:
		return d.SignalRange(p.Stage, p.KnownTechs, -1)
	}
	return 0
}

// InRaceReach reports whether loc lies within the reach of any world the
// player holds (the race run's fog: what it can know and view).
func (c *Campaign) InRaceReach(loc *Location) bool {
	if loc == nil {
		return false
	}
	fd := LoadFactionData()
	for _, l := range c.PlayerSites() {
		p := l.Presence(PlayerFaction)
		if r := raceReach(fd.Doctrine(p.Doctrine), p); r > 0 && math.Hypot(loc.X-l.X, loc.Y-l.Y) <= r {
			return true
		}
	}
	return false
}

// ChartRaceReach charts every site within the reach of a world the player
// holds (the plan's open question 6: the range circle charts sites; their
// presences stay unknown until held or scouted). It returns the newly
// charted.
func (c *Campaign) ChartRaceReach() []*Location {
	if !c.SpreadRun() {
		return nil
	}
	var out []*Location
	for _, loc := range c.sortedLocations() {
		if !loc.Discovered && loc.HasSite() && c.InRaceReach(loc) {
			loc.Discovered = true
			out = append(out, loc)
		}
	}
	return out
}
