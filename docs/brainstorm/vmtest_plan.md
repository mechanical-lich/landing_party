# VM Play-Testing Framework — Implementation Plan

Status (2026-11-02): **plan only, nothing implemented.** Every API and behaviour
claim marked *probed* was run against the real game on this Mac (Ebitengine
2.10.5, arm64, 2x display) with a throwaway host; the probe sources are in the
session notes folder, not the repo. Depends on the uncommitted `go.mod`/`go.sum`
Ebiten bump in the working tree.

## Decision

Add `cmd/vmtest`, a host program that launches the game as an Ebitengine VM
guest, plays a **data script** (JSON lines) with real injected input, and writes
numbered PNG frames, state snapshots and a log to an output directory. A
guest-only sidecar socket lets scripts ask the game for facts and for UI element
bounds, so scripts assert state rather than pixels and click named widgets
rather than coordinates.

Rationale: every feature branch so far ends "unseen on screen". Headless tests
drive `MainState.stepWorld` but skip input routing, widget layout, modals and
drawing. A session should be able to run the actual UI, look at the frames,
fix something, and rerun the same script.

This is not OS-level input against the desktop: input travels over the VM socket
to a guest with no window, and the host's window is hidden. It stays inside the
"no remote control" rule.

`cmd/vmtest` is a program, not a `go test` under a tag, because:

- the host must call `ebiten.RunGame` on the main thread; the helper that makes
  this work in vmhost's own tests (`internal/testing.MainWithRunLoop`) is not
  importable outside the ebiten module;
- its output is a directory of artifacts for a session to read, not pass/fail;
- it has to run one script on demand with labels, which suits a CLI.

The parser, step runner logic, diff and sandbox do get ordinary Go tests in
`internal/vmtest`, which need no GPU.

## Where the code is today

| Topic | Finding | Code |
|---|---|---|
| Ebiten version | v2.10.5 in **both** `go.mod` files. Both repos build. | `go.mod:14`, `../mlge/go.mod:6` |
| mlge | **Not** a local replace: the replace is commented out, so the game builds against module-cache `mlge v1.14.0`. | `go.mod:8` |
| Game entry | `main` calls `game.NewGame` then `g.Run()`, which is plain `ebiten.RunGame(g)`. **No change is needed for guest mode**: `-tags ebitenginevmguest` plus `EBITENGINE_VM_ENDPOINT` is enough (*probed*). `RunGameOptions.VMGuestEndpoint` is not needed. | `cmd/game/main.go:60`, `internal/game/game.go:49` |
| Normal builds | The engine's guest code is always linked in 2.10, but without the tag `vmGuestEndpointFromEnv` returns `""`, so the env var cannot activate it. `exp/vmhost` is not a dependency of `cmd/game`. | ebiten `internal/ui/ui_notvmguest.go` |
| Timing | `ebiten.SetTPS(60)` is advisory to the host: `RequestedTPS` reports 60, ticks run at whatever rate the host asks. All game timers count `Update` calls, so nothing needs changing. | `internal/game/game.go:27` |
| Draw-to-sim coupling | `s.viewport` is only set in `MainState.Draw` and fed to FOV and positional audio on the next `Update`. A tick renders nothing, so compressed ticks use a stale viewport. See Design 3. | `internal/game/main_state.go:798,900` |
| Wall clock | `time.Now` drives tile animation and emotes (drawing only) and task `Created` stamps. | `internal/world/drawing.go:410,593`, `internal/world/emotes.go:26`, `internal/workerai/task_handlers.go:152` |
| Paths | `chdirToExecutableIfBundled` chdirs to the binary's directory if `data/` sits beside it. Everything is relative: `data/...`, `saves/`, `saves/campaign/`, `data/config.local.json` (read and written). | `cmd/game/main.go:19`, `internal/config/config.go:92`, `internal/game/saves.go:21`, `internal/game/world_manager.go:45` |
| Bounds, HUD | `GUIManager.ElementBounds(id)` returns `x, y, w, h, ok`; false for unknown or hidden elements. HUD only. | `internal/gui/hud_screen.go:1591` |
| Bounds, other | The tutorial highlight resolver already maps `hud:`, `dash:`, `star:`, `ship:`, `inspector:`, `region:`, `entity:` names to rectangles. Those functions read only `c`, `wm`, `lastSite`, `lastDash`, `lastCtx`, not the director, so they work on a bridge built just for this. | `internal/game/tutorial_highlight.go:29-258` |
| Current state | `mlge/state.StateMachine` has no getter for the current state and its fields are unexported. | mlge `state/statemachine.go` |
| Title screen | Widgets are `TitleState` fields with fixed positions, not in any registry. Seed is typed into `seedInput`. | `internal/game/title_state.go:85-206` |
| Message log | `message.MessageLog []string`, a public package variable. | mlge `message` |
| Cheat console | `CheatModal.runCommand(raw)` is callable directly. Site only. | `internal/game/cheat_modal.go:138` |
| Incident | Tests raise one through `noteAwayDamage` with an `EntityDamagedEvent`. | `internal/game/incident.go:59`, `internal/game/bg_scheduler_test.go:143` |
| Tutorial | `Director.StepID/Index/Count`, `activeTutorial` package variable, 24 steps. The input order is already written down in `TestTutorialWalkthrough`. | `internal/tutorial/director.go`, `internal/game/tutorial_walkthrough_test.go:54` |
| Release contents | `make bin` copies all of `data/`, so scripts must not live there. | `Makefile` |

### Verified `exp/vmhost` surface (v2.10.5, *probed*)

```go
vmhost.EndpointURLFromAddr(ln.Addr()) (string, error)   // "unix:///path"
vmhost.NewGuestSession(conn, &vmhost.NewGuestSessionOptions{IdleTimeout, OnAudioStream, OnTextInput, ...})
(*GuestSession).SetOutsideScreen(img) error  // in device pixels; first host Update, before any tick
.AdvanceTicks(n)  .WaitTicks() bool  .ProcessedTicks() int  .PendingTicks() int
.AdvanceFrame()   .WaitFrame() bool  .CompositeFrame() bool  // Composite and Close: inside a host frame only
.PressKey/ReleaseKey(ebiten.Key)  .TypeRune(r)
.MoveCursor(x, y)  .PressMouseButton/ReleaseMouseButton(b)  .ScrollWheel(x, y)
.RequestedTPS() int  .AudioSampleRate() int  .Err() error  .Close() error
```

The host spawns the guest itself (`os/exec`), listens on a unix socket, accepts,
and creates the session. There is no launch or attach API, and the session holds
no process handle. Frames are read with `img.ReadPixels`.

Probe results:

| Behaviour | Result |
|---|---|
| Real game as guest, title to Star Map to landing to site | Works, rendering looks correct |
| Guest load to connect | about 1.1 s |
| Click coordinates | Logical screen pixels (1152x800), 1:1 with `config.json`, on a 2x display |
| Image size | Physical: 2304x1600 |
| Click recipe | Move, 2 ticks, press, 1 tick, release, 2 ticks: every click landed |
| One-tick key press | 280 of 280 toggles registered, across frame pacings |
| `TypeRune` | One per tick works once the field has focus. The cheat input needs a click first; typing without one is silently lost. |
| Tick speed | Star Map 120 ticks in 14 ms; lit generated planet 1800 ticks in 2.1 s (about 14x real time) |
| Frame cost | 40-130 ms, first frame about 500 ms |
| Planet generation tick | 139 ms for travel on seed 777 |
| Save, close, relaunch in one host run, load, resume | Works |
| Guest calling `os.Exit` | `WaitTicks` false, `Err` = EOF, exit status 0 |
| Audio | 13 streams at 32 kHz, no stall; a session with no handler also ran fine |
| Two sockets | A second unix socket works beside the VM socket |

## Design

### 1. Process layout

```
cmd/vmtest (host, hidden window)               guest (cmd/game -tags ebitenginevmguest)
  script goroutine ---- VM socket ------------> ebiten RunGame as guest
         |                                        |
         +------------- sidecar socket ---------> internal/game/vmhook_on.go
  host Update: performs one frame request         (state, targets, debug commands)
```

`vmtest run` does: build guest, make sandbox, listen on both sockets, start guest,
accept both, `RunGame` with a hidden window, run script, close, reap, write
`result.json`. A failed launch retries once, as the Ebitengine skill advises.
`IdleTimeout` is 60 s. Close waits at most 20 s for the guest, then kills it.

### 2. One guest frame per host Update (required)

**Probed failure:** rendering guest frames in a loop inside a single host `Update`,
which is the shape the bundled driver's `snapshot` suggests, silently freezes
capture somewhere between 1200 and 1500 frames. After that every PNG repeats the
old image while `WaitFrame` and `CompositeFrame` still return true and the game
keeps running. The run looked like a dead `O` key; the save files proved the
dashboard had opened. Root cause not identified.

So the script runs on its own goroutine, which the API allows for ticks and input.
Frame requests go over a channel and the host's `Update` serves one per call.
*Probed:* 5000 frames this way with correct final capture, at about 60 host
frames per second. `SetOutsideScreen` goes in the first host `Update`: calling it
and ticking before `RunGame` processed no ticks in the probe.

### 3. Host ticks, with a frame after input

Ticks are the only clock. `wait` is a tick count, never seconds.

Because `s.viewport` only updates in `Draw`, the runner renders one discarded frame
after each input step and before a `wait`, so FOV and audio see the current camera.
Running 2562 ticks with no frames versus a frame every tick produced **identical
saves** (ignoring IDs and timestamps), but on a nearly idle tutorial site, so this
is reassurance, not proof. `--frame-every 1` gives the real-play 1:1 ratio when a
result looks suspicious; it costs about 16 ms per tick.

### 4. Sandbox and working directory

The guest never runs in the repo. Each run builds this layout:

```
<out>/sandbox/guest        binary, so chdirToExecutableIfBundled picks this directory
<out>/sandbox/assets ->    symlink to repo assets
<out>/sandbox/data/*  ->   symlink per child of data/, except config.local.json
<out>/sandbox/data/config.local.json   written from the script's "config" header
<out>/sandbox/saves/       starts empty
```

*Probed:* saves land in the sandbox and the repo stays clean. This also matters
for correctness: the owner's `data/config.local.json` sets `debugDisableLighting`
and `unlockAllResearch`, and a guest run from the repo silently applies them.

### 5. Sidecar channel

The vmhost protocol has no custom messages, so the guest dials a second unix socket
named by `LP_VMTEST_SOCKET` and speaks JSON lines:

```
> {"op":"state"}                     < {"ok":true,"state":{...}}
> {"op":"target","name":"dash:travelBtn"}  < {"ok":true,"x":170,"y":738,"w":150,"h":30}
> {"op":"targets"}                   < every name resolvable now, for script writing
> {"op":"debug","cmd":"cheat give metal_ore 50"}   < {"ok":true}
```

A goroutine serves requests while holding a mutex that `Game.Update` and
`Game.Draw` also take. Queries therefore **cost no ticks**, so adding an assert to
a script cannot shift its timeline, and there is no data race. *Probed on a small
guest:* 4000 queries interleaved with 6000 ticks, 0 mismatches. Not yet tried in
the game itself.

Rejected: a debug key, because injected keys are game input and need ticks. An env
flag dumping every tick, because it costs time and gives no targets.

Guest side:

- `internal/game/vmhook_on.go` (`//go:build ebitenginevmguest`): socket, mutex,
  state, targets, debug commands.
- `internal/game/vmhook_off.go` (`//go:build !ebitenginevmguest`): empty
  `vmStart(g)`, `vmLock() func()`, `vmNote(state)`.
- `Game.Update`/`Draw` gain `defer vmLock()()`; `NewGame` calls `vmStart`.
- Each state's `Update` calls `vmNote(s)` to record the current state, since
  `StateMachine` has no getter. This avoids an mlge release.

With the tag but no `LP_VMTEST_SOCKET` the hook stays inactive.

### 6. State summary

| Field | Source |
|---|---|
| `state` (`title`, `dashboard`, `site`, `ship`, `settings`, `end`), `title_screen`, `dash_tab` | `vmNote`; `TitleState.screen`; `DashboardState.tabs[active].id` |
| `tick`, `day`, `hour` | `MainState.tick`, `Campaign.Day`, `Level.Hour` |
| `campaign`: `name`, `seed`, `location_id`, `location_name`, `adrift`, `in_orbit`, `won`, `lost` | `Campaign`, `CurrentLocation`, `IsAdrift`, `wm.Current` |
| `fuel`, `hold` (blueprint to count) | `storage.CountResource` with `ShipProvider` |
| `site_storage` (blueprint to count) | `storage.LevelProvider` on the current level |
| `crew`: name, where (`ship` or location ID), x, y, z, task | Worker entities on `shipLevel` and each of `wm.loaded` |
| `loaded` | Keys of `wm.loaded`, with `wm.owed` |
| `tutorial`: `step`, `index`, `count`, `done`, `skipped`, `ended` | `activeTutorial.dir` |
| `modal`: open names | `ModalOpen`, `mapModal`, `cheatModal`, `storageInspector`, `scanOpen` |
| `incident`, `notifications` | `wm.ActiveIncident`, `wm.RecentNotifications(5)` |
| `paused`, `pause_text` | `clockStopped`, `pauseIndicatorText` |
| `selected`, `cursor_mode`, `build_mode`, `camera` | `MainState` fields |
| `messages`: last 10 | `message.MessageLog` |

### 7. Named targets

Reuse the tutorial's grammar so highlight names and script names are one vocabulary.
`vmTargets` builds a throwaway `tutorialBridge{c, wm, lastSite, lastDash, lastCtx}`
and calls its `*Highlight` methods, so it works with the tutorial off. A ring
resolves to its centre, a rectangle to its middle.

Added names, in a table in `vmhook_on.go`:

- `title:new`, `title:load`, `title:name`, `title:seed`, `title:tutorial`, `title:launch`, `title:load_confirm`
- `tab:starmap`, `tab:crew`, `tab:inventory`, `tab:quests`, `tab:encyclopedia`
- `row:locList:<location id>` via `starMapPanel.rowRect`; `row:shipList:<n>`, `row:planList:<n>`
- `tutorial:skip`, `tutorial:manual`, `tutorial:close`
- `popup:ignore`, `popup:pause`, `popup:travel`
- `tile:x,y,z` via `camera.WorldToScreenF`

A target that does not resolve fails the step with the `targets` list in the log.
That catches a button moved off screen or hidden, which is what this tool is for.

### 8. Script format

JSON lines using only the standard library (no new dependency). `#` lines are
comments. The first line is an optional header.

```jsonl
{"name":"new_expedition","seed":12345,"config":{"debugShowScenarioID":false}}
{"op":"wait","ticks":10}
{"op":"screenshot","name":"title"}
{"op":"click","target":"title:new"}
{"op":"click","target":"title:seed"}
{"op":"type","text":"12345"}
{"op":"click","target":"title:launch"}
{"op":"wait_until","path":"state","eq":"dashboard","max_ticks":120}
{"op":"assert","path":"campaign.adrift","eq":true}
{"op":"click","at":[244,751]}
{"op":"key","key":"O","hold":1}
```

| Op | Meaning |
|---|---|
| `wait` | Advance `ticks` |
| `wait_until` | Advance in 30-tick chunks until `path` matches or `max_ticks` passes, then fail |
| `key` | Press, hold `hold` ticks (default 1), release, 2 ticks. `mods` holds Shift etc. around it |
| `click` | `target` or `at`; `button`. The recipe proven above. `press`/`release` split it for drags |
| `move`, `scroll` | Cursor and wheel |
| `type` | One `TypeRune` per tick. Focus the field first |
| `screenshot` | Frame PNG plus `state_NNNN.json`; `name` required |
| `assert` | `path` with `eq`, `ne`, `gte`, `lte`, `contains` |
| `debug` | Sidecar debug command: `cheat ...`, `incident <location id>` |
| `relaunch` | Close session, reap guest, start a new one in the same sandbox |
| `expect_exit` | Next ticks must end the session with exit status 0 |

The first failure stops the run, takes a frame and state dump named `FAIL`, and
exits 1. A guest panic stack goes to `guest.log` and the run fails.

### 9. Determinism

The seed comes from the script. Startup randomness beyond it is the global
`math/rand`, used by `lore.RandomSettlementName` and many others; Go seeds it
randomly and `rand.Seed` is a no-op on this Go. The runner sets
`GODEBUG=randautoseed=0` on the guest, which needs no game change.

*Probed*, same seed twice on the tutorial and on a generated lit planet:

| Check | Without the setting | With it |
|---|---|---|
| Crew names and ship level | Differ | Same |
| Frames at 1800-2400 ticks | Differ up to 0.56% | 0 to 0.1% |
| Saves apart from IDs and timestamps | Differ | Same, except below |

What still varies, none of which blocks use:

- `StableID` uses `math/rand/v2`, which cannot be seeded. Not visible; excluded from state diffs.
- `known_techs` order with `unlockAllResearch` (map order). Sort in the summary.
- `time.Now` animation: up to about 0.1% of pixels. Handled by the diff threshold.
- **Generated planets:** 11 of 61 entities differed (yeti versus elder yeti) on
  `yeti_territory` with the same seed. Source not found; the concurrent workers at
  `internal/generation/primer_planet.go:198` and map iteration are the first
  suspects. Starter scripts use the tutorial planet where they can, and assert
  counts rather than specific enemies on generated ones.

### 10. Starter scripts (`vmtest/scripts/`)

| Script | Plays | Asserts |
|---|---|---|
| `new_expedition.jsonl` | Title, seed, tutorial off, Star Map, select a destination, Travel, beam two down, To Site, 600 ticks | Adrift then in orbit; fuel fell by the shown cost; 2 crew at the location; `state` = `site` |
| `tutorial.jsonl` | All 24 steps with real input, following `TestTutorialWalkthrough` | `tutorial.step` at each stage via `wait_until`; `done`; a frame per step |
| `incident.jsonl` | Land on tutorial planet, beam down, To Ship, `debug incident`, click `popup:travel`; second pass `popup:ignore` | `incident` set; `pause_text` = `PAUSED - incident`; frame of popup; arrival or clock resumes |
| `save_load.jsonl` | Land, wait, Save, `relaunch`, Load, To Site; finally `expect_exit` from the title | Day, location, fuel, crew and hold equal to before save (`assert` against a stored `save_as` snapshot) |
| `storage_beam.jsonl` | Open ship hold inspector, beam metal down, open site container, beam up | `hold` and `site_storage` move by the beamed amount and total is conserved; frames of inspector |

### 11. How a session uses it

```bash
make vmtest SCRIPT=new_expedition LABEL=before
```

Frames land in `vmtest/out/new_expedition/before/` (gitignored):
`0001_title.png`, `state_0001.json`, `log.txt` (each step with tick and result),
`guest.log`, `result.json`. The label defaults to the short git hash. The session
reads frames with its image tool and, after a fix, runs:

```bash
make vmtest SCRIPT=new_expedition LABEL=after
```

```bash
make vmtest-diff SCRIPT=new_expedition A=before B=after
```

The diff pairs frames by name and prints a table of changed-pixel percentages.
Threshold default 0.2%, per-channel tolerance 8. Frames over it get a
`diff_<name>.png` with changes in red. State JSON files are diffed too, ignoring
entity IDs and timestamps. Exit 1 if anything is over threshold.

To compare with a committed revision, run from a `git worktree` of it. Do not use
`git stash`: the stash is shared across sessions. Baselines are not committed
because they depend on the machine's GPU and display.

A short `.claude/skills/playtest/SKILL.md` tells sessions about these commands and
the target listing (`vmtest targets SCRIPT`, which runs to the end and prints them).

### 12. Staying out of release builds

- All guest code is behind `ebitenginevmguest`. The off-file functions are empty.
- `cmd/vmtest` is the only importer of `exp/vmhost`; release targets name `cmd/game`.
- Scripts live in `vmtest/`, not `data/`, so `make bin` does not ship them.
- `make vmtest-check` fails unless both of these hold:

```bash
go list -deps ./cmd/game | grep -c 'exp/vmhost\|landing_party/internal/vmtest'
```

```bash
go build -o "$TMPDIR/lp" ./cmd/game && go tool nm "$TMPDIR/lp" | grep -c vmStartServer
```

Both must print 0. `vmStartServer` is the sidecar entry point in `vmhook_on.go`.

Makefile targets: `vmtest`, `vmtest-all`, `vmtest-diff`, `vmtest-check`.

## Files

| File | Change |
|---|---|
| `cmd/vmtest/main.go` | CLI: `run`, `diff`, `targets`; host `ebiten.Game`, frame channel |
| `internal/vmtest/script.go` | JSONL parser, validation |
| `internal/vmtest/runner.go` | Step execution against a small `Session` interface, so it tests without a GPU |
| `internal/vmtest/guest.go` | Build, sandbox, spawn, relaunch, reap |
| `internal/vmtest/frames.go` | PNG writing, downsample |
| `internal/vmtest/diff.go` | Frame and state diff |
| `internal/vmproto/proto.go` | Sidecar request and reply types, no dependencies |
| `internal/game/vmhook_on.go`, `vmhook_off.go` | Guest hook, tagged |
| `internal/game/game.go` | `vmStart`, `vmLock` calls |
| `internal/game/title_state.go`, `main_state.go`, `dashboard_state.go`, `settings_state.go`, `campaign_end_state.go` | One `vmNote(s)` line each |
| `vmtest/scripts/*.jsonl` | Starter scripts |
| `Makefile`, `.gitignore` | Targets; `vmtest/out/` |
| `.claude/skills/playtest/SKILL.md` | Usage for sessions |

## Tests

- `internal/vmtest`: parser accepts and rejects; runner against a fake session
  (click recipe tick counts, `wait_until` timeout, assert ops, frame requests never
  run off the host goroutine); diff on small generated images; sandbox excludes
  `config.local.json`.
- `internal/game/vmhook_test.go` (tagged): state summary and target resolution on
  `newExpedition`, run with `go test -tags ebitenginevmguest ./internal/game`.
- Regular `go test ./...` must stay green with no tag, proving the off-file compiles.
- Smoke check, by hand, since there is no CI by design: `make vmtest-all` twice, then
  `vmtest-diff` must be under threshold on all but the known generated-planet case.
- Stale-capture guard: a test script of 3000 framed ticks ending in a dashboard
  frame, asserted by a pixel sample. It reproduces Design 2 if pacing regresses.

## Order of work

1. Host skeleton: spawn, sandbox, paced frames, `wait`, `key`, `click at`, `type`, `screenshot`. Run a coordinate-only `new_expedition`. Probe code is the starting point.
2. Sidecar with `state`, tagged hook, `vmNote`, `assert`, `wait_until`.
3. `targets`, and convert `new_expedition` to names.
4. `diff`, labels, Makefile, `vmtest-check`.
5. `relaunch`, `expect_exit`, `save_load`.
6. `debug`, `incident`, `storage_beam`.
7. `tutorial` end to end. Largest script; expect it to find real issues.
8. Skill file, and a `docs/developer` note on writing scripts.

Steps 1-4 are useful alone. Stop and use it on a real branch after step 4.

## Open questions

| Question | Default |
|---|---|
| Wall clock or host ticks? | **Host ticks.** Faster, repeatable. `--frame-every 1` for real-play pacing. |
| How to seed? | Script `seed` typed into the seed field, plus `GODEBUG=randautoseed=0`. Leave generation's residual variance until it bites. |
| Frame size and scale? | Write **1152x800** by box-downsampling the physical image, so frame pixels equal click coordinates and images stay small. `--full` keeps the physical size. |
| Audio? | Ignore: nil `OnAudioStream`. Ran fine without a handler. |
| `ebitenginevmguest` or our own tag for the hook? | Same tag. One switch, and the hook cannot exist without guest mode. |
| JSONL or YAML? | JSONL. YAML is only an indirect dependency today. |
| Other platforms? | macOS only for now. Linux would need Xvfb or EGL per the skill. Windows untested. |
| Fix the generated-planet variance? | Separate task, only if script noise justifies it. |

## Risks

1. **Stale capture.** One frame per host `Update` fixed it to 5000 frames, but the
   cause is unknown and the package is experimental. A longer run or a future
   Ebiten could bring it back, and it fails silently. The guard script covers this.
2. **Compressed ticks may not equal real play.** `viewport` coupling is the known
   case. Equality was only shown on a nearly idle site, not with hauling, combat or
   a moving camera. Mitigated by frames after input and `--frame-every 1`.
3. **Determinism under load.** Runs matched with little happening. A long tutorial
   with busy workers, or generated planets, may diverge further. State asserts and
   the threshold absorb small drift, but `wait_until` budgets could need slack and
   frame diffs could get noisy.
