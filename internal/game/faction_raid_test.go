package game

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// stepLevel steps ms n ticks as the game's Update does.
func stepLevel(ms *MainState, n int, each func(tick int)) {
	for tick := 1; tick <= n; tick++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		if ms.wm != nil {
			ms.wm.TickBackground(ms)
		}
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		ms.advanceClock()
		if each != nil {
			each(tick)
		}
	}
}

// A hostile rival colony raids: its director sends a party at the player's
// colonist, and its workers take up the attack. A neutral one on the same
// site never does.
func TestHostileRivalRaidsThePlayer(t *testing.T) {
	_, ms, loc := mirrorOnStart(t, 1, 4)
	name := "mirror:" + loc.ID
	// Beaming down onto its site made it hostile (contact); make it
	// neutral again for the first half.
	ms.campaign.SetStance("mirror", campaign.StanceNeutral, loc)
	components.SetFactionHostile("mirror", false)
	t.Cleanup(components.ResetFactionHostility)
	stepLevel(ms, 900, nil)
	if dir := ms.factionDirectors[name]; dir == nil || dir.Ordered["raids"] != 0 {
		t.Fatalf("a neutral rival raided: %v", dir)
	}

	ms.campaign.SetStance("mirror", campaign.StanceHostile, loc)
	components.SetFactionHostile("mirror", true)
	attacking := false
	victim := colonyWorkers(ms)[0]
	hc := victim.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
	hurt := -1
	stepLevel(ms, 6000, func(tick int) {
		if hurt < 0 && hc.Health < hc.MaxHealth {
			hurt = tick
		}
		for _, e := range settlementWorkers(ms.level, name) {
			wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
			if wc.CurrentTask != nil && wc.CurrentTask.Action == task_requests.AttackAction {
				if tgt, ok := wc.CurrentTask.Data.(*ecs.Entity); ok && tgt == victim {
					attacking = true
				}
			}
		}
	})
	dir := ms.factionDirectors[name]
	if dir.Ordered["raids"] == 0 {
		t.Fatalf("a hostile rival with four crew never raided: %v", dir.Log)
	}
	if !attacking {
		t.Error("no rival worker took up the raid against the player's colonist")
	}
	if hurt < 0 {
		t.Errorf("the raid never reached the player's colonist (health %d/%d)", hc.Health, hc.MaxHealth)
	}
	t.Logf("raid hurt the colonist at tick %d; rivals %d alive; %v", hurt, len(settlementWorkers(ms.level, name)), dir.Log)
}

// The food rung keeps a rival colony fed: four crew with no rations in the
// hold, forty days on a site with wildlife, everyone alive and nobody
// starving at the end, and food hunted and brought home.
func TestMirrorFedFortyDays(t *testing.T) {
	if testing.Short() {
		t.Skip("forty days headless")
	}
	randSeed := int64(1)
	if v, err := strconv.ParseInt(os.Getenv("LP_FLAKE_SEED"), 10, 64); err == nil {
		randSeed = v // tracing the starvation path over other draws
	}
	seedGlobalRand(t, randSeed)
	_, ms, loc := mirrorOnStart(t, 1, 4)
	name := "mirror:" + loc.ID
	ms.campaign.SetStance("mirror", campaign.StanceNeutral, loc) // fed, not fighting
	components.SetFactionHostile("mirror", false)
	t.Cleanup(components.ResetFactionHostility)
	starving := 0
	trace := os.Getenv("LP_FLAKE_LOG") != ""
	stepLevel(ms, 40*750, func(tick int) {
		if trace && tick%250 == 0 {
			t.Log(mirrorFoodLine(ms, name, tick))
		}
		if tick%750 != 0 {
			return
		}
		for _, e := range settlementWorkers(ms.level, name) {
			if nc := e.GetComponent(components.Needs).(*components.NeedsComponent); nc.IsStarving() {
				starving++
			}
		}
		if n := len(settlementWorkers(ms.level, name)); n < 4 {
			t.Logf("day %d: %d alive", tick/750, n)
		}
	})
	ws := settlementWorkers(ms.level, name)
	if len(ws) != 4 {
		t.Fatalf("%d of 4 rival crew alive after forty days", len(ws))
	}
	dir := ms.factionDirectors[name]
	hunts := 0
	for k, v := range dir.Ordered {
		if len(k) > 5 && k[:5] == "hunt:" {
			hunts += v
		}
	}
	if hunts == 0 {
		t.Error("never hunted")
	}
	if starving > 0 {
		t.Errorf("%d crew-days starving", starving)
	}
	for _, e := range ws {
		nc := e.GetComponent(components.Needs).(*components.NeedsComponent)
		hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		t.Logf("crew hunger %d health %d/%d", nc.Hunger, hc.Health, hc.MaxHealth)
	}
	t.Logf("hunts %d, orders %v", hunts, dir.Ordered)
}

// mirrorFoodLine is one LP_FLAKE_LOG line for TestMirrorFedFortyDays: the
// food the rung counts, each crew member's hunger, health, AI state and task,
// and the open food orders.
func mirrorFoodLine(ms *MainState, name string, tick int) string {
	var b strings.Builder
	food, loose := 0, 0
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e == nil {
				continue
			}
			if e.HasComponent(components.Storage) {
				sc := e.GetComponent(components.Storage).(*components.StorageComponent)
				if sc.OwnedBy == name {
					for _, it := range sc.Items {
						if it.HasComponent(rlcomponents.Food) {
							food++
						}
					}
				}
			} else if e.HasComponent(rlcomponents.Food) && e.HasComponent(rlcomponents.Position) {
				loose++
				if os.Getenv("LP_FLAKE_LOG") == "2" {
					pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
					st := settlement.Settlements[name]
					fmt.Fprintf(&b, "[%s %d,%d,%d d%d] ", e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ(),
						max(abs(pc.GetX()-st.CenterX), abs(pc.GetY()-st.CenterY)))
				}
			}
		}
	}
	fmt.Fprintf(&b, "t%d d%.1f food %d loose %d |", tick, float64(tick)/750, food, loose)
	for _, e := range settlementWorkers(ms.level, name) {
		nc := e.GetComponent(components.Needs).(*components.NeedsComponent)
		hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		st := ""
		if e.HasComponent(rlcomponents.AIMemory) {
			st = e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).State
		}
		act := "-"
		if wc.CurrentTask != nil {
			act = fmt.Sprint(wc.CurrentTask.Action)
		}
		carry := 0
		if e.HasComponent(rlcomponents.Inventory) {
			for _, it := range e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
				if it.HasComponent(rlcomponents.Food) {
					carry++
				}
			}
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		fmt.Fprintf(&b, " h%d hp%d %s/%s c%d @%d,%d,%d;", nc.Hunger, hc.Health, st, act, carry, pc.GetX(), pc.GetY(), pc.GetZ())
	}
	if dir := ms.factionDirectors[name]; dir != nil && len(dir.Log) > 0 {
		fmt.Fprintf(&b, " | %s", dir.Log[len(dir.Log)-1])
	}
	return b.String()
}
