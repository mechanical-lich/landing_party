# Campaign & Overworld

Landing Party wraps the single-level colony sim in a campaign: a space
overworld of locations, an abstract ship hub, fuel-gated travel, and
beam-down/up of colonists. Only one location's level is live at a time; the
rest are paused (serialized to disk or parked in memory).

## Packages

| Concern | Location |
|---------|----------|
| Campaign / overworld model | `internal/campaign/` |
| Shared stockpile accessor | `internal/storage/` |
| Level swap / lifecycle | `internal/game/world_manager.go` |
| Star Map UI | `internal/game/overworld_state.go` |
| Overworld data | `data/overworld.json` |

### `internal/campaign`

- `Campaign` — `Name`, `Seed`, `CurrentLocationID`, `Locations map[string]*Location`,
  `Ship *ShipState`, `Day`. `NewCampaign` builds it from the loaded overworld
  defs; `FuelCost(toID)` returns the rounded star-map distance from the current
  location (0 if it *is* current; falls back to the static `FuelCost` only when
  coordinates are absent).
- `Location` — a **descriptor**, never holds a `*world.Level`. Holds the
  generation recipe (`MapID`, `ScenarioID`, `Seed`), star-map `X`/`Y`,
  discovery/visited flags, the per-location `SaveFile` (set once frozen), and
  cached view state (`CameraX/Y/Z`, `BuildMode`).
- `ShipState` — `Hold []*world.SaveEntity` (a `StorageComponent` container
  owned by the synthetic `ShipSettlementName` = `"ship"`), `Roster
  []*world.SaveEntity` (aboard colonists), `RosterCap`. `LiveHold()` lazily
  rebuilds the hold into mutable entities; `Sync()` flushes them back before a
  save. The hold/roster never enter a per-location level save.
- `LoadOverworldDefs(path)` — reads `data/overworld.json` (mirrors the
  `mapdef`/`scenario` loaders).
- `BeamUp` / `OpenToSky` — roster transfer + the "no beaming through ground"
  rule (clear column above, no ceiling).

### `internal/storage`

Centralizes settlement-stockpile queries so resource counting/checking/
deduction works against any source of `StorageComponent` entities instead of
assuming one `*world.Level`:

- `Provider` interface (`Sources() [][]*ecs.Entity`), with `LevelProvider`,
  `ShipProvider`, and `MultiProvider`.
- `CountResource` / `CountAll` / `Check` / `Deduct`, all taking an `owners
  []string` set so colony-owned and `"ship"`-owned containers both match.

Worker AI resolves its provider through the `ai.StorageProviderFor` hook.
`installCampaignStorageHook` (in `world_manager.go`) points it at
`MultiProvider{LevelProvider, ShipProvider}` so a landing party crafts from
**both** on-planet storage and the ship hold.

## WorldManager lifecycle

`WorldManager` holds the `Campaign` and the loaded location's `*MainState`
(`current`). A location is in one of three states:

- **frozen** — serialized to `loc_<id>.json.gz`, not in memory.
- **parked** — in memory as `wm.current`, but its event listeners are detached
  (`MainState.teardown`) so it doesn't react to input while the Star Map is up.
- **live** — entered; listeners attached (`MainState.reattach`).

Key methods:

- `buildParked(locID)` — loads (`SaveFile` set) or generates the location's
  `MainState` with `SettlementConfig.CampaignMode = true` (suppresses the
  legacy 5-colonist auto-spawn), then `teardown()`s it (parked, not entered).
- `Travel(locID)` — if a different location is loaded, `Freeze` it first;
  set `CurrentLocationID`; `buildParked` the destination. Stays on the Star
  Map. (`overworld_state.go` charges fuel around this call, capturing the cost
  *before* Travel changes "current".)
- `EnterCurrent()` — clears the parked state's stale `done`/`next` (set when it
  opened the Star Map, otherwise it bounces straight back), `reattach()`es it,
  and returns it for the state machine. This is **Resume / Land**.
- `Freeze(s)` — `sweepResourcesToShip` (moves colony-owned `ResourceItem`
  stacks into the ship hold), records view state, and writes the location's
  `saveFile` gzip.
- `BeamDown(rosterIdx)` / `BeamUp(e)` — move one colonist between roster and
  the loaded level. Beam-down uses a cached `landingZone()` (colony settlement
  centre, else a one-time plaza search) and `freeLandingTile()` (spiral search
  for a standable, unoccupied tile) so the party clusters without stacking;
  `landSet` resets on `Travel`.
- `SeedNewCampaign(c, colonists, fuel)` — stocks a new expedition's hold and
  roster via `factory.Create`.

`MainState.openStarMap` parks the live state (`teardown`, `wm.current = s`) and
pushes `OverworldState` — it does **not** serialize, so beaming is lossless and
instant; disk writes happen only on Travel-away or Save.

### Listener hygiene

`newMainStateBase` registers process-wide singleton listeners exactly once
(`sharedListenersOnce`) and per-instance listeners via `registerListeners`.
`teardown` (UnregisterListenerFromAll) detaches; `reattach` re-registers. This
is what makes parking/resuming the same `MainState` safe across many swaps.

## Save format

```
saves/campaign/<sanitized-name>/
    campaign.json.gz      # Campaign incl. ShipState (hold + roster), locations
    loc_<id>.json.gz      # one per visited location: saveFile{Meta, world.SaveData}
```

`SaveCampaign(s)` freezes the live/parked level (if any), `Ship.Sync()`s, and
writes the root. `LoadCampaign(name)` reads only the root; the current location
is loaded lazily on Resume. `ListCampaigns()` enumerates `saves/campaign/*`.

### Entity round-tripping

Roster/hold `SaveEntity`s built in memory by `world.EntityToSaveEntity` hold
live component structs, which plain `RebuildEntity` cannot decode. Use
`world.RebuildLiveEntity` (JSON marshal → unmarshal → rebuild) for anything
reconstructed from the ship — beaming and `LiveHold` both do.

## `data/overworld.json`

```json
{
  "locations": [
    {
      "id": "wreck_site",
      "name": "Crash Site",
      "kind": "planet",
      "map_id": "earth_like",      // reuses data/maps/*.json
      "scenario_id": "survival",   // reuses data/scenarios/*.json
      "x": 0, "y": 0,              // star-map position (drives fuel cost)
      "fuel_cost": 0,              // legacy fallback when x/y absent
      "discovered": true,          // false = hidden until a quest reveals it
      "summary": "...",
      "reveal_quest": "decode_signal" // optional: quest whose completion reveals this
    }
  ]
}
```

`map_id`/`scenario_id` flow through the unchanged `generation.BuildWorld`
pipeline. The first `discovered` location is the campaign start.

## Quests / objectives

The old scenario win/loss system was removed. The generic condition engine
(`TriggerType`, `Rule`, `Evaluator`, `EvalContext`) now lives in
`internal/objective` (no win/lose framing). Quests are the only consumer.

- `internal/campaign/quest.go` — `Quest` (data-driven, its `Objective` is one
  `objective.Rule`), `QuestReward`, `QuestProgress` (serialized status:
  hidden/available/active/completed), `QuestStatus`. `LoadQuestDefs` reads
  `data/quests.json`; `AttachQuestDefs` binds defs each session and reconciles
  saved progress (a persistent per-quest `objective.Evaluator` is kept so
  "kill all" objectives retain their seen-entity history).
- Lifecycle: `AcceptQuest` (available→active), `EvaluateQuests(ctx)` (active
  objectives → completed, unlocks `requires_quest` dependents, `auto_accept`).
- Evaluation is driven from `MainState.evaluateQuests` on the periodic
  `tick%30` refresh (decoupled from the old day-tick), building the shared
  `buildEvalContext` and merging ship-hold resource counts so
  gather/deliver objectives see the campaign-wide stockpile.
- Rewards: `WorldManager.applyQuestReward` adds fuel/resources to the ship
  hold and reveals locations (explicit `reward.reveal_locations` or any
  location whose `reveal_quest` names the quest).
- UI: `QuestLogState` (opened from the Star Map) lists offered/active/
  completed with an Accept action; active quests also surface in the in-game
  Goals tab.

`data/quests.json`:

```json
{
  "quests": [
    {
      "id": "decode_signal",
      "name": "Decode the Distress Signal",
      "description": "Build a research lab ...",
      "requires_quest": "study_metallurgy",
      "auto_accept": true,
      "objective": { "trigger": "structure_built", "structure": "research_lab" },
      "reward": { "fuel": 25, "reveal_locations": ["derelict_station"] }
    }
  ]
}
```

## Not yet implemented (Phase 2)

- Bio-printer (grow new roster colonists from biomass).
- Procedural lore / location variety.
