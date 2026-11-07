package campaign

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

// loreHistoryQuestKind names which history template a bound quest came from.
func loreHistoryQuestKind(q *Quest) string {
	switch {
	case !strings.HasPrefix(q.ID, "hq_"):
		return ""
	case strings.Contains(q.Name, "'s Log"):
		return "figure_log"
	case strings.HasPrefix(q.Name, "Relics of the"):
		return "civ_relic"
	case strings.HasPrefix(q.Name, "Source of the Threat"):
		return "chapter_origin"
	}
	return "?"
}

// Across 300 charted campaigns every history template binds, with its fills
// read from the site's history (no {placeholder} left), and binding moves no
// site (the golden test covers seeds; here the quests themselves).
func TestLoreHistoryQuestsBindAcrossCampaigns(t *testing.T) {
	useRepoData(t)
	counts := map[string]int{}
	var example string
	for seed := int64(1); seed <= 300; seed++ {
		c := loreCampaign(t, seed)
		for i := range c.QuestDefs {
			q := &c.QuestDefs[i]
			kind := loreHistoryQuestKind(q)
			if kind == "" {
				continue
			}
			counts[kind]++
			if strings.ContainsAny(q.Name+q.Description, "{}") {
				t.Errorf("seed %d: unfilled quest %q / %q", seed, q.Name, q.Description)
			}
			if q.EventID == "" || c.History.Event(q.EventID).Origin == nil {
				t.Errorf("seed %d: %s is not about an event with an origin", seed, q.ID)
			}
			if q.Objective.Flag != KnownFlag(q.EventID) {
				t.Errorf("seed %d: %s waits on %q", seed, q.ID, q.Objective.Flag)
			}
			if kind == "civ_relic" && example == "" {
				example = q.Name + ": " + q.Description
			}
		}
	}
	t.Logf("history quests over 300 campaigns: %v; e.g. %s", counts, example)
	for _, k := range []string{"figure_log", "civ_relic", "chapter_origin"} {
		if counts[k] == 0 {
			t.Errorf("template %s never bound", k)
		}
	}
	if counts["?"] > 0 {
		t.Errorf("%d history quests of no known template", counts["?"])
	}
}

// A history quest completes when its event becomes known, through the normal
// evaluation, and its completion is recorded.
func TestLoreHistoryQuestCompletesWhenItsEventIsKnown(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 50; seed++ {
		c := loreCampaign(t, seed)
		for i := range c.QuestDefs {
			q := &c.QuestDefs[i]
			if loreHistoryQuestKind(q) == "" {
				continue
			}
			c.CurrentLocationID = q.Location
			c.Quests[q.ID].Status = QuestActive
			if done := c.EvaluateQuests(objective.EvalContext{}); len(done) != 0 {
				t.Fatalf("seed %d: %s completed before its event was known", seed, q.ID)
			}
			c.RevealEvent(q.EventID) // its datapad, or a colonist at its origin
			done := c.EvaluateQuests(objective.EvalContext{})
			if len(done) != 1 || done[0].ID != q.ID || c.Quests[q.ID].Status != QuestCompleted {
				t.Fatalf("seed %d: %s did not complete once its event was known", seed, q.ID)
			}
			if loreLiveCount(c.History, "quest_completed") != 1 {
				t.Error("completion not recorded")
			}
			return
		}
	}
	t.Fatal("no history quest in 50 campaigns")
}
