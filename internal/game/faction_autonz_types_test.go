package game

import (
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// autonzTypeX is each autonz type's column on scifi_entities
// (docs/brainstorm/autonz_plan.md, "The sheet"): steel in row 936, bronze
// one row down.
var autonzTypeX = map[string]int{
	"wire_placer": 48, "enforcer": 192, "relay_drone": 0, "seeker": 96,
	"harvester": 288, "sentinel": 240, "warden": 216,
}

// Every autonz type spawns from its own blueprint with its column, the
// steel row, its stats and role; once its bronze entry is known a print is
// one row down with health x1.5 and str x1.25, and records its tier.
func TestAutonzTypesSpawn(t *testing.T) {
	d := campaign.LoadFactionData().Doctrine("autonz")
	if d == nil || !d.HasTypes() || d.Broods() {
		t.Fatalf("autonz doctrine typed %v, broods %v", d.HasTypes(), d.Broods())
	}
	if len(d.Types) != len(autonzTypeX) {
		t.Fatalf("autonz types %v", d.TypeKeys())
	}
	// A bronze entry for every type (the tree's own come with step 3).
	dir := t.TempDir()
	tree := `{`
	first := true
	for typ := range autonzTypeX {
		if !first {
			tree += ","
		}
		first = false
		tree += `"test_bronze_` + typ + `": {"name": "x", "tree": "autonz", "assimilate": {"tier": {"type": "` + typ + `", "tier": "bronze"}}}`
	}
	tree += `}`
	path := filepath.Join(dir, "research.json")
	if err := os.WriteFile(path, []byte(tree), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := research.Load(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loadContentTables() })

	for typ, x := range autonzTypeX {
		dt := d.Types[typ]
		if dt.Blueprint == "" || !factory.BlueprintExists(dt.Blueprint) {
			t.Fatalf("%s: blueprint %q", typ, dt.Blueprint)
		}
		for _, bronze := range []bool{false, true} {
			id := "fp_steel_" + typ
			var techs []string
			if bronze {
				id = "fp_bronze_" + typ
				techs = []string{"test_bronze_" + typ}
			}
			notePresenceTechs(id, techs)
			e, err := factory.CreateTyped(dt.Blueprint, 0, 0, 0, typ, id)
			if err != nil {
				t.Fatal(err)
			}
			a := e.GetComponent(components.Appearance).(*components.AppearanceComponent)
			h := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
			s := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
			k := components.KinOf(e)
			wantY, wantHP, wantStr, wantTier := 936, dt.Health, dt.Str, "steel"
			if bronze {
				wantY, wantTier = 960, "bronze"
				wantHP = int(float64(dt.Health)*1.5 + 0.5)
				wantStr = int(float64(dt.Str)*1.25 + 0.5)
			}
			if a.SpriteX != x || a.SpriteY != wantY {
				t.Errorf("%s bronze=%v: sprite (%d,%d), want (%d,%d)", typ, bronze, a.SpriteX, a.SpriteY, x, wantY)
			}
			if h.MaxHealth != wantHP || s.Str != wantStr || s.Dex != dt.Dex || s.AC != dt.AC || s.BasicAttackDice != dt.Attack {
				t.Errorf("%s bronze=%v: hp %d str %d dex %d ac %d %s", typ, bronze, h.MaxHealth, s.Str, s.Dex, s.AC, s.BasicAttackDice)
			}
			if k == nil || k.Type != typ || k.Tier != wantTier {
				t.Errorf("%s bronze=%v: kin %+v", typ, bronze, k)
			}
		}
	}
}

// Every autonz type's sprite cell is drawn on the sheet in both rows.
func TestAutonzSheetCells(t *testing.T) {
	f, err := os.Open("assets/entities/oryx_16bit_scifi_creatures.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	cells := map[string]int{"sleeper (dark tripod)": 144, "core block": 384, "ferrite": 336}
	for typ, x := range autonzTypeX {
		cells[typ] = x
	}
	for what, x := range cells {
		for _, y := range []int{936, 960} {
			n := 0
			for dy := 0; dy < 24; dy++ {
				for dx := 0; dx < 24; dx++ {
					if _, _, _, a := im.At(x+dx, y+dy).RGBA(); a > 0 {
						n++
					}
				}
			}
			if n < 50 {
				t.Errorf("%s at (%d,%d): %d drawn pixels", what, x, y, n)
			}
		}
	}
}

// A stage with nothing unlocked keeps Phase E's crew: a node 0.75 wire
// placers / 0.25 enforcers, a hub 0.6 / 0.4.
func TestAutonzMixFallsBack(t *testing.T) {
	d := campaign.LoadFactionData().Doctrine("autonz")
	for stage, want := range map[int]map[string]float64{0: {"wire_placer": 0.75, "enforcer": 0.25}, 1: {"wire_placer": 0.6, "enforcer": 0.4}} {
		got := d.TypeMix(stage, nil, nil)
		if len(got) != len(want) {
			t.Errorf("stage %d mix %v", stage, got)
		}
		for k, v := range want {
			if got[k] < v-1e-9 || got[k] > v+1e-9 {
				t.Errorf("stage %d %s %.3f, want %.2f", stage, k, got[k], v)
			}
		}
	}
	if e := d.EconomyTypes(); len(e) != 2 || e[0] != "wire_placer" || e[1] != "relay_drone" {
		t.Errorf("economy %v", e)
	}
}

// Each autonz type's print_cost is its print recipe's cost, the recipe
// prints its blueprint at a station that exists, and its tech is the type's
// unlock.
func TestAutonzPrintCosts(t *testing.T) {
	useContentTables(t)
	d := campaign.LoadFactionData().Doctrine("autonz")
	seen := map[string]bool{}
	for _, r := range crafting.AllRecipes() {
		key := r.Key
		dt := d.Types[r.KinType]
		if r.KinType == "" || dt == nil {
			continue
		}
		seen[r.KinType] = true
		if r.Output != dt.Blueprint || r.RequiresTech != dt.Unlock || !r.DeployOnCraft {
			t.Errorf("%s: output %s tech %q deploy %v; type %s unlock %q", key, r.Output, r.RequiresTech, r.DeployOnCraft, dt.Blueprint, dt.Unlock)
		}
		if len(r.Cost) != len(dt.PrintCost) {
			t.Errorf("%s cost %v, type %v", key, r.Cost, dt.PrintCost)
		}
		for k, v := range dt.PrintCost {
			if r.Cost[k] != v {
				t.Errorf("%s cost %v, type %v", key, r.Cost, dt.PrintCost)
			}
		}
		if !factory.BlueprintExists(r.Station) && r.Station != "autonz_foundry" {
			t.Errorf("%s station %s", key, r.Station)
		}
	}
	for typ := range d.Types {
		if !seen[typ] {
			t.Errorf("no print recipe for %s", typ)
		}
	}
	if r, ok := crafting.GetRecipe("refine_ferrite"); !ok || r.Station != "autonz_foundry" || r.Output != "ferrite" || r.Count != 2 {
		t.Errorf("refine_ferrite %+v", r)
	}
}
