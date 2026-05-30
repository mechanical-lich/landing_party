# Storage & Materials

Everything your colonists gather, craft, or carry is treated as a **material**. Materials live in **storage containers** — lockers and silos you build on the colony, plus the persistent **ship hold** in orbit. This page covers how materials are categorised, what each container will take, and how to move things around manually.

---

## Materials & tags

Every material in the game carries one or more **tags** that describe what it is. Tags drive two things:

- Which **containers** will accept the material.
- (Future) which **crafting recipes** can pull from it.

The starting resources and their tags:

| Material | Tags | Max stack |
|---|---|---|
| Metal Ore | metal, mineral, raw_material | 500 |
| Crystal | crystal, mineral, raw_material | 500 |
| Stone | stone, mineral, raw_material | 500 |
| Biomass | biomass, organic, raw_material | 500 |
| Fuel Cell | fuel, energy | 200 |
| Radioactive Material | radioactive, mineral, raw_material | 200 |

A material's tags are visible in the [Storage Inspector](#the-storage-inspector). They aren't editable — they're part of the material's identity.

---

## Storage containers

Build menu → **Structures**. Each container has a **game-defined allowed list** of tags. A container will only accept materials whose tags overlap that list. A generic Storage Locker has *no* allowed list, so it takes anything.

| Container | Accepts (tags) | Cost | Notes |
|---|---|---|---|
| **Storage Locker** | anything | 2 metal_ore | Universal. Use early. |
| **Mineral Bin** | metal, crystal, stone, mineral | 3 metal_ore | All raw minerals (including radioactive — see below). |
| **Fuel Tank** | fuel, energy | 2 metal_ore, 1 crystal | Sealed for fuel cells. |
| **Bio Silo** | biomass, organic, food | 2 metal_ore, 5 biomass | Organic seal. Holds food too. |
| **Hazmat Vault** | radioactive | 4 metal_ore, 2 crystal | Shielded. Heavy 8-tick build. |

> **Note:** the Mineral Bin accepts the "mineral" tag, which radioactive material also carries. If you want radioactive segregated to the Hazmat Vault, set the bin's player filters to exclude it (see [Allowed Tags filters](#allowed-tags-filters)).

A container's **OwnedBy** is set to your colony when the worker places it. Workers from your colony will haul to and from any container they own; they won't touch hostile containers.

---

## The Storage Inspector

Click any storage container in the world (including the ship hold via the Star Map's **Beam Resources...** button) to open the **Storage Inspector**.

Layout:

```
┌─ Storage Inspector — <container name> ──────────────────┐
│                                                          │
│ Tag Filter   Inventory                  Allowed Tags    │
│ ┌────────┐  ┌────────────────────────┐ ┌──────────────┐ │
│ │ All    │  │ Biomass            ×100│ │     Biomass  │ │
│ │ Biomass│  │ Crystal            ×100│ │     Crystal  │ │
│ │ Crystal│  │ Fuel               ×400│ │     Energy   │ │
│ │ Fuel   │  │ Metal Ore          ×100│ │ ✓   Fuel     │ │
│ │ ...    │  │ ...                    │ │ ...          │ │
│ └────────┘  └────────────────────────┘ └──────────────┘ │
│                                                          │
│ Selected: Metal Ore — 100 available  (metal, mineral)   │
│ Amount: [____] [All] [▲ Beam Up] [↔ Relocate]          │
│                                                          │
│                          [Close]                         │
└──────────────────────────────────────────────────────────┘
```

- **Tag Filter (left)** — single-select. Pick a tag to narrow the inventory view to materials with that tag. Pick **All** to show everything.
- **Inventory (centre)** — the container's contents, sorted, with current quantities. Click a row to select that material; the action panel below populates with its details.
- **Allowed Tags (right)** — the tags this container will accept. A `✓` marks tags currently in your player filter list (see below).
- **Action panel** — shows the selected material's quantity, tag list, and the available actions (Beam, Relocate). The **All** button fills the Amount field with the full available count.

The inspector auto-refreshes the inventory after each action so live counts stay correct.

---

## Allowed Tags filters

Storage containers carry two tag lists:

1. **Allowed Tags** — game-defined. Fixed by the container blueprint. A Fuel Tank will always be limited to "fuel/energy" no matter what; a Storage Locker will always accept everything.
2. **Filter Tags** — player-defined, a subset of Allowed Tags. When non-empty, the container *only* accepts tags you've ticked here.

You edit Filter Tags in the inspector's right-hand **Allowed Tags** list. Click a tag to toggle the `✓` mark.

**Example.** Your Mineral Bin accepts metal, crystal, stone, and mineral. You want it dedicated to metal only. In the inspector, tick `Metal` in the Allowed Tags list. Now workers stop depositing crystal or stone there — those go elsewhere.

Filter Tags persist with your save.

---

## Beaming between ship and site

Materials move between the **ship hold** (in orbit) and **site storage** (on a planet) only via explicit beam operations. There is no automatic sweep when you leave a location — what's on a planet stays on the planet.

**Star Map → Beam Resources...** opens the inspector pointed at the **ship hold**. The action button reads **▼ Beam Down**: each click moves the entered Amount of the selected material from the ship hold into the first colony-owned storage container at your current site that accepts it. (You must have travelled to a location for this to work.)

**Clicking a storage container in-game** opens the inspector pointed at *that* container. The action button reads **▲ Beam Up**: each click moves the entered Amount from that container into the ship hold. The ship hold has effectively unlimited capacity.

Constraints:

- **Beam Down** requires at least one accepting storage container on site. If you've only built specialised containers and you're trying to beam down something none of them accept, the order will fail.
- The ship hold itself has no Allowed Tags, so **Beam Up** always works as long as the source container has the material.

---

## Orders — Store and Relocate

Two manual orders let you direct individual materials yourself.

### Store

In the Orders menu (left sidebar) under **Orders**, pick **Store**. The cursor enters Store mode:

1. **Click an item on the ground.** Any loose item with material tags works — dropped crystals from mining, a forgotten food ration, etc. The tooltip turns into `Pick: <item>`.
2. **Click a storage container that accepts it.** The tooltip confirms `Store In: <container>` when valid, or `Won't Accept: <container>` when the container's filter rejects the material.

A colonist will walk to the item, pick it up, and deliver it to the exact container you chose. After each successful order, the mode resets so you can issue another without leaving Store mode. Right-click to cancel back to Default.

This is different from clicking an item in **Default mode**, which also queues a pickup but lets the colonist drop off in *any* accepting container nearby — Store is for when you want the item in a specific locker.

### Relocate

The Relocate order is initiated from the **Storage Inspector** on a site container (it doesn't apply to the ship hold). Select a material, enter an amount (or click **All**), then click **↔ Relocate**. The inspector closes and the cursor enters Relocate mode:

- **Click another storage container** to move that quantity into it. The destination must accept the material.
- **Click an open tile** to drop the material as a loose stack on the ground. Workers will later pick it up like any loose material.

Tooltip feedback per hover target:

| Hover | Tooltip |
|---|---|
| Source container | "Source — Pick another container or open tile." |
| Accepting container | "Drop Into: <name>" |
| Rejecting container | "Won't Accept: <name>" |
| Walkable tile | "Drop on Ground" |
| Wall / solid entity / void | "Blocked" |

Right-click cancels Relocate mode. A worker will fulfil the relocation as a standard task: walk to source, withdraw, walk to destination, deposit. If the source runs short before the worker arrives, the worker takes what's left rather than canceling.

---

## Worker behaviour around storage

Workers respect tag filters automatically:

- When a worker drops items at a container, items the container rejects stay in the worker's bag — nothing vanishes.
- When a worker needs to deposit, they look for *any* accepting container, not just the closest. If the only nearby locker rejects what they're carrying, they walk further to find a fit.
- Once a container's player filter changes (you tick or untick a tag), the new rule applies to *future* deposits. Already-stored items aren't moved out.

This means typed containers are most useful when you've covered every category — pair a Mineral Bin with a Bio Silo and a Fuel Tank, and your workers will sort everything correctly without ever using a generic Storage Locker.
