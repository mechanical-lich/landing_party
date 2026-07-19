package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/minimap"
	"github.com/mechanical-lich/landing_party/internal/view"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

const (
	mapTilePx       = 4 // minimap pixels per world tile
	mapModalPad     = 16
	mapModalHeaderH = 28
	mapModalFooterH = 28
)

const mapModalDoubleClickTicks = 30 // ~0.5s at 60 TPS

// techResourceScanner is the research key that reveals resource deposits on the
// minimap (see internal/minimap and data/research.json).
const techResourceScanner = "resource_scanner"

type MapModal struct {
	Visible bool
	level   *world.Level
	mm      *minimap.Minimap

	viewedZ int
	offsetX int // scroll offset into the full map image (pixels)
	offsetY int

	dragging  bool
	dragLastX int
	dragLastY int

	// double-click tracking
	lastClickTick     int
	tick              int
	OnTileDoubleClick func(x, y, z int)

	// cam is the main camera, set by main_state each frame, so the modal can
	// draw the current viewport box over the map.
	cam view.Camera

	// layout computed once per frame
	contentX, contentY, contentW, contentH int
}

func newMapModal(level *world.Level) *MapModal {
	return &MapModal{
		level: level,
		mm:    minimap.NewMinimap(level, level.GetWidth()*mapTilePx, level.GetHeight()*mapTilePx),
	}
}

// SetCamera tells the modal where the main camera is so it can draw the viewport box.
func (m *MapModal) SetCamera(cam view.Camera) {
	m.cam = cam
}

// worldToScreen converts a world tile coordinate to a screen pixel position.
// Returns (-1,-1) if outside the content area.
func (m *MapModal) worldToScreen(wx, wy int) (int, int) {
	sx := m.contentX + wx*mapTilePx - m.offsetX
	sy := m.contentY + wy*mapTilePx - m.offsetY
	return sx, sy
}

// Open shows the modal, centering the viewport on (worldX, worldY) at layer z.
func (m *MapModal) Open(z, worldX, worldY int) {
	m.viewedZ = z
	m.mm.InvalidateAll()
	m.Visible = true
	m.centerOn(worldX, worldY)
}

func (m *MapModal) centerOn(worldX, worldY int) {
	m.computeLayout()
	cx := worldX*mapTilePx - m.contentW/2
	cy := worldY*mapTilePx - m.contentH/2
	m.setOffset(cx, cy)
}

func (m *MapModal) setOffset(x, y int) {
	maxX := m.level.GetWidth()*mapTilePx - m.contentW
	maxY := m.level.GetHeight()*mapTilePx - m.contentH
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if x < 0 {
		x = 0
	}
	if x > maxX {
		x = maxX
	}
	if y < 0 {
		y = 0
	}
	if y > maxY {
		y = maxY
	}
	m.offsetX = x
	m.offsetY = y
}

func (m *MapModal) computeLayout() {
	cfg := config.Global()
	sw := cfg.ScreenWidth
	sh := cfg.ScreenHeight
	overlayX := mapModalPad
	overlayY := mapModalPad
	overlayW := sw - mapModalPad*2
	overlayH := sh - mapModalPad*2
	m.contentX = overlayX
	m.contentY = overlayY + mapModalHeaderH
	m.contentW = overlayW
	m.contentH = overlayH - mapModalHeaderH - mapModalFooterH
}

func (m *MapModal) Update() {
	if !m.Visible {
		return
	}

	m.tick++
	m.computeLayout()

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if m.viewedZ > 0 {
			m.viewedZ--
			m.mm.InvalidateZ(m.viewedZ)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if m.viewedZ < m.level.GetDepth()-1 {
			m.viewedZ++
			m.mm.InvalidateZ(m.viewedZ)
		}
	}

	cx, cy := ebiten.CursorPosition()
	inContent := cx >= m.contentX && cx < m.contentX+m.contentW &&
		cy >= m.contentY && cy < m.contentY+m.contentH

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && inContent {
		if m.tick-m.lastClickTick <= mapModalDoubleClickTicks && m.OnTileDoubleClick != nil {
			tx := (cx - m.contentX + m.offsetX) / mapTilePx
			ty := (cy - m.contentY + m.offsetY) / mapTilePx
			m.OnTileDoubleClick(tx, ty, m.viewedZ)
		}
		m.lastClickTick = m.tick
		m.dragging = true
		m.dragLastX = cx
		m.dragLastY = cy
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && m.dragging {
		dx := m.dragLastX - cx
		dy := m.dragLastY - cy
		m.setOffset(m.offsetX+dx, m.offsetY+dy)
		m.dragLastX = cx
		m.dragLastY = cy
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		m.dragging = false
	}
}

func (m *MapModal) Draw(screen *ebiten.Image) {
	if !m.Visible {
		return
	}

	cfg := config.Global()
	sw := float64(cfg.ScreenWidth)
	sh := float64(cfg.ScreenHeight)

	ebitenutil.DrawRect(screen, 0, 0, sw, sh, color.RGBA{0, 0, 0, 200})

	overlayX := float64(mapModalPad)
	overlayY := float64(mapModalPad)
	overlayW := sw - float64(mapModalPad)*2
	overlayH := sh - float64(mapModalPad)*2
	ebitenutil.DrawRect(screen, overlayX, overlayY, overlayW, overlayH, color.RGBA{20, 22, 28, 255})
	ebitenutil.DrawRect(screen, overlayX, overlayY, overlayW, 1, color.RGBA{80, 80, 100, 255})
	ebitenutil.DrawRect(screen, overlayX, overlayY+overlayH-1, overlayW, 1, color.RGBA{80, 80, 100, 255})
	ebitenutil.DrawRect(screen, overlayX, overlayY, 1, overlayH, color.RGBA{80, 80, 100, 255})
	ebitenutil.DrawRect(screen, overlayX+overlayW-1, overlayY, 1, overlayH, color.RGBA{80, 80, 100, 255})

	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("MAP  Left/Right arrows = browse floors   drag to pan   double-click = move camera   ESC or M = close   Z: %d", m.viewedZ),
		int(overlayX)+8, int(overlayY)+8)

	img := m.mm.GetImage(m.viewedZ)
	if img != nil {
		// Clip drawing to the content area, then draw the map offset by the scroll position.
		viewport := screen.SubImage(image.Rect(
			m.contentX, m.contentY,
			m.contentX+m.contentW, m.contentY+m.contentH,
		)).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(m.contentX-m.offsetX), float64(m.contentY-m.offsetY))
		viewport.DrawImage(img, op)
	}

	// Camera viewport box — only shown when browsing the same Z the camera is on.
	if m.viewedZ == m.cam.Z {
		vx1, vy1 := m.worldToScreen(m.cam.X, m.cam.Y)
		vx2, vy2 := m.worldToScreen(m.cam.X+m.cam.ViewW(), m.cam.Y+m.cam.ViewH())
		boxColor := color.RGBA{220, 220, 80, 200}
		ebitenutil.DrawRect(screen, float64(vx1), float64(vy1), float64(vx2-vx1), 1, boxColor)
		ebitenutil.DrawRect(screen, float64(vx1), float64(vy2), float64(vx2-vx1), 1, boxColor)
		ebitenutil.DrawRect(screen, float64(vx1), float64(vy1), 1, float64(vy2-vy1), boxColor)
		ebitenutil.DrawRect(screen, float64(vx2), float64(vy1), 1, float64(vy2-vy1+1), boxColor)
	}

	// Entity dots for all entities on the viewed Z level.
	for _, e := range m.level.Entities {
		if !e.HasComponent(rlcomponents.Position) {
			continue
		}
		if e.HasComponent(rlcomponents.Dead) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetZ() != m.viewedZ {
			continue
		}
		if !m.level.GetVisible(pc.GetX(), pc.GetY(), m.viewedZ) {
			continue
		}
		sx, sy := m.worldToScreen(pc.GetX(), pc.GetY())
		if sx < m.contentX || sx >= m.contentX+m.contentW || sy < m.contentY || sy >= m.contentY+m.contentH {
			continue
		}
		var dotColor color.RGBA
		switch {
		case e.HasComponent(components.Worker):
			dotColor = color.RGBA{80, 220, 120, 255} // colonists: green
		case components.IsAttackTarget(e):
			dotColor = color.RGBA{220, 60, 60, 255} // hostiles: red
		default:
			dotColor = color.RGBA{200, 200, 80, 255} // other: yellow
		}
		ebitenutil.DrawRect(screen, float64(sx), float64(sy), float64(mapTilePx), float64(mapTilePx), dotColor)
	}

	footerY := int(overlayY) + int(overlayH) - mapModalFooterH + 8
	ebitenutil.DebugPrintAt(screen, m.floorNavLine(), int(overlayX)+8, footerY)

	cx, cy := ebiten.CursorPosition()
	if cx >= m.contentX && cx < m.contentX+m.contentW && cy >= m.contentY && cy < m.contentY+m.contentH {
		tx := (cx - m.contentX + m.offsetX) / mapTilePx
		ty := (cy - m.contentY + m.offsetY) / mapTilePx
		coords := fmt.Sprintf("x:%d y:%d z:%d", tx, ty, m.viewedZ)
		ebitenutil.DebugPrintAt(screen, coords, int(overlayX+overlayW)-80, footerY)
	}
}

func (m *MapModal) floorNavLine() string {
	line := ""
	for z := 0; z < m.level.GetDepth(); z++ {
		if z > 0 {
			line += "  "
		}
		if z == m.viewedZ {
			line += fmt.Sprintf("[Z%d]", z)
		} else {
			line += fmt.Sprintf("Z%d", z)
		}
	}
	return line
}
