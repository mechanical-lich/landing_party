package game

import (
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/construction"
)

// ValidateData is the start-up check of the data the game cannot run
// without and that would otherwise fail quietly: data/factions.json (the
// player's own doctrine lives there; a malformed file used to be one stderr
// line and a ship with no crew, build menu or research), its stage
// structures against build.json.
func ValidateData() error {
	all := construction.ListBuildables()
	if err := validateInterfaces(); err != nil {
		return err
	}
	return campaign.ValidateFactionData(func(k string) bool {
		if len(all) == 0 {
			return true // build.json not loaded: nothing to check against
		}
		_, ok := all[k]
		return ok
	})
}
