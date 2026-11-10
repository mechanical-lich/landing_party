package director

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/crafting"
)

// grow is the grow rung (phase D, the greyish): while the crew and the units
// being printed are under the site's crew cap, print the unit type most
// under its crew share (ties in the doctrine's order) at its station, when
// the stock covers it. One print at a time.
func (d *Director) grow(s *Site, snap *snapshot, tick int) bool {
	if s.CrewCap <= 0 {
		return false
	}
	count := map[string]int{}
	for _, e := range snap.colonists {
		count[e.Blueprint]++
	}
	printing := 0
	for _, key := range s.Doctrine.Units {
		if n := snap.openCraft[key]; n > 0 {
			printing += n
			if r, ok := crafting.GetRecipe(key); ok {
				count[r.Output] += n
			}
		}
	}
	if printing > 0 || len(snap.colonists) >= s.CrewCap {
		return false
	}
	best, bestKey := 0.0, ""
	var bestStation string
	for _, key := range s.Doctrine.Units {
		r, ok := crafting.GetRecipe(key)
		if !ok || snap.stations[r.Station] == nil {
			continue
		}
		if r.RequiresTech != "" && !has(s.Known, r.RequiresTech) {
			continue
		}
		deficit := s.Doctrine.ShareOf(r.Output)*float64(s.CrewCap) - float64(count[r.Output])
		if bestKey == "" || deficit > best {
			best, bestKey, bestStation = deficit, key, r.Station
		}
	}
	if bestKey == "" {
		return false
	}
	r, _ := crafting.GetRecipe(bestKey)
	if !snap.afford(r.Cost) {
		return false
	}
	before := len(s.Tasks.GetTasks())
	if !s.Orders.Craft(bestKey, snap.stations[bestStation]) {
		return false
	}
	snap.openCraft[bestKey]++
	snap.consuming++
	d.watchNew(s, before, tick)
	d.note("grow:"+r.Output, fmt.Sprintf("print %s (%d/%d)", r.Output, len(snap.colonists), s.CrewCap), tick)
	return true
}
