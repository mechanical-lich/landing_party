package game

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
)

// contentCombos expands param lists into every combination.
func contentCombos(axes map[string][]any) []map[string]any {
	out := []map[string]any{{}}
	keys := make([]string, 0, len(axes))
	for k := range axes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var next []map[string]any
		for _, base := range out {
			for _, v := range axes[k] {
				m := map[string]any{}
				for bk, bv := range base {
					m[bk] = bv
				}
				m[k] = v
				next = append(next, m)
			}
		}
		out = next
	}
	return out
}

var (
	contentTechs   = []any{"primitive", "medieval", "industrial", "atomic", "spacefaring"}
	contentDamages = []any{0.0, 1.0, 2.0, 3.0}
)

// The ground structures and hazard props: each builds inside its footprint
// at its smallest and largest size, with every param combination it
// supports, no doubled or lone wall, in under 100 ms; with a history event
// it leaves its datapad, and its lockers take the holds table.
func TestContentGroundStructuresBuild(t *testing.T) {
	loreLoadData(t)
	for _, c := range []struct {
		script string
		sizes  [][2]int
		axes   map[string][]any
	}{
		{"village", [][2]int{{17, 13}, {31, 25}}, map[string][]any{"tech": {"primitive", "medieval"}, "damage": contentDamages}},
		{"city_block", [][2]int{{21, 17}, {37, 29}}, map[string][]any{"tech": {"industrial", "atomic"}, "damage": contentDamages}},
		{"outpost", [][2]int{{17, 9}, {31, 15}}, map[string][]any{"damage": contentDamages}},
		{"standing_stones", [][2]int{{11, 11}, {21, 21}}, map[string][]any{"damage": contentDamages}},
		{"fort", [][2]int{{17, 13}, {33, 25}}, map[string][]any{"tech": contentTechs, "damage": contentDamages}},
		{"landing_platform", [][2]int{{13, 13}, {25, 25}}, map[string][]any{"damage": contentDamages}},
		{"mine", [][2]int{{17, 13}, {29, 21}}, map[string][]any{"tech": contentTechs, "purpose": {"", "forced_labour"}, "damage": contentDamages}},
		{"missile_silo", [][2]int{{19, 13}, {29, 19}}, map[string][]any{"damage": contentDamages}},
		{"burial_mound", [][2]int{{11, 11}, {19, 19}}, map[string][]any{"tech": {"primitive", "medieval"}, "damage": contentDamages}},
		{"graveyard", [][2]int{{15, 11}, {29, 21}}, map[string][]any{"tech": contentTechs, "damage": contentDamages}},
		{"battlefield", [][2]int{{15, 11}, {41, 31}}, map[string][]any{"tech": contentTechs}},
		{"nest", [][2]int{{9, 9}, {15, 15}}, map[string][]any{"egg": {"xeno_egg", "troid_egg"}, "occupants": {"dormant", "none"}}},
		{"cave_in", [][2]int{{9, 7}, {17, 13}}, map[string][]any{"tech": contentTechs}},
		{"defence_post", [][2]int{{9, 9}, {15, 15}}, map[string][]any{"tech": contentTechs, "occupants": {"dormant", "none"}, "turrets": {1.0, 3.0}, "damage": contentDamages}},
		{"hull_breach", [][2]int{{9, 7}, {15, 11}}, map[string][]any{}},
		{"castle", [][2]int{{18, 16}, {33, 27}}, map[string][]any{"purpose": {"", "prison"}, "damage": contentDamages}},
		{"castle_town", [][2]int{{36, 28}, {48, 40}, {30, 24}}, map[string][]any{"tech": {"primitive", "medieval"}, "occupants": {"none", "guards"}, "damage": contentDamages}},
		{"bunker", [][2]int{{9, 7}, {15, 11}}, map[string][]any{"damage": contentDamages}},
		{"underground_bunker", [][2]int{{11, 9}, {17, 15}}, map[string][]any{"damage": contentDamages}},
		{"underground_hideout", [][2]int{{17, 13}, {23, 19}}, map[string][]any{"damage": contentDamages}},
		{"circle_tower", [][2]int{{21, 21}, {29, 29}}, map[string][]any{"purpose": {"temple", "observatory"}, "damage": contentDamages}},
		{"circle_ruined_tower", [][2]int{{21, 21}, {25, 25}}, map[string][]any{}},
		{"abandoned_reactor", [][2]int{{21, 19}, {27, 23}}, map[string][]any{"damage": contentDamages}},
	} {
		for _, size := range c.sizes {
			for _, p := range contentCombos(c.axes) {
				if _, ok := p["occupants"]; !ok {
					p["occupants"] = "none"
				}
				p["event"], p["clue"], p["holds"] = "ev1", "a clue", "machine_parts"
				level, _ := contentBuild(t, c.script, size[0], size[1], p, 2, "*")
				pads, items := 0, 0
				for _, e := range level.Entities {
					if e.HasComponent(components.Datapad) {
						pads++
					}
					if e.HasComponent(components.Storage) {
						items += len(e.GetComponent(components.Storage).(*components.StorageComponent).Items)
					}
				}
				what := fmt.Sprintf("%s %v %v", c.script, size, p)
				if pads == 0 {
					t.Errorf("%s: no datapad for its event", what)
				}
				if len(level.Rooms) == 0 {
					t.Errorf("%s: no named room", what)
				}
				if c.script != "battlefield" && c.script != "nest" && items == 0 {
					t.Errorf("%s: its lockers hold nothing", what)
				}
			}
		}
	}
}

// Without history params the structures quest fixtures and scenarios stamp
// build exactly what they did before the library: tiles and entities alike.
func TestContentExistingStructuresUnchanged(t *testing.T) {
	loreLoadData(t)
	for _, c := range []struct {
		script string
		w, h   int
	}{{"castle", 28, 22}, {"bunker", 11, 9}, {"underground_bunker", 15, 13}, {"underground_hideout", 21, 17}, {"abandoned_reactor", 21, 19}} {
		old, err := os.ReadFile(filepath.Join("internal", "game", "testdata", "frozen_structures", c.script+".basic"))
		if err != nil {
			t.Fatal(err)
		}
		contentScript(t, "content_old_"+c.script, string(old))
		for seed := int64(1); seed <= 6; seed++ {
			for _, p := range []map[string]any{{}, {"occupants": "none"}, {"quest_id": "q1"}} {
				if a, b := contentSnapshot(t, c.script, c.w, c.h, p, seed), contentSnapshot(t, "content_old_"+c.script, c.w, c.h, p, seed); a != b {
					t.Errorf("%s seed %d %v: differs from the frozen script", c.script, seed, p)
				}
			}
		}
	}
}

// contentSnapshot is a structure's tiles (every level) and entities.
func contentSnapshot(t *testing.T, script string, w, h int, params map[string]any, seed int64) string {
	t.Helper()
	level := loreFlatLevel(t, w+10, h+10)
	if err := RunStructureScriptWithParams(level, script, 5, 5, w, h, params, seed); err != nil {
		t.Fatal(err)
	}
	var out []string
	for z := 0; z < level.GetDepth(); z++ {
		for y := 0; y < h+10; y++ {
			for x := 0; x < w+10; x++ {
				tl := level.GetTilePtr(x, y, z)
				out = append(out, fmt.Sprint(tl.Floor.Type, "/", tl.Middle.Type))
			}
		}
	}
	var ents []string
	for _, e := range level.Entities {
		if pc, ok := entityPos(e); ok {
			ents = append(ents, fmt.Sprint(e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ(), e.HasComponent(components.QuestTarget)))
		}
	}
	sort.Strings(ents)
	return fmt.Sprint(out, ents)
}

// The hive presence's stage stamps build inside their footprints with no
// doubled or lone wall and a named room; they spawn nothing themselves.
func TestContentHiveStampsBuild(t *testing.T) {
	loreLoadData(t)
	for _, c := range []struct {
		script string
		sizes  [][2]int
	}{{"hive_nest", [][2]int{{11, 11}, {13, 13}, {17, 17}}}, {"hive_core", [][2]int{{17, 17}, {21, 21}, {25, 25}}}} {
		for _, size := range c.sizes {
			for seed := int64(1); seed <= 4; seed++ {
				level, _ := contentBuild(t, c.script, size[0], size[1], map[string]any{}, seed, "*")
				if len(level.Rooms) == 0 {
					t.Errorf("%s %v: no named room", c.script, size)
				}
				if n := len(level.Entities) + len(level.StaticEntities); n != 0 {
					t.Errorf("%s %v: spawned %d entities", c.script, size, n)
				}
			}
		}
	}
}
