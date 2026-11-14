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
	if t == nil || t.Middle.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Middle.Type].Name
}

func floorName(t *world.Tile) string {
	if t == nil || t.Floor.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Floor.Type].Name
}

func TestBuildShipLevel_RoomsAndFloors(t *testing.T) {
	lvl := BuildShipLevel("ship")
	if lvl.GetWidth() != LevelWidth || lvl.GetHeight() != LevelHeight || lvl.GetDepth() != LevelDepth {
		t.Fatalf("ship size = %dx%dx%d, want %dx%dx%d",
			lvl.GetWidth(), lvl.GetHeight(), lvl.GetDepth(), LevelWidth, LevelHeight, LevelDepth)
	}

	// Every room interior is walkable hull_floor with an empty middle.
	for ry := 0; ry < roomsY; ry++ {
		for rx := 0; rx < roomsX; rx++ {
			ix, iy := roomInterior(rx, ry)
			for dy := 0; dy < roomH; dy++ {
				for dx := 0; dx < roomW; dx++ {
					x, y := ix+dx, iy+dy
					tile := lvl.GetTilePtr(x, y, HullDeck)
					if floorName(tile) != "hull_floor" {
						t.Fatalf("room (%d,%d) interior (%d,%d) floor=%q, want hull_floor", rx, ry, x, y, floorName(tile))
					}
					// interior may hold the ship hold at the start room's corner;
					// otherwise the middle is open.
				}
			}
		}
	}
}

func TestBuildShipLevel_OuterHullIntact(t *testing.T) {
	lvl := BuildShipLevel("ship")
	gx, gy := gridOrigin()
	// The outer perimeter of the grid must be solid hull_wall (never damaged),
	// so digging never breaches the ship into space.
	for lx := 0; lx < gridW(); lx++ {
		for _, ly := range []int{0, gridH() - 1} {
			if got := midName(lvl.GetTilePtr(gx+lx, gy+ly, HullDeck)); got != "hull_wall" {
				t.Errorf("outer hull (%d,%d) = %q, want hull_wall", gx+lx, gy+ly, got)
			}
		}
	}
	for ly := 0; ly < gridH(); ly++ {
		for _, lx := range []int{0, gridW() - 1} {
			if got := midName(lvl.GetTilePtr(gx+lx, gy+ly, HullDeck)); got != "hull_wall" {
				t.Errorf("outer hull (%d,%d) = %q, want hull_wall", gx+lx, gy+ly, got)
			}
		}
	}
}

func TestBuildShipLevel_DoorwaysAreRubble(t *testing.T) {
	lvl := BuildShipLevel("ship")
	gx, gy := gridOrigin()
	// Every doorway between adjacent rooms is a rubble pile to dig out.
	for ry := 0; ry < roomsY; ry++ {
		for rx := 0; rx < roomsX-1; rx++ {
			lx := (rx + 1) * (roomW + 1)
			ly := ry*(roomH+1) + 1 + roomH/2
			if got := midName(lvl.GetTilePtr(gx+lx, gy+ly, HullDeck)); got != "rubble_pile" {
				t.Errorf("h-doorway (%d,%d) = %q, want rubble_pile", gx+lx, gy+ly, got)
			}
		}
	}
	for rx := 0; rx < roomsX; rx++ {
		for ry := 0; ry < roomsY-1; ry++ {
			ly := (ry + 1) * (roomH + 1)
			lx := rx*(roomW+1) + 1 + roomW/2
			if got := midName(lvl.GetTilePtr(gx+lx, gy+ly, HullDeck)); got != "rubble_pile" {
				t.Errorf("v-doorway (%d,%d) = %q, want rubble_pile", gx+lx, gy+ly, got)
			}
		}
	}
}

func TestBuildShipLevel_HoldInStartRoom(t *testing.T) {
	lvl := BuildShipLevel("ship")
	var holds []*ecs.Entity
	for _, e := range lvl.Entities {
		if e != nil && e.HasComponent(components.Storage) {
			holds = append(holds, e)
		}
	}
	if len(holds) != 1 {
		t.Fatalf("ship should have exactly 1 hold, got %d", len(holds))
	}
	if holds[0].GetComponent(components.Storage).(*components.StorageComponent).OwnedBy != "ship" {
		t.Errorf("hold owner = %q, want ship", holds[0].GetComponent(components.Storage).(*components.StorageComponent).OwnedBy)
	}
	// The hold sits inside the start room.
	rx, ry := startRoom()
	ix, iy := roomInterior(rx, ry)
	pc := holds[0].GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if pc.GetX() < ix || pc.GetX() >= ix+roomW || pc.GetY() < iy || pc.GetY() >= iy+roomH {
		t.Errorf("hold at (%d,%d) not inside start room [%d,%d)+%dx%d", pc.GetX(), pc.GetY(), ix, iy, roomW, roomH)
	}
}

// A saucer (phase F step 6c) is a closed round hull of its radius at the
// hull centre: floor on every disc cell, wall on its outline only, nothing
// outside it, the hold inside, owned by the ship.
func TestBuildSaucerLevel(t *testing.T) {

	level := BuildSaucerLevel("ship", Saucer{Radius: 8, Decks: 1, Floor: "hull_floor", Wall: "hull_wall"})
	cx, cy, z := HullCenter()
	rr := world.NewRoundRoom(8)
	floors, walls := 0, 0
	for dy := -10; dy <= 10; dy++ {
		for dx := -10; dx <= 10; dx++ {
			tile := level.GetTilePtr(cx+dx, cy+dy, z)
			in := rr.In(dx, dy)
			if !in {
				if !tile.Floor.IsEmpty() || !tile.Middle.IsEmpty() {
					t.Fatalf("hull outside the disc at %d,%d", dx, dy)
				}
				continue
			}
			if tile.Floor.IsEmpty() {
				t.Fatalf("no floor at %d,%d", dx, dy)
			}
			floors++
			if !tile.Middle.IsEmpty() {
				walls++
				if !rr.Wall[[2]int{dx, dy}] {
					t.Fatalf("a wall inside the hull at %d,%d", dx, dy)
				}
			}
		}
	}
	if floors < 190 || floors > 230 || walls != len(rr.Wall) {
		t.Errorf("floors %d (about 200), walls %d of %d", floors, walls, len(rr.Wall))
	}
	holds := 0
	for _, e := range level.Entities {
		if e != nil && e.HasComponent(components.Storage) {
			holds++
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if !rr.In(pc.GetX()-cx, pc.GetY()-cy) || rr.Wall[[2]int{pc.GetX() - cx, pc.GetY() - cy}] {
				t.Errorf("the hold at %d,%d is not inside the hull", pc.GetX(), pc.GetY())
			}
		}
	}
	if holds != 1 {
		t.Errorf("%d holds", holds)
	}
}
