# TODO

Status audited 2026-10-29 against `dev` (d3f4e56). Items confirmed shipped
were moved to the Done list; "Verify" items could not be confirmed from code
alone and need a play-test or a judgment call.

## Open

- Door sounds. `data/blueprints/structures/doors.json` has no `Sound`
  component and `data/audio.json` has no door entries.
- Unified storage (lorewise a hyper-dimensional storage network where every
  chest is a portal into one pool). **Partial:** colonists already walk to
  the *closest container that holds the item* (`FindClosestStorageWith` in
  `internal/workerai/storage.go`) and containers have tag filters, but each
  container still holds its own stock; any chest does not expose everything.

## Verify

- Fix the resource tracker. No tracker-named code found; confirm which HUD
  readout this meant and whether it still misbehaves.
- Balance all building resource costs.
- Fix gear items to better match what they are.
- Replace the notify and research sounds. Both `notify.ogg` and
  `research_done.ogg` were (re)added on 2026-07-17; decide if those are the
  final ones.

## Done

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
