# Content Variety From the Art Already in the Repo: Implementation Plan

Planned 2026-11-14 against `dev` at 999140e. Not built. A plan only: no
game code or data was changed to write it. Every name and line of
player-facing text here is a draft for the owner to rewrite. Where this
plan says a sprite "is" something (a wolf, a vent, a monolith wall), that
is a reading of the sheet, and the owner decides what it is in the game.

## Decision

The owner plans a galaxy of 40, 80 or 150 systems made in one pass
(`pregenerated_galaxy_plan.md`). Today's content (11 biomes, 8 playable
world types, 9 archetypes, 20 scenarios, one harmless animal) repeats
quickly at that size. The owner asked how much variety can be added using
only art that is already in the repo, and for a plan to add it.

Constraints: no new art is assumed (gaps are listed for the owner);
scenarios and maps stay rolled; no CI test job; the owner commits; tests,
soaks and vmtests run locally.

## Headline

What this plan recommends, all from existing sprites:

| Kind | Today | Added | Look only | Plays differently |
|---|---|---|---|---|
| Biomes | 11 | **13** | 8 | 5 (slow ground, liquid barriers, surface radiation, ice underground, vacuum ice) |
| World and site types (maps) | 8 | **13** | 6 | 5 by data alone, 2 by a 50-line code pass |
| Existing maps given more biomes | | **5** | 5 | |
| Ruin and dungeon styles (wall and floor pairs) | 9 | **13** | 13 | |
| Structures (stamps) | about 50 scripts | **15** | | 7 history ruins, 2 for the elders and the gate, 4 natural landmarks, 2 station modules |
| Wildlife | 1 harmless, worms, yetis | **30** | | 10 harmless, 17 that hunt or guard, 3 stationary vents |
| Archetypes | 9 | **10** (19 in all) | | |

Two world types that would need new rules (a true cavern world with an
underground landing; cold and heat) are described and **not** recommended
now.

Code is small and optional in the order given: about 60 lines to draw
ground edges, 100 for ruin styles, 50 to let a new map inherit another's
chapter weights and roll a subset of its biomes, 50 for station palettes,
120 for galaxy distribution rules. Everything else is JSON and BASIC.

### What the survey corrected in the first estimate

The first survey was by eye. Measured against the data and the sheets:

1. **Ground colour sets: four unused, not nine.** The fantasy sheet has
   ten ground sets. Six are already the game's ground (dark green is
   `grass`, dark brown `dirt_floor`, white `ice_floor`, both peach rows
   `sand`, orange clay `alien_ground`). Only bright green, straw yellow,
   khaki and dark olive are unused as fills. What *is* unused on all ten
   is the edge tiles. Two more ground sets with edges, missed the first
   time, are on the sci-fi sheet (blue-grey and dark grey, rows 35 to 38).
2. **The edge tiles fit an autotile mode that already exists**, but are
   not usable without a small draw change (see "The two unknowns").
3. **Station palettes are not unused.** The green and teal wall and floor
   sets are `bunker_wall`/`bunker_floor` and `advanced_wall`/
   `advanced_floor`. What is unused is the green and teal furniture,
   consoles, doors and rubble. A station in another palette is a code
   question (the station primer writes `hull_wall` by name), not an art
   one.
4. **Unused wall sets: about twenty, not seven.** Of the "seven", teal and
   blue are in use (`ancient_temple_wall`, `eldritch_wall`). Sixteen
   fantasy rows and four sci-fi rows are unused.
5. **The single trees are all in use.** Autumn tree, bare bush, pine and
   snowy pine are `orange_tree`, `alien_flora`, `pointed_tree` and
   `frosted_pointed_tree`. Unused flora is smaller: a cut stump, two
   cacti, two rock spires, a mound, a slab, two ferns, flowers, bushes,
   lily pads, leaf litter. The forest art is 3×3 canopy blocks, which the
   scatter placer cannot place (one cell per entity).
6. **There is a second creature sheet nobody listed.**
   `assets/entities/oryx_16bit_scifi_creatures_extra.png` (registered as
   `scifi_entities_extra`, no cell used) is the fantasy sheet's animals
   with the knights and wizards already removed: 60 creatures, about 45 of
   which fit. The estimate of thirty or more wildlife holds.
7. **There is no temperature in the game.** "Temperature" exists only as
   the climate coordinate that places biomes on a map. Frozen and scorched
   worlds differ today by tiles, features and scenario text. This plan
   does not add a temperature rule.

## Where the code is today

### How a world is made

1. **Archetype** (`data/location_templates.json`, `weightedArchetype` in
   `internal/campaign/generate.go:964`): a weight, a `kind` (`planet`,
   `moon`, `asteroid_field`, `station`: the Star Map draws by kind,
   `starmap_panel.go:1074`), `start_ok`, tags, a summary and a list of
   **maps**.
2. **Map** (`data/maps/*.json`, `internal/mapdef`): size ranges, a primer
   (`planet`, `asteroid_field`, `station`, `void`) with params, a
   `biome_map` (noise over temperature and humidity, or one biome), a list
   of features, a `present` block (what history must explain: life,
   status, arcs, tier weights, density) and a `base_scenario`.
3. **History** binds the site: an arc ends in a tier (threat, hazard,
   quiet), then a chapter of that tier **by per-map weight**. All 38
   chapters in `data/history/events.json` carry a `weight` object keyed by
   map id. A map with no weights can only ever be quiet.
4. **Scenario** (`data/scenarios/*.json`): spawn rules keyed by blueprint,
   each with a list of **ground tile names** it may spawn on; lighting
   mode; a setup script; extra features.
5. **Generation** (`internal/generation`): the primer paints defaults, the
   biome's `rules` paint each column (surface, subsurface, underground,
   cavern, atmosphere), then map, scenario and biome features run
   (`ore_vein`, `radiation_pocket`, `lava_lake` with any `tile`,
   `scatter_entity`, `scatter_tile`, `cave_spawner`, `stamp` and so on).
6. **Tiles** (`data/tiledefinitions/*.json`): a name, a layer (floor or
   middle), flags (`solid`, `water`, `air`, `space`), an autotile mode,
   variants with sprite coordinates, and per-faction `speed` (used by hive
   resin: `internal/game/resin_speed.go`).

### Autotile modes

From `ml-rogue-lib` `rllayered` and `internal/world/drawing.go`:

| Mode | Slot | Variants | Used by |
|---|---|---|---|
| 0 none | any | picked by position hash | floors, deposits |
| 1 wall | middle | 2 (has a neighbour below or not) | nothing |
| 2 bitmask | middle | 16 by N, S, W, E, plus an optional 17th for "all eight neighbours match" | every Oryx wall set, wires |
| 2 on a **floor** def | floor | 16 by N, S, W, E, same tile only (`resolveFloorAutotileVariant`, `drawing.go:327`) | **nothing** |
| 3 blob47 | middle | 47, with corners | `rock`, `dirt`, `ice` (the owner's sheets) |

`mixId` makes different middle tiles count as neighbours (rock, dirt and
ice share 1). It applies to middle tiles only.

### What each addition costs

| Addition | Data | Code | Notes |
|---|---|---|---|
| A floor tile | about 15 lines | none | The minimap colours by flag, so no colour entry |
| A 16-variant wall | about 90 lines | none | Same column layout on every Oryx row, so a row is a copy with one `spriteY` changed |
| A biome | a 20-line file, plus its id in a map's list | none | Its climate centroid must differ from the map's others (`TestEveryMapBiomeAppears`) |
| A biome on an existing map | one id | none | The cheapest addition there is |
| A world type (map) | a 45-line file; a base scenario (30 lines) and setup script (40 to 80) unless one is shared; **up to 38 chapter weights** | none, or 20 lines once | See `weights_as` below |
| An archetype | 8 lines | none while it keeps an existing `kind` | A new `kind` needs a Star Map marker |
| A ruin style | wall and floor tiles | today, an edit in every script | Each structure script picks its wall by tech in its own copy of `wall_of`/`floor_of`, or hard-codes it (`castle`, `circle_tower`, `underground_dungeon`) |
| A harmless animal | a 45-line blueprint, a spawn rule | none | `FactionAI` `wander` |
| A hunting animal | the same, with an AI script | none | Counts as hostile, so it may not appear on a quiet start (`TestContentQuietBasesSpawnNothingHostile` treats any `ScriptedAI` as hostile) |
| Per-biome wildlife | spawn rules list ground tiles | none | Works only where each biome has **its own ground tile name**, which is why this plan gives new biomes their own tile definitions |

### The two unknowns, settled

**Can the fantasy ground sets be used with their edges?** Measured on the
sheet (edge opacity of every cell in rows 15 and 16):

- Each set is two rows by 18 columns (cols 28 to 45) plus slivers. The
  second row holds **all sixteen N/S/W/E cases** as ground on a
  transparent rim: island (28), east cap (29), east-west run (30), west
  cap (31), a north-south run (32 to 34), four corners (35 to 38), four
  edges (40 to 43). The first row holds the fill (28, 29) and inner-corner
  pieces.
- So the set **resolves under the existing floor autotile** (mode 2 on a
  floor definition). No new mode is needed.
- It is still not usable as is, for one reason: **nothing is drawn under a
  floor.** The rim is transparent, and at a border both neighbours show
  their rim, so the border would be a black gap. The fix is small: a
  ground tile names an `underlay` tile that is drawn first (`dirt_floor`
  under grass, `rock_floor` under sand). About 60 lines in
  `internal/world/drawing.go` and the game's own `TileDefinition`
  (`internal/world/tileDefinitions.go`); the library is not touched.
- Two smaller costs: an autotiled floor always uses the fill for its
  interior, so the two fill variants need the 17th "interior" slot or a
  hash; and the 16-case mode ignores diagonals, so a ground touching
  another only at a corner shows a rounded notch, which is how the art is
  meant to look.
- **Verdict: flat floor today with no code; edged ground after one
  60-line step.** Every biome below works flat. The edges are polish that
  improves every existing biome border too, and are their own step.

**What does a new world do for underground walls?** The biome's
`subsurface` and `underground` rules name the tile, so it is data. Three
choices:

- **The owner's blob sheets** (`rock`, `dirt`, `ice`): free. `ice` is only
  used for surface lakes today, so an ice underground is a new look at no
  cost.
- **An Oryx 16-variant natural wall**, the way `rock_old` uses row 15:
  orange cave rock (row 13), sand, grey and red boulder walls (rows 20 to
  22), snow wall (row 18). No code. They have no inner corners, so they
  read blockier than the blob sheets, and they need `mixId` 1 to join
  cleanly where they meet `rock`.
- **A new blob sheet** is new art (see "Art gaps": a recolour of
  `rock.png` is the single most useful missing piece).

Mining does not look at the wall's name, only at deposit names
(`world.DepositDrop`), so a new wall tile digs like rock. This was read
in the code, not tested.

## The art available

Cells are 24 px; coordinates below are (column, row), so pixels are
24 times each. "Used" means referenced by any file under `data/` (tile
variants and blueprint `Appearance`; `EquipmentAppearance` blocks checked
separately). Sets were read at 2× and 4× with the used cells outlined.
Floor swatch cells marked * were read by eye and should be confirmed in
the tile editor before use.

### `fantasy_world`: `assets/tilesets/oryx_16bit_fantasy_world.png` (1319×965, 54×40 cells, 154 used)

**Wall sets, rows 0 to 22.** Every row has the same layout: wall blocks
at cols 0 to 2 (plain, cracked, broken), floors at cols 3 to 6, stairs at
7 and 8, the sixteen autotile pieces in cols 9 to 24, cracked variants at
25 and 26. In use: rows 5 (`temple_wall`), 6 (`ancient_temple_wall`), 7
(`eldritch_wall`), 10 (`dungeon_wall`), 11 (`stone_wall`), 12
(`overgrown_stone_wall`), 15 (`rock_old`), and cell (0,2) as `bedrock`.

| Row | Looks like | Proposed use |
|---|---|---|
| 0 | pale grey dressed stone | ruin style (iron age) |
| 1 | grey stone, nearly row 0 again, with checker floors | its floors as a variant for row 0; not a style of its own |
| 2 | khaki stone | ruin style (stone age or desert peoples) |
| 3 | red studded blocks | ruin style (iron age, warlike) |
| 4 | dark carved timber | ruin style (stone and iron age halls) |
| 8 | bright yellow | none recommended; it reads as gold or a placeholder |
| 9 | grey grated metal (its floors are `temple_floor`) | ruin style (industrial) |
| 13 | orange cave rock | natural wall: desert and dead-world underground |
| 14 | hedge | garden walls on a garden world; dressing |
| 16 | iron fence (cols 9 to 26 only) | graveyard and compound fencing; see-through |
| 17 | stone under snow | ruin style on cold worlds |
| 18 | packed snow or ice | natural wall on ice worlds |
| 19 | sandstone blocks | ruin style (desert peoples) |
| 20 | sand-coloured boulders | drystone ruin style; natural wall |
| 21 | grey boulders | drystone ruin style; natural wall |
| 22 | red boulders | drystone ruin style; natural wall on a red world |

Rows 20 to 22 are unresolved. Composed in the standard order for the
sample images (2026-11-14) they looked jumbled in a mass. At 3× the
columns look like the standard ones, so the likelier cause is that these
sets have gaps and no solid interior piece: fine for a one-tile-thick
ruin wall, wrong for a mountain. Check in the tile editor before use.
Rows 14 (hedge) and 16 (fence) look standard at 3×; row 13 composed
correctly.

**Ground sets, cols 28 to 52, two rows each.**

| Rows | Colour | Fill cells | Today |
|---|---|---|---|
| 15, 16 | bright green | (28,15) (29,15) | **unused** |
| 17, 18 | straw yellow | (28,17) (29,17) | **unused** |
| 19, 20 | khaki | (28,19) (29,19) | **unused** |
| 21, 22 | orange clay | (28,21) (29,21) | `alien_ground` |
| 23, 24 | dark olive | (28,23) (29,23) | **unused** |
| 25, 26 | dark brown | (28,25) (29,25) | `dirt_floor` |
| 27, 28 | white | (28,27) (29,27) | `ice_floor` |
| 29, 30 | peach, streaked | (28,29) (29,29) | `sand` |
| 31, 32 | peach, plain | (28,31) (29,31) | `sand` variant |
| 33, 34 | dark green | (28,33) (29,33) | `grass` |

Edge pieces of all ten are unused.

**Flat swatches, rows 12 and 13, cols 27 to 41*.** Bright green, tan,
olive, a grass-blade texture, wood grain, white, peach, layered brown
cliff, red-brown cobble, green and yellow ground with dirt patches (three
variants each), dark foliage, moss, grey gravel, red leaf litter. Useful
as second variants and as forest-floor and gravel grounds.

**Liquids, rows 27 to 37, cols 4 to 26.** Four colours, each with a fill
of two cells and a border set: blue fill (4,30) (4,31) is `water`; green
(7,30) (7,31), red (10,30) (10,31) and brown (13,30) (13,31) are
**unused**. Large round pool decals in the same four colours sit at cols
22 to 26. Small one-cell pools: blue and red at (51..54,1), green and
brown at (51..54,2).

**Flora and props, cols 43 to 54, rows 0 to 12.**

| Cells | What | Today |
|---|---|---|
| (43,3) (44,3) (45,3) (46,3) (48,3) (49,3) (49,2) (45,2) | round, fruit and autumn trees, bare bush, pine, snowy pine, cactus, grey spire | used |
| (47,3) | cut tree stump (pale rings) | unused |
| (48,2) (50,2) | small and tall cactus | unused |
| (44,2) (46,2) | brown spire, second grey spire | unused |
| (47,1) (48,1) (47,2) | brown mound, grey slab, pebbles | unused |
| (49,1) (50,1) | ferns or star plants | unused |
| (43..45,1) | white flowers | unused |
| (43..47,0) (48..52,0) (53,0) (54,0) | green bushes, autumn bushes, lily pads | unused |
| (50..53,3) | green and orange leaf litter | unused |
| (43,2) | cave mouth | unused |
| (43..54, 4..7) and (43..54, 9..12) | 3×3 and 2×1 canopy blocks: green, autumn, bare, pine, snowy pine, grey and brown spire ranges | unused; needs multi-cell placing |

**Fantasy furniture, cols 28 to 41, rows 0 to 9** (read at 4×; one cell
used, a torch at (40,1)). Fits stone and iron age ruins; nothing else.

| Cells | What |
|---|---|
| (28,0) (29,0) (30,0) | gravestone, cracked gravestone, rubble |
| (31..37,0) | scattered bones, bone piles, skull and bones |
| (38,0) (39,0) | campfire, two frames |
| (28..33,1) | cobwebs |
| (28..34,2) (35,2) (36,2) (39,2) (40,2) | wooden doors, stone doors, barred doors |
| (28,3) (29,3) | portcullis, shut and open |
| (35..37,3) | coffins |
| (38,3) (39,3) (40,3) (41,3) | barrel, broken barrel, buckets |
| (28..31,4) (32..34,4) (35,4) | cabinets and shelves, tables and a chair, a throne |
| (40,4) (41,4) | crates |
| (32..35,5) | statues |
| (36..41,5) (36..41,6) (36..41,7) | urns in grey, blue, green, red and orange, each whole, cracked and broken |
| (28,6) (33,6) (34,6) (35,6) | floor spikes, a round pit, floor grates |
| (28,7) (29..35,7) | fountain, pedestals with and without figures |
| (28..31,8) | weapon racks |
| (32,8) (33,8) | well, well with a roof |
| (34,8) (35,8) | grain heap, gravel heap |
| (40,8) (40,9) (41,8) (41,9) | skulls, a horned skull, a ribcage |
| (28..35,9) | long tables, three cells each |

**Paving and boards, rows 23 to 26.** Six paving colours, three cells by
two rows each: olive (11..13, 23..24), grey (14..16), sand (17..19), brown
(11..13, 25..26), teal (14..16), mossed dark (17..19). Plank floors at
(20..22, 23..26) and parquet at (24..26, 23..26). All unused.

### `scifi_world`: `assets/tilesets/oryx_16bit_scifi_world.png` (840×984, 35×41 cells, 140 used)

- **Rows 0 to 17: three palettes of ship and station art** (grey 0 to 5,
  green 6 to 11, teal 12 to 17). Walls and floors of all three are used.
  Unused in every palette: wall panels, vents, pipes; in green and teal
  also the consoles, lockers, doors, beds and the rubble row (rows 9 and
  15, cols 18 to 32).
- **Rows 18 to 20: gantry frames** in orange, cyan, olive and grey.
  Unused.
- **Row 23:** fire (10, 11), gears, crystals blue (14, used), red (15),
  pink (16, used as the egg launcher), green (17). Two unused crystal
  colours.
- **Row 24: gas clouds**, four frames each: red (6..9), green (10..13),
  purple (14..17), grey (18..21), orange (22..25). Unused.
- **Rows 26, 27: purple and green organic** floors (cols 0, 2, 3; col 1
  is the ore and crystal node) and a wall set each (cols 5 to 27).
  Walls unused. **The pieces are not in the hull's 16-piece order**
  (23 cells, with two-cell corner pieces); they need mapping by hand,
  and it is not confirmed that all sixteen cases are there.
- **Rows 28, 29: a dark and a steel-blue smooth wall set** (cols 5 to
  22), unlike anything in use: seamless panels, the nearest thing in the
  repo to "built by someone else". Unused.
- **Rows 28 to 31, cols 23 to 28:** rock spire ranges in blue-grey and
  grey, 3×3 with pairs. Unused, multi-cell.
- **Row 31, cols 0 to 3:** red liquid beside the orange `lava` of row 30.
  Unused.
- **Rows 35 to 38:** two ground sets with edges in blue-grey and dark
  grey, same layout as the fantasy ones. Unused; they match `regolith`
  and `crater_ground`.

### Creatures

**`scifi_entities_extra`** (360×192, 15×8, **nothing used**). Frames are
stacked (`BounceAxis` `y`). Rows 0, 2, 4, 6 hold:

| Row | Cols 0 to 14 |
|---|---|
| 0 | white yeti, brown yeti, pale worm, orange worm, brown bear, black bear, white bear, red scorpion, tan scorpion, six flames (orange, blue, green, pink, dark red, grey) |
| 2 | bulb creature, root creature, four droplets (red, blue, grey, olive), eye, eye cluster, three humanoid blobs, four cube slimes |
| 4 | blue gargoyle, red demon, olive demon, four golems (dark, clay, flesh, red-black), skeleton, treant, ice spectre, stone spectre, three birds (white, teal, black), a chicken |
| 6 | purple slime, green slime, dark bat, red bat, floating eye, red spider, black spider, grey rat, brown rat, snake, black beetle, pink beetle, grey wolf, brown wolf, black wolf |

**`fantasy_entities`** (480×648) holds the same animals among 200
humanoids; 3 cells used (the critter and both worms). Prefer the extra
sheet.

**`scifi_entities`** (768×984; frames side by side). Unused creature
cells: red and purple xeno bodies (cols 6 to 17, row 29), orange and
green troid bodies (cols 6 to 17, row 30), an orange lizard in two poses
(18..21, 30), a grey lizard (22, 30), eggs in pink, orange, green and
cream, slimes in red, blue, grey and yellow (rows 31, 32), blue and red
"hologram" humanoids (24..31, rows 29 and 30), most of the rust drones
(row 40). Six raider palettes (rows 21 to 26) are unused but belong to a
factions plan, not this one.

**Tint.** `Appearance` has R, G, B, so one sprite gives several animals
(a frost-tinted wolf) with no art. Tiles have no tint.

### Others

- `scifi_vehicles` (`assets/entities/oryx_16bit_scifi_vehicles.png`,
  576×384, **nothing used**): tanks (rows 0 and 1, six colours, four
  facings, two cells each), trucks (rows 2 and 3, the same colours and
  facings), and four small aircraft shapes (rows 4 to 15, six colours).
  As wrecks and parked machines they give the industrial, atomic and
  star ages something of their own to leave behind.
- `fantasy_items` (16 px cells, nothing used): feathers, fangs, eyes,
  hearts, mushrooms, gems, jelly pieces. Usable as creature drops. No
  meat or hide.
- `scifi_world_extra` (nothing used): twelve large planets, six small
  ones, stars, nebula clouds, asteroids. Not a level asset; see open
  question 9.
- `assets/tilesets/world.png` (the owner's own, not referenced by data):
  left alone.

## Tone

Fits a science-fiction colony game as wildlife or hazard:

- **Plainly fits:** wolves, bears, rats, snakes, scorpions, spiders,
  beetles, bats, birds, worms, slimes, cube slimes, droplets, the bulb and
  root creatures, the floating eye and eye cluster, lizards, eggs, the
  flames read as vents or wisps, gas clouds.
- **Fits with a reading:** golems as dormant constructs left by a people
  (dark and clay only); the treant as walking flora on an alien world;
  the chicken as feral livestock on a fallen colony world; the hologram
  humanoids as recordings in a ruin.
- **Does not fit:** every knight, wizard, elf, dwarf, goblin, orc and
  troll; demons, the gargoyle, the imp, the fairy; skeletons, mummies,
  ghosts and vampires (the game's undead are its own zombies); the mimic;
  the flesh golem; the wyverns (they read as dragons whatever they are
  called).

Props: cacti, spires, bushes, ferns, flowers, stumps and slabs fit
anywhere. Graves, pots, barrels, shelves and statues fit stone and iron
age ruins only. Floor glyphs, cauldrons and crystal balls do not fit.

## Design

### Three small mechanisms

Most of the list below is data. Three things are worth building once
because they make the rest cheap.

1. **`underlay` on a ground tile** (60 lines): the edged ground described
   above.
2. **`weights_as` on a map's `present` block** (20 lines in
   `internal/history`): "use the chapter weights of this other map". A
   look-only world then costs one file and no edits to `events.json`, and
   plays every threat and hazard its model plays. A map that should
   differ keeps its own weights. With it, **`biome_map.pick`** (25 lines
   in `internal/generation/biome_map.go`): a map lists six candidate
   biomes and each site rolls four from its seed, so two worlds of one
   type differ. At 150 systems this matters more than any single new
   biome.
3. **Ruin style as a stamp param** (100 lines; see "Ruin and dungeon
   styles").

### Biomes

Each gets its own ground tile name even where a sprite is shared, so
spawn rules can tell biomes apart. "T, H" is the climate centroid.

**First, on existing maps (no new map, no code):**

| # | Biome | Ground and art | Features | Goes on | Differs by | Cost |
|---|---|---|---|---|---|---|
| B1 | `glacier` | `ice_floor`; subsurface and underground `ice` (the owner's blob sheet) | snowy pines, more `ice` lakes, crystal veins | `frozen_planet` (cold, wet) | **ice underground**: a different mine, crystal-rich, ore-poor | 20 lines |
| B2 | `badlands` | khaki fill (28,19); underground orange cave rock (row 13) | brown and grey spires (44,2) (46,2), cacti (48,2) (50,2), pebbles | `desert_planet`, `earth_like` (hot, dry) | spire fields block paths; ore near the surface, no biomass | 2 tiles, 4 props, 25 lines |
| B3 | `savanna` | straw fill (28,17) | sparse `orange_tree`, boulders, leaf litter | `earth_like` (warm, middling) | open ground, little wood | 1 tile, 20 lines |
| B4 | `meadow` | bright green fill (28,15) | white flowers, green bushes, a few `round_tree` | `earth_like` (mild, wet) | no trees to clear, little biomass; the easy start | 1 tile, 3 props, 20 lines |
| B5 | `dust_mare` | dark grey sci-fi ground (0,37) (1,37); atmosphere `space` | grey spires | `moon` | flat, low radiation, no ore: the safe place to build on a moon | 1 tile, 20 lines |
| B6 | `fern_thicket` | dark foliage swatch (28,13); ferns (49,1) (50,1), green bushes (43..46,0) | dense ferns and bushes | `alien` | biomass without trees; home of the bulb and droplet animals | 1 tile, 3 props, 25 lines |

**Then, for the new worlds:**

| # | Biome | Ground and art | Differs by | Cost |
|---|---|---|---|---|
| B7 | `marsh` | `grass` sprite under its own name; green pools by `lava_lake` with a green liquid tile (7,30); lily pads, ferns | water-flag pools break the ground into paths | 2 tiles, 25 lines |
| B8 | `mudflat` | brown liquid sprite (13,30) as a **walkable floor** with `speed` `{"*": 1.5}` | **slow ground** for everyone, by the rule hive resin already uses | 1 tile, 20 lines |
| B9 | `ash_waste` | dark olive fill (28,23); bare bushes, cut stumps (47,3), `lava` pools; surface `radiation` 30 | low constant radiation, dense ruins (the map's `density`) | 1 tile, 25 lines |
| B10 | `scorched_crust` | `rock_floor` sprite under its own name; surface `radiation` 120; red liquid (0,31) pools | **the surface is not survivable for long; caves are** (existing radiation system) | 2 tiles, 25 lines |
| B11 | `living_crust` | purple organic floor (0,26) (2,26) (3,26); mountains and underground in the purple organic wall (row 26) | the strongest new look in the repo; pairs with the hive arcs | 2 tiles (one a 16-variant wall), 25 lines |
| B12 | `sludge_sea` | green liquid as the surface, like `shallow_sea` | a recoloured ocean | 20 lines |
| B13 | `frozen_regolith` | `ice_floor` sprite; atmosphere `space`; underground `ice` | **vacuum and ice together**: suits needed, crystal underground | 20 lines |

Ranked for the owner, cheapest and most distinct first: B1 glacier (art
all in use already), B2 badlands, B11 living crust, B10 scorched crust,
B3 savanna, B4 meadow, B8 mudflat, B7 marsh, B9 ash waste, B5 dust mare,
B13 frozen regolith, B6 fern thicket, B12 sludge sea.

What the lore system can place: every biome on a solid-ground map takes
the history stamps as today. `sludge_sea` and `marsh` pools are skipped
by the stamp scorer like water.

### World and site types

**A new look on existing rules** (one map file each, an archetype, and
`weights_as`; base scenario shared with the model unless noted):

| # | Map | Kind | Biomes (candidates; `pick` in brackets) | Model for weights | Start |
|---|---|---|---|---|---|
| W1 | `garden_world` | planet | meadow, forest, marsh, savanna, plains (4) | `earth_like` | yes |
| W2 | `dead_world` | planet | ash_waste, badlands, desert (2); life none; `density` 80, arcs of collapse | `desert_planet` | no |
| W3 | `living_world` | planet | living_crust, alien_forest, fern_thicket (2) | `alien` | no |
| W4 | `toxic_ocean` | planet | volcanic_island, sludge_sea | `ocean_planet` | no |
| W5 | `ice_moon` | moon | frozen_regolith, crater (2) | `moon` | no |
| W6 | `comet_field` | asteroid_field | a `comet_ice` biome (asteroid primer, `ice` for rock) | `asteroid` | no |

Honest ranking: W3 and W1 look most unlike what exists; W5 and W6 change
what is mined; W2 changes how much history is on the ground; W4 is a
recolour and the least worth having.

**Play differently, by data alone:**

| # | Map | What changes | Needs |
|---|---|---|---|
| W7 | `mire_world` (marsh, mudflat, forest) | slow ground across much of the map; pools cut routes | a base scenario with marsh animals |
| W8 | `irradiated_world` (scorched_crust, badlands) | surface radiation everywhere; the colony must dig in or leave quickly; deep ore | a base scenario whose text warns of it; tuning of the radiation number against the suit; not near a landfall |
| W9 | `rogue_world` (glacier, tundra) | **no sun**: the base scenario's lighting is `fixed` and dim, so work lights and sight range matter all day | a base scenario and setup script (lighting belongs to the scenario) |
| W10 | `outpost_station` | a small station: 60×50, 3 decks, two or three modules | size and `station_purposes` only |
| W11 | `hub_station` | a large one: 180×140, 6 decks, a larger lore budget; at most two per galaxy | the same; check the station primer's frame at that size |

**Need a small code pass:**

| # | Map | What it needs |
|---|---|---|
| W12 | `military_station` (green) | a `retile` map on the map definition, applied after the primer and stamps: `hull_wall` to `bunker_wall`, `hull_floor` to `bunker_floor`. About 50 lines. The station primer, its module scripts and `hull_breach` all write `hull_*` by name; rewriting once at the end avoids touching them. Add rubble tiles from rows 9 and 15 |
| W13 | `research_station` (teal) | the same pass with `advanced_*` |

**Would need new rules, not recommended now:**

- **A cavern world with an underground landing.** The art supports it
  (natural wall sets, bats, the cave mouth), but the landing reserve, the
  ship, the sun and the stamp placer all assume a surface. W8 gives most
  of the feeling (a surface you cannot stay on) with no new rule. A true
  cavern start is a plan of its own, 300 lines or more.
- **Cold and heat.** No system exists. Frozen and scorched worlds would
  gain the most from one, and it is a needs-system design, not content.
- **Liquids that hurt.** A caustic sea or lava that damages on contact
  needs a small damage-on-tile rule (about 60 lines). Without it the new
  liquids are barriers, like water.

### Ruin and dungeon styles

Today a people's walls follow its tech level and nothing else: stone age
is always `overgrown_stone_wall`, iron age always `stone_wall`, so every
castle in the galaxy is the same castle.

**The mechanism.** `data/history/tech.json` gains, per level, a list of
styles (`{"wall": ..., "floor": ..., "weight": ...}`). `stampFor`
(`internal/history/render.go:533`) picks one **by the civilisation**, not
the site, and passes `wall` and `floor` to the stamp, as `ruin_pass`
already accepts them. Each script reads the param before falling back to
its own choice (two lines in each of about fourteen scripts). With no
param a script builds exactly what it builds today, so
`TestContentExistingStructuresUnchanged` holds.

Picking by civilisation is the point: the history pool's peoples span
sites, so **a people's masonry becomes recognisable across the galaxy**.
The player who has seen red studded walls on two worlds knows who built
the third. That is worth more at 150 systems than the count of styles.

| Level | Today | Added styles (fantasy rows unless noted) |
|---|---|---|
| stone age | overgrown stone | sand drystone (20), grey drystone (21), red drystone (22), khaki stone (2) |
| iron age | stone | pale grey stone (0), red studded (3), dark timber (4), sandstone (19) |
| industrial, atomic | bunker (green) | grated metal (9) |
| star age | advanced (teal) | steel smooth (sci-fi 29) |
| hive and alien | resin floor only | purple organic (sci-fi 26), green organic (sci-fi 27) |
| **the gate's builders** | | **dark smooth (sci-fi 28), reserved** |

Thirteen pairs. Floors come from the same row's cols 3 to 6. Hedge (14)
and iron fence (16) are dressing for villages and graveyards, not styles.
Row 17 (stone under snow) is a variant the cold maps can substitute, if
the owner wants weathering by world; default no.

**The history stamps.** No new structure scripts are needed for the
styles to show: villages, forts, castles, towers, mines, burial mounds,
bunkers and the three underground stamps all take the param. The
underground dungeon in particular (`dungeon_wall` everywhere today) takes
its builder's style.

**The gate's controls ruin** (galaxy plan, step 4). The forced dungeon
ending passes a `gate` style: the dark smooth set, used nowhere else in
the galaxy. The player learns that wall once and knows the controls world
on sight; a datapad or a scan can then describe it without naming it.
This costs one tile pair and one line in the binding, and should land
with the gate.

**Dressing.** The props the new structures need (next section) are also
scattered by the existing stone and iron age scripts: a few lines each.

### New structures

Added 2026-11-15 at the owner's request. The styles above change what a
ruin is built of; these add shapes. The library has about 50 scripts, and
a structure reaches a site in one of four ways, which is how they are
grouped. Costs are from the existing scripts (`landing_platform` 111
lines, `graveyard` 126, `standing_stones` 156).

What a new script costs: 90 to 160 lines of BASIC; the param contract
(`tech`, `damage`, `occupants`, `holds`, `clue`, `room_<key>`, and now
`wall`/`floor`); `mirror` for four orientations; `ruin_pass` for wear; a
row in the content tests' footprint table; the presets regenerated
(`TestContentPresetsMatchData`). No Go, except one line per station
module (`ModuleKinds`).

**Props first** (about 22 `Inanimate` blueprints, 450 lines of JSON): from
the furniture block, a table, chair, shelf, barrel, urn (three colours),
coffin, gravestone, bones, cobweb, statue, fountain, well, campfire,
weapon rack, grain heap, skull, ribcage; from `scifi_vehicles`, a wrecked
tank, truck and aircraft (tinted, so six colours are not six blueprints);
the cave mouth (43,2). Today's scripts spawn almost nothing but lockers,
lights, beds and tanks, so these props lift every existing ruin as well.

**1. History ruins: a weighted alternative on a role in `tech.json`.**
No new events; history already asks for the role, and a site rolls which
shape it gets (as `seat` rolls castle or castle town today).

| # | Script | Role and age | What | Art | Differs in play | Footprint |
|---|---|---|---|---|---|---|
| S1 | `farmstead` | dwelling; stone, iron | fenced or hedged plots of `dirt_floor` with bushes in rows, a barn, a well | hedge (row 14), fence (row 16), planks, barrels, grain heaps | open ground with nothing to breach; holds food, not trade goods | 21×17, small 17×13 |
| S2 | `longhall` | seat; stone | one long hall, a hearth down the middle, a high seat, benches | timber (row 4), long tables (row 9), campfire, throne | a small seat for small maps; one room, one door | 19×11, small 15×9 |
| S3 | `market` | dwelling; iron, industrial | a paved square, stalls round a fountain, a counting house | paving (rows 23 to 26), tables, urns, crates, fountain | trade goods spread over many small containers in the open | 21×17, small 17×13 |
| S4 | `catacomb` | grave; iron and later | a chapel over a stair to one level of niches | coffins, bones, cobwebs, urns, gravestones | grave goods underground; the natural home of `restless_graves`; counts against the underground lore budget | 15×13, small 11×11 |
| S5 | `quarry` | works; stone, iron | a cut one level down reached by a stair, squared blocks, a foreman's hut | slab (48,1), gravel heaps, pebbles, boulder walls if they resolve | a ready way underground with ore showing in the cut | 19×15, small 17×13 |
| S6 | `compound` | fort; industrial, atomic (and the `prison` and `forced_labour` purposes) | a fence round rows of huts, corner posts, a gate, a yard with a wrecked truck | fence (row 16), bunker tiles, beds, trucks | see-through perimeter: hostiles inside see the party coming | 27×19, small 17×13 |
| S7 | `airbase` | fort; atomic | a strip, two or three parked or wrecked aircraft, fuel tanks, a control hut | road lines (sci-fi rows 30, 31), aircraft and tanks, `fuel_tank` | fuel on the ground: the only ruin worth visiting for it | 31×13, small 19×11 |

Also, with no new script: the `battlefield` at the atomic and star ages
gains wrecked tanks among its wreckage.

**2. The elders and the gate.**

| # | Script | Placed by | What | Why |
|---|---|---|---|---|
| S8 | `waystone` | a new, cheap, repeatable interlude event (`waystone_raised`) on any solid map | a ring or pillar of the builders' own wall with one datapad; 7×7 to 11×11 | the carrier for datapads that point at **other places**, which the pregenerated galaxy makes possible (its Decision: "datapads that point at other places get simpler"). Small enough that the lore budget can afford several per site. A waystone of the gate's builders, in the reserved wall, points toward the gate |
| S9 | `gate_vault` | the galaxy plan's forced ending on the controls world | a wrapper: a marked entry building, `underground_dungeon` two or three levels deep in the reserved style, and a final chamber holding the gate controls, with the two constructs as its last guard | the galaxy plan says the controls sit "deep in it, not at the door" and leaves how they are guarded open (its question 12). This answers it from existing content. About 90 lines; lands with galaxy plan step 4, not before |

**3. Natural landmarks: a `stamp` feature in a biome or map file.** Not
history: nobody built them, they carry no datapad, and as unanchored
stamps they are never dropped by the lore budget. They are what makes a
biome have places in it.

| # | Script | Biomes | What | Differs in play | Footprint |
|---|---|---|---|---|---|
| S10 | `oasis` | desert, badlands, savanna | an irregular pool ringed by trees, bushes and ferns | the one source of biomass in a dry biome; harmless animals gather there | 13×11 |
| S11 | `sinkhole` | any with caverns | a ring of rubble round a collapsed shaft: stairs down into the first cavern, the cave mouth as its marker | a way underground on day one without digging; bats come up it | 9×9 |
| S12 | `predator_den` | by hunter (see Wildlife) | a hollow under a rock overhang, bones, a skull, the pack, a small cache | gives the `predator_pack` hazard a place and an origin, as the yetis' has none today | 9×9 |
| S13 | `vent_field` | scorched_crust, volcanic_island; glacier with cold vents | vents clustered round a lava or red pool, red and green crystals among them | crystal for the taking, guarded by ground that hurts to stand beside | 11×11 |

**4. Station modules.** One line of Go each, a script, and a place in the
`optional` lists of `data/history/purposes.json`.

| # | Script | Purposes | What |
|---|---|---|---|
| S14 | `station_brig` | garrison | cells behind blast doors, beds, a guard desk; sealed like the armoury |
| S15 | `station_workshop` | shipyard, refinery | `foundry`, `robot_workbench` and `gun_bench` (blueprints that exist), parts lockers |

Ranked for the owner: S11 sinkhole and S10 oasis first (small, on every
world, change play); S8 waystone (small, and the galaxy needs it); S1
farmstead and S6 compound (the two most unlike anything in the library);
S4 catacomb; S7 airbase; S3 market; S12 and S13 with the wildlife step;
S9 with the gate; S2, S5, S14, S15 last.

Not proposed: anything built of the gantry frames (sci-fi rows 18 to 20)
or the two pipe sets. Both are multi-piece art with their own layouts and
would each cost a mapping session for one structure.

### Wildlife

Behaviour comes from what exists:

| Behaviour | Source | Hostile by the quiet test |
|---|---|---|
| wanders, harmless | `FactionAI` `wander` (the critter) | no |
| each animal rolls how aggressive it is; the bold ones hunt workers within 14 tiles | `data/scripts/ai/hostile_critter.basic` (in the repo, **used by nothing**) | yes |
| territorial heavy beast | `data/scripts/yeti/behavior.basic` | yes |
| burrower | `worm.basic` | yes |
| does not move, hurts whoever stands beside it | `spore_pod.basic` | yes |

Because every scripted animal counts as hostile, hunters cannot be in a
calm base scenario's spawn rules. They arrive three ways, all existing:
as a **hazard chapter** (finite, at an origin, like `hostile_fauna`), in
the spawn rules of threat scenarios, and by `cave_spawner` underground.
Harmless animals go in the calm scenarios' spawn rules, keyed by ground
tile, which is what makes them per-biome.

All cells below are on `scifi_entities_extra` unless marked.

**Harmless (10): spawn rules of the calm scenarios**

| Animal (draft name) | Cell | Biomes | Drops |
|---|---|---|---|
| fowl | (14,4) | meadow, plains, garden worlds with a fallen colony | protein, a feather |
| brown scurrier | (8,6) | forest, savanna, badlands | protein |
| black beetle | (10,6) | desert, badlands, ash_waste | nothing |
| pink beetle | (11,6) | alien_forest, fern_thicket, living_crust | biomass |
| white bird | (11,4) | tundra, glacier | a feather |
| teal bird | (12,4) | forest, marsh, meadow | a feather |
| bulb walker | (0,2) | fern_thicket, alien_forest | biomass |
| mote (four tints of one droplet) | (2..5,2) | living_crust, crystal_field | crystal, rarely |
| green slime | (1,6) | marsh, mudflat | biomass |
| purple slime | (0,6) | living_crust, caves on alien worlds | biomass |

**Hunters (11): `hostile_critter.basic`, by hazard chapter and cave spawner**

| Animal | Cell | Biomes | Threat |
|---|---|---|---|
| grey, brown and black wolf | (12,6) (13,6) (14,6) | forest and tundra; savanna and badlands; ash_waste and rogue worlds | fast, in twos and threes |
| red and tan scorpion | (7,0) (8,0) | badlands, desert | slow, hard hit |
| red and black spider | (5,6) (6,6) | caves; fern_thicket | cave ambush |
| snake | (9,6) | marsh, savanna | single, weak |
| floating eye | (4,6) | living_crust, alien_forest | flies |
| dark and red bat | (2,6) (3,6) | caves on any living world | weak, many, underground only |

**Heavy and territorial (4): the yeti script**

| Animal | Cell | Biomes |
|---|---|---|
| brown, black and white bear | (4,0) (5,0) (6,0) | forest; ash_waste and dead worlds; glacier and tundra |
| orange lizard | `scifi_entities` (18,30) | badlands, scorched worlds, volcanic islands |

**Guardians (2): placed by ruins, not by biome**

| | Cell | Where |
|---|---|---|
| stone construct, clay construct | (3,4) (4,4) | a new hazard beside `ancient_defences`: one or two dormant in a stone or iron age ruin, waking as the sentry turret does (its script, a melee attack) |

**Stationary (3): `spore_pod.basic`**

| | Cell | Where |
|---|---|---|
| fire vent | (9,0) | scorched_crust, volcanic_island, desert |
| cold vent | (10,0) | glacier, frozen_regolith |
| gas vent | `scifi_world` green cloud (10..13,24) | marsh, mudflat, sludge_sea shores |

Thirty in all. New hazard chapters in `events.json` (three rows, written
like `hostile_fauna`): `predator_pack` (two to four of the biome's
hunters), `old_guardians`, `vent_field`. `vent_field` may be
`safe_start`; the other two may not.

Left for later: the red and purple xeno and orange and green troid bodies
would let a hive differ by region ("strains"), but each is five
blueprints tied to the hive's growth scripts, and belongs to a hive
content pass.

### Distributing it across 40 to 150 systems

Today `weightedArchetype` rolls every system from nine weights. With
nineteen archetypes and a fixed galaxy, four rules are worth adding to the
galaxy generator (galaxy plan step 1, "Archetypes, seeds, names"). All
are fields on the archetype; about 120 lines together.

1. **Weights** (exists). Proposed, with the count each gives:

   | Archetype | Maps | Weight | at 40 | at 80 | at 150 |
   |---|---|---|---|---|---|
   | temperate | earth_like | 12 | 5 | 9 | 17 |
   | garden | garden_world | 8 | 3 | 6 | 12 |
   | alien | alien | 9 | 3 | 7 | 13 |
   | living | living_world | 3 | 1 | 2 | 4 |
   | mire | mire_world | 5 | 2 | 4 | 7 |
   | ocean | ocean_planet, toxic_ocean (3 to 1) | 8 | 3 | 6 | 12 |
   | scorched | desert_planet | 8 | 3 | 6 | 12 |
   | dead | dead_world | 4 | 2 | 3 | 6 |
   | irradiated | irradiated_world | 3 | 1 | 2 | 4 |
   | frozen | frozen_planet | 8 | 3 | 6 | 12 |
   | rogue | rogue_world | 2 | 1 | 2 | 3 |
   | moon | moon | 9 | 3 | 7 | 13 |
   | ice moon | ice_moon | 4 | 2 | 3 | 6 |
   | belt | asteroid | 8 | 3 | 6 | 12 |
   | comet | comet_field | 3 | 1 | 2 | 4 |
   | derelict | abandoned_station, military_station, research_station | 4 | 2 | 3 | 6 |
   | outpost | outpost_station | 2 | 1 | 2 | 3 |
   | hub | hub_station | 1 | 0 or 1 | 1 | 1 or 2 |
   | hive | alien, earth_like, living_world | 2 | 1 | 2 | 3 |

   About half of every galaxy stays the familiar types, so the new ones
   read as finds.

2. **`max` and `min` per galaxy.** `hub` max 2; `living`, `rogue` and
   `irradiated` min 1 at medium and large, so every galaxy of that size
   has each strange world somewhere; at least one station of any kind at
   every size. Without `min`, a 40-system galaxy misses a weight-2 type
   about one time in two.

3. **`region`: core, rim or any**, a bias (not a rule) by a system's
   distance from the galaxy's centre of mass, which every shape in the
   galaxy plan has:
   - **Core:** dead worlds, derelicts and hubs, irradiated worlds. The
     dense middle is where the history pool's peoples were, so its ruins
     are thickest. On a ring galaxy "core" is the inner clump.
   - **Rim:** garden and temperate worlds, belts and comets. This is also
     where every landfall is.
   - **Any:** the rest. Rogue worlds prefer the emptiest neighbourhoods
     (fewest systems within a radius), which puts them in a ring's gap
     and an irregular galaxy's voids.

4. **Landfalls stay mild.** A landfall system is rolled from the
   `start_ok` archetypes only (today temperate and alien; add garden).
   Within the safe radius of a landfall no `harsh` archetype is placed
   (`irradiated`, `rogue`, `dead`, `living`, `hive`): the site histories
   there are already bound safe, and the world itself should not be the
   danger. The new hunting-animal hazards are not `safe_start`.

5. **The controls world** is rolled from archetypes marked `gate_ok`:
   solid ground with an underground lore budget (not a station, belt or
   comet, and not `irradiated`, where the fight to the controls would be
   against the surface as well).

**Unique or regional?** Recommended: nothing is hand-unique. `hub` is
capped, the gate's wall style is unique by construction, and regions are
a bias. A unique named world is objective-plan material.

**Variety inside a type** comes from three rolls that already exist or
are added here: the map list on an archetype, `biome_map.pick`, and the
civilisation's ruin style. A galaxy of 150 has about 17 temperate worlds;
with four of six biomes each and a dozen peoples' styles, no two need
match.

## Order of work

Each step leaves the game playable and is checkable alone. Sizes are
rough; JSON counts are generous because a 16-variant wall is 90 lines.

0. **A content net** (150 lines of test, no content). One test walks
   every tile variant and every blueprint `Appearance` and fails if a cell
   is outside its sheet or fully transparent; another fails on a biome
   rule, feature param, spawn-rule tile or scenario blueprint that names
   something that does not exist (today unknown names "skip cleanly",
   which hides typos); a third generates every map on seeds 1 to 20 and
   fails on a nil level or a missing landing. Everything after leans on
   it.
1. **Biomes on existing maps** (B1 to B6; about 250 lines of JSON, no
   code). Six ground tiles, ten prop blueprints, six biome files, five
   map lists. Re-run `TestEveryMapBiomeAppears` and move centroids until
   every biome shows. vmtest: one surface frame per changed map, one
   underground frame on a glacier.
2. **Harmless wildlife** (450 lines of JSON). Ten blueprints, spawn rules
   in the calm base scenarios, feather and fang items from
   `fantasy_items` if the owner wants drops (else protein and biomass
   only). Test: quiet starts still spawn nothing hostile.
3. **Ground edges** (60 lines of code, about 700 of JSON for seven
   edged grounds, or a 40-line helper that writes them since every set
   has the same layout). The `underlay` field and draw, then `grass`,
   `sand`, `ice_floor`, `alien_ground` and the new grounds switched to
   edged. vmtest: a frame on each biome border before and after
   (`vmtest-diff`).
4. **`weights_as` and `pick`, then the look-only worlds** (45 lines of
   code; W1 to W6: six maps, biomes B7, B9, B11 to B13 and `comet_ice`,
   six archetypes; about 500 lines of JSON). `make history-dump` must
   show each new map's tier shares equal to its model's. vmtest: a
   landing frame per map.
5. **Ruin styles** (100 lines of code, 45 of script, about 1,200 of JSON
   for thirteen walls and floors, or the same helper). Styles in
   `tech.json`, the pick by civilisation, the param in each script, the
   reserved `gate` style left unused until the galaxy plan's gate step.
   Tests: frozen structures unchanged with no param; every style builds
   every preset in `structure_presets.json`. Frames: the structure
   viewer's presets, one per style.
5b. **New structures** (about 1,900 lines of BASIC, 450 of JSON for
   props, 60 of data, 2 lines of Go). Props first; then S10, S11, S8;
   then the history alternatives S1 to S7 one at a time, each with its
   `tech.json` weight at 1 beside the existing script; S14 and S15 with
   or after step 8. S12 and S13 land with step 6, S9 with the galaxy
   plan's gate. Tests: each script at its smallest and largest footprint
   with every param, inside its footprint on every level, intact and
   ruined (the existing content tests' table); `sinkhole` always reaches
   an open cavern or builds nothing; `history-dump -stats` shows each
   alternative on a share of sites. Frames: one structure-viewer preset
   per script, intact and at damage 2.
6. **Hunters, guardians and vents** (about 900 lines of JSON, three
   chapters in `events.json`, 30 lines of BASIC for the guardian).
   Twenty blueprints. Tests: each new hazard spawns at its origin within
   the hostile budget; none appears on a safe start. Re-read
   `history-dump`: three new hazard chapters dilute the others' shares.
7. **Worlds that play differently** (W7 to W11; five maps, three base
   scenarios and setup scripts; about 600 lines). The irradiated world's
   surface number is tuned by playing it: a landing party in suits should
   get a working day on the surface, not a week. vmtest: the rogue
   world's lighting at noon.
8. **Station palettes** (50 lines of code, rubble tiles, two maps). Test:
   after `retile` no `hull_wall` or `hull_floor` remains and the station
   tests pass on all three palettes. The greyish settle check and the
   airlock code that name `hull_wall` must be read first (see riskiest
   assumptions).
9. **Distribution** (120 lines, with or after galaxy plan step 1): `max`,
   `min`, `region`, `harsh`, `gate_ok`, and the weights. Tests: over 200
   seeds at each size, every `min` is met, no `max` exceeded, no harsh
   system inside a safe radius, the controls world is `gate_ok`; a table
   of archetype counts by size is printed for the owner to read.
10. **Docs and a soak.** `docs/developer` (`biomes.md`, `world_generation.md`,
    `tile_definitions.md`, `lore.md`, `entities.md`), the player guide's
    world list, and the observer soak run once across a medium galaxy to
    see that no new map breaks a faction's landing or settlement.

About 550 lines of Go, 2,000 of BASIC (nearly all of it the fifteen new
structures) and 5,000 of JSON (3,100 if the helper writes the walls and
edged grounds). Steps 1 and 2 are a weekend
of data and change how every existing world looks. Steps 3, 5 and 8 are
independent of each other.

## Art gaps

Listed for the owner; nothing above depends on them.

1. **A recoloured blob sheet or two** (`rock.png` in sandstone, red and
   black). The Oryx natural walls work but look blockier than the owner's
   sheets beside them. One recolour gives badlands, dead and irradiated
   worlds an underground of their own; this is the highest return on the
   list.
2. **Two or three single-cell alien plants, a mushroom first.** There is
   no fungus on any sheet (the cell first read as a mushroom is a cut
   stump). The alien biomes have one
   pair of ferns, the bushes and the existing flora to share between
   `fern_thicket`, `living_crust` and `alien_forest`.
3. **Organic ground with edges.** The purple and green organic floors are
   three cells each with no border pieces, so `living_crust` meets its
   neighbours in a straight line.
4. **A meat or hide icon.** Hunting drops are protein bars until there is
   one.
5. **Anything larger than one cell that moves.** Every creature is 24 px.
   The bears read as heavy only by their stats.
6. **Liquid animation.** Water, lava and the new liquids are still.

Not gaps: the canopy blocks and spire ranges exist and are unused only
because placement is one cell per entity; a multi-cell scatter (about 80
lines) would unlock seven forest and mountain looks. It is left out of
the order of work because the single trees already cover the need.

## Open questions

Each with a recommended default.

1. **Own ground tile per biome, or share `grass` and `sand`?** Default:
   own names (needed for per-biome animals), sharing sprites where the
   colour is the same.
2. **Edged ground at all?** It changes the look of every existing world.
   Default: yes, as its own step, judged from the before-and-after frames
   before anything is committed to it.
3. **Do peoples keep one masonry everywhere?** Default: yes, by
   civilisation. Alternative: by site, which gives more mixing and loses
   the recognition.
4. **Is the gate's wall reserved?** Default: yes, the dark smooth set
   appears only at the controls. Alternative: also on one or two "elder"
   ruins as a false lead.
5. **How lethal are the hunters?** Default: a pack is a hazard for an
   unarmed work party and trivial for an armed one, like the two yetis of
   `hostile_fauna`.
6. **Do harmless animals feed the colony?** Default: yes, a protein bar
   as the critter gives today, so living worlds are worth more than their
   biomass.
7. **Should `toxic_ocean` and `sludge_sea` exist without a liquid that
   hurts?** Default: no; drop W4 and B12 until there is one. It is the
   weakest addition.
8. **Should the irradiated world be landable at all early?** Default:
   never inside a safe radius, never the controls world, and its Star Map
   summary says what it is once identified.
9. **Planet portraits on the Star Map.** `scifi_world_extra` has a
   portrait for nearly every world type here (ice, lava, ocean, ringed,
   cratered, green). Default: out of this plan; raise it with the galaxy
   plan's Star Map step, where 150 identical circles are a known worry.
10. **The yellow wall set and the fantasy furniture.** Default: leave the
    yellow set unused; use furniture only inside stone and iron age
    ruins.
11. **Weathering by world** (snowed stone on cold maps). Default: no; it
    fights the recognition in question 3.
12. **Which new structures replace and which join?** Default: every
    history alternative joins at weight 1 beside the existing script, so
    half of stone age dwellings are farmsteads. Alternative: lower
    weights, keeping the village the norm.
13. **Do natural landmarks appear near a landfall?** Default: oasis and
    sinkhole anywhere; dens and vent fields never inside a safe radius.
14. **Is the gate vault this plan's or the galaxy plan's?** Default: the
    galaxy plan builds it in its step 4, using the style, guardians and
    props from here.

## Where the owner rewrites text

- Every biome, map and archetype name and summary; the base scenario
  names and descriptions for the mire, irradiated and rogue worlds and
  their setup scripts' landing lines.
- Every animal's name; the three hazard chapters' history text and
  datapad lines; the guardians' and vents' names.
- The style labels if they are ever shown (they need not be).
- Room names and clue lines for the new structures (barn, hall, counting
  house, crypt, cut, guard post, control hut, cells, workshop), the
  `waystone_raised` event text and the waystones' datapads, and every
  prop's name.
- The player guide's world list.

## Riskiest assumptions

- **The sci-fi organic and smooth wall sets can be mapped.** Rows 26 to
  29 of the sci-fi sheet are laid out differently from the sets in use,
  and rows 20 to 22 of the fantasy sheet may lack an interior piece. If a set lacks some of the
  sixteen cases, the living crust keeps its floor but takes `rock` for
  walls (as the sample image does), the gate's reserved style falls back
  to another unused set, and the drystone styles are dropped.

- **A script can open the ground.** `quarry` and `sinkhole` assume a
  structure script can cut down a level and leave a usable stair, as
  `mine` and the underground stamps do, and that a sinkhole can find a
  cavern under its footprint on most seeds. Where it cannot, it must
  build nothing rather than a stair into rock.
- **Seven more ruin shapes fit the lore budget.** Alternatives replace,
  they do not add, so the count of stamps per site should not move; the
  larger footprints (compound, airbase) will be dropped more often on
  small maps, which is what their `small` sizes are for.
- **The floor autotile works as read.** `resolveFloorAutotileVariant` is
  in the draw path but no tile uses it, so it has never run in the game.
  The case table was measured from pixels, and one piece of the
  north-south run (col 32) did not measure cleanly. Step 3 starts by
  proving one ground.
- **A new solid tile digs, lights and paths like rock.** Read in the
  code (only deposits are named), not tested. The faction directors' dig
  logic, the hive's and the autonz conversion may name `rock` or `dirt`
  in data or scripts; step 0's test and one soak find out.
- **`weights_as` is enough for a look-only map.** Scenarios also carry
  `supported_maps`, and setup scripts name ground tiles; a new map that
  inherits weights may be handed a scenario whose spawn rules list none
  of its tiles, which spawns nothing rather than failing. Step 0's test
  should fail on a scenario bound to a map none of whose tiles it lists.
- **Retiling a station is safe.** Code names `hull_wall` at run time
  (`faction_greyish_settle.go:513`, the ship, the build menu). If
  anything decides airtightness or ownership by that name, a green
  station leaks. Read before step 8; if it does, the palette becomes a
  primer parameter instead (about 150 lines).
- **Surface radiation makes a good world, not a tedious one.** W8 depends
  on one number and on suits. It may need the party to start with them.
- **Hunters behind hazard chapters are seen often enough.** Hazard
  chapters are rolled by weight among a dozen others, so a given world
  has wolves perhaps one time in eight. If animals should be a normal
  part of living worlds, they need a spawn rule in a non-quiet base
  scenario, and the quiet test needs a category between harmless and
  hostile.
- **`hostile_critter.basic` still works.** It is referenced by nothing
  and may have rotted against the builtins.
- **Nineteen archetypes do not thin the good ones.** At 40 systems the
  weights give three garden worlds and one living world. The `min` rule
  keeps the strange ones present, but a small galaxy has only so many
  slots.
- **The sprites read as intended at game scale** with fog and lighting.
  Everything here was judged at 2× on a grey ground; the vmtest frames
  are the real check, and the owner's eye the final one.
- **Three new hazard chapters and thirteen maps move the pinned tables.**
  `history-dump`, the faction pace goldens and the soaks are all
  sensitive to which maps exist. This plan should land either before the
  galaxy plan re-pins them or after, not during.

## Sample images

Composed 2026-11-14 straight from the sheet cells named above (not by the
generator), one per biome plus two edged-ground borders, to check the
cells. They corrected three readings, all folded in above: the
"mushroom" is a stump, the "clover" and "orange flowers" are bushes, and
the organic wall sets are not in the standard order (the boulder sets
are unresolved). Row 13
(orange cave rock) and the sixteen-case ground edges composed correctly
at the first attempt. The images were not committed.

## As built
