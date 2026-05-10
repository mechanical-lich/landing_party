# Crafting — Developer Guide

Crafting turns input resources into output items at a workbench. The recipe registry is `data/crafting_recipes.json` and crafting stations are entity blueprints with a `CraftingStation` component.

Loader: `internal/crafting/crafting.go` reads `data/crafting_recipes.json` once at startup. Stations are instantiated like any other entity blueprint via `factory.FactoryLoadDir`.

---

## Recipe Structure

`data/crafting_recipes.json` is a JSON object keyed by recipe ID.

```json
"medkit": {
    "name": "Medkit",
    "description": "Emergency medical supplies. Restores health when used.",
    "output": "medkit",
    "build_time": 1,
    "cost": { "biomass": 1 },
    "station": "basic_workbench"
}
```

| Field | Type | Description |
|-------|------|-------------|
| key | string | Unique recipe ID |
| `name` | string | Display name in the crafting UI |
| `description` | string | Short description |
| `output` | string | Blueprint ID of the produced item (must exist in `data/blueprints/`) |
| `build_time` | int | Worker-ticks to complete; divided by the crafter's `Int` stat |
| `cost` | object | Map of resource blueprint ID → quantity. Empty `{}` = free. |
| `station` | string | `StationID` of the workbench required to craft this recipe |

---

## Crafting Stations

A crafting station is any entity blueprint with a `CraftingStation` component:

```json
"gun_bench": {
    "Appearance": { ... },
    "Description": { "Name": "Gun Bench" },
    "CraftingStation": {
        "StationID": "gun_bench",
        "RequiresTech": "advanced_metallurgy"
    },
    "Inanimate": {},
    "Solid": {}
}
```

| Field | Description |
|-------|-------------|
| `StationID` | The string recipes match against via their `station` field |
| `RequiresTech` | Optional tech ID from `data/research.json`. The station is hidden from the build menu until that tech is in `settlement.KnownTechs`. |

A station is also a buildable structure — add an entry in `data/build.json` with `is_entity: true` and `type` set to the station's blueprint ID.

### Existing Stations

| StationID | Recipes |
|-----------|---------|
| `basic_workbench` | General-purpose (medkits, simple items) |
| `gun_bench` | Firearms (pistol, rifle, shotgun, plasma cutter, etc.) |
| `basic_armor_bench` | Light armor (enviro suit) |
| `advanced_armor_bench` | Heavy armor (gated behind `advanced_metallurgy`) |

---

## Adding a New Recipe

1. Ensure the output blueprint exists under `data/blueprints/items/` (or wherever appropriate).
2. Add an entry to `data/crafting_recipes.json` with a unique key.
3. Set `output` to the blueprint ID, `station` to an existing `StationID`, and `cost` to required resources (resource IDs must also be valid blueprint IDs in `data/blueprints/items/resources.json`).
4. Set `build_time` to a base tick count — actual time is `build_time / colonist.Int`.
5. The crafting menu picks the recipe up on next load.

---

## Adding a New Workbench

1. Add a new blueprint under `data/blueprints/structures/workbenches.json` with a `CraftingStation` component and a unique `StationID`.
2. (Optional) Set `RequiresTech` if the bench should be locked behind research.
3. Add an entry to `data/build.json` so the player can build it:
   ```json
   "ammo_press": {
       "name": "Ammo Press",
       "type": "ammo_press",
       "description": "Reloads spent casings.",
       "build_time": 5,
       "cost": { "metal_ore": 4 },
       "is_entity": true
   }
   ```
4. Add recipes that target the new `StationID` in `data/crafting_recipes.json`.

---

## Crafting Workflow Summary

1. Player opens the crafting UI on a placed station and queues a recipe.
2. A crafting task is appended to the settlement task queue, scoped to that station.
3. A colonist with sufficient `Int` claims the task, hauls required `cost` items from storage to the station, and works the task to completion.
4. On completion the `output` blueprint is instantiated into the station's inventory (or dropped on the tile if full).
