package generation

import (
	"math/rand"
	"testing"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// Hazard spawners anchored at an origin put their count there, on a small and
// a large map alike: "2 eggs" is 2, next to the origin, never scaled by area.
func TestContentAnchoredSpawnersStayAtOrigin(t *testing.T) {
	loadPlacementData(t)
	for _, size := range []int{64, 200} {
		for _, kind := range []string{"derelict_pod", "fauna_spawner"} {
			level := flatLevel(t, size, size)
			level.FeatureAreaScale = float64(size*size) / (64 * 64)
			ResolveOrigins(level, map[string][2]int{"ev1": {size / 2, size / 2}}, "ev1")
			ox, oy, _, _ := Origin(level, "ev1")
			spec := FeatureSpec{Kind: kind, Count: 2, At: "origin:ev1", Params: map[string]any{"blueprint": "xeno_egg"}}
			if err := GetFeature(kind)(level, spec, rand.New(rand.NewSource(5))); err != nil {
				t.Fatal(err)
			}
			near, far := 0, 0
			for _, e := range level.Entities {
				if e == nil || e.Blueprint != "xeno_egg" {
					continue
				}
				pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				if abs(pc.GetX()-ox) <= 8 && abs(pc.GetY()-oy) <= 8 {
					near++
				} else {
					far++
				}
			}
			if near != 2 || far != 0 {
				t.Errorf("%s on %dx%d: %d eggs at the origin, %d elsewhere; want 2 and 0", kind, size, size, near, far)
			}
			if !Anchorable(kind) {
				t.Errorf("%s honours at but is not listed anchorable", kind)
			}
		}
	}
}
