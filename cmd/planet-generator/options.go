package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// paramDef describes one terrain param a primer reads, with the same default the
// primer falls back to. Params in a map's JSON without an entry here are still
// passed through; they just have no slider.
type paramDef struct {
	key           string
	name          string
	def, min, max float64
	step          float64
}

// primerParams mirrors the param lists in internal/generation/primer_*.go.
var primerParams = map[string][]paramDef{
	"planet": {
		{"mountain_frequency", "Mountain frequency", 0.15, 0, 1, 0.01},
		{"mountain_scale", "Mountain scale", 110, 10, 300, 5},
		{"cavern_density", "Cavern threshold (higher = fewer)", 0.25, -0.5, 0.8, 0.01},
		{"cavern_scale_xy", "Cavern width", 40, 5, 150, 1},
		{"cavern_scale_z", "Cavern height", 10, 1, 40, 1},
		{"terrain_alpha", "Noise alpha", 6, 1, 12, 0.1},
		{"terrain_beta", "Noise beta", 5, 1, 12, 0.1},
		{"terrain_n", "Noise octaves", 2, 1, 8, 1},
	},
	"asteroid_field": {
		{"threshold", "Rock threshold", 0.15, -0.3, 0.8, 0.01},
		{"min_size", "Min asteroid size", 40, 0, 400, 5},
		{"noise_scale", "Noise scale", 22, 4, 100, 1},
	},
	"abandoned_station": {
		{"floors", "Floors", 3, 1, 12, 1},
		{"station_size", "Station size", 50, 10, 300, 5},
		{"hub_radius", "Hub radius", 5, 2, 30, 1},
		{"spokes", "Spokes", 4, 0, 8, 1},
		{"bud_attempts", "Room attempts", 60, 0, 400, 5},
		{"min_room", "Min room", 4, 2, 20, 1},
		{"max_room", "Max room", 8, 3, 40, 1},
	},
}

var biomeMapTypes = []string{"none", "perlin_temp_humidity", "uniform"}

// lightModes lists the lighting picker, with the scenario's own setting first.
var lightModes = []string{"scenario default", world.LightModedayNight, "fixed", "pitch_dark"}

// featureRow is one editable entry in the Features list, pointing into the map's
// or scenario's feature slice.
type featureRow struct {
	block   *mapdef.FeatureBlock
	source  string
	enabled bool
	// touched means the count slider has been moved, fixing the count (no range).
	touched   bool
	count     int
	sliderMax int
}

func (f *featureRow) label() string {
	name := f.block.Kind
	for _, key := range []string{"blueprint", "tile", "structure", "script"} {
		if v, ok := f.block.Params[key].(string); ok {
			name += " " + v
			break
		}
	}
	if f.block.Biome != "" {
		name += " @" + f.block.Biome
	}
	return f.source + ": " + name
}

func newFeatureRow(b *mapdef.FeatureBlock, source string) *featureRow {
	max := b.CountMax
	if b.Count > max {
		max = b.Count
	}
	sliderMax := max * 3
	if sliderMax < 20 {
		sliderMax = 20
	}
	return &featureRow{block: b, source: source, enabled: true, count: max, sliderMax: sliderMax}
}

// options is the editable recipe for the next generation. md and sc are private
// copies of the loaded definitions; the loaded ones are never modified.
type options struct {
	mapDefs   []mapdef.MapDef
	mapNames  []string
	scenarios []scenario.Scenario // index 0 of the picker is "none"; others shifted by 1
	scenNames []string
	primers   []string
	biomes    []string

	mapIdx, scenarioIdx int
	primerIdx           int
	biomeTypeIdx        int
	singleBiomeIdx      int
	lightIdx            int

	md mapdef.MapDef
	sc *scenario.Scenario

	seedText string
	rollSize bool
	w, h, z  int

	biomeOn       map[string]bool
	biomeFeatures bool
	setupScripts  bool
	spawns        bool
	ambient       int
	features      []*featureRow

	// lib is the history library (nil if it failed to load); fromHistory picks
	// the scenario from the site's history; lore is the bound history.
	lib         *history.Library
	fromHistory bool
	lore        siteLore
}

func newOptions() (*options, error) {
	o := &options{
		primers:       generation.PrimerNames(),
		biomes:        generation.BiomeIDs(),
		rollSize:      true,
		spawns:        true,
		setupScripts:  true,
		biomeFeatures: true,
		ambient:       60,
	}
	o.mapDefs = append(o.mapDefs, mapdef.All()...)
	if len(o.mapDefs) == 0 {
		return nil, fmt.Errorf("no map definitions in data/maps")
	}
	sort.Slice(o.mapDefs, func(i, j int) bool { return o.mapDefs[i].ID < o.mapDefs[j].ID })
	for _, m := range o.mapDefs {
		o.mapNames = append(o.mapNames, m.ID)
	}
	if lib, err := history.LoadLibrary(history.DataDir, mapdef.All()); err == nil {
		o.lib = lib
	}
	o.scenarios = append(o.scenarios, scenario.AllEnabled()...)
	sort.Slice(o.scenarios, func(i, j int) bool { return o.scenarios[i].ID < o.scenarios[j].ID })
	o.randomizeSeed()
	o.selectMap(0)
	return o, nil
}

func (o *options) randomizeSeed() {
	o.seedText = strconv.FormatInt(rand.New(rand.NewSource(time.Now().UnixNano())).Int63n(1_000_000_000), 10)
}

// seed parses the seed field. Empty, zero or unparsable text falls back to 1, since
// the game treats seed 0 as "pick one at random" and results should reproduce.
func (o *options) seed() int64 {
	if v, err := strconv.ParseInt(o.seedText, 10, 64); err == nil && v != 0 {
		return v
	}
	return 1
}

// selectMapByID picks a map by ID, reporting whether it exists.
func (o *options) selectMapByID(id string) bool {
	for i := range o.mapDefs {
		if o.mapDefs[i].ID == id {
			o.selectMap(i)
			return true
		}
	}
	return false
}

// selectScenarioByID picks a scenario by ID, reporting whether it exists.
func (o *options) selectScenarioByID(id string) bool {
	for i := range o.scenarios {
		if o.scenarios[i].ID == id {
			o.scenarioIdx = i + 1
			o.applyScenario()
			return true
		}
	}
	return false
}

// selectMap loads a map's recipe into the editor, discarding edits, and picks a
// scenario that supports it.
func (o *options) selectMap(i int) {
	o.mapIdx = i
	o.md = o.mapDefs[i].Clone()
	o.primerIdx = indexOf(o.primers, o.md.Terrain)
	o.biomeTypeIdx = 0
	switch {
	case o.md.BiomeMap.Type == "uniform" || o.md.BiomeMap.Single != "":
		o.biomeTypeIdx = 2
	case o.md.BiomeMap.Type != "" || len(o.md.BiomeMap.Biomes) > 0:
		o.biomeTypeIdx = 1
	}
	o.singleBiomeIdx = indexOf(o.biomes, o.md.BiomeMap.Single)
	o.biomeOn = map[string]bool{}
	for _, b := range o.md.BiomeMap.Biomes {
		o.biomeOn[b] = true
	}
	o.lightIdx = 0

	o.scenarioIdx = o.defaultScenario()
	o.applyScenario()
	o.rerollSize()
}

// defaultScenario picks the map's namesake scenario (earth_like, alien_world...).
// Incursion scenarios layered onto many maps are opt-in, so a map with no namesake
// starts on "none".
func (o *options) defaultScenario() int {
	for j := range o.scenarios {
		if strings.HasPrefix(o.scenarios[j].ID, o.md.ID) {
			return j + 1
		}
	}
	return 0
}

// applyScenario copies the picked scenario into the editor and rebuilds the
// feature list, which covers the map's features then the scenario's.
func (o *options) applyScenario() {
	o.scenNames = append(o.scenNames[:0], "none")
	for j := range o.scenarios {
		name := o.scenarios[j].ID
		if !o.scenarios[j].SupportsMap(o.md.ID) {
			name += " (other maps)"
		}
		o.scenNames = append(o.scenNames, name)
	}
	o.sc = nil
	if o.scenarioIdx > 0 {
		c := o.scenarios[o.scenarioIdx-1]
		c.Features = append([]mapdef.FeatureBlock(nil), c.Features...)
		o.sc = &c
	}
	o.features = o.features[:0]
	for i := range o.md.Features {
		o.features = append(o.features, newFeatureRow(&o.md.Features[i], "map"))
	}
	if o.sc != nil {
		for i := range o.sc.Features {
			o.features = append(o.features, newFeatureRow(&o.sc.Features[i], "scenario"))
		}
	}
}

// rerollSize sets the size the game would roll for this map and seed.
func (o *options) rerollSize() {
	o.w, o.h, o.z = o.md.Size.Roll(o.seed())
}

// param returns a primer param's current value.
func (o *options) param(d paramDef) float64 {
	switch v := o.md.TerrainParams[d.key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return d.def
}

// spec assembles the recipe for generation. Disabled features are left out and
// the option's own map and scenario copies stay intact for further editing.
func (o *options) spec() game.PlanetLabSpec {
	if o.rollSize {
		o.rerollSize()
	}
	md := o.md
	if o.primerIdx >= 0 {
		md.Terrain = o.primers[o.primerIdx]
	}
	md.BiomeMap = mapdef.BiomeMapBlock{Scale: o.md.BiomeMap.Scale, LatitudeWeight: o.md.BiomeMap.LatitudeWeight}
	switch biomeMapTypes[o.biomeTypeIdx] {
	case "perlin_temp_humidity":
		md.BiomeMap.Type = "perlin_temp_humidity"
		for _, id := range o.biomes {
			if o.biomeOn[id] {
				md.BiomeMap.Biomes = append(md.BiomeMap.Biomes, id)
			}
		}
	case "uniform":
		md.BiomeMap.Type = "uniform"
		if o.singleBiomeIdx < 0 {
			o.singleBiomeIdx = 0
		}
		if len(o.biomes) > 0 {
			md.BiomeMap.Single = o.biomes[o.singleBiomeIdx]
		}
	}
	md.SkipBiomeFeatures = !o.biomeFeatures

	md.Features = o.enabledFeatures("map")
	var sc *scenario.Scenario
	if o.sc != nil {
		c := *o.sc
		c.Features = o.enabledFeatures("scenario")
		if !o.setupScripts {
			c.SetupScripts = nil
		}
		sc = &c
	}

	spec := game.PlanetLabSpec{
		Map: md, Scenario: sc,
		Width: o.w, Height: o.h, Depth: o.z,
		Seed:           o.seed(),
		SuppressSpawns: !o.spawns,
	}
	if o.lightIdx > 0 {
		spec.LightingMode = lightModes[o.lightIdx]
		spec.LightingAmbient = o.ambient
	}
	// With the scenario from history the level is built as the game builds a
	// charted site: evidence at resolved origins, lore flags, lore datapads.
	if o.fromHistory && o.lore.site != nil && o.lore.site.Map == md.ID {
		spec.History, spec.Site = o.lore.h, o.lore.site
	}
	return spec
}

// enabledFeatures returns the enabled features from one source.
func (o *options) enabledFeatures(source string) []mapdef.FeatureBlock {
	var out []mapdef.FeatureBlock
	for _, f := range o.features {
		if f.source != source || !f.enabled {
			continue
		}
		b := *f.block
		if f.touched {
			b.Count, b.CountMax = f.count, 0
		}
		out = append(out, b)
	}
	return out
}

func indexOf(items []string, want string) int {
	for i, s := range items {
		if s == want {
			return i
		}
	}
	return -1
}
