# VM Play-Testing in mlge — Implementation Plan

Status (2026-11-03): **plan only, nothing implemented.** This plan spans
**two repositories**: `github.com/mechanical-lich/mlge` (engine, at
`../mlge`, currently v1.14.1) and this one (Landing Party, which consumes
mlge v1.14.1 from the module cache, not through a `replace`). Each section
says which repo it is about. The framework being moved is described in
[vmtest_plan.md](vmtest_plan.md) and [docs/developer/vmtest.md](../developer/vmtest.md).

## Decision

Move the generic half of the VM play-testing framework into mlge as three
packages and one command:

| Package (mlge) | Role | Build tag |
|---|---|---|
| `vmtest` | host library: script parser, runner, `Session` over `exp/vmhost`, sandbox, guest process, sidecar client, frames, diff, release check | none (host side only; never imported by a game binary) |
| `vmtest/proto` | sidecar protocol, dependency-free | none |
| `vmtest/guest` | guest side: the `Hook` interface, `Wrap`, the sidecar server | real code under `ebitenginevmguest`, no-op stubs otherwise |
| `vmtest/guest/minuitarget` | turns minui elements, list rows, tabs and modal parts into target rects | `ebitenginevmguest` |
| `cmd/vmtest` | the host program, configured by a `vmtest.json` in the game's repo | none |

Plus small, independently useful additions to `ui/minui` (rect accessors)
and `state` (`StateMachine.Current`).

**The host binary lives in mlge; games do not keep a `cmd/vmtest`.** A game
adds it to its own `go.mod` as a tool (`go get -tool
github.com/mechanical-lich/mlge/cmd/vmtest`, Go 1.24+ `tool` directive; both
modules are on Go 1.26) and runs it as `go tool vmtest`. Reasons:

- **Ebiten's VM protocol is version-locked** (`vmhost.NewGuestSession` fails
  the handshake if guest and host Ebiten differ). A tool built inside the
  game's module uses the game's pinned Ebiten, so host and guest always
  match. A globally `go install`ed binary would not.
- After moving `BuildGuest`'s package path, `MakeSandbox`'s directory names,
  `screenSize`'s config keys and the scripts/out paths into `vmtest.json`,
  nothing in `cmd/vmtest/main.go` or `host.go` is game-specific. A ten-line
  `main` per game would be pure duplication that drifts.
- A game that needs something unusual can still write its own main:
  `cmd/vmtest` is itself only `vmtest.Main(os.Args)` over the library.

Rationale for doing this at all: every game on the engine gets "run the real
UI from a script and look at the frames" for a hook of a few hundred lines,
most of which is that game's own state summary and target names.

## Where the code is today

### Landing Party (this repo)

Classification: **E** engine-generic, **M** minui-generic (depends only on
`mlge/ui/minui` types), **G** game-specific, **C** generic once a constant
moves to config.

| File | Functions / types | Class |
|---|---|---|
| `internal/vmproto/proto.go` (32) | `Request`, `Reply`, `OpState`/`OpTarget`/`OpTargets`/`OpDebug` | E |
| `internal/vmtest/script.go` (283) | `Header`, `Step`, `Script`, `ParseFile`, `Parse`, `Validate`, `needWhere`, `needComparison`, `safeName`, `ParseKey`, `ParseButton` | E (`Header.Seed` is informational, `Header.Config` is "written to the local config file", both stay with a config-driven filename) |
| `internal/vmtest/runner.go` (487) | `Session`, `Output`, `Runner`, `Run`, `fail`, `describe`, `ticks`, `render`, `where`, `step`, `check`, `Lookup`, `Compare`, `Equal`, `number`, `show`, `wantText`, `SortedKeys`, `LogWriter`, `StepError`, `ErrNoSidecar` | E |
| `internal/vmtest/diff.go` (320) | `DiffOptions`, `FrameDiff`, `StateDiff`, `DiffReport`, `CompareRuns`, `PixelDiff`, `DiffImage`, `diffValues`, `listFrames`, `readPNG`, `readState`, `Format`, `DefaultIgnoreKeys` | E (`DefaultIgnoreKeys` → config default) |
| `internal/vmtest/frames.go` (118) | `Downsample`, `Dir`, `NewDir`, `WriteFrame`, `Logf`, `WritePNG` | E |
| `internal/vmtest/sidecar.go` (84) | `Sidecar`, `NewSidecar`, `call`, `State`, `Target`, `Targets`, `Debug` | E |
| `internal/vmtest/guest.go` (226) | `GuestTag`, `Guest`, `SpawnOptions`, `StartGuest`, `Exited`, `Wait`, `Kill`, `Close` | E |
| | `SidecarEnv = "LP_VMTEST_SOCKET"`, temp dir prefix `"lpvm"` | C (rename `MLGE_VMTEST_SOCKET`, `mlvm`) |
| | `BuildGuest` (hard-codes `./cmd/game`), `MakeSandbox` (hard-codes `assets`, `data/`, `data/config.local.json`, `saves/`) | C |
| `cmd/vmtest/host.go` (281) | `host`, `hostConfig`, `launch`, `Update`, `inFrame`, `sessErr`, `finish`, every `Session` method, `Relaunch`, `ExpectExit` | E (the `GODEBUG=randautoseed=0` env stays as an engine default) |
| `cmd/vmtest/main.go` (336) | `usage`, `main`, `runFlags`, `cmdRun`, `cmdTargets`, `cmdDiff`, `Result`, `play`, `guestPanicked`, `gitLabel` | E |
| | `scriptPath` (`vmtest/scripts/`), `screenSize` (reads `data/config.json` `screenWidth`/`screenHeight`, default 1152×800), out dir `vmtest/out/` | C |
| `internal/game/vmhook_off.go` (13) | stubs `vmStart`, `vmLock`, `vmNote`, `vmNoop` | E (replaced by `guest`'s stubs) |
| `internal/game/vmhook_on.go` (920) | `vm` struct, `vmStart` (reads the env var), `vmStartServer` (the `vmtest-check` symbol), `vmServe`, `vmLock`, `vmNote`, `vmHandle` (op dispatch, panic recovery) — lines 36–125 | E |
| | `vmRectOf` (visible-chain check + absolute rect), `vmListRow` (row n; hard-codes `rowH = 20`, ignores scroll), `vmTabRect` (re-implements minui's private `tabItemWidth`) | M |
| | `vmModalTarget`'s `close` / `<id>` / `row:` / `item:` grammar (the close rect `Width-28, +4, 24×22` copies `minui.Modal`'s private `closeBtnBounds`) | M (the modal lookup itself, `GUIManager.ModalElement`, is G) |
| | `vmTargets` (probe each candidate, keep the ones that resolve) | E |
| | `vmState`, `vmList`, `vmWorldState`, `vmStructures`, `vmStock`, `titleScreenNames`, `vmWorld` | G |
| | `vmTarget`'s kind switch, `vmBridge` (reuses the tutorial's highlight resolvers), `vmTitleTarget`, `vmDashRow`, `vmGITarget`, `vmInspectorTarget`, `vmTutorialTarget`, `vmPopupTarget`, `vmNthEntity`, `vmTileTarget`, `vmCandidates` | G |
| | `vmDebug` (`cheat`, `incident`) | G |
| `internal/game/game.go` | `NewGame` calls `vmStart(g)` after pushing `TitleState`; `Update` and `Draw` each `defer vmLock()()` | call sites |
| `internal/game/{title,settings,dashboard,campaign_end,main}_state.go` | each `Update` starts with `vmNote(s)` because `state.StateMachine` has no getter | call sites |
| `internal/gui/gui_manager.go`, `hud_screen.go` | `GUIManager.ElementBounds` / `HUDScreen.ElementBounds` (find by id, reject hidden ancestors), `ModalElement` (named-modal map + recursive find) — added for the tutorial | M-ish: the bounds half belongs in minui; the modal name map is G |
| Tests | `internal/vmtest/vmtest_test.go` (341: parse, runner against a fake `Session`, downsample, sandbox), `diff_test.go` (86), `internal/game/vmhook_test.go` (87, tagged: state and targets on a tutorial expedition) | E, E, G |
| Tooling | `Makefile` targets `vmtest-host`, `vmtest`, `vmtest-targets`, `vmtest-all`, `vmtest-diff`, `vmtest-check`; `vmtest/scripts/*.jsonl` (6 scripts, 510 lines); `.claude/skills/playtest/SKILL.md`; `docs/developer/vmtest.md` | mixed |

`cmd/game/main.go` does nothing the hook relies on beyond `game.NewGame` and
`g.Run()`; the chdir-to-bundle logic is a no-op in the sandbox because the
guest binary sits next to `data/`, which is what we want.

**Host side needs nothing from the game's module.** `cmd/vmtest` imports
only `ebiten`, `exp/vmhost` and `internal/vmtest`; `internal/vmtest` imports
only `ebiten` and `internal/vmproto`. Every game coupling is a string
(package path, directory names, config keys, env var).

Totals: of ~3,450 non-doc lines, **~2,170 move to mlge as is** (host 617,
library 1,518, proto 32), **~430 test lines move**, **~185 hook lines are
replaced by mlge code** (dispatch/server ~90, rect helpers ~55, modal grammar
~30, probe loop ~10), and **~735 hook lines plus the 87-line hook test stay**
in Landing Party, shrinking to roughly 650 once the helpers are gone.

### mlge (`../mlge`)

- Layout: flat top-level packages (`audio`, `client`, `dice`, `ecs`, `event`,
  `input`, `message`, `particle`, `path`, `resource`, `sense`, `simulation`,
  `state`, `task`, `text`, `tilemap`, `transport`, `ui`, `utility`, `world`).
  **No `cmd/`, no `internal/`, no tooling packages, no build tags anywhere.**
  The module root is itself a `package main` demo (`main.go`).
- Examples: `examples/{client_server,linked,min-ui-demo,particles,tcp,ui-demo}`,
  each a `main` package.
- Tests: 18 `_test.go` files, 12 use testify. `Makefile` has one target,
  `test: go test ./... -v`. No CI test job (the owner's choice).
- Docs: `docs/*.md` per package (a Jekyll site: `_config.yml`, `index.md`).
- `go.mod`: Go 1.26, Ebiten v2.10.5 (same as Landing Party).
- `state/statemachine.go`: `StateMachine{states, currentState}` with
  `Update`, `Draw`, `PushState`, `PopCurrentState`; no accessor for the
  current state, hence `vmNote`.
- `ui/minui`:
  - `GUI.FindElementByID` searches `elements` only, not `modals`; `modals` is
    unexported with no accessor.
  - `ListBox` keeps `itemHeight` (20) and `scrollOffset` unexported; the row
    rect math lives inline in `Update`/`Draw`.
  - `tabbar.go` has unexported `tabItemWidth`, `tabStripLayout`,
    `tabStripHitTest`; `TabBar.region()` is unexported.
  - `Modal`'s close button rect is computed twice inline (`Update`, line
    ~130, and `drawCloseButton`).
  - `GetContentBounds(bounds, style)` and `ElementBase.GetAbsolutePosition`
    are public.

### Other games

- `fantasy_settlements`: mlge v1.9.3 with `replace => ../mlge`, **Ebiten
  v2.9.9**, minui in 2 files.
- `spaceplant`: mlge v1.6.1, **Ebiten v2.9.9**, minui in 14 files.
- `mlge-template`: Ebiten v2.8.8, mlge via `replace`, no minui.

None can adopt this until it is on Ebiten 2.10 and a current mlge. That
upgrade, not the hook, is the bulk of a second game's cost.

## Design

### 1. Package layout (mlge)

```
mlge/
  vmtest/            host library (package vmtest)
    config.go        Config + LoadConfig(vmtest.json) + defaults
    script.go        moved as is; adds Header.Format
    runner.go        moved as is
    session.go       was cmd/vmtest/host.go: Host (ebiten.Game + Session over vmhost)
    play.go          was main.go's play/Result/guestPanicked/gitLabel
    guest.go         StartGuest, BuildGuest(cfg), MakeSandbox(cfg)
    sidecar.go       moved; adds Hello
    frames.go        moved as is
    diff.go          moved as is
    check.go         release-safety check (was the Makefile recipe)
    main.go          Main(args) — the subcommands
  vmtest/proto/      package proto (was internal/vmproto)
  vmtest/guest/      package guest
    hook.go          Hook, Rect, Router (no tag; types only, no imports beyond ebiten)
    guest_on.go      //go:build ebitenginevmguest — Wrap, server, lock, dispatch
    guest_off.go     //go:build !ebitenginevmguest — Wrap returns the game unchanged
  vmtest/guest/minuitarget/   //go:build ebitenginevmguest
  cmd/vmtest/main.go          func main() { os.Exit(vmtest.Main(os.Args[1:])) }
  examples/vmguest/           tiny guest game for the engine's own end-to-end test
  docs/vmtest.md
```

`vmtest` (host) stays untagged but is never imported by a game's main
package; the release check (§6) proves it.

### 2. The hook a game implements (`mlge/vmtest/guest`)

```go
// Rect is a target in logical screen pixels.
type Rect struct{ X, Y, W, H float64 }

// Hook is the game's side of the sidecar. Every method runs on the sidecar
// goroutine while Wrap holds the lock that Update and Draw also take, so it
// may read any game state without races and costs no ticks.
type Hook interface {
	// State is the summary scripts assert on. Any value that marshals to a
	// JSON object (a map or a struct with json tags).
	State() any
	// Target resolves a target name to a rect; false means "not on this screen".
	Target(name string) (Rect, bool)
	// Candidates are the names worth probing for `vmtest targets` and for the
	// FAIL log; the engine keeps those Target resolves.
	Candidates() []string
	// Debug runs a game debug command and returns its output.
	Debug(cmd string) (string, error)
}

// Wrap returns g with the sidecar attached when MLGE_VMTEST_SOCKET is set in
// a guest build; in any other build it returns g itself.
func Wrap(g ebiten.Game, h Hook) ebiten.Game
```

- **Map or struct for State:** `any`. The guest JSON-encodes it, the host
  already decodes into `map[string]any` and `Lookup` walks dotted paths, so
  a game can start with a map (as Landing Party does) and move to a struct
  later without the host noticing. The contract is the JSON, not the Go type.
- **Wrap instead of `vmStart` + `vmLock`:** `Wrap` takes the lock around the
  wrapped game's `Update` and `Draw` and starts the server, so the game's
  own `Update`/`Draw` need no edits. The wrapper forwards `Layout` and, when
  the game implements them, `LayoutF` and `DrawFinalScreen` (a type-switch
  picks one of four wrapper types so ebiten's interface checks still see the
  optional methods). For a game that cannot be wrapped, `guest.Lock() func()`
  and `guest.Start(h)` stay exported as the manual path.
- **Current state without `vmNote`:** add `func (s *StateMachine) Current()
  StateInterface` to `mlge/state` (returns `nil` when empty). Under the lock
  it returns the state that ran the last `Update` and will draw next — the
  same thing `vmNote` recorded, except right after a push, where it already
  names the new state, which is what the next frame will show. The five
  `vmNote` calls go away.
- **Startup:** the hook is registered from a tagged file so release builds
  never see the implementation:

  ```go
  // game.go (untagged)
  var vmHook guest.Hook // set by vmhook.go in guest builds
  func (g *Game) Run() error { return ebiten.RunGame(guest.Wrap(g, vmHook)) }

  // vmhook.go (//go:build ebitenginevmguest)
  func init() { vmHook = lpHook{} }
  ```

  The untagged `guest_off.go` imports only ebiten, so a release build links
  a one-line `Wrap` and nothing else.
- **Target naming convention** (documented in `docs/vmtest.md`): a name is
  `kind:rest`, colon-separated, lower-case kinds, ids as the game spells
  them. `Router` is an optional helper: `r.Handle("tab", fn)`,
  `r.Target(name)` cuts at the first colon and dispatches. Kinds are the
  game's; mlge reserves none, so Landing Party's existing names (`title:new`,
  `hud:build_bed`, `modal:craftingModal:close`, `settle:0,-2`, …) and its
  six scripts keep working unchanged. The docs recommend `ui:<element id>`,
  `modal:<name>:…` and `tab:<id>` for new games so the minui helpers map
  one-to-one.

### 3. Landing Party's hook against the interface (outline)

```go
//go:build ebitenginevmguest
package game

type lpHook struct{ g *Game } // init sets vmHook; NewGame fills g

func (h lpHook) cur() state.StateInterface { return h.g.StateMachine.Current() }
func (h lpHook) State() any          { return vmState(h.cur()) }        // unchanged body, ~240 lines
func (h lpHook) Debug(c string) (string, error) { return vmDebug(h.cur(), c) } // unchanged, ~50
func (h lpHook) Candidates() []string { return vmCandidates(h.cur()) }  // unchanged, ~50
func (h lpHook) Target(name string) (guest.Rect, bool) {               // ~380, was vmTarget + resolvers
	r := guest.Router{}
	r.Handle("tab", func(id string) (guest.Rect, bool) { return minuitarget.Tab(dash.tabBar, id) })
	r.Handle("modal", func(rest string) (guest.Rect, bool) {
		name, sub, _ := strings.Cut(rest, ":")
		return minuitarget.Modal(hud.ModalElement(name, ""), sub) // close | <id> | row:<list>:<n> | item:<list>:<text>
	})
	r.Handle("hud", func(id string) (guest.Rect, bool) { return minuitarget.ByID(hudGUI, id) })
	// title, row, gi, menu, tile, settle, btn, and the tutorial-bridge kinds as today
	return r.Target(name)
}
```

Deleted from Landing Party: `vmhook_off.go`, the `vm` struct, `vmStart`,
`vmStartServer`, `vmServe`, `vmLock`, `vmNote` and its five call sites,
`vmHandle`, `vmRectOf`, `vmListRow`, `vmTabRect`, the modal grammar in
`vmModalTarget`, `vmTargets`. `vmCandidates` stops calling `vmTarget` itself.
`vmBridge` and every tutorial-backed kind stay: they are the game's.

### 4. What minui exposes natively (mlge)

So hooks and the tutorial stop re-deriving layout from private constants:

| Addition | Replaces |
|---|---|
| `func VisibleRect(el Element) (Rect, bool)` — absolute rect, false if `el` or any ancestor is hidden or the rect is empty | `vmRectOf`, `HUDScreen.ElementBounds`' loop |
| `func (g *GUI) FindElement(id string) Element` — searches elements **and** modals (keep `FindElementByID` as is for compatibility) | half of `HUDScreen.ModalElement` |
| `func (g *GUI) Modals() []Element` | the game's need to keep its own modal list just to find one |
| `func (lb *ListBox) RowRect(i int) (Rect, bool)` — absolute, **scroll-aware**, false when scrolled out | `vmListRow` (which assumes `rowH = 20` and no scroll — a latent bug) |
| `func (lb *ListBox) ScrollTo(i int)` | nothing yet; lets `minuitarget` bring a row into view before clicking (optional, step 2) |
| `func (tb *TabBar) TabRect(id string) (Rect, bool)` and the same on `TabPanel`, over the existing `tabStripLayout` | `vmTabRect` (copy of `tabItemWidth`) |
| `func (m *Modal) CloseRect() Rect` — and `Update`/`drawCloseButton` use it | the hard-coded `Width-28, +4, 24×22` |

`GUIManager.ElementBounds` / `HUDScreen.ElementBounds` stay on the game
side as one-line calls to `minui.VisibleRect(gui.FindElementByID(id))`; the
tutorial keeps its API. The modal **name** map in `HUDScreen.ModalElement`
stays game-side (modals are game fields), but its recursive search becomes
`minui.FindIn(m, id)`.

`minuitarget` (tagged) is then ~60 lines: `ByID(gui, id)`, `Element(el)`,
`ListRow(lb, n)`, `ListItem(lb, text)`, `Tab(tb, id)`, `Modal(m, sub)`
implementing the `close | <id> | row:<list>:<n> | item:<list>:<text>` grammar.

### 5. Script format and protocol as the engine's contract

- **Script ops** (unchanged): `wait`, `wait_until`, `key`, `click`, `press`,
  `release`, `move`, `scroll`, `type`, `screenshot`, `assert`, `save_as`,
  `debug`, `relaunch`, `expect_exit`; comparisons `eq`, `ne`, `gte`, `lte`,
  `contains`, `eq_saved`, `ne_saved`. Game-specific behaviour enters only
  through `debug` commands and target names, never new ops.
- **Header** gains `"format": 1` (optional; absent means 1). The host
  rejects a format it does not know with "needs mlge ≥ vX".
- **Versioning rule:** adding an op, a step field or a header field is
  backwards compatible and does not bump `format` (old scripts still parse;
  a new script on an old host fails at parse with "unknown op", which is the
  right error). Changing what an existing op or field means, or removing
  one, bumps `format`, and the host keeps parsing format N−1 for one minor
  release. Parsing stays strict (`DisallowUnknownFields`) so typos fail.
- **Sidecar protocol** gains `OpHello`: the host sends it once after
  accepting the sidecar; the guest replies `{ok, proto: 1, mlge: "<version>",
  screen: {w, h}}`. `proto` follows the same rule as `format`. A mismatch is
  a launch error that names both versions. The reported `screen` replaces
  `screenSize`'s read of `data/config.json`: the guest calls the wrapped
  game's `Layout(1, 1)` — correct for fixed-layout games (all of the
  owner's); `vmtest.json`'s `screen` overrides it for any other.
- **Output layout is part of the contract** (format 1):
  `<out>/<script>/<label>/{log.txt, result.json, guest.log, NNNN_<name>.png,
  state_NNNN.json, diff_NNNN.png, sandbox/}`. `diff` and the playtest skill
  read it; changing it is a format bump.

### 6. Engine-owned behaviour

All of this moves with the code and stops being Landing Party's concern:

- **Spawn and session:** `EndpointURLFromAddr` + `NewGuestSession`,
  `SetOutsideScreen` inside the first host `Update`, one guest frame per
  host `Update` (the `inFrame` channel), vsync off, TPS 1000, hidden 320×240
  host window, launch retry once, 60 s idle timeout.
- **Logical vs physical:** frames downsampled to the logical size unless
  `-full`, so frame pixels equal click coordinates.
- **Determinism:** `GODEBUG=randautoseed=0` (verified still honoured by
  `math/rand` in Go 1.26.4; it does nothing for `math/rand/v2`, which games
  must seed themselves — documented). Inherited `GODEBUG`,
  `EBITENGINE_VM_ENDPOINT` and the sidecar env are stripped.
- **Sandbox**, driven by `vmtest.json`:

  ```json
  {
    "guest": "./cmd/game",
    "link": ["assets", "data"],
    "local_config": "data/config.local.json",
    "empty_dirs": ["saves"],
    "scripts": "vmtest/scripts",
    "out": "vmtest/out",
    "ignore_state_keys": ["id", "stable_id", "created", "timestamp"],
    "screen": null
  }
  ```

  Every entry in `link` is symlinked child by child (so `local_config` can
  be excluded and replaced by the script's `config`), `empty_dirs` are
  created. Landing Party's file is exactly the above.
- **Release-safety check** becomes `go tool vmtest check`, replacing the
  Makefile recipe. It asserts:
  1. `go list -deps <guest>` (untagged) contains neither
     `github.com/hajimehoshi/ebiten/v2/exp/vmhost`, nor
     `github.com/mechanical-lich/mlge/vmtest`, nor `…/vmtest/proto`
     (`…/vmtest/guest` *is* allowed: its untagged half is the stub);
  2. the untagged binary has no `mlge/vmtest/guest.startServer` symbol
     (`go tool nm`);
  3. the tagged binary does have it, so the check can fail.

  The symbol is the engine's, not the game's, so every game gets the same
  check with no configuration. Games that want their own hook proved absent
  too may add `"check_symbols": ["game.lpHook"]` to `vmtest.json`.

### 7. The playtest skill

The skill is one 52-line Markdown file whose steps are generic but whose
examples (script names, target names, 1152×800) are the game's. It **lives
in mlge** as `docs/vmtest-skill.md`, a template with placeholders, and each
game **copies** it to `.claude/skills/playtest/SKILL.md` and fills in its
own starters and targets. Referencing across repos does not work (skills are
read from the project's own `.claude/`), and a copy lets the description
name the game's screens, which is what makes the skill trigger. Commands in
the copy change from `make vmtest …` to the same Make targets, which now
call `go tool vmtest`, so the skill text barely moves.

## Files

### mlge

| File | Change |
|---|---|
| `state/statemachine.go` | `Current() StateInterface` |
| `state/statemachine_test.go` | `Current` after push, pop, empty |
| `ui/minui/element.go` | `VisibleRect`, `FindIn` |
| `ui/minui/gui.go` | `FindElement` (elements + modals), `Modals` |
| `ui/minui/listbox.go` | `RowRect`, `ScrollTo`; `Update`/`Draw` use the same row math |
| `ui/minui/tabbar.go`, `tabpanel.go` | `TabRect` over `tabStripLayout` |
| `ui/minui/modal.go` | `CloseRect`; `Update` and `drawCloseButton` call it |
| `ui/minui/rects_test.go` | new: each accessor, hidden ancestor, scrolled list |
| `vmtest/*.go` | moved from Landing Party `internal/vmtest` + `cmd/vmtest` (§1), plus `config.go`, `check.go`, `main.go` |
| `vmtest/*_test.go` | moved `vmtest_test.go`, `diff_test.go`; new `config_test.go`, `e2e_test.go` |
| `vmtest/proto/proto.go` | moved from `internal/vmproto`; `OpHello`, `Proto`, `Screen` |
| `vmtest/guest/hook.go`, `guest_on.go`, `guest_off.go`, `guest_test.go` | new (§2) |
| `vmtest/guest/minuitarget/minuitarget.go`, `_test.go` | new (§4) |
| `cmd/vmtest/main.go` | new, three lines |
| `examples/vmguest/main.go`, `vmtest.json`, `vmtest/scripts/smoke.jsonl` | new (Tests) |
| `Makefile` | `test-guest: go test -tags ebitenginevmguest ./vmtest/guest/...`; `vmtest-e2e` |
| `docs/vmtest.md`, `docs/vmtest-skill.md`, `docs/index.md`, `docs/ui.md`, `docs/state-machine.md` | new / one-line links |

### Landing Party

| File | Change |
|---|---|
| `go.mod` | mlge ≥ new tag; `tool github.com/mechanical-lich/mlge/cmd/vmtest` |
| `vmtest.json` | new (§6) |
| `cmd/vmtest/`, `internal/vmtest/`, `internal/vmproto/` | deleted |
| `internal/game/vmhook_off.go` | deleted |
| `internal/game/vmhook_on.go` → `vmhook.go` | rewritten against `guest.Hook` (§3) |
| `internal/game/vmhook_test.go` | imports `mlge/vmtest/proto` → calls `lpHook` methods directly |
| `internal/game/game.go` | `vmHook` var; `Run` wraps; drop `vmStart`, `vmLock` |
| `internal/game/{title,settings,dashboard,campaign_end,main}_state.go` | drop `vmNote` |
| `internal/gui/hud_screen.go` | `ElementBounds` → `minui.VisibleRect`; `ModalElement` search → `minui.FindIn` |
| `Makefile` | `vmtest*` targets call `go tool vmtest`; `vmtest-check` → `go tool vmtest check` |
| `.claude/skills/playtest/SKILL.md` | re-copied from the mlge template, same content |
| `docs/developer/vmtest.md` | cut to Landing Party's targets, debug commands, state keys; link mlge `docs/vmtest.md` for the rest |

## Tests

**Move to mlge, unchanged in substance** (`vmtest`, no GPU, plain `go test`):
parse accept/reject table, click recipe, key with mods, unresolved target
fails with a FAIL frame, `wait_until` and timeout, assert ops, `save_as` /
`eq_saved`, `frame_every`, screenshot numbering, downsample, sandbox excludes
the local config (now driven by a `Config`), `PixelDiff`, `CompareRuns`.

**New in mlge:**

- `config_test.go`: defaults, unknown key rejected, Landing Party's file
  round-trips.
- `proto`: `hello` round-trip; format/proto mismatch error text.
- `guest_test.go` (tagged): `Wrap` serves `state`, `target`, `targets`,
  `debug`, `hello` over a `net.Pipe`; a panicking hook returns an error reply
  and the server keeps serving; `Wrap` preserves `LayoutF` /
  `DrawFinalScreen`. Untagged: `Wrap(g, h) == g`.
- `minuitarget_test.go` (tagged) and `ui/minui/rects_test.go`: rects for
  a built GUI, hidden ancestor, scrolled list box, tab widths with an icon,
  modal close rect equals the hit-test rect.
- **End-to-end, no game:** `examples/vmguest` is a ~80-line game — one
  minui button, a counter, a list box, a `quit` debug command — with a
  ~40-line hook. `vmtest/e2e_test.go` builds `cmd/vmtest` and the example,
  runs `smoke.jsonl` (click the button by target, `wait_until counter eq 1`,
  screenshot, `relaunch`, `expect_exit`), and checks `result.json` and the
  frame count. It needs the main thread and a window server, so it runs
  only with `MLGE_VMTEST_E2E=1` (`make vmtest-e2e`); `go test ./...` skips
  it. `go tool vmtest check` against the example runs in the same target.

**Stay in Landing Party:** `internal/game/vmhook_test.go` (state summary
and target resolution on a tutorial expedition), the six scripts as the
walkthroughs, `make vmtest-all` and `make vmtest-check` as the local gate.

## Order of work

Each step is shippable on its own; stop after any of them.

1. **mlge: `state.Current` and the minui rect accessors** (§4, Files rows
   1–8), with tests. Useful regardless of vmtest; Landing Party's tutorial
   can adopt them on its next mlge bump. *Tag mlge v1.15.0.*
2. **mlge: `vmtest`, `vmtest/proto`, `cmd/vmtest`** — copy Landing Party's
   code, introduce `Config`/`vmtest.json`, `hello`, `check`. Move the tests.
   No guest package yet: Landing Party's own hook still works against it if
   `proto` stays wire-identical (it does — `hello` is additive, and an old
   guest's "unknown op hello" reply is treated as proto 0 with the screen
   size taken from `vmtest.json`).
3. **mlge: `vmtest/guest`, `minuitarget`, `examples/vmguest`, e2e test,
   docs, skill template.** Run `make test test-guest vmtest-e2e`. *Tag mlge
   v1.16.0* (or fold 2–3 into one tag).
4. **Landing Party, commit A — switch the host:** bump mlge, add the tool
   and `vmtest.json`, point the Makefile at `go tool vmtest`, delete
   `cmd/vmtest` and `internal/vmtest`. The old hook still speaks the wire
   protocol, so `make vmtest-all` must pass with identical frames:
   run it before the bump with `LABEL=pre` and after with `LABEL=post`,
   then `make vmtest-diff` per script.
5. **Landing Party, commit B — switch the hook:** rewrite `vmhook.go`
   against `guest.Hook`, delete `vmhook_off.go`, `internal/vmproto`, the
   `vmNote` calls and `vmLock`; `hud_screen.go` uses the minui accessors.
   `go test -tags ebitenginevmguest ./internal/game`, `make vmtest-all`,
   `make vmtest-diff` against `pre`, `make vmtest-check`.
6. **Landing Party, commit C — docs and skill:** trim
   `docs/developer/vmtest.md`, re-copy the skill, update `vmtest_plan.md`'s
   status line to point here.

The six scripts and the skill keep working at every step because target
names do not change and the wire protocol stays compatible across 2→4→5.
The owner makes every commit; nothing here commits.

## Adopting it in a second game (checklist)

1. Ebiten ≥ 2.10.5 and mlge ≥ the step-3 tag (for `fantasy_settlements`
   and `spaceplant`, this upgrade from 2.9.9 / v1.6–1.9 is most of the work).
2. `go get -tool github.com/mechanical-lich/mlge/cmd/vmtest`.
3. `vmtest.json` at the repo root (guest package, dirs to link, local
   config file, empty dirs).
4. `RunGame(guest.Wrap(g, vmHook))` and an untagged `var vmHook guest.Hook`.
5. A tagged `vmhook.go`: `State` (a map of the 10–20 facts scripts check),
   `Target` (start with `minuitarget.ByID` for `ui:<id>` and `Modal` for
   `modal:…`), `Candidates`, `Debug` (often just the game's cheat console).
6. If the game does not use `state.StateMachine`, the hook keeps its own
   "current screen" pointer.
7. One script under `vmtest/scripts/`, Make targets, copy the skill
   template, add `vmtest/out/` to `.gitignore`, run `go tool vmtest check`.

Rough size: **150–300 lines** of hook for a minui game with a handful of
screens, ~20 lines of config and Make targets, plus the engine/Ebiten
upgrade. A day for the hook and first script once the upgrade is done.

## Open questions (defaults chosen)

- **Ebiten version coupling.** The VM protocol is version-locked and
  `exp/vmhost` is experimental, so mlge's `go.mod` floor becomes the floor
  for every game that adopts vmtest, and an Ebiten release that changes
  `exp/vmhost` forces an mlge release. *Default:* accept it; host and guest
  match automatically through the `tool` directive; mlge pins the Ebiten it
  tests against and bumps it deliberately; `hello` reports versions so a
  mismatch says so instead of failing the handshake opaquely. Games not
  using vmtest pay nothing extra (they already inherit mlge's Ebiten floor).
- **Is the output layout part of the contract?** *Default:* yes (§5), as
  format 1, because `diff` and the skill read it.
- **Windows / Linux.** Today macOS only (tested on arm64, 2× display).
  Ebiten's endpoint URL already handles Windows unix-socket paths, and Go
  supports AF_UNIX on Windows 10+. The obstacles are the sandbox's symlinks
  (need developer mode on Windows) and the hidden host window needing a
  display (`xvfb` on Linux). *Default:* keep the code portable
  (`filepath`, `os.MkdirTemp`), fall back to copying when `os.Symlink`
  fails, document macOS as supported and Linux as "should work, untested";
  no Windows work until a game needs it.
- **Does diff belong in the engine?** *Default:* yes. It is ~320 lines of
  Go already exposed as `vmtest diff`, depends only on the output layout the
  engine owns, and is what makes before/after runs useful. A shell script
  would need ImageMagick and lose the state diff.
- **Host binary as a tool vs. a per-game main.** *Default:* tool (Decision).
  Revisit only if a game needs host behaviour that config cannot express.
- **Default `ignore_state_keys`.** *Default:* `["id", "created",
  "timestamp"]` in the engine; `stable_id` moves to Landing Party's
  `vmtest.json`.
- **`Wrap` vs. manual lock.** *Default:* `Wrap`, with `Lock`/`Start`
  exported for games whose `ebiten.Game` is not wrappable.

## Riskiest assumptions

1. **`Wrap` is transparent.** Ebiten discovers optional interfaces
   (`LayoutFer`, `FinalScreenDrawer`) by type assertion; a wrapper that
   misses one silently changes rendering. Mitigation: the four-type switch
   and a test per interface; the manual `Lock` path as fallback.
2. **`go tool` builds the host with the game's Ebiten and that is enough
   for the version lock.** True only if the tool is run from the game's
   module (Make targets do this) and nobody `go install`s it. The `hello`
   check turns a mismatch into a readable error, but the handshake fails
   before the sidecar connects, so `launch` must also translate vmhost's
   handshake error.
3. **`StateMachine.Current()` equals what `vmNote` recorded.** It differs
   for the tick a state pushes another: `vmNote` names the state that just
   ran, `Current` names the new one. Scripts that `assert state` right after
   a transition could flip. Mitigation: commit B's `vmtest-diff` against
   the `pre` run compares every `state_NNNN.json`, which catches it; if it
   bites, keep a tagged `vmNote` in Landing Party only.

## What in mlge argues against this

- **One consumer.** Only Landing Party is on Ebiten 2.10; the other games
  are a version and several mlge minors behind. Until one of them upgrades,
  the engine carries ~2,600 lines (code + tests) and an `exp/` dependency for
  a single user. Step 1 (minui/state accessors) is worth doing anyway;
  steps 2–3 could wait for a second game's upgrade without loss, since
  Landing Party's copy works today.
- **No precedent for tooling in mlge.** No `cmd/`, no build tags, no
  `internal/`, a `package main` at the module root. vmtest would introduce
  all three conventions at once, and the guest package's tag split means
  `go test ./...` no longer covers everything (`make test-guest` is needed).
- **`exp/vmhost` stability.** mlge has so far depended only on Ebiten's
  stable API. An experimental API in the engine means an Ebiten bump can
  break mlge's build for every game, not just Landing Party's tool.
- **The engine's tests cannot exercise the loop under `go test ./...`**
  (main thread + window server), so the most important test is opt-in and,
  with no CI, runs only when someone remembers `make vmtest-e2e`.

None of these is a blocker; together they argue for landing step 1 now and
steps 2–3 when a second game is ready to adopt.
