package game

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/history"
)

// What the viewer is shown of the factions (docs/developer/factions.md,
// "What the player sees" and "Observer mode"). In a normal campaign it is
// what the player has seen (the records' Known flags) under the hidden-
// identity rule (a hive is named by its morph). In an observer run the
// viewer is omniscient: every presence, every fleet and every faction event
// from day zero, named by faction as well as morph, with populations. It is
// a rule of the view alone: the Known flags are never written for it, so a
// normal campaign is untouched and an observer run's records read the same.

// shownPresences are the presences on loc the viewer is shown.
func shownPresences(loc *campaign.Location) []*campaign.FactionPresence {
	if !observerRun() {
		return loc.KnownPresences()
	}
	out := make([]*campaign.FactionPresence, 0, len(loc.Factions))
	for i := range loc.Factions {
		out = append(out, &loc.Factions[i])
	}
	return out
}

// shownFleets are the fleets the viewer is shown, in fleet order.
func shownFleets(c *campaign.Campaign) []*campaign.Fleet {
	if !observerRun() {
		return c.KnownFleets()
	}
	out := make([]*campaign.Fleet, 0, len(c.Fleets))
	for i := range c.Fleets {
		out = append(out, &c.Fleets[i])
	}
	return out
}

// fleetShown reports whether f is shown.
func fleetShown(f *campaign.Fleet) bool { return f.Known || observerRun() }

// eventShown reports whether a history event is in Legends: known, or in an
// observer run any faction event.
func eventShown(h *history.History, ev *history.Event) bool {
	if h.Known[ev.ID] {
		return true
	}
	return observerRun() && len(ev.Type) > 8 && ev.Type[:8] == "faction_"
}

// presenceWord is a presence's name in a tag or a Destinations note: the
// morph's ("xeno") while the faction's identity is hidden, and in an
// observer run the faction's too when it differs ("hive/xeno").
func presenceWord(d *campaign.Doctrine, loc *campaign.Location) string {
	m := d.MorphAt(loc).Singular
	if observerRun() && d.Singular != "" && d.Singular != m {
		return d.Singular + "/" + m
	}
	return m
}

func init() { gui.FactionLabel = hudFactionLabel }

// hudFactionLabel is the HUD's "Faction:" line for a selected entity: a
// hidden-identity faction (the hive) is not named in a normal campaign (the
// line is hidden), every faction is in an observer run.
func hudFactionLabel(faction string) string {
	if hiddenFaction(faction) {
		return ""
	}
	return faction
}

// hiddenFaction reports whether faction (a doctrine key) must not be named
// to the viewer: a doctrine with morphs (the hive, named by its morph only)
// outside an observer run.
func hiddenFaction(faction string) bool {
	if faction == "" || observerRun() {
		return false
	}
	d := campaign.LoadFactionData().Doctrine(faction)
	return d != nil && len(d.Morphs) > 0
}

// stageWord is a presence's stage in player-facing text: its seen name
// (hidden identity: the hive's top stage is never "hive"), the true name in
// an observer run.
func stageWord(d *campaign.Doctrine, stage int) string {
	if observerRun() {
		return d.StageName(stage)
	}
	return d.SeenStageName(stage)
}

// presenceCount is " 12" (the population) in an observer run, "" otherwise.
func presenceCount(n int) string {
	if !observerRun() {
		return ""
	}
	return fmt.Sprintf(" %d", n)
}
