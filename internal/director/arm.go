package director

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The arm rung (phase E4, the greys; greyish_plan.md, "The director
// table"): the raid rung's arming on its own. Each worker of the rung's
// roles that holds no weapon (or, with Best, a worse one than the best
// stored) is sent for the best stored weapon by expected damage; with
// Armour, for stored armour in each empty armour slot. One order a worker
// a pass; a stored item goes to one worker.

// weaponValue is a weapon's expected damage: its dice's average plus its
// attack bonus (0 for no weapon).
func weaponValue(e *ecs.Entity) float64 {
	if e == nil || !e.HasComponent(rlcomponents.Weapon) {
		return 0
	}
	w := e.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent)
	return diceAverage(w.AttackDice) + float64(w.AttackBonus)
}

// diceAverage is "XdY+Z"'s average (0 when unreadable).
func diceAverage(s string) float64 {
	bonus := 0.0
	if i := strings.IndexAny(s, "+-"); i > 0 {
		if b, err := strconv.Atoi(s[i:]); err == nil {
			bonus = float64(b)
		}
		s = s[:i]
	}
	n, sides, ok := strings.Cut(s, "d")
	if !ok {
		return bonus
	}
	a, err1 := strconv.Atoi(n)
	b, err2 := strconv.Atoi(sides)
	if err1 != nil || err2 != nil {
		return bonus
	}
	return float64(a)*float64(b+1)/2 + bonus
}

// heldWeapon is the best weapon in a worker's hands.
func heldWeapon(e *ecs.Entity) *ecs.Entity {
	inv, ok := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	if !ok || inv == nil {
		return nil
	}
	var best *ecs.Entity
	for _, h := range []*ecs.Entity{inv.RightHand, inv.LeftHand} {
		if h != nil && weaponValue(h) > weaponValue(best) {
			best = h
		}
	}
	return best
}

// wornCount is how many of blueprint a worker holds, wears or carries.
func wornCount(e *ecs.Entity, blueprint string) int {
	inv, ok := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	if !ok || inv == nil {
		return 0
	}
	n := 0
	seen := map[*ecs.Entity]bool{}
	for _, it := range []*ecs.Entity{inv.RightHand, inv.LeftHand, inv.Head, inv.Torso, inv.Legs, inv.Feet} {
		if it != nil && !seen[it] && it.Blueprint == blueprint {
			seen[it] = true
			n++
		}
	}
	for _, it := range inv.Bag {
		if it != nil && it.Blueprint == blueprint {
			n++
		}
	}
	return n
}

// armourSlotEmpty reports whether a worker's slot (head, torso, legs, feet)
// holds no armour.
func armourSlotEmpty(e *ecs.Entity, slot rlcomponents.ItemSlot) bool {
	inv, ok := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	if !ok || inv == nil {
		return false
	}
	var it *ecs.Entity
	switch string(slot) {
	case "head":
		it = inv.Head
	case "torso":
		it = inv.Torso
	case "legs":
		it = inv.Legs
	case "feet":
		it = inv.Feet
	default:
		return false
	}
	return it == nil
}

// storedGear is one stored item the arm rung may hand out.
type storedGear struct {
	e     *ecs.Entity
	value float64
}

// armRung sends the rung's workers for stored weapons and armour.
func (d *Director) armRung(s *Site, snap *snapshot, a *campaign.DirectorArm, tick int) {
	var weapons, armour []storedGear
	for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy != s.Colony {
				continue
			}
			for _, it := range sc.Items {
				switch {
				case it.HasComponent(rlcomponents.Weapon) && components.ItemIsGear(it):
					weapons = append(weapons, storedGear{it, weaponValue(it)})
				case a.Armour && it.HasComponent(rlcomponents.Armor) && components.ItemIsGear(it):
					armour = append(armour, storedGear{it, float64(it.GetComponent(rlcomponents.Armor).(*rlcomponents.ArmorComponent).DefenseBonus)})
				}
			}
		}
	}
	if len(weapons) == 0 && len(armour) == 0 {
		return
	}
	byValue := func(g []storedGear) {
		sort.SliceStable(g, func(i, j int) bool {
			if g[i].value != g[j].value {
				return g[i].value > g[j].value
			}
			return g[i].e.Blueprint < g[j].e.Blueprint
		})
	}
	byValue(weapons)
	byValue(armour)
	taken := map[*ecs.Entity]bool{}
	for _, w := range withRoles(s, snap, a.Roles) {
		if t := busy(w); t != nil && t.Action == "equip" {
			continue // already on its way
		}
		held := weaponValue(heldWeapon(w))
		sent := false
		for _, g := range weapons {
			if taken[g.e] {
				continue
			}
			if held > 0 && (!a.Best || g.value <= held) {
				break // armed, and nothing stored is better
			}
			if s.Orders.EquipWorker(w, g.e.Blueprint) != nil {
				taken[g.e] = true
				sent = true
				d.note("equip:"+g.e.Blueprint, fmt.Sprintf("arm with %s", g.e.Blueprint), tick)
			}
			break
		}
		if sent {
			continue
		}
		for _, g := range armour {
			if taken[g.e] {
				continue
			}
			slot := g.e.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent).Slot
			if !armourSlotEmpty(w, slot) {
				continue
			}
			if s.Orders.EquipWorker(w, g.e.Blueprint) != nil {
				taken[g.e] = true
				d.note("equip:"+g.e.Blueprint, fmt.Sprintf("armour %s", g.e.Blueprint), tick)
			}
			break
		}
	}
}
