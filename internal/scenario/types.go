package scenario

import "github.com/mechanical-lich/scifi_settlements/internal/wincondition"

type SpawnRule struct {
	SpawnRate        int                `json:"spawn_rate"`
	LightMin         int                `json:"light_min"`
	LightMax         int                `json:"light_max"`
	Tiles            []string           `json:"tiles"`
	MinZ             int                `json:"min_z"`
	MaxZ             int                `json:"max_z"`
	StartingEquipment map[string]float64 `json:"starting_equipment,omitempty"`
}

// LightingConfig controls the ambient light behaviour for a scenario.
// Mode values: "day_night" (default), "fixed", "pitch_dark".
// AmbientLevel is only used when Mode == "fixed" (0–100).
type LightingConfig struct {
	Mode         string `json:"mode"`
	AmbientLevel int    `json:"ambient_level"`
}

// WorldConfig is the v2 scenario block for world generation. Optional —
// scenarios without it fall back to the legacy planet pipeline.
type WorldConfig struct {
	Terrain       string         `json:"terrain"`        // primer name: "planet", "asteroid_field", "abandoned_station"
	TerrainParams map[string]any `json:"terrain_params"` // forwarded to the primer
	BiomeMap      BiomeMapBlock  `json:"biome_map"`      // optional
	Features      []FeatureBlock `json:"features"`       // ordered list of feature placements
}

// BiomeMapBlock mirrors generation.BiomeMapConfig but lives here to keep
// scenario JSON self-contained.
type BiomeMapBlock struct {
	Type   string   `json:"type"`
	Scale  float64  `json:"scale"`
	Biomes []string `json:"biomes"`
	Single string   `json:"single"`
}

// FeatureBlock mirrors generation.FeatureSpec.
type FeatureBlock struct {
	Kind     string         `json:"kind"`
	Count    int            `json:"count"`
	Biome    string         `json:"biome,omitempty"`
	InRegion string         `json:"in_region,omitempty"`
	Jitter   int            `json:"jitter,omitempty"`
	MinZ     int            `json:"min_z,omitempty"`
	MaxZ     int            `json:"max_z,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

type Scenario struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Enabled       bool                    `json:"enabled"`
	HostileMax    int                     `json:"hostile_max"`
	SpawnRules    map[string]SpawnRule    `json:"spawn_rules"`
	SetupScripts  []string                `json:"setup_scripts"`
	WinConditions wincondition.RuleSet    `json:"win_conditions"`
	Lighting      LightingConfig          `json:"lighting"`
	World         *WorldConfig            `json:"world,omitempty"`
}
