package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// contentScript writes a throwaway structure script for one test.
func contentScript(t *testing.T, name, code string) {
	t.Helper()
	p := filepath.Join("data", "scripts", "structures", name+".basic")
	if err := os.WriteFile(p, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(p); ClearStructureScriptCache() })
	ClearStructureScriptCache()
}

// fill_container fills the locker it is pointed at from a loot table;
// spawn_lore_datapad binds a datapad to the structure's event and clue;
// name_room names the tiles the hover panel shows.
func TestContentStructureBuiltins(t *testing.T) {
	loreLoadData(t)
	contentScript(t, "content_builtins", `
function generate(x, y, w, h)
    sz = get_surface_z(x, y)
    spawn_entity("storage_locker", x + 1, y + 1, sz)
    set_flag("filled", fill_container(x + 1, y + 1, sz, "machine_parts"))
    set_flag("nothing", fill_container(x + 2, y + 2, sz, "machine_parts"))
    set_flag("pad", spawn_lore_datapad(x + 3, y + 3, sz))
    name_room(x, y, sz, 4, 4, "Specimen Tanks")
endfunction
`)
	level := loreFlatLevel(t, 20, 20)
	params := map[string]any{"event": "ev7", "clue": "the specimens are not to be fed"}
	if err := RunStructureScriptWithParams(level, "content_builtins", 5, 5, 6, 6, params, 3); err != nil {
		t.Fatal(err)
	}
	z := level.GetSurfaceZ(5, 5)
	if level.Flags["filled"].(float64) < 1 || level.Flags["nothing"].(float64) != 0 {
		t.Errorf("fill_container filled %v stacks (want some) and %v without a locker (want 0)", level.Flags["filled"], level.Flags["nothing"])
	}
	var items int
	for _, e := range level.Entities {
		if e != nil && e.HasComponent(components.Storage) {
			items += len(e.GetComponent(components.Storage).(*components.StorageComponent).Items)
		}
		if e != nil && e.HasComponent(components.Datapad) {
			dp := e.GetComponent(components.Datapad).(*components.DatapadComponent)
			if dp.EventID != "ev7" || dp.Text != params["clue"] {
				t.Errorf("datapad reads %+v", dp)
			}
		}
	}
	if items == 0 || level.Flags["pad"].(float64) != 1 {
		t.Errorf("locker holds %d items, datapad placed %v", items, level.Flags["pad"])
	}
	if got := level.RoomAt(6, 6, z); got != "Specimen Tanks" {
		t.Errorf("room at 6,6 is %q", got)
	}
	if got := level.RoomAt(10, 10, z); got != "" {
		t.Errorf("room outside the rectangle is %q", got)
	}
	// Rooms survive a save and load, entities' datapad text with them.
	b, err := json.Marshal(world.SaveLevel(level))
	if err != nil {
		t.Fatal(err)
	}
	var data world.SaveData
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	loaded := world.LoadSaveData(data)
	if got := loaded.RoomAt(6, 6, z); got != "Specimen Tanks" {
		t.Errorf("after load, room at 6,6 is %q", got)
	}
}

// ruin_pass wears a castle down by damage: more of its wall falls at each
// level, no wall is left standing alone or doubled, and the keep can still
// be walked out of.
func TestContentRuinPass(t *testing.T) {
	loreLoadData(t)
	const x0, y0, w, h = 4, 4, 28, 22
	walls := map[int]int{}
	for _, dmg := range []float64{0, 1, 2, 3} {
		for seed := int64(1); seed <= 6; seed++ {
			level := loreFlatLevel(t, 40, 34)
			if err := RunStructureScriptWithParams(level, "castle", x0, y0, w, h, map[string]any{"occupants": "none"}, seed); err != nil {
				t.Fatal(err)
			}
			if err := RunStructureScriptWithParams(level, "ruin_pass", x0, y0, w, h, map[string]any{"damage": dmg, "wall": "stone_wall"}, seed); err != nil {
				t.Fatal(err)
			}
			z := level.GetSurfaceZ(x0+w/2, y0+h/2)
			if n, where := lorePosts(level, "stone_wall", x0, y0, w, h, z); n > 0 {
				t.Errorf("damage %v seed %d: %d lone or doubled walls at %v", dmg, seed, n, where)
			}
			for y := y0; y < y0+h; y++ {
				for x := x0; x < x0+w; x++ {
					if tl := level.GetTilePtr(x, y, z); !tl.Middle.IsEmpty() && world.TileDefinitions[tl.Middle.Type].Name == "stone_wall" {
						walls[int(dmg)]++
					}
				}
			}
		}
	}
	if !(walls[0] > walls[1] && walls[1] > walls[2] && walls[2] > walls[3]) {
		t.Errorf("walls standing by damage %v: want fewer at each level", walls)
	}
}
