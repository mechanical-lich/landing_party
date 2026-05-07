package systems

import (
	"fmt"
	"log"
	"math/rand"
	"os"

	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

type ScriptSystem struct {
	cache map[string]string // path → source code
}

var scriptSystemRequires = []ecs.ComponentType{components.Script}

func (s *ScriptSystem) Requires() []ecs.ComponentType { return scriptSystemRequires }

func (s *ScriptSystem) UpdateSystem(data interface{}) error { return nil }

func (s *ScriptSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)
	sc := entity.GetComponent(components.Script).(*components.ScriptComponent)
	if sc.OnTurn == "" {
		return nil
	}
	if s.cache == nil {
		s.cache = make(map[string]string)
	}
	src, ok := s.cache[sc.OnTurn]
	if !ok {
		raw, err := os.ReadFile(sc.OnTurn)
		if err != nil {
			log.Printf("ScriptSystem: read %s: %v", sc.OnTurn, err)
			sc.OnTurn = "" // disable broken script
			return nil
		}
		src = string(raw)
		s.cache[sc.OnTurn] = src
	}

	interp := basic.NewMechanicalBasic()
	registerEntityScriptFuncs(interp, entity, level)
	if err := interp.Load(src); err != nil {
		log.Printf("ScriptSystem: load %s: %v", sc.OnTurn, err)
		return nil
	}
	if !interp.HasFunction("on_turn") {
		return nil
	}
	if _, err := interp.Call("on_turn"); err != nil {
		log.Printf("ScriptSystem: on_turn %s: %v", sc.OnTurn, err)
	}
	return nil
}

func registerEntityScriptFuncs(interp *basic.MechBasic, entity *ecs.Entity, level *world.Level) {
	interp.RegisterFunc("get_z_count", func(args ...any) (any, error) {
		return float64(level.GetDepth()), nil
	})
	interp.RegisterFunc("get_width", func(args ...any) (any, error) {
		return float64(level.GetWidth()), nil
	})
	interp.RegisterFunc("get_height", func(args ...any) (any, error) {
		return float64(level.GetHeight()), nil
	})

	interp.RegisterFunc("spawn_entity", func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		bp := fmt.Sprint(args[0])
		x := int(toScriptFloat(args[1]))
		y := int(toScriptFloat(args[2]))
		z := int(toScriptFloat(args[3]))
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			return nil, nil
		}
		level.AddEntity(e)
		return nil, nil
	})

	var lastOpenX, lastOpenY int
	interp.RegisterFunc("find_open_tile", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		z := int(toScriptFloat(args[0]))
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
	interp.RegisterFunc("rnd_int", func(args ...any) (any, error) {
		if len(args) < 1 {
			return float64(0), nil
		}
		n := int(toScriptFloat(args[0]))
		if n <= 0 {
			return float64(0), nil
		}
		return float64(rand.Intn(n)), nil
	})
}

func toScriptFloat(v any) float64 {
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
