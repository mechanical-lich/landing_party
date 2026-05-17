# Design Decisions

Early design decisions and rationale for landing_party.

---

## Art & Rendering

**Decision: Tilesheets over pre-sliced images**

Using combined tilesheets (Oryx 16-bit Scifi + Oryx 16-bit Fantasy) rather than individual pre-sliced PNGs loaded via MLGE's folder loader. Pre-sliced is convenient for development but creates one GPU texture per tile, breaking Ebiten's draw call batching on tile-heavy screens. Tilesheets keep draw calls low.

The combined set gives us scifi characters/buildings plus fantasy world tiles, enabling alien forests, mixed-race native civilizations, and broader monster variety by combining tiles across both sets.

Tile size: 24x24 (vs 16x16 in fantasy_settlements).

---

## World & Scenarios

**Decision: Planet-based world generation via scenarios**

World generates as a planet surface. Scenarios (like fantasy_settlements and spaceplant) drive generation — possible scenario types include standard planet surface, empty space, asteroid field, and space station starts. Multiple planets in a solar system is a long-term goal; for now planets are treated as self-contained levels.

---

## Entities & Races

**Decision: Data-driven races, not per-race AI systems**

fantasy_settlements has GoblinAISystem, ElfAISystem, HumanAISystem — near-identical files, one per race. landing_party uses a single worker AI system driven by a faction/behavior profile defined in blueprint data. A `BehaviorKey` on the AI component looks up state machine rules from data rather than dispatching to a hardcoded struct.

This allows multiple species (native primitives, colonists, hostile aliens, etc.) without adding Go files per race. Visual variation — sprite coords, tilesheet key, color tints — is entirely blueprint data.

---

## Colonists

**Decision: Colonists are entities like goblins**

Colonists are ECS entities with Worker + Settlement components, using the same indirect-control task system as fantasy_settlements goblins. The player designates work areas/tasks; colonists autonomously claim tasks, haul resources to storage, and pull from storage when building. Resources pool into settlement coffers.

**Decision: Stats drive task performance**

Each colonist has Str/Int/Dex stats. These actively gate and scale task outcomes:
- Str: dig speed, haul capacity, melee
- Int: research speed, crafting quality
- Dex: ranged combat, fine work

Stat scaling applied as a multiplier at task execution time (e.g. base_duration / stat_modifier). A consistent modifier table should be defined early and used across all task handlers.

---

## Tasks

**Decision: Progress lives on the task, not the worker**

Currently in fantasy_settlements, task progress (e.g. `buildRequest.BuildTime`) is stored in `workerComponent.CurrentTask.Data` — it's worker-local and lost if the worker stops. In landing_party, progress is a field on the Task object itself:

- `Progress int` — current accumulated ticks
- `Required int` — total ticks to complete
- `Workers []*ecs.Entity` — who is currently contributing

Each tick a worker is present and assigned, they call `task.Advance(entity)` which increments Progress by their stat modifier. Multiple workers on the same task each call Advance — naturally additive, no special multi-worker logic needed.

Partial yields: tasks like mining check progress thresholds (e.g. every 10 ticks drops one ore unit), so stopping halfway still produces partial results. A different worker can resume where another left off.

This also makes saves cleaner — serializing tasks captures all in-progress work state.

---

## Research

**Decision: Research requires physical presence in a building**

Research is a task type like digging or building. A colonist must be adjacent to (or inside) the required building (e.g. Research Lab) for the full duration — they cannot start it remotely and walk away. Their action each turn is to advance the research task's Progress counter, scaled by their Int stat.

`research.json` defines the tech tree:
```json
"advanced_metallurgy": {
    "name": "Advanced Metallurgy",
    "duration": 200,
    "required_building": "research_lab",
    "required_int": 12,
    "unlocks": ["plasma_cutter", "metal_wall", "reinforced_door"]
}
```

On completion, the tech key is added to `settlement.KnownTechs`. The build menu filters `ListBuildables()` against KnownTechs.

`build.json` entries get an optional `required_tech` field:
```json
"plasma_cutter": {
    "required_tech": "advanced_metallurgy",
    "cost": { "metal": 3, "power_cell": 1 }
}
```

Multiple colonists can advance the same research task simultaneously.

---

## Build System

**Decision: Fully data-driven build trees**

Build menus, structures, walls, floors, and research can all be extended by editing JSON data files — no code changes required to add new content. The `build.json` pattern from fantasy_settlements is extended with `required_tech` gating.

---

## Resource Gathering Styles

Multiple distinct task types for resource acquisition (all using the shared progress-on-task model):

- **MineAction** — worker digs tile, yields ore/materials based on tile type, scaled by Str
- **ForageAction** — worker interacts with a living entity (plant/fungus), yields items with or without destroying the source
- **DrillAction** — a building entity that generates resources on a timer autonomously (no worker needed while running)
