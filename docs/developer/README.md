# Developer Guide

Architecture notes, system references, and guides for extending the game.

## Guides

- [Entities & Blueprints](entities.md) — adding and configuring entities via JSON
- [AI Systems](ai.md) — worker AI states and faction hostile behavior
- [Buildings](buildings.md) — adding buildable structures
- [Crafting](crafting.md) — workbenches, recipes, and items
- [Research](research.md) — adding technologies to the tech tree
- [Scenarios](scenarios.md) — scenario configuration, biome maps, spawn rules, and setup scripts
- [Biomes](biomes.md) — biome definitions, terrain rules, and biome-specific features
- [World Generation](world_generation.md) — Z-levels, terrain, and lighting
- [Structure Scripts](structure_scripts.md) — procedural structure generation via mechanical-basic scripts; quest-target binding
- [Tile Definitions](tile_definitions.md) — the `data/tiledefinitions/` catalog, load order, the `empty` sentinel invariant, and save portability
- [Names](names.md) — `data/names.json`, the `<type>` placeholder pattern for blueprints, seeded vs unseeded generation, and epithets
- [Campaign & Overworld](campaign.md) — the ship hub, Star Map, fuel travel, beaming, quests, and multi-location persistence

---

## Architecture Overview

The game uses an **ECS (Entity-Component-System)** architecture provided by `mlge` and `ml-rogue-lib`.

| Layer | Location |
|-------|----------|
| Components | `internal/components/` |
| Systems | `internal/systems/` |
| AI state handlers | `internal/ai/` |
| Game loop & state machine | `internal/game/` |
| Campaign / overworld model | `internal/campaign/` |
| Shared stockpile accessor | `internal/storage/` |
| Level swap / lifecycle | `internal/game/world_manager.go` |
| Generation templates/tuning | `data/location_templates.json`, `data/quest_templates.json`, `data/generation.json` |
| Entity blueprints | `data/blueprints/` (entities, items, structures) |
| Build data | `data/build.json` |
| Crafting recipes | `data/crafting_recipes.json` |
| Research data | `data/research.json` |
| Scenario data | `data/scenarios/` |
| Scenario setup scripts | `data/scripts/scenarios/*.basic` |
| Structure generation scripts | `data/scripts/structures/*.basic` |
| Biome definitions | `data/biomes/` |
| Tile definitions | `data/tiledefinitions/*.json` |
| Name pools | `data/names.json` |
| Asset references | `data/assets.json` |
| Engine config | `data/config.json` |

All content (entities, buildings, crafting, research, scenarios, biomes) is **data-driven via JSON**. Adding new content typically requires only a JSON edit (and possibly a `.basic` script for scenarios or structure generation) — no Go code change.

### Blueprint Loading

`cmd/game/main.go` loads the entire `data/blueprints/` tree at startup via `factory.FactoryLoadDir`. The directory layout is convention only — every `.json` file under that path is read and merged into a single blueprint registry keyed by ID. Files are typically organized as:

```
data/blueprints/
    entities/      # colonists, aliens, critters, flora
    items/         # weapons, armor, consumables, resources
    structures/    # workbenches, doors, furniture
```

Blueprint IDs must be globally unique across the tree.
