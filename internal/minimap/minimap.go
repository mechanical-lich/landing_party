package minimap

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/mlge/resource"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
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

func (m *Minimap) GenerateImageAtZ(z int) {
	cfg := config.Global()
	worldImage := ebiten.NewImage(m.Width, m.Height)
	scaleX := float64(m.Width) / float64(cfg.WorldGenSizeW)
	scaleY := float64(m.Height) / float64(cfg.WorldGenSizeH)
	op := &ebiten.DrawImageOptions{}

	for x := 0; x < cfg.WorldGenSizeW; x++ {
		for y := 0; y < cfg.WorldGenSizeH; y++ {
			tile := m.level.GetTilePtr(x, y, z)
			if tile == nil {
				continue
			}
			// Prefer Middle for color, fall back to Floor.
			slot := tile.Middle
			if slot.IsEmpty() {
				slot = tile.Floor
			}
			if slot.IsEmpty() {
				continue
			}
			def := world.TileDefinitions[slot.Type]
			if def.Air || def.Space {
				continue
			}
			if len(def.Variants) == 0 {
				continue
			}
			v := slot.Variant
			if v < 0 || v >= len(def.Variants) {
				v = 0
			}
			variant := def.Variants[v]
			img := resource.GetSubImage(def.Resource, variant.SpriteX, variant.SpriteY, cfg.SpriteSizeW, cfg.SpriteSizeH)
			if img == nil {
				continue
			}
			op.GeoM.Reset()
			op.GeoM.Scale(scaleX, scaleY)
			op.GeoM.Translate(float64(x)*scaleX, float64(y)*scaleY)
			worldImage.DrawImage(img, op)
		}
	}

	m.mu.Lock()
	m.images[z] = worldImage
	m.mu.Unlock()
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
