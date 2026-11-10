package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// hiveTestLevel is a small level of resin (x < 8) and bare ground, with a
// hive settlement "hive:site" registered for the test.
func hiveTestLevel(t *testing.T) (*world.Level, *settlement.Settlement) {
	t.Helper()
	lvl := world.NewLevel(16, 12, 3)
	for x := 0; x < 16; x++ {
		for y := 0; y < 12; y++ {
			floor := "hive_resin"
			if x >= 8 {
				floor = "alien_ground"
			}
			lvl.SetFloor(x, y, 1, floor, 0)
		}
	}
	st := &settlement.Settlement{Name: "hive:site", CenterX: 4, CenterY: 6, CenterZ: 1}
	settlement.Settlements[st.Name] = st
	t.Cleanup(func() { delete(settlement.Settlements, st.Name) })
	return lvl, st
}

// A hive structure is ordered only on resin, through the build order path
// the player's cursor and every director share.
func TestHiveBuildNeedsResin(t *testing.T) {
	lvl, st := hiveTestLevel(t)
	if construction.GetBuildable("biomass_pool").Name == "" {
		if err := construction.Load("data/build.json"); err != nil {
			t.Fatal(err)
		}
	}
	if orderBuild(st, lvl, "biomass_pool", 10, 4, 1) {
		t.Error("a biomass pool ordered on bare ground")
	}
	if !orderBuild(st, lvl, "biomass_pool", 3, 4, 1) {
		t.Error("a biomass pool refused on resin")
	}
	if !orderBuild(st, lvl, "storage_locker", 10, 4, 1) {
		t.Error("a locker (no floor rule) refused")
	}
}

// A settled egg (one its settlement laid) hatches a nursery hatchling of its
// type, which grows into the morph's crew body: a worker of the settlement
// in its type's colour, stats, role and tasks. A wild egg's line stays wild.
func TestSettledEggGrowsAWorker(t *testing.T) {
	lvl, _ := hiveTestLevel(t)
	egg, err := factory.CreateTyped("xeno_egg", 3, 3, 1, "warrior", "fp1")
	if err != nil {
		t.Fatal(err)
	}
	components.KinOf(egg).Settlement = "hive:site"
	lvl.AddEntity(egg)
	h := factory.SpawnKin(egg, "hatchling", 3, 3, 1)
	if h == nil || h.Blueprint != "xeno_hatchling" {
		t.Fatalf("hatchling %v", h)
	}
	if sc := h.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent); sc.Script != nurseryScript || !h.HasComponent(rlcomponents.AIMemory) {
		t.Errorf("settled hatchling runs %s", sc.Script)
	}
	if n := h.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name; n != "Red Xeno Hatchling" {
		t.Errorf("hatchling named %q", n)
	}
	a := factory.SpawnKin(h, "stage2", 3, 3, 1)
	if a == nil || a.Blueprint != "xeno_crew" {
		t.Fatalf("adult %v", a)
	}
	if !components.IsColonyWorker(a, "hive:site") || a.HasComponent(components.ScriptedAI) {
		t.Error("the adult is not a scriptless worker of its settlement")
	}
	d := campaign.LoadFactionData().Doctrine("hive")
	w := d.Types["warrior"]
	wc := a.GetComponent(components.Worker).(*components.WorkerComponent)
	hp := a.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
	ap := a.GetComponent(components.Appearance).(*components.AppearanceComponent)
	k := components.KinOf(a)
	if wc.Role != "guard" || len(wc.AllowedTasks) != 1 || hp.MaxHealth != w.Health || ap.SpriteX != 48+144 || k.Type != "warrior" || k.Presence != "fp1" {
		t.Errorf("adult: role %s tasks %v hp %d sprite %d kin %+v", wc.Role, wc.AllowedTasks, hp.MaxHealth, ap.SpriteX, k)
	}
	if n := a.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name; n != "Red Xeno" {
		t.Errorf("adult named %q (hidden identity: the morph's seen name)", n)
	}

	wild, _ := factory.Create("xeno_egg", 5, 5, 1)
	wh := factory.SpawnKin(wild, "hatchling", 5, 5, 1)
	if sc := wh.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent); sc.Script == nurseryScript {
		t.Error("a wild hatchling runs the nursery")
	}
	if wa := factory.SpawnKin(wh, "stage2", 5, 5, 1); wa == nil || wa.Blueprint != "xeno_stage2" || wa.HasComponent(components.Worker) {
		t.Errorf("a wild line grew %v", wa)
	}
	if n := wild.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name; n != "Wild Xeno Egg" {
		t.Errorf("wild egg named %q", n)
	}
}

// A kill by a worker of a settlement with kill drops (the hive) puts them in
// its bag: every corpse is biomass.
func TestHiveKillDropsBiomass(t *testing.T) {
	lvl, _ := hiveTestLevel(t)
	d := campaign.LoadFactionData().Doctrine("hive")
	old := d.Settlement
	if d.Settlement == nil {
		d.Settlement = &campaign.DoctrineSettlement{KillDrops: map[string]int{"biomass": 3}}
		t.Cleanup(func() { d.Settlement = old })
	}
	drone, err := factory.Create("xeno_crew", 3, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	enrolHiveWorker(drone, "hive:site")
	lvl.AddEntity(drone)
	victim, err := factory.Create("colonist", 4, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	lvl.AddEntity(victim)
	grantKillDrops(lvl, victim)
	n := 0
	for _, it := range drone.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
		if it.Blueprint == "biomass" {
			n += it.GetComponent(components.Material).(*components.MaterialComponent).Quantity
		}
	}
	if n != d.Settlement.KillDrops["biomass"] {
		t.Errorf("killer carries %d biomass, want %d", n, d.Settlement.KillDrops["biomass"])
	}
}

// spawnHiveAdult is a crew body of typ of presence fp1 in settlement
// hive:site.
func spawnHiveAdult(t *testing.T, lvl *world.Level, bp, typ string, x int) *ecs.Entity {
	t.Helper()
	e, err := factory.Create(bp, x, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	enrolHiveWorker(e, "hive:site")
	factory.ApplyType(e, typ, "fp1")
	lvl.AddEntity(e)
	return e
}

// Known upgrades reach new spawns (multipliers, and the evolved frame with
// the type's second upgrade) and, when researched, the living: a settlement
// that knows Warrior Carapace II draws its warriors in the stage-3 frame
// with 1.56x health.
func TestHiveUpgrades(t *testing.T) {
	lvl, st := hiveTestLevel(t)
	if research.AllTechs()["hive_warrior_carapace_2"].Evolve == nil {
		if err := research.Load("data/research.json"); err != nil {
			t.Fatal(err)
		}
	}
	presenceSettlement["fp1"] = st.Name
	t.Cleanup(func() { delete(presenceSettlement, "fp1") })
	d := campaign.LoadFactionData().Doctrine("hive")
	w := d.Types["warrior"]

	old := spawnHiveAdult(t, lvl, "xeno_crew", "warrior", 2)
	if hp := old.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).MaxHealth; hp != w.Health {
		t.Fatalf("fresh warrior hp %d", hp)
	}
	for _, k := range []string{"hive_warrior_brood", "hive_warrior_carapace_1", "hive_warrior_carapace_2"} {
		st.UnlockTech(k)
		retypeSettlement(lvl, st.Name, k)
	}
	fresh := spawnHiveAdult(t, lvl, "xeno_crew", "warrior", 4)
	for name, e := range map[string]*ecs.Entity{"living": old, "new": fresh} {
		hp := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).MaxHealth
		str := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent).Str
		x := e.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX
		if hp != 50 || str != 19 || x != 96+144 {
			t.Errorf("%s warrior: hp %d (want 32 x 1.5625 = 50), str %d (want 19), sprite x %d (want the red stage-3 frame 240)", name, hp, str, x)
		}
	}
	drone := spawnHiveAdult(t, lvl, "xeno_crew", "drone", 6)
	if x := drone.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX; x != 48 {
		t.Errorf("a drone took the warrior's evolution: sprite %d", x)
	}
	st.UnlockTech("hive_drone_vigour")
	retypeSettlement(lvl, st.Name, "hive_drone_vigour")
	wc := drone.GetComponent(components.Worker).(*components.WorkerComponent)
	if hp := drone.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).MaxHealth; hp != 18 || wc.WorkMult != 1.25 {
		t.Errorf("drone vigour: hp %d (want 18), work %.2f", hp, wc.WorkMult)
	}
	// Fliers get the wing membrane's speed.
	scout := spawnHiveAdult(t, lvl, "troid_crew", "scout", 7)
	st.UnlockTech("hive_wing_membrane")
	retypeSettlement(lvl, st.Name, "hive_wing_membrane")
	if ic := scout.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent); ic.DefaultValue != 5 {
		t.Errorf("scout initiative %d after the wing membrane, want 6 x 0.8 = 5", ic.DefaultValue)
	}
}

// The Factions tab's hive row lists units by type: every type by its name in
// an observer run; in a normal campaign only the site's morph's, by the names
// they are seen by (hidden identity).
func TestHiveUnitsLine(t *testing.T) {
	d := campaign.LoadFactionData().Doctrine("hive")
	p := &campaign.FactionPresence{Units: map[string]int{"drone": 5, "warrior": 2, "scout": 3, "hunter": 1}, Larvae: 2}
	xeno := &campaign.Location{ScenarioID: "earth_like"}
	prev := observerRunActive
	t.Cleanup(func() { observerRunActive = prev })
	observerRunActive = false
	if got := unitsLine(d, p, xeno); got != "Xeno 5, Red Xeno 2" {
		t.Errorf("normal campaign: %q", got)
	}
	observerRunActive = true
	if got := unitsLine(d, p, xeno); got != "5 drone, 1 hunter, 3 scout, 2 warrior, 2 larvae" {
		t.Errorf("observer run: %q", got)
	}
}
