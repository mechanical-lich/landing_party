Ready for review
Select text to add comments on the plan
landing_party — Campaign / Overworld Foundation
Context
landing_party is a fork of scifi_settlements (currently a byte-identical copy at /Users/johngodsey/repos/mechanical-lich-repos/landing_party). The goal is to turn the single-map colony sim into a procedurally-generated, story-driven game: you lead alien colonists from a space life-raft, spend fuel to travel a space overworld between planets/moons/asteroids, establish bases, gather/process materials (to make more fuel), and pursue quests (the repurposed win-condition system).

scifi_settlements today is single-level: one world.Level held as MainState.level, no level stack. Resources are not global — they are scanned on the fly from StorageComponent entities on the live level. Win conditions are declarative JSON per scenario. There is no overworld, no global ship stockpile, no colonist-spawn mechanic.

This plan covers Phase 1 (foundation) in executable detail and outlines Phase 2+.

Decisions made with the user
Ship = abstract hub (no walkable ship tilemap): campaign-level state holding the global stockpile, colonist roster, quest log, and overworld map screen.
Locations persist (paused): each visited location's full Level is serialized to its own file and resumed exactly as left. Only one Level is live at a time.
Resource model = ship-storage entities: keep the StorageComponent/entity model; the ship hold is a persistent collection of StorageComponent-bearing entities owned by a synthetic "ship" settlement living at the campaign level (not on any Level).
Logistics = ship hold + local: a landing party on a planet draws crafting/build materials from BOTH on-planet storage and the global ship hold (MultiProvider).
Replace legacy fully: campaign mode is the only mode; the single-level "New Settlement" start/save paths are removed.
Module path stays github.com/mechanical-lich/scifi_settlements in Phase 1 (a module rename is orthogonal churn; defer).
All paths below are relative to the landing_party repo root.

Phase 1 — Ordered implementation
Step 1 — New package internal/campaign (persistent hub model)
New files:

internal/campaign/campaign.go
type Campaign struct { Name string; Seed int64; CurrentLocationID string; Locations map[string]*Location; Ship *ShipState; Day int }
NewCampaign(name string, seed int64) *Campaign, (c *Campaign) CurrentLocation() *Location
internal/campaign/location.go
type Location struct { ID, Name, Kind, MapID, ScenarioID string; Seed int64; Discovered, Visited bool; FuelCost int; Summary, SaveFile, QuestTag, RevealQuest string }
A Location is a descriptor only — never holds a *world.Level. The level lives on disk (SaveFile) or is generated on demand from MapID/ScenarioID/Seed.
internal/campaign/ship.go
type ShipState struct { Hold []*world.SaveEntity; Roster []*world.SaveEntity; RosterCap int }
Hold = persistent StorageComponent entities owned by settlement "ship". Roster = colonist entities currently aboard (serialized, not on any level).
Reuse world.SaveEntity (internal/world/save.go:16) + world.RebuildEntity (save.go:299) verbatim for (de)serialization — they already round-trip StorageComponent/InventoryComponent. No new serialization code.
internal/campaign/overworld_def.go
LoadOverworldDefs() ([]Location, error) reading new data/overworld.json, mirroring how internal/mapdef / internal/scenario load from data/.
campaign importing world is safe (no cycle — world does not import campaign).

Step 2 — Shared storage accessor (remove the implicit s.level assumption)
Highest-leverage refactor; must precede travel because crafting/research/fuel must read the ship hold while colonists work on a planet Level.

New package internal/storage (internal/storage/storage.go):

type Provider interface { Sources() [][]*ecs.Entity }
CountResource(p, ownedBy, blueprint) int, CountAll(p, owners []string, names) map[string]int, Check(p, owners []string, cost map[string]int) bool, Deduct(p, owners []string, cost)
Implementations: LevelProvider{*world.Level}, ShipProvider{*campaign.ShipState} (rebuilds/caches hold entities), MultiProvider{[]Provider}.
Move the bodies of checkSettlementStorageForCraft / deductFromSettlementStorage / deductFromStorageList (internal/ai/worker.go:707-797) into internal/storage, generalized over Provider. Accept a set of accepted owners {colonyName, "ship"} so ship-hold entities (OwnedBy == "ship") and colony entities both match without mutating ownership on travel.

Inject the provider into worker AI via a package hook in internal/ai (var StorageProviderFor func(level *world.Level) storage.Provider), set by MainState to MultiProvider{LevelProvider, ShipProvider} (logistics decision: ship hold + local). Deduct order: ship-hold first, then on-planet local.

Modify callsites to route through storage.*:

internal/ai/worker.go:707,718
internal/game/main_state.go:1633 (win-condition counts), :872, :997, :1516
internal/gui/hud_screen.go:1474 + resource-summary HUD
Step 3 — WorldManager (live-level swap)
New file internal/game/world_manager.go:

type WorldManager struct { campaign *campaign.Campaign }
Freeze(s *MainState) error: serialize live level to saves/campaign/<name>/loc_<id>.json.gz via a scoped save (Step 5); persist a small LocationRuntime blob (camera CameraX/Y/Z, buildMode, follow); set loc.SaveFile, loc.Visited=true. Invalidate outstanding entity-bearing tasks before serialize (mirror HostileAIComponent.Path = nil, save.go:209).
Activate(locID string, party []*world.SaveEntity) (*MainState, error):
If loc.SaveFile != "": world.LoadLevelFromFile (save.go:287) → newMainStateFromLevel (main_state.go:225).
Else: generate via extracted generateLevelForLocation(loc) — refactor: extract the BuildWorld + scenario/setup-script block (main_state.go:368-487) out of MainState.newGame() into a shared function so both campaign-start and Activate use it (pure generation.BuildWorld, no global side effects — internal/generation/world_builder.go:32).
Beam party colonists onto the level near the landing plaza (reuse findStartingPlaza, main_state.go:427), tagging each with SettlementComponent{colonyName} + WorkerComponent (as main_state.go:480-485).
Restore camera from LocationRuntime.
Rebuild a fresh MainState per location via newMainStateFromLevel (main_state.go:225) rather than mutating one MainState in place — it cleanly re-creates guiManager, gm, systemManager, lighting, winEval.
Hazard: newMainStateBase re-registers event listeners on the global event.GetQueuedInstance() (main_state.go:192-221) every swap → listener leak / double-dispatch. Add scoped deregistration in a new MainState.teardown() called from Done(). If mlge/event has no unregister API, add one (audit mlge/event).

Step 4 — OverworldState (the SpaceMap screen)
New file internal/game/overworld_state.go, modeled on internal/game/title_state.go (same state.StateInterface + minui retained UI + done/next idiom). The mlge state machine is a queue, not a stack (mlge/state/statemachine.go:31) — transition by full state replacement (s.next + s.done=true), as Title↔Main already do (main_state.go:641-668).

Lists Discovered locations: name, Summary, FuelCost, current fuel, a landing-party size picker (bounded by RosterCap & roster size), and a Travel button disabled when fuel < FuelCost or party empty.
On Travel: wm.Freeze(currentMS) → pop selected roster → party []*world.SaveEntity → deduct fuel (Step 6) → ms,_ := wm.Activate(locID, party) → o.next=ms; o.done=true.
Add a "Star Map" affordance in MainState: new gui.OpenOverworldEventType (mirror gui.MainMenuEventType wiring main_state.go:205,636-642) → handler does wm.Freeze(s) then s.next = NewOverworldState(...); s.done = true.
Reuse minui widgets from TitleState (title_state.go:164); no new UI toolkit.
Step 5 — Save format → multi-file campaign bundle
Riskiest data change. Today world.SaveLevel embeds the package-global settlement.Settlements into every save (save.go:239) and load clobbers the global (save.go:283).

Layout: saves/campaign/<name>/campaign.json.gz (Campaign + ShipState + locations + fuel-cache) and per-location loc_<id>.json.gz.
New internal/game/campaign_saves.go: SaveCampaign(c) (Freezes current level first), LoadCampaign(name) — reuse gzip helpers marshalSaveFile/unmarshalSaveFile (saves.go:60-85).
Add world.SaveLevelScoped(level, settlementNames) + world.LoadSaveDataScoped that merge into the global map instead of replacing (save.go:283). On Activate, register the active colony settlement(s); on Freeze, extract & remove them so a different location's load sees no stale settlements. The "ship" settlement is owned by Campaign, never written into a location save.
Scope Settlement.Tasks (settlement.go:18) per-location (serialized in the location save). Task.Data carries *ecs.Entity (main_state.go:152) — pointer identity will not survive freeze/thaw; drop/invalidate entity-bearing tasks on Freeze (workers re-derive). Verify mlge/task JSON-serializability.
Audit all readers of settlement.Settlements: worker.go:148, save.go:239,283, settlement.NewSettlement.
Remove legacy single-level paths in internal/game/saves.go (SaveSettlement/LoadSave single-level) and the "New Settlement" title flow.
Step 6 — Fuel as a resource gating travel
Add fuel as an ordinary item blueprint (data) so it stacks in StorageComponent and round-trips via save.go for free.
Source of truth = fuel units in the ship hold (owner "ship"). OverworldState reads via storage.CountResource(ShipProvider, "fuel"); Travel deducts via storage.Deduct.
Production: add a refine_fuel recipe to data/crafting_recipes.json (internal/crafting) — input ore/biomass → output fuel. The existing crafting loop (worker.go:707-726) produces it with no new systems. Optionally gate behind a tech in data/research.json.
"Fuel: N (need M)" readout in OverworldState and HUD.
Step 7 — Beaming colonists (entity ↔ roster)
New internal/campaign/beam.go:

BeamDown(c, level, rosterIdx, x,y,z) []*ecs.Entity: world.RebuildEntity each roster SaveEntity, set Position, add SettlementComponent{colonyName} + WorkerComponent, level.AddEntity, remove from roster.
BeamUp(c, level, entities): capacity check vs ShipState.RosterCap; open-to-sky rule — only if the colonist's tile has no blocking Ceiling above (world.Tile ceiling slot, save.go:103-105). Convert entity → world.SaveEntity via exported world.EntityToSaveEntity (export the existing unexported entityToSaveEntity, save.go:117), append to roster, remove from level. Clear stale WorkerComponent.CurrentTask (mirror cleanUpSystem, main_state.go:127-130).
Beam-up trigger: a gui event + button in the colonist modal (hud_screen.go:815 ShowColonistModal) → new MainState.HandleEvent case.
Step 8 — Wire campaign through start flow & MainState
internal/game/title_state.go: replace "New Settlement" with "New Expedition" → build Campaign (Step 1) from data/overworld.json, generate the start location's level via extracted generateLevelForLocation, set ms.campaign, ms.wm. "Load" becomes "Load Expedition" → LoadCampaign.
internal/game/main_state.go: add fields campaign *campaign.Campaign, wm *WorldManager; thread through newMainStateBase/newMainStateFromLevel. Register gui.OpenOverworldEventType (main_state.go:205) + HandleEvent case (main_state.go:636). On SaveGameEvent (main_state.go:644) call SaveCampaign.
Data-driven overworld
New data/overworld.json (loaded by internal/campaign/overworld_def.go):

{
  "locations": [
    { "id": "wreck_site", "name": "Crash Site", "kind": "planet",
      "map_id": "earth_like", "scenario_id": "survival",
      "fuel_cost": 0, "discovered": true,
      "summary": "Where your pod went down." },
    { "id": "ice_moon", "name": "Glacial Moon", "kind": "moon",
      "map_id": "moon", "scenario_id": "moon",
      "fuel_cost": 12, "discovered": false,
      "summary": "Frozen. Rumored fuel ice.", "reveal_quest": "find_starmap" }
  ]
}
map_id/scenario_id reuse existing data/maps/*.json + data/scenarios/*.json and the BuildWorld pipeline unchanged. discovered:false locations stay hidden in OverworldState until revealed (reveal mechanism = Phase 2).

Phase 2+ (outline)
Quests from win conditions: generalize internal/wincondition so a satisfied Rule emits a campaign event via the existing-but-unwired internal/eventsystem instead of just pausing (main_state.go:1661-1664). Add QuestLog on Campaign + a quest-log modal (host on the Goals tab / a modal — hud_screen.go:272, RefreshGoalsTab:1171). Quest completion grants fuel/items/blueprints, sets Location.Discovered (drives reveal_quest), or advances lore. winEval/ EvalContext (main_state.go:1650) already aggregates the needed counts.
Bio-printer: a ship-level "recipe" consuming hold biomass → new roster colonist (factory.Create("colonist",...) then world.EntityToSaveEntity). Pure reuse of crafting + roster machinery; UI = a ship modal. Research-gated.
Lore / location variety: expand data/overworld.json (biome/scenario variety, per-location Seed, lore hooks via internal/lore); quest-revealed expansion.
Key files to modify / create
Create: internal/campaign/{campaign,location,ship,beam,overworld_def}.go, internal/storage/storage.go, internal/game/{world_manager,overworld_state,campaign_saves}.go, data/overworld.json, data/crafting_recipes.json (+refine_fuel).

Modify: internal/game/main_state.go (extract generateLevelForLocation, campaign fields, overworld event/handler, teardown, save), internal/world/save.go (scoped save/load, export EntityToSaveEntity), internal/ai/worker.go (storage provider), internal/game/saves.go (remove legacy), internal/game/title_state.go (expedition flow), internal/gui/{gui_manager,hud_screen}.go (overworld + beam events).

Risks / hazards
settlement.Settlements global clobbered on load (save.go:283) → scoped/merged; "ship" settlement never enters a location save. Highest data-corruption risk.
Implicit s.level resource scans (worker.go:729-797, main_state.go:1633, …) → abstract behind storage.Provider before travel.
Event-listener leak across level swaps (main_state.go:192-221) → scoped deregistration in MainState.teardown(); possibly add mlge/event unregister API.
Task.Data holds *ecs.Entity — invalidate entity-bearing tasks on Freeze.
Camera/systems restore on swap → fresh MainState via newMainStateFromLevel + LocationRuntime blob, not in-place mutation.
State machine is a queue not a stack → use s.next+s.done full replacement.
Verification (end-to-end)
cd landing_party && make build then make run.
Title → "New Expedition" → start at wreck_site; confirm it generates and plays like the old single map (regression: gather, build, craft).
Open Star Map (new affordance); confirm only discovered locations show, fuel and per-location fuel cost render, Travel disabled when fuel < cost.
Refine fuel via refine_fuel recipe; confirm ship-hold fuel count rises and the overworld readout updates.
Beam a subset of colonists to roster (open-to-sky enforced), Travel to ice_moon; confirm fuel deducted, new level generates, party beams down at the plaza, ship-hold resources still usable for crafting on the moon (MultiProvider).
Travel back to wreck_site; confirm it resumes exactly as left (mined tiles, built structures, dropped items, colonist positions) — proves paused persistence.
Save, quit, LoadCampaign; confirm campaign + both locations + ship hold/roster + fuel restore correctly. Run go test ./... (esp. internal/path, internal/world).