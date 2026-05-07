# Developer Guide

Architecture notes, system references, and guides for extending the game.

## Guides

- [Entities & Blueprints](entities.md) — adding and configuring entities via JSON
- [AI Systems](ai.md) — worker AI states and faction hostile behavior
- [Buildings](buildings.md) — adding buildable structures
- [Research](research.md) — adding technologies to the tech tree
- [Scenarios](scenarios.md) — scenario configuration, spawn rules, and win conditions
- [World Generation](world_generation.md) — Z-levels, terrain, and lighting

---

## Architecture Overview

The game uses an **ECS (Entity-Component-System)** architecture provided by `mlge` and `ml-rogue-lib`.

| Layer | Location |
|-------|----------|
| Components | `internal/components/` |
| Systems | `internal/systems/` |
| AI state handlers | `internal/ai/` |
| Game loop & state machine | `internal/game/` |
| Blueprint data | `data/entity_blueprints.json` |
| Build data | `data/build.json` |
| Research data | `data/research.json` |
| Scenario data | `data/scenarios/` |
| Tile definitions | `data/tile_definitions.json` |

All content (entities, buildings, research, scenarios) is **data-driven via JSON**. Adding new content typically requires only a JSON edit and no new Go code.
