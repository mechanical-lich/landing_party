package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/resource"
)

// buildDataLoaded is set once the game's generation data is loaded for world
// builds (the station section builds every station site it counts).
var buildDataLoaded bool

func loadBuildData() {
	if buildDataLoaded {
		return
	}
	if err := world.LoadTileDefinitionsDir("data/tiledefinitions"); err != nil {
		log.Fatalf("load tile definitions: %v", err)
	}
	if err := resource.LoadAssetsFromJSON("data/assets.json"); err != nil {
		log.Fatalf("load assets: %v", err)
	}
	if err := factory.FactoryLoadDir(config.Global().BlueprintPath); err != nil {
		log.Fatalf("load blueprints: %v", err)
	}
	if err := game.LoadPlanetLabData(); err != nil {
		log.Fatalf("load generation data: %v", err)
	}
	buildDataLoaded = true
}

// stationRow tallies the station section of -stats: where the sites' origins
// resolved once the level is built, and what the kept stamps built.
type stationRow struct {
	sites                   int
	originsByDeck           map[int]int
	origins, inModule       int
	kept, recorded, painted int
	builds                  int
	buildErrs, noSurface    int
	wrecks, wrecksBuilt     int
	reasons                 map[string]int
	purposes                map[string]int
	arcPurposes             map[string]map[string]int
	chapters                map[string]int
}

// buildSite builds a site's level as the game builds a charted site (the
// planet generator's path), returning nil on failure.
func buildSite(h *history.History, site *history.SiteHistory) *world.Level {
	md := mapdef.ByID(site.Map)
	if md == nil {
		return nil
	}
	sc := scenario.ByID(site.Scenario)
	w, hh, d := md.Size.Roll(site.Seed)
	lab, err := game.NewPlanetLab(game.PlanetLabSpec{Map: md.Clone(), Scenario: sc, Width: w, Height: hh, Depth: d,
		Seed: site.Seed, SuppressSpawns: true, History: h, Site: site})
	if err != nil {
		return nil
	}
	lvl := lab.Level()
	lab.Close()
	return lvl
}

// footprintPainted reports whether a stamp footprint holds anything but
// space through the level's depth.
func footprintPainted(lvl *world.Level, f [6]int) bool {
	for z := 0; z < lvl.GetDepth(); z++ {
		for y := f[1]; y < f[1]+f[3]; y++ {
			for x := f[0]; x < f[0]+f[2]; x++ {
				if t := lvl.GetTilePtr(x, y, z); t != nil && (!t.Floor.IsEmpty() || !world.IsSpaceTile(t) && !t.Middle.IsEmpty()) {
					return true
				}
			}
		}
	}
	return false
}

// tallyStation builds a station site and adds it to r.
func (r *stationRow) tallyStation(h *history.History, site *history.SiteHistory, ev history.SiteEvidence) {
	r.sites++
	if r.purposes == nil {
		r.purposes, r.arcPurposes, r.chapters = map[string]int{}, map[string]map[string]int{}, map[string]int{}
	}
	if ev.Station != nil {
		r.purposes[ev.Station.Purpose]++
		if r.arcPurposes[site.Arc] == nil {
			r.arcPurposes[site.Arc] = map[string]int{}
		}
		r.arcPurposes[site.Arc][ev.Station.Purpose]++
	}
	if site.Chapter != "" {
		r.chapters[h.Event(site.Chapter).Type]++
	}
	for _, d := range ev.Dropped {
		if d.Limit == history.LimitNoSurface {
			r.noSurface++
		}
	}
	for _, f := range ev.Features {
		if f.Kind == "stamp" {
			r.kept++
		}
	}
	var logs bytes.Buffer
	log.SetOutput(&logs)
	lvl := buildSite(h, site)
	log.SetOutput(&bytes.Buffer{})
	for _, line := range strings.Split(logs.String(), "\n") {
		if i := strings.Index(line, "station: "); i >= 0 {
			if r.reasons == nil {
				r.reasons = map[string]int{}
			}
			reason := line[i:]
			if j := strings.Index(reason, " ("); j > 0 {
				reason = reason[:j]
			}
			r.reasons[reason]++
		}
	}
	if lvl == nil {
		r.buildErrs++
		return
	}
	r.builds++
	if r.originsByDeck == nil {
		r.originsByDeck = map[int]int{}
	}
	for id := range ev.Origins {
		x, y, z, ok := generation.Origin(lvl, id)
		if !ok {
			continue
		}
		r.origins++
		r.originsByDeck[z]++
		if insideModule(lvl, x, y, z) {
			r.inModule++
		}
	}
	r.recorded += len(lvl.StampFootprints)
	if ev.Station != nil && ev.Station.Wreck != nil {
		r.wrecks++
		if lvl.ModuleByID(generation.WreckModule) != nil {
			r.wrecksBuilt++
		}
	}
	for _, f := range lvl.StampFootprints {
		if footprintPainted(lvl, f) {
			r.painted++
		}
	}
}

// insideModule reports whether a tile lies in one of a station's modules.
func insideModule(lvl *world.Level, x, y, z int) bool {
	return lvl.ModuleAt(x, y, z) != nil
}

func (r *stationRow) print(w *os.File, mapID string) {
	fmt.Fprintf(w, "== Station sites (%s), %d sites, %d built ==\n", mapID, r.sites, r.builds)
	decks := make([]int, 0, len(r.originsByDeck))
	for z := range r.originsByDeck {
		decks = append(decks, z)
	}
	sort.Ints(decks)
	fmt.Fprintf(w, "origins by deck:")
	for _, z := range decks {
		fmt.Fprintf(w, "  z%d %.0f%%", z, 100*float64(r.originsByDeck[z])/float64(max(r.origins, 1)))
	}
	fmt.Fprintf(w, "  (%d origins)\n", r.origins)
	fmt.Fprintf(w, "origins inside a module: %d of %d (%.0f%%)\n", r.inModule, r.origins, 100*float64(r.inModule)/float64(max(r.origins, 1)))
	fmt.Fprintf(w, "stamps: %d kept in evidence, %d recorded on the level, %d painted something; %d dropped for no_surface\n", r.kept, r.recorded, r.painted, r.noSurface)
	fmt.Fprintf(w, "docked wrecks: %d in the plans, %d built\n", r.wrecks, r.wrecksBuilt)
	for _, k := range sortedKeys(r.reasons) {
		fmt.Fprintf(w, "  %d× %s\n", r.reasons[k], k)
	}
	fmt.Fprintf(w, "purposes:")
	for _, p := range sortedKeys(r.purposes) {
		fmt.Fprintf(w, "  %s %.0f%%", p, 100*float64(r.purposes[p])/float64(max(r.sites, 1)))
	}
	fmt.Fprintln(w)
	for _, a := range sortedKeys(r.arcPurposes) {
		n := 0
		for _, c := range r.arcPurposes[a] {
			n += c
		}
		fmt.Fprintf(w, "  %s (%d):", a, n)
		for _, p := range sortedKeys(r.arcPurposes[a]) {
			fmt.Fprintf(w, "  %s %.0f%%", p, 100*float64(r.arcPurposes[a][p])/float64(max(n, 1)))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "chapters:")
	for _, c := range sortedKeys(r.chapters) {
		fmt.Fprintf(w, "  %s %.1f%%", c, 100*float64(r.chapters[c])/float64(max(r.sites, 1)))
	}
	fmt.Fprintln(w)
	if r.buildErrs > 0 {
		fmt.Fprintf(w, "(%d sites failed to build)\n", r.buildErrs)
	}
	fmt.Fprintln(w)
}

// quietLogs silences the log while f runs (builds log every placement).
func quietLogs(f func()) {
	log.SetOutput(&bytes.Buffer{})
	defer log.SetOutput(os.Stderr)
	f()
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
