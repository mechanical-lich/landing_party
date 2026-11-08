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
			Tech:    TechSpacefaring,
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
	id       string
	name     string
	species  string
	ancestor string // a civ ID, or another newCiv's
}

// formerAncestorPercent is the chance a native people rising on an older
// people's ruins descends from them.
const formerAncestorPercent = 30

// siteSim walks one attempt at a site's history without touching the History.
type siteSim struct {
	h    *History
	lib  *Library
	md   *mapdef.MapDef
	rng  *rand.Rand
	seed int64
	arc  *ArcDef
	// layer is the older people's arc run before arc, if any.
	layer *ArcDef

	state    PlanetState
	events   []draftEvent
	resident string
	// rival is a second people on the site (a war, an overlord); side is
	// each one's half of the map (0 or 1) across axis (0 splits x, 1 y).
	rival string
	side  map[string]int
	axis  int
	// lead and other cast a per-side emission; sideOnly skips the planet
	// effects on all but the last emission.
	lead, other  string
	sideOnly     bool
	lastResident string
	outcomes     map[string]string
	natives      []newCiv
	figs         []draftFigure
	claimed      map[int]bool
	// tech is each civ's level on this site as the history walks (index in
	// TechLevels); peak the highest a native reached. Pool civs are
	// spacefaring and not stored.
	tech, peak map[string]int
	// strict fails an attempt whose required stage could only repeat a beat
	// (the early attempts of a site), so another arc gets a chance first.
	strict bool
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
	c.side = map[string]int{}
	for k, v := range s.side {
		c.side[k] = v
	}
	c.tech, c.peak = map[string]int{}, map[string]int{}
	for k, v := range s.tech {
		c.tech[k] = v
	}
	for k, v := range s.peak {
		c.peak[k] = v
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
	return def.Civ == CivArrive || def.Civ == CivNative || def.Civ == CivTwins || def.Civ == CivRivalNative || len(def.Figures) > 0 ||
		(def.Effects.Population != "" && def.Effects.Population != PopNever)
}

// eligible reports whether def's requirements hold now.
func (s *siteSim) eligible(def *EventDef) bool {
	r := def.Requires
	if !r.Match.Holds(s.state) {
		return false
	}
	if len(r.Site) > 0 && !Has(r.Site, s.md.Present.SiteKind()) {
		return false
	}
	if len(r.Modules) > 0 && !s.stationHasAny(r.Modules) {
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
	if r.Rival != nil && (s.rival != "") != *r.Rival {
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
	case CivTwins:
		if s.resident != "" || s.rival != "" {
			return false
		}
	case CivRivalNative:
		if s.resident == "" || s.rival != "" {
			return false
		}
	case CivRivalVisitor:
		if s.resident == "" || s.rival != "" || len(s.h.Pool) == 0 {
			return false
		}
	case CivRivals, CivRival:
		if s.resident == "" || s.rival == "" {
			return false
		}
	}
	// An advance is not for a people already at their cap.
	if strings.HasPrefix(def.Effects.Tech, "+") && def.Civ == CivResident && s.resident != "" && s.techOf(s.resident) >= s.techCap(s.resident) {
		return false
	}
	if r.TechMin != "" || r.TechMax != "" {
		if lvl, ok := s.actorTech(def); ok {
			if r.TechMin != "" && lvl < TechIndex(r.TechMin) {
				return false
			}
			if r.TechMax != "" && lvl > TechIndex(r.TechMax) {
				return false
			}
		}
	}
	// origin_from alone needs its source; with origin too it is a preference
	// and the event rolls its own origin without one.
	if len(def.OriginFrom) > 0 && !def.Origin && s.originSource(def.OriginFrom) < 0 {
		return false
	}
	return true
}

// techOf is a civ's level on this site: an invented native's as tracked, a
// pool civ's spacefaring.
func (s *siteSim) techOf(civ string) int {
	if v, ok := s.tech[civ]; ok {
		return v
	}
	if strings.HasPrefix(civ, "new") {
		return 0
	}
	return TechIndex(TechSpacefaring)
}

// techCap is the highest level a civ can reach on this site: atomic for a
// native people, lower if its species caps it (civs.json species_tech_max).
func (s *siteSim) techCap(civ string) int {
	if !strings.HasPrefix(civ, "new") {
		return TechIndex(TechSpacefaring)
	}
	top := nativeTechMax
	for _, n := range s.natives {
		if n.id == civ {
			if c := TechIndex(s.lib.Civs.SpeciesTechMax[n.species]); c >= 0 && c < top {
				top = c
			}
		}
	}
	return top
}

// actorTech is the level of the civ that would lead def now; ok is false for
// a new native civ (not yet rolled) or no civ at all.
func (s *siteSim) actorTech(def *EventDef) (int, bool) {
	switch def.Civ {
	case CivResident:
		return s.techOf(s.resident), s.resident != ""
	case CivArrive, CivVisitor, CivWar:
		return TechIndex(TechSpacefaring), true
	case CivNative, CivTwins, CivRivalNative:
		return 0, false
	case CivRivals, CivRivalVisitor:
		// An overlord's gates read the people it comes down on.
		return s.techOf(s.resident), s.resident != ""
	case CivRival:
		return s.techOf(s.rival), s.rival != ""
	}
	if s.resident != "" {
		return s.techOf(s.resident), true
	}
	if s.lastResident != "" {
		return s.techOf(s.lastResident), true
	}
	return 0, false
}

// rollNativeTech is a new native people's starting level, capped by its
// species.
func (s *siteSim) rollNativeTech(species string) int {
	ct := s.lib.Civs
	lvl := TechIndex(weightedPick(s.rng, sortedKeys(ct.NativeTech), ct.NativeTech))
	if lvl < 0 {
		lvl = 0
	}
	if c := TechIndex(ct.SpeciesTechMax[species]); c >= 0 && lvl > c {
		lvl = c
	}
	return lvl
}

// beat reports whether an event type is a memorable beat (it names a figure
// or a place on the map) that should not repeat for the same civ.
func beat(def *EventDef) bool {
	return len(def.Figures) > 0 || def.Origin || def.OriginAt != "" || len(def.OriginFrom) > 0
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
	if def.PerSide {
		s.applyPerSide(def)
		return
	}
	s.applyOne(def)
}

// applyOne records one event (one side's, for a per-side event).
func (s *siteSim) applyOne(def *EventDef) {
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
	// {former} is whoever lived here before (an older layer's people).
	former := s.lastResident
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
		id := s.newNative(def, former)
		d.civs = []string{id}
		s.resident = id
	case CivTwins:
		d.civs = []string{s.lead, s.other}
	case CivRivalNative:
		// A people splits off from the resident: same kind, same level.
		res := s.resident
		id := s.newNative(&EventDef{Species: s.nativeSpecies(res)}, "")
		s.tech[id], s.peak[id] = s.techOf(res), s.techOf(res)
		s.setRival(id)
		d.civs = []string{id, res}
	case CivRivalVisitor:
		w := map[string]int{}
		var cands []string
		for _, c := range s.h.Pool {
			if c != s.resident {
				cands = append(cands, c)
			}
		}
		for _, id := range cands {
			w[id] = 1 + s.lib.Civs.Reuse*len(s.h.Civs[id].Sites)
		}
		id := weightedPick(s.rng, cands, w)
		s.setRival(id)
		d.civs = []string{id, s.resident}
	case CivRivals:
		if s.lead != "" {
			d.civs = []string{s.lead, s.other}
		} else {
			d.civs = []string{s.resident, s.rival}
		}
	case CivRival:
		d.civs = []string{s.rival, s.resident}
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
	if d.origin == nil && def.OriginAt != "" && len(d.civs) > 0 && (!strings.HasPrefix(def.OriginAt, "module:") || s.md.Present.SiteKind() == mapdef.SiteStation) {
		// A station event bound to a module (origin_at "module:<kind>") gets
		// an origin there; on a planet the binding means nothing.
		d.origin = s.rollAt(def.OriginAt, d.civs[0])
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
	if strings.Contains(def.Text, "{former}") {
		name := s.civName(former)
		if name == "" {
			name = "those before"
		}
		d.vars = map[string]string{"former": name}
	}
	for _, k := range sortedVarKeys(def.Vars) {
		if opts := def.Vars[k]; len(opts) > 0 {
			if d.vars == nil {
				d.vars = map[string]string{}
			}
			if len(opts) == 1 && opts[0] == StationPurposeVar {
				d.vars[k] = s.rollStationPurpose()
				continue
			}
			d.vars[k] = opts[s.rng.Intn(len(opts))]
		}
	}
	// The acting civ's level then: evidence and text render it ("@seat" is
	// a castle in a medieval age, a bunker in an atomic one).
	if len(d.civs) > 0 {
		civ := d.civs[0]
		lvl := s.techOf(civ)
		if d.vars == nil {
			d.vars = map[string]string{}
		}
		d.vars["tech"] = TechLevels[lvl]
		if strings.Contains(def.Text, "{weapon}") {
			if td := s.lib.Tech(TechLevels[lvl]); td != nil && len(td.Weapons) > 0 {
				d.vars["weapon"] = td.Weapons[s.rng.Intn(len(td.Weapons))]
			}
		}
		if def.Effects.Tech != "" && strings.HasPrefix(civ, "new") {
			s.tech[civ] = min(applyTech(lvl, def.Effects.Tech, true), s.techCap(civ))
			s.peak[civ] = max(s.peak[civ], s.tech[civ])
		}
	}
	if s.sideOnly {
		s.events = append(s.events, d)
		return
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
	if e.RivalFate != "" && s.rival != "" {
		out := e.RivalFate
		if out == "dwindled" {
			out = OutcomeRemained
		}
		s.outcomes[s.rival] = out
		s.rival = ""
	}
	s.events = append(s.events, d)
}

// newNative invents a native people on this site with its starting level;
// former, if set, is the people it may descend from.
func (s *siteSim) newNative(def *EventDef, former string) string {
	var taken []string
	for _, n := range s.natives {
		taken = append(taken, n.name)
	}
	n := newCiv{id: fmt.Sprintf("new%d", len(s.natives)), name: s.h.civName(s.lib.Civs, s.rng, taken), species: def.Species}
	// A people rising on another's ruins may descend from them.
	if former != "" && strings.Contains(def.Text, "{former}") && s.rng.Intn(100) < formerAncestorPercent {
		n.ancestor = former
	}
	s.natives = append(s.natives, n)
	if s.tech == nil {
		s.tech, s.peak = map[string]int{}, map[string]int{}
	}
	s.tech[n.id] = s.rollNativeTech(n.species)
	s.peak[n.id] = s.tech[n.id]
	return n.id
}

// nativeSpecies is an invented people's species, "" for any other civ.
func (s *siteSim) nativeSpecies(civ string) string {
	for _, n := range s.natives {
		if n.id == civ {
			return n.species
		}
	}
	return ""
}

// setRival makes civ the site's rival people: the resident keeps side 0, the
// rival takes side 1 across a freshly rolled axis.
func (s *siteSim) setRival(civ string) {
	s.rival = civ
	s.axis = s.rng.Intn(2)
	if s.side == nil {
		s.side = map[string]int{}
	}
	s.side[s.resident], s.side[civ] = 0, 1
}

// rollAt rolls an origin "side" (in civ's half, short of the border) or
// "border" (the middle band); anything else rolls anywhere.
func (s *siteSim) rollAt(at, civ string) *SitePoint {
	p := s.rollPoint()
	if s.rival == "" && at != "side" {
		return p
	}
	span := s.mapW
	if s.axis == 1 {
		span = s.mapH
	}
	const margin = 12
	var lo, hi int
	switch at {
	case "side":
		if s.side[civ] == 0 {
			lo, hi = margin, span*42/100
		} else {
			lo, hi = span*58/100, span-margin
		}
	case "border":
		lo, hi = span*45/100, span*55/100
	default:
		return p
	}
	v := lo
	if hi > lo {
		v = lo + s.rng.Intn(hi-lo)
	}
	if s.axis == 0 {
		p.X = v
	} else {
		p.Y = v
	}
	return p
}

// sideEligible reports whether lead may lead its emission of a per-side
// event: the event's tech gates, and an advance below its cap.
func (s *siteSim) sideEligible(def *EventDef, lead string) bool {
	lvl := s.techOf(lead)
	r := def.Requires
	if r.TechMin != "" && lvl < TechIndex(r.TechMin) || r.TechMax != "" && lvl > TechIndex(r.TechMax) {
		return false
	}
	return !strings.HasPrefix(def.Effects.Tech, "+") || lvl < s.techCap(lead)
}

// meanwhileVar marks the second people's half of a per-side event.
const meanwhileVar = "meanwhile"

// applyPerSide emits a per-side event once led by each people (the twins
// event creates both first), the second following from the first; planet
// effects and fates apply with the last emission.
func (s *siteSim) applyPerSide(def *EventDef) {
	if def.Civ == CivTwins {
		a := s.newNative(def, "")
		s.resident = a
		b := s.newNative(def, "")
		s.setRival(b)
	}
	var sides [][2]string
	for _, sd := range [][2]string{{s.resident, s.rival}, {s.rival, s.resident}} {
		if sd[0] != "" && s.sideEligible(def, sd[0]) {
			sides = append(sides, sd)
		}
	}
	for i, sd := range sides {
		s.lead, s.other = sd[0], sd[1]
		s.sideOnly = i < len(sides)-1
		s.applyOne(def)
		if i > 0 {
			d := &s.events[len(s.events)-1]
			if d.vars == nil {
				d.vars = map[string]string{}
			}
			d.vars[meanwhileVar] = "1"
		}
	}
	s.lead, s.other, s.sideOnly = "", "", false
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

// rollStationPurpose rolls a station's purpose from the map's
// station_purposes among those its arc can found; "" when none can.
func (s *siteSim) rollStationPurpose() string {
	w := map[string]int{}
	var keys []string
	for _, id := range sortedKeys(s.md.Present.StationPurposes) {
		if sp, ok := s.lib.StationPurposes[id]; ok && (len(sp.Arcs) == 0 || Has(sp.Arcs, s.arc.ID)) {
			w[id] = s.md.Present.StationPurposes[id]
			keys = append(keys, id)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	return weightedPick(s.rng, keys, w)
}

// stationPurposeID is the purpose the site's founding event rolled, "" for
// none (a planet, or a station not yet founded).
func (s *siteSim) stationPurposeID() string {
	for _, e := range s.events {
		if e.def.Founds() && e.vars["purpose"] != "" {
			return e.vars["purpose"]
		}
	}
	return ""
}

// stationPurpose is the founding event's purpose, nil for none.
func (s *siteSim) stationPurpose() *StationPurposeDef {
	if sp, ok := s.lib.StationPurposes[s.stationPurposeID()]; ok {
		return &sp
	}
	return nil
}

// stationHasAny reports whether the station has a module of one of kinds: its
// purpose's own, or one an earlier event's evidence built.
func (s *siteSim) stationHasAny(kinds []string) bool {
	if s.md.Present.SiteKind() != mapdef.SiteStation {
		return false
	}
	if sp := s.stationPurpose(); sp != nil {
		for _, k := range sp.Modules {
			if Has(kinds, k) {
				return true
			}
		}
	}
	for _, e := range s.events {
		for _, ev := range e.def.ModuleEvidence() {
			if k, _ := ev.Params["module"].(string); Has(kinds, k) {
				return true
			}
		}
	}
	return false
}

// pickWeighted picks one of a stage's eligible events: evenly, or by the
// station purpose's event_weights when it sets any of them.
func (s *siteSim) pickWeighted(ok []*EventDef) *EventDef {
	sp := s.stationPurpose()
	weighted := false
	if sp != nil {
		for _, d := range ok {
			if _, set := sp.EventWeights[d.ID]; set {
				weighted = true
			}
		}
	}
	if !weighted {
		return ok[s.rng.Intn(len(ok))]
	}
	ws := make([]int, len(ok))
	total := 0
	for i, d := range ok {
		ws[i] = 100
		if f, set := sp.EventWeights[d.ID]; set {
			ws[i] = max(int(100*f+0.5), 1)
		}
		total += ws[i]
	}
	r := s.rng.Intn(total)
	for i, w := range ws {
		if r < w {
			return ok[i]
		}
		r -= w
	}
	return ok[len(ok)-1]
}

// walk runs the main arc's stages; false if a required stage had nothing
// eligible.
func (s *siteSim) walk() bool { return s.walkArc(s.arc) }

// walkArc runs an arc's stages. A main arc's epoch stage is skipped after an
// older layer (which set the world up). An optional stage happens at its chance
// (default 50%). A repeating stage picks several events, each re-checked; a
// type already picked in the stage repeats only if repeatable. No event type
// directly follows itself unless repeatable. After each stage one interlude
// may follow (Present.Density).
func (s *siteSim) walkArc(arc *ArcDef) bool {
	for _, st := range arc.Stages {
		if st.Epoch && s.layer != nil && arc != s.layer {
			continue // the older layer already set the world up
		}
		n := 1
		if len(st.Repeat) > 0 {
			n = rollRange(s.rng, st.Repeat, [2]int{1, 1})
		}
		chance := st.Chance
		if chance <= 0 {
			chance = 50
		}
		picked := map[string]bool{}
		for k := 0; k < n; k++ {
			var ok []*EventDef
			for _, id := range st.Pick {
				def := s.lib.Events[id]
				if def == nil || !s.eligible(def) || (!def.Repeatable && (picked[id] || s.lastType() == id)) {
					continue
				}
				ok = append(ok, def)
			}
			// A beat its civ already had elsewhere yields to any other choice;
			// an optional stage with only repeats left is skipped.
			var fresh []*EventDef
			for _, def := range ok {
				if !s.repeats(def) {
					fresh = append(fresh, def)
				}
			}
			if len(fresh) > 0 {
				ok = fresh
			} else if st.Optional || k > 0 {
				break
			} else if s.strict && len(ok) > 0 {
				return false // try another arc before repeating a beat
			}
			if len(ok) == 0 {
				if st.Optional || k > 0 {
					break
				}
				return false
			}
			if k == 0 && st.Optional && s.rng.Intn(100) >= chance {
				break
			}
			def := s.pickWeighted(ok)
			s.apply(def)
			picked[def.ID] = true
		}
		s.interlude()
	}
	return true
}

// lastType is the type of the site's latest event, "" for none.
func (s *siteSim) lastType() string {
	if len(s.events) == 0 {
		return ""
	}
	return s.events[len(s.events)-1].def.ID
}

// interlude rolls the map's density for one small happening after a stage.
// Each interlude type happens once per site unless repeatable.
func (s *siteSim) interlude() {
	if s.md.Present.Density <= 0 || s.rng.Intn(100) >= s.md.Present.Density {
		return
	}
	var ok []*EventDef
	for _, def := range s.lib.Interludes() {
		if s.eligible(def) && (def.Repeatable || !s.hasType(def.ID)) && !s.repeats(def) {
			ok = append(ok, def)
		}
	}
	if len(ok) > 0 {
		s.apply(ok[s.rng.Intn(len(ok))])
	}
}

// civName is a civ's display name inside the simulation (an invented native
// has no Civ yet).
func (s *siteSim) civName(id string) string {
	for _, n := range s.natives {
		if n.id == id {
			return n.name
		}
	}
	if c := s.h.Civs[id]; c != nil {
		return c.Name
	}
	return ""
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

// chooseChapter ends the history in two rolls on the site stream: a tier by
// the map's tiers (safe_tiers on a first jump), then a chapter of that tier by
// the chapters' per-map weights. A tier with nothing eligible falls back
// threat → hazard → quiet; quiet always succeeds. On a first jump threat is
// never rolled and a hazard must be safe_start. It returns the finished sim
// and the chapter (nil for quiet).
func (s *siteSim) chooseChapter(safe bool) (*siteSim, *EventDef) {
	weights := s.md.Present.Tiers
	if safe {
		weights = map[string]int{}
		for k, v := range s.md.Present.SafeTiers {
			if k != TierThreat {
				weights[k] = v
			}
		}
	}
	tier := weightedPick(s.rng, Tiers, weights)
	switch tier {
	case TierThreat:
		if sim, def := s.pickChapter(TierThreat, safe); def != nil {
			return sim, def
		}
		fallthrough
	case TierHazard:
		if sim, def := s.pickChapter(TierHazard, safe); def != nil {
			return sim, def
		}
	}
	return s, nil
}

// pickChapter picks a chapter of tier by per-map weight among those the
// history allows; nil if none can end it.
func (s *siteSim) pickChapter(tier string, safe bool) (*siteSim, *EventDef) {
	type cand struct {
		def *EventDef
		sim *siteSim
		w   int
	}
	var cands []cand
	total := 0
	for _, def := range s.lib.ChapterEvents() {
		w := def.Weight[s.md.ID]
		if w <= 0 || def.ChapterTier() != tier || s.repeats(def) || (safe && !def.SafeStart) {
			continue
		}
		if sp := s.stationPurpose(); sp != nil {
			if f, ok := sp.ChapterWeights[def.ID]; ok {
				w = max(int(float64(w)*f+0.5), 1)
			}
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

// BindSiteSafe is BindSite for a first jump: the tier is rolled by the map's
// safe_tiers (hazard or quiet, never threat) and a hazard must be safe_start.
// With nothing safe eligible it ends quiet.
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
		if alt.ID != eventType && def.Scenario != "" && alt.Scenario == def.Scenario && alt.Weight[mapID] > 0 {
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
			tech:       map[string]int{},
			peak:       map[string]int{},
			mapW:       w,
			mapH:       hh,
			doneBefore: doneBefore,
			strict:     attempt < attempts/2,
		}
		// An older people's layer first, ending with nobody living here.
		if l := md.Present.Layers; l != nil && !s.arc.Uninhabited && len(l.Arcs) > 0 && rng.Intn(100) < l.Chance {
			s.layer = lib.Arcs[l.Arcs[rng.Intn(len(l.Arcs))]]
			if s.layer == nil || !s.walkArc(s.layer) || !s.layer.Ends.Holds(s.state) || s.resident != "" {
				continue
			}
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
	site.Scenario = s.md.BaseScenario
	site.Tier = TierQuiet
	if chapter != nil {
		site.Tier = chapter.ChapterTier()
		if chapter.Scenario != "" {
			site.Scenario = chapter.Scenario
		}
	}
	// Invented natives become real civs.
	civID := map[string]string{}
	for _, n := range s.natives {
		id := fmt.Sprintf("civ%d", len(h.Civs)+1)
		h.Civs[id] = &Civ{ID: id, Name: n.name, Fate: FateExtant, Species: n.species, Tech: TechLevels[s.peak[n.id]]}
		civID[n.id] = id
	}
	resolve := func(id string) string {
		if r, ok := civID[id]; ok {
			return r
		}
		return id
	}
	for _, n := range s.natives {
		if n.ancestor != "" {
			h.Civs[civID[n.id]].Ancestor = resolve(n.ancestor)
		}
	}
	if s.layer != nil {
		site.Layer = s.layer.ID
	}
	// Dates run backwards from the present: each event sits its own gap before
	// the next one, the last its gap before the campaign begins.
	dates := make([]StarDate, len(s.events))
	next := StarDate(0)
	for i := len(s.events) - 1; i >= 0; i-- {
		gap := rollRange(s.rng, s.events[i].def.GapDays, defaultGapDays)
		// The two sides of a per-side event happen together ("meanwhile").
		if i+1 < len(s.events) && s.events[i].def.PerSide && s.events[i+1].def == s.events[i].def {
			gap = 1 + s.rng.Intn(30)
		}
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
	if s.rival != "" {
		s.outcomes[s.rival] = OutcomeRemained
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
