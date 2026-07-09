# Biomes — Developer Guide

Biomes describe how a column of the world is painted from space down through bedrock. Each biome is a small JSON file in `data/biomes/`. A scenario's `world.biome_map` selects which biomes are eligible and how they are distributed across the surface.

The loader is `internal/generation/biome.go`: every `.json` file in `data/biomes/` is read once at startup and registered by its `id`. Missing the directory entirely is non-fatal (scenarios that don't use a biome map still produce playable terrain via the primer).

---

## Biome File Structure

```json
{
  "id": "forest",
  "temp_range": [0.35, 0.7],
  "humidity_range": [0.45, 1.0],
  "rules": [
    { "kind": "surface",     "tile": "grass" },
    { "kind": "subsurface",  "tile": "dirt" },
    { "kind": "underground", "tile": "rock" },
    { "kind": "cavern",      "tile": "air" },
    { "kind": "atmosphere",  "tile": "air" },
    { "kind": "space",       "tile": "space" },
    { "kind": "bedrock",     "tile": "bedrock" }
  ],
  "features": []
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique biome identifier (filename does not need to match) |
| `temp_range` | [2]float | Min/max normalized temperature (0..1) where this biome is eligible |
| `humidity_range` | [2]float | Min/max normalized humidity (0..1) where this biome is eligible |
| `rules` | []BiomeRule | Ordered list of terrain-kind → tile mappings. First match wins. |
| `features` | []FeatureSpec | Optional biome-scoped features (ore veins, scatter entities, etc.). Same schema as scenario/map features — including `count`/`count_max` range rolling and area scaling. See [Scenarios → Feature Specs](scenarios.md#feature-specs). |

### BiomeRule

| Field | Type | Description |
|-------|------|-------------|
| `kind` | string | Terrain kind: `space`, `atmosphere`, `surface`, `subsurface`, `underground`, `cavern`, `bedrock`, `water` |
| `y_offset` | int? | Optional Z offset relative to that column's surface. Negative = below, positive = above. If omitted the rule matches any Z of that kind. |
| `tile` | string | Tile type ID from `data/tiledefinitions/tile_definitions.json` |
| `radiation` | int | Optional baseline radiation level for tiles painted by this rule |

Rules are evaluated in order; the first rule whose `kind` (and optional `y_offset`) matches is applied. This lets you, for example, paint the topmost subsurface tile differently from deeper subsurface.

---

## How Biomes Are Selected

A scenario picks biomes via the `world.biome_map` block:

```json
"world": {
    "biome_map": {
        "type": "perlin_temp_humidity",
        "scale": 80,
        "biomes": ["tundra", "forest", "plains", "desert"]
    }
}
```

| Field | Description |
|-------|-------------|
| `type` | Distribution strategy. `perlin_temp_humidity` samples two Perlin fields (temperature and humidity) per surface column. |
| `scale` | Noise scale; larger = bigger contiguous biome regions. |
| `biomes` | The candidate biome IDs. The biome whose `temp_range` and `humidity_range` contain the column's sampled (T, H) is chosen. |

If no listed biome covers a column's (T, H), the primer's default tiles remain — you should choose ranges that tile the (T, H) unit square if you want full coverage.

---

## Existing Biomes

| ID | Theme |
|----|-------|
| `forest` | Temperate, grass surface, dirt subsurface |
| `plains` | Temperate, mid-humidity grass/dirt |
| `tundra` | Cold, low humidity |
| `desert` | Hot, dry, regolith surface |
| `alien_forest` | Alien biome with custom flora features |
| `crater` | Impact-pocked surface |
| `regolith` | Bare lunar-style surface |
| `asteroid_rock` | Vacuum atmosphere, rock subsurface |
| `crystal_field` | Crystal vein deposits underground |

---

## Adding a New Biome

1. Create a new file in `data/biomes/` (e.g. `swamp.json`). The filename is for humans only — `id` is what matters.
2. Set a unique `id` and the `temp_range` / `humidity_range` covering where this biome should appear.
3. Define one `rules` entry per `kind` you care about. Always include `surface`, `subsurface`, `underground`, `cavern`, `atmosphere`, `space`, and `bedrock` so the entire vertical column has tiles.
4. Reference any new tile types in `data/tiledefinitions/tile_definitions.json` first.
5. Add the new biome ID to the `biomes` array of any scenario's `world.biome_map` that should include it. Make sure the new biome's (T, H) range overlaps the scenario's noise output.
6. (Optional) Add `features` for biome-scoped feature spawning. Scenario-level `world.features` can also target biomes via the `biome` field.
