# Background Planet Simulation

Goal: planets other than the one you're currently on keep simulating in the
background as long as they're staffed, and the player is notified (and can
intervene) when a colonist there comes under attack.

Status: **Phase 1 complete and committed** (`43a9f67` — per-level event bus).
Phase 2+ designed below, not yet built. The per-level event bus is the reusable
foundation: each planet already fires/consumes its own sim events in isolation,
which is what lets multiple planets tick without cross-contamination.

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

## Phase 2 — background scheduler (designed, not built)

Splits into a small/safe piece (A) and a large/risky one (B), then C/D.

### Part A — the background-tick path (`stepBackground`)

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
≠ current that have workers and call `stepBackground()`, **throttled** by a
per-frame budget / reduced cadence (start ~1/4 the live rate, tunable). Honor a
global pause flag.

### Part D — Star-Map pause toggle

A pause button on the Star Map (`OverworldState`) that freezes the whole world
clock (decision #2), including background ticking.

### Risks in Part B (checkpoint before building)

- **Save format** — multiple in-memory planets must all serialize and restore;
  touches the campaign save/load path.
- **Beam logic** assumes the single `current`; audit against multi-loaded.
- **Memory** — N staffed planets fully in RAM (currently unbounded).

### Suggested order

A → B → C → D, building/testing between. Part A first (safe, isolated,
headless-testable); checkpoint before the Part B WorldManager refactor.

### Open decisions for Phase 2

1. `collectDatapads` on background planets — skip (lean) or keep?
2. Throttle default — ~1/4 the live tick rate to start, or a specific target?
3. Memory cap on simultaneously-loaded staffed planets — unbounded or capped?

## Phase 3 — notifications

- A new **`EntityDamagedEvent`** — combat currently just decrements
  `HealthComponent` with **no event**, so this must be added and fired on damage.
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

- Combat fires **no damage event** today — Phase 3 must add `EntityDamagedEvent`.
- `MessageEvent` stays global for now; background-planet messages become
  notifications in Phase 3 rather than log lines.
- `TaskCompletedEvent` is dead code — clean up.
- Future: a dedicated "keep-this-planet-loaded" component (e.g. for hostiles) so
  a planet can stay simulated even with no friendly workers (decision #3).
