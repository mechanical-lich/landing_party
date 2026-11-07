package history

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
)

const loreDataRoot = "../../data"

// loreLibrary loads the real library, maps and scenarios.
func loreLibrary(t testing.TB) *Library {
	t.Helper()
	if err := mapdef.Load(filepath.Join(loreDataRoot, "maps")); err != nil {
		t.Fatal(err)
	}
	if err := scenario.Load(filepath.Join(loreDataRoot, "scenarios")); err != nil {
		t.Fatal(err)
	}
	lib, err := LoadLibrary(filepath.Join(loreDataRoot, "history"), mapdef.All())
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

// historyMaps are the maps with a present block, sorted.
func historyMaps(lib *Library) []string {
	var out []string
	for id, m := range lib.Maps {
		if m.Present != nil {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Every map but the internal tutorial has a present and a base scenario, its
// arcs exist, and it can draw both an uninhabited and an inhabited arc.
func TestLoreMapsHavePresents(t *testing.T) {
	lib := loreLibrary(t)
	for id, m := range lib.Maps {
		if m.Internal {
			if m.Present != nil {
				t.Errorf("internal map %s has a present; history skips it", id)
			}
			continue
		}
		if m.Present == nil {
			t.Errorf("map %s has no present block", id)
			continue
		}
		if m.BaseScenario == "" || scenario.ByID(m.BaseScenario) == nil {
			t.Errorf("map %s: base_scenario %q is not a scenario", id, m.BaseScenario)
		} else if !scenario.ByID(m.BaseScenario).SupportsMap(id) {
			t.Errorf("map %s: base_scenario %q does not support it", id, m.BaseScenario)
		}
		if len(m.Present.Life) == 0 || len(m.Present.Status) == 0 {
			t.Errorf("map %s: present needs life and status", id)
		}
		var un, in int
		for _, a := range m.Present.Arcs {
			arc := lib.Arcs[a]
			if arc == nil {
				t.Errorf("map %s lists unknown arc %q", id, a)
				continue
			}
			if arc.Uninhabited {
				un++
			} else {
				in++
			}
		}
		if un == 0 || in == 0 {
			t.Errorf("map %s needs an uninhabited and an inhabited arc (has %d, %d)", id, un, in)
		}
	}
}

// Arc stages, preludes, origin sources and event requirements name real event
// types; civ roles are known.
func TestLoreLibraryReferencesResolve(t *testing.T) {
	lib := loreLibrary(t)
	known := func(where, typ string) {
		if lib.Events[typ] == nil {
			t.Errorf("%s: unknown event type %q", where, typ)
		}
	}
	for _, id := range lib.ArcIDs() {
		a := lib.Arcs[id]
		if len(a.Stages) == 0 {
			t.Errorf("arc %s has no stages", id)
		}
		for i, st := range a.Stages {
			if len(st.Pick) == 0 {
				t.Errorf("arc %s stage %d picks nothing", id, i)
			}
			for _, e := range st.Pick {
				known("arc "+id, e)
				if lib.Events[e] != nil && lib.Events[e].IsChapter() {
					t.Errorf("arc %s picks chapter %s as a stage", id, e)
				}
			}
		}
	}
	roles := map[string]bool{CivNone: true, CivResident: true, CivArrive: true, CivNative: true, CivVisitor: true, CivWar: true}
	fates := map[string]bool{"": true, "died": true, "fled": true, "dwindled": true}
	for _, id := range lib.EventIDs() {
		e := lib.Events[id]
		for _, p := range e.Prelude {
			known("prelude of "+id, p)
		}
		for _, p := range e.OriginFrom {
			known("origin_from of "+id, p)
		}
		for _, p := range append(append([]string{}, e.Requires.Events...), e.Requires.AnyEvents...) {
			known("requires of "+id, p)
		}
		if !roles[e.Civ] {
			t.Errorf("event %s: unknown civ role %q", id, e.Civ)
		}
		if !fates[e.Effects.ResidentFate] {
			t.Errorf("event %s: unknown resident_fate %q", id, e.Effects.ResidentFate)
		}
		if e.Text == "" {
			t.Errorf("event %s has no text", id)
		}
	}
}

var lorePlaceholder = regexp.MustCompile(`\{([a-z_]+)\}`)

// Every {fill} in a template can be filled from that event.
func TestLoreTextTemplatesFill(t *testing.T) {
	lib := loreLibrary(t)
	for _, id := range lib.EventIDs() {
		e := lib.Events[id]
		ok := map[string]bool{"date": true, "site": true, "civ": true, "during_war": true, "war_end": true}
		if e.Civ == CivWar {
			ok["enemy"] = true
		}
		for _, f := range e.Figures {
			ok[f.Role] = true
		}
		for k := range e.Vars {
			ok[k] = true
		}
		for _, k := range e.Fills {
			ok[k] = true
		}
		for _, m := range lorePlaceholder.FindAllStringSubmatch(e.Text, -1) {
			if !ok[m[1]] {
				t.Errorf("event %s: {%s} has nothing to fill it", id, m[1])
			}
		}
	}
}

// Evidence names only feature kinds that exist (plus the planned lore_datapad),
// structure scripts that exist, and blueprints that exist; "at": "origin" only
// on events that have one.
func TestLoreEvidenceExists(t *testing.T) {
	lib := loreLibrary(t)
	bps := loreBlueprintText(t)
	for _, id := range lib.EventIDs() {
		e := lib.Events[id]
		hasOrigin := e.Origin || len(e.OriginFrom) > 0
		for i, ev := range e.Evidence {
			where := id + " evidence " + string(rune('0'+i))
			if ev.Kind != "lore_datapad" && generation.GetFeature(ev.Kind) == nil {
				t.Errorf("%s: unknown feature kind %q", where, ev.Kind)
			}
			switch ev.At {
			case "":
			case "origin":
				if !hasOrigin {
					t.Errorf("%s: at origin, but the event has no origin", where)
				}
				if ev.Kind != "lore_datapad" && !generation.Anchorable(ev.Kind) {
					t.Errorf("%s: %s can't be anchored at an origin", where, ev.Kind)
				}
			default:
				t.Errorf("%s: unknown anchor %q", where, ev.At)
			}
			if s, ok := ev.Params["script"].(string); ok {
				if _, err := os.Stat(filepath.Join(loreDataRoot, "scripts", "structures", s+".basic")); err != nil {
					t.Errorf("%s: structure script %q does not exist", where, s)
				}
			} else if ev.Kind == "stamp" {
				t.Errorf("%s: stamp without a script", where)
			}
			if bp, ok := ev.Params["blueprint"].(string); ok && !strings.Contains(bps, `"`+bp+`"`) {
				t.Errorf("%s: blueprint %q does not exist", where, bp)
			}
		}
	}
}

func loreBlueprintText(t *testing.T) string {
	t.Helper()
	var sb strings.Builder
	err := filepath.Walk(filepath.Join(loreDataRoot, "blueprints"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(p) != ".json" {
			return err
		}
		b, err := os.ReadFile(p)
		sb.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

// The plan's threat-origins table: each scenario has these origin events.
var loreThreatOrigins = map[string][]string{
	"zombie_plague":      {"colony_plague", "lab_leak", "crash_pathogen", "wreck_pathogen"},
	"crystalline_blight": {"blight_spread"},
	"alien_world":        {"reactor_mutation", "wreck_reactor_leak"},
	"abandoned_station":  {"station_experiment"},
	"crashed_ufo":        {"ship_shot_down", "fled_battle", "scout_accident"},
	"grey_contact":       {"salvage_expedition"},
	"exterminate_xenos":  {"meteor_seeding", "infested_colonists", "deliberate_drop", "wreck_eggs_hatch"},
	"troid_outbreak":     {"impact_colony"},
	"autonz_incursion":   {"swarm_landing", "wreck_nanites_wake"},
	"worms":              {"burrowers_stir"},
	"yeti_territory":     {"yeti_return"},
}

// Chapters carry real scenarios onto maps those scenarios support; every
// scenario has its origin events and is reachable on at least one map.
func TestLoreScenariosCoveredAndSupported(t *testing.T) {
	lib := loreLibrary(t)
	reach := map[string]bool{}
	for _, e := range lib.ChapterEvents() {
		sc := scenario.ByID(e.Scenario)
		if sc == nil {
			t.Errorf("chapter %s: unknown scenario %q", e.ID, e.Scenario)
			continue
		}
		if len(e.Weight) == 0 {
			t.Errorf("chapter %s has no map weights", e.ID)
		}
		for m, w := range e.Weight {
			md := lib.Maps[m]
			if md == nil || md.Present == nil {
				t.Errorf("chapter %s weighs map %q, which has no history", e.ID, m)
				continue
			}
			if !sc.SupportsMap(m) {
				t.Errorf("chapter %s carries %s onto %s, outside its supported_maps", e.ID, e.Scenario, m)
			}
			if w > 0 {
				reach[e.Scenario] = true
			}
		}
	}
	for _, m := range historyMaps(lib) {
		reach[lib.Maps[m].BaseScenario] = true
	}
	for _, sc := range scenario.AllEnabled() {
		if sc.ID == "tutorial" {
			continue
		}
		if !reach[sc.ID] {
			t.Errorf("scenario %s is unreachable", sc.ID)
		}
	}
	for sc, evs := range loreThreatOrigins {
		for _, id := range evs {
			e := lib.Events[id]
			if e == nil || e.Scenario != sc {
				t.Errorf("scenario %s: origin event %s missing or not carrying it", sc, id)
			}
		}
	}
	for _, id := range []string{"ancient_wreck", "stellar_flare", "nuclear_war", "reactor_meltdown", "impact", "nanite_conversion", "hive_consumption", "ice_age"} {
		if lib.Events[id] == nil {
			t.Errorf("missing event %s", id)
		}
	}
	if a := lib.Arcs["lifeless"]; a == nil || !a.Uninhabited {
		t.Error("the lifeless arc is missing or not uninhabited")
	}
}

// Every arc has a Star Map summary and every chapter an omen; summaries only
// use {civ}.
func TestLoreSummariesAndOmens(t *testing.T) {
	lib := loreLibrary(t)
	for _, id := range lib.ArcIDs() {
		a := lib.Arcs[id]
		if a.Summary == "" {
			t.Errorf("arc %s has no summary", id)
		}
		for _, m := range lorePlaceholder.FindAllStringSubmatch(a.Summary, -1) {
			if m[1] != "civ" {
				t.Errorf("arc %s summary uses {%s}", id, m[1])
			}
		}
	}
	for _, e := range lib.ChapterEvents() {
		if e.Omen == "" {
			t.Errorf("chapter %s has no omen", e.ID)
		}
	}
}

// Every structure history stamps says who is inside: a history ruin must not
// bring a structure's default guards onto a quiet site.
func TestLoreEvidenceDeclaresOccupants(t *testing.T) {
	lib := loreLibrary(t)
	for _, id := range lib.EventIDs() {
		for _, ev := range lib.Events[id].Evidence {
			if ev.Kind != "stamp" {
				continue
			}
			if occ, _ := ev.Params["occupants"].(string); occ == "" {
				t.Errorf("event %s stamps %v without occupants", id, ev.Params["script"])
			}
		}
	}
}
