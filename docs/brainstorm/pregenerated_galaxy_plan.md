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

### Step 1: the galaxy (built 2026-11-15)

- `GenerateGalaxy(seed, size)` (`internal/campaign/galaxy.go`) makes every
  system from the galaxy stream alone: shape, positions, rim and landfalls,
  the controls world and the gate, archetypes, names and seeds, then every
  site's history in ID order, then each system's posted and datapad quests.
  `GenerateCampaignOpts` calls it, then adds the expedition from streams of
  its own (the landfall roll, the Wreck Site, the opening contract, rival
  seeding). A test proves the galaxy is the same with any race, rival count
  or rival boxes.
- Shapes are density functions in `galaxy_shapes.go` keyed by the ids in
  `generation.json`'s new `galaxy` block (all six, weight 1); sizes, radii,
  the systems' minimum separation (8), landfall spacing (40), rim bins (32)
  and the gate's hop (7 to 11) are data there too.
- **`Campaign.Galaxy` holds less than the plan listed.** It records the
  size, shape, radius, centre of mass, landfalls and the gate and controls
  ids. The systems stay in `Campaign.Locations` and the past in
  `Campaign.History`: a `Location` also carries the expedition's knowledge
  of it (visited, the level save, presences) and `History` the events made
  in play, so moving them would not draw the boundary the plan wants
  without also splitting those types. That split is the revisit plan's save
  change.
- **The landfall system is not always the rim system.** A landfall's point
  is `start_radius` (8) outward from the outermost system of its angular
  bin; the system nearest the point owns it. Every system within the safe
  radius of any landfall point is given a start archetype and bound safe,
  and a safe binding now wins over an archetype's forced ending, so no
  safe start can hold a hive (a test found one before the fix).
- **The controls world cannot be half a diameter from every landfall.**
  With nine landfalls round the rim the centre is nearer than that to all of
  them on every shape and size (0 of 540 layouts managed it). It is rolled
  among the systems within 85% of the farthest any system lies from its
  nearest landfall: in practice 0.25 to 0.45 of the radius from the nearest
  one, near the middle.
- Landfalls: `factions_max + 1 + landfall_spare` = 9 (the plan's `+4`
  counted the player's in the four). The player's is rolled from its own
  stream; the Wreck Site sits on its point and `Campaign.StartID` names its
  system (the fixed `start` id is gone). `firstJump` still measures from the
  Wreck Site, so the player's safe start keeps its protection from rivals.
- The gate (`GateKind`, id `gate`) replaces Home: a site in space beside the
  controls world, inert. Travelling there leaves the ship in empty space as
  at the Wreck Site (`IsAdrift` covers both). Reaching it wins nothing; the
  run is endless until step 4. The Star Map draws it as a dark ring and the
  dashed line to Home is gone.
- Creation paths removed: `Expand` (a quest's `spawn_systems` is now star
  charts, `RevealCharts`: the nearest unknown systems become known),
  `Scan`'s roll (no `ScanSeq`, no chance in `scanner_tiers`), the
  new-system branch of `MaybeTravelQuests` (`new_system_one_in` gone),
  `makeContract`'s charting (it picks an unvisited system of its archetype,
  outside every safe start, nearer the ship more likely, one whose chapter is
  the template's `spawn_event` first). `placeTowardHome`, `home_bias`,
  `home_radius`, the nearby-system keys and the location templates' `home`
  block are gone. Boarding decks (`EncounterKind`) are still made in play:
  they are a faction ship's deck, not a system, and the count test leaves
  them out.
- Every system is `Discovered` as the plan's stopgap, so step 1 plays with
  the whole galaxy listed (and its posted quests all offered).
- Arriving factions start at their first stage (`seedArrival`, shared by
  the three seeding paths): an autonz seeded on a cramped site used to
  arrive already a hub.
- The lore budget's per-stamp log lines print only with `LP_LORE_LOG` set.
- Saves: `CampaignVersion` 5; every older save is refused with the existing
  "from an older version of the game ... start a new expedition" message
  (the v2 and v3 migrations are deleted).
- Re-pinned with `-update-golden`: `lore_golden_sites.json` (now pins the
  galaxy's maps, positions and names) and `greyish_pace.golden` (rivals
  still seeded the old way on the bigger map; step 3 re-pins it again).

### Step 2: knowledge and scanning (built 2026-11-15)

- `Location.Discovered` is now `Identified` (json `identified`). Seen is
  not stored: every location is drawn (`Campaign.MapLocations`). A new
  expedition identifies only its landfall system (the crew can see the
  nearest star); an observer run identifies everything (a spectator).
- Identifying: `Scan` (every system and the gate within the tier's range,
  `IdentifyWithin`), arriving (`WorldManager.Travel`, the gate and the
  Wreck Site included), accepting a quest, star charts (`RevealCharts`,
  which may chart the gate), a spreading race's reach (`ChartRaceReach`, now
  also every world the player holds).
- **Quests follow knowledge**, which the plan did not say: the 1 to 2
  quests each system is generated with would otherwise all be offered on day
  one (about 120 on a medium galaxy). A system's own quests are offered
  once it is identified; news (contracts and a travel roll's quests,
  `Quest.Offered`) is offered at once and identifies its system on accept.
- Scanner tuning (drafts, data): ranges 40 / 70 / 110 (were 25 / 50 / 100),
  fuel 40 / 25 / 15 (were 200 / 100 / 50, which bought a 10% roll). A tier 1
  scan from a landfall identifies about 3 to 6 systems; the research texts
  are rewritten to match (drafts).
- Blind jumps need no confirmation: an unidentified system lists as
  "Unidentified system — fuel N", its info column says what is not known,
  and Travel reads **Jump Blind** (drafts).
- Star Map: unidentified systems (the gate too) are dim 3.5 px points with
  no label or badges; framing is the identified systems plus the scanner's
  reach round the ship (at least 40 units), at the fit's zoom; the range
  circle shows while Scan is hovered or its report is open; the canvas
  header reads "<shape> · N systems · M identified" (draft names in
  `galaxy.shapes`). Fleet tags may still name a rival ship's unidentified
  destination (not changed).
- New Expedition has a Galaxy size row (`size_prev` / `size_next`, vmtest
  targets `title:size_prev|next`), between Rival factions and Race.
- No automatic win remains: the hive's and autonz' `race.win` is now
  `race.milestone` (same six worlds and draft texts); `SpreadMilestone`
  records a `race_milestone` Legends event once and the game posts the
  alert. `Campaign.Won` is kept for the gate step.
- The tutorial has no scan steps (the plan assumed some); its Star Map step
  text that named Home is rewritten (draft).

### Step 3: factions on the full galaxy (built 2026-11-15)

- `arrivalSite` gives every rival the system of a landfall rolled from the
  free ones (not the player's, never two on one) in all three seeding paths
  (the count, the rival boxes, a spreading race's rivals); it falls back to
  the old economy-weighted pick only with no landfall free. `seedArrival`
  puts a planet-carried rival on it at its first stage, a ship faction's
  first ship landed there.
- **The grey mothership orbits its landfall system, not the rim point.** A
  mothership's position is the site it orbits (ships return to it, build at
  it and upgrade there by `ToID`), and the rim point is not a location. It
  sits 8 units in from the point instead. `min_home_distance` is gone.
- The observer expedition already starts at the Wreck Site on the player's
  landfall; the tutorial's Training Ground was moved beside the landfall in
  step 1.
- **There was no pull toward Home in the rival colony** to remove: its ship
  picks sites in range by its expedition table (`shipCandidates`,
  `fleetView`), which never read Home. It now has the whole galaxy to pick
  from. Seeking the gate is step 4.
- Re-pinned on a medium galaxy: `TestFactionPaceRivalOne` (1 at day 40, 1 to
  3 at day 100 with two on five seeds of six, 8 to 15 at day 300) and
  `greyish_pace.golden`; the tables and what moved are in
  `docs/developer/factions.md`, "On the pregenerated galaxy". Rivals hold
  about the same by day 100 and far more by day 300 (Rival factions 1: 8 to
  15 hives, was 6 to 9; Rival factions 4: 18 to 36 presences of 80 systems).
- **The observer expedition does not cross the galaxy** (env-gated
  `TestObserverCrossing`, `LP_CROSSING=1`): over 1000 days on six medium
  galaxies it makes 15 to 23 jumps and lands no farther than 13 to 142 units
  from its landfall (the far side is 180 to 288). It is not short of fuel:
  its tank is capped at its table's `fleet_fuel` (80) and it ends with 58 to
  165. Its table scores nearby unvisited sites and charges 0.1 a fuel unit,
  so it explores locally and never heads anywhere. Before the galaxy it
  could not cross either: without quests the opening cluster never grew.
  Reaching Home was at least 3000 fuel (about 1000 travel days at 3 fuel a
  day, and some 80 jumps at the old 18 to 50 unit spacing); a medium
  crossing is about 250 units, less than the old starting fuel. First fuel
  re-tune: `start_fuel` 400 to 200, so a crossing needs at least one stop to
  refine. Fuel per unit, jump days per fuel and the leaving cost are left for
  the gate step, when there is somewhere to go.
- Soaks re-read: every threshold is about the landed site's level
  (milestones, survival, conversion, ore reached, no raids), not the map, so
  none was changed. The mirror level tests (`faction_raid_test.go`) moved to
  seed 3: on the new seed 1 and 6 landfalls the rival colony lost crew to
  hunger in forty days (1 to 3 of 4 alive). Fixed afterwards in the worker
  AI, not the director: crew crowding a container swapped places in
  `dropoff` forever and hunger never interrupted it (seed 6), and food a
  worker carried was never stored, so the others starved beside full
  bags (seed 1). See `docs/developer/ai.md`. Seeds 1, 3, 4 and 6 now pass
  (`LP_RAID_SEED`).
- vmtest: every script that pinned the old opening map was re-found on the
  new galaxy (new seeds, sites, assertions; fuel 200); the new `galaxy`
  script covers each size, three shapes, a scan and a blind jump. New
  script helpers: `row:locList:start`, `row:locList:unidentified:<n>`, the
  id `start` in debug commands, `debug travel <id>`, `debug unlock <tech>`.
- Results at the end of step 3: `go build`, `go vet`, `go test ./...` pass;
  `make soak` passes, all eight soaks (24 minutes); `make vmtest-all`
  passes, 41 of 41 scripts.
- Found while re-finding the vmtest scripts: a jump to a grey faction's
  landfall always meets its mothership (it orbits that system), and two
  landfalls can be 200 or more fuel apart (seed 777: the hive's is 207 from
  the player's, a 69-day jump).

### Step 4: the gate (built 2026-11-15)

- **The controls' vault is a structure fixture, not a forced history
  ending.** The controls world carries a `QuestFixture` (kind
  `gate_controls`, structure `gate_vault`, 27x23) built with its level, the
  way a quest's lair is: no lore budget can drop it and no history roll can
  miss it. Its site is bound safe (never a threat world on top of the
  vault). `gate_vault.basic` is `underground_dungeon`'s BSP dungeon two or
  three levels down a stair shaft, always guarded: a brute (Vault Warden)
  beside the dormant controls in the far room and three guards on the way,
  every one set to hostility 10 (`set_ai_var`, a new structure-script
  function), a door in each wall of its surface room, and a passage carved
  from the stair to the controls.
- **Two latent bugs found in `underground_dungeon.basic`** (which quest
  lairs and the `ice_age` stamp use), fixed in the vault's copy and then in
  the original (`TestUndergroundDungeonWalkable`): its shaft's stair columns
  are chosen by a parity sum that divides with `/` (not whole in this
  BASIC), so every level gets a down stair over a down stair and the game's
  path check (a down and an up stair for every level changed) rejects the
  way down; and its BSP corridors sometimes leave the far room cut off.
  Lairs already built in a save keep their old shaft.
- The vault goes 16 to 62 tiles from the landing on ground the landing
  walks to on its own level (`vaultSite`), else a fixed offset clamped
  inside the map. Over seeds 1-20 on every shape and size
  (`LP_GATE_SWEEP=1 TestGateVaultCompletable`, 360 sites): the controls
  stand 2 (168) or 3 (192) levels down, a walk reaches them from the vault's
  entrance on every site, guards stand by them (four on 351, three on 9),
  and the entrance is a walk from the landing on 355 (the other five land
  on a plateau too small for anything without stairs built).
- **The controls world is chosen differently.** "Farthest from every
  landfall" picked systems alone in a gap (a ring's core across its void)
  that no ship could reach: it is now picked among the systems linked to
  most landfalls by hops of at most 40 (`linkedToLandfalls`), then as far
  from them as before. Its archetype (a start archetype: a planet with
  ground for the vault), map and the gate's position come from a stream of
  their own, so every other system keeps its step 3 name, map and place.
- The activate order is the existing repair order: the dormant controls
  (`gate_controls_dormant`, in a new file `data/blueprints/structures/gate.json`)
  are `Repairable` into `gate_controls`, setting the level flag `gate:on`
  (and `gate:on:by`, the repairing settlement, a small addition to
  `replaceRepairable`); a hidden `gate_controls` buildable costs nothing and
  takes 120 work. The world manager turns the flag into `SwitchGateOn`
  (`Galaxy.GateOn`, `GateOnBy`, `GateOnDay`), on for good: the gate and its
  controls world identified, a `gate_switched_on` Legends line, an alert
  when a rival did it. Identifying the gate (scan, chart, arrival) also
  identifies the controls world and says the gate is dark with its controls
  there.
- **Leave the Galaxy** takes Scan's place on the Star Map while the ship is
  beside the gate: offered only while the gate is lit and the hold covers
  `gate.leave_fuel` (300), it ends the run as a win with a "Through the
  Gate" end screen and a `gate_left` Legends line. The gate draws lit once
  on.
- **Rival colonies seek the gate and leave** (`objective: leave` and a
  `gate` block in the colony's rival view; `gate.go`). The plan let the
  expedition table explore and learn the gate in passing; it never does (it
  explores locally), so a ship seeking the gate has a heading: before it
  knows the gate, the deepest system (farthest from every landfall point on
  the rim, all in sight to anyone) and then new ground farthest from where
  it has been, a target kept until reached; it learns the gate landing
  within `sense_range` (100) of it; then the shortest route of stops within
  its tank's reach (74, past its everyday 40) to the controls world, which
  it may land on whoever holds it. There it switches the gate on: live, the
  world manager orders the controls repaired by its own settlement
  (`orderGateRepairs`); off a loaded level, after `switch_days` (20) at a
  cost of `switch_loss` (1) crew. Then it flies to the lit gate and is gone,
  with a `faction_left_galaxy` Legends line and an alert only if the player
  had seen it. Its camp's refining (`fleet_refuel_per_day`) went from 1 to 4
  a day, the 1 being tuned for the old small map: at 1 a ship crawled a unit
  a day and two seeds never arrived in 500 days.
- Measured, a rival colony alone (`TestRivalColonyLeavesThroughTheGate`,
  bound 500 days): small galaxies day 39 to 138 (mean 76), medium day 69 to
  374 (mean 161); the earliest leave on the day after a 20-day stay when
  their landfall is already within 100 of the gate.
- **Fuel** (data; `LP_CROSSING=1 TestPlayerRouteToTheGate`): fuel per unit
  1 and a day per 3 fuel (unchanged), starting fuel 200 (400 before step 3),
  leaving 300 (new; Home was at least 3000). A ship run on a medium galaxy is
  3 to 7 landings from the landfall system to the controls world by hops of
  at most 60 (66 to 259 units), then 8 to 10 to the gate and 300 to leave:
  384 to 577 fuel, so 184 to 377 must be refined on the way (one fuel
  refiner makes 25 a day: 7 to 15 refiner-days), with the vault fought in
  between. The starting fuel covers the trip to the controls and a scan; the
  leaving reserve is a deliberate stay.
- The gate site stays a plain location a later plan can aim a launch or a
  signal at (open question 15); nothing is built for it.

### Step 5: endings and the rest (built 2026-11-15)

- Endings: no automatic win anywhere (the reach-Home win went in step 1,
  the spread race's in step 2); the run ends as a win only through the
  gate, as a loss when the player's people are gone. `raceEndScreen` keeps
  its win branch for a later objective.
- The tutorial's Star Map step names the gate (draft); it had no scan steps
  to update.
- Player guide: `star_map.md` (what you know, scanning, blind jumps, the
  galaxy and its gate, leaving), `factions.md` and `rival_colonies.md` (the
  rival colony seeks the gate and leaves), the hive's and autonz' pages
  (milestone, not win), `greys.md`. Developer docs: `campaign.md`,
  `factions.md` (the rival colony and the gate, the re-measured tables),
  `ai.md`, `vmtest.md`, `tutorial.md`.

### Sight: unseen systems and the galaxy haze (built 2026-11-15)

This replaces the day-one decision in "What the player knows" (every system
seen from the start). The owner asked to hide systems not yet found while
still showing the galaxy's shape.

- **Four states.** Unseen (not drawn, listed, selectable or a jump target),
  Seen (the dim point), Identified, Visited. `Location.Seen` (json `seen`) is
  saved and never lost; identifying sets it too, and `Campaign.IsSeen` counts
  an identified location as seen. The Wreck Site, the Training Ground and
  boarding decks are always on the map (`Location.GalaxyPoint` is false for
  them). `Campaign.MapLocations` returns only what is seen, so the
  destination list, map clicks, selection (`selectByID`, the Factions tab's
  links) and jumps cover nothing else.
- **What makes a system seen** (defaults are drafts in data):
  - Sight round the ship: `sight_range` 60 units (a size may set its own:
    small galaxies 40, the owner's call), plus `sight_bonus` 10 / 20 / 30 by
    scanner level. Applied at the start from
    the landfall point and on every arrival (`SeeAround`, in
    `WorldManager.Travel`, the gate and Wreck Site included).
  - A scan identifies, so everything in its range becomes seen.
  - A spreading race: `ChartRaceReach` identifies what its reach covers (as
    before) and sees within the sight range of each world it holds.
  - Quests: offering one (`offerQuest`: the opening contract, contracts,
    travel quests) sees its system; accepting identifies it, star charts
    identify. A system's own generated quests are offered only once it is
    identified, so a quest never names an unseen system.
  - The gate and the controls world follow the same rules; an observer run
    identifies, so sees, everything.
  - Old saves (no location with `seen`) load with everything seen, as they
    were played (`migrateSeen`).
- **Seen at the start** (40 seeds per size, the opening contract's system
  included): small 5 to 17 of 41 (mean 9, at sight 40; at 60 it was 6 to 26,
  mean 16), medium 7 to 32 of 81 (mean 18), large 6 to 33 of 151 (mean 16).
- **Haze.** `GalaxyBackdrop(seed, shape, radius)` replays the galaxy's
  density function from the galaxy seed (the same stream `layoutGalaxy`
  uses up to the density builder) and samples specks with a separate stream:
  0.04 per square unit (about 1,500 / 3,200 / 6,100 by size), never from
  system coordinates. Drawn as 1 px specks (1.5 px from zoom 4), alpha 40 to
  120 by density, tinted blue (dense) to violet (thin) with a fixed per-speck
  wander so the arms mix both (the owner picked it), under everything, panning and zooming with the map. Cached
  per campaign in the panel.
- **Map.** Framing fits what is seen plus the sight range round the ship (at
  least 40). A faint ring shows the sight range; the scan ring still shows
  while Scan is hovered. The header reads "<shape> · N systems · F found · M
  identified" (draft).
- **Fleets.** The simpler of the two options: a fleet is drawn only when one
  end of its flight is seen (a parked or orbiting ship: the world it is at);
  when its destination is unseen it shows its heading with no dashed line,
  and its "BOUND FOR" tag reads DEEP SPACE. Badges were already only on
  identified systems. The Factions tab names an unseen world "an unseen
  world" (draft) and does not link it. Legends names places but never
  positions, so it is unchanged. Rivals act on the whole galaxy and track no
  knowledge.
- vmtest: the sidecar's `campaign.seen` and `campaign.sight_range`;
  `galaxy.jsonl` asserts the start counts and has zoomed-out frames
  (`*_whole`) of small irregular, medium ring and large spiral (seed 5,
  was barred spiral seed 4), two blind jumps and a scan.
