package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// In a campaign the player's techs live on the campaign record, not on the
// settlement: a tech-gated recipe must be orderable once the campaign knows
// its tech (review finding 1: every armour/robot recipe was refused).
func TestCampaignPlayerCanOrderTechGatedCraft(t *testing.T) {
	useContentTables(t)
	wm := shipTestWorld(t)
	ms := wm.shipLevel
	if ms.campaign == nil || ms.MainSettlement == nil {
		t.Fatal("ship state has no campaign or settlement")
	}
	bench := &ecs.Entity{}
	bench.AddComponent(&rlcomponents.PositionComponent{})

	if ms.addCraftTask("marine_head", bench) {
		t.Fatal("marine_head ordered before combat_armor is known")
	}
	ms.campaign.UnlockTech("combat_armor")
	if ms.MainSettlement.HasTech("combat_armor") {
		t.Fatal("fixture: the settlement should not carry the campaign's techs")
	}
	if !ms.addCraftTask("marine_head", bench) {
		t.Fatal("marine_head refused although the campaign knows combat_armor")
	}
	// The bench checks again when the crafter works the order: it must read
	// the same campaign record.
	if !workerai.SettlementKnowsTech(ms.MainSettlement.Name, "combat_armor") {
		t.Fatal("the bench's tech check does not see the campaign's techs")
	}
	n := 0
	for _, tk := range ms.MainSettlement.Tasks.GetTasks() {
		if cr, ok := tk.Data.(task_requests.CraftRequest); ok && cr.RecipeID == "marine_head" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d marine_head craft tasks queued, want 1", n)
	}
}
