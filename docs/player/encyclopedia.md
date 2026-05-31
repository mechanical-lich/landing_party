# Encyclopedia

A campaign-wide bestiary of every entity your colonists have encountered. Reached from the Star Map → **Encyclopedia** button.

The button is visible from the start of a campaign but **disabled** until you research [Archive Indexing](research.md#archive-indexing) at a research lab. Discovery still happens before the research — every entity you hover (in default cursor mode) is silently recorded. The moment Archive Indexing finishes, the catalogue is already populated with everything you've seen so far.

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
│ │ Alien Crystal│           Faction: alien               │
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

Left pane is the sorted catalogue. Right pane shows the icon, name, faction, tag chips, and the long-form description. Entities with no recorded lore yet show "No description recorded." — the entry still counts, so the discovery total is accurate.

---

## Controls

| Input | Action |
|---|---|
| Click an entry | Show its details on the right |
| Back button / Esc | Return to the Star Map |

The screen is read-only.
