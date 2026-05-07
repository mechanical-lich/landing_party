# Buildings — Developer Guide

Buildable structures are defined in `data/build.json`. Each entry describes what the player can construct and the cost to do so.

---

## build.json Structure

`build.json` is a JSON object keyed by build ID. Each entry describes one buildable structure.

```json
"hull_wall": {
    "name": "Hull Wall",
    "type": "hull_wall",
    "description": "A reinforced metal wall panel.",
    "build_time": 1,
    "cost": {}
}
```

| Field | Type | Description |
|-------|------|-------------|
| key | string | Build ID (also used as the tile/entity type to place) |
| `name` | string | Display name shown in the build menu |
| `type` | string | Tile or entity type to place on completion |
| `description` | string | Short description shown in the build menu |
| `build_time` | int | Number of worker-ticks to complete construction |
| `is_entity` | bool | If true, spawns an entity blueprint instead of placing a tile |
| `cost` | object | Resource costs: `{ "metal": 5 }`. Empty object = free. |

There is no `RequiresTech` field in `build.json` — tech gating is enforced by the build system reading `data/research.json` and checking `settlement.KnownTechs`.

---

## Existing Buildable Structures

| Build ID | Is Entity | Description |
|----------|-----------|-------------|
| `hull_wall` | no | Solid wall tile |
| `hull_floor` | no | Passable floor tile |
| `storage_locker` | yes | Resource container |
| `research_lab` | yes | Required for research tasks |
| `work_light` | yes | Illumination source |
| `stairs_up` | no | Connects to the level above |
| `stairs_down` | no | Connects to the level below |
| `airlock` | yes | Faction-aware door |
| `blast_door` | yes | Heavy reinforced door |
| `workbench` | yes | Required for crafting tasks |

> Note: All costs are currently set to zero for testing. Set realistic costs before shipping.

---

## Adding a New Buildable Structure

1. Add an entry to `data/build.json` with a unique key and the appropriate fields.
2. Set `is_entity: true` if the structure should spawn an entity (interactive, has inventory, etc.); omit it for simple tile placements.
3. If `is_entity` is true, ensure a matching blueprint exists in `data/entity_blueprints.json`.
4. The build menu picks up the new entry on next load.
