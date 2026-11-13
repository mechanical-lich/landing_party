package components

import (
	"github.com/mechanical-lich/mlge/ecs"
)

// StorageComponent holds items for a settlement-owned container (storage locker,
// ship hold, etc.).
//
// AllowedTags lists the item categories the container physically supports
// (game-defined). An empty slice means "accept anything".
// FilterTags is a player-narrowed subset of AllowedTags. When non-empty, only
// items matching FilterTags are accepted (and highlighted in the UI).
//
// Items is the live runtime slice; it is serialised via world.saveStorageData
// (not by standard JSON marshalling — see world/save.go).
type StorageComponent struct {
	// Capacity caps the number of distinct slots — each merged stack or
	// non-stackable item is one slot. 0 means unlimited (the ship hold).
	Capacity    int           `json:"Capacity,omitempty"`
	Items       []*ecs.Entity `json:"-"`
	OwnedBy     string        `json:"OwnedBy"`
	AllowedTags []string      `json:"AllowedTags,omitempty"`
	FilterTags  []string      `json:"FilterTags,omitempty"`
	// Pocket links a door onto a storage pocket (greyish_plan.md, "The
	// storage pocket"): every container one owner has on one level is a door
	// onto the same inventory. Items stay in whichever door's slice they were
	// put in; a door's takes, puts, counts and room look through every door.
	// Runtime only (the game relinks it), never saved: totals over an owner's
	// containers count each door's own slice (OwnCount), so they are the same
	// with or without it. nil: an ordinary container.
	Pocket *StoragePocket `json:"-"`
}

// StoragePocket is the doors of one storage pocket, in level order.
type StoragePocket struct {
	Doors []*StorageComponent
}

// doors is s first, then the other doors of its pocket.
func (s *StorageComponent) doors() []*StorageComponent {
	if s.Pocket == nil {
		return []*StorageComponent{s}
	}
	out := make([]*StorageComponent, 0, len(s.Pocket.Doors))
	out = append(out, s)
	for _, d := range s.Pocket.Doors {
		if d != s {
			out = append(out, d)
		}
	}
	return out
}

func (s *StorageComponent) GetType() ecs.ComponentType { return Storage }

// Accepts reports whether item may be stored here given the current filter
// configuration. Containers with no AllowedTags accept everything.
func (s *StorageComponent) Accepts(item *ecs.Entity) bool {
	if len(s.AllowedTags) == 0 {
		return true
	}
	if !item.HasComponent(Material) {
		// Non-material items can only go in unrestricted containers.
		return false
	}
	mc := item.GetComponent(Material).(*MaterialComponent)
	return s.AcceptsTags(mc.Tags)
}

// AcceptsTags is the tag-only variant of Accepts, intended for hot UI paths
// that already have a cached tag slice and want to avoid spawning a probe
// entity per frame. Passing nil/empty tags into a tag-restricted container
// returns false (matches Accepts's non-material rejection).
func (s *StorageComponent) AcceptsTags(materialTags []string) bool {
	if len(s.AllowedTags) == 0 {
		return true
	}
	if len(materialTags) == 0 {
		return false
	}
	active := s.AllowedTags
	if len(s.FilterTags) > 0 {
		active = s.FilterTags
	}
	for _, allowed := range active {
		for _, t := range materialTags {
			if allowed == t {
				return true
			}
		}
	}
	return false
}

// AddItem places item into the container, merging into existing stacks for
// stackable materials. Returns false if the item was rejected by the tag
// filter or the container is at capacity; the item is not added in that case.
func (s *StorageComponent) AddItem(item *ecs.Entity) bool {
	if s.Pocket == nil {
		return s.addOwn(item)
	}
	for _, d := range s.doors() {
		if d.addOwn(item) {
			return true
		}
	}
	return false
}

// addOwn is AddItem into this container's own slice.
func (s *StorageComponent) addOwn(item *ecs.Entity) bool {
	if !s.Accepts(item) {
		return false
	}
	if item.HasComponent(Material) {
		mc := item.GetComponent(Material).(*MaterialComponent)
		if mc.IsStackable() {
			qty := mc.Quantity
			if qty <= 0 {
				qty = 1
			}
			for _, existing := range s.Items {
				if existing.Blueprint != item.Blueprint || !existing.HasComponent(Material) {
					continue
				}
				ec := existing.GetComponent(Material).(*MaterialComponent)
				room := mc.MaxStack - ec.Quantity
				if room <= 0 {
					continue
				}
				if qty <= room {
					ec.Quantity += qty
					return true
				}
				ec.Quantity = mc.MaxStack
				qty -= room
			}
			// Remainder needs a fresh slot. Either way the caller keeps only
			// the remainder: the units merged above now live in this container.
			mc.Quantity = qty
			if s.full() {
				return false
			}
			s.Items = append(s.Items, item)
			return true
		}
	}
	// Non-stackable or non-material item: needs its own slot.
	if s.full() {
		return false
	}
	s.Items = append(s.Items, item)
	return true
}

// full reports whether the container has no room for a new slot. Capacity 0
// means unlimited (the ship hold sets a deliberate cap; site lockers all
// declare one).
func (s *StorageComponent) full() bool {
	return s.Capacity > 0 && len(s.Items) >= s.Capacity
}

// HasRoom reports whether a new slot fits: in this container, or for a
// door in any door of its pocket.
func (s *StorageComponent) HasRoom() bool {
	for _, d := range s.doors() {
		if !d.full() {
			return true
		}
	}
	return false
}

// TakeOne removes and returns the first item matching blueprint (a door:
// from its own slice first, then the pocket's other doors).
func (s *StorageComponent) TakeOne(name string) *ecs.Entity {
	for _, d := range s.doors() {
		for i, item := range d.Items {
			if item.Blueprint == name {
				d.Items = append(d.Items[:i], d.Items[i+1:]...)
				return item
			}
		}
	}
	return nil
}

// TakeResourceStack removes and returns the first material stack for blueprint.
func (s *StorageComponent) TakeResourceStack(blueprint string) *ecs.Entity {
	for _, d := range s.doors() {
		for i, item := range d.Items {
			if item.Blueprint == blueprint && item.HasComponent(Material) {
				d.Items = append(d.Items[:i], d.Items[i+1:]...)
				return item
			}
		}
	}
	return nil
}

// CountResource returns the total quantity of blueprint across all stacks,
// or the item count for non-material entities (a door: its whole pocket's).
func (s *StorageComponent) CountResource(blueprint string) int {
	count := 0
	for _, d := range s.doors() {
		count += d.OwnCount(blueprint)
	}
	return count
}

// OwnCount is CountResource over this container's own slice only: what
// totals over an owner's containers add, so a pocket is counted once.
func (s *StorageComponent) OwnCount(blueprint string) int {
	count := 0
	for _, item := range s.Items {
		if item.Blueprint != blueprint {
			continue
		}
		if item.HasComponent(Material) {
			mc := item.GetComponent(Material).(*MaterialComponent)
			count += mc.Quantity
		} else {
			count++
		}
	}
	return count
}

// DeductResource subtracts amount units from stacks, removing depleted slots.
// Returns true if the full amount was successfully deducted.
func (s *StorageComponent) DeductResource(blueprint string, amount int) bool {
	if s.CountResource(blueprint) < amount {
		return false
	}
	remaining := amount
	for _, d := range s.doors() {
		take := min(remaining, d.OwnCount(blueprint))
		if take > 0 {
			d.ownDeduct(blueprint, take)
			remaining -= take
		}
	}
	return remaining == 0
}

// OwnDeduct is DeductResource from this container's own slice only (totals
// draining an owner's containers one by one).
func (s *StorageComponent) OwnDeduct(blueprint string, amount int) bool {
	if s.OwnCount(blueprint) < amount {
		return false
	}
	s.ownDeduct(blueprint, amount)
	return true
}

// ownDeduct takes amount (which the own slice holds) from the own slice.
func (s *StorageComponent) ownDeduct(blueprint string, amount int) {
	remaining := amount
	i := 0
	for remaining > 0 && i < len(s.Items) {
		item := s.Items[i]
		if item.Blueprint != blueprint {
			i++
			continue
		}
		if item.HasComponent(Material) {
			mc := item.GetComponent(Material).(*MaterialComponent)
			if mc.Quantity <= remaining {
				remaining -= mc.Quantity
				s.Items = append(s.Items[:i], s.Items[i+1:]...)
			} else {
				mc.Quantity -= remaining
				remaining = 0
				i++
			}
		} else {
			remaining--
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
		}
	}
}

func (s *StorageComponent) HasItem(name string) bool  { return s.CountResource(name) > 0 }
func (s *StorageComponent) CountItem(name string) int { return s.CountResource(name) }

func (s *StorageComponent) HasItemWithComponent(compType ecs.ComponentType) bool {
	for _, d := range s.doors() {
		for _, item := range d.Items {
			if item.HasComponent(compType) {
				return true
			}
		}
	}
	return false
}

func (s *StorageComponent) TakeOneWithComponent(compType ecs.ComponentType) *ecs.Entity {
	for _, d := range s.doors() {
		for i, item := range d.Items {
			if item.HasComponent(compType) {
				d.Items = append(d.Items[:i], d.Items[i+1:]...)
				return item
			}
		}
	}
	return nil
}

// PocketItems is every item a door's pocket holds (its own slice first; an
// ordinary container's own Items).
func (s *StorageComponent) PocketItems() []*ecs.Entity {
	if s.Pocket == nil {
		return s.Items
	}
	var out []*ecs.Entity
	for _, d := range s.doors() {
		out = append(out, d.Items...)
	}
	return out
}
