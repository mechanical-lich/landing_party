package systems

import (
	"math"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

const DefaultSightRadius = 12

// FOVSystem runs once per system-update (not per entity turn) and rebuilds the
// colony's shared field of view from every worker's position. Tiles with LOS
// are marked visible; all tiles are permanently marked seen once visible.
type FOVSystem struct{}

// Requires returns no required components — UpdateSystem handles everything.
func (s *FOVSystem) Requires() []ecs.ComponentType { return nil }

func (s *FOVSystem) UpdateSystem(data interface{}) error {
	level := data.(*world.Level)
	level.ClearVisible()

	// Reset the worker Z set each pass.
	for k := range level.WorkerZLevels {
		delete(level.WorkerZLevels, k)
	}

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

		updateFOV(level, x, y, z, radius)
		level.WorkerZLevels[z] = true

		// Mark tiles as Seen at the camera Z so scrolling to another level
		// reveals terrain around workers. Only writes Seen (permanent), never
		// Visible — DrawLevel uses Seen directly for non-worker Z levels.
		// Skips tiles already seen to avoid redundant writes every tick.
		if level.CameraZ != z {
			markSeenRadius(level, x, y, level.CameraZ, radius)
		}
	}
	return nil
}

func (s *FOVSystem) UpdateEntity(data interface{}, entity *ecs.Entity) error { return nil }

// markSeenRadius marks tiles within radius of (ox,oy,oz) as permanently seen
// without any LOS check and without touching the Visible array. Used for the
// camera Z projection — DrawLevel uses Seen directly for non-worker Z levels,
// so we don't need to redo this work every tick once tiles are already seen.
func markSeenRadius(level *world.Level, ox, oy, oz, radius int) {
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
			// Conditional write: skip if already seen to avoid cache-dirtying
			// writes on every tick after the first pass at this Z level.
			if !level.GetSeen(tx, ty, oz) {
				level.SetSeen(tx, ty, oz, true)
			}
		}
	}
}

// updateFOV marks all tiles within radius of (ox,oy,oz) as visible + seen if
// they have line of sight to the origin. Uses Bresenham ray casting.
func updateFOV(level *world.Level, ox, oy, oz, radius int) {
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
			if losCheck(level, ox, oy, tx, ty, oz) {
				level.SetVisible(tx, ty, oz)
				level.SetSeen(tx, ty, oz, true)
			}
		}
	}
}

// losCheck casts a Bresenham ray from (px,py) to (tx,ty) on z-level z.
// Returns true if no solid tile or closed door blocks the line.
func losCheck(level *world.Level, px, py, tx, ty, z int) bool {
	dx := px - tx
	dy := py - ty
	adx := math.Abs(float64(dx))
	ady := math.Abs(float64(dy))
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
			if blocksLOS(level, cx, cy, z) {
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
		if blocksLOS(level, cx, cy, z) {
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
