# scifi_settlements — Feature Plan

## Status key
- `[ ]` not started
- `[~]` partial / scaffolded
- `[x]` complete

---

## 1. Hunger / Food `[x]` DONE

`HungerComponent` implemented, hunger drain, eating behavior, food blueprints, worker hunger AI state all wired.

**Remaining**
- [ ] Show hunger bar per colonist in Population tab (HUD)
- [ ] Add food item drops to critter blueprints (`Drops` component)

---

## 2. Dynamic Lighting `[x]` DONE

`LightingSystem` ported and extended. Sun shadow calc, entity light sources, tile light sources (ore deposits, crystal veins, stairs), depth fog overlay, air-layer see-through rendering. Tile-source cache added to fix FPS regression.

---

## 3. Interactive Objects — Faction Doors `[x]` DONE

Airlock and blast_door blueprints with full faction-aware door system:
- `FactionDoorSystem` auto-opens/closes doors for authorized factions on approach and when occupying the tile
- `PathCostFunctionForFaction` routes colonists through their own doors
- `GetPossiblePathForEntity` uses colonist faction for A* cost + path validation
- Door sprite switches between open/closed frames based on `door.Open`
- `door.OwnedBy` set to builder's faction (not settlement name) so matching works correctly

**Remaining**
- [ ] General `InteractComponent` framework for non-door interactables (consoles, supply crates)
- [ ] `InteractedEvent` + `handleInteractTask` in worker AI

---

## 4. Terrain & World Generation `[x]` DONE

Rocky outcrops, underground caverns (second Perlin pass), cliff shadows via depth fog. Camera movement up/down Z levels. Air-layer transparency shows tiles below.

---

## 5. Smart Default Task (Right-click) `[x]` DONE

Right-click in default cursor mode queues a move task. On arrival, worker auto-detects:
- **Attack** — hostile entity with `HealthComponent` + `HostileAI`
- **Pick up** — item entity on tile (chains into dropoff/haul automatically)
- **Mine** — `ore_deposit` or `crystal_vein`
- **Dig** — any other solid tile
- **Scout** — nothing actionable, completes normally

---

## 6. Scripting System (Mechanical Basic) `[ ]`

Port the `.basic` scenario setup script runner from fantasy_settlements. Allows scenarios to spawn entities, set flags, and post messages at game start via a simple scripting language.

**Tasks**
- [ ] Verify `github.com/mechanical-lich/mechanical-basic` is in `go.mod` (add if not)
- [ ] Port `internal/game/setup_script.go` — `RunSetupScripts(scripts []string, level *Level)`
- [ ] Register script host functions: `spawn_entity`, `set_flag`, `get_flag`, `add_message`, `find_open_tile`, `get_open_x/y`, `get_z_count`, `rnd_int`, `num_to_str`
- [ ] Call `RunSetupScripts(sc.SetupScripts, s.level)` at end of `newGame()` after scenario select
- [ ] Add `ScriptComponent` to `components/` — `OnTurn string` callback key for per-turn entity scripts
- [ ] Add `ScriptSystem` to `systems/` — runs per-entity `OnTurn` script each turn
- [ ] Register `ScriptComponent` in `factory/component_registry.go`
- [ ] Wire `ScriptSystem` into `newMainStateBase`
- [ ] Create `data/scripts/scenarios/` folder
- [ ] Write `data/scripts/scenarios/survival_setup.basic` — posts opening message, optionally spawns initial alien patrol

**Dependencies:** `mechanical-basic` package

---

## 7. Crafting & Equipment `[ ]`

Colonists can craft items from stored resources. Crafted items go into inventory slots (weapons, armor) and improve combat effectiveness.

**Tasks**
- [ ] Design `data/crafting_recipes.json` — recipe ID, required materials (map[blueprint]count), output blueprint, build time
- [ ] Create `internal/crafting/` package — `LoadRecipes`, `GetRecipe`, `AllRecipes`
- [ ] Add `craft` task action to `task_requests/requests.go`
- [ ] Add `CraftRequest` struct — `RecipeID string`, `Progress/Required int`
- [ ] Add `handleCraftTask` to `ai/worker.go` — worker walks to workbench, consumes materials from storage, spawns output item into inventory
- [ ] Add workbench blueprint to `entity_blueprints.json` — `WorkbenchComponent` marks it as a craft station
- [ ] Add `WorkbenchComponent` to `components/` and register it
- [ ] Add sci-fi item blueprints to `entity_blueprints.json`:
  - Weapons: `laser_pistol`, `plasma_cutter`, `stun_baton`
  - Armor: `enviro_suit`, `combat_vest`, `helmet`
  - Consumables: `medkit`, `ration_pack`
- [ ] Add `data/equipment_table.json` — per-blueprint loot rolls (chance, max_picked, options)
- [ ] Load equipment table and apply on entity spawn in `factory/entityFactory.go`
- [ ] Add "Craft" tab to HUD sidebar — lists available recipes, queues craft task on click
- [ ] Show equipped items in entity detail panel (HUD)

**Dependencies:** Hunger system (done), storage system (done)

---

## 8. Scenario Setup Scripts `[ ]`

Specific scenarios need custom initialization logic beyond spawn rules.

**Tasks**
- [ ] Write `data/scripts/scenarios/survival_setup.basic`
  - Post "Day 1. The colony pod has landed. Establish a perimeter."
  - Spawn 2 `alien_grunt` patrols at map edges
  - Set flag `survival_started = 1`
- [ ] Add `setup_scripts` field to `survival.json` pointing at the script

**Dependencies:** Scripting system (#6)

---

## 9. HUD & Polish `[~]`

**Done**
- [x] Day/hour counter in title bar
- [x] Doors tab in build menu (airlock, blast door)

**Remaining**
- [ ] Show hunger bar per colonist in Population tab
- [ ] Win/lose modal — outcome modal with New Game / Quit buttons when win condition triggers
- [ ] Minimap widget — small corner minimap using the existing `minimap` package
- [ ] Save button shortcut (Ctrl+S)
- [ ] Add "Craft" tab to sidebar (see #7)

---

## Implementation Order (suggested)

1. **Scripting** — unlocks richer scenarios and interactables
2. **Scenario Setup Scripts** — completes scripting, gives survival scenario a proper start
3. **Crafting & Equipment** — biggest scope; hunger + storage already done
4. **HUD & Polish** — win/lose modal, minimap, hunger bar, Ctrl+S
