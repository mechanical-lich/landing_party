# Abandoned Station Rebuild — Implementation Plan

<<<<<<< HEAD
Status (2026-11-07): **plan only**, nothing implemented. Written against
=======
Status (2026-11-07): **built**, steps 1-9, on `claude/station-5b3f90`
(not yet merged); as-built notes per step at the end. Written against
>>>>>>> refs/heads/claude/station-5b3f90
`dev` at `1c14684`. All draft text here (room names, messages, clue lines,
quest names) is placeholder; the owner rewrites it (see "Where the owner
rewrites text").

## Decision

The Abandoned Station is the one map where the generated-lore system has no
ground to stand on. The owner's direction (2026-11-07):

- **Build the station from modules the way the crashed ship is built from
  sections.** A frame (spine or ring, docking arm, hub stair column) with named
  modules hung off it: reactor, generator, cargo, quarters, hangar,
  bridge/control, lab, cryo, armoury, botany, airlock/dock, archive and more.
  Decks differ because modules are assigned to decks.
- **The history decides what the station is.** The founding event picks a
  station purpose (relay, research, refinery, shipyard, garrison, habitat);
  events add modules (`lab_founded` → lab, `reactor_built` → reactor,
  `archive_sealed` → sealed archive); origins anchor to the module, so the
  evidence lands where the chronicle says.
- **Decay comes from the history.** The drone relay is intact and dark; the
  lost station carries the damage its ending implies (nanites, breach,
  reactor failure, abandonment).
- **Physical goals.** An unpowered generator to restore; doors and lights
  that come on with it; cargo holds; the bridge or archive holding the
  station's log; a sealed section that needs a tool or research.
- **Map size follows station size**, so the void is a margin.

Decided constraints carried over: scenarios stay random from the player's
view (no picker); first jumps stay safe; the owner commits.

## Where the code is today

Reproduced on `dev` at `1c14684` with `make history-dump HSTATS=500
HMAP=abandoned_station`, `go run ./cmd/history-dump -seed 3 -sites 6 -map
abandoned_station`, and a throwaway probe test (built through
`go test -overlay`, nothing written to the repo) that runs the planet
generator's `app.generate()` with "Scenario from history" on for 10 seeds (1,
2, 3, 7, 42, 99, 123, 4242, 777, 31337) at the rolled size, then prints each
origin's resolved tile, the budget, and the entities.

The owner's diagnosis holds. Two parts are worse than described, and one is
slightly off:

| # | Concern | What the code does | Measured | Code / data |
|---|---|---|---|---|
| 1 | Layout | Every deck: a radius-5 hub, 4 radial corridors (4 wide), a 4–6 tile node room at each spoke tip, and up to `bud_attempts` (80) rooms budded off walls. The spoke angle is re-rolled per deck (`angleOffset` is inside the floor loop), so the decks are **not identical, only rotated**. Budding picks random tiles over the **whole map** and keeps only hits on a `hull_wall`, so on a 120–160 map almost every attempt misses. | Seed 1 (157×152): **no budded rooms on decks 0–2**. The station is a hub, a cross of corridors and four node rooms; ~500 floor tiles on deck 0 in a 23,000-tile map. One stair column at `(cx+2, cy)`. | `primer_station.go` `Prime`, `budRooms` |
| 1b | Upper decks | The ceiling pass writes `hull_floor` on deck z+1 above every deck-z structure tile. Since each deck is rotated, each upper deck gets the deck below's outline as **bare, unwalled plates** outside its own walls. | Structure tiles per deck, seed 1: 501 / 605 / 800 / 1103 / 1120. | ceiling pass in `Prime` |
| 1c | Named places | Regions: `station_hub` (every deck), `station_room_large` (the largest room, which is always a spoke-tip node), `starting_floor` (hub, deck 0). No room names, no hull, no docking arm. | Botany bays (`dirt_floor` discs) sit in spoke-tip nodes. | `TagRegion` calls |
| 2 | Origins | `resolveOrigin` looks for open `TKSurface` ground; a station has none (`GetSurfaceZ` is −1 everywhere), so every origin falls back to `nearestFloorOutside` **on `SurfaceZ` = deck 0**. And the landing reserve (21 wide) sits on the hub, with keep-outs of 16 (inert) or 40 (hazards, chapters, spawners) measured in x/y only (`landingDistance` ignores z). Origins must be ≥ 26 tiles from the centre, which on a 50-wide station leaves only the spoke tips. | **48 of 48 origins on deck 0; 44 of 48 more than 15 tiles from the centre** (spoke tips). Datapads (3–7 per site) are the only lore on most sites. | `origins.go` `resolveOrigin`, `nearestFloorOutside`; `landing.go` `landingDistance` |
| 2b | Stamps dropped | Station events stamp ground structures: `reactor_built` → `abandoned_reactor` 21×19, `lab_founded` → `underground_bunker` 15×13, `observatory_built` → `circle_tower` 25×25, `archive_sealed` → `underground_hideout` 21×17. Footprint budget is 3% of the map (432–768 tiles), so a second structure overflows and `circle_tower` never fits. | 10 seeds: 12 stamps dropped for `footprint`. | `History.Evidence` in `internal/history/render.go`; `lore_budget` in `data/generation.json` |
| 2c | **Stamps kept build nothing** | The stamps the budget keeps (`crashed_ship`, `underground_hideout`, `underground_bunker`) are recorded in `StampFootprints`, but every structure script starts with `sz = get_surface_z(...)` and returns when it is −1. `hull_breach` is the one script that falls back to `origin_z`. `lore_occupants_test.go` already notes "a station has none, so none are placed". | Seed 1: the 21×17 and 13×9 footprints contain **zero** non-hull tiles. The `-stats` "1.1 stamps per station site" are phantoms except `hull_breach`. | `crashed_ship.basic` line ~48, every `ship_*`/ground script; `feature_placers.go` `placeStamp` |
| 2d | Planet events on a station | Nothing filters events by kind of site. Station chronicles include `meteor_fall` ("left a glassy scar"), `buried_cache`, an archive "in a vault below the surface", and `civ_rises_on_ruins` in `lost_station`'s first stage (unreachable: it needs native life). | Seed 3 site 1, seeds 123, 4242, 31337. | `data/history/events.json`; `Requires` in `internal/history/data.go` |
| 3 | Too abandoned | Setup spawns one `storage_locker` and one `basic_workbench` at the hub and says "Find the main generator." No generator exists; no beds, consoles, lights, airlocks or doors (door gaps are empty `Middle`). | Typical site: `basic_workbench=1 storage_locker=1 datapad=3..7`, plus the threat's creatures. | `station_setup.basic`; primer |
| 4 | No purpose | The base scenario says "Restore power and reclaim the botany bays". Nothing is powered, the bays are two to four dirt discs, nothing is locked or repairable. Base `spawn_rules` are empty, `lighting` fixed at 20. | 68% of station sites play this base scenario (500 seeds). | `data/scenarios/abandoned_station.json` |

Station sites today (500 seeds): 7.2 events, 4.6 origins, tiers
32/46/22 against the 35/45/20 target. Scenario share: `abandoned_station` 68%,
`exterminate_xenos` 9%, `autonz_incursion` 9%, `grey_contact` 9%,
`station_mutants` 5%. The station's chapters are `station_experiment`,
`salvage_expedition`, `infested_colonists`, `wreck_eggs_hatch`,
`swarm_landing` and `wreck_nanites_wake` (threats), and `wreck_leak` (the
only `safe_start` hazard), `dormant_nest`, `autonz_sleeper`,
`restless_graves`, `mutant_band` and `hull_breach` (hazards).

What already exists and can be reused:

- **Section scripts.** `crashed_ship.basic` lays `ship_engine`,
  `ship_reactor`, `ship_cargo`, `ship_quarters`, `ship_cryo_bay`,
  `ship_armoury`, `ship_hangar` and `ship_bridge` into rects that share walls,
  passes `holds`, `clue`, `room_<key>`, `crew_state`, and finishes with
  `ruin_pass` at `damage` 0–3. Each section carves its own rect at
  `get_surface_z`.
- **Builtins.** `fill_container`, `spawn_lore_datapad`, `name_room` (saved,
  shown on hover), `carve_room`, `mirror`, `tag_region`, `gen_structure`,
  `set_tile`, `clear_middle`, `place_radiation_blob`.
- **Doors.** `airlock` and `blast_door` blueprints. `rlcomponents.DoorComponent`
  (ml-rogue-lib v1.15.0) has `Locked` and `KeyId`. `FactionDoorSystem` skips
  locked doors and `IsDoorPassableByFaction` refuses them, but **nothing in
  the game ever sets or clears `Locked`, and nothing reads `KeyId`.**
- **Lights.** `rlcomponents.LightComponent{Level, Range}`, the `work_light`
  blueprint, and tile light sources in `LightingSystem`. No on/off state.
- **Power.** None. `work_light` is described as "powered", but no power system
  exists.
- **Goals.** Objective triggers `structure_built`, `tech_researched` and
  `flag_set`; quest templates in `data/quest_templates.json` with
  `requires_tag`; the `research_lab`, `bed`, `fuel_tank`, `hazmat_vault`,
  `bio_silo`, `mineral_bin` and `fuel_refiner` blueprints; `BuildRequest` with
  `Delivered` (colonists haul materials to a site, then build).
- **Vacuum.** `space` tiles need `vacuum_resist` (a full enviro outfit) to
  enter. Vacuum blocks movement and does no damage (`hull_breach` relies on
  this).

## Design

### 1. Who builds the modules: primer or history

The two options:

- **(A) The primer composes modules directly** from a station plan that the
  history writes: purpose, frame, deck count, and a list of modules each with
  its event, purpose, damage and seal. The primer lays the frame, allocates
  every module to a slot, and runs each module's script into its slot.
- **(B) The primer lays only the frame**, and the history stamps each module
  into an empty slot through the ordinary `stamp` feature.

**Recommendation: (A).** Reasons:

1. Modules must share walls with the frame, open doors onto its corridor, and
   line up stair columns across decks. A slot is a rect the frame already
   knows. `placeStamp` searches for open ground, keeps `spacing`, avoids the
   landing and charges the footprint budget, and none of that applies to a
   module.
2. The station must be complete without a history: unit tests, the planet
   generator with "Scenario from history" off, and
   `loreUnchangedWithoutHistory`. Under (A) the primer rolls a default plan
   from its own seed. Under (B) a history-less station would be an empty
   frame.
3. One build path. `BuildWorld` already passes `TerrainParams` to the primer,
   and the planet generator reaches the primer the same way the game does, so
   preview parity comes free.
4. History evidence still layers on top as features: datapads, spawners,
   radiation, `hull_breach`, the docked wreck. These anchor to module regions
   the primer has tagged.

The plan travels as `TerrainParams["station_plan"]`, a JSON-shaped
`map[string]any` that `History.Evidence` fills (`SiteEvidence.Station`), so
nothing new is threaded through `BuildWorldOptions`. Flags are set after
`Prime`, so they can't carry it.

### 2. The frame

A frame is one of three shapes, picked by the purpose and drawn the same on
every deck it spans. Upper and lower decks may be shorter, so the silhouette
tapers. The frame consists of:

- **Spine**: a 3-wide corridor along the long axis, walls both sides. Slots
  hang off both sides. The **dock** sits at one end of deck 0, opening onto an
  exterior **docking arm** (a 3-wide gantry of `hull_floor` into the void,
  ending in an airlock). Habitat, refinery, shipyard and relay stations use it.
- **Ring**: a square or octagonal loop corridor round a **hub**, joined by 2–4
  spokes. Slots sit outside the ring and in the hub's corners. Research,
  garrison and habitat stations use it.
- **Cross**: two spines crossing at the hub. A small relay or garrison uses it.

Stairs: a **main column** at the hub or the spine's middle, plus a **service
column** at the far end. Two columns make every deck a loop, so one locked
door never cuts a deck off. Both columns sit on corridor tiles, never inside a
module. The ceiling pass seals only tiles under the next deck's footprint, and
the plates outside the deck above's hull are dropped (fixes 1b).

Region tags written by the primer (the existing ones keep their meaning):

| Region | Where |
|---|---|
| `starting_floor` | the dock's pad on deck 0 (was the hub) |
| `station_hub` | the main stair column, every deck |
| `station_room_large` | the largest module on deck 0 (kept for the old radiation feature) |
| `station_dock` | the dock |
| `module:<id>` | each module's centre, at its deck (`module:lab`, `module:reactor`, `module:cargo:2` for a second hold) |
| `station_deck:<z>` | one tile per deck, on the main column |

The module rects are also kept on the level (`Level.Modules`, saved; id,
kind, rect, z, name, sealed, damage) for `resolveOrigin`, the power system and
the planet generator's overlay.

### 3. Modules

Each module is a structure script run into its slot with the param contract
from `docs/developer/lore.md` plus `z` (new: the deck; see step 2). Sizes are
slot classes: S ≈ 7×7, M ≈ 11×9, L ≈ 15×11, XL ≈ 21×13. Scripts named `ship_*`
are reused with a `z` param and `frame=station`, which turns off ship-only
details (keel-row furniture rules, the crash furrow). The rest are new
`station_*.basic` scripts.

| Module | Script | Slot | Shape | Contents |
|---|---|---|---|---|
| **Dock** | `station_dock` (new) | M | An open pad behind an airlock pair, the docking arm outside | The landing (`starting_floor`). Two `airlock`s to the arm. An empty locker for the party. A wall console (draft: "Docking Control"). |
| **Generator** | `station_generator` (new) | M | Machine hall, the generator against the far wall, conduit tiles along the walls | A `station_generator_broken` (step 7), 1–2 `fuel_tank`s, a `machine_parts` locker. Named "Main Generator". **The base scenario's goal.** |
| **Reactor** | `ship_reactor` + `z` | L | Shielded core room with a double wall toward the corridor | The `radioactive_ore` core, a radiation blob scaled by damage (cold at 0, leaking at 2+), a `fission` locker. On a lost station whose ending is a reactor failure, it is the damage centre. |
| **Cargo** | `ship_cargo` + `z` | L/XL | Racks in rows, a wide door | 3–6 `storage_locker`s filled from the purpose's `holds` (`ship_cargo`, `mine_ore`, `trade_goods`, `colony_stores`). The main loot. |
| **Quarters** | `ship_quarters` + `z` | M/L | Cabins off a short hall | `bed`s, personal lockers (`provisions`), the occupants' datapads. Habitat stations have several. |
| **Hangar** | `ship_hangar` + `z` | XL | Large bay; its outer wall is a `blast_door` run open to space | Shuttle debris or a parked hull, fuel. On a shipyard it becomes a **gantry**. A drone relay has a drone bay. |
| **Bridge / Control** | `ship_bridge` + `z` | M | Consoles in an arc, a command chair | The **station log** (step 7), a keycard locker. `station_built`'s origin. |
| **Lab** | `station_lab` (new) | M/L | Benches, specimen tanks, a sealed inner room | A `research_lab` (working once powered), a `hazmat_vault`, a `medical` locker. Vars `plague_study`, `xenobiology`, `weapons_research`. `lab_founded`'s origin and `station_experiment`'s. |
| **Cryo** | `ship_cryo_bay` + `z` | L | Rows of pods | Sleepers when `occupants: cryo` (`infested_colonists`). |
| **Armoury** | `ship_armoury` + `z` | S/M | Weapon racks behind a `blast_door` | An `old_weapons`/`ship_armoury` locker. **Sealed** on a garrison (keycard). |
| **Botany** | `station_botany` (new) | L | Plots of `dirt_floor` in rows, a `bio_silo`, light strips | Plants, dead by damage. **The base scenario's "botany bays"**, now real rooms. Replaces the `botany_bay` feature on this map. |
| **Archive** | `station_archive` (new) | M | Vault behind a `blast_door`, shelving | An `archive` locker, the archive's datapads. **Sealed** (keycard or research). `archive_sealed`'s origin. |
| **Observatory** | `station_observatory` (new) | M | Round room on the top deck, its outer ring `space`-backed "windows" (a glass tile; see open questions) | A desk and datapad. `observatory_built`'s origin. Always on the top deck. |
| **Relay core** | `station_relay` (new) | L | Machine racks round an antenna shaft through the top deck | `machine_parts`, `starfaring` lockers. The drone station's heart. `drone_station_built`'s and `autonz_sleeper`'s origin. |
| **Refinery** | `station_refinery` (new) | XL | Ore hoppers (`mineral_bin`), a `fuel_refiner`, conveyor floor | `mine_ore` lockers, `fuel_tank`s. |
| **Med bay** | `station_medbay` (new) | S/M | Beds and a cabinet | A `medical` locker. Habitat and garrison only. |
| **Hub** | frame | — | The stair column's junction | Stairs, a wall light. |

Module ids are `<kind>` or `<kind>:<n>`. Every module script calls
`name_room` (default name, or the purpose's `room_<kind>`), honours `damage`
0–3 by finishing with `ruin_pass` (wall `hull_wall`, floor `hull_floor`), takes
`occupants` (step 5) and stays inside its rect on its deck.

**Deck assignment.** Each module kind has a preferred deck band: `dock`,
`hangar`, `cargo`, `refinery` low; `quarters`, `botany`, `medbay`, `cryo`
middle; `bridge`, `lab`, `archive`, `armoury` upper-middle; `observatory` and
the relay's antenna top; `reactor` and `generator` on the lowest deck below
the dock, or the dock deck on a 3-deck station. The allocator fills slots
deck by deck, preferred band first, deterministically from the primer's seed,
so decks differ because their modules differ.

### 4. Purposes

A station purpose is picked by the founding event (`station_built` or
`drone_station_built`) from a new `station_purposes` table in
`data/history/purposes.json`, beside `ship_classes`. Draft table:

| Purpose | Arc | Frame | Decks | Always | Often | Holds | Bias |
|---|---|---|---|---|---|---|---|
| `relay` | drone_station (only), lost_station | spine | 3 | dock, generator, relay | cargo, hangar (drone bay), observatory | `machine_parts` | Drone: no quarters, beds, botany. Intact, dark. |
| `research` | lost_station | ring | 4–5 | dock, generator, bridge, lab, quarters | lab:2, cryo, archive, observatory, medbay | `medical` | `station_experiment` weighted up |
| `refinery` | lost_station | spine | 4 | dock, generator, reactor, refinery, cargo | cargo:2, quarters, hangar | `mine_ore` | Large cargo, reactor-failure endings likelier |
| `shipyard` | lost_station | spine | 4–5 | dock, generator, hangar (gantry), cargo | reactor, quarters, armoury | `ship_cargo` | Docked wreck likelier (a hull in the yard) |
| `garrison` | lost_station | cross / ring | 4 | dock, generator, bridge, armoury, quarters | armoury:2, medbay, hangar | `ship_armoury` | Sealed armoury; future turret hazard |
| `habitat` | lost_station | ring / spine | 5 | dock, generator, bridge, quarters, botany | quarters:2, botany:2, medbay, cargo, cryo | `colony_stores` | The base scenario's botany goal reads best here |

Events add modules beyond the purpose's set (section 6). A module an event
asks for that the purpose already has is reused: that module becomes the
event's origin and no second module is built.

### 5. Occupants

Modules follow the occupants convention the lore content plan set for ground
scripts, with a different default: **a module the primer builds passes
`occupants: none`**, because the station's danger comes from its chapter and
hazards, which the lore budget charges. History can override a module's
occupants through the event that adds it: `cryo` for `infested_colonists`'
cryo bay, `dormant` for a future garrison turret hazard. Module scripts
declare `occupants` in their header, and the content test that checks every
script declares it covers them. No module spawns guards.

### 6. History: schema changes

**Map** (`data/maps/abandoned_station.json`): mark the site kind, weight the
purposes, override the budget, and shrink the size (section 9).

```json
{
  "id": "abandoned_station",
  "size": { "w": { "min": 100, "max": 140 }, "h": { "min": 80, "max": 120 }, "z": { "min": 5, "max": 5 } },
  "terrain": "station",
  "terrain_params": { "margin": 14, "dock_arm": 10 },
  "features": [],
  "present": {
    "kind": "station",
    "life": ["none"],
    "status": ["fallen", "dead"],
    "arcs": ["drone_station", "lost_station"],
    "tiers": {"threat": 35, "hazard": 45, "quiet": 20},
    "safe_tiers": {"hazard": 60, "quiet": 40},
    "density": 40,
    "station_purposes": {"relay": 15, "research": 20, "refinery": 15, "shipyard": 15, "garrison": 15, "habitat": 20},
    "lore_budget": {"stamps": 2, "footprint_percent": 6, "underground": 0}
  },
  "base_scenario": "abandoned_station"
}
```

`present.kind` defaults to `"planet"`. `lore_budget` on a map overrides the
matching fields of its size class. Modules are not stamps and cost nothing
against it.

**Purposes** (`data/history/purposes.json`, new top-level table):

```json
"station_purposes": {
  "research": {
    "frame": ["ring"],
    "decks": [4, 5],
    "modules": ["dock", "generator", "bridge", "lab", "quarters"],
    "optional": {"lab": 50, "cryo": 40, "archive": 40, "observatory": 30, "medbay": 50},
    "holds": "medical",
    "rooms": {"room_bridge": "Operations", "room_lab": "Research Wing"},
    "clue": "Research log of the {civ}: findings filed, funding cut, crew rotated home.",
    "chapter_weights": {"station_experiment": 2.0}
  },
  "relay": {
    "frame": ["spine"],
    "decks": [3, 3],
    "modules": ["dock", "generator", "relay"],
    "optional": {"cargo": 60, "hangar": 40, "observatory": 30},
    "uncrewed": true,
    "holds": "machine_parts",
    "rooms": {"room_relay": "Relay Core", "room_hangar": "Drone Bay"},
    "clue": "Relay telemetry of the {civ}: carrier steady, no operator logged in."
  }
}
```

**Arcs** (`data/history/arcs.json`): the founding event carries the purpose
the way `ancient_wreck` carries a ship's (`vars.purpose`, here resolved from
the map's `station_purposes` weights). `drone_station` keeps one founding
pick; `lost_station` drops `civ_rises_on_ruins` (unreachable) and gains an
optional stage for station-only events.

```json
{
  "id": "lost_station",
  "stages": [
    {"pick": ["station_built", "settles_on_ruins"]},
    {"pick": ["tech_advances"], "optional": true, "chance": 50, "repeat": [1, 2]},
    {"pick": ["lab_founded", "reactor_built", "observatory_built", "archive_sealed", "station_expanded"], "optional": true, "chance": 90, "repeat": [1, 3]},
    {"pick": ["station_lost", "nanite_conversion", "colony_abandoned", "colony_withdrawn", "reactor_failure", "station_breached"]},
    {"pick": ["settles_on_ruins"], "optional": true, "chance": 20},
    {"pick": ["ancient_wreck"], "optional": true, "chance": 65}
  ]
}
```

**Events** (`data/history/events.json`):

- Evidence blocks gain **`on`** (`"planet"` or `"station"`; empty means both),
  so one event keeps its ground stamp on planets and builds a module on a
  station.
- A new evidence kind **`module`** asks the plan for a module (`params.module`,
  plus optional `purpose`, `damage`, `sealed`, `occupants`).
- **`origin_at: "module:<kind>"`** binds the event's origin to that module.
  The field `OriginAt` exists (`side`, `border`), so this is a third value.
- **`requires.site`** (`["planet"]` or `["station"]`) filters events by
  `present.kind`.
- **`station_damage`** on an ending event says what the ending did to the
  station.

```json
{
  "id": "lab_founded",
  "origin": true,
  "origin_at": "module:lab",
  "evidence": [
    {"kind": "stamp", "on": "planet", "count": 1, "params": {"script": "underground_bunker", "w": 15, "h": 13, "occupants": "none"}, "at": "origin"},
    {"kind": "module", "on": "station", "params": {"module": "lab"}, "at": "origin"},
    {"kind": "lore_datapad", "count": 1, "at": "origin"}
  ],
  "vars": {"purpose": ["plague_study", "xenobiology", "weapons_research"]}
}
```

```json
{
  "id": "station_built",
  "origin": true,
  "origin_at": "module:bridge",
  "vars": {"purpose": "@station_purpose"},
  "evidence": [
    {"kind": "module", "on": "station", "params": {"module": "bridge", "log": true}, "at": "origin"},
    {"kind": "lore_datapad", "count": 1, "at": "origin"}
  ]
}
```

```json
{
  "id": "archive_sealed",
  "origin_at": "module:archive",
  "evidence": [
    {"kind": "stamp", "on": "planet", "params": {"script": "underground_hideout", "w": 21, "h": 17, "occupants": "none", "purpose": "archive"}, "at": "origin"},
    {"kind": "module", "on": "station", "params": {"module": "archive", "sealed": "keycard"}, "at": "origin"},
    {"kind": "lore_datapad", "count": 1, "at": "origin"}
  ]
}
```

```json
{
  "id": "nanite_conversion",
  "station_damage": {"scope": "deck", "at": "origin", "level": 2, "kind": "nanite", "spread": 1},
  "evidence": [
    {"kind": "derelict_pod", "count": 1, "count_max": 3, "params": {"blueprint": "wire_placer"}},
    {"kind": "lore_datapad", "count": 1, "at": "origin"}
  ]
}
```

```json
{"id": "meteor_fall", "requires": {"site": ["planet"]}}
{"id": "buried_cache", "requires": {"site": ["planet"]}}
```

`"@station_purpose"` is a var fill resolved at walk time from
`present.station_purposes`, so the founding event's datapad and chronicle can
say what the station was for. Event-to-module map (the rest keep their own
origin and resolve to the nearest module, step 5):

| Event | Module | Damage / effect |
|---|---|---|
| `station_built` | bridge (+ the purpose's set) | — |
| `drone_station_built` | relay (+ set) | uncrewed: no quarters |
| `settles_on_ruins` | quarters (reused, second civ's lockers) | — |
| `lab_founded` | lab | — |
| `reactor_built` | reactor | — |
| `observatory_built` | observatory (top deck) | — |
| `archive_sealed` | archive, sealed | — |
| `station_expanded` (new, interlude-sized) | one optional module of the purpose | — |
| `station_lost` | bridge | station-wide 1, commander's body on the bridge |
| `colony_abandoned` / `colony_withdrawn` | hangar / dock | 1, emptied lockers, hangar empty |
| `nanite_conversion` | the deck it hits | deck 2 `nanite` (wire tiles), spreads ±1 deck at 1 |
| `reactor_failure` (new ending) | reactor | reactor 3 + radiation, its deck 2, the rest 1 |
| `station_breached` (new ending) | an outer module | module 3 → `space` floor, its deck 2 |
| `drone_station_fails` | relay | 0 everywhere: **intact and dark** |
| `ancient_wreck` | docked wreck at the arm (section 9) | the wreck's own damage |
| `station_experiment` | lab | lab 2 |
| `salvage_expedition` | cargo or hangar | — |
| `infested_colonists` | cryo (`occupants: cryo`) | — |
| `swarm_landing` | hangar | hangar 2 |
| `hull_breach` (hazard) | an outer module; `hull_breach` stamps into its rect | that module only |
| `autonz_sleeper`, `dormant_nest`, `wreck_leak` | docked wreck, else relay / cargo | — |
| `mutant_band`, `restless_graves` | quarters | — |

### 7. Origins on a station

`ResolveOriginsKeepOut` gains a station path, chosen when the level has
`Level.Modules`:

1. An origin bound to a module (`SiteEvidence.OriginModules[eventID] =
   "lab"`) resolves to that module's centre tile and deck. **Keep-out doesn't
   apply**: the module was placed to satisfy it (below).
2. An unbound origin goes to the module nearest its rolled (x, y), counting
   each deck of difference as `deck_cost` (12) tiles and skipping the dock.
   Origins rolled on the same tile still move together.
3. Keep-out on a station is met at **allocation**: a module that anchors the
   chapter, or anything that spawns creatures, goes to a slot at least
   `station_hazard_hops` (2) modules from the dock along the frame graph, and
   not on the dock's deck when the station has three or more decks. The
   landing is at the dock (one end of a spine, one spoke of a ring), so
   threats sit at the far end or on other decks rather than the hub.
4. `landingDistance` gains the `deck_cost` term on station maps, so anchored
   features that search rings (spawners, radiation pockets) also keep their
   distance across decks.

Planets are unchanged: no `Level.Modules`, same code path. The frozen-script
and golden tests keep proving that.

### 8. Decay

Damage is per module, plus a floor per deck, from three sources in order:
the ending's `station_damage`, the chapter's own module (2 for threat, 1 for
hazard), and a base by status (`fallen` 1, `dead` 1, a drone relay 0). The
primer passes each module the larger of its own damage and its deck's.
`ruin_pass` already does 0–3 on walls and floors. Station-only kinds layer on:

- `breach`: interior floor becomes `space`, the outer wall opens. This reuses
  `hull_breach`'s logic in its rect, and the plates are reachable in a suit.
- `nanite`: wire tiles over floors, plus `wire_placer` pods on that deck.
- `meltdown`: a radiation blob scaled by level in the reactor, rubble in the
  corridor outside.
- `abandoned`: furniture intact, lockers emptied to a third of their rolls,
  doors left open.

Every station is dark until the generator runs (section 10). Lighting stays
`fixed` at 20 ambient, and `station_light` entities start off.

### 9. Map size and the void

**Recommendation: map size follows station size, with a margin, and the
margin holds the docked wreck and drifting hull debris.** The exterior is
crossable only in suits, which `hull_breach` already relies on. Filling the
whole void with walkable exterior would make a station read like an asteroid
field and give the budget a "surface" it would then fill with ground stamps.

- The map's rolled size stays (so `History.Evidence` keeps rolling the same
  width for its budget class), but the range shrinks to about 100–140 ×
  80–120. The primer sizes the frame to fill `map − 2 × margin` (margin 14),
  so a bigger roll means a longer spine or wider ring, not more void. A 5-deck
  research ring at 120 × 100 gives roughly 18–24 module slots.
- **The docking arm** runs from the dock into the margin. An `ancient_wreck`
  on a station becomes a **docked wreck** at the arm's end: `crashed_ship`
  with `z` set to the dock deck, damage from the event, its hull in the
  margin, and its airlock joined to the arm. Without a wreck the arm ends in
  an empty airlock.
- **Hull debris**: a few `hull_wall`/`hull_floor` fragments on deck 0 in the
  margin, by damage (new small feature kind `hull_debris`, not a stamp, no
  budget).
- **Lore budget for this map**: every width here is in the small class
  (≤ 180). Modules cost nothing. The stamps left are the docked wreck (scout
  117 tiles, freighter 325, colony 527, warship 435) and `hull_breach`
  (63–165). The map's `lore_budget` override sets `stamps` 2,
  `footprint_percent` 6 (720 tiles at 120 × 100, enough for a warship and a
  breach), and `underground` 0 (no underground structures on a station). The
  colony class and the mothership drop to a warship on a station, as
  `large_only` already does for the mothership.

### 10. Physical goals

| Goal | Built from | New engine support |
|---|---|---|
| **Restore the main generator** | `station_generator_broken` entity in the generator module, a "Repair" order that hauls `machine_parts` × N and `fuel` × M to it (cargo holds supply them), then swaps in `station_generator` | **New:** `Repairable` component `{Cost, Into, SetsFlag}`, and an inspector "Repair" button that issues a `BuildRequest` at the entity's tile, so it reuses haul-and-build. On completion: replace the entity, set level flag `power:main` = 1, fire `structure_built` for `station_generator`. |
| **Lights come on** | `station_light` entities on module walls and corridor junctions, placed by module scripts | **New:** `Powered` component `{Grid, Level, Range, Unlock}` and `StationPowerSystem`. While `power:<grid>` is unset it holds `Light.Level` at 0; when set, it restores the light to `Level`/`Range` once and stops. |
| **Doors open** | `airlock` doors at module entrances (frame), `blast_door` on armoury, archive and hangar | Uses the existing `DoorComponent.Locked`. `Powered{Unlock: true}` doors start locked and unlock when power comes on. **New:** a setup builtin `lock_door(x, y, z, key)` so scripts can lock a door and set `KeyId`. |
| **Sealed section** | Archive (`sealed: keycard`) or armoury (garrison) | **New:** a `KeyDoorSystem` that reads the lib's `KeyId`: a colonist carrying an item whose blueprint matches it unlocks the door when adjacent. `station_keycard` item in the bridge's locker (`fill_container` table `bridge_keys`). Research-gated variant in open questions. |
| **Cargo holds** | `ship_cargo` lockers with the purpose's `holds` | none (`fill_container` exists) |
| **The station's log** | The bridge's (or archive's) **log console**: a datapad entity bound to the site's whole chronicle, reading it makes every station event known | **New:** a `station_log` datapad variant with `Events []string` (the `Datapad` component today binds one event), and a builtin `spawn_station_log(x, y, z)` that binds all of the site's events with origins. Ordinary per-event datapads stay at their modules. |
| **Botany bays** | `station_botany` modules, dead plants by damage | Replanting is the colony's normal farming. The goal is a `flag_set` quest once power is on. |
| **Quest hooks** | Quest templates `station_power` (trigger `flag_set` `power:main`) and `station_archive` (trigger `flag_set` `door:archive`), `requires_tag: "station"` | Quest templates exist. **New:** the `station` location tag on station sites, plus the `door:<module>` flag set when a sealed module's door is unlocked. |

`station_setup.basic` (draft wording):

```basic
# deck_word(z) is a small helper in the script: "on the lowest deck", "two decks up".
function on_setup()
    # The dock is the landing; the generator module is the first goal.
    n = get_region_count("module:generator")
    if n > 0 then
        add_message("Day 1. The station is dark and cold. The main generator is " + deck_word(get_region_z("module:generator", 0)) + ". Bring machine parts.")
    else
        add_message("Day 1. The station is dark and cold.")
    endif
    if get_flag_or("station_purpose", "") = "relay" then
        add_message("No crew ever lived here. The relay core still hums on standby.")
    endif
    ... locker and workbench at starting_floor (now the dock) as today ...
    ... experiment_mutants unchanged, origin is now the lab ...
endfunction
```

The scenario description becomes "Restore the main generator, unseal the
archive, and reclaim the botany bays" (draft). `station_purpose` joins the
lore flags (`docs/developer/lore.md` "Lore flags").

### 11. Tiers, `safe_tiers` and purposes

- The tier is rolled first, as now, and the purpose doesn't change tier
  shares. Purpose changes **which chapter** of the rolled tier is picked,
  through `chapter_weights` (a research station makes `station_experiment`
  likelier, a relay makes `autonz_sleeper` likelier). Purposes and modules can
  also gate chapters: `station_experiment` gets
  `requires.any_events: ["lab_founded"]` or the `research` purpose, and
  `infested_colonists` needs a crewed purpose. `make history-dump HSTATS=500`
  must keep station tiers within ±3 of 35/45/20 and every station chapter
  above 3% of station sites.
- **Damage follows the tier**: quiet sites carry only the ending's damage and
  the status base (dark, worn, intact enough to restore); hazard sites add 1
  in the hazard's module; threat sites add 2 in the chapter's module. The
  Star Map tag (`Calm` / `Hazard` / `Threat`) then matches what the player
  finds.
- **First jumps** (`safe_tiers` hazard 60 / quiet 40) must stay safe, and
  `wreck_leak` is the station's only `safe_start` hazard. It needs a wreck, so
  a first-jump station without one ends quiet. Two options: mark `hull_breach`
  `safe_start` (it blocks, it doesn't hurt), or add a station-only safe hazard
  (`reactor_leak`: a radiation pocket in the reactor module behind a door).
  Default: `hull_breach` becomes `safe_start`, and `TestSafeStart` (or its
  equivalent) covers station sites. On a first-jump station the generator is
  always repairable from the dock deck's own cargo (the plan places a
  `machine_parts` locker on the dock deck).

### 12. Scenarios that support this map

All four read `starting_floor`, `origin_x/y/z` and `find_open_tile_near`, and
reject `space` tiles. With the dock as `starting_floor` and origins at module
centres, each needs only a check:

| Scenario | Setup script | Anchor it reads | Change |
|---|---|---|---|
| `station_mutants` | `station_setup.basic` | `starting_floor`; origin (the lab) for `experiment_mutants`; `spawn_rules` `hull_floor` with `max_z_delta: -1` (mutants below the landing deck) | With the dock on deck 0, `max_z_delta: -1` would have nowhere to spawn. Put the dock on deck 1 of stations with 4+ decks (reactor and generator below it), or change the rule to `min_z_delta: 1`. **Default: the dock goes on deck 1** and the setup is unchanged. |
| `exterminate_xenos` | `xeno_purge_setup.basic` | `starting_asteroid`, then `starting_floor`; origin ± 20 for eggs | No change; the eggs land in the cryo bay or docked wreck instead of a spoke tip. |
| `autonz_incursion` | `autonz_incursion_setup.basic` | `start_x/y/z` flags; origin ± 12; `find_open_tile(sz)` | Its fallback scatter uses `find_open_tile(sz)` on the landing deck, which now has modules rather than corridors, so it should work. Check that `wire_placer`s on the dock deck stay outside `hazard_keep_out`. |
| `grey_contact` | `grey_contact_setup.basic` | `starting_floor`; origin ± 15; `find_open_tile_any_z` | `salvage_expedition` reuses `station_built`'s origin (now the bridge) through `origin_from`. Make it prefer cargo/hangar with `origin_at: "module:cargo"`. |

`loreUnchangedWithoutHistory` (`lore_grey_test.go`, the mutants test) builds a
station with no history twice, new script against the frozen copy. Both
builds go through the new primer, so it still compares like with like. It
fails if the default plan leaves the old script nowhere to spawn, so run it
at step 4.

### 13. Tutorial and vmtest

- **The tutorial** lands on the Training Ground (internal map, no history) and
  never builds a station. Of the code this plan changes it reaches only
  `landingDistance` (through `ReserveLanding`); it has no history, so no
  `resolveOrigin`, and no wreck, so no `ship_*` scripts. Run `go test ./internal/tutorial ./internal/game -run
  Tutorial` and `make vmtest SCRIPT=tutorial` at every step that touches
  `internal/generation`.
- **No vmtest script lands on a station.** Checked against the scripts' headers
  and `history-dump -find`: `tutorial`, `incident` (Training Ground);
  `new_expedition`, `lore_site`, `storage_beam`, `esc_menu` and `save_load`
  (seeds 777/4242's start system, earth_like); `lore_quiet` (66), `lore_war`
  (125), `lore_hazard` (407) and `lore_ship` (11) on earth_like start systems;
  `lore_autonz` (23 `sys_74`), `lore_xenos` and `lore_legends` (1512
  `sys_10`), `lore_ufo` (1372 `sys_46`) on asteroids; `lore_legends_long`
  (746) reveals the whole chronicle. **Seeds 66, 125 and 407 have station
  sites on their Star Maps** (`sys_27`, `sys_34`/`sys_92`, `sys_21`). Changing
  station arcs changes those sites' histories, and through the shared pool of
  civs it may change which civs appear elsewhere. `lore_legends` asserts on
  chronicle text ("an Ixix ship came down hard on Scorch Expanse"), so it is
  the most fragile. Run the whole suite after steps 3 and 4 and re-find seeds
  with `make history-dump HFIND=...` where a site's story moved.
- `lore_ship` exercises `crashed_ship` and its sections, which step 2 edits. It
  must be unchanged tile for tile when no `z` is passed.

## Order of work

Each step leaves the game playable and the suite green. Stop after any.

### Step 1: Stop the silent failures

- Stamps on a level with no surface get `z` from their origin (as
  `hull_breach` does), or are dropped with a `lore budget:`-style log line and
  listed in `Dropped` (limit `no_surface`), never recorded as placed. Default:
  drop them until step 4 gives them somewhere to go.
- `requires.site` and `present.kind`; tag `meteor_fall`, `buried_cache`,
  `impact`, `impact_cataclysm` and the other surface-only events `planet`, and
  drop `civ_rises_on_ruins` from `lost_station`.
- `history-dump -stats` gains a station section: origins by deck, share of
  origins inside a module (0 today), stamps placed vs recorded.
- **Files:** `internal/generation/feature_placers.go`,
  `internal/history/data.go`, `internal/history/generate.go`,
  `internal/history/render.go`, `internal/mapdef/types.go`,
  `data/maps/abandoned_station.json`, `data/history/events.json`,
  `data/history/arcs.json`, `cmd/history-dump/main.go`.
- **Done:** no station chronicle has a meteor scar or buried cache;
  `StampFootprints` on a station holds only stamps that painted something.
- **Verify:** new `internal/generation` test that a stamp on an all-space
  level is not recorded; `internal/history` test that no `site: planet` event
  appears on 500 station sites; `make history-dump HSTATS=500
  HMAP=abandoned_station` shows "stamps placed = recorded" and station tiers
  unchanged within ±2.

### Step 2: Module scripts

- `z` param on the eight `ship_*` sections and `ruin_pass`
  (`sz = get_param("z")`, falling back to `get_surface_z` when the param is
  empty, as `hull_breach` falls back to `origin_z`), plus `frame=station` on
  the seven reused sections.
- New `station_dock`, `station_generator`, `station_lab`, `station_botany`,
  `station_archive`, `station_observatory`, `station_relay`,
  `station_refinery` and `station_medbay`. New blueprints `station_light`,
  `station_generator_broken`/`station_generator` (inert until step 7) and the
  `station_keycard` item.
- Structure-viewer presets: every module at its slot sizes, damage 0–3.
- **Files:** `data/scripts/structures/ship_*.basic`, `ruin_pass.basic`, new
  `station_*.basic`, `data/blueprints/structures/furniture.json`,
  `data/blueprints/items/*.json`, `data/history/structure_presets.json`,
  `data/history/loot.json` (`bridge_keys`), `docs/developer/lore.md`
  structure table.
- **Done:** `stamp station_lab 15 11 z=0` in the cheat console builds a lab;
  every module stays in its rect on its deck at damage 0–3.
- **Verify:** `internal/game/content_*_test.go` (footprint, no lone walls,
  < 100 ms, `occupants` declared) over the new scripts;
  `TestContentPresetsMatchData` and `TestContentPresetsBuild`;
  `crashed_ship` with no `z` is unchanged tile for tile (frozen copy);
  `make vmtest SCRIPT=lore_ship`.

### Step 3: Purposes and the station plan

- `station_purposes` in `purposes.json`; `present.station_purposes` and
  `@station_purpose`; `module` evidence kind, `on`, `origin_at:
  "module:<kind>"`, `station_damage`; new events `station_expanded`,
  `reactor_failure` and `station_breached`; `History.Evidence` builds
  `SiteEvidence.Station` (purpose, frame, decks, modules with event, damage,
  sealed, occupants) and `OriginModules`.
- `history-dump` prints each station site's plan (purpose, modules per deck).
- **Files:** `data/history/purposes.json`, `events.json`, `arcs.json`;
  `internal/history/data.go`, `render.go`, `generate.go`, `cmd/history-dump`.
- **Done:** every station site has a plan, and every origin event that names a
  module is in it.
- **Verify:** `internal/history` tests: plan deterministic per seed; every
  purpose's `modules` present; a module asked for twice is built once;
  `drone_station` is always `relay` with no quarters; `lore_*` reference tests
  (new events' templates, `module` kind known); `HSTATS=500` purpose shares
  within ±5 of the weights; tier shares and chapter floor (section 11); the
  whole vmtest suite (histories moved).

### Step 4: The modular primer

- New `primer_station_modules.go`, registered as `station`: frame by shape,
  slots, allocation by deck band and keep-out hops, module scripts through
  the structure runner with `z`, region tags (section 2), `Level.Modules`
  (saved), stair columns, a ceiling pass with no stray plates, dock on deck 1
  for 4+ decks. Without `station_plan` it rolls a default plan from its seed.
- Map size and margin (section 9); `botany_bay` and `radiation_pocket`
  features removed from the map (now in modules).
- The old `abandoned_station` primer stays registered until step 9, then goes.
- **Files:** `internal/generation/primer_station_modules.go` (new),
  `internal/world` (`Level.Modules` and save), `internal/game` (pass
  `SiteEvidence.Station` into `TerrainParams`),
  `data/maps/abandoned_station.json`, `cmd/planet-generator` (draw module rects
  and names in the overlay).
- **Done:** `go run ./cmd/planet-generator -history -map abandoned_station
  -seed N` shows a spine or ring station with named modules on every deck.
  Decks differ, both stair columns connect every deck, the dock is the
  landing.
- **Verify:** `internal/generation` tests with a stub runner: determinism;
  every module inside the map margin; every module reachable from the dock on
  foot across decks (flood fill through doors and stairs, ignoring locks);
  no `hull_floor` plate outside a deck's hull; chapter modules at ≥ 2 hops
  from the dock. `internal/game` test with real scripts over 50 seeds: every
  plan module has a named room. `lore_mutants_test`, `lore_grey_test`
  (`loreUnchangedWithoutHistory`), `lore_occupants_test` (update the station
  exception: stamps now place). Planet-generator seeds 1, 3, 7, 42, 123 and
  777 by eye (look at the frames headlessly with the `playtest` skill).

### Step 5: Origins anchor to modules

- Station path in `ResolveOriginsKeepOut` (section 7), `deck_cost` in
  `landingDistance` on station maps, the docked wreck at the arm.
- **Files:** `internal/generation/origins.go`, `landing.go`,
  `feature_placers.go` (anchored features read the origin's z),
  `internal/history/render.go` (wreck at the arm).
- **Done:** each event's datapad and evidence stands in its module, on that
  module's deck.
- **Verify:** `internal/generation/lore_origins_test.go`: a module-bound origin
  resolves inside its module's rect on its z; unbound origins spread over
  decks; the planet path is unchanged (existing tests). Rerun the step-1 probe:
  share of origins on deck 0 falls from 100% to roughly the dock deck's share
  of modules, share inside a module rises from 0 to 100%. `HSTATS` station
  section. `make vmtest SCRIPT=lore_hazard` and `lore_xenos` (planet keep-outs
  unchanged).

### Step 6: Decay

- Per-module and per-deck damage from `station_damage`, the chapter tier and
  the status base; `breach`, `nanite`, `meltdown` and `abandoned` kinds; the
  drone relay intact.
- **Files:** `internal/history/render.go` (damage into the plan), the
  primer, `ruin_pass.basic`, the module scripts, `hull_breach.basic` (into a
  module rect).
- **Done:** a `nanite_conversion` station has wire on one or two decks; a
  `reactor_failure` station's reactor deck is rubble round a hot core; a
  drone relay has every wall standing.
- **Verify:** `internal/game` test: for fixed seeds per ending, wall loss per
  deck is ordered as the plan says (damage-0 decks lose none);
  `TestContentPresetsBuild` at damage 3; planet-generator seeds for each
  ending (found with `HFIND=abandoned_station,reactor_failure` etc.).

### Step 7: Power, doors and goals

- `Powered`, `Repairable`, `StationPowerSystem`, `KeyDoorSystem`, the Repair
  inspector button, `lock_door` and `spawn_station_log` builtins, the
  `station_log` datapad; quest templates `station_power` and
  `station_archive`; the `station` location tag; `station_setup.basic` and
  the base scenario's description; the `station_purpose` lore flag.
- **Files:** `internal/components` (new components),
  `internal/factory/component_registry.go`, `internal/systems` (two new
  systems), `internal/gui/hud_screen.go` (Repair), `internal/workerai`
  (repair completion), `internal/game/setup_script.go` (builtins, flag),
  `internal/campaign/generate.go` (tag), `data/quest_templates.json`,
  `data/scenarios/abandoned_station.json`,
  `data/scripts/scenarios/station_setup.basic`, blueprints.
- **Done:** on a quiet station, colonists haul parts to the generator and
  repair it; lights come on and powered doors unlock; the keycard from the
  bridge opens the archive; the log console reveals the chronicle.
- **Verify:** `internal/systems` tests (lights off until the flag, on after;
  powered door unlocks; key door opens only for the key); `internal/game`
  test (repair request completes, flag set, `structure_built` fires,
  `station_power` quest completes); save/load keeps lock and power state;
  `make vmtest SCRIPT=save_load`.

### Step 8: The four scenarios

- `station_mutants` (dock on deck 1 or the spawn rule), `grey_contact`
  (`origin_at` cargo), and checks on `exterminate_xenos` and
  `autonz_incursion` (section 12).
- **Files:** `data/scenarios/station_mutants.json`,
  `data/history/events.json`, the four setup scripts if needed.
- **Done:** each scenario on a new station puts its threat in the chapter's
  module, outside the hazard keep-out, on a non-space tile.
- **Verify:** `lore_mutants_test`, `lore_grey_test`, a new
  `lore_station_scenarios_test` over 20 seeds per scenario (threat count > 0,
  none within keep-out of the dock, none on `space`); `HSTATS` scenario share
  per map within ±3 of today's.

### Step 9: vmtest, docs, retire the old primer

- New `vmtest/scripts/lore_station.jsonl`: New Expedition with a seed whose
  Star Map has a quiet station (re-find after step 3 with `make history-dump
  HFIND=quiet,abandoned_station,reactor_built`; today seed 20 `sys_82`) →
  jump → beam one down → land at the dock → assert the room name at the dock
  → `debug module walk generator` (new debug command beside `debug origin
  walk`) → assert room "Main Generator" and a `station_generator_broken` in
  view → screenshot each deck from the main column. Lighting off so the whole
  module shows, as `lore_ship` does.
- Remove the old `abandoned_station` primer and `TagRegion("station_hub")`
  callers that only it served.
- **Files:** `vmtest/scripts/lore_station.jsonl`, `docs/developer/vmtest.md`
  (script table), `internal/game` (the debug command),
  `docs/developer/lore.md` (station section, module table, lore flags),
  `docs/developer/structure_scripts.md` (`z` param), `docs/developer/scenarios.md`,
  `internal/generation/primer_station.go` (removed).
- **Done:** `make vmtest SCRIPT=lore_station LABEL=station` passes and its
  frames show a named, furnished, dark station.
- **Verify:** the whole vmtest suite; `go test ./...`.

**Parallel work.** Step 2 (scripts and blueprints only) can run alongside
step 1. Steps 3 and 4 both touch `internal/history/render.go` and the map, so
they run one after another. Step 7's engine side (components, systems,
inspector) touches no generation code and can start after step 2 in its own
worktree, joining at its data half after step 4.

## Where the owner rewrites text

Room names and the `station_purposes` clue lines (`purposes.json`); the new
events' `text` and the station `summary`s (`arcs.json`); `station_setup.basic`
messages; the base scenario's name and description; quest template names and
descriptions; blueprint names (`station_light`, the generator, the keycard,
the log console). All marked `"_draft"` as the existing files are.

## Open questions (defaults chosen)

- **Primer or history builds the modules.** Default (A): the primer composes
  them from a history-written plan (section 1). Say if you'd rather the
  history stamp modules into an empty frame.
- **Sealed section key.** Default: a keycard item from the bridge. The
  alternative is a research tech ("Station Systems") that unlocks every
  sealed door, or both (keycard for the archive, research for the armoury).
- **Station log.** Default: one log console that reveals the whole station
  chronicle when read. Alternative: a stack of ordinary per-event datapads in
  the bridge. That is simpler, but duplicates the datapads at the modules.
- **Dock deck.** Default: deck 1 on stations with 4+ decks, so
  `station_mutants`' "mutants below you" rule still has decks below.
  Alternatively keep the dock on deck 0 and flip the rule to "above".
- **Map size.** Default: 100–140 × 80–120 with a 14-tile margin. Smaller maps
  mean a smaller station; say if you want stations to read as large as the
  planet maps.
- **Observatory windows.** Default: a new `viewport` tile (solid, transparent,
  shows space). The cheap alternative is plain walls.
- **First-jump stations.** Default: `hull_breach` becomes `safe_start`, so
  first-jump stations aren't always quiet when there is no wreck.
- **Purpose weights.** Default 15–20% each, drone stations always `relay`.
  Should a `lost_station` also be a relay sometimes (crewed relay)?
- **Restored power reward.** Default: lights, doors, a working `research_lab`
  in the lab module. Should power also raise ambient light, or run the
  station's `fuel_refiner`?
<<<<<<< HEAD
=======

## As built (2026-11-07)

Every open question took its default except where noted. Commits: step 1
`fd886f5`, 2 `4cb5c99`, 3 `059aa4b`, 4 `d88662f`, 5 `c2538dc`, 6 `7af1642`,
7 `71fc039`, 8 `d46917d`, 9 the last commit on the branch.

**Numbers** (`make history-dump HSTATS=500 HMAP=abandoned_station`, all
steps in):

- Tiers 31/51/19 at 500 seeds; 34/46/20 at 2000. The 500-seed set swings
  about ±3 by sampling alone (2000 seeds sit on the 35/45/20 target), so the
  ±2/±3 checks were judged at 2000.
- Purposes: lost stations garrison 15%, habitat 20%, refinery 17%, relay
  14%, research 19%, shipyard 15% (weights 15/20/15/15/15/20); drone
  stations relay 100%; all station sites relay 35%, the rest 11-15%.
- Origins by deck z0 9%, z1 14%, z2 40%, z3 28%, z4 9%; inside a module
  2369 of 2369 (100%; 0% before, all on deck 0).
- Stamps 39 kept in evidence, 39 recorded, 39 painted (`hull_breach`);
  docked wrecks 326 in the plans, 326 built. Before step 1, 1.1 stamps per
  site were recorded and only `hull_breach` painted.
- Every station chapter above 3% (lowest `wreck_eggs_hatch` 3.0% at 500,
  4.0% at 2000; `wreck_nanites_wake` 3.2% / 3.6%).
- Scenario shares on the station map (500 seeds): base 69.4%, autonz 8.6%,
  xenos 7.6%, grey 7.6%, mutants 6.8% (before: 68/9/9/9/5).

**Planet-generator seeds** (`go run ./cmd/planet-generator -history -map
abandoned_station -seed N`, or `-png <dir>` headless): purposes: relay 7
and 42 (spine, 3 decks), refinery 1 (spine, 4), habitat 3 (ring, 5), 5
(spine) and 777 (ring, 5), garrison 14 (cross) and 123 (ring, 4), research
12 (ring, 4), shipyard 38 (spine). Endings:
reactor_failure 1, station_lost 3, colony_abandoned 4, station_breached 6,
nanite_conversion 12, colony_withdrawn 19, drone_station_fails 48 (quiet).
`TestStationDecayFollowsTheEnding` logs the first seed of each ending.

**Step 1.** As planned. A stamp counts as placed only if it changed a tile
or added an entity in its footprint (`footprintSignature`), whatever the
reason it built nothing. Two tests that used do-nothing stub runners now
paint a tile. Planet-only events: `impact`, `impact_cataclysm`,
`famine_year`, `migration`, `raid`, `quake`, `herds_migrate`, `meteor_fall`,
`buried_cache`, `civ_rises_on_ruins`. history-dump's `-stats` now honours
`-map` and builds every station site (about 7 s for 500).

**Step 2.** The modules keep their centre row and column clear (the frame
opens doors at a wall's centre); ship sections don't, so the primer clears
the way through every door after the script. New builtin `has_param` (a
numeric `z` of 0 could not be told from unset). `station_console` is a new
inert blueprint for consoles and racks. Observatory windows are plain hull:
the tile set has no see-through solid tile, so the `viewport` default would
only have been a renamed wall with no art (the plan's cheap alternative).
Ship sections mirror on a station (`frame=station`), so their presets vary
by seed as `TestContentPresetsBuild` requires.

**Step 3.** `station_purposes` gained `arcs`, `sealed` and `event_weights`
(the plan's "reactor-failure endings likelier" and "docked wreck likelier"
biases are event weights within an arc stage). `station_experiment` builds
its lab (module evidence) instead of requiring one: gating it starved the
threat tier. A crewed relay gets quarters. `archive_sealed` lost "below
the surface" from its text. The `station_purpose` lore flag landed here.

**Step 4.** Ordinary module doorways are open, not airlocks: a door with
no `OwnedBy` costs 1000 to path through for every faction and a
colony-owned one shuts creatures out, so doors stand only where they mean
something (sealed modules, the bridge, the dock's airlock pair). Spare slots
hold `station_bay` rooms (maintenance, storeroom, mess, office) so decks
read as a station. There is no ceiling pass at all (the old one laid the
stray plates). The old primer's stairs put `stairs_up` over `stairs_down`
on middle decks; each column now alternates two tiles. Modules that don't
fit move to their smallest size, then to another deck. A cross whose map
is too short for its vertical arm lays a spine. The planet generator gained
`-png` (per-deck PNGs, a montage and an index of modules and origins), the
way stations were looked at headlessly.

**Step 5.** The docked wreck berths at the far end of a deck away from the
dock (the top deck down, the dock's deck last) with its own gangway, not at
the end of the dock's arm: at the arm it would sit inside the landing's
keep-out, so the eggs, swarms and leaks that grow from a wreck would spawn
on the landing or be refused. A colony hull docks as a warship. On a cross the berth keeps clear of the
vertical arm; where it cannot, the wreck's deck lays no arm (found in step
9 on seed 14). Unbound
origins take their deck from their rolled tile (`(x + y) % decks`), so they
spread. Anchored stamps on a station build into their module's rect.

**Step 6.** `nanite_conversion` binds to a module (`origin_at:
"module:@any"`) so its deck is known before the primer runs; with the
plan's `spread: 1` the wire covers its deck and one either side, so up to
three decks (the plan's "one or two" holds only at the station's top or
bottom). Its `wire_placer` pods are anchored at the origin on stations.
Bays take their deck's and the station's damage too. `hull_breach` is
`safe_start`.

**Step 7.** The Repair order is a click on the dead generator (as a click
on a bed orders sleep), not an inspector button. Its cost lives in a hidden
`station_generator` buildable (4 machine parts, 5 fuel) rather than on
`Repairable`. Colonists only fetch from colony-owned storage, so
`station_setup.basic` claims the dock's lockers (`claim_module_storage`)
and the dock's spares locker uses a new `station_spares` table that always
covers the repair. The keycard lies on the bridge floor (else the
generator's) and counts once a colonist carries it or it is in colony
storage. The bridge door stays locked until power, so the order is power,
bridge, keycard, archive. Station quests are attached per station site from
its own data (`addStationQuests`; `station_archive` only with a sealed
archive): putting them in the tag-rolled pool moved every later campaign
roll (`TestLoreHistoryKeepsSeedsMapsPositionsNames`), so there is no
`station` location tag.

**Step 8.** `station_mutants` and `grey_contact` stations get at least four
decks (their creatures spawn below the landing deck). The autonz setup
script never started wire placers on space. The scenario test checks that
no threat stands on the dock's deck within the hazard keep-out; threats on
other decks are judged by deck, not distance.

**Step 9.** `debug module [walk] <id>` and `debug deck <z>`, and state
fields `view_room`, `view_module`, `in_view`. `lore_station` uses seed 114
(`sys_57`, fuel 10); seed 20 `sys_82` no longer has a quiet station.

>>>>>>> refs/heads/claude/station-5b3f90
