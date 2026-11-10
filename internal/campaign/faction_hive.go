package campaign

import (
	"math"
	"sort"
)

// The hive's Tier 1 bookkeeping (docs/brainstorm/hive_plan.md, "Tier 1: the
// abstract hive agrees with the live one"). Phase A's growth stays
// authoritative: these tallies never block it, they are what a frozen hive
// lands with (its larvae, the biomass in its pools, its resin) and what the
// soak checks the live hive against. No random draws: the step's stream is
// untouched.

// DoctrineBiomass is a biomass economy's abstract daily tally: every drone
// forages PerDrone (times 1 + Tag per site tag of the economy), every body
// eats Eat, every new body spends its type's cost; the stock is held between
// 0 and Cap + PerPool for each pool the director would have (its pools
// rung's want).
type DoctrineBiomass struct {
	PerDrone float64 `json:"per_drone"`
	Tag      float64 `json:"tag"`
	Eat      float64 `json:"eat"`
	Cap      int     `json:"cap"`
	PerPool  int     `json:"per_pool"`
}

// hiveTally advances a typed settlement presence's abstract larvae, biomass
// and resin one day; born is the bodies it grew today (each spent a larva and
// its egg's cost). A no-op for any other doctrine.
func (c *Campaign) hiveTally(d *Doctrine, loc *Location, p *FactionPresence, born map[string]int) {
	if !d.Broods() || d.Settlement == nil {
		return
	}
	// Larvae: the timer's day of them, held at the stage's cap; a new body
	// spends one (never below none: the tally does not block growth).
	if lv := d.Settlement.Larvae; lv != nil && lv.EveryTicks > 0 {
		n := 0
		for _, v := range born {
			n += v
		}
		p.Larvae += 750/lv.EveryTicks - n
		p.Larvae = min(max(p.Larvae, 0), d.LarvaCap(p.Stage))
	}
	// Biomass: forage less eating less the eggs laid, within the pools.
	if b := d.Biomass; b != nil {
		tags := 1.0
		for _, t := range loc.CapacityTags() {
			if t == d.Economy {
				tags += b.Tag
			}
		}
		drones := float64(p.Units[d.FallbackType(d.DefaultMorph)])
		stock := float64(p.Stockpile[d.Economy]) + drones*b.PerDrone*tags - b.Eat*float64(p.Population)
		keys := make([]string, 0, len(born))
		for k := range born {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if t := d.Types[k]; t != nil {
				stock -= float64(born[k] * t.Cost)
			}
		}
		v := int(math.Round(math.Min(math.Max(stock, 0), float64(d.BiomassCap(p.Population)))))
		if p.Stockpile == nil {
			p.Stockpile = map[string]int{}
		}
		if v > 0 {
			p.Stockpile[d.Economy] = v
		} else {
			delete(p.Stockpile, d.Economy)
			if len(p.Stockpile) == 0 {
				p.Stockpile = nil
			}
		}
	}
	// Resin: the convert economy's daily share, capped (floor 1: it carries
	// no capacity).
	c.convertWork(d, loc, p)
}

// LarvaCap is the most larvae a presence of d at stage holds: the timer's
// cap plus per_cluster for each egg cluster the stage stands.
func (d *Doctrine) LarvaCap(stage int) int {
	lv := d.Settlement.Larvae
	if lv == nil {
		return 0
	}
	clusters := 0
	if stage >= 0 && stage < len(d.Stages) {
		for _, st := range lv.Stations[min(1, len(lv.Stations)):] {
			clusters += d.Stages[stage].Structures[st]
		}
	}
	return lv.Cap + lv.PerCluster*clusters
}

// BiomassCap is the most biomass a presence of pop holds: Cap plus PerPool
// for each pool its director wants (the pools rung's min + per body).
func (d *Doctrine) BiomassCap(pop int) int {
	b := d.Biomass
	if b == nil {
		return 0
	}
	pools := 1
	if d.Settlement != nil && d.Settlement.Director != nil {
		for _, pr := range d.Settlement.Director.Priorities {
			if pr.Build != "" && d.Settlement.Camp != nil && pr.Build == d.Settlement.Camp.Storage {
				pools = max(pr.Want(pop), 1)
			}
		}
	}
	return b.Cap + b.PerPool*pools
}
