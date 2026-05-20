# Structure Scripts — Developer Guide

Structure scripts are `.basic` programs in `data/scripts/structures/` that procedurally stamp a self-contained structure (rooms, NPCs, loot) onto the world. They are invoked at runtime — when a location is first entered — so each structure is generated fresh inside the live level.

Structures can be generated two ways:

- **Quest fixture** — a quest template specifies `fixture_structure`; when the player lands at the bound location the fixture is stamped and the structure's NPC becomes the quest target.
- **Plain fixture** — a `Location` carries a `QuestFixture` with `Structure` set and `QuestID` empty; the structure is stamped but is purely flavor (no quest linkage).

---

## File Convention

Every structure script must live at:

```
data/scripts/structures/<name>.basic
```

and must define exactly one entry function:

```basic
function generate(x, y, w, h)
    # x, y = top-left corner of the allocated footprint
    # w, h = width and height in tiles
    ...
endfunction
```

The script is called by `gen_structure(name, x, y, w, h)` (see below). The dispatcher resolves the name to the path automatically; do **not** include a path or extension.

Scripts are source-cached; the file is read once per process lifetime and the parsed program is reused across all invocations.

---

## Builtins Available in Structure Scripts

Structure scripts share the same builtin set as scenario setup scripts, plus the following additions.

### `gen_structure(name, x, y, w, h)`

Invokes another structure script by name, stamping it into the given footprint. The child script inherits the parent's param frame (a copy — changes do not propagate back up). Recursion depth is capped at 8.

Use this to compose larger structures from smaller reusable pieces.

### `carve_room(x, y, z, w, h, wall_tile, floor_tile)`

Paints a rectangular room: floor inside, wall border. The floor and wall tiles must be existing tile definition IDs.

### `clear_middle(x, y, z)`

Removes the blocking middle layer at the given tile (e.g., a hull wall that was painted by `carve_room`). Use this to punch doorways or breaches after calling `carve_room`.

> **Why not `set_tile`?** `set_tile` (and `carve_room`) route tiles through `PaintTile`, which writes to a tile's declared layer. A hull wall sits in the Middle layer; painting a floor tile only updates the Floor slot and the wall remains. `clear_middle` bypasses layer routing and removes whatever is in the Middle slot directly.

### `spawn_entity(blueprint, x, y, z)`

Spawns an entity by blueprint ID at the given position.

### `spawn_entity_named(blueprint, x, y, z, name)`

Spawns an entity and sets its `DescriptionComponent.Name`. Use for unique characters — quest targets, named guards, boss creatures.

### `add_epithet(x, y, z)`

Decorates the name of the entity at `(x, y, z)` with a random epithet from `data/names.json` — e.g. `"Grosk"` → `"Grosk the Destroyer, mauler of cities"`. A title epithet is always added; a suffix epithet is added about half the time.

Use it to promote a generic NPC into a boss name without hardcoding text. Call it before `mark_quest_target` so the rewritten quest title adopts the full boss name. If the entity has no `DescriptionComponent`, one is added with an empty base name.

The epithet pools live under `"epithets": { "title": [...], "suffix": [...] }` in `names.json`; edit them there.

### `mark_quest_target(x, y, z)`

Finds the entity at `(x, y, z)`, attaches a `QuestTargetComponent` linking it to the active quest, and rewrites the quest's title to include the NPC's name.

- Reads the current quest ID from the `quest_id` param (set automatically before the script runs when the fixture is quest-backed).
- If `quest_id` is empty (flavor fixture, no quest), this is a no-op — the NPC is still spawned, it just isn't a bounty target.
- Must be called *after* the NPC is spawned, since it reads the entity at the given tile.

### `set_param(name, value)` / `get_param(name)`

Read and write named parameters in the current param frame. Parameters are the primary mechanism for passing configuration into a script without embedding magic constants.

The caller (quest fixture dispatch) pre-populates `quest_id` before invoking `generate`. Child scripts invoked via `gen_structure` receive a copy of the parent's frame and cannot write back to it.

---

## Quest-Backed Fixtures

### Template fields (`data/quest_templates.json`)

Add these fields to a quest template to make it generate a structure fixture instead of a simple entity fixture:

| Field | Type | Description |
|-------|------|-------------|
| `fixture_structure` | string | Name of the structure script (without path or `.basic`) |
| `fixture_struct_w` | int | Width of the allocated footprint in tiles |
| `fixture_struct_h` | int | Height in tiles |

When `fixture_structure` is set, the template must **not** include `target_blueprints` or `target_names`. The structure script is the sole authority on what spawns and who the target is.

Example — the bounty template:

```json
{
  "id": "bounty",
  "weight": 2,
  "spawn_archetype": "alien",
  "name": "Bounty",
  "desc": "A marked killer lairs on this world. Hunt it down and put it in the ground.",
  "trigger": "target_killed",
  "fixture_structure": "bunker",
  "fixture_struct_w": 11,
  "fixture_struct_h": 9,
  "reward_fuel_flat": 45,
  "reward_resources": { "crystal": 4 }
}
```

### Deferred quest naming

When a structure-backed bounty is first generated, the quest title is set to `"<template_name> — <location_name>"` (e.g., `"Bounty — Sigma Outpost"`). The NPC's identity is not known until the player lands and the structure script runs.

When `mark_quest_target` fires inside the script it calls `Campaign.BindQuestTargetName`, which rewrites the title to `"<template_name>: <npc_name> — <location_name>"` (e.g., `"Bounty: Grosk the Destroyer, mauler of cities — Sigma Outpost"`). This is intentional — the full name surfaces only after landing.

### `QuestFixture` struct fields

| Field | Type | Description |
|-------|------|-------------|
| `quest_id` | string | ID of the linked quest |
| `structure` | string | Structure script name; when set, `blueprint` is unused |
| `struct_w` | int | Footprint width (defaults to 9 if 0) |
| `struct_h` | int | Footprint height (defaults to 7 if 0) |
| `blueprint` | string | Entity blueprint (legacy path — not set for structure-backed fixtures) |
| `name` | string | Named override for entity fixtures (legacy) |
| `kind` | string | `"boss"`, `"datapad"`, etc. |
| `spawned` | bool | Set to `true` after the fixture has been stamped; prevents re-stamping |

---

## Example: `bunker.basic`

```basic
# bunker — a small walled hull room a quest fixture spawns inside.
# Entrypoint contract: generate(x, y, w, h) where (x,y) is the top-left
# corner of the footprint the dispatcher allocated.
function generate(x, y, w, h)
    cx = x + w / 2
    cy = y + h / 2
    sz = get_surface_z(cx, cy)
    if sz < 0 then
        return
    endif

    carve_room(x, y, sz, w, h, "bunker_wall", "bunker_floor")

    # Punch a 2-wide breach in a random wall.
    side = rnd_int(4)
    if side = 0 then
        bx = x + 1 + rnd_int(w - 3)
        clear_middle(bx, y, sz)
        clear_middle(bx + 1, y, sz)
    endif
    if side = 1 then
        bx = x + 1 + rnd_int(w - 3)
        clear_middle(bx, y + h - 1, sz)
        clear_middle(bx + 1, y + h - 1, sz)
    endif
    if side = 2 then
        by = y + 1 + rnd_int(h - 3)
        clear_middle(x, by, sz)
        clear_middle(x, by + 1, sz)
    endif
    if side = 3 then
        by = y + 1 + rnd_int(h - 3)
        clear_middle(x + w - 1, by, sz)
        clear_middle(x + w - 1, by + 1, sz)
    endif

    spawn_entity("storage_locker", x + 1, y + 1, sz)

    # The brute rolls its own mutant name from the blueprint (which uses
    # the "<mutant>" name placeholder). add_epithet promotes it to a boss
    # name before mark_quest_target picks it up. mark_quest_target no-ops
    # when there's no associated quest, so the warden exists either way.
    spawn_entity("mutant_brute", cx, cy, sz)
    add_epithet(cx, cy, sz)
    mark_quest_target(cx, cy, sz)

    spawn_entity_named("mutant_grunt", cx - 2, cy, sz, "Bunker Sentry")
    spawn_entity_named("mutant_grunt", cx + 2, cy, sz, "Bunker Sentry")
endfunction
```

---

## Adding a New Structure Script

1. Create `data/scripts/structures/<name>.basic`.
2. Define `function generate(x, y, w, h)`. Use `get_surface_z(cx, cy)` to find the surface Z and bail if it returns `< 0` (off-map or underground-only).
3. Use `carve_room` + `clear_middle` to carve the layout, then `spawn_entity` / `spawn_entity_named` to populate it.
4. If the structure can host a quest target, call `mark_quest_target(x, y, z)` immediately after spawning the target NPC. Use `add_epithet(x, y, z)` first (before `mark_quest_target`) if you want a boss name; pairs naturally with blueprints that already use a `<type>` name placeholder.
5. Reference the script name in a quest template (`fixture_structure`) or add a `QuestFixture{Structure: "<name>"}` directly to a `Location`.
