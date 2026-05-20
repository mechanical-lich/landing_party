# Tile Definitions — Developer Guide

Tile definitions live in `data/tiledefinitions/`. Every `*.json` file in that directory is loaded at startup and concatenated into one global catalog, so the schema can be split into logical files (hull, biomes, props, advanced, …) without code changes.

---

## Loader

```go
world.LoadTileDefinitionsDir("data/tiledefinitions")
```

- Reads every `*.json` in the directory in **alphabetical filename order** (`os.ReadDir` returns sorted entries). Subdirectories and non-JSON files are ignored.
- Each file is an array of `TileDefinition` objects (the same shape as the original `tile_definitions.json`).
- Definitions are appended in file order, then in array order within each file.
- A single-file form, `world.LoadTileDefinitions(path)`, exists for tests and one-off fixtures but isn't used in production.

A `TileDefinition` extends `rllayered.TileDefinition` with a `Space bool` flag. The other fields (`name`, `layer`, `solid`, `air`, `autoTile`, `mixId`, `variants`, `resource`, …) are inherited from rllayered; see `cmd/tile-editor/main.go` for the full set of editable fields.

```json
[
  {
    "name": "hull_floor",
    "layer": "floor",
    "resource": "scifi_world",
    "variants": [
      { "variant": 0, "spriteX": 0,  "spriteY": 0 },
      { "variant": 1, "spriteX": 24, "spriteY": 0 }
    ]
  }
]
```

---

## The `empty` Sentinel Invariant

The runtime engine hardcodes a single rule: **the tile at index 0 is the empty sentinel.** `rllayered.Slot.IsEmpty()` is literally `return s.Type == 0`. Freshly-allocated tiles have `Type == 0` and the engine relies on that to mean "nothing painted yet" (skipped by the renderer, ignored by collision, etc.).

That invariant is enforced by the loader, not by filename ordering:

1. After concatenating all files, the loader hoists the entry named `empty` to index 0 (`pinEmptySentinel`) regardless of which file it sits in. The order of every other tile is preserved.
2. If no `empty` entry exists across any file, the loader errors out with `tile definitions missing required "empty" sentinel`.
3. Duplicate `name` across files also errors out (`duplicate tile definition name "x"`), so a copy-paste mistake during a split can't silently mask a tile.

You can put `empty` in any file (it currently lives wherever the base catalog is split) and rename/reorder files freely — index 0 stays correct.

The constant `world.EmptyTileName` (`"empty"`) is the single source of truth for this name.

---

## Splitting the Catalog

The directory loader exists so the catalog can be organized by theme. Current shape (roughly):

| File | Contents |
|------|----------|
| `tile_definitions.json` | Base catalog: `empty`, `space`, `air`, rock/ore/dirt, etc. |
| `hull.json` | Ship-interior tiles (`hull_floor`, `hull_wall`). |
| `bunker.json` | Bunker tiles (`bunker_floor`, `bunker_wall`). |
| `world.json`, `advanced.json` | Biome tiles, exterior props, advanced fixtures. |

Splitting rules:

- Pick whatever filenames make organizational sense. Alphabetical order determines the index assignment of any new tiles, but nothing else cares about it — the `empty` pin makes the engine indifferent, and saves are name-keyed (see below).
- A tile definition belongs to **one** file. Cross-file duplicate names fail to load.
- The tile editor (`cmd/tile-editor`) reads and writes each file in place: a def edited in the editor is saved back to the file it came from, so your split stays split.

---

## Save Portability

Saves persist tile data as RLE runs of `(type, variant, count)` triples, where `type` is the integer index. Indices depend on load order, so historically any catalog reshuffle would silently corrupt old saves.

To make that durable, every save written today carries a `TileCatalog []string` field — a snapshot of `TileIndexToName` at save time:

- **On save**: a copy of `TileIndexToName` is embedded in `SaveData`.
- **On load**: `buildTileRemap(savedCatalog)` produces an old-index → current-index translation by name. `decodeLayeredTiles` applies it once per run as it decodes the floor/middle/ceiling streams.
- **Names removed from the catalog** map to `0` (the empty sentinel) and emit a `log.Printf` warning; the saved tile becomes empty space.
- **Names that still exist but moved to a different index** are remapped transparently. This is the common case after a split or a new tile insertion.
- **Saves with no `TileCatalog`** (pre-feature saves) fall back to raw indices — same behavior as before, no regression, but they're vulnerable to reshuffles.

In practice: once a save round-trips through a build that has this code, that save is robust against any later catalog reorder, addition, or removal. Renames are the only thing that still bite — the name is the durable key. Add an alias map there if it becomes a concern.

---

## Adding a New Tile

1. Open `cmd/tile-editor` (`go run ./cmd/tile-editor`) — it reads every file in `data/tiledefinitions/` and lets you edit variants visually. Or edit the JSON directly.
2. Pick the right file (or create a new one) based on theme. Filename governs alphabetical placement; in practice this only matters for the index a new tile lands on, which the save remap makes irrelevant after one save cycle.
3. Set `name` (unique across the catalog), `layer` (`"floor"` | `"middle"` | `"ceiling"`; default middle), `resource`, and `variants` with sprite coordinates.
4. Set the engine-aware flags as needed: `solid`, `air`, `space`, `water`, `door`, `stairsUp` / `stairsDown`, `autoTile` (autotile mask kind), `mixId` (autotile fusion group), `movementCost`.
5. Restart the game. Reference the tile by name from anywhere — scripts (`carve_room(..., "your_tile", ...)`), generation, or `data/build.json` (`"type": "your_tile"`).

---

## Editor Quirks

- The tile editor doesn't currently support **creating** or **deleting** tile defs — only editing variants of existing ones. To add a brand-new tile, edit JSON directly, then re-open the editor.
- Editor save format is `json.MarshalIndent` with 4-space indent — matches handwritten files, so diffs stay clean.
- Editor load order is alphabetical (same as runtime), so the tile list shown in the editor matches the indices the game will assign.
