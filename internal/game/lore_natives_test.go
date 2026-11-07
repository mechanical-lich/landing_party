package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
)

func loreLogHas(text string) bool {
	event.GetQueuedInstance().HandleQueue()
	for _, line := range message.MessageLog {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
}

// Yeti stand where the map puts them; the history only names the ancient
// people they descend from.
func TestLoreYetiNamedByTheirHistory(t *testing.T) {
	message.MessageLog = nil
	ms, _, _, _ := loreBuild(t, loreSite{Map: "frozen_planet", Scenario: "yeti_territory", Seed: 71, Chapter: "yeti_return",
		Flags: map[string]any{"origin_civ": "Ulrani"}})
	if !loreLogHas("descendants of the Ulrani") {
		t.Errorf("no message names the Ulrani: %v", message.MessageLog)
	}
	plain, _, _, _ := loreBuild(t, loreSite{Map: "frozen_planet", Scenario: "yeti_territory", Seed: 71})
	yeti := []string{"yeti", "elder_yeti", "ancient_yeti"}
	if a, b := lorePositions(ms.level, yeti), lorePositions(plain.level, yeti); strings.Join(a, " ") != strings.Join(b, " ") {
		t.Errorf("the history moved the yeti:\n %v\n %v", a, b)
	}
}

func TestLoreNativesUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "frozen_planet", "yeti_territory", "yeti_territory_setup", 71, []string{"yeti", "elder_yeti", "ancient_yeti"})
	loreUnchangedWithoutHistory(t, "moon", "worms", "worms_setup", 73, []string{"worm"})
}
