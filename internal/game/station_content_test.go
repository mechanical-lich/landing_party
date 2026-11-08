package game

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// stationModules are the station module scripts with their smallest and
// largest slot sizes (station_plan.md, section 3).
var stationModules = []struct {
	script string
	sizes  [][2]int
}{
	{"station_dock", [][2]int{{9, 7}, {11, 9}, {15, 11}}},
	{"station_generator", [][2]int{{9, 7}, {11, 9}, {15, 11}}},
	{"station_lab", [][2]int{{9, 7}, {11, 9}, {15, 11}, {17, 13}}},
	{"station_botany", [][2]int{{9, 7}, {15, 11}, {21, 13}}},
	{"station_archive", [][2]int{{7, 7}, {11, 9}, {15, 11}}},
	{"station_observatory", [][2]int{{7, 7}, {11, 9}, {15, 11}}},
	{"station_relay", [][2]int{{9, 7}, {15, 11}, {21, 13}}},
	{"station_refinery", [][2]int{{11, 9}, {15, 11}, {21, 13}}},
	{"station_medbay", [][2]int{{7, 7}, {11, 9}, {13, 9}}},
	{"station_bay", [][2]int{{7, 7}, {11, 9}, {15, 11}}},
}

// stationFurniture is everything a module may place: furniture, lights,
// doors, plants and datapads. Anything else (a creature) breaks the
// convention that a module spawns no one.
var stationFurniture = map[string]bool{
	"storage_locker": true, "mineral_bin": true, "fuel_tank": true, "bio_silo": true, "hazmat_vault": true,
	"research_lab": true, "station_console": true, "station_light": true, "station_generator_broken": true,
	"bed": true, "blast_door": true, "airlock": true, "fuel_refiner": true, "alien_flora": true, "datapad": true,
	"work_light": true,
}

// Every station module builds inside its slot at each size and damage, with
// no doubled or lone wall, no stacked furniture, in under 100 ms; it names
// its room, leaves its event's datapad, places no creature, keeps its centre
// row and column clear of solid furniture (the frame's doors open there) and
// declares occupants in its header.
func TestStationModulesBuild(t *testing.T) {
	loreLoadData(t)
	ground := loreFlatLevel(t, 20, 20)
	z0 := float64(ground.GetSurfaceZ(10, 10))
	for _, m := range stationModules {
		src, err := os.ReadFile(filepath.Join("data", "scripts", "structures", m.script+".basic"))
		if err != nil {
			t.Fatal(err)
		}
		header := strings.SplitN(string(src), "function generate", 2)[0]
		if !strings.Contains(header, "occupants") {
			t.Errorf("%s: header does not declare occupants", m.script)
		}
		for _, size := range m.sizes {
			for _, dmg := range contentDamages {
				for seed := int64(1); seed <= 3; seed++ {
					p := map[string]any{"z": z0, "damage": dmg, "frame": "station", "occupants": "none",
						"event": "ev1", "clue": "a clue", "holds": "machine_parts"}
					level, z := contentBuild(t, m.script, size[0], size[1], p, seed, "hull_wall")
					what := fmt.Sprintf("%s %v damage %v seed %d", m.script, size, dmg, seed)
					if len(level.Rooms) == 0 {
						t.Errorf("%s: no named room", what)
					}
					pads := 0
					cx, cy := 4+size[0]/2, 4+size[1]/2
					for _, e := range level.Entities {
						pc, ok := entityPos(e)
						if !ok || e.Blueprint == "" || contentGroundCover[e.Blueprint] {
							continue
						}
						if e.HasComponent(components.Datapad) {
							pads++
						}
						if !stationFurniture[e.Blueprint] {
							t.Errorf("%s: placed a %s", what, e.Blueprint)
						}
						onAisle := pc.GetX() == cx && pc.GetY() > 4 && pc.GetY() < 3+size[1] ||
							pc.GetY() == cy && pc.GetX() > 4 && pc.GetX() < 3+size[0]
						if onAisle && e.HasComponent(rlcomponents.Solid) && !e.HasComponent(rlcomponents.Door) {
							t.Errorf("%s: %s blocks the aisle at (%d,%d)", what, e.Blueprint, pc.GetX()-4, pc.GetY()-4)
						}
					}
					if pads == 0 && m.script != "station_bay" {
						t.Errorf("%s: no datapad for its event", what)
					}
					_ = z
				}
			}
		}
	}
}

// stationDeckLevel is a level of space, no ground anywhere: a station's.
func stationDeckLevel(w, h, d int) *world.Level {
	level := world.NewLevel(w, h, d)
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				level.SetMiddle(x, y, z, "space", 0)
			}
		}
	}
	return level
}

// With the param z a module, a ship section and a whole docked hull build on
// a level with no ground at all, on that deck only.
func TestStationModulesBuildOnADeck(t *testing.T) {
	loreLoadData(t)
	scripts := []string{"crashed_ship"}
	for _, m := range stationModules {
		scripts = append(scripts, m.script)
	}
	for _, s := range []string{"ship_engine", "ship_reactor", "ship_cargo", "ship_quarters", "ship_cryo_bay", "ship_armoury", "ship_hangar", "ship_bridge"} {
		scripts = append(scripts, s)
	}
	for _, script := range scripts {
		level := stationDeckLevel(40, 30, 4)
		w, h := 15, 11
		p := map[string]any{"z": 2.0, "frame": "station", "damage": 1.0, "occupants": "none", "class": "freighter"}
		if script == "crashed_ship" {
			w, h = 25, 13
		}
		if err := RunStructureScriptWithParams(level, script, 5, 5, w, h, p, 3); err != nil {
			t.Fatal(err)
		}
		hull := map[int]int{}
		for z := 0; z < 4; z++ {
			for y := 0; y < 30; y++ {
				for x := 0; x < 40; x++ {
					if tl := level.GetTilePtr(x, y, z); !tl.Floor.IsEmpty() {
						hull[z]++
					}
				}
			}
		}
		if hull[2] < w*h/2 || hull[0]+hull[1]+hull[3] != 0 {
			t.Errorf("%s with z=2: floor tiles by deck %v", script, hull)
		}
		if len(level.Rooms) == 0 {
			t.Errorf("%s with z=2: no named room", script)
		}
	}
}

// Without z, crashed_ship and its sections build exactly what they did before
// the station modules: tile for tile, entity for entity (frozen copies in
// testdata/frozen_structures).
func TestStationShipUnchangedWithoutZ(t *testing.T) {
	loreLoadData(t)
	tree := []string{"crashed_ship", "ship_engine", "ship_reactor", "ship_cargo", "ship_quarters", "ship_cryo_bay", "ship_armoury", "ship_hangar", "ship_bridge", "ruin_pass"}
	for _, name := range tree {
		old, err := os.ReadFile(filepath.Join("internal", "game", "testdata", "frozen_structures", name+".basic"))
		if err != nil {
			t.Fatal(err)
		}
		code := string(old)
		// Only script names: "ship_cargo" is also a loot table.
		for _, n := range tree {
			code = strings.ReplaceAll(code, `gen_structure("`+n+`"`, `gen_structure("content_old_`+n+`"`)
			code = strings.ReplaceAll(code, `return "`+n+`"`, `return "content_old_`+n+`"`)
		}
		contentScript(t, "content_old_"+name, code)
	}
	for _, class := range []string{"scout", "freighter", "colony", "warship", "mothership"} {
		for _, dmg := range []float64{0, 1, 2, 3} {
			for seed := int64(1); seed <= 3; seed++ {
				p := map[string]any{"class": class, "damage": dmg, "occupants": "none", "purpose": "cargo", "holds": "ship_cargo"}
				w, h := 31, 17
				if class == "mothership" {
					w, h = 60, 60
				}
				if a, b := contentSnapshot(t, "crashed_ship", w, h, p, seed), contentSnapshot(t, "content_old_crashed_ship", w, h, p, seed); a != b {
					t.Errorf("crashed_ship %s damage %v seed %d: differs from the frozen script", class, dmg, seed)
				}
			}
		}
	}
}

// Over 50 charted station sites built with the real module scripts, every
// module of the site's plan is laid out as a named room on its deck, and the
// colony lands at the dock.
func TestStationSitesBuildTheirPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("builds 50 stations")
	}
	loreLoadData(t)
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(1); seed <= 50; seed++ {
		h := history.New(seed * 7919)
		site, err := h.BindSite(lib, "abandoned_station", h.Seed+131)
		if err != nil {
			t.Fatal(err)
		}
		ev := h.Evidence(lib, site)
		level := contentBuildTerrain(t, lib, h, site)
		for _, pm := range ev.Station.Modules {
			m := level.ModuleByID(pm.ID)
			if m == nil {
				t.Errorf("seed %d: plan module %s not laid out", seed, pm.ID)
				continue
			}
			x, y, z := m.Center()
			if m.Name == "" || level.RoomAt(x, y, z) == "" {
				t.Errorf("seed %d: module %s has no named room", seed, pm.ID)
			}
		}
		lx, ly, lz, ok := generation.Landing(level)
		if dock := level.ModuleByID("dock"); !ok || dock == nil || !dock.Contains(lx, ly, lz) {
			t.Errorf("seed %d: the landing is not at the dock", seed)
		}
	}
}

// stationEnding is the type of the event that ended a station site's arc
// (the last of the station endings in its chronicle).
func stationEnding(h *history.History, site *history.SiteHistory) string {
	end := ""
	for _, id := range site.Events {
		switch t := h.Event(id).Type; t {
		case "station_lost", "nanite_conversion", "colony_abandoned", "colony_withdrawn", "reactor_failure", "station_breached", "drone_station_fails":
			end = t
		}
	}
	return end
}

// buildStationPlan builds a site's terrain with its plan replaced.
func buildStationPlan(t *testing.T, lib *history.Library, h *history.History, site *history.SiteHistory, plan *generation.StationPlan) *world.Level {
	t.Helper()
	ev := h.Evidence(lib, site)
	saved := *ev.Station
	*ev.Station = *plan
	defer func() { *ev.Station = saved }()
	md := mapdef.ByID(site.Map)
	w, hh, z := md.Size.Roll(site.Seed)
	tp := map[string]any{generation.StationPlanParam: plan.ToParam()}
	for k, v := range md.TerrainParams {
		tp[k] = v
	}
	level, err := generation.BuildWorld(generation.BuildWorldOptions{Width: w, Height: hh, Depth: z, Seed: site.Seed,
		Terrain: md.Terrain, TerrainParams: tp, Origins: ev.Origins, ChapterEvent: ev.Chapter, OriginKeepOut: ev.KeepOut,
		OriginModules: ev.OriginModules, Flags: h.LoreFlags(lib, site)})
	if err != nil {
		t.Fatal(err)
	}
	return level
}

// stationTally counts, per deck, hull walls, wired floors, space tiles inside
// modules and irradiated tiles.
func stationTally(level *world.Level) (walls, wires, vacuum, rad map[int]int) {
	walls, wires, vacuum, rad = map[int]int{}, map[int]int{}, map[int]int{}, map[int]int{}
	for z := 0; z < level.GetDepth(); z++ {
		for y := 0; y < level.GetHeight(); y++ {
			for x := 0; x < level.GetWidth(); x++ {
				tl := level.GetTilePtr(x, y, z)
				if !tl.Middle.IsEmpty() && world.TileDefinitions[tl.Middle.Type].Name == "hull_wall" {
					walls[z]++
				}
				if !tl.Floor.IsEmpty() && world.TileDefinitions[tl.Floor.Type].Name == "blue_wires" {
					wires[z]++
				}
				if world.IsSpaceTile(tl) && level.ModuleAt(x, y, z) != nil {
					vacuum[z]++
				}
				if tl.Radiation > 0 {
					rad[z]++
				}
			}
		}
	}
	return
}

// Decay follows the history: per ending, walls fall only on decks whose
// modules carry damage, the ending's own deck worst; a nanite station is
// wired on one or two decks; a reactor failure's reactor deck is rubble
// round a hot core; a breached station has vacuum in a module; a quiet drone
// relay has every wall standing. The first seed of each ending is logged:
// go run ./cmd/planet-generator -history -map abandoned_station -seed N.
func TestStationDecayFollowsTheEnding(t *testing.T) {
	if testing.Short() {
		t.Skip("builds stations")
	}
	loreLoadData(t)
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for seed := int64(1); seed <= 400 && len(seen) < 7*2; seed++ {
		h := history.New(seed)
		site, err := h.BindSite(lib, "abandoned_station", seed)
		if err != nil {
			t.Fatal(err)
		}
		end := stationEnding(h, site)
		if seen[end] >= 2 || end == "drone_station_fails" && site.Tier != history.TierQuiet {
			continue
		}
		seen[end]++
		if seen[end] == 1 {
			t.Logf("ending %s: planet-generator seed %d (%s tier)", end, seed, site.Tier)
		}
		plan := h.Evidence(lib, site).Station
		intact := *plan
		intact.Modules = append([]generation.PlanModule(nil), plan.Modules...)
		for i := range intact.Modules {
			intact.Modules[i].Damage, intact.Modules[i].DamageKind = 0, ""
		}
		intact.DeckDamage, intact.StationDamage, intact.StationKind = nil, 0, ""
		a := buildStationPlan(t, lib, h, site, plan)
		b := buildStationPlan(t, lib, h, site, &intact)
		wa, wires, vacuum, rad := stationTally(a)
		wb, _, _, _ := stationTally(b)
		what := fmt.Sprintf("seed %d %s (%s)", seed, end, site.Tier)
		damaged := map[int]bool{}
		for _, m := range a.Modules {
			if m.Damage > 0 {
				damaged[m.Z] = true
			}
		}
		for z := range wb {
			if loss := wb[z] - wa[z]; !damaged[z] && plan.StationDamage == 0 && loss != 0 {
				t.Errorf("%s: deck %d has no damaged module but lost %d walls", what, z, loss)
			}
		}
		switch end {
		case "drone_station_fails":
			for z := range wb {
				if wb[z] != wa[z] {
					t.Errorf("%s: a quiet drone relay lost walls on deck %d", what, z)
				}
			}
		case "nanite_conversion":
			n := 0
			for _, c := range wires {
				if c > 0 {
					n++
				}
			}
			if n < 1 || n > 3 {
				t.Errorf("%s: wire on %d decks (%v)", what, n, wires)
			}
		case "reactor_failure":
			r := a.ModuleByID("reactor")
			if r == nil {
				t.Errorf("%s: no reactor", what)
				break
			}
			if rad[r.Z] == 0 {
				t.Errorf("%s: the failed reactor is cold", what)
			}
			for z := range wb {
				if wb[z]-wa[z] > wb[r.Z]-wa[r.Z] {
					t.Errorf("%s: deck %d lost more walls (%d) than the reactor's deck %d (%d)", what, z, wb[z]-wa[z], r.Z, wb[r.Z]-wa[r.Z])
				}
			}
		case "station_breached":
			total := 0
			for _, v := range vacuum {
				total += v
			}
			if total == 0 {
				t.Errorf("%s: no module is open to space", what)
			}
		}
	}
	for _, e := range []string{"nanite_conversion", "reactor_failure", "station_breached", "drone_station_fails", "station_lost"} {
		if seen[e] == 0 {
			t.Errorf("no site ended in %s in 400 seeds", e)
		}
	}
}
