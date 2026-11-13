package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

const pocketOwnerA, pocketOwnerB = "greyish:pocket_a", "greyish:pocket_b"

// pocketLocker puts an owned storage locker on level at (x, y, 0).
func pocketLocker(t *testing.T, level *world.Level, owner string, x, y int) (*ecs.Entity, *components.StorageComponent) {
	t.Helper()
	e, err := factory.Create("storage_locker", x, y, 0)
	if err != nil {
		t.Fatal(err)
	}
	sc := e.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = owner
	level.AddEntity(e)
	return e, sc
}

// stack is a material item of bp with quantity q.
func stack(t *testing.T, bp string, q int) *ecs.Entity {
	t.Helper()
	it, err := factory.Create(bp, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if it.HasComponent(components.Material) {
		it.GetComponent(components.Material).(*components.MaterialComponent).Quantity = q
	}
	return it
}

// Two grey lockers are two doors onto one pocket: each holds what the other
// holds, a worker finds the nearest door (not the locker the ore is in), a
// take at one door comes from the other's slice, the room is the sum, and
// totals over the owner's containers are the same with or without the link.
func TestPocketDoors(t *testing.T) {
	level := world.NewLevel(40, 10, 1)
	a, sa := pocketLocker(t, level, pocketOwnerA, 2, 2)
	b, sb := pocketLocker(t, level, pocketOwnerA, 30, 2)
	if !sa.AddItem(stack(t, "metal_ore", 12)) {
		t.Fatal("no room at A")
	}
	before := storage.Summarize(storage.LevelProvider{Level: level}, []string{pocketOwnerA})
	cBefore := storage.CountResource(storage.LevelProvider{Level: level}, []string{pocketOwnerA}, "metal_ore")
	if got := linkPockets(level); len(got[pocketOwnerA]) != 2 {
		t.Fatalf("doors %v", got)
	}
	if sb.CountResource("metal_ore") != 12 || !sb.HasItem("metal_ore") {
		t.Errorf("door B holds %d of the pocket's 12 ore", sb.CountResource("metal_ore"))
	}
	after := storage.Summarize(storage.LevelProvider{Level: level}, []string{pocketOwnerA})
	cAfter := storage.CountResource(storage.LevelProvider{Level: level}, []string{pocketOwnerA}, "metal_ore")
	if after["metal_ore"] != before["metal_ore"] || cAfter != cBefore || cAfter != 12 {
		t.Errorf("totals moved with the link: summarize %v -> %v, count %d -> %d", before, after, cBefore, cAfter)
	}
	if !storage.Check(storage.LevelProvider{Level: level}, []string{pocketOwnerA}, map[string]int{"metal_ore": 12}) ||
		storage.Check(storage.LevelProvider{Level: level}, []string{pocketOwnerA}, map[string]int{"metal_ore": 13}) {
		t.Error("storage.Check counts the pocket twice or not at all")
	}
	// The nearest door to a worker at (28, 2) is B, though the ore is in A.
	if got := workerai.FindClosestStorageWith(level, pocketOwnerA, "metal_ore", 28, 2, 0); got != b {
		t.Errorf("nearest door with ore: %v, want B", got)
	}
	if got := workerai.FindClosestStorageWith(level, pocketOwnerA, "metal_ore", 3, 2, 0); got != a {
		t.Errorf("nearest door with ore from A's side: %v, want A", got)
	}
	// A take at B comes out of A's slice.
	if it := sb.TakeResourceStack("metal_ore"); it == nil || len(sa.Items) != 0 {
		t.Errorf("take at B: %v, A's slice %d", it, len(sa.Items))
	}
	// Room: fill A to capacity; B still takes, and a put at full A goes to B.
	for i := len(sa.Items); i < sa.Capacity; i++ {
		x, _ := factory.Create("laser_pistol", 0, 0, 0)
		sa.Items = append(sa.Items, x)
	}
	if !sa.HasRoom() {
		t.Error("door A reports no room while B is empty")
	}
	if !sa.AddItem(stack(t, "crystal", 3)) || len(sb.Items) != 1 {
		t.Errorf("a put at full A did not land in B (B has %d)", len(sb.Items))
	}
	// Deduct at B across both slices.
	sa.Items = sa.Items[:0]
	sa.Items = append(sa.Items, stack(t, "crystal", 4))
	if !sb.DeductResource("crystal", 6) || sa.CountResource("crystal") != 1 {
		t.Errorf("deduct 6 of 7 crystal across doors left %d", sa.CountResource("crystal"))
	}
}

// Pockets never meet: two grey settlements and the colony on one level each
// keep their own; an ordinary (colony) locker is never linked.
func TestPocketOwnersNeverMeet(t *testing.T) {
	level := world.NewLevel(40, 10, 1)
	_, a1 := pocketLocker(t, level, pocketOwnerA, 2, 2)
	_, a2 := pocketLocker(t, level, pocketOwnerA, 4, 2)
	_, b1 := pocketLocker(t, level, pocketOwnerB, 10, 2)
	_, c1 := pocketLocker(t, level, "Colony", 12, 2)
	_, c2 := pocketLocker(t, level, "Colony", 14, 2)
	a1.AddItem(stack(t, "metal_ore", 5))
	b1.AddItem(stack(t, "crystal", 5))
	c1.AddItem(stack(t, "biomass", 5))
	linkPockets(level)
	if a2.CountResource("crystal") != 0 || b1.CountResource("metal_ore") != 0 || a2.CountResource("metal_ore") != 5 {
		t.Error("two grey pockets met")
	}
	if c1.Pocket != nil || c2.Pocket != nil || c2.CountResource("biomass") != 0 {
		t.Error("the colony's lockers were linked")
	}
	if workerai.FindClosestStorageWith(level, "Colony", "metal_ore", 3, 2, 0) != nil {
		t.Error("a colonist can fetch from a grey door")
	}
}

// A door destroyed while another stands moves its slice into the survivor;
// the last door destroyed spills the pocket round its tile, one stack a
// tile (a stack keeps its quantity), and stored gear rolls its GearDrop:
// a certain fail breaks, a certain pass lands intact.
func TestPocketDoorDestroyedAndSpill(t *testing.T) {
	level := loreFlatLevel(t, 30, 30)
	z := level.GetSurfaceZ(15, 15)
	place := func(x, y int) (*ecs.Entity, *components.StorageComponent) {
		e, sc := pocketLocker(t, level, pocketOwnerA, x, y)
		pc, _ := entityPos(e)
		pc.SetPosition(x, y, z)
		return e, sc
	}
	a, sa := place(10, 15)
	b, sb := place(20, 15)
	linkPockets(level)
	sa.Items = append(sa.Items, stack(t, "metal_ore", 40), stack(t, "crystal", 9))
	rifle, _ := factory.Create("grey_beam_rifle", 0, 0, 0)
	rifle.GetComponent(components.GearDrop).(*components.GearDropComponent).Chance = 0
	pistol, _ := factory.Create("grey_beam_pistol", 0, 0, 0)
	pistol.GetComponent(components.GearDrop).(*components.GearDropComponent).Chance = 1
	sb.Items = append(sb.Items, rifle, pistol)
	level.RemoveEntity(a)
	if n := pocketDoorLost(level, a); n != 0 {
		t.Fatalf("a door lost with another standing spilled %d", n)
	}
	if sb.OwnCount("metal_ore") != 40 || sb.OwnCount("crystal") != 9 || len(sb.Items) != 4 {
		t.Fatalf("B after A fell: %d items, ore %d", len(sb.Items), sb.OwnCount("metal_ore"))
	}
	level.RemoveEntity(b)
	if n := pocketDoorLost(level, b); n != 3 {
		t.Fatalf("the last door spilled %d entities, want 3 (ore, crystal, the pistol; the rifle broke)", n)
	}
	loose := map[string]int{}
	tiles := map[[2]int]int{}
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(rlcomponents.Item) {
				continue
			}
			pc, _ := entityPos(e)
			tiles[[2]int{pc.GetX(), pc.GetY()}]++
			q := 1
			if e.HasComponent(components.Material) {
				q = e.GetComponent(components.Material).(*components.MaterialComponent).Quantity
			}
			loose[e.Blueprint] += q
			if absI(pc.GetX()-20) > pocketDoorRadius || absI(pc.GetY()-15) > pocketDoorRadius {
				t.Errorf("%s spilled at %d,%d, beyond %d of the door", e.Blueprint, pc.GetX(), pc.GetY(), pocketDoorRadius)
			}
		}
	}
	if loose["metal_ore"] != 40 || loose["crystal"] != 9 || loose["grey_beam_pistol"] != 1 || loose["grey_beam_rifle"] != 0 {
		t.Errorf("spill %v", loose)
	}
	for tile, n := range tiles {
		if n > 1 {
			t.Errorf("%d stacks on %v: one a tile while tiles are free", n, tile)
		}
	}
}

// A colony locker destroyed spills nothing and moves nothing (as before).
func TestOrdinaryLockerDestroyedUnchanged(t *testing.T) {
	level := world.NewLevel(20, 10, 1)
	c, sc := pocketLocker(t, level, "Colony", 2, 2)
	_, other := pocketLocker(t, level, "Colony", 5, 2)
	sc.AddItem(stack(t, "metal_ore", 5))
	level.RemoveEntity(c)
	if n := pocketDoorLost(level, c); n != 0 || len(other.Items) != 0 {
		t.Errorf("a colony locker's death spilled %d, moved %d", n, len(other.Items))
	}
}

// The player cannot beam up from a rival's pocket door (it would hand over
// the whole pocket); an ordinary rival locker is as before.
func TestBeamOwnerCheck(t *testing.T) {
	level := world.NewLevel(20, 10, 1)
	door, sc := pocketLocker(t, level, pocketOwnerA, 2, 2)
	sc.AddItem(stack(t, "metal_ore", 5))
	linkPockets(level)
	wm := &WorldManager{Campaign: &campaign.Campaign{Name: "Beam"}}
	if err := wm.BeamResourceUpFrom(door, "metal_ore", 5); err == nil || sc.CountResource("metal_ore") != 5 {
		t.Errorf("beamed up from a grey door: err %v, left %d", err, sc.CountResource("metal_ore"))
	}
}
