package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// stationIs reports whether blueprint station is a crafting station of that
// id (a grey station's blueprint is named for it).
func stationIs(station string) bool {
	e, err := factory.Create(station, 0, 0, 0)
	return err == nil && e.HasComponent(components.CraftingStation) &&
		e.GetComponent(components.CraftingStation).(*components.CraftingStationComponent).StationID == station
}

// The greys' content holds together (greyish_plan.md, step 2): each type's
// print recipe costs its print_cost and makes its blueprint at the grey vat;
// every grey recipe's station is a blueprint; the colony's bio-printer has no
// grey recipe and the vat no colony one; every grey buildable has a
// blueprint, and the modules each hull tier carries are grey buildables.
func TestGreyishContent(t *testing.T) {
	useContentTables(t)
	d := greyishDoctrineT(t)
	byType := map[string]crafting.Recipe{}
	for _, k := range d.Settlement.Units {
		r, ok := crafting.GetRecipe(k)
		if !ok {
			t.Fatalf("unit recipe %q missing", k)
		}
		byType[r.KinType] = r
		if r.Station != "grey_vat" || !r.DeployOnCraft {
			t.Errorf("%s: station %q deploy %v", k, r.Station, r.DeployOnCraft)
		}
	}
	for _, typ := range d.TypeKeys() {
		dt := d.Types[typ]
		r, ok := byType[typ]
		if !ok {
			t.Errorf("no print recipe for %s", typ)
			continue
		}
		if r.Output != dt.Blueprint || len(r.Cost) != len(dt.PrintCost) {
			t.Errorf("%s: recipe makes %s, cost %v; type %s, %v", typ, r.Output, r.Cost, dt.Blueprint, dt.PrintCost)
		}
		for k, v := range dt.PrintCost {
			if r.Cost[k] != v {
				t.Errorf("%s: recipe %s %d, type %d", typ, k, r.Cost[k], v)
			}
		}
		if dt.Unlock != "" && r.RequiresTech != dt.Unlock {
			t.Errorf("%s: recipe requires %q, type unlocks by %q", typ, r.RequiresTech, dt.Unlock)
		}
	}
	grey := map[string]bool{}
	for _, k := range append(append([]string(nil), d.Settlement.Units...), d.Settlement.Recipes...) {
		r, _ := crafting.GetRecipe(k)
		grey[k] = true
		if !stationIs(r.Station) {
			t.Errorf("%s: no blueprint is station %q", k, r.Station)
		}
		if !factory.BlueprintExists(r.Output) {
			t.Errorf("%s: output %q has no blueprint", k, r.Output)
		}
	}
	for _, r := range crafting.AllRecipes() {
		if r.Station == "bio_printer" && grey[r.Key] {
			t.Errorf("the colony's bio-printer offers the grey %s", r.Key)
		}
		if (r.Station == "grey_vat" || r.Station == "grey_armory") && !grey[r.Key] {
			t.Errorf("a grey station offers %s, which is not the greys'", r.Key)
		}
	}
	for _, k := range d.Settlement.Buildables {
		if construction.GetBuildable(k).IsEntity && !factory.BlueprintExists(k) { // hull tiles and stairs are tiles
			t.Errorf("buildable %s has no blueprint", k)
		}
	}
	for tier := range d.ShipTiers() {
		for k := range d.ShipModules(tier, []string{"grey_armory_module", "grey_field_emitter"}) {
			if !d.Settlement.CanBuild(k) {
				t.Errorf("tier %d carries %s, not a grey buildable", tier, k)
			}
		}
	}
	if m := d.ShipModules(0, nil); m["grey_vat"] != 1 || m["grey_console"] != 1 || len(m) != 2 {
		t.Errorf("a scout carries %v", m)
	}
	if m := d.ShipModules(1, nil); m["grey_armory"] != 0 {
		t.Errorf("a freighter without the armory module carries %v", m)
	}
	if m := d.ShipModules(2, []string{"grey_armory_module", "grey_field_emitter"}); m["grey_armory"] != 1 || m["grey_emitter"] != 2 {
		t.Errorf("a warship with both techs carries %v", m)
	}
	for _, k := range []string{"grey_vat", "grey_console"} {
		if !construction.GetBuildable(k).ModuleOnly {
			t.Errorf("%s is not module_only", k)
		}
	}
	for _, k := range []string{"grey_armory", "grey_array", "grey_emitter"} {
		if construction.GetBuildable(k).ModuleOnly {
			t.Errorf("%s is module_only: the ground builds it", k)
		}
	}
}

// A faction director never orders a module built (its build order path
// refuses a module_only buildable); a ground structure goes through.
func TestModuleOnlyRefused(t *testing.T) {
	useContentTables(t)
	st := &settlement.Settlement{Name: "greyish:test"}
	o := settlementOrders{st: st}
	if o.Build("grey_vat", 3, 3, 0) || o.Build("grey_console", 4, 4, 0) {
		t.Error("a director ordered a module built")
	}
	if !o.Build("grey_armory", 5, 5, 0) {
		t.Error("the ground armory was refused")
	}
	n := 0
	for _, tk := range st.Tasks.GetTasks() {
		if tk.Action == task_requests.BuildAction {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d build orders, want the armory's only", n)
	}
}

// The emitter fires on a colonist in range while the greys are hostile,
// holds fire while they are neutral, and never fires on a grey or a wild grey
// beside it.
func TestGreyEmitterTargets(t *testing.T) {
	components.ResetFactionHostility()
	t.Cleanup(components.ResetFactionHostility)
	components.SetFactionHostile("greyish", true)
	level, em, col := contentHazardScene(t, "grey_emitter", 5)
	before := contentHealth(col)
	contentTurns(level, em, 40)
	if contentHealth(col) >= before {
		t.Errorf("a hostile emitter did no harm to a colonist 5 tiles off in 40 turns")
	}
	components.SetFactionHostile("greyish", false)
	level, em, col = contentHazardScene(t, "grey_emitter", 5)
	before = contentHealth(col)
	contentTurns(level, em, 40)
	if contentHealth(col) != before {
		t.Errorf("a neutral emitter shot a colonist")
	}
	// Greys beside it, the greys hostile: it never shoots them.
	components.SetFactionHostile("greyish", true)
	level, em, col = contentHazardScene(t, "grey_emitter", 30) // the colonist far out of range
	z := level.GetSurfaceZ(15, 15)
	var greys []*ecs.Entity
	for i, bp := range []string{"greyish", "pinkish_crew"} {
		g, err := factory.Create(bp, 17+i, 15, z)
		if err != nil {
			t.Fatal(err)
		}
		if bp == "pinkish_crew" {
			g.AddComponent(&components.WorkerComponent{})
		}
		level.AddEntity(g)
		greys = append(greys, g)
	}
	hp := []int{contentHealth(greys[0]), contentHealth(greys[1])}
	contentTurns(level, em, 40)
	for i, g := range greys {
		if contentHealth(g) != hp[i] {
			t.Errorf("the emitter shot the %s beside it", g.Blueprint)
		}
	}
	_ = col
}

// A wreck is solid, unowned and drops its grey alloy and crystal when it is
// destroyed (Drops).
func TestGreyWrecksDropAlloy(t *testing.T) {
	for bp, alloy := range map[string]int{"grey_armory_wreck": 1, "grey_array_wreck": 2, "grey_emitter_wreck": 1} {
		e, err := factory.Create(bp, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !e.HasComponent(rlcomponents.Solid) || e.HasComponent(components.CraftingStation) || e.HasComponent(components.ResearchBuilding) {
			t.Errorf("%s: solid %v, still works", bp, e.HasComponent(rlcomponents.Solid))
		}
		n := 0
		for _, it := range e.GetComponent(components.Drops).(*components.DropsComponent).Items {
			if it == "grey_alloy" {
				n++
			}
		}
		if n != alloy {
			t.Errorf("%s drops %d alloy, want %d", bp, n, alloy)
		}
	}
	_ = campaign.FleetShip
}

// At a landed grey camp, with the director held off: a soldier is printed
// at the vat from alloy (a typed soldier of the settlement), alloy is
// refined and a beam rifle made at an armory.
func TestGreyVatAndArmoryWork(t *testing.T) {
	_, ms, loc, f := greyOnStart(t, 1, 3)
	d := greyishDoctrineT(t)
	dir := d.Settlement.Director
	hold := d.Ships.HoldTarget
	d.Settlement.Director, d.Ships.HoldTarget = nil, 1000 // no director, and no lift-off on a full hold
	t.Cleanup(func() { d.Settlement.Director, d.Ships.HoldTarget = dir, hold })
	name := factionSettlementName(&f.Presence, loc.ID)
	st := settlement.Settlements[name]
	st.UnlockTech("grey_beam_weapons")
	fillFactionStock(ms.level, name, map[string]int{"metal_ore": 30, "crystal": 20, "grey_alloy": 8}, st.CenterX, st.CenterY, st.CenterZ)
	var vat *ecs.Entity
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e != nil && e.Blueprint == "grey_vat" {
				vat = e
			}
		}
	}
	if vat == nil {
		t.Fatal("no grey vat at the camp")
	}
	ax, ay, az := freeLandingTile(ms.level, st.CenterX-2, st.CenterY+3, st.CenterZ)
	armory := placeHiveThing(ms.level, "grey_armory", f.Presence.ID, name, ax, ay, az)
	o := settlementOrders{st: st, level: ms.level, ds: d.Settlement}
	if !o.Craft("print_pinkish", vat) || !o.Craft("refine_alloy", armory) || !o.Craft("grey_beam_rifle", armory) {
		t.Fatal("a grey craft was refused")
	}
	soldiers := func() int {
		n := 0
		for _, e := range settlementWorkers(ms.level, name) {
			if k := components.KinOf(e); k != nil && k.Type == "grey_soldier" {
				n++
			}
		}
		return n
	}
	before := soldiers()
	stepLevel(ms, 12000, nil)
	if soldiers() != before+1 {
		t.Errorf("soldiers %d, want %d: the vat printed none", soldiers(), before+1)
	}
	stock := map[string]int{}
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e == nil {
				continue
			}
			if e.HasComponent(rlcomponents.Inventory) {
				inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
				for _, it := range append(wornGear(inv), inv.Bag...) {
					stock[it.Blueprint]++
				}
			}
			if e.HasComponent(components.Storage) {
				for _, it := range e.GetComponent(components.Storage).(*components.StorageComponent).Items {
					stock[it.Blueprint]++
				}
			}
			if e.HasComponent(rlcomponents.Item) && e.Blueprint == "grey_beam_rifle" {
				stock[e.Blueprint]++
			}
		}
	}
	if stock["grey_beam_rifle"] == 0 {
		t.Errorf("no beam rifle was made: %v", stock)
	}
	for _, tk := range st.Tasks.GetTasks() {
		if cr, ok := tk.Data.(task_requests.CraftRequest); ok && !tk.Completed {
			t.Errorf("craft %s still open", cr.RecipeID)
		}
	}
}
