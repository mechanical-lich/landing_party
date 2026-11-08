// Package mapdef holds map (terrain) definitions: the recipe for generating a
// world's terrain, biomes, and features. It is deliberately decoupled from the
// scenario package — a scenario picks which maps it supports, but the map
// definition owns generation. generation must not import scenario/mapdef, so
// these types mirror generation's option shapes.
package mapdef

import (
	"fmt"
	"math/rand"
)

// MapDef is the full world-generation recipe for one map type. Loaded from
// data/maps/*.json.
type MapDef struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Size          SizeBlock      `json:"size"`           // generation dimensions; rolled per-location
	Terrain       string         `json:"terrain"`        // primer name: "planet", "asteroid_field", "station"
	TerrainParams map[string]any `json:"terrain_params"` // forwarded to the primer
	BiomeMap      BiomeMapBlock  `json:"biome_map"`      // optional
	Features      []FeatureBlock `json:"features"`       // ordered feature placements
	// SkipBiomeFeatures leaves out the features biomes declare for themselves.
	SkipBiomeFeatures bool `json:"skip_biome_features,omitempty"`
	// Internal marks a map that only hand-placed locations use (the tutorial's
	// Training Ground). Random() never returns it.
	Internal bool `json:"internal,omitempty"`

	// Present is the state a site's generated history must end in on this map
	// (see internal/history). nil means history skips the map (the tutorial).
	Present *Present `json:"present,omitempty"`
	// BaseScenario is what a site on this map plays as when its history ends
	// on the quiet outcome rather than a threat chapter.
	BaseScenario string `json:"base_scenario,omitempty"`
}

// Present is a map's present-day planet state: what its history has to explain.
type Present struct {
	// Kind is the kind of site: "planet" (the default) or "station". Events
	// with requires.site keep to their kind, and evidence blocks with "on"
	// build only on theirs.
	Kind string `json:"kind,omitempty"`
	// Life and Status list the allowed final values (any of).
	Life   []string `json:"life"`
	Status []string `json:"status"`
	// Arcs are the story shapes (data/history/arcs.json) that can lead here.
	Arcs []string `json:"arcs"`
	// Tiers weighs how a site's history ends: "threat" (a scenario chapter),
	// "hazard" (a hazard chapter on the base scenario) and "quiet" (no chapter,
	// BaseScenario). The tier is rolled first, then a chapter of that tier by
	// the chapters' per-map weights.
	Tiers map[string]int `json:"tiers"`
	// SafeTiers replaces Tiers on a first jump (see history.BindSiteSafe):
	// "hazard" and "quiet" only; threat is never rolled there.
	SafeTiers map[string]int `json:"safe_tiers"`
	// Density is the percent chance, after each arc stage, that one
	// interlude (a small happening) is added.
	Density int `json:"density,omitempty"`
	// Layers is the chance a site first runs an older people's layer arc.
	Layers *PresentLayers `json:"layers,omitempty"`
	// StationPurposes weighs what a station site was built for (the
	// station_purposes of data/history/purposes.json), rolled by its founding
	// event's "@station_purpose" var.
	StationPurposes map[string]int `json:"station_purposes,omitempty"`
	// LoreBudget overrides the matching fields of the map's size class in
	// the lore budget (a station: two stamps, a bigger footprint share, no
	// underground). nil fields keep the class's.
	LoreBudget *LoreBudgetOverride `json:"lore_budget,omitempty"`
}

// LoreBudgetOverride is a map's own lore budget limits.
type LoreBudgetOverride struct {
	Stamps           *int     `json:"stamps,omitempty"`
	FootprintPercent *float64 `json:"footprint_percent,omitempty"`
	Underground      *int     `json:"underground,omitempty"`
}

// Site kinds (Present.Kind).
const (
	SitePlanet  = "planet"
	SiteStation = "station"
)

// SiteKind is the present's kind of site, "planet" when unset.
func (p *Present) SiteKind() string {
	if p == nil || p.Kind == "" {
		return SitePlanet
	}
	return p.Kind
}

// PresentLayers lists the layer arcs a site of the map may run first, and how
// often (percent).
type PresentLayers struct {
	Chance int      `json:"chance"`
	Arcs   []string `json:"arcs"`
}

// Clone returns a copy whose terrain params, biome list and feature list can be
// edited without touching the original. Feature params stay shared.
func (m MapDef) Clone() MapDef {
	c := m
	c.TerrainParams = make(map[string]any, len(m.TerrainParams))
	for k, v := range m.TerrainParams {
		c.TerrainParams[k] = v
	}
	c.BiomeMap.Biomes = append([]string(nil), m.BiomeMap.Biomes...)
	c.Features = append([]FeatureBlock(nil), m.Features...)
	return c
}

// IntRange is an inclusive integer range. Authors write {"min": 200, "max": 360}
// in JSON; a fixed-size map sets min == max.
type IntRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Roll picks a value uniformly in [Min, Max]. Returns Min when Max <= Min.
func (r IntRange) Roll(rng *rand.Rand) int {
	if r.Max <= r.Min {
		return r.Min
	}
	return r.Min + rng.Intn(r.Max-r.Min+1)
}

// SizeBlock declares the per-dimension generation size for a map. Each location
// rolls once at creation time and stores the result in the level's metadata, so
// re-entering the same site reuses the same dimensions.
type SizeBlock struct {
	W IntRange `json:"w"`
	H IntRange `json:"h"`
	Z IntRange `json:"z"`
}

// Validate ensures every dimension has a sane range. Loader rejects map
// files that omit the block or set non-positive / inverted ranges, so callers
// can trust SizeBlock.Roll without fallback paths.
func (s SizeBlock) Validate() error {
	for _, d := range []struct {
		name string
		r    IntRange
	}{{"w", s.W}, {"h", s.H}, {"z", s.Z}} {
		if d.r.Min <= 0 {
			return fmt.Errorf("%s.min must be > 0 (got %d)", d.name, d.r.Min)
		}
		if d.r.Max < d.r.Min {
			return fmt.Errorf("%s.max (%d) must be >= %s.min (%d)", d.name, d.r.Max, d.name, d.r.Min)
		}
	}
	return nil
}

// MidArea returns the map's expected footprint: the product of the width and
// height range midpoints. Generation uses it as the reference area for
// density-scaling areal feature counts, so a map's resource density stays
// constant across its size roll instead of thinning out on larger rolls.
func (s SizeBlock) MidArea() int {
	midW := (s.W.Min + s.W.Max) / 2
	midH := (s.H.Min + s.H.Max) / 2
	return midW * midH
}

// Roll picks one (w, h, z) triple from this size block using a seeded rng so
// runs of the same campaign reproduce.
func (s SizeBlock) Roll(seed int64) (w, h, z int) {
	rng := rand.New(rand.NewSource(seed))
	w = s.W.Roll(rng)
	h = s.H.Roll(rng)
	z = s.Z.Roll(rng)
	return
}

// BiomeMapBlock mirrors generation.BiomeMapConfig.
type BiomeMapBlock struct {
	Type           string   `json:"type"`
	Scale          float64  `json:"scale"`
	Biomes         []string `json:"biomes"`
	Single         string   `json:"single"`
	LatitudeWeight float64  `json:"latitude_weight"`
}

// FeatureBlock mirrors generation.FeatureSpec.
type FeatureBlock struct {
	Kind string `json:"kind"`
	// Count is how many instances to place. When CountMax > Count it acts as
	// the minimum and the actual number is rolled in [Count, CountMax] per
	// generation, so sibling maps of the same type vary instead of reading as
	// area-scaled copies of one another.
	Count    int            `json:"count"`
	CountMax int            `json:"count_max,omitempty"`
	Biome    string         `json:"biome,omitempty"`
	InRegion string         `json:"in_region,omitempty"`
	Jitter   int            `json:"jitter,omitempty"`
	MinZ     int            `json:"min_z,omitempty"`
	MaxZ     int            `json:"max_z,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
	// At anchors the feature at a resolved site origin: "origin" is the site
	// chapter's (history evidence uses its own event's). See generation.Origin.
	At string `json:"at,omitempty"`
}
