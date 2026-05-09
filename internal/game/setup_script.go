package game

import (
	"fmt"
	"log"
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
