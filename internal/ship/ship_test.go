package ship

import (
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

func TestMain(m *testing.M) {
	if err := world.LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		panic("load tile definitions: " + err.Error())
	}
	if err := factory.FactoryLoadDir("../../data/blueprints"); err != nil {
		panic("load blueprints: " + err.Error())
	}
	os.Exit(m.Run())
}

func midName(t *world.Tile) string {
	if t.Middle.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Middle.Type].Name
}

func floorName(t *world.Tile) string {
	if t.Floor.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Floor.Type].Name
}

func TestBuildShipLevel_LargeWithCentredHull(t *testing.T) {
	lvl := BuildShipLevel("ship")

	if lvl.GetWidth() != LevelWidth || lvl.GetHeight() != LevelHeight || lvl.GetDepth() != LevelDepth {
		t.Fatalf("ship size = %dx%dx%d, want %dx%dx%d",
			lvl.GetWidth(), lvl.GetHeight(), lvl.GetDepth(), LevelWidth, LevelHeight, LevelDepth)
	}

	ox, oy := (LevelWidth-HullSize)/2, (LevelHeight-HullSize)/2

	// Hull region: floor everywhere, walls on the perimeter, open inside.
	for dy := 0; dy < HullSize; dy++ {
		for dx := 0; dx < HullSize; dx++ {
			x, y := ox+dx, oy+dy
			tile := lvl.GetTilePtr(x, y, HullDeck)
			if got := floorName(tile); got != "hull_floor" {
				t.Fatalf("hull floor at (%d,%d) = %q, want hull_floor", x, y, got)
			}
			perimeter := dx == 0 || dy == 0 || dx == HullSize-1 || dy == HullSize-1
			mid := midName(tile)
			if perimeter && mid != "hull_wall" {
				t.Errorf("hull perimeter (%d,%d) = %q, want hull_wall", x, y, mid)
			}
			if !perimeter && mid != "" && !(x == ox+2 && y == oy+2) {
				t.Errorf("hull interior (%d,%d) = %q, want empty", x, y, mid)
			}
		}
	}

	// Outside the hull is empty void.
	if got := floorName(lvl.GetTilePtr(0, 0, HullDeck)); got != "" {
		t.Errorf("corner (0,0) floor = %q, want empty void", got)
	}
}

func TestBuildShipLevel_OneStorageLockerOwnedByShip(t *testing.T) {
	lvl := BuildShipLevel("ship")
	ox, oy := (LevelWidth-HullSize)/2, (LevelHeight-HullSize)/2

	var lockers []*ecs.Entity
	for _, e := range lvl.Entities {
		if e != nil && e.HasComponent(components.Storage) {
			lockers = append(lockers, e)
		}
	}
	if len(lockers) != 1 {
		t.Fatalf("ship should have exactly 1 storage container, got %d", len(lockers))
	}
	sc := lockers[0].GetComponent(components.Storage).(*components.StorageComponent)
	if sc.OwnedBy != "ship" {
		t.Errorf("locker OwnedBy = %q, want \"ship\"", sc.OwnedBy)
	}
	pc := lockers[0].GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if pc.GetX() != ox+2 || pc.GetY() != oy+2 {
		t.Errorf("locker at (%d,%d), want (%d,%d)", pc.GetX(), pc.GetY(), ox+2, oy+2)
	}
}
