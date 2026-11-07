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
	// TierTags are the Star Map's tag per tier (tiers.json).
	TierTags map[string]TierTag
	// TechDefs are the tech levels in order (tech.json).
	TechDefs []TechDef
	// Purposes and ShipClasses are purposes.json.
	Purposes    map[string]PurposeDef
	ShipClasses map[string]ShipClass
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
	// Layer arcs are an older people's story run before a site's main arc
	// (Present.Layers): short, ending with its resident gone, leaving ruins
	// the main arc's people may build over.
	Layer bool `json:"layer,omitempty"`
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
	// Chance is the percent chance an optional stage happens (default 50).
	Chance int `json:"chance,omitempty"`
	// Repeat [min, max] picks that many events, each re-checked for
	// eligibility; a type already picked in the stage repeats only if it is
	// repeatable. Default one.
	Repeat []int `json:"repeat,omitempty"`
	// Epoch marks a stage that sets the world up (a biosphere): skipped when
	// an older layer already did.
	Epoch bool `json:"epoch,omitempty"`
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
	// TechMin and TechMax bound the acting civ's tech level on the site (a
	// pool civ is spacefaring; a new native civ is not checked).
	TechMin string `json:"tech_min,omitempty"`
	TechMax string `json:"tech_max,omitempty"`
	// Rival, when set, requires a rival people on the site (true) or none.
	Rival *bool `json:"rival,omitempty"`
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
	// Tech changes the acting native civ's level: "+1", "-1", "-2" or a
	// level name. Pool civs stay spacefaring.
	Tech string `json:"tech,omitempty"`
	// RivalFate ends the rival's story on this site, as ResidentFate does
	// the resident's.
	RivalFate string `json:"rival_fate,omitempty"`
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
	// Two peoples on one site (the rival slot beside the resident):
	CivTwins        = "twins"         // two native peoples arise at once: resident and rival
	CivRivalNative  = "rival_native"  // a native people splits off as the rival: {civ} new, {enemy} the resident
	CivRivalVisitor = "rival_visitor" // a pool civ occupies the site as the rival (an overlord): {civ} it, {enemy} the resident
	CivRivals       = "rivals"        // {civ} the resident (or the side leading a per-side event), {enemy} the rival
	CivRival        = "rival"         // {civ} the rival, {enemy} the resident
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
	// OriginAt rolls the origin where two peoples live: "side" in the
	// leading civ's half of the map, "border" in the band between them.
	OriginAt string `json:"origin_at,omitempty"`
	// PerSide emits the event once led by each of the two peoples (resident
	// and rival), each with its own figures, origin and level; the second
	// follows from the first. Its planet effects apply once.
	PerSide bool `json:"per_side,omitempty"`
	// TextSide, on a per-side event, is the leading people's own account,
	// read on its datapad ({civ} is "us"); the chronicle keeps Text.
	TextSide string `json:"text_side,omitempty"`
	// TextMeanwhile, on a per-side event, is the chronicle line for the second
	// people's half, which follows the first ("meanwhile, across the
	// border..."), so the chronicle does not tell one happening twice.
	TextMeanwhile string `json:"text_meanwhile,omitempty"`
	// OriginFrom reuses, and claims, the origin of an earlier event of one of
	// these types on the site, the first type found winning (the wreck_*
	// threats grow from the ancient wreck). With Origin also set it is a
	// preference: without such an event the event rolls its own origin.
	OriginFrom OneOrMany `json:"origin_from,omitempty"`
	// Scenario makes this a chapter: the gameplay package the site plays as
	// when its history ends here.
	Scenario string `json:"scenario,omitempty"`
	// Tier is the chapter's weight class: "threat" (a full scenario; the
	// default for an event with a Scenario) or "hazard" (a local, finite danger
	// that plays the map's base_scenario, or a gentle Scenario of its own, with
	// the event's evidence on top). An event with a hazard tier is a chapter
	// even without a Scenario.
	Tier string `json:"tier,omitempty"`
	// Weight is per map: how often this chapter ends a history there, relative
	// to the other chapters of its tier. No weight for a map means it can't end
	// a history there.
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
	// SafeStart marks a hazard calm enough for a first landing: a site within
	// one jump of the Wreck Site rolls its tier by the map's safe_tiers (never
	// threat), and a hazard there must be safe_start.
	SafeStart bool `json:"safe_start,omitempty"`
	// KeepOut is how far (tiles from the landing reserve box) this event's
	// origin stays; 0 takes the budget's default (hazard_keep_out for hazards,
	// threats and anything that spawns hostiles, else keep_out).
	KeepOut int `json:"keep_out,omitempty"`
	// Place names where the event left its trace, for quest lines ("the
	// vault", "the reactor").
	Place string `json:"place,omitempty"`
	// Interlude marks a small happening (a figure born, a festival, a comet)
	// that walk may add after any stage (Present.Density); it never stamps.
	Interlude bool `json:"interlude,omitempty"`
	// Repeatable lets an event type follow itself on a site and be picked
	// more than once in a repeating stage (battles, interludes).
	Repeatable bool `json:"repeatable,omitempty"`
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

// Chapter tiers: how dangerous the site's present is. Quiet is the outcome
// with no chapter.
const (
	TierThreat = "threat"
	TierHazard = "hazard"
	TierQuiet  = "quiet"
)

// Tiers lists the tiers in roll order.
var Tiers = []string{TierThreat, TierHazard, TierQuiet}

// IsChapter reports whether the event can end a history: it carries a
// scenario or is a hazard.
func (e *EventDef) IsChapter() bool { return e.Scenario != "" || e.Tier == TierHazard }

// ChapterTier is the chapter's tier: its Tier, else threat for an event with a
// scenario; "" for an event that is not a chapter.
func (e *EventDef) ChapterTier() string {
	switch {
	case e.Tier != "":
		return e.Tier
	case e.Scenario != "":
		return TierThreat
	}
	return ""
}

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
	// NativeTech weighs a native civ's starting level.
	NativeTech map[string]int `json:"native_tech"`
	// SpeciesTechMax caps a native species' level ("yeti": "medieval").
	SpeciesTechMax map[string]string `json:"species_tech_max"`
}

// TierTag is how the Star Map shows a tier: a word and its colour.
type TierTag struct {
	Label string   `json:"label"`
	Color [3]uint8 `json:"color"`
}

type tiersFile struct {
	Tiers map[string]TierTag `json:"tiers"`
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
	var tf tiersFile
	if err := readJSON(filepath.Join(dir, "tiers.json"), &tf); err != nil {
		return nil, err
	}
	lib.TierTags = tf.Tiers
	var tech techFile
	if err := readJSON(filepath.Join(dir, "tech.json"), &tech); err != nil {
		return nil, err
	}
	lib.TechDefs = tech.Levels
	var pf purposesFile
	if err := readJSON(filepath.Join(dir, "purposes.json"), &pf); err != nil {
		return nil, err
	}
	lib.Purposes, lib.ShipClasses = pf.Purposes, pf.ShipClasses
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

// Interludes lists the interlude event types in file order.
func (l *Library) Interludes() []*EventDef {
	var out []*EventDef
	for _, id := range l.eventOrder {
		if e := l.Events[id]; e.Interlude {
			out = append(out, e)
		}
	}
	return out
}

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
