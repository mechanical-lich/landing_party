// history-dump prints a campaign's generated prehistory, headless: the civ
// pool, N site histories rendered from the event text templates (by site, then
// by civ), and a table of how often each scenario ends a history on each map
// across many seeds. It is the place to judge whether the stories read as
// connected and whether civs recur enough (docs/brainstorm/lore_plan.md, step 1).
//
// Run from the project root (so data/ is accessible):
//
//	go run ./cmd/history-dump -seed 42 -sites 8
//	go run ./cmd/history-dump -seed 42 -map moon -sites 3 -freq 0
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

func main() {
	seed := flag.Int64("seed", 1, "campaign seed")
	mapID := flag.String("map", "", "chart every site on this map (default: cycle through the history maps)")
	sites := flag.Int("sites", 8, "number of sites to chart")
	freq := flag.Int("freq", 500, "seeds for the scenario-frequency table (0 skips it)")
	charted := flag.Bool("charted", false, "chart a real campaign (GenerateCampaign, expansions, travel rolls) and print its sites' histories under their real names")
	campaigns := flag.Int("campaigns", 0, "campaigns to chart for a scenario-frequency table measured through real charting (newLocation)")
	find := flag.String("find", "", "list New Expedition seeds (1..-findmax) whose opening Star Map has a site matching every comma-separated term: a tier (threat, hazard, quiet), a map, a location ID, or an event type on the site; for vmtest scripts")
	findMax := flag.Int("findmax", 200, "highest seed -find scans")
	stats := flag.Int("stats", 0, "seeds for the density table (events, origins, stamps, civs, span, tiers per map and per population) and pool-civ recurrence over a tenth as many charted campaigns; 0 skips it")
	flag.Parse()

	if err := mapdef.Load("data/maps"); err != nil {
		log.Fatal(err)
	}
	lib, err := history.LoadLibrary(history.DataDir, mapdef.All())
	if err != nil {
		log.Fatal(err)
	}
	maps := historyMaps(lib)
	if *mapID != "" {
		if md := lib.Maps[*mapID]; md == nil || md.Present == nil {
			log.Fatalf("map %q has no history (maps with one: %s)", *mapID, strings.Join(maps, ", "))
		}
	}

	if *find != "" {
		printFind(os.Stdout, lib, *find, *findMax)
		return
	}
	if *stats > 0 {
		printStats(os.Stdout, lib, maps, *stats, max(*stats/10, 1))
		return
	}

	if *charted || *campaigns > 0 {
		w := os.Stdout
		if *charted {
			c := chartCampaign(*seed)
			names := map[string]string{}
			for _, loc := range c.Locations {
				if loc.History != "" {
					names[loc.History] = loc.Name
				}
			}
			printPool(w, lib, c.History, names)
			printBySite(w, lib, c.History, names)
			printByCiv(w, lib, c.History, names)
		}
		if *campaigns > 0 {
			printChartedFrequency(w, *campaigns)
		}
		return
	}

	h := history.New(*seed)
	names := map[string]string{}
	taken := map[string]bool{}
	for i := 0; i < *sites; i++ {
		m := *mapID
		if m == "" {
			m = maps[i%len(maps)]
		}
		// Site seeds as charting derives them: campaign seed + GenSeq*131.
		siteSeed := *seed + int64(i+1)*131
		site, err := h.BindSite(lib, m, siteSeed)
		if err != nil {
			log.Fatal(err)
		}
		names[site.ID] = uniqueName(rand.New(rand.NewSource(siteSeed)), taken)
	}

	w := os.Stdout
	printPool(w, lib, h, names)
	printBySite(w, lib, h, names)
	printByCiv(w, lib, h, names)
	if *freq > 0 {
		printFrequency(w, lib, maps, *freq)
	}
}

// uniqueName draws a planet name from the campaign's location pool, without
// replacement: a chronicle never names two sites the same. (Dump-only; the
// campaign names its own locations.)
func uniqueName(rng *rand.Rand, taken map[string]bool) string {
	for try := 0; try < 200; try++ {
		n := lore.RandomNameSeeded(rng, "location")
		if !taken[n] {
			taken[n] = true
			return n
		}
	}
	n := fmt.Sprintf("%s %d", lore.RandomNameSeeded(rng, "location"), len(taken)+1)
	taken[n] = true
	return n
}

func historyMaps(lib *history.Library) []string {
	var out []string
	for id, m := range lib.Maps {
		if m.Present != nil {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func sortedSites(h *history.History) []*history.SiteHistory {
	var out []*history.SiteHistory
	for _, s := range h.Sites {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return siteNum(out[i].ID) < siteNum(out[j].ID) })
	return out
}

func siteNum(id string) int {
	var n int
	fmt.Sscanf(id, "site%d", &n)
	return n
}

func sortedCivs(h *history.History) []*history.Civ {
	var out []*history.Civ
	for _, c := range h.Civs {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		var a, b int
		fmt.Sscanf(out[i].ID, "civ%d", &a)
		fmt.Sscanf(out[j].ID, "civ%d", &b)
		return a < b
	})
	return out
}

func printPool(w *os.File, lib *history.Library, h *history.History, names map[string]string) {
	fmt.Fprintf(w, "== Campaign %d: %d civs in the pool ==\n", h.Seed, len(h.Pool))
	for _, c := range sortedCivs(h) {
		kind := "pool"
		if !history.Has(h.Pool, c.ID) {
			kind = "native"
		}
		extra := ""
		if c.Species != "" {
			extra += ", today: " + c.Species
		}
		if c.Ancestor != "" {
			extra += ", descended from the " + h.Civs[c.Ancestor].Name
		}
		fmt.Fprintf(w, "  %-12s %-6s %-11s %-8s on %d site(s)%s\n", c.Name, kind, c.Tech, c.Fate, len(c.Sites), extra)
	}
	var offWorld []history.Event
	for _, e := range h.Events {
		if e.Site == "" {
			offWorld = append(offWorld, e)
		}
	}
	sort.SliceStable(offWorld, func(i, j int) bool { return offWorld[i].Date < offWorld[j].Date })
	for i := range offWorld {
		fmt.Fprintf(w, "  * %s\n", h.Render(lib, &offWorld[i], ""))
	}
	fmt.Fprintln(w)
}

func printBySite(w *os.File, lib *history.Library, h *history.History, names map[string]string) {
	fmt.Fprintln(w, "== By site ==")
	for _, s := range sortedSites(h) {
		chapter := "quiet"
		if s.Chapter != "" {
			chapter = "chapter " + h.Event(s.Chapter).Type
		}
		fmt.Fprintf(w, "-- %s (%s, arc %s) plays %s [%s]\n", names[s.ID], s.Map, s.Arc, s.Scenario, chapter)
		fmt.Fprintf(w, "   present: life %s, population %s, status %s", s.State.Life, s.State.Population, s.State.Status)
		if len(s.State.Remains) > 0 {
			fmt.Fprintf(w, ", remains %s", strings.Join(s.State.Remains, "/"))
		}
		if len(s.State.Contaminants) > 0 {
			fmt.Fprintf(w, ", contaminants %s", strings.Join(s.State.Contaminants, "/"))
		}
		fmt.Fprintln(w)
		for _, id := range s.Events {
			e := h.Event(id)
			at := ""
			if e.Origin != nil {
				at = fmt.Sprintf("  @%d,%d", e.Origin.X, e.Origin.Y)
			}
			fmt.Fprintf(w, "   %s%s\n", h.Render(lib, e, names[s.ID]), at)
		}
		fmt.Fprintln(w)
	}
}

func printByCiv(w *os.File, lib *history.Library, h *history.History, names map[string]string) {
	fmt.Fprintln(w, "== By civ ==")
	for _, c := range sortedCivs(h) {
		var evs []history.Event
		for _, e := range h.Events {
			if history.Has(e.Civs, c.ID) {
				evs = append(evs, e)
			}
		}
		if len(evs) == 0 {
			fmt.Fprintf(w, "-- the %s (%s): not seen on any charted site\n", c.Name, c.Fate)
			continue
		}
		sort.SliceStable(evs, func(i, j int) bool { return evs[i].Date < evs[j].Date })
		fmt.Fprintf(w, "-- the %s (%s)\n", c.Name, c.Fate)
		for i := range evs {
			fmt.Fprintf(w, "   %s\n", h.Render(lib, &evs[i], names[evs[i].Site]))
		}
	}
	fmt.Fprintln(w)
}

// printFrequency charts one site per history map per seed and tabulates the
// share of each scenario on each map.
func printFrequency(w *os.File, lib *history.Library, maps []string, seeds int) {
	counts := map[string]map[string]int{}
	tiers := map[string]map[string]int{}
	scenarios := map[string]bool{}
	for _, m := range maps {
		counts[m] = map[string]int{}
		tiers[m] = map[string]int{}
	}
	for s := 1; s <= seeds; s++ {
		h := history.New(int64(s) * 7919)
		for i, m := range maps {
			site, err := h.BindSite(lib, m, h.Seed+int64(i+1)*131)
			if err != nil {
				log.Fatal(err)
			}
			counts[m][site.Scenario]++
			tiers[m][site.Tier]++
			scenarios[site.Scenario] = true
		}
	}
	printTiers(w, lib, maps, tiers, seeds)
	var cols []string
	for sc := range scenarios {
		cols = append(cols, sc)
	}
	sort.Strings(cols)
	fmt.Fprintf(w, "== Scenario frequency per map, %d seeds (%% of sites) ==\n", seeds)
	fmt.Fprintf(w, "%-18s", "map")
	for _, c := range cols {
		fmt.Fprintf(w, " %8s", abbrev(c))
	}
	fmt.Fprintln(w)
	for _, m := range maps {
		fmt.Fprintf(w, "%-18s", m)
		for _, c := range cols {
			if n := counts[m][c]; n > 0 {
				fmt.Fprintf(w, " %7.1f%%", 100*float64(n)/float64(seeds))
			} else {
				fmt.Fprintf(w, " %8s", "-")
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "\ncolumns:")
	for _, c := range cols {
		fmt.Fprintf(w, "  %-8s %s\n", abbrev(c), c)
	}
}

// printTiers tabulates the tier share per map against the map's tiers target.
func printTiers(w *os.File, lib *history.Library, maps []string, tiers map[string]map[string]int, seeds int) {
	fmt.Fprintf(w, "== Tier share per map, %d seeds (%% of sites; target in brackets) ==\n", seeds)
	fmt.Fprintf(w, "%-18s", "map")
	for _, t := range history.Tiers {
		fmt.Fprintf(w, " %14s", t)
	}
	fmt.Fprintln(w)
	for _, m := range maps {
		fmt.Fprintf(w, "%-18s", m)
		target, total := lib.Maps[m].Present.Tiers, 0
		for _, v := range target {
			total += v
		}
		for _, t := range history.Tiers {
			want := 0.0
			if total > 0 {
				want = 100 * float64(target[t]) / float64(total)
			}
			fmt.Fprintf(w, " %6.1f%% (%3.0f)", 100*float64(tiers[m][t])/float64(seeds), want)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
}

func abbrev(s string) string {
	if len(s) <= 8 {
		return s
	}
	parts := strings.Split(s, "_")
	if len(parts) == 1 {
		return s[:8]
	}
	out := ""
	for _, p := range parts {
		n := 8 / len(parts)
		if n > len(p) {
			n = len(p)
		}
		out += p[:n]
	}
	return out
}

// chartCampaign generates a campaign the way New Expedition does and grows it
// the way play does (expansions, travel rolls), so every site is charted
// through newLocation.
func chartCampaign(seed int64) *campaign.Campaign {
	c, err := campaign.GenerateCampaign("Dump", seed)
	if err != nil {
		log.Fatal(err)
	}
	c.Expand(4)
	for i := 0; i < 12; i++ {
		c.MaybeTravelQuests()
	}
	return c
}

// printChartedFrequency tabulates the scenario every charted site plays, per
// map, across n campaigns charted through the game's own charting.
func printChartedFrequency(w *os.File, n int) {
	counts := map[string]map[string]int{}
	totals := map[string]int{}
	scen := map[string]bool{}
	for s := 1; s <= n; s++ {
		c := chartCampaign(int64(s) * 7919)
		for _, loc := range c.Locations {
			if !loc.HasSite() || loc.Tutorial {
				continue
			}
			if counts[loc.MapID] == nil {
				counts[loc.MapID] = map[string]int{}
			}
			counts[loc.MapID][loc.ScenarioID]++
			totals[loc.MapID]++
			scen[loc.ScenarioID] = true
		}
	}
	var maps, cols []string
	for m := range counts {
		maps = append(maps, m)
	}
	for sc := range scen {
		cols = append(cols, sc)
	}
	sort.Strings(maps)
	sort.Strings(cols)
	fmt.Fprintf(w, "== Scenario frequency per map through charting, %d campaigns (%% of that map's sites) ==\n", n)
	fmt.Fprintf(w, "%-18s %6s", "map", "sites")
	for _, c := range cols {
		fmt.Fprintf(w, " %8s", abbrev(c))
	}
	fmt.Fprintln(w)
	for _, m := range maps {
		fmt.Fprintf(w, "%-18s %6d", m, totals[m])
		for _, c := range cols {
			if k := counts[m][c]; k > 0 {
				fmt.Fprintf(w, " %7.1f%%", 100*float64(k)/float64(totals[m]))
			} else {
				fmt.Fprintf(w, " %8s", "-")
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "\ncolumns:")
	for _, c := range cols {
		fmt.Fprintf(w, "  %-8s %s\n", abbrev(c), c)
	}
}

// findMatches reports whether every comma-separated term of want names the
// site's tier, map or location, or an event type in its history; a term
// "stamped:<type>" needs every event of that type to keep a structure, and
// "<var>=<value>" an event that rolled that var (purpose=warship).
func findMatches(h *history.History, site *history.SiteHistory, loc *campaign.Location, want string) bool {
	for _, term := range strings.Split(want, ",") {
		if k, v, ok := strings.Cut(term, "="); ok {
			// <var>=<value>: an event on the site rolled that var.
			hit := false
			for _, id := range site.Events {
				if e := h.Event(id); e != nil && e.Vars[k] == v {
					hit = true
				}
			}
			if !hit {
				return false
			}
			continue
		}
		if typ, ok := strings.CutPrefix(term, "stamped:"); ok {
			if !allStamped(h, site, typ) {
				return false
			}
			continue
		}
		ok := term == site.Tier || term == loc.MapID || term == loc.ID
		for _, id := range site.Events {
			if ev := h.Event(id); ev != nil && ev.Type == term {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// allStamped reports whether the site has events of type typ and each keeps
// a stamp in its evidence (not dropped by its tech level or the budget).
func allStamped(h *history.History, site *history.SiteHistory, typ string) bool {
	lib, err := history.DefaultLibrary()
	if err != nil {
		return false
	}
	log.SetOutput(io.Discard)
	ev := h.Evidence(lib, site)
	log.SetOutput(os.Stderr)
	stamped := map[string]bool{}
	for _, f := range ev.Features {
		if f.Kind == "stamp" {
			stamped[strings.TrimPrefix(f.At, "origin:")] = true
		}
	}
	n := 0
	for _, id := range site.Events {
		if e := h.Event(id); e != nil && e.Type == typ {
			if !stamped[id] {
				return false
			}
			n++
		}
	}
	return n > 0
}

// printFind lists, per seed, the sites of a fresh campaign (as New Expedition
// generates it, ship at the Wreck Site) whose chapter event type or tier is
// want, with the fuel the jump costs.
func printFind(w *os.File, lib *history.Library, want string, maxSeed int) {
	fmt.Fprintf(w, "== Seeds 1..%d with a %s site (fuel from the Wreck Site) ==\n", maxSeed, want)
	for seed := int64(1); seed <= int64(maxSeed); seed++ {
		c, err := campaign.GenerateCampaign("Find", seed)
		if err != nil {
			log.Fatal(err)
		}
		var ids []string
		for id := range c.Locations {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			loc := c.Locations[id]
			site := c.SiteHistory(loc)
			if site == nil || !loc.Discovered {
				continue
			}
			chapter := ""
			if ev := c.History.Event(site.Chapter); ev != nil {
				chapter = ev.Type
			}
			if !findMatches(c.History, site, loc, want) {
				continue
			}
			fmt.Fprintf(w, "seed %-5d %-8s %-18s %-7s %-20s %-18s fuel %d\n", seed, id, loc.MapID, site.Tier, chapter, loc.ScenarioID, c.FuelCost(id))
		}
	}
}
