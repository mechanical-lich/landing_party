package world

import (
	"image"
	"image/color"
	_ "image/png"
	"math"
	"time"

	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/resource"
	"github.com/mechanical-lich/mlge/task"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/task_requests"
)

var drawOp = &ebiten.DrawImageOptions{}

type pendingEntityDraw struct {
	entity *ecs.Entity
	tX, tY float64
}

var pendingEntities []pendingEntityDraw

func DrawLevel(level *Level, screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int, viewW, viewH int) {
	pendingEntities = pendingEntities[:0]
	screenX := 0
	for x := cameraX; x < cameraX+viewW; x++ {
		screenY := 0
		for y := cameraY; y < cameraY+viewH; y++ {
			tile := level.GetTilePtr(x, y, cameraZ)
			drawTile(screen, level, tile, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)

			level.entitiesBuffer = level.entitiesBuffer[:0]
			level.GetEntitiesAt(x, y, cameraZ, &level.entitiesBuffer)

			// See through cells whose Middle and Floor are both effectively empty
			// — i.e. nothing opaque at this z. Layered cells block lookdown if
			// they have a Floor (a ground surface) or a non-air Middle.
			drawnZ := cameraZ
			isTransparent := tile == nil || (tile.Floor.IsEmpty() && tileMiddleTransparent(tile))
			if isTransparent {
				for z := cameraZ - 1; z >= 0; z-- {
					below := level.GetTilePtr(x, y, z)
					if below == nil {
						break
					}
					drawTile(screen, level, below, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
					level.GetEntitiesAt(x, y, z, &level.entitiesBuffer)
					if !below.Floor.IsEmpty() || !tileMiddleTransparent(below) {
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
				alpha := 15 + depth*18
				if alpha > 180 {
					alpha = 180
				}
				vector.DrawFilledRect(screen, float32(tX), float32(tY), float32(tileSizeW), float32(tileSizeH), color.RGBA{0, 0, 0, uint8(alpha)}, false)
			}

			for _, entity := range level.entitiesBuffer {
				pendingEntities = append(pendingEntities, pendingEntityDraw{entity: entity, tX: tX, tY: tY})
			}

			// Debug overlay: print the blob47 pruned mask on each cell whose
			// Middle uses AutoTileBlob47. Useful for aligning a tilesheet.
			if config.Global().DebugShowAutotileMask && tile != nil && !tile.Middle.IsEmpty() {
				def := TileDefinitions[tile.Middle.Type]
				if def.AutoTile == rllayered.AutoTileBlob47 {
					mask := blob47MaskFor(level, tile)
					mlge_text.Draw(screen, fmt.Sprintf("%d", mask), 10, int(tX)+1, int(tY)+1, color.RGBA{255, 255, 0, 255})
				}
			}

			screenY++
		}
		screenX++
	}

	// Second pass: draw all entities on top of every tile so an entity with a
	// movement offset never gets clipped by tiles drawn after it.
	for _, p := range pendingEntities {
		drawEntity(screen, p.entity, p.tX, p.tY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}
}

// tileMiddleTransparent reports whether the cell's Middle slot lets light /
// vision through to lower z-levels. Empty Middle, "air", and "space" all
// count.
func tileMiddleTransparent(t *Tile) bool {
	if t == nil || t.Middle.IsEmpty() {
		return true
	}
	def := TileDefinitions[t.Middle.Type]
	return def.Air || def.Space
}

// outOfBoundsSlot is a synthetic Slot pointing at the "space" tile, used to
// render cells outside the level bounds (so the camera doesn't smear last
// frame's pixels at the edges).
var outOfBoundsSlot = func() rllayered.Slot {
	if idx, ok := TileNameToIndex["space"]; ok {
		return rllayered.Slot{Type: idx, Variant: 0}
	}
	return rllayered.Slot{}
}

func drawTile(screen *ebiten.Image, level *Level, tile *Tile, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	if tile == nil {
		slot := outOfBoundsSlot()
		if !slot.IsEmpty() {
			drawSlot(screen, level, tile, slot, false, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
		}
		return
	}
	// Floor → Middle → Ceiling. Each slot draws independently. Air/space
	// Middle slots are skipped — they're vision markers, not visuals, and
	// painting them clobbers tiles drawn through them by the z-lookdown.
	if !tile.Floor.IsEmpty() {
		drawSlot(screen, level, tile, tile.Floor, false, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}
	if !tile.Middle.IsEmpty() && !TileDefinitions[tile.Middle.Type].Air {
		drawSlot(screen, level, tile, tile.Middle, true, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}
	if !tile.Ceiling.IsEmpty() {
		drawSlot(screen, level, tile, tile.Ceiling, false, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}
}

// drawSlot renders one slot of a tile. autotileEligible is true only for the
// Middle slot (Floor/Ceiling don't autotile in the POC).
func drawSlot(screen *ebiten.Image, level *Level, tile *Tile, slot rllayered.Slot, autotileEligible bool, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	def := TileDefinitions[slot.Type]
	if len(def.Variants) == 0 {
		return
	}

	var variant TileVariant
	if autotileEligible && def.AutoTile > 0 && level != nil {
		variant = level.ResolveVariant(tile)
	} else {
		v := slot.Variant
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
	if config.Global().DebugDisableLighting {
		return
	}
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
				if tileMiddleTransparent(tile) && tile.Floor.IsEmpty() {
					// Look through transparent layers to find the first opaque
					// tile and sample its LightLevel (set by LightingSystem).
					found := false
					for z := cameraZ - 1; z >= 0; z-- {
						below := level.GetTilePtr(cameraX+sx, cameraY+sy, z)
						if below == nil {
							break
						}
						if !below.Floor.IsEmpty() || !tileMiddleTransparent(below) {
							lightLevel = below.LightLevel
							found = true
							break
						}

					}
					if !found {
						// Pure-space column (no solid below). Use ambient sun
						// so entities standing in vacuum aren't covered by an
						// opaque black overlay.
						lightLevel = level.EffectiveSunIntensity()
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

// DrawRadiationOverlay paints a green tint over tiles with nonzero Radiation.
// Intensity scales with the tile's Radiation byte (0..255).
func DrawRadiationOverlay(level *Level, screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, viewW, viewH int) {
	for sx := 0; sx < viewW; sx++ {
		for sy := 0; sy < viewH; sy++ {
			tile := level.GetTilePtr(cameraX+sx, cameraY+sy, cameraZ)
			if tile == nil || tile.Radiation == 0 {
				continue
			}
			// Cap alpha low so colonists/items on radioactive tiles stay readable.
			alpha := uint8(int(tile.Radiation) * 70 / 255)
			tint := color.RGBA{60, 220, 80, alpha}
			vector.DrawFilledRect(screen,
				float32(sx*tileSizeW), float32(sy*tileSizeH),
				float32(tileSizeW), float32(tileSizeH),
				tint, false,
			)
		}
	}
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
		bounce := false
		if entity.HasComponent(components.Appearance) {
			bounce = entity.GetComponent(components.Appearance).(*components.AppearanceComponent).Bounce
		}
		spriteX, spriteY = eac.ResolveSprite(entity, bounce)

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
	wx, wy := workingOffset(entity, float64(tileSizeW), float64(tileSizeH))
	drawOp.GeoM.Scale(scale, scale)
	drawOp.GeoM.Translate(tX+offsetX+wx, tY+offsetY+wy)
	screen.DrawImage(src, drawOp)
}

// workingOffset returns a small in-place sway toward whatever the entity is
// currently interacting with — a worker's task target (research/craft/pickup),
// a dropoff destination, or a hostile AI's tracked prey. Returns (0, 0) when
// the entity isn't engaged or isn't adjacent to its target yet.
func workingOffset(entity *ecs.Entity, tileW, tileH float64) (float64, float64) {
	// Fast-path: walls, items, doors, etc. never animate — bail before doing
	// any of the per-target component lookups below. This runs every frame on
	// every visible entity, so the early exit matters for FPS.
	if !entity.HasComponent(components.Worker) &&
		!entity.HasComponent(rlcomponents.HostileAI) &&
		!entity.HasComponent(components.FactionAI) {
		return 0, 0
	}
	if !entity.HasComponent(rlcomponents.Position) {
		return 0, 0
	}
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	tx, ty, tz, ok := interactionTarget(entity)
	if !ok {
		return 0, 0
	}
	dx := tx - pc.GetX()
	dy := ty - pc.GetY()
	dz := tz - pc.GetZ()
	if dz != 0 || dx < -1 || dx > 1 || dy < -1 || dy > 1 || (dx == 0 && dy == 0) {
		return 0, 0
	}
	phase := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())*math.Pi/500.0)
	const maxNudge = 0.25
	return float64(dx) * tileW * maxNudge * phase, float64(dy) * tileH * maxNudge * phase
}

// isInteractAction returns true for tasks that animate as on-target work.
// Plain "move to" orders have an empty action and are excluded.
func isInteractAction(a task.TaskAction) bool {
	switch a {
	case task_requests.PickupAction,
		task_requests.BuildAction,
		task_requests.DigAction,
		task_requests.MineAction,
		task_requests.ForageAction,
		task_requests.AttackAction,
		task_requests.ResearchAction,
		task_requests.CraftAction:
		return true
	}
	return false
}

func interactionTarget(entity *ecs.Entity) (x, y, z int, ok bool) {
	// Worker task (research, craft, pickup, build, attack, ...).
	// Plain "go here" orders have an empty Action and should NOT animate as
	// interactions — only known interact actions do.
	if entity.HasComponent(components.Worker) {
		wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed && isInteractAction(wc.CurrentTask.Action) {
			// If the task is targeted at an entity (e.g. attack), prefer that
			// entity's current position so a moving target still pulls a lean.
			if target, ok := wc.CurrentTask.Data.(*ecs.Entity); ok && target != nil &&
				target.HasComponent(rlcomponents.Position) && !target.HasComponent(rlcomponents.Dead) {
				tpc := target.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				return tpc.GetX(), tpc.GetY(), tpc.GetZ(), true
			}
			return wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, true
		}
	}
	// Dropoff is AI-state driven, not task driven.
	if entity.HasComponent(rlcomponents.AIMemory) {
		mem := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
		if mem.State == "dropoff" {
			return mem.TargetX, mem.TargetY, mem.TargetZ, true
		}
	}
	// Hostile AI mobs track their prey on HostileAIComponent.TargetX/Y.
	if entity.HasComponent(rlcomponents.HostileAI) {
		hc := entity.GetComponent(rlcomponents.HostileAI).(*rlcomponents.HostileAIComponent)
		if hc.TargetX != 0 || hc.TargetY != 0 {
			pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			return hc.TargetX, hc.TargetY, pc.GetZ(), true
		}
	}
	// Faction AI mobs (aliens et al) track their prey on FactionAIComponent.
	if entity.HasComponent(components.FactionAI) {
		fac := entity.GetComponent(components.FactionAI).(*components.FactionAIComponent)
		if fac.HasTarget {
			return fac.TargetX, fac.TargetY, fac.TargetZ, true
		}
	}
	return 0, 0, 0, false
}
