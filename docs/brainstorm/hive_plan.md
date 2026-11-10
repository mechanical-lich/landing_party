# The Hive as a Settlement Faction: Implementation Plan

Status (2026-11-08): **built** on `claude/factions-a-a379f7`, steps 1 to 7
(see "As built" at the end: each step's deviations and the hive soak's
table). Plan agreed earlier the same day (every open question decided; see
"Open questions"). It is a new phase on the factions
branch (`claude/factions-a-a379f7`), **Phase E2**, after E and observer
mode v2 and before F (playable races). It was written against the branch
after `fcf456b`. All draft text here (unit names, egg and structure names,
research names, alerts, Legends lines) is placeholder that the owner
rewrites (see "Where the owner rewrites text").

Read with: [enemy_factions_plan.md](enemy_factions_plan.md) (design,
phasing, "The hive: morphs, tiers, hidden identity", every "as built"
section) and [factions.md](../developer/factions.md) ("How a faction moves
and acts", "`data/factions.json`", "The player's doctrine", "The
director", "Faction settlements", "ConvertAction", "Raids and hunts", "The
autonz", "Tier 2: the hive on a level", "The hive's creatures").

## Decision

The owner's direction (2026-11-08): the hive becomes a settlement faction
in the style of a classic RTS swarm race. It gets a nest that makes
larvae, eggs as its build menu, specialised units, creep, and an evolution
tech tree. It runs on the same doctrine, settlement and director machinery
as the mirror, the greyish and the autonz, so that F can hand it to the
player with no new code paths.

Taken as given:

- **Colour means specialisation, not tier.** There are six unit types, and
  the egg colour tells you what will hatch:

  | Morph | Colour | Type | Job |
  |---|---|---|---|
  | crawler | olive | **drone** | forage, haul, lay resin, build, tend eggs, evolve |
  | crawler | red | **warrior** | melee guard, raids |
  | crawler | purple | **burrower** | dig, tunnel, break walls |
  | flier | blue | **scout** | fast, cheap, flying, finds the enemy |
  | flier | orange | **hunter** | flying attacker, hunts, raids over moats |
  | flier | green | **spitter** | ranged flying attacker |

  A grey egg is an **unassigned larva**. Upgrades from evolution are stat
  multipliers and do not change colour.
- **Biomass is the only resource.** Units eat it and eggs cost it. Drones
  gather it from flora, corpses and loose biomass. The nest makes larvae on
  a timer, and eggs are assigned from those larvae.
- **Resin is the hive's creep.** Drones lay it outward from the nest
  through `ConvertAction`, using a hive floor tile. It speeds hive units,
  slows everyone else, and new hive structures must stand on it.
- **An evolve tree** in `research.json` unlocks unit types (drone and scout
  are known from the start), stat upgrades per type, launcher range (which
  already exists), resin spread rate and flier speed. The hive researches
  it at an evolution chamber, not a lab.
- **A hive director table** runs drones, resin, nest growth, warriors up to
  a guard ratio, evolution, the spire at the launcher stage, launching,
  raids when hostile and strong, and hunting. It uses the same order
  functions as every other director, and nothing is teleported.
- **Hidden identity** works as the plan already has it. Players see morph
  names until they discover the hive, and **unit type names are revealed
  together with the morph link**.
- **Everything is data** in `data/factions.json` and `research.json`. The
  player side is not built here; "Phase F: what the player will see"
  describes it so that the data is shaped for it now.

Constraints carried over: Phase A's spread pace is kept (the
`TestFactionPaceRivalOne` test and the history-dump balance table must not
change); physical storage (colonists, and now drones, haul everything);
scenarios stay random; the owner commits.

## What changes from "colour = tier"

The faction plan's hive section (2026-11-08) made the sheet's three colours
per morph the hive's **research tier**. Phase B built that:
`factory.ApplyTier` adds `sprite_dx` (144 per colour block) and multiplies
health and strength, `Doctrine.TierOf(known_techs)` picks the highest tier
whose tech is known, and the `tier_advanced` / `tier_elite` research
entries move a presence from olive/blue to red/orange to purple/green.
After this phase:

- **Tiers are gone as a concept.** The `tiers` list becomes `types`, keyed
  by type, not by index. The colour block becomes the unit's **type**. The
  same mechanism stays: the sprite offset is added at spawn and the stats
  are set at spawn. Only its meaning changes. `ApplyTier` becomes
  `ApplyType`, `Kin.Tier` (an index) becomes `Kin.Type` (a string), and
  `TierOf` becomes `TypesUnlocked(known_techs)`.
- **The egg offsets already line up.** Crawler eggs are at row y=744
  starting at x=144 (olive). +144 gives red and +288 gives **pink** (the
  sheet has a full pink egg with cracked and hatched frames at x=432, the
  closest it has to purple). Flier eggs are at row y=768 starting at x=0
  (blue), so +144 is orange and +288 is green. The grey egg at (0, 744) is
  the larva. So the type's `sprite_dx` moves the body and the egg the same
  amount, just as tiers did. The spare beige egg at (432, 768) is unused.
- **Upgrades get no colour.** Their visual is the **size** of the body. A
  settlement unit works in the stage-2 frame (x+48 in its colour block). It
  is drawn in the stage-3 frame (x+96) once its type's second upgrade is
  known: a "fully evolved" form, so an attentive player can tell an evolved
  warrior from a fresh one. Wild creatures (scenario and GameMaster) keep
  ageing from stage 2 into stage 3 as they do today. This is open question
  4; the alternative is no visual at all.
- **Research changes.** `tier_advanced` and `tier_elite` leave the walk.
  Unlock entries take their places, at the same cost and position (see
  "The evolve tree"), so the launcher-range entries land on the same day
  and the pace does not move.
- **Saves.** Living creatures with `Kin.Tier` 1 or 2 map to the type
  that matches their colour (red crawler to warrior, orange flier to
  hunter, purple crawler to burrower, green flier to spitter), so their
  sprite stays right. Presences whose `known_techs` contain `tier_advanced`
  / `tier_elite` get the unlocks at those positions through an alias table
  in the doctrine (`tech_aliases`). These are additive fields, so
  `CampaignVersion` stays 4.
- **The "a tougher one" hidden-identity fact** (`hive:tiers`) becomes
  `hive:castes`: the player learns that the colours are different jobs, not
  different strengths. B+ is not built, so here this is only a renamed key
  in the plan and in data.

Scenario and GameMaster creatures had no tier and now have no type. They
stay at the base colour block (olive crawlers, blue fliers), so a
0-rival-factions game looks exactly as it does today.

## Where the code is today

- **Hive bodies are creatures.** `data/blueprints/entities/hive.json` has
  one blueprint per morph and life stage, all with `ScriptedAI`. Their
  scripts (`data/scripts/hive/*.basic`) hunt any `Worker` within 12–20
  tiles and lay eggs every 50 turns. They have no `Worker` and no
  settlement. `find_nearest_worker` (`internal/systems/ScriptedAISystem.go`)
  matches *every* worker, so once hive units are workers a wild hive
  creature would attack them. Step 4 fixes this.
- **The hive has no settlement block.** Its doctrine has `stages` with
  morph `mix`es, `tiers`, a Tier 1 `research` walk (with launcher `range`)
  and a `level` block (`egg_share`, `adult_share`, `launcher`, `husk`).
- **Materialise and reconcile** (`internal/game/faction_level.go`) stamp
  `hive_nest` / `hive_core` (terrain only, `eldritch_wall` and
  `eldritch_floor`), place the `egg_launcher` at the launcher stage, and
  top up or cull bodies by `presenceComposition`. Reconcile counts bodies
  into `population`. The live rule (`stepLiveFactions`) launches once a
  star-date day and removes the eggs nearest the spire.
- **The settlement machinery already has everything the hive needs, with
  small extensions**:
  - a staged camp (`camp.staged`, the autonz) with a `station`;
  - the grow rung (prints the type most under its crew share, up to the
    crew cap);
  - the convert rung and `generation.ApplyConversion` / `ConversionShare`
    for a share of a patch;
  - the mine rung, which chops choppables that drop the wanted item (flora
    drops `biomass`);
  - the haul rung;
  - the research rung (`required_building`, Int check);
  - the hunt rung (food only today) and the raid rung (`roles`, `unarmed`);
  - `deploy_on_craft`;
  - `WorkerComponent.AllowedTasks` and `Role`.
- **Biomass** is an item (`data/blueprints/items/resources.json`, tags
  `biomass`, `organic`, `raw_material`, stack 500), dropped by flora
  (`alien_flora`, `frosted_pointed_tree`, …), and is a location tag the
  capacity reads. **There are no biomass tiles.** The owner's "existing
  biomass tiles" means flora, loose biomass and the site tag. Nothing
  regrows (open question 7).
- **Eating** looks for items with a `Food` component (`HandleFindFood`,
  `FoodAvailable`). Biomass has none.
- **Speed** is the `Initiative` counter (`DefaultValue`: lower is faster;
  hatchling 6, stage 2 10, stage 3 14). No tile changes speed today.
- **Recipes** have `station`, `cost`, `output`, `build_time`,
  `deploy_on_craft`, but no `required_tech`. Buildables do have
  `required_tech`.
- `research.json` is one flat map (`name`, `description`, `duration`,
  `cost`, `required_building`, `required_int`, `requires_tech`). The
  doctrine's `settlement.research` names entries in it.
- A day is 750 ticks (`history.TicksPerDay`). A 30,000-tick soak is 40
  days.

## Design

### Two kinds of hive body

- **Wild**: scenario and GameMaster creatures, and the creatures of a
  presence's husk. These stay exactly as they are: scripted, ageing egg →
  hatchling → stage 2 → stage 3, olive or blue, the same faction string
  `hive`. **They are named "Wild"** (owner, 2026-11-08): "Wild Xeno",
  "Wild Xeno Hatchling", "Wild Xeno Egg", "Wild Troid", and so on. That
  way a player can tell a scenario's wild brood from a presence's typed
  bodies ("Xeno", "Red Xeno") in the inspector, the log and the bestiary.
  The name lives on the wild blueprints' `Description.Name`. The blueprint
  ids (`xeno_*`, `troid_*`) stay the same, so history evidence, quests,
  objectives and saves are untouched. A settled egg's hatchling and adult
  get the type's `seen_name` from `ApplyType`, which overrides the wild
  name. The word is a morph-level name, so it never names the faction
  and hidden identity still holds.
- **Settled**: the bodies of a presence. An egg is laid from a larva at the
  hive's order. It hatches through the existing egg script. The hatchling
  is a scripted juvenile that stays near the heart. When the hatchling
  script calls `spawn_kin("stage2")`, `factory.SpawnKin` sees the parent's
  presence is a settlement and spawns the morph's **crew** blueprint
  (`xeno_crew`, `troid_crew`): a `Worker` in `hive:<locID>`, with no
  script, typed by `ApplyType`. That body is the working adult. So the
  life stages stay in the existing scripts, the eggs and hatchlings stay
  shared, and only the adult differs.

The crew blueprints are one per morph (two, not six). The type adds the
sprite offset, the stats, `Role`, `AllowedTasks`, skills and starting
equipment at spawn, as tiers did.

### The unit table

Sprite positions are pixels on `scifi_entities`. Body rows are y=696 for
crawlers and y=720 for fliers; each colour block is 144 wide, holding the
hatchling at +0, the working adult (stage 2) at +48 and the evolved adult
(stage 3) at +96. Egg rows are y=744 and y=768; each egg has intact at +0,
cracked at +48 and hatched at +96.

| Type | Morph | Body block x | Egg (x, y) | Cost (biomass) | Lay time | Hatch turns | HP | Str | Dex | AC | Attack | Initiative | Skills / kit | Role, allowed tasks | Unlocked by |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| larva | — | — | grey (0, 744) | free (the heart's timer) | — | — | 4 | — | — | 4 | — | 40 | inanimate station | — | — |
| drone | crawler | 0 (olive) | olive (144, 744) | 2 | 60 | 20 | 14 | 8 | 12 | 6 | 1d4 | 10 | — | `worker`: chop, mine (biomass), haul, build, convert, craft, research | start |
| scout | flier | 0 (blue) | blue (0, 768) | 2 | 60 | 16 | 10 | 6 | 18 | 8 | 1d4 | 6 | `flying` | `scout`: none (patrol orders only) | start |
| warrior | crawler | 144 (red) | red (288, 744) | 4 | 90 | 24 | 32 | 16 | 12 | 12 | 2d6 | 12 | — | `guard`: attack | `hive_warrior_brood` |
| hunter | flier | 144 (orange) | orange (144, 768) | 5 | 90 | 24 | 24 | 14 | 16 | 10 | 1d8 | 8 | `flying` | `hunter`: attack | `hive_hunter_brood` |
| burrower | crawler | 288 (purple) | pink (432, 744) | 5 | 120 | 30 | 24 | 14 | 10 | 14 | 1d8 | 14 | — | `digger`: mine, dig, haul, attack (walls and doors too) | `hive_burrower_brood` |
| spitter | flier | 288 (green) | green (288, 768) | 6 | 120 | 30 | 18 | 10 | 14 | 8 | `spit_gland` 1d10, range 6 | 10 | `flying`, `spit_gland` | `spitter`: attack | `hive_spitter_brood` |

Benchmarks: today's wild adult (stage 3) is HP 32, Str 16, AC 12, 2d6,
Initiative 14. A **warrior is today's adult, a little quicker**, so a raid
by a hive feels as dangerous as a GameMaster wave does now. A drone is
weaker than a wild stage 2. A hive therefore stops being "every body
hunts you", and becomes "workers you can pick off, and a guard you have to
get past". Upgrades multiply these numbers (see "The evolve tree").

The spitter's `spit_gland` is a ranged natural weapon: an item in the
spitter's inventory, flagged so that it is not dropped on death or looted
(open question 9). It is the same path as the autonz enforcer's laser
pistol, so ranged combat needs no new code.

### Types in `data/factions.json`

`tiers` is replaced by `types`. The stage `mix` is now by type.
`morph_fallback` names where a locked type's share goes, so the
crawler/flier ratio per stage stays as Phase B had it (nest 0.8/0.2, hive
0.6/0.4). Phase B's `factions_site` frames will still change colour (see
step 1).

```json
"types": {
  "drone":    { "morph": "crawler", "sprite_dx": 0,   "name": "Drone",    "seen_name": "Xeno",
                "cost": 2, "hatch_turns": 20, "health": 14, "str": 8,  "dex": 12, "ac": 6,  "attack": "1d4", "initiative": 10,
                "role": "worker", "tasks": ["chop", "mine", "haul", "build", "convert", "craft", "research"] },
  "scout":    { "morph": "flier",   "sprite_dx": 0,   "name": "Scout",    "seen_name": "Troid",
                "cost": 2, "hatch_turns": 16, "health": 10, "str": 6,  "dex": 18, "ac": 8,  "attack": "1d4", "initiative": 6,
                "role": "scout", "tasks": [] },
  "warrior":  { "morph": "crawler", "sprite_dx": 144, "name": "Warrior",  "seen_name": "Red Xeno",
                "unlock": "hive_warrior_brood", "cost": 4, "hatch_turns": 24,
                "health": 32, "str": 16, "dex": 12, "ac": 12, "attack": "2d6", "initiative": 12,
                "role": "guard", "tasks": ["attack"] },
  "hunter":   { "morph": "flier",   "sprite_dx": 144, "name": "Hunter",   "seen_name": "Orange Troid",
                "unlock": "hive_hunter_brood", "cost": 5, "hatch_turns": 24,
                "health": 24, "str": 14, "dex": 16, "ac": 10, "attack": "1d8", "initiative": 8,
                "role": "hunter", "tasks": ["attack"] },
  "burrower": { "morph": "crawler", "sprite_dx": 288, "name": "Burrower", "seen_name": "Purple Xeno",
                "unlock": "hive_burrower_brood", "cost": 5, "hatch_turns": 30,
                "health": 24, "str": 14, "dex": 10, "ac": 14, "attack": "1d8", "initiative": 14,
                "role": "digger", "tasks": ["mine", "dig", "haul", "attack"] },
  "spitter":  { "morph": "flier",   "sprite_dx": 288, "name": "Spitter",  "seen_name": "Green Troid",
                "unlock": "hive_spitter_brood", "cost": 6, "hatch_turns": 30,
                "health": 18, "str": 10, "dex": 14, "ac": 8,  "attack": "1d4", "initiative": 10,
                "equip": ["spit_gland"], "role": "spitter", "tasks": ["attack"] }
},
"morph_fallback": { "crawler": "drone", "flier": "scout" },
"tech_aliases": { "tier_advanced": "hive_warrior_brood", "tier_elite": "hive_hunter_brood" },
"stages": [
  { "pop": 0, "name": "brood", "mix": { "drone": 1.0 } },
  { "pop": 6, "share": 0.3, "name": "nest", "stamp": "hive_nest", "w": 13, "h": 13,
    "mix": { "drone": 0.5, "warrior": 0.2, "burrower": 0.1, "scout": 0.1, "hunter": 0.1 },
    "structures": { "egg_cluster": 1, "evolution_chamber": 1 } },
  { "pop": 12, "share": 0.6, "name": "hive", "launcher": true, "stamp": "hive_core", "w": 21, "h": 21,
    "mix": { "drone": 0.35, "warrior": 0.15, "burrower": 0.1, "scout": 0.1, "hunter": 0.15, "spitter": 0.15 },
    "structures": { "egg_cluster": 2, "evolution_chamber": 1, "egg_launcher": 1 } }
]
```

`seen_name` is what the body's `Description.Name` says until B+ knows the
morph link. `name` is what it says after. Hidden identity is in data: B+
swaps the name, so the simulation never reads either field.

### Larvae and eggs (the build menu)

- **The heart.** `hive_heart` (Kin structure, inanimate, HP 80) is the
  camp's `station`, placed at the centre like the autonz fabricator. It
  and each `egg_cluster` produce a **larva** (`hive_larva`, the grey egg)
  every `every_ticks` on an open resin tile beside it. This stops once the
  settlement's larvae reach `cap + per_cluster × clusters`. It is a small
  Go step in the faction director tick (`stepHiveLarvae`), read from
  `settlement.larvae`, deterministic per tick and seed. It is not a script,
  because it has to count larvae across stations.
- **A larva is the station** of every egg recipe. An egg recipe has
  `station: "hive_larva"` and `consume_station: true`. A drone is given the
  craft by the director (or, in F, by the player), carries the recipe's
  biomass from a pool to the larva (the craft's materials are fetched like
  any other craft), and works it for `build_time`. `deploy_on_craft` then
  **replaces the larva** with the output egg on the larva's tile instead of
  putting it beside the station. Nothing is created from nothing: the
  biomass walked there, and the larva was there.
- **The output** is the morph's egg (`xeno_egg` or `troid_egg`) with
  `kin_type` set. `ApplyType` gives the egg its colour (the type's
  `sprite_dx`), its presence and its `hatch_turns`. The egg script reads
  `hatch_turns` from a var seeded at spawn (falling back to 20, so wild
  eggs are unchanged), then `spawn_kin` passes the type and presence on to
  the hatchling and the adult.
- **Recipe fields** that are new: `kin_type`, `consume_station`,
  `required_tech` (refused until the settlement knows it; a recipe's
  version of the buildable field).

```json
"hive_egg_drone":    { "name": "Drone Egg",    "description": "_draft", "output": "xeno_egg",  "kin_type": "drone",
                       "cost": { "biomass": 2 }, "build_time": 60,  "station": "hive_larva", "consume_station": true, "deploy_on_craft": true },
"hive_egg_scout":    { "name": "Scout Egg",    "description": "_draft", "output": "troid_egg", "kin_type": "scout",
                       "cost": { "biomass": 2 }, "build_time": 60,  "station": "hive_larva", "consume_station": true, "deploy_on_craft": true },
"hive_egg_warrior":  { "name": "Warrior Egg",  "description": "_draft", "output": "xeno_egg",  "kin_type": "warrior",
                       "cost": { "biomass": 4 }, "build_time": 90,  "station": "hive_larva", "consume_station": true, "deploy_on_craft": true,
                       "required_tech": "hive_warrior_brood" },
"hive_egg_hunter":   { "name": "Hunter Egg",   "description": "_draft", "output": "troid_egg", "kin_type": "hunter",
                       "cost": { "biomass": 5 }, "build_time": 90,  "station": "hive_larva", "consume_station": true, "deploy_on_craft": true,
                       "required_tech": "hive_hunter_brood" },
"hive_egg_burrower": { "name": "Burrower Egg", "description": "_draft", "output": "xeno_egg",  "kin_type": "burrower",
                       "cost": { "biomass": 5 }, "build_time": 120, "station": "hive_larva", "consume_station": true, "deploy_on_craft": true,
                       "required_tech": "hive_burrower_brood" },
"hive_egg_spitter":  { "name": "Spitter Egg",  "description": "_draft", "output": "troid_egg", "kin_type": "spitter",
                       "cost": { "biomass": 6 }, "build_time": 120, "station": "hive_larva", "consume_station": true, "deploy_on_craft": true,
                       "required_tech": "hive_spitter_brood" }
```

The cost lives on the recipe, and the type's `cost` mirrors it for Tier 1.
A content test checks that the two agree.

The hatchling of a settled egg runs `data/scripts/hive/nursery.basic`
instead of `hatchling.basic`. It wanders within 4 tiles of where it
hatched and fights only if attacked. A brood of juveniles charging the
player's landing would undercut the "workers and guards" shape (open
question 10). The blueprint keeps `hatchling.basic`, and `SpawnKin` sets
the script when the parent has a settlement presence.

### Resin (the creep)

- **Tile.** `hive_resin` is a new floor in `data/tiledefinitions/dungeons.json`
  that uses the `eldritch_floor` sprites, which is what the stamps already
  look like. `hive_nest` and `hive_core` lay `hive_resin` instead of
  `eldritch_floor`, so a stamp counts as resin.
- **Laying.** The convert rung (`convert: "hive_resin"`) gives
  `ConvertAction`s outward from the heart, nearest first, as the autonz do.
  A drone stands on or beside the tile and lays it in 8 ticks. A tile with
  no floor (open rock on an asteroid) finishes at once without resin.
  That is where burrowers come in (the excavate rung, below).
- **Speed.** Tile definitions gain an optional `speed` map: a multiplier on
  the `Initiative` refill of a walking entity standing on the tile, by
  faction, with `*` for everyone else. For `hive_resin` it is
  `{ "hive": 0.75, "*": 1.4 }`: hive walkers act a third more often, and
  the player's colonists and robots take 40% longer per step. Flying units
  ignore it both ways. One hook in the game's initiative step
  (`internal/game/resin_speed.go`) reads the tile under the entity after
  `InitiativeSystem` resets the counter. The rogue library is not changed.
- **Structures need resin.** Buildables gain `required_floor`. A site
  whose footprint is not entirely that floor is refused, both in the
  director's site pick (`internal/director`) and in the player's placement
  check (`internal/construction`), so F needs nothing more.
- **Share.** The doctrine's `conversion` block works as it does for the
  autonz (`tile`, `radius`, `per_day`, `cap`, `tags`), with `floor: 1.0`,
  which means resin **does not cap capacity**. That keeps Tier 1 growth
  exactly as it is. The resin share is the presence's `conversion`.

### Structures

Built by drones from biomass with the player's build order path, on resin,
within the director's `radius`:

| Key | What | Blueprint / script | Cost | Rule |
|---|---|---|---|---|
| `hive_nest`, `hive_core` | the stage chambers | structure scripts (exist), floor now `hive_resin` | — | the camp: `staged`, re-stamped when the stage rises (hold lifted out first, as the autonz do) |
| `hive_heart` | larva source, the camp's station | blueprint in `hive.json` (Kin structure; sprite: the green slime blob at (576, 744), open question 11) | — | placed by the camp at the centre |
| `biomass_pool` | the hive's storage (`AllowedTags: ["biomass"]`) | blueprint in `furniture.json` (sprite: the violet blob at (720, 744)) | 2 biomass | the camp places one (new `camp.storage`). Director: one per 10 bodies |
| `egg_cluster` | more larvae (+2 cap) and a second larva site | build.json entity, `required_floor: hive_resin` | 8 biomass | from the nest stage: one per 12 bodies, at most the stage's `structures` count |
| `evolution_chamber` | where the hive evolves (`required_building` of every evolve entry) | build.json entity, `required_floor: hive_resin` | 12 biomass | from the nest stage, one |
| `egg_launcher` | the Resin Spire (exists) | becomes a build.json entity, `required_floor: hive_resin` | 20 biomass | at a launcher stage, one. Materialise still places it on a frozen presence that reached the stage while away, as now |

The build rung gains `stage_min`, the lowest stage it runs at. That is what
lets the director's spire rung wait for the launcher stage.

### The evolve tree

`research.json` is a flat map today, and doctrines already name their
entries through `settlement.research`. The owner's "hive key" is taken as:
**the hive's entries are `hive_*` keys carrying `"tree": "hive"` and an
`evolve` block**. The shared schema keeps working, and the colony's
`TestColonyDoctrineCoversContent` accepts entries another doctrine owns
(it already does this for greyish recipes). Open question 1 offers a
nested object instead.

`evolve` effects: `unlock` (a type), `type` with `health` / `str` /
`initiative` multipliers (Initiative below 1 is faster), `work` (a work
speed multiplier for drones), `range` (launcher range, as the Tier 1 walk
has now), `resin_rate` (added to the conversion rate multiplier, like the
autonz' `conversion_rate`) and `flier_speed` (an Initiative multiplier for
every flier).

```json
"hive_warrior_brood":   { "name": "Warrior Brood", "description": "_draft", "tree": "hive",
                          "duration": 9000, "cost": { "biomass": 20 }, "required_building": "evolution_chamber", "required_int": 0,
                          "evolve": { "unlock": "warrior" } },
"launcher_range_1":     { "name": "Spire Reach I", "description": "_draft", "tree": "hive",
                          "duration": 12000, "cost": { "biomass": 25 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_warrior_brood", "evolve": { "range": 15 } },
"hive_hunter_brood":    { "name": "Hunter Brood", "description": "_draft", "tree": "hive",
                          "duration": 15000, "cost": { "biomass": 30 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_warrior_brood", "evolve": { "unlock": "hunter" } },
"launcher_range_2":     { "name": "Spire Reach II", "description": "_draft", "tree": "hive",
                          "duration": 18000, "cost": { "biomass": 40 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "launcher_range_1", "evolve": { "range": 20 } },
"hive_resin_flow_1":    { "name": "Resin Flow", "description": "_draft", "tree": "hive",
                          "duration": 9000, "cost": { "biomass": 15 }, "required_building": "evolution_chamber", "required_int": 0,
                          "evolve": { "resin_rate": 0.5 } },
"hive_drone_vigour":    { "name": "Drone Vigour", "description": "_draft", "tree": "hive",
                          "duration": 9000, "cost": { "biomass": 15 }, "required_building": "evolution_chamber", "required_int": 0,
                          "evolve": { "type": "drone", "work": 1.25, "health": 1.25 } },
"hive_warrior_carapace_1": { "name": "Warrior Carapace I", "description": "_draft", "tree": "hive",
                          "duration": 12000, "cost": { "biomass": 25 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_warrior_brood", "evolve": { "type": "warrior", "health": 1.25 } },
"hive_warrior_carapace_2": { "name": "Warrior Carapace II", "description": "_draft", "tree": "hive",
                          "duration": 18000, "cost": { "biomass": 40 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_warrior_carapace_1", "evolve": { "type": "warrior", "health": 1.25, "str": 1.2 } },
"hive_burrower_brood":  { "name": "Burrower Brood", "description": "_draft", "tree": "hive",
                          "duration": 15000, "cost": { "biomass": 30 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_drone_vigour", "evolve": { "unlock": "burrower" } },
"hive_spitter_brood":   { "name": "Spitter Brood", "description": "_draft", "tree": "hive",
                          "duration": 18000, "cost": { "biomass": 40 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_hunter_brood", "evolve": { "unlock": "spitter" } },
"hive_wing_membrane":   { "name": "Wing Membrane", "description": "_draft", "tree": "hive",
                          "duration": 12000, "cost": { "biomass": 25 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_hunter_brood", "evolve": { "flier_speed": 0.8 } },
"hive_hunter_talons_1": { "name": "Hunter Talons I", "description": "_draft", "tree": "hive",
                          "duration": 12000, "cost": { "biomass": 25 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_hunter_brood", "evolve": { "type": "hunter", "str": 1.25 } },
"hive_hunter_talons_2": { "name": "Hunter Talons II", "description": "_draft", "tree": "hive",
                          "duration": 18000, "cost": { "biomass": 40 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_hunter_talons_1", "evolve": { "type": "hunter", "str": 1.2, "health": 1.25 } },
"hive_resin_flow_2":    { "name": "Resin Surge", "description": "_draft", "tree": "hive",
                          "duration": 15000, "cost": { "biomass": 30 }, "required_building": "evolution_chamber", "required_int": 0,
                          "requires_tech": "hive_resin_flow_1", "evolve": { "resin_rate": 0.5 } }
```

The second upgrade of a type (`*_2`, and `hive_drone_vigour` for drones) is
the one that switches its adults to the stage-3 frame. Scouts, burrowers
and spitters get their `_1` / `_2` pairs on the same pattern (scout
`eyes` for sight, burrower `mandibles` for dig speed, spitter `acid` for
spit damage). They are listed in step 3's data, not repeated here.

**The Tier 1 walk keeps its spine.** The doctrine's `research` list (the
abstract walk with point costs) becomes:

| # | Entry | Cost (points) | Was |
|---|---|---|---|
| 1 | `hive_warrior_brood` | 25 | `tier_advanced` 25 |
| 2 | `launcher_range_1` (range 15) | 40 | the same |
| 3 | `hive_hunter_brood` | 60 | `tier_elite` 60 |
| 4 | `launcher_range_2` (range 20) | 80 | the same |
| 5+ | `hive_resin_flow_1`, `hive_drone_vigour`, `hive_warrior_carapace_1`, `hive_burrower_brood`, `hive_spitter_brood`, … | 60–120 | new, after the spine |

The range entries reach the same cumulative points as before, so the
launcher range grows on the same day and the spread pace is the same. The
settlement's `research` list is the same order, so the live research rung
walks it the same way the abstract step does.

**Live durations** are chosen so that a live hive evolves at about the
abstract pace. Tier 1 gives one point a day per stage reached (a nest earns
1, a hive 2). Phase B measured most hives reaching `tier_advanced` about 25
days after the nest. A 9,000-tick entry, worked by one drone at the
chamber about half the time, finishes in about 24 days. The soak measures
this (see "Balance").

**Upgrades apply to living units.** When a type upgrade completes
(`ResearchDoneEvent` with the settlement), every living adult of that type
in the settlement is re-typed: the multiplier is applied, and the evolved
frame is used if this was the second upgrade. New spawns get it from
`ApplyType`. Open question 5.

### The settlement block

```json
"settlement": {
  "crew": [ { "blueprint": "xeno_crew",  "type": "drone", "share": 0.75, "role": "worker" },
            { "blueprint": "troid_crew", "type": "scout", "share": 0.25, "role": "scout" } ],
  "needs": true,
  "diet": [ "biomass" ],
  "build_menu": [ { "id": "orders", "label": "Orders" }, { "id": "hive", "label": "Hive" } ],
  "buildables": [ "biomass_pool", "egg_cluster", "evolution_chamber", "egg_launcher" ],
  "research": [ "hive_warrior_brood", "launcher_range_1", "hive_hunter_brood", "launcher_range_2",
                "hive_resin_flow_1", "hive_drone_vigour", "hive_warrior_carapace_1", "hive_burrower_brood",
                "hive_spitter_brood", "hive_wing_membrane", "hive_hunter_talons_1", "hive_warrior_carapace_2",
                "hive_hunter_talons_2", "hive_resin_flow_2" ],
  "units": [ "hive_egg_drone", "hive_egg_scout", "hive_egg_warrior", "hive_egg_hunter",
             "hive_egg_burrower", "hive_egg_spitter" ],
  "recipes": [],
  "larvae": { "blueprint": "hive_larva", "every_ticks": 250, "cap": 3, "per_cluster": 2,
              "stations": [ "hive_heart", "egg_cluster" ] },
  "kill_drops": { "biomass": 3 },
  "camp": { "staged": true, "w": 7, "h": 7, "station": "hive_heart", "storage": "biomass_pool" },
  "director": { "...": "see The director table" }
}
```

New fields and what reads them:

| Field | Meaning | Read by |
|---|---|---|
| `crew[].type` | the type `ApplyType` gives that crew entry; `CrewBlueprints` passes it along | materialise, the grow rung, F's starting brood |
| `diet` | Material tags that count as food for this settlement's workers, eaten one item at a time (biomass: nutrition 30, so about one a day) | `HandleFindFood`, `FoodAvailable` (`internal/workerai/storage.go`) |
| `larvae` | the larva timer: blueprint, ticks, base cap, extra per cluster, the stations they appear beside | `stepHiveLarvae` |
| `kill_drops` | items a kill by this settlement's worker drops into the killer's bag (the rival-kill path from Phase D), on top of the victim's own drops: corpses as biomass | the kill handler |
| `camp.storage` | the blueprint the staged camp places when it has no storage (the autonz place a locker) | `buildStagedCamp` |
| `camp.w`, `h` | the brood's camp footprint. Stage 0 has no stamp: the brood is a heart, a pool and a 3-tile resin patch on open ground | `buildStagedCamp` |

A brood has nothing to build on until resin is laid, and the camp's heart
and pool are placed, not built. So the first thing a brood's director does
is lay resin. That is also the order a player hive would follow.

### The director table

The ladder is walked in order every 30 ticks, with the same rules as every
director (`max_open`, `radius`, reach flood, stuck orders cancelled). New
rung features are marked **new**.

```json
"director": {
  "every_ticks": 30, "max_open": 3, "radius": 14, "reach": 50,
  "max_mines": 4, "max_hauls": 4, "stuck_ticks": 3000,
  "dig_depth": 0, "max_hunts": 3, "hunt_ticks": 1500, "hunt_reach": 60,
  "priorities": [
    { "id": "drones",   "grow": { "type": "drone", "min": 2, "share": 0.35 } },
    { "id": "forage",   "mine": { "biomass": 30 } },
    { "id": "haul",     "haul": "biomass" },
    { "id": "resin",    "convert": "hive_resin", "min": 30, "per_colonist": 12 },
    { "id": "pools",    "build": "biomass_pool", "min": 1, "per_colonist": 0.1 },
    { "id": "clusters", "build": "egg_cluster", "per_colonist": 0.08, "stage_min": 1 },
    { "id": "guard",    "grow": { "type": "warrior", "share": 0.2 },
                        "guard": { "roles": ["guard"], "radius": 12, "post": 6 } },
    { "id": "chamber",  "build": "evolution_chamber", "min": 1, "stage_min": 1 },
    { "id": "evolve",   "research": true },
    { "id": "brood",    "grow": true },
    { "id": "spire",    "build": "egg_launcher", "min": 1, "stage_min": 2 },
    { "id": "launch",   "launch": { "walk": 3 } },
    { "id": "scouts",   "patrol": { "roles": ["scout"], "radius": 30, "every_ticks": 600 } },
    { "id": "excavate", "excavate": { "roles": ["digger"], "radius": 14 } },
    { "id": "raid",     "raid": { "fighters": 4, "party": 5, "home": 2, "cooldown_ticks": 6000, "max_ticks": 6000,
                                  "reach": 400, "unarmed": true, "roles": ["guard", "hunter", "spitter", "digger"],
                                  "scouted": true } },
    { "id": "hunt",     "hunt": "biomass", "min": 10, "per_colonist": 1, "hunger": 0 }
  ]
}
```

| # | Rung | Runs when | What it orders |
|---|---|---|---|
| 1 | `drones` | drones (adults, hatchlings and drone eggs) under `min + share × bodies`, a larva free, stock covers 2 biomass | the drone egg at the larva nearest the heart (grow, **new** `type`, `share` and the larva as its station) |
| 2 | `forage` | stored + carried biomass under 30 beyond open orders | chop the nearest flora that drops biomass (the mine rung's choppable path, unchanged) |
| 3 | `haul` | loose biomass in reach (drops, kills, chopped flora) | haul to a pool |
| 4 | `resin` | resin tiles within `radius` under `30 + 12 × bodies` | `ConvertAction` outward from the heart (unchanged rung) |
| 5 | `pools` | pools under `1 + 0.1 × bodies` | build a pool on resin |
| 6 | `clusters` | stage ≥ nest, clusters under `0.08 × bodies` (capped by the stage's `structures`) | build a cluster on resin (**new** `stage_min`, **new** cap from `structures`) |
| 7 | `guard` | warriors under `0.2 × bodies` (warrior unlocked) | warrior eggs. **New** guard behaviour: guard-role adults hold posts `post` tiles out from the heart, and any hostile within `radius` of the heart gets attack orders from every guard (`Orders.Send`), called back once it is dead or gone |
| 8 | `chamber` | stage ≥ nest, no chamber | build the evolution chamber on resin |
| 9 | `evolve` | a chamber stands | the next entry of the tree that is unlocked and not known, one at a time (research rung, `required_int` 0) |
| 10 | `brood` | bodies under the crew cap (the site's capacity, as for the greyish), a larva free | the egg of the type most under the stage's `mix`, among unlocked types (locked shares go to `morph_fallback`) |
| 11 | `spire` | stage ≥ hive (the launcher stage), no spire | build the Resin Spire on resin |
| 12 | `launch` | the Tier 1 firing rule would fire today (`Campaign.LaunchReady`: threshold, cooldown, launcher standing, a target in range) | **new**: send `shot_pop` adults (drones first, then scouts) to stand beside the spire. The daily live rule fires once they are within `walk` tiles and removes those bodies, not the eggs nearest the spire |
| 13 | `scouts` | scouts idle | **new** `patrol`: a waypoint on a ring at `radius` from the heart every `every_ticks`. A scout that sees a colonist or a player structure records **contact** on the settlement (`Site.Scouted`) |
| 14 | `excavate` | the resin rung found no convertible tile within `radius` but solid rock does lie there (asteroids, caves), burrowers exist | **new**: dig orders on the nearest rock tiles inside the radius (the dig rung's tunnel step, on the surface level), so resin has floor to spread on |
| 15 | `raid` | hostile (always, for the hive), the player's colony on the level, **new** `scouted` (a scout has made contact), fighters ≥ threshold | the raid rung, unchanged: warriors, hunters, spitters and burrowers (burrowers attack walls and doors in the way) |
| 16 | `hunt` | stored biomass under `10 + 1 × bodies` | attack orders on the nearest creature (**new**: `hunt: "biomass"` targets any non-hive creature, since `kill_drops` makes every kill biomass). Hunters first, then warriors |

Ordering notes:

- `forage`/`haul` sit above `resin`, so a hungry hive feeds before it
  spreads.
- `guard` sits above `evolve`, so a hive under threat buys warriors before
  research.
- The hive has no `dig` rung (no ore), so `dig_depth` is 0.
- The hunt rung runs whenever stock is short, with no hunger gate
  (`hunger: 0`), because hunting is the hive's second income, not an
  emergency.
- The last rungs (`launch`, `raid`, `hunt`) send bodies away from the
  heart, so they come after everything that builds.

### Tier 1: the abstract hive agrees with the live one

Phase A's growth stays authoritative. Its numbers do not change, and
neither do stage-ups, the research walk's spine or the firing rule. The new
presence fields are **bookkeeping that the live economy must match**. They
never block growth in the step:

| Field | Abstract step (per day) | Live (reconcile) |
|---|---|---|
| `units` (`map[type]int`) | a new body goes to the type most under the stage's mix among unlocked types; a starving body leaves from the type most over its mix, drones last; a shot takes drones first, then scouts; a landed brood is all drones | living bodies of the presence by `Kin.Type` (eggs count as their type, larvae do not count) |
| `larvae` | `+ 750 / every_ticks` (3), held at `cap + per_cluster × clusters` of the stage's `structures`; a new body spends one | living `hive_larva` of the settlement |
| `stockpile["biomass"]` | `+ forage` (`biomass_per_drone` 3, × (1 + 0.5 per `biomass` tag)) `− eat` (0.5 per body) `− eggs laid × type cost`, held between 0 and `biomass_cap` (60 + 10 per pool) | `storage.Summarize` over the settlement (as every settlement) |
| `conversion` (resin share) | the autonz formula: `per_day` (0.008) × the population's share of capacity × (1 + `resin_rate` techs) × (1 + tags), capped at 0.8; `floor: 1.0`, so capacity is unchanged | `ConversionShare` of `hive_resin` within `radius` 14 |
| `known_techs` | the walk (spine unchanged) | the settlement's techs (as every settlement) |

`population` stays the sum of `units`. Larvae are capped at 3 a day, and
the most a presence grows in a day (capacity 38 × 0.065) is 2.5, so the
larva count never limits Tier 1. Nor does biomass, which floors at 0. What
the fields do:

1. **Materialise from them.** A hive that grew while frozen lands with the
   right number of warriors, the grey eggs in its nest, biomass in its
   pools and resin out to its share.
2. **The soak checks the live hive against them.** Over 30,000 ticks, the
   live population's growth per day must be within 0.6–1.5× of the Tier 1
   rate at the same population and capacity. Live resin, first evolve and
   first launch must also land in the abstract bands (see "Balance").

If the soak finds the live hive starving where Tier 1 grows, that is the
signal to retune the live numbers (egg cost, forage, flora). Tier 1 is not
the thing that changes.

### Materialise and reconcile

**Materialise** (the hive moves onto the settlement path; creature
presences remain for husks only):

1. **Centre** as now (origin flag, keep-outs, by the chapter's origin on a
   home scenario).
2. **Camp, staged.** At stage 0, the heart, a pool and a 3-tile resin
   patch. At the nest and hive stages, the stage's stamp (now laid with
   `hive_resin`) re-stamped over the last, with the hold lifted out and put
   back. The heart and the pool are then placed at the centre if missing.
3. **Resin** to `conversion` (`ApplyConversion`, `hive_resin`, radius 14).
   This must run before the structures, which need resin.
4. **Structures by stage**: the stage's `structures` count of
   clusters, chamber and spire (the spire only at a launcher stage that is
   not `launcher_lost`, as now). They are placed on resin sites from the
   director's grid, so they stand where the director would have built
   them. The pools needed for the stock are placed too: one per 100
   biomass, at least one.
5. **Stock**: `stockpile["biomass"]` into the pools on the first build
   (as for every settlement).
6. **Bodies by type**: living bodies counted by `Kin.Type` and topped up
   or culled to `units`, as adults of each type (eggs: `egg_share` of each
   type's count laid on resin beside the heart, as coloured eggs). Larvae
   are topped up to `larvae` beside the heart and clusters. An old save's
   `units` is empty: fill it from `presenceComposition`, now by type.
7. **Adopt old bodies.** A presence creature with no `Worker` (a scripted
   Phase B adult or stage 2 that has the presence ID) is replaced by its
   type's crew body on the same tile. Its type comes from its colour
   (`Kin.Tier`, mapped as in "What changes from colour = tier"). This runs
   once and is idempotent.

**Reconcile** (every `writeLocation`): `units` and `population` from
living bodies by type; `larvae`; biomass stock; resin share; known techs;
a stage-rise as now; the spire lost when the launcher stage has none
standing (as now). Clusters and the chamber are *not* recorded as lost.
The director rebuilds them, and materialise places the stage's count
again. A wipe is no bodies, as now; eggs and larvae do not count.

**Live rule** (`stepLiveFactions`, daily): the live counts become the
record, the stage is stamped when it is reached (the autonz' rule), and
the launch fires only when the launch rung's bodies stand by the spire.
With the spire standing and the cooldown passed but no bodies there, it
waits a day, which delays the cooldown and does not spend it.

### Scenarios and the GameMaster

- **`troid_outbreak` and `exterminate_xenos` stay wild.** Their setup
  scripts and spawn rules spawn wild creatures as now (olive crawlers, blue
  fliers), so a 0-rival-factions game is unchanged in behaviour and sprites.
  The only change is the "Wild" names, which show in the log, the
  inspector and the scenario text that quotes a creature name. They stay
  random scenarios.
- **With a presence on a home scenario**, the presence's settlement builds
  by the chapter's origin (as now). Wild creatures are the same faction and
  never fight it. They are not adopted into it and do not count toward its
  population (open question 12). The GameMaster keeps spawning the
  scenario's wild waves, and every hive body (wild or settled) counts
  toward `hostile_max` (it already counts `ScriptedAI`; add `Kin` workers).
- **Objectives.** `exterminate_xenos` wins when every xeno and every egg is
  dead. Its `threat_blueprints` gain `xeno_crew`, `troid_crew` and
  `hive_larva`, and `troid_outbreak`'s gain `troid_crew` (and `xeno_crew`
  and `hive_larva`, since a presence there has crawlers too), so a hive's
  drones cannot outlive a win.
- **Wild and settled stay apart.** `find_nearest_worker` skips workers
  of the calling creature's own faction, so a wild xeno never attacks the
  hive's drones. The same rule applies to the mirror's or the autonz'
  workers if a scenario creature of their faction ever exists.
- **Husks** stay as now (the stamp, cracked eggs of the site's morph, the
  creatures removed).

### Hidden identity

The plan's rules are unchanged: every player-facing name is the morph's
until `hive:crawler=flier` is known. This phase adds:

- Each type has `seen_name` (the morph with its colour: "Red Xeno",
  "Green Troid") and `name` ("Warrior"). Bodies are spawned with
  `seen_name` in `Description.Name`. When B+ is built, learning the morph
  link renames every type (bestiary, inspector, log) at once. The reward
  is seeing the hive's castes named.
- Eggs say "Red Xeno Egg" until then, and larvae "Grey Egg". The egg's
  colour still tells an attentive player what will hatch, before they know
  why.
- New player-facing lines (a hive raid alert, an evolve Legends event)
  name the morph, and are checked by `TestFactionHiveMorphNames`.
- Observer runs show the faction and type names (the view rule, as now:
  `HIVE/XENO NEST 12 · 5 drones 2 warriors …`).

### Phase F: what the player will see

Not built here. The data above is shaped for it:

- **Build menu.** The `Hive` category: pool, egg cluster, evolution
  chamber, Resin Spire, each placeable only on resin (`required_floor` is
  already checked in the player's placement). **Orders** gains *Lay resin*:
  `queueConvert`, whose gate becomes "the doctrine has a `conversion.tile`"
  instead of the colony's `terraforming`, so the colony is unchanged.
- **Larvae.** The HUD's resource bar shows biomass (it already has a slot)
  and `Larvae 2 / 5`. Clicking a grey egg opens the crafting modal with the
  larva as its station: one row per egg type, with cost and lay time,
  locked types greyed with their evolve entry named. Queueing one sends a
  drone with the biomass.
- **Evolution.** Clicking the chamber opens the research modal over the
  `hive` tree (`settlement.research`), titled "Evolve". It shows each
  entry's effect line (`unlock`, multipliers, range).
- **Crew panel** lists units by type, with juveniles and eggs counted
  under their type.
- **No ship.** The launcher campaign mode, the hive's Star Map actions and
  its win rule are F's own work.

## Balance

**Tier 1 is unchanged.** `TestFactionPaceRivalOne` passes as it is, and
`make history-dump HFACTIONS=0,1,2,3 HFSEEDS=1-6 HFCOLS=0,40,100,200,300`
prints the same table as Phase A. Step 1 and step 6 both run it. Any
difference is a bug, not a retune.

**Live targets** (the hive soak):

| Measure | Target |
|---|---|
| resin share | +0.10 or more by 20,000 ticks, 0.25–0.45 by 30,000 |
| population growth per day | 0.6–1.5× Tier 1's rate at the same population and capacity, measured over the run |
| units at the end | drones 30–50%; warriors ≥ 1 once unlocked; every unlocked type present |
| eggs laid | ≥ 1 per 1.5 days |
| starvation deaths | ≤ 1 per seed |
| first evolve (warrior brood) | between tick 11,000 and 30,000 (days 15–40) on 8 of 10 seeds |
| spire built | within 3 days of reaching the hive stage, on seeds that reach it |
| first launch day after reaching the hive stage | within ±10 days of Tier 1's 20–37 |
| a raid | only after a scout's contact, never in the neutral soak |

**The hive soak** (`TestHiveSoak`, `internal/game/hive_soak_test.go`, run
by `make soak` after the autonz soak): ten New Expeditions. On each start
system, a hive presence of 8 (nest stage, drones and scouts only, 30
biomass in its pool) is put down with the stance forced neutral, as the
autonz soak does, so it does not raid the beamed-down colonist that keeps
the level live. It runs 30,000 ticks headless. Printed per seed:

- resin at 0 / 10k / 20k / 30k;
- population at the start and end, against the cap, and the Tier 1
  expectation;
- units by type at the end;
- eggs laid by type;
- peak larvae;
- biomass at the end;
- forage chops, hunts and hauls;
- first evolve tick and techs known;
- spire tick;
- launch-ready tick;
- starvation deaths;
- cancelled orders.

A seed **misses** when resin rose by less than 0.05, the population ended
below its start, no egg was laid, or more than one body starved. More than
two misses fails. The other columns are measured, not required, and are
recorded in the plan's "as built". `SOAKSEEDS` and `SOAKTICKS` apply as for
the other soaks.

**Expected first problem**: flora runs out within the radius on poor
starts (moons, asteroids). Hunting and the `kill_drops` corpses are meant
to carry those starts. If the soak shows them starving, open question 7
(regrowing resin bloom) is the fix.

## Order of work

Each step leaves the game playable and the suite green. Stop after any.
Steps 1–3 change no behaviour of a live hive. Step 4 is where the hive
becomes a settlement.

### Step 1: Types replace tiers

- `types`, `morph_fallback` and `tech_aliases` in the hive doctrine. Stage
  `mix` by type. `tiers` removed.
- `factory.ApplyType(e, faction, type, upgrades)`: `sprite_dx` on body and
  egg, absolute stats from the type (health, str, dex, ac, attack dice,
  initiative), `Role`, `AllowedTasks`, skills, equipment, the `hatch_turns`
  var, `seen_name`. Replaces `ApplyTier`. `TierSource` becomes
  `TypeSource`.
- `Kin.Type` (string) replaces `Kin.Tier`. When a save is loaded, the
  component's `tier` maps to the type of that colour.
  `Doctrine.TypesUnlocked(techs)`, with aliases.
- Tier 1: `FactionPresence.Units` and the composition rule (growth,
  starvation, shot, landing). `presenceComposition` by type. Phase B
  bodies (still scripted creatures in this step) spawn typed: a red crawler
  is a warrior by colour and stats, still running the stage scripts.
- **Files:** `data/factions.json`, `internal/campaign/faction.go`,
  `internal/campaign/doctrine_types.go` (new), `internal/factory/tier.go`
  → `type.go`, `internal/components/KinComponent.go`,
  `internal/game/faction_tiers.go`, `internal/game/faction_level.go`,
  `data/scripts/hive/egg.basic` (`hatch_turns`).
- **Done:** a presence's bodies carry types, colours match types, wild
  creatures are unchanged.
- **Verify:**
  - `internal/factory/type_test.go` (every type's offset, egg colour and
    stats; spawn_kin carries the type);
  - `internal/campaign/faction_types_test.go` (composition follows the
    mix, locked shares fall back by morph, aliases, units sum to
    population over 300 days, the shot takes drones);
  - `TestFactionPaceRivalOne` unchanged;
  - the history-dump table identical to Phase A's;
  - `make vmtest SCRIPT=factions_site` (frames change only in body colours;
    inspected and noted);
  - `lore_troid` and `lore_xenos` unchanged.

### Step 2: Hive content and engine hooks

- Blueprints: `xeno_crew`, `troid_crew` (Worker, Kin, `Needs`, no script;
  `hive.json`), `hive_larva`, `hive_heart`, `biomass_pool`, `egg_cluster`,
  `evolution_chamber`, and `spit_gland` (a natural ranged weapon). The
  Resin Spire becomes buildable.
- Tile `hive_resin` with `speed`. The stamps lay it.
- build.json `required_floor` and the build rung's `stage_min`.
- Recipes: the six egg recipes. Recipe fields `kin_type`,
  `consume_station`, `required_tech`.
- `settlement.diet` in the eating path, and `kill_drops` in the kill
  handler.
- The resin speed hook. `find_nearest_worker` skips the caller's faction.
- The wild blueprints are renamed "Wild …" (`Description.Name` only; the
  ids stay). `ApplyType` sets the type's `seen_name` on settled bodies.
  Check `lore_troid_test`, `lore_xenos_test` and any test or vmtest
  frame that matches a creature's display name.
- **Files:** `data/blueprints/entities/hive.json`,
  `data/blueprints/structures/furniture.json`,
  `data/blueprints/items/*.json` (gland), `data/build.json`,
  `data/crafting_recipes.json`, `data/tiledefinitions/dungeons.json`,
  `data/scripts/structures/hive_nest.basic`, `hive_core.basic`,
  `data/scripts/hive/nursery.basic` (new),
  `internal/crafting/crafting.go`, `internal/construction/buildables.go`
  (and placement), `internal/workerai/storage.go`,
  `internal/workerai/states.go`, `internal/workerai/task_handlers.go`
  (craft at a consumed station), `internal/game/resin_speed.go` (new),
  `internal/systems/ScriptedAISystem.go`,
  `internal/campaign/doctrine_settlement.go` (fields).
- **Done:** in the cheat console, a drone laid on resin builds a pool, a
  larva crafted into a red egg hatches a warrior (through the nursery) that
  joins the settlement, and a colonist on resin walks slower.
- **Verify:**
  - `internal/workerai` `TestCraft_ConsumesStation`,
    `TestCraft_RequiredTech`, `TestEat_Diet`;
  - `internal/construction` `TestRequiredFloor`;
  - `internal/game/resin_speed_test.go` (hive faster, others slower,
    fliers unaffected);
  - `internal/systems` find_nearest test (same faction skipped);
  - a kill by a hive worker drops biomass into its bag;
  - the content tests for the stamps' footprints (existing);
  - `TestColonyDoctrineCoversContent` (the new entries belong to the
    hive).

### Step 3: The evolve tree

- The `hive_*` entries in `research.json` (`tree`, `evolve`), including
  the scout, burrower and spitter upgrade pairs. The doctrine walk is
  renamed on its spine. The settlement's `research` list.
- `ApplyType` applies known upgrades. Living units are re-typed when an
  upgrade completes, and the second upgrade switches the frame.
  `resin_rate` feeds the conversion rate, `flier_speed` every flier's
  Initiative, and `range` stays as now.
- **Files:** `data/research.json`, `data/factions.json`,
  `internal/research/*.go` (the `tree` and `evolve` fields),
  `internal/campaign/faction.go` (`Range`, the conversion rate),
  `internal/factory/type.go`, `internal/game/faction_settlement.go`
  (`ResearchDoneEvent`).
- **Done:** a hive settlement that knows `hive_warrior_carapace_2` draws
  its warriors in the stage-3 frame with 1.56× health.
- **Verify:**
  - `internal/research` test (the hive tree walks in order; the colony's
    tree is unchanged);
  - `TestFactionPaceRivalOne`;
  - the Tier 1 walk test (range entries at their old cumulative cost);
  - upgrades on spawn and on living units;
  - the C0 snapshot tests (the colony's research list is unchanged).

### Step 4: The hive as a settlement

- The `settlement` block (crew by type, diet, build menu, buildables,
  research, units, larvae, kill_drops, staged camp with storage) and the
  `conversion` block.
- `stepHiveLarvae`. `SpawnKin`: a settled presence's stage-2 spawns the
  crew body with the nursery script for hatchlings.
- Materialise and reconcile as described: resin first, structures by stage,
  stock, bodies by type, larvae, the adoption of old scripted bodies. The
  live rule stamps the stage.
- Content `threat_blueprints`. The GameMaster's hostile count includes
  `Kin` workers.
- At this step the director runs the **autonz-shaped minimum** (grow,
  forage, haul, resin, pools), so the hive feeds and grows. The full table
  is step 5.
- **Files:** `data/factions.json`, `data/scenarios/exterminate_xenos.json`,
  `troid_outbreak.json`, `internal/game/faction_level.go`,
  `internal/game/faction_settlement.go`, `internal/game/faction_hive.go`
  (new: larvae, adoption, structures by stage), `internal/factory/type.go`,
  `internal/game/gm.go`, `internal/campaign/faction.go` (the live rule's
  bodies).
- **Done:** landing on seed 777's Rust Tangle after `debug days 36` shows
  the nest laid with resin, the heart with grey eggs, coloured eggs, drones
  chopping and hauling into a pool, and warriors standing by. A save, a
  freeze and a thaw round-trip it.
- **Verify:** `internal/game/faction_hive_test.go`:
  - materialise is deterministic and idempotent;
  - resin before structures, every structure on resin;
  - bodies by type match `units`;
  - larvae to the cap;
  - stock into pools;
  - adoption replaces scripted bodies once;
  - reconcile round-trips units, larvae, stock, resin and techs;
  - a larva becomes an egg becomes a working drone;
  - a wild xeno ignores the hive's drones;
  - `exterminate_xenos` is not won while drones live.

  Also existing tests: `faction_level_test.go` (husks, launcher) and the
  `lore_*` scenario tests.

### Step 5: The hive director

- Rung features: grow `type` / `share` / larva station; `guard`;
  build `stage_min` and the stage's structure cap; `launch`; `patrol` with
  contact (`Site.Scouted`); `excavate`; raid `scouted`; `hunt: "biomass"`.
- The full table in the doctrine. The live launch takes the bodies at the
  spire.
- `Doctrine.RaidAlert` for the hive (morph-named).
- **Files:** `internal/director/grow.go`, `internal/director/guard.go`
  (new), `internal/director/patrol.go` (new),
  `internal/director/launch.go` (new), `internal/director/dig.go`
  (excavate), `internal/director/raid.go`, `internal/director/director.go`
  (`stage_min`, the hunt target),
  `internal/campaign/doctrine_settlement.go`,
  `internal/game/settlement_orders.go`, `internal/game/faction_level.go`,
  `data/factions.json`.
- **Done:** a nest-stage hive left live for 30 days lays resin, builds a
  cluster and the chamber, researches the warrior brood, keeps a guard
  ratio, and at the hive stage builds the spire and launches with bodies
  walking to it.
- **Verify:**
  - `internal/director/hive_test.go` (each rung's trigger and order on a
    built level: drones first when short, guard ratio, stage_min gates the
    chamber and spire, the brood picks the type most under the mix among
    unlocked, launch sends shot_pop to the spire only on a ready day, patrol
    records contact, raid waits for contact, excavate digs only when resin
    has no floor, hunt targets non-hive creatures);
  - `internal/game/faction_hive_live_test.go` (a live hive launches with
    its bodies; guards attack a colonist inside the radius; a hostile hive
    raids after a scout's contact);
  - `TestFactionHiveMorphNames`.

### Step 6: Tier 1 bookkeeping

- Abstract `larvae`, biomass stock and resin per day. The structures'
  counts from the stage. history-dump columns: units by type, resin,
  biomass (`HFDETAIL=1`).
- The Factions tab lists units by type and resin for a hive row (observer:
  type names; normal campaign: morph `seen_name`s of what has been seen).
  The Star Map tag is unchanged.
- **Files:** `internal/campaign/faction.go`,
  `internal/campaign/faction_hive.go` (new), `cmd/history-dump/main.go`,
  `internal/game/factions_panel.go`, `internal/game/faction_view.go`.
- **Done:** a presence grown frozen for 60 days lands with the warriors,
  larvae, stock and resin its record says.
- **Verify:**
  - `internal/campaign/faction_hive_test.go` (larvae and stock bounded
    and never blocking growth; resin follows the formula and does not
    change capacity; save round-trip of the new fields; a version 4 save
    without them loads);
  - `TestFactionPaceRivalOne`;
  - the history-dump table identical;
  - `TestRevealOnlyInObserverRun` (the tab's fog).

### Step 7: Soak, vmtest, docs

- `TestHiveSoak` in `make soak`, with numbers recorded in the "as built".
- vmtest `factions_hive.jsonl`: seed 777, Rival factions 1, `debug days
  36`. Fly to Rust Tangle and beam down. Frames:
  1. the Star Map tag;
  2. the nest from the landing's camera, panned to the heart: resin,
     pools, grey and coloured eggs, drones at work;
  3. after `debug days 3`, the cluster or chamber standing and new eggs
     hatched;
  4. the Factions tab.

  The same script in an observer run (`observer on` at New Expedition,
  View Rust Tangle) adds a frame with the type names.
- Docs: `factions.md` ("The hive as a settlement", replacing "The hive's
  creatures" tier paragraph; the table in "How a faction moves and acts":
  the hive gets a settlement director), `vmtest.md` (the new row),
  `enemy_factions_plan.md` (E2 in the phasing table, the hive section's
  colour note pointing here, "Phase E2 as built"), and the todo.
- **Files:** `internal/game/hive_soak_test.go` (new), `Makefile`,
  `vmtest/scripts/factions_hive.jsonl` (new), `internal/game/vmhook.go`
  (sidecar: units by type, larvae, resin for faction settlements),
  `docs/developer/factions.md`, `docs/developer/vmtest.md`,
  `docs/brainstorm/enemy_factions_plan.md`, `docs/brainstorm/todo.md`.
- **Done:** the soak passes, the frames are inspected, and the docs say
  what was built.
- **Verify:**
  - `make soak`;
  - `make vmtest SCRIPT=factions_hive`;
  - every other vmtest script passes, with `factions_site` frames
    differing only as noted in step 1;
  - `go test ./...`.

Steps 2 and 3 touch different files and can run side by side after step
1. Steps 4 and 5 run in order. Step 6 can start after step 4 in its own
worktree (it touches `internal/campaign` and the panel, while step 5
touches `internal/director`).

## Open questions (decided 2026-11-08)

The owner accepted every default below (2026-11-08), and also decided
that wild creatures are named "Wild" (question 12). They are decisions
now; the alternatives are kept for the record. Phase F still owns the
launcher-mode win rule.

1. **"A hive key in research.json".** Default: flat `hive_*` keys with
   `"tree": "hive"`, because the file is a flat map and doctrines already
   pick their entries. The alternative is a nested `"hive": { … }` object,
   which needs a loader change and touches every reader of the file.
2. **Green flier: spitter or carrier.** Default: **spitter** (ranged, with
   a natural weapon item). A carrier (which hauls eggs, perhaps to the
   spire, or carries drones over moats) would need a "carry a body"
   mechanic that does not exist.
3. **Purple crawler: burrower or evolved worker.** Default: **burrower**
   (dig, excavate rock for resin, break walls on raids). An evolved worker
   would overlap the drone and its `hive_drone_vigour` upgrade.
4. **A visual for upgrades.** Default: a type's second upgrade draws its
   adults in the stage-3 (larger) frame. The alternative is no visual
   (stats only), or a small marker drawn over the unit.
5. **Upgrades reach living units.** Default: yes, re-typed when the
   research completes. Otherwise only new hatches get them.
6. **Launching is physical.** Default: `shot_pop` adults walk to the spire
   and the live rule takes them. The alternative is an egg-loading recipe
   at the spire (larvae and biomass in, a shot out), which is more
   recipe-shaped but means a recipe output that fires a fleet.
7. **Biomass regrowth.** Default: none in this phase; flora, corpses and
   hunting only, measured by the soak. If poor starts starve: a `resin
   bloom` choppable that grows on resin tiles over days (a hive farm).
8. **Corpses as biomass.** Default: `kill_drops` 3 biomass for any kill by
   the hive, the player's colonists included. That is grim, and it is the
   owner's call.
9. **The spit gland.** Default: an inventory weapon flagged so it is not
   dropped or looted. The alternative is a `RangedAttack` stat on the
   blueprint, which would be new combat code.
10. **Settled hatchlings.** Default: a nursery script that stays near the
    heart and fights only if attacked. The alternative is to keep
    `hatchling.basic` (juveniles hunt any colonist within 12).
11. **Art for the heart, pool, cluster and chamber.** Default: the slime
    blobs on the same sheet (row y=744/768, x=576–767: green, magenta, red,
    violet, blue, grey, yellow, crimson) for the heart and pools, and the
    egg-cluster and chamber sprites borrowed from the existing
    `eldritch_*` furniture. Say if you want new art first.
12. **Wild creatures and a presence.** Decided in part (owner,
    2026-11-08): wild xenos and troids are **named "Wild"** to set them
    apart from a presence's bodies (see "Two kinds of hive body"). Also
    decided, as the default: they stay wild (friendly to the hive, not
    counted, not adopted). The alternative is that wild adults within the
    radius join the settlement as their type by colour, and drop the
    "Wild" name when they do.
13. **The GameMaster on a home scenario with a presence.** Default:
    unchanged (it keeps the wave pressure). The alternative is to suppress
    its egg spawns while a presence's heart stands, so the nest is the
    only source of eggs.
14. **Scouts and contact.** Default: a raid needs a scout to have seen the
    player's colony. Without the gate, the hive raids whenever it is strong
    enough, as the mirror and greyish do.
15. **Phase name.** Default: Phase E2, so that F keeps its letter.

## Where the owner rewrites text

- Type `name`s and `seen_name`s (`data/factions.json` `types`).
- The wild creatures' names ("Wild Xeno", "Wild Troid Egg", …) in
  `data/blueprints/entities/hive.json`, and any scenario description or
  quest text that quotes them.
- Egg recipe names and descriptions (`crafting_recipes.json`).
- Every `hive_*` research name and description, and the renamed
  `launcher_range_*` names ("Spire Reach").
- Blueprint names and `LongDescription`s: `xeno_crew`, `troid_crew`,
  `hive_larva` ("Grey Egg"), `hive_heart`, `biomass_pool`, `egg_cluster`,
  `evolution_chamber`, `spit_gland`.
- The tile name `hive_resin` (shown in the inspector).
- The hive's raid alert (`Doctrine.RaidAlert`) and any evolve alert.
- A new Legends event if one is added (`faction_evolved`, morph-named).
- The nursery script's and egg script's messages ("was heard hatching!").
- The Factions tab's hive row labels.
- `hive:castes` lore-link wording when B+ lands.

All of these are marked `"_draft"`, as the existing files are.

## As built (2026-11-08)

Commits on `claude/factions-a-a379f7` after `c2eb389` (this plan): step 1
`fea579c`, step 2 `1246056`, step 3 `b4ffbd2`, steps 4 and 5 `2b38a3e`, then
steps 6 and 7 with the soak's tuning and the docs. Where the plan and the code
disagreed the plan's intent was kept; the notes say why.

### Step 1: types replace tiers

- `types`, `morph_fallback` and `tech_aliases` in the doctrine; stage `mix`
  by type; `tiers` gone. `factory.ApplyType` (`internal/factory/type.go`),
  `Kin.Type`, `Doctrine.TypesUnlocked` / `TypeMix`, `FactionPresence.Units`
  with the growth, starvation, shot and landing rules
  (`internal/campaign/doctrine_types.go`). No random draw and no change to
  `population`: `TestFactionPaceRivalOne` passes unchanged and the
  history-dump table (`HFACTIONS=0,1,2,3 HFSEEDS=1-6
  HFCOLS=0,40,100,200,300`) is identical to Phase A's at every step (the
  detail listing differs only in tech names and the new units line).
- Stats apply to adults only (stage 2, stage 3, crew); an egg gets its
  colour, its name ("Red Xeno Egg") and `hatch_turns`; a hatchling and a
  shell their colour.
- **Troid worlds.** Phase B swapped the default and site morphs' shares. With
  types, the fighting types swap with their other-morph counterpart (warrior
  ↔ hunter, burrower ↔ spitter) and the economy types keep theirs, since the
  drones are the workers: a troid world's hive has more fliers than a xeno
  world's, not most (the Phase B test was changed to say so).
- Old saves: a Kin component's `tier` maps to the type of that colour in
  `KinComponent.UnmarshalJSON` (a fixed table: the components package has no
  doctrine); a presence body with no type counts as its morph's fallback.
- Reconcile counts `units` by type from step 1, since bodies were typed from
  then on.
- `factions_site` frames: the same layout; a red warrior among olive drones
  and blue scouts.

### Step 2: content and engine hooks

- Blueprints: `xeno_crew`, `troid_crew`, `hive_larva` ("Grey Egg", a crafting
  station), `hive_heart`, `egg_cluster`, `evolution_chamber` (hive.json),
  `biomass_pool` (furniture.json), `spit_gland` (weapons.json: ranged, 1d10,
  range 6). Sprites (open question 11): the heart the green blob (576,744),
  pools the violet (720,744), clusters the magenta (624,744), the chamber the
  crimson (720,768). No `eldritch_*` furniture exists to borrow, so the
  chamber is a blob too.
- The recipe tech field is the existing `requires_tech` (already read by the
  grow rung), not a new `required_tech`; it is now also enforced in
  `orderCraft` and at craft time. `kin_type`, `consume_station` are new.
- **`Kin.Settlement`** (new): the settlement that laid an egg, carried by
  `spawn_kin`. It is how a settled line is told from a wild one: a settled
  hatchling runs `nursery.basic` (with an AIMemory so `was_attacked` works)
  and its stage 2 is the crew body (`factory.KinStageOverride`,
  `KinPrepare`). Built here rather than in step 4 so step 2's done criterion
  holds.
- Tile `speed`: a wrapper system round the rogue library's initiative system
  (`internal/game/resin_speed.go`) rescales the counter when it refills;
  fliers, eggs, larvae, Inanimate things and structures are untouched.
- `kill_drops` (`grantKillDrops`): the killer is the hive worker at the
  victim's remembered attacker tile, else one beside it; structures and
  items are not corpses. Diet: `workerai.SettlementDiet`, a unit of a biomass
  stack per meal.
- The spit gland needs no flag: a body's bag is never dropped on death.
- Wild blueprints are named "Wild Xeno", "Wild Troid Egg" and so on; no test,
  scenario text or vmtest frame quoted the old names.
- The done criterion was checked by Go tests, not the console
  (`TestHiveBuildNeedsResin`, `TestSettledEggGrowsAWorker`,
  `TestCraft_ConsumesStation`, `TestCraft_RequiredTech`, `TestEat_Diet`,
  `TestResinSpeed`, `TestFindNearestWorkerSkipsOwnFaction`,
  `TestHiveKillDropsBiomass`, `TestRequiredFloor`).

### Step 3: the evolve tree

- The plan's 14 entries plus scout eyes (initiative 0.85, then health),
  burrower mandibles (work 1.25, then str) and spitter acid (str, then str
  and health: spit damage is the gland's dice). A type's second upgrade is
  marked `"evolved": true` in data rather than counted.
- **Durations are work steps**, one per worker turn, not ticks: the plan's
  9,000 took one drone about 90,000 ticks. They were divided by ten (the
  warrior brood 900), which is the plan's "about 24 days at half a drone's
  time".
- The Tier 1 walk has all twenty entries, the spine at Phase A's costs.
  Old saves' `tier_advanced` / `tier_elite` are renamed in `known_techs` on
  load (same walk position), so a thawed hive never researches the brood
  twice.
- Upgrade techs come from the presence's settlement (its live techs), else
  its record. `resin_rate` is the walk's `conversion_rate` (Tier 1 resin);
  the live resin rung lays by count.

### Steps 4 and 5: the settlement and its director

These landed in one commit: they share `faction_settlement.go`,
`faction_level.go`, `faction_hive.go`, `director.go` and the doctrine table,
and step 5's Go was written while step 4's soak ran.

- The typed grow rung (type, share, reserve, the larva as station) came with
  step 4: the old grow rung counts crew by blueprint and every egg recipe
  outputs the same egg, so the minimum ladder could not grow without it.
  `grow` takes `true` or an object.
- Brood camp: the heart at the centre, a pool beside it and a 3×3 resin
  patch (the plan's "3-tile patch" read as radius 1). A stage rise re-stamps
  the chamber with the hive's own things lifted out and put back at the
  nearest free tile.
- **Resin is creep beyond the camp.** A view of a brood planet laid its
  patch and changed the record (`TestViewedPlanetStepsAsBeforeOnceLeft`);
  the presence's `conversion` is now the resin share beyond the camp's own
  floor (the patch, the chamber's circle), and materialise lays the camp's
  ground plus that share, nearest first.
- **Radiation.** A moon's surface is irradiated everywhere (40 on the test
  moon), which killed every hive body in about 100 ticks (Phase B's scripted
  bodies too). The doctrine's `body_skills: ["radiation_resist"]` goes on
  every typed body at every life stage and on the larva; wild creatures are
  unchanged.
- **On a live level nothing is placed.** The daily rule first re-ran the
  whole materialise at a stage rise, placing the spire from nothing; now it
  only stamps the chamber and the director builds.
- `presenceBodies` leaves out larvae and shells (Phase B counted a cracked
  shell as a body); `presenceLauncher` matches the launcher blueprint (the
  heart is a Kin structure too); the hive's spire is any one within its
  radius (a built one has no Kin presence).
- The threat check (`checkThreatCleared`) looks at static entities too, so
  grey eggs keep `exterminate_xenos` from being won; the GameMaster counts
  settled hive workers toward `hostile_max`.
- New order paths: `Orders.Move` and `Orders.Hold`
  (`task_requests.MoveAction`, a plain walk; `HoldHere` stays). A director's
  move first used the default move handler, which on arrival "interacts"
  with the tile and picked up a drone standing there (a guard walking to its
  post); it is now a plain walk (`TestMoveTaskIsAWalk`).
- Guard: a hostile is the player's colonist (when hostile) or a scripted or
  faction mob of another faction within 12 of the heart. Contact is the
  level flag `contact:<settlement>`, set when a scout is within 8 of the
  player's colonist or storage. Excavate runs when the resin rung had no
  candidate at all. Hunt `biomass` takes any non-Inanimate creature of
  another faction; "hunters first, then warriors" is the queue (only attack
  types take attack orders), not an ordering.
- The launch is physical: `Campaign.LaunchReady` is the Tier 1 check factored
  out of `tryLaunch` without its random draw; the launch rung walks the shot
  (drones, then scouts) to the spire and holds it; the live rule fires only
  with `shot_pop` workers holding there and removes those. The first version
  counted any worker within 3 of the spire, so the drone working the
  evolution chamber (two tiles off) was launched mid-research; bodies
  removed from a level now hand their task back first (`removeBody`).
- The raid alert names the morph ("Xenos are swarming toward your colony on
  ..."), checked in `TestFactionHiveMorphNames` with the types' names.

### Step 6: Tier 1 bookkeeping

- `larvae` (+3 a day, held at the stage's cap, one spent per new body), the
  biomass stock (a `biomass` block: 3 per drone, ×1.5 on a biomass site, less
  0.5 a body and each egg's cost, held under 60 + 10 per pool) and resin
  (the convert formula, `per_day` 0.008). The `conversion` block has **no
  tags**: a conversion tag also weights seeding (`economyWeight`) and would
  move every seed's hives.
- history-dump `-fdetail` prints a units / resin / biomass / larvae line per
  hive. The Factions tab lists units by type under the site row (an observer
  run: type names and larvae; a normal campaign: only the site's morph's
  types by their seen names, so the row never shows the other morph).

### Step 7: soak, vmtest, docs, and the tuning the soak asked for

The first soak seeds starved (16 deaths on seed 1, biomass at zero, 34
chops in forty days: a tree drops one biomass) and never evolved. The live
numbers were retuned, as the plan says, and Tier 1 left alone:

- **The resin bloom** (open question 7's fix): `resin_bloom`, a choppable
  dropping two biomass, grows every 150 ticks on an open resin tile within
  the radius while fewer than one per 12 resin tiles stand
  (`stepHiveBloom`; settlement `bloom`).
- Crew `HungerRate` 8 (was 5); forage target 40 with six open mines; a
  `reserve` on the grow rungs (0.5 biomass a body for drones, 1 for warriors
  and the brood): a hungry hive feeds before it breeds.
- The resin rung's want counts the chamber's own tiles, so it is 60 + 15 per
  worker (was 30 + 12).
- The drones and guard rungs grow past the crew cap (the plan's table caps
  only the brood); without that a hive at its cap never raised a warrior
  after unlocking them. `stuck_ticks` 6000 (an interrupted evolve was
  cancelled and its biomass lost).
- **The hive does not sleep.** An evolve stalled after about 600 work turns
  on most seeds: the drone working it reached its exhaustion limit (600),
  went to rest still holding the research task, and the task sat "in
  progress" until cancelled. The crew bodies' `MaxExhaustion` is now out of
  reach (they have no beds). The underlying workerai behaviour (a resting
  worker keeps a long task it cannot work) is untouched; the colony's
  research is short enough never to meet it.

**The hive soak** (`TestHiveSoak`, `internal/game/hive_soak_test.go`;
`make soak` runs it after the autonz soak, `HIVETICKS` 30,000, `SOAKSEEDS`
applies): ten New Expeditions, a hive of eight at the nest (6 drones, 2
scouts, 3 larvae, 30 biomass) on the start system, neutral, one colonist
beamed down, 30,000 ticks.

| Seed | Resin 0 / 10k / 20k / 30000 | Pop start / end (cap; Tier 1) | Stage | Units at the end | Eggs laid | Peak larvae | Biomass | Chops / hunts / hauls | First evolve | Techs | Hive stage / spire / launch-ready / first launch tick | Launches | Starved | Cancelled |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 0.01 / 0.17 / 0.26 / 0.35 | 9 / 24 (24; 24) | 2 | drone 11, scout 8, warrior 5 | drone 7, scout 6, warrior 5 | 7 | 44 | 62 / 2 / 0 | 7350 | hive_warrior_brood | 2250 / 19140 / 19500 / 20250 | 1 | 0 | 6 |
| 2 | 0.01 / 0.19 / 0.27 / 0.31 | 9 / 25 (22; 22) | 2 | drone 11, scout 9, warrior 5 | drone 7, scout 7, warrior 5 | 7 | 32 | 59 / 3 / 0 | 16590 | hive_warrior_brood | 1500 / 17070 / 17250 / 18000 | 1 | 0 | 9 |
| 3 | 0.01 / 0.17 / 0.28 / 0.34 | 9 / 24 (24; 24) | 2 | drone 14, scout 10 | drone 11, scout 8 | 7 | 8 | 71 / 16 / 0 | 25320 | hive_warrior_brood | 2250 / 16710 / 17250 / 18000 | 1 | 1 | 11 |
| 4 | 0.01 / 0.17 / 0.29 / 0.39 | 9 / 29 (24; 24) | 2 | drone 13, scout 10, warrior 6 | drone 9, scout 8, warrior 6 | 7 | 38 | 55 / 3 / 0 | 17910 | hive_warrior_brood | 1500 / 7170 / 7500 / 17250 | 1 | 0 | 4 |
| 5 | 0.01 / 0.23 / 0.31 / 0.35 | 9 / 24 (24; 24) | 2 | drone 14, scout 10 | drone 13, scout 8 | 7 | 45 | 55 / 3 / 0 | - |  | 1500 / 6930 / 7500 / 21000 | 1 | 3 | 5 |
| 6 | 0.01 / 0.19 / 0.26 / 0.31 | 9 / 22 (22; 22) | 2 | drone 13, scout 9 | drone 9, scout 7 | 7 | 39 | 66 / 3 / 0 | - |  | 2250 / 17610 / 18000 / 18750 | 1 | 0 | 6 |
| 7 | 0.01 / 0.16 / 0.22 / 0.28 | 9 / 20 (22; 22) | 2 | drone 13, scout 7 | drone 9, scout 7 | 7 | 0 | 28 / 20 / 0 | 24240 | hive_warrior_brood | 1500 / 6390 / 6750 / 7500 | 1 | 2 | 11 |
| 8 | 0.01 / 0.18 / 0.24 / 0.36 | 9 / 24 (22; 22) | 2 | drone 11, scout 8, warrior 5 | drone 7, scout 6, warrior 5 | 7 | 41 | 57 / 0 / 0 | 16650 | hive_warrior_brood | 3000 / 16920 / 17250 / 18000 | 1 | 0 | 9 |
| 9 | 0.01 / 0.08 / 0.19 / 0.26 | 9 / 27 (24; 24) | 2 | drone 12, scout 9, warrior 6 | drone 8, scout 7, warrior 6 | 7 | 38 | 65 / 4 / 0 | 17340 | hive_warrior_brood | 1500 / 19740 / 20250 / 21000 | 1 | 0 | 8 |
| 10 | 0.01 / 0.22 / 0.28 / 0.31 | 9 / 22 (22; 22) | 2 | drone 13, scout 9 | drone 9, scout 7 | 7 | 15 | 68 / 9 / 0 | 25290 | hive_warrior_brood | 1500 / 5700 / 6000 / 6750 | 1 | 0 | 6 |

Ticks are counted from the landing (750 a day). "Pop … (cap; Tier 1)" is the
live end against the site's capacity and what Tier 1 reaches from the same
start in the same days; "Launches" counts fleets still in flight at the end
(a landed egg leaves the list).

Against the plan's targets: the run passes (no seed missed: resin rose, the population held, eggs
were laid; two seeds lost more than one body to hunger, within the two
misses allowed).

| Measure | Target | Measured |
|---|---|---|
| resin | +0.10 by 20,000; 0.25–0.45 by 30,000 | +0.18 to +0.30 by 20,000; 0.26–0.39 at 30,000 on all ten |
| population growth | 0.6–1.5× Tier 1's | every seed reached its capacity (as Tier 1 does) inside about twelve days; ends 20–29 against Tier 1's 22–24 |
| units at the end | drones 30–50%; warriors once unlocked; every unlocked type | drones 45–58%: the nest's locked shares fall back to drones; five or six warriors on the five seeds that learned the brood by day 24, none yet on the three that learned it after day 32 (eggs take a day or two to grow into workers) |
| eggs laid | ≥ 1 per 1.5 days | 14–21 in forty days (one per two to three days): growth stops at the cap within about twelve days |
| starvation | ≤ 1 per seed | eight seeds none or one; seed 5 three, seed 7 two |
| first evolve | days 15–40 on 8 of 10 | eight of ten evolved: seven inside the band (days 22–34), one early (day 10), two not by day 40 |
| spire | within 3 days of the hive stage | no: the hive stage comes at days 2–4 and the spire at days 8–27; it costs 20 biomass and its rung comes after the evolve and brood rungs |
| first launch | within ±10 days of Tier 1's 20–37 after the hive stage | 7 to 26 days after the hive stage |
| raid | only after contact | none (the soak runs neutral) |

The misses against the plan's measured (not required) targets are recorded,
not tuned away: the spire's place in the ladder and the egg rate at the cap
are the next balance questions, and the owner's call.

**vmtest**: `factions_hive` (seed 777, Rival factions 3 rather than 1: with
one the seed's hive is on Glass Expanse, not Rust Tangle; `debug stance hive
neutral` keeps its guards off the landing party; a three-day `wait` instead
of `debug days 3`, which steps only Tier 1 and so leaves a loaded level
alone) and `factions_hive_observer` (the observer frame with type names).
New vmtest command `stance`; the sidecar's `faction` gains `units`, `eggs`,
`larvae`, `resin`, `biomass`, `built` and `ordered`.

**Not verified / not built**: B+ naming (`name` waits for it); the player's
side (F); a hostile hive's raid on a live level watched in the VM (the rung
and contact are tested in Go; the soak runs neutral); burrowers, hunters and
spitters in the live soak (forty days reach only the warrior brood); the
excavate rung on an asteroid level (tested on a built level only).

