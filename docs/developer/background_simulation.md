# Background Planet Simulation

Goal: planets other than the one you're currently on keep simulating in the
background as long as they're staffed, and the player is notified (and can
intervene) when a colonist there comes under attack.

Status (revised 2026-10-31):

- **Phase 1 complete** (`43a9f67` — per-level event bus).
- **Phase 2 Part A complete** (`2231c47`, 2026-06-30 — `stepBackground` and
  the logic-only `bgSystemManager`, built for The Ship and ticking it today).
- **Phase 2 Parts B0–D, Phase 3, Phase 4: designed, not built.** Part B0 is
  new in this revision: an audit against the code found two process-global
  singletons (the active scenario and the name-keyed settlement map) that
  the original Part B did not account for and that would break as soon as a
  second planet is loaded. See Part B0.

The per-level event bus is the reusable foundation: each planet already
fires/consumes its own sim events in isolation, which is what lets multiple
planets tick without cross-contamination.

## Decisions (settled)

| # | Decision | Choice |
|---|----------|--------|
| 1 | Pacing | **Throttled** — away planets run the full system stack minus rendering, at reduced cadence (frame budget), not 1:1 with the live planet. |
| 2 | Star-Map time | Away planets **keep ticking** while on the Star Map. Add a **pause button** to the Star Map to stop them. |
| 3 | What keeps a planet loaded | Presence of the **`Worker` component** (colonists *and* robots). Future: a dedicated "keep-loaded" component (e.g. for hostiles). |
| 4 | Empty planets | Zero workers ⇒ **frozen**, not ticked. |
| 5 | Skip rendering systems | Away planets **skip `LightingSystem` and `FOVSystem`** (player-view only). |
| 6 | Notification trigger | **First damage** to a colonist on an away planet. |
| 7 | Notification batching | **Per incident, with a cooldown** (no per-hit spam). |
| 8 | Popup pause | Popup pauses the **whole world clock** while up. |
| 9 | Ignore consequences | If ignored, the planet ticks normally; if the worker dies, **they die**. |
| 10 | Persistence | Staffed planets stay **parked in memory** and serialize on save (per-location save format already exists). |
| 11 | Event system | **Per-level simulation event bus**; UI/input events stay on the global bus. |

## Why cooperative, not threaded

The simulation leans on process-global singletons every system touches — the
event bus (`event.GetQueuedInstance()`), the message log (`message.PostMessage`),
the effect manager, global `rand`, `settlement.Settlements`. The ECS systems are
not re-entrant. True multithreaded simulation would require making all of that
per-level and re-entrant — a large, race-prone refactor — for a payoff (CPU
parallelism) we don't need, since only one planet is on-screen.

Instead we tick away-planets **cooperatively on the main goroutine**, throttled.
Systems are already level-scoped (`UpdateSystems(s.level)`), so each parked
`MainState` can be stepped directly. This is deterministic (good for saves) and
avoids every thread-safety landmine. The real cost lever is **cadence**, not
running fewer systems — most systems feed AI/senses (Vision, Hearing, Scent,
Needs, Combat) and must run; only the two rendering systems are skippable.

## Phase 1: per-level event bus (the foundation)

`QueuedEventManager` is a plain struct (`&QueuedEventManager{}`), so each level
can own one. Split events by concern:

- **Simulation events → per-level** `level.Events`. Each planet fires and
  consumes its own; no cross-talk between planets.
- **UI / input events → global bus** (unchanged). They only ever target the one
  live planet.

This also **deletes the parking/`teardown` dance for sim events**: each level's
sim listeners live on its own bus permanently; only the live planet's UI/input
listeners attach/detach on focus. The `sharedListenersOnce` guard goes away.

### Audit results

Simulation events (move to `level.Events`) — small and well-bounded:

| Event | Fired at | Consumer(s) | Kind |
|-------|----------|-------------|------|
| `ResearchDone` | task_handlers.go:429 | MainState → `unlockTech` (**state**) + ResearchListener (log) | state + presentation |
| `EntityKilled` | task_handlers.go:690,708 | KillListener (log) | presentation |
| `StructureBuilt` | task_handlers.go:818 | StructureListener (log) | presentation |
| `EntityDied` | FactionAISystem.go:42 | *(none)* | safe to move |
| `ItemStored` | states.go:206,251 | *(none)* | safe to move |
| `TaskCompleted` | *(never fired)* | TaskListener (log) | **dead** — listener+type unused |

Everything else stays global: all `input.*` events, all GUI button/cursor/modal
events, `EntitySelected`/`StationClicked`/`ColonistSelected`, and `MessageEvent`
(the player log itself).

### Key insight: presentation vs state consumers

The four "sim" listeners (Kill/Task/Structure/Research) all just call
`message.AddMessage(...)` — they are **presentation**, not core simulation.
Only the `ResearchDone` → `unlockTech` path mutates real state.

So sim-event consumers split two ways, which directly informs the notification
design:

- **State mutation** (e.g. `ResearchDone` → unlock tech): must run for the
  *owning* planet whether live or background. Register per-level.
- **Presentation** (`message.AddMessage` log lines): only the **live** planet
  should write to the on-screen log. Background planets route these to the
  **notification system** instead of spamming the log — this is the seam for the
  whole feature.

### Migration steps

1. Add `Events *event.QueuedEventManager` to `world.Level`; init in `NewLevel`.
2. Repoint the ~8 sim firing sites from `event.GetQueuedInstance()` to
   `level.Events` (all have `level` in scope).
3. Register the sim/state listeners per-level on `s.level.Events`; leave
   UI/input on the global bus. Drop `sharedListenersOnce`.
4. Drain `s.level.Events.HandleQueue()` inside `stepWorld` (per planet); the
   global `HandleQueue()` in `game.go` keeps draining UI/input.
5. Delete the dead `TaskCompleted` listener/type (or leave the type, drop the
   listener).

## Phase 2 — background scheduler

Order: **A (done) → B0 → B → C (+ the damage event) → D.**

### Part A — the background-tick path (`stepBackground`) — BUILT

Implemented in `MainState.stepBackground`
([rogue.go](../../internal/game/rogue.go)) with the `addLogic` / `addRender`
dual-registration in `newMainStateBase`
([main_state.go](../../internal/game/main_state.go)). `WorldManager.TickBackground`
calls it for The Ship every frame from both the live level and the dashboard,
honouring `wm.paused`. **Gap:** there is no headless test for it, although it
is testable headless; add one before Part B so the WorldManager refactor has a
regression guard. The original design notes follow for reference.

A trimmed copy of `MainState.stepWorld` ([rogue.go](../../internal/game/rogue.go))
that runs the logic/AI stack but skips the player-view systems. From auditing
`stepWorld` and the system registrations in `newMainStateBase`
([main_state.go](../../internal/game/main_state.go) ~lines 233–317):

**Keep (logic/AI/senses — needed for an away planet to live and fight):**
- `gm.Update()` — hostile-wave spawns (`GameMaster`, [gm.go](../../internal/game/gm.go)).
  **This is what makes attacks happen while you're away.**
- Initiative, FactionAI, ScriptedAI, **VisionSystem** (AI reads `vc.Visible`),
  HearingSystem, ScentSystem, SmellSystem, NeedsSystem, WorkerSystem,
  RadiationSystem, DoorSystem, FactionDoorSystem, ScriptSystem,
  StatusConditionSystem, combat.
- `cleanUpSystem.Update(level)` and `level.Events.HandleQueue()` (per-level bus).

**Skip (live-view only):**
- `LightingSystem`, `FOVSystem` — rendering/fog; not read by AI (FOVSystem
  produces `level.Visible`/`Seen`, which no AI system consumes; VisionSystem does
  its own LOS). Skipping FOV means the explored map doesn't update while away —
  cosmetic, re-runs on return.
- `EmoteSystem` (speech bubbles), `effect.GetEffectManager().Update()` (visual).
- `collectDatapads()` (player-discovery flavor) — open decision, lean skip.
- `forceQuestEval` — quests evaluate on the live planet / periodic tick.

**Mechanism:** `ecs.SystemManager` has no skip/subset API. Build a **second
`bgSystemManager`** containing only the logic systems, populated via a dual-add
helper so there's a single source of truth:

```
addLogic := func(sys) { s.systemManager.AddSystem(sys); s.bgSystemManager.AddSystem(sys) }
addRender := func(sys) { s.systemManager.AddSystem(sys) }   // live only
```

Part A is self-contained and unit-testable headless (assert a level advances —
needs drain, AI acts, hostiles spawn — without any rendering system running).

### Part B0 — de-globalise what a second loaded planet would share

Found by auditing Part B against the code. Both are small, both are
prerequisites, and neither is mentioned in the original plan.

**1. The active scenario is a process global.** `scenario.Active()` is the
*orbited* location's scenario, and `GameMaster.Update`
([gm.go](../../internal/game/gm.go)) reads it for `HostileMax` and spawn rules.
A background planet would therefore spawn the orbited planet's hostiles at the
orbited planet's cap. The Ship dodges this with `suppressSpawns`; planets
cannot. [the_ship.md](the_ship.md) chose to keep the scenario global so a
location's scenario can affect the ship — that still works if the *ship* reads
the orbited scenario explicitly, but each planet must read its own.

Fix: give each `MainState` (or `world.Level`) its own `*scenario.Scenario`,
set at generation/load, and make `GameMaster` and the other two
`scenario.Active()` readers take it from there. `scenario.Active()` remains
the "orbited" scenario for the Star Map and the ship.

**2. Settlements are a global map keyed by name, and every planet's colony
has the same name.** `settlement.Settlements[name]` is read at eight sites
(worker task pick, task queues, storage ownership); `campaignColonyName`
returns `"<campaign> Colony"` for every location. Two loaded planets would
share one task queue and one storage owner. The ship already hit exactly
this and was fixed with a distinct `"<colony> Ship"` name.

Fix (mirror the ship): a per-location settlement name, e.g.
`"<colony> @ <location name>"`, assigned at generation and recorded on the
`Location`. Beam-down/up already re-tags colonists between ship and colony
(`tagColonistSettlement`); extend that to the destination planet's name.
**Save/load is not scoped either:** `world.SaveLevel` embeds the *entire*
global map, and `LoadSaveData` ([save.go](../../internal/world/save.go)) used to
**replace** it. This was assumed survivable because the ship "re-registers
itself", but tracing it showed a live bug: `ensureShipSettlement` only runs in
`buildShip`, *before* any location thaw. Thawing a location afterwards swapped
the ship's map entry for a decoded copy from the location's snapshot, while the
ship's `MainSettlement` kept pointing at the old object, so player orders went
to a queue workers never read. It hit every campaign load (`buildShip` then
`EnterStartingOrbit`) and every return to a visited planet.

Fixed, with a regression guard in
[ship_settlement_test.go](../../internal/game/ship_settlement_test.go):
`LoadSaveData` now **merges** into the global map, and `buildParked` calls
`WorldManager.pinShipSettlement` after a thaw to restore the live ship entry.
The pin is needed because saves are still unscoped: a merge alone still
overwrites the ship with the snapshot's stale copy. Multiple loaded planets
have the same problem, so Part B0 must still make a location save carry only
its own settlement(s) (the "scoped save/load" the original campaign plan called
for in its Step 5 but which was never built), and can then drop the pin.

Task queues now persist with the settlement
([persist.go](../../internal/settlement/persist.go)), because
`task.TaskScheduler` hides its tasks from encoding/json. Tasks restore with
their typed `Data` and in-progress ones return to the queue, while
`LoadSaveData` clears each worker's `CurrentTask` so workers pick them back up.
Tasks bound to a live entity (attack, pickup, retrieve, relocate, entity mining)
are **not** saved, since entities have no stable ID to re-link by, and tasks
assigned directly to a worker without being queued (equip, unequip, needs-driven
rest) end on load. Stale snapshots now matter more: every save carries every
settlement's queue, which is another reason to scope saves.

Test: load two levels headless with workers on both, queue a build task on
each, step both, and assert neither worker picks up the other planet's task
and neither level spawns from the other's scenario. Then freeze and thaw one
of them and assert the other's settlement survives in the global map.

### Part B — WorldManager: single `current` → multi-loaded (the big one)

Today `WorldManager` holds **one** `current *MainState`, and `Travel` **freezes
the departed planet to disk** (`Freeze` → `loc_<id>.json.gz`, `wm.current = nil`).
Background sim needs staffed planets to stay in memory:

- Replace `current` with `loaded map[string]*MainState` + `currentID string`,
  and a `Current()` accessor. **~18 `wm.current` references** in
  [world_manager.go](../../internal/game/world_manager.go) migrate to `Current()`.
- `Travel(dest)`: the departed planet **stays in `loaded` if staffed** (has any
  `Worker`-component entity — colonists *or* robots, per decision #3); otherwise
  `Freeze` it to disk and drop it from `loaded` as today.
- **Save** must serialize *every* loaded planet, not just current
  (`SaveCampaign` path).
- Beam-up/down, `addToSite`, storage queries, etc. repoint to `Current()`.

### Part C — scheduler

In the game `Update` loop, after the live `stepWorld`: iterate `loaded` planets
≠ current that have workers and call `stepBackground()`. Honor `wm.paused`.

**Throttle: an owed-ticks budget, not a fixed cadence.** The original "~1/4
the live rate" is a permanent time dilation: away planets always run slow,
even when the frame has headroom. Instead:

- Every live tick, each staffed away planet accrues **one owed tick**.
- Each frame the scheduler has a **budget** of background ticks (start with
  a wall-clock budget, e.g. 2 ms, or a count, e.g. 2 ticks/frame; tunable in
  `data/config.json`). It round-robins across away planets paying down owed
  ticks until the budget is spent.
- Owed ticks are **capped** (e.g. 60). A planet that falls further behind
  simply loses the excess — it never fast-forwards in a burst on return.

Result: with one or two cheap away planets they run at real time; with many
or expensive ones they degrade smoothly toward the old throttled behaviour.
The cap bounds worst-case cost and keeps saves deterministic enough.

**Do the damage event here, not in Phase 3.** Once away planets spawn
hostiles (Part A keeps `gm.Update`), an unattended crew or robot team can die
with no signal to the player. `EntityDamagedEvent` is a one-line fire in the
combat path on the per-level bus plus a listener; land it with Part C even if
the popup UI waits for Phase 3. Until the popup exists, route it to a
message-log line.

### Part D — Star-Map pause toggle

A pause button on the Star Map that freezes the whole world clock (decision
#2), including background ticking. `WorldManager.SetPaused` / `Paused`
already exist; no UI calls them yet. The Star Map lives in the dashboard
(`dashboard_state.go` / `starmap_panel.go`) rather than a separate
`OverworldState` now.

### Risks in Part B (checkpoint before building)

- **Save format** — multiple in-memory planets must all serialize and restore;
  touches the campaign save/load path.
- **Beam logic** assumes the single `current`; audit against multi-loaded.
- **Memory** — N staffed planets fully in RAM (currently unbounded).

### Suggested order

A (done) → headless test for A → B0 → B → C with the damage event → D.
Checkpoint before B. Do not interleave with the physical-storage work
([physical_storage_plan.md](../brainstorm/physical_storage_plan.md)): both
touch the worker loop and WorldManager; finish one before starting the other.

### Open decisions for Phase 2

1. `collectDatapads` on background planets — **skip** (default; already what
   `stepBackground` does).
2. Throttle — replaced by the owed-ticks budget above; the tunables are the
   per-frame budget and the owed cap.
3. Memory cap on simultaneously-loaded staffed planets — **measure first.**
   Instrument a loaded `MainState` (tile volume × tile size, entity count,
   lighting buffers) on a typical and a large map before deciding. Decide
   between unbounded and a small cap (e.g. 3 plus the ship, freezing the
   least-recently-visited staffed planet) from the numbers, not by argument.

## Phase 3 — notifications

- **`EntityDamagedEvent`** — moved up to Part C (see above). Phase 3 consumes
  it rather than introducing it.
- On an away planet, the first such event raises the **Pause / Travel / Ignore**
  popup, **per-incident with a cooldown** (decisions #6–9). Pause freezes the
  whole world clock while the popup is up; Ignore lets the fight (and possible
  death) play out.
- This is where the **presentation-vs-state** seam from Phase 1 pays off:
  background planets route their presentation events to notifications instead of
  the live message log.

## Phase 4 — polish

World-clock rules, robot-staffing edge cases, save integration for in-memory
staffed planets, notification batching/cooldown tuning.

## Open follow-ups discovered during the audit

- Combat fires **no damage event** today — added in Part C.
- `scenario.Active()` and `settlement.Settlements` are process globals that a
  second loaded planet would share — Part B0.
- `MessageEvent` stays global for now; background-planet messages become
  notifications in Phase 3 rather than log lines.
- `TaskCompletedEvent` is dead code — clean up.
- Future: a dedicated "keep-this-planet-loaded" component (e.g. for hostiles) so
  a planet can stay simulated even with no friendly workers (decision #3).
