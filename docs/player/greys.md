# Greys

The greys are few, well armed and expensive. They fly saucers between
worlds, land to dig, carry what they dig home to a mothership, and come
back out in bigger ships. They are hostile from the start.

See [Factions](factions.md) for how factions in general spread, show on the
Star Map and turn hostile.

---

## As a rival

### Ships and the mothership

A grey faction is a **mothership** and up to three **ships**.

| Ship | Crew | What it carries | Bought by the mothership for |
|---|---|---|---|
| Scout | 3 | Grey Vat, Grey Console | 60 metal ore, 20 crystal (a new ship) |
| Freighter | 5 | Grey Vat, Grey Console, Grey Armory once they know Armory Module | 40 metal ore, 15 crystal (upgrade) |
| Warship | 8 | as the freighter, plus two Field Emitters once they know Field Emitter | 90 metal ore, 40 crystal (upgrade) |
| Mothership | 10 | the hoard | — |

The rhythm:

1. A ship lands on a free world within 50 of where it is (in the same
   units as the Star Map's fuel cost), flying 5 a day.
2. It stays up to **30 days**, digging and printing crew.
3. When its hold reaches 30 units it flies home to the mothership and
   docks. What it dug goes into the hoard, and if the hoard can pay for
   the next ship class, the ship is upgraded. Then it sets out again.
4. When the hoard covers a new scout, the mothership builds one.

The mothership waits at the rim, at its faction's own arrival point.

### On the Star Map

Grey ships are drawn as saucers in their colour; the mothership is a larger
saucer with a ring, and a ship docked at it sits above it.

| Where | Tag |
|---|---|
| landed on this world | `GREY SCOUT · HOSTILE` |
| the mothership over this world | `GREY MOTHERSHIP IN ORBIT · HOSTILE` |
| heading here | `GREY FREIGHTER INBOUND · 3 DAYS` |
| left this world | `GREY SCOUT BOUND FOR EMBER PIT · 4 DAYS`, or `BOUND FOR ITS MOTHERSHIP` |

The Destinations list notes `[grey]` for a landed ship and
`[grey mothership]` for the mothership. Legends records landings,
departures, and ships docking "and came away a freighter".

### On the ground

A grey camp is their saucer, set down well away from your landing, with
its modules unpacked inside: the **Grey Vat** that prints every grey body
and the **Grey Console** they research at, and on bigger ships the **Grey
Armory** and **Field Emitters**. Around it they build more on the ground:
an armory, a **Research Array**, emitters (turrets that fire at anything
hostile in range).

Greys tunnel fast. Ore is often deep, and they dig down for it and put a
locker by their furthest mine. They hunt the local wildlife for
**specimens**.

While a grey camp is on a world you are playing, the mothership beams
metal, crystal and plastic down to it each day.

### When they come for you

Grey patrols range out from the camp. Once they have found you, they raid:
up to three soldiers, heavies and lancers, while your colonists are on
their world. Once they know Cloning, a raid only sets out with a heavy to
lead it.

### The storage pocket

Every locker a grey camp owns on a world is a door onto **one** shared
inventory. You can see what is inside, but you cannot beam anything up out
of a grey locker.

- Destroy one locker and what it held moves into the others.
- Destroy the **last** one and everything spills onto the ground round it.
- A day later the ship puts its cockpit locker back and the greys start
  hauling the spill home. Be quick.

### Gear

A dead grey's weapon or armour usually breaks. Now and then it drops
intact, and your colonists can use it. Gear spilled from the storage
pocket rolls the same chance.

| Item | Slot | Stats | Chance to drop intact |
|---|---|---|---|
| Beam Pistol | hand | ranged 1d8 +3, range 8, energy | 15% |
| Beam Rifle | hand, two-handed | ranged 2d6 +4, range 12, energy | 10% |
| Disruptor | hand, two-handed | ranged 3d8 +3, range 10, energy | 5% |
| Field Harness | torso | defence 6, stopping power 6 | 10% |
| Field Cowl | head | defence 3, stopping power 3 | 15% |

Heavies' tentacles and lancers' lances are part of their bodies and never
drop.

### Fighting them

- **Kill the whole crew** of a landed ship and it is wiped out. Its camp
  stays where it stood.
- **Board a ship** you meet in space, or the mothership: see
  [Meeting ships in space](factions.md#meeting-ships-in-space).
- A ship that lifts off leaves a **landing scar**: scorched ground where
  the hull stood, empty unowned lockers, and **Wrecked Armory**, **Wrecked
  Array** and **Wrecked Emitter** structures where theirs stood. Break a
  wreck for grey alloy and crystal. See
  [Dangers, Ruins and Relics](dangers_and_ruins.md#what-factions-leave-behind).

### The mothership

You meet the mothership by arriving at the world it is orbiting. Evading it
costs 8 fuel and fails 70% of the time. Board it and you fight its crew of
10 on a warship-sized deck; if the greys know **Mothership Bulwark**, two
emitters stand on the deck too. Its whole **hoard** is in a locker aboard.

Take the mothership and the greys stop growing: no ship is upgraded or
built again. The ships they still have keep flying at the size they are,
and any that were heading home scatter to the nearest free world.

---

## Units

Every grey is printed at the Grey Vat. Greys need no food and never sleep.
Only technicians research. Lower speed numbers act more often.

| Unit | Role | Cost | Work | Health | Str | Dex | Int | AC | Attack | Speed | Unlocked by | Special |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Worker | mines, digs, hauls, builds, crafts | 8 metal ore, 3 crystal, 1 grey alloy | 450 | 18 | 10 | 12 | 8 | 10 | 1d6 | 10 | start | digs and tunnels fast |
| Technician | researches, crafts, builds, hauls | 6 metal ore, 5 crystal, 2 grey alloy | 600 | 14 | 6 | 12 | 16 | 10 | 1d4 | 10 | start | the only researcher |
| Soldier | fights | 10 metal ore, 4 crystal, 3 grey alloy | 600 | 32 | 14 | 14 | 8 | 14 | 1d6 | 9 | start | armed from the armory |
| Heavy | fights, leads raids | 14 metal ore, 6 crystal, 5 grey alloy | 900 | 50 | 18 | 10 | 6 | 16 | 1d8 tentacles | 12 | Cloning | |
| Skimmer | flies; hauls, patrols | 6 metal ore, 4 crystal, 2 grey alloy, 1 specimen | 600 | 16 | 8 | 18 | 8 | 12 | 1d6 | 6 | Skimmer Pattern | |
| Lancer | flies; fights with its lance (2d6, range 8) | 12 metal ore, 8 crystal, 6 grey alloy, 2 specimen | 1200 | 30 | 12 | 16 | 8 | 14 | 1d6 | 8 | Lancer Pattern | posted at emitters |

A rival's soldiers carry beam pistols until the greys know Beam Weapons,
then beam rifles; heavies carry disruptors once they know Disruptor; both
wear field harnesses once they know Field Armour.

## Structures

| Structure | What it does | Cost | Work | Health | Needs |
|---|---|---|---|---|---|
| Grey Vat | Prints every grey body. | 12 metal ore, 6 crystal, 4 grey alloy | 60 | 40 | aboard the ship |
| Grey Console | Research station. | 10 metal ore, 6 crystal, 2 grey alloy | 50 | 40 | aboard the ship |
| Grey Armory | Workbench: refines alloy, presses plastic, makes beam weapons and field armour. | 10 metal ore, 4 crystal | 50 | 40 | — |
| Research Array | Research station for the advanced entries. | 6 metal ore, 10 plastic, 8 crystal, 4 grey alloy | 80 | 40 | Research Array; aboard the ship |
| Field Emitter | A turret: a beam (1d10 +2, range 8) at anything hostile. | 3 metal ore, 5 plastic, 6 crystal, 3 grey alloy | 60 | 40 | Field Emitter |
| Grey Hull Floor | Saucer deck plating. | 1 metal ore | 1 | — | — |
| Grey Hull Wall | Saucer rim. | 1 metal ore, 1 grey alloy | 2 | — | — |
| Storage Locker, Stairs | As for the colony. | see [Buildings](buildings.md) | | | — |

A rival grey ship never builds a vat or console; it carries them.

## Materials and recipes

| Material | Where it comes from |
|---|---|
| Grey Alloy | refined at the armory |
| Plastic | pressed from biomass at the armory; research, the array and emitters use it |
| Specimen | every kill a grey makes of a creature gives it one |

| Recipe | Makes | Cost | Work | Needs |
|---|---|---|---|---|
| Refine Alloy | 1 grey alloy | 3 metal ore, 2 crystal | 180 | — |
| Press Plastic | 2 plastic | 2 biomass | 120 | — |
| Beam Pistol | beam pistol | 2 crystal, 1 grey alloy | 60 | — |
| Beam Rifle | beam rifle | 2 metal ore, 3 crystal, 2 grey alloy | 120 | Beam Weapons |
| Disruptor | disruptor | 4 metal ore, 8 crystal, 4 grey alloy | 240 | Disruptor |
| Field Harness | field harness | 4 metal ore, 2 crystal, 3 grey alloy | 160 | Field Armour |
| Field Cowl | field cowl | 2 metal ore, 1 crystal, 2 grey alloy | 100 | Field Armour |

All are made at the Grey Armory. The unit prints are listed under
[Units](#units).

## Research

Every entry needs Int 12. Most are worked at the Grey Console; those marked
Array need the Research Array.

| Research | Cost | Work | At | Requires | Effect |
|---|---|---|---|---|---|
| Hull Plating | 2 metal ore, 4 plastic, 4 crystal, 1 grey alloy | 600 | Console | — | soldiers, heavies and lancers: AC +2 |
| Beam Weapons | 3 metal ore, 5 plastic, 6 crystal, 2 grey alloy | 900 | Console | — | the beam rifle |
| Cloning | 4 metal ore, 6 plastic, 8 crystal, 3 grey alloy | 1200 | Console | — | heavies |
| Salvage Rigs | 2 metal ore, 4 plastic, 2 crystal | 500 | Console | — | mining takes three quarters of the time |
| Armory Module | 4 metal ore, 6 plastic, 4 crystal, 2 grey alloy | 700 | Console | — | freighters and warships carry an armory (rival only) |
| Field Armour | 2 metal ore, 4 plastic, 4 crystal, 2 grey alloy | 700 | Console | Hull Plating | the field harness and cowl |
| Skimmer Pattern | 4 crystal, 2 grey alloy, 2 specimen | 600 | Console | — | skimmers |
| Field Emitter | 3 metal ore, 5 plastic, 6 crystal, 3 grey alloy | 800 | Console | Beam Weapons | the Field Emitter; warships carry two |
| Research Array | 4 metal ore, 6 plastic, 8 crystal, 3 grey alloy | 900 | Console | — | the Research Array |
| Hull Lattice | 4 metal ore, 6 plastic, 6 crystal, 3 grey alloy | 900 | Console | Hull Plating | saucer rings 12 and 15, the third deck |
| Hull Spine | 6 metal ore, 8 plastic, 8 crystal, 5 grey alloy | 1200 | Console | Hull Lattice | saucer ring 18, the fourth deck |
| Disruptor | 3 metal ore, 5 plastic, 10 crystal, 4 grey alloy | 1000 | Array | Beam Weapons | the disruptor |
| Print Refinement | 3 metal ore, 5 plastic, 8 crystal, 4 grey alloy | 900 | Array | Cloning | prints take three quarters of the time |
| Tractor Lock | 4 metal ore, 6 plastic, 8 crystal, 3 grey alloy | 900 | Array | — | grey ships catch evading ships 10% more often (rival only) |
| Lancer Pattern | 8 crystal, 4 grey alloy, 4 specimen | 1200 | Array | Skimmer Pattern | lancers |
| Heavy Frame | 4 metal ore, 8 plastic, 8 crystal, 4 grey alloy | 1000 | Array | Cloning | heavies: health ×1.25 |
| Cloak | 4 metal ore, 6 plastic, 12 crystal, 5 grey alloy | 1200 | Array | Tractor Lock | grey ships must pass closer to meet a ship in space (rival only) |
| Mothership Bulwark | 5 metal ore, 9 plastic, 10 crystal, 6 grey alloy | 1200 | Array | Field Emitter | two emitters on the mothership's deck when boarded (rival only) |

Entries marked rival only act on rival grey ships and motherships. They
are not on your research screen in a grey run; you build the Grey Armory
without Armory Module.

---

## Playing the greys

Pick **Greyish** in the Race row of New Expedition. Unlike the hive and the
autonz, the greys have a ship: the run is the colony's campaign, with the
Star Map, Travel, beaming, quests and an open-ended run (see
[The Ship & the Star Map](star_map.md)), but your crew are greys and your
ship is a saucer. There is no tutorial.

### The start

Your saucer is one round deck, radius 8. Aboard:

- three greys: a Worker, a Technician and a Soldier with a beam pistol,
- the Grey Vat and the Grey Console,
- in the hold: 30 metal ore, 12 crystal, 4 grey alloy, 10 biomass, 20 stone.

Your greys need no food and no sleep, so their panel has no needs. The
crew panel groups them by type. The HUD shows **metal ore**, **crystal**,
**grey alloy**, **plastic** and **specimens**.

You grow the crew by printing at the Grey Vat, not with a Bio-Printer.
Technicians are your only researchers. Specimens come from your greys'
kills of creatures; skimmers and lancers need them.

### Building

The build menu has **Structures**, **Greyish**, **Hull** and **Stairs**. The
Grey Vat, Grey Console and Research Array can only be built aboard your
ship. The armory and emitters go anywhere.

### Growing the saucer

The saucer grows in **rings** (outward) and **decks** (upward). Two orders
in the Orders menu do it:

- **Expand Hull**: click a deck to grow it to its next ring. Your greys cut
  a breach, lay the new floor, build the new rim and dig out the old one.
- **Add Deck**: add a deck above, at radius 8. Your greys build the stairs,
  the floor, the rim and the walls.

Each new floor tile is a Grey Hull Floor (1 metal ore) and each rim tile a
Grey Hull Wall (1 metal ore, 1 grey alloy).

| Ring | Needs |
|---|---|
| 8 | the start |
| 10 | materials only |
| 12 | Hull Lattice |
| 15 | Hull Lattice |
| 18 | Hull Spine |

| Deck | Needs |
|---|---|
| 1 | the start |
| 2 | materials only |
| 3 | Hull Lattice |
| 4 | Hull Spine |

The research screen has a **Hull** section showing each deck's radius, the
next deck, and the work in progress.

### Linked lockers

Your storage lockers work like the rival greys' pocket: every locker you
own aboard the saucer is a door onto one inventory, and every locker you
own on a world is a door onto that world's. Beaming moves things between
the two. The Global Inventory's tabs are open from the start.

### No director

There is no director in a grey run, so there is no Take Over: you give
every order yourself.

### Losing

As for the colony: the run goes on with no automatic win; lose when no grey of yours is left
anywhere. Other grey factions are rivals and stay hostile to you.
