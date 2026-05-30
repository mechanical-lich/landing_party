// Package storage centralizes settlement-stockpile queries so resource
// counting/checking/deduction works against any source of StorageComponent
// entities — a live level, the abstract ship hold, or a combination — instead
// of assuming the caller's single *world.Level.
package storage

import (
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
)

// Provider yields the entity slices that may contain storage containers.
type Provider interface {
	Sources() [][]*ecs.Entity
}

// LevelProvider scans a live level's dynamic and static entities.
type LevelProvider struct{ Level *world.Level }

func (p LevelProvider) Sources() [][]*ecs.Entity {
	if p.Level == nil {
		return nil
	}
	return [][]*ecs.Entity{p.Level.Entities, p.Level.StaticEntities}
}

// ShipProvider scans the persistent ship hold (rebuilt lazily, mutations are
// flushed by ShipState.Sync at save time).
type ShipProvider struct{ Ship *campaign.ShipState }

func (p ShipProvider) Sources() [][]*ecs.Entity {
	if p.Ship == nil {
		return nil
	}
	return [][]*ecs.Entity{p.Ship.LiveHold()}
}

// MultiProvider concatenates several providers (e.g. on-planet + ship hold).
type MultiProvider struct{ Providers []Provider }

func (m MultiProvider) Sources() [][]*ecs.Entity {
	var out [][]*ecs.Entity
	for _, p := range m.Providers {
		if p == nil {
			continue
		}
		out = append(out, p.Sources()...)
	}
	return out
}

func ownerMatch(owners []string, owned string) bool {
	for _, o := range owners {
		if o == owned {
			return true
		}
	}
	return false
}

func eachStorage(p Provider, owners []string, fn func(*components.StorageComponent)) {
	if p == nil {
		return
	}
	for _, src := range p.Sources() {
		for _, e := range src {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if !ownerMatch(owners, sc.OwnedBy) {
				continue
			}
			fn(sc)
		}
	}
}

// CountResource totals one resource across all matching storage.
func CountResource(p Provider, owners []string, blueprint string) int {
	total := 0
	eachStorage(p, owners, func(sc *components.StorageComponent) {
		total += sc.CountResource(blueprint)
	})
	return total
}

// CountAll totals each named resource across all matching storage.
func CountAll(p Provider, owners []string, names []string) map[string]int {
	totals := make(map[string]int, len(names))
	eachStorage(p, owners, func(sc *components.StorageComponent) {
		for _, n := range names {
			totals[n] += sc.CountResource(n)
		}
	})
	return totals
}

// Check reports whether the combined storage holds the full cost.
func Check(p Provider, owners []string, cost map[string]int) bool {
	totals := make(map[string]int, len(cost))
	eachStorage(p, owners, func(sc *components.StorageComponent) {
		for name := range cost {
			totals[name] += sc.CountResource(name)
		}
	})
	for name, required := range cost {
		if totals[name] < required {
			return false
		}
	}
	return true
}

// isMaterialItem returns true for items carrying a Material component.
func isMaterialItem(item *ecs.Entity) bool {
	return item != nil && item.Blueprint != "" && item.HasComponent(components.Material)
}

// ListResources returns the sorted set of material blueprint names present in
// any matching storage across the provider.
func ListResources(p Provider, owners []string) []string {
	seen := map[string]bool{}
	eachStorage(p, owners, func(sc *components.StorageComponent) {
		for _, item := range sc.Items {
			if isMaterialItem(item) {
				seen[item.Blueprint] = true
			}
		}
	})
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Summarize totals every material across all matching storage as a flat
// blueprint→total-quantity map. Non-material items count as 1 each. Used to
// cache a snapshot for the Global Inventory modal so parked sites can be
// inspected without loading their full level.
func Summarize(p Provider, owners []string) map[string]int {
	out := make(map[string]int)
	eachStorage(p, owners, func(sc *components.StorageComponent) {
		for _, item := range sc.Items {
			if item == nil || item.Blueprint == "" {
				continue
			}
			if item.HasComponent(components.Material) {
				mc := item.GetComponent(components.Material).(*components.MaterialComponent)
				out[item.Blueprint] += mc.Quantity
			} else {
				out[item.Blueprint]++
			}
		}
	})
	return out
}

// Deduct removes cost from the combined storage, draining containers in
// Provider/source order (callers put the ship hold first when desired).
func Deduct(p Provider, owners []string, cost map[string]int) {
	remaining := make(map[string]int, len(cost))
	for k, v := range cost {
		remaining[k] = v
	}
	eachStorage(p, owners, func(sc *components.StorageComponent) {
		for name, needed := range remaining {
			if needed <= 0 {
				continue
			}
			have := sc.CountResource(name)
			if have <= 0 {
				continue
			}
			d := needed
			if have < d {
				d = have
			}
			sc.DeductResource(name, d)
			remaining[name] -= d
		}
	})
}
