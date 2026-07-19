package game

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mechanical-lich/landing_party/internal/audio"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/effect"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/game/listeners"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/objective"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/systems"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/view"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlsystems"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlworld"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/input"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/resource"
	"github.com/mechanical-lich/mlge/state"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/mlge/ui/minui"
)

type SettlementConfig struct {
	Name            string
	ScenarioID      string
	MapID           string // "" = random map
	Seed            int64  // 0 = pick a random seed at generation time
	LightingMode    string // "" = use scenario default
	LightingAmbient int    // only used when LightingMode == "fixed"

	// Campaign integration: when PartyEntities is non-empty, newGame beams
	// those colonists in at the plaza instead of spawning the default squad.
	PartyEntities []*world.SaveEntity
	ColonyName    string // settlement name to reuse across the campaign
	// CampaignMode suppresses the legacy 5-colonist auto-spawn; colonists
	// arrive only via the landing-party beam-down.
	CampaignMode bool
}

type MainState struct {
	level *world.Level
	// camera is the single source of truth for the on-screen world view: the
	// visible tile rect, displayed z, and (zoomable) tile/sprite pixel sizes.
	// All world→screen / screen→world math and framing goes through it.
	camera view.Camera
	// viewport is the camera's view rect, snapshotted each Draw and consumed next
	// frame by FOV clearing and positional audio.
	viewport view.Viewport
	// wheelActive is true while a scroll gesture is in progress. The wheel fires an
	// event every frame its delta changes, so one gesture is a burst with a
	// decaying momentum tail (and often an inertial rebound the opposite way). We
	// zoom once on the leading edge and ignore the rest until the wheel rests.
	wheelActive   bool
	CursorMode    gui.CursorModeType
	guiManager    *gui.GUIManager
	systemManager *ecs.SystemManager
	// bgSystemManager mirrors systemManager minus the render-only systems, used
	// to tick this level in the background (e.g. The Ship while you're planetside).
	bgSystemManager *ecs.SystemManager
	// fovSystem is held so the current viewport can be handed to it each frame for
	// bounding the Visible clear (it no longer reads a copy off Level).
	fovSystem        *systems.FOVSystem
	gm               *GameMaster
	selectedEntity   *ecs.Entity
	Paused           bool
	MainSettlement   *settlement.Settlement
	op               *ebiten.DrawImageOptions
	initiativeSystem *rlsystems.InitiativeSystem
	cleanUpSystem    *rlsystems.CleanUpSystem
	worldImage       *ebiten.Image
	buildMode        string
	tick             int
	day              int
	lastDay          int
	mouseDragging    bool
	lastDragTileX    int
	lastDragTileY    int
	hoverTileX       int
	hoverTileY       int
	hoverActive      bool
	settlementCfg    SettlementConfig
	done             bool
	next             state.StateInterface
	mapModal         *MapModal
	cheatModal       *CheatModal
	storageInspector *StorageInspectorModal
	// cursorTools dispatches the modal cursor modes (Relocate/Store/Drop/Pickup);
	// each tool owns its pending selection state. The named fields expose the
	// public entry points (Begin/Request) the storage inspector and colonist
	// modal call into.
	cursorTools       map[gui.CursorModeType]cursorTool
	relocateTool      *relocateTool
	storeTool         *storeTool
	dropTool          *dropTool
	pickupTool        *pickupTool
	smallMap          *SmallMapWidget
	followEntity      *ecs.Entity
	rogueEntity       *ecs.Entity
	rogueMoveTargetX  int
	rogueMoveTargetY  int
	rogueMoveTargetZ  int
	rogueMoveActive   bool
	rogueAutoMoveTick int
	roguePath         [][2]int
	campaign          *campaign.Campaign
	wm                *WorldManager
	dashboardBtn      *minui.Button
	// forceQuestEval requests an immediate quest re-check next Update (set when
	// a quest target dies) so completion isn't delayed by the periodic tick —
	// important in Rogue mode where ticks only advance per player action.
	forceQuestEval bool

	// saveNamesLoaded/saveNamesDirty gate the HUD save-list refresh so it reads
	// the saves dir once and then only after a save, not every refreshHUD.
	saveNamesLoaded bool
	saveNamesDirty  bool
}

const (
	dashboardBtnW = 120
	dashboardBtnH = 26
)

// sharedListenersOnce guards process-wide singleton listeners against being
// re-registered on every campaign level swap.
var sharedListenersOnce sync.Once

// teardown unregisters this MainState's per-instance listeners so a swapped-out
// or parked level's state no longer receives input/gui events.
func (s *MainState) teardown() {
	event.GetQueuedInstance().UnregisterListenerFromAll(s)
}

// reattach re-registers listeners for a parked MainState being resumed.
func (s *MainState) reattach() {
	s.registerListeners()
}

// registerListeners wires this MainState to the input/gui event types it
// handles. Split out so a parked location can be re-attached on Resume.
func (s *MainState) registerListeners() {
	eq := event.GetQueuedInstance()
	eq.RegisterListener(s, input.MouseClickEventType)
	eq.RegisterListener(s, input.MouseReleasedEventType)
	eq.RegisterListener(s, input.KeyPressEventType)
	eq.RegisterListener(s, input.MouseWheelEventType)
	eq.RegisterListener(s, gui.BuildOptionChangedEventType)
	eq.RegisterListener(s, gui.CursorModeChangedEventType)
	eq.RegisterListener(s, gui.MainMenuEventType)
	eq.RegisterListener(s, gui.SaveGameEventType)
	eq.RegisterListener(s, gui.LoadGameEventType)
	eq.RegisterListener(s, gui.CraftRequestedEventType)
	eq.RegisterListener(s, gui.StationClickedEventType)
	eq.RegisterListener(s, gui.ResearchStationClickedEventType)
	eq.RegisterListener(s, gui.ResearchRequestedEventType)
	eq.RegisterListener(s, gui.ColonistSelectedEventType)
	eq.RegisterListener(s, gui.EquipItemRequestedEventType)
	eq.RegisterListener(s, gui.UnequipItemRequestedEventType)
	eq.RegisterListener(s, gui.SetTaskFilterEventType)
	eq.RegisterListener(s, gui.DropOffRequestedEventType)
	eq.RegisterListener(s, gui.PickupRequestedEventType)
	eq.RegisterListener(s, gui.SetSelfDefendEventType)
	eq.RegisterListener(s, gui.EnterRogueModeEventType)
	eq.RegisterListener(s, gui.ExitRogueModeEventType)
}

// registerLevelListeners wires the simulation listeners onto THIS level's own
// event bus (not the global one), so each planet handles its own kills,
// structures, and research independently — the basis for background simulation.
// Called once when the level is associated with the MainState; the level bus is
// never torn down on park, so a parked planet keeps handling its own events.
// Must run after s.level is set. (MessageListener stays global; the dead
// TaskCompleted listener and listener-less EntityDied/ItemStored are omitted.)
func (s *MainState) registerLevelListeners() {
	if s.level == nil || s.level.Events == nil {
		return
	}
	ev := s.level.Events
	ev.RegisterListener(s, eventsystem.ResearchDone) // state: unlock tech
	ev.RegisterListener(&listeners.KillListener{}, eventsystem.EntityKilled)
	ev.RegisterListener(&listeners.StructureListener{}, eventsystem.StructureBuilt)
	ev.RegisterListener(&listeners.ResearchListener{}, eventsystem.ResearchDone)
}

func newMainStateBase(cfg SettlementConfig) (*MainState, error) {
	gcfg := config.Global()
	s := &MainState{
		CursorMode:      gui.CursorModeDefault,
		systemManager:   &ecs.SystemManager{},
		bgSystemManager: &ecs.SystemManager{},
		op:              &ebiten.DrawImageOptions{},
		// Camera X/Y/Z start at 0; newGame / newMainStateFromLevel frame it on the
		// spawn once the level exists. Tile/sprite/canvas sizes come from config.
		camera: view.Camera{
			TileW: gcfg.TileSizeW, TileH: gcfg.TileSizeH,
			SpriteW: gcfg.SpriteSizeW, SpriteH: gcfg.SpriteSizeH,
			CanvasW: gcfg.WorldWidth, CanvasH: gcfg.WorldHeight,
		},
		settlementCfg: cfg,
		buildMode:     "hull_wall",
	}
	s.initCursorTools()

	// addLogic registers a system on both the live and background managers;
	// addRender only on the live one (render-only systems are skipped when
	// ticking a level in the background). Shared instances so stateful systems
	// keep one state.
	addLogic := func(sys ecs.SystemInterface) {
		s.systemManager.AddSystem(sys)
		s.bgSystemManager.AddSystem(sys)
	}
	addRender := func(sys ecs.SystemInterface) { s.systemManager.AddSystem(sys) }

	s.initiativeSystem = &rlsystems.InitiativeSystem{
		Speed: 1,
		OnEntityTurn: func(entity *ecs.Entity) {
			if entity.HasComponent(components.Appearance) {
				ac := entity.GetComponent(components.Appearance).(*components.AppearanceComponent)
				ac.Bounce = !ac.Bounce
			}
		},
	}
	addLogic(s.initiativeSystem)

	s.cleanUpSystem = &rlsystems.CleanUpSystem{
		OnEntityDead: func(levelInterface rlworld.LevelInterface, entity *ecs.Entity) {
			level := levelInterface.(*world.Level)
			// Death cry: entity's own "death" sound (or the generic scream clip),
			// played positionally at the corpse.
			if entity.HasComponent(rlcomponents.Position) {
				dpc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				deathClip := components.ResolveSound(entity, components.SoundDeath)
				level.EmitSoundClip(dpc.GetX(), dpc.GetY(), dpc.GetZ(), 8, world.SoundTagScream, deathClip, entity)
			}
			if s.campaign != nil && entity.HasComponent(components.QuestTarget) {
				qt := entity.GetComponent(components.QuestTarget).(*components.QuestTargetComponent)
				s.campaign.MarkTargetKilled(qt.QuestID)
				s.forceQuestEval = true
			}
			if entity.HasComponent(components.Worker) {
				wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
				if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
					wc.CurrentTask.Stop()
					wc.CurrentTask = nil
				}
			}
			if entity.HasComponent(components.Drops) {
				dropsC := entity.GetComponent(components.Drops).(*components.DropsComponent)
				pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				killer := findAdjacentWorker(level, pc.GetX(), pc.GetY(), pc.GetZ())
				isChoppable := entity.HasComponent(components.Choppable)
				for _, drop := range dropsC.Items {
					dropEntity, err := factory.Create(drop, pc.GetX(), pc.GetY(), pc.GetZ())
					if err != nil {
						continue
					}
					if isChoppable {
						// Harvested resources drop to ground and get a Retrieve task
						// so dedicated haulers can collect them.
						level.AddEntity(dropEntity)
						if killer != nil && killer.HasComponent(components.Settlement) {
							sc := killer.GetComponent(components.Settlement).(*components.SettlementComponent)
							if ms, ok := settlement.Settlements[sc.Name]; ok {
								ms.Tasks.AddTask(&task.Task{
									X: pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(),
									Action:  task_requests.RetrieveAction,
									Data:    task_requests.RetrieveRequest{Item: dropEntity},
									Created: time.Now(),
								})
							}
						}
					} else {
						// Combat drops go straight to the adjacent killer's bag
						// so they can auto-equip useful gear immediately.
						if killer != nil && killer.HasComponent(rlcomponents.Inventory) {
							inv := killer.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
							inv.AddItem(dropEntity)
						} else {
							level.AddEntity(dropEntity)
						}
					}
				}
				if !isChoppable && killer != nil && killer.HasComponent(rlcomponents.Inventory) {
					inv := killer.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
					inv.EquipAllBest()
				}
			}
		},
	}

	aiSystem := rlsystems.NewAISystem()
	aiSystem.GetPath = func(levelInterface rlworld.LevelInterface, from, to rlworld.TileInterface, reuse []int) []int {
		return fspath.GetPossiblePath(levelInterface.(*world.Level), from.(*world.Tile), to.(*world.Tile), reuse)
	}
	addLogic(aiSystem)
	addLogic(&systems.HearingSystem{})
	addLogic(&systems.VisionSystem{})
	addLogic(&systems.SmellSystem{})
	addLogic(systems.NewFactionAISystem())
	addLogic(&systems.ScriptedAISystem{})
	// ScentSystem runs after the AI systems so passive emission happens
	// at the entity's new tile (post-move). UpdateSystem (decay+diffuse)
	// still runs at the start of stepWorld regardless of registration order.
	addLogic(&systems.ScentSystem{})
	addRender(&systems.EmoteSystem{})
	addLogic(&systems.NeedsSystem{})
	addLogic(&systems.WorkerSystem{})
	addLogic(&systems.RadiationSystem{})
	addRender(&systems.LightingSystem{})
	// FOVSystem is a light AI dependency, not purely render: findBed picks the
	// nearest *visible* bed, so background colonists need FOV to discover beds.
	s.fovSystem = &systems.FOVSystem{}
	addLogic(s.fovSystem)
	addLogic(&rlsystems.DoorSystem{AppearanceType: components.Appearance})
	addLogic(&systems.FactionDoorSystem{})
	addLogic(&systems.ScriptSystem{})
	addLogic(&rlsystems.StatusConditionSystem{})

	// MessageListener feeds the global player log (driven by message.PostMessage,
	// a separate global system), so it's registered once for the process. The
	// simulation listeners (kills, structures, research) moved to each level's
	// own event bus — see registerLevelListeners and
	// docs/developer/background_simulation.md.
	sharedListenersOnce.Do(func() {
		event.GetQueuedInstance().RegisterListener(&listeners.MessageListener{}, message.MessageEventType)
	})

	s.registerListeners()

	// Sit just left of the minimap's "Follow" button, top-aligned with it.
	s.dashboardBtn = minui.NewButton("dashboard_btn", "Dashboard")
	followBtnX := (config.Global().ScreenWidth - smallMapSize - smallMapMargin) - followBtnW - 2
	s.dashboardBtn.SetPosition(followBtnX-dashboardBtnW-6, smallMapMargin)
	s.dashboardBtn.SetSize(dashboardBtnW, dashboardBtnH)
	s.dashboardBtn.OnClick = func() { s.openDashboard() }

	return s, nil
}

func newMainStateFromLevel(level *world.Level, cfg SettlementConfig) (*MainState, error) {
	s, err := newMainStateBase(cfg)
	if err != nil {
		return nil, err
	}
	s.level = level
	s.registerLevelListeners()
	s.guiManager = gui.NewGUIManager()
	s.mapModal = newMapModal(level)
	s.refreshResourceScanner()
	s.cheatModal = newCheatModal(s)
	s.smallMap = newSmallMapWidget(level, s.mapModal.mm)
	s.smallMap.OnClick = func() { s.mapModal.Open(s.camera.Z, s.camera.X, s.camera.Y) }
	s.smallMap.OnFollowClick = func() { s.toggleFollowMode() }
	s.mapModal.OnTileDoubleClick = func(x, y, z int) {
		s.camera.CenterOn(x, y, z)
	}
	s.guiManager.SetDetailsTopOffset(smallMapSize + smallMapMargin*2)
	s.gm = &GameMaster{}
	s.gm.Init(level)

	s.camera.Z = level.SurfaceZ
	x, y := s.gm.GetFreeSpaceAtZ(s.camera.Z)
	if x != -1 {
		s.camera.CenterOn(x, y, s.camera.Z)
	}
	if cfg.Name != "" {
		if ms, ok := settlement.Settlements[cfg.Name]; ok {
			s.MainSettlement = ms
		}
	}
	if len(scenario.AllEnabled()) > 0 {
		if cfg.ScenarioID != "" {
			_ = scenario.SelectByID(cfg.ScenarioID)
		}
		s.applyScenarioLighting(level)
	}
	return s, nil
}

func NewMainState(cfg SettlementConfig) (*MainState, error) {
	s, err := newMainStateBase(cfg)
	if err != nil {
		return nil, err
	}
	s.newGame()
	return s, nil
}

// knownTechs returns the campaign's tech list if in campaign mode, else the settlement's.
func (s *MainState) knownTechs() []string {
	if s.campaign != nil {
		return s.campaign.KnownTechs
	}
	if s.MainSettlement != nil {
		return s.MainSettlement.KnownTechs
	}
	return nil
}

// hasTech reports whether the given tech key has been researched.
func (s *MainState) hasTech(key string) bool {
	if s.campaign != nil {
		return s.campaign.HasTech(key)
	}
	if s.MainSettlement != nil {
		return s.MainSettlement.HasTech(key)
	}
	return false
}

// refreshResourceScanner syncs the minimap's resource reveal to the current
// research state. Safe to call at any time (no-op if the map isn't built yet);
// the campaign field is assigned after construction, so this must run again once
// the campaign is wired and whenever research completes.
func (s *MainState) refreshResourceScanner() {
	if s.mapModal != nil {
		s.mapModal.mm.SetRevealResources(s.hasTech(techResourceScanner))
	}
}

// unlockTech records a tech as researched in the appropriate store.
func (s *MainState) unlockTech(key string) {
	if s.campaign != nil {
		s.campaign.UnlockTech(key)
		return
	}
	if s.MainSettlement != nil {
		s.MainSettlement.UnlockTech(key)
	}
}

// findAdjacentWorker returns the first Worker entity within one tile of (x,y,z),
// presumed to be the killer of a freshly-dead hostile. Returns nil if no
// friendly worker is adjacent (drops will fall to the ground in that case).
func findAdjacentWorker(level *world.Level, x, y, z int) *ecs.Entity {
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for _, e := range level.Entities {
				if !e.HasComponent(components.Worker) || e.HasComponent(rlcomponents.Dead) {
					continue
				}
				pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				if pc.GetX() == x+dx && pc.GetY() == y+dy && pc.GetZ() == z {
					return e
				}
			}
		}
	}
	return nil
}

func (s *MainState) newGame() {
	// Resolve the seed; persist it so saves/regeneration reproduce the map.
	seed := s.settlementCfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
		s.settlementCfg.Seed = seed
	}

	// Resolve the map definition (the terrain recipe). The map — not the
	// scenario — drives terrain generation.
	var md *mapdef.MapDef
	if s.settlementCfg.MapID != "" {
		md = mapdef.ByID(s.settlementCfg.MapID)
	}
	if md == nil {
		md = mapdef.Random()
	}
	mapID := ""
	if md != nil {
		mapID = md.ID
		s.settlementCfg.MapID = mapID
	}

	// Dimensions come from the map definition's Size block, rolled with the
	// location's seed so re-generation reproduces the same size. The loader
	// rejects any map JSON missing a valid size, so md is always non-nil here
	// with a positive range.
	mapW, mapH, mapZ := md.Size.Roll(seed)

	// Select a scenario compatible with the chosen map.
	compatible := scenario.ForMap(mapID)
	selected := false
	if s.settlementCfg.ScenarioID != "" {
		for i := range compatible {
			if compatible[i].ID == s.settlementCfg.ScenarioID {
				if err := scenario.SelectByID(s.settlementCfg.ScenarioID); err == nil {
					selected = true
				}
				break
			}
		}
		if !selected {
			log.Printf("scenario %q not compatible with map %q, picking a compatible one",
				s.settlementCfg.ScenarioID, mapID)
		}
	}
	if !selected && len(compatible) > 0 {
		_ = scenario.SelectByID(compatible[rand.Intn(len(compatible))].ID)
		selected = true
	}

	var activeScenario *scenario.Scenario
	if selected {
		activeScenario = scenario.Active()
		s.settlementCfg.ScenarioID = activeScenario.ID
	}

	// md is always non-nil here (mapdef.Random() guarantees it above, and
	// md.Size.Roll already dereferenced it). BuildWorld is the single terrain
	// path; it returns a usable (if degraded) level even on partial failure.
	opts := generation.BuildWorldOptions{
		Width:               mapW,
		Height:              mapH,
		Depth:               mapZ,
		Seed:                seed,
		Terrain:             md.Terrain,
		TerrainParams:       md.TerrainParams,
		BiomeMapType:        md.BiomeMap.Type,
		BiomeMapScale:       md.BiomeMap.Scale,
		BiomeLatitudeWeight: md.BiomeMap.LatitudeWeight,
		BiomeIDs:            md.BiomeMap.Biomes,
		BiomeSingle:         md.BiomeMap.Single,
		ReferenceArea:       md.Size.MidArea(),
	}
	for _, fb := range md.Features {
		opts.Features = append(opts.Features, generation.FeatureSpec{
			Kind: fb.Kind, Count: fb.Count, CountMax: fb.CountMax, Biome: fb.Biome,
			InRegion: fb.InRegion, Jitter: fb.Jitter,
			MinZ: fb.MinZ, MaxZ: fb.MaxZ, Params: fb.Params,
		})
	}
	if activeScenario != nil {
		for _, fb := range activeScenario.Features {
			opts.Features = append(opts.Features, generation.FeatureSpec{
				Kind: fb.Kind, Count: fb.Count, CountMax: fb.CountMax, Biome: fb.Biome,
				InRegion: fb.InRegion, Jitter: fb.Jitter,
				MinZ: fb.MinZ, MaxZ: fb.MaxZ, Params: fb.Params,
			})
		}
	}
	level, err := generation.BuildWorld(opts)
	if err != nil {
		log.Printf("BuildWorld %s: %v (using degraded level)", mapID, err)
	}
	if level == nil {
		// Only reachable if the rolled dimensions were invalid, which the map
		// loader's SizeBlock validation prevents. Guard anyway so a bad map
		// degrades to an empty world instead of nil-panicking below.
		level = world.NewLevel(mapW, mapH, mapZ)
	}
	s.level = level
	s.registerLevelListeners()
	s.guiManager = gui.NewGUIManager()
	s.mapModal = newMapModal(s.level)
	s.refreshResourceScanner()
	s.cheatModal = newCheatModal(s)
	s.smallMap = newSmallMapWidget(s.level, s.mapModal.mm)
	s.smallMap.OnClick = func() { s.mapModal.Open(s.camera.Z, s.camera.X, s.camera.Y) }
	s.smallMap.OnFollowClick = func() { s.toggleFollowMode() }
	s.mapModal.OnTileDoubleClick = func(x, y, z int) {
		s.camera.CenterOn(x, y, z)
	}
	s.guiManager.SetDetailsTopOffset(smallMapSize + smallMapMargin*2)
	s.gm = &GameMaster{}
	s.gm.Init(s.level)

	// Use the level's own surface Z (set by the primer — station maps put 0
	// there for the top floor, planet maps put it at the regolith surface).
	// Initial plaza search — used for camera positioning and as the script
	// anchor. On asteroid maps the tiles are solid rock here; the setup script
	// will carve the room, so we re-search after the script for colonist spawn.
	x, y, startingZ := findStartingPlaza(s.level, s.level.SurfaceZ, 5)

	name := s.settlementCfg.ColonyName
	if name == "" {
		name = s.settlementCfg.Name
	}
	if name == "" {
		name = "Colony Alpha"
	}

	if x != -1 {
		// Position camera centred on the starting area.
		s.camera.CenterOn(x, y, startingZ)
		s.MainSettlement = settlement.NewSettlement(x, y, startingZ)
		oldName := s.MainSettlement.Name
		s.MainSettlement.Name = name
		delete(settlement.Settlements, oldName)
		settlement.Settlements[name] = s.MainSettlement
	}

	if activeScenario != nil {
		s.applyScenarioLighting(s.level)
		if sc := activeScenario; len(sc.SetupScripts) > 0 {
			s.level.Flags["start_x"] = float64(x)
			s.level.Flags["start_y"] = float64(y)
			s.level.Flags["start_z"] = float64(startingZ)
			if s.MainSettlement != nil {
				s.level.Flags["settlement_name"] = name
			}
			s.level.Flags["colonist_faction"] = "colony"
			RunSetupScripts(sc.SetupScripts, s.level, seed)
		}
	}

	// Spawn colonists after the setup script so any carved rooms are in place.
	// Prefer the original plaza (where the camera and buildings are). Only
	// re-search if the original location is still not standable — this handles
	// asteroid scenarios where the script carves the room after the first search.
	spawnX, spawnY, spawnZ := x, y, startingZ
	if spawnX == -1 || !isStandable(s.level, spawnX, spawnY, spawnZ) {
		spawnX, spawnY, spawnZ = findStartingPlaza(s.level, startingZ, 5)
	}
	if spawnX != -1 && s.MainSettlement != nil {
		s.camera.CenterOn(spawnX, spawnY, spawnZ)
		if len(s.settlementCfg.PartyEntities) > 0 {
			beamPartyOntoLevel(s.level, s.settlementCfg.PartyEntities, name, spawnX, spawnY, spawnZ)
		} else if s.settlementCfg.CampaignMode {
			// Campaign: no auto-spawn — colonists beam down from the ship.
		} else {
			for i := 0; i < 5; i++ {
				colonist, err := factory.Create("colonist", spawnX+i-2, spawnY, spawnZ)
				if err == nil {
					colonist.AddComponent(&components.SettlementComponent{Name: name})
					colonist.AddComponent(&components.WorkerComponent{SelfDefend: true})
					s.level.AddEntity(colonist)
				}
			}
		}
	}
	s.day = 0
	s.lastDay = 0
}

func (s *MainState) applyScenarioLighting(level *world.Level) {
	// Player/config override wins. Otherwise fall back to the active scenario's
	// lighting — but only if one is selected. The Ship is built at campaign
	// start, before any location is entered, and supplies its own LightingMode.
	if s.settlementCfg.LightingMode != "" {
		level.LightMode = s.settlementCfg.LightingMode
		level.FixedAmbient = s.settlementCfg.LightingAmbient
	} else if scenario.HasActive() {
		sc := scenario.Active()
		level.LightMode = sc.Lighting.Mode
		level.FixedAmbient = sc.Lighting.AmbientLevel
	}
	if level.LightMode == "" {
		level.LightMode = world.LightModedayNight
	}
	if level.LightMode == world.LightModedayNight {
		level.Hour = 8 // start at morning
	}
}

func (s *MainState) Update() state.StateInterface {
	fspath.ResetFrameCounter()
	audio.SetMusicState("field") // base state in-game; combat overrides it
	s.handleInput()
	s.guiManager.SetKnownTechs(s.knownTechs())
	s.guiManager.SetInputBlocked(s.mapModal.Visible || s.cheatModal.Visible ||
		(s.storageInspector != nil && s.storageInspector.Visible) ||
		s.guiManager.ModalOpen("colonistModal"))
	s.guiManager.Update()
	// In Rogue mode the controlled colonist died or was lost — exit cleanly.
	if s.rogueEntity != nil &&
		(!s.rogueEntity.HasComponent(rlcomponents.Position) || s.rogueEntity.HasComponent(rlcomponents.Dead)) {
		s.exitRogueMode()
	}
	// If following an entity (manual follow or Rogue mode), center camera on it.
	if s.followEntity != nil && (s.CursorMode == gui.CursorModeFollow || s.rogueEntity != nil) {
		if s.followEntity.HasComponent(rlcomponents.Position) && !s.followEntity.HasComponent(rlcomponents.Dead) {
			pc := s.followEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			s.camera.CenterOn(pc.GetX(), pc.GetY(), pc.GetZ())
		} else if s.rogueEntity == nil {
			s.cancelFollowMode()
		}
	}
	s.mapModal.SetCamera(s.camera)
	s.smallMap.SetCamera(s.camera)
	if !s.mapModal.Visible {
		s.smallMap.Update()
	}
	if s.wm != nil && !s.mapModal.Visible && !s.guiManager.GetInputFocused() {
		s.dashboardBtn.Update()
	}
	s.mapModal.Update()
	s.cheatModal.Update()
	if s.storageInspector != nil {
		s.storageInspector.Update()
	}
	s.updateHovered()

	fps := ebiten.ActualFPS()
	tps := ebiten.ActualTPS()
	day := s.day
	if s.level != nil {
		day = s.level.Day
	}
	ebiten.SetWindowTitle(fmt.Sprintf("%s — Day:%d Hour:%d Z:%d FPS:%.0f TPS:%.0f", config.Global().Title, day, s.level.Hour, s.camera.Z, fps, tps))

	// Hand the last-drawn viewport to FOV so it bounds this tick's Visible clear
	// (it no longer reads a copy off Level).
	s.fovSystem.Viewport = s.viewport

	// In Rogue mode the world is turn-based: it only advances when the player
	// commits an action (see advancePlayerTurn). Otherwise it runs in real time.
	if !s.Paused && s.rogueEntity == nil {
		s.stepWorld()
		// Advance The Ship (and other background levels) alongside the live one.
		if s.wm != nil {
			s.wm.TickBackground(s)
		}
	}

	// Play positional audio for any in-world sounds the live level just emitted
	// (gated to the camera view). Reads only s.level, so background planets stay
	// silent.
	audio.PlayWorldSounds(s.level, s.viewport)

	if s.tick%30 == 0 {
		s.purgeCompletedTasks()
		s.refreshHUD()
		if !s.Paused {
			s.evaluateQuests() // periodic catch-all; target kills resolve in stepWorld
		}
	}

	if !s.Paused && s.tick%750 == 0 && s.tick > 0 {
		if s.level != nil {
			if s.level.LightMode == world.LightModedayNight {
				s.level.NextHour()
			} else {
				// Still advance the day counter even without light changes
				s.level.Day++
			}
			if s.level.Day != s.lastDay {
				s.lastDay = s.level.Day
				s.day = s.level.Day
			}
		}
	}

	return s.next
}

func (s *MainState) Draw(screen *ebiten.Image) {
	if s.worldImage == nil {
		s.worldImage = ebiten.NewImage(config.Global().WorldWidth, config.Global().WorldHeight)
	}
	// Clear to black once per frame so never-seen tiles need no per-tile fill.
	s.worldImage.Fill(color.Black)

	// Snapshot the on-screen rect; Update (next frame) hands it to FOV clearing
	// and positional audio. Kept out of Level so the world model carries no view.
	s.viewport = s.camera.Viewport()
	world.DrawLevel(s.level, s.worldImage, s.camera)
	world.DrawRadiationOverlay(s.level, s.worldImage, s.camera)

	s.drawTasks(s.worldImage)
	effect.GetEffectManager().Draw(s.worldImage, s.camera)
	screen.DrawImage(s.worldImage, nil)
	s.guiManager.Draw(screen)
	if !s.mapModal.Visible {
		s.smallMap.Draw(screen)
	}
	if s.wm != nil && !s.mapModal.Visible {
		s.dashboardBtn.Draw(screen)
	}
	s.mapModal.Draw(screen)
	s.cheatModal.Draw(screen)
	if s.storageInspector != nil {
		s.storageInspector.Draw(screen)
	}
}

func (s *MainState) Done() bool { return s.done }

func (s *MainState) HandleEvent(e event.EventData) error {
	switch ev := e.(type) {
	case input.MouseClickEvent:
		s.handleMouseClick(ev)
	case input.MouseReleasedEvent:
		if ev.Button == ebiten.MouseButtonLeft {
			s.mouseDragging = false
		}
	case input.KeyPressEvent:
		s.handleKeyPress(ev)
	case input.MouseWheelEvent:
		s.handleMouseWheel(ev)
	case gui.BuildOptionChangedEvent:
		s.buildMode = ev.Option
		if s.buildMode == "" {
			s.buildMode = "hull_wall"
		}
	case gui.CursorModeChangedEvent:
		if s.CursorMode == gui.CursorModeFollow {
			s.cancelFollowMode()
		}
		// Leave the old modal tool (unless re-entering the same mode) and enter
		// the new one. Each tool clears its own pending state + context.
		if old, ok := s.cursorTools[s.CursorMode]; ok && ev.Mode != s.CursorMode {
			old.Exit()
		}
		s.CursorMode = ev.Mode
		if nt, ok := s.cursorTools[ev.Mode]; ok {
			nt.Enter()
		}
	case gui.MainMenuEvent:
		switch ev.Action {
		case "quit":
			os.Exit(0)
		case "newgame":
			s.teardown()
			s.next = NewTitleState()
			s.done = true
		}
	case gui.SaveGameEvent:
		if s.wm != nil {
			if err := s.wm.SaveCampaign(s); err != nil {
				log.Printf("campaign save failed: %v", err)
			} else {
				message.PostMessage("Mission", "Expedition saved.")
				s.saveNamesDirty = true
			}
			return nil
		}
		name := s.settlementCfg.Name
		if name == "" {
			name = "colony"
		}
		if err := SaveSettlement(s.level, SaveMeta{
			Name:       name,
			ScenarioID: s.settlementCfg.ScenarioID,
			MapID:      s.settlementCfg.MapID,
			Seed:       s.settlementCfg.Seed,
			MapSizeW:   s.level.GetWidth(),
			MapSizeH:   s.level.GetHeight(),
			MapSizeZ:   s.level.GetDepth(),
		}); err != nil {
			log.Printf("save failed: %v", err)
		} else {
			message.AddMessage("Game saved.")
		}
	case gui.LoadGameEvent:
		loaded, err := LoadSave(ev.Name)
		if err != nil {
			log.Printf("load failed: %v", err)
		} else {
			s.teardown()
			s.next = loaded
			s.done = true
		}
	case gui.CraftRequestedEvent:
		s.addCraftTask(ev.RecipeID, ev.Station)
	case gui.StationClickedEvent:
		if s.MainSettlement != nil {
			cs := ev.Station.GetComponent(components.CraftingStation).(*components.CraftingStationComponent)
			knownTechs := map[string]bool{}
			for _, k := range s.knownTechs() {
				knownTechs[k] = true
			}
			allRecipes := crafting.RecipesByStation(cs.StationID)
			var recipes, lockedRecipes []crafting.Recipe
			for _, r := range allRecipes {
				if r.RequiresTech == "" || knownTechs[r.RequiresTech] {
					recipes = append(recipes, r)
				} else {
					lockedRecipes = append(lockedRecipes, r)
				}
			}
			title := cs.StationID
			if ev.Station.HasComponent(rlcomponents.Description) {
				title = ev.Station.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.OpenCraftingModal(title, recipes, lockedRecipes, ev.Station)
		}
	case gui.ResearchStationClickedEvent:
		if s.MainSettlement != nil {
			available, locked, completed, inProgress, queue := s.researchSnapshot()
			s.guiManager.OpenResearchModal(ev.Station, available, locked, completed, inProgress, queue)
		}
	case gui.ResearchRequestedEvent:
		s.addResearchTask(ev.TechKey, ev.Station)
	case eventsystem.ResearchDoneEvent:
		s.unlockTech(ev.TechKey)
		s.guiManager.SetKnownTechs(s.knownTechs())
		s.refreshResourceScanner()
	case gui.ColonistSelectedEvent:
		if ev.Entity.HasComponent(rlcomponents.Position) {
			pc := ev.Entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			s.camera.CenterOn(pc.GetX(), pc.GetY(), pc.GetZ())
		}
		s.openColonistModal(ev.Entity)
	case gui.EquipItemRequestedEvent:
		s.equipItem(ev.ColonistEntity, ev.ItemBlueprint)
	case gui.UnequipItemRequestedEvent:
		s.addUnequipTask(ev.ColonistEntity, ev.Slot)
	case gui.SetTaskFilterEvent:
		s.applyTaskFilter(ev.Colonist, ev.Action, ev.Enabled)
	case gui.DropOffRequestedEvent:
		s.requestDropOff(ev.Colonist, ev.Item)
	case gui.PickupRequestedEvent:
		s.requestPickup(ev.Colonist)
	case gui.SetSelfDefendEvent:
		s.applySelfDefend(ev.Colonist, ev.Enabled)
	case gui.EnterRogueModeEvent:
		s.enterRogueMode(ev.Entity)
	case gui.ExitRogueModeEvent:
		s.exitRogueMode()
	}
	return nil
}

func (s *MainState) purgeCompletedTasks() {
	if s.MainSettlement == nil {
		return
	}
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.Completed {
			s.MainSettlement.Tasks.RemoveTask(t)
		}
	}
}

// researchSnapshot collects the lists the research modal needs: techs that
// can be started, those already known, those currently being researched, and
// the live progress queue.
func (s *MainState) researchSnapshot() (available, locked, completed, inProgress []research.Tech, queue []gui.ResearchQueueEntry) {
	if s.MainSettlement == nil {
		return
	}

	for _, k := range s.knownTechs() {
		if t, ok := research.GetTech(k); ok {
			completed = append(completed, t)
		}
	}

	inProgressSet := map[string]bool{}
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.Completed || t.Action != task_requests.ResearchAction {
			continue
		}
		rr, ok := t.Data.(*task_requests.ResearchRequest)
		if !ok {
			continue
		}
		inProgressSet[rr.TechKey] = true
		techDef, found := research.GetTech(rr.TechKey)
		if found {
			inProgress = append(inProgress, techDef)
		}
		name := rr.TechKey
		if found {
			name = techDef.Name
		}
		progress := 0.0
		if rr.Required > 0 {
			progress = float64(rr.Progress) / float64(rr.Required)
		}
		queue = append(queue, gui.ResearchQueueEntry{Name: name, Progress: progress})
	}

	for _, t := range research.AvailableTechs(s.knownTechs()) {
		if inProgressSet[t.Key] {
			continue
		}
		available = append(available, t)
	}
	locked = research.LockedTechs(s.knownTechs())
	return
}

func (s *MainState) addResearchTask(techKey string, station *ecs.Entity) {
	if s.MainSettlement == nil {
		return
	}
	tech, found := research.GetTech(techKey)
	if !found {
		log.Printf("[RESEARCH] tech not found: %s", techKey)
		return
	}
	if s.hasTech(techKey) {
		message.AddMessage(tech.Name + " already researched.")
		return
	}
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.Completed || t.Action != task_requests.ResearchAction {
			continue
		}
		if rr, ok := t.Data.(*task_requests.ResearchRequest); ok && rr.TechKey == techKey {
			message.AddMessage(tech.Name + " is already being researched.")
			return
		}
	}
	pc := station.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.ResearchAction,
		Data: &task_requests.ResearchRequest{
			TechKey:  techKey,
			Building: tech.RequiredBuilding,
			Required: tech.Duration,
		},
		X: pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(),
	})
	message.AddMessage("Queued research: " + tech.Name)
}

func (s *MainState) addCraftTask(recipeID string, station *ecs.Entity) {
	if s.MainSettlement == nil {
		return
	}
	recipe, found := crafting.GetRecipe(recipeID)
	if !found {
		log.Printf("[CRAFT] recipe not found: %s", recipeID)
		return
	}
	wbPC := station.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.CraftAction,
		Data: task_requests.CraftRequest{
			RecipeID:   recipeID,
			WorkbenchX: wbPC.GetX(),
			WorkbenchY: wbPC.GetY(),
			WorkbenchZ: wbPC.GetZ(),
			Required:   recipe.BuildTime,
		},
		X: wbPC.GetX(), Y: wbPC.GetY(), Z: wbPC.GetZ(),
	})
	message.AddMessage("Queued: craft " + recipe.Name)
}

// BeginRelocate is called from the storage inspector; it delegates to the
// Relocate cursor tool (see relocateTool.Begin).
func (s *MainState) BeginRelocate(source *ecs.Entity, blueprint string, qty int) {
	s.relocateTool.Begin(source, blueprint, qty)
}

// tileWalkable reports whether (x,y,z) is a non-solid tile suitable for a
// ground-drop destination — same predicate the move-task uses.
func (s *MainState) tileWalkable(x, y, z int) bool {
	tile := s.level.GetTileAt(x, y, z)
	if tile == nil {
		return false
	}
	t := tile.(*world.Tile)
	if t.Floor.IsEmpty() {
		return false
	}
	if !t.Middle.IsEmpty() && world.TileDefinitions[t.Middle.Type].Solid {
		return false
	}
	if s.level.GetSolidEntityAt(x, y, z) != nil {
		return false
	}
	return true
}

func (s *MainState) findCraftingStation(stationID string) *ecs.Entity {
	for _, e := range append(s.level.Entities, s.level.StaticEntities...) {
		if !e.HasComponent(components.CraftingStation) {
			continue
		}
		cs := e.GetComponent(components.CraftingStation).(*components.CraftingStationComponent)
		if cs.StationID == stationID {
			return e
		}
	}
	return nil
}

func (s *MainState) openColonistModal(colonist *ecs.Entity) {
	if colonist == nil || s.MainSettlement == nil {
		return
	}
	sc := s.MainSettlement
	var storageItems []gui.StorageItemEntry
	seen := map[string]bool{}
	for _, e := range s.level.Entities {
		if !e.HasComponent(components.Storage) {
			continue
		}
		storage := e.GetComponent(components.Storage).(*components.StorageComponent)
		if storage.OwnedBy != sc.Name {
			continue
		}
		for _, item := range storage.Items {
			if !item.HasComponent(rlcomponents.Item) {
				continue
			}
			// Skip bulk resources (metal_ore, biomass, fuel …) — those are for
			// crafting/building, not carrying. Everything else is offerable:
			// gear to equip, consumables/sleeping bags/medkits to carry.
			if item.HasComponent(components.Material) {
				continue
			}
			ic := item.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent)
			if seen[item.Blueprint] {
				continue
			}
			seen[item.Blueprint] = true
			name := item.Blueprint
			if item.HasComponent(rlcomponents.Description) {
				name = item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			storageItems = append(storageItems, gui.StorageItemEntry{
				Blueprint: item.Blueprint,
				Name:      name,
				Slot:      string(ic.Slot),
			})
		}
	}
	s.guiManager.ShowColonistModal(colonist, storageItems)
}

// equipItem equips an item onto the colonist. When the item is already carried
// in the colonist's bag it is equipped immediately — no walk to storage is
// needed — and the open colonist modal is refreshed in place so the change
// shows without closing it. Otherwise it falls back to queueing an equip task
// that fetches the item from settlement storage.
func (s *MainState) equipItem(colonist *ecs.Entity, blueprint string) {
	if colonist == nil {
		return
	}
	// Only short-circuit for gear the colonist already carries — equip it in
	// place. Consumables (rations, sleeping bags…) should always fetch another
	// from storage even if one is already in the bag, so the player can stock up.
	if colonist.HasComponent(rlcomponents.Inventory) {
		inv := colonist.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range inv.Bag {
			if item.Blueprint == blueprint && components.ItemIsGear(item) {
				inv.Equip(item)
				s.openColonistModal(colonist) // rebuild the modal with the updated bag/slots
				return
			}
		}
	}
	s.addEquipTask(colonist, blueprint)
}

func (s *MainState) addEquipTask(colonist *ecs.Entity, blueprint string) {
	if colonist == nil || !colonist.HasComponent(components.Worker) {
		return
	}
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		return
	}
	t := &task.Task{
		Action:    task_requests.EquipAction,
		Escalated: true,
		Data:      task_requests.EquipRequest{ItemBlueprint: blueprint},
	}
	wc.CurrentTask = t
}

func (s *MainState) addUnequipTask(colonist *ecs.Entity, slot string) {
	if colonist == nil || !colonist.HasComponent(components.Worker) {
		return
	}
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		return
	}
	t := &task.Task{
		Action:    task_requests.UnequipAction,
		Escalated: true,
		Data:      task_requests.UnequipRequest{Slot: slot},
	}
	wc.CurrentTask = t
}

func (s *MainState) applySelfDefend(colonist *ecs.Entity, enabled bool) {
	if colonist == nil || !colonist.HasComponent(components.Worker) {
		return
	}
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	wc.SelfDefend = enabled
}

func (s *MainState) applyTaskFilter(colonist *ecs.Entity, action string, enabled bool) {
	if colonist == nil || !colonist.HasComponent(components.Worker) {
		return
	}
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	a := task.TaskAction(action)
	// Reject toggles for actions outside the chassis cap — these should
	// never reach us since the modal hides them, but defensive against any
	// stale event or scripted call.
	if !wc.IsTaskAvailable(a) {
		return
	}
	// universe is the set of actions the player can toggle: AvailableTasks
	// when set, otherwise every FilterableAction.
	universe := wc.AvailableTasks
	if len(universe) == 0 {
		universe = make([]task.TaskAction, 0, len(task_requests.FilterableActions))
		for _, fa := range task_requests.FilterableActions {
			universe = append(universe, fa.Action)
		}
	}
	if enabled {
		for _, existing := range wc.AllowedTasks {
			if existing == a {
				return
			}
		}
		wc.AllowedTasks = append(wc.AllowedTasks, a)
		// Only collapse to nil ("no filter") for uncapped workers; for
		// capped workers AllowedTasks stays explicit so the cap is
		// preserved if AvailableTasks is ever later cleared.
		if len(wc.AvailableTasks) == 0 && len(wc.AllowedTasks) >= len(universe) {
			wc.AllowedTasks = nil
		}
	} else {
		if wc.AllowedTasks == nil {
			wc.AllowedTasks = append(wc.AllowedTasks, universe...)
		}
		for i, existing := range wc.AllowedTasks {
			if existing == a {
				wc.AllowedTasks = append(wc.AllowedTasks[:i], wc.AllowedTasks[i+1:]...)
				return
			}
		}
	}
}

// requestDropOff is fired from the colonist inventory modal's per-item Drop
// button; it delegates to the Drop cursor tool (see dropTool.Request).
func (s *MainState) requestDropOff(colonist *ecs.Entity, item *ecs.Entity) {
	s.dropTool.Request(colonist, item)
}

// requestPickup is fired from the colonist modal's Pickup button; it delegates
// to the Pickup cursor tool (see pickupTool.Request).
func (s *MainState) requestPickup(colonist *ecs.Entity) {
	s.pickupTool.Request(colonist)
}

func (s *MainState) handleInput() {
	if s.mouseDragging {
		cX, cY := ebiten.CursorPosition()
		tX, tY := s.camera.ScreenToWorld(cX, cY)
		if tX != s.lastDragTileX || tY != s.lastDragTileY {
			switch s.CursorMode {
			case gui.CursorModeBuild:
				s.addBuildTask(tX, tY)
			case gui.CursorModeDig:
				s.addDigTask(tX, tY)
			}
			s.lastDragTileX = tX
			s.lastDragTileY = tY
		}
	}

	if s.rogueEntity != nil && s.rogueMoveActive {
		wasdHeld := ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyS) ||
			ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyD)
		if wasdHeld {
			s.rogueMoveActive = false
			s.rogueAutoMoveTick = 0
		} else {
			interval := config.Global().RogueAutoMoveInterval
			if interval <= 0 {
				interval = 8
			}
			s.rogueAutoMoveTick++
			if s.rogueAutoMoveTick >= interval {
				s.rogueAutoMoveTick = 0
				s.rogueStepToward(s.rogueMoveTargetX, s.rogueMoveTargetY, s.rogueMoveTargetZ)
			}
		}
	}
}

func (s *MainState) handleKeyPress(e input.KeyPressEvent) {
	if s.guiManager.GetInputFocused() {
		return
	}

	// Developer console: Shift+ESC opens it; while open it swallows all keys
	// except Esc (close) so typing doesn't drive the camera.
	if s.cheatModal.Visible {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			s.cheatModal.Visible = false
		}
		return
	}
	shift := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	if shift && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.cheatModal.Open()
		return
	}

	if s.rogueEntity != nil {
		s.handleRogueKeys()
		return
	}

	// Closing the map modal must be handled independently of e.JustPressed
	// (which only reflects the first held key, so it can mask Esc/M when a
	// camera key is also down) — otherwise the modal can't be dismissed.
	if s.mapModal.Visible {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyM) {
			s.mapModal.Visible = false
		}
		return
	}

	for _, k := range e.Keys {
		switch k.String() {
		case "W":
			s.camera.Y--
		case "S":
			s.camera.Y++
		case "A":
			s.camera.X--
		case "D":
			s.camera.X++
		}
	}

	if !e.JustPressed {
		return
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		s.Paused = !s.Paused
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) && s.camera.Z > 0 {
		s.camera.Z--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) && s.camera.Z < s.level.GetDepth()-1 {
		s.camera.Z++
	}
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		s.initiativeSystem.Speed = 1
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		s.initiativeSystem.Speed = 2
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		s.initiativeSystem.Speed = 4
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if s.mapModal.Visible {
			s.mapModal.Visible = false
		} else {
			s.guiManager.ToggleModal("mainMenu")
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		if s.mapModal.Visible {
			s.mapModal.Visible = false
		} else {
			s.mapModal.Open(s.camera.Z, s.camera.X, s.camera.Y)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyO) && s.wm != nil {
		s.openDashboard()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) && s.campaign != nil {
		s.beamUpSelected()
	}
}

// beamUpSelected sends the currently selected colonist up to the ship roster.
func (s *MainState) beamUpSelected() {
	e := s.selectedEntity
	if e == nil || !e.HasComponent(components.Worker) {
		message.PostMessage("Ship", "Select a colonist to beam up.")
		return
	}
	if e.HasComponent(components.Worker) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			wc.CurrentTask.Stop()
		}
		wc.CurrentTask = nil
	}
	if s.wm == nil {
		message.PostMessage("Ship", "No ship available.")
		return
	}
	if err := s.wm.BeamUp(e); err != nil {
		message.PostMessage("Ship", err.Error())
		return
	}
	if s.selectedEntity == e {
		s.selectedEntity = nil
	}
	message.PostMessage("Ship", "Colonist beamed up to the ship.")
}

// openDashboard parks the live location (keeps it in memory, detaches its event
// listeners) and switches to the Dashboard. The level is not serialized here —
// it stays loaded so colonists can be shuffled, and is only frozen to disk on
// Travel-away or Save.
func (s *MainState) openDashboard() {
	if s.wm == nil {
		return
	}
	s.teardown()
	// The ship parks back into its own slot; only a location updates `current`
	// (the orbited planet), so leaving the ship must not clobber it.
	if s != s.wm.shipLevel {
		s.wm.current = s
	}
	s.next = NewDashboardState(s.campaign, s.wm)
	s.done = true
}

func (s *MainState) handleMouseWheel(e input.MouseWheelEvent) {
	// Rest-detection runs before the modal/focus guards so the gesture latch
	// always rearms on wheel-rest — otherwise opening a modal mid-gesture swallows
	// the Y:0 reset and eats the first scroll after the modal closes.
	if e.Y > -0.1 && e.Y < 0.1 {
		s.wheelActive = false // wheel at rest — this gesture is over
		return
	}
	if s.mapModal.Visible || s.cheatModal.Visible {
		return
	}
	if s.guiManager.GetMouseFocused() {
		return
	}
	if s.wheelActive {
		return // momentum tail (or inertial rebound) of the current gesture — ignore
	}
	s.wheelActive = true

	// ZoomAt clamps the tile size to [view.MinTileSize, view.MaxTileSize], so a
	// scroll past a bound is a harmless no-op.
	mX, mY := ebiten.CursorPosition()
	s.camera.ZoomAt(mX, mY, e.Y > 0)
}

func (s *MainState) handleMouseClick(e input.MouseClickEvent) {
	if s.mapModal.Visible || s.cheatModal.Visible {
		return
	}
	if s.storageInspector != nil && s.storageInspector.Visible {
		return
	}
	if s.guiManager.GetMouseFocused() || s.guiManager.WithinModalBounds(ebiten.CursorPosition()) {
		return
	}
	cXg, cYg := ebiten.CursorPosition()
	if s.smallMap.WithinBounds(cXg, cYg) {
		return
	}
	if s.wm != nil {
		fx := (config.Global().ScreenWidth - smallMapSize - smallMapMargin) - followBtnW - 2
		bx := fx - dashboardBtnW - 6
		by := smallMapMargin
		if cXg >= bx && cXg <= bx+dashboardBtnW && cYg >= by && cYg <= by+dashboardBtnH {
			return // Dashboard button
		}
	}

	cX, cY := ebiten.CursorPosition()
	tX, tY := s.camera.ScreenToWorld(cX, cY)

	if s.rogueEntity != nil {
		if e.Button == ebiten.MouseButtonLeft {
			pc := s.rogueEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			s.rogueMoveTargetX = tX
			s.rogueMoveTargetY = tY
			s.rogueMoveTargetZ = pc.GetZ()
			s.rogueMoveActive = true
			s.rogueComputePath()
		} else if e.Button == ebiten.MouseButtonRight {
			s.rogueMoveActive = false
			s.rogueFire(tX, tY)
		}
		return
	}

	if e.Button == ebiten.MouseButtonLeft {
		if s.CursorMode == gui.CursorModeFollow {
			ent := s.level.GetEntityAt(tX, tY, s.camera.Z)
			if ent != nil && ent.HasComponent(rlcomponents.Position) {
				s.followEntity = ent
				name := "Entity"
				if ent.HasComponent(rlcomponents.Description) {
					name = ent.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
				}
				s.guiManager.SetSelectionLabel("Following: "+name, "Right-click to cancel follow.")
			} else {
				s.cancelFollowMode()
			}
			return
		}
		if s.CursorMode == gui.CursorModeDefault {
			ent := s.level.GetEntityAt(tX, tY, s.camera.Z)

			if ent != nil && components.IsAttackTarget(ent) && s.MainSettlement != nil {
				// Attack hostile entity (faction raiders, scripted mutants/zombies, etc.)
				s.MainSettlement.Tasks.AddTask(&task.Task{
					Action: task_requests.AttackAction, Data: ent,
					X: tX, Y: tY, Z: s.camera.Z, Escalated: true,
				})
			} else if ent != nil && ent.HasComponent(components.Choppable) && s.MainSettlement != nil {
				// Harvest choppable entity (flora, crystals, etc.)
				s.addMineTask(tX, tY)
			} else if ent != nil && ent.HasComponent(rlcomponents.Item) && s.MainSettlement != nil {
				// Retrieve dropped item
				s.MainSettlement.Tasks.AddTask(&task.Task{
					Action: task_requests.RetrieveAction,
					Data:   task_requests.RetrieveRequest{Item: ent},
					X:      tX, Y: tY, Z: s.camera.Z, Escalated: true,
				})
			} else if ent != nil && ent.HasComponent(components.Bed) && s.MainSettlement != nil {
				s.addSleepTask(tX, tY)
			} else {
				// Select entity (colonist, building, etc.)
				for _, e := range s.level.Entities {
					e.RemoveComponent(components.Selected)
				}
				for _, e := range s.level.StaticEntities {
					e.RemoveComponent(components.Selected)
				}
				s.selectedEntity = nil
				if ent == nil {
					for _, se := range s.level.StaticEntities {
						pc := se.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
						if pc.GetX() == tX && pc.GetY() == tY && pc.GetZ() == s.camera.Z {
							ent = se
							break
						}
					}
				}
				if ent != nil {
					ent.AddComponent(&components.SelectedComponent{})
					s.selectedEntity = ent
					event.GetQueuedInstance().SendEvent(gui.EntitySelectedEvent{Entity: ent})
					if ent.HasComponent(components.CraftingStation) {
						event.GetQueuedInstance().QueueEvent(gui.StationClickedEvent{Station: ent})
					} else if ent.HasComponent(components.ResearchBuilding) {
						event.GetQueuedInstance().QueueEvent(gui.ResearchStationClickedEvent{Station: ent})
					} else if ent.HasComponent(components.Worker) {
						event.GetQueuedInstance().QueueEvent(gui.ColonistSelectedEvent{Entity: ent})
					} else if ent.HasComponent(components.Storage) && s.storageInspector != nil {
						s.storageInspector.Open(ent)
					}
				} else if s.MainSettlement != nil {
					// Walkable empty tile — queue a move task
					tile := s.level.GetTileAt(tX, tY, s.camera.Z)
					if tile != nil {
						t := tile.(*world.Tile)
						walkable := t.Middle.IsEmpty() || !world.TileDefinitions[t.Middle.Type].Solid
						if walkable && !t.Floor.IsEmpty() {
							s.MainSettlement.Tasks.AddTask(&task.Task{
								X: tX, Y: tY, Z: s.camera.Z, Escalated: true,
							})
						}
					}
				}
				// Escalate any pending task at this tile
				if s.MainSettlement != nil {
					for _, t := range s.MainSettlement.Tasks.GetTasks() {
						if t.X == tX && t.Y == tY && t.Z == s.camera.Z {
							t.Escalated = true
						}
					}
				}
			}
		} else if s.MainSettlement != nil {
			// Perform the active order
			switch s.CursorMode {
			case gui.CursorModeBuild:
				s.mouseDragging = true
				s.lastDragTileX = -1
				s.lastDragTileY = -1
				s.addBuildTask(tX, tY)
			case gui.CursorModeDig:
				s.mouseDragging = true
				s.lastDragTileX = -1
				s.lastDragTileY = -1
				s.addDigTask(tX, tY)
			case gui.CursorModeMine:
				s.addMineTask(tX, tY)
			case gui.CursorModeCancel:
				for _, t := range s.MainSettlement.Tasks.GetTasks() {
					if t.X == tX && t.Y == tY && t.Z == s.camera.Z {
						t.Complete()
						s.MainSettlement.Tasks.RemoveTask(t)
					}
				}
			case gui.CursorModeSleep:
				s.addSleepTask(tX, tY)
			case gui.CursorModeAttack:
				target := s.level.GetEntityAt(tX, tY, s.camera.Z)
				if components.IsAttackTarget(target) {
					for _, colonist := range s.level.Entities {
						if colonist.HasComponent(components.Worker) && !colonist.HasComponent(rlcomponents.Dead) {
							wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
							if wc.CurrentTask == nil || wc.CurrentTask.Completed {
								s.MainSettlement.Tasks.AddTask(&task.Task{
									Action: task_requests.AttackAction,
									Data:   target,
									X:      tX, Y: tY, Z: s.camera.Z,
								})
								break
							}
						}
					}
				}
			case gui.CursorModeRelocate, gui.CursorModeStore, gui.CursorModeDrop, gui.CursorModePickup:
				s.cursorTools[s.CursorMode].Click(tX, tY, s.camera.Z)
			}
		}
	}

	if e.Button == ebiten.MouseButtonRight {
		if s.CursorMode == gui.CursorModeFollow {
			s.cancelFollowMode()
			return
		}
		if s.CursorMode != gui.CursorModeDefault {
			// Cancel active order, return to default
			event.GetQueuedInstance().QueueEvent(gui.CursorModeChangedEvent{Mode: gui.CursorModeDefault})
		} else if s.MainSettlement != nil {
			// Cancel any pending task at this tile
			for _, t := range s.MainSettlement.Tasks.GetTasks() {
				if t.X == tX && t.Y == tY && t.Z == s.camera.Z {
					t.Complete()
					s.MainSettlement.Tasks.RemoveTask(t)
				}
			}
		}
	}
}

func (s *MainState) toggleFollowMode() {
	if s.CursorMode == gui.CursorModeFollow {
		s.cancelFollowMode()
		return
	}
	s.followEntity = nil
	s.CursorMode = gui.CursorModeFollow
	s.smallMap.FollowActive = true
	s.guiManager.SetSelectionLabel("Follow Mode", "Click an entity to follow it.  Right-click to cancel.")
}

func (s *MainState) cancelFollowMode() {
	s.followEntity = nil
	s.CursorMode = gui.CursorModeDefault
	s.smallMap.FollowActive = false
	s.guiManager.SetSelectionLabel("", "")
}

func (s *MainState) addBuildTask(x, y int) {
	if s.MainSettlement == nil {
		return
	}
	// For multi-tile entities, offset position so the click lands on the top-left
	// corner. entityFootprint centers on the stored position using startX = x - w/2,
	// startY = y - h/2, so adding w/2 and h/2 back makes the click tile the origin.
	buildable := construction.GetBuildable(s.buildMode)
	if buildable.IsEntity {
		if sc := factory.GetSize(s.buildMode); sc != nil {
			x += sc.Width / 2
			y += sc.Height / 2
		}
	}
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == s.camera.Z && t.Action == task_requests.BuildAction {
			return
		}
	}
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.BuildAction,
		Data:   task_requests.BuildRequest{X: x, Y: y, Z: s.camera.Z, Type: s.buildMode, Required: buildable.BuildTime},
		X:      x, Y: y, Z: s.camera.Z,
	})
	if !buildable.AllowMultiple {
		s.CursorMode = gui.CursorModeDefault
	}
}

func (s *MainState) addSleepTask(x, y int) {
	if s.MainSettlement == nil {
		return
	}
	bed := s.level.GetEntityAt(x, y, s.camera.Z)
	if bed == nil || !bed.HasComponent(components.Bed) {
		return
	}
	pc := bed.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	bx, by, bz := pc.GetX(), pc.GetY(), pc.GetZ()
	// Dedup: one sleep task per bed, whether queued or already assigned.
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.Action == task_requests.SleepAction && t.X == bx && t.Y == by && t.Z == bz {
			return
		}
	}
	for _, colonist := range s.level.Entities {
		if !colonist.HasComponent(components.Worker) {
			continue
		}
		wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask == nil || wc.CurrentTask.Completed {
			continue
		}
		if wc.CurrentTask.Action == task_requests.SleepAction &&
			wc.CurrentTask.X == bx && wc.CurrentTask.Y == by && wc.CurrentTask.Z == bz {
			return
		}
	}
	// Find the idle worker in this settlement with the lowest current health.
	var bestWorker *ecs.Entity
	var bestHealth int
	for _, colonist := range s.level.Entities {
		if !colonist.HasComponents(components.Worker, components.Settlement, rlcomponents.Health) {
			continue
		}
		if colonist.HasComponent(rlcomponents.Dead) {
			continue
		}
		csc := colonist.GetComponent(components.Settlement).(*components.SettlementComponent)
		if csc.Name != s.MainSettlement.Name {
			continue
		}
		wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			continue
		}
		hc := colonist.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		if bestWorker == nil || hc.Health < bestHealth {
			bestWorker = colonist
			bestHealth = hc.Health
		}
	}
	sleepTask := &task.Task{
		Action: task_requests.SleepAction,
		Data:   &task_requests.SleepRequest{X: bx, Y: by, Z: bz, Required: 10},
		X:      bx, Y: by, Z: bz,
	}
	if bestWorker != nil {
		sleepTask.Start()
		bestWorker.GetComponent(components.Worker).(*components.WorkerComponent).CurrentTask = sleepTask
		return
	}
	// No idle workers — fall back to the settlement queue so the next free worker picks it up.
	s.MainSettlement.Tasks.AddTask(sleepTask)
}

func (s *MainState) addDigTask(x, y int) {
	if s.MainSettlement == nil {
		return
	}
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == s.camera.Z && t.Action == task_requests.DigAction {
			return
		}
	}
	tile := s.level.GetTileAt(x, y, s.camera.Z)
	if tile == nil || !tile.IsSolid() {
		return
	}
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.DigAction,
		Data:   task_requests.DigRequest{X: x, Y: y, Z: s.camera.Z, Required: 5},
		X:      x, Y: y, Z: s.camera.Z,
	})
}

func (s *MainState) addMineTask(x, y int) {
	if s.MainSettlement == nil {
		return
	}
	// Prefer a Choppable entity at the tile (e.g. alien_crystal). Required is
	// derived from the entity's Choppable.Health so harder things take longer.
	var entBuf []*ecs.Entity
	s.level.GetEntitiesAt(x, y, s.camera.Z, &entBuf)
	for _, e := range entBuf {
		if !e.HasComponent(components.Choppable) {
			continue
		}
		ch := e.GetComponent(components.Choppable).(*components.ChoppableComponent)
		req := task_requests.MineRequest{X: x, Y: y, Z: s.camera.Z, Required: ch.Health * 10, Target: e}
		s.MainSettlement.Tasks.AddTask(&task.Task{
			Action: task_requests.MineAction,
			Data:   req,
			X:      x, Y: y, Z: s.camera.Z,
		})
		return
	}

	tile := s.level.GetTileAt(x, y, s.camera.Z)
	if tile == nil {
		return
	}
	t := tile.(*world.Tile)
	if t.Middle.IsEmpty() {
		return
	}
	tileName := world.TileDefinitions[t.Middle.Type].Name
	if !world.IsDepositTileName(tileName) {
		return
	}
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.MineAction,
		Data:   task_requests.MineRequest{X: x, Y: y, Z: s.camera.Z, Required: 50},
		X:      x, Y: y, Z: s.camera.Z,
	})
}

func (s *MainState) updateHovered() {
	cfg := config.Global()
	cX, cY := ebiten.CursorPosition()
	if cX < view.SidebarWidth || cX >= cfg.WorldWidth || cY < 0 || cY >= cfg.WorldHeight {
		s.guiManager.ClearHover()
		s.hoverActive = false
		return
	}
	tX, tY := s.camera.ScreenToWorld(cX, cY)
	s.hoverTileX = tX
	s.hoverTileY = tY
	s.hoverActive = true

	tile := s.level.GetTileAt(tX, tY, s.camera.Z)
	entity := s.level.GetEntityAt(tX, tY, s.camera.Z)

	if tile == nil && entity == nil {
		s.guiManager.ClearHover()
		if s.CursorMode == gui.CursorModeDefault {
			s.guiManager.SetDefaultContext("", "", "")
		} else if tool, ok := s.cursorTools[s.CursorMode]; ok {
			tool.Hover(tX, tY, s.camera.Z)
		}
		s.hoverActive = false
		return
	}

	if tile != nil {
		t := tile.(*world.Tile)
		// Middle slot wins for display (walls, ore, doors). Floor is the fallback
		// for cells where Middle is air/empty (caverns, open surface).
		var def world.TileDefinition
		if !t.Middle.IsEmpty() {
			def = world.TileDefinitions[t.Middle.Type]
		}
		floorName := ""
		if !t.Floor.IsEmpty() {
			floorName = world.TileDefinitions[t.Floor.Type].Name
		}
		var smells []gui.TileSmell
		for tag, tagMap := range s.level.SmellMap {
			if v := tagMap[world.PackCoord(tX, tY, s.camera.Z)]; v >= 0.1 {
				smells = append(smells, gui.TileSmell{Tag: string(tag), Strength: v})
			}
		}
		resourceAmt := 0
		if !t.Middle.IsEmpty() && world.IsDepositTileName(def.Name) {
			resourceAmt = s.level.ResourceAmountAt(tX, tY, s.camera.Z)
		}
		s.guiManager.SetHoveredTile(gui.HoveredTileInfo{
			Name:           def.Name,
			FloorName:      floorName,
			X:              tX,
			Y:              tY,
			Z:              s.camera.Z,
			LightLevel:     t.LightLevel,
			Radiation:      int(t.Radiation),
			ResourceAmount: resourceAmt,
			Solid:          def.Solid,
			Water:          def.Water,
			Air:            def.Air,
			Space:          def.Space,
			Smells:         smells,
		})
	}

	s.guiManager.SetHoveredEntity(entity)

	if entity != nil {
		// Register the entity in the Encyclopedia (no-op if already known or
		// no Description). AddKnownEntity returns true exactly once per
		// blueprint, so the message log gets one "Catalogued: …" line per
		// new discovery without any extra dedup state.
		if s.campaign != nil && entity.HasComponent(rlcomponents.Description) {
			if s.campaign.AddKnownEntity(entity.Blueprint) {
				desc := entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
				name := desc.DisplayName()
				if name == "" {
					name = entity.Blueprint
				}
				message.PostMessage("Encyclopedia", "Catalogued: "+name)
			}
		}
	}

	if s.CursorMode == gui.CursorModeDefault {
		s.updateDefaultContext(tX, tY)
	} else if tool, ok := s.cursorTools[s.CursorMode]; ok {
		tool.Hover(tX, tY, s.camera.Z)
	}
}

// updateDefaultContext detects what a Default-mode left-click would do at (tX,tY)
// and updates the HUD context hint accordingly.
func (s *MainState) updateDefaultContext(tX, tY int) {
	if entity := s.level.GetEntityAt(tX, tY, s.camera.Z); entity != nil {
		if components.IsAttackTarget(entity) {
			name := "Enemy"
			if entity.HasComponent(rlcomponents.Description) {
				name = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.SetDefaultContext("Attack: "+name, "Order colonists to attack this target.", entity.Blueprint)
			return
		}
		if entity.HasComponent(components.Choppable) {
			name := "Flora"
			if entity.HasComponent(rlcomponents.Description) {
				name = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.SetDefaultContext("Harvest: "+name, "Order colonists to harvest this.", entity.Blueprint)
			return
		}
		if entity.HasComponent(rlcomponents.Item) {
			name := "Item"
			if entity.HasComponent(rlcomponents.Description) {
				name = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.SetDefaultContext("Retrieve: "+name, "Order colonists to pick this up.", entity.Blueprint)
			return
		}
		if entity.HasComponent(components.Bed) {
			name := "Bed"
			if entity.HasComponent(rlcomponents.Description) {
				name = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.SetDefaultContext("Sleep: "+name, "Order the most wounded idle colonist to rest here.", entity.Blueprint)
			return
		}
		if entity.HasComponent(components.Worker) {
			name := "Colonist"
			if entity.HasComponent(rlcomponents.Description) {
				name = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.SetDefaultContext("Inspect: "+name, "View equipment, inventory, and task filters.", entity.Blueprint)
			return
		}
	}

	tile := s.level.GetTileAt(tX, tY, s.camera.Z)
	if tile != nil {
		t := tile.(*world.Tile)
		walkable := t.Middle.IsEmpty() || !world.TileDefinitions[t.Middle.Type].Solid
		hasFloor := !t.Floor.IsEmpty()
		if walkable && hasFloor {
			s.guiManager.SetDefaultContext("Move Here", "Send colonists to this location.", "")
			return
		}
	}

	s.guiManager.SetDefaultContext("", "", "")
}

// refreshSaveNames re-reads the saves directory and updates the HUD list. Called
// on the first HUD refresh and after a save (gated by saveNamesDirty, not run
// every frame).
func (s *MainState) refreshSaveNames() {
	metas, err := ListSaves()
	if err != nil {
		return
	}
	names := make([]string, len(metas))
	for i, m := range metas {
		names[i] = m.Name
	}
	s.guiManager.SetSaveNames(names)
}

func (s *MainState) refreshHUD() {
	if s.MainSettlement == nil {
		return
	}

	// Count resources held in on-site storage only. Ship hold is separate and
	// managed via the Star Map beam interface.
	resources := map[string]int{"metal_ore": 0, "crystal": 0, "food": 0, "fuel": 0, "biomass": 0}
	var popEntries []gui.PopulationEntry
	for _, entity := range s.level.Entities {
		if entity.HasComponent(components.Storage) {
			st := entity.GetComponent(components.Storage).(*components.StorageComponent)
			for _, item := range st.Items {
				if item.Blueprint != "" {
					if item.HasComponent(components.Material) {
						mc := item.GetComponent(components.Material).(*components.MaterialComponent)
						resources[item.Blueprint] += mc.Quantity
					} else {
						resources[item.Blueprint]++
					}
				}
				if item.HasComponent(rlcomponents.Food) && !item.HasComponent(components.Material) {
					resources["food"]++
				}
			}
		}
		if entity.HasComponent(components.Worker) {
			dc := entity.GetComponent(rlcomponents.Description)
			name := "?"
			if dc != nil {
				name = dc.(*rlcomponents.DescriptionComponent).Name
			}
			state := "idle"
			taskName := ""
			if aiRaw := entity.GetComponent(rlcomponents.AIMemory); aiRaw != nil {
				state = string(aiRaw.(*rlcomponents.AIMemoryComponent).State)
			}
			wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
			if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
				taskName = string(wc.CurrentTask.Action)
			}
			popEntries = append(popEntries, gui.PopulationEntry{Name: name, State: state, Task: taskName, Entity: entity})
		}
	}

	for id, count := range resources {
		s.guiManager.UpdateResource(id, count)
	}
	s.guiManager.RefreshPopulationTab(popEntries)

	// The save list only changes when the player saves — don't hit disk every
	// refreshHUD (~2×/sec). Load once, then only when a save marks it dirty.
	if !s.saveNamesLoaded || s.saveNamesDirty {
		s.refreshSaveNames()
		s.saveNamesLoaded = true
		s.saveNamesDirty = false
	}

	var goalLines []string
	goalLines = append(goalLines, fmt.Sprintf("Day: %d", s.day))
	if s.campaign != nil {
		active := s.campaign.ActiveQuests()
		if len(active) > 0 {
			goalLines = append(goalLines, "Quests:")
			for _, q := range active {
				goalLines = append(goalLines, fmt.Sprintf("  %s", q.Name))
				goalLines = append(goalLines, fmt.Sprintf("    %s", q.Description))
			}
		}
		if done := s.campaign.CompletedQuests(); len(done) > 0 {
			goalLines = append(goalLines, fmt.Sprintf("Completed quests: %d", len(done)))
		}
	}
	s.guiManager.RefreshGoalsTab(goalLines)
	s.refreshCraftQueue()
	if s.MainSettlement != nil {
		available, locked, completed, inProgress, queue := s.researchSnapshot()
		s.guiManager.RefreshResearchModal(available, locked, completed, inProgress, queue)
	}
}

func (s *MainState) refreshCraftQueue() {
	if s.MainSettlement == nil {
		return
	}
	var entries []gui.CraftQueueEntry
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.Action != task_requests.CraftAction || t.Completed {
			continue
		}
		cr, ok := t.Data.(task_requests.CraftRequest)
		if !ok {
			continue
		}
		recipe, found := crafting.GetRecipe(cr.RecipeID)
		if !found {
			continue
		}
		progress := 0.0
		if cr.Required > 0 {
			progress = float64(cr.Progress) / float64(cr.Required)
		}
		var sprite *ebiten.Image
		if ac := factory.GetAppearance(recipe.Output); ac != nil {
			if tex := resource.Textures[ac.Resource]; tex != nil {
				size := ac.SpriteSize
				if size <= 0 {
					size = 24
				}
				sprite = tex.SubImage(image.Rect(ac.SpriteX, ac.SpriteY, ac.SpriteX+size, ac.SpriteY+size)).(*ebiten.Image)
			}
		}
		entries = append(entries, gui.CraftQueueEntry{
			Name:     recipe.Name,
			Sprite:   sprite,
			Progress: progress,
		})
	}
	s.guiManager.RefreshCraftQueue(entries)
}

// buildEvalContext snapshots the live level into the shared win/quest
// evaluation context.
func (s *MainState) buildEvalContext() objective.EvalContext {
	colonistPop := 0
	entityCounts := map[string]int{}
	resourceCounts := map[string]int{}
	accumulate := func(e *ecs.Entity) {
		if e == nil || e.HasComponent(rlcomponents.Dead) {
			return
		}
		if e.HasComponent(components.Worker) {
			colonistPop++
		}
		if e.Blueprint != "" {
			entityCounts[e.Blueprint]++
		}
		if e.HasComponent(components.Storage) {
			st := e.GetComponent(components.Storage).(*components.StorageComponent)
			for _, item := range st.Items {
				if item.Blueprint == "" {
					continue
				}
				if item.HasComponent(components.Material) {
					mc := item.GetComponent(components.Material).(*components.MaterialComponent)
					resourceCounts[item.Blueprint] += mc.Quantity
				} else {
					resourceCounts[item.Blueprint]++
				}
				if item.HasComponent(rlcomponents.Food) && !item.HasComponent(components.Material) {
					resourceCounts["food"]++
				}
			}
		}
	}
	// Built structures (research lab, workbenches, storage lockers) live in
	// StaticEntities — scan both so structure_built / resource objectives see
	// them.
	for _, e := range s.level.Entities {
		accumulate(e)
	}
	for _, e := range s.level.StaticEntities {
		accumulate(e)
	}
	return objective.EvalContext{
		Entities:        s.level.Entities,
		Flags:           s.level.Flags,
		SettlementPop:   map[string]int{"colony": colonistPop},
		StructuresBuilt: entityCounts,
		EntityCounts:    entityCounts,
		ResourceCounts:  resourceCounts,
		KnownTechs:      s.knownTechs(),
		Day:             s.day,
	}
}

// collectDatapads recovers any datapad a colonist is standing on: it consumes
// the item and unlocks its (hidden) quest into the available pool, with a
// "new lead" message. Runs every world step so it works in both real-time and
// Rogue mode.
func (s *MainState) collectDatapads() {
	if s.campaign == nil || s.level == nil {
		return
	}
	// Tiles occupied by living colony colonists.
	occupied := map[[3]int]bool{}
	for _, e := range s.level.Entities {
		if e == nil || e.HasComponent(rlcomponents.Dead) || !e.HasComponent(components.Worker) {
			continue
		}
		if !e.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		occupied[[3]int{pc.GetX(), pc.GetY(), pc.GetZ()}] = true
	}
	if len(occupied) == 0 {
		return
	}
	for _, e := range s.level.Entities {
		if e == nil || !e.HasComponent(components.Datapad) || !e.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if !occupied[[3]int{pc.GetX(), pc.GetY(), pc.GetZ()}] {
			continue
		}
		dp := e.GetComponent(components.Datapad).(*components.DatapadComponent)
		s.level.RemoveEntity(e)
		if q := s.campaign.UnlockQuest(dp.QuestID); q != nil {
			message.PostMessage("Datapad", fmt.Sprintf("Recovered a datapad — new lead: %s. %s", q.Name, q.Description))
		}
	}
}

// evaluateQuests progresses campaign quests against the shared eval context,
// additionally counting the ship hold so "deliver/gather"
// objectives recognise the campaign-wide stockpile.
func (s *MainState) evaluateQuests() {
	if s.campaign == nil {
		return
	}
	ctx := s.buildEvalContext()
	// Resource quests are evaluated against the current site's stockpile only.
	// Players must beam resources to site before a gather quest completes.
	ctx.KilledTargets = s.campaign.KilledTargets

	for _, q := range s.campaign.EvaluateQuests(ctx) {
		if s.wm != nil {
			s.wm.applyQuestReward(q)
		}
		msg := "Quest complete: " + q.Name
		if r := questRewardText(q); r != "" {
			msg += "  (reward: " + r + ")"
		}
		message.PostMessage("Mission", msg)
	}
	s.checkTotalWipe()
}

// checkTotalWipe ends the run in defeat if no colonists remain anywhere — none
// on the live level, none in the ship roster, none left on any frozen system.
func (s *MainState) checkTotalWipe() {
	c := s.campaign
	if c == nil || c.Lost || c.Won {
		return
	}
	stored := c.StoredColonists()
	if cur := c.CurrentLocation(); cur != nil {
		stored -= cur.Colonists // replace this location's stale tally with live
	}
	if stored < 0 {
		stored = 0
	}
	// The ship crew lives on the ship level. Count it here, but avoid
	// double-counting when the ship itself is the live level.
	shipCrew := 0
	if s.wm != nil && s.level != s.wm.ShipLevel() {
		shipCrew = len(s.wm.ShipColonists())
	}
	if stored+countLevelColonists(s.level)+shipCrew > 0 {
		return
	}
	c.Lost = true
	message.PostMessage("Mission", "All colonists are lost. The expedition ends here.")
	s.teardown()
	s.next = NewCampaignEndState(false, "No colonists remained to carry on.")
	s.done = true
}

func (s *MainState) drawTasks(screen *ebiten.Image) {
	if s.MainSettlement != nil {
		viewW := s.camera.ViewW()
		viewH := s.camera.ViewH()
		tw := float32(s.camera.TileW)
		th := float32(s.camera.TileH)
		for _, t := range s.MainSettlement.Tasks.GetTasks() {
			if t.Completed || t.Z != s.camera.Z {
				continue
			}
			var fill, border color.RGBA
			switch t.Action {
			case task_requests.BuildAction:
				fill = color.RGBA{R: 100, G: 200, B: 255, A: 40}
				border = color.RGBA{R: 100, G: 200, B: 255, A: 200}
			case task_requests.DigAction:
				fill = color.RGBA{R: 220, G: 140, B: 40, A: 40}
				border = color.RGBA{R: 220, G: 140, B: 40, A: 200}
			case task_requests.MineAction:
				fill = color.RGBA{R: 200, G: 200, B: 50, A: 40}
				border = color.RGBA{R: 200, G: 200, B: 50, A: 200}
			default:
				fill = color.RGBA{R: 255, G: 200, B: 0, A: 40}
				border = color.RGBA{R: 255, G: 200, B: 0, A: 200}
			}
			// For multi-tile build tasks, highlight the full footprint.
			// t.X/Y is the offset position; startX = t.X - Width/2, startY = t.Y - Height/2.
			fpW, fpH := 1, 1
			if t.Action == task_requests.BuildAction {
				if br, ok := t.Data.(task_requests.BuildRequest); ok {
					if sc := factory.GetSize(br.Type); sc != nil && sc.Width > 0 && sc.Height > 0 {
						fpW, fpH = sc.Width, sc.Height
					}
				}
			}
			startX := t.X - fpW/2
			startY := t.Y - fpH/2
			for dx := 0; dx < fpW; dx++ {
				for dy := 0; dy < fpH; dy++ {
					tx := startX + dx
					ty := startY + dy
					if tx < s.camera.X || tx >= s.camera.X+viewW || ty < s.camera.Y || ty >= s.camera.Y+viewH {
						continue
					}
					sx, sy := s.camera.WorldToScreenF(tx, ty)
					vector.DrawFilledRect(screen, sx, sy, tw, th, fill, false)
					vector.StrokeRect(screen, sx, sy, tw, th, 1, border, false)
				}
			}
		}
	}

	for i, p := range s.roguePath {
		sx, sy := s.camera.WorldToScreenF(p[0], p[1])
		tw := float32(s.camera.TileW)
		th := float32(s.camera.TileH)
		alpha := uint8(60)
		if i == len(s.roguePath)-1 {
			alpha = 100 // destination tile slightly brighter
		}
		vector.DrawFilledRect(screen, sx, sy, tw, th, color.RGBA{R: 80, G: 200, B: 255, A: alpha}, false)
		vector.StrokeRect(screen, sx, sy, tw, th, 1, color.RGBA{R: 80, G: 200, B: 255, A: 180}, false)
	}

	if s.hoverActive {
		tw := float32(s.camera.TileW)
		th := float32(s.camera.TileH)
		footprintW, footprintH := 1, 1
		if s.CursorMode == gui.CursorModeBuild {
			if sc := factory.GetSize(s.buildMode); sc != nil && sc.Width > 0 && sc.Height > 0 {
				footprintW = sc.Width
				footprintH = sc.Height
			}
		}
		// hoverTileX/Y is the top-left corner of the footprint, matching addBuildTask.
		for dx := 0; dx < footprintW; dx++ {
			for dy := 0; dy < footprintH; dy++ {
				sx, sy := s.camera.WorldToScreenF(s.hoverTileX+dx, s.hoverTileY+dy)
				vector.DrawFilledRect(screen, sx, sy, tw, th,
					color.RGBA{R: 255, G: 255, B: 255, A: 40}, false)
				vector.StrokeRect(screen, sx, sy, tw, th,
					1, color.RGBA{R: 255, G: 255, B: 255, A: 120}, false)
			}
		}
	}

	s.drawStorePickHighlight(screen)
}

// drawStorePickHighlight renders a yellow halo over the tile of the item
// picked in Phase 1 of Store mode, on top of the SelectedComponent's sprite
// brightening.
func (s *MainState) drawStorePickHighlight(screen *ebiten.Image) {
	item := s.storeTool.item
	if item == nil || !item.HasComponent(rlcomponents.Position) {
		return
	}
	pc := item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if pc.GetZ() != s.camera.Z {
		return
	}
	tw := float32(s.camera.TileW)
	th := float32(s.camera.TileH)
	sx, sy := s.camera.WorldToScreenF(pc.GetX(), pc.GetY())

	stroke := color.RGBA{R: 255, G: 220, B: 60, A: 220}
	vector.DrawFilledRect(screen, sx, sy, tw, th, color.RGBA{R: 255, G: 220, B: 60, A: 60}, false)
	vector.StrokeRect(screen, sx, sy, tw, th, 3, stroke, false)
	vector.StrokeRect(screen, sx+2, sy+2, tw-4, th-4, 1, stroke, false)
}

// findStartingPlaza picks a spawn anchor with `slots` adjacent standable
// tiles in a horizontal row, preferring region anchors emitted by terrain
// primers ("starting_floor", "starting_asteroid", "station_hub"). Falls back
// to a random scan at preferZ. Returns (centerX, centerY, z); x = -1 if no
// valid plaza was found.
func findStartingPlaza(level *world.Level, preferZ, slots int) (int, int, int) {
	for _, tag := range []string{"starting_floor", "starting_asteroid", "station_hub"} {
		for _, p := range level.Regions[tag] {
			if cx, cy, ok := tryPlaza(level, p[0], p[1], p[2], slots); ok {
				return cx, cy, p[2]
			}
		}
	}
	w, h := level.GetWidth(), level.GetHeight()
	for i := 0; i < 1000; i++ {
		x := rand.Intn(w)
		y := rand.Intn(h)
		if cx, cy, ok := tryPlaza(level, x, y, preferZ, slots); ok {
			return cx, cy, preferZ
		}
	}
	return -1, -1, preferZ
}

func tryPlaza(level *world.Level, x, y, z, slots int) (int, int, bool) {
	half := slots / 2
	for i := -half; i < slots-half; i++ {
		if !isStandable(level, x+i, y, z) {
			return 0, 0, false
		}
	}
	return x, y, true
}

func isStandable(level *world.Level, x, y, z int) bool {
	t := level.GetTilePtr(x, y, z)
	if t == nil {
		return false
	}
	if t.IsSolid() || t.IsWater() {
		return false
	}
	if world.IsSpaceTile(t) {
		return false
	}
	if level.GetEntityAt(x, y, z) != nil {
		return false
	}
	return true
}
