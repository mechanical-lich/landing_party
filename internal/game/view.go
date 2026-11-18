package game

import (
	"fmt"
	"log"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/settlement"
)

// Viewing a planet (observer mode v2; docs/developer/background_simulation.md,
// "Viewed planets"): the Star Map's View button opens any charted site's
// level with the free camera, whether or not the ship is there and whether
// or not it is staffed. A level not in memory is built or thawed as
// buildParked does and kept loaded while viewed, ticking live like the
// planet in orbit; once the viewer leaves (the Star Map, the ship, another
// planet) it is frozen again unless a colony worker is on it, so a planet
// only viewed goes back to Tier 1 exactly as before. While it is loaded
// Tier 1 skips it and its presences step by the live rule instead.

// ViewLocation opens id's level for the viewer and returns it ready to
// switch to. The planet in orbit is entered as To Site does.
func (wm *WorldManager) ViewLocation(id string) (*MainState, error) {
	c := wm.Campaign
	loc := c.Locations[id]
	if !loc.HasSite() || (!loc.Identified && !observerMode()) {
		return nil, fmt.Errorf("nothing to view there")
	}
	if id == wm.currentID && wm.Current() != nil {
		return wm.EnterCurrent()
	}
	ms := wm.loaded[id]
	if ms == nil {
		if wm.viewVisited == nil {
			wm.viewVisited = map[string]bool{}
		}
		if _, ok := wm.viewVisited[id]; !ok {
			wm.viewVisited[id] = loc.Visited
		}
		var err error
		if ms, err = wm.buildParked(id); err != nil {
			return nil, err
		}
		wm.addLoaded(id, ms)
	} else {
		wm.visits++
		wm.lastVisit[id] = wm.visits
	}
	if wm.viewedID != "" && wm.viewedID != id {
		wm.handBack(wm.viewedID)
	}
	wm.viewedID = id
	if c.SpreadRun() && !c.HoldsSite(loc) {
		// A race run sees a world in its reach as a ship in orbit would:
		// what holds it, and the whole level (none of its bodies is there to
		// light the fog).
		c.RevealPresences(loc)
		ms.level.RevealAll = true
	}
	if err := wm.enforceLoadedCap(); err != nil {
		return nil, err
	}
	wm.frameView(ms, loc)
	ms.done = false
	ms.next = nil
	ms.reattach()
	installCampaignStorageHook(c)
	return ms, nil
}

// Viewing is the planet the viewer is on ("" on the ship or the dashboard).
func (wm *WorldManager) Viewing() string { return wm.viewedID }

// ViewedElsewhere reports whether the viewer is on a planet the ship is not
// over.
func (wm *WorldManager) ViewedElsewhere() bool {
	return wm.viewedID != "" && wm.viewedID != wm.currentID
}

// EndView is the viewer leaving the planet it was on: a planet the ship is
// not over and no colony worker is on is frozen (its Visited as it was
// before it was viewed); any other stays loaded as before.
func (wm *WorldManager) EndView() {
	id := wm.viewedID
	wm.handBack(id) // a race run's world goes back to its director
	wm.viewedID = ""
	if id == "" || id == wm.currentID {
		return
	}
	ms, loc := wm.loaded[id], wm.Campaign.Locations[id]
	if ms == nil || loc == nil || isStaffed(ms) {
		return
	}
	ms.teardown()
	if err := wm.Freeze(loc, ms); err != nil {
		log.Printf("unview %s: %v", id, err)
		return
	}
	wm.restoreViewVisited(loc)
}

// restoreViewVisited puts back a viewed-only planet's Visited.
func (wm *WorldManager) restoreViewVisited(loc *campaign.Location) {
	if v, ok := wm.viewVisited[loc.ID]; ok {
		loc.Visited = v
		delete(wm.viewVisited, loc.ID)
	}
}

// syncObserverReveal draws the whole level while observer mode is on (the
// viewer is a spectator: a planet nobody is on, or a camp beyond the
// colony's sight, would otherwise be black under the fog of war). It only
// changes drawing (world.Level.RevealAll); fog and sight are untouched.
// Turning the mode off puts the fog back.
func (s *MainState) syncObserverReveal() {
	if s.level == nil || observerMode() == s.observerReveal {
		return
	}
	s.observerReveal = observerMode()
	s.level.RevealAll = s.observerReveal
}

// viewLabel is the HUD's view line on a planet the ship is not over:
// "VIEWING <planet> · SHIP AT <site>" ("" otherwise).
func (wm *WorldManager) viewLabel(s *MainState) string {
	if wm == nil || s == nil || s == wm.shipLevel || s.locationID == "" {
		return ""
	}
	if shiplessRun() {
		// No ship: the label names the planet.
		if loc := wm.Campaign.Locations[s.locationID]; loc != nil {
			return "VIEWING " + loc.Name
		}
		return "VIEWING " + s.locationID
	}
	if s.locationID == wm.currentID {
		return ""
	}
	where := "IN TRANSIT"
	if cur := wm.Campaign.CurrentLocation(); cur != nil {
		where = "SHIP AT " + cur.Name
	}
	name := s.locationID
	if loc := wm.Campaign.Locations[s.locationID]; loc != nil {
		name = loc.Name
	}
	return "VIEWING " + name + " · " + where
}

// frameView opens a viewed planet on something worth seeing: the
// expedition's camp when it is there, else the largest presence on the
// planet (its stamp's centre: the nest, the hub, the camp), else the view
// the level opened with (the landing). A planet's presences are often far
// from the scenario's plaza, where the camera would otherwise sit.
func (wm *WorldManager) frameView(ms *MainState, loc *campaign.Location) {
	c := wm.Campaign
	if f := c.Expedition(); f != nil && f.Landed() && f.LandedID == loc.ID {
		if st := settlement.Settlements[campaign.ExpeditionFaction+":"+loc.ID]; st != nil {
			ms.camera.CenterOn(st.CenterX, st.CenterY, st.CenterZ)
			return
		}
	}
	best, bestPop := "", -1
	for _, p := range loc.Factions {
		if p.Population > bestPop {
			best, bestPop = p.ID, p.Population
		}
	}
	for _, f := range c.LandedFleets(loc.ID) {
		if f.Presence.Population > bestPop {
			best, bestPop = f.Presence.ID, f.Presence.Population
		}
	}
	if best == "" {
		return
	}
	if x, y, z, ok := generation.Origin(ms.level, factionOriginKey(best)); ok {
		ms.camera.CenterOn(x, y, z)
	}
}
