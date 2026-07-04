package game

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// questRewardText renders a quest's reward as a short human-readable string.
// Shared by the Quests dashboard panel and the in-level quest HUD.
func questRewardText(def *campaign.Quest) string {
	var parts []string
	if def.Reward.Fuel > 0 {
		parts = append(parts, fmt.Sprintf("%d fuel", def.Reward.Fuel))
	}
	for bp, n := range def.Reward.Resources {
		parts = append(parts, fmt.Sprintf("%d %s", n, bp))
	}
	if def.Reward.SpawnSystems > 0 {
		parts = append(parts, "charts new space")
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
