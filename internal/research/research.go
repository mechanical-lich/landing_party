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
}

var techs map[string]Tech

func init() {
	techs = make(map[string]Tech)
	file, err := os.OpenFile("data/research.json", os.O_RDONLY, 0644)
	if err != nil {
		log.Print("Failed to open research.json:", err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		log.Print("Failed to read research.json:", err)
		return
	}
	if err := json.Unmarshal(data, &techs); err != nil {
		log.Print("Failed to unmarshal research.json:", err)
	}
	// Stamp each tech with its map key so callers can round-trip.
	for k, t := range techs {
		t.Key = k
		techs[k] = t
	}
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
func AvailableTechs(knownTechs []string) []Tech {
	known := make(map[string]bool, len(knownTechs))
	for _, k := range knownTechs {
		known[k] = true
	}
	var result []Tech
	for _, t := range techs {
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

// LockedTechs returns techs that are not yet researchable because their
// prerequisite has not been researched. Results are sorted by name.
func LockedTechs(knownTechs []string) []Tech {
	known := make(map[string]bool, len(knownTechs))
	for _, k := range knownTechs {
		known[k] = true
	}
	var result []Tech
	for _, t := range techs {
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
