package history

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// History randomness draws from its own streams, salted away from the
// campaign's, so adding history moves none of the existing campaign rolls.
const (
	historySalt int64 = 0x4c6f7265486973 // "LoreHis"
	siteStreamK int64 = 2654435761
)

// Outcomes a civ can have on a site.
const (
	OutcomeRemained = "remained"
	OutcomeDied     = "died"
	OutcomeFled     = "fled"
)

// WarEventType is the off-world event the pool's wars are recorded as.
const WarEventType = "war_begins"

// ErrNoHistory is returned for a map history skips (no present block).
var ErrNoHistory = errors.New("history: map has no present")

// maxArcAttempts bounds the retries for one site before giving up on its arcs.
const maxArcAttempts = 60

func (h *History) poolRng() *rand.Rand {
	return rand.New(rand.NewSource(h.Seed ^ historySalt))
}

func (h *History) siteRng(seed int64) *rand.Rand {
	return rand.New(rand.NewSource((h.Seed ^ historySalt) + seed*siteStreamK))
}

func (h *History) nextEventID() string {
	h.NextID++
	return fmt.Sprintf("ev%d", h.NextID)
}

// RollPool rolls the campaign's civ pool and its wars, once. BindSite calls it.
func (h *History) RollPool(lib *Library) {
	if h.PoolRolled {
		return
	}
	h.PoolRolled = true
	if h.Civs == nil {
		h.Civs = map[string]*Civ{}
	}
	ct := lib.Civs
	rng := h.poolRng()
	lo, hi := ct.PoolMin, ct.PoolMax
	if lo < 1 {
		lo = 1
	}
	if hi < lo {
		hi = lo
	}
	n := lo + rng.Intn(hi-lo+1)
	species := sortedKeys(ct.Species)
	for i := 0; i < n; i++ {
		c := &Civ{
			ID:      fmt.Sprintf("civ%d", len(h.Civs)+1),
			Name:    h.civName(ct, rng, nil),
			Fate:    FateExtant,
			Species: weightedPick(rng, species, ct.Species),
		}
		if i > 0 && rng.Intn(100) < ct.AncestorChance {
			c.Ancestor = h.Pool[rng.Intn(len(h.Pool))]
		}
		h.Civs[c.ID] = c
		h.Pool = append(h.Pool, c.ID)
	}
	for i := 0; i < len(h.Pool); i++ {
		for j := i + 1; j < len(h.Pool); j++ {
			if rng.Intn(100) >= ct.WarChance {
				continue
			}
			a, b := h.Pool[i], h.Pool[j]
			if rng.Intn(2) == 0 {
				a, b = b, a
			}
			start := StarDate(-(100 + rng.Intn(300)) * 365)
			ev := Event{ID: h.nextEventID(), Date: start, Type: WarEventType, Civs: []string{a, b}}
			// Some wars still run; the rest lasted a few decades.
			if rng.Intn(100) >= ongoingWarPercent {
				end := start + StarDate((10+rng.Intn(70))*365)
				if end > -1 {
					end = -1
				}
				ev.Vars = map[string]string{warEndedVar: strconv.Itoa(int(end))}
			}
			h.Events = append(h.Events, ev)
		}
	}
}

// ongoingWarPercent is the share of the pool's wars still being fought when
// the campaign begins; warEndedVar holds a finished war's last star date.
const (
	ongoingWarPercent = 40
	warEndedVar       = "ended"
)

// wars returns the pool's wars (Civs = [aggressor, enemy]); ongoingOnly keeps
// the ones still being fought.
func (h *History) wars(ongoingOnly bool) []Event {
	var out []Event
	for _, e := range h.Events {
		if e.Type == WarEventType && e.Site == "" && len(e.Civs) == 2 {
			if ongoingOnly && e.Vars[warEndedVar] != "" {
				continue
			}
			out = append(out, e)
		}
	}
	return out
}

// WarEnd is the last star date of a war event; 0 for one still being fought.
func WarEnd(e *Event) StarDate {
	if v, err := strconv.Atoi(e.Vars[warEndedVar]); err == nil {
		return StarDate(v)
	}
	return 0
}

// WarAt returns the war civ was fighting on date, or nil.
func (h *History) WarAt(civ string, date StarDate) *Event {
	for i := range h.Events {
		e := &h.Events[i]
		if e.Type == WarEventType && e.Site == "" && Has(e.Civs, civ) && e.Date <= date && date <= WarEnd(e) {
			return e
		}
	}
	return nil
}

// civName rolls a civ name unused by any known civ or the extra names.
func (h *History) civName(ct CivTable, rng *rand.Rand, extra []string) string {
	pick := func(l []string) string {
		if len(l) == 0 {
			return ""
		}
		return l[rng.Intn(len(l))]
	}
	var name string
	for try := 0; try < 50; try++ {
		name = pick(ct.NameStart) + pick(ct.NameMid) + pick(ct.NameEnd)
		if name != "" && !h.civNameTaken(name, extra) {
			return name
		}
	}
	return fmt.Sprintf("%s %d", name, len(h.Civs)+1)
}

func (h *History) civNameTaken(name string, extra []string) bool {
	for _, c := range h.Civs {
		if c.Name == name {
			return true
		}
	}
	return Has(extra, name)
}

func figureName(ct CivTable, rng *rand.Rand) string {
	if len(ct.FigureStart) == 0 {
		return "Unnamed"
	}
	name := ct.FigureStart[rng.Intn(len(ct.FigureStart))]
	if len(ct.FigureEnd) > 0 {
		name += ct.FigureEnd[rng.Intn(len(ct.FigureEnd))]
	}
	return name
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func weightedPick(rng *rand.Rand, keys []string, w map[string]int) string {
	total := 0
	for _, k := range keys {
		if w[k] > 0 {
			total += w[k]
		}
	}
	if total == 0 {
		return ""
	}
	r := rng.Intn(total)
	for _, k := range keys {
		if w[k] <= 0 {
			continue
		}
		if r < w[k] {
			return k
		}
		r -= w[k]
	}
	return ""
}

func rollRange(rng *rand.Rand, r []int, def [2]int) int {
	lo, hi := def[0], def[1]
	if len(r) >= 1 {
		lo, hi = r[0], r[0]
	}
	if len(r) >= 2 {
		hi = r[1]
	}
	if hi <= lo {
		return lo
	}
	return lo + rng.Intn(hi-lo+1)
}

// defaultGapDays is the time between two events when the type sets none.
var defaultGapDays = [2]int{5 * 365, 40 * 365}

// --- the site simulation ----------------------------------------------------

// draftFigure is a figure an event asked for, named but not yet committed.
type draftFigure struct {
	role, fate, name string
	civ              string
}

// draftEvent is one event of a site being built, not yet in the History.
type draftEvent struct {
	def     *EventDef
	civs    []string
	figures []int // indexes into the sim's figs
	origin  *SitePoint
	causes  []int // indexes into the draft's events
	vars    map[string]string
}

// newCiv is a native civ invented on this site; ID "new<N>" until committed.
type newCiv struct {
	id      string
	name    string
	species string
}

// siteSim walks one attempt at a site's history without touching the History.
type siteSim struct {
	h    *History
	lib  *Library
	md   *mapdef.MapDef
	rng  *rand.Rand
	seed int64
	arc  *ArcDef

	state        PlanetState
	events       []draftEvent
	resident     string
	lastResident string
	outcomes     map[string]string
	natives      []newCiv
	figs         []draftFigure
	claimed      map[int]bool
	// doneBefore holds (civ, event type) beats from sites already bound.
	doneBefore map[string]bool
	mapW, mapH int
}

func (s *siteSim) clone() *siteSim {
	c := *s
	c.state.Contaminants = append([]string(nil), s.state.Contaminants...)
	c.state.Remains = append([]string(nil), s.state.Remains...)
	c.events = append([]draftEvent(nil), s.events...)
	c.outcomes = map[string]string{}
	for k, v := range s.outcomes {
		c.outcomes[k] = v
	}
	c.natives = append([]newCiv(nil), s.natives...)
	c.figs = append([]draftFigure(nil), s.figs...)
	c.claimed = map[int]bool{}
	for k, v := range s.claimed {
		c.claimed[k] = v
	}
	return &c
}

func (s *siteSim) hasType(t string) bool {
	for _, e := range s.events {
		if e.def.ID == t {
			return true
		}
	}
	return false
}

// originSource finds an unclaimed earlier event of one of types with an origin.
func (s *siteSim) originSource(types []string) int {
	for _, t := range types {
		for i, e := range s.events {
			if e.def.ID == t && e.origin != nil && !s.claimed[i] {
				return i
			}
		}
	}
	return -1
}

// populates reports whether the event would put people on the site.
func populates(def *EventDef) bool {
	return def.Civ == CivArrive || def.Civ == CivNative || len(def.Figures) > 0 ||
		(def.Effects.Population != "" && def.Effects.Population != PopNever)
}

// eligible reports whether def's requirements hold now.
func (s *siteSim) eligible(def *EventDef) bool {
	r := def.Requires
	if !r.Match.Holds(s.state) {
		return false
	}
	for _, t := range r.Events {
		if !s.hasType(t) {
			return false
		}
	}
	if len(r.AnyEvents) > 0 {
		any := false
		for _, t := range r.AnyEvents {
			if s.hasType(t) {
				any = true
				break
			}
		}
		if !any {
			return false
		}
	}
	if r.Resident != nil && (s.resident != "") != *r.Resident {
		return false
	}
	if s.arc.Uninhabited && populates(def) {
		return false
	}
	switch def.Civ {
	case CivResident:
		if s.resident == "" {
			return false
		}
	case CivWar:
		if len(s.h.wars(def.IsChapter())) == 0 {
			return false
		}
	case CivArrive, CivVisitor:
		if len(s.h.Pool) == 0 {
			return false
		}
	}
	if len(def.OriginFrom) > 0 && s.originSource(def.OriginFrom) < 0 {
		return false
	}
	return true
}

// beat reports whether an event type is a memorable beat (it names a figure
// or a place on the map) that should not repeat for the same civ.
func beat(def *EventDef) bool {
	return len(def.Figures) > 0 || def.Origin || len(def.OriginFrom) > 0
}

func beatKey(civ, typ string) string { return civ + "\x00" + typ }

// done reports whether civ already had a typ beat in this campaign.
func (s *siteSim) done(civ, typ string) bool {
	if s.doneBefore[beatKey(civ, typ)] {
		return true
	}
	for _, e := range s.events {
		if e.def.ID == typ && len(e.civs) > 0 && e.civs[0] == civ {
			return true
		}
	}
	return false
}

// avoidRepeats narrows cands to civs that haven't had a typ beat, unless every
// one has.
func (s *siteSim) avoidRepeats(def *EventDef, cands []string) []string {
	if !beat(def) {
		return cands
	}
	var fresh []string
	for _, c := range cands {
		if !s.done(c, def.ID) {
			fresh = append(fresh, c)
		}
	}
	if len(fresh) == 0 {
		return cands
	}
	return fresh
}

// actors lists the civs that could lead def now (nil: a new or no civ).
func (s *siteSim) actors(def *EventDef) []string {
	switch def.Civ {
	case CivResident:
		return []string{s.resident}
	case CivArrive, CivVisitor:
		exclude := ""
		if def.Civ == CivVisitor {
			exclude = s.resident
		}
		var out []string
		for _, c := range s.h.Pool {
			if c != exclude || len(s.h.Pool) == 1 {
				out = append(out, c)
			}
		}
		if def.Species != "" {
			var sp []string
			for _, c := range out {
				if s.h.Civs[c].Species == def.Species {
					sp = append(sp, c)
				}
			}
			if len(sp) > 0 {
				out = sp
			}
		}
		return out
	case CivWar:
		var out []string
		for _, w := range s.h.wars(def.IsChapter()) {
			out = append(out, w.Civs[0])
		}
		return out
	case CivNone:
		if c := s.inheritedCiv(def, s.originSource(def.OriginFrom)); c != "" {
			return []string{c}
		}
	}
	return nil
}

// repeats reports whether def would repeat a beat for every civ that could
// lead it.
func (s *siteSim) repeats(def *EventDef) bool {
	if !beat(def) {
		return false
	}
	cands := s.actors(def)
	if len(cands) == 0 {
		return false
	}
	for _, c := range cands {
		if !s.done(c, def.ID) {
			return false
		}
	}
	return true
}

// inheritedCiv is the civ a civ-less event belongs to when its text names one:
// whoever its origin came from, else whoever lives (or last lived) here, so the
// chronicle stays connected.
func (s *siteSim) inheritedCiv(def *EventDef, src int) string {
	if !strings.Contains(def.Text, "{civ}") {
		return ""
	}
	switch {
	case src >= 0 && len(s.events[src].civs) > 0:
		return s.events[src].civs[0]
	case s.resident != "":
		return s.resident
	}
	return s.lastResident
}

// apply records def as the next event and applies its effects.
func (s *siteSim) apply(def *EventDef) {
	d := draftEvent{def: def}
	prev := len(s.events) - 1
	// Origin: reuse and claim an earlier event's, or roll a fresh one.
	src := -1
	if len(def.OriginFrom) > 0 {
		src = s.originSource(def.OriginFrom)
	}
	if src >= 0 {
		s.claimed[src] = true
		p := *s.events[src].origin
		d.origin = &p
		d.causes = append(d.causes, src)
	} else if def.Origin {
		d.origin = s.rollPoint()
	}
	if prev >= 0 && prev != src {
		d.causes = append(d.causes, prev)
	}
	// Cast the civs.
	switch def.Civ {
	case CivResident:
		d.civs = []string{s.resident}
	case CivArrive, CivVisitor:
		w := map[string]int{}
		cands := s.avoidRepeats(def, s.actors(def))
		for _, id := range cands {
			w[id] = 1 + s.lib.Civs.Reuse*len(s.h.Civs[id].Sites)
		}
		id := weightedPick(s.rng, cands, w)
		d.civs = []string{id}
		if def.Civ == CivArrive {
			s.resident = id
		}
	case CivNative:
		var taken []string
		for _, n := range s.natives {
			taken = append(taken, n.name)
		}
		n := newCiv{id: fmt.Sprintf("new%d", len(s.natives)), name: s.h.civName(s.lib.Civs, s.rng, taken), species: def.Species}
		s.natives = append(s.natives, n)
		d.civs = []string{n.id}
		s.resident = n.id
	case CivWar:
		wars := s.h.wars(def.IsChapter())
		if beat(def) {
			var fresh []Event
			for _, w := range wars {
				if !s.done(w.Civs[0], def.ID) {
					fresh = append(fresh, w)
				}
			}
			if len(fresh) > 0 {
				wars = fresh
			}
		}
		w := wars[s.rng.Intn(len(wars))]
		d.civs = append([]string(nil), w.Civs...)
	default:
		if c := s.inheritedCiv(def, src); c != "" {
			d.civs = []string{c}
		}
	}
	if s.resident != "" {
		s.lastResident = s.resident
	}
	// Figures belong to the event's civ. A living figure of the same role and
	// civ already on this site is reused (the commander who built the station
	// sends its last message), else one is created.
	for _, f := range def.Figures {
		civ := ""
		if len(d.civs) > 0 {
			civ = d.civs[0]
		}
		idx := -1
		for i, old := range s.figs {
			if old.role == f.Role && old.civ == civ && old.fate == "" {
				idx = i
				break
			}
		}
		if idx < 0 {
			s.figs = append(s.figs, draftFigure{role: f.Role, name: figureName(s.lib.Civs, s.rng), civ: civ})
			idx = len(s.figs) - 1
		}
		if f.Fate != "" {
			s.figs[idx].fate = f.Fate
		}
		d.figures = append(d.figures, idx)
	}
	// Template vars.
	for _, k := range sortedVarKeys(def.Vars) {
		if opts := def.Vars[k]; len(opts) > 0 {
			if d.vars == nil {
				d.vars = map[string]string{}
			}
			d.vars[k] = opts[s.rng.Intn(len(opts))]
		}
	}
	// Planet state.
	e := def.Effects
	if e.Life != "" {
		s.state.Life = e.Life
	}
	if e.Population != "" {
		s.state.Population = e.Population
	}
	if e.Status != "" {
		s.state.Status = e.Status
	}
	for _, c := range e.AddContaminants {
		s.state.Contaminants = addTo(s.state.Contaminants, c)
	}
	for _, c := range e.RemoveContaminants {
		s.state.Contaminants = removeFrom(s.state.Contaminants, c)
	}
	for _, r := range e.AddRemains {
		s.state.Remains = addTo(s.state.Remains, r)
	}
	if e.RuinsIfPopulated && s.state.Population != PopNever {
		s.state.Remains = addTo(s.state.Remains, "ruins")
	}
	if e.ResidentFate != "" && s.resident != "" {
		out := e.ResidentFate
		if out == "dwindled" {
			out = OutcomeRemained
		}
		s.outcomes[s.resident] = out
		s.resident = ""
	}
	s.events = append(s.events, d)
}

func sortedVarKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// rollPoint is a surface tile of the site's rolled map size, clear of the edge.
func (s *siteSim) rollPoint() *SitePoint {
	const margin = 12
	x, y := s.mapW/2, s.mapH/2
	if s.mapW > 2*margin {
		x = margin + s.rng.Intn(s.mapW-2*margin)
	}
	if s.mapH > 2*margin {
		y = margin + s.rng.Intn(s.mapH-2*margin)
	}
	return &SitePoint{X: x, Y: y}
}

// walk runs the arc's stages; false if a required stage had nothing eligible.
func (s *siteSim) walk() bool {
	for _, st := range s.arc.Stages {
		var ok []*EventDef
		for _, id := range st.Pick {
			if def := s.lib.Events[id]; def != nil && s.eligible(def) {
				ok = append(ok, def)
			}
		}
		// A beat its civ already had elsewhere yields to any other choice; an
		// optional stage with only repeats left is skipped.
		var fresh []*EventDef
		for _, def := range ok {
			if !s.repeats(def) {
				fresh = append(fresh, def)
			}
		}
		if len(fresh) > 0 {
			ok = fresh
		} else if st.Optional {
			continue
		}
		if len(ok) == 0 {
			if st.Optional {
				continue
			}
			return false
		}
		if st.Optional && s.rng.Intn(2) == 0 {
			continue
		}
		s.apply(ok[s.rng.Intn(len(ok))])
	}
	return true
}

func presentHolds(p *mapdef.Present, st PlanetState) bool {
	return (len(p.Life) == 0 || Has(p.Life, st.Life)) && (len(p.Status) == 0 || Has(p.Status, st.Status))
}

// tryChapter applies def's prelude and def to a copy of s; nil if it can't end
// this history.
func (s *siteSim) tryChapter(def *EventDef) *siteSim {
	c := s.clone()
	for _, p := range def.Prelude {
		pd := c.lib.Events[p]
		if pd == nil || !c.eligible(pd) {
			return nil
		}
		c.apply(pd)
	}
	if !c.eligible(def) {
		return nil
	}
	c.apply(def)
	if !presentHolds(c.md.Present, c.state) {
		return nil
	}
	return c
}

// chooseChapter picks the threat chapter by per-map weight among those the
// history allows, against the map's quiet weight. It returns the finished sim
// and the chapter (nil for the quiet outcome).
func (s *siteSim) chooseChapter(safe bool) (*siteSim, *EventDef) {
	type cand struct {
		def *EventDef
		sim *siteSim
		w   int
	}
	var cands []cand
	total := 0
	if q := s.md.Present.QuietWeight; q > 0 && (!safe || s.md.Present.QuietSafe) {
		cands = append(cands, cand{w: q})
		total += q
	}
	for _, def := range s.lib.ChapterEvents() {
		w := def.Weight[s.md.ID]
		if w <= 0 || s.repeats(def) || (safe && !def.SafeStart) {
			continue
		}
		// The trial runs on its own copy of the stream so rejected chapters
		// don't consume the site's rolls.
		trial := s.clone()
		trial.rng = rand.New(rand.NewSource(s.rng.Int63()))
		if sim := trial.tryChapter(def); sim != nil {
			cands = append(cands, cand{def: def, sim: sim, w: w})
			total += w
		}
	}
	if total == 0 {
		return s, nil
	}
	r := s.rng.Intn(total)
	for _, c := range cands {
		if r < c.w {
			if c.def == nil {
				return s, nil
			}
			return c.sim, c.def
		}
		r -= c.w
	}
	return s, nil
}

// BindSite builds the history of a site on mapID with the given site seed (the
// Location's Seed), ending in the map's present and a chapter, and records it.
// It is deterministic for a History's seed, the order sites are bound and
// (mapID, seed). In step 1 only the dump tool and tests call it; charting does
// not yet.
func (h *History) BindSite(lib *Library, mapID string, seed int64) (*SiteHistory, error) {
	return h.bind(lib, mapID, seed, "", false)
}

// BindSiteSafe is BindSite for a first jump: the chapter is drawn only from
// the safe set, the map's quiet outcome if its present is quiet_safe and the
// chapters flagged safe_start. With nothing safe on the map it ends quiet.
func (h *History) BindSiteSafe(lib *Library, mapID string, seed int64) (*SiteHistory, error) {
	return h.bind(lib, mapID, seed, "", true)
}

// BindSiteEnding is BindSite for a caller that wants a particular chapter (a
// contract's distress call wants a hive): it builds a history from the map's
// arcs that ends in eventType or, failing that, in another chapter playing the
// same scenario on this map. It fails if none can.
func (h *History) BindSiteEnding(lib *Library, mapID string, seed int64, eventType string) (*SiteHistory, error) {
	def := lib.Events[eventType]
	if def == nil || !def.IsChapter() {
		return nil, fmt.Errorf("history: %q is not a chapter event", eventType)
	}
	site, err := h.bind(lib, mapID, seed, eventType, false)
	if err == nil {
		return site, nil
	}
	for _, alt := range lib.ChapterEvents() {
		if alt.ID != eventType && alt.Scenario == def.Scenario && alt.Weight[mapID] > 0 {
			if site, err2 := h.bind(lib, mapID, seed, alt.ID, false); err2 == nil {
				return site, nil
			}
		}
	}
	return nil, err
}

// maxWantAttempts bounds the retries for a site that must end in one chapter.
const maxWantAttempts = 400

func (h *History) bind(lib *Library, mapID string, seed int64, want string, safe bool) (*SiteHistory, error) {
	md := lib.Maps[mapID]
	if md == nil || md.Present == nil {
		return nil, ErrNoHistory
	}
	h.RollPool(lib)
	// A site charted ahead of time (spread from a neighbour) and not yet
	// placed is used before a new one is built.
	if want == "" {
		if site := h.unplaced(mapID); site != nil {
			return site, nil
		}
	}
	rng := h.siteRng(seed)
	w, hh, _ := md.Size.Roll(seed)

	// A wanted chapter may need a populated or an empty past, so it draws from
	// every arc of the map; otherwise a quarter of sites are uninhabited.
	uninhabited := rng.Intn(100) < lib.Civs.LifelessPercent
	var arcs []*ArcDef
	for _, id := range md.Present.Arcs {
		if a := lib.Arcs[id]; a != nil && (want != "" || a.Uninhabited == uninhabited) {
			arcs = append(arcs, a)
		}
	}
	if len(arcs) == 0 {
		return nil, fmt.Errorf("history: map %s has no %s arcs", mapID, map[bool]string{true: "uninhabited", false: "inhabited"}[uninhabited])
	}
	doneBefore := map[string]bool{}
	for _, e := range h.Events {
		if def := lib.Events[e.Type]; def != nil && e.Site != "" && beat(def) && len(e.Civs) > 0 {
			doneBefore[beatKey(e.Civs[0], e.Type)] = true
		}
	}
	attempts := maxArcAttempts
	if want != "" {
		attempts = maxWantAttempts
	}
	for attempt := 0; attempt < attempts; attempt++ {
		s := &siteSim{
			h: h, lib: lib, md: md, rng: rng, seed: seed,
			arc:        arcs[rng.Intn(len(arcs))],
			state:      InitialState(),
			outcomes:   map[string]string{},
			claimed:    map[int]bool{},
			mapW:       w,
			mapH:       hh,
			doneBefore: doneBefore,
		}
		if !s.walk() || !s.arc.Ends.Holds(s.state) || !presentHolds(md.Present, s.state) {
			continue
		}
		if want == "" {
			sim, chapter := s.chooseChapter(safe)
			return h.commit(sim, chapter), nil
		}
		def := lib.Events[want]
		if done := s.tryChapter(def); done != nil {
			return h.commit(done, def), nil
		}
	}
	if want != "" {
		return nil, fmt.Errorf("history: no arc of map %s ends in %s", mapID, want)
	}
	return nil, fmt.Errorf("history: no arc of map %s reached its present", mapID)
}

// commit writes a finished simulation into the History.
func (h *History) commit(s *siteSim, chapter *EventDef) *SiteHistory {
	if h.Sites == nil {
		h.Sites = map[string]*SiteHistory{}
	}
	if h.Figures == nil {
		h.Figures = map[string]*Figure{}
	}
	h.SiteSeq++
	site := &SiteHistory{
		ID:    fmt.Sprintf("site%d", h.SiteSeq),
		Map:   s.md.ID,
		Seed:  s.seed,
		Arc:   s.arc.ID,
		State: s.state,
	}
	if chapter != nil {
		site.Scenario = chapter.Scenario
	} else {
		site.Scenario = s.md.BaseScenario
	}
	// Invented natives become real civs.
	civID := map[string]string{}
	for _, n := range s.natives {
		id := fmt.Sprintf("civ%d", len(h.Civs)+1)
		h.Civs[id] = &Civ{ID: id, Name: n.name, Fate: FateExtant, Species: n.species}
		civID[n.id] = id
	}
	resolve := func(id string) string {
		if r, ok := civID[id]; ok {
			return r
		}
		return id
	}
	// Dates run backwards from the present: each event sits its own gap before
	// the next one, the last its gap before the campaign begins.
	dates := make([]StarDate, len(s.events))
	next := StarDate(0)
	for i := len(s.events) - 1; i >= 0; i-- {
		gap := rollRange(s.rng, s.events[i].def.GapDays, defaultGapDays)
		if gap < 1 {
			gap = 1
		}
		dates[i] = next - StarDate(gap)
		next = dates[i]
	}
	figIDs := make([]string, len(s.figs))
	for i, f := range s.figs {
		figIDs[i] = fmt.Sprintf("fig%d", len(h.Figures)+1)
		h.Figures[figIDs[i]] = &Figure{ID: figIDs[i], Name: f.name, Role: f.role, Civ: resolve(f.civ), Fate: f.fate}
	}
	ids := make([]string, len(s.events))
	for i := range s.events {
		ids[i] = h.nextEventID()
	}
	for i, d := range s.events {
		ev := Event{ID: ids[i], Date: dates[i], Type: d.def.ID, Site: site.ID, Origin: d.origin}
		for _, c := range d.civs {
			ev.Civs = append(ev.Civs, resolve(c))
		}
		for _, c := range d.causes {
			ev.Causes = append(ev.Causes, ids[c])
		}
		for _, f := range d.figures {
			ev.Figures = append(ev.Figures, figIDs[f])
		}
		if len(d.vars) > 0 {
			ev.Vars = d.vars
		}
		h.Events = append(h.Events, ev)
		site.Events = append(site.Events, ev.ID)
		for _, c := range ev.Civs {
			if civ := h.Civs[c]; civ != nil && (len(civ.Sites) == 0 || civ.Sites[len(civ.Sites)-1] != site.ID) {
				civ.Sites = append(civ.Sites, site.ID)
			}
		}
	}
	if chapter != nil && len(ids) > 0 {
		site.Chapter = ids[len(ids)-1]
	}
	switch {
	case s.lastResident != "":
		site.Civ = resolve(s.lastResident)
	default:
		for i := range s.events {
			if len(s.events[i].civs) > 0 {
				site.Civ = resolve(s.events[i].civs[0])
				break
			}
		}
	}
	// Whoever still lives here remained.
	if s.resident != "" {
		s.outcomes[s.resident] = OutcomeRemained
	}
	if len(s.outcomes) > 0 {
		site.Outcomes = map[string]string{}
		for c, o := range s.outcomes {
			site.Outcomes[resolve(c)] = o
		}
	}
	h.Sites[site.ID] = site
	h.deriveFates()
	return site
}

// deriveFates sets each civ's Fate from its outcomes on every site: extant if
// it remains anywhere or never settled; extinct if every world it settled died
// and it was never seen elsewhere; otherwise fled.
func (h *History) deriveFates() {
	type tally struct{ remained, died, fled, visits int }
	t := map[string]*tally{}
	for _, c := range h.Civs {
		t[c.ID] = &tally{}
	}
	for _, site := range h.Sites {
		for c, o := range site.Outcomes {
			if t[c] == nil {
				continue
			}
			switch o {
			case OutcomeRemained:
				t[c].remained++
			case OutcomeDied:
				t[c].died++
			case OutcomeFled:
				t[c].fled++
			}
		}
	}
	for _, c := range h.Civs {
		k := t[c.ID]
		k.visits = len(c.Sites) - (k.remained + k.died + k.fled)
		switch {
		case k.remained > 0 || k.died+k.fled == 0:
			c.Fate = FateExtant
		case k.fled == 0 && k.visits <= 0:
			c.Fate = FateExtinct
		default:
			c.Fate = FateFled
		}
	}
}

// unplaced returns the oldest site on mapID not yet bound to a location.
func (h *History) unplaced(mapID string) *SiteHistory {
	var best *SiteHistory
	for _, s := range h.Sites {
		if s.Location == "" && s.Map == mapID && (best == nil || siteOrder(s.ID) < siteOrder(best.ID)) {
			best = s
		}
	}
	return best
}

func siteOrder(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "site"))
	return n
}

// Place binds a site history to a campaign location.
func (h *History) Place(siteID, locationID string) {
	if s := h.Sites[siteID]; s != nil {
		s.Location = locationID
	}
}
