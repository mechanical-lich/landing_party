package minimap

import (
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// Minimap flat colors — chosen for readability at small scale.
var (
	colUnseen = color.RGBA{0, 0, 0, 255}
	colGhost  = color.RGBA{30, 35, 40, 255}    // seen tile that exists but not currently visible
	colSolid  = color.RGBA{70, 75, 85, 255}    // rock / wall
	colFloor  = color.RGBA{140, 148, 160, 255} // walkable floor
	colWater  = color.RGBA{40, 90, 160, 255}   // water
	colSpace  = color.RGBA{5, 5, 12, 255}      // space / vacuum
)

type Minimap struct {
	mu            sync.Mutex
	Width, Height int
	level         *world.Level
	images        map[int]*ebiten.Image
	buffers       map[int][]byte
}

func NewMinimap(level *world.Level, width, height int) *Minimap {
	return &Minimap{
		Width:   width,
		Height:  height,
		level:   level,
		images:  make(map[int]*ebiten.Image),
		buffers: make(map[int][]byte),
	}
}

func (m *Minimap) InvalidateAll() {
	m.mu.Lock()
	m.images = make(map[int]*ebiten.Image)
	m.buffers = make(map[int][]byte)
	m.mu.Unlock()
}

func (m *Minimap) InvalidateZ(z int) {
	m.mu.Lock()
	delete(m.images, z)
	delete(m.buffers, z)
	m.mu.Unlock()
}

func (m *Minimap) GenerateImageAtZ(z int) {
	m.mu.Lock()
	img := m.images[z]
	if img == nil {
		img = ebiten.NewImage(m.Width, m.Height)
		m.images[z] = img
	}
	if m.buffers[z] == nil {
		m.buffers[z] = make([]byte, m.Width*m.Height*4)
	}
	m.mu.Unlock()
	m.drawRegion(img, z, 0, 0, m.level.GetWidth(), m.level.GetHeight())
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

	worldW, worldH := m.level.GetWidth(), m.level.GetHeight()
	if tileX < 0 {
		tileX = 0
	}
	if tileY < 0 {
		tileY = 0
	}
	if tileX+tileW > worldW {
		tileW = worldW - tileX
	}
	if tileY+tileH > worldH {
		tileH = worldH - tileY
	}
	m.drawRegion(img, z, tileX, tileY, tileW, tileH)
}

// drawRegion recomputes the minimap pixels covering tile rect
// [x0,x0+w)×[y0,y0+h) into the per-Z CPU buffer, then uploads the whole
// buffer to the image with a single WritePixels (one GPU op instead of one
// DrawRect per tile).
func (m *Minimap) drawRegion(img *ebiten.Image, z, x0, y0, w, h int) {
	worldW := m.level.GetWidth()
	worldH := m.level.GetHeight()
	scaleX := float64(m.Width) / float64(worldW)
	scaleY := float64(m.Height) / float64(worldH)

	m.mu.Lock()
	buf := m.buffers[z]
	if buf == nil {
		buf = make([]byte, m.Width*m.Height*4)
		m.buffers[z] = buf
	}
	m.mu.Unlock()

	// Pixel rectangle covering the requested tile region.
	px0 := int(float64(x0) * scaleX)
	px1 := int(float64(x0+w)*scaleX) + 1
	py0 := int(float64(y0) * scaleY)
	py1 := int(float64(y0+h)*scaleY) + 1
	if px0 < 0 {
		px0 = 0
	}
	if py0 < 0 {
		py0 = 0
	}
	if px1 > m.Width {
		px1 = m.Width
	}
	if py1 > m.Height {
		py1 = m.Height
	}

	for py := py0; py < py1; py++ {
		ty := int(float64(py) / scaleY)
		if ty >= worldH {
			ty = worldH - 1
		}
		rowBase := py * m.Width
		for px := px0; px < px1; px++ {
			tx := int(float64(px) / scaleX)
			if tx >= worldW {
				tx = worldW - 1
			}

			tile := m.level.GetTilePtr(tx, ty, z)
			seen := m.level.GetSeen(tx, ty, z)

			var c color.RGBA
			if tile == nil {
				c = colUnseen
			} else if !seen {
				c = colGhost
			} else {
				c = tileColor(tile)
			}

			idx := (rowBase + px) * 4
			buf[idx+0] = c.R
			buf[idx+1] = c.G
			buf[idx+2] = c.B
			buf[idx+3] = c.A
		}
	}

	img.WritePixels(buf)
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
