package world

import (
	"image"
	"image/color"
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
			drawTile(screen, level, tile, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)

			level.entitiesBuffer = level.entitiesBuffer[:0]
			level.GetEntitiesAt(x, y, cameraZ, &level.entitiesBuffer)

			// See through air and space tiles: draw tiles and entities on lower z-levels
			drawnZ := cameraZ
			isTransparent := tile == nil || TileDefinitions[tile.Type].Air || TileDefinitions[tile.Type].Space
			if isTransparent {
				startZ := cameraZ - 1
				if tile != nil {
					_, _, startZ = tile.Coords()
					startZ--
				}
				for z := startZ; z >= 0; z-- {
					below := level.GetTilePtr(x, y, z)
					if below == nil {
						break
					}
					belowDef := TileDefinitions[below.Type]
					drawTile(screen, level, below, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
					level.GetEntitiesAt(x, y, z, &level.entitiesBuffer)
					if !belowDef.Air && !belowDef.Space {
						drawnZ = z
						break
					}
				}
			}

			// Depth fog: darken tiles seen through air layers
			depth := cameraZ - drawnZ
			tX := float64(screenX * tileSizeW)
			tY := float64(screenY * tileSizeH)
			if depth > 0 {
				alpha := 40 + depth*35
				if alpha > 210 {
					alpha = 210
				}
				vector.DrawFilledRect(screen, float32(tX), float32(tY), float32(tileSizeW), float32(tileSizeH), color.RGBA{0, 0, 0, uint8(alpha)}, false)
			}

			for _, entity := range level.entitiesBuffer {
				drawEntity(screen, entity, tX, tY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
			}

			screenY++
		}
		screenX++
	}
}

var spaceTile = &Tile{}

func drawTile(screen *ebiten.Image, level *Level, tile *Tile, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	if tile == nil {
		tile = spaceTile
	}
	def := TileDefinitions[tile.Type]
	if len(def.Variants) == 0 {
		return
	}

	var variant TileVariant
	if def.AutoTile > 0 && level != nil {
		variant = level.ResolveVariant(tile)
	} else {
		v := tile.Variant
		if v < 0 || v >= len(def.Variants) {
			v = 0
		}
		variant = def.Variants[v]
	}

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

var lightOverlayImg *ebiten.Image
var lightOverlayPixels []byte
var lightOverlayW, lightOverlayH int

func DrawLightOverlay(level *Level, screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, viewW, viewH int) {
	imgW := viewW * tileSizeW
	imgH := viewH * tileSizeH

	if lightOverlayImg == nil || lightOverlayW != imgW || lightOverlayH != imgH {
		lightOverlayImg = ebiten.NewImage(imgW, imgH)
		lightOverlayPixels = make([]byte, imgW*imgH*4)
		lightOverlayW = imgW
		lightOverlayH = imgH
	}

	// Clear to transparent
	for i := range lightOverlayPixels {
		lightOverlayPixels[i] = 0
	}

	for sx := 0; sx < viewW; sx++ {
		for sy := 0; sy < viewH; sy++ {
			tile := level.GetTilePtr(cameraX+sx, cameraY+sy, cameraZ)
			lightLevel := 0
			if tile != nil {
				def := TileDefinitions[tile.Type]
				if def.Air || def.Space {
					// Look through transparent layers to find the lit tile below
					for z := cameraZ - 1; z >= 0; z-- {
						below := level.GetTilePtr(cameraX+sx, cameraY+sy, z)
						if below == nil {
							break
						}
						belowDef := TileDefinitions[below.Type]
						if !belowDef.Air && !belowDef.Space {
							lightLevel = below.LightLevel
							break
						}
					}
				} else {
					lightLevel = tile.LightLevel
				}
			}
			if lightLevel >= 100 {
				continue
			}
			alpha := byte(255 - lightLevel*255/100)
			baseX := sx * tileSizeW
			baseY := sy * tileSizeH
			for py := 0; py < tileSizeH; py++ {
				row := (baseY+py)*imgW + baseX
				for px := 0; px < tileSizeW; px++ {
					lightOverlayPixels[(row+px)*4+3] = alpha
				}
			}
		}
	}

	lightOverlayImg.WritePixels(lightOverlayPixels)
	screen.DrawImage(lightOverlayImg, nil)
}

func drawEntity(screen *ebiten.Image, entity *ecs.Entity, tX, tY float64, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	var resourceName string
	var spriteX, spriteY, srcW, srcH int

	var colorR, colorG, colorB uint8 = 255, 255, 255

	switch {
	case entity.HasComponent(components.EquipmentAppearance):
		eac := entity.GetComponent(components.EquipmentAppearance).(*components.EquipmentAppearanceComponent)
		size := eac.SpriteSize
		if size <= 0 {
			size = spriteSizeW
		}
		srcW, srcH = size, size
		resourceName = eac.Resource
		spriteX, spriteY = eac.ResolveSprite(entity, false)

	case entity.HasComponent(components.Appearance):
		ac := entity.GetComponent(components.Appearance).(*components.AppearanceComponent)
		srcW, srcH = spriteSizeW, spriteSizeH
		if ac.SpriteSize > 0 {
			srcW = ac.SpriteSize
			srcH = ac.SpriteSize
		}
		resourceName = ac.Resource
		colorR, colorG, colorB = ac.R, ac.G, ac.B
		spriteX, spriteY = ac.SpriteX, ac.SpriteY
		if entity.HasComponent(rlcomponents.Door) {
			door := entity.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
			if door.Open && (door.OpenedSpriteX != 0 || door.OpenedSpriteY != 0) {
				spriteX, spriteY = door.OpenedSpriteX, door.OpenedSpriteY
			}
		}
		if ac.Bounces && ac.Bounce {
			if ac.BounceAxis == "y" {
				spriteY += srcH
			} else {
				spriteX += srcW
			}
		}

	default:
		return
	}

	tex := resource.Textures[resourceName]
	if tex == nil {
		return
	}

	src := tex.SubImage(image.Rect(spriteX, spriteY, spriteX+srcW, spriteY+srcH)).(*ebiten.Image)
	drawOp.GeoM.Reset()
	drawOp.ColorScale.Reset()
	r := float32(colorR) / 255.0
	g := float32(colorG) / 255.0
	b := float32(colorB) / 255.0

	// Darken entities on lower z-levels
	if entity.HasComponent(rlcomponents.Position) {
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		depth := cameraZ - pc.GetZ()
		if depth > 0 {
			tint := 1.0 - float32(depth)*0.2
			if tint < 0.1 {
				tint = 0.1
			}
			r *= tint
			g *= tint
			b *= tint
		}
	}
	drawOp.ColorScale.SetR(r)
	drawOp.ColorScale.SetG(g)
	drawOp.ColorScale.SetB(b)

	if entity.HasComponent(components.Selected) {
		drawOp.ColorScale.SetR(1.5)
		drawOp.ColorScale.SetG(1.5)
		drawOp.ColorScale.SetB(1.5)
	}

	if entity.HasComponent(rlcomponents.Dead) {
		drawOp.ColorScale.SetA(0.4)
	}

	// Scale sprite to its natural tile size, then center within the tile
	scale := float64(tileSizeW) / float64(srcW)
	if float64(tileSizeH)/float64(srcH) < scale {
		scale = float64(tileSizeH) / float64(srcH)
	}
	drawW := float64(srcW) * scale
	drawH := float64(srcH) * scale
	offsetX := (float64(tileSizeW) - drawW) / 2
	offsetY := (float64(tileSizeH) - drawH) / 2
	drawOp.GeoM.Scale(scale, scale)
	drawOp.GeoM.Translate(tX+offsetX, tY+offsetY)
	screen.DrawImage(src, drawOp)
}
