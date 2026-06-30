# The Ship

The Ship is the campaign's home base, reframed from an abstract roster/hold into
a **real, always-loaded level** that simulates in the background. It's where
every colonist who isn't currently beamed down physically lives, and it's the
first concrete consumer of the background-simulation architecture
(see [background_simulation.md](background_simulation.md)).

Status: **designed, not built.**

## Concept

- The Ship is a small fixed **10×10** level built from the abandoned-station
  tiles, with a single storage container to start.
- Every colonist not beamed down exists as a **real entity on the ship level**.
- It is **always loaded and always ticking** — in the foreground when you visit
  it, in the background (logic only, no rendering) otherwise. It is never frozen
  to disk while a campaign is active.
- Reached from the **Star Map via its own "Ship" button**.
- Beaming works exactly as today: open the Star Map, pick a colonist from the
  list, click **Beam Down** / **Beam Up** — but that now transfers an entity
  between the ship level and the orbited planet level.

## Why this reuses the background-sim work

| Piece | How the Ship uses it |
|-------|----------------------|
| Phase 1 — per-level event bus (`level.Events`, done) | The ship fires/consumes its own sim events, isolated from whatever planet is live. Free. |
| Phase 2 Part A — `stepBackground` | The ship **is** the first always-background ticker. Building it builds that path. |
| Phase 2 Part B — multi-loaded WorldManager | Contained to a **dedicated `shipLevel` slot + the current planet** = 2 levels, not the full `loaded map[string]` rewrite. A clean stepping stone to generalizing later. |

## The reframe: abstract `ShipState` → real `*world.Level`

Today `campaign.ShipState` is abstract — `Hold []*SaveEntity` + `Roster
[]*SaveEntity` with lazy `LiveHold()`/`Sync()`/`RebuildLiveEntity` round-tripping.
The Ship replaces that with a level:

| Today (abstract) | The Ship (real level) |
|---|---|
| `Roster []*SaveEntity` | colonist **entities on the ship level** |
| `Hold []*SaveEntity` | a **storage container entity** on the ship level |
| `storage.ShipProvider` scans `Hold` | scans the ship level's storage entities |
| bespoke ship serialization | the ship saves/loads like any level |

"Storage linked to the containers on the ship" falls out for free — the hold *is*
the container on the map. Beam-down/up become plain **entity transfer between two
live levels**.

## Decisions (settled)

1. **Layout** — fixed hand-authored 10×10 station template for v1 (reuses station
   tiles; not the `abandoned_station` *generator*, which makes 120–160-tile maps).
2. **What ticks** — **everything**, including `ScriptedAI`. The long-term vision
   is colonists keeping **"terrariums"** — captured creatures / found things they
   rebuild from mined material — so the ship must run the full AI stack, not a
   reduced subset. (Background ticks still skip only the rendering systems:
   Lighting, FOV, Emote, effects.)
3. **Needs** — colonists on the ship have needs (hunger, exhaustion) **just like
   on a planet**. This makes keeping the ship stocked with food a real logistics
   layer.
4. **Migration** — none. Nobody is playing yet, so the abstract `ShipState` can
   be replaced outright; no save converter needed.
5. **Capacity** — keep `RosterCap` as a numeric cap (limits bio-printer growth).
6. **Beaming** — unchanged flow: Star Map → pick colonist from the roster list →
   Beam Down / Beam Up. The roster list is now "colonists currently on the ship
   level." Beam-down targets the **currently orbited location's** planet, which
   loads/enters on beam-down while the ship keeps ticking.

## Architecture

- **`shipLevel` slot.** `WorldManager` gains a dedicated always-loaded ship
  `MainState` (separate from `Locations`, which are star-map planets). It is
  never frozen while the campaign is active and is background-ticked whenever it
  isn't the live level.
- **Template.** A fixed 10×10 station layout (a stamp / structure template) with
  one storage container. Built once when a campaign is seeded.
- **Seeding.** `SeedNewCampaign` stops stocking abstract `Hold`/`Roster` and
  instead: build the ship level from the template, spawn the starting colonists
  as entities on it, place a storage container stocked with starting resources.
- **Storage.** `ShipProvider` reads the ship level's storage entities (owned by
  `ShipSettlementName = "ship"`), so crafting/quests still see the campaign-wide
  stockpile via the existing `MultiProvider{Level, Ship}`.
- **Star-Map "Ship" button.** `OverworldState` gets a button that enters the ship
  level (foreground) like landing on a planet; leaving returns to the Star Map,
  ship stays loaded + ticking.
- **Beam transfer.** `BeamDown(rosterIdx)` moves a colonist entity from the ship
  level → orbited planet level; `BeamUp(e)` moves planet → ship. Both levels are
  loaded simultaneously (ship always, planet when landed).
- **Background tick.** The ship runs `stepBackground` (Phase 2 Part A) — full
  logic/AI/needs stack, skipping rendering — whenever it isn't the live level.
  Its scenario has no hostile spawns, so `gm.Update()` is a no-op there.
- **Win/lose.** The total-wipe loss check must now count **ship-level colonists**
  (they were the `Roster` before).

## Build order

1. **Ship template + level** — fixed 10×10 station stamp with one storage
   container; build it headless and verify it generates.
2. **`shipLevel` slot on WorldManager** — always loaded, never frozen.
3. **Reseat Hold/Roster onto it** — colonists as entities, hold as the container;
   repoint `ShipProvider`; update `SeedNewCampaign`; fix the wipe check.
4. **Star-Map "Ship" button** — enter/leave the ship level.
5. **Beam-down/up as entity transfer** between ship and orbited planet.
6. **`stepBackground` for the ship** + tick it whenever it isn't live.

Steps 1–5 give a visitable, persistent ship. Step 6 makes it *live* in the
background — and is the reusable Phase 2 Part A machinery.

## Scenario interaction (decided)

The scenario stays a **global singleton** (= the currently-orbited location's
scenario); it is deliberately **not** isolated per-level. This is a feature hook:
a location's scenario can have **ship side-effects** (boarding parties,
contamination leaking up, derelict encounters) since the ship reads the same
active scenario.

Consequences:
- The ship's **lighting is intrinsic**, not scenario-derived — it's an enclosed
  powered hull, so `buildShip` sets `LightingMode: "fixed"`, `LightingAmbient:
  100` via `SettlementConfig`. `applyScenarioLighting` was guarded with
  `scenario.HasActive()` because the ship is built at campaign start, before any
  scenario is selected (this was a startup panic).
- **Gotcha for the background-tick step:** `gm.Update` spawns hostiles up to the
  active scenario's `HostileMax`. With a shared scenario, once the ship ticks in
  the background it would spawn the *orbited planet's* hostiles **on the ship**.
  So ship hostile-spawning must be **gated off** until a scenario explicitly
  opts in via a future `ship_effects` block — otherwise orbiting an infested
  planet dumps enemies onto your ship. (Not an issue yet: the ship is parked,
  not ticking.)

## Future

- **Scenario `ship_effects`** — opt-in block letting a location's scenario drive
  events on the ship (boarders, contamination, supply finds). The motivation for
  keeping the scenario global rather than per-level.

- **Terrariums** — captured creatures and found things, housed on the ship and
  rebuilt from mined material (the motivation for full `ScriptedAI` ticking).
- **Generalize** the `shipLevel`-slot pattern to N background planets (the
  original Phase 2 Part B), once the 2-level case is proven.
- **Ship expansion** beyond the fixed 10×10 (build out rooms, more storage,
  research/crafting stations aboard).
