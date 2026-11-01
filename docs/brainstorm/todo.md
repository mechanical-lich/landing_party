# TODO

Status audited 2026-10-29 against `dev` (d3f4e56). Items confirmed shipped
were moved to the Done list; "Verify" items could not be confirmed from code
alone and need a play-test or a judgment call.

## Open

- Door sounds. `data/blueprints/structures/doors.json` has no `Sound`
  component and `data/audio.json` has no door entries.
- Tutorial planet: a guided first landing with non-blocking prompts that
  teach the basics, switchable off on the New Expedition screen. Planned
  2026-10-31, code-grounded. Plan: [tutorial_plan.md](tutorial_plan.md).
  Implemented 2026-11-01 on branch `claude/tutorial-planet` (ten commits,
  headless walkthrough of all 24 steps in `internal/game`); not yet merged.
  Still to do: one real playthrough for pacing and panel layout, since no
  headless test can judge how the prompts read. Watch the ration burn aboard
  the ship, how long hauling single mined units takes after the mining step,
  and the two needs/generation bugs under Verify, which the walkthrough found.
- Expanding enemy factions: two-tier model (abstract per-location presence
  on the campaign, materialized into worker-driven faction settlements on
  loaded levels), data-driven doctrines, greyish fleets met in space.
  Planned 2026-10-31. Plan: [enemy_factions_plan.md](enemy_factions_plan.md).

## Verify

- Colonists can starve beside food: a worker who passes out from exhaustion
  (no bed or bag in reach) stays in the passout task while hunger climbs past
  the starve threshold and health ticks down; the hunger interrupt and the
  idle food-seeking in `NeedsSystem` never run for them. Seen 2026-11-01 in
  the tutorial walkthrough at tick 18391 with three rations in the pad five
  tiles away (`internal/systems/NeedsSystem.go`, `assignRestTask`).
- Planet generation race: `PlanetPrimer.Prime` paints from several worker
  goroutines and each `SetFloor`/`SetMiddle` appends to the level's
  `DirtyColumns` (`rllayered.InvalidateSunColumn`) with no lock. About one
  generation in twenty panics (nil dereference or index out of range inside
  `append`). Seen 2026-11-01 in `internal/game` tests; fix is a per-worker
  collect or a mutex in `internal/generation/primer_planet.go`.

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
