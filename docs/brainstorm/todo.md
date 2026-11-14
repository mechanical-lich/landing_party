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
  hive (xenos + troids merged, two morphs, colour tiers, identity hidden
  until discovered) → mirror colony → greyish → autonz. Hive spreads via a
  research-ranged egg launcher; doctrine-driven player settlement so races
  become playable later. Planned 2026-10-31, updated 2026-11-05.
  Plan: [enemy_factions_plan.md](enemy_factions_plan.md). Phase A (campaign
  tier: presences, hive doctrine, daily step, Rival factions stepper, Star
  Map badges, Legends events) and Phase B (hive morphs and tiers at spawn,
  presences materialised on their level and reconciled on save/freeze, husks
  and wipes, eggs as fleets on the Star Map, live hives launching) and
  Phase C0 (the player's settlement reads its build menu, research tree,
  printer units, recipes and crew from the `colony` doctrine; the player's
  techs on its own presence, save version 4; `deploy_on_craft`) built
  2026-11-08 on `claude/factions-a-a379f7`, not yet merged, and Phase C1
  (the director on the player's settlement, observer mode, the soak) and
  Phase C2 (the director digs to ore below, faction settlements on loaded
  levels, the mirror colony on a ship, neutral until the player lands on its
  site, hostility seeding, `ConvertAction`) and Phase D (raids and
  hunting, ships with tiers and holds landing and lifting off live, the
  greyish and their mothership, interception and boarding, ruins on
  departure, Star Map ship tags) and Phase E (the autonz: planet-carried,
  converting, spreading by signals that found dormant sleeper nodes; Rival
  factions 4 brings them in) and Phase E2 (the hive as a settlement
  faction: unit types instead of colour tiers, drones laying eggs on larvae,
  resin creep, an evolve tree, the hive director, Tier 1 bookkeeping, the
  hive soak; [hive_plan.md](hive_plan.md)) and Phase E3 (the autonz as a
  settlement faction: unit types with steel and bronze print tiers, the
  assimilation tree, structures on wire, relay pylons deployed by drones,
  the transmit walk into the Signal Core, the autonz director, the parallel
  soak harness; [autonz_plan.md](autonz_plan.md)); Phase F (playable races,
  [playable_races_plan.md](playable_races_plan.md)) steps 1 to 6 are built
  (2026-11-13): the Race row on New Expedition, hive and autonz runs with no
  ship, View your worlds to give orders, the director in assist with
  rung-group toggles, the Spire and Signal Core aimed by hand, the
  doctrine's `interface` block, the world seen from the race's side; the
  greys as a ship race (a grey crew, linked lockers, a saucer grown in
  rings and decks). Rival selection
  ([rival_selection_plan.md](rival_selection_plan.md), built 2026-11-14):
  factions as instances of doctrines with groups, the mirror folded into
  the colony doctrine, and New Expedition's rival boxes (each rival's
  doctrine or Random, and its group); then B+ (hidden identity: `LoreLinks`, per-morph
  naming, the types' names). Doc:
  [factions.md](../developer/factions.md).
- Start camps can open onto rock. Seed 2's start (a 7x7 bunker room from a
  scenario setup script) was laid against an outcrop: its airlock (the
  colony's own) opens onto solid rock, so the crew is sealed in. Found by the
  observer soak, 2026-11-08. The camp scripts need a cleared approach outside
  the airlock (or the site picker should keep the camp's door side open).
- Rival colonists use the player's sprites (phase C2).
- Phase D leftovers (factions): boarded decks stay on the Star Map as
  destinations. (A ship crew dying on a loaded level and boarded mirror crew
  with no AI were closed in phase E; the greys' live research and their
  scout's single soldier were taken up by phase E4.)
- Phase E4 leftovers (greyish, branch claude/greyish-build-571417; see
  greyish_plan.md "As built"): mining metal is scarce on today's maps (ore
  2-5 levels down and far off); the mothership's beam-down carries the
  camps (owner's call, 2026-11-12); beamed stock does not count toward the
  hold target; research takes plastic pressed from biomass (scouts learn
  1-2 entries a stay): balance questions in "As built". The grey player,
  the growing saucer and the player's linked lockers are Phase F step 6
  (playable_races_plan.md "As built"). Not built: research progress
  carried across a lift-off; research-modal effect lines. Checked in Go
  tests only, not played: lancers posted at emitters, a heavy-led raid,
  colonists smashing every grey locker.
- Phase E leftovers (autonz): a stage reached on a loaded level is stamped
  once a day by the live rule, not the moment it is reached; the autonz
  research walk is abstract (no lab on the level), like the greys'; the
  start-system soak sites are poor (capacity 4 to 7 at a third converted),
  so the crew barely grows in 20,000 ticks, and seeds 2 and 7 never find ore
  within the dig reach; autonz spread is 1 to 2 sites at day 100 and 6 to 9
  by day 300 at Rival factions 4 (owner's call on pace); the player's
  ConvertAction stays gated behind a `terraforming` tech no entry grants; the
  boarded-mirror fight and an autonz raid were checked in Go tests only, not
  played in the VM.
- Phase E3 leftovers (autonz): the hive's crew bodies lack `NeverSleep`, so
  every non-colonist worker stops at night (the autonz got it in E3; the
  hive's evolve stall in E2 may be the same night stop, owner's call); the
  harvester's "repairs" (no repair task), a pylon seeing the colony as
  contact, and mine orders preferring harvesters were not built; materialise
  places structures with the director's grid rule but not its reachability
  check, so positions agree with a live build only where the first free
  wired site is the same; a hostile autonz raid led by a warden and the
  transmit walk ending in a live signal were checked in Go tests, not
  played in the VM; balance questions in autonz_plan.md "As built" (live
  conversion well above Phase E's now that the crew works nights and wires
  at its cap; research is slow live: two or three entries in forty days, no
  bronze yet at a hub started with the relay and foundry patterns; a node's
  relay pylon comes 6 to 10 days after its pattern, not within 2).
- Observer mode v2 leftovers: take over the expedition mid-run (an
  observer run cannot be switched off yet); the expedition never scans for
  new systems; a fleet does not refuel while its site is loaded; crew keep
  the food they carry rather than sharing it with a starving crewmate
  (worker AI); the owner's stray beam (three colonists on a planet the ship
  had left) was not reproduced (beams pinned to the planet in orbit by a
  test).
- `protein_press` (biomass into a protein bar at the workbench) joined the
  player's recipes with phase D's food rung: the game had no other renewable
  food. Owner's call whether the player keeps it.

- Phase F leftovers (playable races; playable_races_plan.md "As built"):
  a grey run's ring is slow (8 to 10 took some 70 days with the start's two
  builders: a haul from the hold per floor tile); the greys' start, texts
  and hull tiles are placeholders; task stops wait five wall-clock seconds (`task.Stop`), so a headless run
  replays them differently from a live one (the soaks use
  `deterministicTaskStops`); the level's opening
  message is the colony's ("Welcome to your new home. Establish a
  perimeter."); the Star Map still draws the ship's range circle and its
  marker at the Wreck Site in a race run; the rival colony met the brood on
  no soak seed (its raids on a hive are tested in Go and the vmtest, not
  measured); the GameMaster counts a player's Resin Spire (a scripted
  structure) toward `hostile_max`; the soaks vary a little run to run, as the
  hive soak does; balance questions for the owner in the plan's "As built"
  (an autonz node of six signals on day 2; no hive run wins by day 150 at
  Rival factions 1 but two of ten do at 2; the hive's spire comes earlier in
  a race run than in E2's soak).

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

- Observer mode / self-play, 2026-11-08, on `claude/factions-a-a379f7` (not
  yet merged; phase C1 of the factions plan): the director runs the
  player's settlement under config `observerMode`, settlement stats in the
  vmtest sidecar, the `observer` script and `make soak` (ten seeds, written
  thresholds). Found and fixed: sized builds ate their materials when a
  worker stood in the footprint. Found then: the director could not reach
  ore below the surface (it digs since phase C2) and seed 2's start camp is
  sealed (its airlock opens onto rock; open, above). See the plan's "Phase
  C1 as built" and "Phase C2 as built".
- Observer mode v2, 2026-11-08, on `claude/factions-a-a379f7` (not yet
  merged): an observer run (observer mode on at New Expedition) has no
  player ship; the expedition is a colony-doctrine fleet flown by the
  expedition table, like the mirror's ship now is, its colony run by the
  settlement director when viewed; the Star Map's View opens any planet;
  `observer on|off` is the ground lock on a normal campaign; the run's
  viewer sees every faction (a view-only reveal), and a Factions tab
  (both modes, each with its own fog) lists what every faction holds. Tests no
  longer read `data/config.local.json`. Fixed: `TestMirrorFedFortyDays`
  starved crew because the food haul did not reach the hunt's kills. See
  the plan's "Observer mode v2 as built".
- Bio-printer fixed, 2026-11-08, on `claude/factions-a-a379f7` (not yet
  merged): the crafting modal queued a recipe's output as its ID, so
  printing a colonist (`print_colonist`, output `colonist`) never queued;
  it sends the recipe key now. The Bio-Printer is in the Structures build
  menu (it had no category, so it could not be placed). It needs no
  research. See the plan's "Phase C0 as built".
- Abandoned Station rebuilt from its history, 2026-11-07, on
  `claude/station-5b3f90` (not yet merged): stations built from modules on
  a spine, ring or cross frame, purposes from the founding event, origins
  in their modules on their decks, decay from the ending, a dead generator
  to repair, powered lights and doors, a keycard-sealed archive, the station
  log. Plan and as-built notes: [station_plan.md](station_plan.md).

- Fixed 2026-11-07 on `claude/todo-fixes-9fd09a` (not yet merged):
  - The tutorial panel drops to its title row, below the modal, while any
    HUD modal is open (Esc menu, save, load, crafting, research, colonist),
    as it did for the storage inspector; it no longer covers the research
    and crafting modals' bottom rows. vmtest `tutorial.panel_covered`.
  - `find_nearest_worker(r)` and the other scripted-AI finders search r
    tiles in every direction, inclusive (rlworld's box was exclusive on its
    far side), and drop a cached target that has left r. The spore pod and
    old munition ask for radius 1 instead of searching 2 and checking
    distance.
  - City blocks and missile silos are stamped: an industrial or atomic
    people's golden-age stage may instead rebuild (`city_grows`,
    `silos_dug`). About 1.7% of sites keep a city block and 0.8% a missile
    silo; events per site unchanged. `history-dump -stats` gains a stamps by
    script table.

- Lore content plan, steps 1–8, implemented 2026-11-07 on
  `claude/lore-content-649632` (not yet merged): hazard tiers and calm
  bases, Star Map danger tags, landing reserve and lore budget, denser
  histories with layers, tech levels, the two-peoples war arc and variants,
  the five new-mechanic hazards, the 35-script structure library (with castle towns), and the
  previews (structure viewer presets and damage control; planet generator
  tier, budget readout, origin labels, landing reserve and keep-outs). Plan
  and implementation notes: [lore_content_plan.md](lore_content_plan.md).

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
