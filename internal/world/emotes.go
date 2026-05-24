package world

import (
	"image"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/emotes"
	"github.com/mechanical-lich/mlge/resource"
)

const bouncePeriodMs = 600

var emoteOp = &ebiten.DrawImageOptions{}

// DrawEmotes renders speech-bubble emotes above any entity in pending that has
// an active EmoteComponent. Call after the entity draw pass.
func DrawEmotes(screen *ebiten.Image, pending []pendingEntityDraw, tileSizeW, tileSizeH int) {
	tex := resource.Textures["emotes"]
	if tex == nil {
		return
	}

	nowMs := time.Now().UnixMilli()
	bounceAmp := float64(tileSizeH) * 0.08
	bounceY := math.Sin(float64(nowMs)*math.Pi*2/bouncePeriodMs) * bounceAmp

	scale := float64(tileSizeW) / emotes.SpriteSizePx
	drawW := emotes.SpriteSizePx * scale
	drawH := emotes.SpriteSizePx * scale

	for _, p := range pending {
		if !p.entity.HasComponent(components.Emote) {
			continue
		}
		ec := p.entity.GetComponent(components.Emote).(*components.EmoteComponent)
		if ec.Active == "" || ec.Alpha <= 0 {
			continue
		}
		coords, ok := emotes.Sprite(ec.Active)
		if !ok {
			continue
		}

		src := tex.SubImage(image.Rect(
			coords.X, coords.Y,
			coords.X+emotes.SpriteSizePx, coords.Y+emotes.SpriteSizePx,
		)).(*ebiten.Image)

		x := p.tX + float64(tileSizeW)/2 - drawW/2
		y := p.tY - drawH + bounceY

		emoteOp.GeoM.Reset()
		emoteOp.ColorScale.Reset()
		emoteOp.GeoM.Scale(scale, scale)
		emoteOp.GeoM.Translate(x, y)
		emoteOp.ColorScale.SetA(ec.Alpha)
		screen.DrawImage(src, emoteOp)
	}
}
