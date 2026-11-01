# Tutorial Planet — Implementation Plan

Status (2026-10-31): **plan only, nothing built.** Revised around the owner's
intended flow. Line references are to `dev` at `b8e7bbb`, which already includes
background simulation.

## Decision

A new expedition can start with a **guided first landing**. The player starts
the normal way, in orbit of the generated start system, and the first lesson is
to choose the tutorial planet on the Star Map and jump to it. A toggle on the
New Expedition screen turns the whole thing off, and with it off the game
generates and plays exactly as it does today.

The flow has four phases:

1. **Choose.** Open on the Star Map, find the highlighted Training Ground, read
   its fuel cost, Travel.
2. **Land.** The basics on a small, safe, hand-authored planet.
3. **Pack up.** Beam mined metal and crystal up to the ship hold, beam the crew
   up, jump back to the generated start system, which is the one newly unlocked
   destination.
4. **Build.** Beam the crew and metal down there and build a locker. The
   tutorial ends with the "galaxy is yours" panel.

- The tutorial planet is **one extra location added to a normally generated
  campaign**, with its own map and scenario. `CurrentLocationID` stays on the
  generated start. The same seed gives the same starting galaxy with the
  tutorial on or off.
- **Nothing is hidden.** While the tutorial runs, a **Travel lock** disables
  Travel to every destination not yet unlocked, with a one-line reason. Home
  stays visible and contracts stay as they are. It is not a picker, and it lifts
  at the handoff or on Skip.
- Steps are **data** (`data/tutorial.json`), each with a completion rule that
  reuses `objective.Rule`. A small director in `internal/tutorial` advances
  them and has no Ebiten or game dependency.
- The director learns what the player did by **polling a snapshot** of game
  state each frame, not by adding events. No new event types are needed.
- Prompts **never pause** the game.

Rationale: the game's first screen today is a Star Map with the crew on the
ship, no site chosen and no hint of what to do. Starting there makes that screen
the first lesson. Everything else the tutorial teaches already exists and is
reachable through normal UI, so it should add as little machinery as possible
and be easy to delete.

## Where the code is today

| Concern | Behaviour | Code |
|---|---|---|
| New expedition | `startNewSettlement` → `StartNewExpedition(name, seed)`, which calls `GenerateCampaign`, `SeedNewCampaign`, `NewWorldManager`, `SeedShipCrew`, `StockShipLocker`, `EnterStartingOrbit`, then opens `NewDashboardState`. **The player starts on the Star Map with the crew aboard, not on the planet.** | `internal/game/title_state.go` |
| Starting location | `startArchetype` picks a `start_ok` archetype, `newSystem` binds a random map and scenario at the origin, then 5–10 nearby systems (radius 9–27) are charted, all visible. | `GenerateCampaign` in `internal/campaign/generate.go`; `data/generation.json` |
| Fuel | Cost is the rounded star-map distance, paid in `TravelPaying`. Start fuel is 400 (`data/generation.json`; the code default of 40 is overridden). | `Campaign.FuelCost`, `internal/game/incident.go:124` |
| Travel callers | Only two: the Star Map's `travel()` and `TravelToIncident`. | `starmap_panel.go:343`, `incident.go:111` |
| Destination list | Sorted nearest first, and row 0 (the current location) is selected by default. Unvisited sites draw as a muted "?" disc. | `refreshLocations`, `drawSiteIcon` in `starmap_panel.go` |
| Beam up, materials | Click a container on site, then **▲ Beam Up** in its inspector. Goes through `BeamResourceUpFrom` into the hold. No sky rule for cargo. | `storage_inspector_modal.go:573`, `world_manager.go:779` |
| Beam down, materials | Global Inventory opens the hold's inspector. Lands in the **first** colony-owned container that accepts it; it cannot target one. | `addToSite` in `world_manager.go` |
| Beam up, colonists | Star Map list or **B** on a selected colonist. Blocked only by a **solid** tile above; a floor overhead does not block. `docs/player/star_map.md` says "a roof" also blocks, which is not what the code does. | `BeamUp`, `campaign.OpenToSky` |
| Hold stock | 100 each of metal, crystal, biomass, stone and radioactive material, 20 rations, 400 fuel. | `StockShipLocker` |
| Background ticking | The dashboard calls `TickBackground(nil)`, so the ship and any staffed planet keep running on the Star Map, including needs. | `dashboard_state.go`, `world_manager.go:310` |
| Level generation | `newGame` resolves map then scenario, runs `BuildWorld`, picks a plaza with `findStartingPlaza`, creates the settlement there, then runs setup scripts. | `internal/game/main_state.go` |
| Scenario binding | `newGame` only selects `scenario.ForMap(mapID)` entries, and `ForMap` and `SelectByID` only see `enabled` scenarios. | `internal/scenario/loader.go` |
| Hostiles | `GameMaster.Update` returns early when `hostile_max <= 0`. | `internal/game/gm.go` |
| Quests | Each quest is one `objective.Rule`. `EvaluateQuests` runs every 30 ticks and not while paused. | `internal/campaign/quest.go`, `evaluateQuests` in `main_state.go` |
| Rule engine | Triggers cover days, structures, techs, population, kills and resources. `Op: ""` means **eq**, not gte (except `structure_built`, where no op means at least one). `When` conditions can test `game_flag`, but no trigger tests a flag on its own. | `internal/objective` |
| Eval context | `buildEvalContext` counts all live entities and all stored items on the level. **Items in a colonist's bag are not counted.** | `main_state.go` |
| Save | `Campaign` is written whole to `campaign.json.gz` by `SaveCampaign`, so a new `omitempty` field saves and loads with no migration. | `world_manager.go` |
| Settings | Player overrides go to the gitignored `data/config.local.json` through `SaveAudioSettings` / `mergeAudioOverrides`. | `internal/config` |
| Encyclopedia | A dashboard tab listing hovered entity blueprints. No free-text articles. | `encyclopediaPanel` in `internal/game/dashboard_panels.go` |

Also relevant, and easy to get wrong:

- **The state machine has no "current state" accessor** (`mlge/state`), so
  `Game.Update` and `Game.Draw` cannot host the tutorial without an engine
  change. `MainState` and `DashboardState` have to call into it.
- **Each `MainState` owns its own `GUIManager`**, and `DashboardState` has none
  at all. A prompt added to `HUDScreen` would not survive the move to the
  dashboard, or to the ship, which has its own `MainState`.
- **Mouse clicks reach the world before any state updates**:
  `InputManager.HandleInput` uses `SendEvent`, which is synchronous. A prompt
  panel must therefore be excluded by position in `handleMouseClick`, the way
  `smallMap.WithinBounds` and the dashboard button are.
- **Docs and code disagree on the Encyclopedia.** `docs/player/encyclopedia.md`
  says it is disabled until Archive Indexing, but `techEncyclopedia` has no
  callers and the tab is always enabled. This plan relies on the code's
  behaviour; the doc needs correcting either way.
- **`config.local.json` on this machine sets `unlockAllResearch: true`.** The
  research step will complete instantly in dev runs. That is correct behaviour,
  not a bug.

### Events: what exists and what does not

| Player action | Signal today | How the tutorial sees it |
|---|---|---|
| Pan, zoom, change z | Raw `KeyPress` / `MouseWheel` only | Poll `camera` against a baseline |
| Select an entity or colonist | `EntitySelected`, `ColonistSelected` (global bus) | Poll `selectedEntity` |
| Choose an order or build item | `CursorModeChanged`, `BuildOptionChanged` | Poll `CursorMode`, `buildMode` |
| **Drag a dig/mine box** | None (`startMarquee`, `commitMarquee`) | Poll `marqueeActive` and the box size |
| **Place a build order** | None (`addBuildTask`) | Poll the settlement task queue |
| Structure finished | `StructureBuilt` (level bus) | `structure_built` rule |
| Research finished | `ResearchDone` (level bus) | `tech_researched` rule |
| **Item stored** | `ItemStored` (level bus, no listener; not fired by beaming) | `resource_gathered` rule |
| **Craft finished** | None, only a message (`task_handlers.go:438`) | Count bags, storage and ground |
| **Open dashboard, change tab** | None (`openDashboard`) | Which state called the hook; `tabs[active].id` |
| **Beam colonists** | None (`BeamDown`, `BeamUp`) | `PlanetColonists()`, `ShipColonists()` |
| **Beam resources down** | None (`BeamResourceDown`) | Site count against a baseline |
| **Beam resources up** | None (`BeamResourceUpFrom`) | Hold count against a baseline |
| **Select a destination** | None (`locList.SelectedIndex`) | `starMapPanel.selectedLocation()` |
| **Travel** | None (`Travel`) | `Campaign.CurrentLocationID` |
| **Open a storage inspector** | None (direct call, `main_state.go:1711`) | `storageInspector.Visible` |

Everything marked bold has no event, and all of it can be read from state that
is already there. Adding events would mean firing sites in `main_state.go`,
`world_manager.go` and `starmap_panel.go`, purely to serve a feature meant to be
deletable. Polling touches none of them. It also keeps working while the game is
paused, when `s.tick` and the quest evaluation stop.

## Verified by experiment

These ran as headless tests using `go test -overlay`, so **no repo file was
added**. Sources are in the session scratchpad and are not part of this plan. The
first set ran before background simulation merged; the second ran on `b8e7bbb`.

| Question | Result |
|---|---|
| Does a 64×64×10 planet generate? | Yes, in about 4 ms. Tested with `mountain_frequency: 0` and a uniform biome. |
| Is the plaza stable? | **Not on planet maps.** No planet primer tags a start region, so `findStartingPlaza` uses the global `rand` (not the seed) and lands anywhere, including the map edge: (49,3), (17,1) and (7,2) in three runs. Tagging `starting_floor` at map centre gave (32,32) on all five seeds. |
| Can a setup script author the site? | Yes. `spawn_entity_owned("teleporter", ...)` makes a colony-owned container, and `set_tile(..., "ore_deposit")` makes a tile `addMineTask` accepts. |
| Does the bootstrap work end to end? | Yes, on five seeds, through the real worker AI. Two colonists, 20 metal in the pad: locker built at tick 90, ore tile mined out at about 900–1100 ticks, ore stored, 3–7 metal net from two tiles. |
| When do colonists get hungry? | First hungry at tick 14190, about 4 minutes at 60 TPS and speed 1. The colonist then walked to the container and ate a ration. |
| Do idle colonists haul loose items? | **No.** Three rations on the ground were untouched after 6000 ticks with an idle colonist and a free container. Hauling needs an order. |
| Can rations be beamed down? | **Not through the UI.** The storage inspector skips anything without `Material` (`storage_inspector_modal.go:436`). The same filter applies to beaming up. The `addToSite` quantity bug seen earlier is fixed on `dev` (`TestBeamDown_NonStackablesAllArrive`). |

Run on `b8e7bbb`, through the real `GenerateCampaign` and `EnterStartingOrbit`:

| Question | Result |
|---|---|
| What does the generated start system offer? | Seeds 1–24 hit all nine `start_ok` scenarios. **Eight pre-place a colony `storage_locker` and `basic_workbench`. `worms` has no container, and Beam Down there fails with "no storage locker on site".** No scenario has a pad. |
| Is it safe? | **No.** `hostile_max` is 40–200. Each had 0–10 hostiles at generation, the nearest on the landing level 25 tiles away (`crystalline_blight`). |
| Does a round trip off the start system work? | Yes, on all nine. It freezes unstaffed, thaws intact, keeps beamed metal and containers, and charges fuel both ways (nearest real system: 9–13 each way). |
| Does the hunger clock run before landing? | **Yes, more slowly.** Six crew aboard, ticked by `TickBackground(nil)`: first hungry at frame 22190, against 14190 planetside. They ate from the hold: rations went from 20 to 2 by frame 40000, with hunger back at 40. |

Test-harness trap: a second `MainState` driven through `stepWorld` in the same
process does no work, because `fspath.ResetFrameCounter` only runs in
`MainState.Update`. Tests that call `stepWorld` directly in long loops must
reset it each tick.

### Colonist stats

`data/blueprints/entities/colonists.json` gives Int 10, and the techs used here
need 8, so any colonist qualifies for research. `StartColonists` is 6.

### What the hunger numbers mean now

Hunger only reaches 71 by accumulation, so time aboard counts for less than time
on the planet. A minute on the Star Map (3600 frames at 60 TPS) uses about 16% of
the way to hungry. The crew then gets hungry about **3 min 20 s after landing**,
rather than 4 minutes. This is derived from the two probe figures, not run as one
scenario. The food lesson therefore moves up to step 9 (§11).

## Design

### 1. A lock, not a reveal

The first brief proposed hiding the Star Map and revealing it on completion. That
stays rejected, and the new flow makes the case stronger, because the Star Map is
now the first lesson rather than the last. The remaining reasons still apply:

- `Discovered` is not a free flag. Contract destinations are deliberately
  `Discovered = false` until accepted (`makeContract`), and `BindQuests`
  re-reveals on load. Hiding would need a saved list of what to restore.
- It would hide Home, which is visible from day one by design.
- Skipping would have to run that reveal path too, one more way to strand a player.

**The Travel lock** gives the guidance a hidden map would have, and changes only
one button. The previous plan let a player wander off and pause the tutorial; the
lock replaces that.

```go
// internal/campaign/tutorial.go
// TutorialTravelLock returns why the ship may not jump to toID, or "" if it may.
func (c *Campaign) TutorialTravelLock(toID string) string {
	t := c.Tutorial
	if t == nil || t.Done || t.Skipped || toID == c.CurrentLocationID {
		return ""
	}
	for _, id := range t.Unlocked {
		if id == toID {
			return ""
		}
	}
	if len(t.Unlocked) < 2 {
		return "Complete the first landing first"
	}
	return "Finish the first build first"
}
```

It lives on `Campaign` and reads saved state only, so the panel does not depend on
the bridge, a loaded game is locked correctly from the first frame, and it is
testable without a level. New worlds from scans or `MaybeTravelQuests` are locked
without extra code because the check is an allow list.

**`TravelToIncident` deliberately bypasses the lock.** It only goes to a planet the
player has already staffed, and blocking it would leave an incident popup with a
button that cannot work. See §8.

### 2. Map and scenario

**Dedicated pair, and a small primer change.** A setup script alone cannot do
it, because the plaza is chosen and the settlement created **before** scripts
run, so a script cannot move the landing site. `tag_region` is too late.

**`internal/generation/primer_planet.go`** gains a param, documented with the
others above `Prime`:

```go
// center_start bool (default false) — tag map centre as "starting_floor" so
// findStartingPlaza lands there instead of at a random spot.
```

**`data/maps/tutorial.json`**

```json
{
  "id": "tutorial",
  "name": "Training Ground",
  "internal": true,
  "size": { "w": {"min":64,"max":64}, "h": {"min":64,"max":64}, "z": {"min":10,"max":10} },
  "terrain": "planet",
  "terrain_params": { "mountain_frequency": 0, "center_start": true },
  "biome_map": { "type": "uniform", "single": "tutorial_plain" },
  "features": []
}
```

**`data/biomes/tutorial_plain.json`** is a copy of `plains` with no features.
Stock `plains` scatters trees, and a tree on the centre row makes `tryPlaza`
fail and fall back to a random spot.

**`internal/mapdef`** gets `Internal bool` on `MapDef`, and `Random()` skips
internal maps. `mapdef.Random` is only a fallback for a blank map ID, but the
tutorial map should never be rolled.

**`data/scenarios/tutorial.json`**: `enabled: true` (required for selection),
`supported_maps: ["tutorial"]` so no other map can draw it, `hostile_max: 0`,
empty `spawn_rules`, fixed lighting. It is in no `location_templates.json`
pool, so the generator can never bind it elsewhere.

**`data/scripts/scenarios/tutorial_setup.basic`** builds the site relative to
`start_x/y/z`, which are now the map centre:

| Thing | How | Why |
|---|---|---|
| Teleporter pad, colony-owned | `spawn_entity_owned("teleporter", ...)` | **Bootstrap.** Beam Resources Down fails with "no storage locker on site" if there is no container, and materials must be fetched from a container since physical storage. Every other scenario solves this by pre-placing a locker. Pre-placing a pad lets the player still build their first locker. |
| Rock outcrop | `set_tile` rock | Something to drag a Dig box over |
| Ore and crystal tiles in the outcrop | `set_tile` `ore_deposit`, `crystal_vein` | Mining; see the tile count below |
| About 12 rations on the ground | `spawn_entity("ration_pack", ...)` | Food, and the hauling lesson (see below) |
| At least 4 ore and 4 crystal tiles | `set_tile` | **The cargo.** Two ore tiles netted 3–7 metal after a locker in the probe, and the player spends some of it here. |
| Region tags | `tag_region("tut_outcrop", ...)` etc. | Highlight targets. `Regions` is saved with the level. |

**Why rations sit on the ground.** They cannot be beamed down (see "Verified by
experiment"), and idle colonists will not fetch them unprompted. So the
lesson is real: colonists get hungry a little over 3 minutes after landing, food
is on the ground, and it needs an order. Clicking an item queues a `RetrieveAction`
(`handleMouseClick`), and the Store order does the same with a chosen container.

**Fixed seed.** The location gets a constant seed. With a flat uniform biome and
no features, the seed only affects the caverns below the surface. Everything the
tutorial points at comes from the script relative to the centre, so nothing
depends on it. The seed is kept only so the map is the same for everyone.

### 3. Adding the location

New `campaign.AddTutorialLocation(c *Campaign)` in `internal/campaign/tutorial.go`,
called after `GenerateCampaign`:

- Creates location `"tutorial"`, Kind `planet`, with fixed name and summary, and
  sets `Discovered` so it appears on the map.
- **Leaves `CurrentLocationID` alone.** `EnterStartingOrbit` still parks the
  generated start, unchanged. The tutorial level is built by `buildParked` on the
  first jump, which is the existing path for any unvisited system.
- Places it with `placeSeparated`, using its own `rand.Source`, so the campaign's
  RNG and `GenSeq` are not touched.
- Sets `c.Tutorial = &TutorialProgress{...}`, with `NextLocationID` set to the
  generated start and `Unlocked` set to `["tutorial"]`.

**Placement: radius 6–8 from the start, on the side away from Home.**

| Consideration | Effect |
|---|---|
| Fuel | The jump there costs 6–8, and the jump back the same, so **12–16 of 400 in all**, about 4%. Small but real, as asked. There is no refund. |
| Finding it | `min_separation` is 5 and nearby systems are at radius 9–27, so the tutorial planet is **always the nearest**. It is always the second row of the list, directly under the current location. |
| Away from Home | Keeps it clear of the dashed Home line drawn from the current location. |
| On screen | `fitView` centres on the ship and zooms past a pure fit, so on a narrow cluster it can start off the canvas: an 8-unit offset needs a span of about 20 to fit vertically. The list row is always visible and **Locate** frames it, so step 1 highlights the row first (§9). I chose not to change `fitView`. |

`Location` gains `Tutorial bool` (`json:"tutorial,omitempty"`). The one generator
change: `MaybeTravelQuests` excludes it from `existing`, so random contracts are
never posted to the training ground. `GenerateCampaign` itself is unchanged.

**Only the starting galaxy is identical.** The extra location counts in `tooClose`
for systems charted later, and the tutorial's two jumps each roll
`MaybeTravelQuests`, so later growth can differ. The determinism test covers what
`GenerateCampaign` returns, not the rest of the run.

After the tutorial the planet remains a normal, safe location with whatever the
player built, and they can return or leave colonists there. It is deliberately
small and hostile-free, and has only scripted ore, so it is not a useful farm.

**The second destination is the generated start system.** It is close and keeps its
quests, but three things follow from the survey above, and §11 handles each:

- **It may have no container.** On `worms` Beam Down fails. On first arrival, if
  `wm.SiteHoldEntity()` is nil, the bridge spawns one colony-owned `teleporter`
  near the landing zone and sets `PadPlaced`.
- **It usually has a locker already.** On eight of nine scenarios, a bare
  `structure_built storage_locker` rule would pass on arrival, so the last step
  compares against a saved baseline.
- **It is not safe.** It can hold live hostiles. The stay is short (a locker takes
  build time 5), and the prompt says so plainly.

### 4. Saved state

```go
// internal/campaign/tutorial.go
// TutorialProgress is the guided first landing's state. Nil on Campaign means
// no tutorial, which covers every older save and any expedition that turned
// it off.
type TutorialProgress struct {
	StepID         string         `json:"step_id"` // by ID, so editing the step list keeps saves valid
	Done           bool           `json:"done,omitempty"`
	Skipped        bool           `json:"skipped,omitempty"`
	NextLocationID string         `json:"next_location_id"`    // the second destination
	Unlocked       []string       `json:"unlocked,omitempty"`  // the Travel lock's allow list
	Baselines      map[string]int `json:"baselines,omitempty"` // counts at step start
	PadPlaced      bool           `json:"pad_placed,omitempty"`
}
```

`Campaign.Tutorial *TutorialProgress` (`json:"tutorial,omitempty"`). No
`CampaignVersion` bump. An unknown `StepID` after a data edit falls back to the
first step, and the step-skipping in §6 catches up immediately.

The lock and the next destination have to be saved: the lock reads them directly,
and a save made on the second planet must not relock the way back. `Baselines` has
to be saved too, or a reload after building the locker would raise the baseline and
ask for a second one. Only count baselines are saved. Camera baselines are not,
because redoing a pan costs nothing.

### 5. `data/tutorial.json`

```json
{
  "steps": [
    {
      "id": "dig_outcrop",
      "at": "tutorial",
      "context": "site",
      "title": "Give orders by dragging",
      "text": "Open Build > Orders > Dig, then drag a box over the rock outcrop. A plain click marks one tile.",
      "highlight": ["hud:cat_orders", "hud:order_dig", "region:tut_outcrop"],
      "topic": "orders",
      "done": { "trigger": "flag_set", "flag": "ui.marquee_committed" }
    },
    {
      "id": "build_locker",
      "at": "tutorial",
      "context": "site",
      "title": "Build a storage locker",
      "text": "Build > Structures > Storage Locker, then click open ground. A colonist fetches the metal from the pad.",
      "highlight": ["hud:cat_structures", "hud:build_storage_locker"],
      "topic": "building",
      "done": { "trigger": "structure_built", "structure": "storage_locker" }
    }
  ],
  "topics": [
    { "id": "orders", "title": "Orders", "body": "..." }
  ]
}
```

| Field | Meaning |
|---|---|
| `id` | Stable key, saved in `TutorialProgress` |
| `context` | Where the step can be done: `site`, `ship`, `dash:<tab id>`, or `any`. Elsewhere the panel shows a short "go to..." nudge (§8). This is the brief's per-step *trigger*: it decides when the prompt is live. Steps run in order, so there is no separate start condition. |
| `at` | `tutorial`, `next` or empty. The rule is only evaluated while orbiting that location (§6). |
| `target` | `tutorial` or `next`, the destination `ui.dest_selected` and the Star Map highlight refer to. |
| `unlock_next` | True on the step that unlocks `NextLocationID`. Unlocked from that step onward. |
| `title`, `text` | Prompt content |
| `highlight` | Optional, in order. The first target currently visible is drawn (§9). |
| `topic` | Optional Encyclopedia link (§10) |
| `done` | An `objective.Rule`, or `{"trigger":"acknowledged"}` for steps that only inform |

**One change to the rule engine**, in `internal/objective`:

```go
// TriggerFlagSet fires when Rule.Flag is truthy in EvalContext.Flags. Lets a
// rule test a non-simulation fact supplied by the caller.
TriggerFlagSet TriggerType = "flag_set"
```

A new `Flag` field on `Rule` and one `case` in `Evaluate`, reusing `isTruthy`.
Everything UI-shaped becomes a `ui.*` flag. It is generic and does nothing unless
a rule uses it.

`acknowledged` is **not** an `objective` trigger. The director handles it, since
it comes from the panel's "Got it" button.

Rules must state `op` where they use one: `resource_gathered` with no op means
**equal**, so it would miss a player who beamed down more than the threshold. A
data test enforces this (see Tests).

**Why not model steps as quests.** `requires_quest` gives ordering, and
`Evaluator` is shared anyway. But quests appear in the Quests tab and Goals tab
for the rest of the run, stop evaluating while paused, carry rewards and
location binding that mean nothing here, and still would not cover UI actions.
Using `objective.Rule` directly takes the useful part only.

### 6. The director (`internal/tutorial`)

Plain Go, with no `internal/game`, `internal/world` or UI dependency, so it is
testable with literals.

```go
type Snapshot struct {
	Context string                // "site", "ship", "dash:starmap", ...
	At      string                // "tutorial", "next" or "" for anywhere else
	Eval    objective.EvalContext // simulation facts plus ui.* flags
}

type Director struct { /* steps, index, evaluator, progress */ }

func New(def Definition, stepID string) *Director
func (d *Director) StepID() string      // the bridge copies this to c.Tutorial
func (d *Director) Observe(s Snapshot)  // evaluate; advance past every satisfied step
func (d *Director) Acknowledge()        // "Got it"
func (d *Director) Skip()
func (d *Director) Current() *Step      // nil when finished
func (d *Director) InContext(ctx string) bool
func (d *Director) StepChanged() bool   // edge, so the bridge can reset baselines
func (d *Director) NextUnlocked() bool  // at or past the unlock_next step
```

`Observe` loops while the current step's rule holds, so **doing things out of
order just works**: a player who builds a locker early never sees that prompt.
Each step gets a fresh `objective.Evaluator`. An `acknowledged` step is not
auto-passed.

**`Observe` stops at a step whose `at` does not match.** The eval context always
comes from the location in orbit, so without this a locker pre-placed on the
generated start could satisfy a step meant for the Training Ground.

`NextUnlocked` is a position test rather than an edge, so catching up past several
steps in one `Observe` cannot miss the unlock.

`internal/tutorial` imports only `internal/objective` from this project. It takes
and returns a step ID rather than `campaign.TutorialProgress`, because `campaign`
imports `internal/world` and the director should not inherit that. The bridge
copies the ID, `Done` and `Skipped` into `c.Tutorial`.

### 7. The bridge (`internal/game/tutorial_bridge.go`)

All game-side code lives in this one new file, which owns the panel, the flags
and the highlight. Package `game` lets it read unexported `MainState` fields, so
those fields need no new accessors.

```go
var activeTutorial *tutorialBridge // nil when no tutorial is running

func startTutorial(c *campaign.Campaign, wm *WorldManager) // both new and loaded games
func tutorialFrameSite(s *MainState)                       // from MainState.Update
func tutorialFrameDash(d *DashboardState)                  // from DashboardState.Update
func tutorialDraw(screen *ebiten.Image)                    // from both Draw methods
func tutorialBlocksClick(x, y int) bool                    // from handleMouseClick
```

A package variable follows the `workerai.StorageProviderFor` precedent and keeps
`WorldManager` out of it. `startTutorial` always assigns it, to nil when
`c.Tutorial` is nil or done, so a previous game cannot leak into a new one.

Each frame the bridge:

1. Reads the context from the caller: `s == wm.shipLevel` → `ship`, a
   `MainState` → `site`, a dashboard → `dash:` plus `d.tabs[d.active].id`. It sets
   `At` by comparing `CurrentLocationID` to `"tutorial"` and `NextLocationID`.
2. Updates the latches below, against baselines captured when the step started.
3. Every 30 **real frames**, calls `wm.Current().buildEvalContext()`, whichever
   state is live, merges the `ui.*` flags into a **copy** of `Flags` (they must not
   go into `level.Flags`, which is saved), and calls `Observe`. Its own counter
   means it keeps running while paused. UI flags are evaluated every frame so a
   response feels immediate; only the entity scan is throttled, as
   `evaluateQuests` already does.
4. On a step change: updates baselines, plays `audio.PlayGlobal("notify")`, and
   posts the title to the log with `message.PostMessage("Tutorial", ...)`, so a
   record remains after the panel moves on.
5. Adds `NextLocationID` to `Unlocked` once `NextUnlocked()` is true, and places
   the pad when first in orbit of it if the site has no container (§3).
6. While `wm.ActiveIncident()` is set, skips the panel's buttons, because the popup
   owns input.

| Flag | Source |
|---|---|
| `ui.dest_selected` | The Star Map's `selectedLocation()` is this step's `target` |
| `ui.at_tutorial`, `ui.at_next` | `CurrentLocationID` |
| `ui.camera_panned` | `camera.X/Y` differs from baseline |
| `ui.camera_zoomed` | `camera.TileW` differs from baseline |
| `ui.z_changed`, `ui.z_returned` | `camera.Z` went away from the baseline, then back |
| `ui.paused` | `s.clockStopped()` seen true |
| `ui.colonist_selected` | `selectedEntity` has `Worker` |
| `ui.marquee_committed` | `marqueeActive` seen true, then false, with a dig or mine cursor |
| `ui.dig_queued`, `ui.mine_queued` | Task of that action in `MainSettlement.Tasks` |
| `ui.inspector_opened` | `storageInspector.Visible` seen true |
| `ui.at_site` | Context became `site` |
| `ui.cargo_mined` | Site `metal_ore` above baseline and site `crystal` ≥ 3 |
| `ui.cargo_packed` | Hold `metal_ore` and `crystal` both above baseline, read with `storage.CountResource` as `fuelAvailable` does |
| `ui.crew_aboard` | `len(wm.PlanetColonists()) == 0` and `len(wm.ShipColonists()) > 0` |
| `ui.cargo_landed` | Site `metal_ore` above baseline |
| `ui.locker_added` | `StructuresBuilt["storage_locker"]` above baseline |
| `ui.carrying:<blueprint>` | Blueprint in any colonist's `Inventory.Bag`, for craft output, which `buildEvalContext` misses |

The site is always `wm.Current()`; the `tutorialSite` helper in the earlier draft
is gone now that the merge has landed. Note `openDashboard` builds a **new**
`DashboardState` each time, so the bridge must not hold a panel pointer between
frames. It looks the Star Map panel up from `d.tabs`.

### The Travel lock seam (`starmap_panel.go`)

This is the one new seam the lock needs, in an existing file, and it is small:

```go
// in Update, after p.locList.Update() and before p.travelBtn.Update()
p.travelLock = ""
if sel := p.selectedLocation(); sel != nil {
	p.travelLock = p.campaign.TutorialTravelLock(sel.ID)
}
p.travelBtn.SetEnabled(p.travelLock == "")
```

- `travel()` also returns early with `p.status = p.travelLock`. `minui.Button.Update`
  ignores clicks while disabled, so this only guards non-button callers.
- `Draw` adds the reason as one line under the fuel line in the info column, in the
  existing status colour, moving the summary down.

A selected locked destination therefore still shows its fuel cost, summary and path
line, as asked. Only the button changes.

### 8. Off-script, skip and "don't show again"

**Off-script:**

| Situation | Behaviour |
|---|---|
| Does steps early or in a different order | Satisfied steps are passed silently, within the step's location (§6) |
| In the wrong place for this step | Panel collapses to a nudge, such as "Return to Training Ground to continue", with Skip |
| Tries to jump elsewhere | Button disabled with the reason. The destination is still selectable and readable. |
| Beams crew down at the start system before step 2 | Allowed: the lock is on Travel only. They can go To Site, and the panel nudges them to the Star Map. Those colonists keep that planet loaded and simulating, on a site with hostiles, and can raise an incident. |
| Incident on a planet they staffed | `TravelToIncident` goes there, past the lock. The panel nudges them back, and the way back to the tutorial planet is unlocked. |
| Stuck on a first-planet step | **The lock raises the cost of a stall**, since they cannot leave. Skip stays one click away on every step and lifts the lock at once. |
| Leaves colonists behind on the first planet | Step 20 waits for them. They can Skip. |
| Spends the fuel, for example by beaming it down | Same as the normal game: Travel reports "not enough fuel". They can beam it back up. |
| Uses up the materials | The pad is refilled by beaming, and the hold starts with 100 of each. A step never depends on a limited scripted resource alone. |
| Colonists die | The Training Ground has no hostiles. Starvation is possible if they ignore food for a long time. A total wipe follows the normal `checkTotalWipe` path. |
| Second planet's crew is attacked or dies | Normal game rules. There is crew aboard to send, and Skip still works. |
| Enters Rogue mode | Sidebar hides, so sidebar highlights are skipped. Steps still evaluate. |

**Skip** is on the panel, and takes two clicks (the button changes to "Really
skip?" and reverts after a few seconds). It sets `Skipped`, removes the panel and
leaves the world exactly as it is: the lock lifts everywhere, the player is in orbit
of a normal location and plays on. There is no teleport or state repair, because
nothing was hidden.

**Don't show again:** the panel shown on finishing or skipping has a toggle,
"Offer the tutorial on new expeditions". It writes
`tutorialOffDefault` to `config.local.json` through a new
`config.SaveTutorialDefault`, following `mergeAudioOverrides` so other settings
survive. It only sets the **default** of the New Expedition toggle, which the
player can still flip each time. It belongs in config rather than `Campaign`
because it outlives any one campaign.

### 9. The prompt panel

**Non-blocking, and it fits existing widgets.** The panel is a `minui.Panel`
with `Button`s, with text via `drawWrapped` (`encyclopedia_helpers.go`). It is
hosted directly, like `dashboardBtn` and the dashboard's widgets, rather than
through a `minui.GUI`, since it must live across `MainState`s and on a dashboard
with no GUI at all. `minui.Modal` is not suitable: `GUI.Update` gives a visible
modal exclusive input.

```
┌ Tutorial 12/24 ─ Give orders by dragging ────────────── [–] ┐
│ Open Build > Orders > Dig, then drag a box over the rock   │
│ outcrop. A plain click marks one tile.                     │
│                       [Manual]  [Got it]   [Skip tutorial] │
└────────────────────────────────────────────────────────────┘
```

Placement, at the 1152×800 layout:

- **Site and ship:** x 220–728, y about 650–760. That is right of the 216 px
  sidebar, above the resource bar (`sh-35`), and left of the message log
  (`sw-416`). The top strip is taken by `selectionTooltip` and the small map.
  This overlaps the Rogue exit button only in Rogue mode, when the panel
  collapses.
- **Dashboard:** top right of the content area, below the tab bar (y≥84), over
  the star map canvas. Locate centres a planet on the canvas, clear of the panel.
- **While a storage inspector is open:** it is 760 px wide and centred, and steps
  19 and 23 are done inside it, so the panel shrinks to its title row at the top of
  the screen instead of covering the Beam buttons.

`[–]` collapses it to its title row, so nothing is ever stuck under it. There
are no keyboard shortcuts, since most keys are taken (`handleKeyPress`).

**Click-through.** `tutorialBlocksClick` is checked in `handleMouseClick` beside
the `smallMap.WithinBounds` check. On the dashboard, `starMapPanel.inCanvas` gets
the same check so clicking a prompt button does not also select a star. Other
dashboard panels hold no click-to-act canvas behind that spot.

**Draw order.** Drawn last in `MainState.Draw` and `DashboardState.Draw`, so it
sits above modals. If a modal needs that space the player can collapse the panel.

**Highlights.** `tutorialDraw` strokes a pulsing rectangle with `vector.StrokeRect`:

| Target | Resolves through |
|---|---|
| `hud:<element id>` | New `GUIManager.ElementBounds(id)` → `uiGUI.FindElementByID`. IDs already exist: `cat_orders`, `order_dig`, `build_storage_locker`, etc. |
| `dash:<widget>` | A small map in the bridge to known dashboard widgets (`locList`, `locateBtn`, `shipList`, `beamDownBtn`, `travelBtn`, ...) |
| `star:<tutorial or next>` | `starMapPanel.worldToScreen`, a pulsing ring around the planet on the canvas, drawn only when inside `inCanvas` |
| `inspector:<widget>` | The inspector's beam button and material list, for the pack-up and beam-down steps |
| `region:<tag>` | `level.Regions[tag]`, drawn through `camera.WorldToScreenF` when `camera.Contains` it |
| `entity:<blueprint>` | First match on the level, same drawing |

The list is in order and the first **visible** target wins, which handles
sub-menus naturally: `cat_structures` until the sub-menu is open, then
`build_storage_locker`. A target that does not resolve just draws nothing.

### 10. Does it pause? And the Encyclopedia

**No pausing.** Colonists working while the player reads is the game, and most
steps wait on colonists anyway. The Pause step teaches Space early so the player
can take their own time. The panel keeps working while paused, because the bridge
has its own frame counter.

**Encyclopedia as reference.** It is the right home, but it has no articles
today, only hovered blueprints. It gains a **Field Manual** section:

- `encyclopediaPanel.populate` lists `topics` from `data/tutorial.json` ahead of
  the catalogue, using keys like `topic:hauling` in the existing parallel
  `blueprints` slice.
- `Draw` branches on that prefix: title plus `drawWrapped(body)`, no portrait.
- The manual is always available, including with the tutorial off or finished,
  so "Skip" never loses the reference.

The panel's **[Manual]** button opens the step's topic. From a site it calls
`s.openDashboard()`, then `s.next.(*DashboardState)` gets
`selectTabByID("encyclopedia")` and a new `encyclopediaPanel.selectTopic(id)`.
No new navigation is needed: `openDashboard` parks the level and `Esc` returns.

### 11. The step list

Crew get hungry about 3 min 20 s after landing if the Star Map took a minute, so
hauling food now comes right after the first look around. Tune in play.

| # | id | At | Context | Teaches | Done when |
|---|---|---|---|---|---|
| 1 | `choose_planet` | | `dash:starmap` | Find the highlighted planet, read its fuel cost; others are locked | `ui.dest_selected` |
| 2 | `first_jump` | | `dash:starmap` | Travel Here spends fuel | `ui.at_tutorial` |
| 3 | `beam_party` | tutorial | `dash:starmap` | Select crew, Beam Down | `settlement_population colony gte 2` |
| 4 | `to_site` | tutorial | `dash:starmap` | The To Site button | `ui.at_site` |
| 5 | `camera` | tutorial | `site` | WASD, wheel zoom | `ui.camera_panned` |
| 6 | `z_levels` | tutorial | `site` | Q/E change depth | `ui.z_returned` |
| 7 | `pause` | tutorial | `site` | Space, speeds 1/2/3 | `ui.paused` |
| 8 | `select` | tutorial | `site` | Click a colonist, the modal | `ui.colonist_selected` |
| 9 | `haul_food` | tutorial | `site` | Click a ration, or use Store; hauling needs orders; it goes to the pad | `resource_gathered food gte 3` |
| 10 | `beam_metal` | tutorial | `dash:inventory` | O, Global Inventory, Beam Down to the pad | `resource_gathered metal_ore gte 10` |
| 11 | `build_locker` | tutorial | `site` | Build menu; materials are physically fetched | `structure_built storage_locker` |
| 12 | `dig` | tutorial | `site` | Drag a box with Dig | `ui.marquee_committed` |
| 13 | `mine_cargo` | tutorial | `site` | Mine ore and crystal; output is carried to storage | `ui.cargo_mined` |
| 14 | `storage` | tutorial | `site` | Click a locker, tag filters | `ui.inspector_opened` |
| 15 | `needs` | tutorial | `site` | Hunger, sleep; build a bed | `structure_built bed` |
| 16 | `workbench` | tutorial | `site` | Build a workbench | `structure_built basic_workbench` |
| 17 | `craft` | tutorial | `site` | Click bench, queue Sleeping Bag; beam biomass | `ui.carrying:sleeping_bag` |
| 18 | `research` | tutorial | `site` | Build a lab, queue Basic Metallurgy | `tech_researched basic_metallurgy` |
| 19 | `pack_cargo` | tutorial | `site` | Click a container, ▲ Beam Up metal and crystal to the hold | `ui.cargo_packed` |
| 20 | `crew_up` | tutorial | `dash:starmap` | Beam Up or B; solid rock overhead blocks it | `ui.crew_aboard` |
| 21 | `next_jump` | | `dash:starmap` | One new destination unlocks (`unlock_next`); Travel | `ui.at_next` |
| 22 | `land_again` | next | `dash:starmap` | Land on an unprepared world; hostiles can be here | `settlement_population colony gte 2` |
| 23 | `land_cargo` | next | `dash:inventory` | Beam the carried metal down to site storage | `ui.cargo_landed` |
| 24 | `first_build` | next | `site` | Build a locker from that metal | `ui.locker_added` |

Material choices are data and cheap to change. The sleeping bag (3 biomass, no
tech) doubles as the needs follow-up. Basic Metallurgy costs 5 metal, 100 ticks,
and needs Int 8. Step 17 needs a real `resource_gathered biomass` sub-step if
play-testing shows players stall there.

**Beam Down is taught twice on purpose.** Step 10 keeps the first planet's materials
from depending on how much the player has mined. Step 23 is the practice run the
flow asks for.

**Step 24 is the handoff.** On `ui.locker_added` the director finishes, `Done` lifts
the lock everywhere, and the final panel says the galaxy is theirs and offers
"don't show again". It also says **hungry crew eat from the ship's rations when
aboard**, since that site has no food of its own and rations cannot be beamed
through the UI. Random scenarios and quests carry on from there with no switch to
flip, because they were never altered.

### 12. The toggle

`TitleState` gets `tutorialToggle *minui.Toggle`, the widget the settings screen
already uses, with `On = !cfg.TutorialOffDefault`. The New Expedition screen
moves Launch and Cancel down about 34 px to fit it between Seed and Launch. It is
updated and drawn with the other widgets in the `screenNewSettlement` branches.

```go
func StartNewExpedition(name string, seed int64, tutorial bool) (state.StateInterface, error)
```

It calls `campaign.AddTutorialLocation(c)` after `GenerateCampaign` when `tutorial`
is true, stocks the hold with the reduced tutorial metal (see Open questions), and `startTutorial(c, wm)` before returning, in either case.
`EnterStartingOrbit` is unchanged and still parks the generated start.
`confirmLoad` also calls `startTutorial(c, wm)`. The only caller of
`StartNewExpedition` is `startNewSettlement`, and the Enter-key path goes through
the same function, so the toggle applies to both.

This is **not** a map or scenario picker, and adds none. The rest of the
expedition stays random.

## Files

| File | Change |
|---|---|
| `internal/tutorial/definition.go` (new) | Step and topic types, `Load`, path var as in `campaign.LocationTemplatePath` |
| `internal/tutorial/director.go` (new) | `Director` (§6) |
| `internal/game/tutorial_bridge.go` (new) | Panel, flags, highlight, hooks (§7, §9) |
| `internal/campaign/tutorial.go` (new) | `TutorialProgress`, `AddTutorialLocation`, `TutorialTravelLock` |
| `data/tutorial.json` (new) | Steps and manual topics |
| `data/maps/tutorial.json`, `data/biomes/tutorial_plain.json`, `data/scenarios/tutorial.json`, `data/scripts/scenarios/tutorial_setup.basic` (new) | §2 |
| `internal/campaign/campaign.go` | `Tutorial` field |
| `internal/campaign/location.go` | `Tutorial` field |
| `internal/campaign/generate.go` | `MaybeTravelQuests` excludes the tutorial location |
| `internal/objective/types.go`, `evaluator.go` | `TriggerFlagSet`, `Rule.Flag` |
| `internal/generation/primer_planet.go` | `center_start` param |
| `internal/mapdef/types.go`, `loader.go` | `Internal`; `Random()` skips it |
| `internal/config/*.go` | `TutorialOffDefault`, `SaveTutorialDefault` |
| `internal/game/title_state.go` | Toggle, `StartNewExpedition` param, `confirmLoad` call |
| `internal/game/main_state.go` | **Three small hooks** (below) |
| `internal/game/dashboard_state.go` | Two hooks: `Update`, `Draw` |
| `internal/game/starmap_panel.go` | **Travel lock** (§7): `travelLock` field, `Update`, `travel()`, `Draw`; `inCanvas` guard |
| `internal/game/dashboard_panels.go` | Field Manual in `encyclopediaPanel` |
| `internal/gui/gui_manager.go`, `hud_screen.go` | `ElementBounds(id)` |
| `docs/player/README.md`, `docs/player/tutorial.md` (new) | Player docs |
| `docs/player/encyclopedia.md` | Field Manual; correct the research-gating claim |
| `docs/player/star_map.md` | Travel lock; correct the "roof" claim and the Beam Resources button, which is now on Global Inventory |
| `docs/developer/scenarios.md` | `center_start`, internal maps |
| `docs/developer/tutorial.md` (new) | Step schema and how to add one |
| `docs/brainstorm/todo.md` | Note this plan |

### Sequencing against `dev` (background simulation merged)

The branch has merged, so this is no longer a race against it. These are the seams
as they stand at `b8e7bbb`. The enemy-factions branch is still open and touches
none of `main_state.go`, `title_state.go`, `game.go` or `internal/campaign` (as read
from its earlier diff; not rechecked).

**`internal/game/world_manager.go`: no changes.** The bridge only calls existing
methods. `incident.go` is also untouched: `TravelToIncident` bypasses the lock on
purpose.

**`internal/game/main_state.go`: three additions, no edits to existing lines.**

1. `MainState.Update` (`:734`): `tutorialFrameSite(s)` at the top, beside
   `fspath.ResetFrameCounter`.
2. `MainState.Draw` (`:918`): `tutorialDraw(screen)` after `storageInspector.Draw`
   and **before** `s.wm.drawPopup(screen)`, so an incident popup stays on top.
3. `handleMouseClick` (`:1616`): one guard beside `smallMap.WithinBounds`.

**`internal/game/dashboard_state.go`: two additions.** `tutorialFrameDash(d)` at the
top of `Update`, before the `TickBackground` block, because the incident and
inspector branches return early; and `tutorialDraw` before `d.wm.drawPopup` in `Draw`.

**`internal/game/starmap_panel.go`: the new seam** (§7). Nothing else in flight
touches it now that the branch has landed. Two background-simulation behaviours
matter here:

- A Training Ground with crew left on it stays loaded and counts against
  `maxLoadedPlanets`. That is correct, and it never spawns anything, since
  `hostile_max` is 0. Step 20 normally empties it, so it freezes on the second jump.
- `settlement_name` in the script flags comes from `settlementCfg.ColonyName`, which
  `buildParked` sets to `locationColonyName`. So the pad should be colony-owned on
  dev. **The site probe with the pad ran before the merge, so re-run it first**;
  step 10 fails if it is not.

Everything can start now. There is no longer a step that waits on another branch.

## Tests

`internal/tutorial` (pure, no data on disk):

- Advances when the rule holds and not before.
- Passes several already-satisfied steps in one `Observe`.
- An `acknowledged` step waits for `Acknowledge`.
- `Skip` sets `Skipped`, and `Current()` is nil afterwards.
- A saved `StepID` resumes there; an unknown one restarts and catches up.
- `InContext` matches `any`, `site`, `dash:<tab>`.
- A step with `at` does not advance from a snapshot at another location, even with
  its rule satisfied.
- `NextUnlocked` is false before the `unlock_next` step and true at it and after,
  including when several steps pass in one `Observe`.

`internal/tutorial` data test against the real `data/tutorial.json`:

- Step IDs are unique, `topic` values exist.
- Blueprint, tech and structure references exist in `data/`.
- **Every `resource_gathered` rule has an `op`**, since a blank one means equal.

`internal/objective`: `flag_set` true, false and missing.

`internal/campaign`:

- **The same seed gives identical locations and quests with and without
  `AddTutorialLocation`**, apart from the added location and current ID. This is
  the guard for "randomness intact".
- `CurrentLocationID` is the generated start, not the tutorial planet.
- `TutorialTravelLock`: everything but `tutorial` is locked at the start, including
  Home and hidden contracts; `next` opens once unlocked; the current location never
  reports a lock; `Done`, `Skipped` and a nil `Tutorial` all report none.
- `TutorialProgress` round-trips through JSON including `Unlocked` and `Baselines`,
  and the lock answers the same after a reload; an old save has nil.
- `MaybeTravelQuests` never binds to the tutorial location over many rolls.
- `AddTutorialLocation` keeps `MinSeparation`, is `Discovered`, and is the nearest
  discovered location over many seeds, with `FuelCost` of 6–8 from the start.
- No `location_templates.json` archetype lists the tutorial map or scenario.

`internal/mapdef`: `Random()` never returns an internal map.

`internal/game`, headless, following `physical_e2e_test.go` and resetting
`fspath.ResetFrameCounter` each tick:

- The tutorial map lands at centre on several seeds.
- The whole site script runs: pad is colony-owned, deposits accept mine orders,
  rations are on the ground, regions are tagged, nothing spawns over 10000 ticks.
- A colonist builds a locker from the pad, and mined crystal reaches storage.
- A headless walkthrough: drive each step by doing the real thing through
  `MainState` methods and the worker AI, and assert the director reaches the end.
  It starts from `StartNewExpedition`, jumps both ways and checks the hold's fuel.
- Mined metal and crystal go up with `BeamResourceUpFrom`, survive the jump, and
  metal lands with `BeamResourceDown` on the second site.
- Second planet, forced to `worms` and to `earth_like`: with no container the pad
  appears once and survives a save and load; with a pre-placed locker, `first_build`
  does not complete until a second one exists.
- Star Map panel: a locked selection disables `travelBtn` and `travel()` does not
  move the ship or spend fuel; `TravelToIncident` still works.
- Flags: a marquee cycle sets `ui.marquee_committed`; `Flags` on the level is
  left without any `ui.*` key.
- `tutorialBlocksClick` stops a world click under the panel and not beside it.
- With the toggle off: `activeTutorial` is nil, and `c.Tutorial` and the location
  are absent.

Manual: one full playthrough on a hostile start scenario for pacing, since nothing headless can judge whether
the prompts read well or hunger lands at a bad moment.

## Order of work

1. Rule engine and primer param (`flag_set`, `center_start`, `Internal`), with
   tests. Pure additions.
2. Map, biome, scenario, script, and an e2e test of the site. No player-facing
   change yet; proves the planet. **Re-run the pad ownership check first.**
3. `AddTutorialLocation`, `TutorialProgress`, the toggle. The planet appears on the
   Star Map as a normal destination, with no prompts. Determinism test lands here.
4. The Travel lock, with its tests. Nothing can lift it until Skip exists in step 6,
   so land the two together.
5. `internal/tutorial` director and step data, with tests.
6. Bridge: flags, panel, the hooks, Skip, second-planet pad. **This is the first
   complete tutorial.**
7. Highlights, including the Star Map ring.
8. Field Manual and the [Manual] button.
9. "Don't show again", docs, todo.

Each step leaves the game playable, with 4 to 6 as one unit, so stop after any
other step.

## Open questions (defaults chosen)

- **The mined metal must actually matter — decided 2026-10-31.** A tutorial
  expedition starts with a reduced hold: **metal = 10**, exactly what step 10
  beams down, with every other resource, the rations and the fuel unchanged.
  After step 10 the hold has no metal, so the second planet's locker (step 24)
  can only be built from metal the player mined on the first planet and packed
  in step 19. Implementation: `StockShipLocker` takes the per-resource stock
  from a small `holdStock` map instead of the flat `startingResourceQty`, and
  `StartNewExpedition` passes the tutorial map when the toggle is on. Normal
  starts are untouched. Step 19's rule should require at least the locker's
  metal cost in the hold (`ui.cargo_packed` checks hold metal ≥ cost), and step
  23's prompt should say the site has no metal of its own. If play-testing shows
  players reach step 19 with less metal than a locker needs (they spent it on
  the bed, bench and lab), raise the tutorial planet's scripted ore rather than
  the hold.
- **"The first locker" and "the pad" do not match most second planets.** Eight of
  nine start scenarios already have a locker and none has a pad, and Beam Down
  always uses the first accepting container. Default: prompts say "site storage" and
  "build a locker", the pad appears only where nothing else exists, and the rule
  counts a new locker. The alternative is always dropping a pad, but metal would
  usually land in the existing locker anyway unless Beam Down gains a target.
- **A dangerous end.** The tutorial ends on a live hostile site, by design of the
  flow. Default: warn in step 22 and keep the stay short. If it plays badly, a
  gentler option is unlocking the nearest safer system, at the cost of the start
  system's quests being the natural next stop.
- **Ration burn aboard.** Six idle crew took the hold from 20 rations to 2 in about
  11 minutes of frames. That is more than one meal each and I did not find out why.
  A slow tutorial with four crew aboard could reach the handoff short of rations.
  This predates the tutorial, but it now matters more. Check it in the playthrough.
- **Toggle default: on,** so a first-time player gets it without looking. After
  "don't show again" it defaults off.
- **Beaming rations.** Rations cannot be beamed through the UI in either direction.
  The tutorial works around it with ground rations, and this plan does not change
  it. The owner may want them beamable: step 9 can then change, though ground items
  remain the better hauling lesson, and the second planet would have food.
- **Idle colonists not hauling** may or may not be intended. Three loose rations
  sat untouched for 6000 ticks. Step 9 teaches the behaviour as it is. If hauling
  becomes automatic, change that step's text and keep the rule.
- **The open-sky rule is told, not shown.** Nothing on the flat site puts solid rock
  over a colonist unless they dig down, so step 20 explains it and the normal error
  covers the rest.
- **Sleep cannot be demonstrated on demand.** Exhaustion builds from work, so the
  needs step explains it and asks for a bed instead of waiting for it to happen.
- **Reopening a skipped tutorial: no.** The Field Manual stays available, and a
  new expedition offers the tutorial again. Avoids restart logic for a world the
  player has already changed.
- **Tutorial during an existing save: never.** It only starts from New Expedition.
- **Panel position** is authored for the fixed 1152×800 layout, like the rest of
  the HUD. If screen size becomes configurable, it moves with everything else.

## Unverified

- The panel layout has not been drawn. Positions come from reading constants in
  `hud_screen.go`, `small_map_widget.go`, `starmap_panel.go` and
  `storage_inspector_modal.go`. Check that it does not crowd the colonist or crafting
  modals in practice.
- Clicking a loose ration (`RetrieveAction`) ending in storage was not run; the
  same action was seen working for mined ore, which queues the same task.
- The pacing is an estimate, including the 3 min 20 s, which combines two separate
  probes. The planetside hunger figure of 14190 is from before the merge.
- The Training Ground site probes (plaza, pad, locker, mining) ran before the merge
  with a simulated primer param, and have not been repeated on `b8e7bbb`.
- Mined cargo was not run end to end. `BeamResourceUpFrom` was read, not run; the
  round trip and Beam Down were run on generated start systems with hold metal, and
  not with the tutorial location itself, which does not exist yet.
- Whether the tutorial planet starts on the canvas was worked out from `fitView`, not
  measured across seeds.
