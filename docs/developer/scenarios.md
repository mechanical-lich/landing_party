# Scenarios — Developer Guide

Scenarios are defined as JSON files in `data/scenarios/`. Each file configures the hostile spawns, lighting, setup scripts, and any extra feature placements layered onto a generated site. Scenarios are not selected at game start — the procedural generator binds each generated system to a `scenario_id` (and `map_id`) via `data/location_templates.json`, and travelling there loads that scenario. **Terrain, size, and the biome map come from the map definition** (`data/maps/*.json`), not the scenario. See [Campaign & Overworld](campaign.md) and [World Generation](world_generation.md).

---

## Scenario File Structure

```json
{
    "id": "survival",
    "name": "Survival",
    "description": "Survive on a hostile alien world.",
    "enabled": true,
    "supported_maps": ["earth_like", "alien"],
    "setup_scripts": ["data/scripts/scenarios/survival_setup.basic"],
    "hostile_max": 20,
    "lighting": { "mode": "day_night" },
    "spawn_rules": { ... },
    "features": [ ... ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique scenario identifier |
| `name` | string | Display name |
| `description` | string | Short flavor description |
| `enabled` | bool | Whether the generator may bind systems to this scenario (`scenario.AllEnabled()`) |
| `supported_maps` | []string | Map IDs (`data/maps/*.json`) this scenario may run on; empty/omitted = any map |
| `setup_scripts` | []string | Paths to `.basic` scripts run once at site setup |
| `hostile_max` | int | Maximum concurrent hostile entities before spawning pauses |
| `lighting.mode` | string | Lighting mode: `"day_night"`, `"fixed"`, or `"pitch_dark"` |
| `spawn_rules` | object | Ongoing hostile-spawn rules keyed by blueprint ID (see below) |
| `features` | []FeatureBlock | Optional. Extra feature placements layered onto the map's own, run after biome painting (see [Feature Specs](#feature-specs)) |

> Scenarios no longer define win/loss conditions. The old `win_conditions`
> block was removed; objectives are procedurally generated quests using the
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
        "min_z_delta": 0,
        "max_z_delta": 0,
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
| `light_max` | int | Optional. Maximum ambient light level (0 = no upper bound) |
| `tiles` | []string | Tile types the entity may spawn on |
| `min_z_delta` | int? | Lower Z bound as an **offset from the surface** (`SurfaceZ`); `null`/omitted = no lower bound. |
| `max_z_delta` | int? | Upper Z bound as an offset from the surface; `null`/omitted = no upper bound. |
| `starting_equipment` | map[string]float | Optional. Map of item blueprint ID → probability (0..1) of being equipped at spawn. |

Z bounds are **deltas from the column's `SurfaceZ`**, not absolute Z, so the same rule scales across maps whose surface sits at different depths: `{min:0, max:0}` = on the surface, `{min:null, max:-1}` = anywhere underground, `{min:1, max:null}` = above-surface (aerial). Spawning stops for all rules when the total hostile count reaches `hostile_max`.

---

## Features

A scenario's top-level `features` array places extra content on top of what the **map** already generates. Terrain, size, and the biome map belong to the map definition (`data/maps/*.json`) — see [World Generation](world_generation.md) and [Biomes](biomes.md); the scenario only adds these placements, run after biome painting in the order listed.

```json
"features": [
    { "kind": "stamp", "count": 1, "params": { "script": "crashed_ufo", "w": 17, "h": 17 } },
    { "kind": "scatter_entity", "count": 50, "count_max": 120, "biome": "forest", "params": { "blueprint": "alien_flora", "kind": "surface" } },
    { "kind": "cave_spawner", "count": 3, "count_max": 8, "params": { "blueprint": "cave_lurker", "zone": "mountain" } }
]
```

Map definitions carry their own `features` block with the identical schema, and biomes can carry biome-scoped features — all three run during the feature phase.

### Feature Specs

Each entry in `features` has:

| Field | Description |
|-------|-------------|
| `kind` | Feature kind. Built-ins: `ore_vein`, `radiation_pocket`, `crystal_grove`, `lava_lake`, `scatter_entity`, `scatter_tile`, `fauna_spawner`, `derelict_pod`, `botany_bay`, `structure`, `stamp`, `cave_spawner`, `buried_spawner`. |
| `count` | Number of placements to attempt. When `count_max` is set this is the **minimum** of a rolled range. |
| `count_max` | Optional. When greater than `count`, the placement count is rolled uniformly in `[count, count_max]` per generation, so sibling maps of the same type vary instead of reading as copies. Omit (or set ≤ `count`) for a fixed count. |
| `biome` | Optional. Restricts placement to columns of this biome. Placement samples the biome's columns directly, so a restricted feature fills its count even when the biome is a small fraction of a large map (rather than a uniform sampler mostly missing it). |
| `params` | Per-kind parameters (e.g. `tile`, `radius`, `blueprint`, `peak`). |

Features can also be defined per-biome inside the biome JSON; those run for any column tagged with that biome regardless of scenario.

#### Placer kinds

Each `kind` places something specific; `params` are per-kind (defaults in parentheses; `req` = required). All kinds also accept `count`/`count_max`; the surface/underground kinds additionally accept `biome` (restrict to a biome) and `in_region`/`jitter` (place near a tagged region anchor).

| Kind | Places | Where | Key params | Area-scaled |
|------|--------|-------|------------|:-----------:|
| `ore_vein` | deposit tiles | underground rock | `tile` (`ore_deposit`), `radius` (3), `radius_max` (opt; rolls each vein's radius in `[radius, radius_max]`), `density` (1.0), `min_z`/`max_z` | ✓ |
| `radiation_pocket` | radiation field + ore seeds | surface (or `min_z`/`max_z`) | `peak` (180), `radius` (4), `ore_tile` (`radioactive_ore`) | ✓ |
| `crystal_grove` | crystal entities | surface | `blueprint` (`alien_crystal`), `radius` (4) | ✓ |
| `lava_lake` | lava tiles | surface | `tile` (`lava`), `radius` (5) | ✓ |
| `scatter_tile` | single tiles | surface of a `kind` | `tile` (req), `kind` (`surface`) | ✓ |
| `scatter_entity` | single entities | surface of a `kind` | `blueprint` (req), `kind` (`surface`) | ✓ |
| `fauna_spawner` | entities | open air above surface | `blueprint` (req) | — |
| `derelict_pod` | entity | open air above surface | `blueprint` (req) | — |
| `botany_bay` | soil tiles + plants | tagged station room, else surface | `tile` (`dirt`), `plant_blueprint`, `radius` (3) | — |
| `structure` | entity | `region` anchor, else surface | `blueprint` (req), `region` | — |
| `stamp` | runs a structure script | surface | `script` (req), `w` (9), `h` (7) | — |
| `cave_spawner` | entities | open cavern tiles | `blueprint` (req), `zone` (`any`) | ✓ |
| `buried_spawner` | entities | inside solid rock | `blueprint` (req), `zone` (`any`) | ✓ |

Unknown blueprints/tiles skip cleanly (one-time warning), and every placer logs a shortfall when it can't fill its count.

#### Cave and buried spawners

`cave_spawner` and `buried_spawner` place entities *below or inside the terrain* rather than on the surface — the way to populate caverns and mountains (which surface placers skip):

- **`cave_spawner`** spawns in **open cavern tiles** (`TKCavern`) — cave-dwelling creatures, loot. Reached by mining in.
- **`buried_spawner`** spawns **inside solid rock** — buried caches, fossils, dormant creatures revealed when the rock is mined out. (Use a blueprint that behaves sensibly while encased.)

Both take `params.blueprint` (required) and `params.zone`, which filters by elevation relative to the surface: `"underground"` (below), `"mountain"` (above, i.e. inside mountains), or `"any"` (default). Counts roll (`count`/`count_max`) and area-scale like other features. Example:

```json
{ "kind": "cave_spawner", "count": 3, "count_max": 8, "params": { "blueprint": "cave_lurker", "zone": "mountain" } }
```

#### Count rolling and area scaling

Two things adjust the authored count at generation time, both deterministically (derived from the location seed, so the same seed reproduces exactly):

1. **Range roll** — if `count_max > count`, the count is rolled in `[count, count_max]`. Applies to every feature kind.
2. **Area scaling** — the *areal* kinds (`ore_vein`, `radiation_pocket`, `crystal_grove`, `lava_lake`, `scatter_entity`, `scatter_tile`, `cave_spawner`, `buried_spawner`) then multiply their rolled count by `rolledArea / referenceArea`, where the reference is the map's expected footprint (`SizeBlock` width×height midpoints). This keeps **density** constant across the map-size roll instead of thinning out on larger maps. Singular/region-anchored kinds (`derelict_pod`, `botany_bay`, `structure`, `stamp`, `fauna_spawner`) roll their range but are **not** area-scaled.

The final count is `round(roll × areaScale)`, floored at 1 so an authored feature never drops to zero. Order is roll → scale, so a planet varies on both axes: two `earth_like` worlds might land at ~4 vs ~9 ore veins.

---

## Objectives (formerly win/loss conditions)

Scenarios no longer carry win/loss rules. The rule engine that used to live in
`internal/wincondition` is now `internal/objective` (a generic condition
engine, no win/lose framing) and is consumed by procedurally generated
**quests**. A quest's `objective` is one `objective.Rule` using the
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

A setup script defines an `on_setup()` function. A representative slice of the builtins available:

- **Flags** set by the scenario loader: `start_x`, `start_y`, `start_z`, `settlement_name`, `colonist_faction`. Read with `get_flag(name)`.
- **Tile placement**: `set_tile(x, y, z, tile_type)`, `clear_middle(x, y, z)`, room carving (`carve_room`, `carve_rect`, `carve_circle`).
- **Entity spawning**: `spawn_entity(blueprint, x, y, z)`, `spawn_entity_owned(blueprint, x, y, z, faction_or_settlement)`, `spawn_entity_named(...)`.
- **Feature placement**: `place_feature(kind, count)` invokes a registered feature placer (see [Feature Specs](#feature-specs)); `gen_structure(name, x, y, w, h)` stamps a structure script.
- **Queries**: `find_open_tile(z)`/`get_open_x()`/`get_open_y()`, `get_surface_z(x, y)`, `get_biome(x, y)`, region lookups (`get_region_x/y/z`, `get_region_count`), `get_width()`/`get_height()`.
- **Messaging**: `add_message(text)` writes to the in-game event log.
- Standard control flow: `for`/`next`, `if`/`then`/`else`/`endif`, arithmetic, `rnd_int(n)`.

The full builtin set (shared with structure scripts) is documented in [Structure Scripts](structure_scripts.md#builtins-available-in-structure-scripts). Setup-script randomness is seeded per-location, so a site's scripted content reproduces from its seed.

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
4. (Optional) Set `supported_maps` to restrict which maps it runs on, and add a top-level `features` array for scenario-specific placements (terrain/biomes come from the map).
5. Add a setup script in `data/scripts/scenarios/<id>_setup.basic` for starting structures and colonists, then list it in `setup_scripts`.
6. Add the scenario's `id` to a `data/location_templates.json` archetype's `scenarios` pool so the generator can bind systems to it; add matching `data/quest_templates.json` entries for any objectives.
