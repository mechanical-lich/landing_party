package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// DataDir and MapsDir are where DefaultLibrary reads the history library and
// the map presents from. Overridable in tests.
var (
	DataDir = "data/history"
	MapsDir = "data/maps"
)

var (
	defaultMu  sync.Mutex
	defaultLib *Library
)

// DefaultLibrary loads the history library and map presents once and caches
// them; campaign charting binds every site against it. A failed load is not
// cached, so a later call can retry.
func DefaultLibrary() (*Library, error) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultLib != nil {
		return defaultLib, nil
	}
	maps, err := mapdef.ReadDir(MapsDir)
	if err != nil {
		return nil, err
	}
	lib, err := LoadLibrary(DataDir, maps)
	if err != nil {
		return nil, err
	}
	defaultLib = lib
	return lib, nil
}

// SetDefaultLibrary replaces the cached library (tests point it at fixtures).
func SetDefaultLibrary(lib *Library) {
	defaultMu.Lock()
	defaultLib = lib
	defaultMu.Unlock()
}

// Library is the data the generator walks: story shapes, event types and the
// civ-pool tables. Loaded once by Load.
type Library struct {
	Arcs   map[string]*ArcDef
	Events map[string]*EventDef
	Civs   CivTable
	// Maps are the map definitions whose present blocks the arcs end in, keyed
	// by map ID.
	Maps map[string]*mapdef.MapDef

	arcOrder   []string
	eventOrder []string
}

// OneOrMany is a string list that may be written in JSON as a bare string.
type OneOrMany []string

func (o *OneOrMany) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*o = OneOrMany{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*o = many
	return nil
}

// ArcDef is a story shape: stages of candidate events walked in order.
type ArcDef struct {
	ID   string `json:"id"`
	Note string `json:"note,omitempty"`
	// Uninhabited arcs never have anyone live on the site: no resident civ, no
	// figures, no population. A quarter of sites draw one (see BindSite).
	Uninhabited bool `json:"uninhabited,omitempty"`
	// Summary is the site's one-line description on the Star Map; {civ} is
	// the site's civ (its last resident, else its first visitor).
	Summary string     `json:"summary,omitempty"`
	Stages  []StageDef `json:"stages"`
	// Ends is the state the arc must finish in (any of each listed value),
	// checked together with the map's present.
	Ends Match `json:"ends"`
}

// StageDef is one step of an arc: one event type from Pick whose requirements
// hold. A required stage with no eligible event fails the attempt.
type StageDef struct {
	Pick     []string `json:"pick"`
	Optional bool     `json:"optional,omitempty"`
}

// Match tests a planet state. Life, Population and Status are any-of; Remains
// and Contaminants are all-of. Empty fields match anything.
type Match struct {
	Life         OneOrMany `json:"life,omitempty"`
	Population   OneOrMany `json:"population,omitempty"`
	Status       OneOrMany `json:"status,omitempty"`
	Remains      []string  `json:"remains,omitempty"`
	Contaminants []string  `json:"contaminants,omitempty"`
}

// Holds reports whether st matches.
func (m Match) Holds(st PlanetState) bool {
	if len(m.Life) > 0 && !Has(m.Life, st.Life) {
		return false
	}
	if len(m.Population) > 0 && !Has(m.Population, st.Population) {
		return false
	}
	if len(m.Status) > 0 && !Has(m.Status, st.Status) {
		return false
	}
	for _, r := range m.Remains {
		if !Has(st.Remains, r) {
			return false
		}
	}
	for _, c := range m.Contaminants {
		if !Has(st.Contaminants, c) {
			return false
		}
	}
	return true
}

// Requires is an event type's precondition on the site and the campaign.
type Requires struct {
	Match
	// Events must all already have happened on the site (by type).
	Events []string `json:"events,omitempty"`
	// AnyEvents needs at least one of these on the site.
	AnyEvents []string `json:"any_events,omitempty"`
	// Resident, when set, requires a civ to live on the site (true) or nobody
	// (false).
	Resident *bool `json:"resident,omitempty"`
}

// Effects is what an event does to the planet and its resident civ.
type Effects struct {
	Life               string   `json:"life,omitempty"`
	Population         string   `json:"population,omitempty"`
	Status             string   `json:"status,omitempty"`
	AddContaminants    []string `json:"add_contaminants,omitempty"`
	RemoveContaminants []string `json:"remove_contaminants,omitempty"`
	AddRemains         []string `json:"add_remains,omitempty"`
	// RuinsIfPopulated adds "ruins" when someone lived here (a flare scorching
	// an inhabited world).
	RuinsIfPopulated bool `json:"ruins_if_populated,omitempty"`
	// ResidentFate ends the resident civ's story on this site: "died", "fled"
	// or "dwindled" (still here, diminished). The civ's Fate is derived from
	// these outcomes across every site.
	ResidentFate string `json:"resident_fate,omitempty"`
}

// FigureDef asks for a named figure in an event, by role.
type FigureDef struct {
	Role string `json:"role"`
	Fate string `json:"fate,omitempty"`
}

// Civ roles an event can cast.
const (
	CivNone     = ""
	CivResident = "resident" // whoever lives here
	CivArrive   = "arrive"   // a pool civ settles: becomes the resident
	CivNative   = "native"   // a new civ arises here: invented, becomes the resident
	CivVisitor  = "visitor"  // a pool civ passes through
	CivWar      = "war"      // two pool civs at war: {civ} and {enemy}
)

// Evidence is one generation input an event leaves on the map: an ordinary
// feature block; "at": "origin" anchors it at this event's resolved origin.
type Evidence struct {
	mapdef.FeatureBlock
}

// EventDef is an event type from events.json.
type EventDef struct {
	ID       string   `json:"id"`
	Requires Requires `json:"requires"`
	Effects  Effects  `json:"effects"`
	// Civ casts the event's civ (see the Civ* roles).
	Civ string `json:"civ,omitempty"`
	// Species, with Civ "native", is what the invented civ became; with a pool
	// role it is a preference ("greyish" salvagers).
	Species string      `json:"species,omitempty"`
	Figures []FigureDef `json:"figures,omitempty"`
	// Origin gives the event a SitePoint, rolled from the site's stream.
	Origin bool `json:"origin,omitempty"`
	// OriginFrom reuses, and claims, the origin of an earlier event of one of
	// these types on the site, the first type found winning (the wreck_*
	// threats grow from the ancient wreck).
	OriginFrom OneOrMany `json:"origin_from,omitempty"`
	// Scenario makes this a chapter: the gameplay package the site plays as
	// when its history ends here.
	Scenario string `json:"scenario,omitempty"`
	// Weight is per map: how often this chapter ends a history there. No
	// weight for a map means it can't end a history there.
	Weight map[string]int `json:"weight,omitempty"`
	// Prelude are event types that happen just before this chapter (the
	// trigger that opens an old wreck).
	Prelude []string `json:"prelude,omitempty"`
	// GapDays is the time from this event to the next one on the site, or to
	// the present for the last, rolled in [min, max]. Default: a few decades.
	// Long epochs (a biosphere blooming, an old wreck) set long gaps; chapters
	// set days, so the threat is recent.
	GapDays []int `json:"gap_days,omitempty"`
	// Vars are template fills picked at random: {"how": ["...", "..."]}.
	Vars     map[string][]string `json:"vars,omitempty"`
	Evidence []Evidence          `json:"evidence,omitempty"`
	// SafeStart marks a chapter calm enough for a first landing: a site within
	// one jump of the Wreck Site only ends in these (or a quiet_safe quiet).
	SafeStart bool `json:"safe_start,omitempty"`
	// Place names where the event left its trace, for quest lines ("the
	// vault", "the reactor").
	Place string `json:"place,omitempty"`
	// Live marks an event type the recorder writes during play (a colonist's
	// death, a quest completed), never part of an arc or a chapter. Fills are
	// the template fills it sets in the event's Vars when recorded.
	Live  bool     `json:"live,omitempty"`
	Fills []string `json:"fills,omitempty"`
	// Flags, on a chapter, are extra lore flags for the site's build and
	// setup scripts (a scout crash leaves no crew: chapter_crew = "dead").
	Flags map[string]any `json:"flags,omitempty"`
	// Omen, on a chapter, is the line the Star Map adds after the arc's
	// summary: what is going on there now.
	Omen string `json:"omen,omitempty"`
	// Text is the chronicle template: {date}, {site}, {civ}, {enemy}, each
	// figure's {role}, and each var.
	Text string `json:"text"`
}

// IsChapter reports whether the event can end a history as its threat chapter.
func (e *EventDef) IsChapter() bool { return e.Scenario != "" }

// CivTable is civs.json: what the civ pool is rolled from.
type CivTable struct {
	PoolMin int `json:"pool_min"`
	PoolMax int `json:"pool_max"`
	// Syllables build civ names: one from each list in turn, the middle one
	// optional.
	NameStart []string `json:"name_start"`
	NameMid   []string `json:"name_mid"`
	NameEnd   []string `json:"name_end"`
	// FigureStart/FigureEnd build figure names.
	FigureStart []string `json:"figure_start"`
	FigureEnd   []string `json:"figure_end"`
	// Species weighs what a pool civ maps to today; "" is a people with no
	// descendants in the game.
	Species map[string]int `json:"species"`
	// AncestorChance is the percent chance a pool civ descends from an earlier
	// one in the pool.
	AncestorChance int `json:"ancestor_chance"`
	// WarChance is the percent chance each pair of pool civs went to war.
	WarChance int `json:"war_chance"`
	// LifelessPercent is the share of sites that draw an uninhabited arc.
	LifelessPercent int `json:"lifeless_percent"`
	// Reuse weighs a pool civ already in the chronicle against a fresh one:
	// weight = 1 + Reuse * (sites it already appears on).
	Reuse int `json:"reuse"`
}

type arcsFile struct {
	Arcs []*ArcDef `json:"arcs"`
}

type eventsFile struct {
	Events []*EventDef `json:"events"`
}

// LoadLibrary reads arcs.json, events.json and civs.json from dir and takes the
// present blocks from maps (normally mapdef.All()).
func LoadLibrary(dir string, maps []mapdef.MapDef) (*Library, error) {
	lib := &Library{
		Arcs:   map[string]*ArcDef{},
		Events: map[string]*EventDef{},
		Maps:   map[string]*mapdef.MapDef{},
	}
	var af arcsFile
	if err := readJSON(filepath.Join(dir, "arcs.json"), &af); err != nil {
		return nil, err
	}
	for _, a := range af.Arcs {
		if _, dup := lib.Arcs[a.ID]; dup {
			return nil, fmt.Errorf("history: duplicate arc %q", a.ID)
		}
		lib.Arcs[a.ID] = a
		lib.arcOrder = append(lib.arcOrder, a.ID)
	}
	var ef eventsFile
	if err := readJSON(filepath.Join(dir, "events.json"), &ef); err != nil {
		return nil, err
	}
	for _, e := range ef.Events {
		if _, dup := lib.Events[e.ID]; dup {
			return nil, fmt.Errorf("history: duplicate event type %q", e.ID)
		}
		lib.Events[e.ID] = e
		lib.eventOrder = append(lib.eventOrder, e.ID)
	}
	if err := readJSON(filepath.Join(dir, "civs.json"), &lib.Civs); err != nil {
		return nil, err
	}
	for i := range maps {
		m := maps[i]
		lib.Maps[m.ID] = &m
	}
	return lib, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("history: %s: %w", path, err)
	}
	return nil
}

// ArcIDs and EventIDs list the library in file order (stable for iteration).
func (l *Library) ArcIDs() []string   { return l.arcOrder }
func (l *Library) EventIDs() []string { return l.eventOrder }

// ChapterEvents lists the chapter event types in file order.
func (l *Library) ChapterEvents() []*EventDef {
	var out []*EventDef
	for _, id := range l.eventOrder {
		if e := l.Events[id]; e.IsChapter() {
			out = append(out, e)
		}
	}
	return out
}
