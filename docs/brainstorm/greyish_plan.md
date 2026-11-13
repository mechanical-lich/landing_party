# The Greyish as a Full Settlement Faction: Implementation Plan

Status (2026-11-09): **plan, not agreed**. Thirteen open questions were
decided by the owner on 2026-11-09 (see "The owner's decisions"); the rest
carry a recommended default. It is a new phase after the autonz (E3) and
before Phase F's step 6 ("ship mode with a grey crew"), proposed as **Phase
E4** (open question 20). It was written against `dev` at `5e7e783` (the
squashed factions work: phases A to E3, observer mode v2, the review fixes),
checked against the code and data there, not against the docs. All draft
text here (unit, structure, item and research names, alerts, Legends lines)
is placeholder that the owner rewrites (see "Where the owner rewrites
text"). New strings go in marked `_draft`.

Read with: [hive_plan.md](hive_plan.md) and [autonz_plan.md](autonz_plan.md)
(types, the trees, structures by stage, the director rungs, materialise and
reconcile, and both "As built" sections, which this plan follows where they
differ from their own plans); [enemy_factions_plan.md](enemy_factions_plan.md)
("Phase D as built"); [factions.md](../developer/factions.md) ("Ships", "The
greyish", "Interception", "Ruins", "Faction settlements", "The director");
[playable_races_plan.md](playable_races_plan.md) on
`claude/factions-f-7be267` (section "Greyish", step 6, and the `interface`
block that step 2 is building).

This plan is the hive's and autonz's in shape. It is different in one way
that matters: the greyish do not hold planets. A grey presence is a ship
that lands, works for at most thirty days and leaves. Everything here
(research, a workbench, a bio-printer, defences) has to make sense inside
that short stay, and has to leave Tier 1's pace exactly where it is.

## Decision

The owner's direction (2026-11-09), taken as given:

- **The greyish are the high-tech, expensive faction**, the opposite
  extreme from the hive and the autonz (cheap, numerous, spreading), with
  the colonists in the centre. Few bodies, each individually strong. Every
  body and every weapon is costly and slow to make. "Expensive and elite"
  is made measurable in "Identity in numbers".
- **Research stations, workbenches that make weapons, and research that
  unlocks new units at the bio-printer.** A real tech tree under a
  `greyish` tree key in `research.json`. The three placeholder entries
  become real entries under the same ids.
- **The ship is the settlement.** What the greys need on every landing (the
  bio-printer, the research console, from the freighter tier the armory)
  is carried aboard as **modules**: unpacked when the ship lands, packed
  when it lifts off. What only pays on a long stay (the research array,
  defences, extra storage) is **built on the ground** by the director and
  left behind as **wrecks** in the ruin.
- **The mothership is the bank.** Ships haul metal, crystal and the refined
  alloy home. Hull tiers stay hoard purchases, exactly as now.
- **Tier 1 does not move.** Every tech effect is live-only (on the level)
  or Tier 3-only (interception and boarding). No tech changes a ship's
  yield, crew cap, stay, range, hold target, tier cost or the mothership's
  ship cost. The greyish pace is captured as a table **before** any change
  (step 0) and must stay byte-identical.
- **Everything is data**: `data/factions.json`, `research.json`,
  blueprints, recipes and structure scripts, so that F's step 6 can hand
  the greys to the player with no new code path. The player's side is
  described ("Phase F: what a greyish player will see") but not built.

### The owner's decisions (2026-11-09)

Thirteen of the open questions were decided; they are out of the open list
(the rest keep their numbers):

- **Q1, the roster:** grey = worker, white = technician, pink = soldier,
  green = heavy (the default).
- **Q3, the flies:** the green fly (skimmer) and the purple fly (lancer)
  are bio-printed grey constructs (the default).
- **Q4, specimens and alloy:** specimens come from wildlife kills only, no
  colonist abductions (the default). **Not the default on alloy:** the
  greys' refined material is a new grey-only item, `grey_alloy`; the
  existing `alien_alloy` relic, its lore and the `starfaring` loot table
  are untouched.
- **Q6, grey gear for the player — not the default:** rare usable drops. A
  dead grey's beam weapon or field armour usually breaks with its owner
  and only occasionally drops intact, usable by colonists (`GearDrop`
  chances on the items; see "Items"). Wrecks still drop `grey_alloy`.
- **Q9, art:** tinted placeholders; the owner swaps real art in later (the
  default).
- **Q16, the player's win:** ship mode's reach-Home win, unchanged (the
  default).
- **Q21, planets for a grey player:** support only (linked lockers, the
  armory, emitters, mining; the vat, console and array ship-only), and
  every grey locker, on a planet or aboard, is a linked beam locker of the
  kind the colonists' starting locker is (see "Linked lockers").
- **Q23, a colony use for grey alloy:** "Stores for now" (the default):
  no colony recipe takes it this phase.
- **Q24, hauling between lockers — the storage pocket:** "Greys haul from
  storage to build site or mining location to storage. But they don't have
  to haul from one storage locker to another. All lockers have access to
  the same inventory on the planet. It's 'inter-dimensional'." Every grey
  locker on a level is a door onto one shared inventory (see "The storage
  pocket").
- **Q25, the pocket for rivals:** "Rivals get this perk too. But not all
  greyishs share the same inventory. So rivals get their own
  inter-dimensional storage pocket." Each grey owner (the player, and each
  rival grey presence) has its own pocket; pockets are never shared.
- **Q26, the pocket's scope:** one pocket per owner per level (the
  default): a grey player's site pocket and ship pocket, joined by the
  beam while in orbit; a rival's pocket on its site.
- **Q27, the last door destroyed — not the default:** the pocket spills on
  the last door's tile, so smashing every grey locker is a way to loot a
  grey camp. A door destroyed while others stand still moves its slice
  into the survivors.
- **Q28, stored gear in a spill:** an unequipped grey weapon or armour
  piece in a spilled pocket rolls its `GearDrop` chance (the default), so
  the spill does not bypass the rare-gear decision.

Constraints carried over: physical storage (workers haul everything,
nothing teleports, and nothing is created from nothing), with exactly
three exceptions: the materialise floors the autonz already set as
precedent; the beam between the ship and the site it orbits, which the
colony already has and grey lockers reuse; and **the greys' storage
pocket**, under which an item stored at one grey locker can be taken out
at any other grey locker of the same owner on the same level. Hauling
stays physical at both ends: a worker carries every item from where it is
mined or dropped to a locker, and from a locker to the build site, bench,
vat or console.
scenarios and maps stay random; no CI test job (tests, soaks and vmtests
run locally); the owner commits; `data/config.local.json` is never touched.

## Identity in numbers

The target the design is tuned to. "Hauled units" are items a worker
carries (one ore, one crystal, one biomass, one alloy each count one), so
the factions' costs compare on one scale; a refined item counts what went
into it (a grey alloy is 3 metal + 2 crystal: **5 units**; a ferrite is
2.5). "Fight value" is a rough per-body number: health × the expected
damage of its best attack (dice average + attack bonus), before armour.

| Measure | Hive | Autonz | Colony (player) | Greyish today | **Greyish target** |
|---|---|---|---|---|---|
| Cheapest body, hauled units | drone egg **2** biomass | wire placer **4** metal | colonist **25** biomass (bio-printer); hauler robot 13 | grey **7** (6 metal, 1 crystal) | worker **16** (8 metal, 3 crystal, 1 alloy) |
| Dearest body, hauled units | spitter **6** | warden **22** | security robot **18** | pinkish **10** | lancer **52**; heavy **45** |
| Weapon on top of the body | natural (eggs hatch armed) | natural, or a laser pistol in the type's kit | gun bench, no research: laser pistol 2 crystal | none (tentacles) | armory, after research: beam rifle **15**, disruptor **32** |
| Make time per body (build_time) | 60–120 | 120–240 | 150 (colonist), 120–160 (robots) | 120 | **450–1,200** |
| Health, typical / top | 14 / 32 | 8 / 30 | 20 (+ armour) / 35 | 12 / 12 | 18 / **50** |
| Fight value, line / top | 35 (drone) / 224 (warrior) | 66 (enforcer) / 210 (warden) | 110 (colonist, laser pistol) / 193 (security robot) | 42 / 78 | **352** (soldier, beam rifle) / **825** (heavy, disruptor) |
| Fight value per hauled unit (line fighter) | 56 (warrior) | 9.5 (warden) | 4 (colonist + pistol) | 8 | **8** (soldier + rifle) |
| Bodies per site | 6–24 (capacity) | 4–16 (conversion cap) | 4–12 | 3 / 5 / 8 (ship tier) | **3 / 5 / 8, unchanged** |
| Spread | 6–9 sites by day 300 (Rival factions 1) | 6–9 sites by day 300 (+ dormant) | one ship, outposts | none: 1–3 ships, 4–12 landings in 300 days (step 0's table) | **unchanged** |
| Research, live: steps per entry and who | 900–1,800, any drone | 900–1,800, any placer | 60–220, any colonist of Int 8+ at a lab | none live | **500–1,200, technicians only** (1–2 per ship) |
| Research, Tier 1 | 1 a day + nothing | 1 a day + assimilation 1–2 by stage | — | 1 a day **while landed** (3 entries) | spine unchanged; a second walk while landed, 0.5–1.5 a day by hull tier |

Read as targets: the greys have **the highest fight value and cost per
body, the fewest bodies, the slowest prints and no spread**. Per hauled
unit they are about as efficient as the autonz and far worse than the hive,
so the hive and the greys are the two extremes on cost per body and bodies
per site, with the colony between them. The soak checks the live
quantities (bodies, prints, research per landing); the cost and stat
columns are data and a content test asserts the ordering (every grey type's
hauled-unit cost above every autonz type's and every hive egg's; the grey
soldier's fight value above the colonist's and the security robot's).

**Today's greys are cheaper than a colonist** (7 hauled units against 25,
at 120 against 150 build time) and weaker (12 health, 1d6, Int 4). This
plan reverses both.

## The art available

### Bodies: `scifi_entities`, row y=408

`assets/entities/oryx_16bit_scifi_creatures.png` (768×984, 24 px cells).
The greys are drawn through `EquipmentAppearance`: a block origin, two
frames per column (`AnimationFrames` 2), and `WeaponColumns` choosing the
column from the held weapon's `Display` (or `*` for any weapon). Measured
on the sheet for this plan:

| x | Silhouette | Columns (x of the first frame) | Today | Proposed type |
|---|---|---|---|---|
| 0, 24 | grey, big black eyes | col 0 bare (0); col 1 with a gun (48) | `greyish`, `greyish_crew` (hauler) | **worker** |
| 96, 120 | white | col 0 bare (96); col 1 gun (144) | `whitish`, `whitish_crew` (scientist) | **technician** |
| 192, 216 | pink, brain dome, green tunic | col 0 bare (192); col 1 gun (240); col 2 tentacle legs (288) | `pinkish`, `pinkish_crew` (soldier) | **soldier** |
| 336, 360 | green, brain dome, blue tunic | col 0 bare (336); col 1 gun (384); col 2 tentacle legs (432) | `greenish`, `greenish_crew` (miner) | **heavy** |
| 480, 504 | green fly, orange compound eyes | col 0 wings folded (480); col 1 wings out (528); col 2 wings out, legs hanging (576) | **unused** | **skimmer** (flying) |
| 624, 648 | purple fly, blue compound eyes | col 0 folded (624); col 1 wings out (672); col 2 hanging (720) | **unused** | **lancer** (flying) |

Notes:

- **The gun column is already wired.** Every grey crew blueprint has
  `"*": 1`, so a grey holding any weapon is drawn with a gun. Pink and
  green have `"tentacles": 2`, and their crew start with the `tentacles`
  item, so today's soldiers and miners walk in the tentacle pose. Grey and
  white have only two columns (their column 2 would read the next body).
- **The flies are not greys** by silhouette. They sit on the same "alien
  contact" row and are the only unused bodies there, so this plan uses them
  as bio-printed constructs (decided 2026-11-09). Their column 1 (wings out)
  is the flying default (`DefaultWeaponColumn` 1), and column 2 (legs
  hanging) is drawn when the natural weapon is held (`"*": 2`), so a lancer
  in a fight looks different from one patrolling.
- **No armour rows.** `EquipmentAppearance.ArmorRows` picks a row by armour
  tags; the greys have one row only. A grey wearing the field harness looks
  the same as one without it. **Art gap.**
- **The heavy is the same size as everyone else.** The green body differs
  from the pink only in colour. A heavy reads as "the green one", not as a
  bigger alien. **Art gap** (a larger silhouette would help).

### Structures: `scifi_world`

`assets/tilesets/oryx_16bit_scifi_world.png` (840×984, 24 px). What is
referenced today near the tech furniture: `bio_printer` (672,24), the
station generator / fuel refiner / autonz converter and defence pylons
(696,24), the relay pylon (720,24), the airlock (528,48), the research lab /
station console / cogitator (576,48), and every workbench (168,120).
**Unused**, measured for this plan:

| Cells | What | Proposed use |
|---|---|---|
| (744,24) over (744,48) | a glass pod, empty, green light in its base | **`grey_vat`** (the bio-printer). The pod is two cells tall; a one-tile entity shows the base (744,48) only |
| (768,24) over (768,48) | the same pod, cracked, red light | `grey_array_wreck` |
| (792,24) over (792,48) | the pod filled red, dark light | spare (a vat at work, if the engine ever swaps a station's sprite while crafting) |
| (552,48) | a floor computer with keyboard | **`grey_console`** (tinted green) |
| (600,48), (624,48) | wall-mounted computers | **`grey_array`** (624, tinted teal) |
| (648,48), (672,48), (696,48) | screens: broken, cracked, dark | `grey_console`'s and `grey_armory`'s wrecks if they ever need one |

There is **no grey workbench, no turret and no grey hull tile art**. The
armory borrows the shared bench (168,120) tinted green, as the autonz
foundry did. The emitter turret has no art at all: `sentry_turret` is drawn
as the autonz' red orb (240,936), which would read as an autonz unit.
**Art gap** (decided: a tinted placeholder until the owner swaps real art
in). The saucer hull is `hull_wall` and
`hull_floor` (`crashed_ship.basic`, `saucer`), the colony's own tiles.

### Items: `scifi_items`

16 px cells. The greys' refined material is a **new grey-only item,
`grey_alloy`** (decided 2026-11-09). The existing `alien_alloy` relic
(`data/blueprints/items/relics.json`: "a plate of metal no human foundry
could pour", tags `relic`,
`trade_good`, in the `starfaring` loot table) stays exactly as it is, its
lore and loot untouched. Both are drawn with metal ore's sprite (320,16)
today; `grey_alloy` gets a teal tint of it as the placeholder. **Unused**: glowing orbs (0–64,32: blue,
green, red, purple, yellow) and power cells (0–80,48). Proposed:
`specimen` at the green orb (16,32). A real `grey_alloy` icon is an **art
gap** (the teal tint stands in).
There are no beam-weapon icons: the grey weapons are the colony's pistol
(32,80), rifle (0,80) and rocket launcher (64,80) **tinted green**. **Art
gap**.

### Vehicles: `scifi_vehicles`

`assets/entities/oryx_16bit_scifi_vehicles.png` (576×384) is **not
referenced by anything**. Its grey row (y=240–287) holds a fighter in four
directions (x=384, 432, 480, 528 at y=240) and a rounded hull in four
directions (the same x at y=264). The Star Map draws saucers and the
mothership procedurally (`drawSaucerMarker`). Nothing in this plan needs
them; noted for a lift-off or landing effect later.

## Where the code is today

- **Doctrine** (`data/factions.json` `greyish`): `seedable` from the third
  rival faction, `seed_population` 3, hostile, `mobility.kind` fleet (range
  50, speed 5, stay 30 days), economy `ore`, growth 0.05 a day, capacity
  base 6 (ore +4, salvage +4, radioactive +2, biomass −2), one stage
  `landing` (17×17), aggression 6, kit 12 metal and 6 crystal. Its Tier 1
  walk is `grey_hull_plating` 30, `grey_beam_weapons` 45, `grey_cloning`
  60 at `research_per_day` 1.
- **Ships** (`ships`): marker `saucer`; tiers scout (crew 3), freighter (5;
  40 metal, 15 crystal), warship (8; 90 metal, 40 crystal); `hold_target`
  30; `yield` per crew member a day by site tag (base 0.15 metal; ore 0.25
  metal + 0.1 crystal; salvage 0.2 + 0.05; radioactive 0.1 crystal); the
  mothership (crew 10, at least 60 from Home, a new ship for 60 metal and
  20 crystal, three at most).
- **Settlement block**: crew `greenish_crew` 0.4 miner, `greyish_crew` 0.3
  hauler, `pinkish_crew` 0.2 soldier, `whitish_crew` 0.1 scientist (split
  by largest remainder); `needs: false`; buildables `storage_locker`;
  `research: []`; units `print_greenish` … `print_whitish` at
  `bio_printer` (6–8 metal, 1–4 crystal, 120); camp the `crashed_ship`
  stamp, `class: saucer`, `crew: dead`, 17×17, station `bio_printer`.
  Director: grow, mine (metal 200, crystal 80), one locker, dig (depth 4,
  reach 60), raid (unarmed, soldiers only, fighters 2).
- **The crew bodies** (`greyish_crew.json`) are the scenario greys
  (`greyish.json`) without scripted AI: health 12, Str 8, **Int 4**, Dex
  12, AC 8, 1d6, Initiative 12, skill `radiation_heal`. No `Kin`, no
  `NeverSleep` (so, like the autonz before E3's fix, they stop at night).
- **The three research entries** (`research.json`) are `tree: "greyish"`,
  durations 600, 900, 1200, no cost, no building, no effect. Only
  `faction_greyish_test.go` names one.
- **Tier 1** (`internal/campaign/faction_ship.go`): `stepShip` (line 101)
  grows a landed ship to its site's capacity under its tier's crew cap,
  digs its hold by tags (`shipYield`), walks `d.Research` at
  `research_per_day` **only while landed and off a loaded site**, and
  leaves at the stay's end or a full hold. `dock` (218) puts hold and techs
  into the mothership, copies the mothership's techs back, buys the next
  tier when the hoard covers it, and sends the ship to a free site in range
  of the mothership, or leaves it **waiting docked** (re-docking each day)
  when there is none. `stepMothership` (272) orders a ship when the hoard
  covers `ship_cost`; `buildShip` (300) builds it with the mothership's
  techs and **an empty hold** (a re-launched ship's hold is empty too: only
  the seeded scout ever carries the kit). `shipCandidates` (363) skips any
  site with a planet-carried presence, any site another ship holds or is
  bound for, and the mothership's site. The second walk the autonz use
  (`Assimilation`, `AssimPoints`, `faction.go:1110`) runs only for
  planet-carried presences, never for ships.
- **The mothership destroyed** (`faction_ships_d.go:394`,
  `DestroyMothership`): the hoard is loot, tiers freeze, no ship is built,
  ships bound home scatter.
- **Live** (`internal/game/faction_ship_live.go`): a ship bound for a loaded
  site lands at once (`materialiseSettlement`, the generic path;
  `buildStampedCamp`, `faction_settlement.go:229`, stamps the saucer,
  claims its cockpit locker as the hold, opens its doors to the faction and
  places the station **unowned** by the centre); `liveHold` brings the
  record up to date each day; `liftOffLive` (155) removes the crew, empties
  the settlement's storage and drops the settlement. **Everything else on
  the level stays**, including the bio-printer.
- **Ruins** (`internal/game/faction_ruin.go`, `stampRuin` line 49): a
  departure leaves the camp as a ruin: the saucer hull broken by
  `ruin_pass` at damage 2, storage emptied, a datapad. So **the hull the
  ship flew away in is left behind**, and so is its bio-printer, which the
  player can use (colony recipes run at any `bio_printer`:
  `print_colonist` from 25 biomass). Both are oddities this plan fixes
  (see "Ruins").
- **Boarding** (`faction_encounter.go`): a boarded ship's crew spawn in
  their fighting forms (`greenish_crew` fights as `greenish`, scripted AI);
  the hold is in a locker aboard.
- **What already exists and this phase reuses**: E2's `types` with
  `ApplyType`, `Kin.Type`, `TypesUnlocked`, type `fallback`,
  `type_fallback`, upgrades applied to living units (`factory.Retype`);
  recipe `kin_type`, `requires_tech`, `count`; typed prints that become
  workers of the crafter's settlement (E3); buildable `required_tech`,
  `hidden` and `deploy_only` (E3's pattern for a buildable the build rung
  refuses); the research rung (`required_building`, the worker's `Int`
  against `required_int`, `AllowedTasks`); the craft rung (`craft`, `min`);
  `guard` with `posts` by structure; `patrol` and `Site.Scouted`; raid
  `scouted`, `leader`, `roles`, `unarmed`; `kill_drops` and the hunt rung
  (the hive's biomass from kills); `orderEquip` (it equips any gear,
  armour included; only the raid rung's `arm` calls it, and only for
  weapons); `sentry_turret` (a stationary `ScriptedAI` body with a weapon,
  `sentry_turret.basic`, whose target pick is `find_nearest_worker`, which
  E2 made skip the caller's faction); `Drops` on death; `FactionPresence.Units`
  and `Built` (E3); `NeverSleep`.
- **Weapons and armour the colony has** (`weapons.json`, `armor.json`, the
  `gun_bench` and `advanced_armor_bench` recipes): laser pistol 1d6+2 range
  8 (2 crystal, no research); rifle 1d8+3 range 10; autorifle 1d6+4 range
  8; rocket launcher 3d6+3 range 12 (8 metal, 2 crystal); laser sword 2d8+4
  melee (6 crystal). The best armour is the siege set (`super_heavy_*`,
  torso defence 6, stopping power 5, after `powered_exoskeleton`). **No
  colony weapon needs research**; armour does. The colony's ship scanners
  (`ship_scanner_1..3`, `scan_efficiency_1..2`) are the Star Map side.
- **Phase F** (`claude/factions-f-7be267`): step 2 is adding a settlement
  `interface` block (`internal/campaign/doctrine_interface.go`, uncommitted
  on 2026-11-09): `resources` (bar slots by source), `unit_label`,
  `units_label`, `research_label`, `orders`, `unit_modal` (section kinds),
  `crew_group` (`""`, `type`, `type_tier`), `panels` (a structure's title
  and rows of kinds `readiness`, `target_list`, `action`, `text`) and
  `groups` (rung groups the player may take over). Its step 6 waits for this
  plan's content. If F lands the block differently, this plan follows what
  F built.
- A day is 750 ticks. A research `duration` is in **work steps** (one per
  researching worker's turn, roughly ten ticks; E3's finding). A stay of 30
  days is 22,500 ticks, about 2,250 work steps for one worker working full time.

### Tier 1 today (the baseline this plan must not move)

`TestFactionPaceRivalOne` runs at Rival factions 1, which seeds only the
hive: **it never sees a grey**. `make history-dump` lists every fleet as
`egg …` (`cmd/history-dump/factions.go:157`), so **the greyish pace is not
visible in any table today**. Measured for this plan with a throwaway test
(seeds 1–6, Rival factions 3, no player, Tier 1 only; not committed):

| Seed | d40 | d100 | d200 | d300 | Landings to d300 | Hoard d300 (metal / crystal) | Techs d300 |
|---|---|---|---|---|---|---|---|
| 1 | scout→freighter, flying | freighter landed, scout landed | 2 freighters (1 docked) | 3 ships (2 freighters, 1 scout), all docked | 7 | 105 / 10 | 3 |
| 2 | scout docked, hold 31 | freighter, scout, both landed | 2 freighters | 2 freighters + 1 warship, all docked | 10 | 3 / 16 | 2 |
| 3 | scout landed | freighter landed | 2 freighters docked | the same (nothing moved after d200) | 7 | 89 / 13 | 2 |
| 4 | scout docked | freighter landed | freighter + scout docked | the same | 4 | 58 / 6 | 2 |
| 5 | scout landed | freighter flying | freighter docked, scout landed | 2 freighters docked | 5 | 96 / 2 | 2 |
| 6 | freighter flying | freighter landed | 3 ships | 3 warships (1 landed) | 12 | 134 / 34 | 3 |

What it shows:

- **From about day 200 most ships wait docked.** No free site lies within 50
  of the mothership: the hive (8–10 sites by day 300 at Rival factions 3)
  and the others take them. Docked ships do not grow, dig or research. This
  is today's behaviour; this plan keeps it (open question 18 asks whether
  to change it, as its own decision with its own pace table).
- **Crystal holds the tiers.** The hoard runs 2–34 crystal against the
  warship's 40; two seeds in six bought a warship by day 300.
- **The walk's three entries take most of the campaign** (about 135 landed
  days for all three).

## Design

### The roster

Six types, as the hive has six and the autonz seven. Types are E2's
mechanism: `types` in the doctrine, `ApplyType` at spawn, `Kin.Type`,
unlocks by research, a `fallback` per type. There are no visual tiers (no
second palette on the sheet); upgrades are stat multipliers applied to
living bodies, as the hive's are.

| Type | Sprite (x, y=408) | Station | Cost | build_time | HP | Str | Dex | AC | Int | Natural attack | Init | Kit / traits | Role: allowed tasks | Unlocked by |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| worker | 0 | vat | 8 metal_ore, 3 crystal, 1 grey_alloy | 450 | 18 | 10 | 12 | 10 | 8 | 1d6 | 10 | — | `worker`: mine, dig, haul, build | start |
| technician | 96 | vat | 6 metal_ore, 5 crystal, 2 grey_alloy | 600 | 14 | 6 | 12 | 10 | **16** | 1d4 | 10 | — | `technician`: research, craft, build, haul | start |
| soldier | 192 | vat | 10 metal_ore, 4 crystal, 3 grey_alloy | 600 | 32 | 14 | 14 | 14 | 8 | 1d6 | 9 | armed at the armory (beam pistol, then rifle) | `soldier`: attack | start |
| heavy | 336 | vat | 14 metal_ore, 6 crystal, 5 grey_alloy | 900 | 50 | 18 | 10 | 16 | 6 | `tentacles` (1d8+2) | 12 | armed at the armory (disruptor once known) | `heavy`: attack (raid leader) | `grey_cloning` |
| skimmer | 480 | vat | 6 metal_ore, 4 crystal, 2 grey_alloy, 1 specimen | 600 | 16 | 8 | 18 | 12 | 8 | 1d6 | 6 | `flying` | `skimmer`: haul (patrols, contact) | `grey_skimmer_pattern` |
| lancer | 624 | vat | 12 metal_ore, 8 crystal, 6 grey_alloy, 2 specimen | 1,200 | 30 | 12 | 16 | 14 | 8 | `grey_lance` (2d6+2, range 8) | 8 | `flying`, natural lance | `lancer`: attack (posted at emitters, raids over walls) | `grey_lancer_pattern` |

- **The scout's crew of three stays a worker, a technician and a soldier**
  (see the stage mix below), so the smallest grey camp can mine, research
  and defend, which is what makes it elite rather than a mining party.
- **A soldier is a security robot that shoots like a rocket team.** 32
  health and AC 14 (the robot is 35 and 14), with a beam rifle (2d6+4,
  range 12) it out-ranges every colony gun but the rocket launcher and
  out-damages all of them per shot but the launcher and the laser sword.
- **A heavy is the toughest body in the game.** 50 health and AC 16 (the
  hive warrior is 32 and 12; a wild stage-3 xeno about 30). It is slow
  (Initiative 12), fights in the tentacle pose until a disruptor is made
  for it, and leads raids (raid `leader`).
- **A technician is the only researcher.** The research rung needs Int 12
  (`required_int`) and `research` in the worker's tasks; only technicians
  have both. Technicians also run the vat and the armory (`craft`), so a
  ship with one technician is choosing every hour between learning and
  making. That is the intended pressure.
- **A skimmer is the fast flier that finds the colony.** It hauls while
  idle (the greys' only flying hauler: it crosses water and pits) and is
  the patrol rung's body, so contact (and so raids) waits for it or for the
  player walking into a grey camp.
- **A lancer is a flying gunner**, the autonz sentinel's role at grey
  prices: posted at an emitter, it shoots over walls. Its lance is a
  natural weapon (an inventory weapon with no `GearDrop`, so it is lost
  with the body, as the spit gland is).
- **Specimens are the research items.** The two fly patterns, and every
  fly print, cost specimens: samples taken from the planet's wildlife by a
  grey kill (`kill_drops: { "specimen": 1 }` on the settlement, the hive's
  biomass mechanism) and hauled home. So the greys' new units come from
  what they collect on the worlds they visit (decided: wildlife kills only,
  no colonist abductions).
- **Crew blueprints** keep their ids so saves and the scenario greys stay
  put: `greyish_crew` is the worker, `whitish_crew` the technician,
  `pinkish_crew` the soldier, `greenish_crew` the heavy, plus new
  `skimmer_crew` and `lancer_crew`. Each gains `Kin`, `NeverSleep` (as the
  autonz crew; open question 7) and loses its starting `tentacles` except
  the heavy's. `ApplyType` sets role, tasks and stats, so the blueprints
  carry only the look.
- **The scenario greys** (`greyish.json`: `grey_contact`, `crashed_ufo`,
  the saucer stamp's `crew: alive`) are unchanged: wild, no type, today's
  stats. A wild grey is the same faction: never fought, never adopted, not
  counted (E2's rule for wild hive creatures).

### Types in `data/factions.json`

```json
"types": {
  "grey_worker":     { "blueprint": "greyish_crew", "name": "Worker", "print_cost": { "metal_ore": 8, "crystal": 3, "grey_alloy": 1 },
                       "health": 18, "str": 10, "dex": 12, "ac": 10, "int": 8, "attack": "1d6", "initiative": 10,
                       "role": "worker", "tasks": ["mine", "dig", "haul", "build"] },
  "grey_technician": { "blueprint": "whitish_crew", "name": "Technician", "print_cost": { "metal_ore": 6, "crystal": 5, "grey_alloy": 2 },
                       "health": 14, "str": 6, "dex": 12, "ac": 10, "int": 16, "attack": "1d4", "initiative": 10,
                       "role": "technician", "tasks": ["research", "craft", "build", "haul"] },
  "grey_soldier":    { "blueprint": "pinkish_crew", "name": "Soldier", "print_cost": { "metal_ore": 10, "crystal": 4, "grey_alloy": 3 },
                       "health": 32, "str": 14, "dex": 14, "ac": 14, "int": 8, "attack": "1d6", "initiative": 9,
                       "role": "soldier", "tasks": ["attack"] },
  "grey_heavy":      { "blueprint": "greenish_crew", "name": "Heavy", "unlock": "grey_cloning", "fallback": "grey_soldier",
                       "print_cost": { "metal_ore": 14, "crystal": 6, "grey_alloy": 5 },
                       "health": 50, "str": 18, "dex": 10, "ac": 16, "int": 6, "attack": "1d8", "initiative": 12,
                       "equip": ["tentacles"], "role": "heavy", "tasks": ["attack"] },
  "grey_skimmer":    { "blueprint": "skimmer_crew", "name": "Skimmer", "unlock": "grey_skimmer_pattern", "fallback": "grey_worker",
                       "print_cost": { "metal_ore": 6, "crystal": 4, "grey_alloy": 2, "specimen": 1 },
                       "health": 16, "str": 8, "dex": 18, "ac": 12, "int": 8, "attack": "1d6", "initiative": 6,
                       "flying": true, "role": "skimmer", "tasks": ["haul"] },
  "grey_lancer":     { "blueprint": "lancer_crew", "name": "Lancer", "unlock": "grey_lancer_pattern", "fallback": "grey_soldier",
                       "print_cost": { "metal_ore": 12, "crystal": 8, "grey_alloy": 6, "specimen": 2 },
                       "health": 30, "str": 12, "dex": 16, "ac": 14, "int": 8, "attack": "1d6", "initiative": 8,
                       "flying": true, "equip": ["grey_lance"], "role": "lancer", "tasks": ["attack"] }
},
"type_fallback": "grey_worker",
"stages": [
  { "pop": 0, "name": "landing", "w": 17, "h": 17,
    "mix": { "grey_worker": 0.35, "grey_technician": 0.25, "grey_soldier": 0.25,
             "grey_heavy": 0.1, "grey_skimmer": 0.05 } }
]
```

- `int` is new on a type (E2's types set Str, Dex, AC, health, attack and
  initiative). It is one more stat in `ApplyType`.
- **Largest-remainder check, nothing unlocked** (heavy to soldier, skimmer
  and lancer to their fallbacks): 3 crew → worker 1, technician 1, soldier
  1; 5 → 2, 1, 2 (two soldiers, so the freighter raids as today); 8 → 3, 2,
  3. With the heavy known, 8 → 3 workers, 2 technicians, 2 soldiers, 1
  heavy. The lancer has no share of its own (it comes from the `grow`
  rung's emitter posts, below); the stage mix leaves it out so the Tier 1
  composition never names it, which keeps materialise and the director in
  agreement (open question 5).
- `economy_types` (E3's: what starvation takes last) is not used: the
  greys never starve.

### Recipes

One recipe per type at the vat, `deploy_on_craft`, `kin_type`,
`requires_tech` where locked, the cost equal to the type's `print_cost` (a
content test asserts it, as E2 and E3 do). The existing four recipe ids are
kept (renamed only in their names), so saves holding an open craft order
load:

```json
"print_greyish":  { "name": "Print Worker",     "description": "_draft", "output": "greyish_crew",  "kin_type": "grey_worker",
                    "cost": { "metal_ore": 8, "crystal": 3, "grey_alloy": 1 }, "build_time": 450, "station": "grey_vat", "deploy_on_craft": true },
"print_whitish":  { "name": "Print Technician", "description": "_draft", "output": "whitish_crew",  "kin_type": "grey_technician",
                    "cost": { "metal_ore": 6, "crystal": 5, "grey_alloy": 2 }, "build_time": 600, "station": "grey_vat", "deploy_on_craft": true },
"print_pinkish":  { "name": "Print Soldier",    "description": "_draft", "output": "pinkish_crew",  "kin_type": "grey_soldier",
                    "cost": { "metal_ore": 10, "crystal": 4, "grey_alloy": 3 }, "build_time": 600, "station": "grey_vat", "deploy_on_craft": true },
"print_greenish": { "name": "Print Heavy",      "description": "_draft", "output": "greenish_crew", "kin_type": "grey_heavy",
                    "cost": { "metal_ore": 14, "crystal": 6, "grey_alloy": 5 }, "build_time": 900, "station": "grey_vat", "deploy_on_craft": true,
                    "requires_tech": "grey_cloning" },
"print_skimmer":  { "name": "Print Skimmer",    "description": "_draft", "output": "skimmer_crew",  "kin_type": "grey_skimmer",
                    "cost": { "metal_ore": 6, "crystal": 4, "grey_alloy": 2, "specimen": 1 }, "build_time": 600, "station": "grey_vat",
                    "deploy_on_craft": true, "requires_tech": "grey_skimmer_pattern" },
"print_lancer":   { "name": "Print Lancer",     "description": "_draft", "output": "lancer_crew",   "kin_type": "grey_lancer",
                    "cost": { "metal_ore": 12, "crystal": 8, "grey_alloy": 6, "specimen": 2 }, "build_time": 1200, "station": "grey_vat",
                    "deploy_on_craft": true, "requires_tech": "grey_lancer_pattern" },

"refine_alloy":   { "name": "Refine Alloy",     "description": "_draft", "output": "grey_alloy",
                    "cost": { "metal_ore": 3, "crystal": 2 }, "build_time": 180, "station": "grey_armory" },

"grey_beam_pistol":   { "name": "Beam Pistol",   "description": "_draft", "output": "grey_beam_pistol",
                        "cost": { "crystal": 2, "grey_alloy": 1 }, "build_time": 60,  "station": "grey_armory" },
"grey_beam_rifle":    { "name": "Beam Rifle",    "description": "_draft", "output": "grey_beam_rifle",
                        "cost": { "metal_ore": 2, "crystal": 3, "grey_alloy": 2 }, "build_time": 120, "station": "grey_armory",
                        "requires_tech": "grey_beam_weapons" },
"grey_disruptor":     { "name": "Disruptor",     "description": "_draft", "output": "grey_disruptor",
                        "cost": { "metal_ore": 4, "crystal": 8, "grey_alloy": 4 }, "build_time": 240, "station": "grey_armory",
                        "requires_tech": "grey_disruptor" },
"grey_field_harness": { "name": "Field Harness", "description": "_draft", "output": "grey_field_harness",
                        "cost": { "metal_ore": 4, "crystal": 2, "grey_alloy": 3 }, "build_time": 160, "station": "grey_armory",
                        "requires_tech": "grey_field_armour" },
"grey_field_cowl":    { "name": "Field Cowl",    "description": "_draft", "output": "grey_field_cowl",
                        "cost": { "metal_ore": 2, "crystal": 1, "grey_alloy": 2 }, "build_time": 100, "station": "grey_armory",
                        "requires_tech": "grey_field_armour" }
```

The station moves from the colony's `bio_printer` to a grey `grey_vat`, so
a grey camp's printer never offers `print_colonist` and the colony's never
offers greys (the owner check F added on its branch covers rival stations;
the separate station key covers the ruin, where nobody owns anything).

### Items

| Item | Kind | Numbers | Against the colony's best | Notes |
|---|---|---|---|---|
| `grey_alloy` (new) | refined input | stack 100; tags `raw_material`, `alloy`, `grey`; sprite: metal ore's (320,16) tinted teal | — | the scarce input: 5 hauled units and a technician's 180 at the armory each. The `alien_alloy` relic is a different item and is not touched |
| `specimen` (new) | research and print input | stack 50, tags `organic`, `specimen`; sprite (16,32) | — | dropped into storage by a grey kill of wildlife (`kill_drops`) |
| `grey_beam_pistol` | weapon, 1 hand | 1d8+3 energy, range 8 | laser pistol 1d6+2, range 8 | every soldier's first weapon; sprite: the pistol tinted green; `GearDrop` 0.15 |
| `grey_beam_rifle` | weapon, 2 hands | 2d6+4 energy, range 12 | rifle 1d8+3 / 10; autorifle 1d6+4 / 8 | the soldier's weapon once beam weapons are known; `GearDrop` 0.10 |
| `grey_disruptor` | weapon, 2 hands | 3d8+3 energy, range 10 | rocket launcher 3d6+3 / 12 | the heavy's; more damage, less range than the launcher; `GearDrop` 0.05 |
| `grey_field_harness` | torso armour | defence 6, stopping power 6, tag `grey_field` | siege vest 6 / 5 | invisible when worn (no armour row: art gap); `GearDrop` 0.10 |
| `grey_field_cowl` | head armour | defence 3, stopping power 3, tag `grey_field` | siege helmet 4 / 3 | `GearDrop` 0.15 |
| `grey_lance` | natural weapon (lancer) | 2d6+2 energy, range 8 | — | no `GearDrop`, so it never drops (as the spit gland) |
| `grey_emitter_beam` | natural weapon (emitter turret) | 1d10+2 energy, range 8 | the sentry turret's laser pistol 1d6+2 | no `GearDrop`: never drops |

**What the player gets (decided 2026-11-09: rare usable drops).** A dead
grey's beam weapon or field armour usually breaks with its owner; only now
and then does one drop intact, and then a colonist can use it (the colony
still cannot make one: no colony recipe).

How death drops work today (`internal/game/main_state.go`,
`CleanUpSystem.OnEntityDead`, checked for this plan):

- **A dead body's inventory is lost.** Nothing spills its hands, armour
  slots or bag; the entity is removed with them. No item carries a "never
  drop" flag, because none is needed: the spit gland and the sentinel
  emitter "never drop" only because nothing does. (The autonz plan's line
  that the enforcer's pistol drops "as it does today" does not match the
  code.)
- **`Drops`** (a blueprint list on the body) always spawn, all of them: a
  combat kill puts them in the bag of a colony worker standing next to the
  body (who then `EquipAllBest`), otherwise on the ground. The autonz'
  ferrite drops this way.
- **`kill_drops`** (on the killer's doctrine) go to the killer's
  settlement storage (`grantKillDrops`: the hive's biomass, the greys'
  specimens).
- Chance rolls exist only for container loot (`internal/game/loot.go`:
  `LootEntry` with `chance`), not for deaths.

The rare drop is **one small hook beside `Drops`, as data on the item**:

- A new item component **`GearDrop { "Chance": 0.1 }`**. In
  `OnEntityDead`, after `Drops`, each item in the dead body's hands and
  armour slots that has `GearDrop` rolls once against the simulation's
  seeded random stream (`seedSimRand`, so a replay drops the same). A pass
  drops that item through the `Drops` path (the adjacent colony killer's
  bag and `EquipAllBest`, else the ground). A fail drops nothing: it broke
  with its owner.
- An item without `GearDrop` behaves exactly as today (lost), so no other
  faction's deaths change. Natural weapons simply have none.
- **Chances** (data, on the item blueprints): beam pistol 0.15, field
  cowl 0.15, beam rifle 0.10, field harness 0.10, disruptor 0.05. Killing
  a two-soldier raid with rifles yields a rifle about one time in five; a
  heavy's disruptor one time in twenty.
- **Rival bookkeeping.** A grey's gear dies with it either way: reconcile
  counts only items held by living bodies (`gear`), so a dropped or broken
  item leaves the record and the ship must make another.

Wrecks still drop **`grey_alloy`** (see "Ruins"). The colony has no recipe
that takes it this phase (decided: "stores for now"): it stores (tag
`raw_material`), counts in the inventory, and beams up like any material,
but it builds nothing.

### Structures

Two kinds. **Modules** are the ship: carried, placed by materialise when
the ship lands, never built by the director (a `module_only` buildable:
the build rung refuses it, as E3's `deploy_only`), packed (removed from the
level) on lift-off and never ruined. **Ground structures** are built by the
director with the hold's materials, on the player's build-order path, and
become wrecks when the ship leaves.

Which modules a ship carries is per hull tier, filtered by known techs:

```json
"tiers": [
  { "name": "scout",     "crew": 3, "modules": ["grey_vat", "grey_console"] },
  { "name": "freighter", "crew": 5, "cost": { "metal_ore": 40, "crystal": 15 },
    "modules": ["grey_vat", "grey_console", "grey_armory"] },
  { "name": "warship",   "crew": 8, "cost": { "metal_ore": 90, "crystal": 40 },
    "modules": ["grey_vat", "grey_console", "grey_armory", "grey_emitter", "grey_emitter"] }
]
```

A module whose buildable has a `required_tech` is carried only once that
tech is known: `grey_armory` from `grey_armory_module`, the emitters from
`grey_field_emitter`. Until then a freighter builds its armory on the ground
like a scout.

| Key | Kind | What it does | Blueprint | Sprite | Cost (ground or F) | Unlocked by | Director rule |
|---|---|---|---|---|---|---|---|
| `grey_vat` | module (every tier) | crafting station `grey_vat`: every print | `workbenches.json` | pod base (744,48) | 12 metal, 6 crystal, 4 alloy (the player's, in F) | start | placed by materialise by the hull's centre; never built |
| `grey_console` | module (every tier) | `ResearchBuilding` `grey_console`: the core tree | `furniture.json` | computer (552,48), tinted green | 10 metal, 6 crystal, 2 alloy (F) | start | placed in the cockpit; never built |
| `grey_armory` | module from the freighter (with `grey_armory_module`), else ground | crafting station `grey_armory`: alloy, weapons, armour | `workbenches.json` | bench (168,120), tinted green | 10 metal, 4 crystal (ground) | start | ground: one, once the stay has 8 or more days left and no armory is carried |
| `grey_array` | ground only | `ResearchBuilding` `grey_array`: the advanced entries' `required_building` | `greyish_structures.json` (Kin structure, faction greyish) | wall computer (624,48), tinted teal | 16 metal, 8 crystal, 4 alloy | `grey_research_array` | one, freighter tier or above, once the stay has 12 or more days left |
| `grey_emitter` | module on the warship (with `grey_field_emitter`), else ground | a turret: `ScriptedAI` `sentry_turret.basic`, natural `grey_emitter_beam`, HP 40, AC 14; a lancer's post | `greyish_structures.json` | **none** (placeholder: the sentry turret's blueprint look tinted green; art wanted) | 8 metal, 6 crystal, 3 alloy | `grey_field_emitter` | ground: up to 2, at the hull's breaches facing the colony when it is on the level, else evenly spaced, once the stay has 6 or more days left |
| cockpit locker | stamp | the hold (exists) | — | — | — | — | the stamp's; claimed by materialise (as now) |
| `storage_locker` | ground | another door onto the pocket: 20 more slots and a shorter walk from the mine or the bench (exists) | — | — | (as now) | start | one per 4 crew past the cockpit locker, placed nearest the open mine orders (more room and fewer steps) |

- **Effects that need code** are small and named once: `module_only`
  buildables; `ships.tiers[].modules` placed by materialise and removed by
  lift-off; the rung condition `stay_left` (below); the turret as a faction
  structure (the sentry script under a faction other than `sentry`;
  step 2 checks `find_nearest_worker` and the turret's hostility seeding).
- **Power**: none. The game's power is a station level flag
  (`PoweredComponent`, "power:main") for lights and doors, not a grid
  buildings draw on. A grey power system would be new code for flavour;
  open question 10 (default no).
- **Ground structures are a stay's luxury.** A scout landing for a short
  stay builds nothing but the armory it needs to refine alloy and arm its
  soldier; a freighter or warship staying the full thirty days raises the
  array and emitters. The `stay_left` rule makes that visible and
  deterministic.

### Ruins

What a departure leaves, after this phase:

- **The hull goes with the ship.** Today's ruin is the saucer's hull broken
  by `ruin_pass`, which means the ship that flew away left its hull
  behind. The ruin becomes the **landing scar**: the hull's footprint as
  scorched floor (`rubble.json`'s floor at the hull's circle, the walls
  gone), the ground structures as wrecks, the datapad as now, the footprint
  kept for the lore budget as now (open question 8). Ruins already stamped
  in saves stay as they are.
- **Modules go aboard.** The vat, console, carried armory and carried
  emitters are removed, so the player no longer inherits a free
  bio-printer from every grey landing.
- **Ground structures become wrecks** in place (the replace-in-place E3's
  deploy uses): `grey_armory_wreck`, `grey_array_wreck` (the cracked pod
  pair, 768), `grey_emitter_wreck`. A wreck is solid, unowned, HP 20, and
  `Drops` on destruction: alloy 1 (armory), 2 (array), 1 (emitter), and
  crystal 1–2 (`Drops`, so always). The player breaks them for grey alloy,
  which no colony recipe takes this phase (decided: "stores for now").
- Lockers built on the ground are emptied (as now: the pocket went aboard
  with the hold), lose their pocket flag and owner, and stay standing as
  plain empty lockers.
- **Unseen departures** (the ship came and went while the level was frozen)
  build the scar the same way: the record's `Built` (below) says which
  ground structures stood.

### The tree

`research.json` stays a flat map. The grey entries carry `"tree":
"greyish"` and an effect block **`develop`**, beside the hive's `evolve` and
the autonz' `assimilate`. The three existing entries become real entries
under the same ids, so saves and the Tier 1 walk need no alias.

`develop` effects:

| Effect | Shape | Where it acts | Tier 1 |
|---|---|---|---|
| `unlock` | a type | `TypesUnlocked` | composition only |
| `upgrade` | `{ "type": ..., "health": 1.25, "ac": 2 }` | E2's type upgrades, living bodies included (`Retype`) | none |
| `craft` | a recipe id | the UI's effect line (the recipe's `requires_tech` is the gate) | none |
| `build` | a buildable | the UI line (the buildable's `required_tech` is the gate) | `Built` only |
| `module` | a buildable | carried from the tiers that list it | `Built` only |
| `print_time` | a factor | the vat's build times | none |
| `salvage` | a factor | live mine work (`MineAction` time) for grey workers | **none** (the abstract yield is not touched) |
| `intercept` | `{ "catch_chance": 0.1 }` or `{ "range": -2 }` | Tier 3: the player's evade roll against a grey ship; the distance a jump meets one | none |
| `boarding` | `{ "turrets": 2 }` | a boarded mothership's deck | none |

A content test asserts that no grey entry has an effect Tier 1 reads (no
`range`, `conversion_rate`, `yield`, crew or tier key), so the pace cannot
move through the tree.

Durations are work steps. Costs are what the researching technician
carries to the console or array, as for every research entry.

```json
"grey_hull_plating":       { "name": "Hull Plating", "description": "_draft", "tree": "greyish",
                             "duration": 600,  "cost": { "metal_ore": 6, "crystal": 4, "grey_alloy": 1 },
                             "required_building": "grey_console", "required_int": 12,
                             "develop": { "upgrade": [ { "type": "grey_soldier", "ac": 2 }, { "type": "grey_heavy", "ac": 2 },
                                                       { "type": "grey_lancer", "ac": 2 } ] } },
"grey_beam_weapons":       { "name": "Beam Weapons", "description": "_draft", "tree": "greyish",
                             "duration": 900,  "cost": { "metal_ore": 8, "crystal": 6, "grey_alloy": 2 },
                             "required_building": "grey_console", "required_int": 12,
                             "develop": { "craft": "grey_beam_rifle" } },
"grey_cloning":            { "name": "Cloning", "description": "_draft", "tree": "greyish",
                             "duration": 1200, "cost": { "metal_ore": 10, "crystal": 8, "grey_alloy": 3 },
                             "required_building": "grey_console", "required_int": 12,
                             "develop": { "unlock": "grey_heavy" } },

"grey_salvage_rigs":       { "name": "Salvage Rigs", "description": "_draft", "tree": "greyish",
                             "duration": 500,  "cost": { "metal_ore": 6, "crystal": 2 },
                             "required_building": "grey_console", "required_int": 12,
                             "develop": { "salvage": 0.75 } },
"grey_armory_module":      { "name": "Armory Module", "description": "_draft", "tree": "greyish",
                             "duration": 700,  "cost": { "metal_ore": 10, "crystal": 4, "grey_alloy": 2 },
                             "required_building": "grey_console", "required_int": 12,
                             "develop": { "module": "grey_armory" } },
"grey_field_armour":       { "name": "Field Armour", "description": "_draft", "tree": "greyish",
                             "duration": 700,  "cost": { "metal_ore": 6, "crystal": 4, "grey_alloy": 2 },
                             "required_building": "grey_console", "required_int": 12, "requires_tech": "grey_hull_plating",
                             "develop": { "craft": "grey_field_harness" } },
"grey_skimmer_pattern":    { "name": "Skimmer Pattern", "description": "_draft", "tree": "greyish",
                             "duration": 600,  "cost": { "crystal": 4, "grey_alloy": 2, "specimen": 2 },
                             "required_building": "grey_console", "required_int": 12,
                             "develop": { "unlock": "grey_skimmer" } },
"grey_field_emitter":      { "name": "Field Emitter", "description": "_draft", "tree": "greyish",
                             "duration": 800,  "cost": { "metal_ore": 8, "crystal": 6, "grey_alloy": 3 },
                             "required_building": "grey_console", "required_int": 12, "requires_tech": "grey_beam_weapons",
                             "develop": { "build": "grey_emitter", "module": "grey_emitter" } },
"grey_research_array":     { "name": "Research Array", "description": "_draft", "tree": "greyish",
                             "duration": 900,  "cost": { "metal_ore": 10, "crystal": 8, "grey_alloy": 3 },
                             "required_building": "grey_console", "required_int": 12,
                             "develop": { "build": "grey_array" } },

"grey_disruptor":          { "name": "Disruptor", "description": "_draft", "tree": "greyish",
                             "duration": 1000, "cost": { "metal_ore": 8, "crystal": 10, "grey_alloy": 4 },
                             "required_building": "grey_array", "required_int": 12, "requires_tech": "grey_beam_weapons",
                             "develop": { "craft": "grey_disruptor" } },
"grey_print_refinement":   { "name": "Print Refinement", "description": "_draft", "tree": "greyish",
                             "duration": 900,  "cost": { "metal_ore": 8, "crystal": 8, "grey_alloy": 4 },
                             "required_building": "grey_array", "required_int": 12, "requires_tech": "grey_cloning",
                             "develop": { "print_time": 0.75 } },
"grey_tractor_lock":       { "name": "Tractor Lock", "description": "_draft", "tree": "greyish",
                             "duration": 900,  "cost": { "metal_ore": 10, "crystal": 8, "grey_alloy": 3 },
                             "required_building": "grey_array", "required_int": 12,
                             "develop": { "intercept": { "catch_chance": 0.1 } } },
"grey_lancer_pattern":     { "name": "Lancer Pattern", "description": "_draft", "tree": "greyish",
                             "duration": 1200, "cost": { "crystal": 8, "grey_alloy": 4, "specimen": 4 },
                             "required_building": "grey_array", "required_int": 12, "requires_tech": "grey_skimmer_pattern",
                             "develop": { "unlock": "grey_lancer" } },
"grey_heavy_frame":        { "name": "Heavy Frame", "description": "_draft", "tree": "greyish",
                             "duration": 1000, "cost": { "metal_ore": 12, "crystal": 8, "grey_alloy": 4 },
                             "required_building": "grey_array", "required_int": 12, "requires_tech": "grey_cloning",
                             "develop": { "upgrade": [ { "type": "grey_heavy", "health": 1.25 } ] } },
"grey_cloak":              { "name": "Cloak", "description": "_draft", "tree": "greyish",
                             "duration": 1200, "cost": { "metal_ore": 10, "crystal": 12, "grey_alloy": 5 },
                             "required_building": "grey_array", "required_int": 12, "requires_tech": "grey_tractor_lock",
                             "develop": { "intercept": { "range": -2 } } },
"grey_mothership_bulwark": { "name": "Mothership Bulwark", "description": "_draft", "tree": "greyish",
                             "duration": 1200, "cost": { "metal_ore": 14, "crystal": 10, "grey_alloy": 6 },
                             "required_building": "grey_array", "required_int": 12, "requires_tech": "grey_field_emitter",
                             "develop": { "boarding": { "turrets": 2 } } }
```

Sixteen entries: nine at the console, seven at the array. The array is a
ground structure, so **the advanced half of the tree is researched only on
long stays**, which is the shape of the faction: a ship that keeps getting
chased off its sites never learns the disruptor.

**Ship and mothership upgrades, folded in.** Hull tiers stay what they are:
bought at the mothership from the hoard (`dock`), per ship, and never a
tech (a tech is shared by every ship through the mothership, a tier is
not). What the tree adds to them:

| Upgrade | How it is bought | What it does |
|---|---|---|
| freighter hull | hoard, 40 metal, 15 crystal (as now) | crew 5; carries the armory once `grey_armory_module` is known |
| warship hull | hoard, 90 metal, 40 crystal (as now) | crew 8; carries two emitters once `grey_field_emitter` is known |
| a new ship | hoard, 60 metal, 20 crystal (as now) | a scout with the mothership's techs |
| `grey_tractor_lock`, `grey_cloak` | research | Tier 3: harder to evade, harder to meet |
| `grey_mothership_bulwark` | research | the mothership's deck, if boarded, holds two emitters |

The research modal shows a **Hull** section (read-only: the ship's tier,
its crew cap, the modules it carries and the next tier's hoard cost) above
the tree, so the whole progression reads in one place (see "Phase F").

### Tier 1: two walks, both landed-only, neither moving the pace

- **The spine** is the doctrine's `research` list, unchanged: the same
  three ids, the same costs, the same position, `research_per_day` 1, while
  landed. The live entries now have effects, but none Tier 1 reads.
- **A second walk** for ships: the autonz' `assimilation` mechanism, given
  neutral JSON keys (`second_walk`, `second_walk_per_day`; the autonz'
  `assimilation` keys stay readable as aliases, so their data and saves do
  not change). For a planet-carried presence the per-day list is by stage
  (as now); **for a ship it is by hull tier**, and it advances only on the
  days the spine does (landed, off a loaded site). Docking merges a ship's
  techs into the mothership's and copies them back, as for the spine
  (`dock` already does this for all of `KnownTechs`).

```json
"research_per_day": 1,
"research": [ { "id": "grey_hull_plating", "cost": 30 }, { "id": "grey_beam_weapons", "cost": 45 }, { "id": "grey_cloning", "cost": 60 } ],
"second_walk_per_day": [0.5, 1, 1.5],
"second_walk": [
  { "id": "grey_salvage_rigs",       "cost": 20 },
  { "id": "grey_armory_module",      "cost": 30 },
  { "id": "grey_field_armour",       "cost": 30 },
  { "id": "grey_skimmer_pattern",    "cost": 30 },
  { "id": "grey_research_array",     "cost": 40 },
  { "id": "grey_field_emitter",      "cost": 40 },
  { "id": "grey_disruptor",          "cost": 50 },
  { "id": "grey_tractor_lock",       "cost": 40 },
  { "id": "grey_print_refinement",   "cost": 50 },
  { "id": "grey_lancer_pattern",     "cost": 60 },
  { "id": "grey_heavy_frame",        "cost": 50 },
  { "id": "grey_cloak",              "cost": 60 },
  { "id": "grey_mothership_bulwark", "cost": 60 }
]
```

- Neither walk feeds back: no entry changes growth, yield, the hold, the
  stay, the range, the tier costs or ship building. `KnownTechs` grows; the
  composition (`units`) and `built` (below) read it; nothing else does.
  **So the step 0 table does not move.** Any difference is a bug.
- Rough abstract arc (from the baseline's landed days: a faction lands one
  or two ships at a time, each researching on its own and merging at
  dock): salvage rigs around day 40, the armory module around day 80, the
  array around day 160, the disruptor around day 220. Many seeds will not
  finish the second walk inside 300 days, because their ships sit docked
  from about day 200. `HFDETAIL=1` prints it (step 0 and step 6).
- **New presence fields** (bookkeeping only, never blocking growth or a
  departure):

| Field | Abstract (Tier 1) | Live (reconcile) |
|---|---|---|
| `units` (E2) | the stage mix over unlocked types, by largest remainder of the population | living bodies by `Kin.Type` |
| `built` (E3) | the ground structures a ship of this tier with these techs **would have built by its days landed**, by the director's own order and day thresholds (`Doctrine.ShipBuilt(tier, techs, daysLanded)`) | standing ground structures of the settlement by key |
| `gear` (new, `map[item]int`) | the **gear floor**: one beam pistol per soldier, a rifle instead once beam weapons are known, a disruptor per heavy once known, a harness per fighter once field armour is known | items held or worn by bodies, counted when the level is written and when the ship lifts off |
| `assim_points` (E3, reused) | the second walk | — |
| `stockpile` | the hold, unchanged | `storage.Summarize` (as now) |

The gear floor is the materialise precedent E3 set with its stock floor
(an abstract presence is assumed to hold what it would have made). It is
the one place "nothing from nothing" bends, and it bends only for ships
never seen by the player (open question 12).

### Economy: what "expensive" means physically

- **Inputs.** Metal and crystal, mined and hauled to the nearest grey
  locker, which is the hold: every locker the ship's settlement owns on
  the level is a door onto the same storage pocket, the cockpit locker
  included. **Grey alloy** refined from them
  at the armory by a technician (3 metal + 2 crystal → 1, 180), the input
  every body, weapon and late entry needs. **Specimens** from kills, for
  the fly patterns and prints.
- **Why it is scarce.** Crystal comes only from ore and radioactive sites'
  veins (the yield table gives salvage sites 0.05 a crew member a day, base
  sites none). Alloy costs 2 crystal and a technician's time. A soldier is
  3 alloy (6 crystal) + 4 crystal + a rifle's 3 crystal + 2 alloy (4
  crystal): **17 crystal for one armed soldier**, where a colonist's laser
  pistol is 2. A ship of five that loses its two soldiers needs about 35
  crystal and three days of a technician to replace them.
- **The bank.** The mothership's hoard takes everything a docking ship
  brings, alloy and specimens included (`dock` copies the whole hold). Tier
  purchases and new ships are paid in metal and crystal, as now; alloy and
  specimens in the hoard are wealth that the boarding party loots
  (`DestroyMothership` returns the whole hoard).
- **Ships start empty**, as now: a ship built or re-launched has no hold.
  A landed ship's first days are mining; prints and weapons come after.
  The gear floor arms bodies printed abstractly; a body printed live waits
  for the armory. (Provisioning ships from the hoard would make the bank
  feel like one, but every unit drawn from the hoard moves the day a tier
  or ship is bought; open question 13 keeps it out.)
- **Spending versus going home.** Live, the director spends the hold on
  alloy, prints, weapons and ground structures, which slows the hold toward
  `hold_target` and so tends to keep a live ship the full thirty days.
  Every spending rung keeps a reserve: it orders only from what is above
  `reserve` (E2's grow `reserve`, given to the craft and build rungs) of the
  hold target (half by default), so a live ship still goes home with
  something. Tier 1 spends nothing (as now). The soak records the hold at
  lift-off.

### The storage pocket (decided 2026-11-09)

The owner's words (open questions 24 and 25): greys haul from storage to a
build site and from a mine to storage, but never from one locker to
another; "all lockers have access to the same inventory on the planet.
It's 'inter-dimensional'"; and "rivals get their own inter-dimensional
storage pocket". Draft name: **the pocket** (`storage_pocket` in data;
the player-facing name is the owner's).

**What it means.** Every grey locker one owner has on one level is a door
onto one shared inventory. A worker deposits at whichever door is nearest
and withdraws at whichever door is nearest; nothing is ever carried from
locker to locker. Both ends of every haul stay physical.

**The owner is the settlement, which the code already keys.** A rival grey
presence runs a settlement named `"<faction>:<locID>"`
(`factionSettlementName`, `faction_settlement.go:33`), and every container
it owns carries that name in `StorageComponent.OwnedBy`. Two grey ships
never share a site (`shipCandidates` refuses a site another ship holds or
is bound for), so on a level a rival grey settlement is exactly one ship,
and its pocket is that ship's hold on that site. A different ship on
another site is another settlement and another pocket; the mothership's
hoard is abstract (Tier 1) and is no pocket. The player's greys have their
site settlement (the colony name on the site) and the ship's own
settlement (`"<colony> Ship"`, `the_ship.md`): one pocket on the loaded
site and one aboard, joined by the beam (see "Linked lockers"). **The
pocket key is `OwnedBy`; no new key exists.**

**How it is stored: the union of the doors, no new store.** Today each
container holds its own `StorageComponent.Items` slice; saves write each
container's slice (`saveStorageData`, `internal/world/save.go`); totals
(`storage.Summarize`, `CountResource`, `Check`) add every container of an
owner; workers find containers through `internal/workerai/storage.go`
(`FindClosestStorageWith`, `findClosestStorageWithRoomFor`,
`FindAvailableStorageFor`, `FindClosestStorageWithAny`,
`FindClosestStorageWithComponent`) and take or put with the component's
own methods (`TakeOne`, `TakeResourceStack`, `AddItem`, `DeductResource`).
The pocket keeps all of that and widens only the door's view:

- `StorageComponent` gains **`Pocket bool`** (saved: one field in
  `saveStorageData`). A container is a door when `Pocket` is set.
- **The pocket's contents are the union of its doors' `Items`.** Items stay
  in whichever door's slice they were put in. Totals do not change at all
  (they already add every container of the owner, and each item is in
  exactly one slice), so reconcile's `storage.Summarize` (the hold) is the
  same number with or without the pocket.
- **Finding.** The finders treat a door as holding what its pocket holds
  and as having the room its pocket has (the sum of its doors' free
  slots). The worker walks to the **nearest door**, not to the locker that
  happens to hold the item. One helper does it,
  `storage.PocketView(level, door)` (the doors of the same owner on the
  same level), and the finders call it when `Pocket` is set.
- **Taking and putting.** At a door, a take looks in the door's own slice
  first and then in the other doors' slices; a put goes into the door's own
  slice, then any door's with room. The move between slices is the pocket
  itself (no worker carries it). This is one wrapper around `TakeOne`,
  `TakeResourceStack`, `AddItem` and `DeductResource`, used by the deposit
  and fetch handlers (`internal/workerai/states.go` `loadFromStorage` and
  the deposit states, `task_handlers.go` equip and retrieve, `diet.go`'s
  edible take).
- **No reservations, as today.** Nothing reserves a stored item now: a
  worker re-checks the container on arrival (`storageStillUsable`) and
  re-plans if it is empty. Under the pocket the re-check asks the pocket,
  so two workers racing for the last unit behave as they do now: one
  arrives, finds none, and re-plans.
- **Capacity** is the sum of the doors' (`storage_locker` holds 20 slots),
  so a new locker is both more room and a shorter walk.

**A door destroyed.** Its slice moves into the surviving doors (the most
free room first). The hook is `OnEntityDead` in
`internal/game/main_state.go`, beside `Drops`.

**The last door destroyed spills the pocket** (decided 2026-11-09, open
question 27). Everything the pocket holds lands on the ground around that
door's tile, and smashing every grey locker of a camp is a way to loot it.
There is no hidden store: the earlier "pocket anchor" is gone, and nothing
else needed it (materialise's overflow now uses the ground branch, below).

- **How a large spill lands.** By the rule the faction stock already uses
  when a camp has no container (`fillFactionStock`'s ground branch,
  `faction_settlement.go:271`, with `groundNear`): **one stack a tile,
  nearest first**, on standable tiles within 6 of the door, in blueprint
  order. A stackable stays one entity with its whole quantity (40 ore is
  one pile); each non-stackable (a weapon, a harness) takes a tile of its
  own. If the tiles run out, the rest piles onto the nearest tile, several
  entities on one tile as the `Drops` path already allows. No Retrieve
  task is made for anyone: the items are loose, and whoever orders a haul
  first gets them.
- **Stored grey gear in a spill** rolls its `GearDrop` chance, as gear on a
  dead grey does: an unequipped beam rifle in the pocket usually breaks in
  the spill and only now and then lands intact. Without this, a camp's
  spare weapons (the craft rungs keep one per soldier in stock) would spill
  whole and bypass the rare-gear decision (decided, open question 28).
- **Resources spill whole**: metal, crystal, grey alloy, specimens.

**After a spill, for a rival:**

- **Its hold total** is what reconcile writes from its containers
  (`reconcileSettlement` → `storage.Summarize` over the settlement's
  containers). Loose items belong to no container, so the record's
  stockpile drops by the spill at the next write of the level. That is the
  player's doing, as a kill lowers the record's population: Tier 1's rules
  do not change, and the step 0 table, which loads no level, cannot see it.
- **A new door.** The cockpit locker is part of the hull, so for a ship it
  is a **module**: one day (750 ticks) after its last door falls, the ship
  re-places an empty cockpit door at the hull's centre, unbuilt, as
  materialise places modules. That gives the player a day to carry the
  spill off, and it avoids a deadlock (a locker costs 2 metal, and with no
  door there is nowhere to fetch it from).
- **Picking the spill back up.** A new haul value, **`"haul": "any"`**
  (beside the existing `"food"`), orders loose items of any kind inside the
  director's radius back to the nearest door. It runs once a door stands.
  What the player's colonists already carried away is gone; what is left
  comes back into the pocket and into the next reconcile's total.
- **Lift-off with no door standing** packs nothing from the ground:
  `emptySettlementStorage` empties containers, and there are none. The
  record's hold is the last reconcile's total; the loose items stay on the
  level as the player's loot and, after the ruin is stamped, as part of
  the ruin site.
- **Mining meanwhile.** The mine rung counts stored and carried ore, not
  loose ore, so a rival without a door keeps mining; its miners hold what
  they dig until a door returns (they cannot deposit), as any worker with
  no container does today.

**After a spill, for a grey player:** the same spill on the site. The
player's own workers pick it back up with ordinary haul orders. Their next
locker costs 2 metal, which they cannot fetch with no door standing, so
**the first beam-down to a site with no grey locker places one** at the
landing zone, charged 2 metal from the ship pocket (the beam exception
covers it), and the beamed goods land in it. The ship pocket aboard is
never spilled by anything on the site.

**Freeze, thaw and saves.** Loose items are ordinary item entities, saved
with the level as any dropped item is. The record was reconciled without
them when the level froze, so Tier 1 steps the ship from the smaller hold
and nothing is counted twice. On thaw the presence is already
materialised, so its stock is not refilled; the loose items are still
there, and the haul rung takes them back once a door stands. If the ship
lifted off while frozen, the ruin is stamped at the next build and the
loose items stay on its site. The one-day door timer is saved on the
presence (`DoorLostTick`, live only, cleared when a door stands).

**What changes for rival greys** (this is part of their design, not a
player-only feature):

- **Hauling rungs.** Unchanged in code: the haul and mine rungs already
  send items to the nearest container with room. They now always find the
  nearest door, and fetches for builds, crafts, research costs and prints
  start from the nearest door. Rival greys walk less per item.
- **The cockpit hold.** It is one door among the settlement's lockers,
  claimed at materialise as now (`buildStampedCamp`), with `Pocket` set. A
  doctrine flag, **`settlement.storage_pocket: true`**, sets `Pocket` on
  every container the settlement claims or builds.
- **"A full hold"** is unchanged: `hold_target` is read from the record's
  stockpile, which reconcile fills from `storage.Summarize` over every
  container of the settlement. The pocket does not change the number (a
  spill does: loose items are in no container; see above).
- **Lift-off** packs what it packs today: `emptySettlementStorage` empties
  every container the settlement owns, after reconcile wrote the
  stockpile; with no door standing it packs nothing, and the spill stays.
- **A destroyed locker** drops nothing while another door stands (its
  slice moves to the survivors); **the last one spills the pocket**.
- **A colonist raiding a grey camp** cannot take from a standing grey
  locker: colony workers only fetch from containers their own settlement
  owns, and the inspector's beam-up gains an owner check for pocket doors
  (see "Linked lockers"). To loot the camp they **smash every grey
  locker**, then haul the spill away before the ship re-places its cockpit
  door a day later. That is the main reward for beating a grey camp; the
  rest is gear that drops intact (`GearDrop`) and what wrecks drop.

**Tier 1 does not move.** The pocket is live bookkeeping inside the level:
the record's stockpile is the same `Summarize` total, the abstract
`shipYield`, `holdUnits`, `ShipLeaving`, `dock`, tier purchases and ship
building never read a container, and `history-dump` runs with no level
loaded. The step 0 table stays byte-identical. What it does move is
**live hauling**: fewer steps per item, so a live grey camp gets slightly
more done per day. The greyish soak records it (the "Hold at lift-off"
and armory columns); nothing in "Identity in numbers" is tuned on walking
distance.

### The short stay and an outpost

- **What stays on the ship**: modules (vat, console; armory from the
  freighter; emitters on the warship), the crew and their gear, the hold,
  the techs.
- **What is rebuilt each landing**: the ground armory (until the module is
  known), the array, ground emitters, extra lockers, each only if the stay
  has the days left for it (`stay_left`).
- **The research in progress** goes with the ship: a half-done entry's
  progress is kept on the presence (the settlement's research progress is
  reconciled like its techs), so a ship that lifts off mid-entry resumes it
  at the next landing's console. Entries at the array resume only where an
  array stands again.
- **Long-stay outposts: not in this phase** (open question 14). An outpost
  would be a grey presence carried by a planet, not a ship: a new
  `landFleet`-style handover, a stage, a pace of its own and a Tier 1
  change. It is the natural next phase if the owner wants grey footholds;
  this phase stays inside the ship.

### Needs

The greys have none today and **should keep none** (open question 7):

- An upkeep would be the hive's lesson again: E2's live hive starved on two
  seeds in ten and its research stalled at night. A grey ship of three that
  starves loses a third of its faction's strength.
- Tier 1 has no upkeep for ships (no starvation: `starve_per_day` 0), so an
  upkeep live would be a live-only drain with no abstract twin, and the two
  would stop agreeing.
- "Expensive" is already carried by capital (prints, weapons, alloy,
  structures) and by the crew cap. An elite faction that never eats is
  consistent with a printed crew.
- **Default**: `needs: false`, and `NeverSleep` on every grey crew body (the
  autonz' fix), so technicians research through the night. **The
  alternative** is a power-cell upkeep: `needs: true` with the hive's
  `diet` set to a new `power_cell` (the unused cells at (16,48)), crafted
  at the armory (1 crystal → 2 cells), one a body every five days. It is
  data (diet and needs exist), but it brings beds, sleep and hunger
  incidents with it, and it needs its own soak.

### The settlement block

```json
"settlement": {
  "crew": [ { "blueprint": "greyish_crew", "type": "grey_worker",     "share": 0.4,  "role": "worker" },
            { "blueprint": "whitish_crew", "type": "grey_technician", "share": 0.25, "role": "technician" },
            { "blueprint": "pinkish_crew", "type": "grey_soldier",    "share": 0.35, "role": "soldier" } ],
  "needs": false,
  "kill_drops": { "specimen": 1 },
  "build_menu": [ { "id": "orders", "label": "Orders" }, { "id": "structures", "label": "Structures" } ],
  "buildables": [ "storage_locker", "grey_armory", "grey_array", "grey_emitter", "grey_vat", "grey_console" ],
  "research": [ "grey_salvage_rigs", "grey_hull_plating", "grey_armory_module", "grey_beam_weapons", "grey_field_armour",
                "grey_skimmer_pattern", "grey_research_array", "grey_cloning", "grey_field_emitter", "grey_disruptor",
                "grey_tractor_lock", "grey_print_refinement", "grey_lancer_pattern", "grey_heavy_frame", "grey_cloak",
                "grey_mothership_bulwark" ],
  "units": [ "print_greyish", "print_whitish", "print_pinkish", "print_greenish", "print_skimmer", "print_lancer" ],
  "recipes": [ "refine_alloy", "grey_beam_pistol", "grey_beam_rifle", "grey_disruptor", "grey_field_harness", "grey_field_cowl" ],
  "camp": { "stamp": "crashed_ship", "params": { "class": "saucer", "crew": "dead" }, "w": 17, "h": 17, "modules": true },
  "director": { "...": "see The director table" }
}
```

- `crew` here is the materialise split for a body with no record; the
  stage `mix` above is what Tier 1 and the grow rung use. Both give 1/1/1
  at three.
- `camp.station` is replaced by `camp.modules: true` (read the hull tier's
  `modules`). The colony's `bio_printer` station leaves the greys entirely.
- `grey_vat` and `grey_console` are `module_only` for a rival (the build
  rung refuses them) and ordinary buildables for the player in F.
- The live research list interleaves the two walks so the cheap,
  useful entries come first: salvage rigs (the ship mines faster from day
  one), hull plating, the armory module, beam weapons.

### The director table

Walked every 30 ticks with the usual rules. **New** marks a feature added
in this phase.

```json
"director": {
  "every_ticks": 30, "max_open": 2, "radius": 12, "reach": 50,
  "max_mines": 3, "max_hauls": 3, "stuck_ticks": 3000, "dig_depth": 4, "dig_reach": 60,
  "priorities": [
    { "id": "grow",      "grow": { "share": 1.0, "reserve": 0.5 } },
    { "id": "materials", "mine": { "metal_ore": 200, "crystal": 80 } },
    { "id": "haul",      "haul": "grey_alloy" },
    { "id": "storage",   "build": "storage_locker", "min": 1, "per_colonist": 0.25 },
    { "id": "armory",    "build": "grey_armory", "min": 1, "stay_left": 8 },
    { "id": "alloy",     "craft": "refine_alloy", "min": 6, "reserve": 0.5 },
    { "id": "research",  "research": true },
    { "id": "pistols",   "craft": "grey_beam_pistol", "per_role": { "soldier": 1 }, "reserve": 0.5 },
    { "id": "rifles",    "craft": "grey_beam_rifle", "per_role": { "soldier": 1 }, "reserve": 0.5 },
    { "id": "disruptors","craft": "grey_disruptor", "per_role": { "heavy": 1 }, "reserve": 0.5 },
    { "id": "harness",   "craft": "grey_field_harness", "per_role": { "soldier": 1, "heavy": 1 }, "reserve": 0.5 },
    { "id": "arm",       "arm": { "roles": ["soldier", "heavy"], "armour": true, "best": true } },
    { "id": "guard",     "guard": { "roles": ["soldier", "heavy"], "radius": 10, "post": 5 } },
    { "id": "emitters",  "build": "grey_emitter", "min": 2, "stay_left": 6,
                         "guard": { "roles": ["lancer"], "posts": "grey_emitter" } },
    { "id": "array",     "build": "grey_array", "min": 1, "stay_left": 12, "tier_min": 1 },
    { "id": "specimens", "hunt": "specimen", "min": 4, "per_colonist": 0, "hunger": 0 },
    { "id": "dig",       "dig": true },
    { "id": "skimmers",  "patrol": { "roles": ["skimmer"], "radius": 30, "every_ticks": 600 } },
    { "id": "raid",      "raid": { "fighters": 2, "party": 3, "home": 1, "cooldown_ticks": 4500, "max_ticks": 6000,
                                   "reach": 400, "roles": ["soldier", "heavy", "lancer"],
                                   "scouted": true, "leader": "heavy" } }
  ]
}
```

| # | Rung | Runs when | What it orders |
|---|---|---|---|
| 1 | `grow` | crew under the ship's crew cap (`Site.CrewCap`, as now) and the hold above the reserve | the type most under the stage mix among unlocked types, at the vat (E3's typed grow) |
| 2 | `materials` | stored + carried metal under 200, crystal under 80 (as now: always, in practice) | mine |
| 3 | `haul` | loose alloy or specimens in reach (a kill's drop, a craft left at the station) | haul to the hold |
| 4 | `storage` | lockers under 1 + 0.25 × crew | build a locker |
| 5 | `armory` | no armory standing or carried, and **new** `stay_left` ≥ 8 days | build one inside the hull or beside it |
| 6 | `alloy` | an armory stands, stored alloy under 6, the hold above the reserve | `refine_alloy` |
| 7 | `research` | a console (or the array, for its entries) stands | the next entry of `research` that is unlocked, unknown and affordable (the research rung; Int 12 and `research` in the tasks: technicians only) |
| 8–11 | `pistols`, `rifles`, `disruptors`, `harness` | the recipe is known (its `requires_tech`), and **new** `per_role`: stored + held items under one per living body of the role | the craft |
| 12 | `arm` | **new** rung (the raid rung's `arm` on its own): an unarmed or unarmoured body of the roles, a better item in storage | `orderEquip` for the best weapon (by expected damage) and, with `armour`, any empty armour slot |
| 13 | `guard` | always | soldiers and heavies hold posts 5 out from the centre and answer hostiles within 10 (E2 guard) |
| 14 | `emitters` | the emitter known, fewer than 2 standing or carried, `stay_left` ≥ 6 | build at the hull's breaches facing the colony when it is on the level, else evenly; lancers posted one per emitter (E3 `posts`) |
| 15 | `array` | the array known, none standing, **new** `tier_min` 1 (freighter or above), `stay_left` ≥ 12 | build one beside the hull |
| 16 | `specimens` | stored specimens under 4 | attack orders on wildlife (the hunt rung with the doctrine's `kill_drops`, as the hive's biomass) |
| 17 | `dig` | no ore in reach on the surface (as now) | stairs and tunnels |
| 18 | `skimmers` | skimmers idle | patrol a ring at 30; contact records `Site.Scouted` (E2) |
| 19 | `raid` | hostile, the colony on the level, contact made, fighters ≥ 2 | the raid rung; **leader** heavy once heavies exist (E3's `leader`); otherwise as now |

New rung features, each small:

- **`stay_left`**: the director's `Site` gains `StayLeft` (days), from the
  fleet: `stay_days − (today − LandedDay)`, and 0 once the hold has reached
  the target (it is leaving at the next daily step). Planet presences pass
  a large number, so the field means nothing for them.
- **`tier_min`**: the ship's hull tier, beside E2's `stage_min`.
- **`per_role`** on the craft rung: the wanted count from living bodies of
  a role (held and stored items both count).
- **`arm`** as a rung: the raid rung's arming, with `armour` (armour slots)
  and `best` (swap for a better weapon). It is the raid rung's code moved
  out, so the raid keeps calling it.
- **`reserve`** on the craft and build rungs, read like the grow rung's.

Ordering notes:

- `grow` stays first, as in Phase D, so the crew reaches its cap (the
  Tier 1 rule) before anything else is bought.
- `armory` and `alloy` sit above `research` because almost every entry
  costs alloy.
- Weapons and the `arm` rung sit above `guard`, so a soldier posted is an
  armed soldier.
- `specimens`, `skimmers` and `raid` send bodies away from the hull, so
  they come last, as in the hive and autonz.

### The campaign daily step and the level

| Tier 1 (per day) | On the level |
|---|---|
| a landed ship grows `per_day` 0.05 toward the site's capacity under the tier's crew cap (as now) | the `grow` rung prints at the vat up to the same cap, paying alloy |
| the hold gains `yield` by tags (as now) | mining and hauling into the cockpit locker and ground lockers; `salvage` speeds mine work live only |
| the spine and the second walk, while landed | the console and the array, technicians only; reconciled techs and progress |
| leaves at the stay's end or a full hold (as now) | `liveHold` each day, then `DepartShip` and `liftOffLive` (as now): modules packed, ground structures wrecked, gear and progress recorded |
| docks: hold and techs to the mothership, a tier bought (as now) | — |
| `units` from the mix; `built` from `ShipBuilt`; `gear` floor | living bodies by type; standing ground structures; held items |

### Materialise and reconcile

**Materialise** (a new `materialiseGreyish` in `faction_greyish_settle.go`,
as the hive and autonz have theirs; the generic path stays for the
mirror):

1. **Centre** as now.
2. **Camp**: the saucer stamp (as now), the cockpit locker claimed as the
   hold (as now) and made a pocket door (`Pocket` set, step 2b), doors
   opened to the faction (as now).
3. **Modules**: the hull tier's `modules` filtered by known techs, owned by
   the settlement and tagged with the presence (`Kin` structure): the vat
   by the centre, the console in the cockpit, the armory and emitters by
   the director's own site rule.
4. **Ground structures**: `built` if recorded, else `ShipBuilt(tier,
   techs, daysLanded)`, placed by the same site rule the build rung uses
   (E3's `autonzSite` pattern: the director's grid, here around the hull's
   outer ring, emitters at the breaches). A ship that has been landed a day
   has none; one that has been there three weeks at the freighter tier with
   the array known has its array, armory and emitters.
5. **Stock**: the hold into the cockpit locker (as now); past its 20
   slots, into the pocket's other doors, else on the ground by the centre
   (`fillFactionStock`'s ground branch, as for a camp with no container). An empty hold on a
   first build gets nothing extra (ships start empty; open question 13).
6. **Bodies by type**: topped up or culled to `units` (an old save's empty
   `units` filled from the mix), each with its type's stats and upgrades.
7. **Gear**: the record's `gear` (or the floor) handed out, soldiers first,
   then heavies, best item first, worn and held.
8. **Adopt Phase D bodies** (once, idempotent): a crew body with no
   `Kin.Type` is replaced on its tile by its type's print: `greyish_crew`
   and `greenish_crew` (today's hauler and **miner**) → worker,
   `whitish_crew` → technician, `pinkish_crew` → soldier. A green miner
   becomes a grey worker rather than a free heavy (open question 2).

**Agreement between materialise and the director** (the autonz' "the two
must agree"): a test lands a freighter abstractly for 20 days with the
array known and materialises it; another lands it live with enough in the
hold and runs the director for 20 days. Both hold the same kinds and
counts of ground structure. As in E3, positions agree only where the grid's
first free site is the same, and the live count agrees only when the hold
paid for it; the test gives the live ship the hold to do so.

**Reconcile** (every write of the level): `units`, `built`, `gear`, techs
and research progress, the hold (as now). A wipe is no bodies (as now,
`wipeShipLive`); structures do not count.

**Lift-off** (`liftOffLive`, extended): reconcile first (as now), then
modules removed, ground structures replaced by their wrecks, bodies and
gear aboard, storage emptied (as now), the scar stamped by the ruin path
(at once on a loaded level, as now).

**Boarding** (`materialiseBoarding`, extended): the boarded ship's crew by
`units`, each in its type's fighting form (`armBoarder` given the type's
stats) and holding its `gear`; a warship's deck carries its emitters; a
mothership's deck two more with `grey_mothership_bulwark`.

### Interception

Two entries act here and nowhere else:

- `grey_tractor_lock`: the player's evade roll against a grey ship uses
  `catch_chance + 0.1` (scout 0.2, freighter 0.35, warship 0.55, mothership
  0.8).
- `grey_cloak`: a jump's line meets a grey ship in flight only within
  `intercept.range − 2` (4 instead of 6), so the player meets cloaked greys
  less often. The colony's ship scanners do not counter it in this phase
  (open question 15).

Both read the ship's `KnownTechs`. Neither changes a fleet's movement, so
the pace is untouched; `faction_intercept_test.go` gains rows for each.

### Scenarios and the GameMaster

- `grey_contact` and `crashed_ufo` keep their wild greys (scenario
  blueprints, scripted AI, no type, today's stats and looks). With a grey
  ship landed there, the camp builds as now.
- `hostile_max` counts the settlement's `Kin` workers (E2's rule), now
  that the greys have `Kin`.
- The emitter turret runs `sentry_turret.basic` under faction `greyish`:
  step 2 checks it never shoots its own settlement's bodies or a wild grey
  (`find_nearest_worker` skips the caller's faction since E2), and that
  hostility seeding gives it the settlement's stance toward the colony.

### Phase F: what a greyish player will see

Not built here. F's step 6 is "ship mode with a grey crew": the player
flies a ship (`CampaignMode` `ship`), `SeedShipCrew` spawns
`CrewBlueprints(n)` by the doctrine's crew split, and the ship's door
faction follows the player's doctrine. The data above is shaped for it.

**The owner's direction (2026-11-09): a grey player's ship works like the
colonists' ship, as a saucer you upgrade.** The greys lean toward a fancy
ship rather than bases on planets. The colonists sit in the middle,
equipped for both, and the hive and autonz hold planets and have no ship.

| | Hive, autonz | Colony | Greyish |
|---|---|---|---|
| Home | planets (no ship) | the ship and planet bases, both easily | **the saucer first**; planets are where they mine |
| Ship shape | — | a grid of rooms in open space, built out freely tile by tile (`internal/ship/ship.go`: a 100×100×4 level, a 3×2 grid of 6×6 rooms at the centre of deck 0) | a round hull grown in **rings** (a bigger radius) and **decks** (new floors), each an upgrade |
| Inside | — | free | free: the player organises the rooms and structures inside the hull |
| Planet building | everything | everything | support only (decided): linked lockers, the armory, emitters and mining; the vat, console and array are **ship-only** |

#### The saucer (the grey ship level)

- **Same level, different layout.** The ship level stays the colony's
  100×100×4 always-loaded level with its own settlement (`the_ship.md`, "The
  ship is its own settlement"). A doctrine `ship` block picks the layout:
  `grid` (the colony's, today's code, the default) or `saucer`.
  `ship.BuildShipLevel` takes the layout; the saucer is drawn by the same
  round-room carving the rival's landed saucer uses (`carve_round_room` in
  `crashed_ship.basic`), at the centre of deck 0.
- **The start**: one deck at radius 8 (about 200 floor tiles, near the
  colony's starting grid of 216). Aboard: the hold (as the colony's), the
  **vat** and the **console** (the scout's modules) and a locker. Crew: a
  worker, a technician and a soldier with a beam pistol.
- **Rings.** Radius 8 → 10 → 12 → 15 → 18 on any deck. Deck 0 can reach 18
  inside the 100×100 level with room to spare.
- **Decks.** Up to the level's four. A new deck sits above the top one with
  a radius no larger than the deck below it, and is joined by the existing
  `stairs_up` / `stairs_down` buildables.
- **An upgrade is physical work, not a swap.** *Expand hull* (choose a deck)
  and *Add deck* queue ordinary orders through the player's paths: dig out
  the old rim wall, lay floor on the new ring, build the new rim wall, place
  the stairs. Workers haul and build every tile, as for any colony
  construction. Until the new rim wall is closed the old rooms stay sealed,
  so an upgrade in progress never opens the ship to space.
- **Why it is expensive.** The grey hull tiles are their own buildables
  (`grey_hull_floor`, `grey_hull_wall`), costing alloy as well as metal, so a
  ring of radius 15 is a few hundred alloy-laced tiles. Each ring step and
  each deck is gated by research (data in the `ship` block, below), so the
  hull grows with the tree.
- **Where the gates live.** Two entries under the same `greyish` tree,
  player-facing only (no rival reads them, Tier 1 never sees them):
  `grey_hull_lattice` (rings 12 and 15, decks 2 and 3) and
  `grey_hull_spine` (ring 18, deck 4), after `grey_hull_plating`. The ring
  to 10 and the second deck need only alloy.

```json
"ship": {
  "layout": "saucer",
  "saucer": {
    "start": { "radius": 8, "decks": 1 },
    "rings": [ { "radius": 10 },
               { "radius": 12, "requires_tech": "grey_hull_lattice" },
               { "radius": 15, "requires_tech": "grey_hull_lattice" },
               { "radius": 18, "requires_tech": "grey_hull_spine" } ],
    "decks": [ { "deck": 1 },
               { "deck": 2, "requires_tech": "grey_hull_lattice" },
               { "deck": 3, "requires_tech": "grey_hull_spine" } ],
    "floor": "grey_hull_floor", "wall": "grey_hull_wall"
  }
}
```

- **The Hull section** of the research modal is this ship in a player run:
  each deck's radius, the next ring and deck with their gate and tile cost,
  and *Expand hull* / *Add deck* buttons (open question 17).
- **The rival's saucer** keeps its 17×17 landed stamp. Making the rival's
  stamp radius follow its hull tier (scout smaller, warship larger), so the
  player's upgrades read as the same ships, is open question 22.

#### Linked lockers (decided 2026-11-09)

The owner's words: "lockers are also all the type of lockers that can beam
directly to the ship like the starting locker for colonists. All of them
are linked too." Open questions 24 and 25 then made "linked" the storage
pocket (see "The storage pocket"), for rivals as well as the player.

**How the colonists' beam works today** (checked for this plan):

- The **storage inspector** opens on a click on any container
  (`main_state.go`, the click handler; `storage_inspector_modal.go`). Its
  direction comes from the level: a container on the ship beams **down**
  (`WorldManager.BeamResourceDown`: from the ship hold into the current
  site's colony-owned containers, the first with room first), and a
  container on the site beams **up** (`BeamResourceUpFrom(container, …)`:
  from that container into the ship's beam hold, the `teleporter` if one
  stands aboard, else the ship's first container). Only what lands is
  debited.
- **No locker is special in code.** "The starting locker" is just the
  first colony-owned container on the site (`SiteHoldEntity`), which the
  dashboard's *Beam Up* button opens. Every colony container on a site can
  already beam up from its own inspector, and beam-down fills them all.
- The beam is the ship ↔ site exception to physical storage: the ship must
  be in orbit (the site loaded).

**What a grey player gets**, all on the existing beam and the pocket:

1. **One pocket on the site, one aboard.** Every grey locker on the loaded
   site is a door onto the site pocket; every grey locker aboard is a door
   onto the ship pocket. Workers on either level deposit and withdraw at
   the nearest door and never carry between lockers.
2. **Beaming opens the whole pocket.** Beam-up from any site door takes
   from the site pocket (any door's slice) into the ship pocket; beam-down
   from the ship lands in the site pocket. The beam code is unchanged: it
   already calls the component's own take and add, which the pocket wraps,
   and `BeamResourceDown`'s "first with room" becomes "the pocket's room".
3. **One inventory view.** The global inventory's "all" and "detailed"
   views, which the colony unlocks by research (`global_inv_all`,
   `global_inv_detailed`), are open to the greys from the start: the two
   pockets are the grey player's whole stock.

**What a rival grey gets**: its own pocket on its site (see "The storage
pocket"). It has no ship level and no beam; at lift-off its whole pocket
goes into the record's hold, as all its storage does today.

**Pockets never meet.** The player's pockets and each rival's are keyed by
their own settlement name, so a rival's door is never a door onto the
player's pocket, and a player's colonist cannot withdraw at a grey door.

**Scope.** The owner's words left one point open: "the same
inventory on the planet" against grey lockers that beam to the ship. The
default is **one pocket per owner per level** (the site's and the ship's,
joined by the beam), because that is what `OwnedBy` and the per-level
storage lookups already give, and the ship stays reachable only in orbit.
This is **decided (2026-11-09, open question 26): one pocket per owner
per level.** The alternative (the pocket *is* the ship's hold, with no
beam) was not taken.

**The owner-check gap, closed here.** `BeamResourceUpFrom` has no owner
check, and the inspector opens on any container, so today a colony player
can beam up a rival camp's locker. A pocket door would hand over the whole
pocket that way, so this phase adds the check for doors: the inspector
offers a beam only from containers owned by the viewer's settlement (the
colony's or the ship's). The same check on ordinary lockers is a one-line
extension the owner may want for the mirror's camps too; it is noted, not
assumed.

#### The rest of the player's side

- **Build menu** (`build_menu`, `buildables`): *Structures*: locker,
  armory, array, emitter, vat, console, the grey hull floor and wall, the
  stairs, each placeable by the player (`module_only` binds the rival only),
  locked entries naming their research. The vat, console and array place
  only on the ship level (a buildable `ship_only` flag, refused on a
  planet), the rest anywhere (decided: planets get support only). Every
  locker is a linked beam locker (above). *Hull*: *Expand hull*, *Add deck*.
- **The vat.** A click opens the crafting modal with the six print rows
  (cost, time, locked rows naming their entry). There are no larvae or
  eggs: a print is a craft order a technician works.
- **The armory.** The crafting modal: refine alloy, the weapons and the
  armour, locked rows naming their entry.
- **The console and the array.** The research modal over the `greyish`
  tree, with a **Hull** section on top (for a rival ship: its tier, crew cap
  and modules; in a player run: the saucer's decks, rings and upgrade
  buttons, above). Effect lines from `develop` ("Unlocks Heavy", "Soldier,
  Heavy, Lancer: AC +2", "Crafts Beam Rifle", "Builds Research Array",
  "Prints 25% faster", "Harder to evade").
- **The `interface` block** (F step 2's shape):

```json
"interface": {
  "resources": [ { "id": "metal_ore", "source": "storage" }, { "id": "crystal", "source": "storage" },
                 { "id": "grey_alloy", "source": "storage" }, { "id": "specimen", "source": "storage" } ],
  "unit_label": "_draft grey", "units_label": "_draft greys",
  "research_label": "_draft Research",
  "orders": [ { "id": "mine" }, { "id": "chop" }, { "id": "haul" }, { "id": "dig" }, { "id": "attack" }, { "id": "cancel" } ],
  "unit_modal": [ "type", "stats", "equipment", "tasks" ],
  "crew_group": "type",
  "panels": {}
}
```

  The unit modal keeps the colonist's equipment section (greys wear and
  hold gear, unlike the hive and autonz); *Sleep* is absent (no bed, no
  need); the crew panel groups by type. No structure needs a panel: every
  grey structure is a crafting or research modal.
- **Needs.** None: the incident tables that hit hunger, sleep or morale
  are skipped for a crew without needs (F step 6's own item).
- **The win (decided 2026-11-09).** Ship mode's reach-Home win, unchanged.
  Per-faction objectives (a later plan the owner is sketching) may add
  others; this plan adds none.

## Balance

**Tier 1 is unchanged.** Step 0 records the greyish table before anything
else (`make history-dump HFACTIONS=3,4,5 HFSEEDS=1-6
HFCOLS=0,40,100,200,300 HFDETAIL=1`, now with ship lines, and a Go golden
test of the same numbers). Every later step must leave it byte-identical.
`TestFactionPaceRivalOne` passes as it is (it never seeds a grey).

**Live targets** (the greyish soak):

| Measure | Target |
|---|---|
| armory | standing (built or carried) by day 3 of a landing on 9 of 10 seeds with ore in reach |
| alloy | 6 or more refined within the first 10 days on scout seeds with crystal in reach |
| soldiers armed | every soldier holds a beam weapon by day 5 (the gear floor at materialise; live prints armed within 2 days of the armory) |
| research | 2–3 entries per 30-day landing with one technician; the array's first entry on warship seeds |
| prints | the crew back to its cap within 10 days of losing a body when the hold covers it; never above the cap |
| the hold at lift-off | at least half the hold target on 8 of 10 seeds (spending stays above the reserve) |
| ground structures | on warship seeds: the array by day 14, two emitters by day 10, lancers posted when known |
| raid | never in the neutral soak |

**Expected first problem: crystal.** Alloy, prints, weapons, the array and
most entries want crystal, and base sites yield none. If scout seeds stall
on crystal, the default fix is cheaper alloy (`refine_alloy` at 4 metal +
1 crystal) before any change to the units.

**Second problem: the technician.** One technician researches, refines and
prints. If research stalls behind crafting, the default fix is to let the
worker run `refine_alloy` (add `craft` to the worker's tasks), keeping
research technician-only.

## Order of work

Each step leaves the game playable and the suite green. Sizes are estimates
against E2 (the hive build, about 8,500 lines with tests and data).

### Step 0: Make the greyish pace visible

- `cmd/history-dump/factions.go` prints ships as ships under `-fdetail`:
  per fleet its doctrine, kind (ship, mothership, egg), tier, state
  (landed, flying, returning, docked-waiting), crew, hold, techs; per
  mothership its hoard. Eggs keep their line.
- `internal/campaign/faction_greyish_pace_test.go`: `TestGreyishPaceRivalThree`
  (seeds 1–6, Rival factions 3, days 40/100/200/300: ships, tiers, landings,
  hoard, techs) as a golden table from today's code.
- **Files:** `cmd/history-dump/factions.go`,
  `internal/campaign/faction_greyish_pace_test.go`.
- **Done:** the greyish rows print, and the golden test passes on `dev`
  before any other change.
- **Verify:** `go test ./internal/campaign -run Greyish`; the existing
  history-dump table (no `-fdetail`) is byte-identical.
- **Size:** about 250 lines (3%).

### Step 1: Types, crew bodies and items

- `types`, `type_fallback`, the stage `mix` in the greyish doctrine; `int`
  on a type in `ApplyType`.
- Crew blueprints: `Kin`, `NeverSleep`, no starting tentacles but the
  heavy's; new `skimmer_crew` (480) and `lancer_crew` (624) with `flying`,
  `DefaultWeaponColumn` 1 and `"*": 2`.
- Items: `grey_alloy` (new: stack, tags, tinted sprite), `specimen`, the five
  grey weapons and two armour pieces with their `GearDrop` chances,
  `grey_lance`, `grey_emitter_beam` (natural: no `GearDrop`). The
  `GearDrop` component and its roll in `OnEntityDead`.
- Crew entries carry `type`; materialise applies it (the generic path, for
  now).
- **Files:** `data/factions.json`, `data/blueprints/entities/greyish_crew.json`,
  `data/blueprints/items/{resources,weapons,armor}.json` (`relics.json`
  untouched), `internal/components/GearDropComponent.go` (new),
  `internal/factory/component_registry.go`, `internal/game/main_state.go`
  (the roll in `OnEntityDead`),
  `internal/factory/type.go`, `internal/campaign/doctrine_types.go`.
- **Done:** every grey type spawns with its sprite, stats, Int, role and
  tasks; a soldier holding a beam rifle is drawn in the gun pose; a lancer
  in a fight in the legs-hanging pose.
- **Verify:**
  - `internal/factory/type_test.go` (a greyish row per type);
  - a content test that every type's cells are non-empty on the sheet;
  - the identity content test (hauled-unit costs and fight values ordered
    as "Identity in numbers");
  - `internal/game` `TestGearDrop` (an item with `GearDrop` drops on a
    seeded pass and not on a fail; an item without it is lost as today;
    a colonist's and an autonz' death unchanged);
  - `TestGreyishPaceRivalThree` and the step 0 table unchanged;
  - `make vmtest SCRIPT=factions_greyish` (frames differ in stats only).
- **Size:** about 900 lines (11%).

### Step 2: Structures, modules, recipes and wrecks

- Blueprints `grey_vat` (station), `grey_console` (research building),
  `grey_armory` (station), `grey_array`, `grey_emitter` (turret), the three
  wrecks with `Drops`. `module_only` buildables. `ships.tiers[].modules`.
- Recipes: the six prints at `grey_vat` (four ids kept), `refine_alloy`,
  the weapons and armour; `print_*` costs match the types.
- The emitter under faction `greyish`: the sentry script's target pick and
  hostility seeding checked.
- **Files:** `data/blueprints/structures/{workbenches,furniture}.json`,
  `data/blueprints/entities/greyish_structures.json` (new), `data/build.json`,
  `data/crafting_recipes.json`, `data/factions.json`,
  `internal/construction/buildables.go`, `internal/campaign/faction_ships_d.go`
  (tier modules), `data/scripts/ai/sentry_turret.basic` (only if the check
  finds a gap).
- **Done:** in the cheat console, a technician prints a soldier at a vat
  from alloy, refines alloy and makes a beam rifle at an armory; an emitter
  shoots a colonist and ignores greys; a destroyed wreck drops alloy; the
  build rung refuses a vat.
- **Verify:**
  - `internal/construction` `TestModuleOnlyRefused`;
  - the content test (type cost = recipe cost; every grey recipe's station
    exists; the colony's `bio_printer` has no grey recipe and the vat no
    colony recipe);
  - `internal/game` `TestGreyEmitterTargets` (colonist yes, grey no, wild
    grey no);
  - `TestColonyDoctrineCoversContent` (the new entries belong to the greys).
- **Size:** about 1,300 lines (15%).

### Step 2b: The storage pocket

- `StorageComponent.Pocket` (saved); the doctrine's
  `settlement.storage_pocket`; claim and build set it on the settlement's
  containers.
- `storage.PocketView`; the finders in `internal/workerai/storage.go` read
  a door as its pocket (holds, room, nearest door); the take and put
  wrapper used by the deposit, fetch, equip, retrieve and edible paths.
- Door destroyed: contents into the surviving doors; the last door spills
  the pocket (one stack a tile, nearest first, `fillFactionStock`'s ground
  rule), stored gear rolling `GearDrop`.
- For a rival ship: the cockpit door re-placed as a module a day after the
  last door falls (`DoorLostTick`); the `"haul": "any"` rung value.
- For a grey player: a beam-down to a site with no grey locker places one
  (2 metal from the ship pocket).
- The beam: `BeamResourceUpFrom` and `BeamResourceDown` through the
  wrapper; the inspector's owner check for doors.
- **Files:** `internal/components/StorageComponent.go`, `internal/world/save.go`,
  `internal/storage/storage.go`, `internal/workerai/{storage,states,task_handlers,diet}.go`,
  `internal/game/main_state.go` (door death), `internal/game/faction_settlement.go`
  (claim, the spill's ground placement), `internal/game/faction_greyish_settle.go`
  (the door module), `internal/director/director.go` (`haul: "any"`),
  `internal/campaign/faction.go` (`DoorLostTick`),
  `internal/game/world_manager.go`, `internal/game/storage_inspector_modal.go`,
  `internal/campaign/doctrine_settlement.go`, `data/factions.json`.
- **Done:** in the cheat console, a grey worker drops ore at locker A and a
  technician takes it at locker B across the camp; smashing A moves its
  slice into B; smashing B too spills the pocket around B's tile; a day
  later a rival ship has its cockpit door back and its workers haul the
  rest of the spill into it; a colonist cannot take from or beam up a
  standing grey door; a colony camp's lockers behave exactly as before.
- **Verify:**
  - `internal/workerai` `TestPocketDoorHoldsWhatPocketHolds`,
    `TestPocketNearestDoor`, `TestPocketTakeFromOtherSlice`,
    `TestPocketRoomIsTheSum`, `TestPocketRaceReplans`;
  - `internal/game` `TestPocketDoorDestroyed`, `TestPocketLastDoorSpills`
    (one stack a tile, overflow piles, stored gear rolls `GearDrop`),
    `TestRivalSpillReconcile` (the stockpile drops by the spill; Tier 1's
    step unchanged), `TestRivalDoorRestored` (the module a day later, the
    `any` haul), `TestRivalLiftOffNoDoor` (packs nothing, the spill stays),
    `TestPlayerBeamPlacesDoor`, `TestSpillFreezeThaw` (loose items kept,
    nothing counted twice),
    `TestPocketSaveRoundTrip`, `TestPocketOwnersNeverMeet` (two rival grey
    settlements and the player's on one level), `TestBeamOwnerCheck`;
  - `storage.Summarize` over a settlement with and without `Pocket` gives
    the same totals;
  - the colony's storage tests and soaks unchanged (no colony container
    has `Pocket`);
  - `TestGreyishPaceRivalThree` and the step 0 table byte-identical.
- **Size:** about 950 lines (11%).

### Step 3: The tree and the second walk on ships

- The sixteen `tree: "greyish"` entries with `develop` blocks (the three
  existing ids real). `develop` read beside `evolve` and `assimilate`:
  `unlock` through `TypesUnlocked`, `upgrade` through E2's upgrades,
  `print_time` at the vat, `salvage` in the mine work, `intercept` in
  `DetectInterception` and the evade roll, `boarding` in
  `materialiseBoarding` (step 4 uses it).
- The second walk generalised (`second_walk`, `second_walk_per_day`; the
  autonz keys as aliases); `stepShip` walks it by hull tier while landed.
- The content test that no grey entry has a Tier 1 effect.
- **Files:** `data/research.json`, `data/factions.json`,
  `internal/research/*.go`, `internal/campaign/faction.go`,
  `internal/campaign/faction_ship.go`, `internal/campaign/faction_intercept.go`,
  `internal/workerai` (mine work, `salvage`).
- **Done:** a grey presence at day 200 knows the spine's entries on the
  same days as before and part of the second walk; the autonz' walks are
  unchanged.
- **Verify:**
  - `internal/campaign/faction_greyish_tree_test.go` (both walks; the
    spine's days unchanged; no Tier 1 effect; techs merge at dock; a save
    without the new keys loads);
  - `TestGreyishPaceRivalThree` and the step 0 table byte-identical;
  - `faction_autonz_tree_test.go` unchanged (the aliases);
  - `faction_intercept_test.go` rows for tractor lock and cloak;
  - the colony research snapshot unchanged.
- **Size:** about 1,200 lines (14%).

### Step 4: The ship as a settlement (materialise, reconcile, lift-off, ruins, boarding)

- `materialiseGreyish`: modules by tier and techs, ground structures from
  `built` or `ShipBuilt`, bodies by type, gear by record or floor, Phase D
  adoption. `Doctrine.ShipBuilt`.
- Reconcile: `units`, `built`, `gear`, research progress.
- `liftOffLive`: modules packed, wrecks placed, gear recorded.
- Ruins: the landing scar instead of the broken hull; unseen departures
  build the scar from `built`.
- Boarding by `units` and `gear`, the warship's emitters, the bulwark.
- The director runs Phase D's table plus `research` and `arm` at this
  step, so a live ship researches and arms; the rest is step 5.
- **Files:** `internal/game/faction_greyish_settle.go` (new),
  `internal/game/faction_settlement.go`, `internal/game/faction_ship_live.go`,
  `internal/game/faction_ruin.go`, `internal/game/faction_encounter.go`,
  `internal/campaign/faction.go` (`Gear`), `internal/campaign/faction_ships_d.go`,
  `data/scripts/structures/crashed_ship.basic` (a `scar` class beside
  `saucer`), `data/factions.json`.
- **Done:** seed 55 with Rival factions 3: landing on Ember Pit shows the
  saucer with a vat and a console, three typed greys, the soldier armed;
  after `debug days 60` and a revisit, a freighter's camp with its armory
  carried; watching it lift off leaves a scar with wrecks, no hull, no vat;
  a save, a freeze and a thaw round-trip it.
- **Verify:** `internal/game/faction_greyish_test.go` (new):
  - materialise is deterministic and idempotent;
  - modules by tier and techs; ground structures by `ShipBuilt`;
  - materialise and a live run agree on kinds and counts (above);
  - gear handed out, best first; the floor only without a record;
  - adoption runs once (a green miner becomes a worker);
  - reconcile round-trips `units`, `built`, `gear`, techs and progress;
  - lift-off leaves wrecks and no modules; a ruin stamped unseen matches
    one stamped live;
  - a boarded warship has its emitters and armed crew;
  - `faction_ship_live_test.go` and `faction_ship_ruin_test.go` updated.
- **Size:** about 1,700 lines (20%).

### Step 5: The greyish director

- New rung features: `stay_left`, `tier_min`, craft `per_role`, the `arm`
  rung (armour, best), `reserve` on craft and build rungs; the hunt rung on
  `kill_drops` items other than food, if it does not already take them
  (the hive's biomass path is checked first); raid `leader` heavy.
- The full table in the doctrine. `Doctrine.RaidAlert` for the greys as
  themselves.
- **Files:** `internal/director/director.go`, `internal/director/raid.go`
  (arming moved out), `internal/director/arm.go` (new),
  `internal/director/grow.go`, `internal/campaign/doctrine_settlement.go`,
  `internal/game/settlement_orders.go`, `data/factions.json`.
- **Done:** a freighter left live for 30 days on an ore site with crystal
  builds or unpacks its armory, refines alloy, researches two or three
  entries, arms its soldiers with rifles once known, builds the array and
  emitters when the stay allows, hunts specimens, and lifts off with at
  least half its hold target.
- **Verify:**
  - `internal/director/greyish_test.go` (each rung's trigger and order:
    grow under the cap only, the armory skipped when carried, `stay_left`
    gates the array and emitters, `per_role` counts held items, `arm`
    picks the best weapon and fills armour slots, the reserve stops
    spending, raids form around a heavy);
  - `internal/game/faction_greyish_live_test.go` (an emitter's lancer
    posted; guards answer a colonist; a hostile ship raids after contact;
    colonists smash every grey locker, carry part of the spill off, and
    the ship's workers haul the rest back after the door returns).
- **Size:** about 1,100 lines (13%).

### Step 6: Soak, vmtest, docs

- **The soak** `TestGreyishSoak` (`internal/game/greyish_soak_test.go`), in
  `scripts/soak_par.sh` as `make soak-greyish` and in `make soak`; the join
  test `TestGreyishSoakJoin`. 22,500 ticks (a full 30-day stay), neutral,
  two starts:
  - **scout** (ten seeds): a scout of three landed on an ore site with
    today's kit (12 metal, 6 crystal), no techs;
  - **warship** (five seeds): a warship of eight (3 workers, 2
    technicians, 2 soldiers, 1 heavy) with 40 metal, 16 crystal, 8 alloy,
    4 specimens and the spine's three entries plus the armory module,
    beam weapons and field emitter known.

  Columns per seed:

  | Seed | Start | Crew start / end (cap) | Prints by type | Armory tick (built or carried) | Alloy refined | Weapons made by kind | Armed fighters at end | Techs (first tick) | Array / emitters tick | Specimens | Hold start / at the end (target) | Departure (hold or stay) | Cancelled |

  A seed **misses** when no armory stands by tick 3,000, when no research
  entry is learned by the end, or when the hold ends below a quarter of the
  target. More than two misses per start fails. `SOAKSEEDS`, `SOAKTICKS`
  and a new `GREYTICKS` (22,500) apply. The numbers are recorded in "As
  built".
- **history-dump**: the step 0 ship lines gain units by type, `built`,
  `gear` and the second walk's techs under `-fdetail`. The Factions tab's
  grey rows list units by type and the hull tier's modules.
- **vmtest**: `factions_greyish` keeps its frames and asserts (seed 55,
  Rival factions 3, Ember Pit) and adds frames of the vat, console and
  typed crew; new `factions_greyish_warship` (an observer run, `debug tech`
  granting the array, field emitter and lancer pattern, `faction tier
  greyish 2`, a **new** cheat, so the frame does not depend on the day): the
  warship's camp with its emitters and lancers, the array, armed soldiers
  and a heavy; then the lift-off and the scar with wrecks.
  `factions_intercept`: the mothership's deck with two emitters after
  `debug tech greyish grey_mothership_bulwark`. The sidecar gains `greyish`
  (units by type, built, gear, modules).
- **Docs**: `factions.md` ("The greyish": roster, tree, modules and ground
  structures, ruins, the second walk on ships; the table in "How a faction
  moves and acts"), `vmtest.md`, `enemy_factions_plan.md` (E4 in the
  phasing table, "Phase E4 as built"), `todo.md`, and the Phase F plan's
  "Greyish" section pointing here.
- **Files:** `internal/game/greyish_soak_test.go` (new), `scripts/soak_par.sh`,
  `Makefile`, `cmd/history-dump/factions.go`, `internal/game/factions_panel.go`,
  `internal/game/console*.go` (the tier cheat), `internal/game/vmhook.go`,
  `vmtest/scripts/factions_greyish.jsonl`,
  `vmtest/scripts/factions_greyish_warship.jsonl` (new),
  `vmtest/scripts/factions_intercept.jsonl`, the docs above.
- **Done:** the soak passes, the frames are inspected, the docs say what was
  built.
- **Verify:** `make soak-greyish`; `make soak`; `make vmtest
  SCRIPT=factions_greyish`, `factions_greyish_warship`,
  `factions_intercept`; every other vmtest script passes (those with a grey
  ruin differ in the ruin only); `go test ./...`.
- **Size:** about 1,500 lines (18%).

Step 0 comes first and alone. Steps 1 and 3 touch different files and can
run side by side after step 0; step 2 follows step 1. Step 2b touches only
storage and can run beside steps 2 and 3; step 4 needs it. Steps 4 and 5 run in
order. The history-dump and panel part of step 6 can start after step 4.

**Total: about 8,900 lines, 1.05 times the hive build** (7,950 before the
pocket, 8,750 before the spill). F's step 6 (ship mode
with a grey crew) is on top: F's own estimate of about 1,000 lines, plus
about 900 for the saucer (the `ship.layout` switch, the round ship level,
*Expand hull* and *Add deck* as batches of existing orders, the grey hull
tiles, `ship_only` placement, the Hull section's buttons), plus about 100
for linked lockers (the global inventory views open to the greys; the
pocket itself is step 2b): **about 2,000**.

## Open questions (each with a recommended default)

Questions 1, 3, 4, 6, 9, 16, 21 and 23 to 28 were decided on 2026-11-09
(see "The owner's decisions" under "Decision"); the rest keep their
numbers.

2. **Phase D bodies in saves.** Default: replaced once by their type's
   print (miner and hauler → worker, scientist → technician, soldier →
   soldier), so a green miner does not become a free heavy. Alternative:
   adopt by blueprint (green → heavy), which is simpler and upgrades old
   camps.
5. **Lancers in the stage mix.** Default: none; lancers come only from the
   emitter posts (one per emitter), so Tier 1's composition never names
   them and materialise agrees with the director. Alternative: a small
   lancer share in the mix, which the grow rung then fills too.
7. **Needs.** Default: `needs: false` and `NeverSleep` on the crew.
   Alternative: the power-cell upkeep sketched in "Needs" (data only, with
   its own soak).
8. **The ruin.** Default: the hull leaves with the ship; the ruin is a
   landing scar with wrecks and the datapad. Alternative: today's broken
   hull stays (cheaper, but the ship that left also stays).
10. **Power.** Default: no power system. Alternative: a `grey_reactor`
    module that the array and emitters need standing within range (new
    code for flavour).
11. **Research speed.** Default: 500–1,200 work steps per entry, technicians
    only, about 2–3 entries a 30-day landing. Alternative: faster (the
    colony's 60–220) to make "high tech" read as fast learning rather than
    as rare researchers.
12. **The gear floor.** Default: abstract ships are assumed armed by their
    techs (the E3 stock-floor precedent). Alternative: no floor; a ship the
    player first meets has unarmed soldiers until its armory works.
13. **Provisioning ships from the hoard.** Default: no; ships start empty as
    now, so the pace cannot move. Alternative: a ship draws a supply of
    alloy from the hoard at launch (a Tier 1 change and a new pace table).
14. **Outposts.** Default: not in this phase. Alternative: a long-stay
    outpost as a planet-carried grey presence, its own phase and pace.
15. **Cloak against scanners.** Default: the colony's ship scanners do not
    counter the cloak. Alternative: each `ship_scanner_N` the player knows
    gives back one of the cloak's two range points.
17. **The player's ship upgrades.** Decided in direction (2026-11-09): the
    saucer grows in rings and decks, built tile by tile. Open within it:
    the default ring steps (8, 10, 12, 15, 18), four decks, and two gating
    entries (`grey_hull_lattice`, `grey_hull_spine`). Alternatives: every
    ring gated by its own entry; or ungated, paid by alloy alone. There is no
    player mothership and no hull tier (giving the player those would be a
    large F change).
18. **Docked ships with nowhere to go.** Default: unchanged (today's
    behaviour, kept so the pace does not move). Alternative: let a waiting
    ship land on a site another faction holds, beside it (a separate change
    with its own pace table).
19. **The armory as a module.** Default: carried from the freighter tier
    once `grey_armory_module` is known; a scout always builds one on the
    ground. Alternative: every tier carries it (simpler, and the scout's
    first days are no longer spent building).
20. **Phase name.** Default: E4, after E3 and before F's step 6.

22. **The rival's saucer size by hull tier.** Default: no, the rival keeps
    its 17×17 stamp (its footprint feeds the presence centre, ruins and the
    lore budget). Alternative: radius by tier (scout 6, freighter 8,
    warship 10), so a rival warship looks like a player's upgraded saucer.
## Where the owner rewrites text

- Type `name`s (`data/factions.json` `types`).
- Print recipe names (the four kept ids get new names) and the armory
  recipes' names and descriptions (`crafting_recipes.json`).
- Every `grey_*` research name and description, including the three
  existing entries' new descriptions.
- Blueprint names and `LongDescription`s: the six crew bodies' types,
  `grey_vat`, `grey_console`, `grey_armory`, `grey_array`, `grey_emitter`,
  the three wrecks, `specimen`, the grey weapons and armour, `grey_lance`,
  `grey_emitter_beam`, and `grey_alloy` (name and description).
- The grey raid alert (`Doctrine.RaidAlert`), a lift-off alert that
  mentions the wrecks, the landing alert if it names the modules.
- Any new Legends event (a `faction_developed` for a learned entry, if
  added; the departure line naming the scar).
- The research modal's Hull section labels, the Factions tab's grey row
  labels, the `interface` block's labels for F.
- The pocket's player-facing name (draft `storage_pocket`, "the pocket"),
  and the alert when the last grey locker falls and the pocket spills
  ("The greys' storage bursts open on …", draft).
- For F's saucer: *Expand hull*, *Add deck*, the `grey_hull_floor` and
  `grey_hull_wall` names, and the two hull entries (`grey_hull_lattice`,
  `grey_hull_spine`).

All of these go in marked `"_draft"`, as the existing files are.

## Riskiest assumptions

- **Tier 1 neutrality holds through every step.** The only guard is step
  0's new table and golden test, because `TestFactionPaceRivalOne` never
  sees a grey. Any effect that leaks into the abstract step (a yield, a
  crew cap, a stay, a cost) moves every Rival-factions-3+ campaign. The
  content test on `develop` effects and the byte-identical table are the
  net; step 0 must land before anything else.
- **A short stay is long enough to show the faction.** A ship leaves at 30
  days or a full hold, and Tier 1's baseline shows many ships idling docked
  after day 200. If the live greys spend their first ten days mining
  before they can refine alloy, the player meets mostly miners. The gear
  floor and the reserve are the levers; the soak decides.
- **One technician is enough.** It researches, refines and prints. If the
  scout's single technician spends its stay at the armory, research never
  happens live and the tree is Tier 1 only in practice.
- **The sentry turret works as a faction structure.** The emitter is the
  only grey thing that attacks without a body; its script was written for
  a neutral hazard. Hostility, targeting and save round-trips need checking
  before the emitter is built on.
- **Changing the ruin is safe.** The ruin path (stamping, footprints, the
  lore budget, unseen departures, vmtest frames) is Phase D's and works. A
  landing scar touches all of it for a fix the player may never notice.
- **Rare beam weapons stay rare enough, and frequent enough.** At the
  default chances a two-soldier raid killed yields a rifle about one time
  in five, so a colony that fights the greys often ends a campaign with a
  few. If the chances are too low the reward is invisible; too high and the
  colony skips its own gun bench. The chances are item data, so tuning
  costs nothing, but no soak measures it (the greyish soak is neutral);
  the hostile live test and play are the only checks.
- **The pocket's finders catch every storage path.** About forty call
  sites touch `StorageComponent.Items` or pick a container. A path that
  reads one door's own slice instead of its pocket (a modal, a quest
  check, a director count) would see a door as emptier than it is. Totals
  are safe by construction (each item is in one slice); takes and "has
  it" checks are where a miss shows. The colony is safe because no colony
  container has `Pocket`.
- **The spill is the right size of reward.** Beating a grey camp now pays
  mostly through its spilled pocket: up to a full hold (30 or more units of
  metal and crystal, plus refined alloy and specimens) against a rifle one
  kill in five from `GearDrop`. A colony that learns to smash grey lockers
  may out-earn its own mining; one that never does sees almost nothing.
  The levers are data and timing: the one-day door timer, the spill radius,
  and the stored-gear roll (decided). No soak measures it
  (the greyish soak is neutral); the hostile live test and play are the
  checks.
- **The rival without a door does not stall.** A ship whose doors are all
  smashed must re-place its cockpit door, keep its miners holding ore, and
  haul the spill back; a mistake leaves a camp that never stores again and
  never reaches its hold target (it still leaves at the stay's end).
- **Adopting Phase D bodies by replacement** works as it did for the autonz
  (E3 replaced Phase E bodies on their tile). A grey camp mid-raid in a
  save would lose its bodies' orders once.

## As built

Built on `claude/greyish-build-571417` (from `dev` plus this plan), step by
step. Rival greys only: the Phase F parts (a grey player, the growing
saucer, the player's side of linked lockers) are not built.

### Step 0: the greyish pace, pinned

- `internal/campaign/faction_pace.go` (read-only helpers): `ShipState`
  (landed, flying, returning, docked, mothership, boarded), `ShipLine`,
  `ShipFleets`, and `Doctrine.SpineTechs`.
- `cmd/history-dump/factions.go`: under `-fdetail` ships print as `ship`
  lines (where, tier, state, crew, hold or hoard, techs); eggs keep their
  `egg` line. The table without `-fdetail` is byte-identical to `dev`
  (factions 0-5, seeds 1-6, columns 0/40/100/200/300, checked with the old
  file as a `go -overlay`).
- `TestGreyishPaceRivalThree` (`faction_greyish_pace_test.go`, golden
  `testdata/greyish_pace.golden`, rewritten only by `-update-golden`): seeds
  1-6, Rival factions 3, stepped day by day to 300; on days 40, 100, 200,
  300 every grey ship's tier, state, crew, hold and **spine** techs, the
  hoard, and landings so far.
- **Differences from the plan.** (1) The golden counts only the spine's
  techs and the crew as a number, never all of `KnownTechs` or units by
  type: the second walk (step 3) adds techs to `KnownTechs` by design, so a
  table of all techs would have to move. On `dev` the two are the same
  number. (2) Landings: the golden counts every landing (each new
  `LandedDay` of a ship, which matches the `faction_landed` events: 15 for
  seed 1). The plan's measured table said 4-12 landings to day 300; its
  throwaway test counted something else (most likely landed ships at the
  sampled days). The other columns agree with the plan's table (seed 1:
  three ships docked at d300, hoard 105 metal / 10 crystal, 3 techs).

### Step 1: types, crew bodies and items

- `data/factions.json` (greyish only): the six `types` with `int`,
  `type_fallback`, the stage `mix`, and the settlement `crew` as typed
  worker / technician / soldier entries (0.4 / 0.25 / 0.35). `int` is read
  by `ApplyType` (`DoctrineType.Int`, `factory.UnitType.Int`).
- `greyish_crew.json`: every crew body has `NeverSleep`; the pink and green
  bodies lose their starting tentacles (the heavy's type equips them, so a
  printed soldier is unarmed until the armory); new `skimmer_crew` (480) and
  `lancer_crew` (624) with the `flying` skill, `DefaultWeaponColumn` 1 and
  `"*": 2`. `Kin` is added by `ApplyType`, as for the autonz.
- Items: `grey_alloy` and `specimen` (`resources.json`), the beam pistol,
  rifle and disruptor, `grey_lance`, `grey_emitter_beam` (`weapons.json`),
  the field harness and cowl (`armor.json`), appended textually so the
  existing entries' formatting is untouched. `relics.json` untouched.
- `GearDrop` (`internal/components/GearDropComponent.go`) and its roll
  (`internal/game/gear_drop.go`, called from `OnEntityDead` before
  `Drops`): only a body holding or wearing a `GearDrop` item draws a number,
  so no other death changes.
- Tier 1: a landed ship's `Units` are normalised to its crew each day
  (`stepShip`, bookkeeping only). The pace golden is byte-identical.
- **Differences from the plan.**
  - **`Doctrine.Broods()`** used to be "has types and has morphs"; the
    greyish doctrine has a `morphs` block (for its names), so it would have
    been taken for the hive. It is now "a type names its morph". `Builds()`
    (the autonz) excludes fleet doctrines, and a new `ShipSettles()` is the
    greys' predicate (typed, a settlement, mobility `fleet`). The hive and
    the autonz answer as before (asserted in `TestGreyishTypesSpawn`).
  - **The worker can `craft`.** With crafting technician-only, a camp that
    lost its technician could never print another (nobody could run the
    vat): a deadlock, and a crew of two (worker + soldier) never printed at
    all. This is the plan's own fallback from "Balance" ("let the worker run
    refine_alloy"), applied from the start; research stays technician-only.
  - **The identity test** cannot assert "every grey type above every autonz
    type": the plan's own table has the autonz warden (22 hauled units)
    above the grey worker (16). It asserts the cheapest grey above every
    hive egg and the cheapest autonz, the dearest grey above the dearest
    autonz, the greys' mean above the autonz' mean, and the soldier's fight
    value (352) above the colonist's (110) and the security robot's (192).
  - Flying is the `flying` skill on the blueprint (as the hive's fliers),
    not a type field.
- Tests: `TestGreyishTypesSpawn`, `TestGreyishSheetCells`,
  `TestGreyishIdentity`, `TestGearDrop` (`faction_greyish_types_test.go`);
  `TestGreyishCampOnTheLevel` now also asserts every crew body is typed.

### Step 2: structures, modules, recipes and wrecks

- `data/blueprints/entities/greyish_structures.json` (new): `grey_vat`
  (station `grey_vat`), `grey_console` (`ResearchBuilding` `grey_console`),
  `grey_armory` (station `grey_armory`), `grey_array` (`ResearchBuilding`
  `grey_array`), `grey_emitter` (the turret), and the three wrecks with
  `Drops` (armory 1 alloy + 1 crystal, array 2 + 2, emitter 1 + 1), all
  `Kin` structures of faction `greyish`. All grey structures sit in this one
  file rather than split over `workbenches.json` and `furniture.json`.
- `data/build.json`: the five buildables under a `greyish` category;
  `grey_vat` and `grey_console` are `module_only` (new `Buildable.ModuleOnly`,
  refused by a faction director's `settlementOrders.Build`, open to the
  player's own path for phase F).
- `data/crafting_recipes.json`: the four kept print ids rewritten in place
  (names, costs, `kin_type`, station `grey_vat`, the heavy behind
  `grey_cloning`), plus `print_skimmer`, `print_lancer`, `refine_alloy` and
  the five armory recipes.
- `ships.tiers[].modules` (`ShipTierDef.Modules`, `Doctrine.ShipModules`):
  a module is a key or `{ "key", "requires_tech" }`.
- `data/factions.json` (greyish): buildables, units, recipes, `kill_drops`
  `specimen`, a `greyish` build-menu category, and `camp.station`
  `grey_vat` as a stopgap until step 4 places the hull's modules.
- **The emitter** runs its own script, `data/scripts/ai/grey_emitter.basic`
  (the sentry's watch without its "old sentry turret" message), and a new
  script function `hostile_to_player()` (`ScriptedAISystem.go`) so it holds
  its fire while the greys are neutral. `find_nearest_worker` already skips
  the caller's faction, so it never picks a grey.
- **Differences from the plan.**
  - The armory's module gate cannot be its buildable's `required_tech`
    (the ground armory is open from the start), so a module carries its own
    `requires_tech` in the tier list.
  - Wreck drops are fixed counts (`Drops` always drops everything), not the
    plan's "crystal 1-2".
  - Grey prints now cost alloy and the scout's kit has none, so a Phase D
    camp cannot print until it has an armory to refine alloy (step 5's
    rungs). The game tests that land a grey scout give its hold 2 alloy,
    and the camp test runs 8,000 ticks (a print is 450-1,200 work steps).
- Tests (`faction_greyish_content_test.go`): `TestGreyishContent` (print
  costs equal the types', every grey station is a blueprint, the
  bio-printer has no grey recipe and the grey stations no colony one, the
  tier modules), `TestModuleOnlyRefused`, `TestGreyEmitterTargets`
  (hostile: fires on a colonist; neutral: holds; never a grey or a wild
  grey), `TestGreyWrecksDropAlloy`, `TestGreyVatAndArmoryWork` (a soldier
  printed at the vat, a beam rifle made at an armory).
  `TestCraft_DeployOnCraftData` lists the two new prints.
- **Not yet:** the storage pocket (step 2b) comes next.

### Step 2b: the storage pocket

- **The link is on the component, not a wrapper round every call site.**
  `StorageComponent.Pocket` (`*StoragePocket`, the doors of one pocket) is
  runtime only and never saved. A door's own methods look through every
  door: `TakeOne`, `TakeResourceStack`, `CountResource`, `HasItem`,
  `DeductResource`, `HasItemWithComponent`, `TakeOneWithComponent`,
  `AddItem` (its own slice first, then any door with room), and new
  `HasRoom` and `PocketItems`. So every finder in
  `internal/workerai/storage.go` and every take and put in `states.go`,
  `task_handlers.go` and `diet.go` sees the pocket with no change at the
  call site (two own-slice reads moved to `PocketItems`, and the room check
  to `HasRoom`).
- **Totals count each slice once.** `storage.CountResource`, `CountAll`,
  `Check` and `Deduct` use new `OwnCount` / `OwnDeduct`; `Summarize` already
  read `Items`. The record's hold is the same number with or without the
  link (`TestPocketDoors`).
- **Who is a door is the owner, not a saved flag.** The plan's saved
  `Pocket bool` is replaced by the doctrine flag `settlement.storage_pocket`
  read through the container's `OwnedBy` (`pocketOwner`). The build handler
  that sets a new locker's owner knows no doctrine, and every container a
  grey settlement owns is a door anyway, so a flag could only drift.
  `linkPockets(level)` relinks on materialise, every 30 ticks of a pocket
  settlement's director, and when a door falls. A locker built since the
  last relink behaves as an ordinary one for at most 30 ticks.
- **A door destroyed** (`pocketDoorLost`, from `OnEntityDead`): its slice
  moves into the survivors (most free room first, overflow spilled); the
  last door spills the pocket round its tile, one entity a standable tile
  within 6, nearest first (overflow piles on the nearest), stored `GearDrop`
  gear rolling its chance. A draft alert names the spill. Before this
  phase a destroyed locker's contents were simply lost; ordinary lockers
  still are (`TestOrdinaryLockerDestroyedUnchanged`).
- `fillFactionStock` used to drop items silently when its first container
  was full; it now falls back to the ground (and a pocket's other doors
  take the overflow first).
- **The beam owner check**: `BeamResourceUpFrom` refuses a pocket door not
  owned by the player's colony or ship settlement. Colonists already never
  fetch from another settlement's containers.
- **Moved to later steps.** The rival's cockpit door re-placed a day after
  its last door falls (`DoorLostTick`) and the `"haul": "any"` rung value
  are director and materialise features: step 5. A grey player's beam-down
  placing a door is Phase F.
- Tests (`faction_greyish_pocket_test.go`): `TestPocketDoors` (each door
  holds the pocket, the nearest door is found, a take at one door comes
  from the other's slice, room is the sum, totals unchanged),
  `TestPocketOwnersNeverMeet`, `TestPocketDoorDestroyedAndSpill`,
  `TestOrdinaryLockerDestroyedUnchanged`, `TestBeamOwnerCheck`.

### Step 3: the tree and the second walk on ships

- `data/research.json`: the sixteen `tree: "greyish"` entries as the plan
  lists them (the three old ids rewritten in place), each with a `develop`
  block. `research.Tech.Develop` (`research.go`): `unlock`, `upgrade`
  (health, str, initiative factors and an `ac` addition per type), `craft`,
  `build`, `module`, `print_time`, `salvage`, `intercept`, `boarding`.
- **Live effects.** `upgrade` and `salvage` in `upgradedType` (new bodies)
  and `Retype` (living ones; it now also sets AC, and `retypeSettlement`
  fires for develop upgrades). `salvage` is a new `WorkerComponent.MineMult`
  read only by the mine handler (`mineStep`), given to types whose tasks
  include `mine`. `print_time` shortens a print's craft order (a recipe
  with a `kin_type`) when it is placed (`printTime` in `orderCraft`).
  `unlock`, `craft`, `build` and `module` are labels: the type's unlock,
  the recipe's `requires_tech`, the buildable's `required_tech` and the
  tier's module list are the gates. `boarding` is read in step 4.
- **Tier 3.** `grey_tractor_lock` adds its `catch_chance` to the evade roll
  (`AnswerEncounter`); `grey_cloak` takes 2 off the range a jump meets the
  ship in flight at (`DetectInterception`), read from the ship's techs.
- **The second walk.** New neutral keys `second_walk` and
  `second_walk_per_day`, read into the existing `Assimilation` fields when
  those are unset, so the autonz' `assimilation` keys and saves are
  untouched. `stepShip` walks it on every landed day off a loaded site, its
  points per day by hull tier (0.5 / 1 / 1.5), into `AssimPoints`; `dock`
  already merges all `KnownTechs`.
- `data/factions.json` (greyish): `second_walk`, `second_walk_per_day`, and
  the settlement's live `research` list (the two walks interleaved).
- **Pace.** `TestGreyishPaceRivalThree` is byte-identical and the
  history-dump table without `-fdetail` matches step 0's. With the second
  walk, the mothership knows (spine + second walk) 2-3 techs at day 100,
  4-6 at day 200 and 4-10 at day 300 over seeds 1-6, so 2-7 of the 13
  second-walk entries by day 300. That is slower than the plan's rough arc
  (salvage rigs at about day 40 holds; the array by about day 160 holds on
  most seeds; the disruptor by day 220 does not), because most ships sit
  docked from day 200, as the plan expected.
- **Differences from the plan.** No research-modal effect lines: no tree
  has them today (the hive's and autonz' neither), so the plan's UI lines
  are Phase F's. A `requires_tech` on a grey research entry is not enforced
  by the Tier 1 walks (as for the autonz): the walks' order is the gate.
- Tests: `faction_greyish_tree_test.go` (`TestGreyishTreeTwoWalksOnShips`:
  the spine's days the same with or without the second walk, a warship
  faster than a scout; `TestGreyishSecondWalkMergesAtDock`;
  `TestGreyishTreeHasNoTier1Effect`: only `develop` keys, sixteen entries,
  every walked, researched, unlocking and gating id a greyish entry, the
  live list on one of the two walks; `TestGreyishInterceptTechs`: caught
  21 of 200 evades plain, 43 with the tractor lock; a ship 5 off the jump
  met plain, missed cloaked) and `TestGreyishDevelopEffects` (game: AC +2
  living and new, mine speed for workers only, print refinement on prints
  only). The autonz tree tests pass unchanged.

### Step 4: the ship as a settlement

- `internal/game/faction_greyish_settle.go` (new). `materialiseGreyish`:
  the saucer stamped once (its cockpit locker the hold), the hull tier's
  modules and the ground structures (`Built`, else `ShipBuilt`) placed by
  `greySite` (vat by the centre, console in the cockpit, armory beside the
  vat, array north of the hull, emitters spread round outside it), the
  pocket linked and filled, bodies by type (`NormalizeUnits`, the autonz'
  top-up), Phase D bodies adopted once, gear handed out
  (`handOutGreyGear`, idempotent: only what the bodies lack). It is called
  with the campaign from `materialiseFactions` and `stepLiveShips` (the
  ship's tier and days down); `materialiseSettlement` falls back to it
  without one.
- `reconcileGreyish`: units by type, population, `Built` (ground
  structures standing less the tier's carried count), `Gear` (weapons and
  armour the bodies hold, from the settlement's recipes and gear floor),
  the hold, techs. `reconcileSettlementOf` dispatches it for `liveHold` and
  the live lift-off.
- **Campaign**: `FactionPresence.Gear`; `FactionRuin` gains `Built`,
  `Tier`, `Techs`, `DaysLanded` (set at `liftOff` for ship settlements) and
  `Doctrine.RuinBuilt`; `LandShip` and `liftOff` reset a ship's `Built`
  (its ground structures stayed on the last site). `Doctrine.ShipBuilt`,
  `GearFloorFor`; settlement `ground`, `wrecks`, `gear_floor`, `adopt`;
  `camp.modules`; director rung `stay_left` and `tier_min` (read live by
  the director through new `Site.Ship`, `Tier`, `StayLeft`).
- **ShipBuilt rule**: the director's ground build rungs in order, each
  wanting its `min` less the tier's carried modules of it, one structure a
  day from the landing, each only when its buildable's tech is known, the
  tier is at its `tier_min` and on that day the stay has its `stay_left`
  to run. So a scout builds its armory on day 1, a freighter with the
  array known has it from day 2 (if it carries its armory) or 3.
- **The ruin is a landing scar** (`stampGreyScar`, from `stampRuin` for a
  ship settlement): crew and hold gone, modules packed (the tier's count
  of a carried ground key, every module-only one), ground structures
  replaced by their wrecks in place (or placed from `RuinBuilt` when the
  camp was never built), the cockpit's locker gone with the hull and
  outside lockers left empty and unowned, hull walls cleared and the
  hull's floor `crater_ground`. Footprint, origin and datapad as before
  (`finishRuin`). Ruins already stamped in saves are untouched.
- **Boarding** (`boardGreyCrew`): the crew by the ship's units (else the
  stage mix for the boarding's crew count), each its type's body and stats
  with the boarders' script, armed from the ship's gear (else the floor);
  the tier's carried emitters on deck; a mothership's from its develop
  `boarding` entries (the bulwark). Emitters are not tagged with the
  presence, so the boarding is won by killing the crew.
- **The director** now has the ground build rungs (armory, emitters,
  array, gated by `stay_left` / `tier_min`) and the research rung, and the
  typed grow (the stage mix at the vat), since `greyishSiteFields` sets the
  type mix. The `arm` rung and the crafting rungs are step 5.
- **Differences from the plan.** Research progress is not carried across
  a lift-off (the settlement's research is an open task, not a presence
  field; an entry half done is started again at the next landing). The
  ground-structure placement is `greySite`, not the director's grid: the
  agreement test (step 5) compares kinds and counts only, as the plan
  allows. The vmtest `factions_greyish` asserts the vat and console in
  place of the bio-printer.
- Tests (`faction_greyish_ship_test.go`): `TestGreyScoutMaterialises`
  (modules, typed crew, the soldier's beam pistol, idempotent),
  `TestGreyWarshipMaterialisesAndReconciles` (modules, the array from
  `ShipBuilt`, a heavy, rifles; reconcile's `Built`, units, gear;
  `ShipBuilt` cases), `TestGreyPhaseDCrewAdopted`, `TestGreyLiftOffLeavesAScar`
  (no modules, wrecks in place, no hull, the ruin's `Built`; the unseen
  twin), `TestGreyBoardedWarship` (typed crew, rifles, two emitters).
  `TestRuinBuiltAfterAFrozenDeparture` now asserts the scar.
- Checked: the pace golden byte-identical, `make vmtest SCRIPT=factions_greyish`
  and `factions_intercept` pass; the camp frame shows the saucer, the vat
  and console unpacked, the soldier holding its pistol.

### Step 5: the greyish director

- **New rung features** (`internal/director`, `doctrine_settlement.go`):
  `reserve` on any rung (a share of the ship's hold target that `afford`
  leaves unspent: grow, build, craft and research all go through it);
  `per_role` on the craft rung (one per living body of each role; held,
  worn, carried and stored items count); the `arm` rung (`arm.go`: each
  worker of the roles with no weapon, or with `best` a worse one than the
  best stored, is sent for it by expected damage, and with `armour` for
  stored armour in each empty slot, through a new `Orders.EquipWorker`);
  `unless_tech` (a rung retired once a tech is known: pistols once rifles
  are); `stay_left` and `tier_min` (step 4). The hunt rung counts any
  kill-drop item other than food (the hive's `biomass` special case
  generalised: the greys hunt for `specimen`). `haul: "any"` hauls loose
  items of any kind, the static ones (gear) included.
- **The rival's door** (`greyDoorWatch`): with no storage door standing,
  the ship re-places its cockpit door (an empty locker at the cockpit spot)
  750 ticks after the last fell (`FactionPresence.DoorLostTick`); until
  then its director builds no locker (`Site.Structures` caps
  `storage_locker` at 0), because workers would otherwise rebuild one from
  the ore in their bags within a few hundred ticks and close the player's
  window on the spill. The `haul: "any"` rung brings back what is left.
- **The table** is the plan's with these changes: `haul: "any"` in place of
  `haul: "grey_alloy"` (it covers alloy, specimens and a spill); the raid
  rung's `leader` is the type key `grey_heavy`; the pistols rung has
  `unless_tech: grey_beam_weapons`; the emitters rung's guard has a
  radius; **`reserve` is 0.2, not 0.5**: half of the 30-unit hold target
  (15) leaves no room to print anything (a worker is 12 units, so it
  would need 27 in a hold that leaves at 30), and the camp test never
  printed.
- `Doctrine.RaidAlert` reads a new doctrine `raid_alert` ({name}, {place});
  the greys' draft is "{name} soldiers are moving on your colony on
  {place}." (they had the hive's "swarming" line, through their morph).
- Tests: `faction_greyish_live_test.go` (`TestGreyArmRung`: a soldier
  swaps its pistol for a stored rifle and puts on a stored harness;
  `TestGreyStayLeftGatesTheArray`: a freighter with 7 days left orders no
  array, with 25 it does; `TestGreyDoorRestoredAndSpillHauled`: no door for
  600 ticks, one back within a day, the spill hauled into it;
  `TestGreyMaterialiseAgreesWithDirector`: a freighter's armory and array,
  materialised and built live over the same days, agree with `ShipBuilt`)
  and `internal/director/greyish_test.go` (`TestReserveStopsSpending`,
  `TestDiceAverage`, `TestShipRungGates`).
- **Not tested live**: lancers posted at emitters (a lancer needs the
  lancer pattern, which needs specimens and the array; the soak's warship
  start shows whether it happens), a raid led by a heavy (the raid rung's
  `leader` is the autonz' tested code), and colonists smashing the lockers
  in play (the spill and the door are tested by calling the door's death).

### Step 6: soak, digging, vmtest, docs

- **The greyish soak** (`internal/game/greyish_soak_test.go`, `make
  soak-greyish`, in `make soak`; `TestGreyishSoakJoin`; `GREYTICKS` 22,500).
  Ten scout seeds and five warship seeds. **Differences from the plan's
  setup:** the ship lands live on the nearest ore-tagged world the player
  flies to (the start system has no ore in reach on most seeds), arriving
  once the player is down so it lands fresh; the warship starts with 6
  metal, 4 crystal, 4 alloy and 4 specimens (the plan's 68 units are over
  the 30-unit hold target, so it went home before it landed); both start
  with nothing on the ground (`Built` empty), so the director's own armory
  is measured. The real rules run: a ship that fills its hold or ends its
  stay lifts off and the row ends there.
- **What the soak found.** On today's maps the greys' ore lies 2-5 levels
  down and over 80 tiles from the camp on most "ore" sites (6 of 8
  checked). With the plan's director (dig 4 levels, 60 across, after the
  builds) a scout spent its kit on the armory and then had no metal: no
  research (every entry costs metal), no alloy, no prints. Fixes on the
  way, all data or generic director code:
  - `haul: "any"` reaches only the director's radius (the camp; a spill
    lands within 6 of its door): at the full reach it sent the crew after
    distant hunt drops.
  - A locker build waiting on metal held the dig rung up (the ladder's
    `unmet` rule), a deadlock: the dig rung now sits above every build.
  - The warship's three lockers (`per_colonist` 0.25) took every open-order
    slot waiting on metal: the storage rung is `min` 1, and a new
    `near: "mines"` build rung puts a pocket door by the far mine (haulers
    drop ore at the nearest door; the camp has it at once).
  - **The shared dig rung gave up an ore body whenever the next tile toward
    it was open** (a tile already dug, or a cave) and wandered to farther
    ore or stopped (`stepToward` takes only solid tiles). `tunnelStep`
    walks on through open ground to the next solid tile. This changes the
    hive's and autonz' digging too; their soaks were re-run (below).
- **The owner's decision (2026-11-11): the greys dig faster to reach ore**
  ("they have advanced tech"). Built as data on the type and the doctrine:
  the worker's `dig` 4 (dig work at four times the rate,
  `WorkerComponent.DigMult`) and `bore` 4 (a dig it finishes carries on up
  to three more tiles straight through rock, never through ore, a
  structure or open ground: `boreOn`), dig depth 6 and reach 150, and the
  director plans every 15 ticks. Several tunnel orders queued at once was
  tried and dropped: workers took orders they could not reach yet and the
  orders stuck.
- **Thresholds as built**: a seed misses when its camp got nowhere (no
  armory by tick 3,000, no research entry and no metal ore reached); more
  than two misses per start fails; a raid in the neutral soak or a crew
  over its cap fails at once. These are below the plan's targets, which
  today's maps do not allow (next item).
- **The soak as built** (`make soak-greyish`, 22,500 ticks, a row ending
  at lift-off; every row ran its full stay):

  | Start | Seeds | Metal ore reached (tick) | Armory by 3,000 | A research entry | Alloy refines | Hold at the end (target 30) | Got nowhere |
  |---|---|---|---|---|---|---|---|
  | scout | 10 | 5 (3,090-9,630) | 5 (built, 630-780) | 4 (salvage rigs, 3,270-7,680) | 0 | 2-20 | 1 (seed 5) |
  | warship | 5 | 2 (9,480, 9,570) | 5 (carried) | 0 | 0-3 | 12-30 | 0 |

  No prints (every crew stayed at its cap but one scout, which lost its
  worker), no weapons made (the gear floor arms the soldiers), no array or
  ground emitters (the warship's two are carried), no raids. Each warship
  hunted 4-11 specimens.
- **`make soak`** (observer, expedition, autonz, hive, greyish) passes in
  10 min 54 s after the merge of dev's faster soaks (autonz misses node 2 /
  hub 1, as E3 recorded); **`make vmtest-all`** passes (29 scripts, 2 min
  25 s) and again with the new script and the Factions tab rows (below).
- **vmtest** `factions_greyish_warship` (new) and the `ship` debug command
  (`vmhook.go`: a ship's tier, crew, hold, a tech, or its stay ended).
  Frames looked at: the warship's camp (the array north of the hull, the
  console, the vat, the armory bench, soldiers holding rifles, the heavy in
  its tentacle pose, technicians and workers inside the hull), a day on,
  and the landing scar (no hull, a scorched round floor, the datapad). The
  array's wreck is found by `debug focus` but not drawn in the frame (out
  of every colonist's view once the greys left).
- **history-dump** (`-fdetail`): a grey ship's units by type, `ShipBuilt`
  for a landed ship, its gear floor and its second walk's techs and points.
  **The Factions tab**: a grey ship's crew by type and the modules its hull
  carries, on a row under the ship.
- **Not done from the plan's step 6**: the `factions_intercept` frame of a
  bulwarked mothership deck (boarding with the bulwark is tested in Go,
  `TestGreyBoardedWarship` covers the warship's emitters); the
  enemy_factions_plan.md phasing entry and the Phase F plan's pointer here
  (Phase F's plan lives on its own branch, not merged).

### The owner's decisions on the soak (2026-11-12)

1. **Motherships beam materials down** ("like what colonists do: stock on
   the mothership, beamed down as needed to the landing party's storage").
   Built as `ships.mothership.beam`: the mothership starts with a beam stock
   of its own (150 metal, 50 crystal; `FactionPresence.BeamStock`), apart
   from its hoard, which still pays for hulls and new ships. Once a day a
   grey camp on a loaded level is topped up to the beam floor (15 metal,
   6 crystal) from that stock, then the hoard (`beamDownToCamp`,
   `Campaign.BeamFromMothership`). Tier 1 never beams (no level is loaded),
   so the pace table did not move; with no mothership (destroyed) nothing
   comes down.
2. **Every hull carried the armory** for a while (open question 19's
   alternative), then the owner brought the armory module back the same day
   (below): with the beam-down paying for it, a scout's ground armory costs
   a day, not its stay.
3. **One lancer per emitter**: a typed grow rung with `per: "grey_emitter"`
   (`DirectorGrow.Per`) keeps a lancer for each standing emitter once the
   lancer pattern is known. **Over the crew cap**, not within it as first
   offered: the crew is always at its cap long before emitters and the
   pattern exist, so "within the cap" meant no lancer ever. At most one an
   emitter (two on a warship); Tier 1 still never counts them.

The soak after these (`make soak-greyish`; no seed got nowhere):

| Start | Seeds | Research entries a stay | Alloy refines | Weapons and armour made | Fighters armed | Hold at the end (target 30) | Left early on a full hold |
|---|---|---|---|---|---|---|---|
| scout | 10 | 2-3 (salvage rigs by tick 3,200-5,900, hull plating, beam weapons on 6) | 4-11 | 3 beam rifles | 10 of 10 | 16-32 | 1 (tick 11,250) |
| warship | 5 | 0-3 (salvage rigs, field armour, skimmer pattern) | 1-15 | 5 field harnesses | 2 of 3 each (the heavy fights with tentacles until the disruptor) | 26-31 | 2 (ticks 1,500 and 2,250) |

4. **Beamed stock does not send a ship home** (2026-11-12). A ship's hold
   target is measured on `Fleet.HoldForHome`: its hold less the units its
   mothership beamed down that the camp has not spent
   (`FactionPresence.Beamed`, booked by `Fleet.NoteBeamed` at each daily
   beam: what the camp spent since the last beam comes off the beamed units
   first, so mined stock always counts). A new landing starts with nothing
   beamed. Tier 1 never beams, so the pace table did not move. When the
   ship leaves, the beamed stock still aboard goes home with its hold.

The soak after it: no seed got nowhere and **every ship stayed its full 30
days**; scouts learned 2-3 research entries (beam weapons on 8 of 10),
every warship three (salvage rigs, field armour, skimmer pattern); alloy
refines 6-11 (scouts) and 7-15 (warships); holds ended at 10-28 (scouts)
and 26-37 (warships, the beamed part above the target).

### Plastic, and the armory module back (2026-11-12)

The owner's call: "add plastic as a material they can create using biomass
to offset the metal usage. It would take an extra step to create so they
aren't as efficient with it. Maybe gear is more metal based, but research
involves plastic"; named plain `plastic` (open to the colony later);
research and some structures, "it should still cost some metal, but not as
much"; and the armory module back as research, the ship no longer carrying
the armory by default.

- **`plastic`** (`resources.json`, a stackable raw material; a placeholder
  sprite, the item sheet's (48,48) tinted; name and text drafts). Pressed at
  the armory from biomass (`refine_plastic`: 2 biomass, 120 work, 2 plastic;
  first tuned at 3 biomass to 1, which made one plastic three round trips
  to a plant, so research never started). No colony recipe takes it yet.
- **Costs**: every grey research entry, the research array and the
  emitter take a third of their metal (rounded up) as metal and the rest as
  plastic (salvage rigs: 2 metal, 4 plastic, 2 crystal; the array: 6 metal,
  10 plastic). Gear, prints, alloy, the armory and lockers stay on metal.
- **The director**: the mine rung also gathers biomass (4 in store: the
  camp gathers about as fast as it presses), and a plastic rung keeps 6
  pressed. The mothership's beam carries plastic too (stock 60, floor 4;
  confirmed by the owner, 2026-11-12): on a world with no plants (an
  asteroid field) a camp could not make any, and research would never
  start.
- **The armory module** is back as the plan had it: freighters and
  warships carry the armory once it is known (700 work, now 4 metal, 6
  plastic, 4 crystal, 2 alloy), scouts build their own on the ground (the
  beam pays for it; a scout's armory stood by tick 720-4,530 in the soak).
- **Bugs this turned up**:
  - A pocket door on another level (the door by the deep mine) could be
    "nearest" to a worker on the surface (the finders measured across the
    map only): a pocket door on another level now counts 1,000 tiles
    farther (`storageDist`; ordinary containers measured as before).
  - The door-by-the-mine rung took surface plants chopped for biomass for
    far mines and built lockers out by them, using the open-order slots the
    array needed: it now looks only at ore faces.

The soak after it (no seed got nowhere; every ship stayed its full stay):

| Start | Seeds | Research entries a stay | Armory | Alloy refines | Weapons and armour made | Hold at the end (target 30) |
|---|---|---|---|---|---|---|
| scout | 10 | 1-2 (salvage rigs; hull plating on 8) | built, tick 720-4,530 | 6-9 | none (the soldier keeps its floor pistol) | 21-38 |
| warship | 5 | 1-3 (salvage rigs, field armour, skimmer pattern) | carried | 9-12 | field harnesses on 4 | 29-43 |

Research is slower than with metal alone (scouts reached beam weapons on 8
of 10 before plastic): the extra step costs time, as intended. Holds end
over the target on several seeds because alloy, plastic and biomass kept in
stock count toward it (only beamed stock is left out).

**Final checks (2026-11-12, at the plastic commit):** `make soak` passes in
12 min 2 s (observer, expedition, autonz with misses node 2 / hub 1 as E3
recorded, hive, greyish with no seed missing); `make vmtest-all` passes
(30 scripts); `go test ./...`, `go vet` (both build tags) and
`TestGreyishPaceRivalThree` pass, and the history-dump table is
byte-identical to step 0's.

### What the owner may want to decide

1. **Metal from mining** is still scarce on today's maps (ore 2-5 levels
   down and far off; one camp in fifteen reached it); the beam carries the
   camps. A mothership's beam stock (150 metal, 50 crystal) runs down over
   several landings and then the hoard pays, which slows its hull and ship
   purchases in a campaign the player watches (not in the Tier 1 table).
2. **The reserve is 0.2 of the hold target, not 0.5**, and **workers
   craft** (both as built).
4. **Plastic's pace**: scouts learn 1-2 entries a stay with plastic (2-3
   before). Levers: the pressing ratio, more plastic on the beam floor, or
   fewer entries moved onto plastic.

3. **Lancers are over the crew cap**, one an emitter (the crew fills its
   cap first).
