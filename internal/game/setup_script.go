package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"

	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

type setupContext struct {
	Level *world.Level
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
		return world.TileIndexToName[t.Type], nil
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
			// Door must sit on a passable tile so pathfinding can route through it
			if tI := level.GetTileAt(x, y, z); tI != nil {
				t := tI.(*world.Tile)
				world.SetTileTypeAndVariant(t, "hull_floor", world.RandomTileVariant("hull_floor"))
			}
		}
		if e.HasComponent(components.Storage) {
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = owner
		}

		level.AddEntity(e)
		return nil, nil
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
				if t == nil {
					continue
				}
				def := world.TileDefinitions[t.Type]
				if def.Air || def.Space || def.Water {
					continue
				}
				world.SetTileTypeAndVariant(t, "radioactive_ore", world.RandomTileVariant("radioactive_ore"))
			}
		}
		return nil, nil
	})
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
