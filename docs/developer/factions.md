# Rival Factions (campaign tier)

Enemy factions that spread across the campaign while the player is
elsewhere: Phases A and B of
[enemy_factions_plan.md](../brainstorm/enemy_factions_plan.md). Phase C0 put
the player's own settlement on a doctrine too (see "The player's
doctrine"), C1 added the director, and C2 the director digging, faction
settlements on loaded levels, the mirror colony on its ship, hostility and
ConvertAction (see those sections). Phase D added raids and hunting, ships
proper (tiers, holds, landing and lifting off live), the **greyish** and
their mothership, interception in space and ruins on departure (see "Raids
and hunts" onward). Phase E added the **autonz** (see "The autonz"), and
Phase E2 made the hive a settlement faction (see "The hive as a
settlement"). The
factions are the **hive** (xenos and troids merged: one organism, two
morphs), the **mirror** colony, the **greyish** and the **autonz**. A presence is a small record on a location, stepped once per
star-date day (Tier 1); when its site's level is built it is materialised
into its camp, workers and structures, and counted back into the record when
the level is saved or frozen (Tier 2). Eggs it fires travel the Star Map as
fleets. Code: `internal/campaign/faction.go` (the record, the step, fleets,
reconcile rules), `internal/game/faction_level.go` (the level side),
`internal/factory/type.go` (unit types at spawn).

## How a faction moves and acts

Three mechanisms decide what a faction does, and each doctrine uses some
of them. None moves materials or makes things by fiat on a loaded level:
there, workers fetch, haul, build and fight as the player's do.

- **The settlement director** (`internal/director`, [The director](#the-director))
  decides what a camp on a loaded level does: every few ticks it walks its
  doctrine's needs ladder (food, materials, shelter, storage, research,
  digging, arms, raids) and gives the orders a player would, through the
  player's own order paths. It runs every faction settlement (a doctrine
  with a `settlement` block or `settlement_from`), an observer run's
  expedition colony, and under the [observer lock](#observer-mode) the
  player's own colony on every planet.
- **The expedition director** (the expedition table:
  `internal/campaign/doctrine_expedition.go`, flown by `fleetView`,
  `fleetMove` and `DepartShip` in `faction_ships_d.go`; see
  [Observer mode](#observer-mode)) decides where a ship goes and when it
  lands, moves on or packs up, from a doctrine's `expedition` table (fuel,
  site tier and tags, known presences, unvisited sites). It flies every
  fleet-carried colony-doctrine presence: the **mirror colony**'s ship
  (`expedition_from: colony`; [Ships](#ships)) and an observer run's
  **expedition**. A fleet doctrine with no table keeps its `stay_days` hop:
  that is the per-doctrine override, and the greyish use it to keep their
  land, salvage, leave rhythm (stay and hold, then home or on).
- **The mobility rules in the daily step** ([The daily step](#the-daily-step))
  spread a faction with no planner behind them: the hive's launcher fires
  eggs ([Fleets](#fleets)), the autonz signal and wake sleeper nodes
  ([The autonz](#the-autonz)), and a fleet-carried doctrine's ships land,
  stay and lift off ([Ships](#ships)), asking their table first when they
  have one.

| Doctrine | Carrier | Mobility kind | Settlement director | Expedition director | Hostility default |
|---|---|---|---|---|---|
| [hive](#the-hive-as-a-settlement) | planet | `launcher` | yes (phase E2: drones, eggs on larvae, resin, the evolve tree) | no | hostile (no diplomacy) |
| [rival colony](#the-mirror-colony) (`colony!rival`, faction key `mirror`) | ship | `fleet` | yes (the colony's) | yes (the colony's) | neutral until the player lands on its site |
| [greyish](#the-greyish) | ship (and mothership) | `fleet` | yes | no (the stay and hold rhythm) | hostile |
| [autonz](#the-autonz) | planet | `signal` | yes (phase E3: typed prints at the fabricator and foundry, structures on wire, the assimilation tree, the transmit walk) | no | hostile (no diplomacy) |
| [colony](#the-players-doctrine): the player | the player's ship | `ship` | under the observer lock | no (the player flies) | n/a |
| [colony](#observer-mode): an observer run's expedition | ship | `ship` (a fleet in the step) | yes | yes | neutral (no player to be hostile to) |

Planned, not built: playable races (phase F) running another doctrine's
blocks for the player; faction against faction on the campaign tier;
taking over an observer run's expedition mid-run.

## The record

`Location.Factions []FactionPresence`, one per faction on the site:

| Field | Meaning |
|---|---|
| `id` | `fp<n>`, unique in the campaign (`Campaign.FactionSeq`) |
| `faction`, `doctrine` | doctrine key in `data/factions.json` (the same key for every Phase A faction) |
| `population`, `growth` | bodies, and the fractional progress toward the next one (negative toward a loss) |
| `stage` | index into the doctrine's `stages`; only rises |
| `stockpile`, `conversion` | strip / convert tallies; unused by the hive, reserved for Tier 2 |
| `arrived_day`, `last_step_day`, `last_shot_day` | star dates; `last_shot_day` is the launcher cooldown |
| `known_techs`, `research_pts` | the doctrine's research walk |
| `seed` | makes its materialisation deterministic |
| `known`, `events` | whether the player has seen it, and the live events recorded while unseen |
| `materialized`, `level_stage` | built onto its site's level, and the highest stage whose stamp stands there |
| `launcher_lost`, `launcher_lost_day` | its launcher was destroyed on the level; it regrows after `cooldown_days` and the hive cannot fire meanwhile |
| `dormant`, `wake_day` | phase E: founded by a signal and asleep until `wake_day` (no growth, work, research or spread; a sleeper node on its level) |

Known techs live on the presence for every faction, the player's included:
`Campaign.Player` is the player's settlement as a presence (id `player`,
faction and doctrine `colony`), on no location, so the daily step never
touches it. `Campaign.KnownTechs()`, `HasTech` and `UnlockTech` read and
write its `known_techs`.

Campaign fields: `faction_count` (the New Expedition choice), `faction_day`
(last day stepped), `faction_seq`, and `fleets` / `fleet_seq` (things in
flight, see Fleets). `Location.Husks` are presences that died out on their
own, waiting to be stamped on the level.

`Location.Tags` keeps the archetype's tags (`ore`, `biomass`, `hostile`,
`radioactive`, `hive`, `salvage`) for carrying capacity.

## `data/factions.json`

```json
{
  "pressure_per_faction": 0.4,
  "doctrines": {
    "hive": {
      "name": "Hive", "singular": "hive", "seedable": true,
      "morphs": { "crawler": { "name": "Xenos", "singular": "xeno", "tag": "XENO", "blueprint": "xeno" },
                  "flier":   { "name": "Troids", "singular": "troid", "tag": "TROID", "blueprint": "troid" } },
      "morph_by_scenario": { "troid_outbreak": "flier" }, "default_morph": "crawler",
      "home_scenarios": ["exterminate_xenos", "troid_outbreak"],
      "seed_population": 3, "badge": [214, 96, 230], "economy": "biomass",
      "mobility": { "kind": "launcher", "spread_share": 0.7, "spread_min": 5,
                    "crowded_capacity": 12, "base_range": 25,
                    "cooldown_days": 50, "payload": "xeno_egg",
                    "shot_pop": 3, "egg_pop": 2, "egg_speed": 0.5 },
      "growth": { "per_day": 0.065, "starve_per_day": 0.05 },
      "capacity": { "base": 6, "min": 3,
                    "tags": { "biomass": 14, "hive": 12, "hostile": 4, "ore": 2, "salvage": 4, "radioactive": -4 },
                    "kinds": { "station": 2, "asteroid_field": -2, "moon": -2 } },
      "hazard": { "zombie_plague": 0.5, "crystalline_blight": -0.5,
                  "exterminate_xenos": 1.25, "troid_outbreak": 1.25 },
      "stages": [ { "pop": 0, "name": "brood", "mix": { "drone": 1.0 } },
                  { "pop": 6, "share": 0.3, "name": "nest", "stamp": "hive_nest", "w": 13, "h": 13,
                    "mix": { "drone": 0.5, "warrior": 0.2, "burrower": 0.1, "scout": 0.1, "hunter": 0.1 },
                    "structures": { "egg_cluster": 1, "evolution_chamber": 1 } },
                  { "pop": 12, "share": 0.6, "name": "hive", "launcher": true, "stamp": "hive_core",
                    "w": 21, "h": 21, "mix": { "drone": 0.35, "warrior": 0.15, "...": "..." },
                    "structures": { "egg_cluster": 2, "evolution_chamber": 1, "egg_launcher": 1 } } ],
      "research_per_day": 1,
      "research": [ { "id": "hive_warrior_brood", "cost": 25 },
                    { "id": "launcher_range_1", "cost": 40, "range": 15 },
                    { "id": "hive_hunter_brood", "cost": 60 },
                    { "id": "launcher_range_2", "cost": 80, "range": 20 },
                    { "id": "hive_resin_flow_1", "cost": 60, "conversion_rate": 0.5 }, "..." ],
      "aggression": 8,
      "types": { "drone": { "morph": "crawler", "sprite_dx": 0, "name": "Drone", "seen_name": "Xeno", "cost": 2, "...": "..." },
                 "...": "six in all: The hive's creatures" },
      "morph_fallback": { "crawler": "drone", "flier": "scout" },
      "tech_aliases": { "tier_advanced": "hive_warrior_brood", "tier_elite": "hive_hunter_brood" },
      "body_skills": [ "radiation_resist" ],
      "settlement": { "...": "The hive as a settlement" },
      "biomass": { "per_drone": 3, "tag": 0.5, "eat": 0.5, "cap": 60, "per_pool": 10 },
      "conversion": { "tile": "hive_resin", "radius": 14, "per_day": 0.008, "cap": 0.8, "floor": 1.0 },
      "level": { "egg_share": 0.3, "adult_share": 0.4, "launcher": "egg_launcher",
                 "husk": "nest", "husk_w": 11, "husk_h": 11 }
    }
  }
}
```

| Key | Meaning |
|---|---|
| `pressure_per_faction` | the launcher cooldown is divided by `1 + this * (faction_count - 1)` |
| `name`, `singular` | the faction's own names, for code and tools; players never see them while its identity is hidden |
| `morphs`, `morph_by_scenario`, `default_morph` | the body plans players meet and their names (`name`, `singular`, `tag`); a site's morph (`MorphAt`) |
| `seedable` | New Expedition seeding draws from these |
| `seed_population` | a seeded presence's size |
| `badge` | Star Map colour |
| `economy` | the tag spread and seeding prefer (weight 1, +3 per matching tag) |
| `mobility` | `kind` (`launcher` is the only one stepped); the firing threshold `spread_share` of the site's capacity, at least `spread_min`, at most the capacity (`FireThreshold`); `crowded_capacity`, below which the cooldown stretches by `crowded_capacity / capacity` (`CooldownAt`); `base_range` in star-map units (the `FuelCost` metric); `cooldown_days`; `shot_pop` lost per egg, `egg_pop` it lands as; `egg_speed` units per day (at least one day in flight); `payload` (the blueprint Phase B spawns) |
| `growth` | `per_day` fraction added, `starve_per_day` fraction lost above capacity |
| `capacity` | `base` + each of the site's tags + its kind, floored at `min`. A station is the `station` kind with the `salvage` tag |
| `hazard` | growth multiplier by the site's scenario id (from its history). Below 1 slows, negative shrinks the presence every day until it dies. Unlisted is 1 |
| `stages` | reached at `pop`, or at `share` of the site's capacity when that is lower (so a small site still reaches its launcher stage); `launcher` marks a stage that can fire |
| `research_per_day`, `research` | points per day per stage reached; each entry's `cost`, and `range` it adds to the launcher (`conversion_rate` to the conversion rate). The hive's walk keeps Phase A's spine (`hive_warrior_brood`, `launcher_range_1`, `hive_hunter_brood`, `launcher_range_2` at Phase A's costs), then its other evolve entries |
| `aggression` | for materialised units (Phase C) |
| `morphs.*.blueprint` | the prefix of the morph's life-stage blueprints (`xeno`: `xeno_egg`, `xeno_hatchling`, `xeno_stage2`, `xeno_stage3`, `xeno_egg_cracked`) |
| `home_scenarios` | scenarios whose threat is this faction (`exterminate_xenos`, `troid_outbreak`): a presence there builds by the chapter's origin |
| `types`, `morph_fallback`, `tech_aliases`, `body_skills` | unit types (phase E2; [The hive's creatures](#the-hives-creatures)): the colour block, stats, role, tasks, kit and egg of each, the research that unlocks it; where a locked type's share goes; old tech keys; skills on every typed body |
| `stages[].stamp`, `w`, `h`, `mix`, `structures` | the structure script a presence of that stage stands on its level and its footprint; its bodies' share per type (a typed doctrine) or crew blueprint (the autonz); the structures a settlement of that stage stands |
| `biomass` | a biomass economy's abstract daily tally (Tier 1 bookkeeping: [The hive as a settlement](#the-hive-as-a-settlement)) |
| `level` | `egg_share` of a presence's bodies are eggs and `adult_share` of the rest adults (the others stage 2); `launcher` the blueprint at a launcher stage; `husk`, `husk_w`, `husk_h` the stamp an exhausted presence leaves |

## Balance

Tuned 2026-11-08 to: Rival factions 1 about 2–3 hives by day 100, half the
reachable opening sites by day 200, not full before about day 300; Rival
factions 3 about half by day 100, full around day 200; a fresh hive's first
shot 20–30 days after it arrives; a cramped site fires rarely, not never.
Measured with the dump below over seeds 1–6 on the opening map, no player
(a jump costs `ceil(fuel / 3)` days, so day 100 is roughly 12–20 jumps).
Presences / total population, min–max over the seeds:

| Rival factions | Day 0 | Day 40 | Day 100 | Day 200 | Day 300 |
|---|---|---|---|---|---|
| 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |
| 1 | 1 / 3 | 1 / 8–22 | 1–2 / 8–44 | 3–5 / 37–78 | 6–9 / 89–144 |
| 2 | 2 / 6 | 2 / 24–44 | 3–4 / 50–72 | 6–9 / 89–157 | 6–10 / 89–164 |
| 3 | 3 / 9 | 3 / 48–64 | 5–6 / 64–93 | 6–10 / 89–162 | 6–10 / 89–164 |

Each seed's map fills at 6 to 10 presences. Days from arrival to first shot
are 20–37 on an ordinary site and about 75 on a crowded one (capacity 8).
Rival factions 1 at day 100 lands slightly under the target on one seed in
six (seed 4, whose brood starts on a capacity-8 site). `TestFactionPaceRivalOne`
pins the day-40 and day-100 counts for Rival factions 1 so the pace cannot
drift silently; retune it on purpose. Re-measured after Phase B (fleets,
husks, reconcile): the same table, since B changes no Tier 1 number.

## The daily step

`Campaign.StepFactions(loaded)` steps every day from `faction_day` to the
star date. The world manager calls it from `TickBackground` (every world
tick, cheap when the day has not changed) and from `Travel` after the jump's
days are added. `loaded` is `WorldManager.IsLoaded`: presences on a loaded
planet (live or background-ticking) are skipped, since they are simulated
for real (and launch by the live rule, Tier 2), and an egg bound for one
waits in orbit until it unloads. A
day's random draws come from their own stream (`Seed*31 + day*2654435761`),
locations in ID order, so a seed steps to the same presences every time.

Per day: fleets due land first (a new presence, or reinforcement for one of
the same faction; an egg whose target has become ineligible is lost). Then
per presence:

1. **Grow.** `delta = per_day * hazard * pop`, or `-starve_per_day * pop`
   above capacity; the fraction carries in `growth`. Growth stops at
   capacity. Population 0 removes the presence (`faction_wiped`).
2. **Work.** Nothing for the biomass economy in Phase A.
3. **Stage up** to the highest stage whose `pop` is reached
   (`faction_grew`).
4. **Research.** Points accrue from stage 1; each finished entry joins
   `known_techs`.
5. **Spread** (`launcher`): at the firing threshold or more, on a launcher
   stage, with the cooldown passed. The first shot waits only on growth
   (`last_shot_day` is -1, `NeverFired`), except on a crowded site, where
   it waits the stretched cooldown from arrival; after a shot the cooldown
   (stretched when crowded, divided by the faction-count pressure) runs from
   that day, and never while its launcher is lost. The target is an eligible site (below) within
   range, not loaded, with no presence of this faction and no egg of this
   faction already bound for it, weighted toward the economy tag. With none,
   nothing fires and the cooldown is not spent. A shot costs `shot_pop`,
   records `faction_launched` and puts a projectile fleet in the air for
   `ceil(distance / egg_speed)` days.

A presence whose population reaches 0 in the step died out on its own (a
hazard): `faction_exhausted`, and a husk on its site. A lost launcher regrows
once `cooldown_days` have passed since it was lost.

Faction against faction on one site is ignored (the plan's open question).

## Fleets

`Campaign.Fleets []Fleet`: things travelling between locations. Phase B has
one kind, `projectile` (an egg): `id` (`fl<n>`), `from_id`, `to_id`, their
coordinates, `launch_day`, `arrive_day`, `presence` (the brood it lands as:
faction, doctrine, `egg_pop`, the parent's techs) and `known`.
`Fleet.Pos(t)` is its position at star time `t` (`Campaign.StarTime`: the
date plus the day's ticks), interpolated along the line and held at the
target while it waits in orbit of a loaded planet. On arrival it founds a
brood presence (or reinforces one of its faction), built onto the level the
next time it is.

A fleet is known (drawn) when the hive that fired it is known, at launch or
when the player later sees that hive (`RevealPresences`); when a scan sweep
covers it; or when it comes within the ship scanner's range of the ship's
location (checked each day; no scanner, no sighting). Ships (greyish, the
mirror colony) arrived in phase C2: see "The mirror colony".

## Tier 2: the hive on a level

`internal/game/faction_level.go`, `faction_hive.go`. Since phase E2 the hive is
a settlement faction ([The hive as a settlement](#the-hive-as-a-settlement));
the creature path below serves only its husks and any stage doctrine without
a settlement block (none ships).

**Centre.** `buildParked` calls `materialiseFactions` after the quest
fixtures, so the site's history is built first. A presence's centre is the
level's origin flag `faction_<id>` if set, else a site from its seed
(`generation.FactionSiteNear`): the whole largest stage footprint at least
`hazard_keep_out` (40) outside the landing reserve and clear of every stamp,
best open ground of 400 samples. On a home scenario it samples within 30
tiles of the chapter's origin first. The pick is saved as the origin flag.

**Husks** are stamped once: the doctrine's `husk` (`nest` with cracked eggs
of the site's morph) at the dead presence's centre, its remaining bodies
removed.

**Reconcile.** `writeLocation` (every save and every freeze) calls
`reconcileFactions`, which hands `Campaign.ReconcilePresence` each
presence's living body count and whether its launcher stands: the count
becomes `population` (the stage rises with it, never falls), a launcher
stage whose launcher is gone is marked lost, and no bodies left is a wipe:
`faction_wiped`, made known, the presence dropped. A presence not yet built
onto the level is left alone. `presenceBodies` leaves out structures,
larvae and shells; `presenceLauncher` matches a doctrine's launcher
blueprint (the hive's heart and clusters are Kin structures too).

**Live launch.** Tier 1 skips loaded planets, so once a star-date day
`WorldManager.stepLiveFactions` runs `Campaign.LiveLaunch` for every
presence on a loaded level: the live count becomes the record and the Tier 1
rule fires, with the same `last_shot_day` cooldown and only while the
launcher stands. For the hive the launch is physical: it fires only with
`shot_pop` of its workers standing within the launch rung's `walk` of the
spire, and those are the bodies that leave; with nobody there it waits a
day and the cooldown is not spent. On the planet in orbit an alert names
the morph.

The GameMaster is unchanged: it spawns a scenario's waves, and every hive
body, wild or settled, counts toward its `hostile_max`.

## The hive's creatures

`data/blueprints/entities/hive.json`: one blueprint per morph and life stage
plus each morph's egg and cracked shell, all faction `hive` (so xenos and
troids never fight each other). Ids: crawlers `xeno_*`, fliers `troid_*`
(kept from the two old files, so history evidence, quests, objectives and
saves still name them); fliers carry `flying`. Each has a `Kin` component
(`internal/components/KinComponent.go`): `morph`, `type`, `presence`,
`settlement`. `egg_launcher` is the Resin Spire, morph `structure`.

**Wild and settled.** A scenario's and the GameMaster's creatures, and a
husk's, are **wild**: scripted, ageing egg → hatchling → stage 2 → stage 3,
olive or blue, named "Wild Xeno", "Wild Troid Egg" and so on (the blueprint
names; the ids are unchanged). A presence's bodies are **settled**
([The hive as a settlement](#the-hive-as-a-settlement)). `find_nearest_worker`
skips workers of the caller's own faction, so a wild xeno never hunts the
hive's drones.

The stage scripts are shared (`data/scripts/hive/egg`, `hatchling`,
`stage2`, `stage3`, `nursery`, and `launcher`, which does nothing). They grow
and lay through `spawn_kin(stage, x, y, z)`: the blueprint is the creature's
own prefix plus `stage` (`xeno_egg` hatches `xeno_hatchling`), with its type,
presence and settlement passed on (`factory.SpawnKin`). A typed egg hatches
after its type's `hatch_turns` (seeded as the `hatch_turns` var at spawn), a
wild one after 20.

**Types** (phase E2; they replaced Phase B's research tiers). The colour block
is a unit's type, not its strength: `factory.ApplyType(e, type, presence)`
adds the type's `sprite_dx` to body, egg and shell alike, names it by its
`seen_name` ("Red Xeno", "Red Xeno Egg"), seeds an egg's hatch time, and
gives an adult its absolute stats, initiative, skills, kit (the spitter's
gland), role and allowed tasks, with its presence's known upgrades folded in
(`factory.TypeSource`, wired to the doctrine and research by
`internal/game/faction_types.go`); a type whose second upgrade is known
draws its working adults in the stage-3 frame. A wild creature has no type.
Phase B saves map a body's `tier` to the type of that colour.

| Type | Morph | Colour (egg) | Role, tasks | Unlocked by |
|---|---|---|---|---|
| drone | crawler | olive | worker: chop, mine, haul, build, convert, craft, research | start |
| scout | flier | blue | scout: none (patrols) | start |
| warrior | crawler | red | guard: attack | `hive_warrior_brood` |
| hunter | flier | orange | hunter: attack | `hive_hunter_brood` |
| burrower | crawler | purple (pink egg) | digger: mine, dig, haul, attack | `hive_burrower_brood` |
| spitter | flier | green | spitter: attack (the spit gland, range 6) | `hive_spitter_brood` |

Every typed body also gets the doctrine's `body_skills` (`radiation_resist`:
a moon's surface radiation killed every hive body in about 100 ticks).

## The hive as a settlement

Phase E2 ([hive_plan.md](../brainstorm/hive_plan.md)). The hive runs on the
same doctrine, settlement and director machinery as the mirror, the greyish
and the autonz, so that F can hand it to the player with no new code paths.

**Doctrine** (`data/factions.json`, `hive`): `types` (above),
`morph_fallback` (the type a locked type's share goes to: crawler → drone,
flier → scout), `tech_aliases` (`tier_advanced`, `tier_elite` → the warrior
and hunter broods; renamed in old saves on load), stage `mix` by type and
the stage's `structures` (nest: an egg cluster and the evolution chamber;
hive: two clusters, the chamber and the spire). On a troid world the
fighting types swap with their other-morph counterparts (warrior ↔ hunter,
burrower ↔ spitter); the drones and scouts keep their shares.

**Settlement block.** Crew by type (drone 0.75, scout 0.25); `diet`
`["biomass"]` (workers eat a unit of a biomass stack for 30 nutrition;
`workerai.SettlementDiet`); build menu `Orders` and `Hive` (pool, egg
cluster, evolution chamber, Resin Spire, each with `required_floor:
hive_resin`, checked by the build order path the cursor and every director
share and by the director's site pick); research, the 20 `hive_*` evolve
entries; units, the six egg recipes; `larvae`; `kill_drops` (`biomass: 3`:
every kill by a hive worker puts three into its bag); `bloom`; a staged camp
with the heart as its station and the biomass pool as its storage.

**Larvae and eggs.** The heart and every egg cluster lay a grey egg
(`hive_larva`, a crafting station) every 250 ticks on an open resin tile
beside them while the settlement holds fewer than `cap` (3) + `per_cluster`
(2) per cluster (`stepHiveLarvae`). An egg recipe (`hive_egg_drone` …) has
the larva as its station, `consume_station` and `kin_type`: a drone carries
its biomass there and works it, and the larva is replaced on its tile by the
egg of that type, of the crafter's presence and settlement (`Kin.Settlement`).
The egg hatches through the egg script; a settled hatchling runs
`nursery.basic` (stays within 4 tiles, fights only when attacked) and grows
into the morph's crew body (`xeno_crew`, `troid_crew`): a Worker of the
settlement with no script, typed (`factory.KinStageOverride`, `KinPrepare`).

**Resin.** `hive_resin` (the stamps' floor art) has a tile `speed` map:
`{ "hive": 0.75, "*": 1.4 }` multiplies the initiative refill of a walker
standing on it (`internal/game/resin_speed.go`, a wrapper round the rogue
library's initiative system; fliers, eggs and structures are untouched). The
nest and hive stamps lay it; the director's resin rung lays it outward with
`ConvertAction`s. The presence's `conversion` is its **creep**: the resin
share of the radius-14 patch beyond its camp's own floor (the brood's 3×3
patch, the chamber's circle), so a view or a stage stamp does not move the
record (`campResinRadius`, `generation.ConversionCounts`). `floor: 1.0`: resin
does not cap capacity.

**The resin bloom** (the plan's open question 7, built after the soak found
the hive starving: a tree drops one biomass): every 150 ticks a
`resin_bloom` (a choppable dropping two biomass) grows on an open resin tile
within the radius while fewer than one per 12 resin tiles stand
(`stepHiveBloom`). Drones chop it like any flora.

**Materialise** (`materialiseHive`, the settlement path): the settlement
record; the staged camp (`buildHiveCamp`: the brood's resin patch, or the
stage's stamp with the hive's own things lifted out and put back; the heart
at the centre, a pool); resin to the record's creep; the stage's structures
on resin sites of the director's grid (the spire only at a launcher stage
not lost); pools for the stock (one per 100 biomass); the stock into them;
Phase B's scripted bodies adopted (an adult replaced by its type's crew body
on its tile, an egg or hatchling marked the settlement's); bodies topped up
or culled to `units` type by type (`egg_share` as eggs on resin by the
heart, the rest as working adults); larvae to the record within the cap.
Idempotent and deterministic.

**Reconcile** (`reconcileHive`): `units` by type (eggs and young as their
type), `population`, `larvae`, the pools' stock, the creep and the techs.
**Live** (`stepLiveHive`, daily): the live counts become the record, and a
stage reached has its chamber stamped at once; the stage's structures are
the director's to build.

**The director table** (in order; `max_open` 3, radius 14, reach 50):

| Rung | What it orders |
|---|---|
| `drones` | a drone egg on the free larva nearest the heart while drones (adults, young, eggs) are under 2 + 0.35 × bodies, leaving 0.5 biomass a body |
| `forage` | chop the nearest choppable dropping biomass (flora, bloom) while stock is under 40 |
| `haul` | loose biomass into a pool (chopped drops get their own retrieve orders) |
| `resin` | `ConvertAction`s outward from the heart until 60 + 15 per worker tiles within the radius are resin |
| `pools` | a pool (1 + 0.1 per worker) |
| `clusters` | an egg cluster (0.08 per worker, at most the stage's count), from the nest |
| `guard` | warrior eggs to a 0.2 share; guards hold posts 6 out round the heart, and every guard is set on a hostile inside 12 (the player's colonist when hostile, a scripted or faction mob of another faction), recalled once it is dead or gone |
| `chamber` | the evolution chamber, from the nest |
| `evolve` | the next evolve entry, at the chamber |
| `brood` | the egg of the type most under the stage's mix, among unlocked types, up to the site's capacity |
| `spire` | the Resin Spire, at the hive stage |
| `launch` | on a day the launcher would fire (`Campaign.LaunchReady`: the Tier 1 check without its random draw), `shot_pop` workers, drones then scouts first, walk to the spire and hold there (`Orders.Hold`) |
| `scouts` | idle scouts to waypoints on a ring 30 out every 600 ticks; a scout within 8 of the player's colonist or storage sets the level flag `contact:<settlement>` |
| `excavate` | when the resin rung has no ground left to lay on, dig orders on the rock nearest the heart (burrowers dig) |
| `raid` | the raid rung, only after contact (`scouted`), warriors, hunters, spitters and burrowers |
| `hunt` | attack orders on the nearest creature not of the hive's faction while stock is under 10 + 1 per worker (`hunt: "biomass"`) |

New director pieces: the typed grow rung (`grow` takes `true` or an object
with `type`, `min`, `share`, `reserve`), `stage_min` on any rung, the build
rung capped by `Site.Structures`, and `Orders.Move` / `Orders.Hold`
(`task_requests.MoveAction`: a plain walk; with `HoldHere` the worker stays
until released).

**Evolution.** `data/research.json` entries with `"tree": "hive"` and an
`evolve` block (`unlock`, `type` with `health` / `str` / `initiative` /
`work` multipliers, `evolved` for a type's second upgrade, `range`,
`resin_rate`, `flier_speed`), researched at the evolution chamber, durations
in work steps (one per worker turn). The Tier 1 walk keeps Phase A's spine:
`hive_warrior_brood` 25, `launcher_range_1` 40, `hive_hunter_brood` 60,
`launcher_range_2` 80, then the rest, so the launcher range grows on the same
day. A finished upgrade re-types the living (`factory.Retype`).

**Tier 1 bookkeeping** (`internal/campaign/doctrine_types.go`,
`faction_hive.go`): `units` (a new body to the type most under the stage's
mix among unlocked; starvation from the type most over, drones last; a shot
takes drones, then scouts; a landed brood is drones), `larvae` (+3 a day,
held at the stage's cap, one spent per new body), the biomass stock (the
`biomass` block: 3 per drone × 1.5 on a biomass site, less 0.5 a body and
each egg's cost, held under 60 + 10 per pool), and resin (the convert
formula, `per_day` 0.008). None draws a random number or touches
`population`: `TestFactionPaceRivalOne` and the history-dump table are
Phase A's.

**Scenarios.** `troid_outbreak` and `exterminate_xenos` stay wild; their
`threat_blueprints` include `xeno_crew`, `troid_crew` and `hive_larva`, and
the threat check counts static entities too, so a hive's grey eggs keep the
threat alive.

**Hidden identity.** Bodies carry their type's `seen_name` (the morph with
its colour); `name` ("Warrior") waits for B+. The raid alert names the
morph. The Factions tab lists a hive's units by type: every type by name in
an observer run, only the site's morph's by seen name in a normal campaign.

## Factions and doctrines

Since rival selection (rival_selection_plan.md) a faction is an
**instance** of a doctrine, not the doctrine itself. `Campaign.FactionTable`
holds a `Faction` row per instance (key, doctrine, name, badge, slot,
group, ordinal); `EnsureFactionTable` (at `NewWorldManager` and on load)
gives every faction the campaign holds a row, the player in group 1.
Keys are opaque: one faction of a doctrine keeps the doctrine's key (so the
count path and old saves are unchanged), a second is `<key>.2`. Every
lookup by a faction key goes through `Campaign.DoctrineOf` (or the game's
`doctrineOfFaction`). Per faction: the stance toward the player, techs, the
mothership and hoard, spread targets, the Star Map badge and tag, the
Factions tab block and Legends lines (a second of a kind is numbered,
"Xenos 2", while the doctrine's name is hidden). **Groups**:
`Campaign.Hostile(a, b)` (same group friendly, else hostile) is what bodies
consult on a level (`components.HostileFactions`: the scripted AI's enemy
test, the guard and hunt rungs); a faction of the player's group is neutral
to the player and never turns; no faction spreads onto its own side's world
(`HeldBySide`). A second instance's bodies, eggs, young and structures carry
its key, never their blueprint's. A **doctrine view** (`<key>!rival`, from a
doctrine's `rival` block) is what a rival faction of the doctrine runs: the
colony's is the rival colony (see "The mirror colony"). New Expedition's
**rival boxes** pick each rival's doctrine (or Random, rolled from the seed)
and group (`GenOptions.Rivals`, `seedRivals`); without them the Rival
factions number seeds by `seed_from` as before (`seedFactions`).

## The player's doctrine

Phase C0: the player's settlement reads what it can build, research, craft
and crew from a doctrine, so a playable race is a doctrine entry plus a
title-screen choice (phase F). The game's is `colony` in
`data/factions.json`, which reproduces the game as it was before doctrines:

```json
"colony": {
  "name": "Colony", "singular": "colony", "playable": true, "seedable": false,
  "economy": "industry", "mobility": { "kind": "ship" },
  "settlement": {
    "crew": [ { "blueprint": "colonist", "share": 1.0 } ],
    "needs": true,
    "build_menu": [ { "id": "orders", "label": "Orders" }, { "id": "walls", "label": "Walls" }, ... ],
    "buildables": [ "hull_wall", "bunker_wall", ... ],
    "research": [ "basic_metallurgy", ... ],
    "units": [ "print_colonist", "hauler_robot", "excavator_robot", "security_robot", "construction_robot" ],
    "recipes": [ "refine_fuel", "medkit", ... ]
  }
}
```

| Key | Meaning |
|---|---|
| `playable` | the player's settlement can run this doctrine |
| `seedable` | false: the colony is never seeded as a rival |
| `mobility.kind` | `ship`: the campaign shape (`Doctrine.CampaignMode`, `ship` or `launcher`); declared, read in F |
| `economy` | `industry` (build, mine, craft, research); matches no location tag; declared |
| `settlement.crew` | the starting crew: blueprints and shares. `SeedShipCrew(n)` spawns `CrewBlueprints(n)`, `n` split by share with the remainder to the first entry |
| `settlement.needs` | the crew eats and sleeps; declared (the blueprints carry the `Needs` component), read in F |
| `settlement.build_menu` | the build menu's categories in order (`id`, `label`). Each lists the build.json entries whose `category` is its id and that are in `buildables`, by `order` then name. `orders` holds the cursor-mode orders, which stay in code |
| `settlement.buildables` | build.json keys the settlement may build |
| `settlement.research` | research.json keys in its tree; the research modal lists only these, and queuing another is refused |
| `settlement.units` | crafting recipes that make a unit (a printed colonist, a robot) |
| `settlement.recipes` | every other crafting recipe it may make; the crafting modals list a station's recipes that are in `units` or `recipes`, and queuing another is refused |

The lists name entries of the shared content files (build.json,
research.json, crafting_recipes.json), which keep their own schemas. **New
content must be added to the colony's lists** or it drops out of the
player's game; `TestColonyDoctrineCoversContent` fails until it is.

**Selection.** `campaign.PlayerDoctrine(c)` is the one place: the doctrine
of `Campaign.Player`, else `DefaultPlayerDoctrine` (`colony`, also with no
campaign). A new campaign's player presence takes `GenOptions.Doctrine`
(`""` is the colony). The hive has no `settlement` block yet.

**Readers** (`internal/game/player_doctrine.go`): `MainState.applyDoctrine`
hands the HUD the menu (`gui.SetBuildMenu`) when the GUI is built and when a
campaign is attached; `researchSnapshot` and `addResearchTask` use the tree;
the station click and `addCraftTask` use `units` and `recipes`. A recipe's
`requires_tech` is checked against the techs of whoever orders it: in a
campaign the player's live on the campaign record (`MainState.hasTech`),
a faction settlement's on its settlement (`orderCraft` takes the known-tech
func, as `orderResearch` does), and the bench checks again through
`workerai.KnowsTech`, wired to the campaign for player settlements;
`SeedShipCrew` and the non-campaign start use the crew; the unlock-all
debug option unlocks the tree.

**`deploy_on_craft`** (a crafting recipe flag, finding 4): the output is put
on the level at the station as a worker of the crafter's settlement instead
of into the crafter's bag. `print_colonist` sets it; the Robot Workbench's
robots do not (they go into the bag and deploy when put down).

## The director

Phase C1: `internal/director`. What the player's cursor is to colonists, the
director is to a settlement. Every `every_ticks` it reads its level and its
doctrine's `settlement.director` table and gives the orders a player would,
through the player's own order paths (`director.Orders`, implemented in
`internal/game/director_orders.go` with the cursor's functions:
`queueBuild`, `queueMine`, `queueRetrieve`, `addResearchTask`,
`addCraftTask` and the Cancel Task order). It never moves materials or makes
anything itself: workers fetch, haul and build as they do for the player, so
physical storage holds. Phase C1 runs it on the player's settlement only
(observer mode); a faction settlement is a doctrine with its own table.

```json
"director": {
  "every_ticks": 30, "max_open": 2, "radius": 10, "reach": 40,
  "max_mines": 2, "max_hauls": 3, "stuck_ticks": 3000,
  "dig_depth": 6, "dig_reach": 80,
  "priorities": [
    { "id": "food", "haul": "food" },
    { "id": "materials", "mine": { "metal_ore": 12, "crystal": 6 } },
    { "id": "shelter", "build": "bed", "per_colonist": 1 },
    { "id": "storage", "build": "storage_locker", "min": 1, "per_colonist": 0.5 },
    { "id": "workbench", "build": "basic_workbench", "min": 1 },
    { "id": "lab", "build": "research_lab", "min": 1 },
    { "id": "research", "research": true },
    { "id": "dig", "dig": true },
    { "id": "defences", "build": "gun_bench", "min": 1 },
    { "id": "arms", "craft": "stun_baton", "per_colonist": 0.5 }
  ]
}
```

| Key | Meaning |
|---|---|
| `every_ticks` | ticks between plans |
| `max_open` | open orders that spend materials (build, research, craft) at once; the ladder's higher rungs take them first |
| `radius` | how far from the settlement centre it builds, and counts structures (and stations, the lab) as its own |
| `reach` | how far it looks for deposits, choppables and loose items, and how far its reachability flood runs |
| `max_mines`, `max_hauls` | open mining orders per resource, open haul orders |
| `stuck_ticks` | an order of its own with no progress (work done or materials delivered) for this long is cancelled and its site avoided |
| `dig_depth`, `dig_reach` | how many levels below the centre, and how far across, it digs for ore (phase C2; 0 depth: never) |
| `priorities` | the needs ladder, walked in order every plan; each rung is one of `build` (wanted `min + per_colonist × colonists` within `radius`, built or ordered), `research` (the next tech of the doctrine's tree that is unlocked, not known and within the cleverest colonist's Int; one at a time, at a research building within `radius`), `craft` (a recipe at its station within `radius`, until `min + per_colonist × colonists` of its output are stored or carried), `mine` (resource → stock wanted beyond what open orders need; the nearest deposit face or choppable that drops it, on the surface, else below down the settlement's stairs), `haul` (`food`, anything edible, or a blueprint: loose items brought into storage, nearest first), `dig` (see "The director digs"), `convert` (a floor tile: ConvertActions until `min + per_colonist × colonists` tiles within `radius` are it; see "ConvertAction"), and since phase D `hunt`, `raid` (see "Raids and hunts") and `grow` (see "The greyish") |
| `max_hunts`, `hunt_ticks`, `hunt_reach` | the hunt rung's open orders, how long one may run, how far it hunts (phase D) |

**Rules.** A build, research or craft is ordered only when the stock and
carried materials not yet promised to open orders (their cost less what
has been delivered) cover its cost. Build sites are on a grid around the
centre (columns two apart, rows three apart, so a two-tall bed keeps a
walkway), nearest first, the seed picking where each ring starts; a site
needs every footprint tile open and empty, a reachable tile beside it to
work from, and must not cut anything reachable now off from the centre
(open builds count as walls). Reachable is a four-way flood from the
centre (or the colonist nearest it) through open ground, workers and doors
the colonists' faction may pass, within `reach`. Choices are
deterministic for a level, a tick and the seed (`campaign seed`, location
ID).

**What it cannot do yet.** It does not beam supplies, assign colonists,
equip, fight, dig rooms or build walls.

### The director digs

Phase C2: `internal/director/dig.go`. The opening maps have no ore on the
surface; their veins lie one to five levels down, 20 to 250 tiles from the
landing. A `mine` rung that wants a resource and finds no face for it
(short, or under twice its target) records it, and the `dig` rung goes
down for it, one order at a time:

1. **Target**: the nearest deposit yielding it below the centre, within
   `dig_reach` across and `dig_depth` down (a bounded search of the level:
   the ore a player would find by digging; the player's scanner overlay is
   not consulted).
2. **From** the reachable tile nearest that ore, deepest first, so a shaft
   already begun is carried on (everything is read from the level each
   plan, so a reloaded settlement picks up its own shaft). Below the
   surface, reachable is a flood down every Stairs Down in reach.
3. Above the ore's level, a **Stairs Down** (the player's build order): on
   the surface at that tile; below it on the next tile toward the ore,
   since one tile cannot hold an up and a down stair, so the shaft steps
   one tile across per level. A landing that would pave over ore is
   skipped.
4. On the ore's level, a **tunnel**: the next tile toward it Dug, or Mined
   when it is a deposit itself.

Once the ore is a face, the mine rung mines it like any other, and the
miner hauls each chunk to storage as it does for the player. The dig rung
sits after research: it waits (and the rungs below it with it) while a
structure a rung above ordered is still going up, so the lab and its first
research have first claim on the kit. The stairs a shaft still needs are
held back from the rungs below it; once a shaft is begun, they are held back
before the ladder, from every rung, so a second research cannot spend its
last stair. A z step without the right stair under foot now costs 10⁷ in the
path search (`internal/world/tile.go`): at 1,000, a worker idling on a
stair landing made the invalid drop beside the shaft look as cheap, the
search returned it, the path checks rejected it, and the shaft was
unreachable.

## Observer mode

Two things share the name; they differ in whether the player has a ship.

**An observer run** is a New Expedition started with `observerMode` on
(`data/config.local.json`, or a vmtest script header's `config`): the title
screen passes `GenOptions.Observer` and the campaign is saved with
`observer: true`, so the whole run is one (`NewWorldManager` sets the
game's `observerRun()` from it). With observer mode on, the title screen's
tutorial toggle starts off (a run has no ship to teach on, and
`StartNewExpedition` keeps a run tutorial-free whatever the toggle says). There is **no player ship**. The crew, hold
and fuel the player would have flown are the **expedition**: a
fleet-carried presence of the player's doctrine (`colony`), faction
`expedition`, `campaign.AddExpeditionFleet` (`internal/campaign/observer.go`),
starting at the Wreck Site with the start colonists, the normal hold as its
stockpile and the start fuel in its tanks. It is a mirror colony in all but
name: the expedition table (`expedition`, see below and [Ships](#ships))
decides where it goes and when it leaves, it lands, its colony is a faction
settlement (`expedition:<site>`) that the settlement director runs whenever
its planet is loaded, and Tier 1 steps it (grows, researches, refuels)
when nobody is looking. It leaves the Wreck Site on its first day (`launch`)
and may take the first-jump sites (they are kept for the player, and here
it is the player's). Its landing visits the site and reveals what holds it.
Its techs are its own presence's; `Campaign.Player` is left unused. A run
is lost when the expedition fleet is gone (`checkTotalWipe`). The Star Map
shows it as a fleet with the badge and tag `EXPEDITION` (the colony
doctrine's `expedition` morph), and the doctrine carries the Tier 1 fields
a fleet presence needs (the mirror's growth, capacity, camp stage and
research walk; `ship_speed` 3, the player's jump rate).

The player-ship surface is gone in a run, not disabled: the Star Map has
no Travel Here, Beam Down or Up, Scan or crew lists (the expedition's
panel stands there: where it is, crew, fuel, hold, why it last lifted off),
the dashboard has no To Ship or To Site, Escape does nothing there, and the
header reads `OBSERVER · Star Date · expedition on <site>, fuel <n>`. The
destination line gives the expedition's jump there. No popup is addressed
to a player who is not there: interceptions are only detected on the
player's jumps and incidents only on the player's colonists, and both keep
a guard that answers them silently in a run. The viewer is a free camera:
**View** on the Star Map opens any charted site's level (a fleet marker
sits on its planet: select the planet), the HUD shows `VIEWING <planet>`
and `OBSERVER`, the Dashboard button reads Star Map, and the whole level is
drawn (`RevealAll`, rendering only; a planet nobody is on would otherwise
be black under the fog of war). Viewing the expedition's planet opens on
its camp. See [Viewing](#viewing) below. The viewer is omniscient: every
presence, fleet and faction event is on the Star Map, in Legends and in the
Factions tab from day zero, a rule of the view alone (see "What the player
sees").

`observer off` in the console is refused in a run: taking over the
expedition mid-run is not supported in this pass (a todo).

**The inspector.** In observer mode (a run or the lock) a left click with
the default cursor on any entity, on the viewer's own level or a Viewed
planet, opens the read-only entity inspector (`entityInspector`,
`internal/game/entity_inspector.go`): a scrolling text modal with only a
Close button (Escape closes it too), refreshed every 30 ticks while open so
health and task stay live; the default cursor's hover hint says so
(`Inspect: <name>`, `Read: <name>` on a datapad, nothing on an empty tile;
`observerContext`), where a normal campaign's hints name the order a click
gives. `inspectEntity` reads and changes nothing. It
shows the name and blueprint, the tile, the faction (the creature's
doctrine key), its settlement or a container's owner, for a kin body its
unit (morph and type), print tier, the settlement that laid it, its
presence (id, faction, stage, bodies) and the upgrades that presence knows
for its type (the evolve entries, `typeUpgrades`; the same techs
`upgradedType` folds in), the description, then sections: Stats (health,
Str/Dex/Con/Int/Wis, AC with worn armour, attack dice and modifier,
initiative, work speed), Activity (AI state, role, current task and its
tile, behaviour key, hunger and exhaustion), Equipment and Carrying (worn
slots and bag), a container's Contents (items summed by blueprint, slots
used of capacity, accepted tags), a station's kind, power and repair state,
and a datapad's text. In a normal campaign the colonist, crafting,
research and storage modals keep their clicks, and the default cursor's
order branches (attack, harvest, retrieve, sleep, repair) are unchanged;
only a click that selected a body that acts (health and initiative) with no
other modal (a neutral creature, another faction's worker that is not an
attack target) now opens the inspector.

**Datapads.** A datapad (`DatapadComponent`) reads, in the inspector, the
words a colonist picking it up posts: `datapadReading` (its own clue text,
else its event's datapad account, `Campaign.DatapadText`, which renders
like `RevealDatapad` without marking the event known; and its quest's lead
from `Campaign.QuestDef`, without unlocking it); a station log console
lists every record `readStationLog` would read (`stationLogText`).
`collectDatapads` uses the same helper, so the two cannot drift. Reading in
the inspector reveals nothing: an observer run's Legends and quests are
untouched. In a normal campaign every datapad picked up (and every new
station-log record) is kept on `Campaign.DatapadsRead` (`datapads_read`:
star date, site, text; `RecordDatapad`), and the Legends tab's **Datapads
read** filter lists them word for word (the chronicle shows an event's
chronicle line, not a side's own account or a clue's words).

**The observer lock** is the same flag on a normal campaign (one loaded
with the flag set in the file), or the console's `observer on|off`, which
sets the loaded campaign's lock (`observerLock`, reset by every
`NewWorldManager`) and never the config flag: the flag alone seeds a new
observer run, so the console's lock cannot turn the next New Expedition into
one. The player keeps the ship, its popups and
the Star Map's pause; the lock only takes the ground:

- the settlement director runs the player's settlement on every planet,
  live or in the background (`MainState.stepDirector`), never on the ship;
- the cursor gives no orders: a click only selects; the order cursors, the
  dig and mine marquee, right-click cancel, rogue mode, the B-key beam-up and
  the crafting and research modals' requests do nothing;
- the HUD shows the `OBSERVER` marker and the whole level is drawn;
- the Star Map's View button appears (it would scout any planet for free
  in play, so only the lock and a run have it).

Under `go test` the local file is never read (`config.applyLocalOverrides`
checks `testing.Testing()`), so a developer's flag cannot change a test.
(Phase C1's lock also answered incidents Ignore and refused the Star Map
pause; v2 gives those back to the player.)

**The expedition table** (`expedition` in the colony doctrine;
`campaign.DoctrineExpedition`, `internal/campaign/doctrine_expedition.go`).
The decision logic lives on the campaign tier because fleets are stepped
there and `internal/director` imports the campaign, not the other way:
`Destination`, `Worth`, `SiteScore`, `EncounterAnswer`. A site's score is
the table's `score` weights (`tiers`, `tags`, `unvisited`, `presence` per
known presence, `colonized`, `reinforce`, `per_fuel` per unit of the jump)
plus a jitter from the fleet's seed that breaks ties the same way every
run. A site is worth a jump when the ship could found there (nobody of
ours there, a founding crew aboard, scoring at least `found_score`); the
jump must leave `fuel_reserve`. For a fleet (the expedition, a rival
mirror; `fleetView`, `fleetMove`, `refuelShip`, `DepartShip` in
`faction_ships_d.go`): the colony is the whole crew, so founding is the
landing and there is no second colony to reinforce; it **packs up** when
its crew is down to `pack_up_at`, **moves** past `stay_days` (the table's,
4) to a site scoring `fleet_move_margin` better than staying, and otherwise
keeps the old rhythm as the fallback: at `mobility.stay_days` (40) to the
table's best site (`stay`), or with none worth it to a free site the fuel
reaches (`fallback`). `Fleet.last_move` says which. A landed day off a
loaded site refuels from its stockpile's fuel and `fleet_refuel_per_day`,
up to `fleet_fuel` (a ship launched with more, the expedition's 400, is
not cut down). The table's `found_crew`, `reinforce_*`, `kit`,
`incident_fuel` and `encounter` fields are for a ship carrying more than
one colony and for the player's popups; no fleet uses them yet (there is
no fleet against fleet interception).

**Viewing.** <a id="viewing"></a> `WorldManager.ViewLocation`
(`internal/game/view.go`; docs/developer/background_simulation.md,
"Viewed planets"): a level not in memory is built or thawed as
`buildParked` does and kept loaded while viewed, ticking live; leaving it
(the Star Map, the ship, another planet: `EndView`) freezes it again unless
a colony worker of the player's is on it. An observer run has none, so a
viewed planet, the expedition's included, always goes back to Tier 1 when
left, as a rival's does. Enemy camps, hives, autonz hubs and grey saucers
on a viewed planet are materialised (by `buildParked`, after the fixtures)
and running as on any loaded level, and the camera opens on them
(`frameView`: the expedition's camp, else the largest presence's stamp,
else the landing; a planet's presences are often far from the plaza, and an
early build opened on empty ground, which looked like a missing hive).
`debug faction` and `debug rival` frame a viewed level as any other.
What a view leaves behind: the level is saved (`SaveFile`; the Star Map
then knows its size), its presences are `Materialized` with their stamps
standing on the saved level, and its `Visited` is put back as it was.
While viewed, Tier 1 skips it and refuses it as an egg target, and its
presences step by the live rule; once left, it steps exactly as a planet
never viewed (`TestViewedPlanetStepsAsBeforeOnceLeft`).

`MainState.settlementStats` measures the player's settlement (the vmtest
sidecar's `settlement_stats`); `rival` measures the faction settlement on a
viewed level, the expedition's included. The sidecar's `observer_run`,
`expedition` (`landed`, `site`, `to`, `crew`, `fuel`, `last_move`),
`viewing`, `loaded`, `location` and `view_label` show the run
(docs/developer/vmtest.md).

**The soaks.** `make soak` runs the observer soak, the expedition soak, the
autonz soak, the hive soak, the greyish soak and the race run soaks (the
hive run, the autonz run and, since phase F step 6, the grey run:
`TestGreyRunSoak`, `make soak-grey-run`, a grey run's crew beamed down on
the start system and left alone for forty days, a wiped crew failing;
`LP_SOAK=1`). These run their seeds as separate processes side by side
(`make soak-autonz`, `make soak-hive`, `scripts/soak_par.sh`: the soaks share package globals,
so they cannot be parallel subtests): each process runs one seed
(`LP_SOAK_ONLY=node:3`, `hub:3`, `hive:3`) and writes its row into
`LP_SOAK_ROWS`, and a join test (`TestAutonzSoakJoin`, `TestHiveSoakJoin`)
prints the table and applies the thresholds. `SOAKJOBS` (10) bounds the
processes; `AUTONZTICKS`, `HIVETICKS` and `HUBSEEDS` the runs. **Determinism.**
The same campaign seed plays the same way: `NewWorldManager` seeds the
global `math/rand` source the simulation draws from (sleep rolls, script
`rnd_int` and `find_open_tile`, GameMaster spawns, deposit richness, mlge
and ml-rogue-lib dice) from the campaign's seed and day (`seedSimRand`;
`go.mod`'s `godebug randseednop=0` makes `rand.Seed` work again). The other
source was mlge's stopped-task delay, five real seconds (`task.Stop`; the
scheduler skips a stopped task until then), so a headless run faster than
real time depended on CPU load: the director re-queues a task it takes a
worker off (`ReQueue`, no delay), and a worker's own stops go through
`workerai.StopTask`, which the soaks and `TestHiveSoakDeterministic` swap
for `ReQueue` (`deterministicTaskStops`). The wall-clock gate itself is an
mlge issue: a tick-based cooldown belongs in its scheduler. The same seed
twice now gives the same row. The observer soak (`TestObserverSoak`) is
the lock on a normal campaign: ten New Expeditions (seeds 1 to 10), each
landing four colonists and an opening kit of 30 metal and 15 crystal on its
start system, the settlement director running them headless for 20,000
ticks (stepped as `Update` does). It fails when more than two seeds miss a
milestone (a director bed built by tick 5,000, a storage container by
8,000, a research order by 15,000) or end with fewer than three alive. The
expedition soak (`TestExpeditionSoak`, `expedition_soak_test.go`) is ten
observer runs at Rival factions 2 (the hive and a mirror, whose ship flies
by the same table): the world steps as the dashboard does until the
expedition lands, then the viewer opens its planet and it steps live, the
settlement director running the expedition's colony, for the rest of the
20,000 ticks. It fails when more than two seeds never land or end with
fewer than three of the expedition alive, or when any jump leaves the fuel
under the reserve; its table has the mirror ships' lift-offs by reason
(`mir t/s/f`). `make soak SOAKSEEDS=3 SOAKTICKS=6000` for a shorter run;
`LP_SOAK_DEBUG=1` logs more. `LP_EXPEDITION_DAYS=100 go test
./internal/game -run TestObserverRunLog -v` prints the expedition's moves
on seed 777 (`LP_EXPEDITION_SEED`, `LP_EXPEDITION_FACTIONS`). The numbers
are in the plan's "Phase C1 as built" and "Observer mode v2 as built".

## Playable races

Phase F ([playable_races_plan.md](../brainstorm/playable_races_plan.md)).
The New Expedition screen's **Race** row offers the `playable` doctrines (the
colony first, the default; `campaign.PlayableDoctrines`), with the doctrine's
`race.blurb` under it. The choice is `GenOptions.Doctrine`; an observer run
is always the colony's. `Doctrine.CampaignMode()` is `ship` (the colony, the
fleet doctrines) or `spread` (`mobility.kind` `launcher` or `signal`: the
hive, the autonz). `Campaign.SpreadRun()` (never in an observer run) makes
the run a **race run**; the game reads it once (`spreadRun()`, set by
`NewWorldManager`).

**A race run has no ship.** `shiplessRun()` (an observer run or a race run)
gates the ship surface exactly as observer v2 did: no ship crew, hold, fuel,
Travel, Beam, Scan, crew lists, To Ship or To Site, incidents or
interceptions; no tutorial and no quests (they pay fuel into a hold there is
not). The player's people are **presences of faction `player`**
(`campaign.PlayerFaction`) running the race's doctrine, planet-carried like
a rival hive's: `Campaign.AddPlayerBrood` puts the first on the start
system from the doctrine's `race.start` (stage, units by type, larvae,
stockpile), Known, the start system Visited, `Diplomacy["player"]`
neutral, `Campaign.Player.Faction` `player`. From there everything is the
rival machinery: Tier 1 grows, researches and launches the frozen ones;
materialise, the settlement director, larvae, bloom, the live launch and
reconcile run the loaded ones; an egg lands as a new `player` presence
(`landFleet` keeps the fleet's faction and doctrine; `newPresence` now takes
both) and is Known.

**The faction key is not the doctrine key.** `factionDoctrines`
(`internal/game/faction_hive.go`, set from the campaign by
`setFactionDoctrines`) maps `player` to the race and `expedition` to the
colony; `doctrineOfFaction` / `doctrineOfSettlement` resolve a faction or a
settlement name (`player:<loc>`) to its doctrine for `settlementBlock` (diet,
kill drops), unit types at spawn (`doctrineType`) and resin speed
(`bodyDoctrine`). `syncFactionHostility` skips only the player's faction
key, so a rival hive is still marked hostile to a hive player.

**View is the camera; your world gives you the cursor.** The Star Map's View
(`viewing()`: observer mode or a race run) opens a world the player holds
(`Campaign.HoldsSite`); others are refused in a race run. A race run opens
viewing the brood. After a level's presences materialise
(`buildParked`), `bindPlayerSettlement` points `MainState.MainSettlement` at
the player's presence's settlement (`player:<loc>`), so `colonyName()` is that
settlement and every order path, the crew panel (`PlanetColonists` reads the
viewed world in a race run), the keep-loaded rule and lore recording see the
brood. `ordersHere()` is the cursor's gate (never in observer mode; in a race
run only on a bound world). Techs in a race run are each presence's own
(`campaignTechs()` is false: `knownTechs`, `hasTech` and `unlockTech` read
the bound settlement, and a finished evolve retypes the living). The
settlement director keeps running every rung (**assist**): the player's
orders go into the same queue.

**The player spreads; the director never does** (the owner, 2026-11-12). A
race run's launcher or signal fires only at the world the player aimed it at
(`FactionPresence.LaunchTarget`; `campaign.aimed`), on a loaded world and in
Tier 1 alike; unaimed it holds fire, and a shot spends the aim. The director
still walks a ready shot to the Spire or Core toward that aim (the launch
group, which the player can take over and load by hand). **Leaving a world
hands it back:** `WorldManager.handBack` releases every group taken over
there when the viewer leaves it (`EndView`, or View of another world); the
aim stays.

**The end** (`WorldManager.checkSpreadEnd`, once a star-date day in
`TickBackground`; `Campaign.SpreadOutcome`): **lost** when no presence of the
player's (a dormant one counts) and no egg or signal of it in flight is left
anywhere, frozen worlds included; **won** holding awake presences on
`race.win.sites` systems at once (the hive: 6). The end screen takes the
race's `win` / `loss` title and text (draft) and is shown by whichever state
is up (`takeEnd`). `checkTotalWipe` leaves a race run alone.

The Star Map in a race run tags held worlds `[yours]` and shows **Your
worlds** (each with its stage and population, dormant ones with their wake
day, eggs in flight) where the ship's crew lists stand; the dashboard header
reads `Star Date: n      Your worlds: k / 6`.

**The world from a race's side** (step 4). Seeding (`seedRaceFactions`):
the rival colony first, so the humans exist from Rival factions 1; the
player's own doctrine left out of the bag; the rest by `seed_from`, the cycle
repeating; a colony run's seeding is untouched. Contact (`RaceContact`): a
neutral faction turns hostile everywhere when its ship lands on a world the
player holds (`LandShip`) or when a body of the player's comes within 8 tiles
of one of its workers on a loaded world (`WorldManager.raceContacts`, every
30 ticks: a worker passing a camp is there for moments); the hostility marks
are synced and its people take their hostile stance. Wild xenos and troids
are strangers to a player hive (their faction `hive`, the brood's `player`):
they hunt its drones and its guards fight them. The fog: a world at its
launcher or signal stage charts every site within its reach once a day
(`ChartRaceReach`, `InRaceReach`); View opens a held world, or one in reach,
which it sees as a ship in orbit would (its presences revealed, the whole
level drawn: none of the player's bodies is there to light it). Hidden
identity inverted: the player's own bodies and eggs carry their types' names
(`doctrineType`), its Star Map tag is `YOUR <stage> <pop>` and its row
`[yours]`, its Factions block `Your Hive` with units by type, the HUD names
its faction "yours", and its `faction_*` Legends lines name the hive. Its
units' deaths are counted, not named: a world losing five or more in a day
records one `brood_losses` event (draft).

**The autonz run** (step 5) is the same machinery over the signal: the
autonz' `race` block (the autonz soak's node start: three wire placers, an
enforcer, 20 metal ore, 4 crystal), its interface (metal, crystal, ferrite
and the conversion share; Lay Wire and **Deploy Relay**, the `deploy` order:
the nearest idle unit whose type deploys walks to the clicked wired tile and
folds into its structure, `MainState.deployAt`; Assimilation; a unit modal
without needs; the crew grouped by type and tier), its rung groups, and the
Signal Core's panel (the signal's readiness, the worlds in reach, Transmit
here, the group toggles). A signal founds a **dormant** node of the
player's: Known, shown with its wake day on the Star Map's Your worlds, alive
for the loss, not a world held toward the win until it wakes. The title
screen's races are ordered by `race.order` (the hive 1, the autonz 2).

**The greys, a ship race** (step 6). Greyish is `playable` but a `fleet`
doctrine, so its run is a **ship run**, not a race run: the ordinary ship,
Travel, Beam, quests and the reach-Home win. What differs:

- **Faction.** Any race's player presence takes the key `player`
  (`NewPlayerPresence`; the colony keeps `colony`), so rival greys stay
  hostile. `systems.PlayerFaction` (`playerPeopleFaction`: the player's key
  in a ship run, `colony` in a race or observer run) is admitted by every
  door owned `colony` (the ship's, a station frame's, a scenario's airlock)
  and unlocks keyed doors; setup scripts' `colonist_faction` reads it. A
  printed unit takes its crafter's faction, so the player's prints are the
  player's.
- **Crew and hold.** `Doctrine.ShipStart` (a ship race's `race.start`):
  `units` the crew by type (`seedRaceCrew`: typed, faction `player`),
  `gear` each type's kit, `aboard` the structures stood by the hull centre
  (the vat and console), `stockpile` the hold; a crew without needs gets
  no rations. The tutorial is the colony's only.
- **Ship-only structures.** A buildable's `ship_only` refuses the player's
  build order off the ship (`MainState.aboardShip`): the vat, console and
  array. Rival directors are unaffected.
- **Linked lockers.** When the player's doctrine keeps a storage pocket,
  `pocketOwner` counts the player's own settlement names
  (`playerPocketOwner`: the base, `<base> Ship`, `<base> @ <world>`): one
  pocket aboard and one per world, relinked every 30 ticks
  (`linkPlayerPockets`) and before a beam (`relinkPlayerPockets`), joined
  by the beam. The inspector lists the whole pocket and offers no beam from
  another owner's pocket door; the global inventory's tiers are open from
  the start.
- **The saucer.** The settlement block's `ship` block (`DoctrineShip`:
  `layout` `grid`, the default, or `saucer`; `saucer.start` radius and
  decks, `rings`, `decks`, `floor`, `wall`) picks the ship level
  (`buildShipLevelFor`; `ship.BuildSaucerLevel` carves a `world.RoundRoom`
  round `ship.HullCenter`). **Expand Hull** (click a deck) and **Add Deck**
  (`internal/game/race_hull.go`) grow it by ordinary orders in phases kept
  in the ship level's flags (`hull_r:<z>`, `hull_decks`, `hull_job`):
  breach, floor (only the reachable frontier is ordered), open (the old rim
  dug out), rim (the outline's stepped corners, reachable only from the
  outline beside them) and wall; a deck is stairs, floor, rim, wall. A
  ring or deck may need a tech (`requires_tech`; the greys' lattice and
  spine, whose `develop.hull` keeps rival directors off them). The
  research modal's Hull section is `MainState.hullLines`.

Station clicks check the owner (`stationOwner`, `internal/game/station_owner.go`):
a station's owner is the settlement its Kin names (a larva), else the
faction settlement whose director radius covers it; a click on a foreign one
opens the inspector, never the player's crafting or research modal. The
GameMaster's `hostile_max` count and `checkThreatCleared` skip the player's
own bodies (`ownedBySettlement`).

### The interface block

Phase F step 2 (the owner's direction: the interface adapts to a faction
through data, by a middle path, not a layout engine). A playable doctrine's
`settlement.interface` says what the player's HUD and modals show; every list
and label a race needs is here, and Go holds only a fixed set of **kinds**
each field picks from. An absent block, or field, is the colony's interface
as it was; the colony declares its own anyway, as documentation. Code:
`internal/campaign/doctrine_interface.go` (the schema),
`internal/gui/interface.go` (the HUD's spec, section kinds, order registry),
`internal/game/interface_doctrine.go` (the spec from the doctrine, resource
sources, type lines, crew grouping, validation),
`internal/game/structure_panel.go` (panels).

```json
"interface": {
  "resources": [ { "id": "biomass", "source": "storage" }, { "id": "larvae", "source": "larvae" } ],
  "unit_label": "Brood", "units_label": "Brood",
  "research_label": "Evolve",
  "orders": [ { "id": "default" }, { "id": "mine", "label": "Forage" }, { "id": "convert", "label": "Lay Resin", "description": "..." } ],
  "unit_modal": [ "identity", "type_tier", "stats", "needs", "task_filters", "inventory" ],
  "crew_group": "type",
  "panels": { "egg_launcher": { "title": "...", "rows": [ { "kind": "readiness", "source": "launcher" }, ... ] } },
  "groups": [ { "id": "breeding", "label": "Breeding" }, ... ]
}
```

| Key | Meaning | Kinds (registry) |
|---|---|---|
| `resources` | the HUD resource bar's slots, in order; `id` names the slot | `source`: `storage` (the ID as a blueprint in storage), `food` (edible items), `larvae` (the player's larvae on the level), `conversion` (the converted share of the patch, percent); none: the colony's rule (food by edibles, the rest by blueprint). A race run counts only its own settlement's storage; the colony counts every storage on the level, as it did |
| `unit_label`, `units_label` | one of the player's people (the unit modal's title, a fallback name), many (the population tab, the hover hints: "Send brood to this location.") | words, draft |
| `research_label` | the research modal's title | words, draft |
| `orders` | the build menu's Orders category, in order; `label` and `description` override the defaults | `default`, `dig`, `mine`, `store`, `drop`, `cancel`, `attack`, `sleep`, `convert` (lays the doctrine's `conversion.tile` where clicked: the order is open to a doctrine with one, so the colony's stays gated on `terraforming`), `deploy` (the nearest idle unit whose type deploys folds into its structure there), `post` (the nearest idle unit with one of the entry's `roles` walks to the tile and holds it: the player's guard post) |
| `unit_modal` | the unit modal's sections; the crew panel's detail follows `needs`, `stats` and `equipment` too | `identity` (name, Take Control), `type_tier` (the type's name and print tier), `stats`, `needs` (an exhaustion limit out of reach is not drawn), `equipment` (the six slots and the storage column), `task_filters`, `inventory` (Carrying and Pickup) |
| `crew_group` | the crew panel's grouping key; a race run's crew panel lists every loaded world's bodies and a row per frozen world | `""` (none: the colony), `type`, `type_tier` |
| `panels` | a structure blueprint's panel (step 3): a click on one of the player's opens it | row `kind`: `readiness`, `target_list`, `action`, `text`, `group_toggles` (one toggle per group), `world_toggle` (one switch taking every group over: "I run this world"); `source`: `launcher`, `signal` (readiness), `launch_targets`, `signal_targets` (target_list); `action`: `launch_at`, `transmit_to`, `load_launcher` (the shot of the player's units, the launch rung's roles first, walks to hold round the structure: the launch rung's walk by hand) |
| `groups` | the director's rung groups the player may take over (step 3); rungs carry `group`, and every rung of a playable race has one, so the player can take the director's place entirely | ids and labels |

**Validation.** `validateInterfaces` (run by `ValidateData` at start-up and
by `TestInterfaceBlocksValidate`) refuses an unknown section kind, order,
resource source, crew group, panel row kind, source or action, naming the
doctrine. **Adding a kind**: add it to its registry (`gui.UnitModalSections`
and `ShowUnitModal`; `orderRegistry` and a cursor mode; `resourceSources` and
`resourceValues`; `crewGroups` and `crewGroupKey`; `panelRowKinds`,
`panelSources`, `panelActions` and the panel's drawing), then any doctrine may
name it. A fourth doctrine reusing existing kinds needs no Go:
`TestFourthDoctrineUnitModal` loads a made-up one and checks its modal,
titles and orders.

## Faction settlements

Phase C2: `internal/game/faction_settlement.go`. A presence whose doctrine
has a `settlement` block (the mirror colony now; greyish and autonz later)
is not a swarm but a rival landing party. `materialiseFactions` builds it
(`materialiseSettlement`) before the creature presences:

- **Camp**, once: at the presence's centre (`presenceCentre`, the hive's
  rule: outside the landing's keep-out, saved as an origin flag), a bunker
  room the size of the doctrine's first stage (at least 9×7), an airlock on
  its south wall owned by the faction with the ground outside it cleared, a
  storage locker owned by the settlement and a workbench.
- **Settlement** `"<faction>:<locID>"` (finding 5) in
  `settlement.Settlements`, centred on the camp, holding the presence's
  techs; a thawed save restores it instead (`locationSettlements` saves
  every `*:<locID>` settlement with the location, `Freeze` drops them).
- **Workers** topped up or culled to `population`: the doctrine's crew
  blueprints (`CrewBlueprints`), with `Worker` and the faction settlement,
  `Description.Faction` set to the faction. `IsColonyWorker` compares the
  settlement name, so they are never the player's: not in the crew list,
  the loss check, the beam lists or the keep-loaded rule.
- **Stockpile** into the camp's locker on the first build (materials as one
  stack, other items one each).
- **Director**: `MainState.stepFactionDirectors` (live and background
  ticks) runs one `director.Director` per faction settlement on the level,
  the doctrine's table at the settlement's centre with its own techs and
  queue, through `settlementOrders`: the player's order paths, now
  functions of a settlement and level (`internal/game/settlement_orders.go`;
  the cursor's `queue*` wrappers call the same ones). Its workers pick tasks
  from their own settlement through the unchanged `WorkerSystem`; a finished
  research unlocks on their settlement (`ResearchDoneEvent.Settlement`), not
  the player's. Their task chatter stays out of the player's log.

**Reconcile** (`reconcileSettlement`, every `writeLocation`): living
workers into `population`, `storage.Summarize` over the settlement into
`stockpile`, its techs into `known_techs`; a fleet-carried presence is
written onto its fleet. No workers left is a wipe, as for the hive.

## The mirror colony

Since rival selection step 2 (rival_selection_plan.md) the rival colony is
not a doctrine of its own: it is the colony doctrine's **rival view**
(`colony!rival`, derived by the loader from the colony's `rival` block: its
name, badge, fleet mobility, ore economy, kit and seeding), and a
count-seeded campaign keys it `mirror` (the view's legacy key, which also
keeps its seeding order and resolves an old save's `mirror` doctrine). What
follows describes it as it was built, as the `mirror` doctrine.

Phase C2. Doctrine `mirror` ("Rival Colony"): `settlement_from: "colony"`
(the colony's crew, build menu, research and director ladder, copied on
load), `mobility.kind` `fleet`, `diplomacy` `neutral`, a `kit` (30 metal,
15 crystal, 12 rations), `seed_from` 2 (seeding draws it only when the New
Expedition count is at least 2, so Rival factions 1 is still one hive: it
takes one of the first slots, whose hive draw is still made with its site
and presence number kept back, and picks its own site from a stream of its
own, so every later hive lands, and is seeded, where it was before), and
a short abstract research walk. It has no morphs: a rival colony is openly a
colony, so player-facing text names it as itself.

**The ship** (`internal/campaign/faction_ship.go`). A fleet faction's
presence rides a `Fleet` of kind `ship`, landed (`landed_id`,
`landed_day`) or flying (the egg's from/to/days and `Pos`). Seeding puts it
landed on its site with the kit in its hold. Once a star-date day, a landed
ship off a loaded site grows toward the site's capacity and researches; once
`stay_days` (40) have passed it lifts off for a free site (no presence, no
other ship, a site a presence may be on, not loaded) within `base_range`
(40) at `ship_speed` (4 a day), taking everyone and the hold; its camp
stays on the level. It lands on arrival (built fresh there when that level
is next built) and waits in orbit while its target is loaded. A loaded
site's ship is simulated by the level and reconciled onto the fleet. The
plan's `Location.FleetIDs` is not stored: `Campaign.LandedFleets(loc)`
derives it.

**Diplomacy.** `Campaign.Diplomacy` records stances that changed from a
doctrine's starting one (`Campaign.Stance`: the record, else the doctrine's
`diplomacy`, else hostile). The mirror is neutral until the player beams a
colonist down onto a site it holds (`WorldManager.contactFactions` from
`BeamDown`): then it is hostile, everywhere, at once; a `faction_hostile`
Legends event and an alert record it.

**What the player sees.** From orbit (or a scanner sweep, or in the ship
scanner's range) the ship is known: the Star Map draws it as a dart in the
doctrine's colour, beside the site it holds or flying with a dashed line to
its target; the info panel tags a site it holds `RIVAL COLONY · NEUTRAL` /
`· HOSTILE` and a target `RIVAL COLONY SHIP INBOUND · N DAYS`; the
Destinations row notes `[rival colony]`. Legends: `faction_landed`,
`faction_departed`, `faction_hostile`. On the ground: the camp, four
colonists in the player's own sprites, beds and lockers going up.

## Hostility

Phase C2, finding 3 (`internal/components/hostility.go`,
`internal/game/faction_hostility.go`). A faction's stance marks it for the
player's colonists (`components.SetFactionHostile`, synced from the
campaign when a world manager starts and when a stance changes): a living
worker of a hostile faction is an attack target (`IsAttackTarget`: orders,
bump attacks, a colonist meeting one on a tile); a neutral one's are left
alone, and a colonist never picks another settlement's worker up off a
tile. Spawned faction workers defend themselves (`SelfDefend`). A
doctrine's scripted unit (a guard; none spawn yet) has its `hostility` var
seeded before its first turn: the doctrine's `aggression` when hostile, 1
(below every hunting threshold of the humanoid script) when neutral, so the
script does not roll its own. Scenario and hive creatures keep their own
rules. There are no player turrets in the game to update.

## ConvertAction

Phase C2. The conversion economy's primitive: `task_requests.ConvertAction`
with a `ConvertRequest` (tile, shared progress, 8 ticks), worked by
`workerai.handleConvertTask` (stand on or beside the tile, lay the floor;
a tile already converted or with no floor finishes at once) and saved with
the queue. Directors order it through a `convert` rung (no doctrine has one
yet; the autonz will, phase E). A doctrine's `conversion` block (`tile`,
`radius`) makes a presence's `conversion` share a biome patch:
`generation.ApplyConversion` brings the convertible ground within the radius
up to that share, laying the shortfall nearest the centre first, ties by a
hash of the presence seed and the tile (deterministic, idempotent, only
adds), when the presence materialises. Ground already converted anywhere in
the radius counts (a live director lays by reach from its converter nodes,
not nearest first), so a freeze and thaw never ratchets the share up; `ConversionShare` reads it back on reconcile. The
player's order path (`queueConvert`) is refused until the settlement knows
`terraforming` (`ConvertTech`), which no research entry gives yet: the plan's
open question answered as "gated behind research", with no player content.

## Raids and hunts

Phase D (`internal/director/raid.go`, `grow.go`). Two rungs close C2's
gaps: a hostile rival now fights, and every settlement feeds itself.

**Raid** (`{ "id": "raid", "raid": { ... } }`, last in the colony ladder,
so the mirror has it; the greyish too). Only when the site is hostile
(`Site.Hostile`, the faction's stance) and has the player's colony on the
level (`Site.Enemy`); neutral settlements and the player's own never raid.

| Key | Meaning |
|---|---|
| `fighters` | armed workers wanted, at aggression 5; the threshold is `ceil(fighters × 5 / aggression)`, at least 1 (aggression 8 raids with fewer, 2 waits for more) |
| `party` | at most this many sent (never more than the fighters) |
| `home` | workers left at the camp (at least 1) |
| `cooldown_ticks` | between raids at aggression 5, scaled the same way |
| `max_ticks` | a raid out this long is called home (its attack orders end) |
| `reach` | how far from the centre it looks for a target (400: camps are far from the landing) |
| `unarmed` | every worker counts as a fighter (the greyish); else only workers holding a weapon, and stored weapons are equipped first (`Orders.Equip`) |
| `roles` | only crew of these roles fight (the greyish: `soldier`) |

The target is the player's colonist nearest the camp, else a storage
container of the player's that can be hit. Party members are set on it
directly (`Orders.Send`, `orderSend`: the worker drops its task): the task
queue hands out the nearest work and a raid is never near. On the level in
orbit an alert names the raiders (`Doctrine.RaidAlert`). Mirror numbers:
two stun batons (its arms rung) arm two crew, a party of two or three walks
the ~280 tiles from the camp to the landing in about 3,700 ticks.

**Hunt** (`{ "id": "hunt", "hunt": "food", "min": 2, "per_colonist": 1,
"hunger": 30 }`, second in the colony ladder). Once the hungriest
colonist's hunger reaches `hunger` (the opening's builds come first), while
food items stored and carried (plus one per open hunt) are under the want:
attack orders on the nearest creatures whose drops are food (critters drop
protein bars), up to `max_hunts` (2), within `hunt_reach` (90; game
wanders, so beyond the reachability flood a hunt simply runs), each called
off after `hunt_ticks` (1,500). And the doctrine's food recipe: any recipe
in its units or recipes whose output is food, at its station, when the
stock covers it, its inputs chopped or mined first. The colony's is the new
`protein_press` (2 biomass into a protein bar at the workbench), which the
player can craft too. The haul rung brings the drops home.

Fixes made on the way: a rival's kill or chop drops into the rival's bag
(the killer search took only the player's workers); the director's chop
search looks at static flora (trees); and a path the level's greedy A*
drove through a wall (its squared-distance heuristic outweighs a wall's
5,000 cost over a long walk, so a raid party stood at its camp wall) is
searched again with a step cost of 1 and a Manhattan heuristic
(`internal/path`, `crossesSolid`, `steadyGraph`).

## Ships

Phase D (`internal/campaign/faction_ships_d.go`, `faction_ship.go`;
`internal/game/faction_ship_live.go`). A ship fleet carries its presence
(crew, hold as `stockpile`, techs) and `ship_tier`; a doctrine's `ships`
block (none for the mirror):

| Key | Meaning |
|---|---|
| `marker` | the Star Map marker: `dart` (default) or `saucer` |
| `tiers` | in order: `name`, `crew` (the crew cap; growth stops there, on the level the grow rung prints to it), `cost` (what the mothership spends to raise a ship to it) |
| `hold_target` | a landed ship whose hold reaches this many units lifts off for the mothership |
| `yield` | a landed ship's abstract digging per crew member a day, by site tag (`base`: every site), fractions carried in `yield_frac` |
| `mothership` | `crew`, `min_home_distance`, `ship_cost` (a new tier-0 ship), `max_ships` |

**The rhythm** (Tier 1, `stepShip`): a landed ship off a loaded site grows
to its site's capacity under its crew cap, digs into its hold, researches
(the first entry of its walk it does not know, as planet presences do: the
live colony's techs are merged in on reconcile in any order),
and leaves (`ShipLeaving`) when its stay is over or its hold is full
(`DepartShip`): a full ship flies to its mothership (`returning`) and docks
(`dock`: hold and techs into the mothership's, the mothership's techs back,
the next tier bought from the hoard when it covers it, `faction_upgraded`;
then out again from there to a free site in range: `setOut`, which only
puts it in flight, since leaving the mothership's orbit is no landing and
leaves no ruin or `faction_departed` there), else to a free site in
range (no presence, no other ship, not the mothership's site, not loaded).
A mothership (`stepMothership`) orders a new ship when its hoard covers
`ship_cost` and it has fewer than `max_ships`; ships ordered in a day are
built after the day's fleet pass (`buildShip`), and fleets step in place.

**Live** (`WorldManager.stepLiveShips`, once a star-date day with the live
launcher rule). A ship bound for a loaded site no longer waits in orbit:
it lands (`LandShip`, `faction_landed`), its camp and crew are built onto
the level at once (`materialiseSettlement`) and it is seen when the site is
in orbit, with an alert ("A grey ship has come down on Ember Pit."). A
landed ship on a loaded level has its record brought up to date from the
level each day (`liveHold`: crew, hold, techs), and when it is done it
lifts off live: the record reconciled first, then `DepartShip`, and only
once it is going `liftOffLive` (crew aboard, storage emptied, orders
dropped, the settlement gone; `faction_departed`, an alert), leaving its
camp as a ruin (see "Ruins"). With nowhere to go (no free site, no fuel) it
stays exactly as it is: nothing is taken off the level. A landed ship whose whole crew dies on its loaded level is wiped at
once (phase E, `wipeShipLive` and `Campaign.WipeShip`): `faction_wiped` in
Legends, the fleet gone, its settlement and director dropped, its camp left
standing as it is, an alert on the planet in orbit.

**Flown by a table** (observer mode v2). A fleet doctrine with an
expedition table (its own `expedition`, or `expedition_from`: the mirror
flies the colony's) asks the table where to go and when to leave, with
`fuel` on its presence (`fleet_fuel` at launch, a jump costing its
distance, as the player's): see "The expedition table" under
[Observer mode](#observer-mode). An observer run's expedition is such a
fleet of the colony doctrine itself. The same rules run live
(`ShipLeaving`, `DepartShip`). The greyish have no table and fly exactly
as before. A table-flown ship from a save before v2 (no fuel, no
`last_move`) is given a full tank on its first look at the table.

## The greyish

Phase D, rebuilt in Phase E4 as the high-tech, expensive settlement faction
(docs/brainstorm/greyish_plan.md and its "As built"). Doctrine `greyish`
("Greyish"; players meet them as Greys, morph `grey`): `seedable` from the
third rival faction (`seed_from: 3`), `diplomacy` hostile, `mobility.kind`
fleet (range 50, speed 5, a 30-day stay), economy `ore`, a kit of 12 metal
and 6 crystal, aggression 6, and:

- **Ships**: tiers scout (crew 3), freighter (5; 40 metal, 15 crystal),
  warship (8; 90 metal, 40 crystal); `hold_target` 30; yields by tag;
  a mothership of crew 10 at least 60 from Home, a new ship for 60 metal and
  20 crystal, three at most. Each tier lists its `modules` (below). The
  mothership keeps a beam stock apart from its hoard (`mothership.beam`):
  once a day a grey camp on a loaded level is topped up to the beam floor
  from it, then from the hoard (Tier 1 never beams); it carries metal,
  crystal and a little plastic. Beamed stock the camp
  has not spent does not count toward the hold target
  (`Fleet.HoldForHome`).
- **Types** (`types`, E2's mechanism; `Broods()` is false for them because
  their types name no morph, `ShipSettles()` true): worker (`greyish_crew`,
  mine dig haul build craft, `dig` 4 and `bore` 4: it tunnels fast),
  technician (`whitish_crew`, Int 16, the only researcher), soldier
  (`pinkish_crew`), heavy (`greenish_crew`, after `grey_cloning`, tentacles),
  skimmer and lancer (`skimmer_crew`, `lancer_crew`: flying constructs,
  after their patterns). Printed at the `grey_vat` from metal, crystal,
  `grey_alloy` and (the fliers) `specimen`; `print_*` recipes carry
  `kin_type` and cost the type's `print_cost`. Every crew body has
  `NeverSleep`; `needs` false.
- **Items**: `grey_alloy` (refined at the armory, 3 metal + 2 crystal),
  `plastic` (pressed at the armory, 2 biomass to 2: research, the array and
  the emitters take most of their metal as plastic),
  `specimen` (from a grey's kill of wildlife: `kill_drops`), the beam
  pistol, rifle and disruptor, the field harness and cowl (each with a
  `GearDrop` chance: a dead grey's gear usually breaks, now and then drops
  intact), `grey_lance` and `grey_emitter_beam` (natural: never drop).
- **Structures**: modules (carried by the hull tier, unpacked on landing,
  packed on lift-off): `grey_vat` and `grey_console` (`module_only`: a
  director never builds them), from the freighter the armory once the
  armory module is known (a scout builds its own), and on the warship two
  emitters once the field emitter is known. Ground structures
  (`settlement.ground`): `grey_armory`, `grey_array` (the advanced research
  building), `grey_emitter` (a turret, `grey_emitter.basic`, which holds its
  fire while the greys are neutral: `hostile_to_player()`), each left as its
  `wrecks` entry when the ship leaves.
- **The storage pocket** (`settlement.storage_pocket`): every container the
  settlement owns on a level is a door onto one inventory
  (`StorageComponent.Pocket`, a runtime link `linkPockets` rebuilds; totals
  count each door's own slice, `OwnCount`). A door destroyed moves its slice
  into the others; the last spills the pocket round its tile, stored
  `GearDrop` gear rolling its chance; a day later the ship re-places its
  cockpit door (`DoorLostTick`) and `haul: "any"` brings the rest back. The
  player cannot beam up from another's pocket door.
- **The tree**: sixteen `tree: "greyish"` entries with a `develop` effect
  block (`unlock`, `upgrade`, `craft`, `build`, `module`, `print_time`,
  `salvage`, `intercept`, `boarding`): live or Tier 3 only, never a number
  Tier 1 reads (a content test). Tier 1 walks the spine (`research`, three
  entries) and a second walk (`second_walk`, by hull tier 0.5 / 1 / 1.5 a
  day) while a ship is landed off a loaded site; `TestGreyishPaceRivalThree`
  pins the pace.
- **Materialise** (`materialiseGreyish`): the saucer, the tier's modules,
  the ground structures (`Built`, else `Doctrine.ShipBuilt` for its tier,
  techs and days down), the pocket, bodies by type (Phase D crew adopted:
  `adopt`, else the crew entry's type, else `type_fallback`), and gear (the
  record's `Gear`, else the doctrine's `gear_floor`). Reconcile reads units,
  ground structures, gear, techs and the hold back.
- **The director**: a lancer for each standing emitter (`grow` with `per`,
  over the crew cap), grow at the vat (the stage mix, keeping a `reserve` of
  a fifth of the hold target), mine, dig (above every build: ore lies deep
  and far on most sites), a locker by the far mine (`near: "mines"`), haul
  loose things, the armory (`stay_left` 8), research, alloy, weapons and
  armour per soldier and heavy (`per_role`), the `arm` rung, guard,
  emitters with lancers posted, the array (`stay_left` 12, `tier_min` 1),
  specimen hunts, skimmer patrols, raids after contact led by a heavy. It
  plans every 15 ticks. `raid_alert` is the greys' own line.
- **Ruins**: a departure leaves a landing scar (below).
- **Boarding**: the crew by the ship's units, armed from its gear, the
  hull's emitters on deck (a mothership's from `grey_mothership_bulwark`).

The greyish soak (`make soak-greyish`, `TestGreyishSoak`): ten scouts and
five warships landing live on the nearest ore world, a 30-day stay each.

**Seeding** puts the scout landed on its site and the mothership in orbit
of a site at least `min_home_distance` from Home (else the farthest), from
a random stream of its own (`newMothership`), at the top tier.

**The mothership destroyed** (the plan's open question). Taking it
(boarding, below) calls `DestroyMothership`: the mothership is gone and its
hoard returned as the boarding party's loot (it is in a locker aboard),
`faction_mothership_lost` is recorded, no ship's tier rises again and no
ship is built (tiers freeze where they are), and every ship bound home
scatters to the free site nearest where it was. Why: the mothership is the
faction's growth ceiling and the player's target, so destroying it should
stop the escalation (no more tiers, no more ships) without erasing ships
the player has not fought, which stay a threat at their current size and
now land wherever they were; a faction that vanished with its mothership
would make one boarding end a campaign-long threat too cheaply.

## Interception

Phase D, the plan's Tier 3 (`internal/campaign/faction_intercept.go`,
`internal/game/faction_encounter.go`). `Travel` asks
`DetectInterception(from, to, day, jump days)` before the jump's days
pass: a mothership (or a ship docked at one) in orbit of the destination is
met there (`at: orbit`); else the first ship in flight, in fleet order,
whose position comes within `intercept.range` (6) of the jump's line at any
of `samples_per_day` (4) samples a day (`at: crossing`). Eggs and landed
ships are never met. A ship met is known.

On arrival `Campaign.Encounter` waits for an answer in the **Contact**
popup (`encounterPopup`, hosted with the incident popup; the clock stops,
`WorldManager.PopupUp`): "A grey scout crosses your path on the way to …" /
"A grey mothership waits in orbit of …".

| Answer | Result |
|---|---|
| Evade | `evade_fuel` by tier (2, 3, 5; a mothership 8) taken from the hold; caught at `catch_chance` (0.1, 0.25, 0.45; 0.7), then boarded anyway (`faction_evaded` when it works) |
| Board | the ship's deck becomes a location and the ship travels there free (`faction_boarded`) |
| Parley | only when the faction is neutral (the mirror): both go on (`faction_parley`) |

Every roll is drawn from the campaign seed, the fleet's seed and the day,
so a campaign answers the same way every time. Observer mode answers Evade.

**Boarding.** `newBoarding` adds a location of Kind `encounter` (never a
site factions may hold) beside where the ship was met, map and scenario
`boarding`: the new `void` primer (open space round a 46×30 docking deck,
`starting_floor` at its west end, `boarding_ship` east of centre; the map is
`internal`, so history never picks it). On its build `materialiseBoarding`
stamps the hull, `crashed_ship` by the ship's class (scout, freighter,
warship from the tier's name; the greyish saucer; a mothership the warship
hull at 31×21), docked (`frame: station`, no furrow) with breaches, at
`damage` 1; the crew in their fighting forms (`greenish_crew` fights as
`greenish`, scripted AI and all; a crew member whose form has no script of
its own, the mirror's colonists, gets the humanoid fighting script under the
ship's faction at hostility 8 and loses its idle `Worker`: `armBoarder`,
phase E), tagged as the fleet's presence; and the
ship's hold (a mothership's hoard) in a locker aboard. The fleet is held
(`boarded_at`) while its deck is loaded. Its crew dead (checked once a day
and on every write of the level, `ReconcileBoarding`) is the ship taken:
the fleet is gone (`faction_ship_taken`; a mothership by
`DestroyMothership`), the alert says so, the loot stays aboard. Leaving the
deck (it freezes) with crew alive lets the ship go on with them
(`ReleaseBoarding`), and its deck with it: with no colonist left aboard the
encounter location and its level file are dropped (`Freeze`), so boardings
do not pile up. A taken ship's deck stays (its hold is loot).

## Ruins

Phase D4 (`internal/game/faction_ruin.go`). Every ship that lifts off from
a site it landed on records a `FactionRuin` there (never the mothership's
orbit site: a ship setting out from it landed nowhere), folded into an
unstamped ruin of the same presence on that site if one is waiting
(`Location.addRuin`: a presence's camp always sites on the same spot, so a
ship that lands and leaves again before the level is built leaves one ruin,
not one per visit) (`Location.Ruins`: its presence, the
`faction_departed` event, whether its camp stood on the level). The level
ruins it once (`stampRuins`, before any settlement is built, so a ship back
on the same site builds anew): at once on a loaded level (the player
watched it go), else at the level's next build, building the camp first if
the ship came and went while the level was frozen. The crew and whatever
the camp's storage held are gone (the stockpile left with the ship);
`ruin_pass` breaks it at damage 2 (hull or bunker walls and floors); its
footprint joins `StampFootprints` (the lore budget and later stamps keep
clear of it; the footprints are saved with the level, so a presence first
sited after a thaw keeps clear too, and a first siting also keeps clear of
a box over what the player's colony owns, `colonyFootprint`); the departure event gets an origin at the camp; and a datapad
there holds that Legends line.

A ship settlement (the greys, phase E4) leaves a **landing scar** instead
(`stampGreyScar`): the hull went with the ship, so its walls are cleared and
its floor scorched (`crater_ground`); its modules are packed; each ground
structure is replaced by its wreck in place (placed from the ruin's `Built`,
or `ShipBuilt` for its tier, techs and days down, when the camp was never
built); the cockpit's locker goes with the hull and outside lockers are left
empty and unowned; the footprint, origin and datapad are as above.

## Star Map counterplay

Phase D, as far as D covers Tier 3. Ships draw by their doctrine's marker
(a dart, a saucer; a mothership larger with a ring, beside its site; a
ship docked at it above it). The info panel names a ship by what players
know it as and its class (`fleetTag`: `GREY SCOUT`, `GREY MOTHERSHIP`, a
doctrine without tiers `RIVAL COLONY SHIP`): a ship landed here with its
stance (`GREY SCOUT · HOSTILE`; the mirror's tag is unchanged), a ship in
orbit here (`GREY MOTHERSHIP IN ORBIT · HOSTILE`), ships inbound
(`GREY FREIGHTER INBOUND · 3 DAYS`) and ships that left here with their
heading (`GREY SCOUT BOUND FOR EMBER PIT · 4 DAYS`, `BOUND FOR ITS MOTHERSHIP` when going home).
The Destinations row notes `[grey mothership]`. A scan sweep reveals the
ships within its range (landed, flying or in orbit) as it reveals
presences.

## The autonz

Phase E (`internal/campaign/faction_signal.go`, `internal/game/faction_autonz.go`);
Phase E3 made it a full settlement faction (`internal/game/faction_autonz_settle.go`,
[autonz_plan.md](../brainstorm/autonz_plan.md)).
Doctrine `autonz` ("Autonz"; no morphs, so players meet them by name): a
**planet-carried** settlement faction whose economy is conversion and whose
mobility is a signal. Hostile by default (no `diplomacy`); the hidden-identity
rules do not apply (only the hive hides). `seed_from` 4: Rival factions 1 to
3 seed exactly as before (every vmtest seed's sites are unchanged), Rival
factions 4 adds one autonz node (`factions_max` is now 4). Seeding records
`faction_swarm_landed` (`arrival_event`). `home_scenarios`:
`autonz_incursion`, so a presence on an incursion world builds by the
chapter's origin, one organism with the scenario's own wire placers and
enforcers (the same faction string, `autonz`, so they never fight).

**Conversion is the economy** (`economy: convert`, the `conversion` block):

| Key | Meaning |
|---|---|
| `tile`, `radius` | the floor its conversion lays (`blue_wires`) and the patch radius around its centre that `conversion` is a share of (16) |
| `per_day` | the abstract daily rate at full population (0.006), times the rate techs (`conversion_rate` on research entries), the scenario's hazard (never below 0), the population's share of the site's capacity and `1 + tags` |
| `tags` | added to the rate by site tag: ore 0.5, salvage and radioactive 0.25 (ore feeds it) |
| `cap` | the most of the patch it converts (0.85) |
| `floor` | the share of the site's capacity it carries with nothing converted (0.35) |

`Doctrine.ConvertCap(loc, p)` is the capacity under conversion:
`ceil(capacity × (floor + (1 − floor) × min(conversion / cap, 1)))`, at least
the doctrine's minimum. Tier 1 grows toward it, starves above it, and a
**fully converted** presence (`conversion` at the cap) stops growing: from
then on every signal costs it population it does not regrow. On a level the
director's convert rung lays the wire and reconcile reads the share back
(`ConversionShare`) on every save and freeze, and once a day while the level
is loaded (the live rule), so the grow rung's crew cap (`crewCap`: the
conversion-capped capacity for a planet-carried converting presence) rises
with the wire actually laid.

**Signals** (`mobility.kind: signal`). From a stage marked `signal` (the hub),
on `cooldown_days` (60, divided by the faction-count pressure), when the
population reaches the firing threshold of the site's full capacity
(`spread_share` 0.8, at least `spread_min` 6) or `conversion` reaches
`signal_conversion` (0.5), with at least `shot_pop + 2` bodies: a site within
the signal range, eligible, not loaded and holding no autonz, nearer ones
likelier (economy weight over 1 + distance), gets a **dormant** presence of
`egg_pop` (1) that wakes `wake_days` (30) later. The range
(`Doctrine.SignalRange`, Phase E3) is `base_range` (16) plus `pylon_range`
(4) per relay pylon standing, up to the stage's `relay_pylon` count, plus the
range techs: Tier 1 counts the stage's pylons as standing (a hub: 16 + 4,
Phase E's 20), a live level the ones that stand. The signal costs `shot_pop`
(2), taken from `units` by `economy_types` (wire placers, then relay drones),
and records `faction_signalled` (with `to`); with nothing in range nothing
is sent and the cooldown is not spent. It is the same Tier 1 step as the
hive's launcher with no projectile: no fleet, no flight. A dormant presence
neither grows, works, researches nor spreads; on its wake day it wakes
(`faction_woke`, its arrival day reset) and steps as a node.

Pace (seeds 1 to 6, Rival factions 4, opening map, no player; awake + dormant
autonz : mean conversion of the awake): day 40 one node at 12–20%, day 100
1–2 presences at 40–80%, day 200 3–5 + 1 at about 30%, day 300 6–9 at 40–70%.
`make history-dump HFACTIONS=4` prints the autonz row under each seed;
`HFDETAIL=1` adds a line per presence: units by type (bronze where the
type's tier is known), structures, conversion and the assimilation walk.

**Stages**: `node` (pop 0; stamp `autonz_node` 9×9, a small wired chamber
open on four sides with a red-wired centre; structures: a cogitator) and
`hub` (pop 7 or half the capacity; stamp `autonz_hub` 15×15, a round wired
hall with four ways out, a red-wired core and the Signal Core on it;
structures: cogitator, Signal Core, a relay pylon, the foundry, two converter
nodes, two defence pylons; `signal`). Each stage's `mix` is by type (below).

**Types and tiers** (Phase E3, [autonz_plan.md](../brainstorm/autonz_plan.md)).
`types` keyed by unit, one silhouette each on `scifi_entities` (steel row
936, bronze row 960), each with its own crew `blueprint` in
`autonz_crew.json` and a `print_cost` its print recipe matches:

| Type | x | Role, tasks | Unlocked by | Made at |
|---|---|---|---|---|
| wire placer | 48 | `converter`: convert, mine, dig, haul, build, craft, research | start | fabricator, 4 metal |
| enforcer | 192 | `guard`: attack (laser pistol) | start | fabricator, 8 metal 1 crystal |
| relay drone | 0 | `relay`: none (patrol and deploy orders) | `autonz_relay_pattern` | fabricator, 4 metal 2 crystal |
| seeker | 96 | `seeker`: attack (patrols; contact) | `autonz_seeker_pattern` | fabricator, 5 metal 1 crystal |
| harvester | 288 (flying) | `harvester`: mine, dig, haul | `autonz_foundry_pattern` | foundry, 4 metal 2 ferrite |
| sentinel | 240 (flying) | `sentry`: attack (`sentinel_emitter`, range 6) | `autonz_sentinel_pattern` | foundry, 6 metal 1 crystal 3 ferrite |
| warden | 216 | `warden`: attack (laser pistol; HP 30, 2d6) | `autonz_warden_pattern` | foundry, 10 metal 2 crystal 4 ferrite |

A locked type's share goes to its `fallback` (fighters to the enforcer,
workers to the wire placer), so with nothing unlocked a node is Phase E's
0.75 / 0.25 and a hub 0.6 / 0.4. **Tiers**: the doctrine's `tiers` (`steel`,
`bronze`: `sprite_dy` 24, health ×1.5, str ×1.25). A unit is printed bronze
once its type's bronze entry is known (`Doctrine.TierOfType`, from the
tree's `assimilate.tier`), **at spawn only** (`Kin.Tier`; steel units stay
steel), so a mixed army shows how recently the autonz assimilated. Wild
autonz (no presence) are always steel. Every body drops ferrite (two for the
foundry units). Crew bodies carry `NeverSleep`: machines work through the
night.

**The assimilation tree.** `research.json` entries with `tree: "autonz"` and
an `assimilate` block (`unlock` a type, `tier` a type's bronze, `build` a
structure for the effect line, `range`, `conversion_rate`), researched at
the cogitator; durations in work steps. Tier 1 walks two lists: the
`research` spine (the four Phase E entries, unchanged: their days and the
pace do not move) on `research_per_day` × stage, and the `assimilation`
list (unit patterns and bronze tiers, no range or rate) on its own points,
`assimilation_per_day` per stage (node 1, hub 2; `AssimPoints`). Known techs
are the union. Rough abstract arc: relay pattern by day 20, seeker by 50,
foundry about 75 at a hub, bronze enforcer about 100. Live, the settlement's
`research` list interleaves both walks and the assimilate rung works it one
entry at a time (about 300 ticks of one placer per abstract point).

**Structures** (on wire: `required_floor ["blue_wires", "red_wires"]`;
`autonz_structures.json`, the foundry in `workbenches.json`): the
**cogitator** (research station), the **Signal Core** (the hub's
transmitter: no live signal while it is down, and the director rebuilds
it), the **relay pylon** (`deploy_only`: a relay drone deploys into it;
+4 range each), the **foundry** (station `autonz_foundry`: the foundry
units and `refine_ferrite`, 4 metal 1 crystal → 2 ferrite), **converter
nodes** (a second convert origin; `ConvertBoost` halves a ConvertAction's
work within 5 tiles) and **defence pylons** (a sentinel's post). Each is
unlocked by the buildable's `required_tech`. Tier 1 keeps `built` (the
stage's structures its techs allow; `Doctrine.StageBuilt`), reconcile the
ones standing. A nil `built` is unknown (materialise places the stage's
set); an empty one is nothing standing (a razed settlement), kept across a
save (the field has no `omitempty`), so a razed settlement stays razed on
reload and thaw; Tier 1 then rebuilds one missing structure a day
(`tier1Rebuild`), not the whole set at once.

**On a level** (`materialiseAutonz`). The settlement block's camp is
`staged`: the stage's stamp is the camp, its storage claimed and the
`autonz_fabricator` placed by the centre; a risen stage is re-stamped with
the settlement's own things (structures, lockers and their contents, the
fabricator) lifted out and put back. Then the wire to the recorded share,
then the recorded structures on wire (relay pylons at the wire's edge,
defence pylons spread round it, the rest on the director's grid), the stock
(a floor of 20 metal, 4 crystal and 10 ferrite with a foundry when the
record has none), and the bodies by type at their tier, a Phase E crew
adopted once (replaced on its tile by its type's print). Reconcile writes
`units`, `built`, stock, techs and conversion back.

**The director** (`settlement.director`, walked every 30 ticks):

| Rung | What it orders |
|---|---|
| `placers` | a wire placer while under 2 + 0.3 × crew (within the crew cap) |
| `convert` | ConvertActions outward from the hub and each converter node (`convert_origins`) while under 40 + 60 per crew member; at the crew cap it keeps laying (`at_cap`: the cap is the conversion) |
| `materials`, `haul` | mine metal 40, crystal 10; loose ferrite home |
| `storage`, `cogitator`, `assimilate` | lockers (1 + 0.1 per crew), the cogitator, the next research entry |
| `dig` | stairs and tunnels toward ore (depth 6, reach 80) |
| `guard` | enforcers to 0.15 × crew; guards and wardens hold posts 6 out and answer hostiles within 12 |
| `core`, `relay`, `foundry`, `ferrite`, `converters`, `pylons` (hub) | the Signal Core; a relay drone printed, then deployed on the wire's edge toward the nearest site its signal could reach (a deploy that stops, its drone blocked, taken off it or dead, no longer counts against the rung: `forgetDeploys`; a blocked drone keeps its deploy and tries again); the foundry; ferrite under 10; two converter nodes; two defence pylons with a sentinel posted beside each |
| `grow` | the type most under the stage's mix among the unlocked, at its station, within the crew cap (one print a station at a time) |
| `transmit` | on a ready day (`Campaign.SignalReady`) two placers (then relay drones, never wardens) walk to the Signal Core and hold; the live rule signals with them |
| `seekers` | seekers and spare relay drones patrol a ring at 30 (contact) |
| `raid` | after contact, hostile; once wardens are known, only around a warden |

**Dormant on a level**: a single **sleeper node** (`level.sleeper`,
`autonz_sleeper_node`: the core block tinted dark, a `Kin` structure of the
presence) at its centre and nothing else: no camp, settlement, crew or
wire. Destroyed before its wake day, reconcile wipes the presence
(`WipeDormant`, `faction_wiped`). Woken while frozen, the next build removes
the node and builds the node camp; woken on a loaded level, the daily live
rule (`stepLiveAutonz`: `WakePresence`) does it at once with an alert. The
`autonz_sleeper` hazard's derelict is `enforcer_dormant` (the unlit tripod).

**Live** (`stepLiveAutonz`, once a star-date day beside the hive's live
launch): an awake presence's level goes into its record (units, built,
conversion, techs), a stage it has reached is stamped, and it signals by the
Tier 1 rule (`LiveSignal`) only with the Signal Core standing and the
transmit rung's bodies held at it (those bodies leave the level; an alert on
the planet in orbit); its range counts the relay pylons standing.

**The Factions tab** lists an autonz presence's units by type (a type
printing bronze marked so) and its structures under its row, in both modes
(no hidden identity). The cheat console's `faction tech <faction> <tech>`
teaches the presence on the site on screen a tech (its next prints take it).

**Star Map.** The badge is a square chip in the doctrine's colour
(`badge_shape: square`), filled when awake with a pip per stage, an outline
while dormant. The info panel tags `AUTONZ NODE` / `AUTONZ HUB`, and a known
dormant presence `AUTONZ SIGNAL · WAKES IN N DAYS` (`Campaign.SignalNote`);
the Destinations row notes `[autonz hub]` / `[autonz signal]`. A scan sweep
reveals dormant presences in its range like any other.

## Where a presence may be

`Campaign.FactionSiteOK(loc, seeding)`: a site (`HasSite`), not the Wreck
Site, not the Training Ground, and not a first-jump site
(`firstJump`: within `safe_start_radius` of the Wreck Site, whose history
`BindSiteSafe` keeps safe) unless the player has already visited it. Seeding
refuses first-jump sites outright, so the start system is always clean. Home
has no site.

## Seeding and the stepper

New Expedition's **Rival factions** stepper (0 to `factions_max`, starting at
`factions_default`, both in `data/generation.json`) is
`GenOptions{Factions}` to `GenerateCampaignOpts`. `GenerateCampaign` is the
same with 0. After the opening map is charted, `seedFactions` draws doctrines
from the seedable ones without replacement, refilling the bag when it runs
out (so a one-doctrine file still seeds `n` presences), and puts each on a
different eligible site, weighted toward its economy. Its random stream is
its own: the map, names, histories and quests of a seed are the same at
every count. Hidden sites (an opening contract's) can be seeded.

## What the player sees

**Hidden identity.** Players are not told that crawlers and fliers are one
hive until they find out (phase B+, `LoreLinks`). Until then every
player-facing name is the morph's, never the faction's: the Star Map tag and
row note, the orbit message and the Legends lines. A stage is named by its
`seen_name` (`Doctrine.SeenStageName`, `stageWord`: the hive's top stage is
DRAFT "great nest", never "hive"; an observer run shows the true name), the
hive core's rooms are DRAFT "Great Nest" and "Nest Heart", and the HUD's
"Faction:" line and the entity inspector hide a hidden-identity faction (a
doctrine with morphs: `hiddenFaction`) outside an observer run
(`TestHiveNeverNamedInANormalCampaign` scans these paths). Phase A has seen nothing
on the ground, so the morph comes from the site's scenario through the
doctrine's `morph_by_scenario` (`troid_outbreak` is fliers, `TROID`) and
`default_morph` (crawlers, `XENO`) otherwise. The rule lives in data so B+
can replace it (the morph actually seen, the merged hive name once the link
is known) without code changes. The badge keeps the doctrine's colour for
every morph.

A presence is unseen until the ship is in orbit of its site (`Travel` calls
`RevealPresences` and posts "Xenos presence detected on Rust Tangle.", in the morph's name, `Doctrine.OrbitAlert`; no stage word, since the top stage is called "hive") or a ship scan
sweeps it (`Scan` reveals every charted site within the scanner tier's
range). The Star Map draws seen presences only: a diamond in the doctrine's
colour left of the site icon with a pip per stage, a `[xeno nest]` note on
the Destinations row, and an outlined `XENO NEST` tag under the tier tag.
Known fleets draw as a small egg in the doctrine's colour at their position
with a dashed line to their target, and the target's info panel adds an
`XENO EGG INBOUND · N DAYS` tag (the morph of the site that fired it).

**The Factions tab** (dashboard, beside Legends; `internal/game/factions_panel.go`)
is what every faction holds at a glance: one block per faction with its
colour, total population, techs and counts, then a row per site it holds
(stage, population, converted share; a dormant signal with its wake day)
and per fleet (kind or tier, crew, cargo, landed, in orbit or heading and
arrival day, fuel for a table-flown ship; an egg with its arrival day). A
site row selects the site on the Star Map tab (so View can open it); a
fleet row selects the site it is at or bound for. It is rebuilt on entry
and once a star-date day. It exists in both modes with different fog: in a
normal campaign it shows what the player has seen (the Known flags) under
the hidden-identity rule, so a hive's morphs are separate blocks named by
morph (the player's faction intelligence screen); in an observer run it
shows everything, named by faction, the expedition included as one more
colony-doctrine faction. (The tab bar's seventh tab is why To Site and To
Ship are narrower.)

**Observer runs see everything** (`internal/game/faction_view.go`): every
presence and fleet from day zero, badges, tags with the faction's name
beside the morph's (`HIVE/XENO BROOD 3`, the hidden-identity rule off: there
is no player to hide from) and the population, egg and ship markers, dormant
autonz nodes with their wake day, Destinations notes like `[hive/xeno
nest 12]`, and every `faction_*` event in Legends. It is a rule of the view
keyed on the run (`shownPresences`, `shownFleets`, `fleetShown`,
`eventShown`, `presenceWord`): the records' Known flags are never written
for it, so a normal campaign is unaffected (`TestRevealOnlyInObserverRun`).

## Live history

`faction_arrived`, `faction_grew` (fill `stage`), `faction_launched`,
`faction_wiped` (the player cleared it) and `faction_exhausted` (it died out
on its own, leaving a husk) (all with `faction` and `singular`, filled with
the site's morph names, so "a xeno egg came down on ...") are live events in
`data/history/events.json`. They are written with `History.RecordBeside`: the
event names its site, so Legends lists it under the site's filter, but it does
not join the site's own event list, so the generated history (its events,
chapter, scenario, tier) is never changed. An event about an unseen presence
is recorded unknown and kept on the presence (the most recent 64; an
observer run, which shows every faction event, keeps none); revealing the
presence makes its events known. Seeding records `faction_arrived` on day 0.

Phase C2 and D add `faction_landed`, `faction_departed` (with `to`),
`faction_hostile`, `faction_upgraded` (with `tier`),
`faction_mothership_lost`, `faction_evaded`, `faction_parley`,
`faction_boarded` (with `ship`) and `faction_ship_taken`, written the same
way and named by morph (the greyish as "grey").

## Saves

`CampaignVersion` 4. Version 4 (phase C0) moved the player's techs from the
campaign's `known_techs` onto `player`; `MigrateCampaign` moves a version 3
save's there, under the default doctrine. Phase B's fields (`fleets`, `husks`, the presence's
level fields) are additive; the Phase A `eggs` field only existed on the
unmerged branch and is gone. The doctrine was renamed from `xenos` to `hive` before
any save with presences existed, so the version did not change. `MigrateCampaign` (called by `game.LoadCampaign`) takes
a version 2 save to an empty faction layer: no presences, `faction_count` 0,
`faction_day` at the save's star date, and `Tags` from each site's quest tag.
Older saves are refused as before. Phase C2's fields (`diplomacy`, a
fleet's `landed_id` and `landed_day`, ships in `fleets`, the convert task in a
settlement's queue) are additive, so the version stays 4. So are phase D's:
a fleet's `ship_tier`, `mothership`, `returning`, `boarded_at`; a
presence's `yield_frac` and `camped`; `Campaign.encounter` and
`encounter_seq`; a location's `boarding` and `ruins`. And phase E's: a
presence's `dormant` and `wake_day`. The review fixes add a location
level's `StampFootprints`, a build request's `Waited`, and write a
presence's `built` even when empty (no `omitempty`: `null` is unknown, `{}`
razed); a save from before them reads `built` as unknown, as it did.

## Tools and tests

```bash
make history-dump HFACTIONS=0,1,2,3 HFSEEDS=1-5 HFDAYS=100
```

prints, per stepper value and seed, `presences/eggs:population` every ten
days of an untouched opening map, and the days from each site's first
arrival to its first shot (average, range, sites that fired). `HFCOLS=0,40,100,200,300`
picks the days; `HFDETAIL=1` lists each presence with its capacity, stage,
techs and Destinations row.

Tests, Phase E: `internal/campaign/faction_autonz_test.go` (the doctrine;
seeded only from Rival factions 4, over six seeds; conversion drives the
capacity and growth, faster on ore, capped; a fully converted site spends
population on signals and stops growing; signals found dormant presences
within range only, on the cooldown, deterministically; the wake day and
waking a loaded presence; a scan reveals dormant presences and the signal
note), `internal/game/faction_autonz_test.go` (materialise lays the node, its
locker, fabricator and crew by the stage's mix, and the conversion share as
wire; reconcile reads it back and wire laid raises it; the hub stamps over
the node keeping the hold; a dormant presence is one sleeper node, wakes
live on its day, and is wiped when its node is destroyed; an incursion world
joins the chapter; the Star Map notes), the autonz soak
(`TestAutonzSoak`, `make soak`), `TestShipCrewWipedLive` and
`TestBoardedMirrorCrewFightBack` (D's leftovers), and the `factions_autonz`
vmtest script.

**The autonz soak** (`TestAutonzSoak`, `make soak-autonz`; extended in
Phase E3): New Expeditions with an autonz presence put on the start system,
neutral, one colonist beamed down so the level is live, 30,000 ticks
(`AUTONZTICKS`) headless. Two starts: ten node seeds (Phase E's: four, 20
metal and 4 crystal) and five hub seeds (up to nine, 40 metal, 8 crystal,
10 ferrite, the relay and foundry patterns; the start system read as an
ore and salvage world so a hub has room). A seed misses when the converted
share rose by less than 0.05, it never worked toward ore (no mine, stairs or
tunnel order) or no cogitator stood by 15,000; more than two misses of one
start fails, and any raid fails. The table has prints by type, bronze
prints, units at the end, techs with their first tick, the structures
standing and ferrite refined. Numbers in the plan's "Phase E as built" and
[autonz_plan.md](../brainstorm/autonz_plan.md) "As built".

Tests, Phase D: `internal/director/raid_test.go` (a raid only when
hostile and at the threshold, aggression scaling, called home and cooling
down, arming from storage, someone left home; hunting the nearest critter
until food is stored), `internal/game/faction_raid_test.go` (a hostile
mirror's party reaches and hurts the player's colonist, a neutral one never
raids; `TestMirrorFedFortyDays`: four crew, no rations, forty days, all
alive and none starving, skipped with `-short`),
`internal/campaign/faction_greyish_test.go` (seeding with a mothership,
the hold filled on an ore site and the tier raised only at the mothership,
the mothership building ships, destroying it freezes tiers and scatters
ships, a scan reveals ships in range), `faction_intercept_test.go`
(crossing and orbit detection, every answer the same per seed and both
outcomes of an evasion over seeds, boarding holds and takes the ship,
parley only when neutral, boarding the mothership),
`internal/game/faction_ship_live_test.go` (a ship lands and lifts off
live on the planet in orbit), `faction_greyish_test.go` (the saucer camp,
grey crew, hold and bio-printer; printing up to the crew cap),
`faction_encounter_test.go` (an interception on `Travel`, evading costs
fuel, the boarding level with hull, crew and loot, the crew killed takes
the ship), `faction_ruin_test.go` (a ruin left live with its datapad and
origin; one built after a frozen departure), `starmap_fleets_test.go`, and
the `factions_greyish` and `factions_intercept` vmtest scripts.

Tests, Phase C2: `internal/director/dig_test.go` (digging reaches ore two
levels down through stairs and a tunnel, one order at a time, the shaft
held back; nothing out of range; a shaft stays walkable past a worker on
its landing), `convert_test.go` (the convert rung), `internal/generation/
conversion_test.go` (the patch is deterministic, idempotent and read back),
`internal/campaign/faction_ship_test.go` (seeding the mirror from two
factions as a landed ship, a ship staying, lifting off, waiting in orbit and
landing, nowhere to go, stances), `internal/game/faction_settlement_test.go`
(a settlement presence builds, its director builds, reconcile round-trips
through a freeze and a thaw; a mirror ship lands, is seen from orbit, turns
hostile on contact and its workers become targets; scripted hostility
seeding; ConvertAction end to end and the player's gate), the soak's dig
columns and the `factions_mirror` vmtest script.

Tests, Phase C1: `internal/director/director_test.go` (the ladder's order,
mining when short, a plan never moves materials, determinism, stuck orders
cancelled, food hauled), `internal/game/observer_test.go` (observer mode end
to end on a small colony, off by default, the stats),
`internal/workerai` `TestBuild_SizedWaitsForAWorkerInTheFootprint` and
`TestBuild_BuilderStepsOutOfItsFootprint`, the soak above, and the
`observer` vmtest script.

Tests, Phase C0: `internal/game/player_doctrine_test.go` (the colony's
build menu, research lists along a walk of the tree, units and recipes per
station and crew equal snapshots of the old game; content missing from the
colony doctrine fails), `player_techs_migrate_test.go` (a version 3 save
loads through `LoadCampaign` with its techs), `internal/campaign/player_presence_test.go`
(migration, the player presence on a new campaign, doctrine selection),
`internal/workerai` `TestCraft_DeployOnCraft` and `TestCraft_DeployOnCraftData`,
and `TestFactionHiveMorphNames` (the alerts never name the faction).

Tests, Phase B: `internal/factory/tier_test.go` (tier application, kin
spawning, one faction for both morphs), `internal/systems/hive_test.go` (an
egg hatches its kin through the shared script), `internal/game/lore_troid_test.go`
and `lore_xenos_test.go` (the scenarios' hives are fliers and crawlers of the
hive faction), `internal/game/faction_level_test.go` (materialise is
deterministic and idempotent, respects the keep-outs, tops up and tiers,
joins a hive world's chapter; reconcile round-trips population, stage and
launcher, wipes; husks stamp once; a live hive launches on its cooldown),
`internal/campaign/faction_level_test.go` (exhaustion leaves a husk,
reconcile rules, a lost launcher regrows), `faction_fleet_test.go` (fleet
motion and arrival, visibility, save round-trip), and the content test for
the two stamps. vmtest: `factions_site` lands on seed 777's Rust Tangle hive
after `debug days 36`.

Tests, Phase A: `internal/campaign/faction_test.go` (growth, hazard, launcher and
cooldown, the firing threshold, a crowded site firing late, the Rival
factions 1 pace, loaded
skip, determinism and the map unchanged across counts, excluded sites over 40
seeds, events hidden until revealed, save round-trip, migration from version
2, station capacity, histories untouched, morph names, the troid hazard and the research order). vmtest: `new_expedition` walks the
stepper; `factions_starmap` shows a presence unseen, then badged from orbit
as `XENO BROOD`, then in Legends as a xeno egg.
