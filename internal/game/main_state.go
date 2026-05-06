package game

import (
	"fmt"
	"log"
	"os"

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
	"github.com/mechanical-lich/mlge/state"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/construction"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/generation"
	"github.com/mechanical-lich/scifi_settlements/internal/gui"
	fspath "github.com/mechanical-lich/scifi_settlements/internal/path"
	"github.com/mechanical-lich/scifi_settlements/internal/scenario"
	"github.com/mechanical-lich/scifi_settlements/internal/settlement"
	"github.com/mechanical-lich/scifi_settlements/internal/systems"
	"github.com/mechanical-lich/scifi_settlements/internal/task_requests"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

type SettlementConfig struct {
	Name       string
	ScenarioID string
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
				for _, drop := range dropsC.Items {
					dropEntity, err := factory.Create(drop, pc.GetX(), pc.GetY(), pc.GetZ())
					if err == nil {
						level.AddEntity(dropEntity)
					}
				}
			}
		},
	}

	aiSystem := rlsystems.NewAISystem()
	aiSystem.GetPath = func(levelInterface rlworld.LevelInterface, from, to rlworld.TileInterface, reuse []int) []int {
		return fspath.GetPossiblePath(levelInterface.(*world.Level), from.(*world.Tile), to.(*world.Tile), reuse)
	}
	s.systemManager.AddSystem(aiSystem)
	s.systemManager.AddSystem(&systems.WorkerSystem{})
	s.systemManager.AddSystem(&rlsystems.DoorSystem{AppearanceType: components.Appearance})

	event.GetQueuedInstance().RegisterListener(s, input.MouseClickEventType)
	event.GetQueuedInstance().RegisterListener(s, input.MouseReleasedEventType)
	event.GetQueuedInstance().RegisterListener(s, input.KeyPressEventType)
	event.GetQueuedInstance().RegisterListener(s, input.MouseWheelEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.BuildOptionChangedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.CursorModeChangedEventType)
	event.GetQueuedInstance().RegisterListener(s, gui.MainMenuEventType)

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

func (s *MainState) newGame() {
	cfg := config.Global()
	mapW := cfg.WorldGenSizeW
	mapH := cfg.WorldGenSizeH
	mapZ := cfg.WorldGenSizeZ

	planetCfg := generation.DefaultPlanetConfig(mapZ)
	s.level = generation.NewPlanetLevel(mapW, mapH, mapZ, planetCfg)
	s.guiManager = gui.NewGUIManager()
	s.gm = &GameMaster{}
	s.gm.Init(s.level)

	startingZ := cfg.StartingZ
	x, y := s.gm.GetFreeSpaceAtZ(startingZ)
	if x != -1 {
		// Position camera so colonists (at x-2..x+2) appear centered in the
		// visible world area (to the right of the 200px sidebar).
		sidebarTiles := 200/s.TileSizeW + 1
		viewW := cfg.WorldWidth / s.TileSizeW
		viewH := cfg.WorldHeight / s.TileSizeH
		s.CameraX = x - sidebarTiles - (viewW-sidebarTiles)/2
		s.CameraY = y - viewH/2
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
}

func (s *MainState) Update() state.StateInterface {
	fspath.ResetFrameCounter()
	s.handleInput()
	s.guiManager.Update()
	s.updateHovered()

	fps := ebiten.ActualFPS()
	tps := ebiten.ActualTPS()
	ebiten.SetWindowTitle(fmt.Sprintf("%s — Z:%d FPS:%.0f TPS:%.0f", config.Global().Title, s.CameraZ, fps, tps))

	if !s.Paused {
		s.tick++
		s.gm.Update()
		s.systemManager.UpdateSystems(s.level)
		for _, entity := range s.level.Entities {
			if entity.HasComponent(rlcomponents.Inanimate) {
				continue
			}
			s.systemManager.UpdateSystemsForEntity(s.level, entity)
		}
		s.cleanUpSystem.Update(s.level)
	}

	if s.tick%30 == 0 {
		s.refreshHUD()
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

	s.drawTasks(s.worldImage)
	screen.DrawImage(s.worldImage, nil)
	s.guiManager.Draw(screen)
}

func (s *MainState) Done() bool { return s.done }

func (s *MainState) HandleEvent(e event.EventData) error {
	switch ev := e.(type) {
	case input.MouseClickEvent:
		s.handleMouseClick(ev)
	case input.MouseReleasedEvent:
		if ev.Button == ebiten.MouseButtonRight {
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
	}
	return nil
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
	if e.Y > 0.1 && s.TileSizeW < 96 {
		s.TileSizeW *= 2
		s.TileSizeH *= 2
	}
	if e.Y < -0.1 && s.TileSizeW > 16 {
		s.TileSizeW /= 2
		s.TileSizeH /= 2
	}
}

func (s *MainState) handleMouseClick(e input.MouseClickEvent) {
	if s.guiManager.GetMouseFocused() || s.guiManager.WithinModalBounds(ebiten.CursorPosition()) {
		return
	}

	cX, cY := ebiten.CursorPosition()
	tX := cX/s.TileSizeW + s.CameraX
	tY := cY/s.TileSizeH + s.CameraY

	if e.Button == ebiten.MouseButtonLeft {
		// Select entity
		for _, ent := range s.level.Entities {
			ent.RemoveComponent(components.Selected)
		}
		s.selectedEntity = nil
		ent := s.level.GetEntityAt(tX, tY, s.CameraZ)
		if ent != nil {
			ent.AddComponent(&components.SelectedComponent{})
			s.selectedEntity = ent
			event.GetQueuedInstance().SendEvent(gui.EntitySelectedEvent{Entity: ent})
		}
	}

	if e.Button == ebiten.MouseButtonRight && s.MainSettlement != nil {
		switch s.CursorMode {
		case gui.CursorModeDefault:
			ent := s.level.GetEntityAt(tX, tY, s.CameraZ)
			if ent != nil && ent.HasComponent(rlcomponents.Item) {
				s.MainSettlement.Tasks.AddTask(&task.Task{Action: task_requests.PickupAction, X: tX, Y: tY, Z: s.CameraZ, Escalated: true})
			}
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
	tile := s.level.GetTileAt(x, y, s.CameraZ)
	if tile == nil {
		return
	}
	tileName := world.TileDefinitions[tile.(*world.Tile).Type].Name
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
		return
	}

	tile := s.level.GetTileAt(tX, tY, s.CameraZ)
	if tile == nil {
		s.guiManager.ClearHover()
		s.hoverActive = false
		return
	}
	t := tile.(*world.Tile)
	def := world.TileDefinitions[t.Type]
	s.guiManager.SetHoveredTile(def.Name, def.Solid, def.Water, def.Air, def.Space)
}

func (s *MainState) refreshHUD() {
	if s.MainSettlement == nil {
		return
	}

	// Count resources across all storage lockers
	resources := map[string]int{"metal_ore": 0, "crystal": 0}
	var popEntries []gui.PopulationEntry
	for _, entity := range s.level.Entities {
		if entity.HasComponent(components.Storage) {
			st := entity.GetComponent(components.Storage).(*components.StorageComponent)
			for _, item := range st.Items {
				if item.Blueprint != "" {
					resources[item.Blueprint]++
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
			popEntries = append(popEntries, gui.PopulationEntry{Name: name, State: state, Task: taskName})
		}
	}

	for id, count := range resources {
		s.guiManager.UpdateResource(id, count)
	}
	s.guiManager.RefreshPopulationTab(popEntries)
}

func (s *MainState) drawTasks(screen *ebiten.Image) {
	if s.MainSettlement != nil {
		viewW := config.Global().WorldWidth / s.TileSizeW
		viewH := config.Global().WorldHeight / s.TileSizeH
		for _, t := range s.MainSettlement.Tasks.GetTasks() {
			if t.Completed || t.Z != s.CameraZ {
				continue
			}
			if t.X < s.CameraX || t.X > s.CameraX+viewW || t.Y < s.CameraY || t.Y > s.CameraY+viewH {
				continue
			}
			sx := float32((t.X - s.CameraX) * s.TileSizeW)
			sy := float32((t.Y - s.CameraY) * s.TileSizeH)
			vector.DrawFilledRect(screen, sx, sy, float32(s.TileSizeW), float32(s.TileSizeH),
				color.RGBA{R: 255, G: 200, B: 0, A: 40}, false)
			vector.StrokeRect(screen, sx, sy, float32(s.TileSizeW), float32(s.TileSizeH),
				1, color.RGBA{R: 255, G: 200, B: 0, A: 160}, false)
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
