package campaign

import (
	"encoding/json"
	"os"
)

// GenerationConfigPath is the data-driven tuning file for campaign
// generation. Overridable in tests.
var GenerationConfigPath = "data/generation.json"

// GenConfig holds every campaign-generation tuning knob. Missing file or
// missing fields fall back to defaults (see defaultGenConfig), so the game
// runs without the file and any subset can be overridden.
type GenConfig struct {
	HomeRadius     float64 `json:"home_radius"`
	StartFuel      int     `json:"start_fuel"`
	StartColonists int     `json:"start_colonists"`
	RosterCap      int     `json:"roster_cap"`

	NearbyMin       int     `json:"nearby_min"`
	NearbyMax       int     `json:"nearby_max"`
	NearbyRadiusMin float64 `json:"nearby_radius_min"`
	NearbyRadiusMax float64 `json:"nearby_radius_max"`

	DatapadQuestsMax int `json:"datapad_quests_max"` // hidden datapad quests per location (1..N)

	TravelQuestOneIn int `json:"travel_quest_one_in"` // 1-in-N chance per jump
	TravelQuestMax   int `json:"travel_quest_max"`    // up to this many (1..N)
	ContractOneIn    int `json:"contract_one_in"`     // chance a rolled quest is a contract
	NewSystemOneIn   int `json:"new_system_one_in"`   // chance a non-contract rolls a new system

	ExpansionGapMin  float64 `json:"expansion_gap_min"`
	ExpansionGapRand float64 `json:"expansion_gap_rand"`
	HomeCapFrac      float64 `json:"home_cap_frac"`
	AngleJitter      float64 `json:"angle_jitter"`

	// MinSeparation is the smallest allowed distance between any two locations,
	// so systems don't stack into overlapping star-map icons. Placement re-rolls
	// to satisfy it. 0 disables the check.
	MinSeparation float64 `json:"min_separation"`
}

func defaultGenConfig() GenConfig {
	return GenConfig{
		HomeRadius:       220,
		StartFuel:        40,
		StartColonists:   6,
		RosterCap:        12,
		NearbyMin:        2,
		NearbyMax:        3,
		NearbyRadiusMin:  9,
		NearbyRadiusMax:  27,
		DatapadQuestsMax: 20,
		TravelQuestOneIn: 6,
		TravelQuestMax:   4,
		ContractOneIn:    3,
		NewSystemOneIn:   3,
		ExpansionGapMin:  18,
		ExpansionGapRand: 32,
		HomeCapFrac:      0.9,
		AngleJitter:      1.1,
		MinSeparation:    5,
	}
}

var genCfgCache *GenConfig

// genConfig loads (and caches) the tuning config, layering the file over
// defaults so partial files work.
func genConfig() GenConfig {
	if genCfgCache != nil {
		return *genCfgCache
	}
	cfg := defaultGenConfig()
	if b, err := os.ReadFile(GenerationConfigPath); err == nil {
		_ = json.Unmarshal(b, &cfg) // unmarshal overlays only present fields
	}
	if cfg.TravelQuestOneIn < 1 {
		cfg.TravelQuestOneIn = 1
	}
	if cfg.TravelQuestMax < 1 {
		cfg.TravelQuestMax = 1
	}
	if cfg.ContractOneIn < 1 {
		cfg.ContractOneIn = 1
	}
	if cfg.NewSystemOneIn < 1 {
		cfg.NewSystemOneIn = 1
	}
	genCfgCache = &cfg
	return cfg
}
