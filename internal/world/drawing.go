package world

import (
	"image"
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/resource"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
)

var drawOp = &ebiten.DrawImageOptions{}

func DrawLevel(level *Level, screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int, viewW, viewH int) {
	screenX := 0
	for x := cameraX; x < cameraX+viewW; x++ {
		screenY := 0
		for y := cameraY; y < cameraY+viewH; y++ {
			tile := level.GetTilePtr(x, y, cameraZ)
			drawTile(screen, tile, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)

			level.entitiesBuffer = level.entitiesBuffer[:0]
			level.GetEntitiesAt(x, y, cameraZ, &level.entitiesBuffer)

			// See through air and space tiles to draw entities on lower z-levels
			if tile != nil && (TileDefinitions[tile.Type].Air || TileDefinitions[tile.Type].Space) {
				_, _, tileZ := tile.Coords()
				for z := tileZ - 1; z >= 0; z-- {
					below := level.GetTilePtr(x, y, z)
					if below == nil {
						break
					}
					belowDef := TileDefinitions[below.Type]
					level.GetEntitiesAt(x, y, z, &level.entitiesBuffer)
					if !belowDef.Air && !belowDef.Space {
						break
					}
				}
			}

			tX := float64(screenX * tileSizeW)
			tY := float64(screenY * tileSizeH)
			for _, entity := range level.entitiesBuffer {
				drawEntity(screen, entity, tX, tY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
			}

			screenY++
		}
		screenX++
	}
}

var spaceTile = &Tile{}

func drawTile(screen *ebiten.Image, tile *Tile, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	if tile == nil {
		tile = spaceTile
	}
	def := TileDefinitions[tile.Type]
	if len(def.Variants) == 0 {
		return
	}

	v := tile.Variant
	if v < 0 || v >= len(def.Variants) {
		v = 0
	}
	variant := def.Variants[v]

	tex := resource.Textures[def.Resource]
	if tex == nil {
		return
	}

	srcW := spriteSizeW
	srcH := spriteSizeH
	if def.SpriteWidth > 0 {
		srcW = def.SpriteWidth
	}
	if def.SpriteHeight > 0 {
		srcH = def.SpriteHeight
	}

	src := tex.SubImage(image.Rect(variant.SpriteX, variant.SpriteY, variant.SpriteX+srcW, variant.SpriteY+srcH)).(*ebiten.Image)
	drawOp.GeoM.Reset()
	drawOp.ColorScale.Reset()
	drawOp.GeoM.Scale(float64(tileSizeW)/float64(srcW), float64(tileSizeH)/float64(srcH))
	drawOp.GeoM.Translate(float64(screenX*tileSizeW+def.SpriteOffsetX), float64(screenY*tileSizeH+def.SpriteOffsetY))
	screen.DrawImage(src, drawOp)
}

func drawEntity(screen *ebiten.Image, entity *ecs.Entity, tX, tY float64, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	if !entity.HasComponent(components.Appearance) {
		return
	}
	ac := entity.GetComponent(components.Appearance).(*components.AppearanceComponent)
	tex := resource.Textures[ac.Resource]
	if tex == nil {
		return
	}

	spriteX, spriteY := ac.SpriteX, ac.SpriteY
	if ac.Bounces && ac.Bounce {
		if ac.BounceAxis == "y" {
			spriteY += spriteSizeH
		} else {
			spriteX += spriteSizeW
		}
	}

	src := tex.SubImage(image.Rect(spriteX, spriteY, spriteX+spriteSizeW, spriteY+spriteSizeH)).(*ebiten.Image)
	drawOp.GeoM.Reset()
	drawOp.ColorScale.Reset()

	if entity.HasComponent(components.Selected) {
		drawOp.ColorScale.SetR(1.5)
		drawOp.ColorScale.SetG(1.5)
		drawOp.ColorScale.SetB(1.5)
	}

	if entity.HasComponent(rlcomponents.Dead) {
		drawOp.ColorScale.SetA(0.4)
	}

	drawOp.GeoM.Scale(float64(tileSizeW)/float64(spriteSizeW), float64(tileSizeH)/float64(spriteSizeH))
	drawOp.GeoM.Translate(tX, tY)
	screen.DrawImage(src, drawOp)
}
