# World Generation — Developer Guide

World generation creates the multi-level map the colony is placed on. It is driven primarily by **map definitions** (`data/maps/*.json` → `internal/mapdef`) — terrain primer, size, biome map, and features — with the scenario augmenting them (extra features, spawn rules). Everything derives from the location seed, so a given seed reproduces exactly (see [Determinism](#determinism)).

---

## Z-Level Bands

The world is divided into semantic altitude bands. Each band has a range of Z indices:

| Band | Description |
|------|-------------|
| Underground | Caverns, ore deposits, rock tunnels below the surface |
| Surface | Open regolith, the main colony layer (flat) |
| Sky | Air where flat, **mountains** (solid rock + interior caverns) where terrain rises |
| Space | Vacuum cap at the very top |

Z-level metadata is stored in `internal/world/level.go`. Each `Level` struct carries its band, lighting mode, and current sun intensity.

**Band allocation** (`generation.DefaultPlanetConfig`): bedrock at z=0, an underground band (rock + caverns), the single surface near the middle, then a tall "sky" band and a thin space cap. The surface is floored at `surfaceZ >= 4` so **every** size roll keeps an underground cavern band (caverns carve z in `[2, surfaceZ-2]`) and real mining depth, where a plain `depth/2` split used to collapse them to zero on shallow maps. Planet `z` ranges are ~12–14 (moon ~10–12) to leave room for both underground and mountains.

**The surface is a single, flat, walkable z-level** — it is *not* a heightmap. The default colonist pathfinder (`internal/path`) can only change z via stairs (no ramps/slopes exist), so a varying-height surface would strand colonists on unreachable terraces.

**Mountains** rise *above* the flat surface instead: where a low-frequency 2D noise field exceeds a threshold, the sky band is filled with solid rock (`PlanetPrimer` + `mountainHeight`), carved with interior caverns from the same 3D cave noise. At the colony layer a mountain reads as a rock obstacle you path around; its interior is reached exactly like the underground — mine into the base and build stairs up (building a stairs tile auto-carves its matched pair through rock). Params (per map `terrain_params`): `mountain_frequency` (~fraction of the map that is mountainous, default 0.15; Perlin concentrates near its midpoint, so the noise is spread before thresholding and the fraction is approximate) and `mountain_scale` (range breadth). Peak height is capped so the space layer stays clear. Note mountain and underground cave systems are separated by the solid surface layers today; through-surface shafts linking them would be a future addition.

---

## Tile Definitions

Tile types are defined in `data/tiledefinitions/tile_definitions.json`.

```json
"regolith": {
    "Name": "Regolith",
    "Passable": true,
    "Opaque": false,
    "Variants": ["regolith_a", "regolith_b"],
    "ResourceRef": ""
}
```

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | Display name |
| `Passable` | bool | Whether entities can move through this tile |
| `Opaque` | bool | Whether this tile blocks line of sight |
| `Variants` | []string | Sprite variant keys for visual variety |
| `ResourceRef` | string | Resource ID that can be harvested from this tile (e.g., `"ore"`, `"crystal"`) |

### Current Tile Types

| ID | Passable | Opaque | Resource |
|----|----------|--------|----------|
| `space` | false | false | — |
| `air` | true | false | — |
| `regolith` | true | false | — |
| `rock` | false | true | — |
| `bedrock` | false | true | — |
| `ore_deposit` | false | true | `ore` |
| `crystal_vein` | false | true | `crystal` |

---

## Level Generation

Generation runs in three phases (`internal/generation/`):

1. **Primer**: paints a default tile per terrain kind so a column has reasonable defaults even without a biome map.
2. **Biome application**: if the scenario provides a `world.biome_map`, each surface column samples temperature/humidity noise and the matching biome's `rules` paint the entire vertical column. See [Biomes](biomes.md).
3. **Features**: map-, scenario-, and biome-level `features` (ore veins, radiation pockets, scattered entities) are placed in order. A feature's `count` may be a fixed number or a rolled `[count, count_max]` range, and the areal kinds are additionally scaled by map area so density stays constant across the size roll — see [Scenarios → Feature Specs](scenarios.md#feature-specs).

Surface variation is driven by **Perlin noise**; underground levels use a cavern-carving algorithm. Generation parameters (noise scale, cavern density, feature counts) are configurable per map in `data/maps/*.json` and augmented per scenario via the `world` block — see [Scenarios](scenarios.md#world-block).

### Determinism

Generation is fully reproducible from a location's seed:

- Each location derives its size and terrain from `seed`; primers seed their Perlin/RNG sources from it.
- Feature placement, station layout, and setup/structure scripts draw from a single seed-derived `*rand.Rand` (never the global RNG), so the same seed reproduces the same map.
- Tile **variant** selection is hashed from tile position (`world.TileVariantAt`) rather than drawn from an RNG, so it stays reproducible even though primers paint across multiple goroutines.

The only intentionally non-reproducible generation randomness is gameplay-time deposit richness for mined rubble (`world.RollDepositRichness`), which is created during play rather than at generation.

---

## Lighting

Sun intensity is calculated per-turn in `internal/world/level.go` based on the lighting mode:

- `day_night`: intensity follows a sine curve over the day cycle. Full daylight at noon, darkness at midnight.
- `fixed`: intensity held constant at a configured value.
- `pitch_dark`: intensity is always 0; only placed `work_light` entities provide illumination.

Visibility range for colonists and the player camera is derived from ambient intensity plus any local light sources.

---

## Adding a New Tile Type

1. Add an entry to `data/tiledefinitions/tile_definitions.json` with a unique key.
2. Set `Passable`, `Opaque`, and optionally `ResourceRef`.
3. Add sprite variants to the tileset and reference them in `Variants`.
4. Reference the tile type in generation code (`internal/generation/`) or scenario spawn constraints (`TileConstraints`) as needed.
