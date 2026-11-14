# Expanding Enemy Factions — Plan

Status: **Phases A, B, C0, C1, C2, D and E built** (2026-11-08, branch `claude/factions-a-a379f7`,
not yet merged). A: campaign presences, the hive doctrine, the daily step,
seeding, the Rival factions stepper, Star Map badges and Legends events. B:
hive morphs with tiers at spawn, presences materialised on their level and
reconciled on save and freeze, husks and wipes, eggs as fleets moving on the
Star Map, live hives launching. C0: the player's settlement reads from the
`colony` doctrine, the player's techs live on its own presence, and
`deploy_on_craft`. C1: the director runs the player's settlement in
observer mode, with settlement stats in the vmtest sidecar, an `observer`
script and a ten-seed soak. C2: the director digs (stairs and a tunnel to
ore below), faction settlements on loaded levels, the mirror colony on a
ship (neutral until the player lands on its site), hostility seeding and
`ConvertAction`. D: hostile rivals raid and every settlement feeds
itself; ships with tiers, holds, landing and lifting off live; the greyish
and their mothership; interception in space with boarding; ruins on
departure; Star Map ship markers and tags. E: the autonz, planet-carried
and converting, spreading by signals that found dormant sleeper nodes,
brought in by Rival factions 4; and D's two cheap leftovers. Observer
mode v2 (after E): observer runs with no player ship (the expedition a
colony-doctrine fleet on the expedition table, like the mirror), View any
planet, and the console lock. See "Phase A as built" through "Phase E as built" and "Observer
mode v2 as built" below and
[factions.md](../developer/factions.md). B+ and F not started. Planned 2026-10-31; code-reviewed the same day;
see "Code review findings" below for what the audit changed. Updated
2026-11-05 with the design chat: faction order (xenos → greyish → autonz),
presences that live on a fleet rather than a planet, the **mirror colony**
and **observer mode** (see "The mirror colony" and the revised Phasing).
Updated again the same day: xenos spread by an **egg launcher** structure
(population-triggered, research-ranged, shared by AI and player), eggs in
flight are fleets, and a **doctrine-driven player settlement** is adopted
as a design constraint so greyish/autonz/hive can become playable races
(see "Playable races").
Updated 2026-11-08: xenos and troids merged into one **hive** faction with
two morphs (crawler, flier), sprite colours used as research **tiers**, and
the hive's shared identity **hidden from the player** until discovered
(see "The hive").
Sequencing: background simulation Part B0 and Parts B–D are **merged**
(2026-11-01), so finding 1's colony-worker predicate exists
(`components.IsColonyWorker`) and **Phase A can start now**. See "Second
review" below for what the 2026-11-05 revision still needs to absorb.

## Second review (2026-11-05, against `dev` at `a7b0e72`)

Stale against merged code:

- **B0 is done.** Remove it from the prerequisites of C1 and from the
  status; the predicate, per-level scenario and scoped saves are all in.
- **The soak-test stepping question is answered.** The physical-storage
  and tutorial walkthrough tests (`internal/game/physical_e2e_test.go`,
  `tutorial_walkthrough_test.go`) already drive the full system stack
  headlessly through a `MainState` built with `newMainStateFromLevel` and
  stepped with `stepWorld`, no rendering. The Go soak test in C1 should use
  that path; nothing needs extracting. (Remember to reset the per-frame
  pathfinding budget each tick, as those tests do.)
- **Seeding must respect the adrift start and the tutorial.** "Not the
  start system and not within one jump of it" predates both. The current
  start is the Wreck Site (`campaign.AdriftKind`, no site) and a tutorial
  expedition adds the Training Ground (`Location.Tutorial`). Seeding and
  every later spread roll must skip any location where `!HasSite()` and
  any tutorial location, or a presence can land on empty space or on the
  training planet.

Sequencing:

- **The campaign clock exists** (the lore plan shipped it first):
  `Campaign.StarDate`, days counting world ticks (`history.TicksPerDay`),
  advanced by `TickStarDate` and by each jump's `JumpDays`. Phase A defines
  no day length or jump cost of its own; `StepFactions` steps from
  `Campaign.FactionDay` to the star date.
- **Phase B carries fleet machinery.** Eggs in flight are projectile fleets
  that travel between locations, so the `Fleet` record, its movement and
  arrival land in B, not D. B is "materialise plus Star Map motion", and
  its table row should say so; D then adds ship-kind fleets, tiers and the
  mothership on top of a working fleet loop.
- **A live hive must still launch.** Tier 1 skips loaded planets, and the
  director does not exist until C1, so in B a hive on a loaded (live or
  background-ticking) planet needs a minimal launch rule of its own, or
  the xenos stop spreading whenever the player is nearby. Simplest: the
  reconcile-on-Freeze path and a per-tick check on loaded presences both
  consult the same `LastShotDay` cooldown.
- **The doctrine plumbing roughly doubles C1.** Making the player's
  settlement read its build menu, research tree, printer unit list and crew
  seeding from a doctrine touches the build menu, research modal, crafting
  handler and `SeedShipCrew` in one phase whose headline is the director.
  Make it its own step, **C0: doctrine-driven player settlement**, between
  B and C1, with the `"colony"` doctrine reproducing today's game exactly
  and the vmtest scripts as the regression check.

Cross-plan decision (2026-11-05): **history and factions are independent
layers.** The lore plan always generates a site's history and chapter, which
picks the scenario. A faction presence is placed on top of that, exactly as
the player's party lands on a site, and must deal with the live scenario.
So: Tier 1 gains a per-scenario hazard factor on growth (a presence on a
plague or blight world grows slower or dies off; data in `data/factions.json`
per scenario id); Tier 2 materialises the history's evidence and threat
first and the presence second; a xeno presence and an `exterminate_xenos`
chapter on one planet are two hives, not one; and faction arrivals,
departures and wipes are recorded as live-history events. The campaign
clock is the lore plan's star date; Phase A builds it if the lore plan has
not shipped it first, and both plans cite one definition.

Design points to pin down:

- **Observer mode and input.** The cursor is inert, but the incident popup
  and the Star Map pause still expect input. Decide: observer runs
  auto-answer incidents with Ignore (and log them), and soak scripts never
  pause.
- **What a soak failure looks like.** Milestones and ranges are right
  (systems use global `rand`, and the VM work showed same-seed generated
  planets can still differ), but write the thresholds down: e.g. "no bed
  by tick 5,000 in any of ten seeds fails", "colonists alive at tick 20,000
  below N in more than two seeds fails". Without them soak runs are noise.
- **Two homes for known techs.** Presences carry `KnownTechs`; the player's
  live on the campaign. Once the mirror colony is a fleet, its techs ride
  the fleet while the player's do not. Pick one place (proposal: on the
  presence for everyone, with the player's settlement owning a presence
  under C0).
- **New Expedition layout.** The screen already moved Launch and Cancel
  down for the tutorial toggle; the "Rival factions" stepper needs the
  layout re-checked with the `new_expedition` vmtest script.

## Code review findings (2026-10-31)

Each assumption in this plan was checked against `dev` at `becc201`+.
Confirmed as written: `Location.Colonists` / `StorageSummary` are written
on `Freeze`; `Fixtures` are idempotent via a `Spawned` flag;
`rlcombat.IsFriendly` compares `Description.Faction` (and treats `""` /
`"none"` as never friendly); settlements each own a `TaskScheduler` and
workers pick tasks by their own settlement name; `Campaign.Scan`,
`generation.ApplyBiomes`, the `stamp` feature kind, the location template
tags, the listed blueprints and scenarios, and the wire-placer script all
exist. The `MaybeTravelQuests` RNG is `Seed + TravelSeq*K + GenSeq*131`,
close enough to the formula quoted.

**1. "Worker component" means "player colonist" everywhere. This is the
one that breaks the game if Phase C ships as written.** Faction workers
need `Worker` to use the task system, but every place the code counts
colonists tests `HasComponent(Worker)` with no faction check:
`countLevelColonists` (the **total-wipe loss check** and
`Location.Colonists`), `PlanetColonists` / `ShipColonists` (the beam
lists), the population list and task-filter UI in `main_state.go`, the
rogue-mode takeover, the cursor pickup/drop/store tools, and the map modal.
With enemy workers on a planet the player could lose every colonist and
not lose, see hive workers in the crew list, and beam them aboard. The
background doc's decision 3 ("a `Worker` keeps a planet loaded") has the
same hole: an infested planet would stay loaded, contradicting the
Keep-loaded rule below.

Fix, as a **Part B0 item** so it lands before either plan's level work: one
predicate, e.g. `IsColonyWorker(e)` = has `Worker` and its
`SettlementComponent.Name` is the player's colony (or `Description.Faction
== "colony"`), used at every site above and in Part B's staffed check. The
`WorkerSystem` itself stays faction-blind, which is the point.

**2. The campaign clock is per-level, not never-set.** `Campaign.Day` *is*
written, on every save, as a copy of the live level's `level.Day`
(`SaveCampaign`). Each level has its own `Day` that only advances while
that level is live, so a planet frozen on day 12 and revisited on campaign
day 40 still reads day 12. The `AdvanceDay` hook in Tier 1 is still the
right shape, but decide that the **campaign** clock is the source of truth
and levels display it, or Tier 1 and the HUD will disagree about what day
it is. Jump-cost-in-days then applies on `Travel`.

**3. Doctrine `aggression` has nothing to plug into yet.** The
`hostility` script var is randomised 1–10 per entity on its first turn in
`faction_humanoid.basic`; nothing reads it from a blueprint or doctrine.
Phase C needs a small hook: seed `hostility` from the spawning doctrine
(e.g. a `ScriptedAI` initial-vars map) before the script's first tick.

**4. Crafted entities go into the crafter's bag.** The craft handler
special-cases `output == "colonist"` to deploy at the printer; every other
output lands in the crafter's inventory. A director "printing an enforcer at
a fabricator" would put the enforcer in a worker's bag. Generalise the
special case into a recipe or blueprint flag (`deploy_on_craft`) in Phase
C; it also serves the player's Robot Workbench.

**5. Faction settlements and scoped saves.** Naming faction settlements
`"<faction>:<locID>"` keeps them unique in the global `settlement.Settlements`
map, good. But Part B0's scoped save/load must write faction settlements
into the location save and drop them from the global map on `Freeze`, or
they accumulate for the life of the process like the ship-pin note in the
background doc describes. Reconcile should delete the map entry.

**6. Minor name checks.** There is no `primer_station` identifier; the
station primer is in `internal/generation/primer.go` with
`station_setup.basic` as its scenario script. There is no `"encounter"`
location kind; kinds are free strings from the templates, so adding one is
data, not code. Hostile blueprints have no `Needs` component, so faction
workers skip the needs system cleanly (it is gated on the component).


## Goal

Enemy factions that behave like a rival landing party: they work their
planet with the same task system colonists use, grow in number as they
expand, stockpile / strip / convert the planets they hold, and spread from
planet to planet while the player is elsewhere. Several archetypes with
distinct personalities (cyborg, insect, greyish, ...), some planet-bound,
some with ships that can be met in space. One of them, the **mirror
colony**, plays exactly as the player does and doubles as the engine's
self-play harness (observer mode).

## The constraint that shapes the design

The [background simulation](../developer/background_simulation.md) design
settled that a planet with no `Worker` on it is **frozen**, and that staffed
away-planets tick **throttled**. Keeping every infested planet live so its
hive can grow would blow that budget and defeat decision 3/4 of that doc.

So an enemy faction lives on **two tiers**, the same way the ship is an
abstract hub while planets are concrete levels:

| Tier | Where | What it is | Cost |
|---|---|---|---|
| **Campaign** | `campaign.Location` | A small per-faction record: population, stockpile, conversion %, stage. Advanced by a cheap step on the campaign clock. Spread between planets is a roll here. | Negligible |
| **Level** | the live / parked `world.Level` | Real entities with `Worker` + `Settlement` components, a faction settlement, faction-owned storage, posted tasks. Only exists while the level is loaded. | Normal sim cost |

**Materialize** on load (record → entities, containers, biome patch, stamp);
**reconcile** on freeze (entities → record). `Location.Colonists` and
`Location.StorageSummary` already do exactly this for the player's side on
`Freeze` (`internal/game/world_manager.go`), and `Location.Fixtures` +
`spawnLocationFixtures` already materialize things idempotently on load.
This plan generalizes those seams rather than inventing new ones.

The payoff: a planet you left clean comes back with its ore gone and a hive
in the middle, at zero cost while you were away.

## Data model

### Campaign tier

```go
// internal/campaign/faction.go
type FactionPresence struct {
    Faction     string         `json:"faction"`      // "autonz", "hive", "greyish"
    Doctrine    string         `json:"doctrine"`     // key into data/factions.json
    Population  int            `json:"population"`
    Stockpile   map[string]int `json:"stockpile"`    // blueprint → qty, like StorageSummary
    Conversion  float64        `json:"conversion"`   // 0..1 of the planet converted
    Stage       int            `json:"stage"`        // 0 scouts, 1 outpost, 2 colony, 3 hive/fortress
    ArrivedDay  int            `json:"arrived_day"`
    LastStepDay int            `json:"last_step_day"`
    KnownTechs  []string       `json:"known_techs"`  // walked from doctrine.research
    ResearchPts int            `json:"research_pts"` // progress toward the next tech
    LastShotDay int            `json:"last_shot_day"`// launcher cooldown
    Seed        int64          `json:"seed"`         // deterministic materialization
}
```

**Where a presence lives** depends on the faction, and this is the main
design split:

- **Planet-carried** (hive, autonz): `Location.Factions
  []FactionPresence` (omitempty). The planet *is* the colony; when it is
  cleared the presence is gone. Several factions on one planet is allowed
  and is where fights between them happen (see Tier 2).
- **Fleet-carried** (greyish, the mirror colony): `Campaign.Fleets
  []Fleet`, where `Fleet` *embeds* a `FactionPresence` plus `ID`, `X, Y`,
  `FromID, ToID`, `Progress`, `Kind` (`ship` / `projectile`), `ShipTier`. The ship is
  the settlement: population, stockpile, techs and ship size travel with it.
  A "base" is a fleet that has landed; the Location only holds
  `FleetIDs []string` saying who is parked there. Leaving takes the
  stockpile along and leaves the stamped structure behind as ruins.

Materialize and reconcile (Tier 2) take a `*FactionPresence` and do not
care which carrier it came from.
- A per-planet **carrying capacity** derived from the location archetype
  tags (`data/location_templates.json`): `ore`/`biomass` tags feed strippers
  and insects, `hostile` tolerates more. Capacity is what stops the campaign
  snowballing.

### Doctrine (data-driven, no Go per faction)

Consistent with the "data-driven races" decision in
[design_decisions.md](design_decisions.md). One file, `data/factions.json`:

```json
"hive": {
  "name": "Hive",
  "mobility":   { "kind": "launcher", "spread_pop": 16, "base_range": 2.0,
                  "cooldown_days": 6, "payload": "hive_egg" },
  "economy":    "biomass",
  "growth":     { "per_day": 0.08, "cap_per_capacity": 1.0 },
  "conversion": { "tile": "hive_floor", "per_day": 0.0 },
  "morphs": {
    "crawler": { "egg": "hive_egg",       "cost": 1, "roles": ["forage", "haul", "dig", "build"] },
    "flier":   { "egg": "hive_egg_flier", "cost": 3, "roles": ["guard", "scout"], "skills": ["flying"] }
  },
  "stages": [
    { "pop": 0,  "stamp": null,        "mix": { "crawler": 1.0 } },
    { "pop": 8,  "stamp": "hive_nest", "mix": { "crawler": 0.8, "flier": 0.2 } },
    { "pop": 24, "stamp": "hive_core", "mix": { "crawler": 0.6, "flier": 0.4 },
      "launcher": "egg_launcher" }
  ],
  "tiers": [ "basic", "advanced", "elite" ],
  "research": ["tier_advanced", "launcher_range_1", "tier_elite", "launcher_range_2"],
  "aggression": 8
}
```

`mobility.kind` is one of `launcher` (hive), `fleet` (greyish,
mirror), `signal` (autonz, planet-bound slow seeding). `research` is the
ordered list the abstract tier walks and the director researches at the
stage's lab; entries live in `research.json` under a faction key so the
same file serves the player and the AI.

Axes and the four archetypes, in build order (see Phasing):

| Axis | Hive (xenos + troids) | Mirror colony | Greyish | Cyborg (autonz) |
|---|---|---|---|---|
| **Carrier** | planet | fleet | fleet | planet |
| **Mobility** | consume, then **launch an egg** at a system within launcher range once population hits `spread_pop`; no ship | ship, travels like the player | **fleet**: lands, salvages, leaves | planet-bound, spreads to adjacent systems slowly via "signal" |
| **Economy** | **biomass**: forage, eggs, grows with food | the player's: build, mine, craft, research | **stockpile**: mine + haul into their ship, hoard tech | **convert** (wire_placer already lays conversion tiles) + mine ore |
| **Growth** | fast, hard cap from capacity, then starves; two morphs, three research tiers | bioprinter + research, same as the player | bioprinter per unit type; **ship tier** gates crew cap and only rises at the mothership | slow, capped by conversion % |
| **Bases** | permanent until cleared; leave a husk when exhausted | semi-permanent, like the player's sites | temporary; ruins remain, stockpile leaves | permanent and growing |
| **Blueprints today** | `troids.json`, `xenomorphs.json`: egg → hatchling → stage2/3 (identical scripts; merge) | `colonists.json`, `robots.json` | `greyish.json`: greenish/greyish/pinkish/whitish | `autonz.json`: enforcer, wire_placer |
| **Scenario today** | `troid_outbreak`, `exterminate_xenos` | none (a second life-raft crew) | `grey_contact`, `crashed_ufo` (a fleet that never left) | `autonz_incursion` |

Autonz are deliberately last: they combine the two carriers the first two
factions establish (planet-carried growth, fleet-style building and
research) and add nothing structurally new.

### The hive: morphs, tiers, hidden identity

> Phase E2 changed this: the colour blocks are unit **types** (drone,
> warrior, burrower; scout, hunter, spitter), not research tiers. See
> [hive_plan.md](hive_plan.md) and "Phase E2 as built".

Decided 2026-11-08. Xenos and troids are **one faction** with two body
plans. The code already treats them as one species: the troid and xeno
stage scripts are identical apart from the name, with the same egg →
hatchling → stage 2 → adult ladder and the same health values. Today they
carry different faction strings and so fight each other on a shared planet;
after the merge they do not.

**Morphs.** `crawler` (xeno sprites) and `flier` (troid sprites). A flier
is a crawler plus the existing `flying` skill (`workerai/movement.go`
already handles it), which means it ignores moats, trenches and water, so
the player's dug-in defences stop working against it. That is what
justifies its higher biomass cost. Morphs have **roles**, not just a
price: crawlers are the workers (dig, forage, haul, build), fliers are
guards and scouts that carry little. The director spends biomass per the
stage's `mix`; a playable hive player picks which egg to lay.

**Life stage vs tier.** The sprite sheet has three colours per morph,
each with the three life stages, plus six egg colours (intact, cracked,
hatched).

- *Life stage* stays the existing grow-over-turns mechanic in the scripts.
- *Colour* is the hive's **tier**, set by research (`tiers` list, walked by
  `tier_*` research entries). An upgraded hive lays higher-tier eggs; tier
  multiplies health and damage and, at the top, launcher range.
- *Egg colour telegraphs the tier* of what hatches, so a red egg in a nest
  tells an attentive player the hive has advanced.

Do not hand-write 2 morphs × 3 stages × 3 tiers = 18 blueprints plus 6
eggs. Keep one blueprint per morph and stage (6 entities, 2 eggs) and apply
the tier at spawn in `factory`: each colour block on the sheet is a fixed
sprite offset from the previous, and tier adds that offset plus a stat
multiplier. The same mechanism serves tinted colonist variants later.

**Hidden identity.** Lore-wise the player should not know that crawlers
and fliers are one organism until they find out, like being hit by two
different creatures and not knowing they share a hive. This lives
**entirely in presentation**; the simulation, combat friendliness, the
doctrine and the presence record use the single `hive` identity throughout.

- `Campaign.LoreLinks map[string]bool` records discovered facts:
  `"hive:crawler=flier"`, `"hive:tiers"`, `"hive:launcher"`.
- Until the morph link is known: the bestiary (already keyed by
  `KnownEntities`) shows separate entries with the colonists' own names
  ("xeno", "troid"); Star Map badges show the one morph seen there, not a
  hive icon; quests post per morph, so a troid bounty and a xeno
  extermination may be the same hive and clearing one clears the other;
  log lines use the morph name, never the faction name. Tiers are likewise
  "a tougher one" until `"hive:tiers"` is known.
- Once known: entries merge into one species page with morph tabs, badges
  switch to the hive icon with a tier marker, and the Star Map shows the
  spread as one organism. The switch is visible and is the reward.
- Discovery paths, most common first: a colonist with line of sight to an
  egg of one morph hatching the other; a hive-biology research entry that
  needs a carried corpse or egg sample (physical storage plan); datapads
  from earlier expeditions, which may be wrong; seeing a launcher fire or
  finding one (reveals deliberate spread, a separate and bigger fact).
- Links are **per campaign**. Veterans will know, but the merged UI still
  has to be earned in-game.

Existing scenarios become the **materialization templates** for stage 1–2
presences instead of standalone scenario rolls. Scenarios stay random per
[scenarios-are-random](design_decisions.md).

## Tier 1 — Campaign step

Runs whenever the campaign clock advances. Today `Day` increments on the
*level* (`s.level.Day++` in `main_state.go`), not on `Campaign.Day`; the
first task is a single `campaign.AdvanceDay()` hook called from the live
level's day rollover and from `Travel` (a jump costs days proportional to
fuel). Deterministic RNG the way `MaybeTravelQuests` does it: `Seed +
Day*K + GenSeq`.

Per location, per presence, for each elapsed day:

1. **Grow**: `pop += pop * growth.per_day`, clamped to `capacity *
   cap_per_capacity`. Strippers' capacity shrinks as `Stockpile` fills
   from the planet's finite resource tally, so they *exhaust* and go idle.
2. **Work**: move resource from the planet's remaining-resource estimate to
   `Stockpile` (strip/hoard) or raise `Conversion` (convert). This is the
   abstract mirror of what their workers do when the level is live.
3. **Stage up** when `pop` crosses the next stage threshold.
4. **Research**: advance a per-presence research counter; on completion
   push the next `research` entry into the presence's `KnownTechs`
   (launcher range, better units, etc.).
5. **Spread**, by `mobility.kind`:
   - `launcher`: when `pop >= spread_pop`, the current stage has a
     `launcher`, and `cooldown_days` have passed since the last shot, pick
     a location within `base_range` plus any range techs (star-map
     distance, same metric as `FuelCost`) and fire: spawn a **projectile
     fleet** carrying a stage-0 presence with `Population 1` and the
     `payload` blueprint. Prefer unoccupied sites with the economy's tag
     (`biomass`). The hive keeps growing; the shot costs one egg from
     `Stockpile`.
   - `fleet`: the fleet itself decides to move (greyish target rule
     below).
   - `signal`: with a small chance per day, seed a stage-0 presence at an
     adjacent system directly. Autonz only.
6. **Skip** locations whose level is currently loaded (live or parked):
   those are simulated for real and reconciled on freeze.

**Fleets** carry a `Kind`: `ship` (greyish, mirror) or `projectile` (an
egg in flight). Both advance `Progress` along `From→To` by speed; on
arrival a ship is parked at the location and its presence grows/works like
any other, while a projectile is consumed and its presence is added to
`loc.Factions` (or merged into an existing presence of the same faction).
Projectiles are visible on the Star Map once their origin or target is
Discovered, so a human player can see an egg incoming and prepare.
Greyish doctrine: pick the destination whose archetype tags feed the next
entry in the greyish tech list (the abstract mirror of the player choosing
where to land), mine for N days, then leave with the stockpile. Abstract
research is a counter per day that pops the next tech into
`Fleet.KnownTechs`; a tech can unlock a better unit type for the
bioprinter. **Ship tier only rises when the fleet is at the mothership**
location, which caps growth over a long campaign and gives the player a
target. The mothership is itself a fleet with `ShipTier` max and no
`ToID`.

### Initial seeding and the faction-count option

How many AI factions a campaign has is the **player's choice at New
Expedition**, alongside name, seed and the tutorial toggle in
`title_state.go`. Decided 2026-11-05.

- A stepper **"Rival factions: 0 … N"** on the New Expedition screen. `0`
  gives today's game (hostile waves still come from scenarios; nothing
  expands). Default and max come from `data/generation.json`
  (`factions_default`, `factions_max`), so balancing is data.
- The value travels with the seed: `GenerateCampaign(name, seed, opts)`
  gains a `GenOptions{Factions int}` and `Campaign.FactionCount` persists
  it, so a saved game knows what it was created with and `Expand` /
  `MaybeTravelQuests` can scale later seeding to it.
- **Which** factions is random, drawn without replacement from the
  doctrines flagged `"seedable": true` in `data/factions.json`, with the
  campaign RNG (`Seed + GenSeq`). This follows the scenarios-are-random
  decision: the player tunes pressure, not the roster. A per-faction
  toggle can come later as an "advanced" row if wanted; it is UI only.
- Each chosen faction gets one stage-0/1 presence on a generated system
  that is **not** the start system and not within one jump of it (so the
  first landing is the player's), biased toward systems whose tags match
  its economy. Fleet factions start as a parked ship at that system; the
  greyish mothership is placed far from Home. Mirror colonies start as a
  fleet aboard a ship at a system of their own.
- `FactionCount` also scales the Tier 1 **spread** budget: the launcher
  and signal rolls use `cooldown_days` divided by a pressure factor so
  "many factions" and "aggressive factions" are one knob, not two.

## Tier 2 — Level: materialize, simulate, reconcile

### Materialize (on `buildParked` → after `spawnLocationFixtures`)

For each `FactionPresence` on the location (`loc.Factions` plus the
presence of every fleet in `loc.FleetIDs`), seeded by `presence.Seed`:

- Create a **faction settlement** (`settlement.Settlement`) named
  `"<faction>:<locID>"` with a centre picked like the player's plaza
  search. Settlements already own a `TaskScheduler`, which is the whole
  point: the enemy gets its own task queue.
- Stamp the stage's structure script (`stamp` feature kind,
  [stamps.md](../developer/stamps.md)) at the centre. Place faction-owned
  `StorageComponent` containers inside and fill them from `Stockpile`.
- Spawn `Population` workers from the stage's `workers` list, each with
  `Worker` + `SettlementComponent{faction settlement}` + their existing AI.
  Spawn `guards` with the existing `ScriptedAI`/`FactionAI` only.
- Apply `Conversion` as a biome patch radiating from the centre
  (`generation.ApplyBiomes` machinery, new "overlay" entry point).
- Hive: additionally scatter eggs per `troid_outbreak`'s spawn rules so
  the `GameMaster` wave spawner keeps its current role for *fighting*
  pressure; the director (below) handles *economic* pressure.

Idempotent on reload: a `Materialized bool` per presence on the level save,
mirroring how `Fixtures` are skipped once spawned.

### Simulate: the faction director

What the player's cursor is to colonists, a **director** is to a faction
settlement. New system `internal/systems/FactionDirectorSystem.go`, one
pass per faction settlement per N ticks (not per entity):

- Reads the doctrine's `tasks` list and posts tasks onto the settlement's
  `TaskScheduler` using the same request constructors in
  `internal/task_requests`: `MineAction` on the nearest unclaimed deposit
  (strip / hoard), `ForageAction` on flora (biomass), `DigAction` to
  expand the hive, `BuildAction` for the next stage's structure, and
  `PickupAction` to haul loose items home.
- A **new task action**, `ConvertAction`: worker stands on a tile and
  replaces it with `conversion.tile` over `Required` ticks, using the
  progress-on-task model. This is also what gives the *player* terraforming
  later ("just like the players would"), so it goes in `task_requests`, not
  in the director.
- Faction workers run through the existing `WorkerSystem` unchanged. The
  only worker-side change: task pick-up must filter by the worker's
  settlement, which `SettlementComponent` already provides.
- Growth while live: the director spawns a new worker from `workers` when
  the stockpile crosses a food/ore threshold (biomass → hatch an egg; ore →
  print an enforcer at a stamped fabricator). Same mechanism the player
  uses at the Robot Workbench, so later "steal their fabricator" is free.

**The egg launcher on a live planet.** No buildable has a footprint today
(`construction.Buildable` has no size field), so the launcher is a stamped
structure for its visuals plus **one core entity** carrying a
`LauncherComponent{Range, Charge, Required, Payload}`. Building it is a
normal `BuildAction` on the core, which runs the stamp on completion.
Firing is a task chain the director (or the player) posts: haul one
`payload` item to the core (a carried input, per the physical storage
plan), charge for `Required` ticks, then fire, which spawns a projectile
fleet on the campaign exactly as the abstract step does. Range techs raise
`Range` on the component when researched. The same component and tasks
serve the player when xenos are playable (see "Playable races").

Combat stays as it is: `rlcombat.IsFriendly` compares
`Description.Faction`, so two factions on one planet already fight each
other and both fight colonists. Doctrine `aggression` just sets the
`hostility` var the humanoid script already reads.

### The mirror colony and observer mode

A **mirror colony** is a faction whose doctrine is the player's own game:
colonist blueprints, the real build menu, the real crafting recipes and
`research.json`, a bioprinter for growth. It exists for two reasons.

1. **It is the director's unit test.** Everything the greyish need on a
   live planet is "a director running a colony with a different unit list
   and tech tree". Building the director first against the player's own
   settlement proves it on content that already exists, and greyish become
   a doctrine swap (unit list, tech list, leave-when-done rule).
2. **Observer mode.** A flag (config, or a scenario option) attaches the
   director to the *player's* settlement and leaves the cursor inert. The
   game then plays itself, which turns the vmtest harness
   ([vmtest.md](../developer/vmtest.md)) into an integration and balance
   tool with no harness changes:
   - a vmtest script starts a campaign with the flag set and `wait_until`s
     on milestones (first bed, first storage, first research done, N
     colonists alive at tick T) with a tick budget;
   - the sidecar state snapshot gains **settlement stats**: colonist count,
     storage totals, structures built, techs known, tasks pending and tasks
     stuck (no progress for N ticks);
   - a second, faster layer is a plain Go test: `generation.BuildWorld`, attach
     the director, step the logic systems for N ticks, assert on the
     settlement. The `workerai` tests already construct levels; whether the
     full system stack steps cleanly outside `MainState` is **unverified**
     and is the first thing to check.

   Soak runs (ten seeds, 5,000 ticks) directly address two todo Verify
   items, building cost balance and the resource tracker, and act as a
   regression test for the physical-storage work: a director that stops
   building is a hauling bug.

Director design rules that follow from this:

- **Goal-driven, not scripted.** A needs ladder (shelter → storage → food
  → defence → research), with the doctrine supplying weights and the unit
  and tech lists. A build-order script would pass the observer test and
  teach nothing, and would not transfer to greyish or autonz.
- **Determinism.** Systems use global `rand`. Observer runs assert on
  milestones and ranges, not exact state, unless a seed is threaded through
  (frame diffs are already noisy for the same reason).
- **Faction-blind plumbing.** The director posts tasks through
  `task_requests` onto its settlement's `TaskScheduler`; `WorkerSystem`
  stays unchanged. Only the colony-worker predicate (finding 1) tells the
  UI and the loss check which settlement is the player's.

As an in-game faction the mirror is a second life-raft crew competing for
the same planets. It can start neutral and turn hostile only when the
player lands on a planet it holds, which is a different pressure from the
xenos and a natural place for trade later.

### Reconcile (on `Freeze`)

Alongside `loc.Colonists` and `loc.StorageSummary`: count living entities
per faction settlement → `Population`; `storage.Summarize` over the faction
settlement's owners → `Stockpile`; count converted tiles / total →
`Conversion`; keep `Stage`. If `Population == 0` and no eggs remain, drop
the presence (planet cleared). For a fleet-carried presence the record is
written back onto the `Fleet`, and a fleet with zero population is removed
from `Campaign.Fleets` (its ruins stay as a stamped structure in the level
save). Delete the faction settlement from the global map here (finding 5).

### Keep-loaded rule

Decision 3 in the background doc already anticipates "a dedicated
keep-loaded component (e.g. for hostiles)". **Do not** add it here. A
planet with colonists *and* a faction presence is live or throttled
already, which is exactly when you want the attack notification. A planet
with only a faction presence is frozen and Tier 1 handles it.

This only holds once Part B's staffed check uses the colony-worker
predicate from finding 1. With a bare `Worker` check, every infested
planet would count as staffed and stay loaded.

## Tier 3 — Star Map and player counterplay

- `starmap_panel.go` shows each location's presences: icon per faction,
  stage pips, "stripped" / "converted" badges. Unvisited, undiscovered
  presences are hidden; a ship scan (`Campaign.Scan`) reveals them, which
  makes scouting a real decision.
- Fleets draw as moving markers between systems.
- **Interception**: when a `Travel` path crosses a fleet (segment
  distance < threshold) roll an encounter: a `kind: "encounter"` location
  generated with `primer_station` (greyish ship interior) that the player
  can board or decline (fuel penalty to evade). This reuses the existing
  station primer and parked-level flow with no new level type.
- Quests: `exterminate_xenos` / `autonz_incursion` templates gain
  `"target_presence": "<faction>"` so bounties are posted against real
  presences and complete when the presence is dropped on reconcile.
- Stakes: stripped planets lose `ore` tag yield for the player; converted
  planets change biome and spawn rules. That is the real pressure, more
  than the hostiles themselves.

## Playable races (design constraint, not a phase)

The doctrine is the unit list, build menu filter, research list, growth
mechanism and mobility kind. If the **player's settlement reads those from
a doctrine too**, a playable race is a title-screen choice plus a doctrine
entry. If the player side keeps its hardcoded defaults (colonist blueprint,
one build menu, one tech tree, the craft handler's `"colonist"` special
case) while only the AI uses doctrines, the refactor happens twice.

So, adopted now at zero cost to the schedule:

- From C0 on, the player's settlement carries a `Doctrine` and the build
  menu, research tree, printer unit list and `SeedShipCrew` filter through
  it. The default doctrine is `"colony"`, which reproduces today's game
  exactly.
- The colony-worker predicate (finding 1) means "my settlement", never "the
  colonist blueprint".
- Campaign gets a `Mode` derived from the doctrine's mobility: `ship`
  (hold, roster, fuel, travel, reach-Home win) or `launcher` (no ship; the
  player views any live hive from the Star Map for free, launches eggs from
  a charged launcher, several hives tick at once via the background sim,
  and wins by an alternative rule such as infesting N systems or landing an
  egg on Home).

What each race then costs:

| Race | Mode | Extra work beyond the doctrine |
|---|---|---|
| Greyish | ship | Own build/recipe/tech content; `Needs` component (or a race that does not eat); bioprinter unit list. Nearly free. |
| Autonz | ship | Own content; `ConvertAction` as a player build. |
| Hive | launcher | The `launcher` campaign mode: Star Map action set, no-ship hub, alternative win rule. Bounded, mostly UI. |
| Mirror | ship | Nothing; it is the default. |

Needs, stats, equipment slots and the colonist modal stay shared across
races; a race that lacks a component simply skips that system.

## Phasing

Faction order is **hive → mirror colony → greyish → autonz**. The hive needs
only A and B (no director, no worker reuse) and prove the campaign tier.
The mirror proves the director on existing content. Greyish are the
flagship once both exist. Autonz combine the two.

| Phase | Deliverable | Why here |
|---|---|---|
| **A** ✅ | `FactionPresence` + `data/factions.json` + Tier 1 step on `Campaign.StarDate` + save/load (`CampaignVersion` 3). **New Expedition "Rival factions" stepper** and initial seeding. Star Map badges for seen presences, named by morph. Legends events. Hive doctrine only. No level changes. Built 2026-11-08. | Cheapest, fully testable with unit tests (deterministic RNG). |
| **B** ✅ | Merge troid/xeno blueprints into hive morphs (shared scripts, `flying` on fliers, tier-at-spawn in `factory`); materialize the hive using **existing** stamps, scenarios and hostiles; reconcile on freeze; husk on exhaustion; abstract launcher spread (projectile fleets) with Star Map motion; a live hive still launches. No director yet. Built 2026-11-08. | Biggest visible payoff: planets change while you're away. |
| **C0** ✅ | Doctrine-driven player settlement: build menu, research tree, printer unit list and `SeedShipCrew` read from a doctrine; `"colony"` reproduces today's game; vmtest scripts unchanged. Also finding 4's `deploy_on_craft`. The player's techs move onto its own presence (`CampaignVersion` 4). Built 2026-11-08. | Pulled out of C1 (second review); it is the refactor the playable-races constraint requires and should land alone. |
| **C1** ✅ | The director, run against the **player's own settlement**: observer-mode flag, settlement stats in the vmtest sidecar, one observer vmtest script, one Go soak test with written failure thresholds. Built 2026-11-08. Observer mode v2 (observer runs: no player ship, the expedition a fleet on the expedition table like the mirror; View any planet; the console lock) built after E: see "Observer mode v2 as built". | Proves the director on content that exists; delivers the integration/balance tool. |
| **C2** ✅ | The director digs; faction settlements on loaded levels; mirror colony as a fleet-carried faction (ships, landing and lifting off, neutral until contact); `ConvertAction`; finding 3's hostility seeding. Built 2026-11-08. | Rival-colony feel on the live planet. |
| **D** ✅ | Fleets, greyish doctrine (tech list, bioprinter units, ship tiers, mothership), interception encounters, ruins on departure. Built 2026-11-08, with C2's two gaps closed first (raids, food). | Depends on A–C; most new UI. |
| **E** ✅ | Autonz doctrine: convert economy on a planet carrier, signal spread. Built 2026-11-08, with D's crew-wipe-while-loaded and mirror-boarder gaps closed. | Pure data plus `ConvertAction` once C2 exists. |
| **E2** ✅ | The hive as a settlement faction ([hive_plan.md](hive_plan.md)): unit types replace colour tiers; drones, eggs laid on larvae, resin creep, an evolve tree, a hive director; Tier 1 bookkeeping. Built 2026-11-08. | Puts the hive on the machinery F hands the player. |
| **E3** ✅ | The autonz as a settlement faction ([autonz_plan.md](autonz_plan.md)): unit types with steel and bronze print tiers, an assimilation tree (a second Tier 1 walk), structures on wire (cogitator, Signal Core, relay and defence pylons, foundry, converter nodes), the deploy order, the transmit walk, an autonz director. Built 2026-11-08. | The second faction on F's machinery; Tier 1 unchanged. |
| **B+** | Hidden identity: `LoreLinks`, per-morph bestiary/badge/quest/log naming, the four discovery paths. | Presentation only; can land any time after B, best before D so greyish badges reuse it. |
| **F** ✅ | Playable races ([playable_races_plan.md](playable_races_plan.md), every open question decided 2026-11-09): the title screen's Race row; a spreading race (hive, autonz) is a no-ship run whose people are presences of faction `player`, viewed to give orders, run by their director in assist; win holding six worlds, lose with nothing left. Steps 1 to 5 built 2026-11-09 (a playable hive run; the hive's interface from the doctrine's `interface` block; rung-group toggles and the launch as an action; the world from the hive's side; an autonz run); step 6 built 2026-11-13 on the greys' landed content: a ship race with a grey crew, linked lockers and a saucer grown in rings and decks. | Everything it needs exists by E; F is content plus the no-ship Star Map. |

Phase A ships without B being visible, and C1 is useful with no enemy
faction at all, so the todo should list them as separate items.

## Open questions

- ~~**Campaign clock.**~~ Answered: `Campaign.StarDate` is the single
  clock (lore plan); jumps cost `JumpDays(fuel)` (`ceil(fuel /
  jump_fuel_per_day)`). Tier 1 steps on it.
- **Carrying capacity formula.** Tag-based in A (as built: `base` + per tag
  + per kind in the doctrine), replaced by the real tally once B has a level
  to count from.
- **Faction-vs-faction on the campaign tier.** Ignored in A (as built);
  revisit after B.
- **Does the player get `ConvertAction`?** The design wants it ("just like
  the players would"); gate it behind research so it lands with C. Phase C2
  answer: the order path exists and is refused until the settlement knows
  `terraforming`, which no research entry gives yet (no player content).
  Phase E: unchanged. The autonz use `ConvertAction` as their economy (the
  faction order path is ungated); the player's gate stays for F, where a
  playable autonz (or a terraforming tech) decides it.
- **Hive spread: decided.** Population-triggered egg launcher with
  researchable range, shared by AI and player; eggs in flight are
  projectile fleets. The earlier "player as vector" idea (eggs riding the
  hull) is demoted to optional flavour. Phase A answer: each shot needs a
  site with no presence of that faction and no egg of it already bound
  there; with none in range the hive holds fire. Revisit if hives go idle
  too often.
- **Launcher mode win rule.** Infest N systems, land an egg on Home, or
  survive to a day count? Decide with F.
- **Hive merge fallout.** `lore_troid_test` / `lore_xenos_test` and the
  scenario text assume two species; scenarios stay as different hive
  compositions (random-scenario rule) but their copy and the tests move to
  morph names. Also: should the two scenarios' hostiles *both* still appear
  in a `0` rival-factions game, now as one friendly-to-each-other hive?
  Phase B answer: the blueprints are one faction (`hive`), so every xeno and
  troid is friendly to every other, scenario-spawned or presence-born. The
  scenarios stay as they were: `troid_outbreak` a hive of fliers,
  `exterminate_xenos` of crawlers, each spawning only its own morph, so a
  `0` rival-factions game looks as before. A presence on either (a "home
  scenario") builds by the chapter's origin and is one organism with it;
  only the presence's own creatures count toward its population.
- **Greyish growth ceiling.** Ship tier rises only at the mothership
  (decided above). Phase D answer: destroying the mothership (boarding it
  and killing its crew) freezes every greyish ship at its current tier,
  stops new ships being built, and scatters the ships bound home to the
  free site nearest where each was; its hoard is the boarding party's loot.
  Why: the mothership is the faction's growth ceiling and the player's
  target, so destroying it should end the escalation without erasing ships
  the player has not fought (they stay a threat at their size, landing
  wherever they were), and one boarding should not end a campaign-long
  faction outright.
- **Mirror colony diplomacy.** Neutral until the player lands on a planet it
  holds, or hostile from the start? Proposal: neutral in C2, hostility and
  trade as a later phase. Phase C2 answer: neutral until the player beams a
  colonist down onto a site it holds, then hostile everywhere
  (`Campaign.Diplomacy`); trade is later.
- ~~**Go-test stepping.**~~ Answered (second review): step a headless
  `MainState` as the physical-storage and tutorial tests do.

## Phase A as built (2026-11-08)

Developer doc: [factions.md](../developer/factions.md).

- **Record.** `FactionPresence` on `Location.Factions` with the data model's
  fields plus `id`, `growth` (fractional population), `known` and `events`
  (visibility). Eggs in flight are `Campaign.Eggs []EggFlight`: from, to,
  launch and arrival day, no position. That is the minimal in-flight record;
  Phase B replaces it with a `Fleet` and Star Map motion.
- **Clock.** `StepFactions(loaded)` from `TickBackground` and `Travel`,
  catching up day by day from `Campaign.FactionDay`. No `AdvanceDay` hook.
- **Loaded planets** are skipped, and also refused as egg targets; an egg
  already bound for one waits in orbit. Phase B needs the live-hive launch
  rule from the second review.
- **Exclusions.** `FactionSiteOK`: a site, not the Wreck Site, not the
  Training Ground, and not a first-jump site (`firstJump`, the one
  `BindSiteSafe` keeps safe) until the player has visited it; seeding
  refuses first jumps outright, so the start is always clean. This replaces
  "not within one jump of the start", which the adrift start made
  meaningless (every opening site is within one jump of the Wreck Site).
- **Seeding** draws without replacement and refills the bag, so with only
  the hive seedable, `n` means `n` hive broods on `n` sites. Own random
  stream: a seed's map is the same at every count.
- **Hazard** multiplies growth by the site's scenario id; a negative factor
  kills the presence (`crystalline_blight`). The history is read, never
  written: faction events use `History.RecordBeside`, which names the site
  for Legends without joining its event list.
- **Visibility.** A presence is seen from orbit or by a scan sweep within the
  scanner's range. Events about an unseen presence are recorded unknown and
  revealed with it, so Legends never leaks a hive the player has not found.
- **Hive.** One doctrine, `hive` (xenos and troids merged), renamed before
  any save held a presence, so `CampaignVersion` stays 3. `troid_outbreak`
  feeds growth like `exterminate_xenos`. Research is `tier_advanced`,
  `launcher_range_1`, `tier_elite`, `launcher_range_2`; the tier entries are
  inert. Hidden identity: the Star Map, the orbit message and Legends name
  the morph, picked by the site's scenario from the doctrine's
  `morph_by_scenario` (troid_outbreak is fliers, else crawlers) until B+
  records what was seen.
- **Not in A:** stockpile and conversion work, the egg as a cost from
  `Stockpile` (a shot costs population instead), fleets, research entries in
  `research.json` (the walk lives in the doctrine for now), any level change.
- **Firing rule.** The plan's fixed `spread_pop` became a share of the
  site's capacity with a small floor (`spread_share`, `spread_min`), and the
  launcher stage can be reached at a share of capacity too, so a small site
  can fire. Below `crowded_capacity` the cooldown stretches by
  `crowded_capacity / capacity` and the first shot waits it from arrival, so
  a cramped hive fires rarely; elsewhere the first shot waits only on growth.
- **Balance** (tuned 2026-11-08 at the owner's request; seeds 1–6, opening
  map, no player; presences / population, min–max):

| Rival factions | Day 0 | Day 40 | Day 100 | Day 200 | Day 300 |
|---|---|---|---|---|---|
| 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |
| 1 | 1 / 3 | 1 / 8–22 | 1–2 / 8–44 | 3–5 / 37–78 | 6–9 / 89–144 |
| 2 | 2 / 6 | 2 / 24–44 | 3–4 / 50–72 | 6–9 / 89–157 | 6–10 / 89–164 |
| 3 | 3 / 9 | 3 / 48–64 | 5–6 / 64–93 | 6–10 / 89–162 | 6–10 / 89–164 |

  Rival factions 1 holds 2 hives at day 100 on five seeds in six (seed 4,
  starting on a capacity-8 site, has 1), about half the reachable sites at
  day 200 and full at day 300; Rival factions 3 is at about half by day 100
  and full by day 200. A fresh hive's first shot comes 20–37 days after it
  arrives (about 75 on a crowded site). `TestFactionPaceRivalOne` pins it.

## Phase B as built (2026-11-08)

Developer doc: [factions.md](../developer/factions.md) ("Tier 2", "Fleets",
"The hive on a level").

- **Morphs.** `data/blueprints/entities/hive.json` replaces `troids.json`
  and `xenomorphs.json`: one blueprint per morph and stage plus the two eggs
  (and their cracked shells), every one faction `hive`. The blueprint ids
  stayed (`xeno_*` are crawlers, `troid_*` fliers), so history evidence,
  quests, objectives, scenario scripts and saves keep working; the display
  names are the morphs' ("Xeno", "Troid"). Fliers carry `flying`. A
  `Kin` component holds a creature's morph, tier and presence. Shared stage
  scripts in `data/scripts/hive/` grow and lay through a new `spawn_kin`
  builtin, which names the next stage after the creature's own blueprint
  prefix and passes on its tier and presence; old saves' troid/xeno script
  paths map onto them.
- **Tiers** are applied by `factory.ApplyTier` at spawn from the doctrine's
  `tiers` (`sprite_dx` 144 per colour block, `health` and `str`
  multipliers). A presence's tier is the highest whose `tech` it knows, so
  research does move it: on the opening maps most hives know
  `tier_advanced` within about 25 days of reaching the nest stage, and land
  red crawlers and orange fliers. Scenario and GameMaster creatures stay
  basic. The launcher keeps its sprite at any tier.
- **Materialise** (`materialiseFactions`, `buildParked` after the fixtures):
  a centre from the presence's seed with the whole footprint `hazard_keep_out`
  (40) outside the landing reserve and clear of every stamp, by the
  chapter's origin on a home scenario (`exterminate_xenos`,
  `troid_outbreak`), saved as an origin flag; the stage's stamp (new
  `hive_nest` 13×13 and `hive_core` 21×21 resin chambers, terrain only); the
  `egg_launcher` (Resin Spire) at the launcher stage; bodies topped up or
  culled to the record's population: `egg_share` 0.3 eggs, `adult_share`
  0.4 of the rest adults, the stage's mix by morph (swapped where the site's
  morph is the flier), eggs by the scenario's egg spawn rule when it has one.
  It is idempotent and works from what the level already holds, so growth or
  a stage reached while away appears on the next landing. The plan's
  "faction settlement" and stockpile containers wait for the director (C).
- **Reconcile** runs on every `writeLocation` (saves and freezes), so a
  reload never culls live growth: bodies into `population`, the stage rising
  with them, a destroyed launcher marked lost (`launcher_lost`, regrown by
  Tier 1 after the cooldown; no shots meanwhile). No bodies left is a wipe:
  `faction_wiped` (known), presence dropped. A presence that dies out in
  Tier 1 is exhausted: `faction_exhausted` and a `husk` record, which the
  next build stamps once as the doctrine's husk (`nest` with cracked eggs)
  while its creatures die out.
- **Fleets.** `Campaign.Fleets` (kind `projectile`) replaces `Eggs`: the
  carried brood as a `FactionPresence`, the endpoints, launch and arrival
  day; `Pos` interpolates by star date and tick. Spread numbers are
  unchanged (the pace test passes as it was). A fleet is known when the hive
  that fired it is known (at launch or when the player later sees the hive),
  when a scan sweep covers it, or when it is within the ship scanner's range
  of the ship. The Star Map draws known eggs as a small egg in the badge
  colour with a dashed line to the target, and the target's info panel an
  `XENO EGG INBOUND · N DAYS` tag. The `eggs` field only existed on this
  branch, so no migration.
- **Live launch.** Once a star-date day the world manager runs
  `Campaign.LiveLaunch` for presences on loaded levels: the live count
  becomes the record and the Tier 1 rule fires with the same `last_shot_day`
  cooldown, only while the launcher stands; the shot's bodies leave the level
  (eggs nearest the spire first), `faction_launched` is recorded, and on the
  planet in orbit an alert names the morph. A per-day check rather than
  per-tick: the rule counts in days.
- **GameMaster** unchanged: it keeps the fighting pressure, and presence
  creatures count toward a scenario's `hostile_max`.
- **Balance** unchanged from Phase A (the dump below re-run after B gives
  the same table); Phase B changes no Tier 1 number.
- **Not in B:** the director, faction settlements and stockpiles (C), the
  launcher as a task chain, stage stamps while the planet is loaded (a stage
  reached live stamps on the next build), faction-vs-faction, `LoreLinks`
  (B+).

## Phase C0 as built (2026-11-08)

Developer doc: [factions.md](../developer/factions.md) ("The player's
doctrine").

- **The colony doctrine.** `data/factions.json` gains `colony` (the same
  file as the enemies, so one schema serves every race): `playable`, not
  `seedable`, `mobility.kind` `ship`, `economy` `industry`, and a
  `settlement` block. The block is `crew` (blueprint and share), `needs`,
  `build_menu` (category ids and labels in order), `buildables` (build.json
  keys), `research` (research.json keys), `units` (the recipes that make a
  unit: the bio-printer's colonist and the four Robot Workbench robots) and
  `recipes` (every other crafting recipe). The colony's lists are exactly
  what the game offered before: every categorised build.json entry, every
  tech, every recipe, colonists for crew. The shared content files keep
  their own schemas; a doctrine names entries in them. The hive has no
  settlement block yet (no player hive content); a playable hive or greyish
  fills the same block in F.
- **One place picks it.** `campaign.PlayerDoctrine(c)` reads
  `Campaign.Player.Doctrine`, else `DefaultPlayerDoctrine` (`colony`); a new
  campaign's player presence takes `GenOptions.Doctrine` (`""` is the
  colony), which is the title-screen hook for F.
- **Readers** (`internal/game/player_doctrine.go`). The HUD's build menu is
  `SetBuildMenu` with the doctrine's categories, each filled from
  build.json's `category` / `order` and filtered by `buildables`; the orders
  category's cursor modes stay in code (they bind to cursor-mode
  constants). The research modal walks `research.AvailableTechsIn` /
  `LockedTechsIn` over the doctrine's tree, and queuing a tech outside it is
  refused. The crafting modal lists a station's recipes that are the
  doctrine's units or recipes, and queuing another is refused.
  `SeedShipCrew` (and the non-campaign start's five colonists) spawn
  `CrewBlueprints(n)`: `n` split by share. The unlock-all-research debug
  option unlocks the doctrine's tree.
- **Known techs.** `Campaign.Player` is the player's settlement as a
  `FactionPresence` (id `player`, faction and doctrine `colony`), on no
  location, so Tier 1 never steps it. Its `KnownTechs` replace
  `Campaign.KnownTechs` (now a method). `CampaignVersion` 4: a version 3
  save's `known_techs` migrate onto the presence. Non-campaign play keeps
  the techs on its `Settlement`, as before.
- **`deploy_on_craft`** is a recipe flag (finding 4): the output joins the
  crafter's settlement at the station, with a `Worker`, instead of going
  into the bag. Only `print_colonist` sets it, so the robots still go into
  the crafter's bag and deploy when put down, as before; set it on their
  recipes to change that.
- **Bio-printer** (fixed after C0, owner's request). The crafting modal
  sent a recipe's output as its ID, so `print_colonist` (output
  `colonist`) failed the lookup and the bio-printer could never print; it
  sends the recipe key now (`Recipe.Key`), which the craft queue and the
  craft handler already expected. The Bio-Printer had no build-menu
  category, so it could not be placed: it is now last in Structures
  (`order` 15) and in the colony's `buildables`, the one deliberate change
  to the old menu. It has no `required_tech` (nor does `print_colonist`),
  so nothing gates it; the Robot Workbench's `robotics` gate is unchanged.
  `TestBioPrinterPrintsAColonist` opens the printer's modal, queues its
  row and ticks the colonist out at the printer.
- **Declared, not read yet:** `needs` (the crew blueprints carry the
  component), `Doctrine.CampaignMode` (`ship` / `launcher` from the
  mobility), `economy` for the colony.
- **Hidden identity fix.** The orbit alert ended with the presence's stage
  name in parentheses, "(hive)" at the top stage, which named the faction.
  It is now "Xenos presence detected on Rust Tangle." Both faction alerts
  are `Doctrine.OrbitAlert` / `LaunchAlert`, checked by the hidden-identity
  test. The stage word still shows on the Star Map ("XENO HIVE" tag,
  `[xeno hive]` row note) and in Legends (`faction_grew` "grew into a
  hive", `faction_launched` "the Xenos hive on ..."); that wording is the
  owner's to change, with B+.
- **Regression check.** Every vmtest script passes unchanged. `tutorial`,
  `new_expedition`, `storage_beam`, `save_load` and `esc_menu` frames match
  a run from the commit before C0 (at most 0.009% of pixels, tile
  animation); `factions_site` and `factions_starmap` differ only by the
  alert's text. Go tests snapshot the old build menu, tech list, recipe and
  unit lists and crew and compare the colony's, and check that new content
  missing from the colony doctrine fails a test.

## Phase C1 as built (2026-11-08)

Developer doc: [factions.md](../developer/factions.md) ("The director",
"Observer mode").

- **Where it lives.** `internal/director`, not a system: one `Director`
  per settlement, run by `MainState.stepDirector` from `stepWorld` and
  `stepBackground`, every `every_ticks`. It reads a `director.Site` (level,
  colony, centre, task queue, known techs, the doctrine's settlement
  block) and gives orders through `director.Orders`, which the game
  implements with the cursor's own functions (`queueBuild`, `queueMine`,
  `queueRetrieve`, the research and craft requests, Cancel Task). The
  build, dig and mine cursors were refactored onto those `queue*`
  functions with a level argument; the player's behaviour is unchanged.
- **The table** is data: `colony.settlement.director` in
  `data/factions.json` (interval, `max_open`, radius, reach, mining and
  hauling caps, `stuck_ticks`, and the needs ladder: food, materials,
  shelter, storage, workbench, lab, research, defences, arms). A hive
  director is a different table on the same fields.
- **What the plan's rules became.** Goal-driven, as planned: each rung
  wants a count (`min + per_colonist × colonists`) or a stock, and the
  rungs that spend materials share `max_open` orders, higher rungs first.
  Three rules came out of the soak: an order is given only when unpromised
  stock covers it (open orders' remaining costs are promised), build sites
  and mining faces must be reachable from the landing, and a build may not
  cut off anything reachable (beds had sealed a camp's only doorway; open
  builds count as walls). Stuck orders (no work or delivery for 3,000
  ticks) are cancelled and their sites avoided. Choices are deterministic
  for a level, tick and seed; the game's own systems still use global
  `rand`, so runs are measured by milestones and ranges.
- **Observer mode** is the config flag `observerMode` (not a New
  Expedition toggle: no layout change, and a vmtest header's `config` sets
  it). The cursor gives no orders (a click only selects), incidents are
  answered Ignore at once and logged, the Star Map's pause refuses, and the
  HUD shows `OBSERVER` above the pause marker. Space still pauses a site.
- **Stats** (`MainState.settlementStats`, the sidecar's
  `settlement_stats`): alive, dead, beds, colony storage, food stored or
  carried, open builds and orders, research open, techs, day, tick,
  structures built on the level, the director's orders and cancellations.
- **Scripts.** `observer` (seed 777): four colonists and a 30 metal / 15
  crystal kit land, and by day 8 (tick ~3,800) the director has built four
  beds, two lockers, a lab and a gun bench, crafted two stun batons and
  researched Basic Metallurgy, all four alive. The observer does not run
  the landing itself: the script (and the soak) does the player's opening
  (travel, beam the crew and a kit down), since the director plans on a
  site and does not beam.
- **Soak** (`make soak`, `TestObserverSoak`, skipped unless `LP_SOAK=1`):
  seeds 1–10, a party of four and the kit on each start system, 20,000
  ticks headless, about 95 s. Thresholds, written in the test: more than
  two seeds missing a milestone fails (a director bed built by tick 5,000,
  a storage container by 8,000, a research order by 15,000), and more than
  two seeds ending with fewer than three alive fails. First run
  (2026-11-08), first tick each was reached (sampled every 50):

| Seed | Bed | Locker | Lab | Research | Alive 5k / 10k / 20k | Techs | Built |
|---|---|---|---|---|---|---|---|
| 1 | 200 | 250 | 1200 | 1250 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 2 | 200 | — | — | — | 4 / 4 / 4 | 0 | 1 bed |
| 3 | 200 | 250 | 1200 | 1200 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 4 | 250 | 200 | 1200 | 1200 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 5 | 250 | 150 | 1100 | 1100 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 6 | 250 | 200 | 1150 | 1150 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 7 | 250 | 200 | 1150 | 1150 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 8 | 300 | 200 | 1050 | 1050 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 9 | 250 | 200 | 1100 | 1100 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |
| 10 | 250 | 250 | 1000 | 1000 | 4 / 4 / 4 | 1 | 4 beds, 2 lockers, lab, gun bench |

  No workbench was ever built: every start camp has one within reach. No
  colonist died in any seed in 20,000 ticks, and no order was cancelled.
- **What the soak found.**
  1. *Sized builds ate their materials when someone stood in the
     footprint* (a colonist crossing a bed's second tile): the build
     finished with nothing placed. Fixed in the build handler: the work is
     held at done until the tile clears, and a builder standing in its own
     footprint steps out first. Before the fix, seven to eleven beds were
     built for four standing.
  2. *Every seed stalls after the opening kit.* The director mines only on
     the landing's level, and the opening maps have no ore on the surface,
     so once 30 metal is spent (beds, lockers, lab, bench, one tech)
     nothing more is built or researched. Digging down (stairs, then
     mining a lower level) is the next director capability; the soak's
     research milestone is the first order, not a finished second tech,
     for this reason.
  3. *Seed 2's start camp is sealed.* The colony lands inside a walled room
     whose only exit is an airlock its faction may not pass (an unowned
     door), so the crew can never leave it; the director builds one bed
     inside and stops. A player would be trapped the same way. Content or
     scenario fix, not the director's.
  4. The scheduler's restart delay for a stopped task is five seconds of
     wall-clock time, so a headless run (tens of thousands of ticks a
     second) stretches it into thousands of ticks; anything that relies on
     `Stop()` looks slower in the soak than in play.
- **Not in C1:** faction settlements on loaded levels, `ConvertAction`,
  hostility seeding (C2); the director digging, mining below the surface,
  beaming supplies, assigning or equipping colonists, or fighting.

## Observer mode v2 as built (2026-11-08)

The owner's request after E: "I shouldn't control the ship; I should
instead be able to go to any planet I've got selected." Then, before it was
committed, a design change: observer runs are no-player runs. There is no
player ship; the expedition the player would have flown is a fleet-carried
colony-doctrine presence, flown by the expedition table like a mirror, and
the viewer is a free camera. Docs: docs/developer/factions.md, "How a
faction moves and acts" and "Observer mode";
docs/developer/background_simulation.md, "Viewed planets".

- **Tests never read the player's config.** `config.applyLocalOverrides`
  is skipped under `go test` (`testing.Testing()`): the owner's
  `observerMode: true` in `data/config.local.json` had turned the whole
  `internal/game` suite red. The game still applies the file;
  `internal/config/local_override_test.go` proves both.
- **The `TestMirrorFedFortyDays` flake, traced.** The test was seeded
  (campaign seed 1) but the AI, needs, combat and dice code (here and in
  ml-rogue-lib and mlge) draw from the global `math/rand`, which Go seeds
  at random. Seeding it (`rand.Seed` with `//go:debug randseednop=0` in a
  test file, `seedGlobalRand`) makes the run identical every time; twelve
  global seeds then showed the cause. The hunt rung sends hunters out to
  `hunt_reach` (90), the critters die there and drop their protein bars
  where they fall, and the food haul only looked within `reach` (40): the
  meat was never brought home (6 to 13 bars lying loose by day 40), stored
  food sat at zero, the crew ate what they happened to carry, and a crew
  member with an empty bag stood idle while hungry until starving. Before
  the fix, peak hunger was 90 (one short of starving) in five of twelve
  seeds and seed 11 lost a crew member on day 36 (the reported failure);
  after it (the food haul reaches as far as the hunt, `director.haul`),
  peak hunger is 70 to 73 in all twelve, hauls 5 to 8 a run against 1 to 2,
  and all twelve pass. The test now seeds global seed 1;
  `LP_FLAKE_SEED` and `LP_FLAKE_LOG` (1: a line every 250 ticks; 2: with
  the loose food's places) trace it. Crew still keep the food they carry
  rather than sharing it (worker AI, unchanged).
- **Observer runs.** `observerMode` at New Expedition makes the run one
  (`GenOptions.Observer`, `Campaign.Observer`, saved). The crew (6), the
  hold and the start fuel (400) are the expedition fleet
  (`campaign.AddExpeditionFleet`, faction `expedition`, doctrine `colony`)
  at the Wreck Site; nothing is aboard the ship. The colony doctrine gained
  what a fleet presence needs (the mirror's growth, capacity, camp stage,
  research walk; `ship_speed` 3; an `expedition` morph for the Star Map
  tag; neutral diplomacy). Decision, written down: the expedition's techs
  are its own presence's, as a rival's are; `Campaign.Player` is left
  unused in a run. A run is lost when the fleet is gone (the total-wipe
  check counted the player's colonists, of which a run has none, and ended
  the first vmtest run on its first viewed day). The Star Map loses Travel,
  Beam, Scan and the crew lists (the expedition's panel stands there), the
  dashboard To Ship and To Site; the header and destination line give the
  expedition's numbers. View is the whole interface; the HUD reads
  `VIEWING <planet>` and `OBSERVER`; viewing the expedition's planet opens
  on its camp; observer mode draws the whole level (`RevealAll`, rendering
  only: an unstaffed planet was black under the fog in the first frames).
- **The expedition director** is the expedition table on the campaign tier
  (`internal/campaign/doctrine_expedition.go`, `faction_ships_d.go`), not a
  planner in `internal/director` as first briefed: fleets are stepped by the
  campaign, which `internal/director` imports. A first version (a
  `director.Expedition` beaming crew and cargo through the player's
  functions, with Travel and the beams disabled for the viewer) was built,
  verified (vmtest and a three-seed soak) and dropped with the redesign;
  its patch is in the shared notes. Every fleet-carried colony-doctrine
  presence flies by it: the expedition and the mirror (`expedition_from`).
  Moves: `launch` (off the Wreck Site, day 1), `pack_up`, `table` (past the
  table's 4-day stay, a site `fleet_move_margin` better), `stay` (at
  `stay_days` 40, the table's best), `fallback` (the old weighted hop).
  Fuel: `fleet_fuel` 80 at a rival's launch (the expedition's 400 is not cut
  down), `fleet_refuel_per_day` 1 landed, a jump costing its distance,
  never below `fuel_reserve`. On seed 777 over 100 days the expedition
  launched on day 1, landed on Murk Shoals on day 4 (fuel 392), lifted off at
  its stay for Umbral Scar on day 44 (crew grown to 10, landed day 49, fuel
  378) and went back to Murk Shoals on day 89 (landed day 94, fuel 364).
  `TestExpeditionFleetFliesByTheTable` and `TestMirrorFleetFliesByTheTable`:
  four seeds each, 300 days, deterministic, fuel kept; mirror lift-offs
  table/stay/fallback: seed 1 3/3/0, seed 2 5/4/1, seed 3 1/5/0, seed 4
  0/4/0; the expedition lands 4 to 8 times. The greyish have no table.
  No fleet meets another, so the table's `encounter` and `incident_fuel` are
  unused by fleets, as are `found_crew`, `reinforce_*` and `kit` (a fleet
  is one colony).
- **View** (`internal/game/view.go`): load or thaw, keep while viewed,
  freeze on leaving unless a colonist of the player's is there (never, in a
  run), `Visited` restored; a hive site viewed and left steps like one
  never viewed over thirty days (Tier 2 marks aside). Only observer mode
  has the button (in play it would scout for free).
- **The console and the lock.** In a run `observer off` is refused (taking
  over the expedition is a todo). On a normal campaign `observer on|off`
  flips the lock for the session: the settlement director on the player's
  colony, no ground orders, the marker and the reveal; the ship, its
  popups and the Star Map pause stay the player's (C1's lock answered
  incidents Ignore and refused the pause; `TestObserverMode` now asserts
  the popup and the pause).
- **The viewer is omniscient** (the owner's addition after the redesign):
  in an observer run every presence and fleet is on the Star Map from day
  zero, tags name the faction beside the morph (`HIVE/XENO BROOD 3`), notes
  carry populations, and Legends shows every faction event, all as a rule of
  the view (`internal/game/faction_view.go`); the records' Known flags are
  untouched, so a normal campaign is unaffected (`TestRevealOnlyInObserverRun`).
- **The Factions tab** (dashboard, beside Legends): one block per faction
  (colour, population, techs, its sites with stage, population and
  converted share, its fleets with kind, crew, cargo and heading, eggs and
  signals with their days); a row selects its site on the Star Map. Both
  modes, different fog: a normal campaign's shows what the player has seen
  under hidden identity, an observer run's everything. To Site and To Ship
  are narrower to make room for the seventh tab.
- **"No hive on the viewed planet"** (the owner, playing the worktree):
  the hive was built (stamp and bodies; `buildParked` materialises after
  the fixtures), but the camera opened on the scenario's plaza, far from
  it. View now opens on the expedition's camp, else the largest presence's
  stamp (`frameView`). `TestViewBuildsThePresence`: Glass Expanse (seed 777)
  has its stamp and 3 bodies with the camera on them, and a second View
  finds the same 3 bodies and population (leaving reconciles and unloads;
  nothing doubled or lost).
- **The stray beam the owner saw** (three colonists on a planet the ship had
  left, in observer mode before the redesign): not reproduced. Beam Down
  on the Star Map and `WorldManager.BeamDown` act on `wm.Current()`, the
  planet in orbit, never the selected destination; `Travel` keeps
  `wm.currentID` and the campaign's location together; the landing anchor
  is reset on every jump. `TestBeamDownLandsWhereTheShipIs` pins it
  through a jump away from a staffed colony, a View of a third planet with
  the first selected, and a jump back. The likeliest source is the dropped
  first version's director, which founded a second colony of three on its
  second site (Umbral Scar, seed 777) while a viewer stayed on the first; it
  is gone with the redesign.
- **Soaks** (`make soak`, all three green). The expedition soak (ten
  observer runs, Rival factions 2, 20,000 ticks): every seed lands (tick
  2,999 to 6,749: day 4 to 9; six on the start system), keeps all six
  alive on its viewed planet with 11 or 12 builds and 28 to 43 orders, and
  ends with 378 to 392 fuel; four seeds' mirror ships made a table-driven
  lift-off within the 26 days (the `mir` column: 1/0/0). The observer soak
  (the lock on a normal campaign) and the autonz soak passed unchanged.
- **Not verified**: the console refusal and the lock were checked in Go
  tests only, not in the VM; a mirror or the expedition does not refuel
  while its site is loaded (Tier 1 only); the expedition never scans for
  new systems; a viewed planet's level size shows on the Star Map
  afterwards (its icon), a small reveal.

## Phase C2 as built (2026-11-08)

Developer doc: [factions.md](../developer/factions.md) ("The director
digs", "Faction settlements", "The mirror colony", "Hostility",
"ConvertAction").

- **The director digs** (`internal/director/dig.go`). A `dig` rung (colony
  ladder, after research) works toward the nearest deposit below the centre
  that a short resource needs, within `dig_reach` (80) across and
  `dig_depth` (6) down: Stairs Down orders (the shaft steps one tile toward
  the ore per level, since a tile cannot hold an up and a down stair), then
  a tunnel of Dig orders (Mine where the tunnel crosses a deposit), one
  order at a time, from the deepest reached tile so a begun shaft is carried
  on after a reload. The mine rung mines faces below the surface. The
  shaft's remaining stairs are held back from the rungs below, and from
  every rung once begun; the rung waits while a structure above it is
  going up. The ore is found by a bounded search of the level, not the
  player's scanner data. Two fixes came out of it: a z step without the
  right stair now costs 10⁷ in the path search (a worker idling on a stair
  landing made the invalid drop beside the shaft look as cheap as the real
  route, so the search returned it and the shaft was unreachable), and
  the miner's chatter of a faction settlement stays out of the player's log.
- **Ore on the opening maps** is sparse: one to five levels down, 20 to 250
  tiles from the landing; seeds 2, 6 and 9 have no metal within 80.
- **Seed 2's trapped camp** is not an ownership problem: the airlock is the
  colony's. Its 7×7 start room was laid against a rock outcrop, and the
  airlock opens onto rock. Not a data one-liner (every 7×7 camp script
  would need a cleared approach); in the todo.
- **Faction settlements** (`internal/game/faction_settlement.go`): a
  presence whose doctrine has a settlement block gets a camp (bunker room,
  faction airlock, settlement locker with the stockpile, workbench), a
  settlement `"<faction>:<loc>"`, crew workers with the faction string
  (never colony workers), and a director through `settlementOrders`: the
  player's order paths, now functions of a settlement and level that the
  cursor uses too. Reconcile writes population, stockpile and techs back on
  every save and freeze; the save carries the settlement; a thaw restores
  it without doubling anyone. A faction's research unlocks on its own
  settlement.
- **The mirror colony**: doctrine `mirror` with `settlement_from: colony`,
  `mobility.kind: fleet`, a kit, `diplomacy: neutral`, `seed_from: 2`. Its
  presence rides a `Fleet` of kind `ship`, landed or flying: it stays 40
  days, lifts off for a free site within 40 units at 4 a day with everyone
  and its hold, waits in orbit of a loaded site, lands to be built fresh.
  `Location.FleetIDs` became `Campaign.LandedFleets`. Seeding: from Rival
  factions 2 it takes one of the first slots, the hive draw for that slot
  still made with its site and number kept back, so every later hive lands
  where it did before (the seed-777 scripts still film the same hive), and
  it picks its own site from its own stream. It is openly a colony: Star Map
  dart beside the site or in flight, `RIVAL COLONY · NEUTRAL` /
  `· HOSTILE` and `RIVAL COLONY SHIP INBOUND` tags, `[rival colony]` in
  Destinations, `faction_landed` / `faction_departed` / `faction_hostile`
  in Legends. Beaming a colonist down onto a site it holds turns it hostile.
  On the ground its four colonists are in the player's own sprites.
- **Hostility** (finding 3): stances mark factions for the player's
  colonists (`components.SetFactionHostile`); a hostile faction's workers
  are attack targets; faction workers defend themselves; a scripted unit's
  `hostility` var is seeded from the doctrine's aggression before its first
  turn (passive when neutral). There are no player turrets.
- **`ConvertAction`**: a task (8 ticks) laying a doctrine's conversion floor,
  saved with the queue; a `convert` director rung (unused until autonz);
  `generation.ApplyConversion` applies a presence's `conversion` share as a
  deterministic patch at materialise, `ConversionShare` reads it back.
- **Soak** (`make soak`, thresholds unchanged; the table gains stairs,
  tunnel and mine orders and structures built at 10k and at the end):

| Seed | Bed | Locker | Lab | Research | Alive 20k | Techs | Stairs | Tunnel | Mines | Built 10k / end | Cancelled |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 200 | 250 | 4200 | 4250 | 4 | 1 | 1 | 0 | 0 | 8 / 8 | 1 |
| 2 | 200 | — | — | — | 4 | 0 | 0 | 0 | 0 | 1 / 1 | 0 |
| 3 | 200 | 250 | 1200 | 1200 | 4 | 2 | 2 | 0 | 0 | 9 / 9 | 0 |
| 4 | 250 | 200 | 1200 | 1200 | 4 | 3 | 4 | 14 | 5 | 11 / 12 | 3 |
| 5 | 250 | 150 | 1050 | 1050 | 4 | 1 | 2 | 53 | 1 | 10 / 10 | 3 |
| 6 | 250 | 200 | 1150 | 1150 | 4 | 1 | 0 | 0 | 0 | 8 / 8 | 0 |
| 7 | 250 | 200 | 1450 | 1450 | 4 | 2 | 3 | 35 | 2 | 11 / 12 | 3 |
| 8 | 300 | 200 | 1050 | 1050 | 4 | 3 | 1 | 0 | 9 | 9 / 9 | 5 |
| 9 | 250 | 200 | 1100 | 1100 | 4 | 1 | 2 | 22 | 0 | 10 / 10 | 4 |
| 10 | 250 | 250 | 1000 | 1000 | 4 | 2 | 5 | 2 | 4 | 11 / 12 | 4 |

  Stairs, tunnel and mines are orders given. The shaft reaches ore and is
  mined in seeds 4, 5, 7, 8 and 10 (seed 8's ore is one level down), and
  techs rise past the opening's one in seeds 3, 4, 7, 8 and 10 (C1: one
  everywhere but seed 2). Seeds 2, 6 and 9 have no metal within reach (9
  tunnels for crystal); seed 1's ore is five levels down and 58 across, more
  stairs than the kit leaves after the lab and the first research; seed 3's
  shaft stops two levels down (likely the same, not traced). Still nobody dies, and no workbench is built (every
  camp has one). The remaining stall is the kit: a deeper shaft needs more
  metal than 30 leaves.

- **Not verified**: a ship landing or lifting off while the player watches
  (ships wait in orbit of a loaded site, so it never happens live), a
  hostile rival actually fighting (its workers only defend; no attack orders
  from its director), the mirror's long-run food (twelve rations in the
  kit, no farming in the ladder), and the convert rung on a real doctrine.
- **Not in C2:** greyish and ships with tiers (D), interception, ruins on
  departure beyond the camp left on the level, a hostile director that
  fights, trade, the autonz doctrine (E).

## Phase D as built (2026-11-08)

Developer doc: [factions.md](../developer/factions.md) ("Raids and hunts",
"Ships", "The greyish", "Interception", "Ruins", "Star Map counterplay").

- **C2's gaps first.** A hostile settlement's director raids: a `raid` rung
  (colony ladder, so the mirror; the greyish too) sends armed workers at
  the player's nearest colonist once their count reaches an
  aggression-scaled threshold, leaves someone home, calls the party back
  after `max_ticks` and waits a cooldown; neutral settlements and the
  player's own never raid. Stored weapons are equipped first. Party
  members are set on the target directly: the task queue hands out the
  nearest work, so a queued attack 280 tiles away was never taken. A
  `hunt` rung feeds every settlement once someone is hungry: attack orders
  on food-dropping wildlife and the doctrine's food recipe, the new
  `protein_press` (2 biomass into a protein bar at the workbench; the
  player can craft it too, the one change to the player's content), its
  biomass chopped first. `TestMirrorFedFortyDays`: four mirror crew, no
  rations, forty days, all alive, none starving (11 bars pressed, 17
  hunts).
- **Fixes found on the way.** A path the level's A* drove through a wall
  (its squared-distance heuristic outweighs a wall's 5,000 over a long
  walk) is searched again with a consistent heuristic (`internal/path`); a
  raid party stood at its own camp wall until then. A rival's kill or chop
  now drops to the rival (the killer search took only the player's
  workers). The director's chop search sees static flora.
- **Ships proper.** Tiers (`ships.tiers`: crew cap, cost), a hold the ship
  digs into by its site's tags, a hold target that sends it home to the
  mothership, docking (hold and techs delivered, a tier bought from the
  hoard), a mothership that builds ships. Fleets now step in place and
  ships ordered in a day are built after the pass. A ship bound for a
  loaded site lands live (camp and crew at once, an alert, Legends, seen),
  and one done with a loaded site lifts off live (crew and hold aboard,
  the settlement gone).
- **The greyish.** Seeded from the third rival faction (so Rival factions
  3 is now a hive, the mirror and the greyish; seed 777's Rust Tangle hive,
  which `factions_starmap` and `factions_site` film, is unchanged), hostile
  from the start. Crew blueprints are the scenario greys without their
  scripted AI, in four roles; a grow rung bio-prints them up to the ship
  tier's crew cap; the camp is a landed saucer (`crashed_ship` class
  saucer) with the hold in its cockpit locker and a bio-printer.
  `CrewBlueprints` now splits by largest remainder (three greys are a
  miner, a hauler and a soldier). The mothership parks far from Home.
- **Interception.** A jump's line passing a ship in flight, or arriving
  under a mothership, meets it; the Contact popup stops the clock with
  Evade, Board and (neutral only) Parley, every outcome drawn from the
  fleet's seed and the day (over 40 seeds a warship's evasion: 26 evaded,
  14 caught). Boarding is an encounter location on a new `void` primer
  (`boarding` map, internal) with the ship's hull from crashed-ship
  sections beside a docking deck, its crew in their fighting forms and its
  hold aboard; the ship is held while its deck is loaded and taken when
  its crew is dead.
- **The mothership's fate** (open question answered above): tiers freeze,
  no ship is built, ships bound home scatter, its hoard is loot.
- **Ruins.** Every departure records a ruin on the site; the camp is
  ruined live or at the next build (built first if the ship came and went
  unseen), its storage emptied, `ruin_pass` at damage 2, its footprint a
  stamp, the departure event's origin and a datapad at its centre.
- **Star Map.** Saucer and mothership markers, ship tags by class (landed,
  in orbit, inbound, bound elsewhere), `[grey mothership]` in
  Destinations; scan sweeps reveal ships (they already did, now tested).
- **Frames** (inspected): `factions_greyish` shows the saucer beside Ember
  Pit tagged `GREY SCOUT · HOSTILE` and on the ground the saucer hull with
  grey crew in its cockpit; `factions_intercept` shows the Contact popup
  ("A grey mothership waits in orbit of Verdant Hold", Evade 8 fuel,
  Board), the mothership with its ring over the site tagged `GREY
  MOTHERSHIP IN ORBIT · HOSTILE`, and aboard the warship-class hull with
  its locker and the four grey types turning on the boarder.
- **Soak** (`make soak`, thresholds unchanged, 2026-11-08 after D; the
  hunt rung waits for hunger 30, so the opening is as before; mines now
  count the protein press's biomass chops):

| Seed | Bed | Locker | Lab | Research | Alive 20k | Techs | Stairs | Tunnel | Mines | Built 10k / end | Cancelled |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 200 | 250 | 4200 | 4250 | 4 | 1 | 1 | 0 | 13 | 8 / 8 | 1 |
| 2 | 200 | — | — | — | 4 | 0 | 0 | 0 | 1 | 1 / 1 | 0 |
| 3 | 200 | 250 | 1200 | 1200 | 4 | 2 | 2 | 0 | 13 | 9 / 9 | 1 |
| 4 | 250 | 200 | 1200 | 1200 | 4 | 2 | 7 | 5 | 9 | 10 / 11 | 5 |
| 5 | 250 | 150 | 1050 | 1050 | 4 | 1 | 2 | 34 | 9 | 10 / 10 | 2 |
| 6 | 250 | 200 | 1150 | 1150 | 4 | 1 | 0 | 0 | 12 | 8 / 8 | 0 |
| 7 | 250 | 200 | 1150 | 1150 | 4 | 2 | 2 | 0 | 10 | 9 / 9 | 1 |
| 8 | 300 | 200 | 1050 | 1050 | 4 | 3 | 1 | 2 | 14 | 9 / 9 | 5 |
| 9 | 250 | 200 | 1100 | 1100 | 4 | 1 | 2 | 22 | 13 | 10 / 10 | 2 |
| 10 | 250 | 250 | 1000 | 1000 | 4 | 2 | 5 | 1 | 10 | 11 / 12 | 4 |

  Milestones match C2 to the tick; nobody dies in 20,000 ticks (about 27
  days, before hunger reaches the eating threshold for a crew with no food).
  Seed 7 lost a tech and its tunnel against C2 (2 techs, 35 tunnel then):
  the opening hunting/chopping competes for hands. Seed 2's sealed camp is
  unchanged.
- **Not verified**: a raid by a grey camp on a level (a scout has one
  soldier, under the threshold); a landed ship whose crew all die on a
  loaded level (wiped only once the level unloads); boarding a mirror ship
  (its crew spawn as plain colonists without AI); the boarding fight played
  to the end in the VM; the steady path search's cost on a large level.
- **Not in D:** B+ hidden identity, autonz (E), trade, faction against
  faction, playable greyish (F).

## Phase E as built (2026-11-08)

Developer doc: [factions.md](../developer/factions.md) ("The autonz").

- **The doctrine.** `autonz` in `data/factions.json`: planet-carried (a
  settlement presence on `Location.Factions`), `mobility.kind: signal`,
  `economy: convert`, hostile by default, no morphs (named as themselves:
  only the hive hides), `home_scenarios: [autonz_incursion]`. Seeded from the
  fourth rival faction (`seed_from: 4`, `factions_max` raised to 4): Rival
  factions 1 to 3 are unchanged (no autonz draw is made below 4, so every
  vmtest seed's sites stay where they were; `TestAutonzSeededFromTheFourthFaction`
  checks six seeds at every count), Rival factions 4 seeds one node. The
  plan's "spreads to adjacent systems slowly via signal" became: a hub
  signals on a 60-day cooldown to a site within 20 units (plus range techs),
  nearer likelier, once its population reaches the firing threshold of the
  site's full capacity or its conversion 0.5; the signal founds a dormant
  presence of one that wakes 30 days later. No projectile, no fleet: the
  same Tier 1 spread step as the launcher.
- **Conversion as the economy.** `Conversion` is the share of a 16-tile patch
  laid with blue wire; the daily abstract rate scales with population,
  ore (+50%), salvage and radioactive (+25%) and the rate techs, up to a cap
  of 0.85. It caps the presence: capacity `floor` 0.35 plus the rest by the
  share of the cap converted (`ConvertCap`), so growth follows conversion,
  and a fully converted site stops growing and spends two bodies per signal.
  On a level the director's convert rung lays the wire outward from the hub
  (`ConvertAction`, the faction path ungated) and reconcile (and the daily
  live rule) reads the share back; the materialised patch reflects the share
  when the level is built (C2's `ApplyConversion`).
- **Content.** `autonz_crew.json` (`wire_placer_crew` converter,
  `enforcer_crew` guard, the scenario units without scripted AI;
  `autonz_sleeper_node`, the sleeper hazard's powered-down enforcer), the
  `autonz_fabricator` station with `print_wire_placer` and `print_enforcer`
  (`deploy_on_craft`), stamps `autonz_node` (9×9) and `autonz_hub` (15×15,
  Signal Core), Legends `faction_swarm_landed`, `faction_signalled`,
  `faction_woke`. No extra builder or miner unit: wire placers take every
  job.
- **Materialise and reconcile.** The settlement block's camp is `staged`:
  the stage's stamp is the camp, re-stamped over the node when the hub is
  reached (hold kept), the locker claimed and the fabricator placed; crew by
  the stage's mix (`StageSettlement`). A dormant presence is a single sleeper
  node and nothing else; destroyed before it wakes, the presence is wiped.
  Woken frozen, the next build replaces the node with the camp; woken live,
  the daily rule builds it at once with an alert. `autonz_incursion` is an
  autonz composition: a presence there builds by the chapter's origin, the
  scenario's own swarm the same faction.
- **D's leftovers.** A landed ship whose whole crew dies on its loaded level
  is wiped at once (`faction_wiped`, fleet, settlement and director gone).
  Boarded mirror crew (colonists, whose own AI is the worker's) get the
  humanoid fighting script, hostile at 8, and fight boarders within their
  range and sight. The raid-at-freighter rule is noted in the greyish
  doctrine and left. Not done (over an hour or a design call): greys and
  autonz researching on the level, boarded decks leaving the Star Map.
- **Star Map.** A square chip in the doctrine's colour (filled with stage
  pips when awake, an outline while dormant), `AUTONZ NODE` / `AUTONZ HUB`
  tags, `AUTONZ SIGNAL · WAKES IN N DAYS` for a known dormant presence, and
  `[autonz hub]` / `[autonz signal]` in Destinations. Scan sweeps reveal
  dormant presences in range.
- **Frames** (inspected): `factions_autonz` (seed 65, Rival factions 4,
  `debug days 80`) shows Leaden Ridge with the blue chip and one pip, tagged
  `AUTONZ HUB` above the site's `XENO BROOD` (a hive shares the site), and on
  the ground the round wired hub with its red Signal Core, the locker, the
  fabricator and enforcers, blue wire ringing it (about 29% of the patch).
- **Pace** (Tier 1, seeds 1–6, Rival factions 4, no player; awake + dormant
  autonz : mean conversion): day 40 one node at 12–20%; day 100 1–2 at
  40–80%; day 200 3–5 + 1; day 300 6–9 at 40–70%.
- **Autonz soak** (`make soak`, `TestAutonzSoak`: an autonz node of four with
  20 metal and 4 crystal on each start system, neutral, 20,000 ticks; a seed
  misses below +0.05 conversion or with no order toward ore; more than two
  misses fails):

| Seed | Share 0 / 5k / 10k / 20k | Converts | Mines | Stairs | Tunnel | Prints | Crew start / end (cap) | Stage | Cancelled |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 0.12 / 0.18 / 0.22 / 0.27 | 123 | 26 | 4 | 0 | 0 | 4 / 4 (4) | 1 | 9 |
| 2 | 0.04 / 0.15 / 0.21 / 0.28 | 185 | 0 | 0 | 0 | 6 | 4 / 7 (7) | 1 | 6 |
| 3 | 0.12 / 0.19 / 0.22 / 0.26 | 114 | 31 | 6 | 0 | 0 | 4 / 4 (4) | 1 | 9 |
| 4 | 0.12 / 0.19 / 0.22 / 0.27 | 121 | 23 | 0 | 0 | 0 | 4 / 4 (4) | 1 | 6 |
| 5 | 0.12 / 0.18 / 0.21 / 0.27 | 115 | 7 | 8 | 10 | 0 | 4 / 4 (4) | 1 | 7 |
| 6 | 0.04 / 0.13 / 0.17 / 0.22 | 146 | 8 | 2 | 4 | 3 | 4 / 6 (7) | 1 | 8 |
| 7 | 0.04 / 0.15 / 0.20 / 0.27 | 185 | 0 | 0 | 0 | 6 | 4 / 7 (7) | 1 | 6 |
| 8 | 0.04 / 0.14 / 0.17 / 0.25 | 178 | 0 | 1 | 24 | 3 | 4 / 7 (7) | 1 | 6 |
| 9 | 0.12 / 0.21 / 0.26 / 0.30 | 139 | 2 | 4 | 6 | 0 | 4 / 4 (4) | 1 | 8 |
| 10 | 0.04 / 0.14 / 0.18 / 0.25 | 174 | 0 | 3 | 5 | 2 | 4 / 6 (7) | 1 | 6 |

  Every seed converts (+0.15 to +0.24 in 20,000 ticks, the node stamp
  counting from the start) and reaches the hub; eight work toward ore
  (seeds 2 and 7 find none within the dig reach). Prints follow the crew
  cap: the start systems carry 4 to 7 autonz at a third converted, so the
  poorer ones never print. The observer soak in the same run matches Phase
  D's milestones to the tick (bed, locker, lab, research), all alive.
- **Not verified**: an autonz raid on a level (the rung is there; the soak
  runs neutral), a live signal or a live wake watched in the VM (Go tests
  only), the boarded-mirror fight played in the VM, the long-run autonz
  economy past 20,000 ticks, and `TestMirrorFedFortyDays`, which failed once
  in a full `go test ./...` run (two of four crew dead by day 36) and passed
  every rerun here and six of six on the commit before E: likely a flake
  from the game's global `rand`, not traced here (traced and fixed after
  E: the food haul did not reach the hunt's kills; see "Observer mode v2
  as built").
- **Not in E:** B+ hidden identity, playable races (F), faction against
  faction on the campaign tier, the player's `terraforming`.

## Phase E2 as built (2026-11-08)

The hive as a settlement faction: [hive_plan.md](hive_plan.md), whose "As
built" sections record each step, its deviations and the hive soak's
table; [factions.md](../developer/factions.md), "The hive's creatures" and
"The hive as a settlement". In short: unit types replaced the colour tiers
(Tier 1 keeps a presence's `units` by type, `larvae`, its biomass and its
resin creep; Phase A's pace and history-dump table are unchanged); the hive
is a settlement on its level (heart, pools, egg clusters, evolution chamber
and Resin Spire on resin; drones lay eggs on grey larvae; settled eggs grow
into crew bodies; a resin bloom farm feeds it); its director runs the
plan's sixteen rungs (guards, the walk to the spire, scouts and contact,
excavation, biomass hunts); `make soak` runs the hive soak after the
autonz'.

## Phase E3 as built (2026-11-08)

The autonz as a settlement faction: [autonz_plan.md](autonz_plan.md), whose
"As built" section records each step, its deviations and the soak's table;
[factions.md](../developer/factions.md), "The autonz". In short: the autonz
have unit types (one silhouette each; wire placer, enforcer, relay drone,
seeker, harvester, sentinel, warden) with steel and bronze print tiers
applied at spawn; an assimilation tree researched at their cogitator,
walked in Tier 1 as a second list beside the Phase E spine (so the spine's
days and the pace do not move: the history-dump table is identical); their
structures stand on wire (the Signal Core, the cogitator, a relay pylon a
relay drone deploys into, the foundry and its ferrite, converter nodes,
defence pylons with a sentinel each); the signal's range counts its relay
pylons (16 + 4 a pylon) and a live signal waits for the Signal Core and the
two placers the transmit rung walks into it. `make soak` now runs the
autonz and hive soaks with their seeds as parallel processes
(`make soak-autonz`, `make soak-hive`); vmtest `factions_autonz_hub` and
`factions_autonz_observer` are new.


## Review fixes (2026-11-08)

A code review of the branch against dev (notes: the review session's
`factions-review.md`) found ten ranked issues and a list of confirmed
secondaries. Each is fixed in its own commit with a regression test that
fails on the code before it; [factions.md](../developer/factions.md) is
updated where behaviour changed. The observer-inspect branch (the entity
inspector, datapad reading) was merged in first (d884464).

1. **Tech-gated crafting** (a player regression): `orderCraft` read the
   settlement's techs, which a campaign never fills, so every armour and
   robot recipe was refused. The order and the bench now read the campaign's
   techs for the player (`TestCampaignPlayerCanOrderTechGatedCraft`).
2. **Ruins at the mothership**: a ship setting out from the mothership's
   orbit (docked or just built) is no longer a lift-off (`setOut`), and a
   presence's repeated visits to one site fold into one unstamped ruin
   (`TestGreyishRuinsOnlyWhereItLanded`).
3. **Razed autonz regrowth**: an empty `built` is "nothing standing", kept
   across a save; Tier 1 rebuilds one structure a day
   (`TestAutonzRazedStaysRazed`, `TestTier1Rebuild`).
4. **Conversion ratchet**: a thaw lays only the shortfall of the recorded
   share, counting ground already converted anywhere
   (`TestConversionCountsWhatIsAlreadyLaid`).
5. **Console observer lock**: it sets the loaded campaign's lock, never the
   config flag that seeds a new observer run
   (`TestConsoleObserverLockDoesNotLeakIntoNextRun`).
6. **Hidden identity**: the top stage's `seen_name` (DRAFT "great nest"),
   `faction_launched` without "hive", DRAFT room names, and the HUD's
   faction line and the inspector hidden for a hidden-identity faction
   outside an observer run (`TestHiveNeverNamedInANormalCampaign`). Left as
   it was: pre-existing dev lore that says "hive" (`hive_consumption` in
   events.json, a site summary in location_templates.json), which is not
   the faction's identity.
7. **Relay rung bricking**: a stopped deploy no longer counts against the
   rung; a blocked drone keeps its deploy
   (`TestAutonzRelayRungRecoversFromAStoppedDeploy`). The raid party list is
   bounded by `max_ticks` in every doctrine, so it was left.
8. **Ship research stall**: the walk takes the first unknown entry
   (`TestShipResearchAfterLiveTechs`).
9. **Determinism**: the sim's global rand is seeded from the campaign
   (`seedSimRand`, `godebug randseednop=0`), the director re-queues instead
   of stopping, and soaks swap a worker's `task.Stop` (mlge's five-real-
   second restart gate) for `ReQueue` (`TestHiveSoakDeterministic`). The
   wall-clock gate is an mlge issue: a tick-based cooldown belongs in its
   scheduler.
10. **A stuck landed ship** (confirmed, `TestStuckShipStaysLive`): it is no
    longer taken off the level and rebuilt on itself daily.

Secondaries: a released boarding deck leaves with its ship
(`TestReleasedBoardingDeckGoes`); a presence's unseen events are capped at
64 and an observer run keeps none (`TestPresenceEventsBounded`); stamp
footprints are saved with the level and a first siting keeps clear of the
colony (`TestPresenceSitedAfterThawKeepsClear`); a sized build gives up on a
worker who never leaves its footprint after 300 turns, materials kept
(`TestBuild_SizedGivesUpOnAWorkerWhoNeverMoves`); the soak targets list
seeds portably and the join runs when a seed fails; the greyish' three
Tier 1 techs are research.json entries of their own tree
(`TestDoctrineTechIDsResolve`). Not taken up from the review's lower list:
`fleetView` mutating fuel, a live-risen hive's `LauncherLost`,
`fillFactionStock` with no container, `stampHusk` and Inanimate hive
structures, `LoadFactionData`'s soft failure, and the test-hygiene notes.
