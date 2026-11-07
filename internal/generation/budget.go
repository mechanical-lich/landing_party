package generation

import (
	"encoding/json"
	"os"
	"sync"
)

// LoreBudgetPath is where the lore budget is read from (the lore_budget block
// of the generation config). Missing or unreadable, the defaults below apply.
var LoreBudgetPath = "data/generation.json"

// BudgetClass is what one map size class allows a site's history to leave,
// and how far stamps keep from each other.
type BudgetClass struct {
	// MaxWidth is the widest map in the class; 0 for the last (largest) class.
	MaxWidth int `json:"max_width"`
	// Stamps is the most structures history evidence may stamp.
	Stamps int `json:"stamps"`
	// FootprintPercent caps the stamps' summed footprint, as a percent of the
	// map's surface.
	FootprintPercent float64 `json:"footprint_percent"`
	// Underground caps stamps that dig below the surface (underground_*).
	Underground int `json:"underground"`
	// Hostiles caps creatures history evidence spawns; HazardHostiles is the
	// part a hazard chapter may bring.
	Hostiles       int `json:"hostiles"`
	HazardHostiles int `json:"hazard_hostiles"`
	// Radiation caps radiation pockets from history evidence.
	Radiation int `json:"radiation"`
	// Spacing is the least gap, in tiles, between two stamps' footprints.
	Spacing int `json:"spacing"`
}

// LoreBudget is generation.json's lore_budget: per-size-class limits and the
// landing reserve's keep-outs.
type LoreBudget struct {
	Classes []BudgetClass `json:"classes"`
	// LandingHalf is the landing reserve's half-size: a (2*half+1) box.
	LandingHalf int `json:"landing_half"`
	// KeepOut is how far (tiles from the reserve box) an inert origin stays;
	// HazardKeepOut the same for origins that spawn hostiles (hazards and
	// threats), unless the event sets its own keep_out.
	KeepOut       int `json:"keep_out"`
	HazardKeepOut int `json:"hazard_keep_out"`
}

// DefaultLoreBudget is the plan's numbers (lore_content_plan.md §6).
var DefaultLoreBudget = LoreBudget{
	Classes: []BudgetClass{
		{MaxWidth: 180, Stamps: 3, FootprintPercent: 3, Underground: 1, Hostiles: 6, HazardHostiles: 6, Radiation: 2, Spacing: 20},
		{MaxWidth: 384, Stamps: 5, FootprintPercent: 3, Underground: 2, Hostiles: 10, HazardHostiles: 6, Radiation: 3, Spacing: 24},
		{MaxWidth: 0, Stamps: 7, FootprintPercent: 3, Underground: 2, Hostiles: 14, HazardHostiles: 6, Radiation: 4, Spacing: 32},
	},
	LandingHalf:   10,
	KeepOut:       16,
	HazardKeepOut: 40,
}

var (
	budgetOnce sync.Once
	budget     LoreBudget
)

// LoadLoreBudget reads a generation config's lore_budget block; fields it
// leaves out keep their defaults.
func LoadLoreBudget(path string) (LoreBudget, error) {
	b := DefaultLoreBudget
	raw, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	var f struct {
		LoreBudget *LoreBudget `json:"lore_budget"`
	}
	f.LoreBudget = &b
	if err := json.Unmarshal(raw, &f); err != nil {
		return DefaultLoreBudget, err
	}
	return b, nil
}

// Budget is the lore budget in use: LoreBudgetPath's, read once, else the
// defaults.
func Budget() LoreBudget {
	budgetOnce.Do(func() {
		budget, _ = LoadLoreBudget(LoreBudgetPath)
	})
	return budget
}

// For is the size class of a map width wide.
func (b LoreBudget) For(width int) BudgetClass {
	for _, c := range b.Classes {
		if c.MaxWidth == 0 || width <= c.MaxWidth {
			return c
		}
	}
	if len(b.Classes) == 0 {
		return DefaultLoreBudget.For(width)
	}
	return b.Classes[len(b.Classes)-1]
}
