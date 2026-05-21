package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/message"
)

type setupContext struct {
	Level *world.Level

	// paramStack holds one frame per active gen_structure call. set_param/
	// get_param operate on the top frame; gen_structure pushes a copy of the
	// caller's frame so seeded params flow down without children leaking up.
	paramStack []map[string]any

	// genDepth guards against runaway recursion (a structure that calls
	// itself, directly or via a cycle).
	genDepth int

	// bindTarget, when set, lets mark_quest_target push the script-chosen NPC
	// name back into the campaign quest. nil for plain setup scripts.
	bindTarget func(questID, npc string)
}

const maxGenDepth = 8

func (c *setupContext) topParams() map[string]any {
	if len(c.paramStack) == 0 {
		c.paramStack = append(c.paramStack, map[string]any{})
	}
	return c.paramStack[len(c.paramStack)-1]
}

func (c *setupContext) pushParamsCopy() {
	src := c.topParams()
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	c.paramStack = append(c.paramStack, dst)
}

func (c *setupContext) popParams() {
	if len(c.paramStack) > 0 {
		c.paramStack = c.paramStack[:len(c.paramStack)-1]
	}
}

// structScriptCache memoizes structure script source by name so repeated
// stamping reads each file once (the interpreter still re-parses per call —
// fresh interp per invocation — but file I/O is amortized).
var structScriptCache = map[string]string{}

// runGenStructure resolves name → data/scripts/structures/<name>.basic, builds
// a fresh interpreter with the full builtin set (including gen_structure
// itself, so structures compose), and calls its generate(x,y,w,h) entrypoint.
func runGenStructure(ctx *setupContext, name string, x, y, w, h int) error {
	if ctx.genDepth >= maxGenDepth {
		return fmt.Errorf("gen_structure %q: max recursion depth %d exceeded", name, maxGenDepth)
	}
	code, ok := structScriptCache[name]
	if !ok {
		path := filepath.Join("data", "scripts", "structures", name+".basic")
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("gen_structure %q: %w", name, err)
		}
		code = string(b)
		structScriptCache[name] = code
	}
	interp := basic.NewMechanicalBasic()
	registerSetupFuncs(interp, ctx)
	if err := interp.Load(code); err != nil {
		return fmt.Errorf("gen_structure %q load: %w", name, err)
	}
	if !interp.HasFunction("generate") {
		return fmt.Errorf("gen_structure %q: missing function generate(x,y,w,h)", name)
	}
	ctx.pushParamsCopy()
	ctx.genDepth++
	_, err := interp.Call("generate", float64(x), float64(y), float64(w), float64(h))
	ctx.genDepth--
	ctx.popParams()
	if err != nil {
		return fmt.Errorf("gen_structure %q: %w", name, err)
	}
	return nil
}

// ClearStructureScriptCache drops cached structure-script source. Tools that
// edit and reload a script while running (e.g. structure-viewer) call this
// between stamps so changes on disk take effect.
func ClearStructureScriptCache() {
	structScriptCache = map[string]string{}
}

// RunStructureScript stamps a named structure script onto level at (x,y) with
// the given width and height. Used by tools (e.g. structure-viewer) that want
// to invoke the same script dispatch the runtime generator uses, without a
// surrounding scenario / quest context.
func RunStructureScript(level *world.Level, name string, x, y, w, h int) error {
	ctx := &setupContext{Level: level}
	return runGenStructure(ctx, name, x, y, w, h)
}

// RunSetupScripts executes each .basic file listed in the scenario's setup_scripts.
// Each script must define function on_setup().
func RunSetupScripts(scripts []string, level *world.Level) {
	for _, path := range scripts {
		if err := runSetupScript(path, level); err != nil {
			log.Printf("setup script %s: %v", path, err)
		}
	}
}

func runSetupScript(path string, level *world.Level) error {
	code, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	ctx := &setupContext{Level: level}
	interp := basic.NewMechanicalBasic()
	registerSetupFuncs(interp, ctx)
	if err := interp.Load(string(code)); err != nil {
		return fmt.Errorf("load: %w", err)
	}
	if !interp.HasFunction("on_setup") {
		return fmt.Errorf("missing on_setup function")
	}
	_, err = interp.Call("on_setup")
	return err
}

func registerSetupFuncs(interp *basic.MechBasic, ctx *setupContext) {
	level := ctx.Level

	interp.RegisterFunc("get_z_count", func(args ...any) (any, error) {
		return float64(level.GetDepth()), nil
	})
	interp.RegisterFunc("get_width", func(args ...any) (any, error) {
		return float64(level.GetWidth()), nil
	})
	interp.RegisterFunc("get_height", func(args ...any) (any, error) {
		return float64(level.GetHeight()), nil
	})

	interp.RegisterFunc("set_tile", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		tileName := fmt.Sprint(args[3])
		tI := level.GetTileAt(x, y, z)
		if tI == nil {
			return nil, nil
		}
		t := tI.(*world.Tile)
		world.SetTileTypeAndVariant(t, tileName, world.RandomTileVariant(tileName))
		return nil, nil
	})

	interp.RegisterFunc("get_tile_type", func(args ...any) (any, error) {
		if len(args) < 3 {
			return "", nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		tI := level.GetTileAt(x, y, z)
		if tI == nil {
			return "", nil
		}
		t := tI.(*world.Tile)
		// Prefer Middle (walls/ore), then Floor (ground), else empty.
		slot := t.Middle
		if slot.IsEmpty() {
			slot = t.Floor
		}
		if slot.IsEmpty() {
			return "", nil
		}
		return world.TileIndexToName[slot.Type], nil
	})

	interp.RegisterFunc("spawn_entity", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		bp := fmt.Sprint(args[0])
		x := int(toSetupFloat(args[1]))
		y := int(toSetupFloat(args[2]))
		z := int(toSetupFloat(args[3]))
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			return nil, nil
		}
		level.AddEntity(e)
		return nil, nil
	})

	// spawn_entity_owned(blueprint, x, y, z, owner) — like spawn_entity but sets
	// OwnedBy on Door, Storage, and Workbench components.
	interp.RegisterFunc("spawn_entity_owned", func(args ...any) (any, error) {
		if len(args) < 5 {
			return nil, nil
		}
		bp := fmt.Sprint(args[0])
		x := int(toSetupFloat(args[1]))
		y := int(toSetupFloat(args[2]))
		z := int(toSetupFloat(args[3]))
		owner := fmt.Sprint(args[4])
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			return nil, nil
		}
		if e.HasComponent(rlcomponents.Door) {
			e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).OwnedBy = owner
			// Door must sit in a walkable cell: floor underneath, nothing
			// blocking the middle. Clear the wall that carve_room stamped.
			level.SetFloor(x, y, z, "hull_floor", world.RandomTileVariant("hull_floor"))
			level.ClearMiddle(x, y, z)
		}
		if e.HasComponent(components.Storage) {
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = owner
		}

		level.AddEntity(e)
		return nil, nil
	})

	// spawn_entity_named(blueprint, x, y, z, name) — spawn and set its display
	// name. For guards / flavor NPCs a structure wants to customize.
	interp.RegisterFunc("spawn_entity_named", func(args ...any) (any, error) {
		if len(args) < 5 {
			return float64(0), nil
		}
		bp := fmt.Sprint(args[0])
		x := int(toSetupFloat(args[1]))
		y := int(toSetupFloat(args[2]))
		z := int(toSetupFloat(args[3]))
		name := fmt.Sprint(args[4])
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			return float64(0), nil
		}
		if name != "" {
			if e.HasComponent(rlcomponents.Description) {
				e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name = name
			} else {
				e.AddComponent(&rlcomponents.DescriptionComponent{Name: name})
			}
		}
		level.AddEntity(e)
		return float64(1), nil
	})

	// add_epithet(x, y, z) — decorate the name of the entity at (x,y,z) with a
	// random epithet, e.g. "Warden of the Sealed Bunker the Destroyer". Call
	// before mark_quest_target so the quest title adopts the boss name.
	interp.RegisterFunc("add_epithet", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		e := level.GetEntityAt(x, y, z)
		if e == nil {
			log.Printf("add_epithet: no entity at [%d,%d,%d]", x, y, z)
			return float64(0), nil
		}
		if e.HasComponent(rlcomponents.Description) {
			dc := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
			dc.Name = lore.NameWithEpithet(dc.Name)
		} else {
			e.AddComponent(&rlcomponents.DescriptionComponent{Name: lore.NameWithEpithet("")})
		}
		return float64(1), nil
	})

	// mark_quest_target(x, y, z) — designate the entity already standing at
	// (x,y,z) as this structure's quest target. Tags it with the param
	// "quest_id" so target_killed fires, and feeds its display name back into
	// the campaign so the quest title adopts the script-chosen name.
	interp.RegisterFunc("mark_quest_target", func(args ...any) (any, error) {
		if len(args) < 3 {
			return float64(0), nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		e := level.GetEntityAt(x, y, z)
		if e == nil {
			log.Printf("mark_quest_target: no entity at [%d,%d,%d]", x, y, z)
			return float64(0), nil
		}
		qid := ""
		if v, ok := ctx.topParams()["quest_id"]; ok {
			qid = fmt.Sprint(v)
		}
		if qid == "" {
			return float64(0), nil
		}
		e.AddComponent(&components.QuestTargetComponent{QuestID: qid})
		npc := ""
		if e.HasComponent(rlcomponents.Description) {
			npc = e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Name
		}
		if ctx.bindTarget != nil {
			ctx.bindTarget(qid, npc)
		}
		return float64(1), nil
	})

	var lastOpenX, lastOpenY int
	interp.RegisterFunc("find_open_tile", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		z := int(toSetupFloat(args[0]))
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
	interp.RegisterFunc("get_open_x", func(args ...any) (any, error) {
		return float64(lastOpenX), nil
	})
	interp.RegisterFunc("get_open_y", func(args ...any) (any, error) {
		return float64(lastOpenY), nil
	})

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

	interp.RegisterFunc("add_message", func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, nil
		}
		message.PostMessage("world", fmt.Sprint(args[0]))
		return nil, nil
	})

	interp.RegisterFunc("num_to_str", func(args ...any) (any, error) {
		if len(args) < 1 {
			return "", nil
		}
		return fmt.Sprint(args[0]), nil
	})
	interp.RegisterFunc("chr", func(args ...any) (any, error) {
		if len(args) < 1 {
			return "", nil
		}
		return string(rune(int(toSetupFloat(args[0])))), nil
	})
	interp.RegisterFunc("rnd_int", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		n := int(toSetupFloat(args[0]))
		if n <= 0 {
			return float64(0), nil
		}
		return float64(rand.Intn(n)), nil
	})

	// set_radiation(x, y, z, level) — set a single tile's radiation (0..255).
	interp.RegisterFunc("set_radiation", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		lv := int(toSetupFloat(args[3]))
		if lv < 0 {
			lv = 0
		} else if lv > 255 {
			lv = 255
		}
		t := level.GetTilePtr(x, y, z)
		if t == nil {
			return nil, nil
		}
		t.Radiation = uint8(lv)
		return nil, nil
	})

	// place_radiation_blob(cx, cy, z, radius, peak) — circular blob with linear
	// falloff from `peak` at the center to 0 at the edge. Existing radiation
	// is preserved if higher (blobs add, never reduce).
	interp.RegisterFunc("place_radiation_blob", func(args ...any) (any, error) {
		if len(args) < 5 {
			return nil, nil
		}
		cx := int(toSetupFloat(args[0]))
		cy := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		r := int(toSetupFloat(args[3]))
		peak := int(toSetupFloat(args[4]))
		if r < 1 || peak <= 0 {
			return nil, nil
		}
		if peak > 255 {
			peak = 255
		}
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				dist2 := dx*dx + dy*dy
				if dist2 > r*r {
					continue
				}
				t := level.GetTilePtr(cx+dx, cy+dy, z)
				if t == nil {
					continue
				}
				dist := math.Sqrt(float64(dist2))
				falloff := 1.0 - dist/float64(r)
				lv := int(float64(peak) * falloff)
				if lv > int(t.Radiation) {
					t.Radiation = uint8(lv)
				}
			}
		}
		return nil, nil
	})

	// scatter_radiation(count, max_radius, peak) — drop `count` random blobs
	// across the current Z level. Convenience wrapper that the scenario can
	// call once instead of looping in script. Each blob also seeds 1–3
	// radioactive_ore tiles near its centre to represent the leak's source.
	interp.RegisterFunc("scatter_radiation", func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		count := int(toSetupFloat(args[0]))
		maxR := int(toSetupFloat(args[1]))
		peak := int(toSetupFloat(args[2]))
		if count <= 0 || maxR < 1 || peak <= 0 {
			return nil, nil
		}
		if peak > 255 {
			peak = 255
		}
		w, h, depth := level.GetWidth(), level.GetHeight(), level.GetDepth()
		for i := 0; i < count; i++ {
			cx := rand.Intn(w)
			cy := rand.Intn(h)
			z := rand.Intn(depth)
			r := 1 + rand.Intn(maxR)
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					dist2 := dx*dx + dy*dy
					if dist2 > r*r {
						continue
					}
					t := level.GetTilePtr(cx+dx, cy+dy, z)
					if t == nil {
						continue
					}
					dist := math.Sqrt(float64(dist2))
					lv := int(float64(peak) * (1.0 - dist/float64(r)))
					if lv > int(t.Radiation) {
						t.Radiation = uint8(lv)
					}
				}
			}
			// Plant 1–3 radioactive_ore tiles within a 1-tile cluster at
			// the centre, only on tiles that aren't air/space/water.
			oreSeeds := 1 + rand.Intn(3)
			for j := 0; j < oreSeeds; j++ {
				ox := cx + rand.Intn(3) - 1
				oy := cy + rand.Intn(3) - 1
				t := level.GetTilePtr(ox, oy, z)
				if t == nil || t.Middle.IsEmpty() {
					continue
				}
				def := world.TileDefinitions[t.Middle.Type]
				if def.Air || def.Space || def.Water {
					continue
				}
				world.SetTileTypeAndVariant(t, "radioactive_ore", world.RandomTileVariant("radioactive_ore"))
			}
		}
		return nil, nil
	})

	// get_biome(x, y) — returns the biome ID at column (x,y), or "".
	interp.RegisterFunc("get_biome", func(args ...any) (any, error) {
		if len(args) < 2 {
			return "", nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		return level.GetBiome(x, y), nil
	})

	// get_surface_z(x, y) — returns the surface Z for the column, -1 if none.
	interp.RegisterFunc("get_surface_z", func(args ...any) (any, error) {
		if len(args) < 2 {
			return float64(-1), nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		return float64(level.GetSurfaceZ(x, y)), nil
	})

	// find_cave_tile() — random search for a TKCavern tile anywhere in the level.
	// Returns 1 on success; use get_cave_x/y/z to retrieve the coords.
	var lastCaveX, lastCaveY, lastCaveZ int
	interp.RegisterFunc("find_cave_tile", func(args ...any) (any, error) {
		w := level.GetWidth()
		h := level.GetHeight()
		depth := level.GetDepth()
		for attempt := 0; attempt < 500; attempt++ {
			x := rand.Intn(w)
			y := rand.Intn(h)
			z := rand.Intn(depth)
			if level.GetTerrainKind(x, y, z) != world.TKCavern {
				continue
			}
			tI := level.GetTileAt(x, y, z)
			if tI == nil {
				continue
			}
			t := tI.(*world.Tile)
			if t.IsSolid() || t.IsWater() || level.GetEntityAt(x, y, z) != nil {
				continue
			}
			lastCaveX = x
			lastCaveY = y
			lastCaveZ = z
			return float64(1), nil
		}
		return float64(0), nil
	})
	interp.RegisterFunc("get_cave_x", func(args ...any) (any, error) { return float64(lastCaveX), nil })
	interp.RegisterFunc("get_cave_y", func(args ...any) (any, error) { return float64(lastCaveY), nil })
	interp.RegisterFunc("get_cave_z", func(args ...any) (any, error) { return float64(lastCaveZ), nil })

	// tag_region(name, x, y, z) — record an anchor point for later lookup.
	interp.RegisterFunc("tag_region", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		name := fmt.Sprint(args[0])
		x := int(toSetupFloat(args[1]))
		y := int(toSetupFloat(args[2]))
		z := int(toSetupFloat(args[3]))
		level.TagRegion(name, x, y, z)
		return nil, nil
	})

	// get_region_count(name) — number of anchors for a tag.
	interp.RegisterFunc("get_region_count", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		name := fmt.Sprint(args[0])
		return float64(len(level.Regions[name])), nil
	})

	// get_region_x(name, i) / _y / _z — read the i-th anchor.
	interp.RegisterFunc("get_region_x", func(args ...any) (any, error) {
		return regionCoord(level, args, 0)
	})
	interp.RegisterFunc("get_region_y", func(args ...any) (any, error) {
		return regionCoord(level, args, 1)
	})
	interp.RegisterFunc("get_region_z", func(args ...any) (any, error) {
		return regionCoord(level, args, 2)
	})

	// carve_room(x, y, z, w, h, wall_tile, floor_tile) — bordered room. The
	// floor tile is always stamped (Floor slot) so destroyed walls leave a
	// walkable cell; wall tiles are also stamped on edges (Middle slot).
	interp.RegisterFunc("carve_room", func(args ...any) (any, error) {
		if len(args) < 7 {
			return nil, nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		w := int(toSetupFloat(args[3]))
		h := int(toSetupFloat(args[4]))
		wallTile := fmt.Sprint(args[5])
		floorTile := fmt.Sprint(args[6])
		for dy := 0; dy < h; dy++ {
			for dx := 0; dx < w; dx++ {
				edge := dx == 0 || dy == 0 || dx == w-1 || dy == h-1
				level.SetFloor(x+dx, y+dy, z, floorTile, world.RandomTileVariant(floorTile))
				if edge {
					level.SetMiddle(x+dx, y+dy, z, wallTile, world.RandomTileVariant(wallTile))
				} else {
					level.ClearMiddle(x+dx, y+dy, z)
				}
			}
		}
		return nil, nil
	})

	// carve_rect(x, y, z, w, h, tile) — fill a rectangle (no border).
	interp.RegisterFunc("carve_rect", func(args ...any) (any, error) {
		if len(args) < 6 {
			return nil, nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		w := int(toSetupFloat(args[3]))
		h := int(toSetupFloat(args[4]))
		tile := fmt.Sprint(args[5])
		for dy := 0; dy < h; dy++ {
			for dx := 0; dx < w; dx++ {
				level.UpdateTileAt(x+dx, y+dy, z, tile, world.RandomTileVariant(tile))
			}
		}
		return nil, nil
	})

	// clear_middle(x, y, z) — remove whatever occupies the blocking Middle
	// slot (wall/ore), leaving the Floor intact. Use to knock a hole in a
	// wall or open a passage; the cell becomes walkable if it has a floor.
	interp.RegisterFunc("clear_middle", func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		x := int(toSetupFloat(args[0]))
		y := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		level.ClearMiddle(x, y, z)
		return nil, nil
	})

	// carve_circle(cx, cy, z, radius, tile) — disc fill.
	interp.RegisterFunc("carve_circle", func(args ...any) (any, error) {
		if len(args) < 5 {
			return nil, nil
		}
		cx := int(toSetupFloat(args[0]))
		cy := int(toSetupFloat(args[1]))
		z := int(toSetupFloat(args[2]))
		r := int(toSetupFloat(args[3]))
		tile := fmt.Sprint(args[4])
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r {
					level.UpdateTileAt(cx+dx, cy+dy, z, tile, world.RandomTileVariant(tile))
				}
			}
		}
		return nil, nil
	})

	// place_feature(kind, count) — invoke a registered feature placer with
	// no biome restriction and default params. For richer placement use the
	// scenario JSON's features block.
	interp.RegisterFunc("place_feature", func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		kind := fmt.Sprint(args[0])
		count := int(toSetupFloat(args[1]))
		spec := generation.FeatureSpec{Kind: kind, Count: count}
		if p := generation.GetFeature(kind); p != nil {
			_ = p(level, spec)
		}
		return nil, nil
	})

	// set_param(key, value) — stage a parameter for the next gen_structure
	// call. Children inherit a copy of the current frame.
	interp.RegisterFunc("set_param", func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		ctx.topParams()[fmt.Sprint(args[0])] = args[1]
		return nil, nil
	})
	// get_param(key) — read a parameter seeded by the caller. Returns "" if
	// unset (callers comparing numerically should add +0).
	interp.RegisterFunc("get_param", func(args ...any) (any, error) {
		if len(args) < 1 {
			return "", nil
		}
		if v, ok := ctx.topParams()[fmt.Sprint(args[0])]; ok {
			return v, nil
		}
		return "", nil
	})

	// gen_structure(name, x, y, w, h) — stamp a structure script. Reentrant:
	// a structure may call gen_structure to compose sub-structures. Returns 1
	// on success, 0 on failure (logged).
	interp.RegisterFunc("gen_structure", func(args ...any) (any, error) {
		if len(args) < 5 {
			return float64(0), nil
		}
		name := fmt.Sprint(args[0])
		x := int(toSetupFloat(args[1]))
		y := int(toSetupFloat(args[2]))
		w := int(toSetupFloat(args[3]))
		h := int(toSetupFloat(args[4]))
		if err := runGenStructure(ctx, name, x, y, w, h); err != nil {
			log.Printf("gen_structure: %v", err)
			return float64(0), nil
		}
		return float64(1), nil
	})
}

func regionCoord(level *world.Level, args []any, axis int) (any, error) {
	if len(args) < 2 {
		return float64(0), nil
	}
	name := fmt.Sprint(args[0])
	i := int(toSetupFloat(args[1]))
	pts := level.Regions[name]
	if i < 0 || i >= len(pts) {
		return float64(0), nil
	}
	return float64(pts[i][axis]), nil
}

func toSetupFloat(v any) float64 {
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
