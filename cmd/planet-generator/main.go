// planet-generator is a stand-alone tool for experimenting with location
// generation. It edits a copy of a map recipe (terrain, biomes, features, scenario
// rules), generates the location with the game's own code, and shows the result
// with no fog of war. The simulation can then be run to watch fauna and other
// entities, and dummy colonists can be dropped in to test AI behaviour.
//
// The info panel also shows the site's generated history for the map and seed
// (docs/brainstorm/lore_plan.md), with event origins marked in magenta; the
// "Scenario from history" toggle (or -history) plays the scenario that history
// ends in, as charting will.
//
// Run from the project root (so data/ is accessible):
//
//	go run ./cmd/planet-generator
//
// Controls
//
//	WASD          pan camera
//	Q / E         move camera down / up a z-level
//	Scroll wheel  zoom (over a panel: scroll it)
//	Left click    select entity, or place with the Place Colonist tool
//	Right click   cancel the Place Colonist tool
//	Space         run / pause the simulation
//	. (period)    step one tick
//	1 / 2 / 3     simulation speed
//	C             Place Colonist tool
//	F             follow the selected entity
//	G             generate with the current options
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"slices"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/minimap"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/resource"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

const (
	screenW    = 1440
	screenH    = 900
	leftW      = 300
	rightW     = 270
	toolbarH   = 34
	statusH    = 22
	overviewSz = rightW - 2*10
	// statsEvery is how often the live entity counts refresh, in frames.
	statsEvery = 30
)

type tool int

const (
	toolSelect tool = iota
	toolPlace
)

// app is the ebiten.Game for the tool.
type app struct {
	opts *options
	ui   ui

	lab      *game.PlanetLab
	overview *minimap.Minimap
	stats    worldStats
	genTime  time.Duration
	genErr   string

	// genPending counts down to a generation so a "Generating" frame shows first.
	genPending int
	dirty      bool
	autoGen    bool

	running    bool
	tool       tool
	selected   *ecs.Entity
	follow     bool
	fullbright bool
	showPaths  bool
	note       string // last tool feedback, shown on the status line

	wheelActive bool // see MainState.wheelActive
	frames      int
	hoverX      int
	hoverY      int
	hoverOK     bool
}

func main() {
	mapID := flag.String("map", "", "map definition to start with (default: first by name)")
	scenarioID := flag.String("scenario", "", "scenario to start with (default: one for the map)")
	fromHistory := flag.Bool("history", false, "take the scenario from the site's generated history")
	seed := flag.String("seed", "", "generation seed (default: random)")
	pngDir := flag.String("png", "", "headless: build the location once, write one PNG per level (deck) into this directory and exit")
	flag.Parse()

	cfg := config.Global()
	cfg.ScreenWidth, cfg.ScreenHeight = screenW, screenH
	cfg.WorldWidth, cfg.WorldHeight = screenW, screenH

	if err := world.LoadTileDefinitionsDir("data/tiledefinitions"); err != nil {
		log.Fatalf("load tile definitions: %v", err)
	}
	if err := resource.LoadAssetsFromJSON("data/assets.json"); err != nil {
		log.Fatalf("load assets: %v", err)
	}
	if err := factory.FactoryLoadDir(cfg.BlueprintPath); err != nil {
		log.Fatalf("load blueprints: %v", err)
	}
	if err := game.LoadPlanetLabData(); err != nil {
		log.Fatalf("load generation data: %v", err)
	}
	opts, err := newOptions()
	if err != nil {
		log.Fatal(err)
	}
	if *mapID != "" && !opts.selectMapByID(*mapID) {
		log.Fatalf("unknown map %q (have %v)", *mapID, opts.mapNames)
	}
	if *scenarioID != "" && !opts.selectScenarioByID(*scenarioID) {
		log.Fatalf("unknown scenario %q", *scenarioID)
	}
	opts.fromHistory = *fromHistory && *scenarioID == ""
	if *seed != "" {
		opts.seedText = *seed
		opts.rerollSize()
	}

	if *pngDir != "" {
		opts.fromHistory = *fromHistory && *scenarioID == ""
		if err := writeLevelPNGs(opts, *pngDir); err != nil {
			log.Fatal(err)
		}
		return
	}

	a := &app{opts: opts, fullbright: true, autoGen: true, genPending: 2}
	cfg.DebugDisableLighting = a.fullbright

	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Planet Generator")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(a); err != nil {
		log.Fatal(err)
	}
}

func (a *app) Layout(_, _ int) (int, int) { return screenW, screenH }

func (a *app) Update() error {
	a.frames++
	if a.genPending > 0 {
		a.genPending--
		if a.genPending == 0 {
			a.generate()
		}
		return nil
	}

	a.ui.begin()
	a.buildOptionsPanel()
	if a.lab != nil {
		a.buildToolbar()
		a.buildInfoPanel()
	}
	if a.autoGen && a.dirty && !a.ui.down && a.ui.focus == "" {
		a.requestGenerate()
	}
	if a.lab == nil {
		game.DrainEvents()
		return nil
	}

	a.handleWorldInput()
	if a.running {
		a.lab.Step()
	}
	game.DrainEvents()
	a.dropDeadSelection()
	if a.follow && a.selected != nil {
		if pc := position(a.selected); pc != nil {
			a.centerOn(pc.GetX(), pc.GetY(), pc.GetZ())
		}
	}
	if a.frames%statsEvery == 0 {
		a.stats.refreshEntities(a.lab.Level())
		if a.running {
			a.overview.GenerateImageAtZ(a.lab.Camera().Z)
		}
	}
	return nil
}

func (a *app) requestGenerate() {
	a.dirty = false
	if a.genPending == 0 {
		a.genPending = 2
	}
}

// generate replaces the current location with one built from the options. The
// camera stays put when regenerating the same-shaped map, so tweaks to one spot
// can be compared; otherwise it frames the new spawn.
func (a *app) generate() {
	a.opts.rollHistory()
	spec := a.opts.spec()
	var prevCam struct {
		ok              bool
		x, y, z, tw, th int
	}
	if a.lab != nil {
		c := a.lab.Camera()
		prev := a.lab.Level()
		prevCam.ok = prev.GetWidth() == spec.Width && prev.GetHeight() == spec.Height && prev.GetDepth() == spec.Depth
		prevCam.x, prevCam.y, prevCam.z, prevCam.tw, prevCam.th = c.X, c.Y, c.Z, c.TileW, c.TileH
		a.lab.Close()
		a.lab = nil
	}
	a.selected, a.follow, a.running, a.tool = nil, false, false, toolSelect

	start := time.Now()
	lab, err := game.NewPlanetLab(spec)
	a.genTime = time.Since(start)
	a.genErr = ""
	if err != nil {
		a.genErr = err.Error()
		return
	}
	a.lab = lab
	lvl := lab.Level()
	a.overview = minimap.NewMinimap(lvl, overviewSz, overviewSz)
	a.stats.compute(lvl)
	c := lab.Camera()
	if prevCam.ok {
		c.X, c.Y, c.Z, c.TileW, c.TileH = prevCam.x, prevCam.y, prevCam.z, prevCam.tw, prevCam.th
	} else {
		// Generation framed the spawn for the game's HUD; reframe it for these panels.
		st := c.SidebarTiles()
		a.centerOn(c.X+st+(c.ViewW()-st)/2, c.Y+c.ViewH()/2, c.Z)
	}
	a.note = fmt.Sprintf("Generated in %dms", a.genTime.Milliseconds())
}

// centerOn frames a tile in the world area between the panels.
func (a *app) centerOn(wx, wy, wz int) {
	c := a.lab.Camera()
	midPx := leftW + (screenW-leftW-rightW)/2
	c.X = wx - midPx/c.TileW
	c.Y = wy - (toolbarH+screenH-statusH)/2/c.TileH
	c.Z = wz
}

// overPanel reports whether a screen point is covered by UI rather than the world.
func overPanel(x, y int) bool {
	return x < leftW || x >= screenW-rightW || y < toolbarH || y >= screenH-statusH
}

func (a *app) handleWorldInput() {
	c := a.lab.Camera()
	mx, my := a.ui.mx, a.ui.my
	a.hoverOK = !overPanel(mx, my)
	if a.hoverOK {
		a.hoverX, a.hoverY = c.ScreenToWorld(mx, my)
	}

	// Same gesture handling as MainState.handleMouseWheel: one zoom per scroll.
	_, wy := ebiten.Wheel()
	if wy > -0.1 && wy < 0.1 {
		a.wheelActive = false
	} else if a.hoverOK && !a.wheelActive {
		a.wheelActive = true
		c.ZoomAt(mx, my, wy > 0)
	}

	if a.hoverOK && a.ui.pressed {
		a.clickWorld()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		a.tool = toolSelect
	}

	if a.ui.focus != "" {
		return // typing in a field; keys aren't commands
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		c.Y--
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		c.Y++
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		c.X--
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		c.X++
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyS) ||
		ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyD) {
		a.follow = false
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyQ) && c.Z > 0:
		c.Z--
	case inpututil.IsKeyJustPressed(ebiten.KeyE) && c.Z < a.lab.Level().GetDepth()-1:
		c.Z++
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		a.running = !a.running
	case inpututil.IsKeyJustPressed(ebiten.KeyPeriod):
		a.running = false
		a.lab.Step()
	case inpututil.IsKeyJustPressed(ebiten.Key1):
		a.lab.SetSpeed(1)
	case inpututil.IsKeyJustPressed(ebiten.Key2):
		a.lab.SetSpeed(2)
	case inpututil.IsKeyJustPressed(ebiten.Key3):
		a.lab.SetSpeed(4)
	case inpututil.IsKeyJustPressed(ebiten.KeyC):
		a.toggleTool(toolPlace)
	case inpututil.IsKeyJustPressed(ebiten.KeyF):
		a.follow = !a.follow && a.selected != nil
	case inpututil.IsKeyJustPressed(ebiten.KeyG):
		a.requestGenerate()
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		a.tool = toolSelect
	}
}

func (a *app) toggleTool(t tool) {
	if a.tool == t {
		a.tool = toolSelect
	} else {
		a.tool = t
	}
}

func (a *app) clickWorld() {
	z := a.lab.Camera().Z
	switch a.tool {
	case toolPlace:
		e, err := a.lab.PlaceColonist(a.hoverX, a.hoverY, z)
		if err != nil {
			a.note = err.Error()
			return
		}
		a.selected = e
		a.note = fmt.Sprintf("Placed colonist at %d,%d,%d", a.hoverX, a.hoverY, z)
	default:
		a.selected = a.lab.Level().GetEntityAt(a.hoverX, a.hoverY, z)
		a.follow = a.follow && a.selected != nil
	}
}

// dropDeadSelection clears a selection whose entity has left the level.
func (a *app) dropDeadSelection() {
	if a.selected == nil {
		return
	}
	if a.selected.HasComponent(rlcomponents.Dead) || !inLevel(a.lab.Level(), a.selected) {
		a.note = "Selected entity is gone"
		a.selected, a.follow = nil, false
	}
}

func inLevel(lvl *world.Level, e *ecs.Entity) bool {
	return slices.Contains(lvl.Entities, e) || slices.Contains(lvl.StaticEntities, e)
}

func (a *app) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	if a.lab != nil {
		a.lab.Draw(screen)
		a.drawWorldOverlays(screen)
		a.drawOriginsWorld(screen)
	}
	a.ui.render(screen)
	a.drawStatusLine(screen)
	if a.genPending > 0 {
		mlge_text.Draw(screen, "Generating...", 24, leftW+40, toolbarH+40, colWarn)
	}
}

func (a *app) drawWorldOverlays(screen *ebiten.Image) {
	c := a.lab.Camera()
	tw, th := float32(c.TileW), float32(c.TileH)
	if a.selected != nil {
		if pc := position(a.selected); pc != nil && pc.GetZ() == c.Z {
			sx, sy := c.WorldToScreenF(pc.GetX(), pc.GetY())
			vector.StrokeRect(screen, sx, sy, tw, th, 2, color.RGBA{255, 220, 80, 255}, false)
		}
		a.drawPath(screen)
	}
	if a.hoverOK {
		col := color.RGBA{255, 255, 255, 90}
		if a.tool == toolPlace {
			col = color.RGBA{80, 230, 120, 200}
		}
		sx, sy := c.WorldToScreenF(a.hoverX, a.hoverY)
		vector.StrokeRect(screen, sx, sy, tw, th, 1, col, false)
	}
}

// drawPath traces the selected entity's cached path, which is what AI debugging
// usually comes down to.
func (a *app) drawPath(screen *ebiten.Image) {
	if !a.showPaths || !a.selected.HasComponent(rlcomponents.AIMemory) {
		return
	}
	c := a.lab.Camera()
	lvl := a.lab.Level()
	mem := a.selected.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	for _, idx := range mem.CurrentSteps {
		t := lvl.Level.GetTilePtrIndex(idx)
		if t == nil {
			continue
		}
		x, y, z := t.Coords()
		if z != c.Z {
			continue
		}
		sx, sy := c.WorldToScreenF(x, y)
		vector.DrawFilledRect(screen, sx, sy, float32(c.TileW), float32(c.TileH), color.RGBA{0, 110, 160, 90}, false)
	}
}

func (a *app) drawStatusLine(screen *ebiten.Image) {
	y := screenH - statusH
	vector.DrawFilledRect(screen, leftW, float32(y), screenW-leftW-rightW, statusH, colPanel, false)
	txt := a.note
	if a.genErr != "" {
		txt = "Generation failed: " + a.genErr
	} else if a.lab != nil && a.hoverOK {
		txt = describeTile(a.lab.Level(), a.hoverX, a.hoverY, a.lab.Camera().Z)
	}
	mlge_text.DrawClipped(screen, txt, textSize, leftW+8, y+5, screenW-leftW-rightW-16, colText)
}

func position(e *ecs.Entity) *rlcomponents.PositionComponent {
	if e == nil || !e.HasComponent(rlcomponents.Position) {
		return nil
	}
	return e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
}

// describeTile is the hover readout for the status line.
func describeTile(lvl *world.Level, x, y, z int) string {
	t := lvl.GetTilePtr(x, y, z)
	if t == nil {
		return fmt.Sprintf("%d,%d,%d out of bounds", x, y, z)
	}
	s := fmt.Sprintf("%d,%d,%d  floor:%s  middle:%s", x, y, z, slotName(t.Floor), slotName(t.Middle))
	if b := lvl.GetBiome(x, y); b != "" {
		s += "  biome:" + b
	}
	s += fmt.Sprintf("  light:%d", t.LightLevel)
	if t.Radiation > 0 {
		s += fmt.Sprintf("  rad:%d", t.Radiation)
	}
	return s
}

// colonyTaskName names a worker's current activity for the inspector.
func colonyTaskName(wc *components.WorkerComponent) string {
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		return fmt.Sprintf("%v at %d,%d,%d", wc.CurrentTask.Action, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
	}
	return "none"
}

// countsSorted orders a blueprint tally by count, then name.
func countsSorted(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}
