# Physical Storage — Implementation Plan

Status: **planned, not started** (2026-10-31). Supersedes the "unified
storage" line in [todo.md](todo.md).

## Decision

All material use is **physical**. A colonist must carry the inputs for a
task to the place the task happens. Nothing is deducted from a container
the colonist is not standing next to. This applies to building, crafting,
and research alike.

Rationale: resource management is part of the game. Pooled, instant
deduction (what crafting and research do today) removes the logistics of
where materials are kept and makes beaming from the ship feel free.

Beaming stays as it is: Beam Resources Down still moves items from the ship
hold into the site's teleporter container (or first container). The
difference is that beamed-down material now has to be hauled from there by
a colonist before it can be used, so the teleporter becomes a staging area
rather than a bypass.

## Where the code is today

| Consumer | Behaviour | Code |
|---|---|---|
| Build | Physical. `gather_materials` state fetches **one material per trip**, from the container closest to the **build site** (not the worker), then returns. | `HandleGatherMaterialsState` in `internal/workerai/states.go`; entry from `handleBuildTask` in `task_handlers.go` |
| Craft | Pooled. At the bench, on first tick, checks the settlement-wide pool and deducts from it on completion. | `handleCraftTask` in `task_handlers.go`, via `checkSettlementStorageForCraft` / `deductFromSettlementStorage` in `storage.go` |
| Research | Pooled. Same pattern as craft, deducted on first tick at the lab. | `handleResearchTask` in `task_handlers.go` |
| Haul | Physical. Picks up loose ground items and deposits into the first container whose tag filter accepts them. | `HandleHaulState` in `states.go` |

History worth knowing: crafting **was** physical once. A
`gather_materials_craft` state was added on 2026-05-06 ("Adding crafting
basics") and removed on 2026-05-10 ("Improving drop off and resource
stacking and usage while crafting"), when crafting switched to the pooled
deduct. A dead `case "gather_materials_craft"` redirect still sits in
`internal/systems/WorkerSystem.go`. The earlier attempt fetched one item at
a time, which is likely why it was abandoned; this plan's trip planner is
the fix for that.

Also relevant:

- `storage.Provider` / `Check` / `Deduct` in `internal/storage` stay. They
  remain the right tool for *counting* (Global Inventory, affordability
  messages) even once nothing *consumes* through them.
- `installCampaignStorageHook` in `internal/game/world_manager.go` scopes the
  provider to the live level only. Unchanged.
- `InventoryComponent.Bag` (ml-rogue-lib) has **no capacity**. A worker can
  carry anything today.
- `MaterialComponent` has `Quantity` and `MaxStack`; stackables merge,
  non-stackables are one per slot.
- `StorageComponent.Capacity` caps slots in a container, not quantity.

## Design

### 1. A carry limit

Add a carry limit so "gather everything in one trip" has a reason to be a
trade-off. Without one, a single colonist empties a chest and trip count is
always one.

- New helper `CarryCapacity(entity) int` in `internal/workerai`, next to
  `workRate` in `work_speed.go`. Units are **stack units** of stackable
  material (a non-stackable item counts as its blueprint's "weight", default
  1). Formula follows the `workRate` precedent: a baseline at Str 10, scaled
  by `Str / 10`, floored at the baseline so stats are only ever a bonus.
  Suggested baseline **20 units**. Robots inherit it (the Str-12 excavator
  carries a bit more; the hauler robot can set a higher baseline through a
  blueprint field if that feels right later).
- `CarryUsed(entity) int` sums the bag. `CarryFree = capacity - used`.
- The limit is enforced **only by the gather planner**. Haul, pickup orders,
  and crafted output landing in the bag are unchanged so no existing flow
  can get stuck on a full bag.

### 2. A shopping list on the worker

Replace the per-tick "find the first deficit" scan with a list the worker
builds once and works through.

New fields on `WorkerComponent`:

```go
// GatherList is what the worker still needs to fetch for CurrentTask,
// blueprint -> remaining quantity. Rebuilt when the task starts gathering;
// entries hit zero as items land in the bag.
GatherList map[string]int
// GatherTarget is the container the worker is currently walking to, or nil
// when it needs to pick the next hop.
GatherTarget *ecs.Entity
```

Both are transient: nil them in the same places `DropOffItem` is cleared,
and on task release / cancel. Saves don't need them (a freshly loaded worker
re-plans on its next tick, same as today).

### 3. The trip planner (`gather_materials` state)

One state handles all three consumers. Rewrite `HandleGatherMaterialsState`
as:

1. **Build the list** if `GatherList == nil`: take the task's cost map
   (build cost, recipe cost, or tech cost — see §4), subtract what is in the
   bag, keep the positive deficits.
2. **Done?** If the list is empty, clear `GatherTarget`, set state `"task"`.
3. **Pick the next hop** if `GatherTarget == nil`: among colony-owned
   containers on the level that hold **at least one listed item**, choose
   the one closest to the **worker's current position** (not the task
   site). Greedy nearest-first is enough; there are rarely more than a
   handful of containers. If no container holds anything on the list:
   stop the task, post `"Insufficient materials to <verb> <name>"`, go idle.
   (Affordability was already checked against the pool when the task was
   picked, so this only fires if stock moved mid-trip.)
4. **Walk** to `GatherTarget` with `MoveTowardsTarget`. On a failed move use
   `releaseTaskAfterFailedMove` and clear the gather fields, matching the
   craft/research handlers.
5. **Load** when adjacent: for every listed blueprint the container has,
   take `min(remaining, available, CarryFree)` — stackables via
   `DeductResource` + a new material entity with that quantity (as the
   current code does), non-stackables via `TakeOne` in a loop. Decrement
   the list. Then clear `GatherTarget` so step 3 picks the next hop.
6. **Capacity reached** with items still on the list: go to `"task"`
   anyway. The task handler will consume what was delivered (§4) and send
   the worker back out with a rebuilt, smaller list.

Step 5's "take everything on the list that is here" is what turns N trips
into one when a single chest covers several inputs.

### 4. Delivered inputs live on the task, not the worker

A build task today is only "paid" when the worker reaches the site with a
full bag (`checkInventoryForMaterials` then
`removeMaterialsFromInventory`). That breaks under a carry limit (two trips)
and under multiple workers sharing one task (craft and research progress is
already shared through the request struct).

So each request struct gains a delivered tally:

```go
// Delivered is what has already been carried to the task site and
// consumed from a worker's bag, blueprint -> quantity. The task's cost is
// satisfied when Delivered covers it.
Delivered map[string]int
```

on `BuildRequest`, `CraftRequest`, and `ResearchRequest` in
`internal/task_requests`. (Craft and research requests are pointers in
`Task.Data`; `BuildRequest` is a value that handlers write back — keep
doing that.)

Each task handler then follows the same shape:

1. `need := cost - Delivered`. If `need` is empty, work (advance progress).
2. Else if the bag holds any of `need`: walk to the task site, and when
   adjacent move those items from the bag into `Delivered`.
3. Else: set state `"gather_materials"`.

Progress only advances once `Delivered` covers the cost, so a half-delivered
craft does not start. Multiple workers on one task all deliver into the same
tally and all advance it once it is paid.

`checkInventoryForMaterials` and `removeMaterialsFromInventory` in
`materials.go` become the generic "move what I carry into Delivered" step
and should take a `map[string]int` cost instead of a `Buildable`.

The pooled `checkSettlementStorageForCraft` / `deductFromSettlementStorage`
calls are removed from the craft and research handlers. Keep the functions
for now; the task picker should use `storage.Check` once at pick time (see
below) so a worker never sets off for a task the colony cannot afford.

### 5. Picking up listed items along the route

Scope this tightly: it is **only** for items already on the worker's
`GatherList`.

- In step 4 of the planner, before each move, look at loose ground items
  (`FindLooseItemOnGround`-style scan, `materials.go`) within **1 tile** of
  the worker. If one matches a listed blueprint, take it (respecting
  `CarryFree`) and decrement the list. Do not detour for it; this is a
  free grab, not a goal.
- Containers along the way are **not** opened opportunistically. The
  greedy hop already visits the nearest useful container, so a container
  "on the way" is by construction the next hop.
- Anything not on the list is ignored. General haul behaviour (grab any
  loose ore) stays with the haul task and the tag-filtered deposit rules.

That keeps bags predictable for the colonist modal and the "insufficient
materials" message, and never fights the player's container filters.

### 6. Affordability gating at task pick

Because the handlers no longer check the pool, the **task picker** must.
When a worker considers a build/craft/research task, call `storage.Check`
against the level provider with the task's remaining cost
(`cost - Delivered`). If it fails, skip the task this tick (do not cancel
it; stock may arrive). This is the one place pooled counting still gates
work, and it is the right place: it answers "is there any point starting"
without deciding where the materials are.

### 7. Cancellation and cleanup

- On task cancel/stop (right-click, `Task.Stop`, failed move): clear
  `GatherList` / `GatherTarget`. Items already in the bag stay there; the
  worker's existing idle behaviour (haul/dropoff) returns them to storage.
  `Delivered` items on a cancelled task are **lost** — same as today for a
  half-built structure. Acceptable; document it.
- Remove the dead `gather_materials_craft` case from `WorkerSystem.go`.

## Files

| File | Change |
|---|---|
| `internal/components/WorkerComponent.go` | `GatherList`, `GatherTarget` |
| `internal/task_requests/*.go` | `Delivered` on Build/Craft/Research requests |
| `internal/workerai/work_speed.go` | `CarryCapacity`, `CarryUsed` |
| `internal/workerai/states.go` | rewrite `HandleGatherMaterialsState` (§3, §5) |
| `internal/workerai/task_handlers.go` | build/craft/research handlers use `Delivered` (§4); drop pooled check/deduct |
| `internal/workerai/materials.go` | generalise bag-to-Delivered helpers over a cost map; adjacent-ground grab |
| `internal/workerai/storage.go` | `FindClosestStorageWithAny(level, owner, list, x,y,z)`; mark pooled check/deduct as pick-time only |
| worker task picker (idle state in `states.go`) | `storage.Check` gate (§6) |
| `internal/systems/WorkerSystem.go` | remove `gather_materials_craft` case |
| `docs/player/storage.md` | "Worker behaviour around storage": materials are physically carried; carry limit; teleporter as staging |
| `docs/player/crafting.md`, `docs/player/research.md` | one line each: inputs are fetched to the bench/lab |
| `docs/developer/ai.md` | the gather state and `Delivered` tally |
| `docs/brainstorm/todo.md` | move "unified storage" to Done, pointing here |

## Tests (`internal/workerai`)

Existing `workerai_test.go` builds levels with storage entities; extend it:

- Planner takes two listed materials from one container in a single stop.
- Planner visits the nearer container first measured from the worker, not
  the task site.
- Carry limit splits a large cost across two trips, and `Delivered`
  accumulates across them.
- Craft and research do not advance progress until `Delivered` covers cost.
- Two workers on one craft task both deliver, neither double-pays.
- A listed item within one tile of the route is picked up; an unlisted one
  is not.
- Task picker skips an unaffordable task without cancelling it.
- `CarryCapacity` floors at baseline for Str < 10.

## Order of work

1. Carry limit helpers (§1) — pure functions, tests first.
2. `Delivered` on requests + build handler conversion (§4 for build only).
   Build already gathers, so this is the smallest end-to-end change and
   proves the two-trip path.
3. Trip planner rewrite (§3) with the list on the worker (§2). Build now
   does multi-item trips.
4. Switch craft, then research, onto the same path (§4). Remove pooled
   deduct. Add pick-time gate (§6).
5. Route pickup (§5).
6. Cleanup (§7), docs, todo.

Each step leaves the game playable; stop after any of them.

## Open questions (defaults chosen)

- **Baseline capacity 20 units.** Picked so a starting colonist covers most
  early recipes in one trip (hull wall, workbench) but not a bio-printer run
  (25 biomass). Tune in play.
- **Non-stackable weight.** Default 1 per item; add a `Weight` field to
  `MaterialComponent` only if armour/robots need to feel heavy.
- **Beam friction.** None added here. If hauling from the teleporter is not
  enough friction, a cooldown or per-beam fuel cost is a separate, small
  change in `BeamResourceDown`.
