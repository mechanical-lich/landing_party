# VM Play-Testing (`go tool vmtest`)

The game runs as an Ebitengine VM guest, plays a JSON-lines script with
injected mouse and keyboard input, and writes numbered PNG frames, a state JSON
per frame, and a log. Use it to see a change on screen, and to check it with
asserts, without a window and without touching your desktop.

The framework is mlge's: the host is `github.com/mechanical-lich/mlge/cmd/vmtest`,
a `tool` in `go.mod` (so it is built against this module's Ebiten), and the
game side is `mlge/vmtest/guest`. **The engine reference is mlge's
[`docs/vmtest.md`](https://github.com/mechanical-lich/mlge/blob/main/docs/vmtest.md)**:
script ops and comparisons, keys and paths, output layout, `diff`, the hook
interface, `vmtest.json` and the release check. This page covers what is
Landing Party's: how to run it here, the state summary, the target names, the
debug commands and the starter scripts.

macOS only for now. History: `docs/brainstorm/vmtest_plan.md` (the original
design) and `docs/brainstorm/vmtest_mlge_plan.md` (the move into mlge).

## Run a script

```bash
make vmtest SCRIPT=new_expedition LABEL=before
```

`SCRIPT` is a name under `vmtest/scripts/` or a path to a `.jsonl` file.
`LABEL` defaults to the short git hash. `make vmtest-all` runs every starter
script and fails if any fails. Without make:

```bash
go tool vmtest run -label before new_expedition
```

Frames land in `vmtest/out/<script>/<label>/` at the logical 1152x800, so
pixel (x, y) is click coordinate (x, y). Compare two runs:

```bash
make vmtest-diff SCRIPT=new_expedition A=before B=after
```

To compare against a committed revision, run the same script from a
`git worktree` of that revision. Do not use `git stash`: the stash is shared by
every session.

Expected noise between identical runs: up to about 0.2% of pixels from
wall-clock tile animation (the tutorial's later frames sit closest to that
line). State should match exactly. Known game-side noise: the research and
crafting lists print multi-resource costs in map order, so those rows can swap
between runs.

## Configuration

`vmtest.json` at the repository root: the guest is `./cmd/game`; the sandbox
links `assets/` and `data/` except `data/config.local.json`, which is written
from the script header's `config` (your own local config is never used); saves
go to an empty `sandbox/saves/`. The screen size comes from the game's
`Layout`, i.e. `data/config.json`. `stable_id` is ignored by `diff` on top of
the engine's defaults.

## The hook

`internal/game/vmhook.go` (build tag `ebitenginevmguest`) is `lpHook`, the
game's `guest.Hook`. `Game.Run` wraps the game with `guest.Wrap`, which in
release builds returns the game unchanged. The current screen is
`StateMachine.Current()`. Its test runs with
`go test -tags ebitenginevmguest ./internal/game`.

### State summary

`state` (`title`, `dashboard`, `site`, `ship`, `settings`, `end`),
`title_screen`, `title_fields`, `dash_tab`, `status`, `selected_location`
(`id`, `name`, `fuel_cost`, `summary`, `history_summary`: the summary is the
site history's line; `tier`: the Star Map tag, `threat`, `hazard` or `quiet`), `legends` and `legends_text` (the Legends tab's
chronicle, when it is open), `legends_visible` (the chronicle lines on
screen), `legends_at_top`, `legends_at_bottom`,
`tick`, `hour`, `day`, `campaign` (`name`, `seed`, `location_id`,
`location_name`, `scenario`, `chapter` (the site history's chapter event, `""`
when quiet), `tier`, `known_events`, `adrift`, `in_orbit`, `won`, `lost`,
`known_techs`), `fuel`,
`hold`, `site_storage`, `site_structures`, `crew` (name, where, x, y, z, task),
`crew_at`, `crew_on_site`, `loaded`, `owed`, `tutorial` (`step`, `index`,
`count`, `done`, `skipped`, `ended`, `panel_covered`), `modal`, `incident`, `notifications`,
`paused`, `pause_text`, `cursor_mode`, `build_mode`, `camera`, `selected`,
`settlement`, `messages` (last 10). Fields that do not apply to the current
screen are absent or null.

### Targets

Names resolve to the element's centre on the current screen. A name that does
not resolve fails the step and the log lists every name that does. To list
them at the end of a script:

```bash
make vmtest-targets SCRIPT=new_expedition
```

| Kind | Names |
|---|---|
| `title:` | `new`, `load`, `settings`, `quit`, `name`, `seed`, `tutorial`, `launch`, `cancel`, `load_confirm`, `load_cancel`, `save:<campaign dir>` |
| `tab:` | `starmap`, `crew`, `inventory`, `quests`, `encyclopedia`, `legends` |
| `dash:` | the tutorial's names: `travelBtn`, `locateBtn`, `scanBtn`, `shipList`, `planList`, `beamDownBtn`, `beamUpBtn`, `returnSiteBtn`, `returnShipBtn`, `saveBtn`, `locList` |
| `row:` | `locList:<location id>`, `shipList:<n>`, `planList:<n>` (Star Map lists; a row scrolled out of view does not resolve) |
| `star:` | `tutorial`, `next` (planet on the Star Map canvas) |
| `legends:` | `chronicle` (the Legends tab's scrolling chronicle) |
| `gi:` | `beam_down`, `beam_up` (Global Inventory tab) |
| `inspector:` | `beam`, `list`, `amount`, `all`, `close`, `item:<blueprint>` |
| `ship:` | `hold` (the supply pad, aboard the ship) |
| `hud:` | any HUD element ID: `cat_orders`, `cat_structures`, `order_dig`, `order_mine`, `order_default`, `build_<type>`, `sub_back` |
| `menu:` | the Esc menu: `save`, `load`, `newgame`, `quit` |
| `modal:` | `<modal>:close`, `<modal>:<element id>`, `<modal>:row:<list id>:<n>`, `<modal>:item:<list id>:<text>` for `colonistModal`, `craftingModal`, `researchModal`, `mainMenu`, `saveModal`, `loadModal` (mlge's `minuitarget.Modal` grammar) |
| `entity:` | `<blueprint>` (first on screen) or `<blueprint>:<n>` (nth on screen) |
| `region:` | a level region tag, e.g. `tut_rations` |
| `tile:` | `x,y,z` (on screen, camera level) |
| `settle:` | `dx,dy` from the colony centre |
| `tutorial:` | `skip`, `manual`, `close`, `collapse` |
| `popup:` | `pause`, `travel`, `ignore` (incident popup) |
| `btn:` | `dashboard` |

`dash:`, `star:`, `ship:`, `inspector:`, `region:`, `entity:`, `tutorial:` and
`popup:` reuse the tutorial's highlight resolvers, so they work with the
tutorial off.

### Debug commands

| `debug` cmd | Does |
|---|---|
| `cheat <console command>` | Runs a cheat-console command on the live level (site or ship on screen) and returns the new message-log lines |
| `incident <location id or ship>` | Raises the away-damage popup for that loaded level |
| `reveal all` | Marks every history event known (a long Legends chronicle) |
| `datapad` | Puts the first colonist on the nearest lore datapad (the next update picks it up) |
| `origin [walk [event type [n]]]` | Frames the camera on the site history's resolved chapter origin (the `region:lore_origin` tag), or on the origin of the site's event of that type; `walk` also puts the first colonist on the nearest free tile there, so its field of view reveals it; `n` picks the nth event of that type (the two sides of a per-side event) |

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
| `lore_site` | Start system's Star Map summary is its history's line and its tier tag matches, travel, beam one down, land on the site |
| `lore_xenos` | Seed 1512: jump to a `wreck_eggs_hatch` site, land, walk to the origin: the old wreck's hull with the hive's eggs |
| `lore_ufo` | Seed 1372: jump to a `ship_shot_down` site, land, walk to the origin: the hull and its crew |
| `lore_autonz` | Seed 23: jump to a `wreck_nanites_wake` site, land, walk to the origin: the wreck with wires spreading from it |
| `lore_legends` | Seed 1512: land on the wreck site, pick up a lore datapad, open Legends: the datapad's event is in the chronicle |
| `lore_legends_long` | Seed 746: every event known (`debug reveal all`), Legends opens on the newest events, the wheel scrolls back to the oldest |
| `lore_quiet` | Seed 66: the quiet start system's collapsed-civilization castle stands empty (`debug origin walk civ_collapses`); then the native people's dwelling |
| `lore_hazard` | Seed 407: the Star Map's amber Hazard tag on a `dormant_nest` earth_like site, then the eggs by the wreck at its origin |
| `lore_war` | Seed 125: the start system is a `sundered_world` earth site; walk to each side's fort (`debug origin walk fortify 1` and `2`), the battlefield and a side's dwelling |
| `lore_ship` | Seed 11: the start system's old warship wreck (`crashed_ship`, purpose warship), lighting off, walked to: hull, sections, rooms |

Seeds for the lore scripts come from `make history-dump HFIND=<terms>`
(comma-separated, all required: tier, map, location ID, event type, `stamped:<type>` for an event that keeps its structure, `<var>=<value>` for an event var such as `purpose=warship`), which
lists New Expedition seeds with a matching site on the opening Star Map and
the jump's fuel; pick one with low fuel so its row is on screen.

## Release builds

```bash
make vmtest-check
```

Runs `go tool vmtest check`: `cmd/game` must not depend on the VM host or the
vmtest host packages, an untagged binary must contain neither mlge's sidecar
entry point nor `lpHook` (`check_symbols` in `vmtest.json`), and the tagged
binary must contain both, so the check can fail.
