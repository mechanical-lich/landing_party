package game

import (
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// autonzHubSite is a moon site with a materialised autonz hub knowing techs.
func autonzHubSite(t *testing.T, techs ...string) (*MainState, *campaign.Location, *campaign.Campaign, *campaign.FactionPresence) {
	t.Helper()
	useContentTables(t)
	ms, loc, c := autonzTestSite(t, "moon", "", 11, campaign.FactionPresence{Population: 8, Stage: 1, LevelStage: 0,
		Conversion: 0.5, KnownTechs: techs})
	materialiseFactions(c, loc, ms.level, ms.scenario)
	return ms, loc, c, loc.PresenceByID("fpa")
}

// A hub materialises with its Signal Core, cogitator, relay pylon and (its
// pattern known) foundry, every structure on wire within the radius, its
// bodies by type with bronze enforcers once that tier is known; a second
// materialise adds nothing.
func TestAutonzMaterialiseHub(t *testing.T) {
	ms, loc, c, p := autonzHubSite(t, "autonz_relay_pattern", "autonz_foundry_pattern", "autonz_bronze_enforcer")
	d := autonzDoc(t)
	cx, cy, cz, ok := originOf(ms.level, p.ID)
	if !ok {
		t.Fatal("no centre")
	}
	built := autonzBuilt(ms.level, d, cx, cy, cz)
	want := map[string]int{"signal_core": 1, "cogitator": 1, "relay_pylon": 1, "foundry": 1}
	for k, v := range want {
		if built[k] != v {
			t.Errorf("%s: %d standing, want %d (%v)", k, built[k], v, built)
		}
	}
	if built["converter_node"] != 0 || built["defence_pylon"] != 0 {
		t.Errorf("structures without their pattern: %v", built)
	}
	for _, k := range autonzStructureKeys(d) {
		for _, e := range hiveStructures(ms.level, k, cx, cy, cz, autonzRadius(d)+2) {
			pc, _ := entityPos(e)
			if f := floorNameAt(ms.level, pc.GetX(), pc.GetY(), pc.GetZ()); f != "blue_wires" && f != "red_wires" {
				t.Errorf("%s at %d,%d stands on %q", k, pc.GetX(), pc.GetY(), f)
			}
		}
	}
	name := factionSettlementName(p, loc.ID)
	workers := settlementWorkers(ms.level, name)
	if len(workers) != 8 {
		t.Fatalf("%d workers, want 8", len(workers))
	}
	for _, e := range workers {
		k := components.KinOf(e)
		if k == nil || k.Type == "" {
			t.Fatalf("untyped worker %s", e.Blueprint)
		}
		if wantTier := map[bool]string{true: "bronze", false: "steel"}[k.Type == "enforcer"]; k.Tier != wantTier {
			t.Errorf("%s tier %q, want %q", k.Type, k.Tier, wantTier)
		}
	}
	units := map[string]int{}
	for _, e := range workers {
		units[components.KinOf(e).Type]++
	}
	if !reflect.DeepEqual(units, p.Units) {
		t.Errorf("level units %v, record %v", units, p.Units)
	}
	// Idempotent.
	before := len(ms.level.Entities) + len(ms.level.StaticEntities)
	materialiseFactions(c, loc, ms.level, ms.scenario)
	if after := len(ms.level.Entities) + len(ms.level.StaticEntities); after != before {
		t.Errorf("a second materialise changed the level: %d entities, then %d", before, after)
	}
	// Reconcile round-trips units and built.
	p.Units, p.Built = nil, nil
	reconcileFactions(c, loc, ms.level)
	p = loc.PresenceByID("fpa")
	if !reflect.DeepEqual(p.Units, units) || !reflect.DeepEqual(p.Built, built) {
		t.Errorf("reconcile: units %v built %v, want %v %v", p.Units, p.Built, units, built)
	}
}

// Materialise is deterministic: the same presence on the same level places
// the same structures in the same places.
func TestAutonzMaterialiseDeterministic(t *testing.T) {
	place := func() map[string][2]int {
		ms, _, _, p := autonzHubSite(t, "autonz_relay_pattern", "autonz_foundry_pattern")
		d := autonzDoc(t)
		cx, cy, cz, _ := originOf(ms.level, p.ID)
		out := map[string][2]int{}
		for _, k := range autonzStructureKeys(d) {
			for _, e := range hiveStructures(ms.level, k, cx, cy, cz, autonzRadius(d)+2) {
				pc, _ := entityPos(e)
				out[k] = [2]int{pc.GetX() - cx, pc.GetY() - cy}
			}
		}
		return out
	}
	a, b := place(), place()
	if !reflect.DeepEqual(a, b) || len(a) == 0 {
		t.Errorf("two materialises: %v and %v", a, b)
	}
}

// A Phase E crew (untyped, the old sprites) is adopted once: each body is
// replaced by its type's print with the new sprite.
func TestAutonzAdoptsPhaseEBodies(t *testing.T) {
	ms, loc, c, p := autonzHubSite(t)
	name := factionSettlementName(p, loc.ID)
	// Make one worker a Phase E body: no type, the antenna walker's sprite.
	old := settlementWorkers(ms.level, name)[0]
	components.KinOf(old).Type = ""
	old.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX = 0
	bp := old.Blueprint
	materialiseFactions(c, loc, ms.level, ms.scenario)
	for _, e := range settlementWorkers(ms.level, name) {
		if e == old {
			t.Fatal("the Phase E body is still on the level")
		}
		if k := components.KinOf(e); k == nil || k.Type == "" {
			t.Errorf("untyped %s after adoption", e.Blueprint)
		}
	}
	base, _ := factory.Create(bp, 0, 0, 0)
	want := base.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX
	n := 0
	for _, e := range settlementWorkers(ms.level, name) {
		if e.Blueprint == bp && e.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX == want {
			n++
		}
	}
	if n == 0 {
		t.Errorf("no %s at x %d after adoption", bp, want)
	}
	if got := len(settlementWorkers(ms.level, name)); got != 8 {
		t.Errorf("%d workers after adoption, want 8", got)
	}
}

// The live signal needs the Signal Core and the transmit rung's bodies at
// it; its range counts the relay pylons standing.
func TestAutonzLiveSignalNeedsCoreAndBodies(t *testing.T) {
	ms, loc, c, p := autonzHubSite(t, "autonz_relay_pattern")
	d := autonzDoc(t)
	if got := autonzPylons(ms.level, d, p); got != 1 {
		t.Fatalf("%d pylons", got)
	}
	if d.SignalRange(1, nil, 1) != 20 || d.SignalRange(1, nil, 0) != 16 || d.SignalRange(1, nil, -1) != 20 {
		t.Errorf("signal range %v / %v / %v", d.SignalRange(1, nil, 1), d.SignalRange(1, nil, 0), d.SignalRange(1, nil, -1))
	}
	// A site in range, the cooldown long past.
	near := &campaign.Location{ID: "near", Name: "Near", Kind: "planet", ScenarioID: "earth_like", Tags: []string{"ore"}, X: loc.X + 3, Y: loc.Y, Discovered: true, Visited: true}
	c.Locations[near.ID] = near
	p.Conversion = 0.7
	applyPresenceConversion(ms.level, loc, d, p) // past the signal conversion on the level too
	name := factionSettlementName(p, loc.ID)
	// No bodies held at the core: no signal.
	if stepLiveAutonzSettlement(c, loc, ms.level, d, p, nil) {
		t.Fatal("signalled without the transmit bodies")
	}
	// Two placers held at the core: it signals and takes them.
	core := autonzCoreOf(ms.level, d, p)
	if core == nil {
		t.Fatal("no core")
	}
	cp, _ := entityPos(core)
	held := 0
	for _, e := range settlementWorkers(ms.level, name) {
		if held == 2 {
			break
		}
		pc, _ := entityPos(e)
		x, y, z := freeLandingTile(ms.level, cp.GetX()+1, cp.GetY(), cp.GetZ())
		ms.level.PlaceEntity(x, y, z, e)
		_ = pc
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		tk := &task.Task{Action: task_requests.MoveAction, X: x, Y: y, Z: z, Data: task_requests.HoldHere}
		tk.Start()
		wc.CurrentTask = tk
		held++
	}
	before := len(settlementWorkers(ms.level, name))
	// With the core destroyed it waits.
	ms.level.RemoveEntity(core)
	if stepLiveAutonzSettlement(c, loc, ms.level, d, p, nil) {
		t.Fatal("signalled with the core destroyed")
	}
	ms.level.AddEntity(core)
	if !stepLiveAutonzSettlement(c, loc, ms.level, d, p, nil) {
		t.Fatalf("no signal with the core and two bodies (pop %d, conv %.2f)", p.Population, p.Conversion)
	}
	if after := len(settlementWorkers(ms.level, name)); after != before-2 {
		t.Errorf("%d workers after the signal, want %d", after, before-2)
	}
	if near.Presence("autonz") == nil {
		t.Error("no dormant presence on the site in range")
	}
	var _ *ecs.Entity
}
