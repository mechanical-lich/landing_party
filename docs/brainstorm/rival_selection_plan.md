# Rival Factions as Instances, and Choosing Each One: Implementation Plan

Planned 2026-11-12, widened the same day. Built on this branch
(2026-11-13 and 14; see "As built"). Written against
`claude/factions-f-7be267` (Phase F, rebased onto `dev` at b131df6, the
rebuilt greys), because the title screen it changes has that branch's Race
row and the player-as-a-faction code this plan generalises. Every label
here is a draft for the owner to rewrite.

## Decision

The owner's direction (2026-11-12), in three parts.

**1. Choose each rival.** New Expedition keeps the Rival factions number.
Under it the screen shows that many select boxes, one per rival; each
picks which faction that rival is; each defaults to "Random".

- Random is a real roll: any seedable doctrine, from the campaign seed.
- In a race run a rival may be the player's own race, as a hostile rival.

**2. The colonists are one race.** The rival colony stops being a separate
doctrine (`mirror`). A rival colony is the `colony` doctrine run by another
faction, exactly as a rival hive is the `hive` doctrine when the player
plays the hive. The player's colony plays as it does today.

**3. A rival is a faction of its own, not its doctrine.** There can be two
rival hives or two rival grey factions in one campaign. Each is a separate
faction, and each faction's director has context only for the faction it
was assigned to.

**4. Groups decide who fights whom.** Each rival box also has a group
number. Rivals with the same group number are friendly to each other;
rivals in different groups are hostile to each other, regardless of race.

Part 3 replaces the earlier answer that the same faction picked twice is
one faction with two starts: picked twice, it is two factions.

This is a deliberate title-screen choice, like the Race row: an exception
to "scenarios are random" that the owner asked for. Where each rival
starts is still rolled.

## Where the code is today

### Seeding

- **The stepper** is one number, 0 to `factions_max` (4), default 1
  (`data/generation.json`; `internal/game/title_state.go:214-229`, drawn at
  `:454`), passed as `GenOptions.Factions`.
- **Which factions you get is fixed by the number.** `Campaign.seedFactions`
  (`internal/campaign/faction.go:835`) gives, in a colony run: 1 = hive;
  2 = rival colony, hive; 3 = rival colony, greyish, hive; 4 = rival
  colony, greyish, autonz, hive (each doctrine's `seed_from`). Only the
  sites are rolled.
- **A race run** seeds through `seedRaceFactions`
  (`internal/campaign/player_race.go:373`): the rival colony first, the
  player's own doctrine left out, the rest by `seed_from`.

### Faction and doctrine are already two fields, used as one

- `FactionPresence` has both `Faction` ("display key") and `Doctrine` ("key
  into data/factions.json") (`faction.go:35-36`). Seeding sets them to the
  same string (`newPresence(key, key, ...)`).
- The exceptions prove the split works: the player's people in a race run
  are faction `player` with doctrine `hive` or `autonz`
  (`player_race.go:21`), and the observer run's expedition is faction
  `expedition` with doctrine `colony` (`observer.go:17`).
- On a level, a body's faction is stamped from its presence when it is
  materialised (`internal/game/faction_settlement.go:152`,
  `faction_hive.go:222`), and a settlement is named `<faction>:<location>`
  (`faction_settlement.go:33`). So two factions of one doctrine can already
  stand on a level as different sides.
- But about fifty places look a doctrine up **by a faction key**
  (`Doctrine(x.Faction)` and its kin), and several things are held **one
  per key**: the stance toward the player (`Campaign.Diplomacy[faction]`,
  `faction_ship.go:493`), the mothership and its hoard
  (`Campaign.Mothership(faction)`, `faction_ships_d.go:445`), the hostility
  table rebuilt by looping over doctrine keys
  (`internal/game/faction_hostility.go:26`), and contact, which looks up
  `Doctrine(f)` for a faction `f` (`faction_hostility.go:~78`).

### The mirror

- `mirror` in `data/factions.json` is identity and Tier 1 numbers only
  (name, badge, `seed_from` 2, ship mobility, growth, capacity, one stage,
  three research entries, a kit, aggression) plus `settlement_from:
  "colony"` and `expedition_from: "colony"`.
- The `colony` doctrine **already carries the same Tier 1 numbers**, added
  for the observer run's expedition presence ("the mirror's numbers").
- `"mirror"` is a literal in three lines of non-test Go
  (`player_race.go:391-396`) and one setup script function
  (`internal/game/setup_mirror.go`), and in about fifty test lines and
  three vmtest scripts.

So parts 2 and 3 are the same change seen twice: stop treating a faction
key as a doctrine key.

## Design

### Faction instances

A campaign keeps a small table of its factions:

```go
type Faction struct {
    Key      string // opaque, unique in the campaign
    Doctrine string // key into data/factions.json
    Name     string // shown to the player (see "Names")
    Badge    [3]int // Star Map colour (the doctrine's, shifted per instance)
    Slot     int    // the rival box it came from; 0 for the player
    Group    int    // same group: friendly; different: hostile (see "Groups")
}
// Campaign.FactionTable map[string]*Faction, saved, additive.
```

- **One lookup.** `Campaign.DoctrineOf(factionKey)` replaces every
  `Doctrine(x.Faction)`. A presence keeps its own `Doctrine` field, which
  must agree; a test asserts it for every presence after seeding and after
  each daily step.
- **Keys are opaque.** Nothing may parse a doctrine out of a key. The rule
  for a new key is the doctrine key when no faction in the campaign has it
  yet, else `<doctrine>.2`, `<doctrine>.3`. The player keeps `player` in a
  race run and its current key in a colony run. Because keys are opaque,
  an old save's `mirror` faction keeps the key `mirror`; only its doctrine
  is rewritten to `colony` on load.
- **The player and the observer expedition become rows of the same
  table**, so "a rival of your own race" needs no special case: it is two
  rows with one doctrine.

### What "its own director with its own context" covers

Directors already run per settlement. What makes two factions of one
doctrine bleed into each other is state held per key or per doctrine. Each
of these moves to the faction instance, and each gets a test with two
factions of one doctrine:

| State | Today | After |
|---|---|---|
| Stance toward the player | `Diplomacy[key]` | per faction: landing on one rival colony's site turns that colony hostile, not every colony |
| Known techs | on each presence (rivals); a shared pool on the player | per faction; a faction's presences never read another's |
| Mothership, hoard, hull tier | one per key | one per faction; two grey factions are two motherships |
| Spread targets | "no presence of that faction there" | of that faction instance; a rival hive may launch at a world another hive holds |
| Contact flags, raid targets, hunt and guard posts | per settlement | unchanged, already scoped |
| Storage (the grey pocket) | per owner settlement | unchanged, already scoped |
| Hostility table for the player's colonists | loop over doctrine keys | loop over the faction table |
| Tier 1 research walk, growth, launch clock | per presence | unchanged |
| Legends events, Star Map badges, the Factions tab | named by doctrine or morph | named by faction, one row each |
| Hidden identity (the hive's) | per doctrine | per doctrine still: learning what one hive is names them all (open question 4) |

### Groups (decided 2026-11-12)

Each faction instance has a `Group` number, set from its rival box. **Same
group: friendly. Different group: hostile. Race plays no part**: a hive
and a grey faction in group 1 leave each other alone, and two hives in
groups 1 and 2 fight.

- **One relation function.** `Campaign.Hostile(a, b)` answers for any two
  faction keys: false when `a == b`, false when their groups match, true
  otherwise. Everything that asks "is that an enemy" between two rivals
  goes through it.
- **What exists today must be found first.** The hostility table
  (`internal/components/hostility.go`, `SetFactionHostile`) marks a faction
  hostile *for the player's colonists* only. How a rival's bodies decide
  to attack another rival's bodies (script hostility vars, the worker AI's
  target choice, the raid rung's target, creature scripts keyed on a baked
  faction) is not yet traced; the faction plan left "faction-vs-faction"
  unanswered. Step 1 traces it and routes each through `Hostile`. Expect
  the table to become a pair relation, not a per-faction flag.
- **What "friendly" means on a level.** Bodies of friendly factions do not
  attack each other, do not count each other as threats (the guard, raid
  and hunt rungs skip them), and do not take from each other's storage.
  They do not share storage, tech or orders: each still has its own
  director and its own context.
- **What it means between worlds.** A faction does not raid, launch at or
  signal a world held only by friendly factions as an attack; whether two
  friendly factions may *share* a world is open question 10. Hostile
  factions may target each other's worlds (a hive's egg can land on a
  rival hive's world; the two then fight on the level).
- **Tier 1.** Rivals do not fight in the abstract daily step today, and
  this plan does not add that: groups act where bodies meet, on a loaded
  level, and in target choice. The pace goldens therefore hold only if
  target choice does not change for the default lineup, which is what the
  default group numbers must guarantee (open question 3).
- **The player has a group too (decided).** The player starts as group 1
  and rivals start at 2, 3, 4, 5, each its own group, so by default every
  rival is hostile to every other and to the player, as now. The title
  screen shows the player's group beside the Race row and lets it change.
- **A rival in the player's group, first version (decided):**
  - The rival does nothing to the player and ignores them: no raids on the
    player's sites, no attacks on the player's bodies, the player's bodies
    not counted as threats, no contact turning it hostile.
  - The player's units do not attack it automatically (no self-started
    fights, guards and turrets hold fire).
  - Commands that would normally default to attacking it do not: clicking
    one of its bodies or structures does not offer attack as the default
    action. An explicit attack order is still possible; what it does to
    the relation is open question 11.
  - Nothing more: no shared storage, tech, vision or trade.
- **A rival in a different group from the player** is hostile by its
  doctrine's own rule, per faction: the hive from the start, a rival colony
  neutral until the player lands on its site (today's `diplomacy`). Groups
  say who *may* become an enemy; the doctrine says when (open question 13
  if the owner wants different-group to mean hostile at once).
- **Friendly rivals never share a world (decided):** a faction does not
  target a world held by a faction of its own group, the player's
  included.
- **Hive and scenario creatures.** Creatures spawned by a scenario keep
  their baked faction and belong to no instance; they stay hostile to
  everyone as now.

On the title screen each rival row gains a small group stepper beside the
faction picker.

### Folding the mirror into the colony

- `data/factions.json`: `mirror` is deleted. `colony` becomes seedable with
  the mirror's seeding and identity fields that it does not already have
  (`seed_from` 2, `seed_population`, the ship mobility block, `kit`,
  `aggression`, `research`, `diplomacy: "neutral"`). A doctrine block
  `rival` holds what differs when the doctrine is run by a rival and not
  the player: the name ("Rival Colony"), the badge, the default morph.
  `settlement_from` and `expedition_from` stay as features (nothing uses
  them after this; open question 6 asks whether to remove them).
- **The player's colony does not change.** It reads the same `settlement`
  block it reads today. The Tier 1 fields were already on `colony` for the
  observer expedition.
- **Saves.** On load a presence or fleet with `Doctrine: "mirror"` becomes
  `Doctrine: "colony"`, its faction key kept. No version bump is needed if
  the rewrite is unconditional and idempotent; the build decides after
  reading the save code.
- **Code and tests.** The three Go lines that name `mirror` go (the race
  run's "rival colony first" becomes "the colony doctrine first, unless the
  player is the colony"). `setup_mirror.go` keeps working under a neutral
  name. Tests and vmtest scripts that say `mirror` are updated to the
  colony doctrine with a rival faction key.
- **The pace goldens.** `TestFactionPaceRivalOne` never seeds a colony. The
  greyish golden (Rival factions 3) includes the rival colony: its numbers
  must not move; if the golden prints the doctrine name, it is regenerated
  once for the rename and the diff reviewed to be names only.

### Choosing each rival

`GenOptions.Rivals []string`: one entry per rival, a doctrine key or "" for
Random.

- `Rivals == nil` is today's lineup by count, so every test, soak, vmtest
  script and `history-dump` that passes a number is unchanged.
- When set, its length is the count. Each slot becomes one faction
  instance: a picked doctrine stays; a Random slot rolls a seedable
  doctrine from the campaign seed, weighted by a new `seed_weight`
  (default 1), without repeating a doctrine until all have appeared. The
  player's own doctrine is in the roll.
- Each faction is placed in slot order on an eligible site
  (`FactionSiteOK`), economy-weighted, never two on one site, on a random
  stream of its own. A fleet doctrine starts as a landed ship and gets its
  own mothership.
- `seedFactions` and `seedRaceFactions` both end in the same
  `seedRivals(picks)`; the count path builds its picks from `seed_from` as
  now.

### The title screen

- Under the Rival factions stepper, one row per rival: "Rival 1:" with
  previous and next buttons cycling Random, then each seedable doctrine by
  name (Hive, Colony, Greyish, Autonz). The same control as the Race row.
- Raising the number adds a Random row; lowering it removes the last. Rows
  keep their picks.
- Layout: four more rows do not fit under the one-column form. They go in
  a second column or the form tightens; decided when built, with a vmtest
  frame at four rivals.
- Not saved to config. `Campaign.RivalPicks` records the boxes for the
  vmtest sidecar and debugging.

### Names

Two hive factions cannot both be "the hive", and two rival colonies cannot
both be "Rival Colony". Each faction instance gets a name when seeded:

- Default: the doctrine's rival name, with a generated proper name when
  more than one faction shares the doctrine (a `names` list on the
  doctrine, or `data/names.json`): "the Kessler expedition", "the Ash
  Brood". Owner's text (open question 5).
- The badge colour is the doctrine's, shifted per instance so two of a
  kind can be told apart on the Star Map.
- The hive's hidden name stays hidden: an undiscovered hive shows its
  hidden name plus the same distinguishing word.

### Other plans this touches

- **Campaign objectives:** a conquer objective targets one faction
  instance, which is the right unit (wipe out that hive, not every hive).
- **Phase F step 6 (a grey player):** the mothership per faction is what
  lets a grey player and a rival grey faction coexist.
- **Docs:** `docs/developer/factions.md` gains "factions and doctrines";
  the wording that Rival factions 4 "brings in the autonz" becomes "the
  default lineup".

## Order of work

1. **Faction instances and groups** (about 1,200 lines; groups add the
   trace of rival-against-rival hostility, `Campaign.Hostile` and its
   tests: two factions in one group ignore each other on a level, two in
   different groups fight, whatever their doctrines). The table, `DoctrineOf`, opaque
   keys, the per-faction state in the table above, names and badge shifts,
   the Factions tab and Legends by faction. No behaviour change with one
   faction per doctrine: every existing test, both pace goldens,
   `history-dump`, the soaks and the vmtests pass untouched. New tests seed
   two hives and two grey factions and check each row of the table above.
2. **The mirror becomes the colony** (about 400 lines, mostly data and
   tests). The save rewrite, the doctrine's `rival` block, the three Go
   lines, the renamed tests and vmtest scripts, the golden reviewed.
   `factions_mirror` and the observer soak are the checks that a rival
   colony and the player's colony both play as before.
3. **The rival boxes** (about 400 lines). `GenOptions.Rivals`, the real
   roll, `seedRivals`, the title rows, `debug rivals`, a vmtest frame at
   four rivals with mixed picks, and a hive run against a rival hive
   (`race_hive_rival` already exists and is extended).

About 1,700 lines. Steps 1 and 2 change no title screen and can merge on
their own.

## Open questions (each with a recommended default)

1. **A doctrine picked twice.** Decided: two factions. (The earlier answer,
   one faction with two starts, is withdrawn.)
2. **The player's own race as a rival.** Decided: allowed, hostile.
3. **Default group numbers.** Rival against rival is decided by groups
   (see "Groups"). Decided: each rival starts in its own group (the player
   1, rivals 2 and up), so all are hostile to each other unless paired.
   Still to check in step 1: if rivals ignore each other today, the count
   path (tests, soaks, goldens) keeps that behaviour so the pace tables do
   not move, and only a run started from the title screen gets groups.
4. **Hidden identity with several hives.** Default: per doctrine, so
   discovering what one hive is names them all. Alternative: per faction.
5. **How a second faction of a kind is named.** Default: a generated proper
   name from a list on the doctrine. Alternative: a number ("Hive 2").
6. **`settlement_from` / `expedition_from`.** Default: keep the feature,
   unused. Alternative: remove it with the mirror.
7. **The hive's label in the box.** Default: its doctrine name (Random
   keeps the surprise). Alternative: a label that hides it.
8. **More than four rivals.** Default: `factions_max` stays 4.
9. **The colony run's player key.** Default: unchanged. Alternative: the
   player is `player` in every mode, which is tidier and touches saves and
   baked blueprint factions.

10. **Friendly factions on one world.** Decided: never; they do not target
    each other's worlds.
11. **The player attacks a rival of their own group.** The player has a
    group (decided: starts at 1, rivals from 2). Open: an explicit attack
    order on a friendly rival. Default: it is carried out and the rival
    defends itself on that level only, with no lasting change. Alternative:
    it turns that faction hostile for the rest of the run; or the order is
    refused.
13. **Different group from the player: hostile when?** Default: by the
    doctrine's rule (a rival colony stays neutral until contact).
    Alternative: hostile from the first day, whatever the doctrine.
14. **May the player land on a friendly rival's world?** Default: yes, and
    it does not count as contact. (Rivals never target the player's
    worlds in the same group; the player is not so bound.)
12. **Do friendly rivals help each other?** Default: no; they only refrain
    from fighting. Alternative: they defend each other's bodies when a
    shared enemy attacks.

## Where the owner rewrites text

- The row label ("Rival 1:"), "Random", each doctrine's name in the box.
- The colony doctrine's rival name and morph, and the name lists for
  second factions of a kind.

## Riskiest assumptions

- **Fifty lookups can change with no behaviour change.** Step 1's promise
  is that one faction per doctrine plays byte-identically. The goldens,
  `history-dump` and the soaks are the net; a missed lookup shows only
  when two factions share a doctrine, which is why step 1's new tests seed
  exactly that.
- **Nothing parses a faction key.** `faction_hive.go:222` takes the faction
  from a settlement name's prefix; any other place that splits or compares
  key strings to doctrine names breaks silently.
- **Baked blueprint factions.** Blueprints carry a `Faction` string
  (`hive`, `colony`). Bodies are re-stamped from their presence at
  materialise, but scenario-spawned creatures and anything spawned outside
  materialise keep the baked one, and would side with the first faction of
  that name.
- **The colony doctrine serving three roles** (the player, a rival, the
  observer expedition) from one block. The `rival` sub-block must hold
  every difference; a rival colony must stay neutral until contact while
  the player's own never consults diplomacy.
- **A real roll changes the default game.** One Random rival may be the
  greys or the autonz and not the hive.
- **Four more rows fit the New Expedition screen** without a redesign.

## As built

### Step 1: faction instances and groups (2026-11-13)

- **The table.** `Campaign.FactionTable` (`internal/campaign/faction_table.go`,
  saved, additive): a `Faction` row per instance (`Key`, `Doctrine`, `Name`,
  `Badge`, `Slot`, `Group`, `Ordinal`). `EnsureFactionTable` (from
  `NewWorldManager` and `MigrateCampaign`) gives every faction the campaign
  holds a row: the player in group 1, the observer expedition in group 1,
  every presence's and fleet's faction in key order, each in a group of its
  own. So a count-seeded campaign and an old save get rows with no change of
  key. `AddFaction(doctrine, slot, group)` makes a new instance: the
  doctrine's key while free, else `<doctrine>.2`, `.3`; a second of a kind
  gets a name (`instance_names` on the doctrine, else "Hive 2", draft), a
  badge lightened or darkened from the doctrine's, and an `Ordinal`.
- **One lookup.** `Campaign.DoctrineOf(key)` (the row, else the player's,
  the expedition's colony, else the key itself: a scenario creature's baked
  faction). Every lookup by a faction key goes through it or through the
  game's `doctrineOfFaction`, whose table is now filled from every row:
  stance and contact (`Stance`, `SetStance`, `contactFactions`,
  `raceContacts`), hidden identity (`hiddenFaction`), a pocket's owner
  noun, `history-dump`'s autonz columns (by doctrine).
- **Groups.** `Campaign.Hostile(a, b)` / `SameGroup`: same group friendly,
  else hostile; a faction with no row is a side of its own. Traced: a
  scripted body takes any other faction for an enemy (`isEnemy`), the guard
  rung counts the player's colonists and scripted creatures of other
  factions (`hostileNear`), the hunt rung takes any other faction's creature
  for prey, raids go only at the player, and the player's colonists follow
  the per-faction "hostile to the player" flags. Each now asks
  `components.HostileFactions` (set from the campaign by
  `setFactionRelation`; nil keeps "any two different factions"), the
  player's flags follow `Stance`, which is neutral for a faction of the
  player's group (and `SetStance` never turns it), and a faction never
  spreads onto its own side's world (`HeldBySide` in the launch, signal and
  race launch filters). With every rival in its own group this is exactly
  the old behaviour.
- **Baked blueprint factions.** A second instance's bodies carry its key:
  materialise stamps its eggs and composition bodies
  (`stampPresenceFaction`), its structures and its settled young take their
  settlement's faction (`stampSettlementFaction`, from `placeHiveThing` and
  `prepareSettledKin`), and a laid egg takes its layer's faction. All of it
  only for an extra instance (key different from its doctrine) or a layer
  of another faction: the default lineup and the player's race run are
  untouched (wild xenos still take a hive player's young for kin, as
  before).
- **Shown per faction.** Star Map badges and tags, the Factions tab's
  blocks (one per faction already; the second named "Xenos 2" while hidden,
  by its own name in an observer run) and Legends lines (`recordFaction`
  adds the number) tell two of a kind apart; the colour is the instance's.
- **Unchanged and per faction already:** the mothership, hoard and hull
  tier (keyed by faction), techs (per presence), contact flags, raid
  targets, posts, storage pockets.

**Verification:** Go: `TestFactionInstancesKeysAndGroups`,
`TestFactionStanceIsPerFaction`, `TestHeldBySide`,
`TestTwoGreyFactionsTwoMotherships`, `TestEnsureFactionTable` (four
seeded rivals, each its own group; every presence's doctrine agrees with its
row, seeded and a month on), `TestFactionTableSaves`,
`TestFindNearestEnemyRespectsGroups`, `TestTwoHivesOnOneLevel` (two hives
materialised on one world: every member its own key; a second hive's
hatchling its hive's). No behaviour change with one faction per doctrine:
`go test ./...` green; history-dump identical to dev's and a Rival factions
4 dump (ten seeds, 150 days) identical to before the step; `make vmtest-all`
36 of 36 with every script's frames the same; the full `make soak`.

### Step 2: the mirror becomes the colony (2026-11-13)

- **Data.** The `mirror` doctrine is gone. The colony doctrine has a
  `rival` block (`DoctrineRival`): the fields a rival colony reads in place
  of the colony's, which are exactly the mirror's: its name ("Rival
  Colony") and badge, a fleet's mobility, the ore economy, its landing kit,
  its seeding (`seed_from` 2, `seed_population` 4), and `legacy_key`
  `mirror`. Everything else (settlement, expedition table, stages,
  growth, capacity, research, aggression, diplomacy) is the colony's own,
  as the mirror's already was.
- **Views, not a second doctrine.** The loader derives the colony's rival
  view under `colony!rival` (`RivalColonyDoctrine`, `rivalViewOf`): a copy
  with the rival block applied, seedable, never playable, with no morphs
  (named by its rival name, as the mirror was). A rival colony presence
  runs the view (`Doctrine: "colony!rival"`), so the fifty-odd
  `fd.Doctrine(p.Doctrine)` lookups needed no change; the player's colony
  and the observer expedition read the colony unchanged. `BaseDoctrine`
  strips a view. **Not as the plan drew it:** the plan put the seeding and
  mobility fields on the colony itself; they live in the `rival` block
  because the colony's own mobility is the observer expedition's (a ship
  kind, range 0, speed 3) and a rival colony needs a fleet's (40, 4).
- **The legacy key keeps every number.** The view keeps `mirror` as its
  seeding sort key (`seedSortKey`: the seeding bag and its extra slots
  draw in the same order, so every site, population and golden is
  unchanged), its faction key in a count-seeded campaign
  (`FactionKeyFor`: diplomacy, settlement names, body factions and the
  vmtest `campaign.diplomacy.mirror` stay), and an alias (`fd.Doctrine`,
  `Resolve`), so old saves and tests that name it still resolve. A new
  instance of the view is keyed `mirror`, then `mirror.2`.
- **Saves.** `MigrateCampaign` rewrites a presence's, husk's, fleet's or
  row's `mirror` doctrine to the view, keeping its faction key:
  unconditional and idempotent, no version bump.
- **Code.** The race run's "rival colony first" names the view
  (`seedRaceFactions`); seeding registers each placed faction's row
  (`addFactionRow`) and `newShip` takes its doctrine from the row
  (`DoctrineOf`), so a ship's faction and doctrine are separate fields; a
  boarded ship's crew carry the ship's faction, never its doctrine;
  `syncFactionHostility` marks every faction the campaign holds or has a
  stance with, not only doctrine keys; the vmtest `debug presence` resolves
  an alias and registers the faction's row; the sidecar's diplomacy names
  factions by key. `setup_mirror.go` is the setup scripts' geometric
  mirror builtin, unrelated to the rival colony, and stays.
- **Tests.** Doctrine comparisons with `"mirror"` now compare with
  `RivalColonyDoctrine`; faction keys `"mirror"` stay.

**Verification:** Go: `TestRivalColonyIsTheColonyDoctrine` (the view's
fields, the colony unchanged, the alias), `TestRivalColonySeedingAndOldSaves`
(a seeded rival colony runs the view under the key `mirror`; an old save's
`mirror` doctrine migrates, its key kept); `go test ./...` green with both
pace goldens unchanged; history-dump identical to dev's and the Rival
factions 4 dump identical to before step 1; `make vmtest-all` 36 of 36 with
every frame the same as step 1's (the sidecar's fleet `doctrine` now reads
`colony!rival` for the rival colony's ship); the full `make soak`
(`factions_mirror` and the observer soak are the checks that a rival colony
and the player's colony both play as before).

### Step 3: the rival boxes (2026-11-14)

- **Seeding.** `GenOptions.Rivals []RivalPick` (`Doctrine`, "" for Random;
  `Group`, 0 for its own) and `GenOptions.PlayerGroup`
  (`internal/campaign/rival_picks.go`). Nil picks are the count path,
  unchanged. With picks, generation gives the player's row its group, then
  `seedRivals`: one faction instance per box in slot order (`AddFaction`:
  two Hive boxes are `hive` and `hive.2`), each on an eligible site weighted
  toward its economy, never two on one site, each from a random stream of
  its own (seeded from the campaign seed and its slot); a fleet doctrine as
  a ship landed on its site with its own mothership. Random rolls
  (`rollRivals`) draw a seedable doctrine by `seed_weight` (default 1) from
  the campaign seed, without repeating one until every doctrine has
  appeared (picked boxes count as appeared; the player's own race is in the
  roll). `Campaign.RivalPicks` records the boxes. A rival's number ("Hive
  2") counts rival factions only: a hive player's one rival hive is not
  "the second".
- **The title screen** (`internal/game/title_rivals.go`): a second column,
  "Your group" (1 to 9, default 1) and one row per rival up to the Rival
  factions number: `<` Random, then each seedable doctrine by its base
  doctrine's name (Autonz, Greyish, Hive, Colony), `>`, and a group stepper
  (rival n defaults to group n+1, so every rival is hostile to every other
  and to the player, as before). Raising the number shows another Random
  row; lowering it hides the last, which keeps its pick. Labels are drafts.
  `StartNewExpeditionWith` carries the boxes and the group.
- **The count path for the vmtest scripts.** Every row defaults to Random,
  a real roll, as decided, so a run from the title screen no longer seeds
  the count path's lineup. The config switch `rivalsByCount` makes the title
  screen ignore the boxes (nil picks); every existing script that starts a
  new game sets it in its header, so its seed seeds where it always did and
  its frames are unchanged. (The plan's "Rivals == nil leaves every vmtest
  unchanged" assumed the scripts did not go through the boxes; they do.)
- **vmtest.** Title targets `rival_prev_N`, `rival_next_N`,
  `rival_group_minus_N`, `rival_group_plus_N`, `player_group_minus`,
  `player_group_plus`; `title_fields.rivals`, `rival_groups`,
  `player_group`; `campaign.factions` (each faction's doctrine, group, slot,
  name). New scripts: `rivals_title` (four boxes picked by hand: two hives in
  one group, a Random, a colony; an observer run's Factions tab showing the
  two hives apart) and `race_hive_vs_hive` (a hive run against a rival hive
  picked in the box: its own faction and group, its brood put on the
  player's world). **Not as the plan drew it:** a new script rather than
  `race_hive_rival` extended (that script is a count-path run, and a second
  new game inside it would need the Esc menu round trip); no `debug rivals`
  command (the title targets drive the boxes).

**Verification:** Go: `TestRivalPicksSeedEachFaction` (two hives in one
group friendly, a Random rolled, a rival colony as the view, four factions
on four sites, slots recorded), `TestRivalRandomRoll` (one seed rolls the
same; no repeat before every doctrine), `TestRaceRunAgainstItsOwnRace` (a
hive run's rival hive: its own faction, hostile, not numbered),
`TestRivalInThePlayersGroup` (a rival in the player's group neutral and
never turning; the other hostile); `go test ./...`; history-dump identical
to dev's and the wide dump to before step 1; `make vmtest-all` 38 of 38,
every existing script's frames the same as step 2's, the two new scripts'
frames looked at (the boxes; the Factions tab with HIVE and HIVE 2; the
two broods and the rival named XENOS); the full `make soak`.

**After step 3 (2026-11-14): a friend is no default target.** A rival of
the player's group is passed by everything that picks an enemy for the
player unasked: a plain click's attack order, the hover hint ("Attack: ..."),
a rogue bump and the map's red dot (`components.IsDefaultAttackTarget`,
`FriendlyToPlayer` set from the campaign by `setFactionRelation`). Its
workers were already no targets (a friend's stance is always neutral); this
covers its scripted bodies. The explicit Attack order may still target it,
with no lasting change (open question 11's default). Go:
`TestFriendlyRivalIsNoDefaultTarget`.

**Draft instance names (2026-11-14).** Each doctrine has an
`instance_names` list for its second, third and fourth faction in one
campaign (draft, the owner's to rewrite): the hive "the Ash Brood", "the Red
Swarm", "the Hollow Nest"; the greys "the Pale Armada", "the Silent Fleet",
"the Ninth Choir"; the autonz "the Copper Lattice", "the Quiet Array", "Node
Cluster Theta"; the colony (and so the rival colony, through its view) "the
Kessler Expedition", "the Varga Colony", "the Ostrander Outpost". Shown
where a faction is named openly (an observer run's Factions tab); under
hidden identity a second faction is still told apart by its number. Go:
`TestInstanceNames`.

