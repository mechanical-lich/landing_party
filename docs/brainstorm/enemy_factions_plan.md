# Expanding Enemy Factions — Plan

Status: **planned, not started** (2026-10-31). Code-reviewed the same day;
see "Code review findings" below for what the audit changed. Sequencing:
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
some with ships that can be met in space.

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
    Seed        int64          `json:"seed"`         // deterministic materialization
}
```

- `Location.Factions []FactionPresence` (omitempty). Several factions on one
  planet is allowed and is where fights between them happen (see Tier 2).
- `Campaign.Fleets []Fleet` for spaceborne factions (greyish): `Faction`,
  `X, Y`, `FromID, ToID`, `Progress`, `Strength`, `Cargo`. A fleet is a
  faction presence that is *between* locations.
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
  "mobility":   { "kind": "spore", "range": 2.5, "chance_per_day": 0.02 },
  "economy":    "biomass",
  "growth":     { "per_day": 0.08, "cap_per_capacity": 1.0 },
  "conversion": { "tile": "hive_floor", "per_day": 0.0 },
  "stages": [
    { "pop": 0,  "stamp": null,          "workers": ["troid_hatchling"] },
    { "pop": 8,  "stamp": "troid_nest",  "workers": ["troid_hatchling", "troid_stage2"] },
    { "pop": 24, "stamp": "troid_hive",  "workers": ["troid_stage2"], "guards": ["troid_stage3"] }
  ],
  "tasks": ["forage", "haul", "dig"],
  "aggression": 8
}
```

Axes and the three starting archetypes:

| Axis | Cyborg (autonz) | Insect (troids / xenos) | Greyish |
|---|---|---|---|
| **Mobility** | planet-bound, spreads to adjacent systems slowly via "signal" | planet-bound, spores to nearby systems, no ship | **fleet**: moves on the star map, lands, salvages, leaves |
| **Economy** | **convert** (wire_placer already lays conversion tiles) + mine ore | **biomass**: forage, eggs, grows with food | **stockpile**: mine + haul into their ship, hoard tech |
| **Growth** | slow, capped by conversion % | fast, hard cap from capacity | fixed crew per fleet; fleets multiply at a mothership |
| **Blueprints today** | `autonz.json`: enforcer, wire_placer | `troids.json`, `xenomorphs.json`: egg → hatchling → stage2/3 | `greyish.json`: greenish/greyish/pinkish/whitish |
| **Scenario today** | `autonz_incursion` | `troid_outbreak`, `exterminate_xenos` | `grey_contact` |

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
4. **Spread**: with `mobility.chance_per_day`, pick a Discovered-or-not
   location within `range` (star-map distance, same metric as `FuelCost`)
   and seed a stage-0 presence there. Spore/signal factions seed directly;
   fleet factions spawn a `Fleet` that travels and lands.
5. **Skip** locations whose level is currently loaded (live or parked):
   those are simulated for real and reconciled on freeze.

**Fleets**: advance `Progress` along `From→To` by speed; on arrival become a
presence (greyish: stage 1, then leave after N days with the planet's
stockpile delta as `Cargo`, heading to the mothership location).

## Tier 2 — Level: materialize, simulate, reconcile

### Materialize (on `buildParked` → after `spawnLocationFixtures`)

For each `FactionPresence` on the location, seeded by `presence.Seed`:

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

Combat stays as it is: `rlcombat.IsFriendly` compares
`Description.Faction`, so two factions on one planet already fight each
other and both fight colonists. Doctrine `aggression` just sets the
`hostility` var the humanoid script already reads.

### Reconcile (on `Freeze`)

Alongside `loc.Colonists` and `loc.StorageSummary`: count living entities
per faction settlement → `Population`; `storage.Summarize` over the faction
settlement's owners → `Stockpile`; count converted tiles / total →
`Conversion`; keep `Stage`. If `Population == 0` and no eggs remain, drop
the presence (planet cleared).

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

## Phasing

| Phase | Deliverable | Why first |
|---|---|---|
| **A** | `FactionPresence` + `data/factions.json` + `AdvanceDay` + Tier 1 step + save/load. Star Map badges. No level changes. | Cheapest, fully testable with unit tests (deterministic RNG). |
| **B** | Materialize using **existing** stamps, scenarios and hostiles; reconcile on freeze. No director yet. | Biggest visible payoff: planets change while you're away. |
| **C** | Faction settlements + director + `ConvertAction`; faction workers on the `WorkerSystem`. **Prerequisite: finding 1's colony-worker predicate (Part B0), plus findings 3 and 4.** | Rival-colony feel on the live planet. |
| **D** | Fleets, interception encounters, greyish mothership. | Depends on A–C; most new UI. |

Phase A ships without B being visible, so the todo should list them as
separate items.

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
- **Insects without a ship spreading across space** needs a fiction hook
  (spores riding debris, hitching on the player's hull?). The last one is a
  fun mechanic: carrying eggs in the hold seeds the next planet.
