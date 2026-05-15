package minimap

import (
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// Minimap flat colors — chosen for readability at small scale.
var (
	colUnseen  = color.RGBA{0, 0, 0, 255}
	colGhost   = color.RGBA{30, 35, 40, 255}   // seen tile that exists but not currently visible
	colSolid   = color.RGBA{70, 75, 85, 255}   // rock / wall
	colFloor   = color.RGBA{140, 148, 160, 255} // walkable floor
	colWater   = color.RGBA{40, 90, 160, 255}   // water
	colSpace   = color.RGBA{5, 5, 12, 255}      // space / vacuum
)

type Minimap struct {
	mu            sync.Mutex
	Width, Height int
	level         *world.Level
	images        map[int]*ebiten.Image
}

func NewMinimap(level *world.Level, width, height int) *Minimap {
	return &Minimap{
		Width:  width,
		Height: height,
		level:  level,
		images: make(map[int]*ebiten.Image),
	}
}

func (m *Minimap) InvalidateAll() {
	m.mu.Lock()
	m.images = make(map[int]*ebiten.Image)
	m.mu.Unlock()
}

func (m *Minimap) InvalidateZ(z int) {
	m.mu.Lock()
	delete(m.images, z)
	m.mu.Unlock()
}

func (m *Minimap) GenerateImageAtZ(z int) {
	cfg := config.Global()
	img := ebiten.NewImage(m.Width, m.Height)
	m.drawRegion(img, z, 0, 0, cfg.WorldGenSizeW, cfg.WorldGenSizeH)
	m.mu.Lock()
	m.images[z] = img
	m.mu.Unlock()
}

// InvalidatePartial redraws a rectangular region of tiles into the existing
// image for layer z without regenerating the whole map. If no image exists yet
// for z, falls back to a full generation.
func (m *Minimap) InvalidatePartial(z, tileX, tileY, tileW, tileH int) {
	m.mu.Lock()
	img := m.images[z]
	m.mu.Unlock()

	if img == nil {
		m.GenerateImageAtZ(z)
		return
	}

	cfg := config.Global()
	if tileX < 0 {
		tileX = 0
	}
	if tileY < 0 {
		tileY = 0
	}
	if tileX+tileW > cfg.WorldGenSizeW {
		tileW = cfg.WorldGenSizeW - tileX
	}
	if tileY+tileH > cfg.WorldGenSizeH {
		tileH = cfg.WorldGenSizeH - tileY
	}
	m.drawRegion(img, z, tileX, tileY, tileW, tileH)
}

func (m *Minimap) drawRegion(img *ebiten.Image, z, x0, y0, w, h int) {
	cfg := config.Global()
	scaleX := float64(m.Width) / float64(cfg.WorldGenSizeW)
	scaleY := float64(m.Height) / float64(cfg.WorldGenSizeH)

	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			px := float64(x) * scaleX
			py := float64(y) * scaleY

			tile := m.level.GetTilePtr(x, y, z)
			seen := m.level.GetSeen(x, y, z)

			var c color.RGBA
			if tile == nil || !seen {
				if tile == nil {
					c = colUnseen
				} else {
					c = colGhost
				}
			} else {
				c = tileColor(tile)
			}

			ebitenutil.DrawRect(img, px, py, scaleX, scaleY, c)
		}
	}
}

// tileColor picks a flat color for a seen tile based on its category.
func tileColor(tile *world.Tile) color.RGBA {
	// Check Middle slot first (walls, ores, etc.), then Floor.
	slot := tile.Middle
	if slot.IsEmpty() {
		slot = tile.Floor
	}
	if slot.IsEmpty() {
		return colUnseen
	}
	def := world.TileDefinitions[slot.Type]
	switch {
	case def.Space:
		return colSpace
	case def.Air:
		return colUnseen
	case def.Water:
		return colWater
	case def.Solid:
		return colSolid
	default:
		return colFloor
	}
}

func (m *Minimap) GetImage(z int) *ebiten.Image {
	m.mu.Lock()
	img := m.images[z]
	m.mu.Unlock()
	if img == nil {
		m.GenerateImageAtZ(z)
		m.mu.Lock()
		img = m.images[z]
		m.mu.Unlock()
	}
	return img
}
