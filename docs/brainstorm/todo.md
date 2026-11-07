# TODO

Status audited 2026-10-29 against `dev` (d3f4e56). Items confirmed shipped
were moved to the Done list; "Verify" items could not be confirmed from code
alone and need a play-test or a judgment call.

## Open

- Door sounds. `data/blueprints/structures/doors.json` has no `Sound`
  component and `data/audio.json` has no door entries.
- Expanding enemy factions: two-tier model (abstract presence on the
  campaign, carried by a planet or a fleet; materialized into worker-driven
  faction settlements on loaded levels), data-driven doctrines. Order:
  xenos → mirror colony → greyish → autonz. Xenos spread via a
  research-ranged egg launcher; doctrine-driven player settlement so races
  become playable later. Planned 2026-10-31, updated 2026-11-05.
  Plan: [enemy_factions_plan.md](enemy_factions_plan.md).
- Observer mode / self-play: a director that runs the player's own colony
  (phase C1 of the factions plan), driven by vmtest for integration and
  balance soak runs. Useful on its own before any enemy faction exists.

- The tutorial panel covers the bottom rows of the research and crafting
  modals; it could collapse to its title row while a HUD modal is open, as it
  does for the storage inspector.

## Verify

- Fix the resource tracker. No tracker-named code found; confirm which HUD
  readout this meant and whether it still misbehaves.
- Balance all building resource costs.
- Fix gear items to better match what they are.
- Replace the notify and research sounds. Both `notify.ogg` and
  `research_done.ogg` were (re)added on 2026-07-17; decide if those are the
  final ones.
- Ration consumption aboard the ship looks too fast: in a headless probe six
  idle crew went from 20 rations to 2 in about 11 minutes, more than one meal
  each. Seen 2026-10-31 during tutorial planning; cause unknown. Matters for
  the tutorial's food budget.

## Done

- Lore and history, steps 1–5, implemented 2026-11-07 on
  `claude/lore-history-4d1fe2` (not yet merged): star date, generated
  prehistory, histories bound at charting (scenario from the chapter, Star Map
  summary, evidence and lore datapads at resolved origins), every scenario's
  threat grown from its origin, live history and the Legends tab, quests from
  history. Step 6 deferred to the enemy factions work. Plan:
  [lore_plan.md](lore_plan.md); system: `docs/developer/lore.md`.

- Tutorial planet, merged 2026-11-01 (`80d3132`): guided first landing,
  Travel lock, pack-up and jump-start on a second planet, every expedition
  now starts adrift at the Wreck Site. Plan: [tutorial_plan.md](tutorial_plan.md).
  A real playthrough for pacing and prompt wording is still owed; the vmtest
  `tutorial` script drives all steps headlessly.
- Background planet simulation, merged 2026-11-01 (`52151dc`): per-level
  scenario and settlements, scoped saves, staffed planets stay loaded and
  tick on an owed-ticks budget, incident popup, dashboard notifications,
  Star Map pause, HUD pause marker. Plan: `docs/developer/background_simulation.md`.
- VM play-test framework, merged 2026-11-03 (`a2b8b69`), then moved into
  mlge v1.15.0 and run from the engine via `go tool vmtest` (`a7b0e72`,
  2026-11-05). Six starter scripts under `vmtest/scripts/`; see
  `docs/developer/vmtest.md`. Plans: [vmtest_plan.md](vmtest_plan.md),
  [vmtest_mlge_plan.md](vmtest_mlge_plan.md).
- VM play-test UI defects, fixed 2026-11-03 on `claude/ui-fixes-1229e3`:
  - Modals draw their title bar, close X and button/field faces again: mlge
    v1.14.1 strokes rounded borders with `FillRule: ebiten.NonZero` (since
    Ebiten 2.9 the default FillAll filled the whole shape in the border
    colour).
  - Star Map info column clipped above the Scan / Pause row with an ellipsis.
  - Tutorial panel collapse button shows its "-" / "+" (padding left no room).
  - Esc menu buttons pinned to 240px, so "New Game" is no longer "New Ga…".
  - Tutorial highlights stand down while any modal is open (HUD modals, map,
    console, incident popup, scan report).
  - Research and crafting cost lines sorted by material.
  - Pause marker moved above the message log, clear of the tutorial panel.
  - Arrows and other glyphs Roboto lacks replaced with ASCII (`< Back`,
    `Beam Down  v`, `Beam Up  ^`, `Relocate`, `•`, `—`); a test checks every
    UI string against the font.
  - `ModalOpen` knows the colonist, crafting and research modals; the HUD's
    own modals no longer feed `SetInputBlocked` (that would freeze them).
  - Research modal widened to 760px so each tech row fits, its right edge
    kept left of the minimap so the close X stays visible.

- Attack, pickup, retrieve, store, relocate and entity-mine orders survive save
  and load, via a `StableID` component; orders whose target is gone are dropped.
- Rations and other single items list and beam in the Storage Inspector; beam-up
  no longer destroys what a full hold refuses (fixed 2026-11-01).
- Loose stack counts: the hover panel lists "Metal Ore x37" and the map labels
  a pile with its tile total; mined units stay separate entities on the ground.
- Colonists starving while asleep: hunger now wakes a sleeping or passed-out
  worker when food exists, and they keep their exhaustion (fixed 2026-11-01).
- Planet generation race: `PlanetPrimer.Prime` workers no longer touch the
  shared `DirtyColumns`; columns are marked after the join (fixed 2026-11-01).
- Stack duplication in `StorageComponent.AddItem` when a stackable partly
  fit a full container (fixed 2026-10-31; regression test in
  `internal/components/storage_test.go`).
- Physical storage: all material use is carried, with a per-trip shopping
  list and a Str-based carry limit, replacing the old "unified storage" idea.
  Plan: [physical_storage_plan.md](physical_storage_plan.md).
- Colonist detail modal can equip gear from the colonist's own bag as well as
  storage (`[Equip]` on bag rows in `internal/gui/hud_screen.go`).
- Resource deposits hold a quantity that takes a while to mine out
  (`internal/world/resource_amount.go`: ore 25-100, crystal 15-60,
  radioactive 10-40, rubble 3-6).
- "Ancient" hunt scenario: `ancient_yeti` blueprint plus the deep-complex
  quest template in `data/quest_templates.json`.
- Resources glow: `ore_deposit` has `lightLevel`/`lightRange` in
  `data/tiledefinitions/resources.json`; documented in the player research
  guide.
- Task speed scales with worker stats (`internal/workerai/work_speed.go`).
- Worker robots: hauler, excavator, security, and construction chassis in
  `data/blueprints/entities/robots.json`, printed at the Robot Workbench.
