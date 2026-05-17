# Scenarios

A scenario sets the world type, starting equipment, hostile pressure, and win/loss conditions. Scenarios are no longer chosen from a menu — each **Star Map location** is bound to a map + scenario (see `data/overworld.json` and [The Ship & the Star Map](star_map.md)). Travel to a location to play its scenario; the descriptions below cover the scenarios the default overworld uses.

---

## Survival

A hostile alien world. You start with a small colony pod, walls, an airlock, a storage locker, and a basic workbench. Aliens spawn from outside the perimeter and probe for weaknesses.

- **Lighting:** Day/night cycle.
- **Tone:** Defensive — fortify, stockpile food, hold the line.

## Earth-like Colony

An Earth-like world of mixed biomes — forests, plains, deserts, and tundra. Local fauna is mostly ambient critters; pressure is much lower than Survival. Ore and crystal veins are scattered across the map; deserts hide radiation pockets you'll want to avoid.

- **Lighting:** Day/night cycle.
- **Tone:** Exploration and expansion.

## Alien World

A dedicated alien biome map with strange flora and persistent hostile pressure. Expect denser alien spawns and more biomass-based encounters than Earth-like.

## Moon

A barren regolith surface with low humidity, no atmosphere flora, and crater terrain. Resources are largely subterranean — plan to dig.

## Asteroid Field

A small rocky body with a vacuum atmosphere. Confined geography forces tight base layouts and aggressive interior expansion.

## Abandoned Station

A pre-built station interior with **pitch-dark** lighting — no ambient sun. You depend entirely on placed work lights and colonist-equipped lights to see anything. Tight, claustrophobic, hostile-on-arrival.

---

## What Each Scenario Decides

When you pick a scenario, it sets:

- **Terrain template** — open planet, asteroid, station interior.
- **Biome map** — which biomes can appear and how they are distributed.
- **Starting kit** — placed via a setup script: walls, doors, storage, workbench, initial colonists.
- **Spawn rules** — what hostile and ambient creatures show up, how often, and on which tiles.
- **World features** — ore veins, crystal veins, radiation pockets, scatter flora, station prefabs.
- **Lighting mode** — day/night, fixed, or pitch-dark.

Scenarios no longer carry win/lose rules — objectives come from
[quests](star_map.md#quests) accepted at the ship.

Restarting the same scenario gives you a freshly generated map but the same rules.
