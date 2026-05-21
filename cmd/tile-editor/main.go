package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

const (
	screenW   = 1400
	screenH   = 900
	tileSize  = 24 // source sprite size
	zoom      = 2  // spritesheet display zoom
	previewZ  = 3  // preview tile display zoom
	panelTile = 280 // width of tile-list panel
	panelPrev = 420 // width of preview panel
)

// ── Data types ────────────────────────────────────────────────────────────────

type TileVariant struct {
	Variant int `json:"variant"`
	SpriteX int `json:"spriteX"`
	SpriteY int `json:"spriteY"`
}

type TileDefinition struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Layer       string        `json:"layer,omitempty"` // "floor" | "middle" | "ceiling"; default "middle"
	MixID       int           `json:"mixId,omitempty"` // autotile fusion group; 0 = none
	Solid       bool          `json:"solid,omitempty"`
	Water       bool          `json:"water,omitempty"`
	Door        bool          `json:"door,omitempty"`
	Air         bool          `json:"air,omitempty"`
	Space       bool          `json:"space,omitempty"`
	StairsUp    bool          `json:"stairsUp,omitempty"`
	StairsDown  bool          `json:"stairsDown,omitempty"`
	MovementCost int          `json:"movementCost,omitempty"`
	AutoTile    int           `json:"autoTile,omitempty"`
	Variants    []TileVariant `json:"variants"`
	Resource    string        `json:"resource"`
	SpriteWidth  int          `json:"spriteWidth,omitempty"`
	SpriteHeight int          `json:"spriteHeight,omitempty"`
	SpriteOffsetX int         `json:"spriteOffsetX,omitempty"`
	SpriteOffsetY int         `json:"spriteOffsetY,omitempty"`
	Hidden      bool          `json:"hidden,omitempty"`
}

// ── Preview layouts ───────────────────────────────────────────────────────────
// Each entry is (label, top, bottom, left, right, allDiag) neighbor flags for
// bitmask. allDiag indicates that all 4 corners also match — when true and
// all cardinals are true, the resolver picks variant 16 (the "fully
// interior" sprite). For AutoTileWall only the first 2 fields matter.
var previewCases = []struct {
	label string
	t, b, l, r bool
	allDiag bool
}{
	{"isolated",       false, false, false, false, false},
	{"top only",       true,  false, false, false, false},
	{"bottom only",    false, true,  false, false, false},
	{"top+bottom",     true,  true,  false, false, false},
	{"left only",      false, false, true,  false, false},
	{"top+left",       true,  false, true,  false, false},
	{"bottom+left",    false, true,  true,  false, false},
	{"top+bot+left",   true,  true,  true,  false, false},
	{"right only",     false, false, false, true,  false},
	{"top+right",      true,  false, false, true,  false},
	{"bot+right",      false, true,  false, true,  false},
	{"top+bot+right",  true,  true,  false, true,  false},
	{"left+right",     false, false, true,  true,  false},
	{"top+l+r",        true,  false, true,  true,  false},
	{"bot+l+r",        false, true,  true,  true,  false},
	{"all cardinals",  true,  true,  true,  true,  false},
	{"interior (8/8)", true,  true,  true,  true,  true},
}

func bitmaskIndex(t, b, l, r bool) int {
	idx := 0
	if t { idx |= 1 }
	if b { idx |= 2 }
	if l { idx |= 4 }
	if r { idx |= 8 }
	return idx
}

// Blob47 (GMS2-style) neighbor bit assignments — must match rllayered.
const (
	blobN  = 1
	blobS  = 2
	blobW  = 4
	blobE  = 8
	blobNE = 16
	blobNW = 32
	blobSE = 64
	blobSW = 128
)

// pruneBlobMask zeroes any diagonal bit whose adjacent cardinals aren't both set.
func pruneBlobMask(m int) int {
	if m&(blobN|blobE) != blobN|blobE { m &^= blobNE }
	if m&(blobN|blobW) != blobN|blobW { m &^= blobNW }
	if m&(blobS|blobE) != blobS|blobE { m &^= blobSE }
	if m&(blobS|blobW) != blobS|blobW { m &^= blobSW }
	return m
}

// blob47Masks is the ordered list of 47 valid pruned masks (ascending). Used
// when an AutoTileBlob47 tile is being edited — each slot in the editor maps
// to one of these masks.
var blob47Masks = func() []int {
	seen := map[int]bool{}
	out := []int{}
	for raw := 0; raw < 256; raw++ {
		p := pruneBlobMask(raw)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}()

// ── Editor ────────────────────────────────────────────────────────────────────

type Editor struct {
	// assets
	sheets    map[string]*ebiten.Image
	sheetKeys []string

	// tile defs (loaded from JSON files in defsDir). defSources is parallel to
	// defs: defSources[i] is the file each def came from, so Save can write
	// each file back with only its own defs and preserve the catalog split.
	defs       []TileDefinition
	defSources []string
	defsDir    string
	dirty      bool

	// selection state
	selectedDef     int // index into defs
	selectedSlot    int // variant slot index being edited (0–15)
	activeSheet     int // index into sheetKeys

	// spritesheet scroll
	sheetScrollX int
	sheetScrollY int

	// tile-list scroll
	tileScrollY int

	// preview-panel scroll
	previewScrollY int

	// hovered sprite on sheet
	hoverSX, hoverSY int // sprite coords (multiples of tileSize)
	hoverValid        bool

	// drag scroll
	dragging    bool
	dragStartX  int
	dragStartY  int
	dragScrollX int
	dragScrollY int

	op *ebiten.DrawImageOptions
}

func NewEditor(defsDir string, assetsPath string) (*Editor, error) {
	e := &Editor{
		defsDir: defsDir,
		sheets:  make(map[string]*ebiten.Image),
		op:      &ebiten.DrawImageOptions{},
	}

	// Load tile definitions from every *.json in defsDir, in alphabetical
	// filename order (matches the runtime loader so indices align). Each def
	// remembers its source file so Save can write it back to the right place.
	entries, err := os.ReadDir(defsDir)
	if err != nil {
		return nil, fmt.Errorf("read tile defs dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(defsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		var chunk []TileDefinition
		if err := json.Unmarshal(data, &chunk); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, d := range chunk {
			e.defs = append(e.defs, d)
			e.defSources = append(e.defSources, path)
		}
	}
	if len(e.defs) == 0 {
		return nil, fmt.Errorf("no tile definitions found in %s", defsDir)
	}

	// Load assets map
	assetData, err := os.ReadFile(assetsPath)
	if err != nil {
		return nil, fmt.Errorf("load assets: %w", err)
	}
	var assets map[string]string
	if err := json.Unmarshal(assetData, &assets); err != nil {
		return nil, fmt.Errorf("parse assets: %w", err)
	}

	// Load only the sheets referenced by tile defs
	needed := map[string]bool{}
	for _, def := range e.defs {
		if def.Resource != "" {
			needed[def.Resource] = true
		}
	}
	for key, path := range assets {
		if !needed[key] {
			continue
		}
		// path is relative to data dir; go up one level to project root
		fullPath := filepath.Join(filepath.Dir(assetsPath), "..", path)
		f, err := os.Open(fullPath)
		if err != nil {
			log.Printf("warning: could not open %s: %v", path, err)
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			log.Printf("warning: could not decode %s: %v", path, err)
			continue
		}
		e.sheets[key] = ebiten.NewImageFromImage(img)
		e.sheetKeys = append(e.sheetKeys, key)
	}
	sort.Strings(e.sheetKeys)

	return e, nil
}

func (e *Editor) Save() error {
	// Group defs back by source file, preserving original order within each
	// file. Then rewrite each file with just its own defs so the split
	// catalog stays split.
	groups := make(map[string][]TileDefinition, 4)
	order := make([]string, 0, 4)
	for i, def := range e.defs {
		path := e.defSources[i]
		if _, seen := groups[path]; !seen {
			order = append(order, path)
		}
		groups[path] = append(groups[path], def)
	}
	for _, path := range order {
		data, err := json.MarshalIndent(groups[path], "", "    ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", path, err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	e.dirty = false
	return nil
}

// ── Layout constants ──────────────────────────────────────────────────────────
//
//   [0 .. panelTile)         tile list
//   [panelTile .. panelTile+sheetW) spritesheet
//   right panelPrev           preview
//
func (e *Editor) sheetPanelX() int { return panelTile }
func (e *Editor) sheetPanelW() int { return screenW - panelTile - panelPrev }
func (e *Editor) previewPanelX() int { return screenW - panelPrev }

func (e *Editor) currentDef() *TileDefinition {
	if e.selectedDef < 0 || e.selectedDef >= len(e.defs) {
		return nil
	}
	return &e.defs[e.selectedDef]
}

func (e *Editor) currentSheet() *ebiten.Image {
	if len(e.sheetKeys) == 0 {
		return nil
	}
	def := e.currentDef()
	if def != nil && def.Resource != "" {
		if img, ok := e.sheets[def.Resource]; ok {
			return img
		}
	}
	return e.sheets[e.sheetKeys[e.activeSheet]]
}

// ── Update ────────────────────────────────────────────────────────────────────

func (e *Editor) Update() error {
	mx, my := ebiten.CursorPosition()

	// Save
	if inpututil.IsKeyJustPressed(ebiten.KeyS) && ebiten.IsKeyPressed(ebiten.KeyControl) {
		if err := e.Save(); err != nil {
			log.Printf("save error: %v", err)
		}
	}

	// Cycle autoTile mode for current def
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if def := e.currentDef(); def != nil {
			def.AutoTile = (def.AutoTile + 1) % 4
			e.dirty = true
		}
	}

	// Delete selected variant slot
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if def := e.currentDef(); def != nil {
			id := e.slotToVariantID(e.selectedSlot)
			newV := []TileVariant{}
			for _, v := range def.Variants {
				if v.Variant != id {
					newV = append(newV, v)
				}
			}
			def.Variants = newV
			e.dirty = true
		}
	}

	// Scroll tile list
	if my < screenH && mx < panelTile {
		_, dy := ebiten.Wheel()
		e.tileScrollY -= int(dy) * 20
		if e.tileScrollY < 0 { e.tileScrollY = 0 }
	}

	// Scroll preview panel
	if mx >= e.previewPanelX() {
		_, dy := ebiten.Wheel()
		e.previewScrollY -= int(dy) * 40
		if e.previewScrollY < 0 {
			e.previewScrollY = 0
		}
	}

	// Spritesheet panel interactions
	spx := e.sheetPanelX()
	spw := e.sheetPanelW()
	if mx >= spx && mx < spx+spw {
		sheet := e.currentSheet()
		if sheet != nil {
			sw, sh := sheet.Bounds().Dx()*zoom, sheet.Bounds().Dy()*zoom
			sheetAreaH := screenH - 58

			dx, dy := ebiten.Wheel()
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				// Shift+scroll = horizontal
				e.sheetScrollX -= int(dx+dy) * tileSize * zoom
			} else {
				e.sheetScrollY -= int(dy) * tileSize * zoom
				e.sheetScrollX -= int(dx) * tileSize * zoom
			}
			maxScrollX := sw - spw
			if maxScrollX < 0 { maxScrollX = 0 }
			maxScrollY := sh - sheetAreaH
			if maxScrollY < 0 { maxScrollY = 0 }
			if e.sheetScrollX < 0 { e.sheetScrollX = 0 }
			if e.sheetScrollX > maxScrollX { e.sheetScrollX = maxScrollX }
			if e.sheetScrollY < 0 { e.sheetScrollY = 0 }
			if e.sheetScrollY > maxScrollY { e.sheetScrollY = maxScrollY }

			// Drag scroll (middle mouse)
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
				e.dragging = true
				e.dragStartX, e.dragStartY = mx, my
				e.dragScrollX, e.dragScrollY = e.sheetScrollX, e.sheetScrollY
			}
			if e.dragging {
				if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
					e.sheetScrollX = e.dragScrollX - (mx-e.dragStartX)
					e.sheetScrollY = e.dragScrollY - (my-e.dragStartY)
					if e.sheetScrollX < 0 { e.sheetScrollX = 0 }
					if e.sheetScrollX > maxScrollX { e.sheetScrollX = maxScrollX }
					if e.sheetScrollY < 0 { e.sheetScrollY = 0 }
					if e.sheetScrollY > maxScrollY { e.sheetScrollY = maxScrollY }
				} else {
					e.dragging = false
				}
			}

			// Hover tile
			sheetAreaY := 58
			relX := mx - spx + e.sheetScrollX
			relY := my - sheetAreaY + e.sheetScrollY
			e.hoverSX = (relX / (tileSize * zoom)) * tileSize
			e.hoverSY = (relY / (tileSize * zoom)) * tileSize
			e.hoverValid = true

			// Click: assign hovered sprite to selected variant slot
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				e.assignSprite(e.hoverSX, e.hoverSY)
			}
		}
	} else {
		e.hoverValid = false
	}

	// Click on tile list
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx < panelTile {
		itemH := 22
		idx := (my + e.tileScrollY - 30) / itemH
		if idx >= 0 && idx < len(e.defs) {
			e.selectedDef = idx
			e.selectedSlot = 0
			e.sheetScrollX = 0
			e.sheetScrollY = 0
		}
	}

	// Click on variant slots (shown in spritesheet panel header)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= spx && mx < spx+spw && my < 30 {
		slot := (mx - spx) / 30
		e.selectedSlot = slot
	}

	// Right-click in preview area: cycle selected slot forward
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && mx >= e.previewPanelX() {
		if max := e.maxSlots(); max > 0 {
			e.selectedSlot = (e.selectedSlot + 1) % max
		}
	}
	// Left-click in preview area: pick the slot whose preview was clicked.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && mx >= e.previewPanelX() && my >= 38 {
		ppx := e.previewPanelX()
		ts := 24
		gridSize := ts * 3
		gap := 8
		labelH := 14
		cellW := gridSize + gap
		cellH := gridSize + labelH + gap
		cols := (panelPrev - 8) / cellW
		if cols < 1 {
			cols = 1
		}
		col := (mx - ppx - 4) / cellW
		row := (my - 38 + e.previewScrollY) / cellH
		if col >= 0 && col < cols && row >= 0 {
			slot := row*cols + col
			if slot < e.maxSlots() {
				e.selectedSlot = slot
			}
		}
	}

	return nil
}

func (e *Editor) assignSprite(sx, sy int) {
	def := e.currentDef()
	if def == nil {
		return
	}
	id := e.slotToVariantID(e.selectedSlot)
	for i, v := range def.Variants {
		if v.Variant == id {
			def.Variants[i].SpriteX = sx
			def.Variants[i].SpriteY = sy
			e.dirty = true
			e.selectedSlot = (e.selectedSlot + 1) % e.maxSlots()
			return
		}
	}
	def.Variants = append(def.Variants, TileVariant{
		Variant: id,
		SpriteX: sx,
		SpriteY: sy,
	})
	e.dirty = true
	e.selectedSlot = (e.selectedSlot + 1) % e.maxSlots()
}

func (e *Editor) maxSlots() int {
	def := e.currentDef()
	if def == nil {
		return 1
	}
	switch def.AutoTile {
	case 1:
		return 2
	case 2:
		// 16 bitmask variants (variants 0-15) + 1 "fully interior" sprite
		// at slot 16 used when all 8 neighbors match.
		return 17
	case 3:
		// 47-tile blob: 8-direction with corner pruning. Variant indices
		// 0..46 are assigned in ascending pruned-mask order by the engine.
		return 47
	default:
		return 8 // arbitrary max for manual variants
	}
}

// slotToVariantID maps the editor's internal slot index to the actual
// `variant` field stored on each TileVariant. For most modes the slot IS the
// variant ID, but for Blob47 the slot indexes into the table of 47 valid
// pruned masks.
func (e *Editor) slotToVariantID(slot int) int {
	def := e.currentDef()
	if def != nil && def.AutoTile == 3 {
		if slot >= 0 && slot < len(blob47Masks) {
			return blob47Masks[slot]
		}
		return 0
	}
	return slot
}

func (e *Editor) variantAt(slot int) *TileVariant {
	def := e.currentDef()
	if def == nil {
		return nil
	}
	id := e.slotToVariantID(slot)
	for i := range def.Variants {
		if def.Variants[i].Variant == id {
			return &def.Variants[i]
		}
	}
	return nil
}

// ── Draw ──────────────────────────────────────────────────────────────────────

func (e *Editor) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 22, 30, 255})

	e.drawTileList(screen)
	e.drawSheetPanel(screen)
	e.drawPreviewPanel(screen)
}

func (e *Editor) drawTileList(screen *ebiten.Image) {
	// Panel background
	drawRect(screen, 0, 0, panelTile, screenH, color.RGBA{30, 32, 42, 255})
	mlge_text.Draw(screen, "Tile Types", 12, 6, 10, color.RGBA{160, 200, 255, 255})

	itemH := 22
	for i, def := range e.defs {
		y := 30 + i*itemH - e.tileScrollY
		if y < 28 || y > screenH {
			continue
		}
		bg := color.RGBA{30, 32, 42, 0}
		if i == e.selectedDef {
			bg = color.RGBA{60, 90, 140, 255}
		}
		drawRect(screen, 1, y, panelTile-2, itemH-1, bg)
		nameColor := color.RGBA{200, 220, 255, 255}
		if def.AutoTile > 0 {
			nameColor = color.RGBA{180, 255, 180, 255}
		}
		mlge_text.Draw(screen, def.Name, 11, 6, y+5, nameColor)
		// variant count badge
		vc := fmt.Sprintf("%d", len(def.Variants))
		mlge_text.Draw(screen, vc, 10, panelTile-20, y+5, color.RGBA{120, 140, 160, 255})
	}
}

func (e *Editor) drawSheetPanel(screen *ebiten.Image) {
	spx := e.sheetPanelX()
	spw := e.sheetPanelW()

	drawRect(screen, spx, 0, spw, screenH, color.RGBA{18, 20, 28, 255})

	def := e.currentDef()
	if def == nil {
		return
	}

	// Header: tile name, autoTile mode, variant slots
	modeNames := []string{"Manual", "Wall (2)", "Bitmask (16)", "Blob47"}
	modeName := "Unknown"
	if def.AutoTile >= 0 && def.AutoTile < len(modeNames) {
		modeName = modeNames[def.AutoTile]
	}
	header := fmt.Sprintf("%s  |  AutoTile: %s  [Tab=cycle]", def.Name, modeName)
	if e.dirty {
		header += "  *unsaved* (Ctrl+S)"
	}
	mlge_text.Draw(screen, header, 11, spx+6, 6, color.RGBA{220, 220, 100, 255})

	// Variant slot row
	slots := e.maxSlots()
	for i := 0; i < slots; i++ {
		sx := spx + 4 + i*28
		sy := 18
		v := e.variantAt(i)
		bg := color.RGBA{40, 44, 58, 255}
		if i == e.selectedSlot {
			bg = color.RGBA{80, 130, 200, 255}
		}
		drawRect(screen, sx, sy, 26, 26, bg)

		if v != nil {
			sheet := e.currentSheet()
			if sheet != nil {
				e.drawSprite(screen, sheet, v.SpriteX, v.SpriteY, sx+1, sy+1, 24, 24)
			}
		} else {
			mlge_text.Draw(screen, fmt.Sprintf("%d", i), 9, sx+4, sy+7, color.RGBA{100, 100, 130, 255})
		}
	}

	// Bitmask slot labels
	if def.AutoTile == 2 {
		bitmaskLabels := []string{"∅","T","B","TB","L","TL","BL","TBL","R","TR","BR","TBR","LR","TLR","BLR","ALL"}
		for i := 0; i < 16 && i < slots; i++ {
			sx := spx + 4 + i*28
			mlge_text.Draw(screen, bitmaskLabels[i], 8, sx+1, 46, color.RGBA{140, 160, 180, 255})
		}
	}

	// Sheet browser (below header)
	sheet := e.currentSheet()
	if sheet == nil {
		mlge_text.Draw(screen, "No sheet for this tile's resource", 12, spx+10, 100, color.RGBA{200, 100, 100, 255})
		return
	}

	sheetAreaY := 58
	sw, sh := sheet.Bounds().Dx(), sheet.Bounds().Dy()
	dispW := sw * zoom
	dispH := sh * zoom

	// Clip draw to panel
	subScreen := screen.SubImage(image.Rect(spx, sheetAreaY, spx+spw, screenH)).(*ebiten.Image)

	e.op.GeoM.Reset()
	e.op.GeoM.Scale(float64(zoom), float64(zoom))
	e.op.GeoM.Translate(float64(spx-e.sheetScrollX), float64(sheetAreaY-e.sheetScrollY))
	subScreen.DrawImage(sheet, e.op)
	_ = dispW
	_ = dispH

	// Draw grid lines
	gridCol := color.RGBA{60, 60, 80, 180}
	for gx := 0; gx*tileSize*zoom < spw+e.sheetScrollX; gx++ {
		px := spx + gx*tileSize*zoom - e.sheetScrollX
		if px >= spx && px < spx+spw {
			drawRect(screen, px, sheetAreaY, 1, screenH-sheetAreaY, gridCol)
		}
	}
	for gy := 0; gy*tileSize*zoom < screenH+e.sheetScrollY; gy++ {
		py := sheetAreaY + gy*tileSize*zoom - e.sheetScrollY
		if py >= sheetAreaY && py < screenH {
			drawRect(screen, spx, py, spw, 1, gridCol)
		}
	}

	// Highlight assigned variant positions
	for _, v := range def.Variants {
		hx := spx + v.SpriteX*zoom - e.sheetScrollX
		hy := sheetAreaY + v.SpriteY*zoom - e.sheetScrollY
		hw := tileSize * zoom
		col := color.RGBA{80, 180, 80, 180}
		if v.Variant == e.selectedSlot {
			col = color.RGBA{80, 180, 255, 220}
		}
		drawOutlineRect(screen, hx, hy, hw, hw, col)
		mlge_text.Draw(screen, fmt.Sprintf("%d", v.Variant), 9, hx+2, hy+2, col)
	}

	// Hover highlight
	if e.hoverValid {
		hx := spx + e.hoverSX*zoom - e.sheetScrollX
		hy := sheetAreaY + e.hoverSY*zoom - e.sheetScrollY
		hw := tileSize * zoom
		drawOutlineRect(screen, hx, hy, hw, hw, color.RGBA{255, 220, 60, 200})
	}

	// Scrollbars
	sheet2 := e.currentSheet()
	if sheet2 != nil {
		sw2, sh2 := sheet2.Bounds().Dx()*zoom, sheet2.Bounds().Dy()*zoom
		sheetAreaH := screenH - 58

		// Horizontal scrollbar
		if sw2 > spw {
			barW := spw * spw / sw2
			barX := spx + e.sheetScrollX*spw/sw2
			drawRect(screen, spx, screenH-6, spw, 6, color.RGBA{30, 34, 48, 255})
			drawRect(screen, barX, screenH-6, barW, 6, color.RGBA{100, 120, 180, 200})
		}
		// Vertical scrollbar
		if sh2 > sheetAreaH {
			barH := sheetAreaH * sheetAreaH / sh2
			barY := 58 + e.sheetScrollY*sheetAreaH/sh2
			drawRect(screen, spx+spw-6, 58, 6, sheetAreaH, color.RGBA{30, 34, 48, 255})
			drawRect(screen, spx+spw-6, barY, 6, barH, color.RGBA{100, 120, 180, 200})
		}
	}

	// Resource name
	resName := def.Resource
	for _, k := range e.sheetKeys {
		if img, ok := e.sheets[k]; ok && img == sheet {
			resName = k
			break
		}
	}
	mlge_text.Draw(screen, "Sheet: "+resName+"   scroll=wheel  horizontal=Shift+wheel or middle-drag", 10, spx+4, screenH-16, color.RGBA{120, 140, 160, 255})
}

// resolveSlot returns the variant slot index for a tile with the given
// neighbor flags. allDiag=true together with all 4 cardinals selects the
// "fully interior" slot 16.
func (e *Editor) resolveSlot(autoTile int, t, b, l, r, allDiag bool) int {
	switch autoTile {
	case 1:
		if b { return 0 }
		return 1
	case 2:
		if t && b && l && r && allDiag {
			return 16
		}
		return bitmaskIndex(t, b, l, r)
	default:
		return 0
	}
}

// drawPreviewTile draws a single tile cell (or empty background if no variant).
func (e *Editor) drawPreviewTile(screen *ebiten.Image, sheet *ebiten.Image, autoTile int, t, b, l, r, allDiag bool, px, py, ts int) {
	drawRect(screen, px, py, ts, ts, color.RGBA{35, 38, 52, 255})
	slot := e.resolveSlot(autoTile, t, b, l, r, allDiag)
	v := e.variantAt(slot)
	if v != nil {
		e.drawSprite(screen, sheet, v.SpriteX, v.SpriteY, px, py, ts, ts)
	} else {
		// missing variant — show a red X
		mid := ts / 2
		drawRect(screen, px+mid-1, py+2, 2, ts-4, color.RGBA{160, 50, 50, 200})
		drawRect(screen, px+2, py+mid-1, ts-4, 2, color.RGBA{160, 50, 50, 200})
	}
}

func (e *Editor) drawPreviewPanel(screen *ebiten.Image) {
	ppx := e.previewPanelX()
	drawRect(screen, ppx, 0, panelPrev, screenH, color.RGBA{25, 27, 36, 255})

	def := e.currentDef()
	sheet := e.currentSheet()

	mlge_text.Draw(screen, "Preview", 12, ppx+8, 8, color.RGBA{160, 200, 255, 255})
	mlge_text.Draw(screen, "[Del]=clear slot  [RClick]=next slot", 9, ppx+4, 24, color.RGBA{120, 130, 150, 255})

	if def == nil || sheet == nil {
		return
	}

	// Each case is drawn as a 3×3 grid showing the center tile + its present neighbors.
	// ts = size of each cell in the 3×3 grid.
	ts := 24
	gridSize := ts * 3
	gap := 8
	labelH := 14
	cellW := gridSize + gap
	cellH := gridSize + labelH + gap
	cols := (panelPrev - 8) / cellW
	if cols < 1 { cols = 1 }

	if def.AutoTile == 3 {
		e.drawBlob47Previews(screen, sheet, ppx, ts, gridSize, gap, labelH, cols, cellW, cellH)
		return
	}

	cases := previewCases
	if def.AutoTile == 1 {
		cases = previewCases[:2]
	}

	for i, pc := range cases {
		col := i % cols
		row := i / cols
		gx := ppx + 4 + col*cellW
		gy := 38 + row*cellH - e.previewScrollY

		// 3×3 positions: cardinals present per flag, corners present only when
		// allDiag is set (the "fully interior" preview case).
		// Layout: [TL] [T] [TR] / [L] [C] [R] / [BL] [B] [BR]
		corner := pc.allDiag
		draws := [3][3]struct{ present, t, b, l, r bool }{
			{{corner, false,true,false,true}, {pc.t,  false,true,false,false}, {corner, false,true,true,false}},
			{{pc.l,  false,false,false,true}, {true,  pc.t,pc.b,pc.l,pc.r},   {pc.r,   false,false,true,false}},
			{{corner, true,false,false,true}, {pc.b,  true,false,false,false}, {corner, true,false,true,false}},
		}

		for ry := 0; ry < 3; ry++ {
			for rx := 0; rx < 3; rx++ {
				cx := gx + rx*ts
				cy := gy + ry*ts
				cell := draws[ry][rx]
				if !cell.present {
					// Empty cell — dark background
					drawRect(screen, cx, cy, ts-1, ts-1, color.RGBA{20, 22, 30, 255})
					continue
				}
				// In the interior preview, EVERY cell is conceptually fully
				// surrounded (the 3×3 is a slice of a larger mass), so all
				// four cardinals and the diagonal flag should be set so each
				// cell resolves to variant 16.
				ct, cb, cl, cr := cell.t, cell.b, cell.l, cell.r
				if pc.allDiag {
					ct, cb, cl, cr = true, true, true, true
				}
				e.drawPreviewTile(screen, sheet, def.AutoTile, ct, cb, cl, cr, pc.allDiag, cx, cy, ts-1)
			}
		}

		// Highlight center tile if it's the selected slot
		centerSlot := e.resolveSlot(def.AutoTile, pc.t, pc.b, pc.l, pc.r, pc.allDiag)
		labelCol := color.RGBA{140, 160, 180, 255}
		if centerSlot == e.selectedSlot {
			drawOutlineRect(screen, gx+ts, gy+ts, ts-1, ts-1, color.RGBA{80, 180, 255, 220})
			labelCol = color.RGBA{80, 180, 255, 255}
		}

		mlge_text.Draw(screen, pc.label, 8, gx, gy+gridSize+2, labelCol)
	}
}

// drawBlob47Previews renders all 47 valid pruned masks as 3×3 previews so the
// artist can click each one and assign a sprite from the source sheet.
//
// Each surrounding cell of the 3×3 ALSO renders as a real sprite (looked up
// via its own derived mask) so the preview reads as a coherent slice of a
// wall mass — letting you spot mis-assignments visually.
func (e *Editor) drawBlob47Previews(screen *ebiten.Image, sheet *ebiten.Image, ppx, ts, gridSize, gap, labelH, cols, cellW, cellH int) {
	def := e.currentDef()
	if def == nil {
		return
	}
	emptyCol := color.RGBA{20, 22, 30, 255}
	bgCol := color.RGBA{35, 38, 52, 255}

	for i, mask := range blob47Masks {
		col := i % cols
		row := i / cols
		gx := ppx + 4 + col*cellW
		gy := 38 + row*cellH - e.previewScrollY

		for ry := 0; ry < 3; ry++ {
			for rx := 0; rx < 3; rx++ {
				cx := gx + rx*ts
				cy := gy + ry*ts
				// Position in the 3×3, with center at (0,0).
				dx, dy := rx-1, ry-1
				if !blob47IsWallAt(mask, dx, dy) {
					drawRect(screen, cx, cy, ts-1, ts-1, emptyCol)
					continue
				}
				// This cell is a wall. Compute its own pruned mask by asking
				// for each of its 8 neighbors whether THEY are also walls in
				// this preview's context, then look up its sprite.
				cellMask := blob47CellMask(mask, dx, dy)
				v := e.findVariantByID(cellMask)
				if v != nil {
					drawRect(screen, cx, cy, ts-1, ts-1, bgCol)
					e.drawSprite(screen, sheet, v.SpriteX, v.SpriteY, cx, cy, ts-1, ts-1)
				} else {
					drawRect(screen, cx, cy, ts-1, ts-1, bgCol)
					mid := (ts - 1) / 2
					drawRect(screen, cx+mid-1, cy+2, 2, ts-5, color.RGBA{160, 50, 50, 200})
					drawRect(screen, cx+2, cy+mid-1, ts-5, 2, color.RGBA{160, 50, 50, 200})
				}
			}
		}

		// Highlight if this is the selected slot.
		labelCol := color.RGBA{140, 160, 180, 255}
		if i == e.selectedSlot {
			drawOutlineRect(screen, gx+ts, gy+ts, ts-1, ts-1, color.RGBA{80, 180, 255, 220})
			labelCol = color.RGBA{80, 180, 255, 255}
		}
		mlge_text.Draw(screen, fmt.Sprintf("%d", mask), 8, gx, gy+gridSize+2, labelCol)
	}
}

// blob47IsWallAt reports whether the cell at offset (dx, dy) from the center
// is a wall in the preview's context. Center is always wall; cells within the
// 3×3 follow the center's mask; cells further out are extrapolated by
// stepping one tile back toward the center.
func blob47IsWallAt(centerMask, dx, dy int) bool {
	if dx == 0 && dy == 0 {
		return true
	}
	if dx >= -1 && dx <= 1 && dy >= -1 && dy <= 1 {
		switch {
		case dx == 0 && dy == -1:
			return centerMask&blobN != 0
		case dx == 0 && dy == 1:
			return centerMask&blobS != 0
		case dx == -1 && dy == 0:
			return centerMask&blobW != 0
		case dx == 1 && dy == 0:
			return centerMask&blobE != 0
		case dx == 1 && dy == -1:
			return centerMask&blobNE != 0
		case dx == -1 && dy == -1:
			return centerMask&blobNW != 0
		case dx == 1 && dy == 1:
			return centerMask&blobSE != 0
		case dx == -1 && dy == 1:
			return centerMask&blobSW != 0
		}
	}
	// Outside the 3×3: extrapolate by walking one cell back toward center.
	sx, sy := signInt(dx), signInt(dy)
	return blob47IsWallAt(centerMask, dx-sx, dy-sy)
}

func signInt(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// blob47CellMask returns the pruned mask for a cell at offset (dx, dy) from
// the preview's center, given the center's mask.
func blob47CellMask(centerMask, dx, dy int) int {
	m := 0
	if blob47IsWallAt(centerMask, dx, dy-1) {
		m |= blobN
	}
	if blob47IsWallAt(centerMask, dx, dy+1) {
		m |= blobS
	}
	if blob47IsWallAt(centerMask, dx-1, dy) {
		m |= blobW
	}
	if blob47IsWallAt(centerMask, dx+1, dy) {
		m |= blobE
	}
	if blob47IsWallAt(centerMask, dx+1, dy-1) {
		m |= blobNE
	}
	if blob47IsWallAt(centerMask, dx-1, dy-1) {
		m |= blobNW
	}
	if blob47IsWallAt(centerMask, dx+1, dy+1) {
		m |= blobSE
	}
	if blob47IsWallAt(centerMask, dx-1, dy+1) {
		m |= blobSW
	}
	return pruneBlobMask(m)
}

// findVariantByID returns the *TileVariant whose Variant field equals id,
// or nil if none exists yet.
func (e *Editor) findVariantByID(id int) *TileVariant {
	def := e.currentDef()
	if def == nil {
		return nil
	}
	for i := range def.Variants {
		if def.Variants[i].Variant == id {
			return &def.Variants[i]
		}
	}
	return nil
}

func (e *Editor) drawSprite(dst *ebiten.Image, sheet *ebiten.Image, sx, sy, dx, dy, dw, dh int) {
	src := sheet.SubImage(image.Rect(sx, sy, sx+tileSize, sy+tileSize)).(*ebiten.Image)
	e.op.GeoM.Reset()
	e.op.ColorScale.Reset()
	e.op.GeoM.Scale(float64(dw)/float64(tileSize), float64(dh)/float64(tileSize))
	e.op.GeoM.Translate(float64(dx), float64(dy))
	dst.DrawImage(src, e.op)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

var rectImg = ebiten.NewImage(1, 1)

func drawRect(dst *ebiten.Image, x, y, w, h int, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	rectImg.Fill(c)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), float64(h))
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(rectImg, op)
}

func drawOutlineRect(dst *ebiten.Image, x, y, w, h int, c color.RGBA) {
	drawRect(dst, x, y, w, 2, c)
	drawRect(dst, x, y+h-2, w, 2, c)
	drawRect(dst, x, y, 2, h, c)
	drawRect(dst, x+w-2, y, 2, h, c)
}

// ── Ebiten boilerplate ────────────────────────────────────────────────────────

func (e *Editor) Layout(ow, oh int) (int, int) { return screenW, screenH }

func main() {
	// Run from the project root (data/ and assets/ must be accessible)
	defsDir := "data/tiledefinitions"
	assetsPath := "data/assets.json"

	ed, err := NewEditor(defsDir, assetsPath)
	if err != nil {
		log.Fatalf("init editor: %v", err)
	}

	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Tile Definition Editor")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(ed); err != nil {
		log.Fatal(err)
	}
}
