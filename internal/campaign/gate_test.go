package campaign

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// gateRun steps a campaign of size with only a rival colony (and no
// player) day by day to maxDays, and returns the day its faction learned
// the gate, the day the gate came on and the day its last ship left (-1:
// never).
func gateRun(t *testing.T, seed int64, size string, maxDays int) (known, on, left int, trace string) {
	t.Helper()
	c, err := GenerateCampaignOpts("Gate", seed, GenOptions{Size: size, Rivals: []RivalPick{{Doctrine: RivalColonyDoctrine}}})
	if err != nil {
		t.Fatal(err)
	}
	known, on, left = -1, -1, -1
	key := ""
	for _, f := range c.Fleets {
		if c.ShipSeeksGate(&f) {
			key = f.Presence.Faction
		}
	}
	if key == "" {
		t.Fatalf("seed %d %s: no gate-seeking ship seeded", seed, size)
	}
	var b strings.Builder
	prev := ""
	for day := 1; day <= maxDays; day++ {
		c.StarDate = history.StarDate(day)
		c.StepFactions(nil)
		if known < 0 && c.gateKnown(key) {
			known = day
		}
		if on < 0 && c.GateIsOn() {
			on = day
		}
		ships := 0
		for i := range c.Fleets {
			f := &c.Fleets[i]
			if f.Presence.Faction == key {
				ships++
				if st := f.LandedID + ">" + f.ToID; st != prev {
					prev = st
					fmt.Fprintf(&b, " d%d %s fuel %d %s;", day, st, f.Presence.Fuel, f.LastMove)
				}
			}
		}
		if ships == 0 {
			left = day
			break
		}
	}
	return known, on, left, b.String()
}

// A rival colony alone in a small and a medium galaxy finds the gate,
// switches it on at its controls and leaves through it, within a day bound.
func TestRivalColonyLeavesThroughTheGate(t *testing.T) {
	useRepoData(t)
	const bound = 500
	for _, size := range []string{"small", "medium"} {
		var days []int
		for seed := int64(1); seed <= 6; seed++ {
			known, on, left, trace := gateRun(t, seed, size, bound)
			t.Logf("%s seed %d: knows the gate day %d, switches it on day %d, leaves day %d", size, seed, known, on, left)
			if left < 0 || on < 0 || on > left {
				t.Errorf("%s seed %d: never left within %d days (knew %d, on %d):%s", size, seed, bound, known, on, trace)
				continue
			}
			days = append(days, left)
		}
		if len(days) > 0 {
			sum := 0
			lo, hi := math.MaxInt, 0
			for _, d := range days {
				sum += d
				lo, hi = min(lo, d), max(hi, d)
			}
			t.Logf("%s: leaves on day %d to %d, mean %d", size, lo, hi, sum/len(days))
		}
	}
}

// The gate's rules: identifying it identifies its controls world and says
// so; it comes on once, for good, with a Legends line (and an alert for a
// rival's doing); leaving is offered only beside the lit gate with the
// leaving fuel, and ends the run as a win.
func TestGateRules(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 4)
	gate, ctl := c.GateLocation(), c.ControlsLocation()
	if gate.Identified || ctl.Identified {
		t.Fatal("gate or controls identified at the start")
	}
	c.Identify(gate)
	if !ctl.Identified || !strings.Contains(gate.Summary, ctl.Name) {
		t.Fatalf("identifying the gate: controls identified %v, summary %q", ctl.Identified, gate.Summary)
	}
	vault := false
	for _, f := range ctl.Fixtures {
		vault = vault || f.Kind == GateControlsKind && f.Structure == GateVaultStructure
	}
	if !vault || c.LocationTier(ctl) == history.TierThreat {
		t.Fatalf("controls world: vault %v, tier %s", vault, c.LocationTier(ctl))
	}
	c.CurrentLocationID = gate.ID
	if ok, _ := c.CanLeave(1000); ok {
		t.Fatal("leaving offered with the gate dark")
	}
	before := c.KnownEventCount()
	if !c.SwitchGateOn("mirror") || c.SwitchGateOn(GateByPlayer) {
		t.Fatal("the gate should come on once")
	}
	if c.Galaxy.GateOnBy != "mirror" || len(c.News) != 1 || c.KnownEventCount() != before+1 {
		t.Errorf("by %q news %v events %d->%d", c.Galaxy.GateOnBy, c.News, before, c.KnownEventCount())
	}
	if ok, _ := c.CanLeave(LeaveFuel() - 1); ok {
		t.Fatal("leaving offered short of the leaving fuel")
	}
	c.CurrentLocationID = ctl.ID
	if ok, _ := c.CanLeave(LeaveFuel()); ok {
		t.Fatal("leaving offered away from the gate")
	}
	c.CurrentLocationID = gate.ID
	if ok, why := c.CanLeave(LeaveFuel()); !ok {
		t.Fatalf("leaving refused beside the lit gate: %s", why)
	}
	c.LeaveGalaxy()
	if !c.Won {
		t.Fatal("leaving did not end the run as a win")
	}
}
