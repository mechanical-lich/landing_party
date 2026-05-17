# The Ship & the Star Map

Landing Party is a campaign of expeditions. Your colonists begin aboard a
space life-raft. You do not start on a planet — you start **in space**, looking
at the Star Map, and decide where to make planetfall.

## The Ship (your hub)

The ship is an abstract hub — there is no walkable ship deck. It holds:

- **The hold** — a single, campaign-wide stockpile. Everything your colonists
  gather on a planet ends up here when you leave it ("all resources end up on
  the ship"). Fuel, ore, biomass, crystal, etc. are spent from the hold.
- **The roster** — colonists currently aboard, not deployed anywhere.

A new expedition starts with a fuel reserve and a small crew already aboard.

## The Star Map

Open it from the title screen ("New Expedition" / "Load Expedition") or, while
playing, with the **Star Map** button (top-right, next to Follow) or the **O**
key.

The Star Map shows:

- **Locations** — planets, moons, asteroid fields, stations. Each lists its
  travel fuel cost. Some locations are hidden until discovered.
- **On Ship** — the colonists in the roster.
- **On `<location>`** — the colonists currently on the loaded location.
- **Fuel** — what's in the hold.

### Travel vs. Resume

These are deliberately separate:

- **Travel Here** — moves the ship to the selected location. It loads (or
  generates) that location and marks it current, spending fuel if the ship
  actually relocated. It does **not** drop you into the level — you stay on the
  Star Map so you can organize your landing party first.
- **Resume / Land** — actually enters the loaded location and starts playing
  it. (`Esc` on the Star Map does the same once a location is loaded.)

A location you have already established is **paused**, not reset: travel back
and you resume it exactly as you left it — your base, mined tiles, dropped
items, and the colonists you left behind are all still there.

### Fuel cost is distance-based

Every location has a position on the star map. The fuel cost shown is the
distance from **where the ship currently is** to that destination:

- The location you are currently at costs **0**.
- Travelling elsewhere costs roughly the straight-line distance.
- Once you arrive, costs recompute relative to the new position — the place you
  just left now costs fuel to return to.

Make fuel by refining it: the **Refine Fuel Cell** recipe (basic workbench)
turns biomass + ore into fuel. Refined fuel is swept into the ship hold when
you leave the location, making it available for the next jump.

## Beaming colonists

Colonists move between the ship and the loaded location on the Star Map:

- Select a colonist in **On Ship** and click **Beam Down >** to send them to
  the location's landing zone.
- Select a colonist in **On `<location>`** and click **< Beam Up** to bring
  them back to the roster.
- While playing on a planet, select a colonist and press **B** to beam them up
  directly.

The whole landing party beams down to the same plaza (they spread out so they
never stack). Beam-up has one rule: a colonist must be **open to the sky** —
you cannot beam up through solid ground or a roof.

## A typical loop

1. Star Map → pick a destination → **Travel Here** (spends fuel if moving).
2. Beam down a landing party.
3. **Resume / Land** and play: gather, build, research, refine fuel.
4. **O** / Star Map button → beam survivors up (or leave a colony behind).
5. **Travel Here** to the next world, or back to a paused one.
6. **Save Expedition** any time from the Star Map.

## Quests

Quests are your objectives — there is no per-planet win or loss any more.
Open the **Quest Log** from the Star Map:

- **Offered** quests can be **accepted** (some auto-accept).
- **Active** quests track an objective: gather/stockpile a resource, research a
  tech, build a structure, clear out a hostile, survive N days, etc. Resource
  objectives count the campaign-wide ship hold, not just on-planet crates.
- Completing a quest grants its **reward** (fuel and/or resources into the ship
  hold) and can **reveal new locations** on the Star Map. Some quests unlock
  follow-up quests.

Active quests and a completed count also show in the in-game **Goals** tab.
Objectives are evaluated continuously while you play a location, so a quest
completes within moments of you meeting it.

## The journey home

Every expedition is **procedurally generated** from its seed. You start
stranded at a random system with a couple of nearby systems and **Home**
visible far across the map — its fuel cost is enormous because it's so distant.

There is no fixed length. Completing certain quests **charts new systems**
(closer to Home), and **every jump has a chance to turn up fresh contracts**
(for known systems or newly charted ones) — so simply keeping on the move
keeps work and progress flowing. The loop:

1. Work the systems you can reach — quests pay **fuel** and resources.
2. Hop outward; each system you reach is a new staging point, and the
   remaining jump to Home shrinks as you close the distance.
3. When you can finally afford the Home jump, **Travel** there to win.

Lose condition: a **total wipe** — no colonists left anywhere (none planetside,
none in the ship roster, none on any established system). Until the bio-printer
exists, the starting crew is all you get, so don't lose them all.

> Still planned: the bio-printer (growing new colonists from biomass) and
> deeper authored lore on top of the generator.
