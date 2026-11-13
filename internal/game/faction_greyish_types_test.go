package game

import (
	"image"
	_ "image/png"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// greyTypeX is each greyish type's block on scifi_entities, row 408
// (greyish_plan.md, "The art available").
var greyTypeX = map[string]int{
	"grey_worker": 0, "grey_technician": 96, "grey_soldier": 192, "grey_heavy": 336,
	"grey_skimmer": 480, "grey_lancer": 624,
}

func greyishDoctrineT(t *testing.T) *campaign.Doctrine {
	t.Helper()
	d := campaign.LoadFactionData().Doctrine("greyish")
	if d == nil || !d.HasTypes() || d.Broods() || d.Builds() || !d.ShipSettles() {
		t.Fatalf("greyish doctrine: typed %v broods %v builds %v ship-settles %v", d.HasTypes(), d.Broods(), d.Builds(), d.ShipSettles())
	}
	return d
}

// Every grey type spawns from its blueprint with its block, its stats (Int
// included), its kit and its role; a soldier with a beam rifle is drawn in
// the gun column, a lancer holding its lance with its legs hanging, a heavy
// in the tentacle pose; the fliers fly.
func TestGreyishTypesSpawn(t *testing.T) {
	d := greyishDoctrineT(t)
	if len(d.Types) != len(greyTypeX) {
		t.Fatalf("greyish types %v", d.TypeKeys())
	}
	if hd := campaign.LoadFactionData().Doctrine("hive"); !hd.Broods() {
		t.Error("the hive no longer broods")
	}
	if ad := campaign.LoadFactionData().Doctrine("autonz"); !ad.Builds() || ad.ShipSettles() {
		t.Error("the autonz no longer build, or ship-settle")
	}
	for typ, x := range greyTypeX {
		dt := d.Types[typ]
		if dt.Blueprint == "" || !factory.BlueprintExists(dt.Blueprint) {
			t.Fatalf("%s: blueprint %q", typ, dt.Blueprint)
		}
		e, err := factory.CreateTyped(dt.Blueprint, 0, 0, 0, typ, "fp_grey_types")
		if err != nil {
			t.Fatal(err)
		}
		h := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		s := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
		ic := e.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent)
		if h.MaxHealth != dt.Health || s.Str != dt.Str || s.Dex != dt.Dex || s.AC != dt.AC || s.Int != dt.Int ||
			s.BasicAttackDice != dt.Attack || ic.DefaultValue != dt.Initiative {
			t.Errorf("%s: hp %d str %d dex %d ac %d int %d %s init %d", typ, h.MaxHealth, s.Str, s.Dex, s.AC, s.Int, s.BasicAttackDice, ic.DefaultValue)
		}
		if k := components.KinOf(e); k == nil || k.Type != typ {
			t.Errorf("%s: kin %+v", typ, k)
		}
		if !e.HasComponent(rlcomponents.NeverSleep) {
			t.Errorf("%s sleeps", typ)
		}
		ut := upgradedType(d, typ, nil)
		if ut.Role != dt.Role || len(ut.Tasks) != len(dt.Tasks) {
			t.Errorf("%s: role %q tasks %v", typ, ut.Role, ut.Tasks)
		}
		ea := e.GetComponent(components.EquipmentAppearance).(*components.EquipmentAppearanceComponent)
		inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		col := 0
		switch typ {
		case "grey_heavy":
			col = 2 // tentacles
		case "grey_lancer":
			col = 2 // its lance held: legs hanging
		case "grey_skimmer":
			col = 1 // wings out
		}
		if typ == "grey_soldier" || typ == "grey_worker" || typ == "grey_technician" {
			if inv.RightHand != nil || inv.LeftHand != nil {
				t.Errorf("%s spawns armed: printed bodies wait for the armory", typ)
			}
		}
		if sx, sy := ea.ResolveSprite(e, false); sx != x+col*48 || sy != 408 {
			t.Errorf("%s: drawn at (%d,%d), want (%d,408)", typ, sx, sy, x+col*48)
		}
		sk, _ := e.GetComponent(components.Skills).(*components.SkillsComponent)
		flies := sk != nil && sk.Has("flying")
		if want := typ == "grey_skimmer" || typ == "grey_lancer"; flies != want {
			t.Errorf("%s flying %v, want %v", typ, flies, want)
		}
		if typ == "grey_soldier" {
			rifle, err := factory.Create("grey_beam_rifle", 0, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			inv.AddItem(rifle)
			inv.EquipAllBest()
			if sx, _ := ea.ResolveSprite(e, false); sx != x+48 {
				t.Errorf("a soldier with a beam rifle is drawn at x %d, want the gun column %d", sx, x+48)
			}
		}
	}
}

// Every grey type's cells (its two frames in each column it uses) are drawn
// on the sheet.
func TestGreyishSheetCells(t *testing.T) {
	f, err := os.Open("assets/entities/oryx_16bit_scifi_creatures.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	cols := map[string]int{"grey_worker": 2, "grey_technician": 2, "grey_soldier": 3, "grey_heavy": 3, "grey_skimmer": 3, "grey_lancer": 3}
	for typ, x := range greyTypeX {
		for c := 0; c < cols[typ]; c++ {
			for fr := 0; fr < 2; fr++ {
				cx := x + c*48 + fr*24
				n := 0
				for dy := 0; dy < 24; dy++ {
					for dx := 0; dx < 24; dx++ {
						if _, _, _, a := im.At(cx+dx, 408+dy).RGBA(); a > 0 {
							n++
						}
					}
				}
				if n < 20 {
					t.Errorf("%s column %d frame %d at (%d,408): %d pixels", typ, c, fr, cx, n)
				}
			}
		}
	}
}

// diceAvg is the average of "XdY+Z".
func diceAvg(t *testing.T, s string) float64 {
	t.Helper()
	bonus := 0.0
	if i := strings.IndexAny(s, "+-"); i > 0 {
		b, err := strconv.Atoi(s[i:])
		if err != nil {
			t.Fatalf("dice %q", s)
		}
		bonus, s = float64(b), s[:i]
	}
	n, sides, ok := strings.Cut(s, "d")
	if !ok {
		t.Fatalf("dice %q", s)
	}
	a, err1 := strconv.Atoi(n)
	b, err2 := strconv.Atoi(sides)
	if err1 != nil || err2 != nil {
		t.Fatalf("dice %q", s)
	}
	return float64(a)*float64(b+1)/2 + bonus
}

// weaponAvg is a weapon blueprint's expected damage (dice + attack bonus).
func weaponAvg(t *testing.T, bp string) float64 {
	t.Helper()
	e, err := factory.Create(bp, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	w := e.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent)
	return diceAvg(t, w.AttackDice) + float64(w.AttackBonus)
}

// The greys are the expensive, elite extreme (greyish_plan.md, "Identity in
// numbers"; a grey alloy counts 5 hauled units, a ferrite 2.5): the cheapest
// grey costs more than every hive egg and the cheapest autonz, the dearest
// grey more than the dearest autonz, and the greys' mean above the autonz'
// (the plan's own table has the autonz warden, 22, above the grey worker,
// 16, so "every grey above every autonz" cannot hold); and the grey soldier
// with a beam rifle out-fights a colonist with a laser pistol and the
// security robot.
func TestGreyishIdentity(t *testing.T) {
	fd := campaign.LoadFactionData()
	g, a, h := fd.Doctrine("greyish"), fd.Doctrine("autonz"), fd.Doctrine("hive")
	units := map[string]float64{"grey_alloy": 5, "ferrite": 2.5}
	hauled := func(cost map[string]int) float64 {
		n := 0.0
		for k, v := range cost {
			u := units[k]
			if u == 0 {
				u = 1
			}
			n += u * float64(v)
		}
		return n
	}
	span := func(d *campaign.Doctrine) (lo, hi, mean float64) {
		lo = 1e9
		for _, k := range d.TypeKeys() {
			c := hauled(d.Types[k].PrintCost)
			lo, hi, mean = min(lo, c), max(hi, c), mean+c/float64(len(d.Types))
		}
		return
	}
	gLo, gHi, gMean := span(g)
	aLo, aHi, aMean := span(a)
	if gLo <= aLo || gHi <= aHi || gMean <= aMean {
		t.Errorf("hauled units: greys %.0f-%.0f (mean %.1f), autonz %.0f-%.0f (mean %.1f)", gLo, gHi, gMean, aLo, aHi, aMean)
	}
	for _, k := range h.TypeKeys() {
		if c := float64(h.Types[k].Cost); c >= gLo {
			t.Errorf("hive %s egg costs %.1f, the cheapest grey %.1f", k, c, gLo)
		}
	}
	cheapestGrey := gLo
	fight := func(hp int, avg float64) float64 { return float64(hp) * avg }
	soldier := fight(g.Types["grey_soldier"].Health, weaponAvg(t, "grey_beam_rifle"))
	col, err := factory.Create("colonist", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	colonist := fight(col.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).MaxHealth, weaponAvg(t, "laser_pistol"))
	robot, err := factory.Create("security_robot", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	ra := 0.0
	if inv, ok := robot.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent); ok && inv != nil {
		inv.EquipAllBest()
		for _, it := range wornGear(inv) {
			if it.HasComponent(rlcomponents.Weapon) {
				ra = max(ra, weaponAvg(t, it.Blueprint))
			}
		}
	}
	if ra == 0 {
		rs := robot.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
		ra = diceAvg(t, rs.BasicAttackDice)
	}
	security := fight(robot.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).MaxHealth, ra)
	if soldier <= colonist || soldier <= security {
		t.Errorf("grey soldier fight value %.0f, colonist %.0f, security robot %.0f", soldier, colonist, security)
	}
	t.Logf("fight value: grey soldier %.0f, colonist %.0f, security robot %.0f; cheapest grey %.0f hauled units", soldier, colonist, security, cheapestGrey)
}

// gearBody is a body on level at (5, 5, 0) holding (and wearing) items.
func gearBody(t *testing.T, level *world.Level, bp string, items ...string) *ecs.Entity {
	t.Helper()
	e, err := factory.Create(bp, 5, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for _, it := range items {
		x, err := factory.Create(it, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		inv.AddItem(x)
	}
	inv.EquipAllBest()
	level.AddEntity(e)
	return e
}

// looseItems is the item entities lying on level, by blueprint.
func looseItems(level *world.Level) map[string]int {
	out := map[string]int{}
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e != nil && e.HasComponent(rlcomponents.Item) {
				out[e.Blueprint]++
			}
		}
	}
	return out
}

// A dead body's GearDrop items roll: a pass drops them intact (onto the
// ground with no killer beside it), a fail loses them; an item without
// GearDrop is lost as always and draws no number, so a colonist's or an
// autonz' death changes nothing.
func TestGearDrop(t *testing.T) {
	level := world.NewLevel(20, 20, 1)
	// A certain pass and a certain fail, by chance 1 and 0.
	body := gearBody(t, level, "pinkish_crew", "grey_beam_rifle", "grey_field_harness")
	inv := body.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	if inv.Torso == nil || (inv.RightHand == nil && inv.LeftHand == nil) {
		t.Fatalf("the grey wears nothing: %+v", inv)
	}
	inv.Torso.GetComponent(components.GearDrop).(*components.GearDropComponent).Chance = 1
	for _, it := range []*ecs.Entity{inv.RightHand, inv.LeftHand} {
		if it != nil {
			it.GetComponent(components.GearDrop).(*components.GearDropComponent).Chance = 0
		}
	}
	if n := dropSurvivingGear(level, body, nil); n != 1 {
		t.Fatalf("%d items dropped, want the harness only", n)
	}
	if got := looseItems(level); got["grey_field_harness"] != 1 || got["grey_beam_rifle"] != 0 {
		t.Errorf("on the ground %v", got)
	}
	// Into an adjacent killer's bag, equipped.
	killer := gearBody(t, level, "colonist")
	kpc, _ := entityPos(killer)
	kpc.SetPosition(6, 5, 0)
	body2 := gearBody(t, level, "pinkish_crew", "grey_beam_pistol")
	inv2 := body2.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for _, it := range wornGear(inv2) {
		it.GetComponent(components.GearDrop).(*components.GearDropComponent).Chance = 1
	}
	if n := dropSurvivingGear(level, body2, killer); n != 1 {
		t.Fatalf("%d dropped to the killer", n)
	}
	kinv := killer.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	got := false
	for _, it := range append(wornGear(kinv), kinv.Bag...) {
		if it.Blueprint == "grey_beam_pistol" {
			got = true
		}
	}
	if !got {
		t.Error("the killer did not get the pistol")
	}
	// No GearDrop: nothing drops and nothing is drawn.
	col := gearBody(t, level, "colonist", "laser_pistol")
	if hasGearDrop(col) {
		t.Fatal("a laser pistol has a GearDrop")
	}
	if n := dropSurvivingGear(level, col, nil); n != 0 {
		t.Errorf("a colonist's death dropped %d", n)
	}
	for _, bp := range []string{"grey_lance", "grey_emitter_beam", "tentacles", "laser_pistol", "spit_gland", "sentinel_emitter"} {
		e, err := factory.Create(bp, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if e.HasComponent(components.GearDrop) {
			t.Errorf("%s has a GearDrop: natural and colony weapons never drop", bp)
		}
	}
	for bp, want := range map[string]float64{"grey_beam_pistol": 0.15, "grey_beam_rifle": 0.10, "grey_disruptor": 0.05,
		"grey_field_harness": 0.10, "grey_field_cowl": 0.15} {
		e, err := factory.Create(bp, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !e.HasComponent(components.GearDrop) || e.GetComponent(components.GearDrop).(*components.GearDropComponent).Chance != want {
			t.Errorf("%s GearDrop chance, want %v", bp, want)
		}
	}
}

// The develop tree's live effects (greyish_plan.md, "The tree"): hull plating
// adds 2 AC to a soldier, living or new; salvage rigs speed a worker's mine
// work (not a soldier's); print refinement shortens a print at the vat, not
// an armory craft.
func TestGreyishDevelopEffects(t *testing.T) {
	useContentTables(t)
	d := greyishDoctrineT(t)
	id := "fp_grey_develop"
	notePresenceTechs(id, nil)
	soldier, err := factory.CreateTyped("pinkish_crew", 0, 0, 0, "grey_soldier", id)
	if err != nil {
		t.Fatal(err)
	}
	base := soldier.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent).AC
	notePresenceTechs(id, []string{"grey_hull_plating", "grey_salvage_rigs"})
	factory.Retype(soldier)
	if ac := soldier.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent).AC; ac != base+2 {
		t.Errorf("a living soldier's AC %d after hull plating, want %d", ac, base+2)
	}
	fresh, _ := factory.CreateTyped("pinkish_crew", 0, 0, 0, "grey_soldier", id)
	if ac := fresh.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent).AC; ac != base+2 {
		t.Errorf("a new soldier's AC %d, want %d", ac, base+2)
	}
	if ut := upgradedType(d, "grey_worker", []string{"grey_salvage_rigs"}); ut.Mine < 1.3 || ut.Mine > 1.4 {
		t.Errorf("a worker's mine speed with salvage rigs %v, want 1/0.75", ut.Mine)
	}
	if ut := upgradedType(d, "grey_soldier", []string{"grey_salvage_rigs"}); ut.Mine != 0 {
		t.Errorf("a soldier (no mine task) got mine speed %v", ut.Mine)
	}
	st := &settlement.Settlement{Name: "greyish:dev"}
	pr, _ := crafting.GetRecipe("print_pinkish")
	ar, _ := crafting.GetRecipe("refine_alloy")
	if printTime(st, pr) != pr.BuildTime {
		t.Fatal("a print is shortened with nothing known")
	}
	st.UnlockTech("grey_print_refinement")
	if got := printTime(st, pr); got != 450 {
		t.Errorf("print_pinkish with print refinement: %d work, want 450", got)
	}
	if printTime(st, ar) != ar.BuildTime {
		t.Error("print refinement shortened an armory craft")
	}
}
