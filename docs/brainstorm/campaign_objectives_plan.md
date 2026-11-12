# Campaign Objectives: Implementation Plan

Planned 2026-11-10. Not built. Written against `claude/factions-f-7be267`
(Phase F steps 1 to 5), which is not yet on `dev`; file and line references
are that branch's. Every name and line of player-facing text here is a
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
  run is won when it has no presence, ship or mothership left.

Constraints carried over: everything is data (`data/factions.json`); no
save version bump; no CI test job; the owner commits; tests, soaks and
vmtests run locally.

## Where the code is today

- **Colony win.** Hard-coded: a jump whose destination is `HomeKind` sets
  `c.Won` (`internal/game/world_manager.go:1294`). The end screen's title is
  the literal "You Made It Home" (`internal/game/campaign_end_state.go:51`).
  Loss is a total wipe.
- **Hive and autonz win.** `race.win` in `data/factions.json` has one field,
  `sites`, with a title and text (`RaceWin`, `internal/campaign/player_race.go:52`).
  `Campaign.SpreadOutcome` (`player_race.go:166`) checks it once a day
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

So two wins exist, each in its own code path, and neither is chosen.

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
- **`conquer`**: one rival faction is rolled at generation from those the
  run seeds (never the mirror colony unless it is the only one; owner's
  call, open question 3). Won when that faction has no presence on any
  location, no fleet, and no mothership. The rival's name fills `{rival}`.
  If its identity is hidden (the hive before discovery), the text uses its
  hidden name until discovered.
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

### State: `Campaign.Objective`

```go
type RunObjective struct {
    ID      string            // the doctrine objective's id
    Kind    string
    Rival   string            // conquer: the rolled faction key
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
  that does not exist yet. Open question 1 decides between rolling targets
  only among locations generated at the start, and reserving them for
  later systems.
- An unmarked target needs a way to be found. Default: each target's system
  gets a datapad clue on a neighbouring site naming it (the lore datapad
  path), and scanning the planet from orbit shows it. Open question 2.

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
- **Home under another objective.** In ship mode with a `collect` objective
  delivered to `hold`, or a `conquer` objective, Home is not the win. The
  Star Map still draws Home (it is part of the world), without the dashed
  goal line; jumping there before the objective is done is refused with a
  line saying why (the colony does not go home empty-handed). With
  `deliver: "home"` the dashed line appears once the items are aboard.
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
- A "colonize" objective for the colony distinct from conquer: the colony
  has no way to hold worlds as a spread race does. Open question 5.

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
3. **`conquer`** (about 400 lines) for the colony, hive and autonz. The
   rival roll, the remaining-sites count, the outline on the Star Map, the
   win when the rival is gone. Tests on a small campaign where the rival's
   last presence is removed by the player, by another faction, and by a
   ship leaving. The race-run soaks gain a column: days to conquer, runs
   that never do.
4. **Greyish objectives** (data only if 2 and 3 hold): return home, black
   boxes (`marked: true`, wreck sites), samples, conquer. Waits for the
   greyish build (`greyish_plan.md`) and Phase F step 6. This replaces that
   plan's open question 16 default (reach Home only).
5. **The hive's origin story**: its own plan.

Steps 1 to 3 are about 1,900 lines and do not need the greyish.

## Open questions (each with a recommended default)

1. **Targets on undiscovered systems.** Default: roll targets among the
   locations generated at the start plus a reserved slot on later systems
   by jump distance, so a find objective pulls the player outward.
   Alternative: only locations that exist at the start (simpler, all
   targets close).
2. **Finding an unmarked target.** Default: a clue datapad on a
   neighbouring site, and an orbital scan shows it. Alternative: no help;
   the player searches every site.
3. **Can the mirror colony be the conquer target?** Default: only when it
   is the only rival. Alternative: any rival, the mirror included.
4. **The conquer rival dies to someone else.** Default: it still counts;
   the win is the rival being gone. Alternative: the player must destroy a
   share of it (say, the last presence or the mothership).
5. **A colony "colonize" objective.** Default: not in this plan; the
   colony's three are return Home, archaeology and conquer. Alternative:
   hold outposts on N worlds, which needs colony outposts to count as held
   sites.
6. **How many targets, and how far.** Defaults: archaeology 1 relic at 4
   or more jumps; black boxes 4; samples 5. All data, tuned by play.
7. **Jumping Home early under another objective.** Default: refused with a
   line. Alternative: allowed, and it ends the run as a loss or a lesser
   ending.
8. **Does the hive keep "hold six worlds"?** Default: yes, as one of its
   objectives beside conquer (and later the origin story). Alternative:
   conquer replaces it.
9. **Autonz objectives.** The owner's note names none. Default: hold six
   worlds and conquer.

10. **Where an `outbound` colony starts.** Decided 2026-11-10: a named
    staging point (an empty point like the Wreck Site, renamed, the ship
    whole), with Home still far away. Spread races have no origin point
    (`seeded`), also decided. Still open within it: the `outbound` ship's
    starting fuel and stock against `adrift`'s (default: whole and
    fuelled, tuned by play). A there-and-back run starting at Home was the
    alternative not taken.

## Where the owner rewrites text

- Every objective's `name`, `brief`, and `win` title and text.
- The opening alert, the header line's wording per kind, the Home-refused
  line, the three Legends events.
- The `objective_relic`, `black_box` and `specimen_sample` item names and
  descriptions, and the clue datapad's text.

## Riskiest assumptions

- **Conquer is finishable.** Rivals spread faster than a colony fights
  (hive and autonz reach 6 to 9 sites by day 300). Wiping one out may be
  out of reach for the colony at any Rival factions setting. The soak
  column in step 3 is the check; the fallback is open question 4's
  alternative (destroy its seat, not every site).
- **A find objective is fun and not a search of every tile.** It depends on
  the clue and scan (open question 2) and on where the item is placed on
  the level.
- **Moving the Home win behind a registry changes nothing.** It touches the
  jump path, the tutorial and the observer expedition; step 1's tests are
  the net.
- **Random as the default** means a new player's first run may be conquer
  or archaeology and not the plain trip Home. The tutorial pins
  `reach_home`; a first non-tutorial run does not.

## As built
