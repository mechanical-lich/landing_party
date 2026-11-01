# Expanding Enemy Factions — Plan

Status: **planned, not started** (2026-10-31).

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
| **C** | Faction settlements + director + `ConvertAction`; faction workers on the `WorkerSystem`. | Rival-colony feel on the live planet. |
| **D** | Fleets, interception encounters, greyish mothership. | Depends on A–C; most new UI. |

Phase A ships without B being visible, so the todo should list them as
separate items.

## Open questions

- **Campaign clock.** `Campaign.Day` is never incremented today. Decide the
  day length and whether a jump costs days (proposal: `ceil(FuelCost / 3)`).
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
