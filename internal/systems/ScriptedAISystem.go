package systems

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/audio"
	"github.com/mechanical-lich/landing_party/internal/combat"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/emotes"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/skills"
	"github.com/mechanical-lich/landing_party/internal/workerai"
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

	if ai.Interp == nil || ai.InterpScript != ai.Script {
		interp := basic.NewMechanicalBasic()
		registerScriptedAIFuncs(interp, entity, level, ai)
		if err := interp.Load(src); err != nil {
			log.Printf("ScriptedAISystem: load %s: %v", ai.Script, err)
			return nil
		}
		ai.Interp = interp
		ai.InterpScript = ai.Script
		ai.InterpHasOnTurn = interp.HasFunction("on_turn")
	}

	if !ai.InterpHasOnTurn {
		return nil
	}
	if _, err := ai.Interp.Call("on_turn"); err != nil {
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
	// was_attacked() — returns 1 if this entity was hit since its last turn, then clears the flag.
	interp.RegisterFunc("was_attacked", func(args ...any) (any, error) {
		if !entity.HasComponent(rlcomponents.AIMemory) {
			return float64(0), nil
		}
		mem := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
		if mem.Attacked {
			mem.Attacked = false
			return float64(1), nil
		}
		return float64(0), nil
	})
	// get_attacker_x / get_attacker_y — position of the last attacker (valid after was_attacked = 1).
	interp.RegisterFunc("get_attacker_x", func(args ...any) (any, error) {
		if !entity.HasComponent(rlcomponents.AIMemory) {
			return float64(0), nil
		}
		return float64(entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).AttackerX), nil
	})
	interp.RegisterFunc("get_attacker_y", func(args ...any) (any, error) {
		if !entity.HasComponent(rlcomponents.AIMemory) {
			return float64(0), nil
		}
		return float64(entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).AttackerY), nil
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
	interp.RegisterFunc("get_tick", func(args ...any) (any, error) {
		return float64(level.Tick), nil
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
	//
	// find_nearest_worker and find_nearest_enemy cache the last found entity and
	// skip the full ring scan while the target is still alive and in LOS. The
	// cache is cleared when the target dies, goes out of LOS, or LOS is blocked.
	var lastNearX, lastNearY, lastNearZ int
	var cachedWorker *ecs.Entity
	var workerMissCooldown int
	interp.RegisterFunc("find_nearest_worker", func(args ...any) (any, error) {
		radius := 12
		if len(args) >= 1 {
			radius = int(toAIFloat(args[0]))
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()

		if cachedWorker != nil {
			if !cachedWorker.HasComponent(rlcomponents.Dead) &&
				cachedWorker.HasComponent(components.Worker) &&
				cachedWorker.HasComponent(rlcomponents.Position) {
				cp := cachedWorker.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				if losCheck(level, sx, sy, cp.GetX(), cp.GetY(), sz) {
					lastNearX, lastNearY, lastNearZ = cp.GetX(), cp.GetY(), cp.GetZ()
					return float64(1), nil
				}
			}
			cachedWorker = nil
		}

		if workerMissCooldown > 0 {
			workerMissCooldown--
			return float64(0), nil
		}

		found := level.GetClosestEntityMatching(
			sx, sy, sz, radius*2, radius*2, entity,
			func(c *ecs.Entity) bool {
				if c.HasComponent(rlcomponents.Dead) || !c.HasComponent(components.Worker) {
					return false
				}
				if !c.HasComponent(rlcomponents.Position) {
					return false
				}
				cp := c.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				return losCheck(level, sx, sy, cp.GetX(), cp.GetY(), sz)
			},
		)
		if found == nil {
			workerMissCooldown = 4
			return float64(0), nil
		}
		cachedWorker = found
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
	// Caches the last found enemy; skips the ring scan while it's alive and in LOS.
	selfFaction := ""
	if entity.HasComponent(rlcomponents.Description) {
		selfFaction = entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
	}
	ignored := buildIgnored(selfFaction, ai)

	// isEnemy is the shared "hostile, targetable entity" predicate for the enemy
	// finders below (alive, has health, not an ignored/own faction). It does NOT
	// test line-of-sight — callers layer LOS on top when sight is required.
	isEnemy := func(c *ecs.Entity) bool {
		if c == nil || c == entity {
			return false
		}
		if c.HasComponent(rlcomponents.Dead) || !c.HasComponent(rlcomponents.Health) {
			return false
		}
		if c.HasComponent(rlcomponents.Description) {
			cf := c.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
			if ignored[cf] {
				return false
			}
		}
		return c.HasComponent(rlcomponents.Position)
	}

	var cachedEnemy *ecs.Entity
	var enemyMissCooldown int
	interp.RegisterFunc("find_nearest_enemy", func(args ...any) (any, error) {
		radius := 12
		if len(args) >= 1 {
			radius = int(toAIFloat(args[0]))
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()

		if cachedEnemy != nil {
			alive := !cachedEnemy.HasComponent(rlcomponents.Dead) &&
				cachedEnemy.HasComponent(rlcomponents.Health) &&
				cachedEnemy.HasComponent(rlcomponents.Position)
			if alive {
				cp := cachedEnemy.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				if losCheck(level, sx, sy, cp.GetX(), cp.GetY(), sz) {
					lastNearX, lastNearY, lastNearZ = cp.GetX(), cp.GetY(), cp.GetZ()
					return float64(1), nil
				}
			}
			cachedEnemy = nil
		}

		if enemyMissCooldown > 0 {
			enemyMissCooldown--
			return float64(0), nil
		}

		found := level.GetClosestEntityMatching(
			sx, sy, sz, radius*2, radius*2, entity,
			func(c *ecs.Entity) bool {
				if !isEnemy(c) {
					return false
				}
				cp := c.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				return losCheck(level, sx, sy, cp.GetX(), cp.GetY(), sz)
			},
		)
		if found == nil {
			enemyMissCooldown = 4
			return float64(0), nil
		}
		cachedEnemy = found
		fp := found.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		lastNearX, lastNearY, lastNearZ = fp.GetX(), fp.GetY(), fp.GetZ()
		return float64(1), nil
	})

	// sense_nearest_enemy(radius) — like find_nearest_enemy but WITHOUT
	// line-of-sight and across EVERY z-level: a burrowing creature feels prey
	// through the earth, so it can detect a target on the surface (or in another
	// cavern layer) while itself buried in solid rock. Distance is measured
	// horizontally (x,y) so depth never hides an enemy directly overhead. Returns
	// 1 on success and populates get_nearest_x/y/z (including the target's z, so
	// the script can burrow up or down toward it).
	var cachedSensed *ecs.Entity
	var senseMissCooldown int
	interp.RegisterFunc("sense_nearest_enemy", func(args ...any) (any, error) {
		radius := 12
		if len(args) >= 1 {
			radius = int(toAIFloat(args[0]))
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy := pc.GetX(), pc.GetY()

		if cachedSensed != nil {
			if isEnemy(cachedSensed) {
				cp := cachedSensed.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				dx, dy := cp.GetX()-sx, cp.GetY()-sy
				if dx*dx <= radius*radius && dy*dy <= radius*radius {
					lastNearX, lastNearY, lastNearZ = cp.GetX(), cp.GetY(), cp.GetZ()
					return float64(1), nil
				}
			}
			cachedSensed = nil
		}

		if senseMissCooldown > 0 {
			senseMissCooldown--
			return float64(0), nil
		}

		// Scan every z-plane and keep the horizontally-closest hostile. The lib's
		// closest-entity search is confined to a single z-plane, so a burrower has
		// to sweep the column to see across layers.
		var best *ecs.Entity
		bestDistSq := int(^uint(0) >> 1)
		for z := 0; z < level.GetDepth(); z++ {
			found := level.GetClosestEntityMatching(sx, sy, z, radius*2, radius*2, entity, isEnemy)
			if found == nil {
				continue
			}
			fp := found.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			dx, dy := fp.GetX()-sx, fp.GetY()-sy
			if d := dx*dx + dy*dy; d < bestDistSq {
				bestDistSq, best = d, found
			}
		}
		if best == nil {
			senseMissCooldown = 4
			return float64(0), nil
		}
		cachedSensed = best
		fp := best.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		lastNearX, lastNearY, lastNearZ = fp.GetX(), fp.GetY(), fp.GetZ()
		return float64(1), nil
	})

	// --- hearing ---
	// has_ears() — 1 if this entity has a HearingComponent.
	// new_sounds() — count of inbox entries unread since last call this turn.
	//   Sets last_sound_* to the loudest unread entry and advances the read
	//   pointer. Returns 0 if inbox is empty or fully drained.
	// loudest_sound(max_radius?) — strongest sound currently in the level's
	//   event slice that's within sensitivity. Optional 3D radius gate.
	//   Returns perceived loudness (0 if none) and sets last_sound_*.
	// get_sound_x/y/z/tag/loudness — retrieve the last sound returned by
	//   new_sounds() or loudest_sound().
	var lastSoundX, lastSoundY, lastSoundZ int
	var lastSoundTag string
	var lastSoundLoudness float64
	interp.RegisterFunc("has_ears", func(args ...any) (any, error) {
		if entity.HasComponent(components.Hearing) {
			return float64(1), nil
		}
		return float64(0), nil
	})
	interp.RegisterFunc("new_sounds", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Hearing) {
			return float64(0), nil
		}
		hc := entity.GetComponent(components.Hearing).(*components.HearingComponent)
		unread := hc.Inbox[hc.InboxRead:]
		if len(unread) == 0 {
			return float64(0), nil
		}
		best := -1
		var bestLoud float32
		for i, p := range unread {
			if best < 0 || p.Perceived > bestLoud {
				best = i
				bestLoud = p.Perceived
			}
		}
		ev := unread[best]
		lastSoundX = ev.X
		lastSoundY = ev.Y
		lastSoundZ = ev.Z
		lastSoundTag = ev.Tag
		lastSoundLoudness = float64(ev.Perceived)
		count := len(unread)
		hc.InboxRead = len(hc.Inbox)
		return float64(count), nil
	})
	interp.RegisterFunc("loudest_sound", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Hearing) {
			return float64(0), nil
		}
		hc := entity.GetComponent(components.Hearing).(*components.HearingComponent)
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()
		maxRadius := float64(-1)
		if len(args) >= 1 {
			maxRadius = toAIFloat(args[0])
		}
		var bestLoud float32
		var bestEv *world.SoundEvent
		for i := range level.Sounds {
			ev := &level.Sounds[i]
			if ev.Source == entity {
				continue
			}
			perceived := PerceivedLoudness(*ev, sx, sy, sz)
			if perceived < hc.Sensitivity {
				continue
			}
			if maxRadius >= 0 {
				dx := float64(ev.X - sx)
				dy := float64(ev.Y - sy)
				dz := float64(ev.Z-sz) * world.VerticalSoundFalloff
				if math.Sqrt(dx*dx+dy*dy+dz*dz) > maxRadius {
					continue
				}
			}
			if bestEv == nil || perceived > bestLoud {
				bestEv = ev
				bestLoud = perceived
			}
		}
		if bestEv == nil {
			return float64(0), nil
		}
		lastSoundX = bestEv.X
		lastSoundY = bestEv.Y
		lastSoundZ = bestEv.Z
		lastSoundTag = string(bestEv.Tag)
		lastSoundLoudness = float64(bestLoud)
		return float64(bestLoud), nil
	})
	interp.RegisterFunc("get_sound_x", func(args ...any) (any, error) { return float64(lastSoundX), nil })
	interp.RegisterFunc("get_sound_y", func(args ...any) (any, error) { return float64(lastSoundY), nil })
	interp.RegisterFunc("get_sound_z", func(args ...any) (any, error) { return float64(lastSoundZ), nil })
	interp.RegisterFunc("get_sound_tag", func(args ...any) (any, error) { return lastSoundTag, nil })
	interp.RegisterFunc("get_sound_loudness", func(args ...any) (any, error) { return lastSoundLoudness, nil })

	// --- vision ---
	// has_eyes() — 1 if entity has a VisionComponent.
	// visible_enemy_count() — number of currently visible entities not in
	//   this entity's faction or ignored_factions.
	// nearest_visible_enemy() — 1 if any enemy is currently visible; sets
	//   last_seen_* to the nearest one.
	// newly_visible_enemies() — count of enemies that entered FOV since this
	//   entity's last turn; sets last_seen_* to the nearest new one and
	//   drains the inbox.
	// newly_hidden_enemies() — count of enemies that left FOV since this
	//   entity's last turn; sets last_lost_* to the nearest hidden one's
	//   last-known position and drains the inbox.
	// get_seen_x/y/z — position of the last sighting (from nearest_visible_enemy
	//   or newly_visible_enemies).
	// get_lost_x/y/z — last-known position of the last lost sighting.
	var lastSeenX, lastSeenY, lastSeenZ int
	var lastLostX, lastLostY, lastLostZ int
	// isEnemy (the shared hostile predicate) is declared above with the enemy
	// finders and reused here for the vision helpers.
	// Returns the index of the nearest enemy entry in the slice, or -1.
	nearestEnemyIndex := func(entries []components.VisibleEntry, sx, sy int) int {
		best := -1
		bestD2 := 0
		for i, en := range entries {
			if !isEnemy(en.Entity) {
				continue
			}
			dx := en.X - sx
			dy := en.Y - sy
			d2 := dx*dx + dy*dy
			if best < 0 || d2 < bestD2 {
				best = i
				bestD2 = d2
			}
		}
		return best
	}
	interp.RegisterFunc("has_eyes", func(args ...any) (any, error) {
		if entity.HasComponent(components.Vision) {
			return float64(1), nil
		}
		return float64(0), nil
	})
	interp.RegisterFunc("visible_enemy_count", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Vision) {
			return float64(0), nil
		}
		vc := entity.GetComponent(components.Vision).(*components.VisionComponent)
		n := 0
		for e := range vc.Visible {
			if isEnemy(e) {
				n++
			}
		}
		return float64(n), nil
	})
	interp.RegisterFunc("nearest_visible_enemy", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Vision) {
			return float64(0), nil
		}
		vc := entity.GetComponent(components.Vision).(*components.VisionComponent)
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy := pc.GetX(), pc.GetY()
		var bestEntry *components.VisibleEntry
		bestD2 := 0
		for _, en := range vc.Visible {
			if !isEnemy(en.Entity) {
				continue
			}
			dx := en.X - sx
			dy := en.Y - sy
			d2 := dx*dx + dy*dy
			if bestEntry == nil || d2 < bestD2 {
				e := en
				bestEntry = &e
				bestD2 = d2
			}
		}
		if bestEntry == nil {
			return float64(0), nil
		}
		lastSeenX, lastSeenY, lastSeenZ = bestEntry.X, bestEntry.Y, bestEntry.Z
		return float64(1), nil
	})
	interp.RegisterFunc("newly_visible_enemies", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Vision) {
			return float64(0), nil
		}
		vc := entity.GetComponent(components.Vision).(*components.VisionComponent)
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy := pc.GetX(), pc.GetY()
		unread := vc.NewlyVisible[vc.NewlyVisibleRead:]
		idx := nearestEnemyIndex(unread, sx, sy)
		count := 0
		for _, en := range unread {
			if isEnemy(en.Entity) {
				count++
			}
		}
		if idx >= 0 {
			en := unread[idx]
			lastSeenX, lastSeenY, lastSeenZ = en.X, en.Y, en.Z
		}
		vc.NewlyVisibleRead = len(vc.NewlyVisible)
		return float64(count), nil
	})
	interp.RegisterFunc("newly_hidden_enemies", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Vision) {
			return float64(0), nil
		}
		vc := entity.GetComponent(components.Vision).(*components.VisionComponent)
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy := pc.GetX(), pc.GetY()
		unread := vc.NewlyHidden[vc.NewlyHiddenRead:]
		idx := nearestEnemyIndex(unread, sx, sy)
		count := 0
		for _, en := range unread {
			if isEnemy(en.Entity) {
				count++
			}
		}
		if idx >= 0 {
			en := unread[idx]
			lastLostX, lastLostY, lastLostZ = en.X, en.Y, en.Z
		}
		vc.NewlyHiddenRead = len(vc.NewlyHidden)
		return float64(count), nil
	})
	interp.RegisterFunc("get_seen_x", func(args ...any) (any, error) { return float64(lastSeenX), nil })
	interp.RegisterFunc("get_seen_y", func(args ...any) (any, error) { return float64(lastSeenY), nil })
	interp.RegisterFunc("get_seen_z", func(args ...any) (any, error) { return float64(lastSeenZ), nil })
	interp.RegisterFunc("get_lost_x", func(args ...any) (any, error) { return float64(lastLostX), nil })
	interp.RegisterFunc("get_lost_y", func(args ...any) (any, error) { return float64(lastLostY), nil })
	interp.RegisterFunc("get_lost_z", func(args ...any) (any, error) { return float64(lastLostZ), nil })

	// --- smell ---
	// has_nose() — 1 if entity has a SmellComponent.
	// strongest_smell(radius?) — strongest scent within radius (defaults to
	//   SmellComponent.SniffRadius). Returns strength (0 if none) and sets
	//   last_smell_*.
	// smell_at(x, y, z, tag) — strength of a specific tag at one tile.
	// new_smells() — count of unread inbox entries; sets last_smell_* to
	//   strongest unread, drains.
	// get_smell_x/y/z/tag/strength — retrieve the last smell returned.
	var lastSmellX, lastSmellY, lastSmellZ int
	var lastSmellTag string
	var lastSmellStrength float64
	interp.RegisterFunc("has_nose", func(args ...any) (any, error) {
		if entity.HasComponent(components.Smell) {
			return float64(1), nil
		}
		return float64(0), nil
	})
	interp.RegisterFunc("strongest_smell", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Smell) {
			return float64(0), nil
		}
		sc := entity.GetComponent(components.Smell).(*components.SmellComponent)
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		radius := sc.SniffRadius
		if radius <= 0 {
			radius = defaultSniffRadius
		}
		if len(args) >= 1 {
			radius = int(toAIFloat(args[0]))
		}
		tag, x, y, z, strength := level.StrongestSmellWithin(pc.GetX(), pc.GetY(), pc.GetZ(), radius)
		if strength < sc.Sensitivity {
			return float64(0), nil
		}
		lastSmellX, lastSmellY, lastSmellZ = x, y, z
		lastSmellTag = string(tag)
		lastSmellStrength = float64(strength)
		return float64(strength), nil
	})
	interp.RegisterFunc("smell_at", func(args ...any) (any, error) {
		if len(args) < 4 {
			return float64(0), nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		tag := world.SmellTag(fmt.Sprint(args[3]))
		return float64(level.SmellAt(x, y, z, tag)), nil
	})
	interp.RegisterFunc("new_smells", func(args ...any) (any, error) {
		if !entity.HasComponent(components.Smell) {
			return float64(0), nil
		}
		sc := entity.GetComponent(components.Smell).(*components.SmellComponent)
		unread := sc.NewSmells[sc.NewSmellsRead:]
		if len(unread) == 0 {
			return float64(0), nil
		}
		best := -1
		var bestStrength float32
		for i, p := range unread {
			if best < 0 || p.Strength > bestStrength {
				best = i
				bestStrength = p.Strength
			}
		}
		p := unread[best]
		lastSmellX, lastSmellY, lastSmellZ = p.X, p.Y, p.Z
		lastSmellTag = p.Tag
		lastSmellStrength = float64(p.Strength)
		count := len(unread)
		sc.NewSmellsRead = len(sc.NewSmells)
		return float64(count), nil
	})
	interp.RegisterFunc("get_smell_x", func(args ...any) (any, error) { return float64(lastSmellX), nil })
	interp.RegisterFunc("get_smell_y", func(args ...any) (any, error) { return float64(lastSmellY), nil })
	interp.RegisterFunc("get_smell_z", func(args ...any) (any, error) { return float64(lastSmellZ), nil })
	interp.RegisterFunc("get_smell_tag", func(args ...any) (any, error) { return lastSmellTag, nil })
	interp.RegisterFunc("get_smell_strength", func(args ...any) (any, error) { return lastSmellStrength, nil })

	// --- emote ---
	// set_emote(key, duration?, priority?) — queue an emote bubble above this
	// entity. No-op if entity has no Emote component. Defaults: duration=3,
	// priority=0.
	interp.RegisterFunc("set_emote", func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, nil
		}
		key := fmt.Sprint(args[0])
		duration := 3
		priority := 0
		if len(args) >= 2 {
			duration = int(toAIFloat(args[1]))
		}
		if len(args) >= 3 {
			priority = int(toAIFloat(args[2]))
		}
		emotes.Set(entity, key, duration, priority)
		return nil, nil
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
			ai.PathCache = nil
			return float64(0), nil
		}
		// Recompute path only when target changes or cache is empty.
		if ai.PathTargetX != tx || ai.PathTargetY != ty || len(ai.PathCache) == 0 {
			fromTile := level.GetTilePtr(sx, sy, sz)
			toTile := level.GetTilePtr(tx, ty, tz)
			if fromTile == nil || toTile == nil {
				return float64(0), nil
			}
			steps := fspath.GetPossiblePathForEntity(level, entity, fromTile, toTile, ai.PathCache[:0])
			if len(steps) < 2 {
				ai.PathCache = nil
				return float64(0), nil
			}
			// Store steps[1:] — skip the starting tile.
			ai.PathCache = steps[1:]
			ai.PathTargetX = tx
			ai.PathTargetY = ty
		}
		// Advance past any waypoints we've already reached.
		for len(ai.PathCache) > 0 {
			wp := level.Level.GetTilePtrIndex(ai.PathCache[0])
			wx, wy, _ := wp.Coords()
			if wx == sx && wy == sy {
				ai.PathCache = ai.PathCache[1:]
			} else {
				break
			}
		}
		if len(ai.PathCache) == 0 {
			return float64(0), nil
		}
		next := level.Level.GetTilePtrIndex(ai.PathCache[0])
		nx, ny, _ := next.Coords()
		dx, dy := nx-sx, ny-sy
		rlentity.Move(entity, level, dx, dy, 0)
		npc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if npc.GetX() == nx && npc.GetY() == ny {
			workerai.EmitFootstep(level, entity, nx, ny, sz)
		}
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
		npc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if npc.GetX() == destX && npc.GetY() == destY {
			workerai.EmitFootstep(level, entity, destX, destY, destZ)
		}
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
				if e.HasComponent(components.Worker) {
					audio.NotifyCombat() // a colonist is under attack
				}
				pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				level.EmitSound(pc.GetX(), pc.GetY(), pc.GetZ(), 6, world.SoundTagImpact, entity)
				ep := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				level.EmitScent(ep.GetX(), ep.GetY(), ep.GetZ(), world.SmellTagBlood, 3.0)
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
	// set_floor(x, y, z, name) — set the floor slot of a tile.
	interp.RegisterFunc("set_floor", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		name := fmt.Sprint(args[3])
		level.SetFloor(x, y, z, name, world.RandomTileVariant(name))
		return nil, nil
	})
	// get_floor_type(x, y, z) — returns the tile name of the floor slot, or "" if empty.
	interp.RegisterFunc("get_floor_type", func(args ...any) (any, error) {
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
		if t.Floor.IsEmpty() {
			return "", nil
		}
		return world.TileIndexToName[t.Floor.Type], nil
	})

	// get_terrain_kind(x, y, z) — returns the terrain role string for a tile:
	// "underground", "surface", "subsurface", "atmosphere", "void".
	interp.RegisterFunc("get_terrain_kind", func(args ...any) (any, error) {
		if len(args) < 3 {
			return "void", nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		z := int(toAIFloat(args[2]))
		switch level.GetTerrainKind(x, y, z) {
		case world.TKSurface:
			return "surface", nil
		case world.TKSubsurface:
			return "subsurface", nil
		case world.TKUnderground, world.TKCavern, world.TKBedrock:
			return "underground", nil
		case world.TKAtmosphere:
			return "atmosphere", nil
		case world.TKSpace:
			return "space", nil
		default:
			return "void", nil
		}
	})

	// get_surface_z(x, y) — returns the surface z level for the column at (x, y).
	interp.RegisterFunc("get_surface_z", func(args ...any) (any, error) {
		if len(args) < 2 {
			return float64(0), nil
		}
		x := int(toAIFloat(args[0]))
		y := int(toAIFloat(args[1]))
		return float64(level.GetSurfaceZ(x, y)), nil
	})

	// burrow_by(dx, dy, dz) — move through solid tiles (ignores solid middle).
	// Only usable by entities with the burrowing skill. Returns 1 on success.
	interp.RegisterFunc("burrow_by", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		if !skills.Has(entity, fspath.BurrowingSkill) {
			return float64(0), nil
		}
		dx := int(toAIFloat(args[0]))
		dy := int(toAIFloat(args[1]))
		dz := int(toAIFloat(args[2]))
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		destX := pc.GetX() + dx
		destY := pc.GetY() + dy
		destZ := pc.GetZ() + dz
		dest := level.GetTilePtr(destX, destY, destZ)
		if dest == nil {
			return float64(0), nil
		}
		// Block space and void; allow everything else (open air, cave air, solid rock).
		tk := level.GetTerrainKind(destX, destY, destZ)
		if tk == world.TKSpace || tk == world.TKVoid {
			return float64(0), nil
		}
		level.PlaceEntity(destX, destY, destZ, entity)
		rlentity.Face(entity, dx, dy)
		return float64(1), nil
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

	// play_sound(event, loudness?) — emit this entity's sound for a named event
	// (e.g. "alert"), resolved through its SoundComponent + equipped gear, at its
	// current tile so the audio bridge plays it positionally. Optional loudness
	// (default 8) sets how far it carries for AI hearing. A no-op when the entity
	// has no clip for the event, so scripts can call it unconditionally.
	interp.RegisterFunc("play_sound", func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, nil
		}
		clip := components.ResolveSound(entity, fmt.Sprint(args[0]))
		if clip == "" {
			return nil, nil
		}
		loudness := float32(8)
		if len(args) >= 2 {
			loudness = float32(toAIFloat(args[1]))
		}
		pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		level.EmitSoundClip(pc.GetX(), pc.GetY(), pc.GetZ(), loudness, world.SoundTag(fmt.Sprint(args[0])), clip, entity)
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

func buildIgnored(selfFaction string, ai *components.ScriptedAIComponent) map[string]bool {
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
	return ignored
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
