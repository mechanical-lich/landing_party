# Xenos and Troids

Xenos crawl; troids fly. Both come out of eggs, both build nests out of
resin, and both fire their eggs at other worlds. This page starts with what
your expedition can see of them. The second half is a spoiler.

See [Factions](factions.md) for how factions in general spread, show on the
Star Map and turn hostile.

---

## What you see

### On the Star Map

A world with a nest gets a diamond badge in the nest's colour, one pip per
stage. Its info panel is tagged by what lives there and how big it has
grown, and the Destinations list notes it in brackets:

| Stage | Tag | Destinations note |
|---|---|---|
| A brood: a few bodies on bare ground | `XENO BROOD` | `[xeno brood]` |
| A nest: resin floor, an egg cluster | `XENO NEST` | `[xeno nest]` |
| A great nest: a large resin hall with a spire | `XENO GREAT NEST` | `[xeno great nest]` |

A world whose story is a troid outbreak shows `TROID` instead of `XENO`.
Everywhere else they are named xenos.

The first time your ship is in orbit of the world you get an alert:
"Xenos presence detected on Rust Tangle."

### Eggs between worlds

A great nest fires **eggs** at other worlds. An egg in flight is drawn as a
small egg on the Star Map with a dashed line to its target, and the
target's info panel says `XENO EGG INBOUND · 12 DAYS`. Eggs are slow: about
two days for every point of fuel the jump would cost your ship. When one
lands it starts a new brood there, or joins a nest already on that world.

You see an egg when the nest that fired it is one you know, when a scan
covers it, or when it comes within your scanner's range.

A nest fires only when it is big for its world, and at most every 50 days
(less with more rival factions). A nest on a small or crowded world fires
far less often. A nest whose Spire has been destroyed cannot fire until it
regrows, 50 days later.

### On the ground

A nest sits well away from your landing zone. Expect:

- **Resin**, a floor that grows out from the nest. Xenos and troids move
  faster on it; everyone else, your colonists included, moves slower. Fliers
  are not affected.
- **The heart** at the centre, **biomass pools**, **egg clusters** and an
  **evolution chamber**. A great nest also has a tall **Resin Spire**.
  Hover over its rooms: the great nest's are the "Great Nest" and the "Nest
  Heart".
- **Grey Eggs**, small grey eggs laid beside the heart and the clusters,
  and coloured eggs that hatch into new bodies.
- **Resin Blooms**, growths on the resin that drop biomass when cut down.
- Xenos and troids in colours. The colour is what the creature does, not
  how strong it is: a **Xeno** (olive) or a **Troid** (blue) works; a **Red
  Xeno**, **Orange Troid**, **Purple Xeno** or **Green Troid** fights. Green
  troids spit acid from range.

A nest's creatures are not the same as the **Wild Xenos** and **Wild
Troids** some worlds' own stories spawn. The wild ones hatch, grow and hunt
on their own; a nest's creatures work, guard and raid.

### When they come for you

A nest is hostile from the start. Its fliers patrol a wide ring round the
nest. Once one comes within 8 tiles of a colonist or a storage container of
yours, the nest knows where you are and starts **raiding**: up to five
fighters at a time, while your colonists are on that world. Guards round
the heart attack anything hostile that comes within about 12 tiles of it.

When food runs short, the nest hunts other creatures for it.

### Fighting a nest

- **Kill all of its creatures** and the nest is wiped out. Legends records it.
- **Destroy the Resin Spire** to stop it firing eggs. It regrows 50 days
  later.
- **Break its eggs and grey eggs** to slow its growth; it needs both to
  make new bodies.
- **Cut off its food.** A nest eats biomass, and a nest on a world that
  cannot feed it shrinks. Radiation and plague worlds are hard on it.

A nest that dies out on its own leaves a **husk**: an empty nest with
cracked eggs. See [Dangers, Ruins and Relics](dangers_and_ruins.md#what-factions-leave-behind).

---

> ## Spoilers below
>
> The rest of this page says what the xenos and troids really are, and
> names every creature by its true name. The game does not tell you this.
> Stop here if you would rather find out by playing.

---

## The hive

Xenos and troids are one organism: the **hive**. Crawlers (xenos) and fliers
(troids) are two body plans of the same thing, laid by the same heart.
They never fight each other.

A hive grows through three stages:

| Stage | Shown as | Reached at | Stands on its world |
|---|---|---|---|
| Brood | brood | the start | a patch of resin, its heart |
| Nest | nest | 6 bodies, or 30% of what its world can feed | one egg cluster, the evolution chamber |
| Hive | great nest | 12 bodies, or 60% of what its world can feed | two egg clusters, the evolution chamber, the Resin Spire |

A world feeds more of them when it is rich in biomass or already
infested, and fewer on moons, asteroid fields and radioactive worlds.

### How a hive works

1. The **Brood Heart** and every **Egg Cluster** lay a **Grey Egg** (a
   larva) on the resin beside them every so often, up to 3 grey eggs
   plus 2 more for each cluster.
2. A drone carries biomass to a grey egg and works it into an egg of the
   type the hive wants. The egg hatches; the hatchling stays close to the
   heart and grows into an adult.
3. Drones forage biomass from plants and resin blooms, haul it to the
   pools, lay more resin, and build. Every kill by one of the hive's bodies
   gives it 3 biomass.
4. The **Evolution Chamber** researches the evolve tree, which unlocks new
   types and strengthens old ones.
5. At the great nest stage, when the Spire is ready, three bodies walk to
   it and are fired off as an egg. The egg lands as a brood of two.

On a troid world the hive breeds more fliers: its warriors and hunters
swap places, and so do its burrowers and spitters.

### Units

Every hive body eats biomass. Lower speed numbers act more often.

| Type | Seen as | Body | Role | Egg cost | Lay (work) | Hatch (turns) | Health | Str | Dex | AC | Attack | Speed | Unlocked by |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Drone | Xeno (olive) | crawler | worker: forages, mines, hauls, builds, lays resin, researches | 2 biomass | 60 | 20 | 14 | 8 | 12 | 6 | 1d4 | 10 | start |
| Scout | Troid (blue) | flier | scout: patrols, finds enemies | 2 biomass | 60 | 16 | 10 | 6 | 18 | 8 | 1d4 | 6 | start |
| Warrior | Red Xeno | crawler | guard: attacks, holds posts | 4 biomass | 90 | 24 | 32 | 16 | 12 | 12 | 2d6 | 12 | Warrior Brood |
| Hunter | Orange Troid | flier | hunter: attacks | 5 biomass | 90 | 24 | 24 | 14 | 16 | 10 | 1d8 | 8 | Hunter Brood |
| Burrower | Purple Xeno (pink egg) | crawler | digger: mines, digs, hauls, attacks | 5 biomass | 120 | 30 | 24 | 14 | 10 | 14 | 1d8 | 14 | Burrower Brood |
| Spitter | Green Troid | flier | spitter: spits acid (1d10, range 6) | 6 biomass | 120 | 30 | 18 | 10 | 14 | 8 | 1d4 | 10 | Spitter Brood |

Every hive body resists radiation.

### Structures

Every hive structure stands only on **resin**.

| Structure | What it does | Cost | Work | Health | Needs |
|---|---|---|---|---|---|
| Brood Heart | The centre of a hive. Lays grey eggs. Not built: every hive starts with one. | — | — | 80 | — |
| Biomass Pool | The hive's store: holds biomass and nothing else (6 stacks). | 2 biomass | 10 | — | — |
| Egg Cluster | Lays grey eggs; each one lets the hive keep 2 more. | 8 biomass | 30 | 40 | — |
| Evolution Chamber | Where the evolve tree is researched. | 12 biomass | 40 | 60 | — |
| Resin Spire | Fires eggs at other worlds. | 20 biomass | 60 | 60 | fires only at the great nest stage |
| Grey Egg | A larva: an egg recipe turns it into a typed egg. Laid, not built. | — | — | 4 | — |
| Resin Bloom | Grows on resin, about one for every 12 resin tiles. Cut it for 2 biomass. | — | — | — | — |

A rival hive builds its clusters and chamber once it is a nest, and its
Spire once it is a great nest.

### Eggs

Eggs are made at a grey egg, which the egg uses up.

| Recipe | Makes | Cost | Work | Needs |
|---|---|---|---|---|
| Drone Egg | a drone | 2 biomass | 60 | — |
| Scout Egg | a scout | 2 biomass | 60 | — |
| Warrior Egg | a warrior | 4 biomass | 90 | Warrior Brood |
| Hunter Egg | a hunter | 5 biomass | 90 | Hunter Brood |
| Burrower Egg | a burrower | 5 biomass | 120 | Burrower Brood |
| Spitter Egg | a spitter | 6 biomass | 120 | Spitter Brood |

### The evolve tree

Researched at the Evolution Chamber, paid in biomass. "Evolved" upgrades
are a type's second; its adults then show in their largest form.

| Evolution | Cost | Work | Requires | Effect |
|---|---|---|---|---|
| Warrior Brood | 20 biomass | 900 | — | unlocks warriors |
| Spire Reach I | 25 biomass | 1200 | Warrior Brood | Spire range +15 |
| Hunter Brood | 30 biomass | 1500 | Warrior Brood | unlocks hunters |
| Spire Reach II | 40 biomass | 1800 | Spire Reach I | Spire range +20 |
| Resin Flow | 15 biomass | 900 | — | resin is laid and spreads 50% faster |
| Resin Surge | 30 biomass | 1500 | Resin Flow | another 50% faster |
| Drone Vigour | 15 biomass | 900 | — | drones: health ×1.25, work ×1.25 (evolved) |
| Warrior Carapace I | 25 biomass | 1200 | Warrior Brood | warriors: health ×1.25 |
| Warrior Carapace II | 40 biomass | 1800 | Warrior Carapace I | warriors: health ×1.25, strength ×1.2 (evolved) |
| Burrower Brood | 30 biomass | 1500 | Drone Vigour | unlocks burrowers |
| Burrower Mandibles I | 25 biomass | 1200 | Burrower Brood | burrowers: work ×1.25 |
| Burrower Mandibles II | 40 biomass | 1800 | Burrower Mandibles I | burrowers: work ×1.25, strength ×1.2 (evolved) |
| Spitter Brood | 40 biomass | 1800 | Hunter Brood | unlocks spitters |
| Spitter Acid I | 25 biomass | 1200 | Spitter Brood | spitters: strength ×1.25 |
| Spitter Acid II | 40 biomass | 1800 | Spitter Acid I | spitters: strength ×1.2, health ×1.25 (evolved) |
| Wing Membrane | 25 biomass | 1200 | Hunter Brood | every flier acts faster (speed ×0.8) |
| Hunter Talons I | 25 biomass | 1200 | Hunter Brood | hunters: strength ×1.25 |
| Hunter Talons II | 40 biomass | 1800 | Hunter Talons I | hunters: strength ×1.2, health ×1.25 (evolved) |
| Scout Eyes I | 15 biomass | 900 | — | scouts act faster (speed ×0.85) |
| Scout Eyes II | 25 biomass | 1200 | Scout Eyes I | scouts: speed ×0.85, health ×1.25 (evolved) |

A finished evolution changes the living bodies of its type at once.

### The Spire's reach

The Resin Spire's range starts at 25, in the same units as the Star Map's
fuel cost, and grows to 40 with Spire Reach I and 60 with Spire Reach II.

---

## Playing the hive

Pick **Hive** in the Race row of New Expedition. A hive run has **no ship**:
no hold, no fuel, no beaming, no quests and no tutorial.

### The start

You begin as a nest on the start system: 6 drones, 2 scouts, 3 grey eggs and
30 biomass. The game opens viewing it.

### Viewing and ordering

Use **View** on the Star Map to open a world. On a world you hold you have
the cursor; on a world within your Spire's reach you can only look, as from
orbit.

The HUD shows your **biomass** and **larvae** (grey eggs). Your people are
the **Brood**; the research screen is **Evolve**; the crew panel groups them
by type. The Orders menu has:

| Order | What it does |
|---|---|
| Dig | as for the colony |
| Forage | drag to forage flora and break deposits |
| Store, Drop, Cancel | as for the colony |
| Attack | order your brood to attack a target |
| Lay Resin | click ground to have a drone lay resin there; hive structures stand only on resin |
| Post Guard | click ground to post the nearest idle warrior there |
| Take Over | open the This World panel: choose what you run on this world yourself |

The Hive build menu has the Biomass Pool, Egg Cluster, Evolution Chamber
and Resin Spire. Click a grey egg to choose what to lay on it; click the
Evolution Chamber to evolve.

### The director, and taking over

On every world you hold, the director runs the hive as a rival's nest is
run, and your orders join its queue. To take work off it, choose **Take
Over** in the Orders menu on any world you hold. It opens the **This
World** panel (the Brood Heart's panel has the same switches):

- **I run this world** takes everything over.
- Or tick only the parts you want: **I lay the eggs**, **I send the
  foragers**, **I lay the resin**, **I place the structures**, **I choose
  the evolutions**, **I post the guards**, **I send the scouts**, **I dig
  for the resin**, **I order the raids**, **I load the Spire**.

Leaving a world (to the Star Map or another world) hands everything back
to the director.

Worlds you are not viewing grow, breed and evolve on their own. Each world
has its own evolutions; an egg carries its parent world's.

### Spreading

Click the **Resin Spire** to open its panel. It shows whether the Spire is
ready and the worlds in reach.

1. Pick a world from the list and click **Aim at this world**.
2. When the Spire is ready, three of your bodies walk to it and are fired
   off. With **I load the Spire** ticked, click **Load the Spire** to send
   them yourself.

The Spire never fires unaimed, and the director never picks a world for
you. A shot uses up the aim. The aim stays set when you leave the world.
The Spire is ready under the same rules as a rival's: great nest stage, a
full enough nest, and the cooldown passed.

### Winning and losing

You win holding **six worlds** at once ("The Swarm Spreads"). You lose when
no nest and no egg in flight is left anywhere ("The Brood Is Gone"). If a
world loses five or more of its brood in a day, Legends records it.

When a rival colony first sees your brood, you get the alert "Your brood
have been seen on Ember Pit. They are hostile now."

Wild xenos and troids are not part of your brood; they hunt your drones
and your guards fight them. A rival hive is hostile to yours.
