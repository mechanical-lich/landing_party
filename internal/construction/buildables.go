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
	// Category groups this buildable in the build menu (e.g. "walls", "floors",
	// "doors"). Empty = not menu-listed. Order sorts within a category; ties
	// fall back to Name. Together they make new buildables show up in the
	// menu by editing build.json alone, no Go change required.
	Category string `json:"category,omitempty"`
	Order    int    `json:"order,omitempty"`
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

// BuildableTypesInCategory returns the build-type keys (map keys, i.e. the
// build IDs callers pass to GetBuildable) for every non-hidden buildable in
// category, sorted by Order ascending and Name as tiebreaker. Build-menu code
// uses this to render a category's items without listing them in Go.
func BuildableTypesInCategory(category string) []string {
	type entry struct {
		key      string
		order    int
		name     string
	}
	var in []entry
	for key, b := range buildables {
		if b.Hidden || b.Category != category {
			continue
		}
		in = append(in, entry{key: key, order: b.Order, name: b.Name})
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].order != in[j].order {
			return in[i].order < in[j].order
		}
		return in[i].name < in[j].name
	})
	out := make([]string, len(in))
	for i, e := range in {
		out[i] = e.key
	}
	return out
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
