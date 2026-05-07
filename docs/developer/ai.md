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
| `gather_materials` | `internal/ai/worker.go` | Collects resources needed before beginning a task |

### Task Progress Model

Task progress is stored **on the task object, not the worker**. This means:

- Multiple colonists can work the same task simultaneously and progress accumulates from all contributions.
- If a colonist is interrupted (hunger, death, reassignment), the progress is not lost.
- Task duration is modified by the relevant colonist stat: `effective_duration = base_duration / stat_value`.

### Adding New Task Types

1. Define the task struct in `internal/settlement/settlement.go`.
2. Add a handler case in `internal/ai/worker.go` for the new task state.
3. Wire the task into the settlement's task queue creation logic.

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

1. Add a blueprint to `data/entity_blueprints.json` with `FactionAIComponent` set.
2. Set `Faction` to `"alien"` (or a new faction string if you want a distinct group).
3. Add the blueprint ID to a spawn rule in the relevant scenario JSON if it should appear in gameplay.

No new Go code is needed for standard hostile behavior. For unique behavior, extend `FactionAISystem` or add a new system.
