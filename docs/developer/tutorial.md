# Tutorial (Guided First Landing)

The tutorial is a data-driven list of steps evaluated against polled game
state. Nothing is hidden and no event types were added: a director reads a
snapshot each frame and advances when a step's rule holds. The whole feature is
meant to be easy to delete.

## Pieces

| Piece | Where |
|---|---|
| Step and topic data | `data/tutorial.json` |
| Definition, validation, Director | `internal/tutorial` (imports only `internal/objective`) |
| Saved progress, Travel lock, the extra location | `internal/campaign/tutorial.go` |
| Game bridge: flags, snapshot, progress sync, pad | `internal/game/tutorial_bridge.go` |
| Prompt panel | `internal/game/tutorial_panel.go` |
| Highlights | `internal/game/tutorial_highlight.go` |
| Training Ground | `data/maps/tutorial.json`, `data/biomes/tutorial_plain.json`, `data/scenarios/tutorial.json`, `data/scripts/scenarios/tutorial_setup.basic` |
| Toggle and start | `StartNewExpedition` in `internal/game/title_state.go` |
| Field Manual | `encyclopediaPanel` in `internal/game/dashboard_panels.go` |
| "Don't show again" | `config.SaveTutorialDefault`, `Config.TutorialOffDefault` |

Hooks into existing code: `tutorialFrameSite` at the top of `MainState.Update`,
`tutorialFrameDash` at the top of `DashboardState.Update`, `tutorialDraw` in
both `Draw` methods before the incident popup, `tutorialBlocksClick` in
`handleMouseClick` and `starMapPanel.inCanvas`, the Travel lock in
`starmap_panel.go`, `startTutorial` in `StartNewExpedition` and `confirmLoad`.
`activeTutorial` is nil whenever no tutorial runs, and every hook returns at
once then.

## Step schema

```json
{
  "id": "build_locker",
  "at": "tutorial",
  "context": "site",
  "target": "",
  "unlock_next": false,
  "title": "Build a storage locker",
  "text": "Build > Structures > Storage Locker, then click open ground...",
  "highlight": ["hud:cat_structures", "hud:build_storage_locker"],
  "topic": "building",
  "done": { "trigger": "structure_built", "structure": "storage_locker" }
}
```

| Field | Meaning |
|---|---|
| `id` | Stable key; saved in `TutorialProgress.StepID`, so editing the list keeps saves valid. An unknown ID restarts at the first step and catches up. |
| `at` | `tutorial`, `next` or empty. The rule is only evaluated while orbiting that location; `Observe` stops at a step whose `at` does not match. |
| `context` | Where the prompt is live: `site`, `ship`, `dash:<tab id>` or `any`. Elsewhere the panel shows a "go to..." nudge. Rules still evaluate everywhere, so out-of-order play passes steps silently. |
| `target` | `tutorial` or `next`: what `ui.dest_selected` and the `star:` highlight refer to. |
| `unlock_next` | The step from which the second destination is on the Travel lock's allow list. |
| `highlight` | Targets; every visible one is drawn, except that `region:` and `entity:` targets stand down while a storage inspector is open. |
| `topic` | Field Manual article the Manual button opens. |
| `done` | An `objective.Rule`. Every step completes on what the player does; there is no "Got it". |

`resource_gathered` rules must state an `op`: a blank op means equal, and the
loader rejects it. `internal/tutorial`'s data test also checks that every
structure, tech, blueprint and highlight the file names exists in `data/`.

### Highlight targets

| Target | Resolves through |
|---|---|
| `hud:<element id>` | `GUIManager.ElementBounds`: sidebar items such as `cat_orders`, `order_dig`, `build_storage_locker`; hidden when their menu or the sidebar is |
| `dash:<widget>` | Dashboard widgets: `locList`, `locateBtn`, `travelBtn`, `shipList`, `planList`, `beamDownBtn`, `beamUpBtn`, `scanBtn`, `returnSiteBtn`, `returnShipBtn`, `saveBtn` |
| `star:<tutorial or next>` | A ring around the planet on the Star Map canvas |
| `ship:hold` | The ship's supply pad, while aboard with its inspector closed |
| `inspector:beam` / `inspector:list` | The open storage inspector's Beam button or inventory list |
| `region:<tag>` | `level.Regions[tag]`, a 5x5 tile frame when the camera shows it |
| `entity:<blueprint>` | The first such entity in view |

### ui.* flags

The bridge supplies these in a copy of the eval context's `Flags`; they are
never written to `level.Flags`. Latched flags hold for the rest of the step and
clear when it changes; the rest are computed each frame.

| Flag | Source |
|---|---|
| `ui.dest_selected` | Star Map selection is the step's `target` |
| `ui.at_tutorial`, `ui.at_next`, `ui.at_site` | Location in orbit; context is `site` |
| `ui.camera_panned`, `ui.camera_zoomed` | Camera differs from the step-start baseline (latched) |
| `ui.z_changed`, `ui.z_returned` | Camera Z left the baseline, then came back (latched) |
| `ui.paused` | `clockStopped()` seen true (latched) |
| `ui.colonist_panel_open`, `ui.colonist_panel_closed` | The site's colonist modal is up, or not |
| `ui.marquee_committed` | A Dig or Mine marquee was active, then released (latched) |
| `ui.inspector_opened` | A site storage inspector was open (latched) |
| `ui.dig_queued`, `ui.mine_queued` | Such a task is in the site's queue |
| `ui.cargo_mined` | Site metal (stored plus loose on the ground) above baseline and site crystal at least 3 |
| `ui.cargo_packed` | Hold metal above baseline and at least a locker's cost, hold crystal above baseline |
| `ui.crew_aboard` | Nobody planetside, someone aboard |
| `ui.cargo_landed` | Site metal above baseline |
| `ui.locker_added` | Site `storage_locker` count above baseline |
| `ui.carrying:<blueprint>` | In any colonist's bag (craft output, which the eval context misses) |

Baselines (`TutorialProgress.Baselines`) are captured when a step starts and
saved, so a reload mid-step does not move the goalposts. The entity scan runs
every 30 real frames; flags and the director run every frame, paused or not.

## Saved state

`Campaign.Tutorial *TutorialProgress` (`omitempty`, nil on old saves and with
the toggle off): `StepID`, `Done`, `Skipped`, `NextLocationID`, `Unlocked`,
`Baselines`, `PadPlaced`. `Location.Tutorial` marks the Training Ground so
`MaybeTravelQuests` never posts contracts to it.

## The Training Ground

`data/maps/tutorial.json` is `internal: true` (so `mapdef.Random` never rolls
it), 64x64x10, flat (`mountain_frequency: 0`), with `center_start: true` so the
planet primer tags the centre as `starting_floor` and the plaza lands there on
every seed. Its biome `tutorial_plain` is plains without the trees. The scenario
has `hostile_max: 0`, no spawn rules, fixed lighting, and is in no
`location_templates.json` pool. The setup script places a colony-owned
`teleporter` pad, a rock outcrop with five ore and five crystal tiles on its
west face, twelve rations on the ground, and tags `tut_pad`, `tut_outcrop` and
`tut_rations`.

`campaign.AddTutorialLocation` adds the location 5-7 units from the Wreck Site
(where every expedition starts adrift, at its landfall outside the rim) on the
side away from the galaxy's centre, drawing from its own RNG so the generated
galaxy and `GenSeq` are unchanged. It is nearer than every system (its radius
shrinks when the landfall system is nearer than 8), so the landfall system
(`Campaign.StartID`) becomes the tutorial's second destination
(`NextLocationID`); the ship stays adrift until the player jumps.

## Adding or changing a step

1. Edit `data/tutorial.json`. Keep IDs stable for existing steps.
2. If the rule needs a new fact, add a `ui.*` flag in `tutorial_bridge.go`
   (`evalWithFlags` or `updateLatches`) rather than a new event.
3. Run `go test ./internal/tutorial/ ./internal/game/ -run Tutorial`. The
   headless walkthrough (`tutorial_walkthrough_test.go`) drives every step
   through the real game methods and worker AI; extend it for a new step.
