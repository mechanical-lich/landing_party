package game

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// contentASCII draws a footprint (and a 1-tile margin): # wall, o ore,
// r rubble, ~ furrow, e entity, . floor, space ground.
func contentASCII(level *world.Level, x0, y0, w, h, z int) string {
	var sb strings.Builder
	for y := y0 - 1; y <= y0+h; y++ {
		for x := x0 - 1; x <= x0+w; x++ {
			t := level.GetTilePtr(x, y, z)
			c := " "
			mid, floor := "", ""
			if t != nil && !t.Middle.IsEmpty() {
				mid = world.TileDefinitions[t.Middle.Type].Name
			}
			if t != nil && !t.Floor.IsEmpty() {
				floor = world.TileDefinitions[t.Floor.Type].Name
			}
			switch {
			case level.GetEntityAt(x, y, z) != nil:
				c = "e"
			case strings.Contains(mid, "ore"):
				c = "o"
			case mid == "rubble_pile":
				c = "r"
			case mid != "" && t.IsSolid():
				c = "#"
			case floor == "crater_ground":
				c = "~"
			case strings.HasSuffix(floor, "_floor"):
				c = "."
			}
			sb.WriteString(c)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// CONTENT_VIEW="script w h key=value,..." prints one build (a developer aid).
func TestContentView(t *testing.T) {
	spec := os.Getenv("CONTENT_VIEW")
	if spec == "" {
		t.Skip("set CONTENT_VIEW=\"script w h [k=v,...]\"")
	}
	f := strings.Fields(spec)
	w, _ := strconv.Atoi(f[1])
	h, _ := strconv.Atoi(f[2])
	params := map[string]any{"occupants": "none"}
	if len(f) > 3 {
		for _, kv := range strings.Split(f[3], ",") {
			k, v, _ := strings.Cut(kv, "=")
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				params[k] = n
			} else {
				params[k] = v
			}
		}
	}
	loreLoadData(t)
	level := loreFlatLevel(t, w+10, h+10)
	start := time.Now()
	if err := RunStructureScriptWithParams(level, f[0], 5, 5, w, h, params, 1); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s %dx%d %v in %v\n%s", f[0], w, h, params, time.Since(start), contentASCII(level, 5, 5, w, h, level.GetSurfaceZ(5+w/2, 5+h/2)))
}
