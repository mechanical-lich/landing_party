# Stamps — Developer Guide

The `stamp` feature kind runs an existing structure script at a location found during world generation. This lets maps and scenarios place procedurally generated structures — bunkers, ruins, wrecks — as part of the normal generation pipeline, without requiring a quest fixture.

Structure scripts live in `data/scripts/structures/`. See [Structure Scripts](structure_scripts.md) for the full authoring guide.

---

## Usage

Add a `stamp` entry to the `features` array of any map or scenario:

```json
"features": [
  {
    "kind": "stamp",
    "count": 1,
    "params": {
      "script": "bunker",
      "w": 11,
      "h": 9
    }
  }
]
```

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `script` | string | required | Name of the structure script (no path, no `.basic` extension) |
| `w` | int | `9` | Footprint width passed to `generate(x, y, w, h)` |
| `h` | int | `7` | Footprint height passed to `generate(x, y, w, h)` |

The `biome`, `in_region`, `count`, and `jitter` fields work the same as all other feature kinds.

---

## How Placement Works

1. The placer picks an anchor `(cx, cy)` using the standard feature center logic.
2. It reads the surface Z at that column.
3. The footprint top-left is set to `(cx - w/2, cy - h/2)`.
4. `generate(x, y, w, h)` is called on the named script — exactly as quest fixture dispatch calls it.

The script has full access to all structure script builtins: `carve_room`, `clear_middle`, `spawn_entity`, `spawn_entity_named`, `add_epithet`, `mark_quest_target`, `gen_structure`, etc.

---

## Example — crashed ship on a moon scenario

In `data/scenarios/grey_contact.json`:

```json
"features": [
  {
    "kind": "stamp",
    "count": 1,
    "biome": "regolith",
    "params": { "script": "crashed_ship", "w": 13, "h": 9 }
  }
]
```

Then create `data/scripts/structures/crashed_ship.basic` using the standard structure script contract.

---

## Relationship to Quest Fixtures

Quest fixtures and stamps both call the same script dispatcher (`RunStructureScript`). The difference is:

| | Quest fixture | Stamp |
|---|---|---|
| Triggered by | Player landing at a quest-bound location | World generation |
| `quest_id` param | Set automatically | Empty (not quest-backed) |
| `mark_quest_target` | Wires up the bounty target | No-op |
| Location | Chosen by the campaign generator | Chosen by the feature placer |

A script written for a quest fixture works as a stamp without modification — `mark_quest_target` just silently no-ops when there's no quest context.
