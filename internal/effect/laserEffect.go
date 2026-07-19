package effect

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mechanical-lich/landing_party/internal/view"
)

const laserFadeFrames = 12

type LaserEffect struct {
	x0, y0      float64 // attacker tile position
	x1, y1      float64 // beam end tile position
	r, g, b     uint8
	frame       int
	totalFrames int
}

func NewLaserEffect(srcX, srcY, targetX, targetY int, extraLength int, r, g, b uint8) *LaserEffect {
	// Extend beam past target by extraLength tiles
	dx := float64(targetX - srcX)
	dy := float64(targetY - srcY)
	length := math.Sqrt(dx*dx + dy*dy)
	var endX, endY float64
	if length > 0 {
		endX = float64(targetX) + dx/length*float64(extraLength)
		endY = float64(targetY) + dy/length*float64(extraLength)
	} else {
		endX = float64(targetX)
		endY = float64(targetY)
	}
	return &LaserEffect{
		x0: float64(srcX), y0: float64(srcY),
		x1: endX, y1: endY,
		r: r, g: g, b: b,
		totalFrames: laserFadeFrames,
	}
}

func (le *LaserEffect) Update() {
	le.frame++
	if le.frame >= le.totalFrames {
		GetEffectManager().RemoveEffect(le)
	}
}

func (le *LaserEffect) Draw(screen *ebiten.Image, cam view.Camera) {
	alpha := float32(1.0 - float32(le.frame)/float32(le.totalFrames))
	col := color.RGBA{le.r, le.g, le.b, uint8(alpha * 255)}

	half := float32(cam.TileW) / 2.0
	fx0, fy0 := cam.WorldToScreenFloat(le.x0, le.y0)
	fx1, fy1 := cam.WorldToScreenFloat(le.x1, le.y1)
	sx0, sy0 := float32(fx0)+half, float32(fy0)+half
	sx1, sy1 := float32(fx1)+half, float32(fy1)+half

	vector.StrokeLine(screen, sx0, sy0, sx1, sy1, 2, col, false)
}
