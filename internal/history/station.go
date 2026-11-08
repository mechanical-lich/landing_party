package history

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// A station site's history writes its station plan (station_plan.md,
// section 6): the founding event's purpose picks the frame, the decks and
// the modules the station always has, then rolls its optional ones; each
// event that asks for a module (evidence kind "module") adds it, or claims
// the purpose's own; an event bound to a module (origin_at "module:<kind>")
// has its origin there. The primer lays the plan out (generation).

// stationSalt keeps the plan's own rolls off every other stream.
const stationSalt int64 = 0x53746174 // "Stat"

// coreModules are the modules an "@outer" binding (a breach) never picks.
var coreModules = map[string]bool{"dock": true, "generator": true, "bridge": true, "reactor": true, "relay": true}

// belowDeckScenarios spawn their creatures on the decks below the landing's.
var belowDeckScenarios = map[string]bool{"station_mutants": true, "grey_contact": true}

// crewOnlyModules are left off a station nobody ever lived on.
var crewOnlyModules = map[string]bool{"quarters": true, "botany": true, "medbay": true, "cryo": true}

// stationPurposeOf is the purpose a site's founding event rolled, "" for none.
func (h *History) stationPurposeOf(lib *Library, site *SiteHistory) (string, *Event) {
	for _, id := range site.Events {
		ev := h.Event(id)
		if ev == nil {
			continue
		}
		if def := lib.Events[ev.Type]; def != nil && def.Founds() && ev.Vars["purpose"] != "" {
			return ev.Vars["purpose"], ev
		}
	}
	return "", nil
}

// StationPurpose is a station site's purpose ("" for a planet, or a station
// with no founding event).
func (h *History) StationPurpose(lib *Library, site *SiteHistory) string {
	if site == nil {
		return ""
	}
	p, _ := h.stationPurposeOf(lib, site)
	return p
}

// stationPlan builds a station site's plan and binds its events' origins to
// modules (event ID → module ID).
func (h *History) stationPlan(lib *Library, site *SiteHistory, md *mapdef.MapDef, wreck *generation.PlanWreck) (*generation.StationPlan, map[string]string) {
	rng := rand.New(rand.NewSource(site.Seed ^ stationSalt))
	purposeID, founder := h.stationPurposeOf(lib, site)
	if _, ok := lib.StationPurposes[purposeID]; !ok {
		purposeID = "habitat"
	}
	sp := lib.StationPurposes[purposeID]
	plan := &generation.StationPlan{Purpose: purposeID, Frame: generation.FrameSpine, Decks: 4}
	if arc := lib.Arcs[site.Arc]; arc != nil {
		plan.Uncrewed = arc.Uninhabited
	}
	if len(sp.Frame) > 0 {
		plan.Frame = sp.Frame[rng.Intn(len(sp.Frame))]
	}
	plan.Decks = rollRange(rng, sp.Decks, [2]int{4, 4})
	// A scenario whose creatures come up from below the landing deck
	// (spawn_rules max_z_delta -1) needs decks under the dock, which sits on
	// deck 1 of a station of four or more.
	if belowDeckScenarios[site.Scenario] {
		plan.Decks = max(plan.Decks, 4)
	}
	if md != nil && md.Size.Z.Max > 0 {
		plan.Decks = min(plan.Decks, md.Size.Z.Max)
	}
	add := func(kind string) *generation.PlanModule {
		m := plan.AddModule(kind)
		if s, ok := sp.Sealed[kind]; ok {
			m.Sealed = s
		}
		return m
	}
	for _, k := range sp.Modules {
		add(k)
	}
	for _, k := range sortedKeys(sp.Optional) {
		if rng.Intn(100) < sp.Optional[k] {
			add(k)
		}
	}
	if !plan.Uncrewed && !plan.HasKind("quarters") {
		add("quarters") // a crewed relay still needs somewhere to sleep
	}
	if plan.Uncrewed {
		kept := plan.Modules[:0]
		for _, m := range plan.Modules {
			if !crewOnlyModules[m.Kind] {
				kept = append(kept, m)
			}
		}
		plan.Modules = kept
	}
	bound := map[string]string{}
	pointMod := map[SitePoint]string{}
	for _, id := range site.Events {
		ev := h.Event(id)
		if ev == nil {
			continue
		}
		def := lib.Events[ev.Type]
		if def == nil {
			continue
		}
		mod := ""
		if wreck != nil && wreck.Event == ev.ID {
			mod = generation.WreckModule
		} else if shipStamp(def) {
			// A wreck the budget left out: what grows from it stands in the
			// relay or a hold.
			for _, k := range []string{"relay", "cargo", "hangar"} {
				if m := plan.Module(k); m != nil {
					mod = m.ID
					break
				}
			}
		}
		for _, e := range def.ModuleEvidence() {
			kind, _ := e.Params["module"].(string)
			var m *generation.PlanModule
			if kind == "@optional" {
				m = add(pickOptional(rng, plan, sp))
			} else if plan.Uncrewed && crewOnlyModules[kind] {
				continue
			} else if m = plan.Module(kind); m == nil {
				m = add(kind)
			}
			if s, _ := e.Params["sealed"].(string); s != "" {
				m.Sealed = s
			}
			if o, _ := e.Params["occupants"].(string); o != "" {
				m.Occupants = o
			}
			if l, _ := e.Params["log"].(bool); l {
				m.Log = true
			}
			if m.Event == "" {
				m.Event = ev.ID
			}
			if mod == "" {
				mod = m.ID
			}
		}
		if mod == "" && strings.HasPrefix(def.OriginAt, "module:") {
			for _, k := range strings.Split(strings.TrimPrefix(def.OriginAt, "module:"), ",") {
				if k == "@outer" {
					mod = outerModule(rng, plan)
					break
				}
				if k == "@any" {
					mod = anyModule(rng, plan)
					break
				}
				if m := plan.Module(k); m != nil {
					mod = m.ID
					break
				}
			}
		}
		// An event growing from another's origin (origin_from) stands where
		// that one does, unless it names a module of its own.
		if mod == "" && ev.Origin != nil && len(def.OriginFrom) > 0 {
			mod = pointMod[*ev.Origin]
		}
		if mod == "" || ev.Origin == nil {
			continue
		}
		bound[ev.ID] = mod
		if _, seen := pointMod[*ev.Origin]; !seen {
			pointMod[*ev.Origin] = mod
		}
		if m := plan.Module(mod); m != nil && (ev.ID == site.Chapter || spawnsHostiles(def)) && m.Kind != "dock" {
			m.Hazard = true
		}
	}
	plan.Wreck = wreck
	h.stationDecay(lib, site, plan, bound)
	// What each module holds, is named and says: the station's purpose for
	// its own, an event's purpose for the module it built.
	for i := range plan.Modules {
		m := &plan.Modules[i]
		p := map[string]any{}
		for k, v := range sp.Rooms {
			if k == "room_"+m.Kind { // every module script names its room by room_<kind>
				p[k] = v
			}
		}
		if m.Kind == "cargo" && sp.Holds != "" {
			p["holds"] = sp.Holds
		}
		if m.Log && founder != nil {
			p["event"] = founder.ID
			if sp.Clue != "" {
				p["clue"] = h.fill(lib, founder, "", sp.Clue)
			}
		}
		if ev := h.Event(m.Event); ev != nil && ev != founder {
			if pd, ok := lib.Purposes[ev.Vars["purpose"]]; ok {
				p["purpose"] = ev.Vars["purpose"]
				if pd.Holds != "" {
					p["holds"] = pd.Holds
				}
				for k, v := range pd.Rooms {
					p[k] = v
				}
				if pd.Clue != "" {
					p["event"] = ev.ID
					p["clue"] = h.fill(lib, ev, "", pd.Clue)
				}
			}
			if len(ev.Civs) > 0 {
				if c := h.Civs[ev.Civs[0]]; c != nil {
					p["civ"] = c.Name
				}
			}
		}
		if len(p) > 0 {
			m.Params = p
		}
	}
	return plan, bound
}

// anyModule picks any module but the dock (where a deck-wide ending hit).
func anyModule(rng *rand.Rand, plan *generation.StationPlan) string {
	var c []string
	for _, m := range plan.Modules {
		if m.Kind != "dock" {
			c = append(c, m.ID)
		}
	}
	if len(c) == 0 {
		return plan.Modules[0].ID
	}
	sort.Strings(c)
	return c[rng.Intn(len(c))]
}

// stationDecay writes what the history did to the station into its plan
// (station_plan.md, section 8): a base by status (fallen or dead 1, a drone
// relay nobody lived on 0), each event's station_damage (the whole station,
// the deck of its origin's module, or that module with its deck and the
// rest of the station), and the chapter's own module (a hazard 1 more than
// the station's floor, a threat 2). The primer raises every module to the
// largest of its own, its deck's and the station's.
func (h *History) stationDecay(lib *Library, site *SiteHistory, plan *generation.StationPlan, bound map[string]string) {
	floor := 0
	if !plan.Uncrewed && (site.State.Status == StatusFallen || site.State.Status == StatusDead) {
		floor = 1
	}
	for _, id := range site.Events {
		ev := h.Event(id)
		if ev == nil {
			continue
		}
		def := lib.Events[ev.Type]
		if def == nil || def.StationDamage == nil {
			continue
		}
		d := def.StationDamage
		switch d.Scope {
		case "station":
			floor = max(floor, d.Level)
			if d.Kind != "" {
				plan.StationKind = d.Kind
			}
		case "deck":
			if m := plan.Module(bound[ev.ID]); m != nil {
				plan.DeckDamage = append(plan.DeckDamage, generation.DeckDamage{Of: m.ID, Level: d.Level, Kind: d.Kind, Spread: d.Spread})
			}
		case "module":
			if m := plan.Module(bound[ev.ID]); m != nil {
				m.Damage = max(m.Damage, d.Level)
				if d.Kind != "" {
					m.DamageKind = d.Kind
				}
				if d.Deck > 0 {
					plan.DeckDamage = append(plan.DeckDamage, generation.DeckDamage{Of: m.ID, Level: d.Deck})
				}
			}
			floor = max(floor, d.Station)
		}
	}
	plan.StationDamage = floor
	if m := plan.Module(bound[site.Chapter]); m != nil {
		add := 0
		switch site.Tier {
		case TierThreat:
			add = 2
		case TierHazard:
			add = 1
		}
		m.Damage = max(m.Damage, min(floor+add, 3))
	}
}

// shipStamp reports whether an event's evidence stamps a crashed ship.
func shipStamp(def *EventDef) bool {
	for _, e := range def.Evidence {
		if s, _ := e.Params["script"].(string); e.Kind == "stamp" && s == "crashed_ship" {
			return true
		}
	}
	return false
}

// pickOptional is the module a station_expanded adds: one of the purpose's
// optional kinds the station lacks, else any of them, else a hold.
func pickOptional(rng *rand.Rand, plan *generation.StationPlan, sp StationPurposeDef) string {
	var lack, all []string
	for _, k := range sortedKeys(sp.Optional) {
		if plan.Uncrewed && crewOnlyModules[k] {
			continue
		}
		all = append(all, k)
		if !plan.HasKind(k) {
			lack = append(lack, k)
		}
	}
	switch {
	case len(lack) > 0:
		return lack[rng.Intn(len(lack))]
	case len(all) > 0:
		return all[rng.Intn(len(all))]
	}
	return "cargo"
}

// outerModule picks the module a breach opens: any but the core ones (dock,
// generator, bridge, reactor, relay), adding a hold if there is none.
func outerModule(rng *rand.Rand, plan *generation.StationPlan) string {
	var c []string
	for _, m := range plan.Modules {
		if !coreModules[m.Kind] {
			c = append(c, m.ID)
		}
	}
	if len(c) == 0 {
		return plan.AddModule("cargo").ID
	}
	sort.Strings(c)
	return c[rng.Intn(len(c))]
}

// StationPlanText is a plan in one line per deck-less summary, for the dump
// tool: purpose, frame, decks, then each module with its event and marks.
func StationPlanText(plan *generation.StationPlan, events func(id string) string) string {
	if plan == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s station, %s frame, %d decks", plan.Purpose, plan.Frame, plan.Decks)
	if plan.Uncrewed {
		b.WriteString(", uncrewed")
	}
	b.WriteString(":")
	for _, m := range plan.Modules {
		b.WriteString(" " + m.ID)
		var marks []string
		if m.Event != "" && events != nil {
			marks = append(marks, events(m.Event))
		}
		if m.Sealed != "" {
			marks = append(marks, "sealed")
		}
		if m.Hazard {
			marks = append(marks, "hazard")
		}
		if m.Log {
			marks = append(marks, "log")
		}
		if m.Damage > 0 {
			marks = append(marks, fmt.Sprintf("dmg %d%s", m.Damage, map[bool]string{true: " " + m.DamageKind, false: ""}[m.DamageKind != ""]))
		}
		if m.Occupants != "" {
			marks = append(marks, m.Occupants)
		}
		if len(marks) > 0 {
			b.WriteString("(" + strings.Join(marks, ", ") + ")")
		}
	}
	return b.String()
}
