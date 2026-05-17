# Scenarios — Developer Guide

Scenarios are defined as JSON files in `data/scenarios/`. Each file configures spawn rules, win/loss conditions, and world parameters for one game mode. Scenarios are no longer selected at game start — each overworld location binds a `scenario_id` (and `map_id`); travelling there loads that scenario. See [Campaign & Overworld](campaign.md) and `data/overworld.json`.

---

## Scenario File Structure

```json
{
    "id": "survival",
    "name": "Survival",
    "description": "Survive 30 days on a hostile alien world.",
    "enabled": true,
    "setup_scripts": ["data/scripts/scenarios/survival_setup.basic"],
    "hostile_max": 20,
    "lighting": { "mode": "day_night" },
    "spawn_rules": { ... },
    "world": { ... }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique scenario identifier |
| `name` | string | Display name |
| `description` | string | Short description shown at scenario selection |
| `enabled` | bool | Whether the scenario appears in the picker |
| `setup_scripts` | []string | Paths to `.basic` scripts run once at scenario start |
| `hostile_max` | int | Maximum concurrent hostile entities before spawning pauses |
| `lighting.mode` | string | Lighting mode: `"day_night"`, `"fixed"`, or `"pitch_dark"` |
| `spawn_rules` | object | Spawn rules keyed by blueprint ID (see below) |
| `world` | object | Optional world generation config: biome map and feature placement (see below) |

> Scenarios no longer define win/loss conditions. The old `win_conditions`
> block was removed; objectives now live in `data/quests.json` and use the
> shared condition engine in `internal/objective` (see
> [Campaign & Overworld](campaign.md)).

---

## Spawn Rules

`spawn_rules` is a JSON object keyed by blueprint ID. Each entry configures how that entity type spawns.

```json
"spawn_rules": {
    "mutant_grunt": {
        "spawn_rate": 40,
        "light_min": 0,
        "tiles": ["regolith", "dirt"],
        "min_z": 5,
        "max_z": 5,
        "starting_equipment": {
            "knife": 0.5,
            "laser_pistol": 0.5
        }
    }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `spawn_rate` | int | Inverse probability — lower = more frequent. Checked as `rnd(spawn_rate) == 0` each turn. |
| `light_min` | int | Minimum ambient light level required for this entity to spawn |
| `tiles` | []string | Tile types the entity may spawn on |
| `min_z` | int | Minimum Z-level to spawn on |
| `max_z` | int | Maximum Z-level to spawn on |
| `starting_equipment` | map[string]float | Optional. Map of item blueprint ID → probability (0..1) of being equipped at spawn. |

Spawning stops for all rules when the total hostile count reaches `hostile_max`.

---

## World Block

The optional `world` block configures terrain generation, biome distribution, and one-time feature placement.

```json
"world": {
    "terrain": "planet",
    "biome_map": {
        "type": "perlin_temp_humidity",
        "scale": 80,
        "biomes": ["tundra", "forest", "plains", "desert"]
    },
    "features": [
        { "kind": "ore_vein", "count": 30, "params": { "tile": "ore_deposit", "radius": 3 } },
        { "kind": "ore_vein", "count": 15, "params": { "tile": "crystal_vein", "radius": 2 } },
        { "kind": "radiation_pocket", "count": 2, "biome": "desert", "params": { "peak": 160, "radius": 4 } },
        { "kind": "scatter_entity", "count": 80, "biome": "forest", "params": { "blueprint": "alien_flora", "kind": "surface" } }
    ]
}
```

| Field | Description |
|-------|-------------|
| `terrain` | High-level terrain template (e.g. `"planet"`, `"asteroid"`, `"station"`) |
| `biome_map` | Biome distribution. See [Biomes](biomes.md) for the full schema. Omit if the scenario should use a single uniform terrain. |
| `features` | Ordered list of feature placements applied after biome painting |

### Feature Specs

Each entry in `features` has:

| Field | Description |
|-------|-------------|
| `kind` | Feature kind. Built-ins include `ore_vein`, `radiation_pocket`, `scatter_entity`. |
| `count` | Number of placements to attempt |
| `biome` | Optional. Restricts placement to columns whose biome matches this ID. |
| `params` | Per-kind parameters (e.g. `tile`, `radius`, `blueprint`, `peak`). |

Features can also be defined per-biome inside the biome JSON; those run for any column tagged with that biome regardless of scenario.

---

## Objectives (formerly win/loss conditions)

Scenarios no longer carry win/loss rules. The rule engine that used to live in
`internal/wincondition` is now `internal/objective` (a generic condition
engine, no win/lose framing) and is consumed by **quests** in
`data/quests.json`. A quest's `objective` is one `objective.Rule` using the
same triggers (`days_survived`, `resource_gathered`, `entity_killed`,
`tech_researched`, `structure_built`, …). See
[Campaign & Overworld](campaign.md) for the quest schema and lifecycle.

---

## Lighting Modes

| Mode | Description |
|------|-------------|
| `day_night` | Sun intensity cycles over time; darkness at night reduces visibility |
| `fixed` | Constant lighting level, no cycle |
| `pitch_dark` | No ambient light; only placed `work_light` entities provide visibility |

---

## Setup Scripts

`setup_scripts` is a list of `.basic` scripts in `data/scripts/scenarios/` run once when the scenario starts. They place the starting structures, spawn the initial colonists, and seed any scenario-specific entities.

A setup script defines an `on_setup()` function and has access to:

- **Flags** set by the scenario loader: `start_x`, `start_y`, `start_z`, `settlement_name`, `colonist_faction`. Read with `get_flag(name)`.
- **Tile placement**: `set_tile(x, y, z, tile_type)`.
- **Entity spawning**: `spawn_entity(blueprint, x, y, z)`, `spawn_entity_owned(blueprint, x, y, z, faction_or_settlement)`.
- **Tile queries**: `find_open_tile(z)`, `get_open_x()`, `get_open_y()`.
- **Messaging**: `add_message(text)` writes to the in-game event log.
- Standard control flow: `for`/`next`, `if`/`then`/`else`/`endif`, arithmetic.

Example skeleton (see `data/scripts/scenarios/survival_setup.basic` for a complete one):

```basic
function on_setup()
    sx = get_flag("start_x")
    sy = get_flag("start_y")
    sz = get_flag("start_z")
    sname = get_flag("settlement_name")
    faction = get_flag("colonist_faction")

    add_message("Day 1. The colony pod has landed.")

    set_tile(sx, sy, sz, "hull_floor")
    spawn_entity_owned("storage_locker", sx + 1, sy, sz, sname)
    spawn_entity_owned("airlock", sx, sy + 1, sz, faction)
endfunction
```

---

## Adding a New Scenario

1. Create a new file in `data/scenarios/` (e.g., `siege.json`).
2. Set a unique `id` and `enabled: true`.
3. Add entries to `spawn_rules` for each entity type you want to appear during gameplay.
4. (Optional) Add a `world` block selecting biomes and listing one-time features.
5. Add a setup script in `data/scripts/scenarios/<id>_setup.basic` for starting structures and colonists, then list it in `setup_scripts`.
6. Bind the scenario to a Star Map location in `data/overworld.json` (`scenario_id`); add quests in `data/quests.json` for any objectives.
