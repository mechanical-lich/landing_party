package systems

import (
	"slices"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/view"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

const DefaultSightRadius = 12

// FOVSystem runs once per system-update (not per entity turn) and rebuilds the
// colony's shared field of view from every worker's position. Tiles with LOS
// are marked visible; all tiles are permanently marked seen once visible.
//
// Viewport is the on-screen rect used to bound the per-pass Visible clear; the
// game state sets it before running systems. A zero Viewport clears nothing,
// so callers that run FOV must keep it current.
type FOVSystem struct {
	Viewport view.Viewport

	// blocked remembers blocksLOS for the cells one pass has looked at:
	// nothing a pass does changes what blocks sight, and the workers of a
	// colony stand close together, so their rays cross the same cells many
	// times over. origins is the worker positions the pass has already
	// cast from (several workers on one tile see the same).
	blocked losMemo
	origins []fovOrigin
}

type fovOrigin struct{ x, y, z, radius int }

// losMemo is blocksLOS by cell, each z plane filled as it is first asked:
// 0 not asked, 1 clear, 2 blocked.
type losMemo struct {
	width, height int
	planes        map[int][]uint8
}

// reset empties the memo for a pass over level.
func (m *losMemo) reset(level *world.Level) {
	if m.planes == nil || m.width != level.Width || m.height != level.Height {
		m.width, m.height = level.Width, level.Height
		m.planes = map[int][]uint8{}
		return
	}
	for _, plane := range m.planes {
		clear(plane)
	}
}

func (m *losMemo) blocks(level *world.Level, x, y, z int) bool {
	if x < 0 || y < 0 || x >= m.width || y >= m.height {
		return blocksLOS(level, x, y, z)
	}
	plane := m.planes[z]
	if plane == nil {
		plane = make([]uint8, m.width*m.height)
		m.planes[z] = plane
	}
	i := y*m.width + x
	if plane[i] == 0 {
		plane[i] = 1
		if blocksLOS(level, x, y, z) {
			plane[i] = 2
		}
	}
	return plane[i] == 2
}

// Requires returns no required components — UpdateSystem handles everything.
func (s *FOVSystem) Requires() []ecs.ComponentType { return nil }

func (s *FOVSystem) UpdateSystem(data interface{}) error {
	level := data.(*world.Level)
	level.ClearVisibleViewport(s.Viewport, level.GetDepth()-1)

	// Reset the worker Z set each pass.
	for k := range level.WorkerZLevels {
		delete(level.WorkerZLevels, k)
	}
	s.blocked.reset(level)
	s.origins = s.origins[:0]

	for _, entity := range level.Entities {
		if !entity.HasComponent(components.Worker) {
			continue
		}
		if !entity.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()

		radius := DefaultSightRadius
		if entity.HasComponent(rlcomponents.Light) {
			lc := entity.GetComponent(rlcomponents.Light).(*rlcomponents.LightComponent)
			if lc.Range > radius {
				radius = lc.Range
			}
		}

		level.WorkerZLevels[z] = true
		if o := (fovOrigin{x, y, z, radius}); !slices.Contains(s.origins, o) {
			s.origins = append(s.origins, o)
			updateFOV(level, &s.blocked, x, y, z, radius)
		}
	}
	return nil
}

func (s *FOVSystem) UpdateEntity(data interface{}, entity *ecs.Entity) error { return nil }

// updateFOV marks all tiles within radius of (ox,oy,oz) as visible + seen if
// they have line of sight to the origin. Uses Bresenham ray casting.
// After marking each tile visible, propagates upward through open-air tiles
// (no floor = no ceiling blocking the view from below).
func updateFOV(level *world.Level, memo *losMemo, ox, oy, oz, radius int) {
	r2 := radius * radius
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy > r2 {
				continue
			}
			tx, ty := ox+dx, oy+dy
			if !level.InBounds(tx, ty, oz) {
				continue
			}
			if losWalk(ox, oy, tx, ty, func(x, y int) bool { return memo.blocks(level, x, y, oz) }) {
				level.SetVisible(tx, ty, oz)
				level.SetSeen(tx, ty, oz, true)
				propagateVisibilityUp(level, tx, ty, oz)
			}
		}
	}
}

// propagateVisibilityUp marks tiles above (x,y,z) as visible + seen as long as
// the tile above has no floor (no ceiling blocking upward sight) and is in bounds.
func propagateVisibilityUp(level *world.Level, x, y, z int) {
	for above := z + 1; level.InBounds(x, y, above); above++ {
		tile := level.GetTilePtr(x, y, above)
		// A non-empty floor at this level acts as a ceiling — stop here.
		if tile != nil && !tile.Floor.IsEmpty() {
			break
		}
		level.SetVisible(x, y, above)
		level.SetSeen(x, y, above, true)
		// A solid middle at this level blocks further upward sight.
		if tile != nil && tile.IsSolid() {
			break
		}
	}
}

// losCheck casts a Bresenham ray from (px,py) to (tx,ty) on z-level z.
// Returns true if no solid tile or closed door blocks the line.
func losCheck(level *world.Level, px, py, tx, ty, z int) bool {
	return losWalk(px, py, tx, ty, func(x, y int) bool { return blocksLOS(level, x, y, z) })
}

// losWalk walks the Bresenham ray from (tx,ty) back to (px,py), asking
// blocks of every cell between them; true when none blocks.
func losWalk(px, py, tx, ty int, blocks func(x, y int) bool) bool {
	dx := px - tx
	dy := py - ty
	adx, ady := dx, dy
	if adx < 0 {
		adx = -adx
	}
	if ady < 0 {
		ady = -ady
	}
	sx := sign(float64(dx))
	sy := sign(float64(dy))
	cx, cy := tx, ty

	if adx > ady {
		t := ady*2 - adx
		for {
			if t >= 0 {
				cy += sy
				t -= adx * 2
			}
			cx += sx
			t += ady * 2
			if cx == px && cy == py {
				return true
			}
			if blocks(cx, cy) {
				return false
			}
		}
	}
	t := adx*2 - ady
	for {
		if t >= 0 {
			cx += sx
			t -= ady * 2
		}
		cy += sy
		t += adx * 2
		if cx == px && cy == py {
			return true
		}
		if blocks(cx, cy) {
			return false
		}
	}
}

func blocksLOS(level *world.Level, x, y, z int) bool {
	tile := level.GetTilePtr(x, y, z)
	if tile == nil {
		return true
	}
	if tile.IsSolid() {
		return true
	}
	// Closed doors block LOS.
	e := level.GetSolidEntityAt(x, y, z)
	if e != nil && e.HasComponent(rlcomponents.Door) {
		door := e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
		return !door.Open
	}
	return false
}

func sign(v float64) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}
