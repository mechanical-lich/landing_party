package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// statRow accumulates one row of the -stats table.
type statRow struct {
	events, origins, stamps, civs, span []int
	tiers                               map[string]int
}

func (r *statRow) add(events, origins, stamps, civs, span int, tier string) {
	r.events = append(r.events, events)
	r.origins = append(r.origins, origins)
	r.stamps = append(r.stamps, stamps)
	r.civs = append(r.civs, civs)
	r.span = append(r.span, span)
	if r.tiers == nil {
		r.tiers = map[string]int{}
	}
	r.tiers[tier]++
}

func mean(v []int) float64 {
	if len(v) == 0 {
		return 0
	}
	t := 0
	for _, x := range v {
		t += x
	}
	return float64(t) / float64(len(v))
}

func p90(v []int) int {
	if len(v) == 0 {
		return 0
	}
	s := append([]int(nil), v...)
	sort.Ints(s)
	return s[(len(s)*9)/10]
}

func maxOf(v []int) int {
	m := 0
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

// printStats binds one site per history map per seed (as -freq does) and
// tabulates, per map and per population at the present (sites with an older
// layer apart): events per site (happenings: a per-side pair counts once)
// (mean, p90, max), origins and kept stamps per site, civs per site, the
// chronicle's span in years and the tier shares; then pool-civ recurrence
// across charted campaigns.
func printStats(w *os.File, lib *history.Library, maps []string, seeds, campaigns int) {
	byMap := map[string]*statRow{}
	byPop := map[string]*statRow{}
	nativeTech := map[string]int{}
	eventTech := map[string]int{}
	natives := 0
	sites := 0
	scriptStamps := map[string]int{} // kept stamps by script
	scriptSites := map[string]int{}  // sites with at least one
	scriptDrops := map[string]int{}  // stamps the lore budget dropped
	// Budget drops are logged; the table counts what is kept.
	log.SetOutput(&bytes.Buffer{})
	for s := 1; s <= seeds; s++ {
		h := history.New(int64(s) * 7919)
		for i, m := range maps {
			site, err := h.BindSite(lib, m, h.Seed+int64(i+1)*131)
			if err != nil {
				log.SetOutput(os.Stderr)
				log.Fatal(err)
			}
			ev := h.Evidence(lib, site)
			stamps := 0
			sites++
			onSite := map[string]bool{}
			for _, f := range ev.Features {
				if f.Kind == "stamp" {
					stamps++
					script, _ := f.Params["script"].(string)
					scriptStamps[script]++
					onSite[script] = true
				}
			}
			for sc := range onSite {
				scriptSites[sc]++
			}
			for _, d := range ev.Dropped {
				if d.Kind == "stamp" && d.Limit != "tech" {
					scriptDrops[d.Script]++
				}
			}
			civs := map[string]bool{}
			span := 0
			for _, id := range site.Events {
				e := h.Event(id)
				if t := e.Vars["tech"]; t != "" {
					eventTech[t]++
				}
				for _, c := range e.Civs {
					civs[c] = true
				}
				if y := int(-e.Date) / 365; y > span {
					span = y
				}
			}
			for j, row := range []map[string]*statRow{byMap, byPop} {
				key := m
				if j == 1 {
					key = site.State.Population
					if site.Layer != "" {
						key += " +layer"
					}
				}
				if row[key] == nil {
					row[key] = &statRow{}
				}
				row[key].add(h.Happenings(lib, site), len(ev.Origins), stamps, len(civs), span, site.Tier)
			}
		}
		for _, c := range h.Civs {
			if !history.Has(h.Pool, c.ID) {
				natives++
				nativeTech[c.Tech]++
			}
		}
	}
	log.SetOutput(os.Stderr)
	header := func(title string) {
		fmt.Fprintf(w, "== %s, %d seeds ==\n", title, seeds)
		fmt.Fprintf(w, "%-18s %6s %5s %4s %7s %6s %5s %6s  %s\n", "", "events", "p90", "max", "origins", "stamps", "civs", "years", "threat/hazard/quiet %")
	}
	line := func(k string, r *statRow) {
		n := len(r.events)
		fmt.Fprintf(w, "%-18s %6.1f %5d %4d %7.1f %6.1f %5.1f %6.0f  %.0f/%.0f/%.0f\n", k, mean(r.events), p90(r.events), maxOf(r.events),
			mean(r.origins), mean(r.stamps), mean(r.civs), mean(r.span),
			100*float64(r.tiers[history.TierThreat])/float64(n), 100*float64(r.tiers[history.TierHazard])/float64(n), 100*float64(r.tiers[history.TierQuiet])/float64(n))
	}
	header("Per map")
	for _, m := range maps {
		line(m, byMap[m])
	}
	fmt.Fprintln(w)
	header("Per population at the present")
	for _, p := range []string{history.PopNever, history.PopOutpost, history.PopColony, history.PopCivilization} {
		for _, k := range []string{p, p + " +layer"} {
			if r := byPop[k]; r != nil {
				line(k, r)
			}
		}
	}
	fmt.Fprintf(w, "== Tech, %d seeds ==\n", seeds)
	fmt.Fprintf(w, "%-18s", "")
	for _, l := range history.TechLevels {
		fmt.Fprintf(w, " %11s", l)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-18s", fmt.Sprintf("natives (%d)", natives))
	for _, l := range history.TechLevels {
		fmt.Fprintf(w, " %10.1f%%", 100*float64(nativeTech[l])/float64(max(natives, 1)))
	}
	fmt.Fprintln(w)
	total := 0
	for _, n := range eventTech {
		total += n
	}
	fmt.Fprintf(w, "%-18s", fmt.Sprintf("events (%d)", total))
	for _, l := range history.TechLevels {
		fmt.Fprintf(w, " %10.1f%%", 100*float64(eventTech[l])/float64(max(total, 1)))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "(natives by the highest level reached; events by the acting civ's level then)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "== Stamps by script, %d seeds (%d sites) ==\n", seeds, sites)
	fmt.Fprintf(w, "%-22s %7s %7s %7s\n", "", "stamps", "sites %", "dropped")
	scripts := make([]string, 0, len(scriptStamps))
	for sc := range scriptStamps {
		scripts = append(scripts, sc)
	}
	sort.Slice(scripts, func(i, j int) bool {
		if scriptStamps[scripts[i]] != scriptStamps[scripts[j]] {
			return scriptStamps[scripts[i]] > scriptStamps[scripts[j]]
		}
		return scripts[i] < scripts[j]
	})
	for _, sc := range scripts {
		fmt.Fprintf(w, "%-22s %7d %6.2f%% %7d\n", sc, scriptStamps[sc], 100*float64(scriptSites[sc])/float64(max(sites, 1)), scriptDrops[sc])
	}
	fmt.Fprintln(w, "(kept stamps; sites with at least one; dropped: left out by the lore budget)")
	fmt.Fprintln(w)
	if campaigns > 0 {
		printRecurrence(w, campaigns)
	}
}

// printRecurrence charts campaigns as the game does and reports, among pool
// civs that appear on any charted site, the share that appear on two or more.
func printRecurrence(w *os.File, n int) {
	log.SetOutput(&bytes.Buffer{})
	seen, twice := 0, 0
	for s := 1; s <= n; s++ {
		c := chartCampaign(int64(s) * 7919)
		for _, id := range c.History.Pool {
			sites := 0
			for _, sid := range c.History.Civs[id].Sites {
				if site := c.History.Sites[sid]; site != nil && site.Location != "" {
					sites++
				}
			}
			if sites > 0 {
				seen++
			}
			if sites > 1 {
				twice++
			}
		}
	}
	log.SetOutput(os.Stderr)
	fmt.Fprintf(w, "== Pool-civ recurrence, %d charted campaigns ==\n", n)
	fmt.Fprintf(w, "pool civs on a charted site: %d; on two or more: %d (%.0f%%)\n\n", seen, twice, 100*float64(twice)/float64(max(seen, 1)))
}
