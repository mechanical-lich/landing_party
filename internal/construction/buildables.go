package construction

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sort"
)

type Buildable struct {
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	Description   string         `json:"description"`
	BuildTime     int            `json:"build_time"`
	IsEntity      bool           `json:"is_entity"`
	Hidden        bool           `json:"hidden"`
	RequiredTech  string         `json:"required_tech"`
	AllowMultiple bool           `json:"allow_multiple"`
	Cost          map[string]int `json:"cost"`
}

var buildables map[string]Buildable

func init() {
	buildables = make(map[string]Buildable)
	file, err := os.OpenFile("data/build.json", os.O_RDONLY, 0644)
	if err != nil {
		log.Print("Failed to open build.json:", err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		log.Print("Failed to read build.json:", err)
		return
	}
	if err := json.Unmarshal(data, &buildables); err != nil {
		log.Print("Failed to unmarshal build.json:", err)
	}
}

func GetBuildable(key string) Buildable {
	if b, ok := buildables[key]; ok {
		return b
	}
	log.Print("Buildable not found:", key)
	return Buildable{}
}

func ListBuildables() map[string]Buildable {
	return buildables
}

// ListBuildablesForTechs returns buildables that are available given the known tech keys.
// Entries with RequiredTech set are only included if that tech is in knownTechs.
func ListBuildablesForTechs(knownTechs []string) []Buildable {
	techSet := make(map[string]bool, len(knownTechs))
	for _, t := range knownTechs {
		techSet[t] = true
	}
	var result []Buildable
	for _, b := range buildables {
		if b.Hidden {
			continue
		}
		if b.RequiredTech != "" && !techSet[b.RequiredTech] {
			continue
		}
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
