package game

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/construction"
	"github.com/mechanical-lich/scifi_settlements/internal/crafting"
	"github.com/mechanical-lich/scifi_settlements/internal/effect"
	"github.com/mechanical-lich/scifi_settlements/internal/eventsystem"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/game/listeners"
	"github.com/mechanical-lich/scifi_settlements/internal/generation"
	"github.com/mechanical-lich/scifi_settlements/internal/gui"
	fspath "github.com/mechanical-lich/scifi_settlements/internal/path"
	"github.com/mechanical-lich/scifi_settlements/internal/research"
	"github.com/mechanical-lich/scifi_settlements/internal/scenario"
	"github.com/mechanical-lich/scifi_settlements/internal/settlement"
	"github.com/mechanical-lich/scifi_settlements/internal/systems"
	"github.com/mechanical-lich/scifi_settlements/internal/task_requests"
	"github.com/mechanical-lich/scifi_settlements/internal/wincondition"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

type SettlementConfig struct {
	Name            string
	ScenarioID      string
	LightingMode    string // "" = use scenario default
	LightingAmbient int    // only used when LightingMode == "fixed"
}

type MainState struct {
	level            *world.Level
	CameraX          int
	CameraY          int
	CameraZ          int
	CursorMode       gui.CursorModeType
	guiManager       *gui.GUIManager
	systemManager    *ecs.SystemManager
	gm               *GameMaster
	selectedEntity   *ecs.Entity
	TileSizeW        int
	TileSizeH        int
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
	winEval          *wincondition.Evaluator
	mouseDragging    bool
	lastDragTileX    int
	lastDragTileY    int
	hoverTileX       int
	hoverTileY       int
	hoverActive      bool
	settlementCfg    SettlementConfig
	done             bool
	next             state.StateInterface
}

var _ state.StateInterface = (*MainState)(nil)

func newMainStateBase(cfg SettlementConfig) (*MainState, error) {
	s := &MainState{
		CursorMode:    gui.CursorModeDefault,
		systemManager: &ecs.SystemManager{},
		op:            &ebiten.DrawImageOptions{},
		TileSizeW:     config.Global().TileSizeW,
		TileSizeH:     config.Global().TileSizeH,
		CameraX:       0,
		CameraY:       0,
		CameraZ:       config.Global().StartingZ,
		settlementCfg: cfg,
		buildMode:     "hull_wall",
	}

	s.initiativeSystem = &rlsystems.InitiativeSystem{
		Speed: 1,
		OnEntityTurn: func(entity *ecs.Entity) {
			if entity.HasComponent(components.Appearance) {
				ac := entity.GetComponent(components.Appearance).(*components.AppearanceComponent)
				ac.Bounce = !ac.Bounce
			}
		},
	}
	s.systemManager.AddSystem(s.initiativeSystem)

	s.cleanUpSystem = &rlsystems.CleanUpSystem{
		OnEntityDead: func(levelInterface rlworld.LevelInterface, entity *ecs.Entity) {
			level := levelInterface.(*world.Level)
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
	s.systemManager.AddSystem(aiSystem)
	s.systemManager.AddSystem(systems.NewFactionAISystem())
	s.systemManager.AddSystem(&systems.ScriptedAISystem{})
	s.systemManager.AddSystem(&systems.WorkerSystem{})
	s.systemManager.AddSystem(&systems.RadiationSystem{})
	s.systemManager.AddSystem(&systems.LightingSystem{})
	s.systemManager.AddSystem(&rlsystems.DoorSystem{AppearanceType: components.Appearance})
	s.systemManager.AddSystem(&systems.FactionDoorSystem{})
	s.systemManager.AddSystem(&systems.ScriptSystem{})
	s.systemManager.AddSystem(&rlsystems.StatusConditionSystem{})

	eq := event.GetQueuedInstance()
	eq.RegisterListener(&listeners.MessageListener{}, message.MessageEventType)
	eq.RegisterListener(&listeners.KillListener{}, eventsystem.EntityKilled)
	eq.RegisterListener(&listeners.TaskListener{}, eventsystem.TaskCompleted)
	eq.RegisterListener(&listeners.StructureListener{}, eventsystem.StructureBuilt)
	eq.RegisterListener(&listeners.ResearchListener{}, eventsystem.ResearchDone)

	event.GetQueuedInstance().RegisterListener(s, input.MouseClickEventType)
	event.GetQueuedInstance().RegisterListener(s, input.MouseReleasedEventType)
	event.GetQueuedInstance().RegisterListener(s, input.KeyPressEventType)
	event.GetQueuedInstance().RegisterListener(s, input.MouseWheelEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.BuildOptionChangedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.CursorModeChangedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.MainMenuEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.SaveGameEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.LoadGameEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.CraftRequestedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.StationClickedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.ResearchStationClickedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.ResearchRequestedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.ColonistSelectedEventType)
	event.GetQueuedInstance().RegisterListener(s, eventsystem.ResearchDone)
	event.GetQueuedInstance().RegisterListener(s, gui.EquipItemRequestedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.UnequipItemRequestedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.SetTaskFilterEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.DropOffRequestedEventType)

	return s, nil
}

func newMainStateFromLevel(level *world.Level, cfg SettlementConfig) (*MainState, error) {
	s, err := newMainStateBase(cfg)
	if err != nil {
		return nil, err
	}
	s.level = level
	s.guiManager = gui.NewGUIManager()
	s.gm = &GameMaster{}
	s.gm.Init(level)

	gcfg := config.Global()
	s.CameraZ = gcfg.StartingZ
	x, y := s.gm.GetFreeSpaceAtZ(s.CameraZ)
	if x != -1 {
		sidebarTiles := 200/s.TileSizeW + 1
		viewW := gcfg.WorldWidth / s.TileSizeW
		viewH := gcfg.WorldHeight / s.TileSizeH
		s.CameraX = x - sidebarTiles - (viewW-sidebarTiles)/2
		s.CameraY = y - viewH/2
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
		s.winEval = wincondition.New(scenario.Active().WinConditions)
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
	cfg := config.Global()
	mapW := cfg.WorldGenSizeW
	mapH := cfg.WorldGenSizeH
	mapZ := cfg.WorldGenSizeZ

	// Select the scenario before terrain generation so its WorldConfig (if
	// any) can drive the build. The original flow selected after terrain;
	// the post-terrain block below is a no-op now but kept for safety.
	if s.settlementCfg.ScenarioID != "" {
		if err := scenario.SelectByID(s.settlementCfg.ScenarioID); err != nil {
			log.Printf("scenario %q not found, using random", s.settlementCfg.ScenarioID)
			_ = scenario.SelectRandom()
		}
	} else if len(scenario.AllEnabled()) > 0 {
		_ = scenario.SelectRandom()
	}

	var activeScenario *scenario.Scenario
	if len(scenario.AllEnabled()) > 0 {
		activeScenario = scenario.Active()
	}

	if activeScenario != nil && activeScenario.World != nil {
		sc := activeScenario
		opts := generation.BuildWorldOptions{
			Width:         mapW,
			Height:        mapH,
			Depth:         mapZ,
			Terrain:       sc.World.Terrain,
			TerrainParams: sc.World.TerrainParams,
			BiomeMapType:  sc.World.BiomeMap.Type,
			BiomeMapScale: sc.World.BiomeMap.Scale,
			BiomeIDs:      sc.World.BiomeMap.Biomes,
			BiomeSingle:   sc.World.BiomeMap.Single,
		}
		for _, fb := range sc.World.Features {
			opts.Features = append(opts.Features, generation.FeatureSpec{
				Kind: fb.Kind, Count: fb.Count, Biome: fb.Biome,
				InRegion: fb.InRegion, Jitter: fb.Jitter,
				MinZ: fb.MinZ, MaxZ: fb.MaxZ, Params: fb.Params,
			})
		}
		level, err := generation.BuildWorld(opts)
		if err != nil {
			log.Printf("BuildWorld: %v (falling back to legacy planet)", err)
			planetCfg := generation.DefaultPlanetConfig(mapZ)
			s.level = generation.NewPlanetLevel(mapW, mapH, mapZ, planetCfg)
		} else {
			s.level = level
		}
	} else {
		planetCfg := generation.DefaultPlanetConfig(mapZ)
		s.level = generation.NewPlanetLevel(mapW, mapH, mapZ, planetCfg)
	}
	s.guiManager = gui.NewGUIManager()
	s.gm = &GameMaster{}
	s.gm.Init(s.level)

	// Prefer the level's own surface Z (set by the primer) so station/asteroid
	// scenarios don't try to spawn at the legacy hardcoded z=5.
	startingZ := s.level.SurfaceZ
	if startingZ <= 0 {
		startingZ = cfg.StartingZ
	}
	x, y, startingZ := findStartingPlaza(s.level, startingZ, 5)
	if x != -1 {
		// Position camera so colonists (at x-2..x+2) appear centered in the
		// visible world area (to the right of the 200px sidebar).
		sidebarTiles := 200/s.TileSizeW + 1
		viewW := cfg.WorldWidth / s.TileSizeW
		viewH := cfg.WorldHeight / s.TileSizeH
		s.CameraX = x - sidebarTiles - (viewW-sidebarTiles)/2
		s.CameraY = y - viewH/2
		s.CameraZ = startingZ
		name := s.settlementCfg.Name
		if name == "" {
			name = "Colony Alpha"
		}
		s.MainSettlement = settlement.NewSettlement(x, y, startingZ)
		// Rename to player-chosen name
		oldName := s.MainSettlement.Name
		s.MainSettlement.Name = name
		delete(settlement.Settlements, oldName)
		settlement.Settlements[name] = s.MainSettlement

		// Spawn starting colonists
		for i := 0; i < 5; i++ {
			colonist, err := factory.Create("colonist", x+i-2, y, startingZ)
			if err == nil {
				colonist.AddComponent(&components.SettlementComponent{Name: name})
				colonist.AddComponent(&components.WorkerComponent{})
				s.level.AddEntity(colonist)
			}
		}
	}

	if s.settlementCfg.ScenarioID != "" {
		if err := scenario.SelectByID(s.settlementCfg.ScenarioID); err != nil {
			log.Printf("scenario %q not found, using random", s.settlementCfg.ScenarioID)
			_ = scenario.SelectRandom()
		}
	} else if len(scenario.AllEnabled()) > 0 {
		_ = scenario.SelectRandom()
	}

	if len(scenario.AllEnabled()) > 0 {
		s.winEval = wincondition.New(scenario.Active().WinConditions)
		s.applyScenarioLighting(s.level)
		if sc := scenario.Active(); len(sc.SetupScripts) > 0 {
			s.level.Flags["start_x"] = float64(x)
			s.level.Flags["start_y"] = float64(y)
			s.level.Flags["start_z"] = float64(startingZ)
			if s.MainSettlement != nil {
				s.level.Flags["settlement_name"] = s.MainSettlement.Name
			}
			s.level.Flags["colonist_faction"] = "colony"
			RunSetupScripts(sc.SetupScripts, s.level)
		}
	}
	s.day = 0
	s.lastDay = 0
}

func (s *MainState) applyScenarioLighting(level *world.Level) {
	sc := scenario.Active()
	// Player override takes priority over scenario default.
	if s.settlementCfg.LightingMode != "" {
		level.LightMode = s.settlementCfg.LightingMode
		level.FixedAmbient = s.settlementCfg.LightingAmbient
	} else {
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
	s.handleInput()
	if s.MainSettlement != nil {
		s.guiManager.SetKnownTechs(s.MainSettlement.KnownTechs)
	}
	s.guiManager.Update()
	s.updateHovered()

	fps := ebiten.ActualFPS()
	tps := ebiten.ActualTPS()
	day := s.day
	if s.level != nil {
		day = s.level.Day
	}
	ebiten.SetWindowTitle(fmt.Sprintf("%s — Day:%d Hour:%d Z:%d FPS:%.0f TPS:%.0f", config.Global().Title, day, s.level.Hour, s.CameraZ, fps, tps))

	if !s.Paused {
		s.tick++
		s.gm.Update()
		s.systemManager.UpdateSystems(s.level)
		for _, entity := range s.level.Entities {
			if entity == nil {
				continue
			}
			if entity.HasComponent(rlcomponents.Inanimate) {
				continue
			}
			s.systemManager.UpdateSystemsForEntity(s.level, entity)
		}
		s.cleanUpSystem.Update(s.level)
		effect.GetEffectManager().Update()
	}

	if s.tick%30 == 0 {
		s.purgeCompletedTasks()
		s.refreshHUD()
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
				s.checkWinConditions()
			}
		}
	}

	return s.next
}

func (s *MainState) Draw(screen *ebiten.Image) {
	if s.worldImage == nil {
		s.worldImage = ebiten.NewImage(config.Global().WorldWidth, config.Global().WorldHeight)
	}

	viewW := config.Global().WorldWidth / s.TileSizeW
	viewH := config.Global().WorldHeight / s.TileSizeH
	world.DrawLevel(s.level, s.worldImage, s.CameraX, s.CameraY, s.CameraZ, s.TileSizeW, s.TileSizeH, config.Global().SpriteSizeW, config.Global().SpriteSizeH, viewW, viewH)
	world.DrawLightOverlay(s.level, s.worldImage, s.CameraX, s.CameraY, s.CameraZ, s.TileSizeW, s.TileSizeH, viewW, viewH)
	world.DrawRadiationOverlay(s.level, s.worldImage, s.CameraX, s.CameraY, s.CameraZ, s.TileSizeW, s.TileSizeH, viewW, viewH)

	s.drawTasks(s.worldImage)
	cfg := config.Global()
	effect.GetEffectManager().Draw(s.worldImage, s.CameraX, s.CameraY, s.CameraZ, s.TileSizeW, s.TileSizeH, cfg.SpriteSizeW, cfg.SpriteSizeH)
	screen.DrawImage(s.worldImage, nil)
	s.guiManager.Draw(screen)
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
		s.CursorMode = ev.Mode
	case gui.MainMenuEvent:
		switch ev.Action {
		case "quit":
			os.Exit(0)
		case "newgame":
			s.next = NewTitleState()
			s.done = true
		}
	case gui.SaveGameEvent:
		name := s.settlementCfg.Name
		if name == "" {
			name = "colony"
		}
		if err := SaveSettlement(s.level, SaveMeta{
			Name:       name,
			ScenarioID: s.settlementCfg.ScenarioID,
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
			s.next = loaded
			s.done = true
		}
	case gui.CraftRequestedEvent:
		s.addCraftTask(ev.RecipeID, ev.Station)
	case gui.StationClickedEvent:
		if s.MainSettlement != nil {
			cs := ev.Station.GetComponent(components.CraftingStation).(*components.CraftingStationComponent)
			recipes := crafting.RecipesByStation(cs.StationID)
			title := cs.StationID
			if ev.Station.HasComponent(rlcomponents.Description) {
				title = ev.Station.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.OpenCraftingModal(title, recipes, ev.Station)
		}
	case gui.ResearchStationClickedEvent:
		if s.MainSettlement != nil {
			available, completed, inProgress, queue := s.researchSnapshot()
			s.guiManager.OpenResearchModal(ev.Station, available, completed, inProgress, queue)
		}
	case gui.ResearchRequestedEvent:
		s.addResearchTask(ev.TechKey, ev.Station)
	case eventsystem.ResearchDoneEvent:
		if s.MainSettlement != nil {
			s.MainSettlement.UnlockTech(ev.TechKey)
			s.guiManager.SetKnownTechs(s.MainSettlement.KnownTechs)
		}
	case gui.ColonistSelectedEvent:
		s.openColonistModal(ev.Entity)
	case gui.EquipItemRequestedEvent:
		s.addEquipTask(ev.ColonistEntity, ev.ItemBlueprint)
	case gui.UnequipItemRequestedEvent:
		s.addUnequipTask(ev.ColonistEntity, ev.Slot)
	case gui.SetTaskFilterEvent:
		s.applyTaskFilter(ev.Colonist, ev.Action, ev.Enabled)
	case gui.DropOffRequestedEvent:
		s.requestDropOff(ev.Colonist, ev.Item)
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
func (s *MainState) researchSnapshot() (available, completed, inProgress []research.Tech, queue []gui.ResearchQueueEntry) {
	if s.MainSettlement == nil {
		return
	}

	for _, k := range s.MainSettlement.KnownTechs {
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

	for _, t := range research.AvailableTechs(s.MainSettlement.KnownTechs) {
		if inProgressSet[t.Key] {
			continue
		}
		available = append(available, t)
	}
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
	if s.MainSettlement.HasTech(techKey) {
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
			ic := item.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent)
			if ic.Slot == rlcomponents.BagSlot || ic.Slot == "" {
				continue
			}
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

func (s *MainState) applyTaskFilter(colonist *ecs.Entity, action string, enabled bool) {
	if colonist == nil || !colonist.HasComponent(components.Worker) {
		return
	}
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	a := task.TaskAction(action)
	if enabled {
		// Add to allowed list if not already present
		for _, existing := range wc.AllowedTasks {
			if existing == a {
				return
			}
		}
		wc.AllowedTasks = append(wc.AllowedTasks, a)
		// If every filterable action is now allowed, clear the filter entirely
		if len(wc.AllowedTasks) >= len(task_requests.FilterableActions) {
			wc.AllowedTasks = nil
		}
	} else {
		// Initialize with all filterable actions before removing one
		if wc.AllowedTasks == nil {
			for _, fa := range task_requests.FilterableActions {
				wc.AllowedTasks = append(wc.AllowedTasks, fa.Action)
			}
		}
		for i, existing := range wc.AllowedTasks {
			if existing == a {
				wc.AllowedTasks = append(wc.AllowedTasks[:i], wc.AllowedTasks[i+1:]...)
				return
			}
		}
	}
}

func (s *MainState) requestDropOff(colonist *ecs.Entity, item *ecs.Entity) {
	if colonist == nil {
		return
	}
	if !colonist.HasComponent(rlcomponents.Inventory) || !colonist.HasComponent(rlcomponents.AIMemory) || !colonist.HasComponent(components.Settlement) {
		return
	}
	inv := colonist.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	if len(inv.Bag) == 0 {
		return
	}
	sc := colonist.GetComponent(components.Settlement).(*components.SettlementComponent)
	var storage *ecs.Entity
	for _, e := range append(s.level.Entities, s.level.StaticEntities...) {
		if !e.HasComponent(components.Storage) {
			continue
		}
		st := e.GetComponent(components.Storage).(*components.StorageComponent)
		if st.OwnedBy == sc.Name {
			storage = e
			break
		}
	}
	if storage == nil {
		return
	}
	storagePC := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := colonist.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		wc.CurrentTask.Stop()
		wc.CurrentTask = nil
	}
	aiMemory.TargetX = storagePC.GetX()
	aiMemory.TargetY = storagePC.GetY()
	aiMemory.TargetZ = storagePC.GetZ()
	wc.DropOffItem = item
	aiMemory.State = "dropoff"
}

func (s *MainState) handleInput() {
	if s.mouseDragging {
		cX, cY := ebiten.CursorPosition()
		tX := cX/s.TileSizeW + s.CameraX
		tY := cY/s.TileSizeH + s.CameraY
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
}

func (s *MainState) handleKeyPress(e input.KeyPressEvent) {
	if s.guiManager.GetInputFocused() {
		return
	}

	for _, k := range e.Keys {
		switch k.String() {
		case "W":
			s.CameraY--
		case "S":
			s.CameraY++
		case "A":
			s.CameraX--
		case "D":
			s.CameraX++
		}
	}

	if !e.JustPressed {
		return
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		s.Paused = !s.Paused
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) && s.CameraZ > 0 {
		s.CameraZ--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) && s.CameraZ < config.Global().WorldGenSizeZ-1 {
		s.CameraZ++
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
		s.guiManager.ToggleModal("mainMenu")
	}
}

func (s *MainState) handleMouseWheel(e input.MouseWheelEvent) {
	if s.guiManager.GetMouseFocused() {
		return
	}
	zooming := (e.Y > 0.1 && s.TileSizeW < 96) || (e.Y < -0.1 && s.TileSizeW > 16)
	if !zooming {
		return
	}

	// World tile under the mouse before zoom
	mX, mY := ebiten.CursorPosition()
	worldX := mX/s.TileSizeW + s.CameraX
	worldY := mY/s.TileSizeH + s.CameraY

	if e.Y > 0.1 {
		s.TileSizeW *= 2
		s.TileSizeH *= 2
	} else {
		s.TileSizeW /= 2
		s.TileSizeH /= 2
	}

	// Reposition camera so the same world tile stays under the mouse
	s.CameraX = worldX - mX/s.TileSizeW
	s.CameraY = worldY - mY/s.TileSizeH
}

func (s *MainState) handleMouseClick(e input.MouseClickEvent) {
	if s.guiManager.GetMouseFocused() || s.guiManager.WithinModalBounds(ebiten.CursorPosition()) {
		return
	}

	cX, cY := ebiten.CursorPosition()
	tX := cX/s.TileSizeW + s.CameraX
	tY := cY/s.TileSizeH + s.CameraY

	if e.Button == ebiten.MouseButtonLeft {
		if s.CursorMode == gui.CursorModeDefault {
			ent := s.level.GetEntityAt(tX, tY, s.CameraZ)

			if ent != nil && ent.HasComponent(components.FactionAI) && s.MainSettlement != nil {
				// Attack hostile entity
				s.MainSettlement.Tasks.AddTask(&task.Task{
					Action: task_requests.AttackAction, Data: ent,
					X: tX, Y: tY, Z: s.CameraZ, Escalated: true,
				})
			} else if ent != nil && ent.HasComponent(components.Choppable) && s.MainSettlement != nil {
				// Harvest choppable entity (flora, crystals, etc.)
				s.addMineTask(tX, tY)
			} else if ent != nil && ent.HasComponent(rlcomponents.Item) && s.MainSettlement != nil {
				// Retrieve dropped item
				s.MainSettlement.Tasks.AddTask(&task.Task{
					Action: task_requests.RetrieveAction,
					Data:   task_requests.RetrieveRequest{Item: ent},
					X:      tX, Y: tY, Z: s.CameraZ, Escalated: true,
				})
			} else {
				// Select entity (colonist, building, etc.)
				for _, e := range s.level.Entities {
					e.RemoveComponent(components.Selected)
				}
				s.selectedEntity = nil
				if ent == nil {
					for _, se := range s.level.StaticEntities {
						pc := se.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
						if pc.GetX() == tX && pc.GetY() == tY && pc.GetZ() == s.CameraZ {
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
					}
				} else if s.MainSettlement != nil {
					// Walkable empty tile — queue a move task
					tile := s.level.GetTileAt(tX, tY, s.CameraZ)
					if tile != nil {
						t := tile.(*world.Tile)
						walkable := t.Middle.IsEmpty() || !world.TileDefinitions[t.Middle.Type].Solid
						if walkable && !t.Floor.IsEmpty() {
							s.MainSettlement.Tasks.AddTask(&task.Task{
								X: tX, Y: tY, Z: s.CameraZ, Escalated: true,
							})
						}
					}
				}
				// Escalate any pending task at this tile
				if s.MainSettlement != nil {
					for _, t := range s.MainSettlement.Tasks.GetTasks() {
						if t.X == tX && t.Y == tY && t.Z == s.CameraZ {
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
					if t.X == tX && t.Y == tY && t.Z == s.CameraZ {
						t.Complete()
						s.MainSettlement.Tasks.RemoveTask(t)
					}
				}
			case gui.CursorModeAttack:
				target := s.level.GetEntityAt(tX, tY, s.CameraZ)
				if target != nil && target.HasComponent(components.FactionAI) {
					for _, colonist := range s.level.Entities {
						if colonist.HasComponent(components.Worker) && !colonist.HasComponent(rlcomponents.Dead) {
							wc := colonist.GetComponent(components.Worker).(*components.WorkerComponent)
							if wc.CurrentTask == nil || wc.CurrentTask.Completed {
								s.MainSettlement.Tasks.AddTask(&task.Task{
									Action: task_requests.AttackAction,
									Data:   target,
									X:      tX, Y: tY, Z: s.CameraZ,
								})
								break
							}
						}
					}
				}
			}
		}
	}

	if e.Button == ebiten.MouseButtonRight {
		if s.CursorMode != gui.CursorModeDefault {
			// Cancel active order, return to default
			event.GetQueuedInstance().QueueEvent(gui.CursorModeChangedEvent{Mode: gui.CursorModeDefault})
		} else if s.MainSettlement != nil {
			// Cancel any pending task at this tile
			for _, t := range s.MainSettlement.Tasks.GetTasks() {
				if t.X == tX && t.Y == tY && t.Z == s.CameraZ {
					t.Complete()
					s.MainSettlement.Tasks.RemoveTask(t)
				}
			}
		}
	}
}

func (s *MainState) addBuildTask(x, y int) {
	if s.MainSettlement == nil {
		return
	}
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == s.CameraZ && t.Action == task_requests.BuildAction {
			return
		}
	}
	buildable := construction.GetBuildable(s.buildMode)
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.BuildAction,
		Data:   task_requests.BuildRequest{X: x, Y: y, Z: s.CameraZ, Type: s.buildMode, Required: buildable.BuildTime},
		X:      x, Y: y, Z: s.CameraZ,
	})
}

func (s *MainState) addDigTask(x, y int) {
	if s.MainSettlement == nil {
		return
	}
	for _, t := range s.MainSettlement.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == s.CameraZ && t.Action == task_requests.DigAction {
			return
		}
	}
	tile := s.level.GetTileAt(x, y, s.CameraZ)
	if tile == nil || !tile.IsSolid() {
		return
	}
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.DigAction,
		Data:   task_requests.DigRequest{X: x, Y: y, Z: s.CameraZ, Required: 5},
		X:      x, Y: y, Z: s.CameraZ,
	})
}

func (s *MainState) addMineTask(x, y int) {
	if s.MainSettlement == nil {
		return
	}
	// Prefer a Choppable entity at the tile (e.g. alien_crystal). Required is
	// derived from the entity's Choppable.Health so harder things take longer.
	var entBuf []*ecs.Entity
	s.level.GetEntitiesAt(x, y, s.CameraZ, &entBuf)
	for _, e := range entBuf {
		if !e.HasComponent(components.Choppable) {
			continue
		}
		ch := e.GetComponent(components.Choppable).(*components.ChoppableComponent)
		req := task_requests.MineRequest{X: x, Y: y, Z: s.CameraZ, Required: ch.Health * 10, Target: e}
		s.MainSettlement.Tasks.AddTask(&task.Task{
			Action: task_requests.MineAction,
			Data:   req,
			X:      x, Y: y, Z: s.CameraZ,
		})
		return
	}

	tile := s.level.GetTileAt(x, y, s.CameraZ)
	if tile == nil {
		return
	}
	t := tile.(*world.Tile)
	if t.Middle.IsEmpty() {
		return
	}
	tileName := world.TileDefinitions[t.Middle.Type].Name
	if tileName != "ore_deposit" && tileName != "crystal_vein" {
		return
	}
	s.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.MineAction,
		Data:   task_requests.MineRequest{X: x, Y: y, Z: s.CameraZ, Required: 50},
		X:      x, Y: y, Z: s.CameraZ,
	})
}

func (s *MainState) updateHovered() {
	cfg := config.Global()
	cX, cY := ebiten.CursorPosition()
	const sidebarW = 200
	if cX < sidebarW || cX >= cfg.WorldWidth || cY < 0 || cY >= cfg.WorldHeight {
		s.guiManager.ClearHover()
		s.hoverActive = false
		return
	}
	tX := cX/s.TileSizeW + s.CameraX
	tY := cY/s.TileSizeH + s.CameraY
	s.hoverTileX = tX
	s.hoverTileY = tY
	s.hoverActive = true

	if entity := s.level.GetEntityAt(tX, tY, s.CameraZ); entity != nil {
		s.guiManager.SetHoveredEntity(entity)
		if s.CursorMode == gui.CursorModeDefault {
			s.updateDefaultContext(tX, tY)
		}
		return
	}

	tile := s.level.GetTileAt(tX, tY, s.CameraZ)
	if tile == nil {
		s.guiManager.ClearHover()
		if s.CursorMode == gui.CursorModeDefault {
			s.guiManager.SetDefaultContext("", "", "")
		}
		s.hoverActive = false
		return
	}
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
	s.guiManager.SetHoveredTile(gui.HoveredTileInfo{
		Name:       def.Name,
		FloorName:  floorName,
		X:          tX,
		Y:          tY,
		Z:          s.CameraZ,
		LightLevel: t.LightLevel,
		Radiation:  int(t.Radiation),
		Solid:      def.Solid,
		Water:      def.Water,
		Air:        def.Air,
		Space:      def.Space,
	})
	if s.CursorMode == gui.CursorModeDefault {
		s.updateDefaultContext(tX, tY)
	}
}

// updateDefaultContext detects what a Default-mode left-click would do at (tX,tY)
// and updates the HUD context hint accordingly.
func (s *MainState) updateDefaultContext(tX, tY int) {
	if entity := s.level.GetEntityAt(tX, tY, s.CameraZ); entity != nil {
		if entity.HasComponent(components.FactionAI) {
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
		if entity.HasComponent(components.Worker) {
			name := "Colonist"
			if entity.HasComponent(rlcomponents.Description) {
				name = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
			}
			s.guiManager.SetDefaultContext("Inspect: "+name, "View equipment, inventory, and task filters.", entity.Blueprint)
			return
		}
	}

	tile := s.level.GetTileAt(tX, tY, s.CameraZ)
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

func (s *MainState) refreshHUD() {
	if s.MainSettlement == nil {
		return
	}

	// Count resources across all storage lockers
	resources := map[string]int{"metal_ore": 0, "crystal": 0, "food": 0}
	var popEntries []gui.PopulationEntry
	for _, entity := range s.level.Entities {
		if entity.HasComponent(components.Storage) {
			st := entity.GetComponent(components.Storage).(*components.StorageComponent)
			for _, item := range st.Items {
				if item.Blueprint != "" {
					resources[item.Blueprint]++
				}
				if item.HasComponent(rlcomponents.Food) {
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

	if metas, err := ListSaves(); err == nil {
		names := make([]string, len(metas))
		for i, m := range metas {
			names[i] = m.Name
		}
		s.guiManager.SetSaveNames(names)
	}

	var goalLines []string
	goalLines = append(goalLines, fmt.Sprintf("Day: %d", s.day))
	if len(scenario.AllEnabled()) > 0 {
		sc := scenario.Active()
		goalLines = append(goalLines, sc.Name)
		for _, rule := range sc.WinConditions.Rules {
			goalLines = append(goalLines, fmt.Sprintf("  %s", rule.Message))
		}
	}
	s.guiManager.RefreshGoalsTab(goalLines)
	s.refreshCraftQueue()
	if s.MainSettlement != nil {
		available, completed, inProgress, queue := s.researchSnapshot()
		s.guiManager.RefreshResearchModal(available, completed, inProgress, queue)
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

func (s *MainState) checkWinConditions() {
	if s.winEval == nil {
		return
	}

	colonistPop := 0
	for _, e := range s.level.Entities {
		if e.HasComponent(components.Worker) && !e.HasComponent(rlcomponents.Dead) {
			colonistPop++
		}
	}

	ctx := wincondition.EvalContext{
		Entities:      s.level.Entities,
		Flags:         s.level.Flags,
		SettlementPop: map[string]int{"colony": colonistPop},
		Day:           s.day,
	}

	if rule, ok := s.winEval.EvalDaysSurvived(ctx); ok {
		message.AddMessage(rule.Message)
		s.Paused = true
		return
	}
	if rule, ok := s.winEval.EvalColonistEliminated(ctx); ok {
		message.AddMessage(rule.Message)
		s.Paused = true
	}
}

func (s *MainState) drawTasks(screen *ebiten.Image) {
	if s.MainSettlement != nil {
		viewW := config.Global().WorldWidth / s.TileSizeW
		viewH := config.Global().WorldHeight / s.TileSizeH
		tw := float32(s.TileSizeW)
		th := float32(s.TileSizeH)
		for _, t := range s.MainSettlement.Tasks.GetTasks() {
			if t.Completed || t.Z != s.CameraZ {
				continue
			}
			if t.X < s.CameraX || t.X >= s.CameraX+viewW || t.Y < s.CameraY || t.Y >= s.CameraY+viewH {
				continue
			}
			sx := float32((t.X - s.CameraX) * s.TileSizeW)
			sy := float32((t.Y - s.CameraY) * s.TileSizeH)
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
			vector.DrawFilledRect(screen, sx, sy, tw, th, fill, false)
			vector.StrokeRect(screen, sx, sy, tw, th, 1, border, false)
		}
	}

	if s.hoverActive {
		sx := float32((s.hoverTileX - s.CameraX) * s.TileSizeW)
		sy := float32((s.hoverTileY - s.CameraY) * s.TileSizeH)
		vector.DrawFilledRect(screen, sx, sy, float32(s.TileSizeW), float32(s.TileSizeH),
			color.RGBA{R: 255, G: 255, B: 255, A: 40}, false)
		vector.StrokeRect(screen, sx, sy, float32(s.TileSizeW), float32(s.TileSizeH),
			1, color.RGBA{R: 255, G: 255, B: 255, A: 120}, false)
	}
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
