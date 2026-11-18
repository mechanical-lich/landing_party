# VM Play-Testing (`go tool vmtest`)

The game runs as an Ebitengine VM guest, plays a JSON-lines script with
injected mouse and keyboard input, and writes numbered PNG frames, a state JSON
per frame, and a log. Use it to see a change on screen, and to check it with
asserts, without a window and without touching your desktop.

The framework is mlge's: the host is `github.com/mechanical-lich/mlge/cmd/vmtest`,
a `tool` in `go.mod` (so it is built against this module's Ebiten), and the
game side is `mlge/vmtest/guest`. **The engine reference is mlge's
[`docs/vmtest.md`](https://github.com/mechanical-lich/mlge/blob/main/docs/vmtest.md)**:
script ops and comparisons, keys and paths, output layout, `diff`, the hook
interface, `vmtest.json` and the release check. This page covers what is
Landing Party's: how to run it here, the state summary, the target names, the
debug commands and the starter scripts.

macOS only for now. History: `docs/brainstorm/vmtest_plan.md` (the original
design) and `docs/brainstorm/vmtest_mlge_plan.md` (the move into mlge).

## Run a script

```bash
make vmtest SCRIPT=new_expedition LABEL=before
```

`SCRIPT` is a name under `vmtest/scripts/` or a path to a `.jsonl` file.
`LABEL` defaults to the short git hash. `make vmtest-all` runs every starter
script and fails if any fails. Without make:

```bash
go tool vmtest run -label before new_expedition
```

Frames land in `vmtest/out/<script>/<label>/` at the logical 1152x800, so
pixel (x, y) is click coordinate (x, y). Compare two runs:

```bash
make vmtest-diff SCRIPT=new_expedition A=before B=after
```

To compare against a committed revision, run the same script from a
`git worktree` of that revision. Do not use `git stash`: the stash is shared by
every session.

Expected noise between identical runs: up to about 0.2% of pixels from
wall-clock tile animation (the tutorial's later frames sit closest to that
line). State should match exactly. Known game-side noise: the research and
crafting lists print multi-resource costs in map order, so those rows can swap
between runs.

## Configuration

`vmtest.json` at the repository root: the guest is `./cmd/game`; the sandbox
links `assets/` and `data/` except `data/config.local.json`, which is written
from the script header's `config` (your own local config is never used); saves
go to an empty `sandbox/saves/`. The screen size comes from the game's
`Layout`, i.e. `data/config.json`. `stable_id` is ignored by `diff` on top of
the engine's defaults.

## The hook

`internal/game/vmhook.go` (build tag `ebitenginevmguest`) is `lpHook`, the
game's `guest.Hook`. `Game.Run` wraps the game with `guest.Wrap`, which in
release builds returns the game unchanged. The current screen is
`StateMachine.Current()`. Its test runs with
`go test -tags ebitenginevmguest ./internal/game`.

### State summary

`state` (`title`, `dashboard`, `site`, `ship`, `settings`, `end`),
`title_screen`, `title_fields`, `dash_tab`, `status`, `selected_location`
(`id`, `name`, `fuel_cost`, `summary`, `history_summary`: the summary is the
site history's line; `tier`: the Star Map tag, `threat`, `hazard` or `quiet`), `legends` and `legends_text` (the Legends tab's
chronicle, when it is open), `legends_visible` (the chronicle lines on
screen), `legends_at_top`, `legends_at_bottom`,
`tick`, `hour`, `day`, `campaign` (`name`, `seed`, `location_id`,
`location_name`, `scenario`, `chapter` (the site history's chapter event, `""`
when quiet), `tier`, `known_events`, `adrift`, `in_orbit`, `won`, `lost`,
`known_techs`, `faction_count`, `presences` (location id → population),
`fleets`, `known_fleets`, `ships` (faction ship id → the site it is landed
on, `""` in flight), `diplomacy` (settlement faction → `neutral` /
`hostile`), `ship_fleets` (phase D: ship id → `doctrine`, `tier`, `landed`,
`to`, `mothership`, `returning`, `boarded`, `known`, `crew`) and `encounter`
(the pending or last interception: `fleet`, `doctrine`, `at`, `pending`,
`answer`, `outcome`, `fuel_cost`, `boarding`; null for none)), `faction` (on a site: its first faction presence
as built, `id`, `stage`, `population`, `bodies`, `morphs` (morph → count),
`types` (type → count), `launcher`; for the hive (phase E2) also `units`
(type → bodies, eggs and young as their type), `eggs` (type → eggs),
`larvae`, `resin` (its creep share), `biomass` (in its pools), `built`
(`hive_heart`, `biomass_pool`, `egg_cluster`, `evolution_chamber`,
`egg_launcher`, `resin_bloom` within its radius) and `ordered` (its
director's orders by kind)), `autonz` (on a site with an autonz settlement
built, phase E3: `id`, `stage`, `units` (type → workers), `tiers` ("enforcer
bronze" → workers), `built` (structure → standing within its radius),
`centre`, `conversion`, `techs` (its settlement's) and `ordered`), `rival` (on a site: its first faction settlement,
`name`, `faction`, `stance`, `workers`, `stage` and `level_stage` (the
presence's stage and the stage stamped on the level), `conversion` (phase E:
the converted share of its patch, read live, for a doctrine with a
conversion tile), `built` (what stands within 12 of its camp's centre, by
blueprint) and `ordered` (its director's orders by kind)),
`fuel`,
`hold`, `site_storage`, `site_structures`, `crew` (name, where, x, y, z, task),
`crew_at`, `crew_on_site`, `loaded`, `owed`, `tutorial` (`step`, `index`,
`count`, `done`, `skipped`, `ended`, `panel_covered`), `modal` (open modals, `incident` and `encounter` among them), `incident`, `notifications`,
`context` (the HUD's hover hint: `title`, `body`), `entity_inspector` (the open entity inspector: `title`, `blueprint`, `lines`, `text` (the lines joined); absent when closed, `entity_inspector` then not in `modal`),
`paused`, `pause_text`, `cursor_mode`, `build_mode`, `camera`, `selected`,
`settlement`, `view_room` and `view_module` (the named room and station
module at the view's centre), `in_view` (blueprint → count on screen),
`messages` (last 10), `observer_run` (an observer run), `spread_run` (a race run, phase F), `race` (the player's doctrine), `player_sites` and `player_site_count` (the worlds the player holds in a race run), `hud` on a site (phase F step 2: the doctrine's interface as drawn: `slots`, `resources` (slot → value), `orders`, `unit_modal` (the sections the unit modal drew last), `unit_title`, `research_title`, `research_hull` (the research modal's Hull section: a grey run's saucer, phase F step 6)), `panel` (the open structure panel's `title` and row kinds), `taken_over` and `launch_target` (the player's presence on the site: the rung groups taken over, the armed aim), `shown_presences`
and `shown_fleets` (what the Star Map shows of the factions: everything in
an observer run, the seen ones otherwise), `expedition` (the
run's expedition fleet: `landed`, `site` (the site it is landed on, "" in
flight or at the Wreck Site), `to`, `crew`, `fuel`, `last_move`), `viewing`
(the planet the viewer is on, "" on the ship or the dashboard) and `loaded`
(the planet IDs in memory), and on a site `location` (its ID), `view_label`
(the HUD's `VIEWING …` line), `observer` (observer mode: a run or the lock),
`rival` (the faction settlement on the level, an observer run's expedition
included) and
`settlement_stats`: `alive` and `dead` (colonists alive on the level, and
who died on it), `beds`, `lockers` (storage the colony owns), `food` (food
items stored or carried), `build_queue` and `tasks` (open build orders, all
open orders), `research` (a research order open), `techs`, `day`, `tick`,
`built` (structures the colony finished on this level, by type), `ordered`
(the director's orders by kind, `build:bed`, `research:<tech>`,
`mine:<resource>`, `haul:<blueprint>`, `craft:<recipe>`) and `cancelled`
(its orders given up as stuck). Fields that do not apply to the current
screen are absent or null.

### Targets

Names resolve to the element's centre on the current screen. A name that does
not resolve fails the step and the log lists every name that does. To list
them at the end of a script:

```bash
make vmtest-targets SCRIPT=new_expedition
```

| Kind | Names |
|---|---|
| `title:` | `new`, `load`, `settings`, `quit`, `name`, `seed`, `tutorial`, `factions_minus`, `factions_plus`, `race_prev`, `race_next` (the Race row, phase F), `launch`, `cancel`, `load_confirm`, `load_cancel`, `save:<campaign dir>` |
| `tab:` | `starmap`, `crew`, `inventory`, `quests`, `encyclopedia`, `legends` |
| `dash:` | the tutorial's names: `travelBtn`, `locateBtn`, `scanBtn`, `shipList`, `planList`, `beamDownBtn`, `beamUpBtn`, `returnSiteBtn`, `returnShipBtn`, `saveBtn`, `locList` |
| `row:` | `locList:<location id>`, `shipList:<n>`, `planList:<n>` (Star Map lists; a row scrolled out of view does not resolve); `locList:yours:<n>`: the nth world the player holds in a race run, scrolled into view |
| `star:` | `tutorial`, `next` (planet on the Star Map canvas) |
| `legends:` | `chronicle` (the Legends tab's scrolling chronicle), `datapads` (its "Datapads read" filter row, once a datapad was read) |
| `gi:` | `beam_down`, `beam_up` (Global Inventory tab) |
| `inspector:` | `beam`, `list`, `amount`, `all`, `close`, `item:<blueprint>` |
| `ship:` | `hold` (the supply pad, aboard the ship) |
| `hud:` | any HUD element ID: `cat_orders`, `cat_structures`, `order_dig`, `order_mine`, `order_default`, `build_<type>`, `sub_back` |
| `menu:` | the Esc menu: `save`, `load`, `newgame`, `quit` |
| `modal:` | `<modal>:close`, `<modal>:<element id>`, `<modal>:row:<list id>:<n>`, `<modal>:item:<list id>:<text>` for `colonistModal`, `craftingModal`, `researchModal`, `mainMenu`, `saveModal`, `loadModal` (mlge's `minuitarget.Modal` grammar) |
| `entity:` | `<blueprint>` (first on screen) or `<blueprint>:<n>` (nth on screen); `<blueprint>` also resolves aboard the ship (a grey run's vat and console) |
| `region:` | a level region tag, e.g. `tut_rations` |
| `tile:` | `x,y,z` (on screen, camera level) |
| `focused` | the tile the entity the last `debug focus` found stands on now (pause first so it holds still) |
| `inspect:` | `close` (the entity inspector's Close button) |
| `settle:` | `dx,dy` from the colony centre |
| `tutorial:` | `skip`, `manual`, `close`, `collapse` |
| `popup:` | `pause`, `travel`, `ignore` (incident popup); `evade`, `board`, `parley` (the Contact popup while an encounter is pending, phase D) |
| `btn:` | `dashboard` |
| `factions:` | `site:<location id>`: the Factions tab's first row that selects that site (a site it holds, or where a fleet is or is bound) |

`dash:`, `star:`, `ship:`, `inspector:`, `region:`, `entity:`, `tutorial:` and
`popup:` reuse the tutorial's highlight resolvers, so they work with the
tutorial off.

### The pregenerated galaxy

Every system is listed on the Star Map, so a row target scrolls its row into
view first. `row:locList:start` is the landfall system's row and
`row:locList:unidentified:<n>` the nth nearest seen system nobody has identified
(the galaxy's ids move with the seed); the location id `start` in any debug
command also names the landfall system, and `controls` (and
`row:locList:controls`) the gate's controls world; the gate's id is `gate`. State: `campaign.start_id`,
`campaign.galaxy_size`, `campaign.galaxy_shape`, `campaign.systems` (the
systems and the gate), `campaign.identified`, `campaign.seen` (systems and
the gate on the map), `campaign.sight_range`, `campaign.gate_identified`,
`campaign.location_identified`, `campaign.gate_on`, `campaign.gate_on_by`,
`campaign.controls_id`, and `title_fields.size` on New Expedition
(targets `title:size_prev`, `title:size_next`). A site a script needs is
found with `make history-dump HFIND=<terms>` (every system of the galaxy is
searched) and reached with `debug travel <id>` when it lies beyond the
starting fuel (200).

### Debug commands

| `debug` cmd | Does |
|---|---|
| `gate` | On the controls world's level, frames the gate's controls (dark or lit) down in their vault; click target `entity:gate_controls_dormant` orders them repaired |
| `fuel <n>` | n more fuel in the ship's hold (leaving through the gate without refining the reserve) |
| `travel <location id>` | Jumps the ship there for no fuel, as Travel Here does (a script reaching a site of the pregenerated galaxy beyond the starting fuel) |
| `see <id>` | Puts that system on the Star Map (seen), so a script can click its row beyond the ship's sight |
| `unlock <tech>` | The player knows the tech (a script scanning without researching the scanner) |
| `cheat <console command>` | Runs a cheat-console command on the live level (site or ship on screen) and returns the new message-log lines |
| `incident <location id or ship>` | Raises the away-damage popup for that loaded level |
| `reveal all` | Marks every history event known (a long Legends chronicle) |
| `focus <spec>` | Centres the camera on the first living entity on the level matching spec (a blueprint, `kin:<unit type>`, `datapad`, `storage`, or `structure`: a static entity with health or storage) and returns its blueprint and tile; click target `focused` to open the inspector on it |
| `datapad` | Puts the first colonist on the nearest lore datapad (the next update picks it up) |
| `module [walk] <id>` | Frames a station module (`Level.Modules`: `dock`, `generator`, `lab`, `cargo:2`, `wreck`); `walk` puts the first colonist inside |
| `deck <z>` | Frames deck z of a station on its main stair column (`station_deck:<z>`) |
| `days <n>` | Moves the star date on n days and steps the rival factions through them (a hive grows before the player arrives) |
| `place <blueprint> <dx> <dy>` | Puts a structure at the player settlement's centre plus (dx, dy), as built (a race run's structure panel without waiting days; phase F); a container is owned by that settlement, as a build gives it (a grey run's locker aboard is a door onto the ship's pocket) |
| `presence <location id> <doctrine> <pop>` | Puts a planet-carried presence of that doctrine on a frozen location, built when the level is (a race run meeting a rival on its world; phase F) |
| `aim <location id>` | Aims the player's launcher or signal on that world at the nearest world in its reach (a race run's spread fires only where the player aims it; phase F) |
| `freeze <location id>` | Freezes a loaded planet the viewer is not on, so `days` steps it by Tier 1 (a race run's world stays loaded while its brood lives there; phase F) |
| `faction [walk]` | Frames the camera on the first faction presence of the site on screen (its centre: the nest, the launcher); `walk` also puts the first colonist beside it |
| `stance <faction> <neutral\|hostile>` | Sets a faction's stance toward the player (a script watching a hive build without its guards on the landing party) |
| `tech <location id> <faction> <tech>` | The faction's presence there knows the tech in its record, from the dashboard (phase E3: its next spawns take it, so a script can show a bronze autonz without waiting on the day; the cheat console's `faction tech` does the same on a live site) |
| `ship <fleet id> tier <n>\|pop <n>\|hold <n>\|tech <tech>\|leave` | A faction ship's hull tier, crew (units from the stage mix), hold (emptied to n metal, so a trip's digging does not send it home) or a tech, or `leave`: its stay over, so it lifts off at the next day (phase E4: a grey warship to look at without waiting on the hoard; set before the jump, since the site's level is built on arrival) |
| `autonz [dx dy]` | Frames the camera on the autonz settlement of the site on screen (its centre, offset by dx, dy) |
| `rival [walk]` | Frames the camera on the first faction settlement of the site on screen (its camp; a race run's own brood is no rival); `walk` also puts the first colonist south of it |
| `boarding [walk]` | On a boarded ship's deck: frames the camera on the hull; `walk` also puts the first colonist inside it, so its view shows the hull and the crew (phase D) |
| `origin [walk [event type [n]]]` | Frames the camera on the site history's resolved chapter origin (the `region:lore_origin` tag), or on the origin of the site's event of that type; `walk` also puts the first colonist on the nearest free tile there, so its field of view reveals it; `n` picks the nth event of that type (the two sides of a per-side event) |

### The rival boxes

New Expedition's rival boxes (rival_selection_plan.md step 3) default to
Random, a real roll from the seed, so a run started from the title screen
does not seed the Rival factions count's lineup. A script that wants that
lineup (every script written before the boxes) sets `"rivalsByCount": true`
in its header's `config`: the title screen then ignores the boxes. Targets:
`title:rival_prev_N`, `title:rival_next_N`, `title:rival_group_minus_N`,
`title:rival_group_plus_N` (N from 0), `title:player_group_minus`,
`title:player_group_plus`; the sidecar's `title_fields.rivals` (each shown
choice), `title_fields.rival_groups`, `title_fields.player_group`, and
`campaign.factions` (every faction instance by key: `doctrine`, `group`,
`slot`, `name`; a key with a dot, `hive.2`, cannot be a path segment).

### Determinism

The runner sets `GODEBUG=randautoseed=0` on the guest and scripts type their
seed, so two runs of a starter script produce the same state at every frame.
Generated planets used to vary in enemy types and ore between same-seed runs:
the GameMaster's spawn pick and the asteroid primer's region order both
followed Go's map order. Both now iterate in a fixed order
(`TestSameSiteBuiltTwicePlaysTheSame` builds lore_ufo's site twice and
compares every body for 60 ticks). Anything that picks by walking a map must
sort its keys first, and new tiles go in a tile file that sorts last (tile
variants follow catalogue indices).

## Starter scripts

| Script | Covers |
|---|---|
| `galaxy` | The pregenerated galaxy: New Expedition at each size and three shapes (seed 1 small irregular, seed 2 medium ring, seed 5 large spiral) through `title:size_prev`/`size_next` and `relaunch`, only the landfall system identified and only the systems within sight of the landfall seen (`campaign.seen` 9 / 13 / 9); each map zoomed out (`scroll`) to show the galaxy haze; on the medium run two blind jumps (`row:locList:unidentified:0`, Travel reads Jump Blind) each identify their system and see round it, then `debug unlock ship_scanner_1`, Scan hovered (the range circle) and clicked (the report). Frames: each map and its `_whole` zoom-out, the blind jump selected and arrived, the second jump, the scan hovered and its report |
| `gate` | The gate (plan step 4), seed 3, no rivals: `debug travel controls`, four colonists down, `debug gate` frames the dark controls, a click orders them repaired; the party walks down the vault, fights its warden and guards (one colonist lost on this seed), works the controls and the gate comes on (`campaign.gate_on`, by `player`); the Star Map's lit gate, the jump there, `debug fuel 400`, `dash:leaveBtn` and the Through the Gate end screen. Frames: the controls world, the vault, the order, the party in the vault, the controls lit, the lit gate, Leave offered, the end screen |
| `new_expedition` | Title, seed, tutorial off, Rival factions stepper walked 1→4→0→1 (clamped at both ends), Star Map (the landfall system `sys_55` clean, fuel 200), travel (fuel charged), beam two down, To Site, 600 ticks |
| `factions_starmap` | Seed 777, Rival factions 3: presences seeded but unseen (the hive on its landfall, Ashen Basin `sys_25`, 207 fuel away), `debug travel sys_25` (a 69-day jump): the badge, the XENO GREAT NEST tag (named by morph) and the xeno egg in Legends |
| `factions_mirror` | Seed 19, Rival factions 2: the rival colony's ship is landed on its landfall, Rust Flats (`sys_57`, 28 fuel); from orbit the dart beside the site and the `RIVAL COLONY · NEUTRAL` tag; beam one down (it turns hostile, asserted) and land; its camp, four colonists and first beds after 600 ticks, framed with `debug rival walk` |
| `factions_greyish` | Seed 55, Rival factions 3: the greyish scout (`fl1`) landed on its landfall Scorch Reach (`sys_26`) with the mothership (`fl2`) over it; `debug days 4`, then jump to Saffron Fold (`sys_13`, 51 fuel), where the scout lands on day 18 (a jump to the landfall itself would meet the mothership): the saucer beside the site and the `GREY SCOUT · HOSTILE` tag; beam one down and land: the landed saucer, three typed grey crew and (phase E4) the scout's modules, a grey vat and a console (`rival.built.grey_vat`, `grey_console` asserted), framed after 900 ticks with `debug rival walk` |
| `factions_greyish_warship` | Phase E4, seed 118: the scout (landfall `sys_80`) met at its second site, Murk Basin (`sys_5`): a jump to Quiet Canyon (`sys_56`), `debug days 7`, the scout made a warship of eight there, a 3-day hop (`debug ship fl1 tier 2`, its techs, `pop 8`, `hold 0`): it lands with its vat, console, armory and two emitters and its research array on the ground (`rival.built.*` asserted), soldiers with beam rifles and a heavy; the greys set neutral. Frames: the camp, a day on, the landing scar after `debug ship fl1 leave` (no hull, no modules, the datapad), and `debug focus grey_array_wreck` (the wreck found: it is drawn only in view, and no colonist is near) |
| `factions_autonz` | Seed 65, Rival factions 4 (the count path's default lineup, which brings the autonz in): an autonz node seeded on its landfall, Gaunt Cradle (`sys_46`); `debug days 81` grows it into a hub at about 29% converted; jump there: the blue square chip with a stage pip and the `AUTONZ HUB` tag; beam one down and land: the hub stamp (a round wired hall, red-wired core), blue wire laid by its share, the fabricator and its locker, wire placers (the claw walker) and enforcers (the red-bar tripod), and since phase E3 the Signal Core, the cogitator and a relay pylon; framed after 900 ticks with `debug rival walk` |
| `factions_intercept` | Seed 55, Rival factions 3: jump to the greys' landfall Scorch Reach (`sys_26`, 83 fuel) under the greyish mothership: the Contact popup (Evade 8 fuel, Board, no Parley: hostile) stops the clock; Board: the mothership's deck as an encounter location, the Star Map with the mothership marker and `GREY MOTHERSHIP IN ORBIT · HOSTILE`; beam aboard and `debug boarding walk`: the hull, its crew and hoard |
| `factions_hive` | Seed 777, Rival factions 3 (as `factions_site`): `debug travel sys_25` to the hive's landfall, Ashen Basin (the 69-day jump grows it), and land; `debug stance hive neutral` keeps its guards off the landing party. The hive as a settlement (phase E2): the hive_core chamber laid with resin, the heart with grey eggs round it, coloured eggs by type, magenta egg clusters, violet biomass pools, the evolution chamber and the Resin Spire, olive drones, blue scouts, red warriors; `faction.units`, `faction.larvae`, `faction.built.*` asserted. Frames: the Star Map tag, the hive on its heart (`debug faction walk`), three days on (`wait 2250`: resin laid outward, blooms, eggs hatched), the Factions tab (morph names) |
| `factions_autonz_hub` | Seed 65, Rival factions 4, `debug days 120`: Gaunt Cradle's (`sys_46`) autonz hub as a settlement (phase E3), landed with the autonz set neutral: the Signal Core on the red core, the cogitator, the foundry, a relay pylon at the wire's edge, two defence pylons, wire placers, bronze enforcers, a harvester, a relay drone, a seeker and a sentinel; `autonz.built.*` and `autonz.units.*` asserted. Frames: the hub (`debug rival walk`, `debug autonz 0 -2`), a day on, the Factions tab (units by type, bronze marked, the structures) |
| `factions_autonz_observer` | The same hub in an observer run, `debug days 120` and `debug tech sys_46 autonz autonz_bronze_enforcer` (by day 120 it has learned it anyway) before viewing it: its enforcers materialise bronze (one row down on the sheet) beside its steel wire placers (`autonz.tiers` asserted). Frames: the Factions tab, the hub viewed, a day on |
| `factions_hive_observer` | The same hive in an observer run (header `config` `observerMode`), Rival factions 3, `debug days 36`: the Factions tab names the hive and lists its units by type and larvae (`factions:site:sys_25`); View opens Ashen Basin at work. Then, paused, the entity inspector (`debug focus` and click `focused`): a drone (`kin:drone`, `entity_inspector.text` asserted), a structure (the Brood Heart) and a storage container. Frames: the Factions tab, the hive viewed, a day on, `inspect_drone`, `inspect_structure`, `inspect_storage` |
| `encyclopedia_hidden` | Hidden identity in the Encyclopedia: as `factions_hive`, a colony lands beside the grown hive on Ashen Basin; the cursor rests on the Brood Heart (`debug focus`, `move` to `focused`: catalogued on hover) and its Encyclopedia card is opened (its row clicked by position): no Faction line or sentence, no `id: hive_heart` footer. Frames: the heart hovered, the card |
| `factions_site` | Seed 777, Rival factions 3: `debug travel sys_25` (the hive's landfall, Ashen Basin; 69 days), the XENO HIVE badge and the egg it fired drawn in flight; land and `debug faction walk`: the hive_core chamber, the hive's drones, scouts and warriors by type (a settlement since phase E2), eggs and the Resin Spire launcher |
| `tutorial` | All 24 tutorial steps with real input, a frame per step, ends on the end panel and closes it |
| `incident` | Away-damage popup on the ship: clock stops, Travel goes to the planet; again, Ignore resumes the clock |
| `save_load` | Land, save, relaunch, load: day, location, fuel, hold, storage and crew match; back to title via the Esc menu; Q exits with status 0 |
| `storage_beam` | Ship hold inspector beams metal down, site locker inspector beams some back; totals conserved |
| `esc_menu` | Land on the start site, open the Esc menu (title, button labels), close it with Esc |
| `lore_site` | Seed 777: the landfall system's (`sys_55`, hazard) Star Map summary is its history's line and its tier tag matches, travel, beam one down, land on the site |
| `lore_xenos` | Seed 13: a blind jump to a `wreck_eggs_hatch` site (`sys_23`), land, walk to the origin: the old wreck's hull with the hive's eggs |
| `lore_ufo` | Seed 1372: a blind jump to a `ship_shot_down` site (`sys_10`), land, walk to the origin: the hull and its crew |
| `lore_autonz` | Seed 22: jump to a `wreck_nanites_wake` site (`sys_3`), land, walk to the origin: the wreck with wires spreading from it |
| `lore_legends` | Seed 13: land on the wreck site (`sys_23`), pick up a lore datapad, open Legends: the datapad's event is in the chronicle; the "Datapads read" filter (`legends:datapads`) lists what it said |
| `observer_datapad` | Seed 1512 as an observer run, lighting off: View the xeno wreck site (`sys_10`), `debug focus datapad` and click it: the inspector shows the datapad's text; `campaign.known_events` stays 0. Frames: the site, the datapad read |
| `lore_legends_long` | Seed 746: every event of the galaxy known (`debug reveal all`, 761), Legends opens on the newest events, the wheel scrolls back to the oldest |
| `lore_quiet` | Seed 75: the quiet landfall system's (`sys_25`) collapsed-civilization castle stands empty (`debug origin walk civ_collapses`); then the native people's dwelling |
| `lore_hazard` | Seed 44: a scan from the Wreck Site (`debug unlock ship_scanner_1`) identifies it, then the Star Map's amber Hazard tag on a `dormant_nest` earth_like site (`sys_4`), then the eggs by the wreck at its origin |
| `lore_war` | Seed 56: the landfall system (`sys_62`) is a `sundered_world` earth site; walk to each side's fort (`debug origin walk fortify 1` and `2`), the battlefield and a side's dwelling |
| `lore_ship` | Seed 11: the landfall system's (`sys_1`, alien) old warship wreck (`crashed_ship`, purpose warship), lighting off, walked to: hull, sections, rooms |
| `observer` | An observer run (`observerMode` in the header config at New Expedition), seed 777: no player ship; the expedition fleet launches off the Wreck Site on day 1 and lands on its landfall system Gleaming Cradle (`sys_55`); the viewer sees every faction from day 0 (the Factions tab; its hive row, `factions:site:sys_4`, selects Quiet Expanse on the Star Map, tagged `HIVE/XENO BROOD 3`); View opens the expedition's planet (six crew, the settlement director building), leaving unloads it, then View opens Wether Narrows (`sys_43`), a planet nothing is on. Frames: the Star Map adrift, the Factions tab, the hive at day 0, the Star Map after the landing, the expedition's colony at once and a day on, Wether Narrows, the Star Map after, and the hive on Quiet Expanse viewed from its Factions row (the camera on the brood, `faction.bodies` at least 1, unloaded again on leaving) |
| `race_hive` | Phase F step 1: New Expedition seed 777, the Race row stepped to the hive: no ship, the run opens viewing the brood on its landfall system (`sys_55`) (`spread_run`, `race`, `player_site_count` 1, `faction.units`); the larva's crafting modal (drone and scout eggs, the locked types named) queues a drone egg; the evolution chamber opens the research modal over the evolve tree; the Star Map (`Your worlds`, no Travel, Beam or Scan); `debug freeze start` and `debug days 60`: an egg has founded a second world (`player_site_count` 2), which `row:locList:yours:1` and View open. Frames: the race row, the brood, the larva modal, the evolve modal, the brood later, the Star Map at day 0 and 60, the second world |
| `race_hive_ui` | Phase F step 2: a hive run's own interface from the doctrine's interface block: the resource bar is biomass and larvae (`hud.slots`, `hud.resources`), Orders has Forage and Lay Resin and no Sleep (`hud.orders`), the unit modal of a drone is titled Brood with its type and no equipment (`hud.unit_modal`), the research modal is Evolve; a Lay Resin click queues a convert order; the Crew tab lists the brood grouped by type. Frames: the HUD, the Orders menu, the unit modal, the resin order, the Crew tab |
| `race_hive_takeover` | Phase F step 3: the Brood Heart's panel (a text row and the rung-group toggles: `panel.title`, `panel.rows`); breeding taken over (`taken_over`); a Resin Spire placed by `debug place` opens its panel (readiness, the worlds in reach, Aim at); aiming at the first world takes the launch over; Load sends the shot to hold round it; "I run this world" (`panel_group_*`) takes every group over. Frames: the heart's panel, breeding taken, the Spire's panel, aimed, loaded, the world run by hand |
| `race_hive_rival` | Phase F step 4: a hive run at Rival factions 2 is seeded the rival colony and no rival hive (`campaign.diplomacy.mirror` neutral); a rival colony camp put on the brood's world (`debug presence`), the world viewed again, a drone walked to the camp (`debug rival walk`): the colony turns hostile. The Star Map tags the hive's own world `YOUR NEST 8` and its row `[yours]`; the Factions tab has a `YOUR HIVE` block with its units by type. Frames: the Star Map, the two camps, the colony hostile, the Factions tab |
| `race_autonz` | Phase F step 5: an autonz run (the Race row stepped twice, `race.order`): the node on the start system, the HUD's metal, crystal, ferrite and conversion (`hud.slots`), Assimilation, Lay Wire and Deploy Relay (`hud.orders`); a wire placer's unit modal (type and tier, no needs); a Lay Wire click; a Signal Core placed outside the node opens its panel (readiness, worlds in reach, Transmit here, the group toggles); the node frozen and `debug days 200`: its signal founded more worlds (`player_site_count` 2 or more), the second viewed. Frames: the race row, the node, the unit modal, the core's panel, the Star Map at day 200, the second world |
| `race_autonz_takeover` | The take-over switches on an autonz node with no Signal Core: Orders → Take Over opens the world's panel (`interface.world_panel`: `panel.title` This World, rows world toggle, text, group toggles); taking printing over is recorded (`taken_over`); the cogitator's research modal with the cursor on an entry whose description is the `_draft` placeholder draws no tooltip. Frames: the world panel, printing taken over, the research modal hovered |
| `race_greyish` | Phase F step 6: a grey run (the Race row stepped three times): a ship run, the dashboard adrift with three greys aboard and no tutorial; aboard the saucer (`hud.slots`: metal, crystal, alloy, plastic, specimens), the vat opens its print rows and the console the grey tree with its Hull section (`hud.research_hull`); a locker placed aboard lists the hold (the pocket); Expand Hull clicked on deck 1 and 3000 ticks: the rim breached and the ring's floor spreading; then travel, all three beamed down (`hud.orders` without Sleep), standing after 600 ticks. Frames: the race row, adrift, the saucer, the vat, the console, the locker, the hull growing, the site, the site after 600 ticks |
| `rivals_title` | Rival selection step 3: an observer run (seed 321) with Rival factions 4 picked in the boxes: rival 1 Hive, rival 2 Hive moved into rival 1's group, rival 3 Random, rival 4 Rival Colony (`title_fields.rivals`, `rival_groups`); after launch `campaign.factions` (the hive in group 2, slot 1; the rival colony `colony!rival` in group 5). Frames: the boxes, the Factions tab (HIVE and THE ASH BROOD apart, the second hive by its draft instance name; the rolled Random, the rival colony) |
| `race_hive_vs_hive` | Rival selection step 3: a hive run (seed 777) with rival 1 picked as Hive: `campaign.factions.hive` (doctrine hive, group 2) beside `campaign.factions.player` (group 1); its brood put on the player's world (`debug presence start hive 8`) and viewed. Frames: the boxes, the two broods, the Factions tab (XENOS beside YOUR HIVE) |
| `lore_station` | Seed 114: a blind jump to the quiet station `sys_73`, land at the dock (`view_module` dock, room "Docking Bay"), `debug module walk generator` (room "Main Generator", a `station_generator_broken` in view), then decks 0-2 from the main stair column; lighting off |

Seeds for the lore scripts come from `make history-dump HFIND=<terms>`
(comma-separated, all required: tier, map, location ID, event type, `stamped:<type>` for an event that keeps its structure, `<var>=<value>` for an event var such as `purpose=warship`), which
lists New Expedition seeds with a matching site on the opening Star Map and
the jump's fuel; pick one with low fuel so its row is on screen.

## Release builds

```bash
make vmtest-check
```

Runs `go tool vmtest check`: `cmd/game` must not depend on the VM host or the
vmtest host packages, an untagged binary must contain neither mlge's sidecar
entry point nor `lpHook` (`check_symbols` in `vmtest.json`), and the tagged
binary must contain both, so the check can fail.
