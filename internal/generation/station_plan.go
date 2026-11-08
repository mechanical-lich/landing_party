package generation

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// A station plan is what a station site's history says the station is: its
// purpose, its frame (spine, ring or cross), how many decks, and the modules
// hung off the frame, each with the event that put it there. History writes
// it (history.SiteEvidence.Station); it travels to the primer as
// TerrainParams["station_plan"], JSON-shaped, and the primer lays the frame
// and runs each module's script into a slot. Without one the primer rolls a
// default plan from its seed (DefaultStationPlan), so a station builds
// without a history too.

// StationPlanParam is the TerrainParams key the plan travels under.
const StationPlanParam = "station_plan"

// Station frames.
const (
	FrameSpine = "spine"
	FrameRing  = "ring"
	FrameCross = "cross"
)

// StationPlan is a station's purpose, frame and modules.
type StationPlan struct {
	Purpose string `json:"purpose"`
	Frame   string `json:"frame"`
	Decks   int    `json:"decks"`
	// Uncrewed marks a station nobody ever lived on (a drone relay).
	Uncrewed bool         `json:"uncrewed,omitempty"`
	Modules  []PlanModule `json:"modules"`
	Wreck    *PlanWreck   `json:"wreck,omitempty"`
	// DeckDamage is the least damage of every module on a deck, keyed by the
	// module ID whose deck it is ("deck_of"), so history can say "the deck the
	// nanites hit" before the primer has assigned decks.
	DeckDamage []DeckDamage `json:"deck_damage_of,omitempty"`
	// StationDamage is the least damage of every module (an ending that hit
	// the whole station).
	StationDamage int `json:"station_damage,omitempty"`
	// StationKind is the look of the station-wide damage ("abandoned").
	StationKind string `json:"station_kind,omitempty"`
}

// ModuleDamage is each module's damage (by ID) once laid out on decks
// (deckOf: module ID → deck): the largest of its own, its deck's and the
// station's, at most 3; and its look: its own, else its deck's, else the
// station's.
func (p *StationPlan) ModuleDamage(deckOf map[string]int) (map[string]int, map[string]string) {
	dmg, kind := map[string]int{}, map[string]string{}
	deckLevel, deckKind := p.DeckFloors(deckOf)
	for _, m := range p.Modules {
		z := deckOf[m.ID]
		dmg[m.ID] = min(max(m.Damage, deckLevel[z], p.StationDamage), 3)
		switch {
		case m.DamageKind != "":
			kind[m.ID] = m.DamageKind
		case deckKind[z] != "":
			kind[m.ID] = deckKind[z]
		default:
			kind[m.ID] = p.StationKind
		}
	}
	return dmg, kind
}

// DeckFloors are each deck's least damage and its look (DeckDamage on the
// deck of its Of module, Spread decks either side at one less), the
// station's floor not included.
func (p *StationPlan) DeckFloors(deckOf map[string]int) (map[int]int, map[int]string) {
	deckLevel, deckKind := map[int]int{}, map[int]string{}
	for _, dd := range p.DeckDamage {
		z0, ok := deckOf[dd.Of]
		if !ok {
			continue
		}
		for dz := -dd.Spread; dz <= dd.Spread; dz++ {
			lv := dd.Level
			if dz != 0 {
				lv--
			}
			if lv > deckLevel[z0+dz] {
				deckLevel[z0+dz] = lv
			}
			if dd.Kind != "" && deckKind[z0+dz] == "" {
				deckKind[z0+dz] = dd.Kind
			}
		}
	}
	return deckLevel, deckKind
}

// DeckDamage raises every module on the deck of module Of to at least Level
// (with Kind's look); Spread raises the decks within Spread of it to at least
// Level-1.
type DeckDamage struct {
	Of     string `json:"of"`
	Level  int    `json:"level"`
	Kind   string `json:"kind,omitempty"`
	Spread int    `json:"spread,omitempty"`
}

// PlanModule is one module of a station plan.
type PlanModule struct {
	// ID is "<kind>" or "<kind>:<n>" for the nth of a kind.
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Event is the history event that built or claimed the module ("" for
	// one of the purpose's own).
	Event string `json:"event,omitempty"`
	// Damage 0-3 and its look ("breach", "nanite", "meltdown", "abandoned").
	Damage     int    `json:"damage,omitempty"`
	DamageKind string `json:"damage_kind,omitempty"`
	// Sealed is how its door is sealed ("keycard"), "" for open.
	Sealed    string `json:"sealed,omitempty"`
	Occupants string `json:"occupants,omitempty"`
	// Hazard marks a module that anchors the chapter or spawns creatures: it
	// keeps its distance from the dock (station_hazard_hops).
	Hazard bool `json:"hazard,omitempty"`
	// Log marks the module holding the station's log console.
	Log bool `json:"log,omitempty"`
	// Params pass to the module's script on top of the frame's own (holds,
	// purpose, clue, room_<kind>).
	Params map[string]any `json:"params,omitempty"`
}

// WreckModule is the ID of a docked wreck's module on the level and in
// origin bindings.
const WreckModule = "wreck"

// PlanWreck is a ship docked at the station's arm (an ancient_wreck on a
// station): crashed_ship, docked, on the dock's deck.
type PlanWreck struct {
	Event  string         `json:"event"`
	Params map[string]any `json:"params,omitempty"`
	W      int            `json:"w"`
	H      int            `json:"h"`
}

// ModuleKinds are the module kinds the primer knows, with the script each
// runs and its slot sizes, smallest first (S 7x7, M 11x9, L 15x11, XL 21x13).
var ModuleKinds = map[string]ModuleKind{
	"dock":        {Script: "station_dock", Sizes: []string{"M"}, Band: BandDock},
	"generator":   {Script: "station_generator", Sizes: []string{"M"}, Band: BandBottom},
	"reactor":     {Script: "ship_reactor", Sizes: []string{"L"}, Band: BandBottom},
	"cargo":       {Script: "ship_cargo", Sizes: []string{"L", "XL"}, Band: BandLow},
	"quarters":    {Script: "ship_quarters", Sizes: []string{"M", "L"}, Band: BandMiddle},
	"hangar":      {Script: "ship_hangar", Sizes: []string{"XL"}, Band: BandLow, Outer: true},
	"bridge":      {Script: "ship_bridge", Sizes: []string{"M"}, Band: BandUpper},
	"lab":         {Script: "station_lab", Sizes: []string{"M", "L"}, Band: BandUpper},
	"cryo":        {Script: "ship_cryo_bay", Sizes: []string{"L"}, Band: BandMiddle},
	"armoury":     {Script: "ship_armoury", Sizes: []string{"S", "M"}, Band: BandUpper},
	"botany":      {Script: "station_botany", Sizes: []string{"L"}, Band: BandMiddle},
	"archive":     {Script: "station_archive", Sizes: []string{"M"}, Band: BandUpper},
	"observatory": {Script: "station_observatory", Sizes: []string{"M"}, Band: BandTop},
	"relay":       {Script: "station_relay", Sizes: []string{"L"}, Band: BandTop},
	"refinery":    {Script: "station_refinery", Sizes: []string{"XL"}, Band: BandLow},
	"medbay":      {Script: "station_medbay", Sizes: []string{"S", "M"}, Band: BandMiddle},
}

// ModuleKind is what the primer knows of a module kind.
type ModuleKind struct {
	Script string
	Sizes  []string
	Band   int
	// Outer modules open onto space (a hangar's bay doors).
	Outer bool
}

// Deck bands: where on the station a kind of module prefers to be.
const (
	BandDock   = iota // the dock's deck
	BandBottom        // below the dock (the dock's deck on a 3-deck station)
	BandLow
	BandMiddle
	BandUpper
	BandTop
)

// SlotSizes are the slot classes' footprints.
var SlotSizes = map[string][2]int{"S": {7, 7}, "M": {11, 9}, "L": {15, 11}, "XL": {21, 13}}

// ModuleKindOf is a module ID's kind ("cargo:2" is a cargo).
func ModuleKindOf(id string) string {
	k, _, _ := strings.Cut(id, ":")
	return k
}

// HasKind reports whether the plan has a module of kind.
func (p *StationPlan) HasKind(kind string) bool {
	return p.Module(kind) != nil
}

// Module returns the first module of kind (or with that exact ID), nil if none.
func (p *StationPlan) Module(kindOrID string) *PlanModule {
	for i := range p.Modules {
		if p.Modules[i].ID == kindOrID {
			return &p.Modules[i]
		}
	}
	for i := range p.Modules {
		if p.Modules[i].Kind == kindOrID {
			return &p.Modules[i]
		}
	}
	return nil
}

// AddModule appends a module of kind with the next free ID and returns it.
func (p *StationPlan) AddModule(kind string) *PlanModule {
	n := 0
	for _, m := range p.Modules {
		if m.Kind == kind {
			n++
		}
	}
	id := kind
	if n > 0 {
		id = fmt.Sprintf("%s:%d", kind, n+1)
	}
	p.Modules = append(p.Modules, PlanModule{ID: id, Kind: kind})
	return &p.Modules[len(p.Modules)-1]
}

// ToParam is the plan as a JSON-shaped value for TerrainParams.
func (p *StationPlan) ToParam() map[string]any {
	b, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// ParseStationPlan reads a plan from TerrainParams (nil if absent or bad).
func ParseStationPlan(params map[string]any) *StationPlan {
	v, ok := params[StationPlanParam]
	if !ok || v == nil {
		return nil
	}
	if p, ok := v.(*StationPlan); ok {
		return p
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var p StationPlan
	if json.Unmarshal(b, &p) != nil || len(p.Modules) == 0 {
		return nil
	}
	return &p
}

// DefaultStationPlan is the plan a station without a history gets: a crewed
// habitat-like station, rolled from the primer's seed.
func DefaultStationPlan(seed int64, decks int) *StationPlan {
	rng := rand.New(rand.NewSource(seed ^ 0x53746e)) // "Stn"
	frames := []string{FrameSpine, FrameRing}
	p := &StationPlan{Purpose: "habitat", Frame: frames[rng.Intn(len(frames))], Decks: decks}
	for _, k := range []string{"dock", "generator", "bridge", "quarters", "botany", "cargo"} {
		p.AddModule(k)
	}
	for _, k := range []string{"lab", "medbay", "cargo", "quarters", "botany", "reactor", "archive"} {
		if rng.Intn(100) < 50 {
			p.AddModule(k)
		}
	}
	return p
}

// SortedModuleIDs lists a plan's module IDs, sorted.
func (p *StationPlan) SortedModuleIDs() []string {
	ids := make([]string, 0, len(p.Modules))
	for _, m := range p.Modules {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	return ids
}
