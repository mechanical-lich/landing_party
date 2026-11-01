package game

import (
	"sort"

	"github.com/mechanical-lich/landing_party/internal/storage"
)

// Tech gate keys read by the Global Inventory dashboard panel: Current Site →
// All Sites → By Site.
const (
	techGlobalInvCurrent  = "global_inv_current"
	techGlobalInvAll      = "global_inv_all"
	techGlobalInvDetailed = "global_inv_detailed"
)

// globalInventoryData summarizes campaign-wide storage for the Global Inventory
// panel. It is pure data — the panel renders the results inline. Ship hold and
// the currently-loaded location are computed live; every other site reads its
// cached StorageSummary (refreshed on Freeze).
type globalInventoryData struct {
	wm *WorldManager
}

func newGlobalInventoryData(wm *WorldManager) *globalInventoryData {
	return &globalInventoryData{wm: wm}
}

// aggregateAllSummaries totals ship hold + every loaded site (live) + every frozen
// location's cached summary.
func (g *globalInventoryData) aggregateAllSummaries() map[string]int {
	out := make(map[string]int)
	if g.wm.Campaign == nil {
		return out
	}
	for bp, qty := range storage.Summarize(
		storage.ShipProvider{Level: g.wm.ShipLevel()},
		[]string{g.wm.shipSettlementName()},
	) {
		out[bp] += qty
	}
	for id := range g.wm.Campaign.Locations {
		for bp, qty := range g.siteSummary(id) {
			out[bp] += qty
		}
	}
	return out
}

type giSiteEntry struct {
	key   string // "ship" or location ID
	label string
}

// collectSites returns Ship Hold first, then every established location
// (anything with a SaveFile). The currently-loaded site is included even before
// its first Freeze — its level is in memory, so live counts are available.
func (g *globalInventoryData) collectSites() []giSiteEntry {
	out := []giSiteEntry{{key: "ship", label: "Ship Hold"}}
	if g.wm.Campaign == nil {
		return out
	}
	var ids []string
	for id, loc := range g.wm.Campaign.Locations {
		if loc.SaveFile == "" && !g.wm.IsLoaded(id) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return g.wm.Campaign.Locations[ids[i]].Name < g.wm.Campaign.Locations[ids[j]].Name
	})
	for _, id := range ids {
		loc := g.wm.Campaign.Locations[id]
		label := loc.Name
		if id == g.wm.Campaign.CurrentLocationID {
			if g.wm.Campaign.IsAdrift() {
				label += "  (ship here, adrift)"
			} else {
				label += "  (in orbit)"
			}
		}
		out = append(out, giSiteEntry{key: id, label: label})
	}
	return out
}

// siteSummary returns the storage totals for one site key.
func (g *globalInventoryData) siteSummary(key string) map[string]int {
	if g.wm.Campaign == nil {
		return nil
	}
	if key == "ship" {
		return storage.Summarize(
			storage.ShipProvider{Level: g.wm.ShipLevel()},
			[]string{g.wm.shipSettlementName()},
		)
	}
	loc := g.wm.Campaign.Locations[key]
	if loc == nil {
		return nil
	}
	if ms := g.wm.loaded[key]; ms != nil && ms.level != nil {
		return storage.Summarize(
			storage.LevelProvider{Level: ms.level},
			[]string{ms.colonyName()},
		)
	}
	return loc.StorageSummary
}
