package workerai

import (
	"fmt"
	"github.com/mechanical-lich/landing_party/internal/chatter"
	"log"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/mlge/utility"
)

// FindLooseItemOnGround returns the nearest item entity lying on the ground (not in storage/inventory).
func FindLooseItemOnGround(level *world.Level, x, y, z int) *ecs.Entity {
	var closest *ecs.Entity
	minDist := 99999
	for _, e := range level.Entities {
		if !e.HasComponent(rlcomponents.Item) {
			continue
		}
		if !e.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetZ() != z {
			continue
		}
		d := utility.Distance(pc.GetX(), pc.GetY(), x, y)
		if closest == nil || d < minDist {
			closest = e
			minDist = d
		}
	}
	return closest
}

// grabListedItemsNearby takes loose items on the list from within one tile of the
// worker, up to its free capacity, and takes them off the list. It's a free grab
// on the way past, never a goal: the worker doesn't detour, and anything unlisted
// is left for the haul rules.
func grabListedItemsNearby(level *world.Level, entity *ecs.Entity, list map[string]int) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	free := carryFree(entity)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			x, y, z := pc.GetX()+dx, pc.GetY()+dy, pc.GetZ()
			if !level.InBounds(x, y, z) {
				continue
			}
			var tileEntities []*ecs.Entity
			level.GetEntitiesAt(x, y, z, &tileEntities)
			for _, candidate := range tileEntities {
				if candidate == entity || !candidate.HasComponent(rlcomponents.Item) || !candidate.HasComponent(rlcomponents.Position) {
					continue
				}
				units := carryUnits(candidate)
				take := min(list[candidate.Blueprint], free, units)
				if take <= 0 {
					continue
				}
				if take == units {
					inv.AddItem(candidate)
					level.RemoveEntity(candidate)
				} else {
					// Only a stack can be split: leave the rest on the ground.
					part, err := factory.Create(candidate.Blueprint, 0, 0, 0)
					if err != nil {
						log.Printf("[GATHER] can't create %s: %v", candidate.Blueprint, err)
						continue
					}
					part.GetComponent(components.Material).(*components.MaterialComponent).Quantity = take
					candidate.GetComponent(components.Material).(*components.MaterialComponent).Quantity -= take
					inv.AddItem(part)
				}
				free -= take
				if list[candidate.Blueprint] -= take; list[candidate.Blueprint] <= 0 {
					delete(list, candidate.Blueprint)
				}
			}
		}
	}
}

func PickupItemFromTile(level *world.Level, entity *ecs.Entity, x, y, z int) {
	// Items are always pickupable; deployed Worker entities (robots) are too,
	// at which point they leave the live ECS tick and become dormant in the
	// carrier's bag — mirroring the inactive-in-locker state.
	//
	// Scan every entity at the tile rather than GetEntityAt — the carrier
	// itself may be standing on the target tile and would otherwise pick
	// itself up, vanishing from the level into its own bag.
	var tileEntities []*ecs.Entity
	level.GetEntitiesAt(x, y, z, &tileEntities)
	for _, candidate := range tileEntities {
		if candidate == entity {
			continue
		}
		if !candidate.HasComponent(rlcomponents.Item) && !candidate.HasComponent(components.Worker) {
			continue
		}
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		inv.AddItem(candidate)
		level.RemoveEntity(candidate)
		chatter.Post(rlentity.GetName(entity), fmt.Sprintf("Picked up %s", candidate.Blueprint))
		return
	}
}

// materialCost describes what a task consumes and how to name it in messages.
type materialCost struct {
	Inputs    map[string]int
	Delivered map[string]int
	Verb      string
	Name      string
}

// taskCost resolves a task's cost and delivered tally. ok is false for a task
// that isn't cost-bearing or whose definition can't be found.
func taskCost(t *task.Task) (materialCost, bool) {
	switch req := t.Data.(type) {
	case task_requests.BuildRequest:
		buildable := getBuildable(req.Type)
		if buildable.Name == "" {
			return materialCost{}, false
		}
		return materialCost{buildable.Cost, req.Delivered, "build", req.Type}, true
	case task_requests.CraftRequest:
		recipe, found := getRecipe(req.RecipeID)
		if !found {
			return materialCost{}, false
		}
		return materialCost{recipe.Cost, req.Delivered, "craft", recipe.Name}, true
	case *task_requests.ResearchRequest:
		tech, found := getTech(req.TechKey)
		if !found {
			return materialCost{}, false
		}
		return materialCost{researchInputs(req, tech), req.Delivered, "research", tech.Name}, true
	}
	return materialCost{}, false
}

// researchInputs returns what a research task still has to be paid with. Progress
// only advances once a task is paid, so progress means nothing is owed. That also
// covers research saved before delivery existed, which paid up front from the pool
// and has no tally.
func researchInputs(rr *task_requests.ResearchRequest, tech research.Tech) map[string]int {
	if rr.Progress > 0 {
		return nil
	}
	return tech.Cost
}

// MaterialNeed is one input of a cost-bearing task, for display.
type MaterialNeed struct {
	Blueprint string
	Delivered int
	Required  int
}

// taskTally returns a task's full cost and delivered tally. Research that has
// started reads as fully delivered, matching researchInputs.
func taskTally(t *task.Task) (cost, delivered map[string]int, ok bool) {
	c, ok := taskCost(t)
	if !ok {
		return nil, nil, false
	}
	if rr, isResearch := t.Data.(*task_requests.ResearchRequest); isResearch && rr.Progress > 0 {
		tech, _ := getTech(rr.TechKey)
		return tech.Cost, tech.Cost, true
	}
	return c.Inputs, c.Delivered, true
}

// TaskDeliveryTotals sums a task's delivered and required units over all inputs.
// required is 0 for a task with no material cost.
func TaskDeliveryTotals(t *task.Task) (delivered, required int) {
	cost, tally, ok := taskTally(t)
	if !ok {
		return 0, 0
	}
	for name, qty := range cost {
		required += qty
		delivered += min(tally[name], qty)
	}
	return delivered, required
}

// TaskNeeds lists every input of a task still waiting on materials, ordered by
// blueprint. It returns nil for a paid task or one with no material cost.
func TaskNeeds(t *task.Task) []MaterialNeed {
	cost, tally, ok := taskTally(t)
	if !ok || len(remainingCost(cost, tally)) == 0 {
		return nil
	}
	needs := make([]MaterialNeed, 0, len(cost))
	for name, qty := range cost {
		needs = append(needs, MaterialNeed{Blueprint: name, Delivered: min(tally[name], qty), Required: qty})
	}
	sort.Slice(needs, func(i, j int) bool { return needs[i].Blueprint < needs[j].Blueprint })
	return needs
}

// remainingCost returns the positive part of cost - delivered; empty when the
// task is paid.
func remainingCost(cost, delivered map[string]int) map[string]int {
	need := make(map[string]int, len(cost))
	for name, qty := range cost {
		if left := qty - delivered[name]; left > 0 {
			need[name] = left
		}
	}
	return need
}

// checkInventoryForMaterials reports whether the bag holds at least one unit of
// anything in cost, i.e. whether a walk to the task site would deliver something.
func checkInventoryForMaterials(entity *ecs.Entity, cost map[string]int) bool {
	if !entity.HasComponent(rlcomponents.Inventory) {
		return false
	}
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for _, item := range inv.Bag {
		if cost[item.Blueprint] > 0 && carryUnits(item) > 0 {
			return true
		}
	}
	return false
}

// removeMaterialsFromInventory takes up to cost out of the bag and returns what
// it actually removed, blueprint -> quantity.
func removeMaterialsFromInventory(entity *ecs.Entity, cost map[string]int) map[string]int {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	removed := make(map[string]int, len(cost))
	for name, qty := range cost {
		remaining := qty
		i := 0
		for remaining > 0 && i < len(inv.Bag) {
			item := inv.Bag[i]
			if item.Blueprint != name {
				i++
				continue
			}
			if item.HasComponent(components.Material) {
				mc := item.GetComponent(components.Material).(*components.MaterialComponent)
				if mc.Quantity <= remaining {
					remaining -= mc.Quantity
					inv.Bag = append(inv.Bag[:i], inv.Bag[i+1:]...)
				} else {
					mc.Quantity -= remaining
					remaining = 0
					i++
				}
			} else {
				remaining--
				inv.Bag = append(inv.Bag[:i], inv.Bag[i+1:]...)
			}
		}
		if taken := qty - remaining; taken > 0 {
			removed[name] = taken
		}
	}
	return removed
}

// deliverMaterials runs the delivery half of a cost-bearing task and returns the
// updated tally plus whether the task is paid. Unpaid, it either carries what the
// bag holds to the site and moves it into the tally, or sends the worker to
// gather. The caller must write the tally back to the request (see
// task_requests); wc.CurrentTask may be nil on return after a failed move.
func deliverMaterials(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent, cost, delivered map[string]int, x, y, z int) (map[string]int, bool) {
	need := remainingCost(cost, delivered)
	if len(need) == 0 {
		return delivered, true
	}
	if !checkInventoryForMaterials(entity, need) {
		wc.GatherList = nil
		wc.GatherTarget = nil
		aiMemory.State = "gather_materials"
		return delivered, false
	}
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), x, y, z, 1, 1, 0) {
		if moved, pathFound := MoveTowardsTarget(level, entity, x, y, z); !moved {
			releaseTaskAfterFailedMove(wc, pathFound)
			aiMemory.State = "idle"
		}
		return delivered, false
	}
	if delivered == nil {
		delivered = make(map[string]int, len(cost))
	}
	for name, qty := range removeMaterialsFromInventory(entity, need) {
		delivered[name] += qty
	}
	return delivered, false
}
