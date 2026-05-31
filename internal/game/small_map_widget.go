package game

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/minimap"
	"github.com/mechanical-lich/landing_party/internal/world"
)

const (
	followBtnW = 52
	followBtnH = 16
)

const (
	smallMapSize   = 200
	smallMapMargin = 8
)

type SmallMapWidget struct {
	level              *world.Level
	mm                 *minimap.Minimap
	camX, camY, camZ   int
	camViewW, camViewH int
	OnClick            func()
	OnFollowClick      func()
	FollowActive       bool

	lastZ int

	// screen bounds, computed in Draw
	screenX, screenY int

	// follow button bounds, computed in Draw
	followBtnX, followBtnY int
}

func newSmallMapWidget(level *world.Level, mm *minimap.Minimap) *SmallMapWidget {
	return &SmallMapWidget{level: level, mm: mm}
}

// WithinBounds returns true if (x, y) falls inside the minimap or follow button.
func (w *SmallMapWidget) WithinBounds(x, y int) bool {
	inMap := x >= w.screenX && x < w.screenX+smallMapSize &&
		y >= w.screenY && y < w.screenY+smallMapSize
	inBtn := x >= w.followBtnX && x < w.followBtnX+followBtnW &&
		y >= w.followBtnY && y < w.followBtnY+followBtnH
	return inMap || inBtn
}

func (w *SmallMapWidget) SetCamera(x, y, z, viewW, viewH int) {
	w.camX, w.camY, w.camZ = x, y, z
	w.camViewW, w.camViewH = viewW, viewH
}

func (w *SmallMapWidget) Update() {
	// Track Z changes. The destination Z's cached image is reused if it
	// exists (its content doesn't change just because the camera moved);
	// tile edits and fog are picked up by the per-frame partial refresh
	// below. A missing image is built lazily once by InvalidatePartial.
	if w.camZ != w.lastZ {
		w.lastZ = w.camZ
	}

	// Partial refresh every frame for the visible region, full regen periodically.
	padX := w.camViewW / 2
	padY := w.camViewH / 2
	w.mm.InvalidatePartial(w.camZ, w.camX-padX, w.camY-padY, w.camViewW+padX*2, w.camViewH+padY*2)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		cx, cy := ebiten.CursorPosition()
		if cx >= w.followBtnX && cx < w.followBtnX+followBtnW &&
			cy >= w.followBtnY && cy < w.followBtnY+followBtnH {
			if w.OnFollowClick != nil {
				w.OnFollowClick()
			}
			return
		}
		if w.OnClick != nil &&
			cx >= w.screenX && cx < w.screenX+smallMapSize &&
			cy >= w.screenY && cy < w.screenY+smallMapSize {
			w.OnClick()
		}
	}
}

func (w *SmallMapWidget) Draw(screen *ebiten.Image) {
	cfg := config.Global()
	w.screenX = cfg.ScreenWidth - smallMapSize - smallMapMargin
	w.screenY = smallMapMargin

	// Follow button — sits to the left of the minimap, top-aligned with it.
	w.followBtnX = w.screenX - followBtnW - 2
	w.followBtnY = w.screenY
	btnBg := color.RGBA{30, 40, 55, 230}
	if w.FollowActive {
		btnBg = color.RGBA{60, 160, 100, 230}
	}
	ebitenutil.DrawRect(screen, float64(w.followBtnX), float64(w.followBtnY), float64(followBtnW), float64(followBtnH), btnBg)
	ebitenutil.DrawRect(screen, float64(w.followBtnX), float64(w.followBtnY), float64(followBtnW), 1, color.RGBA{80, 80, 100, 255})
	ebitenutil.DrawRect(screen, float64(w.followBtnX), float64(w.followBtnY+followBtnH-1), float64(followBtnW), 1, color.RGBA{80, 80, 100, 255})
	ebitenutil.DrawRect(screen, float64(w.followBtnX), float64(w.followBtnY), 1, float64(followBtnH), color.RGBA{80, 80, 100, 255})
	ebitenutil.DrawRect(screen, float64(w.followBtnX+followBtnW-1), float64(w.followBtnY), 1, float64(followBtnH), color.RGBA{80, 80, 100, 255})
	label := "Follow"
	if w.FollowActive {
		label = "Unfollow"
	}
	ebitenutil.DebugPrintAt(screen, label, w.followBtnX+4, w.followBtnY+3)

	// Dark background.
	ebitenutil.DrawRect(screen, float64(w.screenX), float64(w.screenY),
		float64(smallMapSize), float64(smallMapSize), color.RGBA{10, 10, 14, 255})

	img := w.mm.GetImage(w.camZ)
	if img == nil {
		return
	}

	worldW := w.level.GetWidth()
	worldH := w.level.GetHeight()

	// Source region: camera view + 50% padding on each side.
	padX := w.camViewW / 2
	padY := w.camViewH / 2
	srcTileX := w.camX - padX
	srcTileY := w.camY - padY
	srcTileW := w.camViewW + padX*2
	srcTileH := w.camViewH + padY*2

	// Clamp to world bounds.
	if srcTileX < 0 {
		srcTileX = 0
	}
	if srcTileY < 0 {
		srcTileY = 0
	}
	if srcTileX+srcTileW > worldW {
		srcTileW = worldW - srcTileX
	}
	if srcTileY+srcTileH > worldH {
		srcTileH = worldH - srcTileY
	}

	// Convert to minimap pixel coords.
	srcPxX := srcTileX * mapTilePx
	srcPxY := srcTileY * mapTilePx
	srcPxW := srcTileW * mapTilePx
	srcPxH := srcTileH * mapTilePx

	sub := img.SubImage(image.Rect(srcPxX, srcPxY, srcPxX+srcPxW, srcPxY+srcPxH)).(*ebiten.Image)

	scaleX := float64(smallMapSize) / float64(srcPxW)
	scaleY := float64(smallMapSize) / float64(srcPxH)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleX, scaleY)
	op.GeoM.Translate(float64(w.screenX), float64(w.screenY))
	screen.DrawImage(sub, op)

	// Camera viewport box.
	camPxX := float64(w.screenX) + float64(w.camX-srcTileX)*float64(mapTilePx)*scaleX
	camPxY := float64(w.screenY) + float64(w.camY-srcTileY)*float64(mapTilePx)*scaleY
	camPxW := float64(w.camViewW) * float64(mapTilePx) * scaleX
	camPxH := float64(w.camViewH) * float64(mapTilePx) * scaleY
	boxCol := color.RGBA{220, 220, 80, 200}
	ebitenutil.DrawRect(screen, camPxX, camPxY, camPxW, 1, boxCol)
	ebitenutil.DrawRect(screen, camPxX, camPxY+camPxH, camPxW, 1, boxCol)
	ebitenutil.DrawRect(screen, camPxX, camPxY, 1, camPxH, boxCol)
	ebitenutil.DrawRect(screen, camPxX+camPxW, camPxY, 1, camPxH+1, boxCol)

	// Widget border.
	bx, by := float64(w.screenX), float64(w.screenY)
	bs := float64(smallMapSize)
	borderCol := color.RGBA{80, 80, 100, 255}
	ebitenutil.DrawRect(screen, bx, by, bs, 1, borderCol)
	ebitenutil.DrawRect(screen, bx, by+bs, bs, 1, borderCol)
	ebitenutil.DrawRect(screen, bx, by, 1, bs, borderCol)
	ebitenutil.DrawRect(screen, bx+bs, by, 1, bs+1, borderCol)
}
