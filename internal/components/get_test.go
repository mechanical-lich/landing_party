package components

import (
	"testing"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

func TestGetPresentAndAbsent(t *testing.T) {
	e := &ecs.Entity{}
	e.AddComponent(&rlcomponents.PositionComponent{X: 3, Y: 4})

	// Value-receiver GetType (rlcomponents) resolves.
	if pc, ok := Get[rlcomponents.PositionComponent](e); !ok || pc.GetX() != 3 {
		t.Fatalf("Get[Position] = %v, ok=%v; want (3,..), true", pc, ok)
	}
	// Pointer-receiver GetType (this package) resolves.
	e.AddComponent(&SoundComponent{Material: "metal"})
	if sc, ok := Get[SoundComponent](e); !ok || sc.Material != "metal" {
		t.Fatalf("Get[Sound] = %v, ok=%v; want metal, true", sc, ok)
	}
	// Absent → (nil, false), no panic.
	if hc, ok := Get[rlcomponents.HealthComponent](e); ok || hc != nil {
		t.Fatalf("Get[Health] on entity without it = %v, ok=%v; want nil, false", hc, ok)
	}
	// nil entity → (nil, false).
	if _, ok := Get[rlcomponents.PositionComponent](nil); ok {
		t.Fatal("Get on nil entity should be false")
	}
}
