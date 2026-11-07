package game

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// contentMaxBuild is the budget for one structure generation.
const contentMaxBuild = 100 * time.Millisecond

// contentBuild runs a structure script on open ground with a margin, and
// checks what every structure must: nothing changed or spawned outside its
// footprint, no wall of the given tile doubled or standing alone, built in
// time. It returns the level and the surface z.
func contentBuild(t *testing.T, script string, w, h int, params map[string]any, seed int64, wall string) (*world.Level, int) {
	t.Helper()
	const m = 4
	level := loreFlatLevel(t, w+2*m, h+2*m)
	z := level.GetSurfaceZ(m+w/2, m+h/2)
	before := map[[2]int]string{}
	ring := func(fn func(x, y int)) {
		for y := 0; y < h+2*m; y++ {
			for x := 0; x < w+2*m; x++ {
				if x < m || y < m || x >= m+w || y >= m+h {
					fn(x, y)
				}
			}
		}
	}
	// Every level: an underground part must stay under the footprint too.
	tileKey := func(x, y int) string {
		var k strings.Builder
		for lz := 0; lz < level.GetDepth(); lz++ {
			tl := level.GetTilePtr(x, y, lz)
			fmt.Fprint(&k, tl.Floor.Type, "/", tl.Middle.Type, ";")
		}
		return k.String()
	}
	ring(func(x, y int) { before[[2]int{x, y}] = tileKey(x, y) })
	start := time.Now()
	if err := RunStructureScriptWithParams(level, script, m, m, w, h, params, seed); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	what := fmt.Sprintf("%s %dx%d %v seed %d", script, w, h, params, seed)
	if took > contentMaxBuild {
		t.Errorf("%s: built in %v, over %v", what, took, contentMaxBuild)
	}
	ring(func(x, y int) {
		if tileKey(x, y) != before[[2]int{x, y}] {
			t.Errorf("%s: changed (%d,%d), outside its footprint", what, x-m, y-m)
		}
	})
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if pc, ok := entityPos(e); ok && e.Blueprint != "" && (pc.GetX() < m || pc.GetY() < m || pc.GetX() >= m+w || pc.GetY() >= m+h) {
				if !contentGroundCover[e.Blueprint] {
					t.Errorf("%s: %s at (%d,%d), outside its footprint", what, e.Blueprint, pc.GetX()-m, pc.GetY()-m)
				}
			}
		}
	}
	contentNoStackedFurniture(t, level, what)
	walls := []string{wall}
	if wall == "*" {
		// Every structural wall tile in the footprint, each checked alone.
		walls = nil
		seen := map[string]bool{}
		for y := m; y < m+h; y++ {
			for x := m; x < m+w; x++ {
				if tl := level.GetTilePtr(x, y, z); !tl.Middle.IsEmpty() {
					n := world.TileDefinitions[tl.Middle.Type].Name
					if strings.HasSuffix(n, "_wall") && !strings.HasPrefix(n, "rubble_") && !seen[n] {
						seen[n] = true
						walls = append(walls, n)
					}
				}
			}
		}
	}
	for _, wl := range walls {
		if wl == "" {
			continue
		}
		if n, where := lorePosts(level, wl, m, m, w, h, z); n > 0 {
			t.Errorf("%s: %d doubled or lone %s at %v", what, n, wl, where)
		}
	}
	return level, z
}

// contentNoStackedFurniture fails when two solid entities (furniture,
// lockers, beds) cover the same tile; a multi-tile one (a bed is 1x2,
// anchored at its lower tile) covers its whole Size.
func contentNoStackedFurniture(t *testing.T, level *world.Level, what string) {
	t.Helper()
	at := map[[3]int]string{}
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			pc, ok := entityPos(e)
			if !ok || !e.HasComponent(rlcomponents.Solid) {
				continue
			}
			w, h := 1, 1
			if e.HasComponent(rlcomponents.Size) {
				sc := e.GetComponent(rlcomponents.Size).(*rlcomponents.SizeComponent)
				w, h = max(sc.Width, 1), max(sc.Height, 1)
			}
			x0, y0 := pc.GetX()-w/2, pc.GetY()-h/2
			for y := y0; y < y0+h; y++ {
				for x := x0; x < x0+w; x++ {
					k := [3]int{x, y, pc.GetZ()}
					if other, taken := at[k]; taken {
						t.Errorf("%s: %s and %s both on (%d,%d,%d)", what, other, e.Blueprint, x, y, pc.GetZ())
					}
					at[k] = e.Blueprint
				}
			}
		}
	}
}

// contentGroundCover is what the flat test ground grows by itself.
var contentGroundCover = map[string]bool{"round_tree": true, "fruit_tree": true, "critter": true}

// Every ship class builds inside its footprint at its smallest and a larger
// size, at every damage, crew state and occupants, with no doubled or lone
// hull wall, in under 100 ms, its rooms named.
func TestContentShipsBuild(t *testing.T) {
	loreLoadData(t)
	for _, c := range []struct {
		class string
		sizes [][2]int
	}{
		{"scout", [][2]int{{13, 9}, {17, 11}}},
		{"freighter", [][2]int{{25, 13}, {31, 15}}},
		{"colony", [][2]int{{31, 17}, {37, 19}}},
		{"warship", [][2]int{{29, 15}, {35, 19}}},
		{"mothership", [][2]int{{60, 60}, {72, 72}}},
		{"saucer", [][2]int{{17, 17}, {21, 21}}},
	} {
		for _, size := range c.sizes {
			for _, dmg := range []float64{0, 1, 2, 3} {
				for _, crew := range []string{"alive", "dead"} {
					for _, occ := range []string{"", "none", "cryo"} {
						for seed := int64(1); seed <= 2; seed++ {
							params := map[string]any{"class": c.class, "damage": dmg, "crew": crew}
							if occ != "" {
								params["occupants"] = occ
							}
							level, z := contentBuild(t, "crashed_ship", size[0], size[1], params, seed, "hull_wall")
							if c.class != "saucer" && len(level.Rooms) < 3 {
								t.Errorf("%s %v damage %v: %d named rooms", c.class, size, dmg, len(level.Rooms))
							}
							_ = z
						}
					}
				}
			}
		}
	}
}

// A ship tells why it came: its bridge holds a datapad with the history's
// clue, and its hold is stocked from the purpose's loot table.
func TestContentShipPurposeDiscoverable(t *testing.T) {
	loreLoadData(t)
	params := map[string]any{"class": "freighter", "damage": 0.0, "occupants": "none", "event": "ev3",
		"clue": "Manifest: forty crates of seed stock, for the colony at the rim.", "holds": "colony_stores", "room_cargo": "Seed Hold"}
	level, z := contentBuild(t, "crashed_ship", 25, 13, params, 4, "hull_wall")
	pad := false
	items := map[string]int{}
	for _, e := range level.Entities {
		pc, ok := entityPos(e)
		if !ok {
			continue
		}
		if e.HasComponent(components.Datapad) {
			dp := e.GetComponent(components.Datapad).(*components.DatapadComponent)
			pad = dp.Text == params["clue"] && dp.EventID == "ev3" && level.RoomAt(pc.GetX(), pc.GetY(), z) == "Bridge"
		}
		if e.HasComponent(components.Storage) && level.RoomAt(pc.GetX(), pc.GetY(), z) == "Seed Hold" {
			for _, it := range e.GetComponent(components.Storage).(*components.StorageComponent).Items {
				items[it.Blueprint]++
			}
		}
	}
	if !pad {
		t.Error("no datapad with the ship's clue on its bridge")
	}
	if items["ration_pack"]+items["medkit"]+items["biomass"] == 0 {
		t.Errorf("the seed hold holds %v, nothing from colony_stores", items)
	}
}

// crashed_ufo is a wrapper: it builds exactly what the old script did,
// tile for tile and entity for entity, quest target included.
func TestContentCrashedUfoUnchanged(t *testing.T) {
	loreLoadData(t)
	old, err := os.ReadFile(filepath.Join("internal", "game", "testdata", "frozen_structures", "crashed_ufo.basic"))
	if err != nil {
		t.Fatal(err)
	}
	contentScript(t, "content_old_crashed_ufo", string(old))
	snap := func(script string, params map[string]any, seed int64) string {
		level := loreFlatLevel(t, 30, 30)
		if err := RunStructureScriptWithParams(level, script, 5, 5, 17, 17, params, seed); err != nil {
			t.Fatal(err)
		}
		z := level.GetSurfaceZ(13, 13)
		var out []string
		for y := 4; y < 23; y++ {
			for x := 4; x < 23; x++ {
				tl := level.GetTilePtr(x, y, z)
				out = append(out, fmt.Sprint(tl.Floor.Type, "/", tl.Middle.Type))
			}
		}
		var ents []string
		for _, e := range level.Entities {
			if pc, ok := entityPos(e); ok {
				ents = append(ents, fmt.Sprint(e.Blueprint, pc.GetX(), pc.GetY(), e.HasComponent(components.QuestTarget)))
			}
		}
		sort.Strings(ents)
		return fmt.Sprint(out, ents)
	}
	for seed := int64(1); seed <= 12; seed++ {
		for _, p := range []map[string]any{{}, {"crew": "dead"}, {"occupants": "none"}, {"quest_id": "q1"}} {
			if snap("crashed_ufo", p, seed) != snap("content_old_crashed_ufo", p, seed) {
				t.Errorf("seed %d %v: crashed_ufo differs from the old script", seed, p)
			}
		}
	}
}
