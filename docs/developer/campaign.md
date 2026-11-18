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
and no static fallback. The whole galaxy is made at the start
(`docs/brainstorm/pregenerated_galaxy_plan.md`); nothing creates a system
during play. Generation (`internal/campaign/galaxy.go`, `generate.go`) is
driven by data templates + a tuning file:

- **Templates:** `data/location_templates.json` (archetypes: kind, candidate
  `map_id` pools, tags, summary, `start_ok`, an optional `spawn_event`) and
  `data/quest_templates.json` (parameterised objectives with amount ranges,
  reward formulas, `requires_tag`, `spawn_systems` (star charts) and
  `spawn_archetype` for "contract" quests that point at a system of that
  archetype). Templates with `trigger: "target_killed"` support either a
  legacy entity list (`target_blueprints` / `target_names`) or a **structure
  script** (`fixture_structure`, `fixture_struct_w`, `fixture_struct_h`) —
  not both. When a structure script is specified the script is the sole
  authority on what spawns and who the target is; see
  [Structure Scripts](structure_scripts.md).
- **Tuning:** `data/generation.json` → `campaign.GenConfig` (`genConfig()`,
  re-read when the file changes; defaults if the file/fields are absent).
  Holds `start_fuel`/`start_colonists`/`roster_cap`, `start_radius` (a
  landfall point's distance outside the rim), `safe_start_radius`,
  `travel_quest_one_in`/`_max`, `contract_one_in`, `min_separation` (for
  locations placed beside the galaxy), the scanner tiers and costs, and the
  `galaxy` block: `sizes` (id, draft name, systems, radius, the systems' own
  `min_separation`), `default_size`, `shapes` (id, draft name, weight),
  `landfall_spare`, `landfall_min_distance`, `rim_bins`, `gate_hop_min/max`.
  `campaign.GenerationConfig()` exposes it.
- **`GenerateGalaxy(seed, size)`** makes everything true of the galaxy
  whoever arrives, from the galaxy stream alone, in order: (1) a shape rolled
  by weight, each a density function over the unit disc
  (`galaxy_shapes.go`: spiral, barred spiral, elliptical, lenticular, ring,
  irregular, each rolling its own arms, bar, axis or clumps), sampled into
  `systems` min-separated points; (2) the **rim** (the outermost system in
  each of `rim_bins` angular slices round the centre of mass, among systems
  with three others within 30) and up to `factions_max + 1 +
  landfall_spare` **landfalls**, `landfall_min_distance` apart, each a point
  `start_radius` outward from its rim system and owning the system nearest
  it; (3) the **controls world**, rolled among the systems farthest from
  every landfall, and the **gate** (`GateKind`, ID `gate`) a short hop
  beside it; (4) archetypes (a start archetype for every system within a
  landfall's safe radius), names (`uniqueName`), seeds and maps; (5) every
  site's history in one loop (`bindHistory`: safe within any landfall's safe
  radius, else the archetype's `spawn_event`, else free); (6) each system's
  1–2 posted quests and its datapad quests. `Campaign.Galaxy` records the
  size, shape, radius, centre, landfalls and the gate and controls IDs.
- **`GenerateCampaignOpts(name, seed, opts)`** makes the galaxy
  (`opts.Size`), then the expedition from streams of its own: a landfall
  rolled for it (`Campaign.Landfall`, its system `Campaign.StartID`), the
  **Wreck Site** (`Kind == AdriftKind`, ID `adrift`) on that landfall's point
  where `CurrentLocationID` points, one opening contract, and the rivals.
  The same seed and size give the same galaxy whatever the options
  (`TestGalaxyIndependentOfExpedition`).
- **Level-less location kinds.** `GateKind` and `AdriftKind` have no map or
  scenario and never get a level; `Location.HasSite()` is false for both and
  `IsAdrift()` true at either. `WorldManager.Travel` to either just clears
  the current level (the Star Map refuses the jump back to the Wreck Site).
  While the current location has no site, `Current()` is nil:
  `EnterStartingOrbit` parks nothing (and still restores away planets),
  `EnterCurrent` returns an error naming the adrift state, Beam Down / To
  Site / Beam Resources refuse with `WorldManager.NoSiteReason()`, the
  dashboard shows "ADRIFT", the ship keeps ticking and the total-wipe check
  counts the crew aboard. The gate is inert until the gate step.
- **Persistence:** generated quests live in `Campaign.QuestDefs` (serialized);
  locations in `Campaign.Locations`. `GenSeq`/`TravelSeq` number systems and
  quests and count jumps. Loading a save **replays** the persisted data and
  `BindQuests()` rebuilds runtime indices/evaluators — content is never
  re-derived. `CampaignVersion` is 5; older saves are refused.
- **Star charts:** a quest reward with `spawn_systems > 0` calls
  `Campaign.RevealCharts(n)`: the n nearest systems the player does not know
  become known. Nothing is created.
- **Travel roll:** every actual jump to a system calls
  `Campaign.MaybeTravelQuests` — a **1-in-6** chance to post **1d4** quests on
  existing unvisited systems, nearer the ship more likely (`questTarget`), a
  third of them contracts (`makeContract`: a system of the template's
  archetype outside every safe start, one whose chapter is the template's
  `spawn_event` first). `TravelSeq` (persisted, incremented every jump)
  feeds the RNG so the roll varies per jump even on a miss while staying
  deterministic.
- **Knowledge.** Unseen, then `Location.Seen` (a point on the map, saved,
  never lost; `Campaign.IsSeen` also counts identified locations, and the
  Wreck Site, Training Ground and boarding decks, which are not
  `GalaxyPoint`s, are always seen), then `Location.Identified` (type, tags,
  tier, summary, holders once revealed, the gate as the gate), then
  `Visited` (landed). Seeing: `SeeAround` (within `SightRange()`:
  `sight_range`, or the galaxy size's own `sight_range`, plus `sight_bonus`
  by scanner level) at the start from the
  landfall point and on every arrival in `WorldManager.Travel`; identifying
  sees; `offerQuest` sees a news quest's system; `ChartRaceReach` sees within
  sight of each world a spreading race holds. `MapLocations` returns only
  seen locations, so the Star Map's list, clicks, selection and jumps cover
  nothing unseen. Saves with no `seen` field anywhere load fully seen
  (`migrateSeen`). A new expedition identifies only its landfall system (an observer
  run everything). Identifying: `Campaign.Scan`, arriving
  (`WorldManager.Travel`, so a blind jump learns on arrival), accepting a
  quest (`revealQuestLocation`), star charts (`RevealCharts`), a spreading
  race's reach (`ChartRaceReach`). Rivals track no knowledge: they act on
  the whole galaxy by their own range rules. `Campaign.KnownName` and
  `UnidentifiedName` are what the Star Map shows. A system's own quests
  are offered (`AvailableQuests`) only once it is identified; news
  (`Quest.Offered`: contracts and a travel roll's quests) is offered at once.
- **Ship scanner:** a **player-triggered**, fuel-costed sweep (the **Scan**
  button on the star map). `Campaign.Scan(level)` identifies every system
  and the gate within the tier's range (`ScanRange`, `IdentifyWithin`), no
  chance roll, and reveals what holds the worlds there; the caller
  (star-map panel) spends the fuel. Two research chains drive it: **power**
  — `ship_scanner_1/2/3` set `ScannerLevel()` (0 = no scanner), indexing
  `scanner_tiers` (range 40/70/110); **cost** — `scan_efficiency_1/2`
  advance `ScanFuelCost()` down `scan_fuel_costs` (40/25/15).
- **The Star Map** (`starmap_panel.go`) lists and draws every seen location:
  unidentified ones as dim points (`unidentifiedRadius`) with no label, listed
  as "Unidentified system — fuel N", Travel reading **Jump Blind**. Under
  them it draws the galaxy haze (`drawBackdrop`, specks from
  `campaign.GalaxyBackdrop(seed, shape, radius)`, which replays the shape's
  density function from the galaxy seed and never reads system positions).
  It frames what is seen plus the sight range round the ship (at least
  `minFrameRadius`), draws the sight range as a faint ring and the scan reach
  while Scan is hovered or its report is open, the gate as a dark ring once
  identified, and names the shape, the system count, the found count and the
  identified count in the canvas header. A fleet is drawn only when one end
  of its flight is seen (`fleetOnMap`), with no line to an unseen
  destination (its tag reads DEEP SPACE); the Factions tab names an unseen
  world "an unseen world" and does not link it.
- **Spreading races** have a milestone, not a win: `race.milestone` (sites,
  title, text) in a doctrine's race block; `Campaign.SpreadMilestone` fires
  once (`MilestoneReached`), recording a `race_milestone` Legends event, and
  the game posts its alert.
- **The gate** (`gate.go`, `internal/game/gate.go`). The controls world
  (`Galaxy.ControlsID`, picked among systems linked to most landfalls by hops
  of at most 40, as far from them as the galaxy allows; a start archetype;
  bound safe) holds a `gate_controls` structure fixture: the `gate_vault`
  script, a guarded BSP dungeon two or three levels down with the dormant
  controls (`gate_controls_dormant`, `Repairable` into `gate_controls`,
  setting the level flag `gate:on` and `gate:on:by`) in its far room. The
  vault is placed on ground the landing walks to (`vaultSite`). Repairing
  the controls is the ordinary repair order (click them with orders on); the
  world manager's `checkGate` turns the flag into `Campaign.SwitchGateOn`:
  `Galaxy.GateOn`, `GateOnBy` (a faction key or `player`), `GateOnDay`, on
  for good, the gate and controls world identified, a `gate_switched_on`
  Legends line, an alert (`Campaign.News`, posted by `postNews`) when a rival
  did it. Identifying the gate identifies the controls world and names it in
  the gate's summary. Beside the lit gate the Star Map offers **Leave the
  Galaxy** (Scan's place) when the hold covers `gate.leave_fuel` (300):
  `LeaveGalaxy` sets `Won` and the run ends with its own end screen.
- **Endings.** No automatic win: the run ends as a win only by leaving
  through the gate, as a loss when the player's people are gone.
- **Lose:** `MainState.checkTotalWipe` (each quest tick) ends the run if no
  colonists remain anywhere — none live on the level, none in the ship roster,
  none recorded on any frozen system (`Location.Colonists`, written on Freeze;
  `WorldManager.TotalColonists`). Both end states route to `CampaignEndState`.

## Crew replenishment

The **Bio-Printer** (`bio_printer` in `data/build.json`, cost 5 metal ore +
5 crystal) is a workbench whose `print_colonist` recipe
(`data/crafting_recipes.json`) turns 25 biomass into a new `colonist` entity.
The printed colonist is deployed like any other Worker-bearing output: it is
dropped onto the level, joins the crafter's settlement, and starts ticking as
an idle worker (see the deploy path in `internal/workerai/states.go`). It is
the only way to grow the crew past the starting roster, so the total-wipe
loss check above is the fail state it guards against.

## Not yet implemented

- Richer lore / quest-chain authoring on top of the generator.
