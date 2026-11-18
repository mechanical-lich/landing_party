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
	StartFuel      int `json:"start_fuel"`
	StartColonists int `json:"start_colonists"`
	RosterCap      int `json:"roster_cap"`

	// StartRadius is how far outside the rim a landfall's arrival point (the
	// Wreck Site, for the player) sits from its landfall system: the first
	// jump's fuel cost.
	StartRadius float64 `json:"start_radius"`

	// Galaxy is the pregenerated galaxy's shape, sizes and landfalls
	// (galaxy.go); Gate the way out of it (gate.go).
	Galaxy GalaxyConfig `json:"galaxy"`
	Gate   GateConfig   `json:"gate"`

	DatapadQuestsMax int `json:"datapad_quests_max"` // hidden datapad quests per location (1..N)

	TravelQuestOneIn int `json:"travel_quest_one_in"` // 1-in-N chance per jump
	TravelQuestMax   int `json:"travel_quest_max"`    // up to this many (1..N)
	ContractOneIn    int `json:"contract_one_in"`     // chance a rolled quest is a contract

	// MinSeparation is the smallest allowed distance between a location placed
	// beside the galaxy (the Training Ground, the gate) and any other, so
	// icons don't stack. Placement re-rolls to satisfy it. 0 disables the
	// check. The galaxy's own systems use their size's min_separation.
	MinSeparation float64 `json:"min_separation"`

	// ScannerTiers is the ship scanner's power by level (index 0 = scanner
	// level 1). The scanner level is the number of ship_scanner_N techs
	// researched; level 0 has no scanner and cannot scan.
	ScannerTiers []ScannerTier `json:"scanner_tiers"`
	// ScanFuelCosts is the fuel one scan burns by efficiency level (index 0 =
	// base, each scan_efficiency_N tech advances one — cheaper).
	ScanFuelCosts []int `json:"scan_fuel_costs"`

	// SightRange is how far round the ship systems become seen (on arrival
	// and at the start); SightBonus adds to it by scanner level (index 0 =
	// scanner level 1).
	SightRange float64   `json:"sight_range"`
	SightBonus []float64 `json:"sight_bonus"`

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

// ScannerTier is one ship-scanner power level: how far from the ship a scan
// reaches.
type ScannerTier struct {
	Range float64 `json:"range"`
}

// GalaxyConfig is the pregenerated galaxy's tuning (generation.json "galaxy").
type GalaxyConfig struct {
	// DefaultSize is the New Expedition Galaxy size row's starting choice.
	DefaultSize string       `json:"default_size"`
	Sizes       []GalaxySize `json:"sizes"`
	// Shapes are rolled by weight; each id is a density function (galaxy_shapes.go).
	Shapes []GalaxyShape `json:"shapes"`
	// LandfallSpare is how many landfalls beyond factions_max+1 the rim
	// gets, so a later expedition has somewhere to arrive.
	LandfallSpare int `json:"landfall_spare"`
	// LandfallMinDistance keeps landfall points apart, so no two share a
	// safe start (it is at least 2*safe_start_radius+1 whatever it says).
	LandfallMinDistance float64 `json:"landfall_min_distance"`
	// RimBins is how many angular slices round the centre of mass the rim
	// is read from: the outermost system of each is a rim system.
	RimBins int `json:"rim_bins"`
	// GateHopMin/Max is the gate's distance from the world holding its controls.
	GateHopMin float64 `json:"gate_hop_min"`
	GateHopMax float64 `json:"gate_hop_max"`
}

// GalaxySize is one Galaxy size choice: how many systems in what radius.
type GalaxySize struct {
	ID            string  `json:"id"`
	NameDraft     string  `json:"name_draft"`
	Systems       int     `json:"systems"`
	Radius        float64 `json:"radius"`
	MinSeparation float64 `json:"min_separation"`
	// SightRange, when set, replaces sight_range on a galaxy of this size.
	SightRange float64 `json:"sight_range,omitempty"`
}

// GalaxyShape is one galaxy shape the generator may roll.
type GalaxyShape struct {
	ID        string `json:"id"`
	NameDraft string `json:"name_draft"`
	Weight    int    `json:"weight"`
}

func defaultGenConfig() GenConfig {
	return GenConfig{
		StartFuel:        40,
		StartColonists:   6,
		RosterCap:        12,
		StartRadius:      8,
		DatapadQuestsMax: 20,
		TravelQuestOneIn: 6,
		TravelQuestMax:   4,
		ContractOneIn:    3,
		MinSeparation:    5,
		ScannerTiers: []ScannerTier{
			{Range: 40},
			{Range: 70},
			{Range: 110},
		},
		Galaxy:          defaultGalaxyConfig(),
		Gate:            GateConfig{LeaveFuel: 300},
		SightRange:      60,
		SightBonus:      []float64{10, 20, 30},
		ScanFuelCosts:   []int{40, 25, 15},
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
	cfg.SightBonus = slices.Clone(cfg.SightBonus)
	cfg.Galaxy.Sizes = slices.Clone(cfg.Galaxy.Sizes)
	cfg.Galaxy.Shapes = slices.Clone(cfg.Galaxy.Shapes)
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
	if cfg.FactionsMax < 0 {
		cfg.FactionsMax = 0
	}
	if cfg.FactionsDefault > cfg.FactionsMax {
		cfg.FactionsDefault = cfg.FactionsMax
	}
	if cfg.FactionsDefault < 0 {
		cfg.FactionsDefault = 0
	}
	if len(cfg.Galaxy.Sizes) == 0 || len(cfg.Galaxy.Shapes) == 0 {
		def := defaultGalaxyConfig()
		if len(cfg.Galaxy.Sizes) == 0 {
			cfg.Galaxy.Sizes = def.Sizes
		}
		if len(cfg.Galaxy.Shapes) == 0 {
			cfg.Galaxy.Shapes = def.Shapes
		}
	}
	if cfg.Galaxy.RimBins < 8 {
		cfg.Galaxy.RimBins = 8
	}
	return cfg
}

// defaultGalaxyConfig is the galaxy tuning without a data file.
func defaultGalaxyConfig() GalaxyConfig {
	return GalaxyConfig{
		DefaultSize: "medium",
		Sizes: []GalaxySize{
			{ID: "small", NameDraft: "Small", Systems: 40, Radius: 110, MinSeparation: 8, SightRange: 40},
			{ID: "medium", NameDraft: "Medium", Systems: 80, Radius: 160, MinSeparation: 8},
			{ID: "large", NameDraft: "Large", Systems: 150, Radius: 220, MinSeparation: 8},
		},
		Shapes: []GalaxyShape{
			{ID: "spiral", NameDraft: "Spiral galaxy", Weight: 1},
			{ID: "barred_spiral", NameDraft: "Barred spiral galaxy", Weight: 1},
			{ID: "elliptical", NameDraft: "Elliptical galaxy", Weight: 1},
			{ID: "lenticular", NameDraft: "Lenticular galaxy", Weight: 1},
			{ID: "ring", NameDraft: "Ring galaxy", Weight: 1},
			{ID: "irregular", NameDraft: "Irregular galaxy", Weight: 1},
		},
		LandfallSpare:       4,
		LandfallMinDistance: 40,
		RimBins:             32,
		GateHopMin:          7,
		GateHopMax:          11,
	}
}

// GalaxySizes is the Galaxy size row's choices and its default id.
func GalaxySizes() (sizes []GalaxySize, def string) {
	g := genConfig().Galaxy
	return g.Sizes, g.DefaultSize
}

// galaxySize is the size with id ("" or unknown: the default, else the first).
func (g GalaxyConfig) size(id string) GalaxySize {
	if id == "" {
		id = g.DefaultSize
	}
	for _, s := range g.Sizes {
		if s.ID == id {
			return s
		}
	}
	for _, s := range g.Sizes {
		if s.ID == g.DefaultSize {
			return s
		}
	}
	return g.Sizes[0]
}
