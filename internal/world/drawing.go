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

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/resource"
	"github.com/mechanical-lich/mlge/task"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

var drawOp = &ebiten.DrawImageOptions{}

// emptySubImage is a 1×1 white pixel used as a texture source when drawing
// solid-color triangles (the vertex color provides the actual colour).
var emptyImage = func() *ebiten.Image {
	img := ebiten.NewImage(3, 3)
	img.Fill(color.White)
	return img
}()
var emptySubImage = emptyImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)

type pendingEntityDraw struct {
	entity         *ecs.Entity
	tX, tY         float64
	worldX, worldY int
}

var pendingEntities []pendingEntityDraw

func DrawLevel(level *Level, screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int, viewW, viewH int) {
	pendingEntities = pendingEntities[:0]

	disableLighting := config.Global().DebugDisableLighting
	disableLookdown := config.Global().DebugDisableLookdown

	screenX := 0
	for x := cameraX; x < cameraX+viewW; x++ {
		screenY := 0
		for y := cameraY; y < cameraY+viewH; y++ {
			tile := level.GetTilePtr(x, y, cameraZ)
			tX := float64(screenX * tileSizeW)
			tY := float64(screenY * tileSizeH)

			seen := disableLighting || level.GetSeen(x, y, cameraZ)
			visible := disableLighting || len(level.Visible) == 0 || level.GetVisible(x, y, cameraZ)

			// Never-seen tiles: skipped entirely. The world image is cleared to
			// black each frame so there's nothing to draw here.
			if !seen && !visible {
				screenY++
				continue
			}

			var brightness float32
			if !visible {
				// Seen but fogged: draw at ~37% brightness (equivalent to 160/255 black overlay).
				brightness = 1.0 - 160.0/255.0
			} else if disableLighting {
				brightness = 1.0
			} else {
				// Visible: derive brightness from the tile's light level.
				lightLevel := 0
				if tile != nil {
					if tileMiddleTransparent(tile) && tile.Floor.IsEmpty() {
						found := false
						for z := cameraZ - 1; z >= 0; z-- {
							below := level.GetTilePtr(x, y, z)
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
							lightLevel = level.EffectiveSunIntensity()
						}
					} else {
						lightLevel = tile.LightLevel
					}
				}
				brightness = float32(lightLevel) / 100.0
				if brightness > 1.0 {
					brightness = 1.0
				}
			}

			// Apply brightness as a ColorScale tint — no overlay image needed.
			drawOp.ColorScale.Reset()
			drawOp.ColorScale.Scale(brightness, brightness, brightness, 1.0)
			drawTile(screen, level, tile, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)

			level.entitiesBuffer = level.entitiesBuffer[:0]
			if visible {
				level.GetEntitiesAt(x, y, cameraZ, &level.entitiesBuffer)
			}

			// See through cells whose Middle and Floor are both effectively empty.
			// Depth fog is folded into each lookdown tile's ColorScale instead of
			// a separate overlay rect, so all tile draws stay in one batch.
			isTransparent := tile == nil || (tile.Floor.IsEmpty() && tileMiddleTransparent(tile))
			if isTransparent && !disableLookdown {
				for z := cameraZ - 1; z >= 0; z-- {
					below := level.GetTilePtr(x, y, z)
					if below == nil {
						break
					}
					depth := cameraZ - z
					depthAlpha := 15 + depth*18
					if depthAlpha > 180 {
						depthAlpha = 180
					}
					df := brightness * (1.0 - float32(depthAlpha)/255.0)
					drawOp.ColorScale.Reset()
					drawOp.ColorScale.Scale(df, df, df, 1.0)
					drawTile(screen, level, below, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
					if visible {
						level.GetEntitiesAt(x, y, z, &level.entitiesBuffer)
					}
					if !below.Floor.IsEmpty() || !tileMiddleTransparent(below) {
						break
					}
				}
			}

			if visible {
				for _, entity := range level.entitiesBuffer {
					pendingEntities = append(pendingEntities, pendingEntityDraw{entity: entity, tX: tX, tY: tY, worldX: x, worldY: y})
				}
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
		drawEntity(screen, p.entity, p.tX, p.tY, p.worldX, p.worldY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}

	// Third pass: emote bubbles above entities.
	DrawEmotes(screen, pendingEntities, tileSizeW, tileSizeH)

	// Fourth pass: shadow pass — entities at Z+1 cast an elliptical shadow on
	// the current camera level. Only drawn where there is no ceiling (floor at
	// aboveZ) and the tile below is currently visible.
	aboveZ := cameraZ + 1
	if aboveZ < level.GetDepth() {
		screenX = 0
		for x := cameraX; x < cameraX+viewW; x++ {
			screenY := 0
			for y := cameraY; y < cameraY+viewH; y++ {
				above := level.GetTilePtr(x, y, aboveZ)
				hasCeiling := above != nil && !above.Floor.IsEmpty()
				shadowVisible := len(level.Visible) == 0 || level.GetVisible(x, y, cameraZ)
				if !hasCeiling && shadowVisible {
					level.entitiesBuffer = level.entitiesBuffer[:0]
					level.GetEntitiesAt(x, y, aboveZ, &level.entitiesBuffer)
					if len(level.entitiesBuffer) > 0 {
						cx := float32(screenX*tileSizeW + tileSizeW/2)
						cy := float32(screenY*tileSizeH + tileSizeH/2)
						drawShadowEllipse(screen, cx, cy, float32(tileSizeW)*0.38, float32(tileSizeH)*0.22)
					}
				}
				screenY++
			}
			screenX++
		}
	}

	// Debug: draw pathfinding steps for all entities that have an AIMemory.
	if config.Global().RenderPathfindingSteps {
		for _, entity := range level.Entities {
			if !entity.HasComponent(rlcomponents.AIMemory) {
				continue
			}
			mem := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
			for i, stepIdx := range mem.CurrentSteps {
				t := level.Level.GetTilePtrIndex(stepIdx)
				if t == nil {
					continue
				}
				tx, ty, tz := t.Coords()
				if tz != cameraZ {
					continue
				}
				sx := tx - cameraX
				sy := ty - cameraY
				if sx < 0 || sy < 0 || sx >= viewW || sy >= viewH {
					continue
				}
				c := color.RGBA{0, 200, 255, 80}
				if i == 0 {
					c = color.RGBA{255, 200, 0, 120} // current position highlight
				}
				vector.DrawFilledRect(screen,
					float32(sx*tileSizeW), float32(sy*tileSizeH),
					float32(tileSizeW), float32(tileSizeH),
					c, false)
			}
		}
	}
}

// drawShadowEllipse draws a filled ellipse by building a path from arc segments
// scaled on the Y axis to produce an oval shadow shape.
func drawShadowEllipse(screen *ebiten.Image, cx, cy, rx, ry float32) {
	var path vector.Path
	const steps = 16
	for i := 0; i <= steps; i++ {
		angle := float32(i) * 2 * 3.14159265 / float32(steps)
		x := cx + rx*float32(math.Cos(float64(angle)))
		y := cy + ry*float32(math.Sin(float64(angle)))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	shadowColor := color.RGBA{0, 0, 0, 110}
	r := float32(shadowColor.R) / 255
	g := float32(shadowColor.G) / 255
	b := float32(shadowColor.B) / 255
	a := float32(shadowColor.A) / 255
	for i := range vs {
		vs[i].ColorR = r
		vs[i].ColorG = g
		vs[i].ColorB = b
		vs[i].ColorA = a
		vs[i].SrcX = 1
		vs[i].SrcY = 1
	}
	screen.DrawTriangles(vs, is, emptySubImage, &ebiten.DrawTrianglesOptions{FillRule: ebiten.FillRuleNonZero})
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
	// Floor → Middle. Each slot draws independently. Air/space
	// Middle slots are skipped — they're vision markers, not visuals, and
	// painting them clobbers tiles drawn through them by the z-lookdown.
	// (A cell's "ceiling" is the Floor of the cell above; it is drawn as that
	// cell's own Floor, not here.)
	if !tile.Floor.IsEmpty() {
		floorSlot := tile.Floor
		if TileDefinitions[floorSlot.Type].AutoTile > 0 && level != nil {
			floorSlot.Variant = resolveFloorAutotileVariant(level, tile)
		}
		drawSlot(screen, level, tile, floorSlot, false, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}
	// Air middles are always skipped — they're vision markers only.
	// Space middles are skipped only when the tile has a floor beneath them
	// (e.g. a hull floor built in open space); otherwise open space must render.
	spaceMiddle := !tile.Middle.IsEmpty() && TileDefinitions[tile.Middle.Type].Space
	skipMiddle := tile.Middle.IsEmpty() ||
		TileDefinitions[tile.Middle.Type].Air ||
		(spaceMiddle && !tile.Floor.IsEmpty())
	if !skipMiddle {
		drawSlot(screen, level, tile, tile.Middle, true, screenX, screenY, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}
}

// resolveFloorAutotileVariant computes an AutoTileBitmask variant index by
// checking which cardinal neighbors share the same floor tile type.
// top=1, bottom=2, left=4, right=8 — matches the Middle-slot bitmask convention.
func resolveFloorAutotileVariant(level *Level, tile *Tile) int {
	x, y, z := tile.Coords()
	tileType := tile.Floor.Type
	same := func(nx, ny int) bool {
		n := level.GetTilePtr(nx, ny, z)
		return n != nil && !n.Floor.IsEmpty() && n.Floor.Type == tileType
	}
	idx := 0
	if same(x, y-1) {
		idx |= 1
	}
	if same(x, y+1) {
		idx |= 2
	}
	if same(x-1, y) {
		idx |= 4
	}
	if same(x+1, y) {
		idx |= 8
	}
	def := TileDefinitions[tileType]
	for i := range def.Variants {
		if def.Variants[i].Variant == idx {
			return i
		}
	}
	return 0
}

// drawSlot renders one slot of a tile. autotileEligible is true only for the
// Middle slot (Floor doesn't autotile in the POC).
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
	drawOp.GeoM.Scale(float64(tileSizeW)/float64(srcW), float64(tileSizeH)/float64(srcH))
	drawOp.GeoM.Translate(float64(screenX*tileSizeW+def.SpriteOffsetX), float64(screenY*tileSizeH+def.SpriteOffsetY))
	screen.DrawImage(src, drawOp)
}

// DrawRadiationOverlay draws a tinted checkerboard sprite over tiles with nonzero
// Radiation. Two frames at (408,960) and (432,960) on the scifi_world sheet alternate
// at ~2 Hz to give a shimmering hazard look without a solid color wash.
func DrawRadiationOverlay(level *Level, screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, viewW, viewH int) {
	tex := resource.Textures["scifi_world"]
	if tex == nil {
		return
	}
	const spriteSize = 24
	frame := int(time.Now().UnixMilli()/500) % 2
	fx := 96 + frame*spriteSize
	src := tex.SubImage(image.Rect(fx, 960, fx+spriteSize, 960+spriteSize)).(*ebiten.Image)

	scaleX := float64(tileSizeW) / spriteSize
	scaleY := float64(tileSizeH) / spriteSize

	op := &ebiten.DrawImageOptions{}
	op.Blend = ebiten.BlendSourceOver
	for sx := 0; sx < viewW; sx++ {
		for sy := 0; sy < viewH; sy++ {
			tile := level.GetTilePtr(cameraX+sx, cameraY+sy, cameraZ)
			if tile == nil || tile.Radiation == 0 {
				continue
			}
			if !level.GetVisible(cameraX+sx, cameraY+sy, cameraZ) {
				continue
			}
			a := uint8(int(tile.Radiation) * 40 / 255) // max ~16% on checker pixels
			op.GeoM.Reset()
			op.GeoM.Scale(scaleX, scaleY)
			op.GeoM.Translate(float64(sx*tileSizeW), float64(sy*tileSizeH))
			op.ColorScale.Reset()
			op.ColorScale.ScaleWithColor(color.RGBA{120, 255, 120, a})
			screen.DrawImage(src, op)
		}
	}
}

func drawEntity(screen *ebiten.Image, entity *ecs.Entity, tX, tY float64, tileWorldX, tileWorldY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
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
		if ac.SpriteWidth > 0 {
			srcW = ac.SpriteWidth
		}
		if ac.SpriteHeight > 0 {
			srcH = ac.SpriteHeight
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
		if entity.HasComponent(rlcomponents.Size) && entity.HasComponent(rlcomponents.Position) {
			sc := entity.GetComponent(rlcomponents.Size).(*rlcomponents.SizeComponent)
			pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if sc.Width > 0 && sc.Height > 0 {
				startX := pc.GetX() - sc.Width/2
				startY := pc.GetY() - sc.Height/2
				subX := tileWorldX - startX
				subY := tileWorldY - startY
				// Per-tile slice is exactly one tile in screen pixels.
				// The full sprite spans sc.Width × sc.Height tiles.
				spriteX += subX * spriteSizeW
				spriteY += subY * spriteSizeH
				srcW = spriteSizeW
				srcH = spriteSizeH
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
