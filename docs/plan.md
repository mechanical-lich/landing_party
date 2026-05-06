# scifi_settlements — Feature Plan

## Status key
- `[ ]` not started
- `[~]` partial / scaffolded
- `[x]` complete

---

## 1. Dynamic Lighting

Port `LightingSystem` from fantasy_settlements. Computes per-tile light levels from a sun value (based on time of day / Z level) plus point-light sources on entities.

**Tasks**
- [ ] Add `LightComponent` alias to `components/component_types.go` (already in `rlcomponents`)
- [ ] Port `internal/systems/LightingSystem.go` — sun shadow calc, entity light sources, parallel tile update
- [ ] Wire `LightingSystem` into `newMainStateBase` (runs each tick)
- [ ] Darken tile/entity draw ops based on `tile.LightLevel` in `drawing.go`
- [ ] Add `LightComponent` to relevant blueprints (colonist helmet light, research lab, etc.)

**Dependencies:** none

---

## 2. Hunger / Food System

Colonists and hostile AI have an energy/hunger stat that drains each turn. When starving, health degrades. Colonists seek food autonomously; player must ensure food supply exists.

**Tasks**
- [ ] Add `HungerComponent` to `components/` — fields: `Energy int`, `MaxEnergy int`, `HungerThreshold int`, `StarveThreshold int`
- [ ] Register `HungerComponent` in `factory/component_registry.go`
- [ ] Add `hunger` AI state to `WorkerSystem` / worker AI — when `Energy < HungerThreshold`, interrupt current task and seek food items
- [ ] Add `FoodComponent` alias to `component_types.go` (already in `rlcomponents`) — marks entities as edible with nutrition value
- [ ] Add food blueprints to `entity_blueprints.json`: `ration_pack`, `protein_bar`, `hydration_gel`
- [ ] Add `eat` handler to worker AI: move to food item, consume it (remove from level, restore Energy)
- [ ] Tick hunger drain in `WorkerSystem.UpdateEntity` each turn
- [ ] When `Energy < StarveThreshold`, deal 1 HP damage per N turns
- [ ] Show hunger state in entity detail panel (HUD)
- [ ] Add food item drops to critter blueprint (`Drops` component)
- [ ] Add food to colonist starting inventory (`StartingInventory` in blueprint)

**Dependencies:** none

---

## 3. Interactive Objects (Doors + Interactables)

`InteractComponent` marks entities that can be activated by adjacent colonists or the player. Doors already use `DoorSystem` from `rlsystems` but there's no general interact framework.

**Tasks**
- [ ] Add `InteractComponent` to `components/` — fields: `Script string` (optional behavior key), `Label string`
- [ ] Register in `factory/component_registry.go`
- [ ] Add `interact` cursor mode (`CursorModeInteract`) to `gui/cursor.go`
- [ ] Handle interact click in `main_state.go` — right-click on entity with `InteractComponent` triggers interaction
- [ ] Add `interact` task action to `task_requests/requests.go`
- [ ] Add `handleInteractTask` to `ai/worker.go` — worker walks to target, fires an interaction event
- [ ] Add `InteractedEvent` to `eventsystem/`
- [ ] Add airlock, console, and supply crate blueprints to `entity_blueprints.json`
- [ ] Add "Interact" cursor mode button to Build > Orders submenu in HUD

**Dependencies:** Scripting system (for script-driven interact behavior), but can stub without it

---

## 4. Scripting System (Mechanical Basic)

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

## 5. Crafting & Equipment

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

**Dependencies:** Hunger system (consumables), storage system (already done)

---

## 6. Scenario Setup Scripts

Specific scenarios need custom initialization logic beyond spawn rules — placing named entities, setting win flags, posting lore messages.

**Tasks**
- [ ] Write `data/scripts/scenarios/survival_setup.basic`
  - Post "Day 1. The colony pod has landed. Establish a perimeter."
  - Spawn 2 `alien_grunt` patrols at map edges
  - Set flag `survival_started = 1`
- [ ] Add `setup_scripts` field to `survival.json` pointing at the script
- [ ] (Future scenarios can add their own scripts without code changes)

**Dependencies:** Scripting system (#4)

---

## 7. HUD & Polish

Small gaps that affect playability.

**Tasks**
- [ ] Show day counter in sidebar Goals tab (already partially done — update to be prominent)
- [ ] Show hunger bar per colonist in Population tab
- [ ] Add "Craft" tab to sidebar (see #5)
- [ ] Win/lose modal — when win condition triggers, show outcome modal with result message and New Game / Quit buttons instead of just pausing
- [ ] Minimap widget in HUD — small corner minimap using the existing `minimap` package
- [ ] Save button shortcut (Ctrl+S)

---

## Implementation Order (suggested)

1. **Hunger / Food** — affects survival scenario immediately; self-contained
2. **Dynamic Lighting** — large visual/gameplay impact; mostly a port
3. **Interactive Objects** — doors + airlocks add exploration depth
4. **Scripting** — unlocks richer scenarios; dependency for #5 setup scripts
5. **Crafting & Equipment** — biggest scope; needs scripting + hunger done first
6. **Scenario Setup Scripts** — completes the scripting work
7. **HUD & Polish** — done incrementally alongside the above
