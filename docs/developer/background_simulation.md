# Background Planet Simulation

Goal: planets other than the one you're currently on keep simulating in the
background as long as they're staffed, and the player is notified (and can
intervene) when a colonist there comes under attack.

Status (revised 2026-10-31):

- **Phase 1 complete** (`43a9f67` — per-level event bus).
- **Phase 2 complete** (Parts A, B0, B, C, D): staffed planets stay loaded and tick
  on a budget, with a Star Map pause.
- **Phase 3 complete**: Pause / Travel / Ignore popup and dashboard notifications.
- **Phase 4 complete**, apart from the items under "Open follow-ups".
- **Covered by headless tests only.** None of the UI (Star Map button, popup,
  notifications) nor a long multi-planet session has been played by eye.

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

Order: A → B0 → B → C (+ the damage event) → D. All built.

### Part A — the background-tick path (`stepBackground`) — BUILT

Implemented in `MainState.stepBackground`
([rogue.go](../../internal/game/rogue.go)) with the `addLogic` / `addRender`
dual-registration in `newMainStateBase`
([main_state.go](../../internal/game/main_state.go)). `WorldManager.TickBackground`
calls it for The Ship every frame from both the live level and the dashboard,
honouring `wm.paused`. `TestStepBackgroundAdvancesParkedLevel` is the regression
guard.

**Deviation from decision #5:** `FOVSystem` stays on the background stack, because
`findBed` reads `level.Visible`, so a background colonist needs FOV to find a bed.
Only `LightingSystem` and `EmoteSystem` are skipped. The notes below saying FOV is
skipped are out of date. `stepBackground` also advances the level's clock and purges
completed tasks, and sets `level.Background`, which makes combat skip its global
effects and music. The original design notes follow for reference.

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

### Part B0 — de-globalise what a second loaded planet would share — BUILT

Four process-global assumptions that a second loaded planet, or an enemy faction
with `Worker` entities, would have broken. Nothing here changes gameplay, apart
from the save bug noted under item 3. Tests are in `internal/game/b0_*_test.go`.

**1. Per-level scenario.** `MainState.scenario` is set at generation or load, and
`GameMaster.Update(sc)` and `applyScenarioLighting` read it. It lives on
`MainState`, not `world.Level`, because `scenario` imports `world`.
`scenario.Active()` remains the orbited scenario and only
`WorldManager.buildParked` moves it; constructing a `MainState` no longer does.
Deviation: nothing in production reads `Active()` today. The Star Map uses
`scenario.ByID` for a location's own scenario, and the ship has no scenario and
sets its own lighting. It stays for the planned `ship_effects` hook
([the_ship.md](the_ship.md)).

**2. Per-location settlement name.** A location's colony is
`"<colony> @ <location name>"`, recorded as `Location.SettlementName` when its
level is first built (`locationColonyName`). Beam-down tags colonists for the
destination planet. Location names are already unique within a campaign.

**Save compatibility: old campaigns break, on purpose.** There is no migration.
`LoadCampaign` refuses a campaign that has a frozen location without a
`SettlementName`, and the title screen shows the error. Loading one would tag
everything on the planet with the old shared name, so its workers would find no
settlement and sit idle. A campaign that never froze a location still loads.

**3. Scoped save/load.** `world.SaveLevel(level, settlementNames...)` carries only
the named settlements. A location save holds its colony plus any
`"<faction>:<locID>"` settlements (`locationSettlements`); the ship's save holds
only the ship's. `Freeze` also removes the location's settlements from the global
map, and `LoadSaveData` merges what the save carries, so `pinShipSettlement` is
gone. Task queues persist as before
([persist.go](../../internal/settlement/persist.go)).

Two consequences:

- `SaveCampaign` keeps playing, so it uses `writeLocation`, which is `Freeze`
  without the removal.
- Saving from aboard the ship used to write the ship level into the orbited
  location's file, destroying that planet's save. `SaveCampaign` now writes the
  orbited location instead. This is the one behaviour change in B0.

Non-campaign saves (`SaveSettlement`) still carry every settlement, since only one
level runs at a time there.

**4. Colony-worker predicate.** `components.IsColonyWorker(e, colony)` requires a
living `Worker` whose `SettlementComponent.Name` is that colony or ship settlement.
`MainState.colonyName()` supplies the name. It replaces the bare `Worker` test in
colonist counts, beam lists, population, objective population, equip and
task-filter orders, sleep and attack orders, rogue takeover, the cursor tools, the
map modal, datapad pickup and the killer lookup for drops. The bare `Worker` tests
left in `internal/game` only guard reading or adding the component, or already
compare settlements.

### Part B — WorldManager: single `current` → multi-loaded — BUILT

As designed below, with these details:

- Staffed means `countLevelColonists > 0`: `IsColonyWorker`, so robots count and
  hostile workers don't.
- `Freeze` and `writeLocation` take the location. `Freeze` also drops the planet from
  `loaded`.
- `Location.Loaded` records what was in memory at save; `EnterStartingOrbit` restores
  those through `restoreLoaded`. Only `Travel` and `EnterStartingOrbit` move
  `scenario.Active()`.
- Past `maxLoadedPlanets`, the least recently visited planet other than the one in
  orbit freezes.
- `TotalColonists` replaces the wipe check's stored-tally arithmetic. That also fixes
  an old bug: crew on the orbited planet went uncounted while the player was aboard.
- Quest fixtures also spawn on arrival at a planet that stayed loaded, since no thaw
  would otherwise pick them up.

The original design:

- Replace `current` with `loaded map[string]*MainState` + `currentID string`,
  and a `Current()` accessor. **~18 `wm.current` references** in
  [world_manager.go](../../internal/game/world_manager.go) migrate to `Current()`.
- `Travel(dest)`: the departed planet **stays in `loaded` if staffed** (has any
  `Worker`-component entity — colonists *or* robots, per decision #3); otherwise
  `Freeze` it to disk and drop it from `loaded` as today.
- **Save** must serialize *every* loaded planet, not just current
  (`SaveCampaign` path).
- Beam-up/down, `addToSite`, storage queries, etc. repoint to `Current()`.

**Memory measurements** (2026-10-31, `BG_MEASURE=1 go test ./internal/game -run
TestMeasureLoadedPlanet -v`: real generated maps, all kept loaded, 5 workers each,
heap after GC, Go heap only):

| Map / scenario | Tiles | Entities | Heap | Per bg tick |
|---|---|---|---|---|
| asteroid / asteroid_field | 144×153×6 = 132k | 5 | 12 MB | 29 µs |
| moon / random | 295×382×10 = 1.13M | 65 | 90 MB | 29 µs |
| alien / alien_world | 482×384×13 = 2.41M | 205 | 191 MB | 58 µs |
| alien / crystalline_blight | 412×452×13 = 2.42M | 185 | 191 MB | 69 µs |
| alien / alien_world | 474×452×12 = 2.57M | 205 | 204 MB | 58 µs |
| earth_like (near largest) | 510×510×14 = 3.64M | 85 | 289 MB | 41 µs |

About 80 bytes per tile, dominated by tile volume rather than entities, and flat
over 500 ticks. Not counted: GPU images, of which each planet's minimap can use up
to about 17 MB once drawn (4 px per tile, RGBA, at 510×510). Lighting buffers are
inside the per-tile figure.

Unbounded isn't small: ten large planets is about 3 GB. So the cap is **3 planets
plus the ship**, about 870 MB in the worst case. CPU isn't the limit; these ticks
are tens of microseconds, though they were measured with idle workers and few fights
in progress, so treat them as a floor.

### Part C — scheduler — BUILT

`WorldManager.TickBackground`. The budget is a tick count for determinism:
`backgroundTicksPerFrame` (2) and `backgroundOwedCap` (60) in `data/config.json`. It
runs from the level and dashboard `Update`s rather than `Game.Update`, since those are
what hold the live level. The pathfinding budget resets before every background tick.

**Deviations:**

- The ship isn't on the budget. It still steps every frame, as before; it's cheap and
  it has no hostiles.
- The orbited planet counts as away whenever the player isn't on it (aboard the ship
  or on the dashboard), so it ticks then too. It used to stand still.
- `combat.Hit` is the one `EntityDamagedEvent` fire site, and every in-repo strike goes
  through it. Not covered: strikes inside ml-rogue-lib's `AISystem`, used only by
  `HostileAI` entities, which no blueprint has today; and non-combat damage such as
  radiation, which raises no alert.

The away-planet log line this part added was replaced by the popup in Phase 3.

The original design:

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

### Part D — Star-Map pause toggle — BUILT

A button on the Star Map's bottom row reads "Running: Pause" or "Paused: Resume". The
Scan button shrank to make room. The original design:

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

### Open decisions for Phase 2

1. `collectDatapads` on background planets — **skip** (default; already what
   `stepBackground` does).
2. Throttle — replaced by the owed-ticks budget above; the tunables are the
   per-frame budget and the owed cap.
3. Memory cap on simultaneously-loaded staffed planets — **measured, capped at 3
   plus the ship** (`maxLoadedPlanets` in `data/config.json`), freezing the
   least-recently-visited staffed planet. See "Memory measurements" in Part B.

## Phase 3 — notifications — BUILT

[incident.go](../../internal/game/incident.go). The incident queue and notifications are
plain data on `WorldManager`; `incidentPopup` is a thin modal over them, hosted by both
levels and the dashboard. An incident is a fight: it ends once the colonists have gone
unhurt for the cooldown, measured from the last hit in the planet's own ticks. Choices
made where the design left room:

- Pause dismisses the popup and leaves the clock stopped.
- Travel costs normal fuel. If the ship is short the error shows and the popup stays.
- Notifications are in memory only, capped at 50, with the last three drawn on the
  dashboard. Hits on The Ship would raise the popup too, though nothing attacks there.

The original design:

- **`EntityDamagedEvent`** — moved up to Part C (see above). Phase 3 consumes
  it rather than introducing it.
- On an away planet, the first such event raises the **Pause / Travel / Ignore**
  popup, **per-incident with a cooldown** (decisions #6–9). Pause freezes the
  whole world clock while the popup is up; Ignore lets the fight (and possible
  death) play out.
- This is where the **presentation-vs-state** seam from Phase 1 pays off:
  background planets route their presentation events to notifications instead of
  the live message log.

## Phase 4 — polish — BUILT

**World-clock rules.** There is one clock.

| Stopped by | Live level | Away planets and ship |
|---|---|---|
| Space (`MainState.Paused`) | stopped | stopped |
| Star Map pause (`wm.paused`) | stopped | stopped |
| Incident popup | stopped | stopped |
| Rogue mode, between turns | stopped | stopped; each round ticks them too |
| Open modals (map, inspector) | running | running |

The pause follows the player: `adoptWorldPause` turns the Star Map pause into the
level's own on entry, so Space releases it, and `openDashboard` hands it back. While
`clockStopped` holds, the HUD shows a marker at bottom centre: "PAUSED" for Space, or
"PAUSED - star map" / "PAUSED - incident" when something else stopped it.

**Robot-only staffing.** Robots staff a planet and count as colonists, so a robot-only
colony also stops the total-wipe defeat. An away planet that loses its crew freezes on
the next frame (decision #4). The one in orbit stays loaded but idle, ready for a
beam-down.

**Save integration.** Nothing further was needed beyond Part B. Owed ticks, incident
cooldowns, pending incidents and notifications aren't saved, so a loaded campaign
starts them clean.

**Tuning.** `incidentCooldownTicks` (1800) and `notificationMergeTicks` (600) in
`data/config.json`.

## Open follow-ups discovered during the audit

- ~~Worker chatter reaching the live log from background levels~~ — done: worker,
  needs and stat-progression chatter now posts through `internal/chatter`, which
  drops lines while `stepBackground` is running (flag set alongside
  `level.Background`). Script `say` lines (`ScriptSystem`, `ScriptedAISystem`)
  still post directly; route them the same way if they prove noisy.
- A level generated with a fallback scenario doesn't record it on the location, so a
  thaw uses the original ID. Predates this work.
- Background tick cost was measured with mostly idle crews. Check it during a real
  multi-planet fight before raising the budget.
- `MessageEvent` stays global.
- Future: a dedicated "keep-this-planet-loaded" component (e.g. for hostiles) so
  a planet can stay simulated even with no friendly workers (decision #3).
