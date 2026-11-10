package game

import (
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/research"
)

// The player's settlement reads what it can build, research, craft and crew
// from its doctrine's settlement block (phase C0; campaign.PlayerDoctrine is
// the one place that picks the doctrine). The "colony" doctrine is the game
// as it was before doctrines.

// playerSettlementOf is the settlement block of c's player doctrine (nil
// only when the doctrine file lacks the doctrine).
func playerSettlementOf(c *campaign.Campaign) *campaign.DoctrineSettlement {
	if d := campaign.PlayerDoctrine(c); d != nil {
		return d.Settlement
	}
	return nil
}

// doctrineSettlement is the player's settlement block for this state's
// campaign (the default doctrine with no campaign).
func (s *MainState) doctrineSettlement() *campaign.DoctrineSettlement {
	return playerSettlementOf(s.campaign)
}

// applyDoctrine hands the HUD the doctrine's build menu. Called when the GUI
// is built and again when a campaign is attached.
func (s *MainState) applyDoctrine() {
	if s.guiManager != nil {
		s.guiManager.SetBuildMenu(buildMenuOf(s.doctrineSettlement()))
	}
}

// buildMenuOf is st's build menu: its categories in order, each with the
// build.json entries of that category it may build, by order then name.
func buildMenuOf(st *campaign.DoctrineSettlement) []gui.BuildMenuCategory {
	if st == nil {
		return nil
	}
	out := make([]gui.BuildMenuCategory, 0, len(st.BuildMenu))
	for _, c := range st.BuildMenu {
		cat := gui.BuildMenuCategory{ID: c.ID, Label: c.Label}
		for _, key := range construction.BuildableTypesInCategory(c.ID) {
			if st.CanBuild(key) {
				cat.Types = append(cat.Types, key)
			}
		}
		out = append(out, cat)
	}
	return out
}

// researchTreeOf is st's research list; an empty, non-nil tree without a
// doctrine, so nothing is researchable.
func researchTreeOf(st *campaign.DoctrineSettlement) []string {
	if st == nil || st.Research == nil {
		return []string{}
	}
	return st.Research
}

// availableResearch and lockedResearch are the research modal's lists for
// st, given the known techs.
func availableResearch(st *campaign.DoctrineSettlement, known []string) []research.Tech {
	return research.AvailableTechsIn(researchTreeOf(st), known)
}

func lockedResearch(st *campaign.DoctrineSettlement, known []string) []research.Tech {
	return research.LockedTechsIn(researchTreeOf(st), known)
}

// craftableAt is the recipes st offers at a crafting station (its units and
// recipes made there), by name.
func craftableAt(st *campaign.DoctrineSettlement, stationID string) []crafting.Recipe {
	var out []crafting.Recipe
	for _, r := range crafting.RecipesByStation(stationID) {
		if st.CanCraft(r.Key) {
			out = append(out, r)
		}
	}
	return out
}

// crewBlueprints is the blueprints of n starting crew for c's player
// doctrine.
func crewBlueprints(c *campaign.Campaign, n int) []string {
	return playerSettlementOf(c).CrewBlueprints(n)
}
