package scenario

import "github.com/mechanical-lich/landing_party/internal/mapdef"

type SpawnRule struct {
	SpawnRate int      `json:"spawn_rate"`
	LightMin  int      `json:"light_min"`
	LightMax  int      `json:"light_max"`
	Tiles     []string `json:"tiles"`
	// MinZDelta and MaxZDelta are offsets from the level's SurfaceZ. nil =
	// no bound on that side. A rule with both nil applies at every Z. So
	// "spawn on the surface" is {min:0, max:0}, "anywhere underground" is
	// {min:nil, max:-1}, and "above-surface aerial" is {min:1, max:nil}.
	//
	// Using deltas rather than absolute Z values lets the same scenario
	// scale across maps with different surface Z (asteroids put surface at
	// z=0, planets put it wherever the depth picks).
	MinZDelta         *int               `json:"min_z_delta,omitempty"`
	MaxZDelta         *int               `json:"max_z_delta,omitempty"`
	StartingEquipment map[string]float64 `json:"starting_equipment,omitempty"`
}

// LightingConfig controls the ambient light behaviour for a scenario.
// Mode values: "day_night" (default), "fixed", "pitch_dark".
// AmbientLevel is only used when Mode == "fixed" (0–100).
type LightingConfig struct {
	Mode         string `json:"mode"`
	AmbientLevel int    `json:"ambient_level"`
}

type Scenario struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	Enabled       bool                 `json:"enabled"`
	HostileMax    int                  `json:"hostile_max"`
	SpawnRules    map[string]SpawnRule `json:"spawn_rules"`
	SetupScripts  []string             `json:"setup_scripts"`
	Lighting      LightingConfig       `json:"lighting"`
	// SupportedMaps lists the map IDs (data/maps/*.json) this scenario can run
	// on. Empty means "any map". Terrain generation is driven by the chosen
	// map, not by the scenario.
	SupportedMaps []string             `json:"supported_maps,omitempty"`
	Features      []mapdef.FeatureBlock `json:"features,omitempty"`
}

// SupportsMap reports whether this scenario can run on the given map ID. An
// empty SupportedMaps list means the scenario supports any map.
func (s *Scenario) SupportsMap(mapID string) bool {
	if len(s.SupportedMaps) == 0 {
		return true
	}
	for _, m := range s.SupportedMaps {
		if m == mapID {
			return true
		}
	}
	return false
}
