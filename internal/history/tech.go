package history

import (
	"encoding/json"
	"hash/fnv"
	"strconv"
	"strings"
)

// Tech levels, in order. Pool civs (the ones that travel) are spacefaring;
// natives start low (civs.json native_tech) and may climb, up to atomic.
const (
	TechPrimitive   = "primitive"
	TechMedieval    = "medieval"
	TechIndustrial  = "industrial"
	TechAtomic      = "atomic"
	TechSpacefaring = "spacefaring"
)

// TechLevels lists the levels from lowest to highest.
var TechLevels = []string{TechPrimitive, TechMedieval, TechIndustrial, TechAtomic, TechSpacefaring}

// nativeTechMax is the highest level a native civ reaches: natives never
// reach space (they would have to join the pool).
const nativeTechMax = 3 // atomic

// TechIndex is a level's place in TechLevels, -1 for an unknown one.
func TechIndex(level string) int {
	for i, l := range TechLevels {
		if l == level {
			return i
		}
	}
	return -1
}

// TechDef is one level of data/history/tech.json: the structure script for
// each role (an evidence "@role"; a role a level lacks drops the stamp), the
// weapons {weapon} fills from, and the relics (item blueprints) its people
// leave behind.
type TechDef struct {
	ID         string              `json:"id"`
	Label      string              `json:"label"`
	Structures map[string]TechRole `json:"structures"`
	Weapons    []string            `json:"weapons"`
	Relics     []string            `json:"relics"`
}

// TechStructure is what a role builds at a level: a structure script and its
// footprint, with a smaller one for small maps (so the lore budget's
// footprint share holds there).
type TechStructure struct {
	Script string `json:"script"`
	W      int    `json:"w,omitempty"`
	H      int    `json:"h,omitempty"`
	Small  []int  `json:"small,omitempty"`
	// Weight is this alternative's share when a role lists several
	// (default 1).
	Weight float64 `json:"weight,omitempty"`
}

// TechRole is what a role builds at a level: one structure, or a list of
// alternatives one of which each stamp picks by weight (a medieval seat is a
// castle town or a castle).
type TechRole []TechStructure

// UnmarshalJSON takes a list of alternatives, or one structure (an object or
// a bare script name).
func (r *TechRole) UnmarshalJSON(b []byte) error {
	var list []TechStructure
	if json.Unmarshal(b, &list) == nil {
		*r = list
		return nil
	}
	var one TechStructure
	if err := json.Unmarshal(b, &one); err != nil {
		return err
	}
	*r = TechRole{one}
	return nil
}

// Pick chooses an alternative by weight from key (stable: the same key
// always picks the same one). The zero TechStructure when there is none.
func (r TechRole) Pick(key string) TechStructure {
	if len(r) == 0 {
		return TechStructure{}
	}
	total := 0.0
	for _, s := range r {
		total += s.weight()
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	u := float64(h.Sum32()) / float64(1<<32) * total
	for _, s := range r {
		if u < s.weight() {
			return s
		}
		u -= s.weight()
	}
	return r[len(r)-1]
}

func (s TechStructure) weight() float64 {
	if s.Weight > 0 {
		return s.Weight
	}
	return 1
}

// UnmarshalJSON takes a bare script name too.
func (t *TechStructure) UnmarshalJSON(b []byte) error {
	var name string
	if json.Unmarshal(b, &name) == nil {
		*t = TechStructure{Script: name}
		return nil
	}
	type plain TechStructure
	return json.Unmarshal(b, (*plain)(t))
}

// Footprint is the structure's w, h on a map of the given size class
// ("small" takes Small); 0, 0 keeps the evidence block's own.
func (t TechStructure) Footprint(size string) (int, int) {
	if size == "small" && len(t.Small) == 2 {
		return t.Small[0], t.Small[1]
	}
	return t.W, t.H
}

// PurposeDef is why a place was built or a ship came (purposes.json): a
// ship's hull class, its rooms' names, the loot its lockers take, and the
// clue its datapad reads.
type PurposeDef struct {
	Class string            `json:"class,omitempty"`
	Holds string            `json:"holds,omitempty"`
	Rooms map[string]string `json:"rooms,omitempty"`
	Clue  string            `json:"clue,omitempty"`
}

// ShipClass is a hull class's footprint; Stamps is what it counts against a
// site's stamp budget (default 1), LargeOnly keeps it to large maps.
type ShipClass struct {
	W         int  `json:"w"`
	H         int  `json:"h"`
	Stamps    int  `json:"stamps,omitempty"`
	LargeOnly bool `json:"large_only,omitempty"`
}

type purposesFile struct {
	ShipClasses map[string]ShipClass  `json:"ship_classes"`
	Purposes    map[string]PurposeDef `json:"purposes"`
}

type techFile struct {
	Levels []TechDef `json:"levels"`
}

// Tech returns a level's definition, or nil.
func (l *Library) Tech(level string) *TechDef {
	for i := range l.TechDefs {
		if l.TechDefs[i].ID == level {
			return &l.TechDefs[i]
		}
	}
	return nil
}

// applyTech changes a level by an effect: "+1", "-2", or a level name.
// Natives stay within primitive..atomic.
func applyTech(cur int, effect string, native bool) int {
	next := cur
	switch {
	case strings.HasPrefix(effect, "+") || strings.HasPrefix(effect, "-"):
		if d, err := strconv.Atoi(effect); err == nil {
			next = cur + d
		}
	default:
		if i := TechIndex(effect); i >= 0 {
			next = i
		}
	}
	top := len(TechLevels) - 1
	if native {
		top = nativeTechMax
	}
	return max(0, min(next, top))
}
