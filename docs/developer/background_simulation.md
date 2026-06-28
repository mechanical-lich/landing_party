# Background Planet Simulation

Goal: planets other than the one you're currently on keep simulating in the
background as long as they're staffed, and the player is notified (and can
intervene) when a colonist there comes under attack.

Status: **planning / in progress.** Phase 1 (event routing) underway.

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

## Later phases

- **Phase 2 — background scheduler.** Tick parked-staffed levels in `Update`,
  skipping Lighting/FOV, on a per-frame time budget. Star-Map pause toggle.
- **Phase 3 — notifications.** A new **`EntityDamagedEvent`** (combat currently
  just decrements `HealthComponent` with no event) fired on damage; on an away
  planet it raises the Pause/Travel/Ignore popup, per-incident with a cooldown.
- **Phase 4 — polish.** World-clock rules, robot-staffing edge cases, save
  integration for in-memory staffed planets.

## Open follow-ups discovered during the audit

- Combat fires **no damage event** today — Phase 3 must add `EntityDamagedEvent`.
- `MessageEvent` stays global for now; background-planet messages become
  notifications in Phase 3 rather than log lines.
- `TaskCompletedEvent` is dead code — clean up.
