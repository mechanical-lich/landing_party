# The Pregenerated Galaxy: Implementation Plan

Planned 2026-11-14 against `dev` at ed54375. Not built. Every name and line
of player-facing text here is a draft for the owner to rewrite.

## Decision

The owner's direction (2026-11-14):

- **The whole galaxy is generated at the start**, the Dwarf Fortress way:
  every system exists from day one. Nothing is created during play.
  Reasons given: history and lore are better made in one pass than on the
  fly; everything existing from the beginning opens the way to revisiting a
  galaxy with a new expedition; datapads that point at other places get
  simpler.
- **Every faction comes from outside the galaxy.** They are all
  expeditions, with their own reasons for being there or for trying to
  leave.
- **Saves may break.** Nobody else is playing; no migration.
- **Runs can go on as long as the player likes.** Play continues until the
  player's people are gone, or (a separate plan) an objective is completed
  and the player chooses to stop.

Decided through the question cards the same day:

- **Size is a New Expedition choice**: small, medium or large (about 40,
  80, 150 systems). A deliberate title-screen choice like Race; what is in
  each system is rolled.
- **Stars are visible, contents unknown.** Every system shows as an
  unidentified point from day one. Its type, history, hazards and holder
  are unknown until scanned or visited.
- **A scan identifies every system in range.** It costs fuel, has no
  chance roll, and better scanner research means longer range and cheaper
  scans.
- **Rivals spread anywhere within their own range**, whether or not the
  player has seen those systems. Their pace tables are re-pinned.
- **Home is a gate that must be found.** The way out of the galaxy exists
  somewhere in it, unidentified like everything else.
- **Every expedition begins at the rim**, each at its own point.
- **Endless by default.** With no objective system yet, a run ends only
  when the player's people are gone. Leaving through the gate is a choice
  that ends the run as a win.
- **Contracts and travel quests point at an existing system** and identify
  it. Nothing charts a new system.

Decided going through the open questions (2026-11-14):

- **The galaxy's shape is rolled from a set of real galaxy shapes**
  (spiral, barred spiral, elliptical, ring, irregular and so on), which
  decides the layout.
- **Blind jumps are allowed**, at the player's risk.
- **The gate is disabled.** The gate is its own site in space, always next
  to the world that holds its controls. The order is: land on that world,
  fight through the ruin or dungeon that guards the controls, switch them
  on, get out, fly to the gate, leave. Leaving also needs a large fuel
  reserve.
- **Once switched on, the gate is on for everyone, for good.**
- **A scan that covers the gate identifies it as the gate.**
- **Arrival points are rolled**, so a run may start beside a rival or far
  from all of them.
- **Sizes are 40 / 80 / 150 systems, default medium.**
- **A rival colony acts on its objective, and its default objective is to
  seek the gate and leave.** (Rival objectives in general belong to the
  objectives plan; this plan builds that default.)
- **Holding six worlds** is an alert and a Legends entry for a hive or
  autonz player; play continues.

Constraints carried over: no CI test job; the owner commits; tests, soaks
and vmtests run locally; scenarios stay rolled.

## Where the code is today

- **A campaign starts small and grows.** `GenerateCampaignOpts`
  (`internal/campaign/generate.go:146`) makes Home (3,000 units out,
  visible), the Wreck Site at the origin, a start system 8 units away and 5
  to 10 nearby systems within 9 to 27 units, then one contract.
- **Four things create systems during play:**
  - `Campaign.Scan` (`generate.go:402`): a fuel-costed roll (10, 20 or 30
    percent by scanner tier) that, on a hit, creates one system within the
    tier's range.
  - `Campaign.Expand` (`generate.go:235`), called for a quest's
    `SpawnSystems` reward (`internal/game/world_manager.go:727`): new
    systems marching toward Home.
  - `Campaign.MaybeTravelQuests` (`generate.go:257`), rolled on every jump
    (`world_manager.go:1368`): quests on existing or freshly charted
    systems.
  - `makeContract` (`generate.go:827`): a quest that charts its own
    location.
- **The map's shape is Home.** New systems are pulled toward it
  (`home_bias`, `placeTowardHome`), and reaching it wins
  (`world_manager.go:1325`).
- **History is half pregenerated.** The pool (civilisations, wars, arcs)
  is rolled up front (`internal/history/generate.go`, `RollPool`); each
  site is bound to it when the location is created
  (`Campaign.bindHistory`, `generate.go:556`), which also sets its
  scenario, tier, summary and lore datapads. A first-jump site gets a safe
  binding (`BindSiteSafe`), decided by distance from the Wreck Site
  (`firstJump`, `generate.go:619`).
- **Knowledge is one flag.** `Location.Discovered` (26 uses) means "exists
  for the player"; `Visited` means landed. Who holds a world is revealed
  separately (`RevealPresencesWithin`). A spread race charts whatever is
  within its reach (`ChartRaceReach`, `player_race.go:488`).
- **Rivals see only what exists.** Seeding, spread targets and landings
  walk `c.Locations`, so today a rival can only go where generation has
  already been.
- **Levels are made on first visit** from the location's seed, and saved
  once visited. Pregeneration does not change that.

## Design

### The galaxy in one pass

`GenerateGalaxy(seed, size)` makes everything that is true of the galaxy
whoever arrives in it, and draws nothing that depends on the player:

1. **Shape and positions.** The galaxy rolls one shape from a data list
   (`galaxy.shapes`, each with a weight), modelled on real galaxy types:

   | Shape | Layout |
   |---|---|
   | spiral | two to four arms winding out from a dense core |
   | barred spiral | a straight bar through the core with an arm off each end |
   | elliptical | one smooth blob, dense in the middle, thinning outward |
   | lenticular | a disc with a bright core and no arms |
   | ring | a core, a gap, and a ring of systems round it |
   | irregular | a few lopsided clumps with no centre |

   Each shape is a density function over the plane that the placer samples
   (min-separated as now), so adding a shape is a small function and a data
   row. The shape is rolled, not chosen: it is part of what a seed is. It
   is recorded on the galaxy and named in the Star Map's header (owner's
   text). Sizes and radii are data:

   | Size | Systems | Radius (units) |
   |---|---|---|
   | small | 40 | 110 |
   | medium | 80 | 160 |
   | large | 150 | 220 |

   (Today's opening cluster is 6 to 11 systems within 27 units, so density
   is about what the player knows.)
2. **Landfalls.** K points on the rim (K = factions_max + 4, so there are
   spare ones). "The rim" is shape-aware: the outermost systems by angle
   round the galaxy's centre of mass, so an irregular or ring galaxy still
   has an edge to arrive at. Landfalls keep a minimum distance from each
   other so no two share a safe start. Each
   landfall owns its nearest system as a **landfall system**, bound safe
   as first-jump sites are today, and counts the systems within the safe
   radius as its safe start. Landfalls are a property of the galaxy, not of
   a run, which is what lets a later expedition arrive in the same galaxy
   at a different one.
3. **The gate and its controls.** Two neighbouring locations, placed
   together at least half a diameter from every landfall where the size
   allows:
   - **The gate** (`GateKind`, replacing `HomeKind`): its own site in
     space with nothing to land on, like the Wreck Site. It starts
     disabled.
   - **The controls world**: an ordinary system a short hop from the gate,
     whose site is bound to hold the gate's controls inside a guarded ruin
     or dungeon (see "The gate and the end of a run").
   Both look like any unidentified point until identified.
4. **Archetypes, seeds, names** for every system, from the galaxy stream.
5. **History, once.** The pool is rolled, then every site is bound in one
   loop in a fixed order, then placed. This plan keeps the per-site binding
   rules as they are; what changes is that all sites exist while it
   happens, so later lore work can make events and datapads refer to real
   places (the gate included) with no reservation or fix-up. Lore datapads
   and fixtures are attached as now.
6. **Quests bound to real sites**, as `addDatapadQuests` does per system.

The galaxy is kept in its own struct on the campaign (`Campaign.Galaxy`:
locations, landfalls, gate id, size, the history) with the expedition's
state beside it. Nothing is split into two files yet; the boundary is
drawn so the later "revisit a galaxy" plan is a save change, not a
redesign.

### Arriving

- **The player** is given a landfall. The origin point (today's Wreck
  Site, an empty point with nothing to land on) sits just outside the rim
  at that landfall; the landfall system is the first jump, always
  affordable. A spread race has no origin point and begins on the landfall
  system itself.
- **Each rival** takes another landfall, rolled from those free (decided),
  so a run may begin beside a rival or far from all of them. A
  planet-carried rival (hive, autonz) starts on its landfall system; a
  ship faction's first ship lands there; a grey mothership parks at its
  landfall's rim point (replacing "far from Home").
- **The observer run's expedition** takes a landfall like the player.
- **The tutorial** adds its Training Ground beside the player's landfall,
  as it adds it beside the start today.

### What the player knows

Three states replace `Discovered`:

| State | Means | How it is reached |
|---|---|---|
| **Seen** | a point on the map, position only | every system, from day one |
| **Identified** | type, tags, tier, story summary, and whether it is the gate | a scan in range; arriving; a contract or datapad that names it |
| **Visited** | landed | as now |

Who holds a world stays a separate reveal, as now (a scan shows holders in
range; contact shows them on arrival).

- **Scanning.** One action, costs fuel (`scan_fuel_costs` by tier),
  identifies every system within the tier's range of the ship and reveals
  holders there. No chance roll, no `ScanSeq`. Scanner research raises the
  range and lowers the cost, as the tiers do now.
- **Blind jumps.** The player may jump to an unidentified system: the
  distance and fuel are known, the destination is not. Arriving identifies
  it. This is the risk a scan buys off (decided).
- **Spread races** identify what is within their reach
  (`ChartRaceReach` becomes "identify in reach").
- **Rivals** do not track knowledge. They act on the whole galaxy by their
  own range rules, as decided.
- **Contracts and travel quests** pick an existing system, usually
  unidentified, and identify it. A quest's `SpawnSystems` reward becomes
  star charts: it identifies that many systems (nearest unidentified
  first).

### The gate and the end of a run

**Finding it.** The gate is found by identifying it: a scan in range
(which names it as the gate, decided), arriving, or a datapad that names
it. History can leave such clues, since the gate exists when history is
made. An identified gate shows that it is disabled and that the world
beside it holds its controls.

**The controls world.** Its site is generated with a guaranteed ruin or
dungeon holding the **gate controls**, a structure the player's people
activate with an ordinary order (a work task at the structure, as research
or crafting is). The ruin is guarded: the site is bound to a hazard-tier
history ending that provides the dungeon stamp and its hostiles, forced
the way a quest's wanted event is today (`BindSiteEnding`), so what guards
it comes from the existing dungeon content and varies by seed. The
controls sit deep in it, not at the door.

**The sequence (decided).** Land on the controls world, fight through to
the controls, switch them on, get the party out and back aboard, fly to
the gate, and leave. **Leave the galaxy** is an action in orbit of the
gate, offered only while the gate is on, and costs a large fuel reserve
(data). It ends the run as a win with its own end screen. Not taking it,
the run goes on.

**The gate's state is the galaxy's.** `Galaxy.GateOn`, with who switched
it on and when. Once on it stays on for everyone (decided): the player may
find it already on because a rival got there first, and gets an alert and
a Legends entry when that happens.

**Rival colonies seek the gate and leave (decided default).** A faction
instance carries an objective; the colony doctrine's default for a rival
is `leave`. Other objectives arrive with the objectives plan; this plan
builds this one:

- The ship explores by its expedition table (the pull toward Home is
  removed). It learns where the gate is when the gate comes within its own
  scan range, or from landing on a site whose history names it.
- Knowing the gate, it flies to the controls world and lands. If the gate
  is off it switches it on: on a level the player is viewing, its director
  sends workers through the ruin to the controls (and may fail to the
  guards); in the abstract step it succeeds after a stay whose length and
  losses are data.
- It lifts off, flies to the gate with the fuel its table already tracks,
  and leaves: the ship and its faction are gone from the galaxy, with a
  Legends entry and an alert if the player knew of it.
- A rival colony with several ships leaves ship by ship.
- The player can stop it only by the means that exist: destroying the
  ship or its crew, or being hostile to it on the controls world.

**A spread race** has no ship and cannot leave. Holding six worlds is a
milestone: an alert and a Legends entry (decided). The run continues.

**Loss is unchanged:** every colonist gone (ship run), or no presence and
nothing in flight (spread run).

The objectives plan later adds chosen goals for the player and for rivals,
and a "keep playing" choice on completing one. This plan removes the
automatic endings and builds the gate.

### Rivals on a full galaxy

- Seeding moves from "economy-weighted site among those generated" to
  "the faction's landfall system". Spread, launch, signal and landing
  rules are unchanged; they simply have every system to choose from.
- The player's safe start keeps its protection (`FactionSiteOK` refuses
  first-jump sites), now measured from the player's landfall.
- **Every abstract table moves.** `TestFactionPaceRivalOne`,
  `TestGreyishPaceRivalThree` and `history-dump` are re-pinned on the new
  galaxy at medium size, with the new tables written into
  `docs/developer/factions.md`. The soaks' thresholds are re-read against
  what the factions do with room to spread; expect the hive and autonz to
  hold more worlds by day 300 than the 6 to 9 they reach now.

### The Star Map

- Draws up to 150 systems: unidentified ones as dim points with no label,
  identified ones as today. Pan and zoom already exist; the framing rule
  (today "the working cluster, ignoring Home") becomes "what is
  identified, plus the scanner's range".
- The scanner's range is drawn as a circle when the Scan action is
  hovered.
- The gate, once identified, gets its own marker. The dashed line to Home
  goes.
- New Expedition gains a Galaxy size row.

### Distances and fuel

Home at 3,000 units set the length of today's run: many hops with fuel
refined along the way. A galaxy 220 to 440 units across is far smaller
than that journey, so fuel per unit distance, the starting fuel and the
gate's leaving cost are re-tuned so that crossing a medium galaxy takes
about as many landings as reaching Home does now. All data; measured with
the observer soak's expedition (jumps and days to cross).

### Saves

`CampaignVersion` is bumped and older saves are refused with a plain
message. No migration.

### What this does not do

- It does not split the galaxy and the expedition into separate saves, or
  let a second expedition enter a played galaxy (a later plan; the struct
  boundary and the spare landfalls are its preparation).
- It does not rewrite history generation to use cross-site knowledge; it
  makes that possible.
- It does not add objectives (`campaign_objectives_plan.md`, which is
  revised after this lands: targets are simply rolled among existing
  systems, "Home hidden" becomes "the gate is not the goal", and the grey
  player's mothership is a destination to place).
- It does not give rivals a reason to leave through the gate.

## Order of work

1. **The galaxy** (about 900 lines). `GenerateGalaxy`, sizes, clustered
   positions, landfalls, the gate, history bound in one pass,
   `Campaign.Galaxy`, the player placed at a landfall, the version bump.
   `Scan`'s creation, `Expand`, the new-system branches of travel quests
   and contracts are removed in favour of picking existing systems. Tests:
   the same seed and size give the same galaxy byte for byte; no code path
   adds a location after generation (a test steps a campaign through
   scans, jumps, quests and 300 faction days and counts locations); every
   landfall has a safe system; the gate is never within the safe radius of
   a landfall.
2. **Knowledge and scanning** (about 800 lines). Seen, Identified,
   Visited; the scan that identifies in range; blind jumps; spread-race
   reach; contracts and star-chart rewards; the Star Map's unidentified
   points, range circle, framing and gate marker; the Galaxy size row.
   vmtest: a new run's map at each size, a scan, a blind jump.
3. **Factions on the full galaxy** (about 500 lines). Landfall seeding, the
   mothership at its rim point, the observer expedition's landfall, both
   pace goldens and `history-dump` re-pinned with the new tables in the
   docs, the soaks re-read and their thresholds rewritten with reasons.
4. **The gate** (about 600 lines). The gate site and its controls world,
   the forced dungeon binding, the gate controls structure and its
   activate order, `GateOn`, the Leave action and end screen, a rival
   colony seeking the gate, switching it on and leaving (live and in the
   abstract step). Tests: the controls world always has a reachable
   controls structure on seeds 1 to 20 at each size; Leave is refused
   while the gate is off or fuel is short; a rival colony alone in a small
   galaxy finds the gate and leaves within a day bound; a gate switched on
   by a rival is on for the player. vmtest: the controls activated and the
   run left.
5. **Endings and the rest** (about 400 lines). The spread milestone, no
   automatic win, the tutorial's
   Training Ground and its scan steps, fuel re-tuned from the observer
   soak, the player guide (`docs/player/star_map.md`, `factions.md`) and
   `docs/developer` updated.

About 3,200 lines. Step 1 alone leaves the game playable (everything
identified, as a stopgap flag) so each step can be checked in play.

## Open questions

Gone through with the owner on 2026-11-14; the answers are in Decision.
For the record, with the alternatives not taken:

1. Shape: **rolled from a set of real galaxy shapes** (not: one clustered
   disc).
2. Blind jumps: **allowed** (not: identify first).
3. Leaving through the gate: **a large fuel reserve**, after switching the
   gate on at its controls.
4. Rival arrival points: **rolled** (not: as far as possible).
5. Sizes: **40 / 80 / 150, default medium**.
6. The rival colony: **acts on its objective; default, seek the gate and
   leave** (not: explores and never leaves).
7. Six worlds held: **an alert and a Legends entry**.
8. The gate: **its own site in space, disabled, beside the world that
   holds its controls in a guarded ruin**.
9. A scan of the gate: **identifies it as the gate**.
10. Once on: **on for everyone, for good**.

Still open, small or tuned by play:

11. **Which shapes ship first, and their weights.** Default: all six,
    equal weights. The irregular and ring shapes are the ones most likely
    to make awkward maps (long empty crossings).
12. **How the controls are guarded.** Default: whatever the forced dungeon
    ending brings, from existing content. Alternative: a guard set of its
    own for the gate (new content, owner's design).
13. **The abstract cost to a rival of switching the gate on.** Default: a
    stay of some days and a roll for crew lost. All data.
14. **Does the player hear when a rival leaves through the gate?** Default:
    only for a rival they have met.
15. **A hive or autonz and the gate.** Default for this plan: they ignore
    it; only ship factions use it. **The owner's idea (2026-11-14), for
    the objectives plan:** the gate is a motive for them too. Through it
    lies another galaxy with life to consume or matter to convert, so
    escaping through the gate is spreading, not going home. It fits their
    existing mechanics with no ship: a hive aims its Resin Spire at the
    gate and launches an egg through it; an autonz network sends its
    signal through. Both need the gate on, so a brood or network must
    first hold the controls world and reach the controls (overrun the
    ruin, or convert it). For a rival it is a way to leave a mark beyond
    the galaxy without disappearing (the presences stay; the faction has
    "seeded beyond"). For a hive or autonz player it is a natural
    objective and ending. **One egg or one signal through the gate is
    enough (owner, 2026-11-14): it is ominous and fits the theme.** Not
    built here: the gate accepts only a ship's
    Leave action in this plan, but the gate site is a valid place for a
    launch or signal to be aimed later.

## Where the owner rewrites text

- The size names, "unidentified system", the scan's result lines, the
  blind-jump warning, the gate's name, marker label, Leave action and end
  screen, the six-worlds milestone, the origin point's name at the rim,
  the refused-save message.

## Riskiest assumptions

- **A guaranteed dungeon is always completable.** The controls world must
  have a reachable controls structure on every seed, shape and size, and
  guards that a mid-run party can beat. A blocked or impossible controls
  ruin makes the galaxy unwinnable for a return-home run, so the test over
  many seeds is the net, and the forced history ending must not be dropped
  by the lore budget.
- **A rival that can leave is good for the game.** A rival colony that
  finds the gate early simply disappears, which may read as content
  vanishing. The abstract stay and its speed are the levers.
- **Shapes with empty space stay playable.** A ring or irregular galaxy
  can put a landfall a long hop from anything; the safe-start rule has to
  hold on every shape.
- **Run length survives the smaller map.** The whole feel of a ship run
  is set by fuel against distance. Re-tuning it from the observer soak
  may take more than one pass.
- **Rivals with a whole galaxy do not overrun it.** Spreading races were
  paced on a map that only grew where the player went. With 80 systems in
  reach they may hold a third of the galaxy by day 300. The re-pinned
  tables show it; the levers are the existing range and cooldown data.
- **Binding every site's history at once is fast enough** and stays
  deterministic at 150 systems (the lore budget logs a line per dropped
  stamp today; a full galaxy will be noisy until that is quieted).
- **150 systems on the Star Map read clearly** with today's markers,
  badges and ship tags.
- **Nothing else assumes Home.** The tutorial highlight, the framing, the
  expedition table, the grey mothership's placement and the end screen
  all name or measure from Home.
- **Removing every creation path leaves quests enough to do.** Contracts
  and travel quests were the game's way of keeping the map growing; on a
  fixed map they must still feel like news.

## As built
