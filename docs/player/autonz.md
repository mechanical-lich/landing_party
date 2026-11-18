# Autonz

The autonz are machines. They thread a world with wire, print more of
themselves from its ore, and send signals to other worlds that wake new
machines there. They are hostile, and they never sleep.

See [Factions](factions.md) for how factions in general spread, show on the
Star Map and turn hostile.

---

## As a rival

### On the Star Map

An autonz world gets a square chip in their colour: filled, with one pip
per stage, while it is awake; an outline while it is asleep. The info panel
and the Destinations list say what it is:

| What | Tag | Destinations note |
|---|---|---|
| A node: a small wired chamber | `AUTONZ NODE` | `[autonz node]` |
| A hub: a round wired hall with a Signal Core | `AUTONZ HUB` | `[autonz hub]` |
| A sleeper: a signal has landed and is waiting | `AUTONZ SIGNAL · WAKES IN 20 DAYS` | `[autonz signal]` |

Legends records them as they land ("an autonz swarm made planetfall"),
signal, and wake ("something powered down ... woke and began laying autonz
wire").

### How they grow

The autonz' economy is **wire**. Their wire placers lay wire outward over
the ground, and the more of the ground round their centre is wired, the
more machines the world can hold: with no wire, about a third of what the
world could support; with most of it wired, all of it. Ore-rich worlds wire
faster.

A node becomes a **hub** at 7 machines (or half of what its world can hold,
if that is fewer). A hub builds a Signal Core, a foundry, converter nodes
and defence pylons.

### Signals

A hub sends a **signal** to another world when:

- its Signal Core stands,
- at least 60 days have passed since its last signal (less with more rival
  factions),
- it is close to full, or half the ground in its patch is wired,
- and it has at least four machines.

Two machines walk to the Signal Core and leave with the signal. A signal
does not fly: it arrives at once, as a **sleeper node** on the target
world. The sleeper wakes **30 days** later as a new node. A nearer world is
more likely to be picked.

Signal range is 16, in the same units as the Star Map's fuel cost, plus 4
while a Relay Pylon stands, plus whatever its research adds (up to 42 in
all).

A world that is fully wired stops growing, so every signal after that costs
it machines it does not replace.

### On the ground

- **Wire**, blue lines over the ground, with a red-wired core at the
  centre. Every autonz structure stands only on wire.
- A node is a small chamber open on four sides with a **Cogitator** and
  the **Autonz Fabricator** that prints new machines. A hub is a round
  hall with four ways out and the **Signal Core** at its heart, an **Autonz
  Foundry**, **Converter Nodes**, **Defence Pylons** with sentinels beside
  them, and a **Relay Pylon** at the wire's edge.
- Machines in two finishes: **steel**, and **bronze** for types the autonz
  have upgraded. Bronze machines are tougher. A mixed group shows how
  recently they upgraded, since older machines stay steel.
- A sleeper world holds only a **Dormant Node** at its centre.

Some worlds' own stories have autonz on them already (an incursion, or a
**Dormant Enforcer** left in an old hull). Those are the same faction and
work alongside a node.

### When they come for you

Seekers patrol a ring well out from the centre. Once they have found you,
the autonz raid with up to four of their enforcers, seekers and wardens,
while your colonists are on their world. Once they know Warden Pattern, a
raid only sets out with a warden to lead it. Enforcers and wardens hold posts round the centre and
attack anything hostile within about 12 tiles.

### Fighting them

- **Kill all of its machines** and the presence is wiped out.
- **Destroy a Dormant Node before it wakes** and that world is clean.
- **Destroy the Signal Core** to stop signals. They rebuild it.
- Every machine drops **ferrite** when destroyed (foundry machines drop
  two). Nothing the colony makes uses ferrite yet.
- A razed settlement left alone rebuilds one structure a day.

---

## Units

All autonz units are printed, need no food and never sleep. Lower speed
numbers act more often.

| Unit | Role | Printed at | Cost | Work | Health | Str | Dex | AC | Attack | Speed | Unlocked by |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Wire Placer | lays wire, mines, digs, hauls, builds, crafts, researches | Fabricator | 4 metal ore | 120 | 8 | 6 | 8 | 4 | 1d4 | 10 | start |
| Enforcer | guard: attacks, holds posts; carries a laser pistol | Fabricator | 8 metal ore, 1 crystal | 160 | 12 | 8 | 12 | 8 | 1d6 | 12 | start |
| Relay Drone | folds into a Relay Pylon on wire (120 turns); patrols | Fabricator | 4 metal ore, 2 crystal | 120 | 8 | 4 | 10 | 4 | 1d4 | 8 | Relay Pattern |
| Seeker | scout and fighter: patrols, finds enemies | Fabricator | 5 metal ore, 1 crystal | 120 | 10 | 6 | 16 | 8 | 1d6 | 6 | Seeker Pattern |
| Harvester | flies; mines, digs, hauls | Foundry | 4 metal ore, 2 ferrite | 160 | 10 | 6 | 8 | 6 | 1d4 | 10 | Foundry Pattern |
| Sentinel | flies; sentry with a beam emitter (1d8, range 6), posted at Defence Pylons | Foundry | 6 metal ore, 1 crystal, 3 ferrite | 200 | 14 | 6 | 14 | 10 | 1d4 | 10 | Sentinel Pattern |
| Warden | heavy guard; carries a laser pistol, leads raids | Foundry | 10 metal ore, 2 crystal, 4 ferrite | 240 | 30 | 14 | 10 | 12 | 2d6 | 14 | Warden Pattern |

**Bronze.** Once a type's bronze research is done, new prints of that type
come out bronze: health ×1.5, strength ×1.25. Machines already standing stay
steel.

## Structures

Every autonz structure stands only on **wire**.

| Structure | What it does | Cost | Work | Health | Needs |
|---|---|---|---|---|---|
| Autonz Fabricator | Prints wire placers, enforcers, relay drones and seekers. Not built: it comes with the node. | — | — | — | — |
| Cogitator | The research station: assimilation runs here. | 12 metal ore, 2 crystal | 30 | 40 | — |
| Signal Core | Sends signals. No signal while it is down. | 20 metal ore, 4 crystal | 60 | 60 | signals only from a hub |
| Autonz Foundry | Refines ferrite and prints harvesters, sentinels and wardens. | 16 metal ore, 4 crystal | 40 | — | Foundry Pattern |
| Converter Node | A second place the wire spreads from; placers within 5 tiles lay wire in half the time. | 10 metal ore, 2 ferrite | 30 | 40 | Converter Node |
| Defence Pylon | A sentinel posts here. | 10 metal ore, 2 ferrite | 30 | 40 | Sentinel Pattern |
| Relay Pylon | +4 signal range. Not built: a relay drone deploys into it. One per world. | — | 120 | 30 | Relay Pattern |
| Storage Locker | As for the colony. | see [Buildings](buildings.md) | | | — |

A rival builds its Signal Core, foundry and relay pylon once it is a hub.

## Recipes

| Recipe | Makes | Cost | Work | Station | Needs |
|---|---|---|---|---|---|
| Refine Ferrite | 2 ferrite | 4 metal ore, 1 crystal | 90 | Autonz Foundry | Foundry Pattern |

The unit prints are listed under [Units](#units).

## The assimilation tree

Researched at the Cogitator, paid in materials.

| Pattern | Cost | Work | Requires | Effect |
|---|---|---|---|---|
| Relay Pattern | 8 metal ore, 2 crystal | 600 | — | unlocks relay drones and the Relay Pylon |
| Seeker Pattern | 10 metal ore, 2 crystal | 900 | — | unlocks seekers |
| Foundry Pattern | 16 metal ore, 4 crystal | 1200 | — | unlocks the Foundry and harvesters |
| Converter Node | 12 metal ore, 2 ferrite | 1200 | Foundry Pattern | unlocks the Converter Node |
| Sentinel Pattern | 14 metal ore, 2 crystal, 4 ferrite | 1500 | Foundry Pattern | unlocks sentinels and the Defence Pylon |
| Warden Pattern | 18 metal ore, 4 crystal, 6 ferrite | 1800 | Foundry Pattern | unlocks wardens |
| Carrier Gain I | 10 metal ore, 2 crystal | 900 | — | signal range +10 |
| Carrier Gain II | 16 metal ore, 4 crystal | 1800 | Carrier Gain I | signal range +12 |
| Lattice Weave I | 12 metal ore, 2 crystal | 1200 | — | wire is laid and spreads 50% faster |
| Lattice Weave II | 20 metal ore, 4 crystal, 4 ferrite | 2400 | Lattice Weave I | another 50% faster |
| Bronze Placer | 12 metal ore, 4 ferrite | 1200 | Foundry Pattern | wire placers print bronze |
| Bronze Enforcer | 14 metal ore, 6 ferrite | 1500 | Foundry Pattern | enforcers print bronze |
| Bronze Relay | 12 metal ore, 4 ferrite | 1200 | Relay Pattern | relay drones print bronze |
| Bronze Seeker | 14 metal ore, 4 ferrite | 1200 | Seeker Pattern | seekers print bronze |
| Bronze Harvester | 16 metal ore, 6 ferrite | 1500 | Foundry Pattern | harvesters print bronze |
| Bronze Sentinel | 18 metal ore, 6 ferrite | 1500 | Sentinel Pattern | sentinels print bronze |
| Bronze Warden | 20 metal ore, 8 ferrite | 1800 | Warden Pattern | wardens print bronze |

---

## Playing the autonz

Pick **Autonz** in the Race row of New Expedition. Like the hive, an autonz
run has **no ship**: no hold, no fuel, no beaming, no quests and no
tutorial. See [Playing another race](factions.md#playing-another-race) for
how viewing worlds and the end of the run work.

### The start

You begin as a node on the start system: 4 wire placers, 2 enforcers, 30
metal ore and 6 crystal.

### Ordering

The HUD shows **metal ore**, **crystal**, **ferrite** and your **conversion**
(how much of the patch round your centre is wired). Your people are
**Units**, grouped in the crew panel by type and finish. The research
screen is **Assimilation**. Units have no needs, so their panel has none.

| Order | What it does |
|---|---|
| Dig, Mine, Store, Drop, Cancel | as for the colony |
| Attack | order your units to attack a target |
| Lay Wire | click ground to have a wire placer lay wire; autonz structures stand only on wire, and wire raises how many units the world can hold |
| Deploy Relay | click wired ground to send the nearest idle relay drone there to fold into a Relay Pylon; refused once a pylon stands or one is on its way ("No more of those would add anything here.") |
| Take Over | open the This World panel: choose what you run on this world yourself |
| Post Guard | click ground to post the nearest idle enforcer, warden or sentinel there |

The build menu's **Autonz** category holds the Cogitator, Signal Core,
Foundry, Converter Node and Defence Pylon. Click the Fabricator or the
Foundry to print; click the Cogitator to assimilate.

### The director, and taking over

The director runs every world you hold, and your orders join its queue.
To take work off it, choose **Take Over** in the Orders menu on any world
you hold, node or hub. It opens the **This World** panel (the Signal Core's
panel has the same switches):

- **I run this world** takes everything over.
- Or tick only the parts you want: **I choose the prints**, **I lay the
  wire**, **I send the miners**, **I place the structures**, **I choose the
  patterns**, **I post the guards**, **I send the seekers**, **I order the
  raids**, **I load the Signal Core**.

Leaving a world hands everything back to the director.

Worlds you are not viewing grow, wire and research on their own, each with
its own research.

### Spreading

The Signal Core's panel shows whether a signal is ready and the worlds in
reach.

1. Pick a world and click **Transmit here**.
2. When the signal is ready, two units walk to the Core and leave with it.
   With **I load the Signal Core** ticked, click **Load the Core** to send
   them yourself.

The Core never transmits unaimed. A signal founds a **sleeper** of yours on
the target world; the Star Map lists it under Your worlds with its wake
day. A sleeper keeps you in the game, but it does not count toward the milestone
until it wakes.

### Milestone and losing

Holding **six awake worlds** at once is a milestone ("Assimilation"): an
alert and a Legends entry, and the run goes on. You lose
when no node, no sleeper and no signal of yours is left anywhere ("Signal
Lost").

When a rival colony first sees your units, you get the alert "Your units
have been seen on Ember Pit. They are hostile now." If a world loses five
or more units in a day, Legends records it.
