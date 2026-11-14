package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlsystems"
	"github.com/mechanical-lich/mlge/ecs"
)

// On hive resin the hive's walkers act a third more often and everyone else
// takes 40% longer per step; fliers and eggs are not touched; off resin
// nothing changes.
func TestResinSpeed(t *testing.T) {
	lvl := world.NewLevel(8, 8, 3)
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			floor := "hive_resin"
			if x >= 6 {
				floor = "alien_ground"
			}
			lvl.SetFloor(x, y, 1, floor, 0)
		}
	}
	make := func(bp string, x int) *ecs.Entity {
		e, err := factory.Create(bp, x, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		lvl.AddEntity(e)
		return e
	}
	sys := tileInitiative{&rlsystems.InitiativeSystem{Speed: 1}}
	refill := func(e *ecs.Entity) int {
		ic := e.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent)
		ic.Ticks = 1
		e.RemoveComponent(rlcomponents.MyTurn)
		if err := sys.UpdateEntity(lvl, e); err != nil {
			t.Fatal(err)
		}
		return ic.Ticks
	}
	cases := []struct {
		bp   string
		x    int
		want float64
	}{
		{"xeno_crew", 1, 0.75}, // the hive, walking
		{"colonist", 2, 1.4},   // everyone else
		{"troid_crew", 3, 1},   // fliers ignore it
		{"xeno_egg", 4, 1},     // eggs do not walk
		{"colonist", 6, 1},     // off resin
		{"xeno_stage3", 7, 1},  // off resin
	}
	for _, c := range cases {
		e := make(c.bp, c.x)
		base := e.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent).DefaultValue
		want := int(float64(base)*c.want + 0.5)
		if got := refill(e); got != want {
			t.Errorf("%s at x %d: refilled to %d, want %d (base %d)", c.bp, c.x, got, want, base)
		}
	}
}

// Resin speeds the hive's walkers by the doctrine their settlement runs, not
// the faction string on the body: a drone of a player hive (faction "player",
// settlement "player:<loc>", the hive doctrine) walks as fast as a rival's
// (before phase F the table was keyed on the faction, so it walked at 1).
func TestResinSpeedByDoctrine(t *testing.T) {
	t.Cleanup(func() { setFactionDoctrines(nil) })
	setFactionDoctrines(&campaign.Campaign{Player: &campaign.FactionPresence{Faction: "player", Doctrine: "hive"}})
	lvl := world.NewLevel(4, 4, 3)
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			lvl.SetFloor(x, y, 1, "hive_resin", 0)
		}
	}
	e, err := factory.Create("xeno_crew", 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	lvl.AddEntity(e)
	enrolHiveWorker(e, "player:start")
	if f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; f != "player" {
		t.Fatalf("faction %q, want player", f)
	}
	if got := tileSpeed(lvl, e); got != 0.75 {
		t.Errorf("player hive drone on resin: speed %v, want 0.75", got)
	}
}
