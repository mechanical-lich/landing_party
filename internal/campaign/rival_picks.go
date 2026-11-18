package campaign

import (
	"math/rand"
)

// Choosing each rival (rival_selection_plan.md, step 3). New Expedition's
// rival boxes: one pick per rival, a doctrine or Random, each with a group.
// A run started with picks seeds them through seedRivals; nil picks keep
// the count path (seedFactions) exactly as before, so every test, soak,
// vmtest and history-dump that passes a number is unchanged.

// RivalPick is one rival box: Doctrine "" is Random (rolled from the
// campaign seed); Group 0 is a group of its own.
type RivalPick struct {
	Doctrine string `json:"doctrine,omitempty"`
	Group    int    `json:"group,omitempty"`
}

// RivalChoices is what a rival box offers after Random: every seedable
// doctrine, in seeding order (the colony's rival view as "colony").
func RivalChoices() []string { return LoadFactionData().seedable() }

// seedWeight is a doctrine's weight in the Random roll (seed_weight, default 1).
func (d *Doctrine) seedWeight() float64 {
	if d.SeedWeight > 0 {
		return d.SeedWeight
	}
	return 1
}

// rollRivals resolves the Random picks: each draws a seedable doctrine by
// seed_weight, from the campaign seed, without repeating one until every
// doctrine has appeared (the player's own included). Picked doctrines count
// as appeared.
func (c *Campaign) rollRivals(picks []RivalPick) []string {
	fd := LoadFactionData()
	all := fd.seedable()
	out := make([]string, len(picks))
	used := map[string]bool{}
	for i, p := range picks {
		if p.Doctrine != "" {
			out[i] = fd.Resolve(p.Doctrine)
			used[out[i]] = true
		}
	}
	rng := rand.New(rand.NewSource(c.Seed*31337 + 271828))
	for i, p := range picks {
		if p.Doctrine != "" {
			continue
		}
		var bag []string
		for _, k := range all {
			if !used[k] {
				bag = append(bag, k)
			}
		}
		if len(bag) == 0 { // every doctrine has appeared: start again
			used = map[string]bool{}
			bag = append(bag, all...)
		}
		total := 0.0
		for _, k := range bag {
			total += fd.Doctrine(k).seedWeight()
		}
		r := rng.Float64() * total
		pick := bag[len(bag)-1]
		for _, k := range bag {
			if r -= fd.Doctrine(k).seedWeight(); r < 0 {
				pick = k
				break
			}
		}
		out[i] = pick
		used[pick] = true
	}
	return out
}

// seedRivals seeds one faction instance per pick, in slot order: each its
// doctrine (Random rolled), its group (0: its own), on an eligible site
// weighted toward its economy, never two on one site, each from a random
// stream of its own; a fleet doctrine as a ship landed on its site, with
// its own mothership.
func (c *Campaign) seedRivals(picks []RivalPick) {
	fd := LoadFactionData()
	c.RivalPicks = append([]RivalPick(nil), picks...)
	c.FactionCount = len(picks)
	c.FactionDay = c.StarDate
	taken := map[string]bool{}
	for i, dk := range c.rollRivals(picks) {
		d := fd.Doctrine(dk)
		if d == nil {
			continue
		}
		f := c.AddFaction(dk, i+1, picks[i].Group)
		rng := rand.New(rand.NewSource(c.Seed*7919 + 104729 + int64(i+1)*15485863))
		loc := c.arrivalSite(rng, d, taken)
		if loc == nil {
			return
		}
		c.seedArrival(f.Key, dk, d, loc, taken)
	}
}

// RivalGroups is the rivals' groups by slot, for the title screen's
// defaults: rival n is in group n+1, each its own.
func RivalGroups(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = PlayerGroup + 1 + i
	}
	return out
}
