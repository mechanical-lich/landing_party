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
| Generation templates | `data/location_templates.json`, `data/quest_templates.json` |
| Generation tuning | `data/generation.json` |

### `internal/campaign`

- `Campaign` — `Name`, `Seed`, `CurrentLocationID`, `Locations map[string]*Location`,
  `Ship *ShipState`, `Day`, persisted `QuestDefs`, `GenSeq`, `TravelSeq`,
  `Won`/`Lost`. Built by `GenerateCampaign` (procedural; there is no static
  constructor). `FuelCost(toID)` is the rounded star-map distance from the
  current location (0 if it *is* current).
- `Location` — a **descriptor**, never holds a `*world.Level`. Holds the
  generation recipe (`MapID`, `ScenarioID`, `Seed`), star-map `X`/`Y`,
  discovery/visited flags, `QuestTag`, `Colonists` (frozen-wipe tally), the
  per-location `SaveFile` (set once frozen), and cached view state
  (`CameraX/Y/Z`, `BuildMode`).
- `ShipState` — `Hold []*world.SaveEntity` (a `StorageComponent` container
  owned by the synthetic `ShipSettlementName` = `"ship"`), `Roster
  []*world.SaveEntity` (aboard colonists), `RosterCap`. `LiveHold()` lazily
  rebuilds the hold into mutable entities; `Sync()` flushes them back before a
  save. The hold/roster never enter a per-location level save.
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
- `Freeze(s)` — records view state and writes the location's `saveFile` gzip.
  Ship and site storage are decoupled: site materials stay at the site,
  transferred only via the Star Map's Storage Inspector.
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

## Quests / objectives

The generic condition engine (`TriggerType`, `Rule`, `Evaluator`,
`EvalContext`) lives in `internal/objective` (no win/lose framing). Quests are
the only consumer.

- `internal/campaign/quest.go` — `Quest` (its `Objective` is one
  `objective.Rule`; `Reward` = fuel/resources/`spawn_systems`; optional
  `RequiresQuest`, `Location`, `AutoAccept`, `TitlePrefix`), `QuestProgress`
  (serialized status: hidden/available/active/completed). `BindQuests` rebuilds
  runtime indices each session from the persisted `QuestDefs` and reconciles
  saved progress (a persistent per-quest `objective.Evaluator` is kept so
  "kill all" objectives retain their seen-entity history).
  - **Deferred quest naming**: quests whose target is identified by a structure
    script (see [Structure Scripts](structure_scripts.md)) carry a `TitlePrefix`
    (e.g., `"Bounty"`). The title starts as `"Bounty — <location>"`. When the
    player lands and the structure script calls `mark_quest_target`, it invokes
    `Campaign.BindQuestTargetName(questID, npcName)` which rewrites the title to
    `"Bounty: <npc> — <location>"`. This is intentional — the full name is only
    known once the structure has been generated.
- Lifecycle: `AcceptQuest` (available→active, reveals a bound `Location`),
  `EvaluateQuests(ctx)` (active objectives → completed, unlocks
  `requires_quest` dependents, `auto_accept`).
- Evaluation runs from `MainState.evaluateQuests` on the periodic `tick%30`
  refresh, building the shared `buildEvalContext` and merging ship-hold
  resource counts so gather/deliver objectives see the campaign-wide stockpile.
- Rewards: `WorldManager.applyQuestReward` adds fuel/resources to the ship
  hold and charts new systems for `spawn_systems`.
- UI: `QuestLogState` (opened from the Star Map) lists offered/active/
  completed with an Accept action; active quests also surface in the in-game
  Goals tab.

## Procedural campaign generation

Campaigns are **fully procedural** — there is no static overworld/quest file
and no static fallback. Generation (`internal/campaign/generate.go`) is driven
by data templates + a tuning file:

- **Templates:** `data/location_templates.json` (archetypes: kind, candidate
  `map_id`/`scenario_id` pools, tags, name prefix/suffix pools, summary; plus a
  `home` block) and `data/quest_templates.json` (parameterised objectives with
  amount ranges, reward formulas, `requires_tag`, `spawn_systems`, and
  `spawn_archetype` for self-locating "contract" quests). Templates with
  `trigger: "target_killed"` support either a legacy entity list
  (`target_blueprints` / `target_names`) or a **structure script**
  (`fixture_structure`, `fixture_struct_w`, `fixture_struct_h`) — not both.
  When a structure script is specified the script is the sole authority on what
  spawns and who the target is; see [Structure Scripts](structure_scripts.md).
- **Tuning:** `data/generation.json` → `campaign.GenConfig` (`genConfig()`,
  cached; defaults if the file/fields are absent). Holds `home_radius`,
  `start_fuel`/`start_colonists`/`roster_cap`, nearby count/radius,
  `travel_quest_one_in`/`_max`, `contract_one_in`, `new_system_one_in`,
  `min_separation`, and expansion placement knobs. `campaign.GenerationConfig()`
  exposes it.
- **Placement invariants:** all coordinate rolls (nearby systems and
  `placeTowardHome`) run through `placeSeparated`, which re-rolls until the spot
  is at least `min_separation` from every existing location (falling back to the
  last roll after a bounded number of tries, so generation never stalls). Each
  location's name comes from `uniqueName`, which re-rolls (then appends a numeric
  suffix) so no two systems share a name. Both keep the seed reproducible — the
  retry count is a deterministic function of campaign state.
- **`GenerateCampaign(name, seed)`** builds: a far, **visible `Home`**
  (`Kind == HomeKind`) at `home_radius` so its distance-priced fuel cost is
  huge; a **start** system at the origin; `nearby_min..max` nearby systems;
  1–2 quests per system from tag-compatible templates; and one opening
  contract. Everything derives from `seed`.
- **Persistence:** generated quests live in `Campaign.QuestDefs` (serialized);
  generated locations are in `Campaign.Locations`. `GenSeq`/`TravelSeq` count
  generation/jump events. Loading a save **replays** the persisted data and
  `BindQuests()` rebuilds runtime indices/evaluators — content is never
  re-derived.
- **Progressive expansion:** a quest reward with `spawn_systems > 0` calls
  `Campaign.Expand(n)` on completion — seeded by `Seed + GenSeq`, it charts new
  systems marching the frontier toward Home (`placeTowardHome`), appends their
  quests, bumps `GenSeq`, and re-binds. The overworld grows as you play.
- **Travel roll:** every actual jump (`WorldManager.Travel` to a non-Home
  system) calls `Campaign.MaybeTravelQuests` — a **1-in-6** chance to generate
  **1d4** quests, each bound to a random discovered system or a freshly charted
  one. `TravelSeq` (persisted, incremented every jump) feeds the RNG so the
  roll varies per jump even on a miss while staying deterministic. This keeps
  the run from stalling as long as the player keeps moving, independent of
  `spawn_systems` quests.
- **Win:** `WorldManager.Travel` to the Home location sets `Campaign.Won`
  (no level is generated/landed); `OverworldState` shows the victory screen.
- **Lose:** `MainState.checkTotalWipe` (each quest tick) ends the run if no
  colonists remain anywhere — none live on the level, none in the ship roster,
  none recorded on any frozen system (`Location.Colonists`, written on Freeze;
  `Campaign.StoredColonists`). Both end states route to `CampaignEndState`.

## Not yet implemented

- Bio-printer (grow new roster colonists from biomass) — until then the
  total-wipe loss is the only fail state and there's no way to replenish crew
  beyond the starting roster.
- Richer lore / quest-chain authoring on top of the generator.
