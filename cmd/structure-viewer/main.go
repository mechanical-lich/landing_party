// structure-viewer is a stand-alone tool for previewing structure scripts
// (data/scripts/structures/*.basic). It builds a basic flat world, lets the
// user pan a camera around with WASD/QE, and stamps a chosen structure at the
// clicked tile so the artist/designer can see the result.
//
// Run from the project root (so data/ is accessible):
//
//	go run ./cmd/structure-viewer
//
// Controls
//
//	WASD          pan camera
//	Q / E         move camera up / down a z-level
//	Mouse         hover shows the structure footprint outline
//	Left click    stamp the structure at the hovered tile
//	F / Shift+F   cycle structure script
//	[ ] / - =     adjust footprint width / height
//	R             reset the world
//	Esc           close picker modal
//
// View-only: the spawned entities are rendered but no game logic runs.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/game"
	"github.com/mechanical-lich/landing_party/internal/view"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/resource"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

const (
	screenW    = 1280
	screenH    = 800
	toolbarH   = 32
	sidebarW   = 260
	tileSize   = 24
	spriteSize = 24
)

// rect is a small ints-only rectangle used for buttons and hit-tests.
type rect struct{ x, y, w, h int }

func (r rect) contains(mx, my int) bool {
	return mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h
}

// paramRow is a single user-defined parameter the viewer will seed into the
// script's top param frame before stamping. Stored as strings; values are
// coerced to float64 just-in-time when they parse as numbers so scripts using
// arithmetic on get_param("...") see numbers, not strings.
type paramRow struct {
	Key   string
	Value string
}

// focusKind identifies which text input is receiving keystrokes. The viewer
// has a handful of edit targets — param key/value cells (indexed by row) and
// the width/height number fields — so we tag the focused control with this
// enum plus a row index that's only meaningful for paramKey / paramVal.
type focusKind int

const (
	focusNone focusKind = iota
	focusParamKey
	focusParamVal
	focusWidth
	focusHeight
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
	scripts   []string
	scriptIdx int
	stampW    int
	stampH    int

	// hover state (tile coords, may be out of bounds)
	hoverTX, hoverTY int
	hoverValid       bool

	// modal state
	pickerOpen   bool
	pickerScroll int

	// params seeded onto the script's top frame before each stamp.
	params    []paramRow
	focusKind focusKind // which kind of text input is active (focusNone if none)
	focusRow  int       // param row index when focusKind is param*; ignored otherwise

	// Edit buffers for the width/height number fields. The canonical values
	// live in stampW/stampH; these strings are the visible/editable form and
	// are reparsed on commit.
	widthText  string
	heightText string

	// log line shown in toolbar (last action / error)
	status string

	// recomputed each frame in drawSidebar — Update() reuses them for hit-tests.
	btnWidthMinus, btnWidthPlus      rect
	btnHeightMinus, btnHeightPlus    rect
	btnPicker, btnReset, btnAddParam rect
	boxWidth, boxHeight              rect
	// Per-row hitboxes for the params list. Length == len(params).
	paramKeyBoxes []rect
	paramValBoxes []rect
	paramDelBtns  []rect
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

	// Lighting is fed by the LightSystem in-game; here we just want full
	// brightness so the tiles read as painted in the editor.
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
		worldW:         *defWidth,
		worldH:         *defHeight,
		worldD:         *defDepth,
		groundZ:        *defGround,
		floorTile:      *defFloor,
		subsurfaceTile: *defSubsurface,
		stampW:         *defStampW,
		stampH:         *defStampH,
		scripts:        scripts,
	}
	v.resetWorld()
	v.camZ = v.groundZ
	v.camX = v.worldW/2 - worldViewTilesW()/2
	v.camY = v.worldH/2 - worldViewTilesH()/2

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

// worldView returns the on-screen rect occupied by the rendered world (i.e. the
// area to the left of the sidebar, below the toolbar).
func worldView() rect {
	return rect{x: 0, y: toolbarH, w: screenW - sidebarW, h: screenH - toolbarH}
}
func worldViewTilesW() int { return (screenW - sidebarW) / tileSize }
func worldViewTilesH() int { return (screenH - toolbarH) / tileSize }

// resetWorld builds a fresh flat level: floor tile across the ground plane,
// subsurface fill below, air above. Cells are marked seen/visible so the
// renderer paints them without an FOV pass.
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
			for z := 0; z < v.groundZ; z++ {
				lvl.SetFloor(x, y, z, v.subsurfaceTile, world.RandomTileVariant(v.subsurfaceTile))
				lvl.SetMiddle(x, y, z, v.subsurfaceTile, world.RandomTileVariant(v.subsurfaceTile))
			}
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
	mx, my := ebiten.CursorPosition()

	if v.pickerOpen {
		v.updatePicker(mx, my)
		return nil
	}

	// A focused text input owns the keyboard — typing into it must not also
	// drive WASD or fire shortcuts. Mouse clicks and the few shortcuts that
	// don't conflict with typing (Q/E z-move via just-pressed alpha keys) are
	// still gated below so they don't fight the input.
	if v.focusKind != focusNone {
		v.updateFocusedTextInput()
		v.handleSidebarClicks(mx, my)
		v.drawHover(mx, my)
		return nil
	}

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

	// Footprint resize via keyboard
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketLeft) {
		v.adjustWidth(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketRight) {
		v.adjustWidth(+1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		v.adjustHeight(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		v.adjustHeight(+1)
	}

	// Reset
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		game.ClearStructureScriptCache()
		v.resetWorld()
	}

	if v.handleSidebarClicks(mx, my) {
		return nil
	}
	v.drawHover(mx, my)
	if v.hoverValid && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		v.stamp(v.hoverTX, v.hoverTY)
	}

	return nil
}

// handleSidebarClicks routes a left-click to whichever sidebar control the
// cursor is over. Returns true when a click was consumed so callers can skip
// world interaction this frame. Also handles defocus when clicking outside a
// text input.
func (v *Viewer) handleSidebarClicks(mx, my int) bool {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return false
	}
	// Width/height number text boxes — clicking focuses, populating the
	// edit buffer from the current canonical int value.
	if v.boxWidth.contains(mx, my) {
		v.commitFocusedNumber()
		v.focusKind = focusWidth
		v.widthText = strconv.Itoa(v.stampW)
		return true
	}
	if v.boxHeight.contains(mx, my) {
		v.commitFocusedNumber()
		v.focusKind = focusHeight
		v.heightText = strconv.Itoa(v.stampH)
		return true
	}
	// Param text-input focus: clicking a key/value field focuses it; clicking
	// the row's delete button removes it.
	for i := range v.params {
		if i < len(v.paramKeyBoxes) && v.paramKeyBoxes[i].contains(mx, my) {
			v.commitFocusedNumber()
			v.focusKind = focusParamKey
			v.focusRow = i
			return true
		}
		if i < len(v.paramValBoxes) && v.paramValBoxes[i].contains(mx, my) {
			v.commitFocusedNumber()
			v.focusKind = focusParamVal
			v.focusRow = i
			return true
		}
		if i < len(v.paramDelBtns) && v.paramDelBtns[i].contains(mx, my) {
			v.params = append(v.params[:i], v.params[i+1:]...)
			v.focusKind = focusNone
			return true
		}
	}
	if v.btnAddParam.contains(mx, my) {
		v.commitFocusedNumber()
		v.params = append(v.params, paramRow{})
		v.focusKind = focusParamKey
		v.focusRow = len(v.params) - 1
		return true
	}
	switch {
	case v.btnWidthMinus.contains(mx, my):
		v.adjustWidth(-1)
	case v.btnWidthPlus.contains(mx, my):
		v.adjustWidth(+1)
	case v.btnHeightMinus.contains(mx, my):
		v.adjustHeight(-1)
	case v.btnHeightPlus.contains(mx, my):
		v.adjustHeight(+1)
	case v.btnPicker.contains(mx, my):
		v.commitFocusedNumber()
		v.pickerOpen = true
	case v.btnReset.contains(mx, my):
		game.ClearStructureScriptCache()
		v.resetWorld()
	default:
		// Click landed outside any control — commit any pending number edit
		// and drop input focus, but don't claim the click so world-clicks
		// below still fire.
		v.commitFocusedNumber()
		v.focusKind = focusNone
		return false
	}
	v.commitFocusedNumber()
	v.focusKind = focusNone
	return true
}

// drawHover updates v.hoverTX/Y/hoverValid for the current cursor position.
// Name reflects its job (driving the footprint outline); no drawing happens
// here — Draw uses the cached fields.
func (v *Viewer) drawHover(mx, my int) {
	wv := worldView()
	if wv.contains(mx, my) {
		v.hoverTX = v.camX + (mx-wv.x)/tileSize
		v.hoverTY = v.camY + (my-wv.y)/tileSize
		v.hoverValid = true
	} else {
		v.hoverValid = false
	}
}

// focusedTarget returns a pointer to the string the currently-focused input
// edits, and whether that input should accept only digits (width/height).
// Returns nil if nothing valid is focused.
func (v *Viewer) focusedTarget() (target *string, numeric bool) {
	switch v.focusKind {
	case focusParamKey:
		if v.focusRow >= 0 && v.focusRow < len(v.params) {
			return &v.params[v.focusRow].Key, false
		}
	case focusParamVal:
		if v.focusRow >= 0 && v.focusRow < len(v.params) {
			return &v.params[v.focusRow].Value, false
		}
	case focusWidth:
		return &v.widthText, true
	case focusHeight:
		return &v.heightText, true
	}
	return nil, false
}

// commitFocusedNumber pushes the width/height edit buffer (if any) back into
// the canonical stampW/stampH ints, clamping to the minimum. Safe to call when
// nothing numeric is focused — it just no-ops.
func (v *Viewer) commitFocusedNumber() {
	switch v.focusKind {
	case focusWidth:
		if n, err := strconv.Atoi(strings.TrimSpace(v.widthText)); err == nil && n >= 3 {
			v.stampW = n
		}
		v.widthText = strconv.Itoa(v.stampW)
	case focusHeight:
		if n, err := strconv.Atoi(strings.TrimSpace(v.heightText)); err == nil && n >= 3 {
			v.stampH = n
		}
		v.heightText = strconv.Itoa(v.stampH)
	}
}

// updateFocusedTextInput appends typed runes to the focused field and handles
// editing keys (backspace, enter/tab to advance/commit, escape to cancel).
func (v *Viewer) updateFocusedTextInput() {
	target, numeric := v.focusedTarget()
	if target == nil {
		v.focusKind = focusNone
		return
	}

	chars := ebiten.AppendInputChars(nil)
	for _, r := range chars {
		if r < 0x20 {
			continue
		}
		if numeric && (r < '0' || r > '9') {
			continue
		}
		*target += string(r)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(*target) > 0 {
		s := *target
		for i := len(s) - 1; i >= 0; i-- {
			if s[i]&0xc0 != 0x80 { // start byte of a rune
				*target = s[:i]
				break
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		// Cancel — drop the edit buffer for numbers (restore from canonical).
		if v.focusKind == focusWidth {
			v.widthText = strconv.Itoa(v.stampW)
		}
		if v.focusKind == focusHeight {
			v.heightText = strconv.Itoa(v.stampH)
		}
		v.focusKind = focusNone
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		// Number fields just commit on Tab/Enter. Param rows cycle:
		// key → value → (next row's key) → … → defocus.
		if v.focusKind == focusWidth || v.focusKind == focusHeight {
			v.commitFocusedNumber()
			v.focusKind = focusNone
			return
		}
		if v.focusKind == focusParamKey {
			v.focusKind = focusParamVal
		} else if v.focusRow+1 < len(v.params) {
			v.focusRow++
			v.focusKind = focusParamKey
		} else {
			v.focusKind = focusNone
		}
	}
}

// buildParams converts the UI param rows into the map seeded onto the
// script's top frame. Empty keys are skipped. Values that parse as numbers
// are stored as float64 so the script's arithmetic sees a number (otherwise
// `+ 0` would coerce "3" → "30"); everything else stays a string.
func (v *Viewer) buildParams() map[string]any {
	if len(v.params) == 0 {
		return nil
	}
	out := make(map[string]any, len(v.params))
	for _, p := range v.params {
		k := strings.TrimSpace(p.Key)
		if k == "" {
			continue
		}
		val := strings.TrimSpace(p.Value)
		if n, err := strconv.ParseFloat(val, 64); err == nil && val != "" {
			out[k] = n
		} else {
			out[k] = val
		}
	}
	return out
}

func (v *Viewer) adjustWidth(delta int) {
	v.stampW += delta
	if v.stampW < 3 {
		v.stampW = 3
	}
}
func (v *Viewer) adjustHeight(delta int) {
	v.stampH += delta
	if v.stampH < 3 {
		v.stampH = 3
	}
}

func (v *Viewer) updatePicker(mx, my int) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		v.pickerOpen = false
		return
	}
	mr := v.pickerRect()
	listX, listY, _, rowH := v.pickerListLayout(mr)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if !mr.contains(mx, my) {
			v.pickerOpen = false
			return
		}
		if mx >= listX && mx < listX+mr.w-32 {
			row := (my - listY + v.pickerScroll) / rowH
			if row >= 0 && row < len(v.scripts) {
				v.scriptIdx = row
				v.pickerOpen = false
				return
			}
		}
	}
	// Wheel scrolls the list.
	if _, dy := ebiten.Wheel(); dy != 0 {
		v.pickerScroll -= int(dy) * 20
		if v.pickerScroll < 0 {
			v.pickerScroll = 0
		}
		max := len(v.scripts)*rowH - (mr.h - 60)
		if max < 0 {
			max = 0
		}
		if v.pickerScroll > max {
			v.pickerScroll = max
		}
	}
}

func (v *Viewer) clampCam() {
	if v.camX < 0 {
		v.camX = 0
	}
	if v.camY < 0 {
		v.camY = 0
	}
	maxX := v.worldW - worldViewTilesW()
	maxY := v.worldH - worldViewTilesH()
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
// (tx, ty). The script picks its working z-plane itself (typically via
// get_surface_z) — same contract as the runtime dispatcher in world_manager.
func (v *Viewer) stamp(tx, ty int) {
	name := v.scripts[v.scriptIdx]
	// Clear cache so an edited script picks up its new source on next stamp.
	game.ClearStructureScriptCache()
	// Seed from the stamp position so previews are reproducible yet vary as you
	// move the cursor around the canvas.
	seed := int64(tx)*2654435761 + int64(ty)*40503
	if err := game.RunStructureScriptWithParams(v.level, name, tx, ty, v.stampW, v.stampH, v.buildParams(), seed); err != nil {
		v.status = fmt.Sprintf("error: %v", err)
		log.Printf("stamp %s @ (%d,%d) %dx%d: %v", name, tx, ty, v.stampW, v.stampH, err)
		return
	}
	// Newly-spawned tiles/entities sit on their own cells — make sure those
	// cells are flagged seen/visible for the renderer.
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

	// World rendered into an offscreen image and blitted into the world view
	// rect so DrawLevel's (0,0) origin doesn't paint over the toolbar/sidebar.
	wv := worldView()
	worldImg := offscreen(wv.w, wv.h)
	worldImg.Fill(color.RGBA{0, 0, 0, 255})
	world.DrawLevel(v.level, worldImg, view.Camera{
		X: v.camX, Y: v.camY, Z: v.camZ,
		TileW: tileSize, TileH: tileSize,
		SpriteW: spriteSize, SpriteH: spriteSize,
		CanvasW: wv.w, CanvasH: wv.h,
	})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(wv.x), float64(wv.y))
	screen.DrawImage(worldImg, op)

	v.drawFootprintOutline(screen)
	v.drawToolbar(screen)
	v.drawSidebar(screen)

	if v.pickerOpen {
		v.drawPicker(screen)
	}
}

func (v *Viewer) drawFootprintOutline(screen *ebiten.Image) {
	if !v.hoverValid {
		return
	}
	wv := worldView()
	x := wv.x + (v.hoverTX-v.camX)*tileSize
	y := wv.y + (v.hoverTY-v.camY)*tileSize
	w := v.stampW * tileSize
	h := v.stampH * tileSize
	// Clip the outline to the world view so it doesn't bleed onto the sidebar.
	maxX := wv.x + wv.w
	if x+w > maxX {
		w = maxX - x
	}
	col := color.RGBA{255, 220, 80, 220}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h),
		color.RGBA{255, 220, 80, 32}, false)
	drawOutline(screen, x, y, w, h, 2, col)
	drawOutline(screen, x, y, tileSize, tileSize, 2, color.RGBA{120, 220, 255, 240})
}

func (v *Viewer) drawToolbar(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, screenW, toolbarH, color.RGBA{30, 32, 42, 255}, false)
	vector.DrawFilledRect(screen, 0, toolbarH-1, screenW, 1, color.RGBA{80, 90, 110, 255}, false)

	hover := "—"
	if v.hoverValid {
		hover = fmt.Sprintf("(%d, %d)", v.hoverTX, v.hoverTY)
	}
	mlge_text.Draw(screen,
		fmt.Sprintf("WASD pan  Q/E z  Click to stamp  R reset  |  Z: %d/%d  Hover: %s  |  %s",
			v.camZ, v.worldD-1, hover, v.status),
		11, 10, 9, color.RGBA{200, 220, 250, 255})
}

func (v *Viewer) drawSidebar(screen *ebiten.Image) {
	x := screenW - sidebarW
	vector.DrawFilledRect(screen, float32(x), float32(toolbarH), float32(sidebarW), float32(screenH-toolbarH),
		color.RGBA{24, 26, 36, 255}, false)
	vector.DrawFilledRect(screen, float32(x), float32(toolbarH), 1, float32(screenH-toolbarH),
		color.RGBA{80, 90, 110, 255}, false)

	pad := 14
	y := toolbarH + 14

	mlge_text.Draw(screen, "Structure", 13, x+pad, y, color.RGBA{160, 200, 255, 255})
	y += 22
	// Current selection block.
	vector.DrawFilledRect(screen, float32(x+pad), float32(y), float32(sidebarW-pad*2), 28,
		color.RGBA{34, 38, 52, 255}, false)
	mlge_text.Draw(screen, v.scripts[v.scriptIdx], 12, x+pad+8, y+9, color.RGBA{220, 230, 255, 255})
	y += 36

	v.btnPicker = rect{x: x + pad, y: y, w: sidebarW - pad*2, h: 30}
	drawButton(screen, v.btnPicker, "Pick Structure…", color.RGBA{60, 100, 160, 255})
	y += 42

	// Footprint section. Layout per row: [label] [ − ] [text box] [ + ]
	mlge_text.Draw(screen, "Footprint", 13, x+pad, y, color.RGBA{160, 200, 255, 255})
	y += 22
	labelW := 48
	btnSize := 24
	gap := 4
	innerW := sidebarW - pad*2 - labelW - btnSize*2 - gap*2

	v.btnWidthMinus = rect{x: x + pad + labelW, y: y, w: btnSize, h: btnSize}
	v.boxWidth = rect{x: v.btnWidthMinus.x + btnSize + gap, y: y, w: innerW, h: btnSize}
	v.btnWidthPlus = rect{x: v.boxWidth.x + innerW + gap, y: y, w: btnSize, h: btnSize}
	mlge_text.Draw(screen, "Width", 11, x+pad, y+8, color.RGBA{200, 220, 240, 255})
	drawButton(screen, v.btnWidthMinus, "−", color.RGBA{50, 54, 70, 255})
	drawButton(screen, v.btnWidthPlus, "+", color.RGBA{50, 54, 70, 255})
	wtext := v.widthText
	if v.focusKind != focusWidth {
		wtext = strconv.Itoa(v.stampW)
	}
	drawTextBox(screen, v.boxWidth, wtext, "", v.focusKind == focusWidth)
	y += btnSize + 8

	v.btnHeightMinus = rect{x: x + pad + labelW, y: y, w: btnSize, h: btnSize}
	v.boxHeight = rect{x: v.btnHeightMinus.x + btnSize + gap, y: y, w: innerW, h: btnSize}
	v.btnHeightPlus = rect{x: v.boxHeight.x + innerW + gap, y: y, w: btnSize, h: btnSize}
	mlge_text.Draw(screen, "Height", 11, x+pad, y+8, color.RGBA{200, 220, 240, 255})
	drawButton(screen, v.btnHeightMinus, "−", color.RGBA{50, 54, 70, 255})
	drawButton(screen, v.btnHeightPlus, "+", color.RGBA{50, 54, 70, 255})
	htext := v.heightText
	if v.focusKind != focusHeight {
		htext = strconv.Itoa(v.stampH)
	}
	drawTextBox(screen, v.boxHeight, htext, "", v.focusKind == focusHeight)
	y += btnSize + 12

	// Params section — one row per UI-defined param. Each row has a key text
	// box, a value text box, and a delete button. Read by buildParams() and
	// seeded onto the script's top frame at stamp time.
	mlge_text.Draw(screen, "Params", 13, x+pad, y, color.RGBA{160, 200, 255, 255})
	mlge_text.Draw(screen, "(seeded via get_param)", 9, x+pad+58, y+5,
		color.RGBA{120, 140, 170, 255})
	y += 22

	rowH := 24
	delW := 22
	// Layout: [ key | val | x ] with a small gap between fields.
	keyW := (sidebarW - pad*2 - delW - 8) * 4 / 9
	valW := sidebarW - pad*2 - delW - 8 - keyW
	v.paramKeyBoxes = v.paramKeyBoxes[:0]
	v.paramValBoxes = v.paramValBoxes[:0]
	v.paramDelBtns = v.paramDelBtns[:0]
	for i, p := range v.params {
		keyBox := rect{x: x + pad, y: y, w: keyW, h: rowH}
		valBox := rect{x: x + pad + keyW + 4, y: y, w: valW, h: rowH}
		delBtn := rect{x: x + sidebarW - pad - delW, y: y, w: delW, h: rowH}
		drawTextBox(screen, keyBox, p.Key, "key",
			v.focusKind == focusParamKey && v.focusRow == i)
		drawTextBox(screen, valBox, p.Value, "value",
			v.focusKind == focusParamVal && v.focusRow == i)
		drawButton(screen, delBtn, "×", color.RGBA{80, 50, 50, 255})
		v.paramKeyBoxes = append(v.paramKeyBoxes, keyBox)
		v.paramValBoxes = append(v.paramValBoxes, valBox)
		v.paramDelBtns = append(v.paramDelBtns, delBtn)
		y += rowH + 4
	}
	v.btnAddParam = rect{x: x + pad, y: y, w: sidebarW - pad*2, h: 24}
	drawButton(screen, v.btnAddParam, "+ Add Param", color.RGBA{50, 80, 60, 255})

	// Reset pinned to the bottom — params can grow without pushing it
	// off-screen; instead the params just abut the reset button.
	v.btnReset = rect{x: x + pad, y: screenH - 14 - 30, w: sidebarW - pad*2, h: 30}
	drawButton(screen, v.btnReset, "Reset World", color.RGBA{120, 60, 60, 255})
}

// drawTextBox renders a one-line text field. When focused the border brightens
// and a caret bar trails the text; an empty unfocused field shows placeholder.
func drawTextBox(dst *ebiten.Image, r rect, value, placeholder string, focused bool) {
	bg := color.RGBA{18, 20, 28, 255}
	border := color.RGBA{70, 80, 100, 255}
	textCol := color.RGBA{220, 230, 255, 255}
	if focused {
		border = color.RGBA{140, 180, 240, 255}
	}
	vector.DrawFilledRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), bg, false)
	drawOutline(dst, r.x, r.y, r.w, r.h, 1, border)
	if value == "" && !focused {
		mlge_text.Draw(dst, placeholder, 11, r.x+6, r.y+r.h/2-5, color.RGBA{100, 110, 130, 255})
	} else {
		// Right-truncate so long text doesn't bleed past the box.
		maxChars := (r.w - 12) / 7
		shown := value
		if maxChars > 2 && len(shown) > maxChars {
			shown = "…" + shown[len(shown)-maxChars+1:]
		}
		mlge_text.Draw(dst, shown, 11, r.x+6, r.y+r.h/2-5, textCol)
		if focused {
			// Caret right after the last visible glyph.
			caretX := r.x + 6 + len(shown)*7
			if caretX > r.x+r.w-4 {
				caretX = r.x + r.w - 4
			}
			vector.DrawFilledRect(dst, float32(caretX), float32(r.y+4),
				1, float32(r.h-8), textCol, false)
		}
	}
}

func (v *Viewer) pickerRect() rect {
	w := 480
	h := 480
	return rect{x: (screenW - w) / 2, y: (screenH - h) / 2, w: w, h: h}
}

// pickerListLayout returns the start (x,y), inner width, and row height for the
// scrollable script list inside the modal.
func (v *Viewer) pickerListLayout(mr rect) (int, int, int, int) {
	listX := mr.x + 16
	listY := mr.y + 50
	listW := mr.w - 32
	rowH := 26
	return listX, listY, listW, rowH
}

func (v *Viewer) drawPicker(screen *ebiten.Image) {
	// Dim background.
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 140}, false)

	mr := v.pickerRect()
	vector.DrawFilledRect(screen, float32(mr.x), float32(mr.y), float32(mr.w), float32(mr.h),
		color.RGBA{28, 32, 44, 255}, false)
	drawOutline(screen, mr.x, mr.y, mr.w, mr.h, 2, color.RGBA{90, 110, 150, 255})

	mlge_text.Draw(screen, "Pick a Structure", 14, mr.x+16, mr.y+14, color.RGBA{220, 230, 255, 255})
	mlge_text.Draw(screen, "(Esc to cancel)", 10, mr.x+mr.w-110, mr.y+18, color.RGBA{140, 160, 190, 255})

	listX, listY, listW, rowH := v.pickerListLayout(mr)
	// Clip the list area so scrolled rows don't bleed outside the modal.
	clip := screen.SubImage(image.Rect(listX, listY, listX+listW, listY+mr.h-60)).(*ebiten.Image)

	mx, my := ebiten.CursorPosition()
	for i, name := range v.scripts {
		y := listY + i*rowH - v.pickerScroll
		row := rect{x: listX, y: y, w: listW, h: rowH - 2}
		bg := color.RGBA{34, 38, 52, 255}
		if i == v.scriptIdx {
			bg = color.RGBA{60, 90, 140, 255}
		} else if row.contains(mx, my) {
			bg = color.RGBA{48, 54, 72, 255}
		}
		vector.DrawFilledRect(clip, float32(row.x), float32(row.y), float32(row.w), float32(row.h), bg, false)
		mlge_text.Draw(clip, name, 12, row.x+8, row.y+7, color.RGBA{220, 230, 255, 255})
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func drawButton(dst *ebiten.Image, r rect, label string, bg color.RGBA) {
	mx, my := ebiten.CursorPosition()
	col := bg
	if r.contains(mx, my) {
		col.R = clampByte(int(bg.R) + 30)
		col.G = clampByte(int(bg.G) + 30)
		col.B = clampByte(int(bg.B) + 30)
	}
	vector.DrawFilledRect(dst, float32(r.x), float32(r.y), float32(r.w), float32(r.h), col, false)
	drawOutline(dst, r.x, r.y, r.w, r.h, 1, color.RGBA{90, 100, 130, 255})
	mlge_text.Draw(dst, label, 12, r.x+8, r.y+r.h/2-6, color.RGBA{230, 235, 245, 255})
}

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

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

func offscreen(w, h int) *ebiten.Image {
	if cachedOffscreen == nil || cachedOffW != w || cachedOffH != h {
		cachedOffscreen = ebiten.NewImage(w, h)
		cachedOffW, cachedOffH = w, h
	}
	return cachedOffscreen
}
