package main

import (
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/history"
)

// printFactions steps the campaign faction tier (docs/developer/factions.md)
// for each seed at each Rival factions count and prints, every `every` days,
// how many presences and eggs there are and their total population; with
// detail, the presences themselves at the end. The opening map only: no
// expansion and no player, so it shows the pressure an untouched campaign
// builds. For balancing data/factions.json by eye.
func printFactions(w io.Writer, seeds []int64, counts []int, days, every int, cols []int, detail bool) {
	if every < 1 {
		every = 10
	}
	if len(cols) == 0 {
		for d := 0; d <= days; d += every {
			cols = append(cols, d)
		}
	}
	fd := campaign.LoadFactionData()
	fmt.Fprintf(w, "== Rival factions over %d days (opening map, no player) ==\n", days)
	for _, n := range counts {
		fmt.Fprintf(w, "\n-- factions %d --\n", n)
		fmt.Fprintf(w, "%-8s", "seed")
		for _, d := range cols {
			fmt.Fprintf(w, " %11s", fmt.Sprintf("d%d", d))
		}
		fmt.Fprintf(w, "  (presences/eggs:population; days from arrival to first shot)\n")
		for _, seed := range seeds {
			c, err := campaign.GenerateCampaignOpts("Dump", seed, campaign.GenOptions{Factions: n})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(w, "%-8d", seed)
			var conv []string // the autonz row (phase E), when there are any
			anyAutonz := false
			for _, d := range cols {
				c.StarDate = history.StarDate(d)
				c.StepFactions(nil)
				pres, pop := 0, 0
				awake, dormant, share := 0, 0, 0.0
				for _, loc := range c.Locations {
					for _, p := range loc.Factions {
						pres++
						pop += p.Population
						if p.Doctrine == "autonz" {
							anyAutonz = true
							if p.Dormant {
								dormant++
							} else {
								awake++
								share += p.Conversion
							}
						}
					}
				}
				fmt.Fprintf(w, " %11s", fmt.Sprintf("%d/%d:%d", pres, len(c.Fleets), pop))
				avg := 0.0
				if awake > 0 {
					avg = share / float64(awake)
				}
				conv = append(conv, fmt.Sprintf("%d+%d:%.0f%%", awake, dormant, avg*100))
			}
			fmt.Fprintf(w, "  %s\n", firstShots(c))
			if anyAutonz {
				fmt.Fprintf(w, "%-8s", "  autonz")
				for _, v := range conv {
					fmt.Fprintf(w, " %11s", v)
				}
				fmt.Fprintf(w, "  (awake+dormant:mean conversion of the awake)\n")
			}
			if detail {
				printPresences(w, c, fd)
			}
		}
	}
}

func printPresences(w io.Writer, c *campaign.Campaign, fd *campaign.FactionData) {
	var ids []string
	for id, loc := range c.Locations {
		if len(loc.Factions) > 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		loc := c.Locations[id]
		for _, p := range loc.Factions {
			d := fd.Doctrine(p.Doctrine)
			stage, capn := "?", 0
			if d != nil {
				stage, capn = d.StageName(p.Stage), d.CapacityAt(loc)
			}
			seen := "hidden"
			if loc.Discovered {
				// Its row on the opening Destinations list (nearest first).
				seen = fmt.Sprintf("row %d", destRow(c, id))
			}
			fmt.Fprintf(w, "    %-8s %-18s %-10s %-20s %-7s pop %3d/%-3d %-6s day %3d techs %s\n",
				id, loc.Name, loc.Kind, loc.ScenarioID, seen, p.Population, capn, stage, p.ArrivedDay, strings.Join(p.KnownTechs, ","))
			if d != nil && d.Broods() {
				// The hive's Tier 1 bookkeeping (phase E2).
				var units []string
				for _, k := range d.TypeKeys() {
					if n := p.Units[k]; n > 0 {
						units = append(units, fmt.Sprintf("%s %d", k, n))
					}
				}
				fmt.Fprintf(w, "             units %s; resin %.2f; %s %d; larvae %d\n",
					strings.Join(units, ", "), p.Conversion, d.Economy, p.Stockpile[d.Economy], p.Larvae)
			} else if d != nil && d.Builds() {
				// The autonz' Tier 1 bookkeeping (phase E3): units by type
				// (bronze where the type's tier is known), structures, the
				// assimilation walk.
				var units, built, assim []string
				for _, k := range d.TypeKeys() {
					if n := p.Units[k]; n > 0 {
						tier := ""
						if t := d.TierOfType(k, p.KnownTechs); t != "" && d.Tiers[0].ID != t {
							tier = " " + t
						}
						units = append(units, fmt.Sprintf("%s %d%s", k, n, tier))
					}
				}
				keys := make([]string, 0, len(p.Built))
				for k := range p.Built {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					built = append(built, fmt.Sprintf("%s %d", k, p.Built[k]))
				}
				for _, a := range d.Assimilation {
					for _, k := range p.KnownTechs {
						if k == a.ID {
							assim = append(assim, strings.TrimPrefix(k, "autonz_"))
						}
					}
				}
				fmt.Fprintf(w, "             units %s; built %s; conversion %.2f; assimilated %s (%.0f pts)\n",
					strings.Join(units, ", "), strings.Join(built, ", "), p.Conversion, strings.Join(assim, ","), p.AssimPoints)
			}
		}
	}
	for i := range c.Fleets {
		e := &c.Fleets[i]
		if e.Kind != campaign.FleetShip {
			fmt.Fprintf(w, "    egg %s %s -> %s, lands day %d\n", e.ID, e.FromID, e.ToID, e.ArriveDay)
		}
	}
	// Ships as ships (greyish_plan.md, step 0): per faction, its mothership
	// and hoard, then each ship's tier, state, crew, hold and techs.
	seenFaction := map[string]bool{}
	for i := range c.Fleets {
		e := &c.Fleets[i]
		if e.Kind != campaign.FleetShip || seenFaction[e.Presence.Faction] {
			continue
		}
		seenFaction[e.Presence.Faction] = true
		d := fd.Doctrine(e.Presence.Doctrine)
		if d == nil {
			continue
		}
		for _, f := range c.ShipFleets(e.Presence.Faction) {
			where := f.LandedID
			switch {
			case f.Mothership:
				where = "over " + f.ToID
			case where == "":
				where = f.FromID + " -> " + f.ToID
			}
			fmt.Fprintf(w, "    ship %-5s %-10s %-22s %s; techs %s\n", f.ID, e.Presence.Doctrine, where,
				c.ShipLine(d, f), strings.Join(f.Presence.KnownTechs, ","))
			if d.ShipSettles() && !f.Mothership {
				// The greys' Tier 1 bookkeeping (phase E4): units by type,
				// the ground structures a landed ship would stand by now
				// (ShipBuilt), the gear floor, the second walk's techs.
				var units, walk []string
				for _, k := range d.TypeKeys() {
					if n := f.Presence.Units[k]; n > 0 {
						units = append(units, fmt.Sprintf("%s %d", strings.TrimPrefix(k, "grey_"), n))
					}
				}
				for _, a := range d.Assimilation {
					for _, k := range f.Presence.KnownTechs {
						if k == a.ID {
							walk = append(walk, strings.TrimPrefix(k, "grey_"))
						}
					}
				}
				built := map[string]int{}
				if f.Landed() {
					built = d.ShipBuilt(f.ShipTier, f.Presence.KnownTechs, int(c.FactionDay-f.LandedDay))
				}
				fmt.Fprintf(w, "             units %s; built %s; gear %s; second walk %s (%.1f pts)\n",
					strings.Join(units, ", "), countLine(built), countLine(d.GearFloorFor(f.Presence.Units, f.Presence.KnownTechs)),
					strings.Join(walk, ","), f.Presence.AssimPoints)
			}
		}
	}
	// Live history the step wrote, for reading the Legends lines.
	n := 0
	for _, ev := range c.History.Events {
		if strings.HasPrefix(ev.Type, "faction_") {
			n++
		}
	}
	fmt.Fprintf(w, "    %d faction events in the live history\n", n)
}

// parseInts reads "1,2,5-8" into ints.
func parseInts(s string) []int64 {
	var out []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if a, b, ok := strings.Cut(part, "-"); ok {
			lo, err1 := strconv.ParseInt(a, 10, 64)
			hi, err2 := strconv.ParseInt(b, 10, 64)
			if err1 != nil || err2 != nil {
				log.Fatalf("bad range %q", part)
			}
			for v := lo; v <= hi; v++ {
				out = append(out, v)
			}
			continue
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			log.Fatalf("bad number %q", part)
		}
		out = append(out, v)
	}
	return out
}

// destRow is a location's index on the Star Map Destinations list (fuel from
// the current location, then ID), for vmtest scripts that click its row.
func destRow(c *campaign.Campaign, id string) int {
	locs := c.DiscoveredLocations()
	sort.Slice(locs, func(a, b int) bool {
		fa, fb := c.FuelCost(locs[a].ID), c.FuelCost(locs[b].ID)
		if fa != fb {
			return fa < fb
		}
		return locs[a].ID < locs[b].ID
	})
	for i, l := range locs {
		if l.ID == id {
			return i
		}
	}
	return -1
}

// firstShots summarises, over every site that got a presence, the days from
// its first arrival to its first launch: "avg (min-max) n fired / m sites".
func firstShots(c *campaign.Campaign) string {
	arrived := map[string]int{}
	shot := map[string]int{}
	for _, ev := range c.History.Events {
		place := ev.Vars["place"]
		switch ev.Type {
		case "faction_arrived":
			if _, ok := arrived[place]; !ok {
				arrived[place] = int(ev.Date)
			}
		case "faction_launched":
			if _, ok := shot[place]; !ok {
				shot[place] = int(ev.Date)
			}
		}
	}
	n, sum, lo, hi := 0, 0, 1<<30, 0
	for place, a := range arrived {
		if s, ok := shot[place]; ok {
			d := s - a
			n++
			sum += d
			lo, hi = min(lo, d), max(hi, d)
		}
	}
	if n == 0 {
		return fmt.Sprintf("none fired / %d sites", len(arrived))
	}
	return fmt.Sprintf("%d (%d-%d) %d fired / %d sites", sum/n, lo, hi, n, len(arrived))
}

// countLine is a count map as "k n, k n" in key order ("-" for none).
func countLine(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v > 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "-"
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s %d", k, m[k])
	}
	return strings.Join(parts, ", ")
}
