package campaign

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/objective"
)

// Every charted station site gets its power quest; one with a sealed archive
// its archive quest too; no planet gets either.
func TestStationSitesGetTheirQuests(t *testing.T) {
	useRepoData(t)
	stations, archives := 0, 0
	for seed := int64(1); seed <= 60; seed++ {
		c := genCampaign(t, seed)

		lib, err := history.DefaultLibrary()
		if err != nil {
			t.Fatal(err)
		}
		byID := map[string]Quest{}
		for _, q := range c.QuestDefs {
			byID[q.ID] = q
		}
		for _, loc := range c.Locations {
			power, ok := byID["sq_station_power_"+loc.ID]
			_, arch := byID["sq_station_archive_"+loc.ID]
			if loc.Kind != "station" || c.SiteHistory(loc) == nil {
				if ok || arch {
					t.Errorf("seed %d %s (%s): station quests on a planet", seed, loc.ID, loc.Kind)
				}
				continue
			}
			stations++
			if !ok || power.Objective.Trigger != objective.TriggerFlagSet || power.Objective.Flag != "power:main" {
				t.Errorf("seed %d %s: no power quest (%+v)", seed, loc.ID, power)
			}
			sealed := false
			for _, m := range c.History.Evidence(lib, c.SiteHistory(loc)).Station.Modules {
				if m.Kind == "archive" && m.Sealed != "" {
					sealed = true
				}
			}
			if sealed != arch {
				t.Errorf("seed %d %s: sealed archive %v, archive quest %v", seed, loc.ID, sealed, arch)
			}
			if arch {
				archives++
			}
		}
	}
	if stations == 0 || archives == 0 {
		t.Errorf("%d station sites, %d with an archive quest", stations, archives)
	}
}
