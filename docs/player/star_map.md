# The Ship & the Star Map

Landing Party is a campaign of expeditions. Your colonists begin aboard a
space life-raft that has just arrived at the edge of a galaxy. You do not
start on a planet — you start **adrift** at the **Wreck Site**, the empty
point just outside the galaxy's rim where the damaged raft came to rest.
There is nothing there to orbit or land on, so the first thing every
expedition does is choose a destination on the Star Map and spend a little
fuel to reach it. The nearest system, your landfall, is a short, cheap jump
away, and it is always a safe first landing.

While adrift, Beam Down, To Site and Beam Resources are unavailable and say
so; the crew stays aboard and eats from the hold; Travel Here is the live
action. The Wreck Site stays on the map as a dim cross, but you cannot
travel back to it.

## The Ship (your hub)

The ship is an abstract hub — there is no walkable ship deck. It holds:

- **The hold** — a single, campaign-wide stockpile. Fuel and any other
  materials you've **beamed up** from a site live here. Fuel for travel is
  spent from the hold.
- **The roster** — colonists currently aboard, not deployed anywhere.

A new expedition starts with a fuel reserve, 100 of each starting material in
the hold, and a small crew already aboard.

> **Ship and site storage are separate.** What your colonists mine, harvest,
> or craft on a planet stays in that planet's storage containers when you
> leave. To move materials between the ship hold and a site, use the
> [Beam Resources](#beaming-resources) flow.

## The Star Map

Open it from the title screen ("New Expedition" / "Load Expedition") or, while
playing, with the **Star Map** button (top-right, next to Follow) or the **O**
key.

The Star Map shows:

- **The galaxy** — every one of its systems is on the map from the first
  day, as a point. Its header names the galaxy's shape (spiral, ring,
  irregular and so on), how many systems it has and how many you have
  identified. See [What you know](#what-you-know).
- **Locations** — planets, moons, asteroid fields, stations. Each lists its
  travel fuel cost. One you have not identified lists as an
  **Unidentified system** with its fuel cost and nothing else.
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
turns biomass + ore into fuel. Refined fuel stays at the site by default — to
spend it on travel, **beam it up** to the ship hold first (see below).

## What you know

The whole galaxy exists from the start: nothing new appears during a run.
What changes is what you know of it. The map always shows the galaxy's
**shape** as a faint haze of distant stars, but the systems in it are hidden
until you have seen them. Every system is in one of four states:

| State | On the Star Map | How |
|---|---|---|
| **Unseen** | nothing: only the haze | |
| **Seen** | a dim point, no name, only its fuel cost | coming within the ship's sight range of it, a scan, a contract or quest that names it, your race's worlds |
| **Identified** | its name, type, danger tag, story and who holds it (once seen) | a scan in range, arriving there, a contract or star chart that names it |
| **Visited** | as identified, and its level is yours | landing |

Once seen, a system stays on the map.

**Sight.** The ship sees every system within **60 units** of wherever it
arrives (**40** in a small galaxy; the faint ring round the ship shows how
far). Each Ship Scanner
upgrade adds 10 to that. You can only jump to systems you have seen, so
exploring means jumping to the edge of what you can see and looking again.

At the start you can see what lies near your landfall, and you have
identified only your landfall system. The header above the map counts the
systems in the galaxy, how many you have found, and how many you have
identified.

### Scanning

Once you research a **Ship Scanner**, the **Scan** button works. A scan
**identifies every system within the scanner's range** of the ship, and shows
who holds the worlds there. There is no chance roll: it costs fuel and always
tells you everything in range. Hover over Scan to see its range drawn round
the ship. A scan that covers the gate names it as the gate.

Research improves it two ways:

- **Scanner tier** (`Ship Scanner` → `Enhanced Scanner Array` → `Deep-Space
  Scanner`) reaches farther: 40, then 70, then 110 fuel from the ship.
- **Scan efficiency** (`Scan Optimization`, `Scan Calibration`) cuts the fuel
  each scan burns (40 → 25 → 15).

The button shows the current fuel cost and greys out if you can't afford it or
have no scanner yet.

### Jumping blind

You may jump to a system you have not identified. The distance and the fuel
are known; what is there is not until you arrive. The Travel button reads
**Jump Blind** for such a system. Arriving identifies it. A scan is what buys
off the risk.

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
solid rock or wall directly overhead blocks it. A floor built above them does
not.

## The tutorial's Travel lock

While the [guided first landing](tutorial.md) runs, Travel is limited to the
Training Ground, and later to the system you started in. Every other
destination stays listed and selectable, with its fuel cost and summary, but
the button reads **Travel locked**, the status line names the one place you can
go, and the same reason sits under the fuel line. The destination the tutorial
wants is marked in magenta on the canvas and in the list, selected and centred
for you. The lock lifts when the tutorial ends or you skip it. Answering an incident on a
planet you have crew on always works.

## Beaming resources

Resources do **not** move automatically. Board the ship with **To Ship** and click
its supply pad to open the hold in the
[Storage Inspector](storage.md#the-storage-inspector). Once you have researched
Inventory Survey, **Beam Down** on the dashboard's **Global Inventory** tab opens
it too. From here:

- Pick a material, set an amount, click **Beam Down v** to send it from the
  ship hold to the first colony-owned storage container at the loaded site
  that accepts it. Requires a loaded location.
- To **beam up** instead, Resume / Land into the location and click a storage
  container on the planet — its inspector has a **Beam Up ^** action that
  sends the selected amount into the ship hold.

Rations, gear and other single items list and beam the same way, by count; they
have no tags, so only an unfiltered container such as a Storage Locker takes
them, and Beam Down says so if the site has none. Beam Up keeps whatever a full
hold can't take in the container it came from.

The ship hold accepts any material (no tag filters). Beam Down can fail in
two ways:

- **No storage container on site at all.** Beam Down lands materials in the
  first colony-owned container it can find. Build at least one Storage
  Locker (or a typed container that accepts what you're sending) before your
  first beam-down.
- **No container accepts the tag.** Even with containers on site, a fuel
  tank won't take metal ore. Build a generalist Storage Locker, or a typed
  container that matches the material's tags.

The status line under the Star Map will tell you which case you've hit. See
[Storage & Materials](storage.md) for the full picture of containers, tags,
and filters.

## A typical loop

1. Adrift at the Wreck Site: Star Map → pick a destination → **Travel Here**
   (spends fuel if moving).
2. Beam down a landing party.
3. **Resume / Land** and play: gather, build, research, refine fuel.
4. **O** / Star Map button → beam survivors up (or leave a colony behind).
5. **Travel Here** to the next world, or back to a paused one.
6. **Save Expedition** any time from the Star Map.

Worlds other factions hold carry a badge and a tag, and their eggs and ships
are drawn in flight; see [Factions](factions.md#on-the-star-map). A jump can
also meet a faction ship on the way (see
[Meeting ships in space](factions.md#meeting-ships-in-space)).

Each world's summary on the Star Map comes from its history, after a
**Calm**, **Hazard** or **Threat** tag that says how dangerous it is (see
[Dangers, Ruins and Relics](dangers_and_ruins.md)); the
[Legends](legends.md) tab of the dashboard collects everything you have learned
about it.

## Quests

Quests are your objectives — there is no per-planet win or loss any more.
Open the **Quest Log** from the Star Map:

- **Offered** quests can be **accepted** (some auto-accept).
- **Active** quests track an objective: gather/stockpile a resource, research a
  tech, build a structure, clear out a hostile, survive N days, etc. Resource
  objectives count the campaign-wide ship hold, not just on-planet crates.
- Completing a quest grants its **reward** (fuel and/or resources into the ship
  hold) and some pay in **star charts**, identifying the nearest systems you
  do not know yet. Some quests unlock follow-up quests.
- A system's own quests are offered once you have identified it. Contracts
  and the quests a jump turns up are news: they are offered at once, name
  their system, and accepting one identifies it.

Active quests and a completed count also show in the in-game **Goals** tab.
Objectives are evaluated continuously while you play a location, so a quest
completes within moments of you meeting it.

## The galaxy and the end of a run

Every expedition arrives in a **galaxy generated whole** from its seed, of the
size chosen on New Expedition (**Small** about 40 systems, **Medium** about
80, **Large** about 150). Its shape is rolled. Rival factions arrive from
outside it too, each at its own point on the rim, so you may start beside one
or far from all of them.

There is no fixed length and no automatic win: the run goes on as long as
your people do. **Every jump has a chance to turn up fresh contracts** on
systems already there, so keeping on the move keeps work flowing.

### The gate

Somewhere deep in the galaxy is **the gate**, the way out. It is dark. A
scan that covers it, a star chart or arriving beside it identifies it, and
with it the world beside it that holds its **controls**: the Star Map names
both.

1. Land on the controls world. Its controls are in a **vault** some way
   from the landing: a stair shaft two or three levels down to a buried
   dungeon, guarded. Expect a fight.
2. In the vault's far room, click the dark **Gate Controls** to order them
   repaired, as you would any broken machine. Your people work them and the
   gate lights, for good and for everyone.
3. Beam your people back up, travel to the gate, and press **Leave the
   Galaxy**. Leaving burns a large reserve of fuel (300): refine it first.
   It ends the run as a win. Not taking it, the run goes on.

Rival colonies want out too. They head for the gate once they know where it
is, switch it on if you have not, and leave. If one switches it on first,
you hear of it and Legends records it; the gate is then lit for you as well.

Lose condition: a **total wipe** — no colonists left anywhere (none planetside,
none in the ship roster, none on any established system). The only way to grow
the crew is to build a **Bio-Printer** and print new colonists from biomass
(see [Crafting](crafting.md)), so protect your starting crew until you can
afford one.

This is the colony's run. A greys run flies it too; a hive or autonz run
has no ship, and holding six worlds at once is a milestone (an alert and a
Legends entry), not the end (see
[Playing another race](factions.md#playing-another-race)).

