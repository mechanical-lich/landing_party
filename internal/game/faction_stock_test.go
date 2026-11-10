package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
)

// A camp with no container still gets its stockpile: on the ground by its
// centre (secondary review note: the items were created and never placed,
// so the stockpile was lost on the first build).
func TestFactionStockWithNoContainerLandsOnTheGround(t *testing.T) {
	useContentTables(t)
	ms, _ := factionTestSite(t, "moon", 7, 4, 0)
	cx, cy := ms.level.GetWidth()/2, ms.level.GetHeight()/2
	cz := ms.level.GetSurfaceZ(cx, cy)
	before := map[string]int{}
	count := func() map[string]int {
		out := map[string]int{}
		for _, e := range ms.level.Entities {
			if e != nil && e.HasComponent(components.Material) {
				if _, ok := entityPos(e); ok {
					out[e.Blueprint] += e.GetComponent(components.Material).(*components.MaterialComponent).Quantity
				}
			}
		}
		return out
	}
	before = count()
	fillFactionStock(ms.level, "nobody:site", map[string]int{"metal_ore": 7, "crystal": 3}, cx, cy, cz)
	after := count()
	if after["metal_ore"]-before["metal_ore"] != 7 || after["crystal"]-before["crystal"] != 3 {
		t.Errorf("stock on the level: metal %d -> %d, crystal %d -> %d; want +7 and +3",
			before["metal_ore"], after["metal_ore"], before["crystal"], after["crystal"])
	}
}
