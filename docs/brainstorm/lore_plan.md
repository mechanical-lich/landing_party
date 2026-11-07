# Lore and History — Plan

Status (2026-11-07): **steps 1–5 implemented** on branch
`claude/lore-history-4d1fe2`, not yet merged; step 6 is deferred to the enemy
factions work. How it was built differs from this plan in places: see
[Implementation notes](#implementation-notes) at the end, and
[docs/developer/lore.md](../developer/lore.md) for the system as built.
The plan below is the original (2026-11-03, line references to `dev` at
`68b0b9a`), with the owner's answers folded in.

## Decision

Every campaign gets a generated history, in the spirit of Dwarf Fortress's
legends, and that history explains each site the player can visit. Play adds to
the same history as it happens.

- **All history is alien.** Humans are new to this region; the crew's own story
  starts with the campaign. Every civilization in the prehistory is alien, and
  each is either still here, died out, or fled. Some are ancestors of the
  creatures in the game today: the mutants descend from someone.
- **History is structured data, not prose.** It is a log of typed events with
  participants, a site, a date and the events that caused them. Text is
  rendered from templates when shown, so wording can change without touching
  saves.
- **The map type is the present; history explains how the planet got there.**
  Generation keeps choosing the map first. The history chosen for a site has to
  end in that map's present state. A desolate moon can have been lifeless
  forever, or full of life until a radiation leak.
- **The threat scenario is the latest chapter of that history.** Zombies,
  xenos, a crashed UFO or mutants each come from an origin event, and that
  event has a place on the map where the player can find evidence of it.
- **Some planets never had anyone.** A quarter of sites are lifeless through
  their whole history and get short histories, mostly of visitors, so inhabited
  ones stand out. Others *become* lifeless: whole-planet extinctions are
  first-class events.
- **Time is the star date.** One campaign-wide clock, independent of any
  planet, is the colonists' own reckoning of time. History is dated in it, and
  so is everything that happens in play.
- **History only claims what the map can show.** Every event lists the evidence
  it leaves. Where the map doesn't already show it, the event adds it through
  the existing feature and `stamp` route.
- **History picks the scenario.** The threat chapter is an event, and the
  event names the scenario it plays as. Nothing rolls a scenario on its own any
  more; a site with no threat chapter plays its map's quiet `base_scenario`.
  Scenarios stay random from the player's view, since the player never picks:
  the dice moved up one level, into which chapter the history ends on.
- **Events are data** (`data/history/*.json`): requirements, effects and what
  they add to generation. A new kind of disaster is a JSON change.

## Where the code is today

| Concern | Behaviour | Code |
|---|---|---|
| Lore package | Name generator only: pools in `data/names.json`, seeded and unseeded rolls, epithets, `<pool>` placeholders in blueprints. | `internal/lore/names.go` |
| Charting a site | `newLocation` picks a map and a scenario uniformly from the archetype's lists using the campaign rng, then sets `Seed = c.Seed + GenSeq*131`. Sites are charted lazily as the frontier grows (`Expand`, `Scan`, travel quests, contracts), so the star map does not exist up front. | `internal/campaign/generate.go:471` |
| Archetype text | `Summary` is one fixed sentence per archetype, copied onto the location. | `data/location_templates.json` |
| Generation inputs | `MainState.generate` builds `BuildWorldOptions` from the map's features, then appends the scenario's. There is no third, per-site list. | `internal/game/main_state.go:578` |
| Scenario setup | Setup scripts get `start_x/y/z`, settlement name and faction through `level.Flags`, and `get_flag`/`set_flag` exist for scripts. Threat placement is decided inside each script. | `internal/game/setup_script.go:143`, `:388` |
| Fixtures | `Location.Fixtures` materialise once per site (`Spawned` flag). Positions are scattered around the plaza by a hash; datapads are spread about ±120 tiles. A fixture can run a structure script. | `spawnLocationFixtures`, `world_manager.go:1057` |
| Datapads | `DatapadComponent` holds only a `QuestID`. Collecting one reveals that quest. | `internal/components/DatapadComponent.go`, `main_state.go:2414` |
| Stamps | The `stamp` feature runs a structure script at a scored open spot. It passes no parameters beyond the footprint. | `placeStamp` in `internal/generation/feature_placers.go` |
| Clock | `Campaign.Day` is only a copy of the live level's `Day`, written on save. Each level's day advances only while it is live. | `world_manager.go:1455`; enemy factions plan, finding 2 |
| Save versioning | `CampaignVersion = 1`, stamped at generation. | `internal/campaign/campaign.go:15` |
| Stable IDs | Entities have stable IDs that survive save and load. | `internal/components/StableIDComponent.go` |

## Data model

All of this lives on `Campaign`, saved in the campaign file:

```go
type History struct {
    Civs     map[string]*Civ      // alien civilizations, alive, gone or fled
    Figures  map[string]*Figure   // named people and creatures, created on demand
    Sites    map[string]*SiteHistory
    Events   []Event              // append-only, ordered by ID
    Known    map[string]bool      // event IDs the player has discovered
    NextID   int
}

type Civ struct {
    ID       string
    Name     string
    Fate     string   // "extant", "extinct", "fled"
    Species  string   // blueprint family it maps to today, if any: "greyish", "mutant", ...
    Ancestor string   // Civ ID this one descends from, if any
}

type Event struct {
    ID      string
    Date    StarDate          // negative for prehistory, before the campaign began
    Type    string            // key into data/history/events.json
    Site    string            // SiteHistory ID, or "" for off-world events
    Civs    []string
    Figures []string
    Causes  []string          // event IDs this one follows from
    Origin  *SitePoint        // where on the planet it happened, if anywhere
    Vars    map[string]string // template fills: "pathogen", "weapon", ...
}

type SiteHistory struct {
    ID       string
    Location string     // campaign Location ID once charted; "" while unplaced
    Arc      string     // story shape this site was built from
    State    PlanetState
    Events   []string
}

type SitePoint struct{ X, Y int } // tile on the surface, as rolled; resolved on build
```

A site's map size is `md.Size.Roll(loc.Seed)` (`main_state.go:548`), so it is
known the moment the site is charted and origins can be tiles. The rolled tile
may still fall on a mountain or in the sea; generation resolves each origin
**once**, to the nearest open surface tile by the same scoring as stamps, and
every consumer (the stamp, the setup script, the datapad) uses that one
resolved tile, so "the hive grew from the wreck" is true on the map.

### Star date

A `StarDate` is a campaign-wide day count. `Campaign.StarDate` is the source of
truth; it advances one day per level-day of simulation of the **live** level
(background planets tick on the same clock and don't advance it separately),
plus the day cost of each jump. Levels keep their own `Day`/`Hour` only for the
day-night cycle; the HUD shows the star date. Prehistory dates are negative.
This is the single campaign clock both this plan and the enemy factions
plan depend on; whichever ships first builds it, and the other cites it.
**Open (2026-11-05):** the clock must count the world manager's ticks, not
the live level's days: since background simulation merged, the player is
often on the Star Map with no live level while away planets tick, and time
must pass then. Pausing the world clock pauses the star date.

### Planet state

Events change this state, and generation reads the final value:

| Field | Values |
|---|---|
| `life` | `none`, `native`, `alien` |
| `population` | `never`, `outpost`, `colony`, `civilization` |
| `status` | `untouched`, `thriving`, `fallen`, `dead` |
| `contaminants` | set of `plague`, `hive`, `nanites`, `radiation`, `blight` |
| `remains` | set of `ruins`, `wrecks`, `graves`, `relics`, `reactor` |

## The present comes from the map

Each map gains a `present` block giving the state its history must end in:

```json
"present": {
  "life": ["none"],
  "status": ["untouched", "dead"],
  "arcs": ["lifeless", "extinct_radiation", "failed_colony", "stellar_flare"],
  "quiet_weight": 4
},
"base_scenario": "moon"
```

Defaults for the current maps:

| Map | Present `life` | Example arcs |
|---|---|---|
| `moon` | none | lifeless; alive until a radiation leak; failed mining colony |
| `asteroid` | none | lifeless; mining claim; battle debris |
| `desert_planet` | none | always barren; living world burned by a stellar flare |
| `frozen_planet` | native | ice age buried a civilization; native life only |
| `earth_like` | native | untouched; collapsed civilization under regrown forest; thriving colony that fell |
| `alien` | alien | untouched alien biosphere; alien civilization ruins |
| `ocean_planet` | alien | drowned civilization; volcanic islands settled and abandoned |
| `abandoned_station` | none | station built, then lost (by event type) |
| `tutorial` | native | none; it stays authored and history skips it |

Home and the adrift Wreck Site have no map and get no site history.

## Story shapes (arcs)

Running events forward and hoping they land on the right map wastes rolls and
gives samey results. Instead each map lists the arcs that can lead to it, and an
arc is a short template of stages:

```json
{
  "id": "extinct_radiation",
  "stages": [
    { "pick": ["native_life_emerges", "civ_settles"] },
    { "pick": ["civ_thrives", "civ_rises"], "optional": true },
    { "pick": ["reactor_built"] },
    { "pick": ["reactor_meltdown", "nuclear_war"] }
  ],
  "ends": { "life": "none", "status": "dead" }
}
```

The generator walks the stages, picks event types whose requirements hold,
applies their effects, and checks the result against the map's `present`. Dates
are spaced backwards from the campaign's start. Figures are created when an
event needs them. Civilizations come from a pool rolled once per campaign,
**1 to 10 of them**, and the generator prefers reusing one over inventing
another, so the same war or empire turns up on several sites and the chronicle
connects rather than reading as unrelated incidents. A civ's `Fate` is decided
by its events: wiped out on its last world, or fled when a world dies with
survivors leaving.

**25% of sites are truly lifeless**: they draw the `lifeless` arc, whose only
events are visitors (surveys, claims, wrecks). The rest start with life or a
population, and some of those end lifeless through an extinction event.

## Event types

`data/history/events.json`:

```json
{
  "id": "reactor_meltdown",
  "requires": { "remains": ["reactor"], "life": ["native", "alien"] },
  "effects": { "life": "none", "status": "dead", "add_contaminants": ["radiation"] },
  "figures": [{ "role": "chief_engineer", "fate": "died" }],
  "origin": true,
  "scenario": "crystalline_blight",
  "weight": { "moon": 3, "earth_like": 1 },
  "evidence": [
    { "kind": "stamp", "params": { "script": "abandoned_reactor", "w": 15, "h": 15 }, "at": "origin" },
    { "kind": "radiation_pocket", "count": 3, "count_max": 6, "params": { "peak": 200, "radius": 5 } },
    { "kind": "lore_datapad", "at": "origin" }
  ],
  "text": "{date}: the {civ} reactor on {site} failed under {chief_engineer}. Nothing survived the fallout."
}
```

- `requires` and `effects` read and write planet state.
- `origin: true` gives the event a `SitePoint`, rolled from the site's stream.
- `evidence` is generation input. It uses existing feature kinds plus two new
  ones: `lore_datapad` and an `at` anchor for any feature.
- `text` is a template with fills from the event's figures, civs and `Vars`.

### Whole-planet events

| Event | Effects | Evidence |
|---|---|---|
| `stellar_flare` | life → none, status → dead | Scorched surface already on the map; ruins if populated |
| `nuclear_war` | life → none, status → dead, radiation | Ruins, radiation pockets, bunker structures |
| `reactor_meltdown` | as above | Reactor ruin at origin, radiation |
| `impact` | status → dead or fallen | Crater field, buried wreckage |
| `nanite_conversion` | contaminants + nanites | Wires, Autonz structures |
| `hive_consumption` | life → alien, contaminants + hive | Hive at origin, empty ruins |
| `ice_age` | status → fallen | Buried ruins below the surface |

## Threat origins

A threat event carries `scenario`: the gameplay package it plays as. The
scenario file stays what it is today (setup scripts, spawn rules, lighting,
`hostile_max`); it just stops being rolled. `weight` is per map and is where the
balance that used to live in the archetype scenario lists now lives: it says how
often this chapter ends a history on that map. An event with no weight for a map
can't happen there. The quiet outcome is weighted the same way, per map, so
lifeless sites don't all get a threat bolted on.

Seen from the scenario side, these are the events that can start each one:

| Scenario | Origin events | Requires |
|---|---|---|
| `zombie_plague` | `colony_plague`, `lab_leak`, `crash_pathogen`, `wreck_pathogen` | Someone died there: a fallen colony, a crashed crew, or an ancient wreck's crew |
| `crystalline_blight` | `blight_spread` | Alien life and crystals |
| Mutants (in `alien_world`, `abandoned_station` and `crystalline_blight`) | `reactor_meltdown`, `blight_spread`, `station_experiment`, `wreck_reactor_leak` | Life to mutate, or a crew or population |
| `crashed_ufo` | `ship_shot_down`, `fled_battle`, `scout_accident` | An alien civilization in the shared pool |
| `grey_contact` | `salvage_expedition` | An earlier wreck or station |
| `exterminate_xenos` | `meteor_seeding`, `infested_colonists`, `deliberate_drop`, `wreck_eggs_hatch` | Usually a hive source world, or an ancient wreck carrying eggs |
| `troid_outbreak` | `impact_colony` | An impact event |
| `autonz_incursion` | `swarm_landing`, `wreck_nanites_wake` | A swarm spread path, or an ancient wreck with a dormant swarm aboard |
| `worms`, `yeti_territory` | none: native | Native life; yeti are an ancient native civilization |
| `earth_like`, `moon`, `asteroid_field`, `abandoned_station` | none: the map's `base_scenario` | — |

The threat chapter is the last event in the site's history, dated recently, and
its `Origin` is where the threat started on the map.

`supported_maps` on a scenario stays as a hard gate but becomes a validation
check rather than a selector: no event may carry a scenario onto a map that
scenario doesn't support. The `scenarios` lists in `location_templates.json`
are retired; archetypes keep their `maps`, `tags`, weights and `start_ok`.

### Ancient wrecks as a source

`ancient_wreck` is an ordinary history event: a ship of a named civ came down
long ago, dated well before the present, with `origin: true` and the
`crashed_ufo` stamp as evidence but no survivors or spawn rules. On its own it
is a lifeless planet's visitor story, or a relic on a living one.

It is also a cause. The four `wreck_*` origins above each require an
`ancient_wreck` on the site, list it in `Causes`, and reuse its `SitePoint` as
their own origin, so the threat is densest around the hull: what it carried
decides the threat.

| Origin | What the wreck carried | Threat |
|---|---|---|
| `wreck_pathogen` | Its dead crew | Zombies rising from the crash site |
| `wreck_reactor_leak` | A leaking drive | Mutated wildlife, radiation around the hull |
| `wreck_eggs_hatch` | Xeno eggs in the hold | A hive growing out of the wreck |
| `wreck_nanites_wake` | A dormant Autonz swarm | Wires spreading from the hull |

A wreck that becomes a source needs a trigger event in the recent past (a
survey team opened it, a quake cracked the hull, the reactor finally failed),
which gives the wreck's datapad its second chapter. The same wreck can't be
two sources; the first `wreck_*` origin claims it.

## Charting a site

`newLocation` changes to:

1. Pick the map from the archetype, as today.
2. Bind a site history: reuse an unplaced site whose arc fits the map, or build
   one from the map's `arcs`.
3. End it with a chapter, chosen by per-map `weight` among the threat events
   whose requirements the history meets, plus the quiet outcome. **The
   scenario is that event's `scenario`**, or the map's `base_scenario` for the
   quiet outcome.
4. Set `Name` and `Summary` from the history when it names the planet.

A caller can ask for a chapter. Contract quest templates today chart a location
by `spawn_archetype` and expect a kind of threat there (a "Distress Call" wants
a hive); they gain `spawn_event`, and the binder builds an arc that ends in
that event. Cross-site spread uses the same door: a plague on a neighbour can
seed `plague_arrives` here, which brings `zombie_plague` with it.

All history randomness draws from its own stream, for example
`rand.New(rand.NewSource(c.Seed ^ historySalt + int64(GenSeq)*K))`, so adding
history does not shift the existing campaign rolls. The old scenario roll is
gone rather than moved, so a given seed's map, position and name stay the same
and only its scenario differs: acceptable, since every campaign after this
change is a new one.

## Generation

`SettlementConfig` gains `SiteFeatures []mapdef.FeatureBlock`, filled from the
site history's evidence by `buildParked`. `MainState.generate` appends them after
the scenario's features, as a third list, so generation still knows nothing
about history.

- **Origins are resolved first.** Before any feature is placed, each origin
  `SitePoint` is resolved to the nearest open surface tile and stored on the
  level. `at: "origin"` features use that tile; features without `at` behave
  as today.
- **Lore feeds the setup scripts.** The resolved origins and the site's
  history go into `level.Flags` before setup scripts run: `origin_x`,
  `origin_y`, `origin_event`, `origin_civ`, and what the history left there
  (`has_wreck`, `has_reactor`, `population`). Scripts grow the threat from the
  origin and name things after the history instead of rolling their own.
  Threat placement moves out of the scripts' own dice and into these
  parameters, one scenario at a time; a script that ignores them behaves as
  today.
- **Stamp parameters:** `placeStamp` gains pass-through `params` so a structure
  can name what it marks (a grave's occupant, a ship's registry).
- **`lore_datapad`:** a datapad carrying an event ID instead of a quest ID.
  `DatapadComponent` gains `EventID`; `collectDatapads` marks the event known
  and posts its text.

Generation stays deterministic: evidence is fixed when the site is charted, and
placement uses the existing seeded build rng.

## Live history

A recorder in `internal/game` subscribes to the per-level event buses and the
campaign, and appends events for a short, strict list:

- A colony founded or abandoned, and a total wipe at a site.
- A named colonist or a boss dying, with cause and killer.
- A quest completed, and a contract taken.
- A threat cleared, such as every xeno egg destroyed.

A site the player abandons or loses keeps its history. If it is visited again
before its save is discarded, it loads as saved; if it is ever rebuilt from
seed (later, with factions), its own history is the explanation.

Live events are dated in the star date, which this plan introduces in step 1.

## The player's view

- **Known events:** only events in `History.Known` are shown. Datapads, scans,
  reaching an origin point and some quest completions reveal events.
- **Datapads are the main way in.** Every history event with an origin leaves
  a lore datapad there, and the quest datapads that already exist gain the
  event behind their quest, so picking one up always tells part of the story.
  Later, datapads can carry mechanical rewards tied to the event: a civ's tech,
  a cache's location.
- **Star Map:** a known site shows its history's summary line instead of the
  archetype sentence.
- **Legends screen:** a chronicle of known events, filterable by site, civ and
  figure. It can be gated behind research like the Encyclopedia.
- **Quests:** templates gain history fills, such as finding the chief
  engineer's log or recovering a relic of a named civ. This is the strongest
  payoff and comes after the rest works.

## Saves

- `CampaignVersion` goes to 2. Version 1 campaigns are not migrated: nobody is
  playing yet, so loading one is refused with a message rather than carrying
  the old scenario rolls forward.
- **Size:** prehistory is a few hundred events and live play adds a few per
  site, which is well within the campaign file.

## Order of work

1. **Model and prehistory, with no gameplay change.** The star date, history
   types, arcs, event types, the generator, and a headless command
   (`cmd/history-dump`) that prints a campaign's chronicle. This is where to
   judge whether the stories are interesting and whether civs recur enough.
2. **Sites, visibly.** Bind histories in `newLocation`, scenarios taken from
   the chapter (and `spawn_event` on contracts), `SiteFeatures`, origin
   resolution and `at: "origin"`, lore
   flags for setup scripts, lore datapads, history-driven names and the Star
   Map summary line. **One script, `zombie_plague`, is converted here** as the
   prototype for growing a threat from its origin, so the first playable
   version already explains itself and proves the script route works.
3. **Threat origins in the other scripts**, one scenario at a time.
4. **Live history and Legends.** The recorder, known events and the Legends
   screen.
5. **Quests from history.**
6. **Deferred to the enemy factions work.** **A living world**, shared with the enemy factions plan: plagues and hives
   spreading between sites, civs acting between jumps.

## Tests

- Same seed gives the same history and the same sites, including after save and
  load.
- Every arc listed by a map can end in that map's `present` state, for every
  map.
- Every scenario is reachable: at least one event carries it, with a weight on
  at least one map that supports it. No event carries a scenario onto a map
  outside its `supported_maps`.
- Every `spawn_event` named by a quest template exists and can end an arc on at
  least one map of its archetype.
- The dump tool reports scenario frequency per map across many seeds, and no
  map's distribution has a scenario at zero that the old archetype lists
  allowed, unless that was decided.
- Every event's evidence names features and structure scripts that exist.
- Lifeless arcs produce no civs, figures or population events on that site.
- A version 1 campaign is refused on load with a clear message.
- ~~Adding history doesn't change a seed's maps, positions or names.~~ Not
  true as written (2026-11-05): the scenario pick is the last of three draws
  per site from the shared campaign rng, so dropping it shifts every later
  site. Either keep a discard draw in its place or accept the shift (saves
  break anyway); if accepted, this test goes.
- Across many seeds, the lifeless share is near 25% and every civ count from
  1 to 10 occurs.
- The star date advances with the live level and with jumps, and a planet
  frozen and revisited shows the campaign's date, not its own.

## Decisions (answered 2026-11-03)

1. **Setting.** All alien. Humans are new here, so no human colonies appear in
   the prehistory. Civs are extant, extinct or fled, and some are ancestors of
   today's creatures. Civs that still exist can map onto in-game species
   (greys, yeti, Autonz); ones that don't need not exist in the game at all.
2. **History across campaigns.** Not in v1. IDs are chosen so a later universe
   file could carry a failed expedition's ruins into the next campaign.
3. **Mechanical effects.** Flavour, site content and quests first. Datapads
   are the carrier for both the story and, later, any mechanical reward.
4. **Lifeless share.** 25% truly lifeless through all of history; others may
   become lifeless by event.
5. **History length.** A few hundred years; the number of civs is rolled 1–10
   per campaign.
6. **Saves.** Breaking them is fine; no migration.

## Unverified

- How far each setup script has to change to take its threat placement from
  the lore flags. `zombie_plague` is the prototype in step 2; read the others
  before step 3.
- Whether the per-level event buses carry enough to record deaths with a cause
  and killer.
- ~~How the enemy factions plan's campaign tier should share `Civs` with
  history.~~ **Decided 2026-11-05: history and factions are independent
  layers.** History is always generated and always picks the site's chapter
  and scenario. An enemy faction is *placed* on a planet exactly as the
  player's party is, on top of whatever the history left there, and has to
  deal with the live scenario (zombies, blight, native hostiles) like the
  player would. A presence never replaces or rewrites a chapter. Consequences
  for the factions plan: Tier 1's abstract step needs a per-scenario hazard
  factor (a presence on a plague world grows slower or dies off), Tier 2
  materialises the chapter's evidence and threat first and the presence
  second, and the live-history recorder logs faction arrivals, departures and
  wipes as ordinary events so the Legends screen can tell them. History
  `Civs` and faction doctrines stay separate too; a history civ that maps to
  a playable species (`Civ.Species`) is flavour, not a presence.

## Implementation notes

Built 2026-11-06/07 on `claude/lore-history-4d1fe2`. Where the code differs
from the plan above:

- **Package.** The model, generator and renderer live in `internal/history`,
  below `campaign` (campaign imports history, never the reverse).
- **Uninhabited, not lifeless.** Maps whose present needs life (earth_like,
  frozen, alien, ocean) can't be "truly lifeless", so the 25% roll picks an
  *uninhabited* arc: `lifeless` on maps that allow no life, `pristine`,
  `pristine_alien` or `drone_station` elsewhere. Uninhabited arcs name pool
  civs only as visitors; no resident, figure or population.
- **reactor_meltdown split.** The plan's example both killed all life and
  carried `crystalline_blight` (alien maps only). `reactor_meltdown` is a
  whole-planet arc event; a new chapter, `reactor_mutation`, carries
  `alien_world`.
- **Desert and frozen quiet scenarios.** No empty scenario exists for them,
  so their `base_scenario` is `worms` and `yeti_territory`.
- **Home keeps its scenario list.** Removing it would drop a random draw and
  shift every later site; archetypes lost theirs, with a discard draw in
  place of the old scenario roll so a seed's maps, positions and names are
  unchanged (golden test).
- **Names never change.** No event names a planet, so charting keeps the
  rolled name; the Star Map summary is the arc's summary plus the chapter's
  omen.
- **Archetype spawn_event.** Retiring the archetype lists would have stopped
  hive worlds having xenos; archetypes (and contracts) can name a
  `spawn_event`, and `hive` ends in `infested_colonists`.
- **Safe first jump.** Sites within `safe_start_radius` of the Wreck Site end
  only in a `quiet_safe` quiet or a `safe_start` chapter (only
  `scout_accident` so far).
- **Origins.** Resolved after terrain and biome painting, before any feature
  (trees included); on a station, to the nearest walkable floor. Stored in
  the level's saved flags and tagged as region `lore_origin`.
- **Wrecks are old hulls.** History wreck stamps pass `crew: "dead"`; before
  that every ancient wreck had live greys. `grey_contact`'s script called a
  function that did not exist and never spawned anything on `dev`; fixed.
- **Death events via EntityDamagedEvent.** `EntityKilledEvent` only fires for
  kills by colonists and carries names only; the recorder remembers each
  victim's last attacker from `EntityDamagedEvent` and records from the
  cleanup death hook. No event was extended.
- **Threat cleared** is recorded only for finite threats (scenario
  `threat_blueprints`: xenos, troids); spawners like zombies never clear.
- **Place nouns.** Events carry a `place` ("the vault", "the reactor") for
  quest lines.
- **Quest datapads** carry the site's story event (its chapter, else its
  latest event): quests are tag-rolled and have no event of their own.
- **History quests must be accepted first.** They are offered like any quest;
  if their event is already known when accepted, they complete on the next
  quest check.

