package generation

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// FeatureSpec is the JSON shape for a feature placement directive.
type FeatureSpec struct {
	Kind     string         `json:"kind"`
	Count    int            `json:"count"`
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

// placeAnchors is the shared body for count-based, surface-scattered features:
// it repeatedly picks an in-biome center and calls place() until `count`
// instances land or the attempt budget (8×count) runs out, then logs any
// shortfall so authors can tell when a map or biome is too small for the
// requested count. place() returns true when it consumed the anchor.
func placeAnchors(level *world.Level, s FeatureSpec, rng *rand.Rand, defCount int, place func(cx, cy int) bool) {
	count := s.Count
	if count <= 0 {
		count = defCount
	}
	maxAttempts := count * 8
	placed := 0
	for tries := 0; placed < count && tries < maxAttempts; tries++ {
		cx, cy, ok := pickFeatureCenter(level, s, rng)
		if !ok {
			break // spec asked for a region with no anchors
		}
		if !columnMatchesBiome(level, cx, cy, s.Biome) {
			continue
		}
		if place(cx, cy) {
			placed++
		}
	}
	if placed < count {
		log.Printf("feature %q: placed %d/%d (map or biome %q too small for the requested count)",
			s.Kind, placed, count, s.Biome)
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

// pickFeatureCenter chooses an (x, y) for one feature instance, honouring
// spec.InRegion (jittered around a random region anchor) when set, else
// falling back to a uniform random column. Returns (-1, -1, false) if the
// spec asks for a region that has no anchors.
func pickFeatureCenter(level *world.Level, s FeatureSpec, rng *rand.Rand) (int, int, bool) {
	if s.InRegion != "" {
		anchors := level.Regions[s.InRegion]
		if len(anchors) == 0 {
			return -1, -1, false
		}
		jitter := s.Jitter
		if jitter <= 0 {
			jitter = 6
		}
		a := anchors[randIntn(rng, len(anchors))]
		dx := randIntn(rng, jitter*2+1) - jitter
		dy := randIntn(rng, jitter*2+1) - jitter
		return a[0] + dx, a[1] + dy, true
	}
	return randIntn(rng, level.GetWidth()), randIntn(rng, level.GetHeight()), true
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
