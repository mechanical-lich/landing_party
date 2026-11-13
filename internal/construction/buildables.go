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
	// RequiredFloor: every tile of the footprint must have one of these
	// floors (the hive's structures stand on resin, the autonz' on wire;
	// a string or a list in the file). Checked by the build order path the
	// player's cursor and every director share, and by the director's site
	// pick.
	RequiredFloor FloorList `json:"required_floor,omitempty"`
	// DeployOnly: never ordered built; a unit deploys into it (the autonz'
	// relay pylon: a relay drone; autonz_plan.md). The build order path
	// refuses it.
	DeployOnly bool `json:"deploy_only,omitempty"`
	// ModuleOnly: a rival never orders it built; it comes with a ship's hull
	// tier (the greys' vat and console; greyish_plan.md, "Structures"). A
	// faction director's build order path refuses it; the player's does not
	// (phase F builds them for a grey player).
	ModuleOnly bool `json:"module_only,omitempty"`
}

// FloorList is a required floor: one tile name or a list of them.
type FloorList []string

// UnmarshalJSON reads a string or a list.
func (f *FloorList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*f = nil
		if one != "" {
			*f = FloorList{one}
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*f = many
	return nil
}

// Has reports whether name is one of the floors.
func (f FloorList) Has(name string) bool {
	for _, v := range f {
		if v == name {
			return true
		}
	}
	return false
}

// FloorOK reports whether a build of b covering tiles may go there: b has no
// RequiredFloor, or floorAt names one of them on every tile.
func (b Buildable) FloorOK(tiles [][2]int, floorAt func(x, y int) string) bool {
	if len(b.RequiredFloor) == 0 {
		return true
	}
	for _, t := range tiles {
		if !b.RequiredFloor.Has(floorAt(t[0], t[1])) {
			return false
		}
	}
	return true
}

var buildables map[string]Buildable

func init() {
	if err := Load("data/build.json"); err != nil {
		log.Print("Failed to load build.json:", err)
	}
}

// Load replaces the buildable table with path's (empty when it cannot be
// read; Load("") clears it). init loads data/build.json relative to the cwd.
func Load(path string) error {
	buildables = make(map[string]Buildable)
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
	return json.Unmarshal(data, &buildables)
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
		if b.Hidden || b.DeployOnly || b.Category != category {
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
		if b.Hidden || b.DeployOnly {
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
