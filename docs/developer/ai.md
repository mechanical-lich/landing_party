# AI Systems — Developer Guide

The game has two distinct AI systems: **WorkerSystem** for colonists and **FactionAISystem** for hostile NPCs.

---

## WorkerSystem

`internal/systems/WorkerSystem.go` dispatches colonist behavior based on the worker's current state. State is stored on the `WorkerComponent`.

### Worker States

| State | Handler | Description |
|-------|---------|-------------|
| `idle` | `internal/ai/worker.go` | Queries the settlement task queue for the nearest unclaimed task; claims it or waits |
| `task` | `internal/ai/worker.go` | Advances progress on the claimed task each turn |
| `haul` | `internal/ai/worker.go` | Carries materials from a source to a build site or storage |
| `dropoff` | `internal/ai/worker.go` | Returns gathered resources to a storage container |
| `findfood` | `internal/ai/worker.go` | Navigates to the nearest accessible food item |
| `gather_materials` | `internal/workerai/states.go` | Trip planner: fetches what a build, craft, or research task still needs from containers (see [Material Delivery](#material-delivery)) |

### Task Progress Model

Task progress is stored **on the task object, not the worker**. This means:

- Multiple colonists can work the same task simultaneously and progress accumulates from all contributions.
- If a colonist is interrupted (hunger, death, reassignment), the progress is not lost.

### Material Delivery

All material use is physical: nothing is deducted from a container the worker isn't standing next to. Each cost-bearing handler (`handleBuildTask`, `handleCraftTask`, `handleResearchTask`) starts with `deliverMaterials` in `internal/workerai/materials.go`:

1. `need := cost - Delivered`. If empty, the task is paid and the handler works it.
2. If the bag holds any of `need`, walk to the site and move it from the bag into `Delivered`.
3. Otherwise enter `gather_materials`.

`Delivered` is a `map[string]int` on `BuildRequest`, `CraftRequest`, and `ResearchRequest`. It lives on the task, so several workers and several trips add to one tally, and it is saved with the settlement's task queue. Progress only advances once it covers the cost. Materials delivered to a task that is later cancelled are lost. Started research counts as paid whatever its tally says, because research saved before delivery existed paid up front.

`HandleGatherMaterialsState` builds `WorkerComponent.GatherList` once (`need` minus the bag), then repeatedly picks `GatherTarget`, the container closest to the worker holding anything listed, and takes every listed item there up to free capacity. While walking it also takes listed loose items within one tile, never detouring. It returns to `task` when the list is empty or the bag is full. Both fields are transient and rebuilt on each entry.

The carry limit is `CarryCapacity` in `work_speed.go`: 20 stack units at Str 10, scaled by Str, never below 20. Only the planner enforces it. Haul, pickup orders, and crafted output can overfill a bag.

`storage.Check` still gates task picking: `pickAffordableTask` passes over a task the colony's containers plus the worker's bag can't cover, without cancelling it, and offers the next closest. `storage.Deduct` no longer has a consumer in the worker AI.

An idle worker with no task returns `Material` items in its bag to the nearest accepting container with a free slot, one per `dropoff`, so surplus counts as stock again (`startReturningMaterials`). Gear stays in the bag, and so does one food item; any more food is returned too, and every `dropoff` that reaches a container also stores the bag's surplus food there (`depositSurplusFood`), so a worker who hauled or picked up food does not carry it from task to task while the rest of the colony finds none in storage. A `dropoff` that can't advance for `maxApproachFailTicks`, or has not delivered within `maxDropOffTicks` (600) whatever its moves (workers crowding a container with one free side swap places forever, each step counting as progress), gives up and goes idle (`AbandonDropOff`, the item kept in the bag). A hungry worker in `dropoff` with food available puts the errand down and goes to eat (`NeedsSystem`): food-seeking otherwise starts only from `idle`.

Delivery shows up read-only through `TaskNeeds` and `TaskDeliveryTotals`: the hover "Needs" block, the `gathering a/b` queue rows, and the bottom-up reticle fill in `drawTaskReticle`.

### Stat Progression

When a task completes, `progression.AwardXP(entity, stat, progression.XPPerTask)` is called in the task handler before `CompleteTaskWithMessage`. This awards 25 XP toward a specific stat (Str, Int, or Dex). Enough accumulated XP levels up that stat, permanently incrementing its value by 1 (capped at +10 per stat).

See `internal/progression/progression.go` for the XP formula and `docs/developer/entities.md` for the task→stat mapping. To hook a new task type into progression, add an `AwardXP` call in the relevant handler in `internal/ai/worker.go`.

### Adding New Task Types

1. Define the task action constant in `internal/task_requests/requests.go`.
2. Add a handler function in `internal/ai/worker.go` and a case for it in `HandleTaskState`.
3. Wire the task into the settlement's task queue creation logic.
4. Optionally call `progression.AwardXP` at completion to hook it into the stat progression system.

---

## FactionAISystem

`internal/systems/FactionAISystem.go` drives hostile NPC behavior. Entities with a `FactionAIComponent` are processed each turn.

### Behavior

The default hostile behavior:

1. Scan for colonist entities within **8 tiles** (configurable via `BehaviorKey` profile).
2. If a colonist is found:
   - If adjacent: **attack** the colonist.
   - If not adjacent: **pathfind** toward the colonist.
3. If no colonist is visible: **wander** randomly.

### FactionAIComponent

```go
type FactionAIComponent struct {
    BehaviorKey string  // references a behavior profile
    Faction     string  // used for friend/foe resolution
}
```

`BehaviorKey` is intended as a lookup into data-driven behavior profiles (see `design_decisions.md`). Currently hostile entities share a common behavior; the key is used to allow future per-faction customization without separate Go systems.

### Faction Resolution

Friend/foe is determined by comparing `Faction` fields:

- `"colony"` faction entities are treated as targets by `"alien"` faction AI.
- Entities of the same faction do not attack each other.
- Doors with faction access lists (airlocks, blast doors) check the entity's `Faction` to allow or block passage.

### Adding New Hostile Types

1. Add a blueprint under `data/blueprints/entities/` (e.g. `aliens.json`) with a `FactionAI` component.
2. Set `Faction` to `"alien"` (or a new faction string if you want a distinct group).
3. Add the blueprint ID to a spawn rule in the relevant scenario JSON if it should appear in gameplay.

No new Go code is needed for standard hostile behavior. For unique behavior, extend `FactionAISystem` or add a new system.
