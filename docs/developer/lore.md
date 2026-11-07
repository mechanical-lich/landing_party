# Lore and History

Every campaign has a generated history of alien civilizations, and every
charted site's history explains what is there: the map is the present, the
threat scenario is the latest chapter, and the evidence stands on the map
where it happened. Play adds to the same history. Design and decisions:
[lore_plan.md](../brainstorm/lore_plan.md) (see its Implementation notes).

## Packages

| Where | What |
|---|---|
| `internal/history` | The model (`History`, `Civ`, `Figure`, `Event`, `SiteHistory`, `SitePoint`, `PlanetState`, `StarDate`), the data library, the generator (`BindSite`, `BindSiteEnding`, `BindSiteSafe`), rendering (`Render`, `Summary`), evidence and lore flags. Below `campaign`: it imports `mapdef`, never `campaign`. |
| `internal/campaign` | `Campaign.History`, `Campaign.StarDate`; charting binds a history in `newLocation` (`bindHistory`); history quests (`history_quests.go`); `RecordLive`, `RevealEvent`. |
| `internal/generation` | Origins (`ResolveOrigins`, `Origin`, the `at` anchor on features), lore flags set on the level before features run. |
| `internal/game` | Site lore into the build (`siteLoreFor`, `lore_site.go`), the recorder (`lore_recorder.go`), the Legends tab (`lore_legends_panel.go`). |
| `cmd/history-dump` | Headless chronicles and frequency tables. |

## The star date

`Campaign.StarDate` (days) and `StarTick` are the one campaign clock. The
world manager ticks it once per world frame in `TickBackground`, so it runs
with a live level and with the dashboard open, and stops whenever the world
clock is stopped. A jump adds `ceil(fuel / jump_fuel_per_day)` days
(`data/generation.json`). Prehistory is dated in negative star dates
("N years before landfall"); levels keep their own `Day`/`Hour` only for the
day-night cycle.

## Data

All under `data/history/`, loaded by `history.DefaultLibrary()` (cached; it
reads maps from `data/maps` itself). The text in them is draft.

**`arcs.json`** — story shapes. `id`, `uninhabited` (no one ever lived there;
a quarter of sites draw one), `layer` (an older people's arc run before a
main arc, see below), `stages`, `ends` (the state the arc must finish in),
`summary` (the Star Map line; `{civ}` is the site's civ). Each stage `pick`s
an eligible event type; `optional` stages happen at their `chance` (percent,
default 50); `repeat: [min, max]` picks that many, each re-checked (a type
already picked in the stage repeats only if `repeatable`); `epoch` marks a
world-setup stage (a biosphere) that a main arc skips after a layer.

**`events.json`** — event types.

| Field | Meaning |
|---|---|
| `requires` | Planet state (`life`, `population`, `status` any-of; `remains`, `contaminants` all-of), `events` / `any_events` already on the site, `resident` true/false |
| `effects` | `life`, `population`, `status`, `add_contaminants`, `remove_contaminants`, `add_remains`, `ruins_if_populated`, `resident_fate` (`died`, `fled`, `dwindled`) |
| `civ` | Who acts: `resident`, `arrive` (a pool civ settles), `native` (a new civ arises), `visitor`, `war` (`{civ}` and `{enemy}`) |
| `species` | For `native`, what the civ became; for pool roles, a preference |
| `figures` | `[{role, fate}]`, created on demand (a living figure of the same role and civ on the site is reused) |
| `origin` / `origin_from` | The event has a place on the map: rolled, or reused (and claimed) from an earlier event of the listed types; with both, `origin_from` is a preference and the event rolls its own origin without such an event |
| `scenario`, `tier`, `weight` | Makes it a chapter. `tier` is `threat` (the default with a `scenario`) or `hazard` (a local, finite danger playing the map's `base_scenario`, or a gentle `scenario` of its own). `weight` is per map and relative to the other chapters of its tier |
| `safe_start` | The hazard may end a first-jump site |
| `prelude` | Event types that happen just before the chapter (a wreck disturbed) |
| `flags` | Extra lore flags for the site's build (`chapter_crew: "dead"`) |
| `gap_days` | Time from this event to the next (or to the present) |
| `vars` | Random template fills |
| `evidence` | Feature blocks the event leaves; `"at": "origin"` anchors one at its origin (anchored counts are absolute, never scaled by map area); `lore_datapad` is implied by any origin; every stamp passes `occupants` (`"none"` for ruins: see structure_scripts.md) |
| `place` | Where its trace is, for quest lines ("the vault") |
| `omen` | A chapter's Star Map line |
| `live`, `fills` | A play event the recorder writes, and the fills it sets |
| `interlude` | A small happening (a figure born, a festival, a comet) the generator may add after any stage; never stamps, at most a third of interlude types take an origin (datapad only) |
| `repeatable` | The type may follow itself on a site, and be picked twice in a repeating stage |
| `text` | The chronicle template: `{date}`, `{site}`, `{civ}`, `{enemy}`, `{during_war}`, `{war_end}`, figure roles, vars, fills |

**`civs.json`** — the civ pool: size range (1–10), name and figure syllables,
species weights, ancestor and war chances, `lifeless_percent`, `reuse`
(preference for civs already in the chronicle).

**Maps** (`data/maps/*.json`) carry `present` (`life`, `status`, `arcs`,
`tiers`, `safe_tiers`, `density`, `layers`) and `base_scenario` (what quiet and hazard sites play;
always calm: passive wildlife at most).
**Archetypes** and **contract templates** may name a `spawn_event`.
**Scenarios** may list `threat_blueprints` and `threat_name` (what "threat
cleared" means).

Validation tests (`internal/history/lore_*_test.go`) check references,
template fills, evidence kinds and structure scripts, anchorable kinds,
scenario support and coverage.

## Density, interludes and layers

After every stage the generator rolls the map's `density` (percent: earth 65,
alien 76, ocean 64, frozen 62, desert 52, moon 40, station 40, asteroid 25)
for one eligible interlude. `layers` (`{"chance": 40, "arcs": [...]}`) runs
an older people's layer arc first (`elder_natives`, `elder_aliens`,
`elder_colony`), which must end with nobody living there; the main arc then
skips its `epoch` stage and its people may rise on the ruins
(`civ_rises_on_ruins`, a native civ, 30% descended from the former one) or
settle them (`settles_on_ruins`, also the optional succession stage after a
decline). Both fill `{former}`. `make history-dump HSTATS=500` measures it:
events per site by map and by population (layered sites apart), origins,
kept stamps, civs, span and tiers, then pool-civ recurrence.

## Tech levels

Every civ has a level: `primitive`, `medieval`, `industrial`, `atomic`,
`spacefaring` (`history.TechLevels`). Pool civs are spacefaring. A native
people rolls its start from `civs.json` `native_tech` (60 primitive / 40
medieval), capped by `species_tech_max` (yetis stop at medieval), and climbs
through `tech_advances` stages (`effects.tech: "+1"`), never past atomic;
`civ_collapses` costs a level. `Civ.Tech` is the highest it reached.

A people's homes reflect its rise, not its peak: the rise stamps a village
(primitive or medieval). A people that has reached industrial or atomic may
then rebuild: the native arcs' golden-age stage (the one after their
`tech_advances`, picking `civ_rises` / `civ_thrives`) also offers
`city_grows` (industrial or atomic: `@dwelling`, a city block) and
`silos_dug` (atomic: `@works` with the `command` purpose, a missile silo) as
alternatives, so they replace a golden age rather than add events. Over 500
seeds about 1.7% of sites keep a city block and 0.8% a missile silo.

Every event with a civ snapshots that civ's level then into `vars.tech`, so
`{tech}` renders its label ("iron age") and `{weapon}` one of its weapons.
`requires.tech_min` / `tech_max` gate the acting civ: reactors and nuclear
war need atomic, labs industrial, observatories and archives medieval;
ships and every pool beat are spacefaring. Arc summaries may use `{tech}`
(the site civ's highest level).

`data/history/tech.json` maps evidence roles (`dwelling`, `seat`, `fort`,
`shrine`, `works`, `grave`, `battlefield`, `landing`) to structure scripts per
level with their footprints (`small` on small maps, to keep the 3% footprint
budget). `data/history/purposes.json` gives each purpose (why a place was
built or a ship came) its ship class, room names, loot table and clue line.
`History.Evidence` passes every history stamp the structure library's
contract: `script` (a role resolved), `w`/`h`, `class` (ships, from their
purpose; a mothership only on a large map, counting three stamps), `event`,
`civ`, `species`, `tech`, `purpose`, `holds`, `clue` (rendered with the
event's fills), `room_<key>` names, `size` and `damage` (at least 1 on a
fallen world, 2 where a later collapse, meltdown or war's end happened at the
same origin). Ship events roll a `purpose` var (ancient wrecks: survey,
cargo, colony hold, warship, flagship, smuggling).

The role table, briefly:
a stamp with `"script": "@seat"` (or `@fort`, `@shrine`, `@works`,
`@landing`) becomes, in a medieval age, a castle town or a plain castle (half
each; a plain castle on small maps) and a bunker in an atomic one. A role may
list alternatives with weights; each stamp picks one by its site seed and
event, so each side of a war rolls its own and a site can hold two castles,
two towns or one of each (about 2% of sites have two or more castles);
a level without the role drops the stamp (its datapad stays). Every history
stamp gets a `tech` param. Each kept structure also leaves one of its
level's relics (`relics`, item blueprints in
`data/blueprints/items/relics.json`) on the ground at its origin.
`history-dump -stats` prints the tech distribution of natives and events,
and the kept (and budget-dropped) stamps by script with the share of sites
that hold one.

## Two peoples: the war arc and its variants

A site can hold a `rival` people beside its resident. Civ roles: `twins`
(two native peoples rise at once), `rival_native` (a people splits off the
resident), `rival_visitor` (a pool civ comes down as an overlord; its tech
gates read the people it comes down on), `rivals` (`{civ}` the resident,
`{enemy}` the rival) and `rival` (the other way round). `requires.rival` and
`effects.rival_fate` mirror the resident's. When a rival appears the map is
split across a rolled axis: the resident holds one half, the rival the
other. `origin_at: "side"` rolls an origin in the leading people's half,
`"border"` in the middle band. A `per_side` event is emitted once led by each
people (own figures, origin and level), days apart, its planet effects
applied once; its `text_side` is that people's own account, read on its
datapad (`History.RenderSide`, `Campaign.RevealDatapad`) while the chronicle
keeps `text`. Density counts a per-side pair as one happening
(`History.Happenings`).

Arcs: `sundered_world` (earth, frozen, alien, desert, moon): twin peoples,
tech advances per side, a border, war, a `@fort` on each side, one or two
great battles at the border, then `mutual_ruin`, `mutual_annihilation`
(atomic; leaves the world dead) or `one_side_prevails` (the survivors fall to
primitive; the `remnant_band` hazard can follow, yetis or mutant scouts).
`uplifted_people` (earth, frozen, alien, ocean): a visitor lands among a
primitive people (`@landing`), lifts them two levels, then leaves or leaves
weapons behind that split them into a war. `enslaved_world` (earth, alien):
an overlord's landing and mines (`@works`), forced labour, then an uprising
won (a warship left behind) or crushed.

## Tiers, the two-roll chapter choice and hazards

A history ends in two rolls on the site stream: a tier by the map's `tiers`
(`{"threat": 35, "hazard": 45, "quiet": 20}` on every map), then a chapter of
that tier by per-map `weight`. A tier with nothing eligible falls back threat →
hazard → quiet. A first jump (`BindSiteSafe`) rolls `safe_tiers` (`hazard` 60,
`quiet` 40) and takes only `safe_start` hazards. `SiteHistory.Tier` records the
outcome (`TierOf` derives it for saves from before tiers).

Hazards spawn finitely at generation, through their evidence, anchored at
their origin (at most six hostiles); they add no spawn rules. Each sets the
lore flag `hazard: <id>`. `derelict_pod` and `fauna_spawner` honour `at`:
anchored, they place their count on free tiles within `jitter` (default 8)
of the origin.

| Hazard | Grows from | Evidence | Safe start |
|---|---|---|---|
| `old_reactor_leak` | `reactor_meltdown`, `reactor_built` (needs `reactor`) | radiation pocket peak 120 | yes |
| `fallout_hotspots` | `nuclear_war`, `reactor_meltdown` (needs `radiation`) | 2–3 radiation pockets peak 100 | yes |
| `wreck_leak` | `ancient_wreck`, `crew_crash` | radiation pocket peak 90 | yes |
| `blight_patch` | own origin | crystal grove r 3, radiation peak 60 | yes |
| `worm_sign` | own origin | 1–2 worms in caves within 40 | yes |
| `scout_accident` | own origin | plays `crashed_ufo` with a dead crew | yes |
| `dormant_nest` | `ancient_wreck` | 2–3 xeno eggs | no |
| `troid_clutch` | `impact`, `impact_cataclysm` | 2–3 troid eggs | no |
| `hostile_fauna` | own origin (native life) | 2 yetis | no |
| `mutant_band` | ruins of a collapse, claim, station or seat, else own | 2–3 mutant grunts, a scout | no |
| `restless_graves` | a collapse, crash, war or impact (needs `graves`) | 2–4 zombies | no |
| `autonz_sleeper` | `ancient_wreck`, `drone_station_built` | an enforcer | no |

Five more need their own mechanics (lore content step 7):

| Hazard | Maps | Mechanic | Safe start |
|---|---|---|---|
| `toxic_bloom` | alien, ocean, earth | 4–6 `spore_pod`s (`data/scripts/ai/spore_pod.basic`): hit a colonist beside them every few turns, never move, can be cut down | yes |
| `unstable_ground` | solid maps | `rubble_field` feature kind: the nearest cave below the origin fills with rubble over a locker of relics (at the surface when no cave is in reach) | yes |
| `ancient_defences` | solid maps (a fort, battle, relay or seat's origin, else its own) | `defence_post` stamp with `sentry_turret`s (`sentry_turret.basic`): dormant until a colonist is within 7 tiles in sight, then fire their weapon for good | no |
| `unexploded_ordnance` | solid maps (a battle's origin) | 3–5 `old_munition`s (`old_munition.basic`): go off when a colonist steps beside one or when struck (two hits on everyone beside it, a crater) | no |
| `hull_breach` | station | `hull_breach` stamp: a section blown open to space; colonists cannot enter space tiles without vacuum resistance (a full enviro outfit), so its stores need a suit. Vacuum blocks, it does not damage | no |

An evidence block may declare `hostiles` (a count) for the budget and
keep-out when its creatures are not spawners (spore pods, munitions,
turrets).

The calm bases: `desert_calm`, `frozen_calm`, `alien_wilds` (alien and
ocean), and `abandoned_station` without mutants; the scenarios they replaced
stay threats (`worms` via `burrowers_stir`, `yeti_territory` via
`yeti_return`, `alien_world`, and `station_mutants` via `station_experiment`).
`make history-dump` prints tier shares per map above the scenario table.

The Star Map shows a tag before a charted site's summary, `Calm`, `Hazard` or
`Threat` in its colour (`data/history/tiers.json`, draft words): the danger
class, never the scenario. `Location.Tier` holds it (saved);
`Campaign.LocationTier` derives it for older saves.

## The landing reserve and the lore budget

`BuildWorld` picks the landing before any feature (`generation.ReserveLanding`:
the primer's anchor, else a plaza search on the location seed's own salted
stream) and tags it `landing_zone`, a box `2*landing_half+1` wide.
`findStartingPlaza` lands in it after the primer anchors, so the landing is
the same on every build of a site. History keeps out of it:

- `ResolveOriginsKeepOut` moves each origin `keep_out` tiles beyond the box
  (an event's own `keep_out`, else `hazard_keep_out` 40 for the chapter and
  anything that spawns creatures, else `keep_out` 16), along the line from the
  landing, then to open ground outside. Origins rolled on one tile move
  together. On a map too small for the distance it goes as far as it can.
- Anchored features never place inside `keep_out` of the box; stamps never
  overlap it and keep the size class's `spacing` from each other (see
  [stamps.md](stamps.md)).

`History.Evidence` spends the site's budget (`lore_budget` in
`data/generation.json`, by the rolled map width: small ≤180, medium ≤384,
large): stamps 3 / 5 / 7, their footprint 3% of the surface, underground
stamps 1 / 2 / 2, creatures 6 / 10 / 14 (a stamp with guards counts five),
radiation pockets 2 / 3 / 4. The chapter's evidence is spent first: its
structures always stay, its radiation pockets and creatures clamp to the
limits (logged). The rest is kept newest first, and a block that would break a limit is dropped with a
`lore budget:` log line and listed in `SiteEvidence.Dropped`. A dropped block
keeps its event's origin and datapad.

## The structure library

History stamps build through structure scripts in `data/scripts/structures/`
(authoring guide: [structure_scripts.md](structure_scripts.md)). Every
script stays inside its `w`×`h` footprint on every level (underground parts
too), lays no doubled or lone wall, and builds in under 100 ms (0.3–5 ms
measured); `internal/game/content_*_test.go` checks each at its smallest and
largest footprint with every param it takes.

**The param contract.** `History.Evidence` passes each history stamp
`script` (an `@role` resolved through `tech.json`), `w`/`h` (the role's
footprint, `small` on maps up to 180 wide), `event`, `civ`, `species`,
`tech`, `purpose` (the event's `vars.purpose`, else the role's default),
`holds` (the purpose's loot table), `clue` (the purpose's clue line with the
event's fills), `room_<key>` names, `size` (`small`/`medium`/`large`),
`damage` (0–3: the block's own, at least 1 on a fallen world, 2 when a later
collapse, meltdown or war's end happened at the same origin) and
`occupants` (`none` unless the event says otherwise). Ships also get `class`
(from the purpose) and `stamps` (3 for a mothership). Quest fixtures and
scenario stamps pass none of these, and every script then builds what it
always did (frozen copies compared tile for tile).

| Script | What | Params beyond the contract | Footprint |
|---|---|---|---|
| `crashed_ship` | a hull of sections aft to fore, crash furrow, damage 0–3 | `class` (scout, freighter, colony, warship, mothership, saucer), `crew` (alive/dead), `occupants: cryo` | scout 13×9 … mothership 60×60 (field) |
| `ship_bridge`, `ship_quarters`, `ship_cargo`, `ship_reactor`, `ship_engine`, `ship_cryo_bay`, `ship_armoury`, `ship_hangar` | one section each (called by `crashed_ship`) | `crew_state`, `holds`, `room_<section>` | the section's rect |
| `crashed_ufo` | wrapper: `crashed_ship` class `saucer` | `crew` | 17×17–21×21 |
| `castle_town` | the castle (unchanged) with cottages in a ring outside its walls, doors facing it, a road from its gate to the edge; below 36×28 the castle alone | `tech` | 36×28–48×40 |
| `village` | huts round a fire; palisade from medieval | `tech` | 17×13–31×25 |
| `city_block` | buildings on a street grid, a block office | `tech` | 21×17–37×29 |
| `outpost` | three starfaring modules in a row | | 17×9–31×15 |
| `standing_stones` | paired stones round a worn floor | | 11×11–21×21 |
| `fort` | palisade / star fort / redoubt (trench, pillboxes, command post) by tech | `tech` | 17×13–33×25 |
| `landing_platform` | lit pad, control booth, fuel | | 13×13–25×25 |
| `mine` | pithead over a shaft and gallery, ore carts, office | `tech`, `purpose: forced_labour` (pens, graves) | 17×13–29×21 |
| `missile_silo` | round hatch over a launch well, control bunker | | 19×13–29×19 |
| `burial_mound` | sealed chamber with grave goods, stone ring | `tech` | 11×11–19×19 |
| `graveyard` | rows of graves, chapel; atomic: mass trenches | `tech` | 15×11–29×21 |
| `battlefield` | craters, stakes/trenches/wreckage, the dead, dropped weapons | `tech` | 15×11–41×31 |
| `castle`, `bunker`, `underground_bunker`, `underground_hideout`, `abandoned_reactor` | as before, plus rooms, goods, clue, wear when history asks | `purpose`, `depth` (underground) | castle 18×16–33×27, bunker 9×7–15×11, underground_bunker 11×9–17×15, hideout 17×13–23×19, reactor 21×19–27×23 |
| `circle_tower` | stacked round chambers | `purpose` (`observatory` adds a desk), `height` | 21×21–29×29 |
| `circle_ruined_tower` | wrapper: `circle_tower` at damage 2 | | 21×21–25×25 |
| `underground_dungeon` | unchanged | `depth` | 25×21 |
| `nest` | resin chamber of eggs (husks when empty) | `egg`, `eggs` | 9×9–15×15 |
| `cave_in` | collapsed chamber, cache to dig out | `tech` | 9×7–17×13 |
| `defence_post` | broken wall round sentry turrets | `tech`, `turrets`, `occupants: dormant` | 9×9–15×15 |
| `hull_breach` | a station section open to space, stores at its heart | | 9×7–15×11 |
| `ruin_pass` | helper: wears a footprint by `damage` | `wall`, `floor` | any |

**Builtins** (all structure and setup scripts): `fill_container(x, y, z,
table)` fills the locker there from `data/history/loot.json`;
`spawn_lore_datapad(x, y, z)` places a datapad bound to the param `event`,
reading the param `clue` when set; `name_room(x, y, z, w, h, name)` names an
area (saved; the hover panel shows `Room: <name>`); `carve_round_room` lays a
single 4-connected outline; `mirror(x, y, w, h, flip_x, flip_y)` reflects
everything the script builds after it, so each ground script, hazard prop and
history ship comes out in one of four seeded orientations (the frozen
quest/scenario scripts excepted). `ruin_pass` is the shared damage helper.

**Data.** `data/history/tech.json` (levels: label, role → script and
footprint, or a list of weighted alternatives; weapons, relics), `data/history/purposes.json` (ship classes; per
purpose its class, rooms, holds and clue), `data/history/loot.json` (loot
tables: blueprint, min, max, percent chance).

**Looking at one.** On a site, the cheat console's `stamp <script> [w h]
[key=value ...]` builds a script around the camera centre with `occupants:
none` (e.g. `stamp crashed_ship 29 15 class=warship damage=2`); with
`debugDisableLighting` the whole footprint shows. `go test ./internal/game
-run ContentView` with `CONTENT_VIEW="script w h k=v,..."` prints one build
as ASCII with its time. The structure viewer (`go run
./cmd/structure-viewer`) has a Presets… list, `data/history/structure_presets.json`:
every ship purpose at its class's footprint, every tech level's roles at
their normal and small footprints with the role's default purpose, and each
fixed event stamp, all with `occupants: none`; its Damage control sets
`damage` 0–3, and hovering a named room shows the name in the toolbar. The
presets are drawn from `tech.json`, `purposes.json` and `events.json`;
`TestContentPresetsMatchData` fails when they drift (regenerate the list
rather than hand-editing it) and `TestContentPresetsBuild` builds each one
intact and ruined.

## Determinism

- History randomness never touches the campaign's rng: the civ pool draws from
  `Seed ^ historySalt`, each site from that plus its site seed, history quests
  from the site seed with their own salt.
- Charting keeps a discard draw where the scenario roll was, and history
  quests take location-based IDs, so a seed's maps, positions and names are
  unchanged (`TestLoreHistoryKeepsSeedsMapsPositionsNames`, golden file).
- A site's history depends on the campaign seed, the order sites are bound and
  its own seed; the same seed charts the same sites and histories, including
  after save and load.

## Lore flags

Set on the level before any feature (so structure scripts see them) and again
before setup scripts:

| Flag | Value |
|---|---|
| `origin_x`, `origin_y`, `origin_z` | The chapter's resolved origin; -1 without one |
| `origin_event` | The chapter's event type, `""` if quiet |
| `origin_civ` | The chapter's civ name |
| `has_wreck`, `has_reactor` | 1 when the history left a wreck or reactor |
| `population` | `never`, `outpost`, `colony`, `civilization` |
| `origin:<event>:x/y/z` | Every event's resolved origin (`generation.Origin`) |
| chapter `flags` | e.g. `chapter_crew` |

Scripts read them with `get_flag` or `get_flag_or(key, default)`; helpers
`find_open_tile_near(x, y, z, r)` and `find_cave_tile_near(x, y, r)` grow a
threat from a point. Every scenario setup script uses the origin when there is
one and keeps its old behaviour without (tested against frozen copies in
`internal/game/testdata/lore_old_scripts`).

## Live history and what the player knows

`History.Known` is all the player sees. Events become known by:

- picking up a lore datapad (one at every event origin; quest datapads carry
  the site's story event);
- a ship scan charting a site (its story event);
- a colonist standing within 2 tiles of an event's origin;
- completing a quest (the event behind it).

The recorder (`loreRecorder`, one per level) writes live events, known at
once and dated in the star date: `colonist_died` and `boss_slain` (killer from
the last `EntityDamagedEvent`), `colony_founded`, `colony_left`,
`colony_wiped`, `quest_completed`, `contract_taken`, `threat_cleared`.
Player-facing text uses display names (the expedition's colony, quest titles
without their site suffix).

History quests (`figure_log`, `civ_relic`, `threat_source` in
`data/quest_templates.json`, `history` field) bind at charting and complete
when their event is known (`flag_set` on `known:<event>`).

## Tools

```bash
make history-dump HSEED=42 HSITES=8
```
```bash
make history-dump HSEED=729 HCHARTED=1 HFREQ=0
```
```bash
make history-dump HCAMPAIGNS=1000 HFREQ=0
```

```bash
make history-dump HFIND=hazard,dormant_nest
```

The first prints a standalone chronicle, the second a campaign charted the way
the game charts it, the third the scenario share per map through real
charting, the fourth New Expedition seeds whose opening Star Map has a site
matching every term (for vmtest scripts). The planet generator shows a site's history and, with "Scenario from
history" on, builds the site the way the game does. Its History section
gives the site's tier, the lore budget readout (stamps, footprint share,
hostiles, underground and radiation against the size class's limits, for the
map's rolled size) and a button to the landing reserve; each event with an
origin lists what stands there (event type, script, class, purpose, tech,
damage; "not placed" when the build found no room; what the budget dropped).
The map and the overview label origins in magenta, outline the landing
reserve and its keep-outs (squares: the keep-out is Chebyshev distance from
the reserve box) in blue, and every placed stamp's footprint in orange.

vmtest scripts: `lore_site` (Star Map summary), `lore_xenos`, `lore_ufo`,
`lore_autonz` (structures at the origin, via `debug origin walk`) and
`lore_legends` (a datapad's event in the Legends tab, via `debug datapad`).
See [vmtest.md](vmtest.md).
