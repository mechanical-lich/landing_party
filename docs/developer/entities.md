# Entities & Blueprints — Developer Guide

All entities are defined as JSON blueprints under `data/blueprints/`. The loader (`factory.FactoryLoadDir`) walks the entire directory tree at startup and merges every `.json` file into a single blueprint registry. The directory layout is convention only — file location does not affect lookup, but blueprint IDs must be globally unique.

```
data/blueprints/
    entities/      # colonists.json, aliens.json, critters.json, flora.json
    items/         # weapons.json, armor.json, consumables.json, resources.json
    structures/    # workbenches.json, doors.json, furniture.json
```

Each file is a JSON object keyed by blueprint ID; each value is a map of component name → component fields.

---

## Blueprint Structure

```json
"alien_grunt": {
    "Description": {
        "Name": "Alien Grunt",
        "Faction": "alien"
    },
    "Appearance": {
        "R": 120, "G": 220, "B": 120,
        "Bounces": true,
        "Resource": "scifi_entities",
        "SpriteX": 0, "SpriteY": 360
    },
    "Health": { "MaxHealth": 12 },
    "Stats": { "Str": 8, "Int": 4, "Dex": 12, "AC": 8, "BasicAttackDice": "1d6" },
    "FactionAI": { "BehaviorKey": "hostile", "Faction": "alien" },
    "Initiative": { "DefaultValue": 12, "Ticks": 1 },
    "AIMemory": {},
    "Solid": {}
}
```

---

## Core Components

### Description

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | Display name shown in the HUD |
| `Faction` | string | Faction membership (`"colony"`, `"alien"`, etc.) |

### Appearance

| Field | Type | Description |
|-------|------|-------------|
| `R`, `G`, `B` | int | RGB color tint (0–255) applied to the sprite |
| `Bounces` | bool | Whether the sprite plays a bounce animation |
| `Resource` | string | Sprite sheet asset key (from `data/assets.json`) |
| `SpriteX`, `SpriteY` | int | Pixel offset into the sprite sheet |

### Health

| Field | Type | Description |
|-------|------|-------------|
| `MaxHealth` | int | Maximum (and starting) hit points |

### Stats

| Field | Type | Description |
|-------|------|-------------|
| `Str` | int | Strength — affects physical task speed and melee damage |
| `Int` | int | Intelligence — affects research and crafting speed |
| `Dex` | int | Dexterity — affects gathering and fine manipulation speed |
| `AC` | int | Armor class — reduces incoming damage |
| `BasicAttackDice` | string | Dice expression for unarmed attacks (e.g. `"1d4"`) |

Stats act as **multipliers on task duration**: higher stat = faster task completion.

### Worker

Marks an entity as a colonist worker subject to `WorkerSystem`. Required for any entity that should receive and execute tasks.

```json
"Worker": {}
```

### Hunger

| Field | Type | Description |
|-------|------|-------------|
| `Energy` | float | Current energy level |
| `MaxEnergy` | float | Energy cap |
| `DrainRate` | float | Energy lost per turn |
| `HungerThreshold` | float | Energy level at which the worker seeks food |
| `StarveThreshold` | float | Energy level at which the worker takes damage |

When `Energy < HungerThreshold`, the worker interrupts its current task to find food. When `Energy < StarveThreshold`, it takes damage each turn.

### FactionAI

Controls hostile NPC behavior. Used by alien entities.

| Field | Type | Description |
|-------|------|-------------|
| `BehaviorKey` | string | Lookup key into the behavior profile (currently `"hostile"`) |
| `Faction` | string | Faction identifier used for friend/foe resolution |

See [AI Systems](ai.md) for behavior details.

### Initiative

Controls turn order and action rate.

| Field | Type | Description |
|-------|------|-------------|
| `DefaultValue` | int | Base initiative value; higher acts sooner |
| `Ticks` | int | Number of actions per initiative cycle |

### Inventory

| Field | Type | Description |
|-------|------|-------------|
| `StartingInventory` | []string | List of blueprint IDs the entity starts with in its inventory |

Equipped slots: `RightHand`, `LeftHand`, `Head`, `Torso`, `Legs`, `Feet`. Two-handed weapons (see `Weapon.TwoHanded`) occupy both hand slots; equipping one bumps anything in the off-hand to the bag, and a second weapon cannot be added until the two-hander is unequipped.

### Weapon

Added to item blueprints that deal damage in combat.

| Field | Type | Description |
|-------|------|-------------|
| `AttackBonus` | int | Flat bonus added to attack rolls |
| `AttackDice` | string | Damage dice expression (e.g. `"2d6"`) |
| `DamageType` | string | Damage type (`"ballistic"`, `"energy"`, `"fire"`, etc.) |
| `Range` | int | Attack range in tiles (0 = melee) |
| `Ranged` | bool | Whether the weapon fires a projectile |
| `Display` | string | Sprite lookup key for `EquipmentAppearance.WeaponColumns`; defaults to the blueprint ID if empty |
| `TwoHanded` | bool | If true, equipping this weapon clears the off-hand slot and prevents dual-wielding |

### Armor

Added to item blueprints that provide defense.

| Field | Type | Description |
|-------|------|-------------|
| `DefenseBonus` | int | Flat bonus added to defense rolls |
| `StoppingPower` | int | Damage absorbed per hit before the remainder is applied |
| `Resistances` | []string | Damage types this armor resists |
| `Tags` | []string | Labels used by `EquipmentAppearance` row scoring (e.g. `"light_helm"`, `"heavy_chest"`) |

### EquipmentAppearance

An alternative to `Appearance` for entities whose sprite changes based on equipped weapons and armor (i.e. colonists). `ResolveSprite` selects a column and row from a single sprite block at runtime.

| Field | Type | Description |
|-------|------|-------------|
| `Resource` | string | Sprite sheet asset key |
| `SpriteSize` | int | Pixel size of one sprite cell |
| `BlockOriginX`, `BlockOriginY` | int | Pixel offset of the sprite block's top-left corner in the sheet |
| `AnimationFrames` | int | Frames per animation cycle (columns consumed per weapon column) |
| `DefaultWeaponColumn` | int | Column index used when no weapon is equipped |
| `DefaultArmorRow` | int | Row index used when no armor tags match |
| `WeaponColumns` | map[string]int | Maps weapon `Display` key → column index within the block |
| `ArmorRows` | []ArmorRowConfig | Ordered list of `{ Row, Tags }` entries; the row whose tags overlap most with equipped armor tags is selected |

**Column** is determined by the `Display` field of the equipped weapon (right hand first, then left hand). If `Display` is empty the weapon's blueprint ID is used as the key. Column lookup priority:

1. Exact key match in `WeaponColumns`
2. `"*"` wildcard entry in `WeaponColumns` — catches any weapon not explicitly listed
3. `DefaultWeaponColumn` — used when no weapon is equipped at all

**Row** is determined by a best-fit score across all armor tags on equipped head, torso, leg, and foot slots. The row whose `Tags` have the most overlap with the entity's equipped armor tags wins. Ties go to the first matching row. `DefaultArmorRow` is used when no rows are configured or all scores are zero.

```json
"EquipmentAppearance": {
    "Resource": "scifi_entities",
    "SpriteSize": 24,
    "BlockOriginX": 0,
    "BlockOriginY": 192,
    "AnimationFrames": 2,
    "DefaultWeaponColumn": 0,
    "DefaultArmorRow": 0,
    "WeaponColumns": {
        "pistol": 3,
        "rifle": 1,
        "shotgun": 2,
        "*": 1
    },
    "ArmorRows": [
        { "Row": 0, "Tags": [] },
        { "Row": 1, "Tags": ["light_helm"] },
        { "Row": 2, "Tags": ["heavy_helm", "heavy_chest"] }
    ]
}
```

Only one of `Appearance` or `EquipmentAppearance` should be present on an entity. The renderer checks `EquipmentAppearance` first.

### Light

| Field | Type | Description |
|-------|------|-------------|
| `Level` | int | Brightness of emitted light (0–100) |
| `Range` | int | Radius in tiles that the light reaches |

### AIMemory

Empty marker component. Required for entities that use faction AI to track remembered targets.

```json
"AIMemory": {}
```

### Solid

Empty marker component. Makes the entity impassable — other entities cannot move through it.

```json
"Solid": {}
```

---

## Adding a New Entity

1. Pick the appropriate file under `data/blueprints/` (or create a new `.json` file in the relevant subdirectory). The subdirectory is convention; the loader merges all files.
2. Add a new top-level key — this is the **blueprint ID** and must be unique across the entire `data/blueprints/` tree.
3. Add the required components for your entity type:
   - **Hostile NPC** (`data/blueprints/entities/aliens.json`): `Description`, `Appearance`, `Health`, `Stats`, `FactionAI`, `Initiative`, `AIMemory`, `Solid`
   - **Colonist** (`data/blueprints/entities/colonists.json`): `Description`, `EquipmentAppearance`, `Health`, `Stats`, `Worker`, `Hunger`, `Initiative`, `Light`, `Inventory`, `Solid`
   - **Item** (`data/blueprints/items/*.json`): `Description`, `Appearance`, `Item`, plus `Weapon` / `Armor` / consumable components. No `Solid` — items are walkable.
   - **Interactive Structure** (`data/blueprints/structures/*.json`): `Description`, `Appearance`, `Solid` (usually), plus role components like `CraftingStation`, `StorageContainer`, `Door`, `Light`. Reference the blueprint from `data/build.json` to make it player-buildable.
   - **Plain Tile Build** (no entity needed): just add an entry to `data/build.json` with a tile `type`.
4. Reference the blueprint ID in `data/build.json` (with `is_entity: true`) if it should be player-buildable, or in a scenario's `spawn_rules` / setup script if it should appear during gameplay.

---

## Existing Entity Categories

| Category | Blueprint IDs |
|----------|--------------|
| Hostile aliens | `alien_grunt`, `alien_scout`, `alien_brute` |
| Ambient creatures | `critter` |
| Colonists | `colonist` |
| Structures (entities) | `airlock`, `blast_door`, `storage_locker`, `research_lab`, `workbench`, `work_light` |
| Items — weapons (one-handed) | `laser_pistol`, `stun_baton`, `plasma_cutter`, `knife`, `pistol_shield`, `energy_sword`, `laser_sword`, `flag_rifle_red`, `flag_rifle_blue` |
| Items — weapons (two-handed) | `rifle`, `shotgun`, `autorifle`, `rocket_launcher`, `flamethrower` |
| Items — armor | `combat_vest`, `enviro_suit`, `helmet` |
| Items — consumables | `ration_pack`, `protein_bar`, `medkit`, `hydration_gel` |
| Resources | `metal_ore`, `crystal`, `stone` |
