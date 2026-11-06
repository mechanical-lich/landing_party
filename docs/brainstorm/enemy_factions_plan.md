# Expanding Enemy Factions — Plan

Status: **planned, not started** (2026-10-31). Code-reviewed the same day;
see "Code review findings" below for what the audit changed. Updated
2026-11-05 with the design chat: faction order (xenos → greyish → autonz),
presences that live on a fleet rather than a planet, the **mirror colony**
and **observer mode** (see "The mirror colony" and the revised Phasing).
Updated again the same day: xenos spread by an **egg launcher** structure
(population-triggered, research-ranged, shared by AI and player), eggs in
flight are fleets, and a **doctrine-driven player settlement** is adopted
as a design constraint so greyish/autonz/xenos can become playable races
(see "Playable races").
Sequencing:
background simulation **Part B0** first (per-level scenario, per-location
settlement names, scoped saves, and the colony-worker predicate from
finding 1), then Phase A here can run in parallel with Part B.

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
    Faction     string         `json:"faction"`      // "autonz", "troid", "greyish"
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

- **Planet-carried** (xenos, troids, autonz): `Location.Factions
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
"troid": {
  "name": "Troids",
  "mobility":   { "kind": "launcher", "spread_pop": 16, "base_range": 2.0,
                  "cooldown_days": 6, "payload": "troid_egg" },
  "economy":    "biomass",
  "growth":     { "per_day": 0.08, "cap_per_capacity": 1.0 },
  "conversion": { "tile": "hive_floor", "per_day": 0.0 },
  "stages": [
    { "pop": 0,  "stamp": null,          "workers": ["troid_hatchling"] },
    { "pop": 8,  "stamp": "troid_nest",  "workers": ["troid_hatchling", "troid_stage2"] },
    { "pop": 24, "stamp": "troid_hive",  "workers": ["troid_stage2"], "guards": ["troid_stage3"],
      "launcher": "egg_launcher" }
  ],
  "tasks": ["forage", "haul", "dig", "build"],
  "research": ["launcher_range_1", "launcher_range_2", "hardened_carapace"],
  "aggression": 8
}
```

`mobility.kind` is one of `launcher` (xenos, troids), `fleet` (greyish,
mirror), `signal` (autonz, planet-bound slow seeding). `research` is the
ordered list the abstract tier walks and the director researches at the
stage's lab; entries live in `research.json` under a faction key so the
same file serves the player and the AI.

Axes and the four archetypes, in build order (see Phasing):

| Axis | Insect (troids / xenos) | Mirror colony | Greyish | Cyborg (autonz) |
|---|---|---|---|---|
| **Carrier** | planet | fleet | fleet | planet |
| **Mobility** | consume, then **launch an egg** at a system within launcher range once population hits `spread_pop`; no ship | ship, travels like the player | **fleet**: lands, salvages, leaves | planet-bound, spreads to adjacent systems slowly via "signal" |
| **Economy** | **biomass**: forage, eggs, grows with food | the player's: build, mine, craft, research | **stockpile**: mine + haul into their ship, hoard tech | **convert** (wire_placer already lays conversion tiles) + mine ore |
| **Growth** | fast, hard cap from capacity, then starves | bioprinter + research, same as the player | bioprinter per unit type; **ship tier** gates crew cap and only rises at the mothership | slow, capped by conversion % |
| **Bases** | permanent until cleared; leave a husk when exhausted | semi-permanent, like the player's sites | temporary; ruins remain, stockpile leaves | permanent and growing |
| **Blueprints today** | `troids.json`, `xenomorphs.json`: egg → hatchling → stage2/3 | `colonists.json`, `robots.json` | `greyish.json`: greenish/greyish/pinkish/whitish | `autonz.json`: enforcer, wire_placer |
| **Scenario today** | `troid_outbreak`, `exterminate_xenos` | none (a second life-raft crew) | `grey_contact`, `crashed_ufo` (a fleet that never left) | `autonz_incursion` |

Autonz are deliberately last: they combine the two carriers the first two
factions establish (planet-carried growth, fleet-style building and
research) and add nothing structurally new.

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
- Insects: additionally scatter eggs per `troid_outbreak`'s spawn rules so
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

- From C1 on, the player's settlement carries a `Doctrine` and the build
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
| Xenos | launcher | The `launcher` campaign mode: Star Map action set, no-ship hub, alternative win rule. Bounded, mostly UI. |
| Mirror | ship | Nothing; it is the default. |

Needs, stats, equipment slots and the colonist modal stay shared across
races; a race that lacks a component simply skips that system.

## Phasing

Faction order is **xenos → mirror colony → greyish → autonz**. Xenos need
only A and B (no director, no worker reuse) and prove the campaign tier.
The mirror proves the director on existing content. Greyish are the
flagship once both exist. Autonz combine the two.

| Phase | Deliverable | Why here |
|---|---|---|
| **A** | `FactionPresence` + `data/factions.json` + `AdvanceDay` + Tier 1 step + save/load. **New Expedition "Rival factions" stepper** and initial seeding. Star Map badges. Xenos doctrine only. No level changes. | Cheapest, fully testable with unit tests (deterministic RNG). |
| **B** | Materialize xenos using **existing** stamps, scenarios and hostiles; reconcile on freeze; husk on exhaustion; abstract launcher spread (projectile fleets). No director yet. | Biggest visible payoff: planets change while you're away. |
| **C1** | The director, run against the **player's own settlement**: observer-mode flag, settlement stats in the vmtest sidecar, one observer vmtest script, one Go soak test. **Prerequisite: finding 1's colony-worker predicate (Part B0), finding 4's `deploy_on_craft`.** | Proves the director on content that exists; delivers the integration/balance tool. |
| **C2** | Faction settlements on loaded levels; mirror colony as a fleet-carried faction; `ConvertAction`; finding 3's hostility seeding. | Rival-colony feel on the live planet. |
| **D** | Fleets, greyish doctrine (tech list, bioprinter units, ship tiers, mothership), interception encounters, ruins on departure. | Depends on A–C; most new UI. |
| **E** | Autonz doctrine: convert economy on a planet carrier, signal spread. | Pure data plus `ConvertAction` once C2 exists. |
| **F** | Playable races: title-screen doctrine choice; greyish content; `launcher` campaign mode for xenos. | Everything it needs exists by E; F is content plus the no-ship Star Map. |

Phase A ships without B being visible, and C1 is useful with no enemy
faction at all, so the todo should list them as separate items.

## Open questions

- **Campaign clock.** `Campaign.Day` is a per-save copy of the live
  level's day (finding 2). Decide the day length, make the campaign clock
  the source of truth, and whether a jump costs days (proposal:
  `ceil(FuelCost / 3)`).
- **Carrying capacity formula.** Per archetype tag or per actual generated
  resource tally (`MapW*MapH` is cached on Location, deposits are not)?
  Proposal: tag-based in A, replaced by the real tally once B has a level
  to count from.
- **Faction-vs-faction on the campaign tier.** Two presences on one planet:
  resolve abstractly (higher `aggression * pop` wins per day) or ignore
  until the level is live? Proposal: ignore in A, revisit after B.
- **Does the player get `ConvertAction`?** The design wants it ("just like
  the players would"); gate it behind research so it lands with C.
- **Insect spread: decided.** Population-triggered egg launcher with
  researchable range, shared by AI and player; eggs in flight are
  projectile fleets. The earlier "player as vector" idea (eggs riding the
  hull) is demoted to optional flavour. Still open: does a hive keep
  launching at the same target, stacking presences, or does each shot need
  an unoccupied site?
- **Launcher mode win rule.** Infest N systems, land an egg on Home, or
  survive to a day count? Decide with F.
- **Greyish growth ceiling.** Ship tier rises only at the mothership
  (decided above). Still open: what happens when the player destroys the
  mothership (fleets freeze at current tier? scatter?).
- **Mirror colony diplomacy.** Neutral until the player lands on a planet it
  holds, or hostile from the start? Proposal: neutral in C2, hostility and
  trade as a later phase.
- **Go-test stepping.** Can the logic system stack be stepped outside
  `MainState` for the soak test, or does it need `stepBackground` extracted
  to a function taking a `*world.Level`? Check before C1.
