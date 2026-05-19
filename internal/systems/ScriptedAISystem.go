package systems

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/combat"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/skills"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcombat"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

type ScriptedAISystem struct {
	cache map[string]string
}

var scriptedAIRequires = []ecs.ComponentType{
	rlcomponents.Position,
	components.ScriptedAI,
	rlcomponents.MyTurn,
}

func (s *ScriptedAISystem) Requires() []ecs.ComponentType { return scriptedAIRequires }

func (s *ScriptedAISystem) UpdateSystem(data interface{}) error { return nil }

func (s *ScriptedAISystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}

	level := levelInterface.(*world.Level)
	ai := entity.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)

	if ai.Script == "" {
		return nil
	}
	if s.cache == nil {
		s.cache = make(map[string]string)
	}
	src, ok := s.cache[ai.Script]
	if !ok {
		raw, err := os.ReadFile(ai.Script)
		if err != nil {
			log.Printf("ScriptedAISystem: read %s: %v", ai.Script, err)
			ai.Script = ""
			return nil
		}
		src = string(raw)
		s.cache[ai.Script] = src
	}

	if ai.Vars == nil {
		ai.Vars = make(map[string]any)
	}

	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, entity, level, ai)

	if err := interp.Load(src); err != nil {
		log.Printf("ScriptedAISystem: load %s: %v", ai.Script, err)
		return nil
	}
	if !interp.HasFunction("on_turn") {
		return nil
	}
	if _, err := interp.Call("on_turn"); err != nil {
		log.Printf("ScriptedAISystem: on_turn %s: %v", ai.Script, err)
	}
	return nil
}

func registerScriptedAIFuncs(interp *basic.MechBasic, entity *ecs.Entity, level *world.Level, ai *components.ScriptedAIComponent) {
	// --- persistent entity state ---
	interp.RegisterFunc("get_var", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		v := ai.Vars[fmt.Sprint(args[0])]
		if v == nil {
			return float64(0), nil
		}
		return v, nil
	})
	interp.RegisterFunc("set_var", func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		ai.Vars[fmt.Sprint(args[0])] = args[1]
		return nil, nil
	})

	// --- self ---
	interp.RegisterFunc("get_self_x", func(args ...any) (any, error) {
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		return float64(pc.GetX()), nil
	})
	interp.RegisterFunc("get_self_y", func(args ...any) (any, error) {
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		return float64(pc.GetY()), nil
	})
	interp.RegisterFunc("get_self_z", func(args ...any) (any, error) {
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		return float64(pc.GetZ()), nil
	})
	interp.RegisterFunc("get_self_health", func(args ...any) (any, error) {
		if !entity.HasComponent(rlcomponents.Health) {
			return float64(-1), nil
		}
		hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		return float64(hc.Health), nil
	})

	// --- world queries ---
	interp.RegisterFunc("get_width", func(args ...any) (any, error) {
		return float64(level.GetWidth()), nil
	})
	interp.RegisterFunc("get_height", func(args ...any) (any, error) {
		return float64(level.GetHeight()), nil
	})
	interp.RegisterFunc("get_z_count", func(args ...any) (any, error) {
		return float64(level.GetDepth()), nil
	})
	interp.RegisterFunc("get_radiation_at", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		t := level.GetTilePtr(x, y, z)
		if t == nil {
			return float64(0), nil
		}
		return float64(t.Radiation), nil
	})
	interp.RegisterFunc("get_tile_type", func(args ...any) (any, error) {
		if len(args) < 3 {
			return "", nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		tI := level.GetTileAt(x, y, z)
		if tI == nil {
			return "", nil
		}
		t := tI.(*world.Tile)
		slot := t.Middle
		if slot.IsEmpty() {
			slot = t.Floor
		}
		if slot.IsEmpty() {
			return "", nil
		}
		return world.TileIndexToName[slot.Type], nil
	})
	interp.RegisterFunc("is_tile_solid", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		tI := level.GetTileAt(x, y, z)
		if tI == nil {
			return float64(1), nil
		}
		if tI.(*world.Tile).IsSolid() {
			return float64(1), nil
		}
		return float64(0), nil
	})
	interp.RegisterFunc("get_hour", func(args ...any) (any, error) {
		return float64(level.Hour), nil
	})
	interp.RegisterFunc("is_night", func(args ...any) (any, error) {
		if level.IsNight() {
			return float64(1), nil
		}
		return float64(0), nil
	})
	interp.RegisterFunc("get_tile_light", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		t := level.GetTilePtr(x, y, z)
		if t == nil {
			return float64(0), nil
		}
		return float64(t.LightLevel), nil
	})

	// --- nearest radiation tile scan ---
	// find_highest_radiation_tile(radius) — scans a square of tiles around the
	// entity and caches the coords of the highest radiation tile found.
	// Returns that radiation level (0 if nothing found).
	var lastRadX, lastRadY, lastRadZ int
	interp.RegisterFunc("find_highest_radiation_tile", func(args ...any) (any, error) {
		radius := 10
		if len(args) >= 1 {
			radius = int(toAIFloat(args[0]))
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()
		best := -1
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				t := level.GetTilePtr(sx+dx, sy+dy, sz)
				if t == nil {
					continue
				}
				if int(t.Radiation) > best {
					best = int(t.Radiation)
					lastRadX = sx + dx
					lastRadY = sy + dy
					lastRadZ = sz
				}
			}
		}
		if best <= 0 {
			return float64(0), nil
		}
		return float64(best), nil
	})
	interp.RegisterFunc("get_rad_x", func(args ...any) (any, error) { return float64(lastRadX), nil })
	interp.RegisterFunc("get_rad_y", func(args ...any) (any, error) { return float64(lastRadY), nil })
	interp.RegisterFunc("get_rad_z", func(args ...any) (any, error) { return float64(lastRadZ), nil })

	// --- open tile search ---
	// find_open_tile(z) — random search for a passable, unoccupied tile on z.
	// Returns 1 on success; use get_open_x/y to retrieve the coords.
	var lastOpenX, lastOpenY int
	interp.RegisterFunc("find_open_tile", func(args ...any) (any, error) {
		z := 0
		if len(args) >= 1 {
			z = int(toAIFloat(args[0]))
		}
		w := level.GetWidth()
		h := level.GetHeight()
		for attempt := 0; attempt < 200; attempt++ {
			x := rand.Intn(w)
			y := rand.Intn(h)
			tI := level.GetTileAt(x, y, z)
			if tI == nil {
				continue
			}
			t := tI.(*world.Tile)
			if !t.IsSolid() && !t.IsWater() && level.GetEntityAt(x, y, z) == nil {
				lastOpenX = x
				lastOpenY = y
				return float64(1), nil
			}
		}
		return float64(0), nil
	})
	interp.RegisterFunc("get_open_x", func(args ...any) (any, error) { return float64(lastOpenX), nil })
	interp.RegisterFunc("get_open_y", func(args ...any) (any, error) { return float64(lastOpenY), nil })

	// --- entity search ---
	// find_nearest_worker(radius) — finds nearest entity with a Worker component.
	// find_nearest_blueprint(blueprint, radius) — finds nearest entity with matching blueprint.
	// Both return 1 on success and populate get_nearest_x/y/z.
	var lastNearX, lastNearY, lastNearZ int
	interp.RegisterFunc("find_nearest_worker", func(args ...any) (any, error) {
		radius := 12
		if len(args) >= 1 {
			radius = int(toAIFloat(args[0]))
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		found := level.GetClosestEntityMatching(
			pc.GetX(), pc.GetY(), pc.GetZ(),
			radius*2, radius*2,
			entity,
			func(c *ecs.Entity) bool {
				return !c.HasComponent(rlcomponents.Dead) && c.HasComponent(components.Worker)
			},
		)
		if found == nil {
			return float64(0), nil
		}
		fp := found.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		lastNearX, lastNearY, lastNearZ = fp.GetX(), fp.GetY(), fp.GetZ()
		return float64(1), nil
	})
	interp.RegisterFunc("find_nearest_blueprint", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		bp := fmt.Sprint(args[0])
		radius := 12
		if len(args) >= 2 {
			radius = int(toAIFloat(args[1]))
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		found := level.GetClosestEntityMatching(
			pc.GetX(), pc.GetY(), pc.GetZ(),
			radius*2, radius*2,
			entity,
			func(c *ecs.Entity) bool {
				return !c.HasComponent(rlcomponents.Dead) && c.Blueprint == bp
			},
		)
		if found == nil {
			return float64(0), nil
		}
		fp := found.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		lastNearX, lastNearY, lastNearZ = fp.GetX(), fp.GetY(), fp.GetZ()
		return float64(1), nil
	})
	interp.RegisterFunc("get_nearest_x", func(args ...any) (any, error) { return float64(lastNearX), nil })
	interp.RegisterFunc("get_nearest_y", func(args ...any) (any, error) { return float64(lastNearY), nil })
	interp.RegisterFunc("get_nearest_z", func(args ...any) (any, error) { return float64(lastNearZ), nil })

	// get_self_faction() — returns this entity's faction string.
	interp.RegisterFunc("get_self_faction", func(args ...any) (any, error) {
		if !entity.HasComponent(rlcomponents.Description) {
			return "", nil
		}
		dc := entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		return dc.Faction, nil
	})
	// find_nearest_enemy(radius) — finds the nearest entity not in this entity's
	// faction and not in the "ignored_factions" var (comma-separated list).
	// Returns 1 on success and populates get_nearest_x/y/z.
	interp.RegisterFunc("find_nearest_enemy", func(args ...any) (any, error) {
		radius := 12
		if len(args) >= 1 {
			radius = int(toAIFloat(args[0]))
		}
		selfFaction := ""
		if entity.HasComponent(rlcomponents.Description) {
			selfFaction = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
		}
		ignored := map[string]bool{}
		if selfFaction != "" {
			ignored[selfFaction] = true
		}
		if raw, ok := ai.Vars["ignored_factions"]; ok {
			for _, f := range strings.Split(fmt.Sprint(raw), ",") {
				if t := strings.TrimSpace(f); t != "" {
					ignored[t] = true
				}
			}
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		found := level.GetClosestEntityMatching(
			pc.GetX(), pc.GetY(), pc.GetZ(),
			radius*2, radius*2,
			entity,
			func(c *ecs.Entity) bool {
				if c.HasComponent(rlcomponents.Dead) {
					return false
				}
				if !c.HasComponent(rlcomponents.Health) {
					return false
				}
				if !c.HasComponent(rlcomponents.Description) {
					return true
				}
				cf := c.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
				return !ignored[cf]
			},
		)
		if found == nil {
			return float64(0), nil
		}
		fp := found.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		lastNearX, lastNearY, lastNearZ = fp.GetX(), fp.GetY(), fp.GetZ()
		return float64(1), nil
	})

	// --- movement ---
	// pathfind_step(tx, ty, tz) — move one step along the path to (tx,ty,tz).
	// Returns 1 if a step was taken, 0 if already adjacent/there or no path.
	interp.RegisterFunc("pathfind_step", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		tx := int(toAIFloat(args[0]))
		ty := int(toAIFloat(args[1]))
		tz := int(toAIFloat(args[2]))
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()
		if sx == tx && sy == ty {
			return float64(0), nil
		}
		fromTile := level.GetTilePtr(sx, sy, sz)
		toTile := level.GetTilePtr(tx, ty, tz)
		if fromTile == nil || toTile == nil {
			return float64(0), nil
		}
		steps := fspath.GetPossiblePathForEntity(level, entity, fromTile, toTile, nil)
		if len(steps) < 2 {
			return float64(0), nil
		}
		next := level.Level.GetTilePtrIndex(steps[1])
		nx, ny, _ := next.Coords()
		dx, dy := nx-sx, ny-sy
		rlentity.Move(entity, level, dx, dy, 0)
		rlentity.Face(entity, dx, dy)
		return float64(1), nil
	})
	// move_by(dx, dy) — raw relative move (no pathfinding).
	// Returns 1 if the move succeeded. Refuses to step into space tiles
	// unless the entity has the spacefaring skill.
	interp.RegisterFunc("move_by", func(args ...any) (any, error) {
		if len(args) < 2 {
			return float64(0), nil
		}
		dx := int(toAIFloat(args[0]))
		dy := int(toAIFloat(args[1]))
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		destX, destY, destZ := pc.GetX()+dx, pc.GetY()+dy, pc.GetZ()
		dest := level.GetTilePtr(destX, destY, destZ)
		spacefaring := skills.Has(entity, fspath.SpacefaringSkill)
		flying := spacefaring || skills.Has(entity, fspath.FlyingSkill) || skills.Has(entity, fspath.ClimbingSkill)
		if dest != nil && !dest.Middle.IsEmpty() && world.TileDefinitions[dest.Middle.Type].Space && !spacefaring {
			return float64(0), nil
		}
		if flying {
			// Bypass rlentity.Move's floor/air checks for 3D-capable entities.
			if dest == nil {
				return float64(0), nil
			}
			if !dest.Middle.IsEmpty() && world.TileDefinitions[dest.Middle.Type].Solid {
				return float64(0), nil
			}
			solid := level.GetSolidEntityAt(destX, destY, destZ)
			if solid != nil && solid != entity {
				return float64(1), nil
			}
			level.PlaceEntity(destX, destY, destZ, entity)
			rlentity.Face(entity, dx, dy)
			return float64(0), nil
		}
		moved := rlentity.Move(entity, level, dx, dy, 0)
		rlentity.Face(entity, dx, dy)
		if moved {
			return float64(1), nil
		}
		return float64(0), nil
	})

	// --- combat ---
	// attack_at(x, y, z) — attack the entity at the given tile, if any.
	// Returns 1 if an attack was made.
	var entitiesBuf []*ecs.Entity
	interp.RegisterFunc("attack_at", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		entitiesBuf = entitiesBuf[:0]
		level.GetEntitiesAt(x, y, z, &entitiesBuf)
		for _, e := range entitiesBuf {
			if e != entity && e.HasComponent(rlcomponents.Health) && !rlcombat.IsFriendly(entity, e) {
				rlcombat.Hit(level, entity, e, true)
				return float64(1), nil
			}
		}
		return float64(0), nil
	})

	// has_ranged_weapon() — returns 1 if the entity has a ranged weapon equipped.
	findRangedWeapon := func() *ecs.Entity {
		if !entity.HasComponent(rlcomponents.Inventory) {
			return nil
		}
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range []*ecs.Entity{inv.LeftHand, inv.RightHand} {
			if item != nil && item.HasComponent(rlcomponents.Weapon) {
				if item.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent).Ranged {
					return item
				}
			}
		}
		return nil
	}
	interp.RegisterFunc("has_ranged_weapon", func(args ...any) (any, error) {
		if findRangedWeapon() != nil {
			return float64(1), nil
		}
		return float64(0), nil
	})
	// ranged_attack_at(x, y, z) — fire the equipped ranged weapon at the given tile.
	// Returns 1 if in range, line of sight is clear, and a shot was fired; 0 otherwise.
	interp.RegisterFunc("ranged_attack_at", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		weapon := findRangedWeapon()
		if weapon == nil {
			return float64(0), nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if !losCheck(level, pc.GetX(), pc.GetY(), x, y, z) {
			return float64(0), nil
		}
		rlentity.Face(entity, x-pc.GetX(), y-pc.GetY())
		if combat.Shoot(level, entity, x, y, z, weapon) {
			return float64(1), nil
		}
		return float64(0), nil
	})

	// --- tile mutation ---
	// break_tile(x, y, z) — clear the middle slot (wall/obstacle) at a tile.
	interp.RegisterFunc("break_tile", func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		level.ClearMiddle(x, y, z)
		return nil, nil
	})
	// set_tile(x, y, z, name) — set the middle slot of a tile.
	interp.RegisterFunc("set_tile", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		name := fmt.Sprint(args[3])
		level.SetMiddle(x, y, z, name, world.RandomTileVariant(name))
		return nil, nil
	})

	// --- entity lifecycle ---
	// spawn_entity(blueprint, x, y, z) — create and add an entity.
	interp.RegisterFunc("spawn_entity", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		bp := fmt.Sprint(args[0])
		x := int(toAIFloat(args[1]))
		y := int(toAIFloat(args[2]))
		z := int(toAIFloat(args[3]))
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			return nil, nil
		}
		level.AddEntity(e)
		return nil, nil
	})
	// despawn_self() — remove this entity from the world.
	interp.RegisterFunc("despawn_self", func(args ...any) (any, error) {
		entity.AddComponent(&rlcomponents.DeadComponent{})
		return nil, nil
	})

	// --- level flags (shared world state) ---
	interp.RegisterFunc("set_flag", func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		level.Flags[fmt.Sprint(args[0])] = args[1]
		return nil, nil
	})
	interp.RegisterFunc("get_flag", func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, nil
		}
		return level.Flags[fmt.Sprint(args[0])], nil
	})

	// --- messages ---
	interp.RegisterFunc("add_message", func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, nil
		}
		message.PostMessage("world", fmt.Sprint(args[0]))
		return nil, nil
	})

	// --- math / util ---
	interp.RegisterFunc("rnd_int", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		n := int(toAIFloat(args[0]))
		if n <= 0 {
			return float64(0), nil
		}
		return float64(rand.Intn(n)), nil
	})
	interp.RegisterFunc("num_to_str", func(args ...any) (any, error) {
		if len(args) < 1 {
			return "", nil
		}
		return fmt.Sprint(args[0]), nil
	})
	interp.RegisterFunc("dist", func(args ...any) (any, error) {
		if len(args) < 4 {
			return float64(0), nil
		}
		dx := toAIFloat(args[0]) - toAIFloat(args[2])
		dy := toAIFloat(args[1]) - toAIFloat(args[3])
		return math.Sqrt(dx*dx + dy*dy), nil
	})
}

func toAIFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	}
	return 0
}
