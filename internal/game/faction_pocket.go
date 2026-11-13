package game

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// The storage pocket (greyish_plan.md, "The storage pocket", decided
// 2026-11-09): every container a settlement whose doctrine keeps a pocket
// (settlement.storage_pocket, the greys) owns on a level is a door onto one
// shared inventory. Workers deposit and withdraw at the nearest door and
// never carry from locker to locker; the move between slices is the pocket.
// The key is the container's OwnedBy (a rival grey settlement is one ship on
// one site), so pockets never meet. The link is runtime only
// (components.StoragePocket), rebuilt from the level by linkPockets: on
// materialise, every director pass, and when a door is destroyed. A door
// destroyed moves its slice into the survivors; the last door spills the
// pocket on its tile (one stack a tile, nearest first), stored GearDrop gear
// rolling its chance.

// pocketOwner reports whether containers owned by name are pocket doors.
func pocketOwner(name string) bool {
	s := settlementBlock(name)
	return s != nil && s.StoragePocket
}

// pocketDoorRadius is how far round a destroyed door its spill lands.
const pocketDoorRadius = 6

// levelContainers is every storage container on level with its entity, in
// level order (Entities, then StaticEntities).
func levelContainers(level *world.Level) ([]*ecs.Entity, []*components.StorageComponent) {
	var es []*ecs.Entity
	var scs []*components.StorageComponent
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(components.Storage) || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			es = append(es, e)
			scs = append(scs, e.GetComponent(components.Storage).(*components.StorageComponent))
		}
	}
	return es, scs
}

// linkPockets links every pocket door on level to its owner's pocket and
// unlinks every other container. It reports the doors by owner.
func linkPockets(level *world.Level) map[string][]*components.StorageComponent {
	if level == nil {
		return nil
	}
	_, scs := levelContainers(level)
	by := map[string][]*components.StorageComponent{}
	isPocket := map[string]bool{}
	for _, sc := range scs {
		if sc.OwnedBy == "" {
			sc.Pocket = nil
			continue
		}
		p, ok := isPocket[sc.OwnedBy]
		if !ok {
			p = pocketOwner(sc.OwnedBy)
			isPocket[sc.OwnedBy] = p
		}
		if !p {
			sc.Pocket = nil
			continue
		}
		by[sc.OwnedBy] = append(by[sc.OwnedBy], sc)
	}
	for _, doors := range by {
		pk := &components.StoragePocket{Doors: doors}
		for _, d := range doors {
			d.Pocket = pk
		}
	}
	return by
}

// pocketDoorLost is a pocket door destroyed on level: its slice moves into
// the surviving doors of its pocket (the most free room first); what does
// not fit, or everything when it was the last door, spills round its tile.
// It reports how many items spilled onto the ground.
func pocketDoorLost(level *world.Level, door *ecs.Entity) int {
	if door == nil || !door.HasComponent(components.Storage) {
		return 0
	}
	sc := door.GetComponent(components.Storage).(*components.StorageComponent)
	if !pocketOwner(sc.OwnedBy) {
		return 0
	}
	var survivors []*components.StorageComponent
	_, scs := levelContainers(level)
	for _, o := range scs {
		if o != sc && o.OwnedBy == sc.OwnedBy {
			survivors = append(survivors, o)
		}
	}
	items := sc.Items
	sc.Items, sc.Pocket = nil, nil
	if len(survivors) == 0 {
		// The last door: everything the pocket held is on this tile's
		// doorstep now. Its slice is all of it.
		pc, ok := entityPos(door)
		if !ok {
			return 0
		}
		n := spillPocket(level, items, pc.GetX(), pc.GetY(), pc.GetZ())
		message.PostMessage("Alert", fmt.Sprintf("The %s storage bursts open, spilling its contents.", pocketOwnerNoun(sc.OwnedBy))) // DRAFT
		linkPockets(level)
		return n
	}
	sort.SliceStable(survivors, func(i, j int) bool { return freeSlots(survivors[i]) > freeSlots(survivors[j]) })
	var left []*ecs.Entity
	for _, it := range items {
		placed := false
		for _, o := range survivors {
			o.Pocket = nil // into its own slice: the pocket is relinked below
			if o.AddItem(it) {
				placed = true
				break
			}
		}
		if !placed {
			left = append(left, it)
		}
	}
	linkPockets(level)
	if len(left) == 0 {
		return 0
	}
	pc, ok := entityPos(door)
	if !ok {
		return 0
	}
	return spillPocket(level, left, pc.GetX(), pc.GetY(), pc.GetZ())
}

// pocketOwnerNoun is the player-facing word for a pocket's owner ("grey").
func pocketOwnerNoun(owner string) string {
	i := strings.IndexByte(owner, ':')
	if i <= 0 {
		return owner
	}
	if d := campaign.LoadFactionData().Doctrine(owner[:i]); d != nil && d.Singular != "" {
		return d.Singular
	}
	return owner[:i]
}

// freeSlots is a container's free slots (a large number for unlimited).
func freeSlots(sc *components.StorageComponent) int {
	if sc.Capacity <= 0 {
		return 1 << 20
	}
	return sc.Capacity - len(sc.Items)
}

// spillPocket lays items on the ground round (cx, cy, cz): one entity a
// standable tile within pocketDoorRadius, nearest first (a stack keeps its
// whole quantity); past the free tiles the rest pile on the nearest. A
// GearDrop item (an unequipped beam rifle) rolls its chance and breaks on a
// fail. It reports how many landed.
func spillPocket(level *world.Level, items []*ecs.Entity, cx, cy, cz int) int {
	tiles := groundNear(level, cx, cy, cz, pocketDoorRadius)
	n := 0
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.HasComponent(components.GearDrop) && !gearSurvives(it) {
			continue // broke in the spill
		}
		x, y := cx, cy
		switch {
		case n < len(tiles):
			x, y = tiles[n][0], tiles[n][1]
		case len(tiles) > 0:
			x, y = tiles[0][0], tiles[0][1]
		}
		dropToKiller(level, nil, it, x, y, cz)
		n++
	}
	if n > 0 {
		log.Printf("[faction] a storage pocket spilled %d items at %d,%d,%d", n, cx, cy, cz)
	}
	return n
}
