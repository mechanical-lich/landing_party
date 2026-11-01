package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/combat"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/event"
)

// bgAway puts the player in orbit of "live" with staffed planets away.
func bgAway(t *testing.T, away ...string) (*WorldManager, *MainState) {
	t.Helper()
	wm := bgWorld(t, append([]string{"live"}, away...))
	bgTravel(t, wm, away...)
	bgTravel(t, wm, "live")
	return wm, wm.Current()
}

func TestOwedTicksPaidAtBudgetPerFrame(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	wm.owed["a"] = 10
	before := a.tick
	wm.TickBackground(live)
	if got := a.tick - before; got != 2 {
		t.Errorf("advanced %d ticks in a frame, want 2", got)
	}
	if wm.owed["a"] != 9 { // 10, plus 1 accrued, minus 2 paid
		t.Errorf("owed = %d, want 9", wm.owed["a"])
	}
	if live.tick != 0 {
		t.Errorf("scheduler stepped the live planet")
	}
}

// Keeps up with one planet: it accrues 1 and is paid up to 2.
func TestOnePlanetRunsAtRealTime(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	before := a.tick
	for i := 0; i < 10; i++ {
		wm.TickBackground(live)
	}
	if got := a.tick - before; got != 10 {
		t.Errorf("advanced %d in 10 frames, want 10", got)
	}
}

func TestOwedCapDropsExcess(t *testing.T) {
	wm, live := bgAway(t, "a")
	wm.owed["a"] = 60
	wm.TickBackground(live)
	if wm.owed["a"] != 58 { // at the cap, so the accrual was dropped; 2 paid
		t.Errorf("owed = %d, want 58", wm.owed["a"])
	}
}

func TestPauseStopsAccrualAndTicks(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	shipBefore, aBefore := wm.shipLevel.tick, a.tick
	wm.SetPaused(true)
	wm.TickBackground(live)
	if wm.owed["a"] != 0 || a.tick != aBefore || wm.shipLevel.tick != shipBefore {
		t.Errorf("paused world moved: owed %d, planet +%d, ship +%d", wm.owed["a"], a.tick-aBefore, wm.shipLevel.tick-shipBefore)
	}
}

// A budget smaller than the planets owing is shared, not given to the first.
func TestBudgetRoundRobinsAcrossPlanets(t *testing.T) {
	wm, _ := bgAway(t, "a", "b")
	ids := []string{"a", "b"}
	for _, id := range ids {
		wm.owed[id] = 30
	}
	start := map[string]int{"a": wm.loaded["a"].tick, "b": wm.loaded["b"].tick}
	wm.payOwedTicks(ids, 1)
	wm.payOwedTicks(ids, 1)
	wm.payOwedTicks(ids, 1)
	da, db := wm.loaded["a"].tick-start["a"], wm.loaded["b"].tick-start["b"]
	if da+db != 3 || da == 0 || db == 0 {
		t.Errorf("ticks a=%d b=%d, want 3 shared between both", da, db)
	}
}

func bgRemoveCrew(ms *MainState) {
	for _, e := range append(ms.level.Entities[:0:0], ms.level.Entities...) {
		if components.IsColonyWorker(e, ms.colonyName()) {
			ms.level.RemoveEntity(e)
		}
	}
}

// An away planet that loses its crew stops simulating and freezes, recording zero.
func TestAwayPlanetFreezesWhenCrewIsLost(t *testing.T) {
	wm, live := bgAway(t, "a")
	bgRemoveCrew(wm.loaded["a"])
	wm.TickBackground(live)
	if wm.IsLoaded("a") {
		t.Error("crewless away planet stayed loaded")
	}
	if got := wm.Campaign.Locations["a"].Colonists; got != 0 {
		t.Errorf("recorded %d colonists", got)
	}
}

// The planet in orbit stays loaded without crew while the player is aboard, so
// crew can beam down, and it doesn't tick.
func TestOrbitedPlanetWithoutCrewStaysButIdles(t *testing.T) {
	wm, _ := bgAway(t, "a")
	cur := wm.Current()
	bgRemoveCrew(cur)
	before := cur.tick
	wm.TickBackground(wm.shipLevel)
	if !wm.IsLoaded(wm.currentID) || cur.tick != before {
		t.Errorf("loaded=%v, ticks +%d", wm.IsLoaded(wm.currentID), cur.tick-before)
	}
}

// Robots alone staff a planet and keep the run alive.
func TestRobotOnlyPlanetKeepsRunning(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	bgRemoveCrew(a)
	bgRemoveCrew(live)
	b0Add(t, a, "hauler_robot", a.colonyName(), 3, 3)
	before := a.tick
	wm.TickBackground(live)
	if !wm.IsLoaded("a") || a.tick == before {
		t.Error("robot-only planet stopped")
	}
	if wm.TotalColonists() != 1 {
		t.Errorf("total = %d, want the robot", wm.TotalColonists())
	}
}

func bgDamage(ms *MainState, victim string) eventsystem.EntityDamagedEvent {
	for _, e := range ms.level.Entities {
		if e.Blueprint == victim {
			return eventsystem.EntityDamagedEvent{Victim: e, Amount: 1, Level: ms.level}
		}
	}
	panic("no " + victim)
}

// A sustained fight on an away planet is one incident, however many hits.
func TestAwayDamageIsOneIncident(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	hit := bgDamage(a, "colonist")

	raised := 0
	for i := 0; i < 50; i++ {
		a.level.QueueEvent(hit)
		wm.owed["a"] = 1
		wm.TickBackground(live)
		if wm.ActiveIncident() != nil {
			raised++
			wm.IgnoreIncident()
		}
	}
	if raised != 1 {
		t.Fatalf("%d incidents for one fight", raised)
	}
	// Quiet for the cooldown, then hurt again: a new incident.
	a.tick += incidentCooldown()
	a.level.QueueEvent(hit)
	wm.TickBackground(live)
	if inc := wm.ActiveIncident(); inc == nil || inc.LocationID != "a" {
		t.Errorf("incident after the cooldown = %+v", inc)
	}
}

// Damage on the live level never raises the popup, nor does a hostile being hurt.
func TestLiveAndHostileDamageRaiseNothing(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]

	live.level.QueueEvent(bgDamage(live, "colonist"))
	live.stepWorld()
	if wm.ActiveIncident() != nil {
		t.Errorf("live damage raised %+v", wm.ActiveIncident())
	}

	b0Add(t, a, "mutant_grunt", "raiders:a", 5, 5)
	a.level.QueueEvent(bgDamage(a, "mutant_grunt"))
	wm.TickBackground(live)
	if wm.ActiveIncident() != nil {
		t.Errorf("hostile damage raised %+v", wm.ActiveIncident())
	}
}

// The popup stops the whole clock until answered; Pause leaves it stopped, Ignore
// starts it again.
func TestIncidentStopsTheClock(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	a.level.QueueEvent(bgDamage(a, "colonist"))
	wm.TickBackground(live)
	if wm.ActiveIncident() == nil {
		t.Fatal("no incident")
	}
	before, ship := a.tick, wm.shipLevel.tick
	wm.TickBackground(live)
	if a.tick != before || wm.shipLevel.tick != ship {
		t.Error("background ticked under the popup")
	}
	if !live.clockStopped() {
		t.Error("live level not stopped under the popup")
	}
	wm.PauseIncident()
	if wm.ActiveIncident() != nil || !wm.Paused() || !live.clockStopped() {
		t.Error("Pause should dismiss the popup and leave the clock stopped")
	}
	wm.SetPaused(false)
	if live.clockStopped() {
		t.Error("clock still stopped after resuming")
	}
}

func TestIncidentTravelGoesThereAndPaysFuel(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	wm.Campaign.Locations["a"].X = 50
	wm.incidents = []Incident{{LocationID: "a", Where: "a", Victim: "v"}}
	if _, err := wm.TravelToIncident(); err == nil {
		t.Fatal("travel with no fuel succeeded")
	}
	if wm.ActiveIncident() == nil || wm.currentID != "live" {
		t.Fatal("failed travel should leave the popup and the orbit alone")
	}
	wm.StockShipLocker(1000, nil)
	ms, err := wm.TravelToIncident()
	if err != nil {
		t.Fatalf("travel: %v", err)
	}
	t.Cleanup(ms.teardown)
	if ms != a || wm.currentID != "a" {
		t.Errorf("arrived at %q", wm.currentID)
	}
	if wm.ActiveIncident() != nil {
		t.Error("popup still up after travel")
	}
	if !wm.IsLoaded("live") || live == nil {
		t.Error("staffed planet left behind froze")
	}
}

// Away presentation lines become notifications and repeats merge; live ones don't.
func TestAwayPresentationBecomesNotifications(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	built := eventsystem.StructureBuiltEvent{Type: "wall"}
	a.level.QueueEvent(built)
	a.level.QueueEvent(built)
	wm.owed["a"] = 1
	wm.TickBackground(live)
	got := wm.RecentNotifications(5)
	if len(got) != 1 || got[0].Count != 2 || got[0].LocationID != "a" {
		t.Fatalf("notifications = %+v", got)
	}
	live.level.QueueEvent(built)
	live.stepWorld()
	if len(wm.RecentNotifications(5)) != 1 {
		t.Error("live event became a notification")
	}
}

// The combat path fires the event with the health actually lost.
func TestCombatHitFiresDamageEvent(t *testing.T) {
	wm, _ := bgAway(t, "a")
	a := wm.loaded["a"]
	victim := bgDamage(a, "colonist").Victim
	attacker := b0Add(t, a, "mutant_grunt", "raiders:a", 2, 1)
	var got []eventsystem.EntityDamagedEvent
	a.level.Events.RegisterListener(listenFunc(func(ev eventsystem.EntityDamagedEvent) { got = append(got, ev) }), eventsystem.EntityDamaged)

	hc := victim.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
	hc.MaxHealth, hc.Health = 100000, 100000
	for i := 0; i < 500 && hc.Health == 100000; i++ {
		combat.Hit(a.level, attacker, victim, false)
	}
	if hc.Health == 100000 {
		t.Fatal("no hit landed in 500 tries")
	}
	a.level.Events.HandleQueue()
	if len(got) != 1 || got[0].Victim != victim || got[0].Attacker != attacker || got[0].Amount != 100000-hc.Health || got[0].Level != a.level {
		t.Errorf("events = %+v", got)
	}
}

type listenFunc func(eventsystem.EntityDamagedEvent)

func (f listenFunc) HandleEvent(e event.EventData) error {
	f(e.(eventsystem.EntityDamagedEvent))
	return nil
}

func TestStarMapPauseButtonDrivesTheClock(t *testing.T) {
	wm, _ := bgAway(t, "a")
	p := newStarMapPanel(wm.Campaign, wm)
	if p.pauseBtn.Text != "Running: Pause" {
		t.Errorf("label = %q", p.pauseBtn.Text)
	}
	p.pauseBtn.OnClick()
	if !wm.Paused() || p.pauseBtn.Text != "Paused: Resume" {
		t.Errorf("after click: paused=%v label=%q", wm.Paused(), p.pauseBtn.Text)
	}
	before := wm.loaded["a"].tick
	wm.TickBackground(nil)
	if wm.loaded["a"].tick != before {
		t.Error("paused clock ticked a planet")
	}
	p.pauseBtn.OnClick()
	if wm.Paused() || p.pauseBtn.Text != "Running: Pause" {
		t.Errorf("after second click: paused=%v label=%q", wm.Paused(), p.pauseBtn.Text)
	}
}

// There is one clock: a pause set on the Star Map arrives at the level as its own
// pause, which Space can release, and goes back to the dashboard the same way.
func TestPauseFollowsThePlayer(t *testing.T) {
	wm, live := bgAway(t, "a")
	wm.SetPaused(true)
	live.adoptWorldPause()
	if wm.Paused() || !live.Paused || !live.clockStopped() {
		t.Errorf("after entering: world %v, level %v", wm.Paused(), live.Paused)
	}
	live.Paused = false // Space
	if live.clockStopped() {
		t.Error("Space should release the pause")
	}
	live.Paused = true
	live.openDashboard()
	if !wm.Paused() || live.Paused {
		t.Errorf("after leaving: world %v, level %v", wm.Paused(), live.Paused)
	}
}

func TestRogueTurnsTickBackground(t *testing.T) {
	wm, live := bgAway(t, "a")
	a := wm.loaded["a"]
	before := a.tick
	live.advancePlayerTurn()
	if a.tick == before {
		t.Error("a rogue turn left away planets behind")
	}
}

func TestNotificationsMergeOnlyInQuickSuccession(t *testing.T) {
	wm, _ := bgAway(t, "a")
	a := wm.loaded["a"]
	wm.notify(a, "built")
	wm.notify(a, "built")
	a.tick += notificationMerge()
	wm.notify(a, "built")
	got := wm.RecentNotifications(5)
	if len(got) != 2 || got[0].Count != 2 || got[1].Count != 1 {
		t.Errorf("notifications = %+v", got)
	}
}

// A planet the player returns to doesn't carry a debt into its next time away.
func TestLivePlanetClearsItsOwedTicks(t *testing.T) {
	wm, _ := bgAway(t, "a")
	a := wm.loaded["a"]
	wm.owed["a"] = 40
	wm.TickBackground(a)
	if wm.owed["a"] != 0 {
		t.Errorf("owed = %d, want 0", wm.owed["a"])
	}
}
