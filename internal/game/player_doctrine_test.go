package game

import (
	"reflect"
	"sort"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/research"
)

// contentTables are the shared content files. Package init reads them
// relative to the cwd, before TestMain moves to the repo root, so TestMain
// loads them again (loadContentTables): every game test sees the real
// tables, whatever ran before it (review note: they were empty unless a
// test loaded them, and some tests loaded them for good and others put
// empty ones back, so a test's tables depended on the order).
var contentTables = []struct {
	fn   func(string) error
	path string
}{
	{construction.Load, "data/build.json"},
	{research.Load, "data/research.json"},
	{crafting.Load, "data/crafting_recipes.json"},
}

// loadContentTables loads the real content tables (TestMain, and after a
// test that swapped one).
func loadContentTables() error {
	for _, ct := range contentTables {
		if err := ct.fn(ct.path); err != nil {
			return err
		}
	}
	return nil
}

// useContentTables makes sure a test runs on the real content tables and
// puts them back after it (a test may swap one for a fixture).
func useContentTables(t *testing.T) {
	t.Helper()
	if err := loadContentTables(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loadContentTables() })
}

func colonySettlement(t *testing.T) *campaign.DoctrineSettlement {
	t.Helper()
	d := campaign.PlayerDoctrine(nil)
	if d == nil || d.Settlement == nil || !d.Playable || d.Seedable {
		t.Fatalf("colony doctrine %+v", d)
	}
	if d.CampaignMode() != "ship" {
		t.Errorf("colony campaign mode %q", d.CampaignMode())
	}
	return d.Settlement
}

// Snapshots of what the game offered before doctrines (phase C0): the build
// menu's hard-coded categories filled from build.json, every research.json
// tech, every crafting recipe, and colonists for the crew.
var (
	oldBuildMenu = []gui.BuildMenuCategory{
		{ID: "orders", Label: "Orders"},
		{ID: "walls", Label: "Walls", Types: []string{"hull_wall", "bunker_wall", "advanced_wall"}},
		{ID: "floors", Label: "Floors", Types: []string{"hull_floor", "bunker_floor", "advanced_floor"}},
		{ID: "stairs", Label: "Stairs", Types: []string{"stairs_up", "stairs_down"}},
		{ID: "structures", Label: "Structures", Types: []string{"storage_locker", "mineral_bin", "fuel_tank", "teleporter",
			"bio_silo", "hazmat_vault", "research_lab", "work_light", "bed", "basic_workbench", "gun_bench",
			"basic_armor_bench", "advanced_armor_bench", "fuel_refiner", "robot_workbench",
			// The bio-printer joined the menu after C0 (it had no category, so
			// it could not be placed).
			"bio_printer"}},
		{ID: "doors", Label: "Doors", Types: []string{"airlock", "blast_door"}},
	}
	oldResearch = []string{"advanced_metallurgy", "advanced_radiation_shielding", "basic_metallurgy", "combat_armor",
		"construction_robotics", "encyclopedia", "excavator_robotics", "fuel_refining", "global_inv_all",
		"global_inv_current", "global_inv_detailed", "powered_exoskeleton", "radiation_shielding",
		"reinforced_plating", "resource_scanner", "robotics", "scan_efficiency_1", "scan_efficiency_2",
		"security_robotics", "ship_scanner_1", "ship_scanner_2", "ship_scanner_3", "stealth_module",
		"tactical_systems", "thermal_insulation"}
	oldUnits = []string{"construction_robot", "excavator_robot", "hauler_robot", "print_colonist", "security_robot"}
	// oldRecipes is every recipe, units included. protein_press joined in
	// phase D, deliberately: the food rung's food source (biomass into a
	// protein bar at the workbench), the player's as well as a rival's.
	oldRecipes = []string{
		"autorifle", "construction_robot", "energy_sword", "enviro_boots", "enviro_helmet", "enviro_suit",
		"excavator_robot", "flag_rifle_blue", "flag_rifle_red", "flamethrower", "hauler_robot",
		"heat_resistant_feet", "heat_resistant_head", "heat_resistant_legs", "heat_resistant_torso", "heavy_feet",
		"heavy_head", "heavy_legs", "heavy_torso", "knife", "laser_pistol", "laser_sword", "marine_feet",
		"marine_head", "marine_legs", "marine_torso", "medkit", "pistol_shield", "plasma_cutter", "print_colonist", "protein_press",
		"radiation_feet", "radiation_head", "radiation_legs", "radiation_torso", "refine_fuel", "rifle",
		"rocket_launcher", "scout_feet", "scout_head", "scout_legs", "scout_torso", "security_robot", "shotgun",
		"sleeping_bag", "stealth_feet", "stealth_head", "stealth_legs", "stealth_torso", "stun_baton",
		"super_heavy_feet", "super_heavy_head", "super_heavy_legs", "super_heavy_torso", "super_radiation_feet",
		"super_radiation_head", "super_radiation_legs", "super_radiation_torso", "tactical_feet", "tactical_head",
		"tactical_legs", "tactical_torso",
	}
)

func TestColonyDoctrineBuildMenuMatchesOldGame(t *testing.T) {
	useContentTables(t)
	st := colonySettlement(t)
	got := buildMenuOf(st)
	if !reflect.DeepEqual(got, oldBuildMenu) {
		t.Errorf("colony build menu\n got %+v\nwant %+v", got, oldBuildMenu)
	}
	// And the old menu was every non-hidden entry of each category.
	for _, c := range oldBuildMenu {
		if all := construction.BuildableTypesInCategory(c.ID); !reflect.DeepEqual(all, c.Types) && !(len(all) == 0 && c.Types == nil) {
			t.Errorf("%s: build.json lists %v, snapshot %v", c.ID, all, c.Types)
		}
	}
}

func TestColonyDoctrineResearchMatchesOldGame(t *testing.T) {
	useContentTables(t)
	st := colonySettlement(t)
	tree := append([]string(nil), st.Research...)
	sort.Strings(tree)
	if !reflect.DeepEqual(tree, oldResearch) {
		t.Errorf("colony research %v, want %v", tree, oldResearch)
	}
	var all []string
	for k, tech := range research.AllTechs() {
		if tech.Tree == "" { // the hive's evolve tree is its own (phase E2)
			all = append(all, k)
		}
	}
	sort.Strings(all)
	if !reflect.DeepEqual(all, oldResearch) {
		t.Errorf("research.json has %v, snapshot %v", all, oldResearch)
	}
	// The modal's lists are the old ones at every point of a walk through the
	// tree.
	var known []string
	for step := 0; ; step++ {
		if a, b := keysOf(availableResearch(st, known)), keysOf(colonyTree(research.AvailableTechs(known))); !reflect.DeepEqual(a, b) {
			t.Fatalf("step %d available %v, old %v", step, a, b)
		}
		if a, b := keysOf(lockedResearch(st, known)), keysOf(colonyTree(research.LockedTechs(known))); !reflect.DeepEqual(a, b) {
			t.Fatalf("step %d locked %v, old %v", step, a, b)
		}
		next := colonyTree(research.AvailableTechs(known))
		if len(next) == 0 {
			break
		}
		known = append(known, next[0].Key)
	}
	if len(known) != len(oldResearch) {
		t.Errorf("walked %d techs, want %d", len(known), len(oldResearch))
	}
	if !st.CanResearch("robotics") || st.CanResearch("hive_warrior_brood") {
		t.Error("CanResearch: robotics should be in the tree, the hive's warrior brood not")
	}
}

func TestColonyDoctrineCraftingMatchesOldGame(t *testing.T) {
	useContentTables(t)
	st := colonySettlement(t)
	units := append([]string(nil), st.Units...)
	sort.Strings(units)
	if !reflect.DeepEqual(units, oldUnits) {
		t.Errorf("colony units %v, want %v", units, oldUnits)
	}
	stations := map[string]bool{}
	var all []string
	others := otherDoctrinesRecipes()
	for _, r := range crafting.AllRecipes() {
		if others[r.Key] {
			continue // another race's (the greyish prints, phase D)
		}
		stations[r.Station] = true
		all = append(all, r.Key)
	}
	sort.Strings(all)
	if !reflect.DeepEqual(all, oldRecipes) {
		t.Errorf("crafting_recipes.json has %v, snapshot %v", all, oldRecipes)
	}
	mine := append(append([]string(nil), st.Units...), st.Recipes...)
	sort.Strings(mine)
	if !reflect.DeepEqual(mine, oldRecipes) {
		t.Errorf("colony units and recipes %v, want %v", mine, oldRecipes)
	}
	offered := 0
	for station := range stations {
		got, old := craftableAt(st, station), withoutOthers(crafting.RecipesByStation(station), others)
		if !reflect.DeepEqual(got, old) {
			t.Errorf("%s: colony offers %v, old game %v", station, keysOf2(got), keysOf2(old))
		}
		offered += len(got)
	}
	if offered != len(all) {
		t.Errorf("colony offers %d recipes, old game %d", offered, len(all))
	}
}

func TestColonyDoctrineCrewMatchesOldGame(t *testing.T) {
	for _, n := range []int{0, 1, 3, 5, 8} {
		got := crewBlueprints(nil, n)
		if len(got) != n {
			t.Errorf("crew of %d: %v", n, got)
		}
		for _, bp := range got {
			if bp != "colonist" {
				t.Errorf("crew of %d: %v, want only colonists", n, got)
			}
		}
	}
	if st := colonySettlement(t); !st.Needs {
		t.Error("the colony's crew has needs")
	}
}

// A mixed crew splits by share, the remainder to the first entry.
func TestDoctrineCrewShares(t *testing.T) {
	st := &campaign.DoctrineSettlement{Crew: []campaign.DoctrineCrew{{Blueprint: "a", Share: 0.5}, {Blueprint: "b", Share: 0.5}}}
	if got := st.CrewBlueprints(5); !reflect.DeepEqual(got, []string{"a", "a", "a", "b", "b"}) {
		t.Errorf("5 split: %v", got)
	}
	var none *campaign.DoctrineSettlement
	if none.CrewBlueprints(3) != nil || none.CanBuild("bed") || none.CanCraft("medkit") || none.CanResearch("robotics") {
		t.Error("a nil settlement offers something")
	}
}

// Every playable doctrine's lists name content that exists, and the colony
// lists every menu buildable, tech and recipe, so new content does not
// silently drop out of the player's game: add it to the colony doctrine
// (data/factions.json) too.
func TestColonyDoctrineCoversContent(t *testing.T) {
	useContentTables(t)
	st := colonySettlement(t)
	otherBuilds := otherDoctrinesBuildables()
	for key, b := range construction.ListBuildables() {
		if b.Category != "" && !b.Hidden && !st.CanBuild(key) && !otherBuilds[key] {
			t.Errorf("build.json %q (category %s) is not in the colony doctrine's buildables", key, b.Category)
		}
	}
	for key, tech := range research.AllTechs() {
		if tech.Tree != "" {
			// Another doctrine's tree (the hive's evolve entries).
			if st.CanResearch(key) {
				t.Errorf("research.json %q of the %s tree is in the colony doctrine's research", key, tech.Tree)
			}
			continue
		}
		if !st.CanResearch(key) {
			t.Errorf("research.json %q is not in the colony doctrine's research", key)
		}
	}
	others := otherDoctrinesRecipes()
	for _, r := range crafting.AllRecipes() {
		if !st.CanCraft(r.Key) && !others[r.Key] {
			t.Errorf("recipe %q is in no doctrine's units or recipes", r.Key)
		}
	}
	for _, d := range campaign.LoadFactionData().Doctrines {
		if d.Settlement == nil {
			continue
		}
		s := d.Settlement
		for _, k := range s.Buildables {
			if _, ok := construction.ListBuildables()[k]; !ok {
				t.Errorf("%s: buildable %q is not in build.json", d.Name, k)
			}
		}
		for _, k := range s.Research {
			if _, ok := research.GetTech(k); !ok {
				t.Errorf("%s: tech %q is not in research.json", d.Name, k)
			}
		}
		for _, k := range append(append([]string(nil), s.Units...), s.Recipes...) {
			if _, ok := crafting.GetRecipe(k); !ok {
				t.Errorf("%s: recipe %q is not in crafting_recipes.json", d.Name, k)
			}
		}
	}
}

func keysOf(ts []research.Tech) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Key
	}
	return out
}

func keysOf2(rs []crafting.Recipe) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Key
	}
	return out
}

// otherDoctrinesRecipes is the recipes another race's settlement lists and
// the colony does not (phase D: the greyish bio-printer's units). Content
// the colony lacks must be some doctrine's.
// colonyTree is ts without other doctrines' trees (the hive's evolve
// entries): the old game's research.
func colonyTree(ts []research.Tech) []research.Tech {
	var out []research.Tech
	for _, t := range ts {
		if t.Tree == "" {
			out = append(out, t)
		}
	}
	return out
}

func otherDoctrinesRecipes() map[string]bool {
	out := map[string]bool{}
	colony := campaign.LoadFactionData().Doctrine(campaign.DefaultPlayerDoctrine).Settlement
	for _, d := range campaign.LoadFactionData().Doctrines {
		if d.Settlement == nil {
			continue
		}
		for _, k := range append(append([]string(nil), d.Settlement.Units...), d.Settlement.Recipes...) {
			if !colony.CanCraft(k) {
				out[k] = true
			}
		}
	}
	return out
}

// otherDoctrinesBuildables are the build.json entries another doctrine's
// settlement lists and the colony does not (the hive's pool, cluster,
// chamber and spire).
func otherDoctrinesBuildables() map[string]bool {
	out := map[string]bool{}
	colony := campaign.LoadFactionData().Doctrine(campaign.DefaultPlayerDoctrine).Settlement
	for _, d := range campaign.LoadFactionData().Doctrines {
		if d.Settlement == nil {
			continue
		}
		for _, k := range d.Settlement.Buildables {
			if !colony.CanBuild(k) {
				out[k] = true
			}
		}
	}
	return out
}

func withoutOthers(rs []crafting.Recipe, others map[string]bool) []crafting.Recipe {
	var out []crafting.Recipe
	for _, r := range rs {
		if !others[r.Key] {
			out = append(out, r)
		}
	}
	return out
}
