# World Generation — Developer Guide

World generation creates the multi-level map the colony is placed on. It is driven by scenario configuration and tile definitions.

---

## Z-Level Bands

The world is divided into semantic altitude bands. Each band has a range of Z indices:

| Band | Description |
|------|-------------|
| Underground | Caverns, ore deposits, rock tunnels below the surface |
| Surface | Open regolith, the main colony layer |
| Atmosphere | Raised structures and upper platforms |
| Space | Orbital layer (scenario-dependent, currently unused) |

Z-level metadata is stored in `internal/world/level.go`. Each `Level` struct carries its band, lighting mode, and current sun intensity.

---

## Tile Definitions

Tile types are defined in `data/tile_definitions.json`.

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
3. **Features**: scenario-level and biome-level `features` (ore veins, radiation pockets, scattered entities) are placed in order.

Surface variation is driven by **Perlin noise**; underground levels use a cavern-carving algorithm. Generation parameters (noise scale, cavern density, feature counts) are configurable per scenario via the `world` block — see [Scenarios](scenarios.md#world-block).

---

## Lighting

Sun intensity is calculated per-turn in `internal/world/level.go` based on the lighting mode:

- `day_night`: intensity follows a sine curve over the day cycle. Full daylight at noon, darkness at midnight.
- `fixed`: intensity held constant at a configured value.
- `pitch_dark`: intensity is always 0; only placed `work_light` entities provide illumination.

Visibility range for colonists and the player camera is derived from ambient intensity plus any local light sources.

---

## Adding a New Tile Type

1. Add an entry to `data/tile_definitions.json` with a unique key.
2. Set `Passable`, `Opaque`, and optionally `ResourceRef`.
3. Add sprite variants to the tileset and reference them in `Variants`.
4. Reference the tile type in generation code (`internal/generation/`) or scenario spawn constraints (`TileConstraints`) as needed.
