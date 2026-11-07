# Lore and History

Every campaign has a generated history of alien civilizations, and every
charted site's history explains what is there: the map is the present, the
threat scenario is the latest chapter, and the evidence stands on the map
where it happened. Play adds to the same history. Design and decisions:
[lore_plan.md](../brainstorm/lore_plan.md) (see its Implementation notes).

## Packages

| Where | What |
|---|---|
| `internal/history` | The model (`History`, `Civ`, `Figure`, `Event`, `SiteHistory`, `SitePoint`, `PlanetState`, `StarDate`), the data library, the generator (`BindSite`, `BindSiteEnding`, `BindSiteSafe`), rendering (`Render`, `Summary`), evidence and lore flags. Below `campaign`: it imports `mapdef`, never `campaign`. |
| `internal/campaign` | `Campaign.History`, `Campaign.StarDate`; charting binds a history in `newLocation` (`bindHistory`); history quests (`history_quests.go`); `RecordLive`, `RevealEvent`. |
| `internal/generation` | Origins (`ResolveOrigins`, `Origin`, the `at` anchor on features), lore flags set on the level before features run. |
| `internal/game` | Site lore into the build (`siteLoreFor`, `lore_site.go`), the recorder (`lore_recorder.go`), the Legends tab (`lore_legends_panel.go`). |
| `cmd/history-dump` | Headless chronicles and frequency tables. |

## The star date

`Campaign.StarDate` (days) and `StarTick` are the one campaign clock. The
world manager ticks it once per world frame in `TickBackground`, so it runs
with a live level and with the dashboard open, and stops whenever the world
clock is stopped. A jump adds `ceil(fuel / jump_fuel_per_day)` days
(`data/generation.json`). Prehistory is dated in negative star dates
("N years before landfall"); levels keep their own `Day`/`Hour` only for the
day-night cycle.

## Data

All under `data/history/`, loaded by `history.DefaultLibrary()` (cached; it
reads maps from `data/maps` itself). The text in them is draft.

**`arcs.json`** — story shapes. `id`, `uninhabited` (no one ever lived there;
a quarter of sites draw one), `stages` (each `pick`s one eligible event type,
`optional` stages may be skipped), `ends` (the state the arc must finish in),
`summary` (the Star Map line; `{civ}` is the site's civ).

**`events.json`** — event types.

| Field | Meaning |
|---|---|
| `requires` | Planet state (`life`, `population`, `status` any-of; `remains`, `contaminants` all-of), `events` / `any_events` already on the site, `resident` true/false |
| `effects` | `life`, `population`, `status`, `add_contaminants`, `remove_contaminants`, `add_remains`, `ruins_if_populated`, `resident_fate` (`died`, `fled`, `dwindled`) |
| `civ` | Who acts: `resident`, `arrive` (a pool civ settles), `native` (a new civ arises), `visitor`, `war` (`{civ}` and `{enemy}`) |
| `species` | For `native`, what the civ became; for pool roles, a preference |
| `figures` | `[{role, fate}]`, created on demand (a living figure of the same role and civ on the site is reused) |
| `origin` / `origin_from` | The event has a place on the map: rolled, or reused (and claimed) from an earlier event of the listed types |
| `scenario`, `weight` | Makes it a chapter: the scenario it plays as, and how often it ends a history, per map |
| `safe_start` | The chapter may end a first-jump site |
| `prelude` | Event types that happen just before the chapter (a wreck disturbed) |
| `flags` | Extra lore flags for the site's build (`chapter_crew: "dead"`) |
| `gap_days` | Time from this event to the next (or to the present) |
| `vars` | Random template fills |
| `evidence` | Feature blocks the event leaves; `"at": "origin"` anchors one at its origin (anchored counts are absolute, never scaled by map area); `lore_datapad` is implied by any origin; every stamp passes `occupants` (`"none"` for ruins: see structure_scripts.md) |
| `place` | Where its trace is, for quest lines ("the vault") |
| `omen` | A chapter's Star Map line |
| `live`, `fills` | A play event the recorder writes, and the fills it sets |
| `text` | The chronicle template: `{date}`, `{site}`, `{civ}`, `{enemy}`, `{during_war}`, `{war_end}`, figure roles, vars, fills |

**`civs.json`** — the civ pool: size range (1–10), name and figure syllables,
species weights, ancestor and war chances, `lifeless_percent`, `reuse`
(preference for civs already in the chronicle).

**Maps** (`data/maps/*.json`) carry `present` (`life`, `status`, `arcs`,
`quiet_weight`, `quiet_safe`) and `base_scenario` (what a quiet site plays).
**Archetypes** and **contract templates** may name a `spawn_event`.
**Scenarios** may list `threat_blueprints` and `threat_name` (what "threat
cleared" means).

Validation tests (`internal/history/lore_*_test.go`) check references,
template fills, evidence kinds and structure scripts, anchorable kinds,
scenario support and coverage.

## Determinism

- History randomness never touches the campaign's rng: the civ pool draws from
  `Seed ^ historySalt`, each site from that plus its site seed, history quests
  from the site seed with their own salt.
- Charting keeps a discard draw where the scenario roll was, and history
  quests take location-based IDs, so a seed's maps, positions and names are
  unchanged (`TestLoreHistoryKeepsSeedsMapsPositionsNames`, golden file).
- A site's history depends on the campaign seed, the order sites are bound and
  its own seed; the same seed charts the same sites and histories, including
  after save and load.

## Lore flags

Set on the level before any feature (so structure scripts see them) and again
before setup scripts:

| Flag | Value |
|---|---|
| `origin_x`, `origin_y`, `origin_z` | The chapter's resolved origin; -1 without one |
| `origin_event` | The chapter's event type, `""` if quiet |
| `origin_civ` | The chapter's civ name |
| `has_wreck`, `has_reactor` | 1 when the history left a wreck or reactor |
| `population` | `never`, `outpost`, `colony`, `civilization` |
| `origin:<event>:x/y/z` | Every event's resolved origin (`generation.Origin`) |
| chapter `flags` | e.g. `chapter_crew` |

Scripts read them with `get_flag` or `get_flag_or(key, default)`; helpers
`find_open_tile_near(x, y, z, r)` and `find_cave_tile_near(x, y, r)` grow a
threat from a point. Every scenario setup script uses the origin when there is
one and keeps its old behaviour without (tested against frozen copies in
`internal/game/testdata/lore_old_scripts`).

## Live history and what the player knows

`History.Known` is all the player sees. Events become known by:

- picking up a lore datapad (one at every event origin; quest datapads carry
  the site's story event);
- a ship scan charting a site (its story event);
- a colonist standing within 2 tiles of an event's origin;
- completing a quest (the event behind it).

The recorder (`loreRecorder`, one per level) writes live events, known at
once and dated in the star date: `colonist_died` and `boss_slain` (killer from
the last `EntityDamagedEvent`), `colony_founded`, `colony_left`,
`colony_wiped`, `quest_completed`, `contract_taken`, `threat_cleared`.
Player-facing text uses display names (the expedition's colony, quest titles
without their site suffix).

History quests (`figure_log`, `civ_relic`, `threat_source` in
`data/quest_templates.json`, `history` field) bind at charting and complete
when their event is known (`flag_set` on `known:<event>`).

## Tools

```bash
make history-dump HSEED=42 HSITES=8
```
```bash
make history-dump HSEED=729 HCHARTED=1 HFREQ=0
```
```bash
make history-dump HCAMPAIGNS=1000 HFREQ=0
```

The first prints a standalone chronicle, the second a campaign charted the way
the game charts it, the third the scenario share per map through real
charting. The planet generator shows a site's history and, with "Scenario from
history" on, builds the site the way the game does.

vmtest scripts: `lore_site` (Star Map summary), `lore_xenos`, `lore_ufo`,
`lore_autonz` (structures at the origin, via `debug origin walk`) and
`lore_legends` (a datapad's event in the Legends tab, via `debug datapad`).
See [vmtest.md](vmtest.md).
