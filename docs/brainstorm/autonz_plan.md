# The Autonz as a Full Settlement Faction: Implementation Plan

Status (2026-11-08): **built** (see "As built" at the end); the open
questions were settled (the owner took every default). It is a new phase on the factions
branch (`claude/factions-a-a379f7`), **Phase E3**. It comes after the hive
phase ([hive_plan.md](hive_plan.md), E2) and before F (playable races). It
was written against the branch after `5ddf940`, with E2 still a plan, so
it names E2's mechanisms as E2 plans them. If E2 lands differently, this
plan follows what E2 built. All draft text (unit, structure and research
names, alerts, Legends lines) is placeholder that the owner rewrites (see
"Where the owner rewrites text").

Read with: [hive_plan.md](hive_plan.md) (types, the evolve tree, `stage_min`,
the stage's `structures`, guard, patrol and launch rungs, `required_floor`,
recipe `required_tech`, structures by stage in materialise),
[enemy_factions_plan.md](enemy_factions_plan.md) ("Phase E as built") and
[factions.md](../developer/factions.md) ("How a faction moves and acts",
"`data/factions.json`", "The player's doctrine", "The director", "Faction
settlements", "ConvertAction", "Raids and hunts", "The autonz").

This plan is shorter than the hive's because the mechanisms already exist.
Phase E built conversion, signals, sleepers and the staged camp, and E2
builds types, trees, structures by stage and the new rungs. What is left
is the **roster, the tree, the buildables and the sheet**.

## Decision

The owner's direction (2026-11-08), taken as given:

- **The sheet's two palettes are tiers.** Steel is standard and bronze is
  upgraded. Both use the existing tier-at-spawn mechanism: the colour
  block is the tier, and the tier multiplies stats. The autonz have no
  hidden identity, so the tier is openly visible.
- **The roster comes from the sheet's silhouettes** (see "The sheet").
- **An assimilation tech tree**, under an autonz key in `research.json`.
  It unlocks units, the bronze tier per unit, the conversion structures,
  signal range and conversion rate (the four existing entries fold in). It
  is researched at a structure the autonz build (the cogitator), not at a
  lab.
- **The director builds structures in place**, on converted ground: relay
  pylon, converter node, foundry, cogitator and defence pylon. Stage stamps
  stay, for materialising a presence that grew while unloaded, so the two
  must agree.
- **Conversion stays the economy.** Ore is the feedstock, and the signal
  trigger stays as built. Phase A and Phase E's spread pace is kept
  (`TestFactionPaceRivalOne`, the Phase E pace table under
  `make history-dump HFACTIONS=4`).
- **Everything is data**: `data/factions.json`, `research.json`,
  blueprints and structure scripts. A playable autonz in F needs no new
  code path. The player's side is described ("Phase F: what the player will
  see") but not built.

Constraints carried over: physical storage (workers haul everything and
nothing teleports), scenarios stay random, the owner commits.

## The sheet

`scifi_entities` (`assets/entities/oryx_16bit_scifi_creatures.png`). Each
cell is 24 px. **Steel is row y=936 and bronze is row y=960**, with the
same x positions in both. So bronze is **+24 in y**, not a colour block in
x as the hive's colours were. Measured on the sheet for this plan:

| x | Silhouette | Frames | Use | Kind |
|---|---|---|---|---|
| 0, 24 | walker with antenna, red eye | pair | **relay drone** | unit |
| 48, 72 | walker with a split claw head | pair | **wire placer** | unit |
| 96, 120 | sphere on legs, red eye | pair | **seeker** | unit |
| 144, 168 | tripod, dark visor, no red | pair | **powered-down enforcer** (the sleeper hazard's derelict; spare otherwise) | look only |
| 192 | tripod, small red bar | single | **enforcer** | unit |
| 216 | tripod, heavy red bar | single | **warden** | unit |
| 240, 264 | orb, red eye | pair | **sentinel** (flying) | unit |
| 288, 312 | orb, green eye | pair | **harvester** (flying) | unit |
| 336, 360 | small cube | pair | **ferrite**, the printer's refined feedstock and the autonz' wreck drop | item |
| 384, 408 | large cube, red slit | pair | **core block**: the Signal Core and the dormant sleeper node | structure |

Notes:

- **Today's sprites do not match the owner's roster.** `wire_placer` and
  `wire_placer_crew` are drawn at (0, 936), the antenna walker. `enforcer`,
  `enforcer_crew` and the sleeper are drawn at (240, 936), the red orb.
  Step 1 moves the wire placer to x=48 and the enforcer to x=192 in both
  the scenario blueprints (`autonz.json`) and the crew blueprints
  (`autonz_crew.json`), so a wild incursion and a settlement look the same.
  The `autonz_incursion` frames change (open question 1).
- **Pairs are animation frames** where the engine animates (`Bounces` is
  the only motion today). Without frame animation, the second frame is
  unused. 192 and 216 differ only in the bar, and are read as two units, as
  the owner listed them.
- The **tripod with no red** (144) is the unlit enforcer. It is the obvious
  look for the hazard's derelict enforcer (`autonz_sleeper` evidence) and
  for an enforcer folded down. It is not a ninth unit.

## Where the code is today

- **Doctrine** (`data/factions.json` `autonz`): `economy: convert`,
  `mobility.kind: signal`, `conversion` (blue_wires, radius 16, 0.006 a
  day, cap 0.85, floor 0.35), stages `node` (pop 0, stamp `autonz_node`
  9×9) and `hub` (pop 7 or share 0.5, `autonz_hub` 15×15, `signal`). The
  Tier 1 `research` walk is `signal_range_1` 30 (+10), `conversion_rate_1`
  40, `signal_range_2` 60 (+12) and `conversion_rate_2` 80, at
  `research_per_day` 1. `level.sleeper` is `autonz_sleeper_node`.
- **Settlement block**: crew `wire_placer_crew` (role `converter`) and
  `enforcer_crew` (role `guard`, laser pistol); `needs: false`; buildables
  `storage_locker`; `research: []`; units `print_wire_placer` (4 metal_ore,
  120 ticks) and `print_enforcer` (8 metal_ore, 1 crystal, 160 ticks), both
  at `autonz_fabricator` with `deploy_on_craft`. The camp is `staged`
  (station `autonz_fabricator`). The director runs grow, convert (40 + 60
  per crew member), mine (metal 40, crystal 10), a locker, dig (depth 6,
  reach 80) and raid (guards, three).
- **Stamps** `data/scripts/structures/autonz_node.basic` and
  `autonz_hub.basic`. The Signal Core is red wire tiles, not an entity.
- **Live** (`internal/game/faction_autonz.go`, `stepLiveAutonz`): daily
  record from the level, stage stamp, `LiveSignal` (the signal's two bodies
  taken off the level), `WakePresence`.
- **Not done in E**: autonz researching on a level (the plan's "as built"
  lists it). No autonz unit has a tier, and `ApplyTier` only shifts x.
- **The soak** `TestAutonzSoak` (`internal/game/autonz_soak_test.go`): a
  node of 4 with 20 metal and 4 crystal, neutral, 20,000 ticks, in
  `make soak`.
- **What E2 adds and this phase reuses**: `types` with `ApplyType` and
  `Kin.Type`; `TypesUnlocked`; recipe `kin_type` and `required_tech`;
  research entries with `tree` and an effect block; the build rung's
  `stage_min` and the stage's `structures` cap; `required_floor`; the
  `guard`, `patrol` (contact, `Site.Scouted`) and `launch` rungs; raid
  `scouted`; `FactionPresence.Units`; structures by stage in materialise; a
  natural ranged weapon item (the spit gland), flagged not to drop or be
  looted; `find_nearest_worker` skipping the caller's faction; the
  GameMaster counting `Kin` workers toward `hostile_max`.

## Design

### Tiers on types

The autonz use E2's `types` (keyed by unit) and a **`tiers`** list, which
is per type rather than per presence:

```json
"tiers": [
  { "id": "steel" },
  { "id": "bronze", "sprite_dy": 24, "health": 1.5, "str": 1.25 }
]
```

- A type's tier is bronze once that type's bronze entry is known
  (`Doctrine.TierOfType(type, known_techs)`, read from the tree's
  `assimilate.tier` effect). Otherwise it is steel.
- `factory.ApplyType` takes the tier and applies it **at spawn**, which is
  the existing mechanism with two changes. The tier offset gains
  `sprite_dy` beside `sprite_dx`. `Kin.Tier` is kept, or put back if E2
  removed it, as the per-type tier index (0 steel, 1 bronze).
- **Prints only.** A unit printed after its bronze entry is known comes out
  bronze. Steel units already standing stay steel until they are
  destroyed, so a mixed army tells the player how recently the autonz
  assimilated (open question 3). The multipliers are the old hive tier
  multipliers (health 1.5, str 1.25), so a bronze enforcer is to a steel
  one what a Phase B red crawler was to an olive one.
- Wild scenario and GameMaster autonz have no presence, so they are always
  steel.

### The unit table

Benchmarks: today's wire placer is HP 8, Str 6, Dex 8, AC 4, 1d4,
Initiative 10. Today's enforcer is HP 12, Str 8, Dex 12, AC 8, 1d6,
Initiative 12, with a laser pistol. Both stay as they are.

| Type | Sprite x (steel y=936, bronze y=960) | Station | Cost | Print ticks | HP | Str | Dex | AC | Attack | Init | Kit / traits | Role, allowed tasks | Unlocked by | Bronze by |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| wire placer | 48 | fabricator | 4 metal_ore | 120 | 8 | 6 | 8 | 4 | 1d4 | 10 | — | `converter`: convert, mine, dig, haul, build, craft, research | start | `autonz_bronze_placer` |
| enforcer | 192 | fabricator | 8 metal_ore, 1 crystal | 160 | 12 | 8 | 12 | 8 | 1d6 | 12 | `laser_pistol` | `guard`: attack | start | `autonz_bronze_enforcer` |
| relay drone | 0 | fabricator | 4 metal_ore, 2 crystal | 120 | 8 | 4 | 10 | 4 | 1d4 | 8 | `deploy` → `relay_pylon` | `relay`: none (patrol and deploy orders only) | `autonz_relay_pattern` | `autonz_bronze_relay` |
| seeker | 96 | fabricator | 5 metal_ore, 1 crystal | 120 | 10 | 6 | 16 | 8 | 1d6 | 6 | — | `seeker`: attack (patrols, finds the colony, harasses) | `autonz_seeker_pattern` | `autonz_bronze_seeker` |
| harvester | 288 | foundry | 4 metal_ore, 2 ferrite | 160 | 10 | 6 | 8 | 6 | 1d4 | 10 | `flying` | `harvester`: mine, dig, haul | `autonz_foundry_pattern` | `autonz_bronze_harvester` |
| sentinel | 240 | foundry | 6 metal_ore, 1 crystal, 3 ferrite | 200 | 14 | 6 | 14 | 10 | `sentinel_emitter` 1d8, range 6 | 10 | `flying`, `sentinel_emitter` | `sentry`: attack (holds a defence pylon) | `autonz_sentinel_pattern` | `autonz_bronze_sentinel` |
| warden | 216 | foundry | 10 metal_ore, 2 crystal, 4 ferrite | 240 | 30 | 14 | 10 | 12 | 2d6 | 14 | `laser_pistol` | `warden`: attack (heavy guard, leads raids) | `autonz_warden_pattern` | `autonz_bronze_warden` |

- A **warden is a hive warrior with a gun**: as tough as today's wild
  stage-3 xeno, and slow. A **seeker is the fast cheap threat** a player
  meets first. A **sentinel is a flying turret**: it posts at a defence
  pylon and shoots over walls. The `sentinel_emitter` takes the spit
  gland's path (an inventory weapon flagged not to drop or be looted), so
  ranged combat needs no new code.
- A **harvester** takes the ore work off the wire placers, which then
  convert. "Repairs" from the owner's list needs a repair task, which does
  not exist (open question 5).
- **Relay drone deploy**: the type carries
  `"deploy": { "as": "relay_pylon", "ticks": 120, "floor": ["blue_wires", "red_wires"] }`.
  A deploy order walks the drone to a wired tile and works there for
  `ticks`. Then **the drone is replaced by the pylon** on that tile: the
  same replace-in-place as E2's `consume_station`, with the matter
  conserved. This is the one new order kind in this phase
  (`Orders.Deploy`; the player's equivalent in F is a button on the unit).
- Crew blueprints: one per unit in `autonz_crew.json`
  (`relay_drone_crew`, `seeker_crew`, `harvester_crew`, `sentinel_crew`,
  `warden_crew`, beside the two that exist). There is no shared body as
  the hive has, because each autonz unit is its own silhouette. Each has
  `Worker`, `Kin`, no script and no `Needs`. `ApplyType` sets `Role`,
  `AllowedTasks` and stats from the type, so the blueprint carries only the
  look.

### Types in `data/factions.json`

```json
"types": {
  "wire_placer": { "blueprint": "wire_placer_crew", "name": "Wire Placer", "cost": { "metal_ore": 4 },
                   "health": 8,  "str": 6,  "dex": 8,  "ac": 4,  "attack": "1d4", "initiative": 10,
                   "role": "converter", "tasks": ["convert", "mine", "dig", "haul", "build", "craft", "research"] },
  "enforcer":    { "blueprint": "enforcer_crew", "name": "Enforcer", "cost": { "metal_ore": 8, "crystal": 1 },
                   "health": 12, "str": 8,  "dex": 12, "ac": 8,  "attack": "1d6", "initiative": 12,
                   "equip": ["laser_pistol"], "role": "guard", "tasks": ["attack"] },
  "relay_drone": { "blueprint": "relay_drone_crew", "name": "Relay Drone", "unlock": "autonz_relay_pattern",
                   "cost": { "metal_ore": 4, "crystal": 2 },
                   "health": 8,  "str": 4,  "dex": 10, "ac": 4,  "attack": "1d4", "initiative": 8,
                   "role": "relay", "tasks": [],
                   "deploy": { "as": "relay_pylon", "ticks": 120, "floor": ["blue_wires", "red_wires"] } },
  "seeker":      { "blueprint": "seeker_crew", "name": "Seeker", "unlock": "autonz_seeker_pattern",
                   "cost": { "metal_ore": 5, "crystal": 1 },
                   "health": 10, "str": 6,  "dex": 16, "ac": 8,  "attack": "1d6", "initiative": 6,
                   "role": "seeker", "tasks": ["attack"] },
  "harvester":   { "blueprint": "harvester_crew", "name": "Harvester", "unlock": "autonz_foundry_pattern",
                   "cost": { "metal_ore": 4, "ferrite": 2 },
                   "health": 10, "str": 6,  "dex": 8,  "ac": 6,  "attack": "1d4", "initiative": 10,
                   "flying": true, "role": "harvester", "tasks": ["mine", "dig", "haul"] },
  "sentinel":    { "blueprint": "sentinel_crew", "name": "Sentinel", "unlock": "autonz_sentinel_pattern",
                   "cost": { "metal_ore": 6, "crystal": 1, "ferrite": 3 },
                   "health": 14, "str": 6,  "dex": 14, "ac": 10, "attack": "1d4", "initiative": 10,
                   "flying": true, "equip": ["sentinel_emitter"], "role": "sentry", "tasks": ["attack"] },
  "warden":      { "blueprint": "warden_crew", "name": "Warden", "unlock": "autonz_warden_pattern",
                   "cost": { "metal_ore": 10, "crystal": 2, "ferrite": 4 },
                   "health": 30, "str": 14, "dex": 10, "ac": 12, "attack": "2d6", "initiative": 14,
                   "equip": ["laser_pistol"], "role": "warden", "tasks": ["attack"] }
},
"type_fallback": "wire_placer",
"stages": [
  { "pop": 0, "name": "node", "stamp": "autonz_node", "w": 9, "h": 9,
    "mix": { "wire_placer": 0.6, "enforcer": 0.2, "relay_drone": 0.1, "seeker": 0.1 },
    "structures": { "cogitator": 1 } },
  { "pop": 7, "share": 0.5, "name": "hub", "stamp": "autonz_hub", "w": 15, "h": 15, "signal": true,
    "mix": { "wire_placer": 0.35, "harvester": 0.15, "enforcer": 0.15, "seeker": 0.1,
             "relay_drone": 0.05, "sentinel": 0.1, "warden": 0.1 },
    "structures": { "cogitator": 1, "signal_core": 1, "relay_pylon": 1, "foundry": 1,
                    "converter_node": 2, "defence_pylon": 2 } }
]
```

- Locked types' shares go to `type_fallback`. The autonz have no morphs,
  so E2's `morph_fallback` (by morph) becomes a single key here, read by
  the same `presenceComposition`. On a fresh node everything locked falls
  back to the wire placer. So a Phase E node, 0.75 / 0.25, becomes
  0.8 wire placers / 0.2 enforcers until relay and seeker patterns are
  known. The stage `mix` is retuned (step 1) so a node with nothing
  unlocked is exactly 0.75 / 0.25. **Open question 2** gives the exact
  numbers.
- A stage's `structures` lists only what the stage may hold. Materialise
  and the director place an entry only once its unlocking entry is known
  (`buildable.required_tech`), so a hub without the foundry pattern has no
  foundry.
- `name` is the only name. There is no `seen_name`, because there is no
  hidden identity.

### Recipes

Every print is `deploy_on_craft` at its station. The tier comes from the
known techs at spawn, so there is **one recipe per unit, not one per
tier**. Bronze costs nothing extra per print, because the research is the
price (open question 4).

```json
"print_wire_placer": { "...": "exists; kin_type added", "kin_type": "wire_placer" },
"print_enforcer":    { "...": "exists; kin_type added", "kin_type": "enforcer" },
"print_relay_drone": { "name": "Print Relay Drone", "description": "_draft", "output": "relay_drone_crew", "kin_type": "relay_drone",
                       "cost": { "metal_ore": 4, "crystal": 2 }, "build_time": 120, "station": "autonz_fabricator",
                       "deploy_on_craft": true, "required_tech": "autonz_relay_pattern" },
"print_seeker":      { "name": "Print Seeker", "description": "_draft", "output": "seeker_crew", "kin_type": "seeker",
                       "cost": { "metal_ore": 5, "crystal": 1 }, "build_time": 120, "station": "autonz_fabricator",
                       "deploy_on_craft": true, "required_tech": "autonz_seeker_pattern" },
"print_harvester":   { "name": "Print Harvester", "description": "_draft", "output": "harvester_crew", "kin_type": "harvester",
                       "cost": { "metal_ore": 4, "ferrite": 2 }, "build_time": 160, "station": "autonz_foundry",
                       "deploy_on_craft": true, "required_tech": "autonz_foundry_pattern" },
"print_sentinel":    { "name": "Print Sentinel", "description": "_draft", "output": "sentinel_crew", "kin_type": "sentinel",
                       "cost": { "metal_ore": 6, "crystal": 1, "ferrite": 3 }, "build_time": 200, "station": "autonz_foundry",
                       "deploy_on_craft": true, "required_tech": "autonz_sentinel_pattern" },
"print_warden":      { "name": "Print Warden", "description": "_draft", "output": "warden_crew", "kin_type": "warden",
                       "cost": { "metal_ore": 10, "crystal": 2, "ferrite": 4 }, "build_time": 240, "station": "autonz_foundry",
                       "deploy_on_craft": true, "required_tech": "autonz_warden_pattern" },
"refine_ferrite":    { "name": "Refine Ferrite", "description": "_draft", "output": "ferrite", "count": 2,
                       "cost": { "metal_ore": 4, "crystal": 1 }, "build_time": 90, "station": "autonz_foundry",
                       "required_tech": "autonz_foundry_pattern" }
```

**Ferrite** (`data/blueprints/items/resources.json`): sprite (336, 936) on
`scifi_entities`, tags `metal`, `salvage`, `raw_material`, `ferrite`, stack
100. It comes from two places. The foundry refines it (above). And a
destroyed autonz unit drops `ferrite` × 1 (× 2 for the foundry units),
through the blueprint's death drops (`Inventory` items that drop, as the
pistol does today). The haul rung brings that home, and a player can carry
it off as salvage (open question 6). Nothing is created from nothing:
ferrite is ore walked to the foundry, or the matter of a wreck.

A content test checks that each type's `cost` matches its recipe's cost,
as E2's does for eggs.

### Structures

Every structure stands on wire (`required_floor: ["blue_wires",
"red_wires"]`). E2's field takes a list; if E2 built it as a string, step 2
widens it. All are built inside the director's `radius` with the
player's build-order path, by wire placers (or harvesters, which may also
build: open question 5). The exception is the relay pylon, which is a
deployed relay drone.

| Key | What it does | Blueprint / script | Sprite (to confirm, open question 7) | Cost | Unlocked by | Director rule |
|---|---|---|---|---|---|---|
| `autonz_node`, `autonz_hub` | the stage chambers (exist) | structure scripts | wire tiles | — | — | the staged camp, re-stamped on stage rise (as now) |
| `autonz_fabricator` | prints the light units (exists) | `workbenches.json` | scifi_world (168, 120) tinted (as now) | — | — | placed by the camp at the centre (as now) |
| `signal_core` | the hub's transmitter: a live signal is sent from it, and the transmit rung's bodies walk into it. HP 60, `Kin` structure | `autonz_crew.json` (Kin structure, inanimate) | core block (384, 936), always steel (open question 8) | 20 metal_ore, 4 crystal (rebuild only) | — | the hub stamp places it at the centre. If it is destroyed, the director rebuilds it (hub stage) |
| `cogitator` | the research station: `required_building` of every assimilation entry | `build.json` entity | scifi_world terminal, tinted blue | 12 metal_ore, 2 crystal | start | from the node stage: one, after the locker |
| `relay_pylon` | **+4 live signal range** each (see "Signals"); sees 12 tiles; a seeker or pylon seeing the colony records contact | `build.json` entity (`deploy_only`: no build order, only a deploy) | scifi_world antenna mast, tinted | a relay drone | `autonz_relay_pattern` | hub stage: deploy a relay drone until the stage's count stands |
| `converter_node` | a **second convert origin**: the convert rung works outward from the hub and from each node. Wire placers within 5 tiles convert in half the ticks | `build.json` entity | scifi_world pylon, tinted | 10 metal_ore, 2 ferrite | `autonz_converter_node` | hub stage: up to the stage's count, at the wire's edge farthest from the hub and any other node |
| `foundry` | crafting station `autonz_foundry`: the heavy prints and ferrite | `workbenches.json` | scifi_world (168, 120) tinted bronze | 16 metal_ore, 4 crystal | `autonz_foundry_pattern` | hub stage: one |
| `defence_pylon` | a **sentinel's post**: the guard rung posts one sentry per pylon. HP 40 | `build.json` entity | scifi_world pylon, tinted red | 10 metal_ore, 2 ferrite | `autonz_sentinel_pattern` | hub stage: up to the stage's count, on the wire's edge facing the player's colony when one is on the level, otherwise evenly spaced |
| `storage_locker` | storage (exists) | — | — | (as now) | start | one per 10 crew (was one) |
| `autonz_sleeper_node` | the dormant presence (exists) | `autonz_crew.json` | **core block (384, 936), tinted dark** (was the enforcer at 240) | — | — | placed by materialise for a dormant presence (as now) |

Effects that need code are small and named once:

- the convert rung's origins (`convert_origins: ["converter_node"]` on the
  rung) and the halved `ConvertAction` time near a node (read from the
  structure's `ConvertBoost` component, radius 5, factor 0.5);
- the live signal's range from standing pylons (`signal.pylon`, `pylon_range`);
- `deploy_only` buildables, refused by the build rung and the player's
  placement list, accepted by a deploy.

**Tier 1 is unchanged by all of them.** The abstract presence has its
stage's structures implicitly. See "Signals" for how the pylon's range
keeps the pace.

### Signals

The signal trigger and cooldown are unchanged. The **range** is split so
that a pylon matters on a level and Tier 1 does not move:

- `mobility.base_range` goes from 20 to **16**, and `mobility.pylon_range`
  is **4** per relay pylon, counting at most the stage's `relay_pylon`
  count (1 at the hub).
- **Tier 1** counts the stage's pylons as standing. Only a hub signals, and
  a hub's count is 1, so its range is 16 + 4 = 20 plus techs, as today. The
  pace does not move. A content test asserts that `base_range +
  pylon_range × hub.structures.relay_pylon` equals the Phase E range.
- **Live** (`LiveSignal`): the range counts the relay pylons actually
  standing. A hub without its pylon signals at 16. The signal is sent from
  the **Signal Core**: with the core destroyed, a live hub does not signal
  (the day waits and the cooldown is not spent), as with E2's spire.
- **The signal's cost is physical.** E2's `launch` rung, aimed at
  `signal_core` and renamed `transmit` in this doctrine, sends `shot_pop`
  (2) bodies into the core on a ready day: wire placers first, then relay
  drones, never wardens. The live rule takes those bodies, not any two on
  the level.

### The assimilation tree

`research.json` stays a flat map, as E2 decided for the hive (E2's open
question 1). The autonz entries carry **`"tree": "autonz"`** and an
**`assimilate`** effect block. The four Phase E entries become real
entries under the same ids, so saves and the Tier 1 walk need no alias.

`assimilate` effects: `unlock` (a type), `tier` (`{ "type": ..., "tier":
"bronze" }`), `build` (a structure; the buildable's `required_tech` names
the entry, and this field is for the UI's effect line), `range` (signal
range, as now) and `conversion_rate` (as now).

```json
"signal_range_1":          { "name": "Carrier Gain I", "description": "_draft", "tree": "autonz",
                             "duration": 9000,  "cost": { "metal_ore": 10, "crystal": 2 }, "required_building": "cogitator", "required_int": 0,
                             "assimilate": { "range": 10 } },
"conversion_rate_1":       { "name": "Lattice Weave I", "description": "_draft", "tree": "autonz",
                             "duration": 12000, "cost": { "metal_ore": 12, "crystal": 2 }, "required_building": "cogitator", "required_int": 0,
                             "assimilate": { "conversion_rate": 0.5 } },
"signal_range_2":          { "name": "Carrier Gain II", "description": "_draft", "tree": "autonz",
                             "duration": 18000, "cost": { "metal_ore": 16, "crystal": 4 }, "required_building": "cogitator", "required_int": 0,
                             "requires_tech": "signal_range_1", "assimilate": { "range": 12 } },
"conversion_rate_2":       { "name": "Lattice Weave II", "description": "_draft", "tree": "autonz",
                             "duration": 24000, "cost": { "metal_ore": 20, "crystal": 4, "ferrite": 4 }, "required_building": "cogitator", "required_int": 0,
                             "requires_tech": "conversion_rate_1", "assimilate": { "conversion_rate": 0.5 } },

"autonz_relay_pattern":    { "name": "Relay Pattern", "description": "_draft", "tree": "autonz",
                             "duration": 6000,  "cost": { "metal_ore": 8, "crystal": 2 }, "required_building": "cogitator", "required_int": 0,
                             "assimilate": { "unlock": "relay_drone", "build": "relay_pylon" } },
"autonz_seeker_pattern":   { "name": "Seeker Pattern", "description": "_draft", "tree": "autonz",
                             "duration": 9000,  "cost": { "metal_ore": 10, "crystal": 2 }, "required_building": "cogitator", "required_int": 0,
                             "assimilate": { "unlock": "seeker" } },
"autonz_foundry_pattern":  { "name": "Foundry Pattern", "description": "_draft", "tree": "autonz",
                             "duration": 12000, "cost": { "metal_ore": 16, "crystal": 4 }, "required_building": "cogitator", "required_int": 0,
                             "assimilate": { "build": "foundry", "unlock": "harvester" } },
"autonz_converter_node":   { "name": "Converter Node", "description": "_draft", "tree": "autonz",
                             "duration": 12000, "cost": { "metal_ore": 12, "ferrite": 2 }, "required_building": "cogitator", "required_int": 0,
                             "requires_tech": "autonz_foundry_pattern", "assimilate": { "build": "converter_node" } },
"autonz_sentinel_pattern": { "name": "Sentinel Pattern", "description": "_draft", "tree": "autonz",
                             "duration": 15000, "cost": { "metal_ore": 14, "crystal": 2, "ferrite": 4 }, "required_building": "cogitator", "required_int": 0,
                             "requires_tech": "autonz_foundry_pattern", "assimilate": { "unlock": "sentinel", "build": "defence_pylon" } },
"autonz_warden_pattern":   { "name": "Warden Pattern", "description": "_draft", "tree": "autonz",
                             "duration": 18000, "cost": { "metal_ore": 18, "crystal": 4, "ferrite": 6 }, "required_building": "cogitator", "required_int": 0,
                             "requires_tech": "autonz_foundry_pattern", "assimilate": { "unlock": "warden" } },

"autonz_bronze_placer":    { "name": "Bronze Placer", "description": "_draft", "tree": "autonz",
                             "duration": 12000, "cost": { "metal_ore": 12, "ferrite": 4 }, "required_building": "cogitator", "required_int": 0,
                             "requires_tech": "autonz_foundry_pattern", "assimilate": { "tier": { "type": "wire_placer", "tier": "bronze" } } },
"autonz_bronze_enforcer":  { "name": "Bronze Enforcer", "description": "_draft", "tree": "autonz",
                             "duration": 15000, "cost": { "metal_ore": 14, "ferrite": 6 }, "required_building": "cogitator", "required_int": 0,
                             "requires_tech": "autonz_foundry_pattern", "assimilate": { "tier": { "type": "enforcer", "tier": "bronze" } } }
```

`autonz_bronze_relay`, `_seeker`, `_harvester`, `_sentinel` and `_warden`
follow the same pattern. Each requires its unit's pattern and costs 12–20
metal_ore and 4–8 ferrite over 12,000–18,000 ticks. They are listed in
step 3's data and not repeated here. Every bronze entry needs ferrite, so
**no bronze before the foundry**, which is the visible arc: steel scouts
and placers first, the foundry, then bronze.

**Tier 1 keeps its spine and gains a second track.** The hive could put
its unlocks in place of `tier_*` entries. The autonz walk has nothing to
replace, and any entry put in front of the range entries would delay the
range and move the pace. So the doctrine gets a second abstract walk:

```json
"research_per_day": 1,
"research": [ "...the four entries, unchanged in order and cost..." ],
"assimilation_per_day": [1, 2],
"assimilation": [
  { "id": "autonz_relay_pattern", "cost": 20 },
  { "id": "autonz_seeker_pattern", "cost": 30 },
  { "id": "autonz_foundry_pattern", "cost": 40 },
  { "id": "autonz_bronze_enforcer", "cost": 50 },
  { "id": "autonz_sentinel_pattern", "cost": 50 },
  { "id": "autonz_converter_node", "cost": 40 },
  { "id": "autonz_warden_pattern", "cost": 60 },
  { "id": "autonz_bronze_placer", "cost": 50 },
  { "id": "autonz_bronze_seeker", "cost": 50 },
  { "id": "autonz_bronze_relay", "cost": 40 },
  { "id": "autonz_bronze_harvester", "cost": 60 },
  { "id": "autonz_bronze_sentinel", "cost": 70 },
  { "id": "autonz_bronze_warden", "cost": 80 }
]
```

- `assimilation_per_day` is per stage (node 1, hub 2). Its own points walk
  its own list (`FactionPresence.AssimPoints`). `known_techs` is the union
  of both walks.
- No assimilation entry has a `range` or `conversion_rate` effect, so the
  spine alone sets the pace, and the pace test and Phase E table do not
  move.
- A signalled (dormant) presence starts with no techs, as now.
- Rough abstract arc: relay pattern on day 20, seeker on day 50, foundry
  around day 75 at a hub, bronze enforcer around day 100, warden around day
  150. `HFDETAIL=1` prints it (step 4).
- **Live**: the settlement's `research` list is one order that interleaves
  both walks: relay, signal range 1, seeker, foundry, conversion rate 1,
  then the rest. The research rung works it one entry at a time at the
  cogitator. Live durations are about 300 ticks per abstract point, worked
  by one placer about half the time (E2's reasoning), so a live node is
  near the abstract pace. The soak measures this.

### The settlement block

```json
"settlement": {
  "crew": [ { "blueprint": "wire_placer_crew", "type": "wire_placer", "share": 0.75, "role": "converter" },
            { "blueprint": "enforcer_crew",    "type": "enforcer",    "share": 0.25, "role": "guard" } ],
  "needs": false,
  "build_menu": [ { "id": "orders", "label": "Orders" }, { "id": "structures", "label": "Structures" } ],
  "buildables": [ "storage_locker", "cogitator", "foundry", "converter_node", "defence_pylon", "signal_core", "relay_pylon" ],
  "research": [ "autonz_relay_pattern", "signal_range_1", "autonz_seeker_pattern", "autonz_foundry_pattern",
                "conversion_rate_1", "autonz_bronze_enforcer", "autonz_sentinel_pattern", "autonz_converter_node",
                "signal_range_2", "autonz_warden_pattern", "autonz_bronze_placer", "autonz_bronze_seeker",
                "conversion_rate_2", "autonz_bronze_relay", "autonz_bronze_harvester", "autonz_bronze_sentinel",
                "autonz_bronze_warden" ],
  "units": [ "print_wire_placer", "print_enforcer", "print_relay_drone", "print_seeker",
             "print_harvester", "print_sentinel", "print_warden" ],
  "recipes": [ "refine_ferrite" ],
  "camp": { "staged": true, "w": 9, "h": 9, "station": "autonz_fabricator" },
  "director": { "...": "see The director table" }
}
```

New here beyond E2: `crew[].type` (E2's) on the autonz; `relay_pylon`
listed as a buildable but `deploy_only`. Nothing else is new in the block.

### The director table

The table is walked every 30 ticks, with the usual rules. **New** marks a
feature added in this phase. **E2** marks one E2 adds.

```json
"director": {
  "every_ticks": 30, "max_open": 3, "radius": 16, "reach": 50,
  "max_mines": 3, "max_hauls": 3, "stuck_ticks": 3000, "dig_depth": 6, "dig_reach": 80,
  "priorities": [
    { "id": "placers",   "grow": { "type": "wire_placer", "min": 2, "share": 0.3 } },
    { "id": "convert",   "convert": "blue_wires", "min": 40, "per_colonist": 60, "convert_origins": ["converter_node"] },
    { "id": "materials", "mine": { "metal_ore": 40, "crystal": 10 } },
    { "id": "haul",      "haul": "ferrite" },
    { "id": "storage",   "build": "storage_locker", "min": 1, "per_colonist": 0.1 },
    { "id": "cogitator", "build": "cogitator", "min": 1 },
    { "id": "assimilate","research": true },
    { "id": "dig",       "dig": true },
    { "id": "guard",     "grow": { "type": "enforcer", "share": 0.15 },
                         "guard": { "roles": ["guard", "warden"], "radius": 12, "post": 6 } },
    { "id": "core",      "build": "signal_core", "min": 1, "stage_min": 1 },
    { "id": "relay",     "deploy": { "type": "relay_drone", "as": "relay_pylon", "edge": 8 }, "stage_min": 1 },
    { "id": "foundry",   "build": "foundry", "min": 1, "stage_min": 1 },
    { "id": "ferrite",   "craft": "refine_ferrite", "min": 10 },
    { "id": "converters","build": "converter_node", "stage_min": 1 },
    { "id": "pylons",    "build": "defence_pylon", "stage_min": 1,
                         "guard": { "roles": ["sentry"], "posts": "defence_pylon" } },
    { "id": "grow",      "grow": true },
    { "id": "transmit",  "launch": { "walk": 2, "at": "signal_core", "roles": ["converter", "relay"] } },
    { "id": "seekers",   "patrol": { "roles": ["seeker", "relay"], "radius": 30, "every_ticks": 600 } },
    { "id": "raid",      "raid": { "fighters": 3, "party": 4, "home": 2, "cooldown_ticks": 6000, "max_ticks": 6000,
                                   "reach": 400, "unarmed": true, "roles": ["warden", "guard", "seeker"],
                                   "scouted": true, "leader": "warden" } }
  ]
}
```

| # | Rung | Runs when | What it orders |
|---|---|---|---|
| 1 | `placers` | wire placers under `2 + 0.3 × crew` and the crew under the conversion-capped cap (`crewCap`) | `print_wire_placer` (E2 grow `type`/`share`) |
| 2 | `convert` | wire within 16 under `40 + 60 × crew` | `ConvertAction` outward from the hub **and each converter node** (**new** `convert_origins`) |
| 3 | `materials` | stored + carried ore under 40, crystal under 10 | mine. Harvesters take mine orders first (role order, **new**: mine prefers the `harvester` role when one is idle) |
| 4 | `haul` | loose ferrite in reach (wrecks, refined output left by the station) | haul to a locker |
| 5 | `storage` | lockers under `1 + 0.1 × crew` | build a locker on wire |
| 6 | `cogitator` | no cogitator | build one on wire. It runs from the node stage, so a node researches |
| 7 | `assimilate` | a cogitator stands | the next entry of `research` that is unlocked, unknown and affordable (the research rung, `required_int` 0). This is the "autonz researching on the level" that E left out |
| 8 | `dig` | the surface has no ore in reach (as now) | stairs and tunnels (as now) |
| 9 | `guard` | enforcers under `0.15 × crew` | enforcer prints. Guard and warden roles hold posts 6 out from the centre and answer hostiles within 12 (E2 guard) |
| 10 | `core` | hub stage, no Signal Core standing | build one at the centre (rebuild after a player kills it) |
| 11 | `relay` | hub stage, relay pattern known, standing pylons under the stage's count | if no relay drone is idle, print one (`print_relay_drone`); then a **deploy** order at a wired tile at least `edge` 8 out, farthest from the hub on the side of the nearest eligible site in signal range (the Star Map position projected onto the level's edge), falling back to farthest from the hub (**new** rung) |
| 12 | `foundry` | hub stage, foundry pattern known, no foundry | build it on wire |
| 13 | `ferrite` | a foundry stands, stored ferrite under 10 | `refine_ferrite` (the existing craft rung) |
| 14 | `converters` | hub stage, converter node known, nodes under the stage's count (2) | build one at the wire's edge farthest from the hub and other nodes (E2 `stage_min` and the `structures` cap) |
| 15 | `pylons` | hub stage, sentinel pattern known, pylons under the stage's count (2) | build one at the wire's edge toward the player's colony if it is on the level. Sentries are posted one per pylon (**new** guard `posts`: a structure key instead of a ring) |
| 16 | `grow` | crew under `crewCap` | print the type most under the stage's `mix` among unlocked types, at its station (fabricator or foundry). This is where seekers, harvesters, sentinels and wardens come from |
| 17 | `transmit` | `Campaign.LiveSignalReady` (threshold, cooldown, hub, core standing, a site in range) | send `shot_pop` bodies of the `converter`, then `relay`, role into the Signal Core (E2 `launch`, **new** `at` and `roles`). The daily live rule signals once they are within `walk` 2 and takes them |
| 18 | `seekers` | seekers or undeployed relay drones idle | patrol a ring at 30 (E2). Contact records `Site.Scouted` |
| 19 | `raid` | hostile, the colony on the level, contact made (E2 `scouted`), fighters ≥ 3 | the raid rung. **New** `leader`: once wardens are unlocked, a party forms only around a warden, and the rest follow its target. Without the warden pattern known, it raids as now |

Ordering notes:

- `placers` and `convert` stay first, as in Phase E, so the Phase E soak's
  conversion numbers hold for a node (the soak checks this).
- `cogitator` and `assimilate` sit above `dig` and `guard`. A node spends
  12 ore once, then one placer part-time. The soak watches what this costs
  in conversion.
- Every hub structure sits below the economy rungs. The `relay` rung sits
  above the `foundry` rung, so the pylon that keeps the signal range comes
  first.
- `transmit`, `seekers` and `raid` send bodies away from the centre, so they
  come last, as in the hive.

### The campaign daily step and the level

| Tier 1 (per day) | On the level |
|---|---|
| growth `per_day` 0.04 toward `ConvertCap` | prints at the fabricator and foundry (`placers`, `guard`, `grow`), up to `crewCap` (the same cap) |
| starvation above `ConvertCap` | none (`needs: false`); the grow rungs stop. Reconcile records the live count |
| `conversion` += `per_day` × rate × share × tags | `convert` rung, converter nodes; reconcile reads `ConversionShare` (as now) |
| spine `research_per_day`; **new** assimilation track per stage | the cogitator's research rung over the interleaved list; the settlement's techs reconciled (as for every settlement) |
| signal: hub, threshold or `signal_conversion`, cooldown, range `base + pylon × count + techs` | `transmit` rung and `LiveSignal`: core standing, pylons actually standing, bodies walked in |
| `shot_pop` taken from `units`: wire placers first, then relay drones | those bodies, walked into the core |
| new body's type: the most under the stage's mix among unlocked (`type_fallback`) | the `grow` rung's pick: the same rule |
| (none) | ferrite, stock, structures. Bookkeeping only (below) |

**New presence fields** (bookkeeping only, never blocking growth):

| Field | Abstract | Live (reconcile) |
|---|---|---|
| `units` (`map[type]int`, E2) | the composition rule above | living bodies by `Kin.Type` |
| `built` (`map[key]int`, **new**) | the stage's `structures`, filtered by known techs, each time the stage or techs change | standing structures of the settlement by blueprint key |
| `stockpile` | unchanged (not stepped). Materialise uses the floor in step 4 when it is empty | `storage.Summarize` (as now) |
| `assim_points` | the second walk | — |

### Materialise and reconcile

**Materialise** (`internal/game/faction_level.go`, `faction_autonz.go`):

1. **Centre** as now.
2. **Camp, staged** as now (node or hub stamp, hold lifted and put back,
   locker claimed, fabricator by the centre). At the hub, the
   **`signal_core` entity** is placed on the stamp's red core.
3. **Wire** to `conversion` (`ApplyConversion`, as now), **before**
   structures, which need wire.
4. **Structures**: `built` if recorded, else the stage's `structures`
   filtered by known techs. They are placed on wired sites from the
   director's own site picker (the same function the build rung uses:
   `director.PickSite(kind, settlement)`), so a materialised hub has its
   foundry where the director would have built it. Relay pylons go where
   the `relay` rung would deploy one, and defence pylons evenly spaced on
   the edge (no colony when frozen).
5. **Stock**: `stockpile` into lockers (as now). An empty stockpile gets
   the floor: 20 metal_ore and 4 crystal, plus 10 ferrite when a foundry is
   placed.
6. **Bodies by type**: topped up or culled to `units`, each at its type's
   tier from the known techs (bronze if known). An old save's empty `units`
   is filled from `presenceComposition` by type.
7. **Adopt Phase E bodies** (once, idempotent): a crew body with no
   `Kin.Type` gets its type from its blueprint (`wire_placer_crew` →
   `wire_placer`, `enforcer_crew` → `enforcer`). It is re-sprited to the
   new x (the save holds the old `SpriteX`), and stats and role come from
   the type. The sleeper node is re-sprited to the core block in the same
   pass.

**Agreement between stamps and the director** (the owner's "the two must
agree"): a test builds a hub by materialise, and another by a live node
run to the hub with the same techs. It checks that both hold the same
kinds and counts of structure, all on wire, inside the radius, with
footprints that do not overlap the stamp's ways out. Placement comes from
one picker, so the positions match too when the wire matches.

**Reconcile** (every `writeLocation`): `units`; `built`; conversion (as
now); techs; stage rise (as now). The Signal Core is in `built`, so a hub
saved with its core dead materialises without it. The director rebuilds
it. A wipe is no bodies (as now). Structures do not count.

**Live rule** (`stepLiveAutonz`): as now, plus the signal waits for the
transmit bodies and the core (see "Signals").

### `autonz_incursion`, `autonz_sleeper` and the GameMaster

- **`autonz_incursion` stays wild.** Its setup script and spawn rules spawn
  the scripted `wire_placer` and `enforcer` as now, steel, with no
  presence. The only visible change is the sprite remap (wire placer to
  the claw walker, enforcer to the red-bar tripod), so its frames change in
  look and not in behaviour. With a presence there, the settlement builds
  by the chapter's origin (as now). Wild units are the same faction: never
  fought, not adopted, not counted (E2's rule for wild hive creatures).
- **`hostile_max`** counts the settlement's `Kin` workers (E2), so a big
  autonz hub on an incursion world throttles the GameMaster's waves rather
  than stacking on them.
- **Same-faction targeting.** The wild enforcer runs
  `faction_humanoid.basic` and the wild placer runs
  `autonz_wire_placer.basic`. E2 fixes `find_nearest_worker`. Step 4 checks
  that the humanoid script's target pick also skips its own faction's
  workers, and adds the skip if not.
- **`autonz_sleeper` (the hazard chapter)** is unchanged in behaviour. Its
  evidence is a derelict pod with the scenario's `enforcer` blueprint. That
  pod's body gets the **unlit tripod** (144, 936) through a new
  `enforcer_dormant` blueprint, so "an enforcer that powered down" looks
  powered down (open question 9). `content_hazard_test.go`'s row changes
  from `enforcer` to `enforcer_dormant`.
- **The faction's sleeper node** (`autonz_sleeper_node`) becomes the dark
  core block, as the owner listed. Its rules are unchanged (wipe if
  destroyed before waking, camp on wake).

### Phase F: what the player will see

Not built here. The data is shaped for it:

- **HUD.** `Conversion 34% (cap 85%)` beside the resource bar, and the crew
  as `7 / 9` with the cap from `ConvertCap`. That is the economy made
  visible: lay wire to raise the cap.
- **Orders**: *Lay wire* (`queueConvert`, gated by the doctrine having a
  `conversion.tile`, as E2 changes it), *Deploy* on a selected relay drone
  (the deploy order, wired tiles highlighted).
- **Structures** menu: locker, cogitator, foundry, converter node, defence
  pylon, Signal Core. Each is placeable only on wire, and locked entries
  name their pattern.
- **Printing.** Clicking the fabricator or the foundry opens the crafting
  modal: one row per unit with cost and time, locked rows greyed with the
  pattern named, and a steel or bronze chip showing what the next print
  will be.
- **Assimilation.** Clicking the cogitator opens the research modal over
  the `autonz` tree, titled "Assimilation", with each entry's effect line
  ("Unlocks Seeker", "Bronze Enforcers: ×1.5 health, ×1.25 strength",
  "Signal range +10", "Builds Converter Node").
- **Crew panel**: units by type, steel and bronze counted apart.
- **Signal.** The Signal Core's panel shows readiness (threshold,
  cooldown, range with pylons) and the sites in range. Firing it and the
  win rule are F's own work.

## Balance

**Tier 1 is unchanged.** `TestFactionPaceRivalOne` passes as it is.
`make history-dump HFACTIONS=0,1,2,3,4 HFSEEDS=1-6 HFCOLS=0,40,100,200,300`
prints the Phase E table. The split range (16 + 4) and the second walk
must not move a number. Any difference is a bug.

**Live targets** (the extended autonz soak):

| Measure | Target |
|---|---|
| conversion | node start: within 0.03 of the Phase E table at 20,000 ticks (the cogitator costs a little); +0.25 or more by 30,000 |
| prints | follow `crewCap` (as in E). Every unlocked type present by the end on hub seeds |
| cogitator | built by tick 6,000 on 9 of 10 seeds |
| first research (relay pattern) | by tick 15,000 on 8 of 10 |
| hub seeds | Signal Core standing; a relay pylon deployed within 2 days of the relay pattern; foundry within 3 days of its pattern; ferrite refined ≥ 10 |
| bronze | at least one bronze print on hub seeds once a bronze entry is known |
| raid | never in the neutral soak |

**The soak** extends `TestAutonzSoak` (it does not add a new one) to
30,000 ticks and two starts. **Node**: the Phase E start, ten seeds. **Hub**
(new, five seeds): a hub of 9 with 40 metal_ore, 8 crystal, 10 ferrite,
and the relay and foundry patterns known. Columns per seed:

| Seed | Start | Share 0 / 10k / 20k / 30k | Converts | Mines | Prints by type | Bronze prints | Crew start / end (cap) | Stage | Cogitator tick | Techs (first tick) | Core / pylons / foundry / converters / defence | Ferrite refined | Cancelled |

A seed **misses** as in E (conversion up less than 0.05, or no order toward
ore), or when no cogitator stands by 15,000 ticks. More than two misses
fails. The other columns are measured and recorded in "Phase E3 as built".
`SOAKSEEDS` and `SOAKTICKS` apply.

**Expected first problem**: crystal. Relay drones, the cogitator, the
foundry and refining all want crystal, and seeds 2 and 7 found no ore in
dig reach in E. If the soak shows hubs stuck on crystal, the default fix is
to let `refine_ferrite` take metal only, at 5 ore → 2 ferrite.

## Order of work

Each step leaves the game playable and the suite green. It runs after E2
lands, because steps 1–5 lean on its types, rungs and fields.

### Step 1: Types, tiers and the sheet

- `types`, `tiers` (`sprite_dy`), `type_fallback` and the stage `mix` by
  type in the autonz doctrine. `Doctrine.TierOfType`.
- `ApplyType` applies a per-type tier with `sprite_dy`. `Kin.Tier` is kept
  or put back.
- Sprite remap: `wire_placer` and `wire_placer_crew` to x=48, `enforcer`
  and `enforcer_crew` to x=192, in both files. New crew blueprints
  `relay_drone_crew`, `seeker_crew`, `harvester_crew`, `sentinel_crew`,
  `warden_crew`. Item `sentinel_emitter`.
- **Files:** `data/factions.json`, `data/blueprints/entities/autonz.json`,
  `data/blueprints/entities/autonz_crew.json`, `data/blueprints/items/*.json`,
  `internal/factory/type.go` (E2's), `internal/components/KinComponent.go`,
  `internal/campaign/doctrine_types.go` (E2's).
- **Done:** every autonz type spawns with its sprite, stats and role. A
  bronze type spawns one row down.
- **Verify:**
  - `internal/factory/type_test.go` (an autonz row per type: sprite x
    and y, steel and bronze stats);
  - a content test that every type's sprite cell is non-empty on the sheet
    in both rows;
  - `TestFactionPaceRivalOne`;
  - the history-dump table identical;
  - `lore_autonz_test.go`;
  - `make vmtest SCRIPT=factions_autonz` (frames change in sprite only;
    inspected and noted).

### Step 2: Structures, items and recipes

- Blueprints `signal_core`, `cogitator`, `foundry` (station
  `autonz_foundry`), `converter_node` (`ConvertBoost`), `defence_pylon`,
  `relay_pylon` (`deploy_only`), item `ferrite`, `enforcer_dormant`. The
  sleeper node re-sprited.
- `required_floor` as a list. The build rung and the player's placement
  refuse `deploy_only` keys.
- Recipes: the five new prints and `refine_ferrite`. `kin_type` on the two
  that exist. Death drops of ferrite.
- `Orders.Deploy` and the deploy task: walk, work, replace the unit with
  the structure. The `ConvertBoost` read in `ConvertAction`.
- The hub stamp places `signal_core`.
- **Files:** `data/blueprints/entities/autonz_crew.json`, `data/build.json`,
  `data/blueprints/structures/workbenches.json`,
  `data/blueprints/items/resources.json`, `data/crafting_recipes.json`,
  `data/scripts/structures/autonz_hub.basic`, `data/history/events.json`
  (the sleeper evidence blueprint), `internal/construction/buildables.go`,
  `internal/workerai/task_handlers.go` (deploy),
  `internal/game/settlement_orders.go`, the ConvertAction file
  (`internal/actions/convert*.go`), `internal/game/content_hazard_test.go`.
- **Done:** in the cheat console, a relay drone deploys into a pylon on
  wire and is refused off it, a foundry refines ferrite, a placer next to a
  converter node lays wire in half the ticks, and a destroyed enforcer drops
  ferrite.
- **Verify:**
  - `internal/workerai` `TestDeploy_ReplacesUnit`, `TestDeploy_FloorRequired`;
  - `internal/construction` `TestDeployOnlyRefused`,
    `TestRequiredFloorList`;
  - a ConvertAction test (boost radius and factor);
  - the content test that each type's cost matches its recipe;
  - `TestColonyDoctrineCoversContent` (the new entries belong to the
    autonz).

### Step 3: The assimilation tree

- The `tree: "autonz"` entries with `assimilate` in `research.json`,
  including the four existing ids and every bronze entry. The doctrine's
  `assimilation` and `assimilation_per_day`. The settlement's interleaved
  `research` list.
- The Tier 1 second walk (`AssimPoints`, the union of known techs). The
  `assimilate` effects: `range` and `conversion_rate` read where they are
  read now, `unlock` through `TypesUnlocked`, `tier` through `TierOfType`.
- Wire placers get `research` in their tasks, so the research rung works
  for the autonz.
- **Files:** `data/research.json`, `data/factions.json`,
  `internal/research/*.go` (the `assimilate` block beside E2's `evolve`),
  `internal/campaign/faction.go`, `internal/campaign/faction_signal.go`
  (range from the entry), `internal/campaign/doctrine_types.go`.
- **Done:** a presence at day 100 knows the relay, seeker and foundry
  patterns, and its range and conversion entries land on the Phase E days.
- **Verify:**
  - `internal/campaign/faction_autonz_tree_test.go` (both walks advance;
    the spine's days are unchanged; assimilation entries never change
    range or rate; a save without `AssimPoints` loads);
  - `TestFactionPaceRivalOne`;
  - the history-dump table identical;
  - the colony research snapshot unchanged.

### Step 4: Settlement, materialise and reconcile

- The settlement block (crew by type, buildables, research, units,
  recipes). `built` on the presence. Split range (`base_range` 16,
  `pylon_range` 4) with the content test for the sum.
- Materialise and reconcile as described: wire, then structures from the
  director's picker, stock floor, bodies by type and tier, adoption of
  Phase E bodies and the sleeper sprite. The live signal needs the core and
  counts pylons.
- The director runs the Phase E table plus `cogitator` and `assimilate`
  at this step, so a live node researches. The rest is step 5.
- The humanoid script's same-faction skip, if missing.
- **Files:** `data/factions.json`, `internal/campaign/faction.go`,
  `internal/campaign/faction_signal.go`, `internal/game/faction_level.go`,
  `internal/game/faction_autonz.go`, `internal/director/director.go`
  (`PickSite` exported for materialise), `data/scripts/ai/faction_humanoid.basic`.
- **Done:** seed 65 with Rival factions 4, after `debug days 120`: Leaden
  Ridge's hub lands with its core, cogitator, a pylon and the foundry if
  its techs have it, typed units, and some bronze if known. A save, a
  freeze and a thaw round-trip it.
- **Verify:** `internal/game/faction_autonz_test.go` (extended):
  - materialise is deterministic and idempotent;
  - wire before structures, every structure on wire;
  - materialise and a live run to the hub agree on kinds and counts;
  - units by type and tier;
  - adoption runs once;
  - reconcile round-trips `units`, `built`, conversion and techs;
  - a hub with its core destroyed does not signal live;
  - a hub's live range counts its pylons;
  - a wild enforcer ignores the settlement's workers.

### Step 5: The autonz director

- New rung features: `convert_origins`; mine prefers `harvester`; the
  `deploy` rung; guard `posts` by structure; launch `at` and `roles`
  (transmit); raid `leader`. The full table in the doctrine.
- `Doctrine.RaidAlert` for the autonz (named as themselves).
- **Files:** `internal/director/convert.go`, `internal/director/director.go`,
  `internal/director/deploy.go` (new), `internal/director/guard.go` (E2's),
  `internal/director/launch.go` (E2's), `internal/director/raid.go`,
  `internal/campaign/doctrine_settlement.go`, `data/factions.json`.
- **Done:** a hub left live for 40 days with the relay and foundry patterns
  deploys a pylon, builds the foundry, refines ferrite, prints a harvester,
  builds converters once known, posts sentinels at defence pylons, and on a
  ready day walks two placers into the core and signals.
- **Verify:**
  - `internal/director/autonz_test.go` (each rung's trigger and order:
    placers first when short, cogitator before research, `stage_min` gates
    the hub structures, relay prints then deploys toward the nearest site,
    converters at the edge, sentries one per pylon, grow picks the type most
    under the mix, transmit only on a ready day, raid forms around a warden
    once wardens are known);
  - `internal/game/faction_autonz_live_test.go` (a live signal with bodies
    walked in; guards answer a colonist in the radius; a hostile hub raids
    after contact).

### Step 6: Soak, vmtest, docs

- `TestAutonzSoak` extended as in "Balance", with numbers recorded.
- history-dump `HFDETAIL=1` columns for the autonz: units by type, built,
  assimilation techs. The Factions tab's autonz row lists units by type
  with a bronze count, and the structures built.
- vmtest: `factions_autonz.jsonl` keeps its frames (seed 65, Rival
  factions 4, `debug days 80`) and adds:
  1. the hub from the landing's camera with the Signal Core, cogitator and
     a relay pylon;
  2. after `debug days 40`, the foundry, a harvester and sentinels on
     pylons (bronze if known);
  3. the Factions tab.

  The same script in an observer run (`observer on`, View Leaden Ridge)
  adds a frame of a bronze unit beside a steel one, using a new console
  `faction tech <site> <id>` cheat to grant `autonz_bronze_enforcer` so the
  frame does not depend on the day.
- Docs: `factions.md` ("The autonz": roster, tree, structures, signals;
  the table in "How a faction moves and acts"), `vmtest.md`,
  `enemy_factions_plan.md` (E3 in the phasing table, "Phase E3 as built"),
  `todo.md`.
- **Files:** `internal/game/autonz_soak_test.go`, `cmd/history-dump/main.go`,
  `internal/game/factions_panel.go`, `internal/game/console*.go` (the
  cheat), `vmtest/scripts/factions_autonz.jsonl`, `internal/game/vmhook.go`
  (sidecar: units by type, built), `docs/developer/factions.md`,
  `docs/developer/vmtest.md`, `docs/brainstorm/enemy_factions_plan.md`,
  `docs/brainstorm/todo.md`.
- **Done:** the soak passes, the frames are inspected, and the docs say
  what was built.
- **Verify:**
  - `make soak`;
  - `make vmtest SCRIPT=factions_autonz`;
  - every other vmtest script passes (`autonz_incursion`-bearing ones
    differing in sprite only);
  - `go test ./...`.

Steps 2 and 3 touch different files and can run side by side after step 1.
Steps 4 and 5 run in order. The history-dump and panel part of step 6 can
start after step 4.

## Open questions (decided: the owner took every default, 2026-11-08)

1. **Remap the existing sprites.** Default: yes. The wire placer moves to
   the claw walker (48) and the enforcer to the red-bar tripod (192), in
   the scenario and the crew, so the owner's roster holds everywhere. The
   alternative keeps today's look for the two existing units (antenna
   walker and red orb) and shifts the roster around them.
2. **Node mix with nothing unlocked.** Default: retune the node `mix` so
   that the fallback gives exactly Phase E's 0.75 / 0.25, which keeps the
   E soak's prints. The alternative takes the small shift to 0.8 / 0.2.
3. **Steel units after a bronze entry.** Default: they stay steel (tier at
   spawn only). The alternative is a foundry *refit* recipe (a steel unit
   and ferrite in, bronze out), or upgrading living units as E2 does for
   the hive.
4. **Bronze print cost.** Default: the same recipe and cost; the research
   is the price. The alternative is +2 ferrite per bronze print (a
   tier-dependent cost, which is new code).
5. **The harvester's "repairs".** Default: no repair in this phase.
   Harvesters mine, dig and haul (and may build: default no). Repair needs
   a repair task for damaged structures and units, and is noted for F.
6. **Ferrite for the player.** Default: an item the player can pick up and
   store as salvage, worth nothing in their recipes yet. The alternative
   gives it to a colony recipe (scrap into metal).
7. **Structure art.** Default: borrowed `scifi_world` terminals and pylons
   tinted per role, and the core block for the Signal Core and sleeper.
   Say if new art should come first.
8. **A bronze Signal Core.** Default: none. Structures have no tier. The
   bronze cube (384, 960) is spare and could mark a hub that knows every
   bronze entry.
9. **The hazard's derelict enforcer.** Default: an `enforcer_dormant`
   blueprint drawn as the unlit tripod (144). The alternative leaves the
   hazard's pod as today's enforcer.
10. **The live signal without a core.** Default: no live signal while the
    Signal Core is down (a player target), and the director rebuilds it.
    The alternative is the Phase E rule (signal regardless).
11. **Raid leader.** Default: once wardens are unlocked, raids form only
    around a warden. Without the gate, wardens join raids like any guard.
12. **Contact before raids.** Default: yes, the E2 `scouted` gate (seekers
    and pylons). Phase E raids whenever armed enough.
13. **Phase name.** Default: E3, so F keeps its letter.

## Where the owner rewrites text

- Type `name`s (`data/factions.json` `types`).
- Print recipe and `refine_ferrite` names and descriptions
  (`crafting_recipes.json`).
- Every `autonz_*` research name and description, and the new names for
  the four existing entries ("Carrier Gain", "Lattice Weave").
- Blueprint names and `LongDescription`s: the five new crew bodies,
  `signal_core`, `cogitator`, `foundry`, `converter_node`, `defence_pylon`,
  `relay_pylon`, `ferrite`, `sentinel_emitter`, `enforcer_dormant`, and the
  re-sprited sleeper node ("Dormant Enforcer" no longer fits a cube).
- The autonz raid alert (`Doctrine.RaidAlert`), the core-destroyed and
  pylon-deployed alerts, and the live signal alert if it changes.
- Any new Legends event (`faction_assimilated` for a bronze entry, if
  added).
- The research modal's title ("Assimilation") and the Factions tab's autonz
  row labels.

All of these are marked `"_draft"`, as the existing files are.

## As built (2026-11-08)

Commits on `claude/factions-a-a379f7` after `1d28af5` (this plan): step 1
`c1c5ee3`, step 2 `d497e09`, step 3 `4a865e7`, steps 4 and 5 `4e8b177` (one
commit: they share `faction_autonz*.go`, `faction_settlement.go`,
`director.go` and the doctrine table), then step 6 with the soak, vmtest and
docs. The plan was written while E2 was a plan; where they differ it follows
what E2 built, and where the plan and the code disagreed the plan's intent
was kept. Tier 1 did not move: `TestFactionPaceRivalOne` passes unchanged and
`make history-dump HFACTIONS=0,1,2,3,4 HFSEEDS=1-6 HFCOLS=0,40,100,200,300`
is byte-identical to the table before this phase at every step (the
`-fdetail` listing differs in the autonz' techs and its new line).

### Step 1: types and tiers

- **No doctrine-wide `tiers` on the hive.** E2 removed the presence tier;
  the autonz' steel and bronze are a doctrine `tiers` list (`steel`, then
  `bronze` with `sprite_dy` 24, health ×1.5, str ×1.25) applied **per type at
  spawn**: `Doctrine.TierOfType(type, techs)` is the last tier a known
  research entry's `assimilate.tier` names for that type, `upgradedType`
  folds its multipliers and `sprite_dy` into the `factory.UnitType`, and
  `ApplyType` adds `SpriteY` and records the tier on the new
  **`Kin.Tier`** (`tier_name` in saves; the old Phase B `tier` integer is
  still read and mapped to a hive type). `factory.Retype` (the hive's
  upgrades on living units) refuses a body whose tier would change, so a
  steel print stays steel. The hive is untouched.
- **Fallbacks.** The autonz have no morphs, so a type's `fallback` names
  where a locked type's share goes (seekers, sentinels and wardens to the
  enforcer; relay drones and harvesters to the wire placer), with
  `type_fallback` (the wire placer) behind it. With nothing unlocked a node
  is 0.75 / 0.25 and a hub 0.6 / 0.4, Phase E's mixes exactly (open
  question 2), from the retuned stage mixes: node placer 0.65, relay 0.1,
  enforcer 0.15, seeker 0.1; hub placer 0.4, harvester 0.15, relay 0.05,
  enforcer, seeker, sentinel and warden 0.1 each. `economy_types` (placers,
  then relay drones) is what a signal takes first and starvation last.
- **`Doctrine.Broods()`** (types with morphs) now marks the hive's paths
  (`isHiveDoctrine`, the hive's Tier 1 tally, the history-dump line, the
  Factions tab's morph-fogged line), since the autonz have types too.
- A type's cost is `print_cost` (a map; the hive's `cost` is biomass), its
  body `blueprint`. Crew entries carry `type`; materialise applies it.
- Sprites as the plan's sheet: placer x48, enforcer x192 in both files; the
  new crew bodies (`relay_drone_crew` x0, `seeker_crew` x96,
  `harvester_crew` x288 and `sentinel_crew` x240 with the `flying` skill,
  `warden_crew` x216). `enforcer_crew` lost its starting pistol (its type
  equips one). `sentinel_emitter` is a ranged weapon item like the spit
  gland (1d8, range 6).

### Step 2: structures, items and recipes

- The tech gate on recipes is the existing `requires_tech`; on buildables the
  existing `required_tech`. `required_floor` reads a string or a list
  (`construction.FloorList`); the autonz' structures stand on
  `["blue_wires", "red_wires"]`.
- Structures (`data/blueprints/entities/autonz_structures.json`, `Kin`
  structures of faction `autonz`): `signal_core` (the core block 384,936,
  HP 60), `cogitator` (`ResearchBuilding`, the research-lab terminal
  tinted blue), `converter_node` (`ConvertBoost` radius 5, factor 0.5; the
  station generator tinted blue), `defence_pylon` (the generator tinted
  red), `relay_pylon` (the green console at 720,24, tinted). `foundry` is in
  `workbenches.json` (station `autonz_foundry`, the fabricator tinted
  bronze). Their build menu category is `autonz` (the colony's
  `structures` snapshot test would otherwise list them).
- `relay_pylon` is `deploy_only` and `hidden`: `orderBuild` refuses it and
  no menu lists it.
- **Deploy** (`Orders.Deploy`, `task_requests.DeployAction`,
  `workerai/deploy.go`): the drone walks onto the tile, works
  `ticks / 10` work steps (as research durations: one a worker turn) and is
  replaced by the structure on that tile with its Kin presence and
  settlement; off the floor it is refused.
- `ConvertAction`'s work is `workerai.ConvertWork`: the strongest
  `ConvertBoost` within five tiles scales it.
- **Typed prints deploy.** A `kin_type` recipe with `deploy_on_craft` and
  without `consume_station` (the hive's eggs keep their path) makes the
  output a worker of the crafter's settlement before its type is applied,
  so the type's role and tasks land on it. Recipes gained `count` (refine:
  two ferrite).
- Ferrite drops through the bodies' `Drops` (one; two for the foundry
  units), wild ones too. The hazard's derelict is `enforcer_dormant` (the
  unlit tripod 144); the faction's sleeper node is the core block tinted
  dark, and an older save's node is re-drawn from its blueprint when its
  level is built.
- The hub stamp spawns the Signal Core one tile north of the centre (the
  centre is the landing, kept open).

### Step 3: the assimilation tree

- 17 `tree: "autonz"` entries with `assimilate` blocks; the four Phase E
  ids are real entries under the same ids (Carrier Gain I/II, Lattice Weave
  I/II). **Durations are work steps**: the plan's ticks over ten (Carrier
  Gain I 900). The bronze entries the plan did not list: relay 12 ore 4
  ferrite 1200, seeker 14/4 1200, harvester 16/6 1500, sentinel 18/6 1500,
  warden 20/8 1800, each after its unit's pattern.
- The second walk: `assimilation` and `assimilation_per_day` [1, 2] on the
  doctrine, `FactionPresence.AssimPoints`. **The spine's next entry is now
  its first unknown entry**, not the one indexed by the count of known
  techs (with a second walk in the same list the index was wrong); the
  table did not move. A signal copies its parent's techs, as in Phase E
  (the plan said "no techs, as now"; the code copied them, and the pace
  depends on it, so it still does).
- `research/hive_tree_test.go` now skips the autonz' tree; the campaign test
  `faction_autonz_tree_test.go` walks it.

### Steps 4 and 5: the settlement and its director

- **Materialise** (`faction_autonz_settle.go`): staged camp; on a stage rise
  the chamber is stamped with the settlement's own things lifted out and put
  back (its structures, lockers with their contents, the fabricator), then
  `furnishStagedCamp` places what is missing (Phase E re-stamped through
  `buildStagedCamp`, which summarised the hold and cleared the rest; a
  first version stamped twice and emptied the lockers). Then the wire, then
  the record's `Built` (Tier 1 keeps it at the stage's structures its techs
  allow, `Doctrine.StageBuilt`) on wire: relay pylons at the farthest wired
  tile 8 or more out, defence pylons spread round the edge, the rest on the
  director's grid nearest the centre (`autonzSite`, the hive's
  `hiveSite` on wire). **One picker for both was not built**: the
  director's own `siteFor` needs its reachability snapshot, so materialise
  uses the same grid rule without it. Kinds and counts agree (both read the
  stage's structures and the buildables' techs); positions agree only
  where the grid's first free wired site is the same.
- The stock floor (20 metal, 4 crystal, 10 ferrite with a foundry) applies to
  a first build with an empty hold. Units: an empty record is filled by the
  growth rule (`NormalizeUnits`), not the composition pattern, which for a
  small hub gave one placer in seven. Phase E bodies (no Kin type) are
  replaced on their tile by their type's print, once.
- **Live rule** (`stepLiveAutonzSettlement`): reconcile (units, `Built`,
  stock, techs, conversion), then `LiveSignal(..., pylons, ready)`: ready
  is the Signal Core standing with `shot_pop` bodies the transmit rung
  holds at it; the signal takes those bodies; without them the day waits.
  A risen stage is only re-stamped (nothing placed on a live level, as E2).
- **Signal range**: `base_range` 16, `pylon_range` 4 per relay pylon up to
  the stage's count (`Doctrine.SignalRange`; Tier 1 passes −1, the stage's
  count). `Campaign.SignalReady` (no random draw) feeds the transmit rung;
  `Campaign.SignalBearing` gives the deploy rung its direction.
- **Director**: the typed grow rung prints at the recipe's station (the
  fabricator or the foundry), one print a station at a time, never past
  the crew cap (the hive's larvae path is unchanged); `convert_origins`
  sorts candidates by the nearest origin; the `deploy` rung prints a relay
  drone when none is idle, then deploys one toward the bearing; guard
  `posts` sends one sentry beside each standing pylon (the guard radius
  defaults to 12); launch `at` and `roles` (transmit: placers, then relay
  drones, never a warden); raid `leader`. The converters and pylons rungs
  carry `min: 2` (the stage's count caps them; with no min the want was 0).
- **Tuning the soak asked for** (live only; Tier 1 untouched):
  - **The autonz never sleep** (`NeverSleep` on every crew body). Without it
    every non-colonist worker stops at night; a cogitator's research
    stalled each night at the same progress. The hive's crew has the same
    gap (its E2 evolve stalls); left alone here, noted as a question.
  - **Wire while at the cap.** The convert rung wants `40 + 60 × crew`
    tiles; a hub at its conversion-capped crew already had them, so it
    never converted again and its cap never rose (a deadlock Phase E's
    nodes never reached). The rung's new `at_cap` keeps one more tile
    wanted while the crew is at its cap (the autonz' convert rung only: a
    first version keyed it on any typed doctrine and moved the hive soak's
    resin).
  - Misses count per start in the soak (the hub seeds reuse the node seeds'
    maps; seed 2's site, with no ore in dig reach, counted twice).
- Not changed: `mine` does not prefer harvesters (orders are a queue the
  workers pull from; harvesters take mine work since it is most of what
  they may do); a pylon seeing the colony does not record contact (seekers
  do, through the patrol rung). `faction_humanoid.basic`'s
  `find_nearest_enemy` already skips its own faction, so a wild enforcer
  never fights the settlement: no change.
- The alert names the autonz as themselves ("Autonz raiders are moving on
  your colony on ..."), the Phase D rule for a doctrine without morphs.

### Step 6: soak, vmtest, docs

**The soak harness** (the owner's direction): the soaks share package
globals (the settlement map, the path budget, the content tables), so their
seeds cannot be `t.Parallel` subtests in one process. Instead
`scripts/soak_par.sh` runs one test process per seed (`LP_SOAK_ONLY`,
`node:3`, `hub:3`, `hive:3`), `SOAKJOBS` (10) at once, each writing its row
as JSON into a temporary directory (`LP_SOAK_ROWS`), and a join test
(`TestAutonzSoakJoin`, `TestHiveSoakJoin`) prints the same table and applies
the same thresholds. `make soak-autonz` and `make soak-hive` run one soak
that way; `make soak` runs the observer and expedition soaks as before
(left sequential: one process, unchanged), then both. `AUTONZTICKS`
(30,000) and `HUBSEEDS` (half `SOAKSEEDS`) are new; a one-process run (no
`LP_SOAK_ROWS`) still works. The hive soak's row fields were exported for
the JSON (a rename; its table and thresholds are the same).

**The autonz soak** (`TestAutonzSoak`, 30,000 ticks: forty days; node: Phase
E's start, ten seeds; hub: a hub of up to nine (its conversion cap less two)
with 40 metal, 8 crystal, 10 ferrite, the relay and foundry patterns, at
conversion 0.45, its cooldown just begun, on the start system read as an ore
and salvage world so it has room to print — the start system carries five
or six autonz, and a hub of nine there starved at once):

| Seed | Start | Share 0 / 10k / 20k / 30000 | Converts | Mines | Prints by type | Bronze prints | Units at the end (* bronze) | Crew start / end (cap) | Stage | Cogitator tick | Techs (first tick) | Core / pylons / foundry / converters / defence | Pylon / foundry tick | Ferrite refined | Signals | Cancelled |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | node | 0.12 / 0.27 / 0.40 / 0.52 | 300 | 26 | wire_placer 1 | 0 | enforcer 1, wire_placer 2 | 4 / 3 (5) | 1 | 30 | relay_pattern 6090 | 1 / 0 / 0 / 0 / 0 | - / - | 0 | 1 | 10 |
| 2 | node | 0.04 / 0.23 / 0.47 / 0.47 | 341 | 0 | relay_drone 1, wire_placer 1 | 0 | enforcer 1, wire_placer 4 | 4 / 5 (9) | 1 | 30 | relay_pattern 6210 | 1 / 1 / 0 / 0 / 0 | 11400 / - | 0 | 0 | 0 |
| 3 | node | 0.12 / 0.28 / 0.42 / 0.42 | 223 | 46 |  | 0 | enforcer 1, wire_placer 3 | 4 / 4 (5) | 1 | 30 | relay_pattern 6090 | 1 / 0 / 0 / 0 / 0 | - / - | 0 | 0 | 8 |
| 4 | node | 0.12 / 0.23 / 0.40 / 0.42 | 227 | 23 |  | 0 | enforcer 1, wire_placer 3 | 4 / 4 (5) | 1 | 30 | relay_pattern 6210, signal_range_1 15690 | 1 / 0 / 0 / 0 / 0 | - / - | 0 | 0 | 0 |
| 5 | node | 0.12 / 0.25 / 0.38 / 0.42 | 222 | 7 |  | 0 | enforcer 1, wire_placer 3 | 4 / 4 (5) | 1 | 30 | relay_pattern 6180 | 1 / 0 / 0 / 0 / 0 | - / - | 0 | 0 | 4 |
| 6 | node | 0.04 / 0.20 / 0.36 / 0.47 | 341 | 7 | relay_drone 1, wire_placer 1 | 0 | enforcer 1, wire_placer 4 | 4 / 5 (9) | 1 | 30 | relay_pattern 6210 | 1 / 1 / 0 / 0 / 0 | 14040 / - | 0 | 0 | 4 |
| 7 | node | 0.04 / 0.23 / 0.42 / 0.46 | 341 | 0 | relay_drone 1, wire_placer 1 | 0 | enforcer 1, wire_placer 4 | 4 / 5 (9) | 1 | 30 | relay_pattern 6750 | 1 / 1 / 0 / 0 / 0 | 11010 / - | 0 | 0 | 0 |
| 8 | node | 0.04 / 0.18 / 0.40 / 0.46 | 327 | 0 | relay_drone 1, wire_placer 1 | 0 | enforcer 1, wire_placer 4 | 4 / 5 (9) | 1 | 30 | relay_pattern 6480 | 1 / 1 / 0 / 0 / 0 | 13350 / - | 0 | 0 | 0 |
| 9 | node | 0.12 / 0.27 / 0.40 / 0.42 | 226 | 4 |  | 0 | enforcer 1, wire_placer 3 | 4 / 4 (5) | 1 | 30 | relay_pattern 6150 | 1 / 0 / 0 / 0 / 0 | - / - | 0 | 0 | 4 |
| 10 | node | 0.04 / 0.20 / 0.37 / 0.44 | 315 | 13 | wire_placer 1 | 0 | enforcer 1, wire_placer 4 | 4 / 5 (9) | 0 | 30 | relay_pattern 6240 | 0 / 0 / 0 / 0 / 0 | - / - | 0 | 0 | 1 |
| 1 | hub | 0.47 / 0.60 / 0.76 / 0.86 | 345 | 26 | enforcer 2, harvester 1, wire_placer 3 | 0 | enforcer 6, harvester 2, relay_drone 1, wire_placer 7 | 10 / 16 (16) | 1 | 30 | signal_range_1 9150 | 1 / 1 / 1 / 0 / 0 | 30 / 30 | 2 | 0 | 4 |
| 2 | hub | 0.47 / 0.66 / 0.82 / 0.94 | 438 | 0 | enforcer 1, harvester 1, wire_placer 3 | 0 | enforcer 5, harvester 2, relay_drone 1, wire_placer 7 | 10 / 15 (22) | 1 | 30 | signal_range_1 9360 | 1 / 1 / 1 / 0 / 0 | 30 / 30 | 0 | 0 | 0 |
| 3 | hub | 0.47 / 0.62 / 0.77 / 0.88 | 362 | 14 | enforcer 2, harvester 1, wire_placer 3 | 0 | enforcer 6, harvester 2, relay_drone 1, wire_placer 7 | 10 / 16 (16) | 1 | 30 | signal_range_1 9180 | 1 / 1 / 1 / 0 / 0 | 30 / 30 | 2 | 0 | 5 |
| 4 | hub | 0.47 / 0.60 / 0.76 / 0.90 | 395 | 16 | enforcer 1, wire_placer 2 | 0 | enforcer 5, harvester 1, relay_drone 1, wire_placer 6 | 10 / 13 (16) | 1 | 30 | signal_range_1 9210, seeker_pattern 18300 | 1 / 1 / 1 / 0 / 0 | 30 / 30 | 0 | 0 | 0 |
| 5 | hub | 0.47 / 0.61 / 0.78 / 0.92 | 414 | 9 | enforcer 1, wire_placer 2 | 0 | enforcer 5, harvester 1, relay_drone 1, wire_placer 6 | 10 / 13 (16) | 1 | 30 | signal_range_1 9120 | 1 / 1 / 1 / 0 / 0 | 30 / 30 | 0 | 0 | 1 |

Ticks are counted from the landing (750 a day). "Prints by type" are the
director's print orders (the materialised bodies are not prints); "Units at
the end" are the workers standing; "Cogitator tick" 30 is the first check:
materialise places a fresh node's cogitator (the stage's structures), so the
cogitator rung only rebuilds it; "Pylon / foundry tick" likewise reads 30 on
a hub start that materialised them. The run passes: two node misses (seeds 2
and 7: no ore in dig reach, as in Phase E) and one hub miss (seed 2's site
again), at most two allowed per start.

Against the plan's targets:

| Measure | Target | Measured |
|---|---|---|
| conversion | node within 0.03 of Phase E at 20,000; +0.25 by 30,000 | **well above Phase E** (0.35 to 0.47 at 20,000 against Phase E's 0.17 to 0.26), +0.30 to +0.43 by 30,000: the crew now works nights and wires at its cap |
| prints | follow `crewCap`; every unlocked type on hub seeds | yes; on hub seeds placers, enforcers and harvesters printed, the relay drone materialised; seekers known on one hub seed by the end, none printed yet |
| cogitator | by 6,000 on 9 of 10 | all, at materialise |
| first research | relay pattern by 15,000 on 8 of 10 | all node seeds, ticks 6,090 to 6,750 |
| hub: core, pylon, foundry, ferrite | core standing; pylon within 2 days of the pattern; foundry within 3 days; ≥ 10 ferrite refined | core on every hub; node seeds that reached the hub deployed their pylon 6 to 10 days after the pattern; hub starts materialised pylon and foundry; ferrite refined 0 to 2 (the start's 10 held the craft rung's want) |
| bronze | a bronze print once a bronze entry is known | no bronze entry was reached in forty days (two or three entries live: research runs about 300 ticks of one placer per abstract point, as planned) |
| raid | never | none |

The autonz soak varies a little run to run (a second run's rows differ by a
few converts and up to 0.04 of share); **the hive soak varies more**: the
same binary run twice on hive seed 1 gave 20 and 24 bodies at 9,000 ticks.
So E2's recorded hive table cannot be matched row for row. After this
phase's changes it passes its thresholds with its numbers in E2's ranges
(resin 0.27 to 0.42 at 30,000, populations at their capacity, warriors on
the seeds that learned the brood).

**vmtest**: `factions_autonz` keeps its frames and asserts; the frames show
the new sprites (the claw-walker placers, the red-bar tripod enforcers) and,
since materialise now places them, the Signal Core, the cogitator and a relay
pylon. `lore_autonz` differs in sprite only (the wild wire placers are the
claw walker). New: `factions_autonz_hub` (seed 65, Rival factions 4, day 120
on Leaden Ridge: the hub with its core, cogitator, foundry, relay pylon, two
defence pylons, bronze enforcers beside steel placers, a harvester, a seeker
and a sentinel; the Factions tab row) and `factions_autonz_observer` (the
same in an observer run, `debug tech` granting the bronze enforcer before
viewing). New vmtest commands `tech` and `autonz`; the sidecar gains
`autonz`. The plan's "after `debug days 40` on the loaded level" frame was
not built: `debug days` steps only Tier 1 and leaves a loaded level alone
(E2's finding), so the hub script lands at day 120 instead, where Tier 1
has already brought the foundry and sentinels. The cheat console's
`faction tech` is the plan's `faction tech <site> <id>` with the faction in
place of the site (the site is the one on screen).

**On the ground in an observer run** (seed 65, Rival factions 4, Leaden
Ridge viewed): at day 30 a node (8% converted): the node chamber, the
cogitator, four steel wire placers and one steel enforcer, the relay pattern
known. At day 100 a hub (40% converted): the Signal Core, the cogitator, the
foundry and a relay pylon; four steel wire placers, two **bronze**
enforcers, a harvester and a relay drone (the bronze enforcer entry known);
a day later the harvester and the relay drone were gone (the planet's xenos
share it). At day 120 the same hub with two defence pylons and a sentinel,
a seeker, three placers.

**Not verified / not built**: the harvester's repairs, contact by pylon
sight, mine orders preferring harvesters (above); a live run to the hub
compared against a materialised one (kinds and counts agree by
construction and the Go tests; positions only where the grid's first free
site is the same); a hostile autonz raid led by a warden and a full
transmit-to-signal day played in the VM (both in Go tests only); the
player's side (F).

**Balance questions (the owner's call)**: live conversion now runs well
ahead of Phase E's (keep the `at_cap` wire and the night shift, or slow
the convert rung); live research is slow (bronze only after about sixty
days at a hub; shorten the durations, or give the cogitator a second
worker); a node's relay pylon comes 6 to 10 days after its pattern (the
relay rung waits for the hub stage); the hive's crew also lacks
`NeverSleep` (its evolve stalls may be night stops, not exhaustion).

