package campaign

import (
	"encoding/json"
	"os"
	"time"
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

	// HomeBias (0..1) pulls each new system's direction toward Home: 0 spreads
	// in every direction (no progress bias), 1 heads straight at Home (a
	// corridor). Between, the map grows as a branching network that trends home
	// while still offering lateral directions to explore.
	HomeBias float64 `json:"home_bias"`

	// MinSeparation is the smallest allowed distance between any two locations,
	// so systems don't stack into overlapping star-map icons. Placement re-rolls
	// to satisfy it. 0 disables the check.
	MinSeparation float64 `json:"min_separation"`

	// ScannerTiers is the ship scanner's power by level (index 0 = scanner
	// level 1). The scanner level is the number of ship_scanner_N techs
	// researched; level 0 has no scanner and cannot scan.
	ScannerTiers []ScannerTier `json:"scanner_tiers"`
	// ScanFuelCosts is the fuel one scan burns by efficiency level (index 0 =
	// base, each scan_efficiency_N tech advances one — cheaper).
	ScanFuelCosts []int `json:"scan_fuel_costs"`
}

// ScannerTier is one ship-scanner power level: the chance a scan reveals a new
// location, and the max distance (= fuel cost to reach) it can appear from the
// ship's current location.
type ScannerTier struct {
	Chance float64 `json:"chance"`
	Range  float64 `json:"range"`
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
		HomeBias:         0.35,
		MinSeparation:    5,
		ScannerTiers: []ScannerTier{
			{Chance: 0.10, Range: 25},
			{Chance: 0.20, Range: 50},
			{Chance: 0.30, Range: 100},
		},
		ScanFuelCosts: []int{200, 100, 50},
	}
}

var (
	genCfgCache     *GenConfig
	genCfgCachePath string
	genCfgCacheMod  time.Time
)

// genConfig loads the tuning config, layering the file over defaults so partial
// files work. The parsed result is cached keyed on the file's path + mtime, so
// edits to data/generation.json are still picked up live (the cache invalidates
// when the file changes) — but a hot caller (the star map polls ScanFuelCost
// every frame) pays only a cheap os.Stat, not a read + JSON unmarshal + allocs.
func genConfig() GenConfig {
	path := GenerationConfigPath
	var mod time.Time
	if info, err := os.Stat(path); err == nil {
		mod = info.ModTime()
	}
	if genCfgCache != nil && genCfgCachePath == path && genCfgCacheMod.Equal(mod) {
		return *genCfgCache
	}

	cfg := defaultGenConfig()
	if b, err := os.ReadFile(path); err == nil {
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
	if cfg.HomeBias < 0 {
		cfg.HomeBias = 0
	} else if cfg.HomeBias > 1 {
		cfg.HomeBias = 1
	}

	genCfgCache = &cfg
	genCfgCachePath = path
	genCfgCacheMod = mod
	return cfg
}
