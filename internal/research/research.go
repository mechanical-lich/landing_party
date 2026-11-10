package research

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sort"
)

type Tech struct {
	Key              string         `json:"-"` // populated post-load from the map key
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Duration         int            `json:"duration"`
	Cost             map[string]int `json:"cost"`
	RequiredBuilding string         `json:"required_building"`
	RequiredInt      int            `json:"required_int"`
	RequiresTech     string         `json:"requires_tech"`
	// Tree names the doctrine tree an entry belongs to ("hive"; "" the
	// colony's), Evolve its effect on a typed doctrine's units
	// (hive_plan.md, "The evolve tree").
	Tree   string  `json:"tree,omitempty"`
	Evolve *Evolve `json:"evolve,omitempty"`
	// Assimilate is an autonz entry's effect (autonz_plan.md, "The
	// assimilation tree").
	Assimilate *Assimilate `json:"assimilate,omitempty"`
}

// Assimilate is an assimilation entry's effect: Unlock a unit type, Tier a
// type's print tier (bronze), Build names the structure the entry unlocks
// (the buildable's requires_tech gates it; this is for the effect line),
// Range the signal range and ConversionRate the conversion rate (the Tier 1
// walk's own numbers are the doctrine's).
type Assimilate struct {
	Unlock         string     `json:"unlock,omitempty"`
	Tier           *AssimTier `json:"tier,omitempty"`
	Build          string     `json:"build,omitempty"`
	Range          float64    `json:"range,omitempty"`
	ConversionRate float64    `json:"conversion_rate,omitempty"`
}

// AssimTier: printed units of Type come out at Tier.
type AssimTier struct {
	Type string `json:"type"`
	Tier string `json:"tier"`
}

// Evolve is an evolve entry's effect: Unlock a unit type; or, for Type,
// multiply its adults' Health, Str and Initiative (below 1 is faster) and
// Work (work speed), Evolved drawing them in the fully evolved frame (the
// type's second upgrade); Range is launcher range, ResinRate added to the
// conversion rate, FlierSpeed an Initiative multiplier for every flier.
type Evolve struct {
	Unlock     string  `json:"unlock,omitempty"`
	Type       string  `json:"type,omitempty"`
	Health     float64 `json:"health,omitempty"`
	Str        float64 `json:"str,omitempty"`
	Initiative float64 `json:"initiative,omitempty"`
	Work       float64 `json:"work,omitempty"`
	Evolved    bool    `json:"evolved,omitempty"`
	Range      float64 `json:"range,omitempty"`
	ResinRate  float64 `json:"resin_rate,omitempty"`
	FlierSpeed float64 `json:"flier_speed,omitempty"`
}

var techs map[string]Tech

func init() {
	if err := Load("data/research.json"); err != nil {
		log.Print("Failed to load research.json:", err)
	}
}

// Load replaces the tech table with path's (empty when it cannot be read;
// Load("") clears it). init loads data/research.json relative to the cwd.
func Load(path string) error {
	techs = make(map[string]Tech)
	if path == "" {
		return nil
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &techs); err != nil {
		return err
	}
	// Stamp each tech with its map key so callers can round-trip.
	for k, t := range techs {
		t.Key = k
		techs[k] = t
	}
	return nil
}

func GetTech(key string) (Tech, bool) {
	t, ok := techs[key]
	return t, ok
}

func AllTechs() map[string]Tech {
	return techs
}

// AvailableTechs returns techs that can be researched given the current known
// tech keys. A tech is available when it isn't already known and any
// prerequisite tech is. Results are sorted by name for stable display order.
func AvailableTechs(knownTechs []string) []Tech { return AvailableTechsIn(nil, knownTechs) }

// LockedTechs returns techs that are not yet researchable because their
// prerequisite has not been researched. Results are sorted by name.
func LockedTechs(knownTechs []string) []Tech { return LockedTechsIn(nil, knownTechs) }

// AvailableTechsIn is AvailableTechs limited to tree (a settlement
// doctrine's research list); a nil tree is every tech.
func AvailableTechsIn(tree, knownTechs []string) []Tech {
	known := keySet(knownTechs)
	var result []Tech
	for _, t := range inTree(tree) {
		if known[t.Key] {
			continue
		}
		if t.RequiresTech != "" && !known[t.RequiresTech] {
			continue
		}
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// LockedTechsIn is LockedTechs limited to tree; a nil tree is every tech.
func LockedTechsIn(tree, knownTechs []string) []Tech {
	known := keySet(knownTechs)
	var result []Tech
	for _, t := range inTree(tree) {
		if known[t.Key] {
			continue
		}
		if t.RequiresTech != "" && !known[t.RequiresTech] {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// inTree is the techs named by tree that exist, or every tech for nil.
func inTree(tree []string) []Tech {
	var out []Tech
	if tree == nil {
		for _, t := range techs {
			out = append(out, t)
		}
		return out
	}
	for _, k := range tree {
		if t, ok := techs[k]; ok {
			out = append(out, t)
		}
	}
	return out
}

func keySet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set
}
