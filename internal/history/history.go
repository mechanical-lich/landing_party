package history

import (
	"fmt"
	"sort"
)

// History is a campaign's generated past plus the record of play. Text is never
// stored: every event is typed data, rendered from its type's template when
// shown, so wording can change without touching saves.
type History struct {
	// Seed is the campaign seed the prehistory is derived from. Every roll draws
	// from streams keyed by it (never from the campaign's own rng), so adding
	// history moves none of the campaign's other rolls.
	Seed int64 `json:"seed"`
	// PoolRolled is set once the campaign's civ pool has been rolled.
	PoolRolled bool `json:"pool_rolled,omitempty"`
	// Pool is the campaign's starfaring civs, rolled once (1 to 10 of them).
	// Civs also holds natives invented on a single site.
	Pool []string `json:"pool,omitempty"`
	// Civs are the alien civilizations of the region, alive, gone or fled.
	Civs map[string]*Civ `json:"civs,omitempty"`
	// Figures are named people and creatures, created when an event needs one.
	Figures map[string]*Figure `json:"figures,omitempty"`
	// Sites are per-planet histories, keyed by SiteHistory.ID.
	Sites map[string]*SiteHistory `json:"sites,omitempty"`
	// Events is append-only, ordered by ID.
	Events []Event `json:"events,omitempty"`
	// Known holds the IDs of events the player has discovered.
	Known map[string]bool `json:"known,omitempty"`
	// NextID numbers events, figures and sites.
	NextID int `json:"next_id,omitempty"`
	// SiteSeq counts site histories built, so each binds its own stream.
	SiteSeq int `json:"site_seq,omitempty"`
}

// Civ fates.
const (
	FateExtant  = "extant"
	FateExtinct = "extinct"
	FateFled    = "fled"
)

// Civ is one alien civilization. Humans are new to the region; every civ in the
// prehistory is alien.
type Civ struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Fate is "extant", "extinct" or "fled", decided by its events.
	Fate string `json:"fate"`
	// Species is the blueprint family it maps to today ("greyish", "mutant",
	// "yeti", ...), or "" for a people with no living descendants in the game.
	Species string `json:"species,omitempty"`
	// Ancestor is the Civ this one descends from, if any.
	Ancestor string `json:"ancestor,omitempty"`
	// Tech is the highest level the civ reached (TechLevels): spacefaring for
	// the pool, primitive to atomic for a native people.
	Tech string `json:"tech,omitempty"`
	// Sites lists the site histories this civ appears in, in order.
	Sites []string `json:"sites,omitempty"`
}

// Figure is a named person or creature an event needed.
type Figure struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
	Civ  string `json:"civ,omitempty"`
	// Fate is how the figure left the story ("died", "fled", ...), "" if alive.
	Fate string `json:"fate,omitempty"`
}

// Event is one dated happening. Its type keys data/history/events.json, which
// holds its requirements, effects, evidence and text template.
type Event struct {
	ID   string   `json:"id"`
	Date StarDate `json:"date"` // negative for prehistory
	Type string   `json:"type"`
	// Site is the SiteHistory it happened at, or "" off-world.
	Site    string   `json:"site,omitempty"`
	Civs    []string `json:"civs,omitempty"`
	Figures []string `json:"figures,omitempty"`
	// Causes are event IDs this one follows from.
	Causes []string `json:"causes,omitempty"`
	// Origin is where on the planet it happened, if anywhere.
	Origin *SitePoint `json:"origin,omitempty"`
	// Vars are template fills ("pathogen", "weapon", ...).
	Vars map[string]string `json:"vars,omitempty"`
}

// SiteHistory is the history of one planet, ending in its map's present.
type SiteHistory struct {
	ID string `json:"id"`
	// Location is the campaign Location it is bound to once charted; "" while
	// unplaced.
	Location string `json:"location,omitempty"`
	// Map is the map whose present the history ends in.
	Map string `json:"map"`
	// Seed is the site's own seed (the Location's), which its origins and
	// evidence are rolled from.
	Seed int64 `json:"seed"`
	// Arc is the story shape the site was built from.
	Arc string `json:"arc"`
	// Layer is the older people's arc run before Arc, "" for none.
	Layer string      `json:"layer,omitempty"`
	State PlanetState `json:"state"`
	// Events are this site's event IDs, oldest first. The last is the chapter
	// when Chapter is set.
	Events []string `json:"events,omitempty"`
	// Chapter is the threat event that ends the history, "" for the quiet
	// outcome.
	Chapter string `json:"chapter,omitempty"`
	// Scenario is what the site plays as: the chapter's scenario, or the map's
	// base_scenario.
	Scenario string `json:"scenario,omitempty"`
	// Tier is how dangerous the present is: "threat", "hazard" or "quiet"
	// (see TierOf for saves from before tiers).
	Tier string `json:"tier,omitempty"`
	// Civ is the site's civ for summaries: its last resident, else the first
	// civ seen there; "" if none.
	Civ string `json:"civ,omitempty"`
	// Outcomes is how each civ that lived here left the story: "remained",
	// "died" or "fled". Civ fates are derived from these across all sites.
	Outcomes map[string]string `json:"outcomes,omitempty"`
}

// TierOf is a site's tier: its saved Tier, else (a save from before tiers)
// derived from its chapter's type, quiet without one. "" for a nil site.
func (h *History) TierOf(lib *Library, site *SiteHistory) string {
	if site == nil {
		return ""
	}
	if site.Tier != "" {
		return site.Tier
	}
	if site.Chapter != "" && lib != nil {
		if ev := h.Event(site.Chapter); ev != nil {
			if def := lib.Events[ev.Type]; def != nil && def.ChapterTier() != "" {
				return def.ChapterTier()
			}
		}
	}
	return TierQuiet
}

// Happenings counts a site's events as the chronicle tells them: the two
// sides of a per-side event are one happening.
func (h *History) Happenings(lib *Library, site *SiteHistory) int {
	n := 0
	prev := ""
	for _, id := range site.Events {
		ev := h.Event(id)
		if ev == nil {
			continue
		}
		if def := lib.Events[ev.Type]; def != nil && def.PerSide && ev.Type == prev {
			prev = ""
			continue
		}
		prev = ev.Type
		n++
	}
	return n
}

// SitePoint is a tile on the surface, as rolled; generation resolves it to the
// nearest open tile once, when the site is built.
type SitePoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Planet state values.
const (
	LifeNone   = "none"
	LifeNative = "native"
	LifeAlien  = "alien"

	PopNever        = "never"
	PopOutpost      = "outpost"
	PopColony       = "colony"
	PopCivilization = "civilization"

	StatusUntouched = "untouched"
	StatusThriving  = "thriving"
	StatusFallen    = "fallen"
	StatusDead      = "dead"
)

// PlanetState is what events read and write; generation reads the final value.
type PlanetState struct {
	Life       string `json:"life"`       // none, native, alien
	Population string `json:"population"` // never, outpost, colony, civilization
	Status     string `json:"status"`     // untouched, thriving, fallen, dead
	// Contaminants is a set of plague, hive, nanites, radiation, blight.
	Contaminants []string `json:"contaminants,omitempty"`
	// Remains is a set of ruins, wrecks, graves, relics, reactor.
	Remains []string `json:"remains,omitempty"`
}

// InitialState is a planet before its history: nothing has happened yet.
func InitialState() PlanetState {
	return PlanetState{Life: LifeNone, Population: PopNever, Status: StatusUntouched}
}

// Has reports whether set holds v.
func Has(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

// addTo adds v to a sorted set.
func addTo(set []string, v string) []string {
	if Has(set, v) {
		return set
	}
	set = append(set, v)
	sort.Strings(set)
	return set
}

// removeFrom drops v from a set.
func removeFrom(set []string, v string) []string {
	out := set[:0]
	for _, s := range set {
		if s != v {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// New is an empty history for a campaign seed. Nothing is rolled until the
// first site is bound.
func New(seed int64) *History {
	return &History{Seed: seed}
}

// Event returns the event with id, or nil.
func (h *History) Event(id string) *Event {
	// Events are ordered by ID number; IDs are "ev<N>" with N = index+1.
	for i := range h.Events {
		if h.Events[i].ID == id {
			return &h.Events[i]
		}
	}
	return nil
}

// Record appends a live event (play, not prehistory): dated in the star date,
// at a site history ("" for a place with none, named by vars["place"]), with
// template fills in vars and any figures it names. Live events are known the
// moment they happen. Returns the event's ID.
func (h *History) Record(typ string, date StarDate, site string, vars map[string]string, figs ...Figure) string {
	if h.Figures == nil {
		h.Figures = map[string]*Figure{}
	}
	ev := Event{ID: fmt.Sprintf("ev%d", h.NextID+1), Date: date, Type: typ, Site: site}
	h.NextID++
	if len(vars) > 0 {
		ev.Vars = map[string]string{}
		for k, v := range vars {
			ev.Vars[k] = v
		}
	}
	for _, f := range figs {
		f.ID = fmt.Sprintf("fig%d", len(h.Figures)+1)
		fc := f
		h.Figures[f.ID] = &fc
		ev.Figures = append(ev.Figures, f.ID)
	}
	if s := h.Sites[site]; s != nil {
		s.Events = append(s.Events, ev.ID)
	}
	h.Events = append(h.Events, ev)
	h.MarkKnown(ev.ID)
	return ev.ID
}

// MarkKnown records that the player has discovered an event; it reports
// whether that is news.
func (h *History) MarkKnown(id string) bool {
	if id == "" || h.Event(id) == nil {
		return false
	}
	if h.Known == nil {
		h.Known = map[string]bool{}
	}
	if h.Known[id] {
		return false
	}
	h.Known[id] = true
	return true
}
