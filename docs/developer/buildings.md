# Buildings — Developer Guide

Buildable structures are defined in `data/build.json`. Each entry describes what the player can construct, where it appears in the build menu, and the cost to do so. The build menu is fully data-driven — adding a wall, floor, door, etc. is a JSON edit; no Go code change required.

---

## build.json Structure

`build.json` is a JSON object keyed by build ID. Each entry describes one buildable.

```json
"hull_wall": {
    "name": "Hull Wall",
    "type": "hull_wall",
    "description": "A reinforced metal wall panel.",
    "build_time": 1,
    "allow_multiple": true,
    "cost": {},
    "category": "walls",
    "order": 1
}
```

| Field | Type | Description |
|-------|------|-------------|
| key | string | Build ID. Used as the map key, the `BuildOptionChangedEvent` payload, and (by convention) usually the tile/entity type too. |
| `name` | string | Display name shown in the build menu. |
| `type` | string | Tile or entity type to place on completion. |
| `description` | string | Short description shown in the menu and tooltip. |
| `build_time` | int | Worker-ticks to complete construction. |
| `is_entity` | bool | If true, spawn an entity blueprint instead of placing a tile. The blueprint must exist in `data/blueprints/entities/`. |
| `allow_multiple` | bool | If true, the player can queue many at once. Walls/floors/lights are true; unique stations are typically false. |
| `hidden` | bool | If true, omitted from the menu and from `ListBuildablesForTechs`. Useful for items only buildable via scripts. |
| `required_tech` | string | Optional tech ID from `data/research.json`. When set, the item appears in the menu but is greyed out until the colony has researched the named tech. |
| `cost` | object | Resource costs: `{ "metal_ore": 5 }`. Empty object = free. |
| `category` | string | Build-menu category this buildable belongs to. Empty string = not shown in the menu. See [Categories](#categories) below. |
| `order` | int | Sort order within the category. Lower first; ties fall back to `name`. Omit (default 0) if order doesn't matter. |

---

## Categories

The category list (id, label, display order) lives in `internal/gui/hud_screen.go` because one entry — `Orders` — holds cursor-mode actions (Dig, Mine, Attack…) that aren't buildables and can't be expressed in JSON.

Current categories:

| ID | Label | Notes |
|----|-------|-------|
| `orders` | Orders | Cursor modes only; not driven by `build.json`. |
| `walls` | Walls | `hull_wall`, `bunker_wall`, … |
| `floors` | Floors | `hull_floor`, `bunker_floor`, … |
| `stairs` | Stairs | `stairs_up`, `stairs_down`. |
| `structures` | Structures | Crafting stations, storage, lights, research lab, etc. |
| `doors` | Doors | `airlock`, `blast_door`. |

A buildable's `category` field must match one of these IDs. An unrecognised category means the buildable is loaded by `construction.GetBuildable` but never surfaced in the menu — equivalent to leaving the field empty.

### Adding a new category

Rare, but: open `internal/gui/hud_screen.go`, find the `categories := []buildCategory{…}` literal, and append `{id: "<id>", label: "<Label>"}` in the position you want it to appear. Buildables in `build.json` with `"category": "<id>"` will then populate it automatically.

---

## Adding a New Buildable

1. Add an entry to `data/build.json` with a unique key.
2. Set `is_entity: true` if the structure spawns an interactive entity (storage, workbench, light). Ensure a matching blueprint exists in `data/blueprints/entities/`. Otherwise it's a tile placement and `type` should match a tile name in `data/tiledefinitions/*.json`.
3. Pick a `category` and an `order` within it.
4. (Optional) Set `required_tech` to gate behind research.
5. (Optional) Add `cost` once balancing is settled.
6. Restart the game. The entry appears under its category automatically.

> All costs are currently zeroed for testing. Set realistic costs before shipping.

---

## How it's wired

- `internal/construction/buildables.go` loads `build.json` once at package init into `map[string]Buildable`.
- `GetBuildable(key)` returns one entry by build ID (used by the place-tile-on-click handler).
- `BuildableTypesInCategory(category)` returns sorted build IDs for one category. The HUD calls this when opening a submenu — so re-opening a menu after a code edit to `build.json` requires a restart, but switching submenus within a session always re-fetches.
- `ListBuildablesForTechs(knownTechs)` returns all non-hidden buildables that pass tech gating. Used by the research-aware UI surfaces.
