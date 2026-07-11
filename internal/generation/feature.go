package generation

import (
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// FeatureSpec is the JSON shape for a feature placement directive.
type FeatureSpec struct {
	Kind string `json:"kind"`
	// Count is the instance count. When CountMax > Count it is the minimum and
	// the count is rolled in [Count, CountMax] (see rollCount), adding
	// per-location variety on top of any area scaling.
	Count    int            `json:"count"`
	CountMax int            `json:"count_max,omitempty"`
	Biome    string         `json:"biome,omitempty"`     // restrict to columns with this biome ("" = any)
	InRegion string         `json:"in_region,omitempty"` // pick anchors from level.Regions[name]
	Jitter   int            `json:"jitter,omitempty"`    // random offset around region anchor (default 6)
	MinZ     int            `json:"min_z,omitempty"`
	MaxZ     int            `json:"max_z,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

// FeaturePlacer places a single instance of a feature at a chosen anchor.
// The placer is responsible for picking suitable locations. rng is a
// seed-derived, single-threaded source shared across a build so placement is
// reproducible for a given location seed.
type FeaturePlacer func(level *world.Level, spec FeatureSpec, rng *rand.Rand) error

var featurePlacers = map[string]FeaturePlacer{}

// RegisterFeature adds a placer.
func RegisterFeature(kind string, p FeaturePlacer) {
	featurePlacers[kind] = p
}

// GetFeature returns the placer for kind, or nil.
func GetFeature(kind string) FeaturePlacer {
	return featurePlacers[kind]
}

// PlaceFeatures runs every spec in order, drawing all placement randomness from
// rng. Failures are logged but don't abort.
func PlaceFeatures(level *world.Level, specs []FeatureSpec, rng *rand.Rand) {
	for _, s := range specs {
		p := GetFeature(s.Kind)
		if p == nil {
			log.Printf("PlaceFeatures: unknown feature kind %q", s.Kind)
			continue
		}
		if err := p(level, s, rng); err != nil {
			log.Printf("PlaceFeatures %s: %v", s.Kind, err)
		}
	}
}

// rollCount resolves a feature's instance count. When CountMax exceeds the
// (defaulted) Count, the number is rolled uniformly in [Count, CountMax] from
// rng so sibling locations differ; otherwise the fixed Count is used. def is
// the placer's fallback when Count is unset. Area scaling, where it applies, is
// layered on top of this roll by the caller (see placeAnchors).
func rollCount(s FeatureSpec, rng *rand.Rand, def int) int {
	min := s.Count
	if min <= 0 {
		min = def
	}
	if s.CountMax > min {
		return min + rng.Intn(s.CountMax-min+1)
	}
	return min
}

// placeAnchors is the shared body for count-based, surface-scattered features:
// it repeatedly picks a candidate center and calls place() until `count`
// instances land or the attempt budget (8×count) runs out, then logs any
// shortfall. place() returns true when it consumed the anchor. Candidates come
// from featureSampler, which draws biome-restricted features directly from that
// biome's columns so a rare biome on a large map still fills its count.
func placeAnchors(level *world.Level, s FeatureSpec, rng *rand.Rand, defCount int, place func(cx, cy int) bool) {
	count := rollCount(s, rng, defCount)
	// Scale by the map's size roll so areal density is constant. Round to
	// nearest, but never drop an authored feature to zero.
	if m := level.FeatureAreaMultiplier(); m != 1 {
		count = int(math.Round(float64(count) * m))
		if count < 1 {
			count = 1
		}
	}
	next, ok := featureSampler(level, s, rng)
	if !ok {
		log.Printf("feature %q: placed 0/%d (%s not present on this map)", s.Kind, count, sampleSource(s))
		return
	}
	maxAttempts := count * 8
	placed := 0
	for tries := 0; placed < count && tries < maxAttempts; tries++ {
		cx, cy := next()
		// The region sampler jitters off anchors and doesn't guarantee the
		// biome; the biome and uniform samplers already do.
		if s.InRegion != "" && !columnMatchesBiome(level, cx, cy, s.Biome) {
			continue
		}
		if place(cx, cy) {
			placed++
		}
	}
	if placed < count {
		log.Printf("feature %q: placed %d/%d (not enough placeable tiles in %s)",
			s.Kind, placed, count, sampleSource(s))
	}
}

// columnMatchesBiome returns true if the (x,y) column's biome label matches
// the spec restriction (empty = any).
func columnMatchesBiome(level *world.Level, x, y int, biome string) bool {
	if biome == "" {
		return true
	}
	return level.GetBiome(x, y) == biome
}

// sampleSource describes where a feature draws candidates from, for logging.
func sampleSource(s FeatureSpec) string {
	switch {
	case s.InRegion != "":
		return fmt.Sprintf("region %q", s.InRegion)
	case s.Biome != "":
		return fmt.Sprintf("biome %q", s.Biome)
	default:
		return "the map"
	}
}

// featureSampler builds a candidate-center generator for one feature,
// precomputing the pool once so per-attempt cost is O(1). A biome-restricted
// feature (without a region) samples directly from that biome's columns —
// otherwise a biome that's a small fraction of a large map starves the uniform
// sampler even though it has ample tiles. Returns ok=false when the feature can
// never place (empty region, or the biome is absent from this map).
func featureSampler(level *world.Level, s FeatureSpec, rng *rand.Rand) (next func() (int, int), ok bool) {
	switch {
	case s.InRegion != "":
		anchors := level.Regions[s.InRegion]
		if len(anchors) == 0 {
			return nil, false
		}
		jitter := s.Jitter
		if jitter <= 0 {
			jitter = 6
		}
		return func() (int, int) {
			a := anchors[randIntn(rng, len(anchors))]
			return a[0] + randIntn(rng, jitter*2+1) - jitter,
				a[1] + randIntn(rng, jitter*2+1) - jitter
		}, true
	case s.Biome != "":
		cols := level.ColumnsWithBiome(s.Biome)
		if len(cols) == 0 {
			return nil, false
		}
		return func() (int, int) {
			c := cols[randIntn(rng, len(cols))]
			return c[0], c[1]
		}, true
	default:
		w, h := level.GetWidth(), level.GetHeight()
		return func() (int, int) {
			return randIntn(rng, w), randIntn(rng, h)
		}, true
	}
}

// randIntn wraps rng.Intn but tolerates n<=0 (returns 0). Saves repetitive
// guards in placers.
func randIntn(rng *rand.Rand, n int) int {
	if n <= 0 {
		return 0
	}
	return rng.Intn(n)
}

func featureParamInt(s FeatureSpec, key string, def int) int {
	return paramInt(s.Params, key, def)
}
func featureParamFloat(s FeatureSpec, key string, def float64) float64 {
	return paramFloat(s.Params, key, def)
}
func featureParamString(s FeatureSpec, key, def string) string {
	return paramString(s.Params, key, def)
}

// errFeature wraps a feature error with kind context.
func errFeature(kind, msg string) error {
	return fmt.Errorf("%s: %s", kind, msg)
}

// StructureRunnerFunc is the signature of the structure-script dispatcher.
// Registered by the game package at startup to avoid a circular import. seed
// makes the invoked script's randomness reproducible; the stamp placer derives
// it from the build's RNG so the whole level reproduces from one location seed.
type StructureRunnerFunc func(level *world.Level, name string, x, y, w, h int, seed int64) error

var structureRunner StructureRunnerFunc

// SetStructureRunner registers the function used by the stamp placer to invoke
// structure scripts. Must be called before world generation begins.
func SetStructureRunner(fn StructureRunnerFunc) {
	structureRunner = fn
}

func runStructure(level *world.Level, name string, x, y, w, h int, seed int64) error {
	if structureRunner == nil {
		return fmt.Errorf("stamp: structure runner not registered")
	}
	return structureRunner(level, name, x, y, w, h, seed)
}

var warnedUnknownTile = map[string]bool{}

// requireTile returns true if the tile name exists in the loaded definitions.
// Unknown names are dangerous because UpdateTileAt silently maps them to
// index 0 (which is "space"). Logs a one-time warning per name.
func requireTile(name string) bool {
	if _, ok := world.TileNameToIndex[name]; ok {
		return true
	}
	if !warnedUnknownTile[name] {
		warnedUnknownTile[name] = true
		log.Printf("generation: unknown tile %q — feature skipped (add it to tile_definitions.json)", name)
	}
	return false
}
