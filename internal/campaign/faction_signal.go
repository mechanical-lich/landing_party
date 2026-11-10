package campaign

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// The autonz (phase E; docs/developer/factions.md, "The autonz"): a
// planet-carried faction whose economy is conversion and whose mobility is a
// signal. A presence converts its site's ground (Conversion, the share of its
// patch laid with the doctrine's tile), and the share is its economy: it
// caps the presence's growth (ConvertCap), it rises faster on ore, and once
// it reaches the doctrine's SignalConversion (or the population its firing
// threshold) a stage marked Signal sends a signal on its cooldown. A signal
// is no projectile: it founds a dormant presence on a site within range,
// which sleeps WakeDays and then wakes as a node. A fully converted site
// (Conversion at the cap) stops growing and spends its population on
// signals.

const (
	// MobilitySignal is the autonz' mobility kind.
	MobilitySignal = "signal"
	// EconomyConvert is the autonz' economy: conversion is the tally.
	EconomyConvert = "convert"
)

// arrivalEvent is the Legends event a seeded presence of d records.
func (d *Doctrine) arrivalEvent() string {
	if d.ArrivalEvent != "" {
		return d.ArrivalEvent
	}
	return "faction_arrived"
}

// Converts reports whether d's economy is conversion.
func (d *Doctrine) Converts() bool { return d != nil && d.Economy == EconomyConvert }

// ConversionCap is the most of its patch a presence of d converts (1 when
// the doctrine sets none).
func (d *Doctrine) ConversionCap() float64 {
	if d.Conversion.Cap > 0 {
		return d.Conversion.Cap
	}
	return 1
}

// FullyConverted reports whether p has converted all it will.
func (d *Doctrine) FullyConverted(p *FactionPresence) bool {
	return d.Converts() && p.Conversion >= d.ConversionCap()-1e-9
}

// ConvertCap is how many bodies p can carry on loc under the convert
// economy: the site's capacity times Floor plus the rest by the share of the
// cap converted, at least the doctrine's minimum. Other economies carry the
// site's capacity.
func (d *Doctrine) ConvertCap(loc *Location, p *FactionPresence) int {
	cap := d.CapacityAt(loc)
	if !d.Converts() {
		return cap
	}
	k := math.Min(p.Conversion/d.ConversionCap(), 1)
	f := d.Conversion.Floor + (1-d.Conversion.Floor)*math.Max(k, 0)
	n := int(math.Ceil(float64(cap) * f))
	return min(max(n, d.Capacity.Min, 1), cap)
}

// ConversionRate is the conversion rate's multiplier for a presence knowing
// techs: 1 plus each known tech's conversion_rate.
func (d *Doctrine) ConversionRate(techs []string) float64 {
	r := 1.0
	for _, t := range d.Research {
		if t.ConversionRate != 0 && hasStr(techs, t.ID) {
			r += t.ConversionRate
		}
	}
	return r
}

// convertWork is a day of converting, abstractly (Tier 1's "work" for the
// convert economy): per_day times the rate techs, the scenario's hazard
// (never below 0), the population's share of the capacity and one plus the
// site's tag bonuses (ore feeds it), up to the cap.
func (c *Campaign) convertWork(d *Doctrine, loc *Location, p *FactionPresence) {
	cv := d.Conversion
	if cv.PerDay <= 0 {
		return
	}
	cap := d.CapacityAt(loc)
	tags := 1.0
	for _, t := range loc.CapacityTags() {
		tags += cv.Tags[t]
	}
	rate := cv.PerDay * d.ConversionRate(p.KnownTechs) * math.Max(d.HazardFactor(loc.ScenarioID), 0) *
		math.Min(float64(p.Population)/float64(cap), 1) * math.Max(tags, 0)
	p.Conversion = math.Min(p.Conversion+rate, d.ConversionCap())
}

// SignalReady reports whether p may send a signal on day: a signal stage,
// at the firing threshold of its site's full capacity or at the doctrine's
// signal conversion, enough bodies left after the signal, and the cooldown
// passed.
func (c *Campaign) signalReady(fd *FactionData, d *Doctrine, loc *Location, p *FactionPresence, day history.StarDate) bool {
	m := d.Mobility
	if p.Dormant || p.Stage >= len(d.Stages) || !d.Stages[p.Stage].Signal {
		return false
	}
	cap := d.CapacityAt(loc)
	trig := p.Population >= d.FireThreshold(cap) || (m.SignalConversion > 0 && p.Conversion >= m.SignalConversion)
	if !trig || p.Population < max(m.ShotPop, 1)+2 {
		return false
	}
	cool := int(math.Ceil(float64(m.Cooldown) / c.pressure(fd)))
	return p.LastShotDay == NeverFired || int(day-p.LastShotDay) >= cool
}

// trySignal sends a signal when p is ready: a site within the signal range
// (base range plus range techs), eligible, not loaded and holding none of
// this faction, nearer ones likelier, gets a dormant presence of EggPop that
// wakes WakeDays later. The signal costs ShotPop. With no site in range
// nothing is sent and the cooldown is not spent. It returns the site.
func (c *Campaign) trySignal(rng *rand.Rand, fd *FactionData, d *Doctrine, loc *Location, p *FactionPresence, day history.StarDate, isLoaded func(string) bool, pylons int) *Location {
	if !c.signalReady(fd, d, loc, p, day) {
		return nil
	}
	m := d.Mobility
	cands := c.signalCands(d, loc, p, isLoaded, pylons)
	target := pickWeighted(rng, cands, func(l *Location) float64 {
		return economyWeight(d, l) / (1 + math.Hypot(l.X-loc.X, l.Y-loc.Y))
	})
	if target == nil {
		return nil
	}
	p.Population -= max(m.ShotPop, 1)
	if d.HasTypes() {
		// The signal's bodies: wire placers first, then relay drones.
		d.ShotUnits(p, loc, p.UnitsTotal()-p.Population)
	}
	p.LastShotDay = day
	q := c.newPresence(p.Faction, d, target, max(m.EggPop, 1))
	q.Dormant = true
	q.WakeDay = day + history.StarDate(max(m.WakeDays, 1))
	q.KnownTechs = append([]string(nil), p.KnownTechs...)
	q.Stage = 0
	if target.ID == c.CurrentLocationID {
		q.Known = true
	}
	c.recordFaction("faction_signalled", loc, p, d, map[string]string{"to": target.Name})
	return target
}

// signalCands are the sites a signal from p on loc may found a presence on:
// eligible, not loaded, holding none of p's faction, within its signal range
// with pylons relay pylons standing (-1: its stage's count, Tier 1).
func (c *Campaign) signalCands(d *Doctrine, loc *Location, p *FactionPresence, isLoaded func(string) bool, pylons int) []*Location {
	r := d.SignalRange(p.Stage, p.KnownTechs, pylons)
	var cands []*Location
	for _, t := range c.sortedLocations() {
		if t == loc || isLoaded(t.ID) || !c.FactionSiteOK(t, false) || t.HasFaction(p.Faction) {
			continue
		}
		if math.Hypot(t.X-loc.X, t.Y-loc.Y) <= r {
			cands = append(cands, t)
		}
	}
	return cands
}

// SignalRange is the signal range of a presence of d at stage knowing techs
// with pylons relay pylons standing: the base range, PylonRange for each
// pylon up to the stage's relay_pylon count, and the range techs. pylons -1
// is the stage's count (Tier 1 counts its pylons as standing; at the hub
// 16 + 4 is Phase E's 20).
func (d *Doctrine) SignalRange(stage int, techs []string, pylons int) float64 {
	n := 0
	if stage >= 0 && stage < len(d.Stages) {
		n = d.Stages[stage].Structures[PylonKey]
	}
	if pylons >= 0 {
		n = min(n, pylons)
	}
	return d.Range(techs) + d.Mobility.PylonRange*float64(n)
}

// PylonKey is the structure whose standing count lengthens a signal (the
// autonz' relay pylon).
const PylonKey = "relay_pylon"

// stepDormant is a dormant presence's day: asleep until its wake day, then
// awake (faction_woke) from that day on. It reports whether p is awake.
func (c *Campaign) stepDormant(loc *Location, p *FactionPresence, d *Doctrine, day history.StarDate) bool {
	if !p.Dormant {
		return true
	}
	if day < p.WakeDay {
		return false
	}
	c.wake(loc, p, d, day)
	return true
}

func (c *Campaign) wake(loc *Location, p *FactionPresence, d *Doctrine, day history.StarDate) {
	p.Dormant = false
	p.ArrivedDay, p.LastStepDay = day, day
	p.LastShotDay = NeverFired
	c.recordFaction("faction_woke", loc, p, d, nil)
}

// WakePresence wakes dormant presence id on loc when its wake day has come
// (a loaded level: Tier 1 skips it). It reports whether it woke now.
func (c *Campaign) WakePresence(loc *Location, id string) bool {
	p := loc.PresenceByID(id)
	if p == nil || !p.Dormant || c.FactionDay < p.WakeDay {
		return false
	}
	d := LoadFactionData().Doctrine(p.Doctrine)
	if d == nil {
		return false
	}
	c.wake(loc, p, d, c.FactionDay)
	return true
}

// WipeDormant drops dormant presence id on loc: its sleeper node was
// destroyed before it woke (faction_wiped, made known).
func (c *Campaign) WipeDormant(loc *Location, id string) bool {
	p := loc.PresenceByID(id)
	if p == nil || !p.Dormant {
		return false
	}
	if d := LoadFactionData().Doctrine(p.Doctrine); d != nil {
		p.Known = true
		c.recordFaction("faction_wiped", loc, p, d, nil)
	}
	loc.RemovePresence(id)
	return true
}

// LiveSignal is the signal rule for a presence on a loaded level (Tier 1
// skips it): the world manager calls it once a day with the level's living
// bodies, converted share and standing relay pylons. The record takes both
// (its stage rising); when ready (the Signal Core stands and the transmit
// rung's bodies are at it) it signals by the Tier 1 rule; it returns the
// site signalled, or nil. The caller takes the signal's bodies off the
// level.
func (c *Campaign) LiveSignal(loc *Location, id string, pop int, conversion float64, pylons int, ready bool, loaded func(string) bool) *Location {
	p := loc.PresenceByID(id)
	if p == nil || !p.Materialized || p.Dormant || pop <= 0 {
		return nil
	}
	fd := LoadFactionData()
	d := fd.Doctrine(p.Doctrine)
	if d == nil || d.Mobility.Kind != MobilitySignal {
		return nil
	}
	p.Population, p.Growth, p.Conversion = pop, 0, conversion
	c.stageUp(loc, p, d, d.CapacityAt(loc))
	if !ready {
		return nil // the day waits, the cooldown unspent
	}
	isLoaded := func(id string) bool { return loaded != nil && loaded(id) }
	rng := rand.New(rand.NewSource(c.Seed*41 + int64(c.FactionDay)*2654435761 + p.Seed))
	return c.trySignal(rng, fd, d, loc, p, c.FactionDay, isLoaded, pylons)
}

// SignalReady reports whether presence id on loc would signal today by the
// Tier 1 rule with pop bodies and pylons standing, a site in range (the
// transmit rung walks its bodies to the core then). No random draw.
func (c *Campaign) SignalReady(loc *Location, id string, pop int, pylons int, loaded func(string) bool) bool {
	p := loc.PresenceByID(id)
	if p == nil || p.Dormant || pop <= 0 {
		return false
	}
	fd := LoadFactionData()
	d := fd.Doctrine(p.Doctrine)
	if d == nil || d.Mobility.Kind != MobilitySignal {
		return false
	}
	q := *p
	q.Population = pop
	if !c.signalReady(fd, d, loc, &q, c.FactionDay) {
		return false
	}
	isLoaded := func(id string) bool { return loaded != nil && loaded(id) }
	return len(c.signalCands(d, loc, &q, isLoaded, pylons)) > 0
}

// SignalBearing is the unit bearing on the Star Map from loc to the nearest
// site presence id's signal could found a presence on (its stage's pylons
// standing), {0, 0} for none: where the autonz deploy their relay pylons.
func (c *Campaign) SignalBearing(loc *Location, id string) [2]float64 {
	p := loc.PresenceByID(id)
	if p == nil {
		return [2]float64{}
	}
	d := LoadFactionData().Doctrine(p.Doctrine)
	if d == nil || d.Mobility.Kind != MobilitySignal {
		return [2]float64{}
	}
	var best *Location
	bestD := math.Inf(1)
	for _, t := range c.signalCands(d, loc, p, func(string) bool { return false }, -1) {
		if dd := math.Hypot(t.X-loc.X, t.Y-loc.Y); dd < bestD && dd > 0 {
			best, bestD = t, dd
		}
	}
	if best == nil {
		return [2]float64{}
	}
	return [2]float64{(best.X - loc.X) / bestD, (best.Y - loc.Y) / bestD}
}

// KnownDormant are loc's dormant presences the player has seen.
func (l *Location) KnownDormant() []*FactionPresence {
	var out []*FactionPresence
	for _, p := range l.KnownPresences() {
		if p.Dormant {
			out = append(out, p)
		}
	}
	return out
}

// SignalNote is the Star Map's line for a dormant presence ("AUTONZ SIGNAL ·
// WAKES IN 4 DAYS").
func (c *Campaign) SignalNote(d *Doctrine, loc *Location, p *FactionPresence) string {
	days := int(p.WakeDay - c.StarDate)
	when := "WAKING"
	if days > 0 {
		when = fmt.Sprintf("WAKES IN %d DAY", days)
		if days != 1 {
			when += "S"
		}
	}
	return fmt.Sprintf("%s SIGNAL · %s", d.MorphAt(loc).Tag, when)
}
