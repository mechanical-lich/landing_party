package game

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
)

// Mirroring a structure script (the builtin mirror): a script calls
// mirror(x, y, w, h, flip_x, flip_y) once, typically from rnd_int, and every
// coordinate it then passes to a builtin is reflected within that rect, so
// one layout comes out in up to four orientations without the script
// rewriting its coordinates. Coordinates builtins return (find_open_tile,
// regions, caves) are reflected back into script space. A structure the
// script builds with gen_structure inherits the reflection; one it set
// itself applies inside it and ends with it. A multi-tile entity (a bed) is
// re-anchored so it covers the reflection of what it would have covered.

// mirrorFlip is one reflection: within [x0, x0+w) when fx, [y0, y0+h) when fy.
type mirrorFlip struct {
	x0, y0, w, h int
	fx, fy       bool
}

func (m mirrorFlip) x(x int) int {
	if m.fx {
		return 2*m.x0 + m.w - 1 - x
	}
	return x
}

func (m mirrorFlip) y(y int) int {
	if m.fy {
		return 2*m.y0 + m.h - 1 - y
	}
	return y
}

// mirrorX/mirrorY map a script-space coordinate to the level: the innermost
// frame's flips first, then each enclosing frame's. unmirrorX/unmirrorY take
// a level coordinate back to script space (the same reflections, outermost
// first).
func (c *setupContext) mirrorX(x int) int   { return c.mapCoord(x, true, false) }
func (c *setupContext) mirrorY(y int) int   { return c.mapCoord(y, false, false) }
func (c *setupContext) unmirrorX(x int) int { return c.mapCoord(x, true, true) }
func (c *setupContext) unmirrorY(y int) int { return c.mapCoord(y, false, true) }

func (c *setupContext) mapCoord(v int, isX, inverse bool) int {
	var flips []mirrorFlip
	for i := len(c.mirrors) - 1; i >= 0; i-- {
		for j := len(c.mirrors[i]) - 1; j >= 0; j-- {
			flips = append(flips, c.mirrors[i][j])
		}
	}
	if inverse {
		for i, j := 0, len(flips)-1; i < j; i, j = i+1, j-1 {
			flips[i], flips[j] = flips[j], flips[i]
		}
	}
	for _, m := range flips {
		if isX {
			v = m.x(v)
		} else {
			v = m.y(v)
		}
	}
	return v
}

func (c *setupContext) mirrored() bool {
	for _, f := range c.mirrors {
		if len(f) > 0 {
			return true
		}
	}
	return false
}

// mirrorArgs says where a builtin's coordinates are: x at pt (y at pt+1);
// rect, a w and h at pt+3, pt+4 (x, y, z, w, h); entity, a blueprint at
// args[0] whose Size re-anchors it; out, a returned "x" or "y".
type mirrorArgs struct {
	pt     int
	rect   bool
	entity bool
	out    byte
}

var mirrorSpecs = map[string]mirrorArgs{
	"set_tile": {pt: 0}, "get_tile_type": {pt: 0}, "clear_middle": {pt: 0},
	"add_epithet": {pt: 0}, "mark_quest_target": {pt: 0},
	"find_open_tile_near": {pt: 0}, "find_cave_tile_near": {pt: 0},
	"set_radiation": {pt: 0}, "place_radiation_blob": {pt: 0},
	"get_biome": {pt: 0}, "get_surface_z": {pt: 0},
	"carve_round_room": {pt: 0}, "carve_circle": {pt: 0},
	"spawn_lore_datapad": {pt: 0}, "fill_container": {pt: 0},
	"carve_room": {pt: 0, rect: true}, "carve_rect": {pt: 0, rect: true}, "name_room": {pt: 0, rect: true},
	"spawn_entity": {pt: 1, entity: true}, "spawn_entity_owned": {pt: 1, entity: true},
	"spawn_entity_named": {pt: 1, entity: true}, "tag_region": {pt: 1},
	"get_open_x": {pt: -1, out: 'x'}, "get_open_y": {pt: -1, out: 'y'},
	"get_cave_x": {pt: -1, out: 'x'}, "get_cave_y": {pt: -1, out: 'y'},
	"get_region_x": {pt: -1, out: 'x'}, "get_region_y": {pt: -1, out: 'y'},
}

// entitySizes caches blueprint footprints for re-anchoring ([w, h]).
var entitySizes = map[string][2]int{}

func entitySize(bp string) [2]int {
	if s, ok := entitySizes[bp]; ok {
		return s
	}
	s := [2]int{1, 1}
	if sc := factory.GetSize(bp); sc != nil {
		s = [2]int{max(sc.Width, 1), max(sc.Height, 1)}
	}
	entitySizes[bp] = s
	return s
}

// registerMirrored registers a builtin, reflecting its coordinates through
// the context's mirrors when it takes any.
func registerMirrored(interp *basic.MechBasic, ctx *setupContext, name string, fn func(args ...any) (any, error)) {
	spec, ok := mirrorSpecs[name]
	if !ok {
		interp.RegisterFunc(name, fn)
		return
	}
	interp.RegisterFunc(name, func(args ...any) (any, error) {
		if !ctx.mirrored() {
			return fn(args...)
		}
		if spec.out != 0 {
			v, err := fn(args...)
			if f, isNum := v.(float64); isNum && err == nil {
				if spec.out == 'x' {
					return float64(ctx.unmirrorX(int(f))), nil
				}
				return float64(ctx.unmirrorY(int(f))), nil
			}
			return v, err
		}
		if len(args) <= spec.pt+1 {
			return fn(args...)
		}
		a := append([]any(nil), args...)
		x, y := int(toSetupFloat(a[spec.pt])), int(toSetupFloat(a[spec.pt+1]))
		switch {
		case spec.rect && len(a) > spec.pt+4:
			// A rect maps to the rect spanning its corners' reflections.
			w, h := int(toSetupFloat(a[spec.pt+3])), int(toSetupFloat(a[spec.pt+4]))
			x1, y1 := ctx.mirrorX(x), ctx.mirrorY(y)
			x2, y2 := ctx.mirrorX(x+w-1), ctx.mirrorY(y+h-1)
			x, y = min(x1, x2), min(y1, y2)
		case spec.entity:
			// The entity covers [x-W/2, x-W/2+W); its reflection is covered
			// from the reflected far edge, so re-anchor from there.
			s := entitySize(fmt.Sprint(a[0]))
			x0, y0 := x-s[0]/2, y-s[1]/2
			nx0 := min(ctx.mirrorX(x0), ctx.mirrorX(x0+s[0]-1))
			ny0 := min(ctx.mirrorY(y0), ctx.mirrorY(y0+s[1]-1))
			x, y = nx0+s[0]/2, ny0+s[1]/2
		default:
			x, y = ctx.mirrorX(x), ctx.mirrorY(y)
		}
		a[spec.pt], a[spec.pt+1] = float64(x), float64(y)
		return fn(a...)
	})
}

// registerMirror adds the builtin mirror(x, y, w, h, flip_x, flip_y): from
// here to the end of the calling structure, coordinates reflect within the
// rect on each axis whose flag is non-zero. The rect is given in the
// script's own coordinates (it is itself mapped through enclosing mirrors).
func registerMirror(interp *basic.MechBasic, ctx *setupContext) {
	interp.RegisterFunc("mirror", func(args ...any) (any, error) {
		if len(args) < 6 {
			return nil, nil
		}
		x, y := int(toSetupFloat(args[0])), int(toSetupFloat(args[1]))
		w, h := int(toSetupFloat(args[2])), int(toSetupFloat(args[3]))
		fx, fy := toSetupFloat(args[4]) != 0, toSetupFloat(args[5]) != 0
		if !fx && !fy {
			return nil, nil
		}
		if len(ctx.mirrors) == 0 {
			ctx.mirrors = append(ctx.mirrors, nil)
		}
		top := len(ctx.mirrors) - 1
		ctx.mirrors[top] = append(ctx.mirrors[top], mirrorFlip{x0: x, y0: y, w: w, h: h, fx: fx, fy: fy})
		return nil, nil
	})
}
