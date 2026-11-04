# TODO

Status audited 2026-10-29 against `dev` (d3f4e56). Items confirmed shipped
were moved to the Done list; "Verify" items could not be confirmed from code
alone and need a play-test or a judgment call.

## Open

- UI defects caught by the VM play-test framework on 2026-11-03 (frames in
  the shared notes folder under `vmtest-report/catches/`); fix with
  `make vmtest` before/after runs:
  1. Star Map scenario description text overflows onto the Scan and Pause
     buttons (`starmap_panel.go`).
  2. The tutorial panel's top-right button draws as an empty square
     (`tutorial_panel.go`).
  3. minui modals draw without their title or child backgrounds: incident
     popup, storage inspector, Esc menu, crafting and research modals (which
     also show no close X). Check whether this came with the Ebiten 2.10 /
     mlge v1.14 bump; `mlge/ui/minui/modal.go` still draws `Title`.
  4. The Esc menu truncates "New Game" to "New Ga…".
  5. The tutorial highlight draws on top of an open crafting modal.
  6. Research and crafting cost lines print in Go map order
     (`hud_screen.go` ~1196 and ~1286), so rows shuffle between identical
     runs; sort them.
  7. The tutorial panel hides the "PAUSED - incident" marker.
  8. The font has no arrow glyphs: "← Back" renders as "Back".
  9. `main_state.go` ~756 checks `ModalOpen("colonistModal")`, which is
     never true, so HUD input is not blocked while the colonist panel is
     open (read from code, not seen on screen).
- Door sounds. `data/blueprints/structures/doors.json` has no `Sound`
  component and `data/audio.json` has no door entries.
- Tutorial planet: a guided first landing with non-blocking prompts that
  teach the basics, switchable off on the New Expedition screen. Planned
  2026-10-31, code-grounded. Plan: [tutorial_plan.md](tutorial_plan.md).
  Implemented 2026-11-01 on branch `claude/tutorial-planet` (ten commits,
  headless walkthrough of all 24 steps in `internal/game`); not yet merged.
  Still to do: one real playthrough for pacing and panel layout, since no
  headless test can judge how the prompts read. Watch the ration burn aboard
  the ship and how long hauling single mined units takes after the mining step.
- Expanding enemy factions: two-tier model (abstract per-location presence
  on the campaign, materialized into worker-driven faction settlements on
  loaded levels), data-driven doctrines, greyish fleets met in space.
  Planned 2026-10-31. Plan: [enemy_factions_plan.md](enemy_factions_plan.md).

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
