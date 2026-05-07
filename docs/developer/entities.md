# Entities & Blueprints — Developer Guide

All entities are defined in `data/entity_blueprints.json`. Each blueprint is a key-value entry where the key is the blueprint ID and the value is a map of component definitions.

---

## Blueprint Structure

```json
"alien_grunt": {
    "Description": {
        "Name": "Alien Grunt",
        "Faction": "alien"
    },
    "Appearance": {
        "R": 120, "G": 220, "B": 120,
        "Bounces": true,
        "Resource": "scifi_entities",
        "SpriteX": 0, "SpriteY": 360
    },
    "Health": { "MaxHealth": 12 },
    "Stats": { "Str": 8, "Int": 4, "Dex": 12, "AC": 8, "BasicAttackDice": "1d6" },
    "FactionAI": { "BehaviorKey": "hostile", "Faction": "alien" },
    "Initiative": { "DefaultValue": 12, "Ticks": 1 },
    "AIMemory": {},
    "Solid": {}
}
```

---

## Core Components

### Description

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | Display name shown in the HUD |
| `Faction` | string | Faction membership (`"colony"`, `"alien"`, etc.) |

### Appearance

| Field | Type | Description |
|-------|------|-------------|
| `R`, `G`, `B` | int | RGB color tint (0–255) applied to the sprite |
| `Bounces` | bool | Whether the sprite plays a bounce animation |
| `Resource` | string | Sprite sheet asset key (from `data/assets.json`) |
| `SpriteX`, `SpriteY` | int | Pixel offset into the sprite sheet |

### Health

| Field | Type | Description |
|-------|------|-------------|
| `MaxHealth` | int | Maximum (and starting) hit points |

### Stats

| Field | Type | Description |
|-------|------|-------------|
| `Str` | int | Strength — affects physical task speed and melee damage |
| `Int` | int | Intelligence — affects research and crafting speed |
| `Dex` | int | Dexterity — affects gathering and fine manipulation speed |
| `AC` | int | Armor class — reduces incoming damage |
| `BasicAttackDice` | string | Dice expression for unarmed attacks (e.g. `"1d4"`) |

Stats act as **multipliers on task duration**: higher stat = faster task completion.

### Worker

Marks an entity as a colonist worker subject to `WorkerSystem`. Required for any entity that should receive and execute tasks.

```json
"Worker": {}
```

### Hunger

| Field | Type | Description |
|-------|------|-------------|
| `Energy` | float | Current energy level |
| `MaxEnergy` | float | Energy cap |
| `DrainRate` | float | Energy lost per turn |
| `HungerThreshold` | float | Energy level at which the worker seeks food |
| `StarveThreshold` | float | Energy level at which the worker takes damage |

When `Energy < HungerThreshold`, the worker interrupts its current task to find food. When `Energy < StarveThreshold`, it takes damage each turn.

### FactionAI

Controls hostile NPC behavior. Used by alien entities.

| Field | Type | Description |
|-------|------|-------------|
| `BehaviorKey` | string | Lookup key into the behavior profile (currently `"hostile"`) |
| `Faction` | string | Faction identifier used for friend/foe resolution |

See [AI Systems](ai.md) for behavior details.

### Initiative

Controls turn order and action rate.

| Field | Type | Description |
|-------|------|-------------|
| `DefaultValue` | int | Base initiative value; higher acts sooner |
| `Ticks` | int | Number of actions per initiative cycle |

### Inventory

| Field | Type | Description |
|-------|------|-------------|
| `StartingInventory` | []string | List of blueprint IDs the entity starts with in its inventory |

### Light

| Field | Type | Description |
|-------|------|-------------|
| `Level` | int | Brightness of emitted light (0–100) |
| `Range` | int | Radius in tiles that the light reaches |

### AIMemory

Empty marker component. Required for entities that use faction AI to track remembered targets.

```json
"AIMemory": {}
```

### Solid

Empty marker component. Makes the entity impassable — other entities cannot move through it.

```json
"Solid": {}
```

---

## Adding a New Entity

1. Open `data/entity_blueprints.json`.
2. Add a new top-level key (the blueprint ID).
3. Add the required components for your entity type:
   - **Hostile NPC**: `Description`, `Appearance`, `Health`, `Stats`, `FactionAI`, `Initiative`, `AIMemory`, `Solid`
   - **Colonist**: `Description`, `Appearance`, `Health`, `Stats`, `Worker`, `Hunger`, `Initiative`, `Light`, `Inventory`, `Solid`
   - **Item**: `Description`, `Appearance` (no `Solid` — items are walkable)
   - **Structure/Building**: typically defined via `build.json` type; add an entity blueprint only if it has interactive components
4. Reference the blueprint ID in `data/build.json` if it should be player-buildable, or in `data/scenarios/` spawn rules if it should appear during gameplay.

---

## Existing Entity Categories

| Category | Blueprint IDs |
|----------|--------------|
| Hostile aliens | `alien_grunt`, `alien_scout`, `alien_brute` |
| Ambient creatures | `critter` |
| Colonists | `colonist` |
| Structures (entities) | `airlock`, `blast_door`, `storage_locker`, `research_lab`, `workbench`, `work_light` |
| Items — weapons | `laser_pistol`, `stun_baton`, `plasma_cutter` |
| Items — armor | `combat_vest`, `enviro_suit`, `helmet` |
| Items — consumables | `ration_pack`, `protein_bar`, `medkit`, `hydration_gel` |
| Resources | `metal_ore`, `crystal`, `stone` |
