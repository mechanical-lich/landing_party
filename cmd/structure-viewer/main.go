// structure-viewer is a stand-alone tool for previewing structure scripts
// (data/scripts/structures/*.basic). It builds a basic flat world, lets the
// user pan a camera around with WASD/QE, and stamps a chosen structure at the
// clicked tile so the artist/designer can see the result.
//
// Run from the project root (so data/ is accessible):
//
//   go run ./cmd/structure-viewer
//
// Controls
//
//   WASD          pan camera
//   Q / E         move camera up / down a z-level
//   Mouse         hover shows the structure footprint outline
//   Left click    stamp the structure at the hovered tile
//   F / Shift+F   cycle structure script
//   [ ] / - =     adjust footprint width / height
//   R             reset the world
//
// View-only: the spawned entities are rendered but no game logic runs.
package main

import (
	"flag"
	"fmt"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/resource"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

const (
	screenW    = 1280
	screenH    = 800
	toolbarH   = 48
	tileSize   = 24
	spriteSize = 24
)

type Viewer struct {
	level *world.Level

	// world dims & ground z (reused on reset)
	worldW, worldH, worldD int
	groundZ                int
	floorTile              string
	subsurfaceTile         string

	// camera
	camX, camY, camZ int

	// structure selection / footprint
	scripts    []string
	scriptIdx  int
	stampW     int
	stampH     int

	// hover state (tile coords, may be out of bounds)
	hoverTX, hoverTY int
	hoverValid       bool

	// log line shown in toolbar (last action / error)
	status string
}

func main() {
	defWidth := flag.Int("w", 64, "world width")
	defHeight := flag.Int("h", 64, "world height")
	defDepth := flag.Int("d", 9, "world depth (z-levels)")
	defGround := flag.Int("ground", 5, "ground z-level")
	defFloor := flag.String("floor", "regolith", "floor tile name used to stamp the ground")
	defSubsurface := flag.String("subsurface", "dirt", "tile filled into z-levels below the ground")
	defStampW := flag.Int("sw", 8, "initial structure footprint width")
	defStampH := flag.Int("sh", 8, "initial structure footprint height")
	flag.Parse()

	// Lighting & lookdown make a flat empty world look black — disable them
	// for the viewer so every tile renders at full brightness.
	cfg := config.Global()
	cfg.DebugDisableLighting = true

	if err := world.LoadTileDefinitionsDir("data/tiledefinitions"); err != nil {
		log.Fatalf("load tile definitions: %v", err)
	}
	if err := resource.LoadAssetsFromJSON("data/assets.json"); err != nil {
		log.Fatalf("load assets: %v", err)
	}
	if err := factory.FactoryLoadDir(cfg.BlueprintPath); err != nil {
		log.Fatalf("load blueprints: %v", err)
	}

	scripts, err := listScripts("data/scripts/structures")
	if err != nil {
		log.Fatalf("list structure scripts: %v", err)
	}
	if len(scripts) == 0 {
		log.Fatalf("no structure scripts found in data/scripts/structures")
	}

	v := &Viewer{
		worldW:    *defWidth,
		worldH:    *defHeight,
		worldD:    *defDepth,
		groundZ:   *defGround,
		floorTile:      *defFloor,
		subsurfaceTile: *defSubsurface,
		stampW:    *defStampW,
		stampH:    *defStampH,
		scripts:   scripts,
	}
	v.resetWorld()
	v.camZ = v.groundZ
	v.camX = v.worldW/2 - viewTilesW()/2
	v.camY = v.worldH/2 - viewTilesH()/2

	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Structure Viewer")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(v); err != nil {
		log.Fatal(err)
	}
}

func listScripts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".basic") {
			continue
		}
		names = append(names, strings.TrimSuffix(n, ".basic"))
	}
	sort.Strings(names)
	return names, nil
}

func viewTilesW() int { return screenW / tileSize }
func viewTilesH() int { return (screenH - toolbarH) / tileSize }

// resetWorld builds a fresh flat level: floor tile stamped across the ground
// z-plane, surface map seeded to groundZ for every column, and all cells
// marked seen/visible so the renderer paints them without an FOV pass.
func (v *Viewer) resetWorld() {
	lvl := world.NewLevel(v.worldW, v.worldH, v.worldD)
	lvl.AllocTerrain()
	lvl.SurfaceZ = v.groundZ
	lvl.AtmosphereZ = v.groundZ + 1
	lvl.SpaceZ = v.worldD
	lvl.LightMode = "fixed"
	lvl.FixedAmbient = 100

	for y := 0; y < v.worldH; y++ {
		for x := 0; x < v.worldW; x++ {
			lvl.SetSurfaceZ(x, y, v.groundZ)
			lvl.SetFloor(x, y, v.groundZ, v.floorTile, world.RandomTileVariant(v.floorTile))
			// Fill every cell below the ground with the subsurface tile so
			// looking down (or stepping a z below) shows solid ground rather
			// than empty void.
			for z := 0; z < v.groundZ; z++ {
				lvl.SetFloor(x, y, z, v.subsurfaceTile, world.RandomTileVariant(v.subsurfaceTile))
				lvl.SetMiddle(x, y, z, v.subsurfaceTile, world.RandomTileVariant(v.subsurfaceTile))
			}
			// Above-ground cells are air — invisible middles that the
			// renderer treats as transparent, so the z-lookdown shows the
			// surface (and any deeper tiles) shaded by depth.
			for z := v.groundZ + 1; z < v.worldD; z++ {
				lvl.SetMiddle(x, y, z, "air", 0)
			}
			for z := 0; z < v.worldD; z++ {
				lvl.SetSeen(x, y, z, true)
				lvl.SetVisible(x, y, z)
			}
		}
	}

	v.level = lvl
	v.status = fmt.Sprintf("world %dx%dx%d ground=%d", v.worldW, v.worldH, v.worldD, v.groundZ)
}

// ── Ebiten game interface ────────────────────────────────────────────────────

func (v *Viewer) Layout(_, _ int) (int, int) { return screenW, screenH }

func (v *Viewer) Update() error {
	// Camera pan — held keys repeat each frame so the camera glides.
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		v.camX--
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		v.camX++
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		v.camY--
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		v.camY++
	}
	v.clampCam()

	// Z move (tap, not hold)
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		if v.camZ < v.worldD-1 {
			v.camZ++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		if v.camZ > 0 {
			v.camZ--
		}
	}

	// Structure selection
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			v.scriptIdx = (v.scriptIdx - 1 + len(v.scripts)) % len(v.scripts)
		} else {
			v.scriptIdx = (v.scriptIdx + 1) % len(v.scripts)
		}
	}

	// Footprint resize
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketLeft) {
		if v.stampW > 3 {
			v.stampW--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketRight) {
		v.stampW++
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		if v.stampH > 3 {
			v.stampH--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		v.stampH++
	}

	// Reset
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		game.ClearStructureScriptCache()
		v.resetWorld()
	}

	// Hover + click
	mx, my := ebiten.CursorPosition()
	if my >= toolbarH {
		v.hoverTX = v.camX + mx/tileSize
		v.hoverTY = v.camY + (my-toolbarH)/tileSize
		v.hoverValid = true
	} else {
		v.hoverValid = false
	}

	if v.hoverValid && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		v.stamp(v.hoverTX, v.hoverTY)
	}

	return nil
}

func (v *Viewer) clampCam() {
	if v.camX < 0 {
		v.camX = 0
	}
	if v.camY < 0 {
		v.camY = 0
	}
	maxX := v.worldW - viewTilesW()
	maxY := v.worldH - viewTilesH()
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if v.camX > maxX {
		v.camX = maxX
	}
	if v.camY > maxY {
		v.camY = maxY
	}
}

// stamp invokes the selected structure script with the current footprint at
// (tx, ty). The script's contract is generate(x, y, w, h) where (x,y) is the
// top-left corner, matching the runtime dispatcher.
func (v *Viewer) stamp(tx, ty int) {
	name := v.scripts[v.scriptIdx]
	// Clear cache so an edited script picks up its new source on next stamp.
	game.ClearStructureScriptCache()
	if err := game.RunStructureScript(v.level, name, tx, ty, v.stampW, v.stampH); err != nil {
		v.status = fmt.Sprintf("error: %v", err)
		log.Printf("stamp %s @ (%d,%d) %dx%d: %v", name, tx, ty, v.stampW, v.stampH, err)
		return
	}
	// Newly-spawned entities sit on their own cells — make sure those cells
	// are flagged seen/visible for the renderer.
	for y := ty; y < ty+v.stampH; y++ {
		for x := tx; x < tx+v.stampW; x++ {
			for z := 0; z < v.worldD; z++ {
				v.level.SetSeen(x, y, z, true)
				v.level.SetVisible(x, y, z)
			}
		}
	}
	v.status = fmt.Sprintf("stamped %s @ (%d,%d) %dx%d", name, tx, ty, v.stampW, v.stampH)
}

// ── Draw ─────────────────────────────────────────────────────────────────────

func (v *Viewer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 22, 255})

	// World is rendered into an offscreen image and blitted below the toolbar
	// so DrawLevel's tile-at-(0,0) origin doesn't paint over the chrome.
	worldImg := offscreen(screenW, screenH-toolbarH)
	worldImg.Fill(color.RGBA{0, 0, 0, 255})

	world.DrawLevel(v.level, worldImg, v.camX, v.camY, v.camZ,
		tileSize, tileSize, spriteSize, spriteSize,
		viewTilesW(), viewTilesH())

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, float64(toolbarH))
	screen.DrawImage(worldImg, op)

	v.drawFootprintOutline(screen)
	v.drawToolbar(screen)
}

func (v *Viewer) drawFootprintOutline(screen *ebiten.Image) {
	if !v.hoverValid {
		return
	}
	x := (v.hoverTX - v.camX) * tileSize
	y := (v.hoverTY-v.camY)*tileSize + toolbarH
	w := v.stampW * tileSize
	h := v.stampH * tileSize
	col := color.RGBA{255, 220, 80, 220}
	// Outline + faint fill.
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h),
		color.RGBA{255, 220, 80, 32}, false)
	drawOutline(screen, x, y, w, h, 2, col)
	// Origin marker at top-left so it's obvious which corner is (x,y).
	drawOutline(screen, x, y, tileSize, tileSize, 2, color.RGBA{120, 220, 255, 240})
}

func (v *Viewer) drawToolbar(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, screenW, toolbarH,
		color.RGBA{30, 32, 42, 255}, false)
	vector.DrawFilledRect(screen, 0, toolbarH-1, screenW, 1,
		color.RGBA{80, 90, 110, 255}, false)

	mlge_text.Draw(screen,
		fmt.Sprintf("Structure: %s  (F / Shift+F)   Size: %dx%d  ([ ] / - =)   Z: %d / %d  (Q/E)   ground=%d",
			v.scripts[v.scriptIdx], v.stampW, v.stampH, v.camZ, v.worldD-1, v.groundZ),
		12, 8, 6, color.RGBA{220, 230, 255, 255})

	hover := "—"
	if v.hoverValid {
		hover = fmt.Sprintf("(%d, %d)", v.hoverTX, v.hoverTY)
	}
	mlge_text.Draw(screen,
		fmt.Sprintf("WASD pan   Click to stamp   R reset   Hover: %s   %s", hover, v.status),
		10, 8, 26, color.RGBA{160, 180, 210, 255})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func drawOutline(dst *ebiten.Image, x, y, w, h, thick int, c color.RGBA) {
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(thick), c, false)
	vector.DrawFilledRect(dst, float32(x), float32(y+h-thick), float32(w), float32(thick), c, false)
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(thick), float32(h), c, false)
	vector.DrawFilledRect(dst, float32(x+w-thick), float32(y), float32(thick), float32(h), c, false)
}

var (
	cachedOffscreen *ebiten.Image
	cachedOffW      int
	cachedOffH      int
)

// offscreen returns a reusable *ebiten.Image of the given size. Recreated only
// when the requested dimensions change (e.g. window resize).
func offscreen(w, h int) *ebiten.Image {
	if cachedOffscreen == nil || cachedOffW != w || cachedOffH != h {
		cachedOffscreen = ebiten.NewImage(w, h)
		cachedOffW, cachedOffH = w, h
	}
	return cachedOffscreen
}

