# VM Play-Testing (`cmd/vmtest`)

`cmd/vmtest` runs the real game as an Ebitengine VM guest, plays a JSON-lines
script with injected mouse and keyboard input, and writes numbered PNG frames,
a state JSON per frame, and a log. Use it to see a change on screen, and to
check it with asserts, without a window and without touching your desktop.

macOS only for now. Design and probe results: `docs/brainstorm/vmtest_plan.md`.

## Run a script

```bash
make vmtest SCRIPT=new_expedition LABEL=before
```

`SCRIPT` is a name under `vmtest/scripts/` or a path to a `.jsonl` file.
`LABEL` defaults to the short git hash. `make vmtest-all` runs every starter
script and fails if any fails. Without make:

```bash
go run ./cmd/vmtest run -label before new_expedition
```

Flags for `run`: `-label`, `-out DIR`, `-frame-every N` (render a discarded
frame every N ticks of a wait; `1` is real-play pacing, about 16 ms a tick),
`-full` (frames at the display's physical size instead of the logical
1152x800).

Exit status is 0 when every step passed.

## Where things land

`vmtest/out/<script>/<label>/` (gitignored):

| File | What |
|---|---|
| `0001_title.png`, ... | One frame per `screenshot`, numbered in order, at logical size, so pixel (x, y) is click coordinate (x, y) |
| `state_0001.json`, ... | The guest's state summary at that frame |
| `NNNN_FAIL.png`, `state_NNNN.json` | Taken when a step fails |
| `log.txt` | Every step with its tick and timing; on failure, every target name resolvable at that moment |
| `guest.log` | The game's own stdout and stderr (a panic stack lands here and fails the run) |
| `result.json` | Pass or fail, the failing line, ticks, frame count and timings |
| `sandbox/` | The guest's working directory: the binary, symlinks to `assets/` and `data/*`, its own `data/config.local.json` and `saves/` |

The guest never runs in the repo. Your `data/config.local.json` is not used:
the sandbox gets one written from the script header's `config`, and saves go
to `sandbox/saves/`.

Read the frames with your image tool. Read `state_NNNN.json` for numbers
instead of squinting at pixels.

## Compare two runs

```bash
make vmtest SCRIPT=new_expedition LABEL=after
```

```bash
make vmtest-diff SCRIPT=new_expedition A=before B=after
```

The diff pairs frames by name and prints the changed-pixel percentage of each.
A pixel counts as changed when any channel differs by more than 8; a frame over
0.2% fails and gets `diff_<name>.png` in the B directory, changed pixels in red
over a dimmed copy. State files are compared too and every differing path is
listed. Exit 1 if anything differs. Flags: `-threshold`, `-tol`, `-ignore`
(state keys to skip). `vmtest diff DIR_A DIR_B` compares any two directories.

To compare against a committed revision, run the same script from a
`git worktree` of that revision, then diff the two directories. Do not use
`git stash`: the stash is shared by every session. Baselines are not committed;
they depend on the machine's GPU and display.

Expected noise between identical runs: up to about 0.2% of pixels from
wall-clock tile animation. State should match exactly. Known game-side noise:
the research and crafting lists print multi-resource costs in map order, so
those rows can swap between runs.

## Write a script

One JSON object per line. `#` lines are comments. The first line may be a header:

```jsonl
{"name":"my_check","seed":777,"config":{"debugShowScenarioID":false}}
{"op":"wait","ticks":10}
{"op":"click","target":"title:new"}
{"op":"click","target":"title:seed"}
{"op":"type","text":"777"}
{"op":"click","target":"title:launch"}
{"op":"wait_until","path":"state","eq":"dashboard","max_ticks":120}
{"op":"assert","path":"campaign.adrift","eq":true}
{"op":"screenshot","name":"adrift"}
```

Ticks are the only clock. The guest runs as fast as it can (about 14x real time
on a lit planet), so `wait` is a tick count, never seconds.

| Op | Fields | Does |
|---|---|---|
| `wait` | `ticks` | Advance that many ticks |
| `wait_until` | `path`, one comparison, `max_ticks` | Advance in 30-tick chunks until the comparison holds; fail after `max_ticks` |
| `key` | `key`, `hold` (default 1), `mods` | Press (mods first), hold, release, 2 ticks |
| `click` | `target` or `at: [x, y]`, `button` | Move, 2 ticks, press, 1 tick, release, 2 ticks |
| `press`, `release`, `move` | as `click` | The parts of a click, for drags (`release` without a place releases where the cursor is) |
| `scroll` | `dx`, `dy` | Wheel |
| `type` | `text` | One rune per tick. Click the field first or the text is lost |
| `screenshot` | `name` | Frame plus state file |
| `assert` | `path`, one comparison | Fail unless it holds |
| `save_as` | `name` | Keep the current state under a name |
| `debug` | `cmd` | `cheat <console command>` on the live level, or `incident <location id or ship>` to raise the away-damage popup |
| `relaunch` | | Close the game and start it again in the same sandbox (saves survive) |
| `expect_exit` | `max_ticks`, optional `key` | Press the key, then require the game to exit with status 0 |

Comparisons: `eq`, `ne`, `gte`, `lte`, `contains` (substring, list element or
map key), `eq_saved` / `ne_saved` (same path in a `save_as` snapshot).

Keys use Ebitengine names (`A`, `Escape`, `Space`, `Enter`, `Backspace`,
`ShiftLeft`, `Digit1`) plus `Shift`, `Ctrl`, `Esc` and bare digits.

Paths are dotted: `campaign.location_id`, `hold.metal_ore`, `crew.0.name`,
`crew.#` (a length). After every input step the runner renders one discarded
frame, because the game updates its viewport only when drawing.

### State summary

`state` (`title`, `dashboard`, `site`, `ship`, `settings`, `end`),
`title_screen`, `title_fields`, `dash_tab`, `status`, `selected_location`,
`tick`, `hour`, `day`, `campaign` (`name`, `seed`, `location_id`,
`location_name`, `adrift`, `in_orbit`, `won`, `lost`, `known_techs`), `fuel`,
`hold`, `site_storage`, `site_structures`, `crew` (name, where, x, y, z, task),
`crew_at`, `crew_on_site`, `loaded`, `owed`, `tutorial` (`step`, `index`,
`count`, `done`, `skipped`, `ended`), `modal`, `incident`, `notifications`,
`paused`, `pause_text`, `cursor_mode`, `build_mode`, `camera`, `selected`,
`settlement`, `messages` (last 10). Fields that do not apply to the current
screen are absent or null. The source is `internal/game/vmhook_on.go`.

### Targets

Names resolve to the element's centre on the current screen. A name that does
not resolve fails the step and the log lists every name that does. To list
them at the end of a script:

```bash
make vmtest-targets SCRIPT=new_expedition
```

| Prefix | Names |
|---|---|
| `title:` | `new`, `load`, `settings`, `quit`, `name`, `seed`, `tutorial`, `launch`, `cancel`, `load_confirm`, `load_cancel`, `save:<campaign dir>` |
| `tab:` | `starmap`, `crew`, `inventory`, `quests`, `encyclopedia` |
| `dash:` | the tutorial's names: `travelBtn`, `locateBtn`, `scanBtn`, `shipList`, `planList`, `beamDownBtn`, `beamUpBtn`, `returnSiteBtn`, `returnShipBtn`, `saveBtn`, `locList` |
| `row:` | `locList:<location id>`, `shipList:<n>`, `planList:<n>` (Star Map lists, unscrolled) |
| `star:` | `tutorial`, `next` (planet on the Star Map canvas) |
| `gi:` | `beam_down`, `beam_up` (Global Inventory tab) |
| `inspector:` | `beam`, `list`, `amount`, `all`, `close`, `item:<blueprint>` |
| `ship:` | `hold` (the supply pad, aboard the ship) |
| `hud:` | any HUD element ID: `cat_orders`, `cat_structures`, `order_dig`, `order_mine`, `order_default`, `build_<type>`, `sub_back` |
| `menu:` | the Esc menu: `save`, `load`, `newgame`, `quit` |
| `modal:` | `<modal>:close`, `<modal>:<element id>`, `<modal>:row:<list id>:<n>`, `<modal>:item:<list id>:<text>` for `colonistModal`, `craftingModal`, `researchModal`, `mainMenu`, `saveModal`, `loadModal` |
| `entity:` | `<blueprint>` (first on screen) or `<blueprint>:<n>` (nth on screen) |
| `region:` | a level region tag, e.g. `tut_rations` |
| `tile:` | `x,y,z` (on screen, camera level) |
| `settle:` | `dx,dy` from the colony centre |
| `tutorial:` | `skip`, `manual`, `close`, `collapse` |
| `popup:` | `pause`, `travel`, `ignore` (incident popup) |
| `btn:` | `dashboard` |

Prefer names over `at` coordinates: a moved or hidden button then fails loudly
instead of clicking empty space.

### Determinism

The runner sets `GODEBUG=randautoseed=0` on the guest and scripts type their
seed, so two runs of a starter script produce the same state at every frame.
Generated planets have shown some variance in enemy types between same-seed
runs; assert counts rather than specific enemies there.

## Starter scripts

| Script | Covers |
|---|---|
| `new_expedition` | Title, seed, tutorial off, Star Map, travel (fuel charged), beam two down, To Site, 600 ticks |
| `tutorial` | All 24 tutorial steps with real input, a frame per step, ends on the end panel and closes it |
| `incident` | Away-damage popup on the ship: clock stops, Travel goes to the planet; again, Ignore resumes the clock |
| `save_load` | Land, save, relaunch, load: day, location, fuel, hold, storage and crew match; back to title via the Esc menu; Q exits with status 0 |
| `storage_beam` | Ship hold inspector beams metal down, site locker inspector beams some back; totals conserved |
| `esc_menu` | Land on the start site, open the Esc menu (title, button labels), close it with Esc |

## Release builds

All guest code is behind the `ebitenginevmguest` build tag
(`internal/game/vmhook_on.go`; `vmhook_off.go` holds empty stubs), and only
`cmd/vmtest` imports `exp/vmhost`. Check it with:

```bash
make vmtest-check
```

It fails if `cmd/game` depends on the VM host or vmtest packages, or if an
untagged binary contains `vmStartServer`. The hook's own test runs with
`go test -tags ebitenginevmguest ./internal/game`.
