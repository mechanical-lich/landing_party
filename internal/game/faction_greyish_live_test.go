package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// holdOff keeps a grey ship down for a test: its stay long and its hold
// target out of reach.
func holdOff(t *testing.T) *campaign.Doctrine {
	t.Helper()
	d := greyishDoctrineT(t)
	stay, hold := d.Mobility.StayDays, d.Ships.HoldTarget
	d.Mobility.StayDays, d.Ships.HoldTarget = 1000, 1000
	t.Cleanup(func() { d.Mobility.StayDays, d.Ships.HoldTarget = stay, hold })
	return d
}

// greyStore puts items into the settlement's pocket.
func greyStore(t *testing.T, ms *MainState, name string, items ...string) {
	t.Helper()
	_, scs := levelContainers(ms.level)
	var door *components.StorageComponent
	for _, sc := range scs {
		if sc.OwnedBy == name {
			door = sc
			break
		}
	}
	if door == nil {
		t.Fatal("no grey door")
	}
	for _, bp := range items {
		it, err := factory.Create(bp, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !door.AddItem(it) {
			t.Fatalf("no room for %s", bp)
		}
	}
}

// The arm rung swaps a soldier's beam pistol for a stored beam rifle (best)
// and puts a stored field harness on it (armour).
func TestGreyArmRung(t *testing.T) {
	holdOff(t)
	_, ms, loc, f := greyLanded(t, 5, 3, 0, []string{"grey_beam_weapons"}, 0, nil, map[string]int{"grey_beam_pistol": 1})
	name := factionSettlementName(&f.Presence, loc.ID)
	greyStore(t, ms, name, "grey_beam_rifle", "grey_field_harness")
	var soldier *ecs.Entity
	for _, e := range settlementWorkers(ms.level, name) {
		if k := components.KinOf(e); k != nil && k.Type == "grey_soldier" {
			soldier = e
		}
	}
	if soldier == nil {
		t.Fatal("no soldier")
	}
	stepLevel(ms, 900, nil)
	inv := soldier.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	rifle := false
	for _, it := range wornGear(inv) {
		if it.Blueprint == "grey_beam_rifle" {
			rifle = true
		}
	}
	if !rifle || inv.Torso == nil || inv.Torso.Blueprint != "grey_field_harness" {
		t.Errorf("the soldier holds a rifle %v, wears %v: want the rifle and the harness (orders %v)", rifle, inv.Torso, ms.factionDirectors[name].Ordered)
	}
}

// The array is built only on a long stay at the freighter tier or above:
// with ten days left a freighter does not start one, with twenty it does.
func TestGreyStayLeftGatesTheArray(t *testing.T) {
	for _, tc := range []struct {
		landedAgo int
		want      bool
	}{{20, false}, {2, true}} {
		d := greyishDoctrineT(t)
		hold := d.Ships.HoldTarget
		d.Ships.HoldTarget = 1000
		techs := []string{"grey_research_array", "grey_armory_module"}
		_, ms, loc, f := greyLanded(t, 6, 5, 1, techs, tc.landedAgo, nil, nil)
		f.Presence.Built = map[string]int{} // nothing standing: the live director's own choice
		name := factionSettlementName(&f.Presence, loc.ID)
		for _, e := range greyStanding(ms.level, d, "grey_array", 0, 0, 0) {
			ms.level.RemoveEntity(e)
		}
		cx, cy, cz, _ := originOf(ms.level, f.Presence.ID)
		for _, e := range greyStanding(ms.level, d, "grey_array", cx, cy, cz) {
			ms.level.RemoveEntity(e)
		}
		greyStore(t, ms, name, "grey_alloy", "grey_alloy", "grey_alloy", "grey_alloy")
		fillFactionStock(ms.level, name, map[string]int{"metal_ore": 30, "crystal": 12, "plastic": 40}, cx, cy, cz)
		stepLevel(ms, 3000, nil)
		got := ms.factionDirectors[name].Ordered["build:grey_array"] > 0
		left := d.Mobility.StayDays - int(ms.campaign.FactionDay-f.LandedDay)
		if got != tc.want {
			t.Errorf("%d days of the stay left: array ordered %v, want %v (log %v)", left, got, tc.want, ms.factionDirectors[name].Log)
		}
		d.Ships.HoldTarget = hold
	}
}

// Smashing every grey locker spills the pocket; a day later the ship has
// its cockpit door back and its workers haul what is left of the spill
// into it.
func TestGreyDoorRestoredAndSpillHauled(t *testing.T) {
	holdOff(t)
	_, ms, loc, f := greyLanded(t, 7, 3, 0, nil, 0, nil, nil)
	name := factionSettlementName(&f.Presence, loc.ID)
	es, scs := levelContainers(ms.level)
	for i, sc := range scs {
		if sc.OwnedBy == name {
			ms.level.RemoveEntity(es[i])
			pocketDoorLost(ms.level, es[i])
		}
	}
	if storeOf(ms, name)["metal_ore"] != 0 {
		t.Fatal("the pocket still holds ore with no door")
	}
	loose := 0
	for _, e := range ms.level.Entities {
		if e != nil && e.Blueprint == "metal_ore" {
			loose++
		}
	}
	if loose == 0 {
		t.Fatal("nothing spilled")
	}
	stepLevel(ms, 600, nil)
	if n := doors(ms, name); n != 0 {
		t.Fatalf("a door is back after 600 ticks (%d): the window is a day", n)
	}
	stepLevel(ms, 2400, nil)
	if n := doors(ms, name); n == 0 {
		t.Fatal("no door re-placed a day later")
	}
	// Hauled back in: none of the spill's ore lies loose any more (the camp
	// may have spent it since).
	if ms.factionDirectors[name].Ordered["haul:metal_ore"] == 0 {
		t.Errorf("the spill's ore was never hauled (orders %v)", ms.factionDirectors[name].Ordered)
	}
	for _, e := range ms.level.Entities {
		if e != nil && e.Blueprint == "metal_ore" && e.HasComponent(rlcomponents.Position) {
			t.Errorf("spilled ore still loose at %v", e.GetComponent(rlcomponents.Position))
			break
		}
	}
}

func doors(ms *MainState, name string) int {
	n := 0
	_, scs := levelContainers(ms.level)
	for _, sc := range scs {
		if sc.OwnedBy == name {
			n++
		}
	}
	return n
}

func storeOf(ms *MainState, name string) map[string]int {
	out := map[string]int{}
	_, scs := levelContainers(ms.level)
	for _, sc := range scs {
		if sc.OwnedBy != name {
			continue
		}
		for _, it := range sc.Items {
			q := 1
			if it.HasComponent(components.Material) {
				q = it.GetComponent(components.Material).(*components.MaterialComponent).Quantity
			}
			out[it.Blueprint] += q
		}
	}
	return out
}

// Materialise and the director agree (greyish_plan.md, "Materialise and
// reconcile"): a freighter landed abstractly with the array known holds the
// same kinds and counts of ground structure as one landed live with the
// hold to pay for them and run by its director over the same days.
func TestGreyMaterialiseAgreesWithDirector(t *testing.T) {
	d := holdOff(t)
	techs := []string{"grey_research_array"}
	const days = 6
	_, abstract, _, _ := greyLanded(t, 8, 5, 1, techs, days, nil, nil)
	want := d.ShipBuilt(1, techs, days+3) // the trip there is three days
	modules := d.ShipModules(1, techs)
	ax, ay, az, _ := originOf(abstract.level, "fpg")
	gotAbstract := greyGround(abstract.level, d, modules, ax, ay, az)
	_, ms, loc, f := greyLanded(t, 8, 5, 1, techs, 0, nil, nil)
	name := factionSettlementName(&f.Presence, loc.ID)
	cx, cy, cz, _ := originOf(ms.level, f.Presence.ID)
	for _, k := range d.Settlement.Ground {
		for _, e := range greyStanding(ms.level, d, k, cx, cy, cz)[modules[k]:] {
			ms.level.RemoveEntity(e) // the ground ones; the carried ones stay
		}
	}
	fillFactionStock(ms.level, name, map[string]int{"metal_ore": 60, "crystal": 30, "grey_alloy": 10, "plastic": 40}, cx, cy, cz)
	f.LandedDay = ms.campaign.FactionDay
	// Twice the days: the live camp also gathers biomass and presses plastic
	// for the array (since 2026-11-12); ShipBuilt has its one armory and one
	// array by day 2 either way.
	stepLevel(ms, 2*days*int(history.TicksPerDay), nil)
	gotLive := greyGround(ms.level, d, modules, cx, cy, cz)
	for _, k := range d.Settlement.Ground {
		live := gotLive[k]
		if gotAbstract[k] != want[k] || live != want[k] {
			t.Errorf("%s: materialised %d, ShipBuilt %d, the live director built %d (orders %v)", k, gotAbstract[k], want[k], live, ms.factionDirectors[name].Ordered)
		}
	}
	t.Logf("ground structures: %v", want)
}

// The mothership beams materials down (the owner's call, 2026-11-12): once
// a day a grey camp on a loaded level is topped up to the beam floor, from
// the mothership's beam stock first and then its hoard; a camp at the floor
// gets nothing; with no mothership, nothing comes down.
func TestGreyMothershipBeamsDown(t *testing.T) {
	d := holdOff(t)
	wm, ms, loc, f := greyLanded(t, 9, 3, 0, nil, 0, nil, nil)
	name := factionSettlementName(&f.Presence, loc.ID)
	beam := d.Ships.Mothership.Beam
	if beam == nil || beam.Floor["metal_ore"] <= 0 {
		t.Fatal("no beam floor in the doctrine")
	}
	if got := wm.beamDownToCamp(f, d); got != nil {
		t.Fatalf("beamed %v with no mothership", got)
	}
	c := wm.Campaign
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{ID: "flms", Kind: campaign.FleetShip, Mothership: true, FromID: "far", ToID: "far",
		Presence: campaign.FactionPresence{ID: "fpms", Faction: "greyish", Doctrine: "greyish", Population: 10,
			BeamStock: map[string]int{"metal_ore": 20, "crystal": 2}, Stockpile: map[string]int{"crystal": 50}}})
	f = c.FleetByPresence("fpg")
	_, scs := levelContainers(ms.level)
	for _, sc := range scs {
		if sc.OwnedBy == name {
			sc.Items = nil
		}
	}
	got := wm.beamDownToCamp(f, d)
	if got["metal_ore"] != beam.Floor["metal_ore"] || got["crystal"] != beam.Floor["crystal"] {
		t.Errorf("beamed %v, want the floor %v", got, beam.Floor)
	}
	if st := storeOf(ms, name); st["metal_ore"] != beam.Floor["metal_ore"] || st["crystal"] != beam.Floor["crystal"] {
		t.Errorf("the camp holds %v after the beam", st)
	}
	m := c.Mothership("greyish")
	if m.Presence.BeamStock["metal_ore"] != 20-beam.Floor["metal_ore"] || m.Presence.BeamStock["crystal"] != 0 ||
		m.Presence.Stockpile["crystal"] != 50-(beam.Floor["crystal"]-2) {
		t.Errorf("the beam stock %v and hoard %v: the stock first, then the hoard", m.Presence.BeamStock, m.Presence.Stockpile)
	}
	if again := wm.beamDownToCamp(f, d); again != nil {
		t.Errorf("a camp at the floor got %v", again)
	}
	_ = ms
}

// A lancer is printed for each standing emitter once the lancer pattern is
// known (the owner's call, 2026-11-12), over the crew cap (which the crew
// fills first), and no more than one an emitter.
func TestGreyLancerPerEmitter(t *testing.T) {
	d := holdOff(t)
	// holdOff's huge hold target would make every rung's reserve (a share of
	// it) unpayable: no reserve for this test.
	pr := d.Settlement.Director.Priorities
	saved := make([]float64, len(pr))
	for i := range pr {
		saved[i], pr[i].Reserve = pr[i].Reserve, 0
	}
	t.Cleanup(func() {
		for i := range pr {
			pr[i].Reserve = saved[i]
		}
	})
	techs := []string{"grey_field_emitter", "grey_beam_weapons", "grey_skimmer_pattern", "grey_lancer_pattern"}
	units := map[string]int{"grey_worker": 3, "grey_technician": 2, "grey_soldier": 2}
	_, ms, loc, f := greyLanded(t, 10, 7, 2, techs, 0, units, nil)
	name := factionSettlementName(&f.Presence, loc.ID)
	cx, cy, cz, _ := originOf(ms.level, f.Presence.ID)
	fillFactionStock(ms.level, name, map[string]int{"metal_ore": 30, "crystal": 20, "grey_alloy": 10, "specimen": 4}, cx, cy, cz)
	if n := countBP(nil, ms)["grey_emitter"]; n != 2 {
		t.Fatalf("%d emitters on a warship with the field emitter", n)
	}
	lancers := func() int { return typesOf(ms, name)["grey_lancer"] }
	for i := 0; i < 20 && lancers() == 0; i++ {
		stepLevel(ms, 750, nil)
	}
	if lancers() == 0 {
		t.Fatalf("no lancer printed for the emitters (orders %v)", ms.factionDirectors[name].Ordered)
	}
	for i := 0; i < 20 && lancers() < 2; i++ {
		stepLevel(ms, 750, nil)
	}
	if n := lancers(); n > 2 {
		t.Errorf("%d lancers for 2 emitters", n)
	}
	if n := len(settlementWorkers(ms.level, name)) - lancers(); n > 8 {
		t.Errorf("%d crew besides the lancers, over the warship's cap 8", n)
	}
}
