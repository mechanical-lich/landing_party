# Encyclopedia

A campaign-wide bestiary of every entity your colonists have encountered, plus the **Field Manual**. It is the **Encyclopedia** tab of the dashboard (**O** or the Star Map button).

The tab is always available. Every entity you hover (in default cursor mode) is silently recorded, so the catalogue fills as you play. ([Archive Indexing](research.md#archive-indexing) no longer gates it.)

---

## Field Manual

The catalogue opens with the Field Manual: short articles on the Star Map, the camera, colonists, orders, building, hauling, storage, needs, crafting, research and beaming. They are the reference behind the [guided first landing](tutorial.md), and they stay here with the tutorial on, off, finished or skipped. The tutorial's **Manual** button opens the article for the current step.

---

## What gets registered

Any entity with a `Description` component, the moment you hover it. That includes colonists, hostile creatures, items on the ground, flora, structures — anything the cursor's tooltip can read a name from.

Entities you never hover stay off the list, even if your colonists walk past them.

---

## Layout

```
┌─ Encyclopedia ─────────────────────────────────────────┐
│                                                         │
│  Discovered: 14                                         │
│  Catalogue                                              │
│ ┌──────────────┐   [icon]  Alien Crystal                │
│ │ Manual: ...  │           Faction: alien               │
│ │ Alien Crystal│                                        │
│ │ Alien Flora  │           Tags: flora, crystal_field   │
│ │ Bio-Printer  │                                        │
│ │ Colonist     │           A vivid crystalline growth   │
│ │ Crab Beast   │           clustered in shallow alien   │
│ │ ...          │           soil. Refracts light.        │
│ └──────────────┘                                        │
│                                                         │
│  [ Back to Star Map ]                                   │
└─────────────────────────────────────────────────────────┘
```

Left pane is the Field Manual followed by the sorted catalogue. Right pane shows the icon, name, faction, tag chips, and the long-form description, or a manual article's title and text. Entities with no recorded lore yet show "No description recorded." — the entry still counts, so the discovery total is accurate.

---

## Controls

| Input | Action |
|---|---|
| Click an entry | Show its details on the right |
| Another tab / Esc | Leave the Encyclopedia (Esc returns to the site or ship) |

The screen is read-only.
