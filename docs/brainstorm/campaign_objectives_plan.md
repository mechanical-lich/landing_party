# Campaign Objectives: Implementation Plan

Planned 2026-11-10, revised 2026-11-14 against `dev` at 153293b, where
playable races (hive, autonz, greyish), the rebuilt greys, factions as
instances with groups and the rival boxes have all landed. Not built. File
and line references are `dev`'s. Every name and line of player-facing text here is a
draft for the owner to rewrite (`_draft` in data, as elsewhere).

## Decision

The owner's direction (2026-11-09 and 2026-11-10):

- **Each playable race has several campaign objectives.** One is chosen or
  rolled for a run. The owner's examples:
  - Colony: return Home; colonize or conquer; archaeology (find an item
    seeded on a planet).
  - Hive: conquer (spread, destroy rivals); find its origin (the hive does
    not know where it started; a storyline toward its true purpose).
  - Greyish: return home; recover wreckage (markers show which planets hold
    UFO wrecks; visit each for its black box); conquer; find samples (N
    samples seeded on planets).
- **The Objective row defaults to Random.** New Expedition gets an Objective
  row under Race: Random first, then that race's objectives by name. This
  is a deliberate title-screen choice like the race pick, the second
  exception to "scenarios are random". Where the targets sit is always
  rolled.
- **One objective per run.** Only the chosen or rolled objective ends the
  run in a win. The race's other objectives play no part in that run.
- **A colony find objective ends at Home.** The find adds a required stop:
  recover the item, then carry it Home. Home stays the colony's ending.
- **Conquer means wiping out one rival faction.** One rival is rolled; the
  run is won when it has no presence, ship or mothership left. Since
  2026-11-14 a rival is a faction instance with a group
  (`rival_selection_plan.md`), so the target is one instance in a group
  other than the player's: one hive of two, never an ally.

Decided 2026-11-14 (the open questions, gone through with the owner):

- **Targets** sit on the starting map and on systems discovered later.
- **An unmarked target** is found by a clue datapad on a neighbouring site
  and shows on an orbital scan.
- **The conquer target is rolled.** If another faction destroys it, that is
  a lose condition: the player is offered the remaining rivals outside
  their group and picks a new target; with none left the run is lost.
- **The colony gets a colonize objective**: hold outposts on N worlds. A
  world is held while at least one living colonist is there and its core
  structure stands.
- **Home is on the map only when the objective uses it.** On a run whose
  objective is not about Home (conquer, colonize), Home does not show on
  the Star Map and cannot be jumped to.
- **The hive keeps "hold six worlds"** beside conquer; **the autonz** have
  the same two.
- **A grey sample** is its own item, one per target world.
- **Home for a grey player is a mothership.**

Constraints carried over: everything is data (`data/factions.json`); no
save version bump; no CI test job; the owner commits; tests, soaks and
vmtests run locally.

## Where the code is today

- **Colony and greyish win.** Hard-coded: a jump whose destination is
  `HomeKind` sets `c.Won` (`internal/game/world_manager.go:1325`). A grey
  run is a ship run and wins the same way (`data/factions.json`, the
  greyish `race` block: "wins by reaching Home as the colony does"). The end screen's title is
  the literal "You Made It Home" (`internal/game/campaign_end_state.go:51`).
  Loss is a total wipe.
- **Hive and autonz win.** `race.win` in `data/factions.json` has one field,
  `sites`, with a title and text (`RaceWin`, `internal/campaign/player_race.go:66`).
  `Campaign.SpreadOutcome` (`player_race.go:180`) checks it once a day
  through `WorldManager.checkSpreadEnd` (`internal/game/race_run.go:68`):
  won holding awake presences on `sites` systems at once, lost with no
  presence and nothing in flight.
- **The header line.** `raceLine` (`race_run.go:120`) prints worlds held
  against the win for a spread run.
- **The title screen.** The Race row passes `GenOptions.Doctrine`
  (`internal/game/title_state.go:300`, `internal/campaign/faction.go:761`).
- **Quests.** `internal/objective` is a rule engine with ten triggers; it
  says of itself that it has no notion of winning. Quests bind a rule to a
  location, chain by `requires_quest`, can be delivered by datapad, and tag
  a target entity with `QuestTargetComponent`
  (`internal/game/world_manager.go:1225`, `setup_script.go:353`). Quests are
  off in a race run.

- **Factions and groups.** A campaign has a faction table
  (`internal/campaign/faction_table.go`): each faction has a key, a
  doctrine, a name, a group and an ordinal among its doctrine's factions.
  `Campaign.Hostile(a, b)` is true when two factions' groups differ. The
  player starts in group 1 and rivals from 2 (`GenOptions.Rivals`,
  `GenOptions.PlayerGroup`). The `mirror` doctrine is gone: a rival colony
  is the `colony` doctrine run by another faction.
- **Grey content an objective can use.** A departed grey ship leaves a
  landing scar with wrecks (`grey_armory_wreck`, `grey_array_wreck`,
  `grey_emitter_wreck`, `data/blueprints/entities/greyish_structures.json`);
  `specimen` and `grey_alloy` are items
  (`data/blueprints/items/resources.json`); a grey player starts with a
  saucer, three crew, a vat and a console, with no mothership of its own.

So two wins exist (reach Home, hold six worlds), each in its own code
path, and neither is chosen.

## Design

### Data: `race.objectives`

`race.win` becomes a list. Each entry:

```json
"objectives": [
  { "id": "spread",  "kind": "hold_sites", "sites": 6,
    "name": "The Swarm Spreads", "brief": "Hold six worlds at once.",
    "win": { "title": "The Swarm Spreads", "text": "Your brood holds six worlds." } },
  { "id": "conquer", "kind": "conquer", "min_factions": 1,
    "name": "Extinction", "brief": "Destroy every trace of {rival}.",
    "win": { "title": "Extinction", "text": "Nothing of {rival} remains." } }
]
```

- `weight` (default 1) for the Random roll.
- `min_factions`: the objective is offered, and can be rolled, only at this
  Rival factions setting or above (conquer needs a rival to exist).
- `brief` is the one line shown on the title row, the dashboard header and
  the run's opening message.
- A doctrine with a `race.win` and no `objectives` loads as a single
  `hold_sites` objective, so nothing breaks while the data moves.
- The colony gets a `race` block for the first time, holding only
  `objectives` (its start stays the ship).

### Kinds: a Go registry

A kind is Go code registered by name, the same middle path as the
`interface` block: data picks and tunes, Go supplies the mechanism. Each
kind implements four things:

| Hook | When | What |
|---|---|---|
| `Seed` | campaign generation | rolls targets, writes them into `Campaign.Objective` |
| `Progress` | header line, Star Map | "2 of 3 recovered", which sites are marked |
| `Done` | once a day, and on arrival at Home | whether the win condition holds |
| `Valid` | title screen and the roll | whether this run's options allow it |

The four kinds of the first version:

- **`reach_home`**: travel to Home. Today's colony win, moved behind the
  registry. For the greyish in ship mode too.
- **`hold_sites`**: awake presences on `sites` systems at once. Today's
  spread win, moved.
- **`hold_outposts`** (the colony's colonize): outposts on `sites` worlds
  at once. A world counts while at least one living colonist of the
  player's is on it and its **core structure** stands: a new buildable, the
  Outpost Beacon (draft name; `core` on the objective names the blueprint,
  so data can point it at an existing structure instead). The beacon is
  ordinary construction with a real cost, one per world. Losing the crew
  or the beacon drops the world from the count; the run is not lost by
  that alone. The header reads "Outposts 2 of 4".
  - **The cost to watch:** a world with colonists on it stays loaded
    (`isStaffed`, `internal/game/world_manager.go:203`), so N outposts are N
    live levels at once. `sites` defaults to 4 and the build measures frame
    and tick time with four staffed levels before settling it (riskiest
    assumptions).
  - An outpost's colonists live by the game's ordinary rules (food, raids);
    nothing new keeps them alive.
- **`conquer`**: one rival faction instance is rolled at generation from
  those the run seeds whose group differs from the player's
  (`Campaign.Hostile(player, rival)`). It is offered only when at least one
  such rival exists, so the rival boxes are resolved before the objective
  is rolled. A rival colony can be the target like any other. Won when that faction has no presence on any
  location, no fleet, and no mothership. Other factions of the same
  doctrine do not count: wiping out one hive of two wins. The faction's
  name fills `{rival}` (its instance name; for a hive whose identity is
  still hidden, its hidden name and ordinal, as the Star Map shows it).
  - **The target destroyed by someone else is a lose condition (decided).**
    Credit is tracked: the objective is won only if the player's side
    removed the target's last presence, ship or mothership (the killing
    blow on its last body on a level, the boarding that took its last ship
    or mothership). If its last holding goes any other way (another
    faction's raid, starvation, a ship lost unseen), the clock stops and a
    prompt lists the remaining rival instances outside the player's group:
    the player picks a new target and the run goes on. With none left, the
    run ends as lost, with its own end-screen words.
  - Credit needs a rule for the abstract step, where nobody "kills"
    anything: a rival's presence that dies on a world the player is not
    viewing was not destroyed by the player. So conquer is won on loaded
    levels and in boarding only. Stated plainly in the objective's brief.
- **`collect`**: recover `count` target items. One kind covers three of the
  owner's objectives through its parameters:

  | Parameter | Archaeology (colony) | Black boxes (greyish) | Samples (greyish) |
  |---|---|---|---|
  | `item` | `objective_relic` | `black_box` | `specimen_sample` |
  | `count` | 1 | 3 to 5 | N |
  | `marked` | false (a clue chain leads to it) | true (Star Map markers) | false |
  | `site` | a ruin or a history site | a wreck stamp | any site with life |
  | `deliver` | `home` | `home` or `hold` | `hold` |

  `deliver` says where the item must end up: `hold` is the player's storage
  (the ship's hold in ship mode, any of the player's containers in a spread
  run); `home` adds "then reach Home with them aboard". A colony find
  objective is therefore `collect` with `deliver: "home"`, and `Done` is
  checked in the same place the Home jump is today.

**Not in the first version: the hive's "find origin".** It is a story, a
chain of steps with text between them, not a single condition. It gets its
own plan (step 5), most likely built on the quest engine's prerequisite
chains and datapad delivery, which are off in race runs today.

### The greyish

The greys are playable, so their objectives ship with the others. The
owner's four:

| Objective | Kind | Start | Notes |
|---|---|---|---|
| Return home | `reach_home` | `adrift`-like (see below) | today's grey run, unchanged |
| Recover wreckage | `collect`, `marked: true` | `outbound` | black boxes at grey wreck sites |
| Find samples | `collect`, `marked: false` | `outbound` | samples on worlds with life |
| Conquer | `conquer` | `outbound` | as for every race |

- **The grey start.** The grey blurb already says "far from the
  mothership", and a grey player has no mothership. The return-home run
  keeps that premise: a start of its own, `stranded`, which is `adrift`
  with grey words (the origin is not called the Wreck Site, and the saucer
  starts whole). The other three are `outbound`: a scout sent out on an
  errand.
- **Home for a grey player is a mothership (decided).** The far-off Home
  location of a grey run is the player's mothership: drawn with the
  mothership marker, named as one, and reaching it wins. It is the Home
  location presented differently (a doctrine `home` block: name, marker,
  end-screen words), not a faction mothership: it has no hoard, builds no
  ships and cannot be boarded, and rival greys' motherships are unrelated
  to it. In the three `outbound` grey objectives it is hidden like any
  Home the objective does not use, unless `deliver: "home"` is chosen for
  a find.
- **Grey wreck sites.** A black box needs a place. Each target world gets a
  **crashed saucer**: the existing saucer hull stamp, broken, with grey
  wrecks around it and the `black_box` item inside. This is new seeded
  content, not a rival's landing scar: a scar belongs to a living rival and
  appears wherever one happened to land, so it cannot carry a rolled
  target. The Star Map marks each wreck world from the start (`marked:
  true`), as the owner described.
- **Samples are not specimens.** `specimen` is a crafting resource dropped
  by wildlife kills; counting those would make the objective a grind on
  one world. A sample is its own item (`objective_sample`), one per target
  world, taken from a marked life form or growth there, so the objective
  sends the saucer to N different worlds (decided).
- **Delivery.** `deliver: "hold"` for both finds: aboard the saucer's
  pocket. The owner may prefer `home` (carry them back), as the colony's
  archaeology does; data either way.

### State: `Campaign.Objective`

```go
type RunObjective struct {
    ID      string            // the doctrine objective's id
    Kind    string
    Rival   string            // conquer: the rolled faction instance's key
    Targets []ObjectiveTarget // collect: one per item
}
type ObjectiveTarget struct {
    LocationID string
    Found      bool // picked up on its site
    Delivered  bool // in the hold
}
```

Additive and `omitempty`: an old save has none and falls back to the
race's first objective (`reach_home` for the colony, `hold_sites` for a
spread race), which is exactly today's behaviour. No version bump.

### Seeding targets

- Targets are rolled at generation from the campaign seed, so a seed and a
  set of title options always give the same run.
- A target is a location id only until its level is generated. On
  generation the item is placed the way a quest target is today (a feature
  or setup script tags an entity; `world_manager.go:1225`), with a new
  `ObjectiveTargetComponent` in place of `QuestTargetComponent`.
- Placement rules are data on the objective: a minimum jump distance from
  the start, never the start system, never Home or the Wreck Site, at most
  one target per location.
- Systems are discovered by scanning, so a target may sit on a location
  that does not exist yet. Decided: some targets are rolled among the
  locations generated at the start and the rest are reserved by jump
  distance, each placed on the first newly discovered system that
  satisfies its rule.
- An unmarked target needs a way to be found. Default: each target's system
  gets a datapad clue on a neighbouring site naming it (the lore datapad
  path), and scanning the planet from orbit shows it (decided).

### Starting points (owner, 2026-11-10)

The owner's point: the Wreck Site is the premise of one story only (a
damaged ship adrift, trying to get Home). A colony that set out to search
for something did not begin at a wreck, and a hive has no wreck site at
all. So **the start is part of the objective's premise**, not a fixed fact
of every campaign.

Today the origin is hard-coded: generation always creates the `adrift`
location named "Wreck Site" at the origin (`internal/campaign/generate.go:174`),
the Star Map draws it as a dim cross (`starmap_panel.go:1058`), the first
alert and the tutorial name it, the safe-start radius is measured from it
(`genconfig.go:65`), and Phase F left the ship's marker and range circle
drawn there in a race run (a listed leftover).

Each objective names a **`start`**, one of a small set of premises
registered in Go like the kinds, with its words in data:

| `start` | Origin on the map | For |
|---|---|---|
| `adrift` | today's Wreck Site: an empty point, the ship damaged and adrift, Home far off | colony `reach_home` (the game as it is; the tutorial) |
| `outbound` | a named staging point (decided): an empty point like the Wreck Site, renamed, the ship whole and fuelled, Home still far off; map generation unchanged | colony archaeology and conquer; greyish finds |
| `stranded` | `adrift` for the greys: the same empty origin point under a grey name, the saucer whole, Home far off | greyish `reach_home` |
| `seeded` | no origin point at all: no adrift location, no ship marker, no range circle; the map opens on the world the brood or node starts on | every spread race (hive, autonz) |

- `seeded` is not new behaviour so much as finishing Phase F's leftover:
  generation skips the adrift location in a spread run, and the safe-start
  radius is measured from the start world. It also suits the hive's origin
  story: the hive's map has no "where we came from" on it.
- The origin's name, its Star Map marker and the run's opening alert come
  from the start's data, so `outbound` does not say "Wreck Site" anywhere.
- Whether the ship starts damaged or short of fuel is the start's data too
  (`adrift` keeps today's numbers; `outbound` may start whole and fuelled,
  since nothing went wrong yet).
- The tutorial stays `adrift` with `reach_home`.
- Saves: an old save has the adrift location and no `start`; it reads as
  `adrift`.

### Checking, and the end

- One function, `Campaign.ObjectiveOutcome`, replaces the win half of
  `SpreadOutcome` and the `HomeKind` branch. It runs once a day and on
  arrival at Home.
- **Loss does not change**: a total wipe in ship mode; no presence and
  nothing in flight in a spread run. A lost conquer target cannot happen
  (the rival being wiped out by someone else still wins it; open question
  4). A collect item cannot be destroyed (the item has no health and
  survives its container).
- **Home under another objective (decided).** When the objective does not
  use Home (`conquer`, `hold_outposts`, a `collect` delivered to `hold`),
  Home does not show on the Star Map and is not a jump destination. The
  location is still generated, hidden, so the map's shape (which is pulled
  toward Home, `HomeBias`) and every seed's systems stay as they are; it
  is skipped by drawing, selection, framing, quests and the tutorial
  highlight. When the objective uses Home (`reach_home`, a `collect` with
  `deliver: "home"`), Home shows as today; with `deliver: "home"` the
  dashed goal line appears once the items are aboard, and a jump Home
  before that is refused with a line.
- The end screen takes the objective's `win` title and text. The colony's
  literal "You Made It Home" moves into the colony's `reach_home` entry.

### What the player sees

- **Title screen.** An Objective row under Race: Random, then the race's
  valid objectives by `name`, with the `brief` under the row. Changing Race
  or the Rival factions stepper rebuilds the list (conquer disappears at
  Rival factions 0). Not saved to config. The tutorial forces the colony's
  `reach_home`.
- **Opening.** The run's first alert states the objective (and, for
  conquer, the rival).
- **Dashboard header.** `raceLine` becomes `objectiveLine` for every mode:
  "Hold 3 of 6 worlds", "Black boxes 1 of 4", "Destroy the Autonz: 5 sites
  remain".
- **Star Map.** Marked targets get a marker; a found-but-not-delivered item
  shows on the site it lies on. The conquer rival's badges are outlined.
- **Legends.** One event when the objective is set, one per target found,
  one at the end.

### Out of scope for the first version

- Rivals pursuing objectives of their own.
- Objectives in observer runs (an observer run keeps today's expedition
  table, which flies toward Home).
- Secondary or optional objectives, rewards for them.

## Order of work

1. **Framework** (about 600 lines). `race.objectives`, the kind registry,
   `RunObjective`, `ObjectiveOutcome`; `reach_home` and `hold_sites` moved
   behind it with no behaviour change; the title row and Random roll; the
   header line; the end screen's words from data; the `start` premises
   (`adrift` as now, `seeded` for spread races with no adrift location,
   ship marker or range circle, `outbound` as a renamed origin with its own
   words and starting stock). Tests: every existing
   win test passes unchanged; a seed plus options rolls the same objective
   twice; an old save loads to its race's first objective. vmtest: the
   title row, a hive run's header.
2. **`collect`, shipping as colony archaeology** (about 900 lines). Target
   rolling and placement, `ObjectiveTargetComponent`, found and delivered
   tracking, the clue datapad, the Home jump refused until the item is
   aboard, Star Map marks. Tests: the item survives freeze and thaw and a
   save round-trip; an unreachable placement is re-rolled; the soak's
   observer colony is unaffected. vmtest: find the relic, beam it up, jump
   Home, the end screen.
3. **`conquer`** (about 400 lines) for all four races. The roll among
   rival instances outside the player's group, the remaining-sites count,
   the outline on the Star Map, the win when that faction is gone. Tests on
   a small campaign where the rival's last presence is removed by the
   player, by another faction, and by a ship leaving; with two factions of
   one doctrine, only the target counts; with every rival in the player's
   group, conquer is not offered. The race-run soaks gain a column: days to
   conquer, runs that never do.
4. **Greyish finds** (about 500 lines): the crashed saucer site and
   `black_box`, the sample and its marked source, the `stranded` start,
   the four grey objectives as data. A vmtest of a black box recovered and
   the run won. This replaces `greyish_plan.md`'s open question 16 default
   (reach Home only).
5. **Colonize** (about 450 lines): `hold_outposts`, the Outpost Beacon
   blueprint and build entry, the held-world count, the header, the
   objective as data on the colony with the `outbound` start and Home
   hidden. A measurement of four staffed levels' tick and frame time. A
   vmtest of a second outpost counted and then lost.
6. **The hive's origin story**: its own plan.

Steps 1 to 5 are about 3,000 lines (conquer grows by about 150 for kill
credit and the new-target prompt). Nothing they need is still unbuilt.

## Open questions

Gone through with the owner on 2026-11-14; the answers are in Decision.
For the record, with the alternatives not taken:

1. Targets on undiscovered systems: **start map plus later systems**
   (not: only the starting map).
2. Finding an unmarked target: **clue datapad plus orbital scan** (not:
   scan only, or no help).
3. The conquer target: **rolled** (not: picked on the title screen).
4. The target destroyed by someone else: **a lose condition; the player
   picks a new target from those left, and with none left the run is
   lost** (not: it still counts).
5. A colony colonize objective: **yes, hold N outposts; a world counts
   with a living colonist and its core structure** (not: any colonist; a
   population threshold).
7. Home under another objective: **not on the map and not a destination**
   (not: refused with a line; allowed as a loss or lesser ending).
8. The hive keeps hold six worlds: **yes**.
9. Autonz objectives: **hold six worlds and conquer** (not: a third,
   full conversion).
10. An `outbound` colony's start: **a named staging point**; spread races
    have none.
11. Grey samples: **a distinct item, one per world** (not: specimens).
12. Home for the greys: **a mothership** (not: the colony's Home renamed).

Still open, all tuned by play or small:

6. **How many targets, and how far.** Defaults: archaeology 1 relic at 4
   or more jumps; black boxes 4; samples 5; outposts 4. All data.
13. **The `outbound` ship's starting fuel and stock** against `adrift`'s.
    Default: whole and fuelled.
14. **Does the new-target prompt cost anything?** Default: no. Alternative:
    a penalty (days, or the new target starts stronger).
15. **What kills count as the player's on a conquer run** when a friendly
    rival (same group) lands the last blow. Default: not the player's; it
    triggers the new-target prompt like any other faction's kill.

## Where the owner rewrites text

- Every objective's `name`, `brief`, and `win` title and text.
- The opening alert, the header line's wording per kind, the Home-refused
  line, the three Legends events.
- The `objective_relic`, `black_box` and `specimen_sample` item names and
  descriptions, and the clue datapad's text.

## Riskiest assumptions

- **Four staffed levels at once.** Colonize keeps N worlds loaded, and
  the game has run with one or two. If four is too slow the objective's
  count drops, or an outpost needs a way to count while frozen (a real
  change: a colony presence carried on the world as a rival colony's is).
- **Kill credit is traceable.** Conquer now depends on knowing who removed
  a faction's last holding. Presences end in several places (a level wipe,
  reconcile on freeze, the daily step, boarding, a ship lost in flight);
  each needs to say who did it, and a wrong attribution turns a win into
  the new-target prompt.
- **Conquer is finishable.** Rivals spread faster than a colony fights
  (hive and autonz reach 6 to 9 sites by day 300). Wiping one out may be
  out of reach for the colony at any Rival factions setting. The soak
  column in step 3 is the check; the fallback is a narrower win (destroy
  its seat or mothership, not every site). Kill credit makes it harder
  still: only the player's own kills count.
- **A find objective is fun and not a search of every tile.** It depends on
  the clue datapad and the orbital scan and on where the item is placed on
  the level.
- **Moving the Home win behind a registry changes nothing.** It touches the
  jump path, the tutorial and the observer expedition; step 1's tests are
  the net.
- **Random as the default** means a new player's first run may be conquer
  or archaeology and not the plain trip Home. The tutorial pins
  `reach_home`; a first non-tutorial run does not.

## As built
