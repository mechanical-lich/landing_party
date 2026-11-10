package systems

import (
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/stretchr/testify/require"
)

// The shared hive egg script hatches either morph through spawn_kin: a typed
// flier egg of a presence leaves a cracked egg and a hatchling of the same
// morph, type and presence.
func TestHiveEggHatchesKin(t *testing.T) {
	require.NoError(t, factory.FactoryLoad("../../data/blueprints/entities/hive.json"))
	lvl := wormLevel(8, 8, 3, func(x, y int) int { return 1 })
	egg, err := factory.CreateTyped("troid_egg", 3, 3, 2, "spitter", "fp1")
	require.NoError(t, err)
	lvl.AddEntity(egg)
	src, err := os.ReadFile("../../data/scripts/hive/egg.basic")
	require.NoError(t, err)
	ai := egg.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)
	ai.Vars = map[string]any{}
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, egg, lvl, ai)
	require.NoError(t, interp.Load(string(src)))
	for i := 0; i < 20; i++ {
		turn(t, interp)
	}
	got := map[string]*components.KinComponent{}
	for _, e := range append(lvl.Entities, lvl.StaticEntities...) {
		if e != egg {
			got[e.Blueprint] = components.KinOf(e)
		}
	}
	for _, bp := range []string{"troid_egg_cracked", "troid_hatchling"} {
		k := got[bp]
		if k == nil || k.Type != "spitter" || k.Presence != "fp1" || k.Morph != "flier" {
			t.Errorf("%s: kin %+v (level holds %v)", bp, k, got)
		}
	}
}
