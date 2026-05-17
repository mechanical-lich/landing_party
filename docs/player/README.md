# Player Guide

Welcome to Landing Party. Build a colony. Keep your colonists alive. Try not to die.

## Sections

- [Getting Started](#getting-started) — what the game is and how it works
- [Controls](controls.md) — keyboard and camera reference
- [Colonists](colonists.md) — stats, needs, and how workers behave
- [Buildings](buildings.md) — what you can build and what each structure does
- [Crafting](crafting.md) — workbenches, recipes, and gear
- [Research](research.md) — the tech tree and how to unlock new structures
- [Scenarios](scenarios.md) — the available game modes and how each one starts
- [Win Conditions](win_conditions.md) — how to win (and lose)

---

## Getting Started

Landing Party is an **indirect-control RTS**. You do not directly control individual colonists — you build structures, issue tasks, and your colonists figure out how to execute them. The game is turn-based and tile-based.

If you want to, you can also take **direct control** of a single colonist (Rogue mode) and pilot them around like a roguelike — see [Controls](controls.md#rogue-mode).

### The Colony

Your colony starts with a small group of colonists on an alien world. They will:

- Automatically seek out and complete queued tasks
- Eat food when hungry, or starve if none is available
- Defend themselves when attacked, but they are not fighters

Hostile alien creatures will spawn outside and attempt to push into your colony. Your goal is to survive long enough to meet the scenario's win condition.

### The World

The world is made up of multiple **Z-levels** organized into altitude bands:

| Band | Description |
|------|-------------|
| Underground | Rock, ore deposits, caverns beneath the surface |
| Surface | Regolith and open terrain where your colony starts |
| Atmosphere | Raised structures, upper platforms |
| Space | Orbital layer (scenario-dependent) |

Use **Q** and **E** to move between Z-levels. A small minimap widget is always visible in the top-right corner — click it or press **M** to open the full-screen map modal.

### Day and Night

Most scenarios use a **day/night cycle**. At night visibility is reduced and hostile spawns become more active. Build `work_light` structures to maintain visibility around your colony at night.

A few scenarios (notably abandoned-station modes) use **pitch dark** lighting — there is no ambient light at all and you depend entirely on placed lights and equipped colonist lights to see anything.

### Biomes

Planet-style scenarios pick from a mix of **biomes** (forest, plains, tundra, desert, alien forest, crystal field, regolith, asteroid rock, crater) based on temperature and humidity. Biome determines the surface tile, what grows there, and which scenario features appear nearby — desert biomes can hide radiation pockets, forest biomes can be peppered with alien flora, crystal fields are studded with crystal veins, and so on.
