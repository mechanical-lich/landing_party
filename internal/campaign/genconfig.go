package campaign

import (
	"encoding/json"
	"os"
	"slices"
	"sync"
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

	// StartRadius is how far the generated start system sits from the Wreck
	// Site at the origin: the first jump's fuel cost. Keep it under
	// NearbyRadiusMin so the start is the nearest destination.
	StartRadius float64 `json:"start_radius"`

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

	// JumpFuelPerDay converts a jump's fuel cost to the star-date days it
	// takes: ceil(fuel / JumpFuelPerDay).
	JumpFuelPerDay int `json:"jump_fuel_per_day"`

	// SafeStartRadius is how far from the Wreck Site a site counts as a first
	// jump, whose history only ends in a safe chapter (see
	// history.BindSiteSafe). 0 means start_radius: the start system only.
	SafeStartRadius float64 `json:"safe_start_radius"`

	// FactionsDefault / FactionsMax are the New Expedition "Rival factions"
	// stepper's starting value and upper bound.
	FactionsDefault int `json:"factions_default"`
	FactionsMax     int `json:"factions_max"`
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
		StartRadius:      8,
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
		ScanFuelCosts:   []int{200, 100, 50},
		JumpFuelPerDay:  3,
		FactionsDefault: 1,
		FactionsMax:     3,
	}
}

// genConfig is the tuning config, the file layered over defaults so partial
// files work. Edits to data/generation.json still take effect without a
// restart: the loaded config is kept, and the file is looked at again (its
// modification time and size) at most once a second, and re-read when it has
// changed. The faction directors read it every step (firstJump), so a call
// between checks touches no file. The check is by wall clock, which reaches
// only when an edited file is noticed, never what an unedited one gives.
func genConfig() GenConfig {
	genCache.Lock()
	defer genCache.Unlock()
	now := time.Now()
	if !genCache.loaded || genCache.path != GenerationConfigPath || now.Sub(genCache.checked) >= genConfigRecheck {
		var mod time.Time
		size := int64(-1) // no file: the defaults
		if fi, err := os.Stat(GenerationConfigPath); err == nil {
			mod, size = fi.ModTime(), fi.Size()
		}
		if !genCache.loaded || genCache.path != GenerationConfigPath || !mod.Equal(genCache.mod) || size != genCache.size {
			genCache.cfg = loadGenConfig()
			genCache.path, genCache.mod, genCache.size, genCache.loaded = GenerationConfigPath, mod, size, true
		}
		genCache.checked = now
	}
	cfg := genCache.cfg
	// The slices are the cache's own: a caller gets copies.
	cfg.ScannerTiers = slices.Clone(cfg.ScannerTiers)
	cfg.ScanFuelCosts = slices.Clone(cfg.ScanFuelCosts)
	return cfg
}

// genConfigRecheck is how often genConfig looks at the file again.
const genConfigRecheck = time.Second

var genCache struct {
	sync.Mutex
	loaded  bool
	path    string
	mod     time.Time
	size    int64
	checked time.Time
	cfg     GenConfig
}

// loadGenConfig reads the file over the defaults and clamps the values.
func loadGenConfig() GenConfig {
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
	if cfg.FactionsMax < 0 {
		cfg.FactionsMax = 0
	}
	if cfg.FactionsDefault > cfg.FactionsMax {
		cfg.FactionsDefault = cfg.FactionsMax
	}
	if cfg.FactionsDefault < 0 {
		cfg.FactionsDefault = 0
	}
	if cfg.HomeBias < 0 {
		cfg.HomeBias = 0
	} else if cfg.HomeBias > 1 {
		cfg.HomeBias = 1
	}
	return cfg
}
