package effect

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/mlge/resource"
)

type ArrowEffect struct {
	X, Y, Z                   float64
	TargetX, TargetY, TargetZ float64
	Resource                  string
	SX, SY                    int
	op                        ebiten.DrawImageOptions
}

func NewArrowEffect(x, y, z, targetX, targetY, targetZ int, res string, sx, sy int) *ArrowEffect {
	return &ArrowEffect{
		X: float64(x), Y: float64(y), Z: float64(z),
		TargetX: float64(targetX), TargetY: float64(targetY), TargetZ: float64(targetZ),
		Resource: res, SX: sx, SY: sy,
	}
}

func (ae *ArrowEffect) Update() {
	dx := ae.TargetX - ae.X
	dy := ae.TargetY - ae.Y
	dz := ae.TargetZ - ae.Z
	ae.X += dx * 0.1
	ae.Y += dy * 0.1
	ae.Z += dz * 0.1
	if dx*dx+dy*dy+dz*dz < 1 {
		GetEffectManager().RemoveEffect(ae)
	}
}

func (ae *ArrowEffect) Draw(screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	cfg := config.Global()
	frame := ae.getFrame()
	img := resource.GetSubImage(ae.Resource, ae.SX+frame*cfg.SpriteSizeW, ae.SY, cfg.SpriteSizeW, cfg.SpriteSizeH)
	screenX := (ae.X - float64(cameraX)) * float64(tileSizeW)
	screenY := (ae.Y - float64(cameraY)) * float64(tileSizeH)
	ae.op.GeoM.Reset()
	ae.op.GeoM.Scale(float64(tileSizeW)/float64(spriteSizeW), float64(tileSizeH)/float64(spriteSizeH))
	ae.op.GeoM.Translate(screenX, screenY)
	screen.DrawImage(img, &ae.op)
}

func (ae *ArrowEffect) getFrame() int {
	dx := ae.TargetX - ae.X
	dy := ae.TargetY - ae.Y
	angle := math.Atan2(dy, dx)
	if angle < 0 {
		angle += 2 * math.Pi
	}
	return int((angle/(math.Pi/4))+0.5) % 8
}
