# Biomes — Developer Guide

Biomes describe how a column of the world is painted from space down through bedrock. Each biome is a small JSON file in `data/biomes/`. A **map's** `biome_map` (`data/maps/*.json`) selects which biomes are eligible and how they are distributed across the surface.

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
| `tile` | string | Tile type ID (see [Tile Definitions](tile_definitions.md)) |
| `radiation` | int | Optional baseline radiation level for tiles painted by this rule |

Rules are evaluated in order; the first rule whose `kind` (and optional `y_offset`) matches is applied. This lets you, for example, paint the topmost subsurface tile differently from deeper subsurface.

---

## How Biomes Are Selected

A map picks biomes via its `biome_map` block:

```json
"world": {
    "biome_map": {
        "type": "perlin_temp_humidity",
        "scale": 80,
        "latitude_weight": 0.7,
        "biomes": ["tundra", "forest", "plains", "desert"]
    }
}
```

| Field | Description |
|-------|-------------|
| `type` | Distribution strategy. `perlin_temp_humidity` samples two Perlin fields (temperature and humidity) per surface column. `uniform` paints a single biome (`single`). |
| `scale` | Noise scale; larger = bigger contiguous biome regions. |
| `latitude_weight` | 0..1 (default 0). Blends a pole-to-equator gradient into **temperature**: 0 = pure noise (isotropic blobs); 1 = pure bands (cold at the top/bottom edges, warm in the middle). Humidity stays pure noise. Raise it so temperature-keyed biomes (polar tundra, equatorial desert) reliably appear and read as climate bands. |
| `biomes` | The candidate biome IDs, matched against each column's sampled (T, H). |

**Matching.** Each column is assigned by its `temp_range`/`humidity_range`, resolving ambiguity by **nearest centroid** (the center of a biome's T/H box): when a point falls in exactly one box it uses that biome; when it falls in several (overlapping ranges) or none (a gap), the biome whose centroid is closest wins. This means the result never depends on the order biomes are listed, and gaps no longer dump into the first-listed biome. Note that because the noise concentrates near the middle of the T/H square, biomes with centrally-placed centroids get the largest share; use `latitude_weight` (and centroid placement) to give extreme biomes a foothold.

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
4. Define any new tile types first (see [Tile Definitions](tile_definitions.md)).
5. Add the new biome ID to the `biomes` array of any map's `biome_map` that should include it. Placement is nearest-centroid, so the ranges don't have to tile the (T, H) square perfectly — but the biome's territory is the region of climate space closest to its centroid, so place that centroid where you want it to appear (and remember the noise clusters near the middle).
6. (Optional) Add `features` for biome-scoped feature spawning. Map- and scenario-level `features` can also target biomes via the `biome` field.
