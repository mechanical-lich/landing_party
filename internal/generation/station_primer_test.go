package generation

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// stubModules is a structure runner that carves each module as a bare room
// on its deck (param z) and names it, as the real scripts do at least.
func stubModules(t *testing.T) {
	t.Helper()
	prev := structureRunner
	t.Cleanup(func() { structureRunner = prev })
	SetStructureRunner(func(l *world.Level, name string, x, y, w, h int, p map[string]any, _ int64) error {
		z := int(p["z"].(float64))
		for yy := y; yy < y+h; yy++ {
			for xx := x; xx < x+w; xx++ {
				l.SetFloor(xx, yy, z, "hull_floor", 0)
				if xx == x || yy == y || xx == x+w-1 || yy == y+h-1 {
					l.SetMiddle(xx, yy, z, "hull_wall", 0)
				} else {
					l.ClearMiddle(xx, yy, z)
				}
			}
		}
		l.AddRoom(x, y, z, w, h, name)
		return nil
	})
}

// testPlans are station plans of every frame, with a chapter's module.
func testPlans() []*StationPlan {
	var out []*StationPlan
	for i, frame := range []string{FrameSpine, FrameRing, FrameCross} {
		for _, decks := range []int{3, 4, 5} {
			p := DefaultStationPlan(int64(i*10+decks), decks)
			p.Frame = frame
			p.AddModule("lab").Hazard = true
			p.AddModule("relay")
			p.AddModule("archive").Sealed = "keycard"
			out = append(out, p)
		}
	}
	return out
}

func buildStation(t *testing.T, plan *StationPlan, w, h int, seed int64) *world.Level {
	t.Helper()
	level := world.NewLevel(w, h, 5)
	params := map[string]any{"margin": 14.0, "dock_arm": 10.0}
	if plan != nil {
		params[StationPlanParam] = plan.ToParam()
	}
	if err := (StationPrimer{}).Prime(level, params, seed); err != nil {
		t.Fatal(err)
	}
	return level
}

// stationWalk floods from the dock over every tile a colonist could stand
// on, doors open (locks ignored), stairs joining decks.
func stationWalk(level *world.Level) map[[3]int]bool {
	dock := level.ModuleByID("dock")
	seen := map[[3]int]bool{}
	if dock == nil {
		return seen
	}
	x, y, z := dock.Center()
	stack := [][3]int{{x, y, z}}
	walk := func(p [3]int) bool {
		t := level.GetTilePtr(p[0], p[1], p[2])
		return t != nil && !t.Floor.IsEmpty() && !world.IsSpaceTile(t) && !t.IsSolid()
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[p] || !walk(p) {
			continue
		}
		seen[p] = true
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			stack = append(stack, [3]int{p[0] + d[0], p[1] + d[1], p[2]})
		}
		t := level.GetTilePtr(p[0], p[1], p[2])
		if !t.Middle.IsEmpty() {
			def := world.TileDefinitions[t.Middle.Type]
			if def.StairsUp {
				stack = append(stack, [3]int{p[0], p[1], p[2] + 1})
			}
			if def.StairsDown {
				stack = append(stack, [3]int{p[0], p[1], p[2] - 1})
			}
		}
	}
	return seen
}

// The station primer builds the same station for the same plan and seed;
// every module lies inside the map's margin (the arm and a docked wreck use
// the margin); every tile one can stand on is reachable from the dock on
// foot through doors and both stair columns; no floor plate floats outside
// the hull; the chapter's module is not on the dock's deck.
func TestStationPrimerLayout(t *testing.T) {
	loadPlacementData(t)
	stubModules(t)
	for pi, plan := range append(testPlans(), nil) {
		for _, size := range [][2]int{{100, 80}, {120, 100}, {140, 120}} {
			seed := int64(pi*7 + size[0])
			what := fmt.Sprintf("plan %d (%v) %dx%d", pi, plan != nil, size[0], size[1])
			if plan != nil {
				what = fmt.Sprintf("%s %d-deck %s %dx%d", plan.Purpose, plan.Decks, plan.Frame, size[0], size[1])
			}
			a := buildStation(t, plan, size[0], size[1], seed)
			b := buildStation(t, plan, size[0], size[1], seed)
			if fmt.Sprint(a.Modules) != fmt.Sprint(b.Modules) {
				t.Errorf("%s: two builds differ", what)
			}
			want := 0
			if plan != nil {
				want = len(plan.Modules)
			} else {
				want = len(DefaultStationPlan(seed, 5).Modules)
			}
			if len(a.Modules) != want {
				t.Errorf("%s: %d modules laid of %d", what, len(a.Modules), want)
			}
			for _, m := range a.Modules {
				if m.X < 14 || m.Y < 14 || m.X+m.W > size[0]-14 || m.Y+m.H > size[1]-14 {
					t.Errorf("%s: module %s at %d,%d %dx%d leaves the margin", what, m.ID, m.X, m.Y, m.W, m.H)
				}
			}
			seen := stationWalk(a)
			for _, m := range a.Modules {
				if x, y, z := m.Center(); !seen[[3]int{x, y, z}] {
					t.Errorf("%s: module %s (z%d) not reachable from the dock", what, m.ID, m.Z)
				}
			}
			stray := 0
			for z := 0; z < a.GetDepth(); z++ {
				for y := 0; y < size[1]; y++ {
					for x := 0; x < size[0]; x++ {
						tl := a.GetTilePtr(x, y, z)
						if !tl.Floor.IsEmpty() && !tl.IsSolid() && !world.IsSpaceTile(tl) && !seen[[3]int{x, y, z}] {
							stray++
						}
					}
				}
			}
			if stray > 0 {
				t.Errorf("%s: %d floor tiles cut off from the dock", what, stray)
			}
			dock := a.ModuleByID("dock")
			if dock == nil {
				t.Fatalf("%s: no dock", what)
			}
			if plan != nil && plan.Decks >= 4 && dock.Z != 1 || plan != nil && plan.Decks < 4 && dock.Z != 0 {
				t.Errorf("%s: dock on deck %d", what, dock.Z)
			}
			if lab := a.ModuleByID("lab"); plan != nil && lab != nil && lab.Z == dock.Z {
				t.Errorf("%s: the chapter's lab is on the dock's deck", what)
			}
			if lx, ly, lz, ok := PickLanding(a, seed); !ok || !dock.Contains(lx, ly, lz) {
				t.Errorf("%s: the landing (%d,%d,%d %v) is not the dock", what, lx, ly, lz, ok)
			}
		}
	}
}

// The default plan (no history) is a crewed station that still has a dock
// and a generator.
func TestStationDefaultPlan(t *testing.T) {
	for s := int64(0); s < 20; s++ {
		p := DefaultStationPlan(s, 5)
		if !p.HasKind("dock") || !p.HasKind("generator") {
			t.Errorf("seed %d: default plan %v", s, p.SortedModuleIDs())
		}
	}
	_ = rand.Int
}

// On a station an origin bound to a module resolves inside that module on
// its deck; unbound origins go to the nearest module, spread over the decks,
// never the dock, and a hazard's never on the dock's deck.
func TestStationOriginsAnchorToModules(t *testing.T) {
	loadPlacementData(t)
	stubModules(t)
	plan := testPlans()[2] // a 5-deck spine
	level := buildStation(t, plan, 120, 100, 5)
	ReserveLanding(level, 5)
	rolled := map[string][2]int{"lab": {10, 10}, "relay": {100, 90}}
	bound := map[string]string{"lab": "lab", "relay": "relay"}
	keep := map[string]int{}
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 40; i++ {
		k := fmt.Sprintf("u%d", i)
		rolled[k] = [2]int{14 + rng.Intn(92), 14 + rng.Intn(72)}
		if i%4 == 0 {
			keep[k] = Budget().HazardKeepOut
		}
	}
	ResolveStationOrigins(level, rolled, "lab", keep, bound)
	for key, id := range bound {
		x, y, z, ok := Origin(level, key)
		m := level.ModuleByID(id)
		if !ok || m == nil || !m.Contains(x, y, z) {
			t.Errorf("origin %s at (%d,%d,%d), not in module %s", key, x, y, z, id)
		}
	}
	if x, y, z, _ := Origin(level, ChapterOrigin); level.ModuleAt(x, y, z) == nil || level.ModuleAt(x, y, z).ID != "lab" {
		t.Errorf("the chapter origin is not the lab")
	}
	decks := map[int]int{}
	for key := range rolled {
		if bound[key] != "" {
			continue
		}
		x, y, z, _ := Origin(level, key)
		m := level.ModuleAt(x, y, z)
		switch {
		case m == nil:
			t.Errorf("unbound origin %s at (%d,%d,%d) is in no module", key, x, y, z)
		case m.Kind == "dock":
			t.Errorf("unbound origin %s resolved to the dock", key)
		case keep[key] > 0 && m.Z == level.SurfaceZ:
			t.Errorf("hazard origin %s on the dock's deck", key)
		}
		decks[z]++
	}
	if len(decks) < 3 {
		t.Errorf("unbound origins on only %d decks: %v", len(decks), decks)
	}
	// Each deck of difference counts in the landing's keep-out.
	lx, ly, lz, _ := Landing(level)
	if landingDistanceAt(level, lx, ly, lz+2) != landingDistance(level, lx, ly)+2*stationDeckCost {
		t.Error("landingDistanceAt has no deck cost on a station")
	}
}

// A plan's wreck docks at the far end of a deck that is not the dock's, a
// module of the level, reachable from the dock.
func TestStationDockedWreck(t *testing.T) {
	loadPlacementData(t)
	stubModules(t)
	for _, plan := range testPlans() {
		plan.Wreck = &PlanWreck{Event: "ev9", W: 25, H: 13, Params: map[string]any{"class": "freighter"}}
		level := buildStation(t, plan, 120, 100, 7)
		w := level.ModuleByID(WreckModule)
		if w == nil {
			t.Errorf("%s %s %d decks: no docked wreck", plan.Purpose, plan.Frame, plan.Decks)
			continue
		}
		if dock := level.ModuleByID("dock"); w.Z == dock.Z && plan.Decks > 1 {
			t.Errorf("%s %s: the wreck is on the dock's deck", plan.Purpose, plan.Frame)
		}
		x, y, z := w.Center()
		if !stationWalk(level)[[3]int{x, y, z}] {
			t.Errorf("%s %s: the wreck is not reachable from the dock", plan.Purpose, plan.Frame)
		}
	}
}
