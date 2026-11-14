# Playable Races (Phase F): Implementation Plan

Status (2026-11-13): **steps 1 to 6 built** on `claude/factions-f-7be267`
(see "As built"); step 6 (the greys as a ship race) followed their content,
which landed on dev on 2026-11-12. The owner took
every default on all eighteen open questions (see "Open questions"), and
during step 1 asked that the interface adapt to factions through data (the
doctrine's `interface` block: "As built", step 2).
Written against `dev` at `5e7e783` (the squashed factions work: phases A to
E3, observer mode v2, the review fixes). It is Phase F of
[enemy_factions_plan.md](enemy_factions_plan.md). All player-facing text
named here (titles, alerts, win and loss lines, labels) is placeholder that
the owner rewrites (see "Where the owner rewrites text"). New strings go in
marked `_draft`.

Read with: [enemy_factions_plan.md](enemy_factions_plan.md) ("Playable
races", the Phasing row F, "Observer mode v2 as built");
[hive_plan.md](hive_plan.md) and [autonz_plan.md](autonz_plan.md) ("Phase F:
what the player will see", "As built"); [factions.md](../developer/factions.md)
("The player's doctrine", "The director", "Observer mode", "Faction
settlements", "Hostility", "The hive as a settlement", "The autonz", "Saves");
[vmtest.md](../developer/vmtest.md).

## Decision

The player picks a race on New Expedition. The colony stays the default and
is unchanged. The hive comes first, then the autonz. The greyish come last
and only if the owner wants them, because they are cheap in code but have
almost no player content (see "Greyish").

Recommended shape (each point is argued below and listed as an open
question where it is the owner's call):

- **A spreading race is a no-ship run, and the player owns presences.** A
  hive or autonz player does not get a ship. They get one or more
  planet-carried `FactionPresence`s of a new faction key, `player`, running
  their doctrine. This is the observer run's expedition again (a player-side
  doctrine on a presence of its own faction key, `campaign.AddExpeditionFleet`,
  `internal/campaign/observer.go:36`), carried by a planet instead of a
  fleet. Tier 1 grows, researches and launches it while frozen.
  Materialise, the director, reconcile and the live launch and signal run it
  while loaded. Eggs and signals found new presences of the same faction key
  (`landFleet` copies `Faction` and `Doctrine`, `internal/campaign/faction.go:1303`).
  So spreading, several planets ticking at once, and losing and winning
  planets all exist already. They have only been somebody else's.
- **View is the player's camera, and viewing your planet gives you the
  cursor.** Observer v2's View (`WorldManager.ViewLocation`,
  `internal/game/view.go`) loads a site for free. In a race run, viewing a
  site you hold binds the cursor's order paths to that site's settlement
  (`player:<loc>`), so the build menu, eggs, evolve, resin, attack and cancel
  act there. Any other site is view only, inside the race's fog.
- **Assist first.** The settlement director runs every rung, as it runs a
  rival's, and the player's orders go into the same queue on top. A later
  step lets the player switch rung groups off ("I place the eggs, you
  forage"). The smallest playable version is "observer run, plus your
  orders, plus a win and a loss".
- **Launch and transmit stay automatic in the first version.** A later
  step makes them player actions with a chosen target.
- **The hive first**, because the hive soak shows it reaches the launcher
  and fires inside forty days (`hive_plan.md`, "As built", step 7), and its
  player interface is mostly modals that already open (see "The player's
  interface").

Constraints carried over: maps and scenarios stay random (the race choice
is the one title-screen choice the owner has allowed); physical storage
(drones and wire placers haul everything); no CI test job (tests, vmtests
and soaks run locally); the owner commits; `data/config.local.json` is
never touched.

## Where the code is today

Every claim here was checked on `dev` at `5e7e783`. Paths are relative to
the repository root.

### What already reads the player's doctrine

- `campaign.PlayerDoctrine(c)` (`internal/campaign/doctrine_settlement.go:336`)
  reads `Campaign.Player.Doctrine`, else `colony`. It is read in exactly two
  places: `playerSettlementOf` (`internal/game/player_doctrine.go:19`), which
  feeds the build menu, research tree, crafting lists and crew blueprints,
  and `AddExpeditionFleet` (`observer.go:37`).
- `GenOptions.Doctrine` (`internal/campaign/faction.go:752`) goes into
  `NewPlayerPresence(opts.Doctrine)` (`generate.go:159`). **No caller sets
  it.** The title screen builds `GenOptions{Factions, Observer}`
  (`internal/game/title_state.go:272`).
- `Doctrine.CampaignMode()` (`doctrine_settlement.go:351`) returns
  `launcher` for `mobility.kind: launcher` and `ship` for everything else.
  **Only a test reads it** (`player_doctrine_test.go:57`). It has no
  `signal` case, so the autonz would come back as `ship`.
- `mobility.kind` is read only by the faction code:
  - Tier 1's spread switch (`faction.go:1122`: `launcher` → `tryLaunch`,
    `signal` → `trySignal`);
  - `LaunchReady` and `LiveLaunch` (`faction.go:1246`, `1271`);
  - the signal functions (`faction_signal.go:255`, `278`, `299`);
  - the fleet step (`faction.go:838`);
  - the live autonz rule (`internal/game/faction_autonz.go:181`).
- `playable` (`faction.go:314`) is checked only by `ValidateFactionData`,
  for the colony (`faction.go:539`). Today only the colony sets it.
- `settlement.needs` is declared and read nowhere in the UI: every needs
  display is guarded by `HasComponent(Needs)` instead
  (`internal/gui/hud_screen.go:1132`, `1911`; `internal/game/crew_panel.go:114`).

### The doctrines

| Doctrine | `mobility.kind` | Settlement block | Player content |
|---|---|---|---|
| colony | ship | full | the game |
| hive | launcher | full (crew, diet, larvae, bloom, kill drops, 4 buildables, a 20-entry evolve tree, 6 egg units, a 16-rung director) | the data was shaped for F |
| autonz | signal | full (7 buildables, a 17-entry assimilation tree, 7 print units, `refine_ferrite`, a 19-rung director) | the data was shaped for F |
| greyish | fleet | thin (one buildable, `storage_locker`; **no research**; 4 bio-printer units; a 5-rung director; `needs: false`) | almost none |
| mirror | fleet | `settlement_from: colony` | the colony |

### The player's settlement and the faction settlement do not meet

- The player's settlement is `MainState.MainSettlement`, named by the
  location's colony name (`locationColonyName`,
  `internal/game/world_manager.go:1555`). The cursor's order wrappers all
  take it (`queueBuild`, `internal/game/main_state.go:1984`;
  `addResearchTask`, `addCraftTask`, `queueConvert`). Its workers are
  `components.IsColonyWorker(e, colonyName())`, which compares only the
  settlement name (`internal/components/colony_worker.go:12`).
- The player's presence, `Campaign.Player`, is on **no location**
  (`campaign.go:244`). Tier 1 walks only `loc.Factions` (`faction.go:1011`).
  `LaunchReady`, `LiveLaunch`, `SignalReady` and `LiveSignal` all start
  from `loc.PresenceByID(id)` and need `Materialized`. So none of the
  launch or signal machinery can act on the player today.
- The hive's own Go steps run only for faction settlements:
  - `stepHiveLarvae`, `stepHiveBloom`, `hiveSiteFields` and
    `autonzSiteFields` run inside `stepFactionDirectors`
    (`internal/game/faction_settlement.go:396-440`), over `settlementPresences`.
  - `settlementBlock`, and through it the diet, `grantKillDrops` and
    `wireCampaignTechs` (`internal/game/faction_hive.go:39-150`), key on the
    `<faction>:` prefix of the settlement name.
  - `enrolHiveWorker` sets `Description.Faction` from that prefix
    (`faction_hive.go:184`).
  - So a hive doctrine on `MainSettlement` (colony name, no colon) would get
    the build menu and nothing that makes a hive work.
- **Consequence for the design:** putting the player's hive on
  `MainSettlement` means porting every hive step to a second carrier.
  Putting it on presences named `player:<loc>` gets all of them for free.
  This plan takes the second path.

### Observer mode v2 already removed the ship

`observerRun()` (`internal/game/director_orders.go:47`) gates the whole ship
surface off. These are the template for a no-ship race run:

- Star Map: no Travel, Beam, Scan or crew lists; `Update` returns after
  View and Pause (`starmap_panel.go:605`); the expedition block replaces the
  ship (`851-921`).
- Dashboard: no To Ship or To Site, Escape does nothing
  (`dashboard_state.go:201`, `211`, `236`, `249`).
- New Expedition: no `SeedShipCrew`, `StockShipLocker`, `EnterStartingOrbit`
  or tutorial (`title_state.go:285-321`); `buildShip` still builds an empty
  hull.
- Incidents and interceptions are answered silently (`incident.go:71`,
  `faction_encounter.go:45`).
- The loss check exempts the run (`main_state.go:2649`).
- The faction view rule (`internal/game/faction_view.go`) is omniscient in a
  run.
- The console refuses `observer off` in a run ("take over the expedition"
  is a todo). That todo is this phase's first step in disguise.

### What still assumes colonists

| Area | Today | For a hive or autonz player |
|---|---|---|
| Resource bar | hardcoded metal, crystal, food, biomass (`internal/gui/hud_screen.go:250-253`); counts every Storage on the level with no owner filter (`main_state.go:2357`) | needs a doctrine list (`resources`), owner-filtered; the autonz' ferrite never shows |
| Build menu | categories and buildables from the doctrine (`player_doctrine.go:32`) | works |
| Orders category | hardcoded in `buildOrderItems` (`hud_screen.go:421-430`): default, dig, mine, cancel, store, drop, attack, sleep | `sleep` needs a bed (`main_state.go:2026`): hide it per doctrine. **No convert cursor mode exists** (`internal/gui/cursor.go:5-20`), and `queueConvert` is gated on `terraforming`, which nothing grants (`director_orders.go:66`, `settlement_orders.go:282`) |
| Cursor tools | drop, pickup, store and relocate filter by `IsColonyWorker` and Inventory, never by blueprint | work; banner text says "colonist" |
| Crew panel | `ShipColonists` / `PlanetColonists` by `IsColonyWorker`; every component guarded | works; the unit type (`Kin.Type`) is never shown; fallback name "Colonist" (`crew_panel.go:177`) |
| Colonist modal | title "Colonist" (`hud_screen.go:884`); always six human equipment slots and an Equip list from storage (`1150-1196`); Inventory and Description asserted without a check (`1037`) | needs a unit variant: no equipment for kin bodies, type and tier shown |
| Needs UI | guarded by `HasComponent(Needs)` | works (hive crew have Needs, autonz none) |
| Research modal | title "Research" (`hud_screen.go:844`); tree from the doctrine; any `ResearchBuilding` click opens it (`main_state.go:1832`), the evolution chamber and the cogitator included | works; title from the doctrine |
| Crafting modal | any `CraftingStation` click opens it (`main_state.go:1830`): `hive_larva`, the fabricator, the foundry; recipes from the doctrine; title is the station's name | works. `hive_heart` and `egg_cluster` have neither component, so a click does nothing |
| Station ownership | a click opens the player's modal on **anyone's** station (no owner check) | needs an owner check (a bug today too: a rival's bio-printer opens the player's modal) |
| Hover hints | "colonist" in `main_state.go:2258-2290`; population tab "Colonists: %d" (`hud_screen.go:981`) | text from the doctrine |
| Loss | `checkTotalWipe` counts `TotalColonists` (`main_state.go:2641`), run only on a live level | a race needs "no presence and no egg in flight anywhere", checked daily on the campaign |
| Win | reaching Home on `Travel` (`world_manager.go:1276`) | needs a race rule; Home has no site, so `FactionSiteOK` refuses it |
| Quests | rewards into the ship hold (`world_manager.go:679`), nearly all fuel; evaluated only at `CurrentLocationID` (`quest.go:237`); rolled only on `Travel` (`world_manager.go:1319`) | off in a race run (see open questions) |
| Tutorial | ship throughout (`tutorial_bridge.go:144`, `263`, `493`) | off, as in observer runs |
| GameMaster | `hiveWorker(e)` counts **every** settled Kin worker toward `hostile_max` (`gm.go:56`, `faction_hive.go:199`) | the player's own drones would throttle the scenario's waves: exclude the player's settlement |
| Threat checks | `checkThreatCleared` counts every live `threat_blueprints` entity, no owner (`lore_recorder.go:121`); `exterminate_xenos` and `troid_outbreak` list `xeno_crew`, `troid_crew`, `hive_larva` | the player's brood would stop "threat cleared": exclude the player's |
| Same-faction targeting | `sameFaction` compares `Description.Faction` (`internal/systems/ScriptedAISystem.go:1368`); every hive blueprint says `hive` | under the `player` key, `enrolHiveWorker` sets `player`, so wild xenos **would** hunt the player's drones. That is a design question (open question 7) |
| Resin speed | keyed on `Description.Faction == "hive"` (`resin_speed.go:42`) | must key on the doctrine, or the player's drones lose their creep speed |
| Hostility | `syncFactionHostility` marks every **doctrine key** except the player's doctrine (`faction_hostility.go:26`) | with the player on `hive` the rival `hive` is never marked hostile. Must compare faction keys |
| Faction key = doctrine key | `Stance(faction)` falls back to `Doctrine(faction)` (`faction_ship.go:470`); also `faction_types.go:58`, `faction_view.go:86` | the `player` key needs `Diplomacy` set and these lookups taught the presence's doctrine, as `expedition` was (7 references to `ExpeditionFaction` today) |
| Hard "colony" faction | the `colonist_faction` flag (`main_state.go:754`), `KeyDoorSystem{Faction:"colony"}` (`main_state.go:429`), `StationPowerSystem.go:87`, station doors (`primer_station_modules.go:601`, `615`) | doors and airlocks a hive would meet are colony-owned: the hive breaks them on raids (burrowers) or they stay shut. Fine for a first version |
| Keep loaded | `isStaffed` is `IsColonyWorker` (`world_manager.go:194`); `enforceLoadedCap` freezes the least recently visited planet past 3 (`world_manager.go:215`) | a planet the player holds stays loaded (throttled) until evicted, then Tier 1 steps it. That is fine |
| Lore | `recordDeath` writes `colonist_died`, role "colonist", for any worker of the colony (`lore_recorder.go:76`); hive crew are named "Xeno", "Troid" | the player's units' deaths need their own event, or none (open question 12) |
| Saves | `CampaignVersion` 4; `Campaign.Player.Doctrine` is saved | see "Saves" |

### What Tier 1 says about pace

`make history-dump HFACTIONS=1,4 HFSEEDS=1-6 HFCOLS=0,40,100,200,300`, run for
this plan (Rival factions 1 is one hive; the autonz column is from the Rival factions 4 run). Each cell
is presences / eggs in flight : population.

| Seed | Hive d40 | d100 | d200 | d300 | Hive's first shot (days after arrival, mean) | Autonz awake at d100 / d200 / d300 |
|---|---|---|---|---|---|---|
| 1 | 1/1:12 | 2/1:20 | 5/2:52 | 8/0:121 | 44 | 1 / 3 / 6 |
| 2 | 1/1:20 | 2/1:28 | 5/4:55 | 9/1:140 | 52 | 1 / 4 / 9 |
| 3 | 1/1:22 | 2/1:35 | 5/3:78 | 8/0:144 | 33 | 1 / 4 / 8 |
| 4 | 1/0:8 | 1/1:8 | 3/1:37 | 6/0:89 | 49 | 1 / 4 / 6 |
| 5 | 1/1:20 | 2/2:44 | 5/4:78 | 9/0:144 | 34 | 1 / 4 / 8 |
| 6 | 1/1:20 | 2/1:28 | 4/3:57 | 7/0:95 | 46 | 1 / 5 / 6 |

A lone Tier 1 hive holds 1 or 2 sites by day 100, 3 to 5 by day 200 and
6 to 9 by day 300. Its first shot comes 33 to 52 days after arrival (mean
per seed). A live,
player-directed hive should beat that, so a "hold N sites" win near N = 6
lands somewhere inside days 150 to 300.

## Design

### 1. The campaign shape per race

`Doctrine.CampaignMode()` becomes the switch, with two values: `ship` (colony,
greyish) and `spread` (hive: launcher; autonz: signal). It replaces the
unused `launcher` value. The game reads it once per run
(`spreadRun()`, beside `observerRun()`).

**The colony** is unchanged.

**The hive (spread, by launcher).**

- *Start.* New Expedition seeds no ship crew. `campaign.AddPlayerBrood`, the
  sibling of `AddExpeditionFleet`, puts a presence of faction `player`,
  doctrine `hive`, on the **start system** at stage 0. It starts with the
  hive soak's opening: 8 bodies (6 drones, 2 scouts), 3 larvae, 30 biomass
  (`hive_soak_test.go`). Seeding never puts a faction on a first-jump site
  (`FactionSiteOK`), so the start system is clean, as the expedition's
  `fleetSiteOK` already arranges. The campaign's `CurrentLocationID` is the
  start system, and the dashboard opens with it Viewed.
- *What the player controls.* The settlement on whichever of their planets
  they are viewing, through the cursor. Every other planet runs its director
  (live while loaded, Tier 1 when frozen).
- *Moving between planets.* No travel. The Star Map's **View** opens any site
  the player holds, for free. That is observer v2's button, limited to own
  sites plus sites in the fog's rules (below). There is no "home brood" that
  is special in the rules. The first presence is labelled as the origin in
  the Factions tab only.
- *Spreading.* Tier 1's `tryLaunch` while frozen; the director's `launch`
  rung and `LiveLaunch` while loaded (unchanged). The target is drawn as for
  a rival (`pickWeighted` by economy). Later, a player action picks the
  target (step 3).
- *Losing.* No presence of faction `player` on any location and no egg of it
  in flight. Checked once a star-date day in the campaign step, not on a live
  level, because the last planet may be frozen.
- *Winning.* The owner's call (open question 1). The default is holding
  presences on `win.sites` systems at once (doctrine data; default 6, from
  the pace table). The alternatives are a stage on N sites, a share of all
  charted sites, or surviving to a day.

**The autonz (spread, by signal).** Everything above, with:

- *Start.* A node of 4 (3 wire placers, 1 enforcer) with 20 metal ore and 4
  crystal: the autonz soak's node start.
- *Spreading.* A signal founds a **dormant** sleeper node that wakes after
  `wake_days` (30). A dormant node is the player's, shown on the Star Map
  with its wake day, but it has no settlement to view until it wakes.
- *Losing.* The same rule, where a dormant node counts as a presence (it can
  still wake). The owner's call is open question 2.
- *Winning.* The same rule with its own `win` block. The default is again
  `sites` 6, waking nodes only.

**The greyish (ship).** See "Greyish" below. Not in the first steps.

**What reads `mobility.kind` and `CampaignMode` after F:** the title screen
(which races exist: `playable` doctrines, labelled by `name`), New Expedition
(ship crew or a player presence), the Star Map and dashboard (the ship
surface or the spread surface), the loss and win checks, quest generation
(off for `spread`), the tutorial (off for `spread`).

### 2. The player's faction key

The player's presences use faction key `player` (a constant beside
`ExpeditionFaction`). Their settlements are `player:<loc>`. Their doctrine is
the race. Doing this rather than reusing `hive`:

- the rival hive, the mirror's diplomacy and the player stay apart in
  `Diplomacy`, `HasFaction` launch targets, the "egg inbound" check
  (`faction.go:1218`), `factionSettlementName`, and `IsHostileWorker`;
- a rival hive and the player's hive can share a planet and fight.

What has to change to carry it, the expedition being the precedent:

- `syncFactionHostility` iterates factions, never "every doctrine but the
  player's".
- `Stance("player")` is never asked (it is the player); `Diplomacy` gets an
  entry so a stray lookup gets `neutral`, as the expedition's does.
- Doctrine lookups by faction key (`faction_types.go:58`,
  `faction_view.go:86`, `Stance`) take the presence's `Doctrine`.
- `enrolHiveWorker` sets `Description.Faction` to `player`. Resin speed keys
  on the body's settlement doctrine instead of the faction string.
- `hiveWorker` in the GameMaster and `checkThreatCleared` skip the player's
  settlement.
- The player's settlement name passes `IsColonyWorker`. In a spread run
  `locationColonyName(loc)` returns `player:<loc>`, so the crew panel, map
  dots, keep-loaded rule, `TotalColonists` and lore recording see the
  player's bodies with no other change.

### 3. The player's interface

**Binding the cursor.** When the viewed site holds a `player` presence,
`MainSettlement` is that presence's settlement (built by
`materialiseSettlement`, the same object the faction director runs). Every
`queue*` wrapper then orders into it. `applyDoctrine` already hands the HUD
the doctrine's menu. Viewing anyone else's site leaves `MainSettlement` nil:
the cursor only selects and inspects, as in an observer run.

**Which rungs become player orders.** Assist is the default. The director
runs every rung of the doctrine's table, and the player adds orders.

| Hive rung | First version | Later (step 3) |
|---|---|---|
| drones, brood, guard (egg laying) | automatic; the player may also queue any egg on a larva | a "Breeding" toggle: off means only the player's eggs are laid |
| forage, haul, hunt | automatic | a "Foraging" toggle |
| resin | automatic; the player's *Lay resin* order adds tiles | a "Creep" toggle |
| pools, clusters, chamber, spire | automatic; the player may place any of them | a "Building" toggle |
| evolve | automatic; the player may queue at the chamber, and a queued entry pre-empts the director's pick | an "Evolution" toggle |
| launch | automatic | a player action (below) |
| scouts, excavate | automatic | stay automatic |
| raid | automatic when hostile and scouted | a "Raids" toggle, plus the attack order the player already has |

| Autonz rung | First version | Later |
|---|---|---|
| placers, grow, seekers (prints) | automatic; the player may print at the fabricator or foundry | "Printing" toggle |
| convert, converters | automatic; *Lay wire* adds tiles | "Wiring" toggle |
| materials, haul, dig, ferrite | automatic | "Mining" toggle |
| storage, cogitator, core, foundry, pylons, relay | automatic; the player may place any of them; *Deploy* on a relay drone | "Building" toggle |
| assimilate | automatic; the player may queue at the cogitator | "Assimilation" toggle |
| transmit | automatic | a player action |
| guard, raid | automatic | "Raids" toggle |

The toggle is a rung filter. `director.Site` gains a `Skip map[string]bool`
of rung ids, and `Plan` skips them beside the `StageMin` check
(`internal/director/director.go:268`). The doctrine's rungs carry a `group`
for the panel. Nothing else in the director changes, so a fully toggled-off
settlement is "the player plays a hive by hand".

**The order paths the player needs that do not exist yet:**

- *Lay resin / Lay wire*: a new cursor mode and HUD order, calling
  `queueConvert` with the doctrine's `conversion.tile`. The gate becomes "the
  doctrine has a `conversion.tile`" (as both race plans proposed) instead of
  `terraforming`, so the colony is unchanged.
- *Deploy* (autonz): `orderDeploy` exists (`director_orders.go:145`). The
  player selects a relay drone and clicks a wired tile.
- *Sleep* is hidden when the doctrine has no bed buildable. *Drop* and
  *store* stay.

**Modals.**

- *Eggs.* A click on a grey egg (`hive_larva`, a `CraftingStation`) already
  opens the crafting modal with the doctrine's egg recipes
  (`craftableAt`). Locked types show their evolve entry. What is missing is
  an owner check: a rival's larva must not open it.
- *Evolve / Assimilation.* A click on the evolution chamber or the cogitator
  (`ResearchBuilding`) already opens the research modal over the doctrine's
  tree. The title comes from a new `settlement.research_label`. Effect lines
  ("Unlocks Warrior", "×1.25 health") are new formatting of the entries'
  effect blocks.
- *Unit modal.* The colonist modal, for a body with a `Kin`: title from its
  type name, no equipment section, type and tier lines, the task filters
  kept. A guard against a missing Inventory or Description (`hud_screen.go:1037`).
- *Spire / Signal Core panel* (step 3): readiness (threshold, cooldown,
  range), the sites in range, and "Launch at" / "Transmit to". This is
  `Campaign.LaunchReady` / `SignalReady` and `launchCands` with a chosen
  target (`tryLaunchAt`), and the director's launch rung walking the shot
  when the player has armed it.

**HUD.** The resource bar reads a new `settlement.resources` list (hive:
biomass and larvae as `n / cap`; autonz: metal, crystal, ferrite and
`Conversion 34% (cap 85%)`; colony: today's four), counted only from the
player settlement's storage. The population line and hints take the
doctrine's unit word (`settlement.unit_label`, draft).

**The crew panel** groups by `Kin.Type` (with the steel and bronze tiers
apart for the autonz), lists eggs and juveniles under their type, and covers
**all** the player's planets, one section a site (Tier 1's `units` for
frozen ones). Today it lists one site and the ship.

**Shared techs.** Each presence has its own `KnownTechs`, and eggs copy the
parent's. For a player that is odd: an evolve learned on one planet would
not reach the others. The default (open question 5) is one pool on
`Campaign.Player.KnownTechs`, as the colony's techs are. Every player
presence reads and adds to it: union on reconcile and on landing; the
settlement's techs set from it on materialise. Tier 1's research walk then
steps the pool once a day, not each presence.

### 4. The fog: what a race player knows

The colony learns the map by flying (orbit reveals, `Scan`). A spread race
does not fly. Default (open question 6):

- A site the player holds is Known, with everything on it.
- Landing an egg or waking a node reveals the target as orbiting does
  (`RevealPresences`).
- At a launcher stage, a site within the launcher's or signal's range is
  charted (`Discovered`), but its presences stay unknown until held or
  scouted. That is what the range circle on the Star Map shows.
- View of a site the player does not hold is allowed only inside that
  range, and it draws only what the fog allows (no `RevealAll`).
- Legends and the Factions tab follow the Known flags, as in a colony
  campaign. There is no omniscient view (that stays observer runs' rule).

### 5. The other factions when the player is a spreading race

- **Seeding.** `seedFactions` draws from the seedable doctrines. When the
  player is not the colony, the default (open question 8) is:
  - the mirror is drawn first (Rival factions 1 is a rival colony), so the
    humans exist from the start;
  - the player's own doctrine is left out of the bag, so a hive player meets
    no rival hive unless the owner wants hive-against-hive;
  - the rest as today.
  The draw keeps its own random stream, so the map is the same at every
  count and for every race.
- **The colony as an enemy.** It is the mirror (it exists, flies by the
  expedition table and builds the colony's camp). Optionally an expedition
  fleet too, started at the Wreck Site as in an observer run, so that "the
  player's game" is literally the hive's enemy (open question 9).
- **Hostility defaults.** The mirror's `diplomacy` is `neutral` until
  contact. For a hive the natural default is that contact is the hive's
  scouts meeting a colony (the patrol rung's `contact:<settlement>` flag),
  and the mirror turns hostile when the hive raids it or it sees a hive
  structure. Open question 10. The autonz and greys keep their doctrines'
  stances.
- **Wild creatures.** Wild xenos and troids carry `Description.Faction`
  `hive`, and the player's bodies carry `player`. With no change, wild ones
  hunt the player's drones. Open question 7.
- **Hidden identity inverted.** The hidden identity hides the hive from a
  colony player. A hive player knows what they are:
  - their own presences and bodies show type names (the observer view rule,
    applied to the `player` faction only);
  - the mirror shows as "Rival Colony" as today (it is openly a colony);
  - greys and autonz as today.
  How the hive *names* humans (a seen-name for colonists) is text, the
  owner's. `TestHiveNeverNamedInANormalCampaign` stays as it is for colony
  runs, and gains a twin that checks a hive run names the hive.
- **Quests and Legends.** Quests are off in a spread run (they pay fuel into
  a hold the run does not have). Legends work as they are (the
  `faction_*` events of the player's own presences, named by type). A
  `colonist_died` for a drone is wrong: the default is no death events for
  the player's units, plus a daily summary event (open question 12).
- **The GameMaster** keeps the scenario's waves on the player's planets (its
  count skips the player's own bodies). A brood on an `exterminate_xenos`
  world is a hive among wild hive creatures; open question 7 decides whether
  they fight.

### 6. Reuse

| Piece | Reused as is | Changed |
|---|---|---|
| Observer run: no ship, no crew seeding, no tutorial, the Star Map and dashboard gates | yes, behind `spreadRun()` beside `observerRun()` | the Star Map's expedition block becomes "your sites" |
| View (`ViewLocation`, `EndView`, `frameView`) | yes | limited by the fog; the cursor bound on own sites; no `RevealAll` |
| The faction settlement path (materialise, the director, larvae, bloom, live launch and signal, reconcile) | yes, all of it, for presences of faction `player` | the faction-key fixes in "The player's faction key" |
| Tier 1 (growth, research, launch, signal, landing) | yes | the shared tech pool; `tryLaunchAt` for a chosen target |
| The director | yes | `Site.Skip`; rung `group` |
| The expedition table | no (the hive does not fly) | — |
| The inspector | yes, for everything that is not the player's | — |
| The soaks | the hive soak and the autonz soak are the pattern | a race-run soak (below) |

**Is assist the cheapest playable path?** Yes. With the director running
every rung, step 1 needs no new player order except binding the cursor. The
player's eggs, evolves and buildings go into the same queue the director
fills. The director already avoids duplicating work in progress: it counts
open builds and crafts, and `cancelStuck` cancels only orders it gave.

What to watch: `max_open` counts **all** open consuming orders
(`snap.consuming`), the player's included, so a player who queues five
buildings starves the director's rungs. That is the right behaviour (the
player took over), but the HUD should show it. The director's `free`
promises (`snapshot.afford`) count the player's open orders' costs, so
nothing is spent twice.

### 7. Title screen

- **Race choice.** A selector row on New Expedition: the `playable`
  doctrines by `name`, colony first (the default). `hive` and `autonz` get
  `playable: true` as their step lands. The choice is passed through
  `StartNewExpedition` into `GenOptions.Doctrine`. It is not saved to
  config (each run chooses).
- **Per-race options.** None in the first version. The Rival factions
  stepper stays (its bag depends on the race, above). The observer flag
  keeps its meaning: an observer run is always the colony's expedition (a
  hive observer run is the owner's call, open question 13).
- **Tutorial.** Colony only. The toggle is hidden for a spread race and
  `StartNewExpedition` forces it off, as for observer runs. A hive tutorial
  is a separate later plan.
- **Crew and seed.** The seed and name fields are unchanged. The race's
  starting brood or node is data: a new `settlement.start` block (bodies by
  type, larvae, stock), so tuning it never touches Go.

### 8. Saves

**No version bump.** `Campaign.Player.Doctrine` is already saved. A race run
adds only additive data:

- presences of faction `player` in `loc.Factions`;
- a `Diplomacy["player"]` entry;
- a `Campaign.Won` reason (or a new `won_by`, additive);
- the shared tech pool on `Campaign.Player` (already the colony's field).

A colony save loads as before (`PlayerDoctrine` defaults to `colony`). A
race save opened by an older binary would read the `player` presences as an
unknown rival faction. That is acceptable: older binaries are not
supported. If the owner prefers a guard, version 5 with no migration (only
"refuse with a message if older") costs ten lines. That is open question 14.

### Greyish

The greyish fit the `ship` mode: the player flies the ordinary ship with a
grey crew. In code that is cheap, because `SeedShipCrew` already spawns
`CrewBlueprints(n)`. The cost is content and loose ends:

- they have **no research tree** and one buildable (`storage_locker`), so
  the player could build almost nothing;
- the bio-printer is placed by the rival camp script, not built (the colony
  places one from Structures);
- `needs: false`, so rations, beds and the incident tables that hit hunger
  go unused;
- their bodies carry `Description.Faction` `greyish`, so the colony-owned
  ship doors (`KeyDoorSystem{Faction:"colony"}`) need the player's faction
  instead;
- the reach-Home win and the fuel quests apply as they are, which is a lore
  question (do the greys want to go to Home?).

Default: the greyish wait until the owner writes their tree and build menu.
Then F's ship-mode work is about 1,000 lines (doors, the printer
buildable, the ship's faction, crew modal variants already done for the
hive).

## Balance

**Tier 1 does not change.** The player's presences step by the rivals'
rules. `TestFactionPaceRivalOne` and the history-dump table must not move for
a colony run, and seeding must stay the same at every count for the colony.

**A race run is new balance.** Targets for the race-run soak (measured, then
tuned in live numbers only, as E2 and E3 did):

| Measure | Hive target | Autonz target |
|---|---|---|
| the start survives (no wipe) by day 40 | 9 of 10 seeds | 9 of 10 |
| first launch / signal | by day 40 on 8 of 10 (the hive soak's band) | by day 60 on 6 of 10 (the hub comes late) |
| sites held at day 100 (assist only, Tier 1 plus one viewed planet) | 2 or more on 8 of 10 | 1 awake, 1 dormant on 6 of 10 |
| sites held at day 200 | 4 to 7 | 3 to 6 |
| the win (default `sites` 6) | between days 150 and 300 on most seeds | between days 200 and 350 |
| the mirror raids the hive | only after contact | only after contact |

An assist-only run is the floor: a player who also lays eggs and queues
evolves should beat it. If the assist run alone wins too fast, the win's
`sites` rises. Tier 1 stays as it is.

**Known balance risks** carried from E2 and E3: the hive's spire comes late
(days 8 to 27 after the hive stage); autonz research is slow live (two or
three entries in forty days); the hive's crew lacks `NeverSleep`, so its
evolves may stall at night. A player will feel all three. Step 1's soak
measures them under the player's start, and the owner decides which to fix
first.

## Order of work

Each step ends playable end to end, with Go tests, a vmtest script and,
where it applies, a soak. Sizes are estimates against E2 (about 8,500
lines with tests and data).

### Step 1: A hive run you can play (assist)

The smallest thing that is actually playable: choose the hive, start with a
brood, view it, give orders, watch it spread, win or lose.

- `CampaignMode()` → `ship` | `spread`; `spreadRun()`; `hive` gets
  `playable: true` and a `settlement.start` and `win` block.
- The title screen's race row; `GenOptions.Doctrine` set.
- `campaign.AddPlayerBrood` (faction `player`, the start system, the start
  block); `Diplomacy["player"]` neutral; no ship crew, no hold, no tutorial,
  no quests.
- The faction-key fixes (hostility by faction, doctrine lookups, resin
  speed by doctrine, the GameMaster and threat checks skip the player,
  `locationColonyName` in a spread run).
- The ship surface gated by `spreadRun()` as by `observerRun()`. The Star
  Map lists the player's sites, its eggs in flight, and View on own sites.
- View binds `MainSettlement` to `player:<loc>`; the director runs the full
  table.
- Station clicks check the owner.
- The loss (daily: no `player` presence, no egg) and the win (`sites`), with
  the end screen's text from the doctrine (draft).
- For step 1 the fog is "own sites Known, landings reveal" (section 4's
  first two bullets); the range-charting comes in step 4.

**Done when:** a new hive run starts from the title screen with the brood on
the start system; viewing it gives the cursor; a queued egg, a placed pool
and a queued evolve are worked by drones; the director keeps the hive fed
and growing alone; a launch founds a second `player` presence that the
player can View and order; the loss and win fire; a colony run is
unchanged.

**Verification:**
- Go: `TestHiveRunStarts` (start presence, no ship crew, `CurrentLocationID`),
  `TestHiveRunOrdersBindToTheViewedSite`, `TestHiveRunLossWhenNoPresence`
  (a frozen last planet wiped by Tier 1 starvation is still a loss),
  `TestHiveRunWin`, `TestPlayerFactionHostility` (the rival hive is
  hostile, the player's bodies are not attack targets),
  `TestGameMasterSkipsPlayersHive`, the colony doctrine snapshot tests
  unchanged, `TestFactionPaceRivalOne` unchanged.
- vmtest: `race_hive` (title → hive → brood viewed; queue an egg from the
  larva modal; the evolve modal open; `debug days` to a launch; the Star Map
  showing the egg; View of the new site). Every existing script passes
  unchanged: frame diffs for `tutorial`, `new_expedition`, `save_load`,
  `esc_menu` as in C0.
- Soak: `TestHiveRunSoak` (ten seeds, the brood start, assist only, the
  viewed origin live for 30,000 ticks, then Tier 1 to day 150), in
  `make soak-hive-run` via `scripts/soak_par.sh`. Its table: survival, first
  launch, sites held at days 40, 100 and 150, wins.

**Size:** about 3,500 lines (40% of E2). Most of it is gates and tests. The
new mechanics are the binding, the loss and the win.

### Step 2: The hive's own interface

- *Lay resin* (cursor mode, gate by `conversion.tile`); *Sleep* hidden
  without a bed.
- The resource bar from `settlement.resources`, owner-filtered, with larvae
  `n / cap`.
- The unit modal (no equipment, type lines); the crew panel by type, over
  all the player's sites.
- The research modal title from `research_label`, effect lines.
- Doctrine text for the population line and hints (draft).
- The shared tech pool on `Campaign.Player` (open question 5).

**Done when:** a hive player can do every hive order by hand (lay resin,
place each structure, lay each unlocked egg, evolve) and read their brood's
state without the inspector.

**Verification:** Go tests for the convert gate (colony still refused), the
shared pool (an evolve on one site reaches a second on its next reconcile
and on a landing), the resource counts (a rival's pool on the same level
not counted). vmtest `race_hive_ui` (frames of the HUD bar, the unit modal,
the crew panel, the evolve modal). Re-run the step 1 soak (the shared pool
changes research pace).

**Size:** about 2,000 lines (25%).

### Step 3: Taking over (toggles, launch as an action)

- `director.Site.Skip` and the rung `group`s; a panel of toggles per site
  (saved on the presence).
- The Spire panel: readiness, targets in range, "Launch at"; `tryLaunchAt`;
  the launch rung waits for the player's arming when "Launch" is toggled to
  manual; Tier 1 does not fire a manual site while it is frozen.

**Done when:** a player who switches every group off plays the hive by hand
and the director does nothing on that site; a chosen launch target is the
one the egg flies to.

**Verification:** Go tests that each group's rungs are skipped and others
run; `TestHiveRunChosenLaunch`. vmtest `race_hive_takeover`. Soak: no new
soak. A short Go run (one seed, 6,000 ticks) checks the toggle itself: with
"Breeding" off, a site lays only the player's eggs.

**Size:** about 1,500 lines (18%).

### Step 4: The world from the hive's side

- Seeding for a spread race (the mirror first, the player's doctrine out of
  the bag, open question 8); the optional expedition enemy (open question 9).
- Contact and hostility for the mirror against a hive (open question 10).
- Wild creatures' stance toward the player's brood (open question 7).
- The fog's range charting and range-limited View (section 4).
- Hidden identity inverted (the `player` faction named by type) and its
  test twin; the Factions tab's "your brood" block.
- Legends for the player's units (open question 12).

**Done when:** a hive run at Rival factions 2 meets a rival colony that
turns hostile at contact and raids, and the Star Map shows only what the
hive could know.

**Verification:** Go tests for the seeding bag per race (the colony's bag
and draws unchanged at every count), the contact rule, the fog
(`TestHiveRunFog`), the name test twin. vmtest `race_hive_rival` (a mirror
raid on a viewed brood). Soak: the step 1 soak at Rival factions 2 (the
mirror's pressure on survival).

**Size:** about 1,500 lines (18%).

### Step 5: An autonz run

Everything in steps 1 to 4, for the signal:

- `autonz` `playable`, its `start` and `win` blocks; the node start.
- *Lay wire*; *Deploy* on a relay drone; ferrite and the conversion cap in
  the resource bar; steel and bronze in the crew panel; the assimilation
  title.
- Dormant nodes as the player's (shown with their wake day; the loss rule,
  open question 2).
- The Signal Core panel and "Transmit to" (`trySignalAt`).

**Done when:** an autonz run plays from the title screen to a woken second
node and to the win or loss.

**Verification:** Go tests as step 1's for the autonz (start, loss with a
dormant node, win, orders bound, the transmit target). vmtest `race_autonz`.
Soak: `TestAutonzRunSoak` (ten node starts, assist only, to day 150).

**Size:** about 2,000 lines (25%). Most of the machinery is shared by then.

### Step 6 (optional): The greyish

Only once the owner has written a greyish tree and build menu. Ship mode
with a grey crew: the ship's door faction from the player's doctrine, a
bio-printer buildable, the crew modal variant, the incident tables for a
crew without needs.

**Size:** about 1,000 lines of code (12%), plus the owner's content.

**The content landed (2026-11-12, dev `b131df6`, greyish_plan.md).** The
greys now have a tree, a build menu, a vat, console, armory, array and
emitters, and a storage pocket. That plan's section "Phase F: what a
greyish player will see" settles the player's side. The ship is a saucer
grown in rings and decks. Planets get support structures only. Every grey
locker is a linked beam locker, and the win is reaching Home. Step 6 takes
its defaults where it left a question open: question 17 (rings 8, 10, 12,
15, 18; four decks; gated by `grey_hull_lattice` and `grey_hull_spine`)
and question 22 (the rival keeps its 17 by 17 stamp). Step 6 is built in
five parts, each its own commit, each leaving the colony run unchanged
(history-dump identical, the colony vmtests within noise):

- **6a. A grey crew on the ship.** Greyish `playable`, with a `race` block
  (order 3, a blurb; no start or win block, since ship mode keeps the
  colony's crew start and the reach-Home win). The player's faction key
  is `player`, as in a race run, so rival greys stay hostile and every
  "the player's own" check holds. The crew is the race's `crew` start (a
  worker, a technician, a soldier with a beam pistol), each body given its
  type, and its faction set to `player`. The ship's doors follow the
  player's faction. The incidents that hit hunger, sleep or morale skip a
  crew without needs. The doctrine gets an `interface` block. The vat and
  console stand aboard from the start, beside the hold. A buildable
  `ship_only` flag refuses the vat, console and array on a planet. The
  title row shows the greys third. Tests: start, hostility, ship-only,
  needs. vmtest `race_greyish`.
- **6b. Linked lockers.** The player's grey lockers are doors onto one
  pocket per level, using the rival's pocket code keyed by the player's
  settlement. Beaming goes through the pocket. The storage inspector
  offers a beam only from a container the viewer's settlement owns.
- **6c. The saucer.** A doctrine `ship` block chooses the layout, `grid`
  (the colony's, the default) or `saucer`. `BuildShipLevel` draws the
  saucer as a round hull of radius 8 on deck 0 with the hold, the vat,
  the console and a locker.
- **6d. Growing the hull.** The `grey_hull_floor` and `grey_hull_wall`
  buildables, and the research entries `grey_hull_lattice` and
  `grey_hull_spine`. *Expand hull* and *Add deck* queue physical orders:
  dig the old rim, floor the ring, wall it, place the stairs. Until the new
  rim is closed, the old one stands. A Hull section sits on the research
  modal.
- **6e. Soak, vmtests, docs.** A grey run soak: the crew beamed down on the
  first site and left to the doors and the player's orders, which must not
  lose a body to the run's own rules in forty days. Then the docs.

### Total

About 11,000 lines for steps 1 to 5, one and a third times E2. Step 1
alone is the playable hive.

## Open questions (decided: the owner took every default, 2026-11-09)

Each default below is now the decision; the alternatives are kept for the
record.

1. **The hive's win.** Default: hold presences on `win.sites` = 6 systems at
   once (doctrine data; from the pace table, inside days 150 to 300).
   Alternatives: a stage (the great nest) on N sites; a share of the charted
   sites; landing on Home (Home would need a site); survive to a day.
2. **The autonz' loss with only a dormant node left.** Default: not lost
   (the node can still wake). Alternative: lost (a node that cannot act is
   no player).
3. **Launch and transmit in the first version.** Default: automatic (the
   director's rung and Tier 1), the player action in step 3. Alternative:
   the player's action from step 1, which needs the Spire panel up front.
4. **Assist as the default.** Default: assist on for every group; the
   player switches groups off. Alternative: off by default (the player does
   everything, the director only on planets not viewed).
5. **Techs across the player's planets.** Default: one shared pool on
   `Campaign.Player`, as the colony's techs are. Alternative: each planet
   evolves alone, as rivals do, and eggs carry their parent's techs.
6. **The race's fog.** Default: own sites Known; landings reveal; launcher
   or signal range charts sites but not their presences; View only inside
   range. Alternative: observer runs' omniscient view (simplest, spoils the
   map).
7. **Wild hive creatures and a player hive.** Default: wild xenos and
   troids are hostile to the player's brood (they hunt drones, the brood's
   guards fight them, kills feed biomass). Alternative: they are kin and
   never fight (the player's bodies keep `Description.Faction` `hive` by a
   special case).
8. **Seeding when the player is a hive or the autonz.** Default: the mirror
   is drawn first, and the player's own doctrine is left out of the bag.
   Alternative: the bag as today (a hive player can meet a rival hive).
9. **The colony as an enemy.** Default: the mirror only. Alternative: also
   the expedition fleet from the Wreck Site, flown by the expedition table,
   so the player's old game is literally the enemy.
10. **The mirror's stance toward a hive.** Default: neutral until the
    hive's scouts make contact with a colony (`contact:<settlement>`) or the
    mirror lands on a planet the hive holds, then hostile everywhere.
    Alternative: hostile from the start (the colony would never be neutral
    to a hive).
11. **Quests in a spread run.** Default: off. Alternative: race contracts
    ("consume the camp on X", biomass rewards), a plan of their own.
12. **Legends for the player's units.** Default: no per-unit death events
    (a hive of 40 would flood Legends); a site's losses go into one daily
    `brood_losses` event when they pass a threshold. Alternative: a
    `drone_died` per body, as colonists have.
13. **A hive observer run.** Default: no (observer runs stay the colony's
    expedition). Alternative: `observerMode` with a hive choice watches a
    hive run play itself, which would make the race-run soak a vmtest too.
14. **Saves.** Default: no version bump (every new field is additive).
    Alternative: version 5, with older binaries refusing race saves.
15. **The greyish.** Default: wait for their content. Alternative: ship them
    in F with the thin tree they have (the player could build only lockers).
16. **The tutorial.** Default: colony only, hidden for a spread race.
    Alternative: a short hive tutorial (its own plan).
17. **The start.** Default: the hive soak's brood (8 bodies, 3 larvae, 30
    biomass) and the autonz soak's node (4 bodies, 20 metal, 4 crystal) on
    the start system. Alternative: a single egg landing on the start system
    (more dramatic, slower, riskier).
18. **The HUD's word for units.** Default: the doctrine's `unit_label`
    (draft). Alternative: keep "crew" for every race.

## Where the owner rewrites text

- Race names and blurbs on the title screen (`name`, a new `blurb` in
  `data/factions.json`).
- The win and loss lines for each race (`win.text`, `loss.text`) and the end
  screen's title ("You Made It Home" is the colony's).
- `settlement.unit_label`, `research_label` ("Evolve", "Assimilation"), the
  resource bar's labels (larvae, conversion cap).
- The new order names and hints: *Lay resin*, *Lay wire*, *Deploy*, the
  toggle group names (Breeding, Foraging, Creep, Building, Evolution,
  Raids, Printing, Wiring, Mining, Assimilation), "Launch at",
  "Transmit to".
- The unit modal's labels and the research modal's effect lines.
- How the hive names colonists and the mirror ("seen names" from the hive's
  side), if the owner wants them different.
- Legends events for the player's units (`brood_losses` or `drone_died`),
  and the `faction_*` lines as the player's own (written as "your brood").
- The Star Map's "your sites" block and the Factions tab's player block.

All of these go in marked `_draft`, as the existing files are.

## Riskiest assumptions

- **Binding `MainSettlement` to a faction settlement is safe.** A lot of
  code treats `MainSettlement` as the player's colony: the starting plaza,
  `beamUpSelected`, the ship's settlement, `drawTasks`, `evaluateQuests`,
  the storage beam. In a spread run most of those are already gated off
  with the ship. A missed one is a crash or a silent no-op. Step 1's tests
  have to walk the HUD's paths with the bound settlement.
- **The director and the player share a queue without fighting.**
  `max_open` and `free` count the player's orders, and `cancelStuck` only
  cancels the director's. But a player's order the director never planned
  for (a pool in an odd place, an egg of a type over its mix) may be undone
  by later rungs' decisions (the brood rung lays toward its mix whatever
  the player laid). Measured in step 1, made explicit by the toggles in
  step 3.
- **The faction-key split catches every faction = doctrine assumption.** The
  expedition found three (`Stance`, `fleetSiteOK`, the diplomacy entry). A
  planet-carried player presence runs more code (materialise, the hive and
  autonz steps, the GameMaster, scripts reading `Description.Faction`), so
  there will be more. The hostility and same-faction tests are the net.
- **The live hive is strong enough to be fun.** The hive soak's live hive
  starves on two seeds in ten, reaches the spire late and evolves once in
  forty days. Assist play inherits that. A player who does nothing should
  not lose in forty days. The step 1 soak decides whether the start needs
  more biomass.
- **Tier 1 is a fair stand-in for planets the player is not watching.** A
  player expects their frozen planets to grow at least as well as the one
  they watch. Tier 1's rate is the authority (live agrees within 0.6 to
  1.5×, `hive_plan.md`), but the player will notice a frozen planet that
  never builds the spire they would have built.

## As built (2026-11-09)

Commits on `claude/factions-f-7be267` after `15a6498` (the plan decided).
Where the plan and the code disagreed the plan's intent was kept; the notes
say why.

### Step 1: a hive run you can play (assist)

**The four bugs found on dev**, each its own commit with a regression test
before the race work:

- A rival's station opened the player's crafting or research modal
  (`c1fab1c`, `TestRivalStationOpensNoPlayerModal`). Stations carry no owner;
  `stationOwner` reads it as the director does (a faction settlement's radius
  round its centre, or the Kin settlement that laid a larva). A click on a
  foreign station opens the inspector.
- The GameMaster's `hostile_max` count and `checkThreatCleared` counted every
  settled hive body whoever owned it (`1d06011`, `TestPlayersHiveIsNoThreat`);
  both skip the player's settlement now (`ownedBySettlement`).
- `syncFactionHostility` skipped every faction whose key matched the player's
  doctrine (`12a7536`, `TestHostilitySkipsOnlyThePlayersFaction`); it skips
  the player presence's faction key.
- Resin speed looked up the body's faction string (`7ac5514`,
  `TestResinSpeedByDoctrine`); it reads the doctrine of the body's
  settlement through `factionDoctrines`, which `settlementBlock` uses too.

**The race run** (see docs/developer/factions.md, "Playable races"):

- `CampaignMode()` is `ship` | `spread`; `Campaign.SpreadRun()`;
  `campaign.PlayerFaction`; `AddPlayerBrood`; `PlayerSites`, `HoldsSite`,
  `SpreadOutcome` (`internal/campaign/player_race.go`). The doctrine's new
  `race` block holds the blurb, the start, the win and the loss (all text
  draft). The hive is `playable`.
- `newPresence` takes the faction and the doctrine apart (a landed player
  egg was about to found a presence of doctrine `player`); a landed egg or a
  signal's node of the player's is Known.
- The game: `spreadRun()`, `shiplessRun()` and `viewing()` beside
  `observerRun()`; the ship-surface gates moved to `shiplessRun()`; the
  title screen's Race row (`race_prev`, `race_next`, the blurb; the
  tutorial toggle hidden for a spreading race); `StartNewExpeditionAs`
  opens the run viewing the brood; `bindPlayerSettlement` after
  materialise; `ordersHere()` replaces `!observerMode()` at every cursor
  order gate; `campaignTechs()`; `PlanetColonists` reads the viewed world;
  `checkSpreadEnd` and `takeEnd` with the race's end screen words; the Star
  Map's **Your worlds** block and `[yours]` tag; the dashboard's `Your
  worlds: k / 6`.
- **Deviation:** the start is the hive soak's nest (stage 1), not stage 0
  as the plan's design section first said; the plan's open question 17 named
  this start, and it is what the soak had measured.
- **Deviation:** the fog is the plan's step 1 version (own worlds Known,
  landings reveal); View refuses a world the player does not hold.
- **Not changed:** the vmtest tutorial bridge's `liveSite` now also accepts
  the viewed planet, so `entity:` and `hud:` targets resolve on a race run's
  world (and an observer's).

**Verification:** Go: `TestHiveRunStarts`, `TestHiveRunOrdersBindToTheViewedSite`
(an egg queued from the larva's modal is laid), `TestCampaignModes`,
`TestPlayerBroodAndItsEggs`, `TestSpreadOutcome`, the four bug tests;
`TestFactionPaceRivalOne` unchanged; `make history-dump HFACTIONS=0,1,2,3,4
HFSEEDS=1-6 HFCOLS=0,40,100,200,300` byte-identical to dev's. vmtest
`race_hive` (frames looked at: the race row, the brood with the Orders and
Hive menus, the larva modal, the evolve modal, the Star Map at day 0 and 60
with Your worlds 2 / 6, the second world on Umbral Scar).
`make vmtest-all`: all 30 scripts pass; against dev's frames the colony
scripts (`tutorial`, `new_expedition`, `save_load`, `esc_menu`,
`storage_beam`, `factions_mirror`, `observer`) differ by at most 0.04% (tile
animation), except New Expedition's screen (0.59%: the Race row, and Launch
and Cancel 40 px lower); the sidecar gains `spread_run`, `race`,
`player_sites`.

**The hive run soak** (`TestHiveRunSoak`, `make soak-hive-run`, ten seeds
side by side through `scripts/soak_par.sh`): a hive run at Rival factions 1,
the player doing nothing, the brood's world viewed and live for 30,000 ticks
(forty days), then frozen and stepped by Tier 1 alone to day 150. The
owner's rule, "a player who does nothing must not lose in forty days",
holds on all ten; the start needed no tuning.

| Seed | Bodies 0 / 10k / 20k / end | Units at the end | Eggs laid | Biomass | Starved | Stage | Spire / first launch tick | Lost tick | Worlds live end / day 100 / day 150 | Second world day | Lost / won day |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 8 / 21 / 24 / 24 | drone 14, scout 10 | 19 | 19 | 0 | 2 | 7470 / 7500 | - | 1 / 3 / 3 | 54 | - / - |
| 2 | 8 / 16 / 20 / 19 | drone 10, scout 5, warrior 3 | 29 | 22 | 0 | 2 | 6930 / 7500 | - | 1 / 2 / 3 | 50 | - / - |
| 3 | 8 / 21 / 24 / 24 | drone 14, scout 10 | 19 | 20 | 0 | 2 | 6990 / 7500 | - | 2 / 2 / 5 | 35 | - / - |
| 4 | 8 / 21 / 26 / 25 | drone 11, scout 10, warrior 4 | 31 | 20 | 0 | 2 | 6900 / 7500 | - | 2 / 2 / 3 | 38 | - / - |
| 5 | 8 / 24 / 24 / 26 | drone 11, scout 10, warrior 4 | 31 | 8 | 0 | 2 | 17910 / 18000 | - | 1 / 2 / 4 | 48 | - / - |
| 6 | 8 / 22 / 22 / 22 | drone 13, scout 9 | 17 | 20 | 0 | 2 | 5730 / 6000 | - | 1 / 3 / 4 | 47 | - / - |
| 7 | 8 / 21 / 25 / 26 | drone 13, scout 9, warrior 3 | 32 | 22 | 0 | 2 | 5220 / 5250 | - | 1 / 2 / 4 | 52 | - / - |
| 8 | 8 / 19 / 24 / 26 | drone 12, scout 9, warrior 4 | 32 | 52 | 0 | 2 | 6270 / 6750 | - | 2 / 2 / 4 | 32 | - / - |
| 9 | 8 / 20 / 24 / 24 | drone 13, scout 9, warrior 2 | 35 | 48 | 1 | 2 | 6900 / 7500 | - | 1 / 3 / 4 | 60 | - / - |
| 10 | 8 / 20 / 22 / 22 | drone 13, scout 9 | 17 | 10 | 0 | 2 | 6270 / 6750 | - | 1 / 3 / 5 | 49 | - / - |

Against the plan's targets: no seed lost (9 of 10 wanted by day 40); first
launch by day 40 on all ten (ticks 5,250 to 18,000 live: days 7 to 24; the
plan wanted 8 of 10); worlds at day 100 two or three on every seed (2 or more
on 8 of 10 wanted); at day 150 three to five (the plan's day-200 band was 4
to 7); no seed won by day 150 (the default `sites` 6 lands later: the
plan's 150 to 300 band holds so far). The live spire comes far earlier here
than in E2's hive soak (days 7 to 10 on nine seeds): the player's brood
starts neutral to everything and nothing raids it. As the hive soak, the
same seed run twice varies a little (seed 2: two worlds live at the end in
a first run, one in this; first launch tick 6,750 and 7,500).

**Rough edges left for later steps:** the player's own bodies are named by
their seen names ("Xeno", "Troid": hidden identity is inverted in step 4);
the Star Map's destination rows tag the player's own world `[xeno nest]`
(step 4); the level's start message "Welcome to your new home. Establish a
perimeter." is the scenario's; the Star Map still draws the ship's range
circle and its marker at the Wreck Site; the resource bar counted every
storage on the level (fixed in step 2); launch is automatic (step 3).

### Step 2: the hive's own interface

Built to the owner's direction given during step 1: the interface adapts to
a faction through data, by a middle path. Every list and label a race shows
is in the doctrine's new `settlement.interface` block, drawn from a fixed
set of kinds in Go (docs/developer/factions.md, "The interface block"):

- `resources` (slot IDs and a source each: `storage`, `food`, `larvae`,
  `conversion`), `unit_label` / `units_label`, `research_label`, `orders`
  (ids from a registry with optional label and description; `convert` is
  new: the Lay Resin cursor, open to a doctrine with a conversion tile),
  `unit_modal` (section kinds `identity`, `type_tier`, `stats`, `needs`,
  `equipment`, `task_filters`, `inventory`), `crew_group` (`type`,
  `type_tier`), and for step 3 `panels` and `groups`.
- The colony declares its interface as it was; `TestColonyInterfaceIsTheOldOne`
  checks it draws exactly what an empty block does. The hive's: biomass and
  larvae, Forage and Lay Resin (no Sleep), Evolve, a "Brood" unit modal with
  its type line and no equipment, the crew grouped by type.
- Unknown kinds fail `ValidateData` at start-up (`validateInterfaces`) and
  `TestInterfaceBlocksValidate`. A made-up fourth doctrine reusing existing
  kinds renders its unit modal, titles and orders with no Go change
  (`TestFourthDoctrineUnitModal`); its panel half lands with step 3.
- The resource bar counts only the player's own settlement's storage in a
  race run; the colony keeps counting every storage on the level, as it did
  (a deliberate keep: changing it would move colony frames).
- The crew panel in a race run lists every loaded world's bodies grouped by
  the doctrine's key and a row per frozen world (its Tier 1 units); its
  detail follows the unit modal's sections. An exhaustion limit out of reach
  (the hive's, the autonz', which never sleep) is not drawn.
- **Shared techs** (open question 5): `Campaign.SyncPlayerTechs` pools every
  player presence's and egg's techs on `Campaign.Player.KnownTechs` and hands
  the pool back; `WorldManager.syncPlayerTechs` folds loaded worlds' live
  research in first and retypes their living. It runs daily, when a world is
  bound and when an evolve finishes. Tier 1's research walk steps only the
  **seat** (the world with the largest presence), so a spread does not
  research faster for every world it holds.
- Deviation: the plan's "unit word in hints" landed as `units_label` in the
  hover hints ("Send brood to this location."); the colony's reads exactly
  as before.

**Verification:** Go: `TestInterfaceBlocksValidate`,
`TestColonyInterfaceIsTheOldOne`, `TestHiveRunInterface`,
`TestConvertOrderByDoctrine`, `TestConvertOrderForAHive`,
`TestFourthDoctrineUnitModal`, `TestRaceRunSharedTechs`; `go test ./...`
green. vmtest `race_hive_ui` (frames looked at: the Orders menu with Forage
and Lay Resin, the Brood unit modal with "Drone" and no equipment, the Crew
tab grouped by type); `make vmtest-all` 31 of 31; the colony scripts against
dev's frames as in step 1 (the tutorial's modal frames 0.002%, its world
frames at most 0.14%, the same as step 1's: worker movement). Size: about
1,500 lines with tests and data.

### Step 3: taking over (toggles, launch as an action)

- **Rung groups.** A director rung carries `group` (the hive's: breeding,
  foraging, creep, building, evolution, raids, launch); `director.Site.Skip`
  skips the groups the player took over, kept on the presence
  (`FactionPresence.TakenOver`, additive). `validateRungGroups` refuses a
  rung group the interface does not declare.
- **The structure panel** is one implementation (`gui.OpenStructurePanel`,
  `internal/game/structure_panel.go`) over the doctrine's `interface.panels`:
  row kinds `text`, `readiness`, `target_list`, `action`, `group_toggles`;
  sources `launcher`, `signal`, `launch_targets`, `signal_targets`; actions
  `launch_at`, `transmit_to`. A click on a structure of the player's that has
  a panel opens it. The hive declares two: the **Brood Heart** (the group
  toggles) and the **Resin Spire** (readiness, the worlds in reach, Aim at).
  A made-up fourth doctrine's panel (a Spore Tower over the signal source
  and `transmit_to`) renders with no Go change (`TestFourthDoctrinePanel`).
- **Launch as an action.** Aiming takes the `launch` group over and arms
  `FactionPresence.LaunchTarget`. Taken over, the launcher (Tier 1's
  `tryLaunch` and the live `LiveLaunch` alike, and the signal's
  `trySignal`) fires only at the armed target, and not at all unarmed; the
  launch rung's `LaunchReady` agrees, so the shot's bodies walk to the spire
  only when armed. The aim is spent by the shot. `Campaign.LaunchStatus` /
  `SignalStatus` give the panel its lines without a random draw.
- **Deviation (a bug found):** the Resin Spire blueprint carries a
  `ScriptedAI` (Phase B's launcher script), so a click on the player's own
  Spire ordered an attack on it. `ownStructure` (a structure inside the
  player's own faction settlement's radius) keeps the attack order, the
  attack cursor and the hover hint off it; the colony is unchanged.
- **Not changed:** the launch group is not a director skip (its rung waits
  on the aim instead); the GameMaster still counts a Spire's script toward
  `hostile_max` (a structure, not the player's settled body).
- New vmtest debug command `place <blueprint> <dx> <dy>`; the sidecar gains
  `panel` (title, row kinds), `taken_over` and `launch_target`.

**Verification:** Go: `TestTakenOverGroupsSkipTheirRungs` (every group
taken over: the brood's director orders no egg, building, resin, research,
forage or haul in 1,500 ticks; breeding given back: it lays again),
`TestSpirePanelAims`, `TestFourthDoctrinePanel`, `TestPlayerAimedLaunch`
(campaign: no shot unarmed, the shot at the aim, the aim spent, given back it
picks as a rival's), `TestInterfaceBlocksValidate`. vmtest
`race_hive_takeover` (frames looked at: the Brood Heart with breeding taken
over, the Spire's readiness and five worlds in reach, aimed at Umbral
Scar). `make vmtest-all` 32 of 32; the tutorial's frames against dev's vary by
0.04 to 0.2% from run to run, and a second dev run against the first shows
the same (mlge's wall-clock stopped-task delay), so the colony is
unchanged. No new soak: the toggles change nothing in assist play (the step 1
soak covers it). Size: about 1,300 lines with tests and data, under the
revised 1,800 (the panel and the readiness sources came out smaller than
feared).

### Step 4: the world from the hive's side

- **Seeding** (`seedRaceFactions`, open question 8): the rival colony first,
  the player's own doctrine out of the bag, the rest by `seed_from`, the cycle
  repeating; a stream of its own. A colony run's seeding is untouched: the
  history-dump table is byte-identical to dev's.
- **Contact** (open question 10): `Campaign.RaceContact` turns a neutral
  faction hostile everywhere when its ship lands on a world the player holds
  (`LandShip`) or when a body of the player's comes within 8 tiles of one of
  its workers (`WorldManager.raceContacts`). **Deviation:** the plan said
  "the day" they meet; a drone passing a camp is there for moments, so the
  check runs every 30 ticks (found by the vmtest: a once-a-day check missed a
  drone walked to the camp and straight back).
- **Wild creatures** (open question 7) needed no code: the brood's bodies
  carry faction `player`, wild ones `hive`, so they hunt each other and the
  brood's guards answer.
- **The fog** (open question 6): `ChartRaceReach` charts, once a day, every
  site within the reach of a world at its launcher (or signal) stage;
  `InRaceReach` lets View open such a world. **Deviation:** a world in
  reach is seen as from orbit (its presences revealed, the whole level drawn)
  rather than "only what the fog allows": none of the player's bodies is
  there to light the fog, so it drew black.
- **Hidden identity inverted:** the player's own bodies and eggs carry their
  types' names (`doctrineType`: a body of faction `player`, or of a presence
  whose settlement is the player's, as an egg's); the Star Map tags its world
  `YOUR <stage> <pop>` and its Destinations row only `[yours]`; the Factions
  tab's block is `Your Hive` with its units by type; the HUD's faction line
  reads "yours"; its `faction_*` Legends lines name the hive.
  `TestHiveNeverNamedInANormalCampaign` is unchanged; its twin is
  `TestHiveRunNamesTheHive`.
- **Legends** (open question 12): a race run's units' deaths are counted;
  a world that loses five or more in a day records one `brood_losses` event
  (`data/history/events.json`, draft).
- New vmtest debug command `presence <location> <doctrine> <pop>`; `rival`
  skips a race run's own brood.

**Verification:** Go: `TestRaceSeeding` (Rival factions 1 to 3: one mirror
ship, no rival hive), `TestRaceContactOnLanding`, `TestRaceContactOnTheGround`,
`TestRaceReachCharts`, `TestHiveRunNamesTheHive` (with the losses event);
`go test ./...` green; `make vmtest-all` 33 of 33; history-dump identical.
vmtest `race_hive_rival` (frames looked at: the Star Map's `YOUR NEST 8` and
`RIVAL COLONY CAMP` tags, the colony's camp with the drone beside it and the
alert "Your brood has been seen on Murk Shoals. They are hostile now.", the
Factions tab's `YOUR HIVE` block). Size: about 900 lines.

**The hive run soak at Rival factions 2** (`make soak-hive-run
RACEFACTIONS=2`; the step 1 soak's start, now seeded the mirror and a
greyish ship rather than a rival hive):

| Seed | Bodies 0 / 10k / 20k / end | Units at the end | Eggs laid | Biomass | Starved | Stage | Spire / first launch tick | Lost tick | Worlds live end / day 100 / day 150 | Second world day | Lost / won day | Mirror hostile |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 8 / 21 / 24 / 24 | drone 14, scout 10 | 19 | 19 | 0 | 2 | 7470 / 7500 | - | 1 / 3 / 6 | 54 | - / 144 | false |
| 2 | 8 / 20 / 20 / 19 | drone 9, scout 6, warrior 2 | 35 | 25 | 3 | 2 | 20070 / - | - | 1 / 2 / 3 | 81 | - / - | false |
| 3 | 8 / 21 / 24 / 24 | drone 14, scout 10 | 19 | 20 | 0 | 2 | 6990 / 7500 | - | 2 / 3 / 6 | 35 | - / 141 | false |
| 4 | 8 / 21 / 26 / 25 | drone 11, scout 10, warrior 4 | 31 | 20 | 0 | 2 | 6900 / 7500 | - | 2 / 2 / 3 | 38 | - / - | false |
| 5 | 8 / 24 / 24 / 26 | drone 11, scout 10, warrior 4 | 31 | 8 | 0 | 2 | 17910 / 18000 | - | 1 / 2 / 4 | 48 | - / - | false |
| 6 | 8 / 22 / 22 / 22 | drone 13, scout 9 | 17 | 20 | 0 | 2 | 5730 / 6000 | - | 1 / 3 / 5 | 47 | - / - | false |
| 7 | 8 / 21 / 25 / 26 | drone 13, scout 9, warrior 3 | 32 | 22 | 0 | 2 | 5220 / 5250 | - | 1 / 2 / 5 | 52 | - / - | false |
| 8 | 8 / 19 / 24 / 26 | drone 12, scout 9, warrior 4 | 32 | 52 | 0 | 2 | 6270 / 6750 | - | 2 / 2 / 4 | 32 | - / - | false |
| 9 | 8 / 20 / 24 / 24 | drone 13, scout 9, warrior 2 | 35 | 48 | 1 | 2 | 6900 / 7500 | - | 1 / 3 / 5 | 60 | - / - | false |
| 10 | 8 / 20 / 22 / 22 | drone 13, scout 9 | 17 | 10 | 0 | 2 | 6270 / 6750 | - | 1 / 3 / 5 | 49 | - / - | false |

No seed lost. Two seeds **won** by day 150 (days 141 and 144) and three more
held five worlds: without a rival hive taking sites, the brood spreads
faster than at step 1's soak (seeded under the old bag, a rival hive among
its rivals). The rival colony met the brood on no seed (its camps are on
other worlds; it raids only after contact), so the soak measures its
competition for sites, not its raids. Seed 2 starved three bodies and never
fired live (its spire came at tick 20,070).

### Step 5: an autonz run

- The autonz are `playable` (`race.order` 2: the title row is Colony, Hive,
  Autonz). Their race block, interface (metal, crystal, ferrite, the
  conversion share; Lay Wire and **Deploy Relay**; Assimilation; a unit modal
  without needs; crew by type and tier), rung groups and the Signal Core's
  panel (readiness, worlds in reach, Transmit here, the group toggles) are
  data; the one new kind is the `deploy` order (`MainState.deployAt`: the
  nearest idle unit whose type deploys folds into its structure on the
  clicked wired tile). Aimed transmits came with step 3's `aimed`.
- A signal founds a dormant node of the player's (Known, shown with its wake
  day, alive for the loss, not held toward the win until awake;
  `TestPlayerAimedSignal`).
- **Tuning the soak asked for:** the plan's start (the autonz soak's node of
  four) lost every unit on one seed of ten within the forty live days (a
  `crashed_ufo` start, greys hostile; a probe of the same seed kept all four:
  run-to-run variance on a fragile start). The start is now six (four wire
  placers, two enforcers, 30 metal ore, 6 crystal), and all ten seeds hold.
- **A bug the soak found:** a loaded world wiped of the player's bodies kept
  its record until it froze, so the run was not lost that day (seed 2 above
  was recorded lost on day 41, after its live run). `dropDeadPlayerWorlds`
  drops such a record at the daily check; the soak now also fails a seed
  that ends the live run with no bodies.
- vmtest `race_autonz` (frames looked at: the race row on Autonz, the node,
  the Unit modal "Wire Placer · steel" with no needs, the Signal Core panel,
  the Star Map at day 200 with three worlds, the second world).

**Verification:** Go: `TestAutonzRunStarts`, `TestAutonzDeployOrder`,
`TestPlayerAimedSignal`, `TestCampaignModes` (the playable order); `go test
./...` green; `make vmtest-all` 34 of 34; history-dump identical to dev's.
Size: about 700 lines with tests and data.

**The full `make soak`** (2026-11-09, at the end of step 5): the observer,
expedition, autonz and hive soaks pass as before; the hive run soak (Rival
factions 1, the new seeding: the mirror, no rival hive):

| Seed | Bodies 0 / 10k / 20k / end | Units at the end | Eggs laid | Biomass | Starved | Stage | Spire / first launch tick | Lost tick | Worlds live end / day 100 / end | Second world day | Lost / won day | Mirror hostile |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 8 / 21 / 24 / 24 | drone 14, scout 10 | 19 | 19 | 0 | 2 | 7470 / 7500 | - | 1 / 3 / 4 | 54 | - / - | false |
| 2 | 8 / 21 / 25 / 23 | drone 10, scout 8, warrior 5 | 33 | 12 | 3 | 2 | 20760 / - | - | 1 / 2 / 3 | 81 | - / - | false |
| 3 | 8 / 21 / 24 / 24 | drone 14, scout 10 | 19 | 20 | 0 | 2 | 6990 / 7500 | - | 2 / 2 / 5 | 35 | - / - | false |
| 4 | 8 / 21 / 26 / 25 | drone 11, scout 10, warrior 4 | 31 | 20 | 0 | 2 | 6900 / 7500 | - | 2 / 2 / 3 | 38 | - / - | false |
| 5 | 8 / 24 / 24 / 26 | drone 11, scout 10, warrior 4 | 31 | 8 | 0 | 2 | 17910 / 18000 | - | 1 / 2 / 4 | 48 | - / - | false |
| 6 | 8 / 22 / 22 / 22 | drone 13, scout 9 | 17 | 20 | 0 | 2 | 5730 / 6000 | - | 1 / 3 / 4 | 47 | - / - | false |
| 7 | 8 / 21 / 25 / 26 | drone 13, scout 9, warrior 3 | 32 | 22 | 0 | 2 | 5220 / 5250 | - | 1 / 2 / 4 | 52 | - / - | false |
| 8 | 8 / 19 / 24 / 26 | drone 12, scout 9, warrior 4 | 32 | 52 | 0 | 2 | 6270 / 6750 | - | 2 / 2 / 4 | 32 | - / - | false |
| 9 | 8 / 20 / 24 / 24 | drone 13, scout 9, warrior 2 | 35 | 48 | 1 | 2 | 6900 / 7500 | - | 1 / 3 / 4 | 60 | - / - | false |
| 10 | 8 / 20 / 22 / 22 | drone 13, scout 9 | 17 | 10 | 0 | 2 | 6270 / 6750 | - | 1 / 3 / 5 | 49 | - / - | false |

No seed lost; first launch by day 24 on nine seeds (seed 2's spire at
tick 20,760, no live launch); three to five worlds at day 150; no win yet
(the Rival factions 2 run above won on two seeds). The autonz run soak:

| Seed | Bodies 0 / 10k / 20k / end | Units at the end | Eggs laid | Biomass | Starved | Stage | Spire / first launch tick | Lost tick | Worlds live end / day 100 / end | Second world day | Lost / won day | Mirror hostile |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 6 / 4 / 4 / 4 | enforcer 2, wire_placer 2 | 0 | 0 | 0 | 1 | 750 / 1500 | - | 2 / 3 / 3 | 32 | - / - | false |
| 2 | 6 / 6 / 6 / 6 | enforcer 2, wire_placer 4 | 0 | 0 | 0 | 1 | 6000 / - | - | 1 / 2 / 3 | 76 | - / - | false |
| 3 | 6 / 4 / 4 / 5 | enforcer 2, wire_placer 3 | 0 | 0 | 0 | 1 | 750 / 1500 | - | 2 / 3 / 3 | 32 | - / - | false |
| 4 | 6 / 4 / 4 / 4 | enforcer 2, wire_placer 2 | 0 | 0 | 0 | 1 | 750 / 1500 | - | 2 / 3 / 3 | 32 | - / - | false |
| 5 | 6 / 4 / 4 / 5 | enforcer 2, wire_placer 3 | 0 | 0 | 0 | 1 | 750 / 1500 | - | 2 / 3 / 3 | 32 | - / - | false |
| 6 | 6 / 7 / 7 / 9 | enforcer 4, wire_placer 5 | 0 | 0 | 0 | 1 | 750 / 17250 | - | 1 / 2 / 3 | 53 | - / - | false |
| 7 | 6 / 7 / 6 / 5 | enforcer 2, wire_placer 3 | 0 | 0 | 0 | 1 | 750 / 25500 | - | 1 / 2 / 3 | 64 | - / - | false |
| 8 | 6 / 7 / 7 / 5 | enforcer 3, wire_placer 2 | 0 | 0 | 0 | 1 | 750 / 24750 | - | 1 / 2 / 3 | 63 | - / - | false |
| 9 | 6 / 4 / 4 / 4 | enforcer 2, wire_placer 2 | 0 | 0 | 0 | 1 | 750 / 1500 | - | 2 / 3 / 3 | 32 | - / - | false |
| 10 | 6 / 7 / 8 / 6 | enforcer 3, wire_placer 3 | 0 | 0 | 0 | 1 | 750 / 21000 | - | 1 / 2 / 3 | 58 | - / - | false |

No seed lost; every seed holds three worlds by day 150. **Balance (the
owner's call):** on five seeds the node of six already meets the signal's
threshold and transmits on day 2 (tick 1,500), spending two of its six
units; the core column reads tick 750, the first check, where materialise
placed it. No autonz run wins by day 150 (its spread is slower than the
hive's: three worlds each).

**Not verified / not built:** the greyish (step 6, then waiting for
content; built 2026-11-13, see "Step 6: the greys, a ship race"); the rival colony raiding a hive on a live level was not seen in a soak (no
seed's colony met the brood; the raid rung against the bound settlement is
the same code as against a colony); a full hand-played run to a win in the
VM (the win and the end screen are Go-tested, and the soaks reach it); an
autonz Deploy in the VM (Go-tested; no relay drone exists at the start).

### After step 5: the player in the director's place (2026-11-12)

The owner asked whether the player factions still need the director. They
keep it (it runs every world the player is not giving orders on, and Tier 1
assumes a brood that tends itself), but the player can now take its place
entirely on any world:

- **Every rung has a group.** The hive's `scouts` (scouting) and `excavate`
  (digging), the autonz' `seekers` (scouting). `TestRunThisWorld` fails
  if a playable race's rung is left without one.
- **Guarding is its own group.** The hive's `guard` rung grew warriors and
  held their posts in one; it is now `warriors` (breeding) and a guard-only
  `guard` (defence). The autonz' `guard` is `enforcers` (printing) and
  `guard` (defence); `pylons` is `defence_pylons` (building) and a
  sentry-only `pylons` (defence). The rung ids the director's tests use are
  kept; a rival's behaviour is the same (the guard ran before the grow in one
  rung, and now runs in the next).
- **Two player orders** for what only the director could do: **Post Guard**
  (`post`, roles from data: the nearest idle guard walks to the tile and
  holds it) and **Load** (`load_launcher` on the Spire and the Signal Core:
  the shot of the player's units, the launch rung's roles first, holds round
  the structure, as the launch rung's walk does; loading takes the launch
  over, and the live launch fires on a ready day at the aim).
- **"I run this world"** (`world_toggle`): one switch on the Brood Heart and
  the Signal Core takes every group over (or gives every one back).
- **Decided (the owner, 2026-11-12): leaving a world hands it back** to the
  director (`handBack`: every group released; the aim stays), and **the
  director never spreads by itself**: a race run's launcher or signal fires
  only at the world the player aimed it at, live and in Tier 1 alike
  (`aimed`), so aiming no longer takes the launch over (loading still
  does). The race soaks' player now aims each world at its nearest reachable
  world once a day, the one thing the director no longer does; the vmtest
  scripts aim with `debug aim` before stepping days.

**Verification:** Go: `TestRunThisWorld` (the switch, then 1,500 ticks with
the director ordering nothing at all), `TestPostGuard`, `TestLoadTheSpire`
(the shot of three, never a warrior), the director's own tests unchanged;
vmtest `race_hive_takeover` extended (frames looked at: the Spire loaded with
three of the brood round it, the Brood Heart with "I run this world" on and
every group with it).

Then dev (`a8e5640`, faster FOV and lighting, soak progress lines) was
merged in (`f0cc49f`) and checked on the merged tree: `go test ./...`
green, `make vmtest-all` 34 of 34, history-dump byte-identical to the new
dev's, and the full `make soak` passing (the rival hive and autonz soaks
within their thresholds with the guard rungs split; the hive and autonz run
soaks with no loss in forty days and the same worlds as before; the
observer and expedition soaks took 142 s against 248 s before the merge).

**Verified (2026-11-12, hand-back and aim-only):** `go test ./...` green
(`TestHandBackOnLeaving`; `TestPlayerAimedLaunch` now also checks that an
unaimed launcher holds fire live and through 60 Tier 1 days); `make
vmtest-all` 34 of 34 (`race_hive_takeover` asserts the launch taken over
after Load, not after Aim); history-dump identical to dev's; the race soaks,
their player aiming once a day: no loss in forty days on any seed of either
race; the hive holds 3 to 5 worlds by day 150 (second world on days 28 to
48), the autonz 3 (as before: its soak player aims on day 0, so the node of
six still transmits on day 2 on five seeds).

### Step 6: the greys, a ship race (2026-11-13)

Built in the five parts planned (see "Step 6"), on dev's greyish content
(`b131df6`, rebased under this branch on 2026-11-12), each its own commit:

- **6a, a grey crew on the ship (`26d622f`).** Greyish is `playable` with a
  `race` block (order 3, a blurb, and a `start` that a ship race reads as
  its crew by type, each type's kit, the structures aboard and the hold).
  A race's player presence takes the faction key `player`
  (`NewPlayerPresence`), so rival greys stay hostile and the player's own
  crew never is. `seedRaceCrew` spawns the worker, the technician and the
  soldier with a beam pistol, typed, faction `player`, and stands the vat
  and console by the hold; a crew without needs gets no rations. Doors:
  `systems.PlayerFaction` (the player's people's key in a ship run, else
  `colony`) is admitted by every door owned `colony` and unlocks keyed
  doors; the scripts' `colonist_faction` flag reads it. A buildable
  `ship_only` flag refuses the vat, console and array off the ship (the
  player's build order; rival directors keep building their array on the
  ground). The greys' `interface` block: metal, crystal, alloy, plastic,
  specimens; the colony's orders less Sleep; the colonist's unit modal less
  needs; the crew panel by type. The tutorial is the colony's only, for a
  ship race too. No incident reads needs, and grey bodies have no Needs
  component, so "the incident tables for a crew without needs" needed no
  code (pinned by `TestGreyRunStarts`).
- **A printed unit takes its crafter's faction (`2d654f8`).** Found while
  wiring 6a, and a bug in the autonz run as well: a player's print kept its
  blueprint's faction (`autonz`, `greyish`), so it fought for the rival.
  The autonz run soak's thin seed 2 now ends with four bodies on three
  worlds (two on two before).
- **6b, linked lockers (`aa237f3`).** `pocketOwner` counts the player's own
  settlements (the base, the ship, each world's) when the player's doctrine
  keeps a pocket; the player's doors are relinked every 30 ticks on any
  level the player plays on and just before a beam. Beaming up from any
  site door takes from the whole site pocket; beam-down lands in it; the
  inspector lists the whole pocket at any door (`PocketItems`), and offers
  no beam from another owner's pocket door (an ordinary locker keeps its
  beam, as greyish_plan.md says). The global inventory's All Sites and By
  Site tabs are open to a pocket player from the start. The beam's owner
  check now knows every one of the player's settlement names (dev's
  compared one, which would have refused the player's own second world).
- **6c, the saucer (`ace7ed3`).** The greys' settlement block has a `ship`
  block (layout `saucer`, radius 8, one deck); `BuildSaucerLevel` carves
  it round the hull centre with `world.RoundRoom` (the round room the
  setup scripts' `carve_round_room` and a rival's landed saucer use, moved
  into `world`). The hull tiles are tinted placeholders (green floor
  plates, the hull wall's art) in `zz_greyish.json`: tile variants follow
  catalogue indices, so a tile added mid-catalogue moves every later
  tile's variants, and new tiles go in a file that sorts last.
- **Two determinism fixes (`02702e1`, `6eb0f5e`)** found by checking 6c's
  frames: the vmtest `lore_ufo` was not repeatable on dev. The asteroid
  primer tagged its rocks in map order on size ties (features sampling the
  `asteroid` region by index landed on a different rock), and the
  GameMaster's spawn pick walked its rules in map order (the same roll, a
  different creature). Both now iterate in a fixed order; three runs of
  `lore_ufo` match, and the autonz hub scripts' first spawns changed once.
- **6d, growing the hull (`1ac0268`).** Expand Hull (click a deck) and Add
  Deck are orders in the greys' Orders menu; the ship block lists the rings
  (8, 10, 12, 15, 18) and decks (up to four, each new one at the start
  radius and no wider than the deck below), gated by two new entries,
  `grey_hull_lattice` and `grey_hull_spine` (live only: Tier 1 never walks
  them and rival directors never research them). The work is ordinary
  orders queued in phases from the ship level's flags (saved with it):
  expand is a breach in the old rim, the ring's floor spreading from it
  (only the reachable frontier is ordered), the old rim dug out, then the
  new rim's stepped corners and the rest of it; a deck is stairs up, its
  floor out from the stairs' head, its rim. **Not as greyish_plan.md
  drew it:** the old rim comes out before the new rim stands, since the
  ring is a tile wide in places and a new rim raised first cut parts of it
  off from the breach (the ship has no vacuum, so nothing is exposed). The
  research modal carries a Hull section (each deck's radius, the next ring
  and deck with their gates, the work in progress). The build menu gains
  Hull (the grey floor and wall) and Stairs.
- **6e, the grey run soak (this commit).** `TestGreyRunSoak`, `make
  soak-grey-run` (in `make soak`): ten grey runs, the crew of three beamed
  down on the start system and left alone for forty days; a wiped crew
  fails.

**Verification:** Go: `TestGreyRunStarts`, `TestGreyRunHostility`,
`TestColonyRunDoorFaction`, `TestShipOnlyBuildables`,
`TestGreyLockersArePockets`, `TestColonyLockersAreNotPockets`,
`TestGreyShipIsASaucer`, `TestBuildSaucerLevel`, `TestGreyHullExpands` (8
to 10 all the way), `TestGreyHullAddsADeck`, `TestShipRaceFaction`,
`TestCraft_TypedPrintTakesTheCraftersFaction`, `TestPickRandomIsRepeatable`,
`TestAsteroidRegionsAreRepeatable`, `TestSameSiteBuiltTwicePlaysTheSame`.
vmtest `race_greyish` (frames looked at: the race row, three greys aboard
the saucer, the vat's print rows, the console's tree with its Hull
section, a locker aboard listing the hold, the ring half built from the
breach, the crew on the start site after 600 ticks); `make vmtest-all` 36
of 36, every colony script the same within threshold; history-dump
identical to dev's; the rival greyish soak and the hive and autonz run
soaks pass; the grey run soak: no crew member lost on any of ten seeds in
forty days.

**Balance and open points for the owner:**

- A ring is slow: with the starting crew's two builders the ring from 8 to
  10 took about 55,000 ticks (some 70 days). Each floor tile is its own
  haul from the hold (the same pace live and in the background). Levers: a
  free floor (the walls keep the alloy), more printed workers.
- The greys' start (a worker, a technician, a soldier; 30 metal, 12
  crystal, 4 alloy, 10 biomass, 20 stone; the vat and console aboard) is a
  first guess; every grey text is draft.
- The hull tiles are placeholders (green plates, the colony's wall).
- No research effect lines yet (greyish_plan.md describes them, nothing
  draws them); the hull entries carry a `develop.hull` line for when it
  does.

