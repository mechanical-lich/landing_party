package game

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// datapadReading is what datapad dp says, without changing anything: text is
// its own words (a clue a structure left) or else its event's datapad
// account; lead is its quest's lead ("<name>. <description>"), "" without
// one. collectDatapads posts these when a colonist picks it up (then reveals
// the event and unlocks the quest); the observer's inspector shows them where
// it lies.
func datapadReading(c *campaign.Campaign, dp *components.DatapadComponent) (text, lead string) {
	if dp == nil {
		return "", ""
	}
	text = c.DatapadText(dp.EventID)
	if dp.Text != "" {
		text = dp.Text
	}
	if q := c.QuestDef(dp.QuestID); q != nil {
		lead = fmt.Sprintf("%s. %s", q.Name, q.Description)
	}
	return text, lead
}

// stationLogIDs are the events a station's log console holds: every event
// with an origin on level (the chapter's own excepted), sorted.
func stationLogIDs(level *world.Level) []string {
	var ids []string
	if level == nil {
		return nil
	}
	for k := range level.Flags {
		if strings.HasPrefix(k, "origin:") && strings.HasSuffix(k, ":x") {
			if id := strings.TrimSuffix(strings.TrimPrefix(k, "origin:"), ":x"); id != generation.ChapterOrigin {
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// stationLogText is the records a station's log console on level reads, in
// order, without making any known.
func stationLogText(c *campaign.Campaign, level *world.Level) []string {
	var out []string
	for _, id := range stationLogIDs(level) {
		if t := c.DatapadText(id); t != "" {
			out = append(out, t)
		}
	}
	return out
}
