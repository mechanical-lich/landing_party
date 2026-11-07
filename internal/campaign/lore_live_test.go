package campaign

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

func loreLiveCount(h *history.History, typ string) int {
	n := 0
	for _, e := range h.Events {
		if e.Type == typ {
			n++
		}
	}
	return n
}

// Completing a quest records it and makes the event behind it known.
func TestLoreQuestCompletionRecordsAndReveals(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 5)
	var q *Quest
	for i := range c.QuestDefs {
		if c.QuestDefs[i].EventID != "" {
			q = &c.QuestDefs[i]
			break
		}
	}
	if q == nil {
		t.Fatal("no quest carries an event")
	}
	if c.History.Known[q.EventID] {
		t.Fatal("event known before the quest was done")
	}
	c.recordQuestDone(q)
	if !c.History.Known[q.EventID] || loreLiveCount(c.History, "quest_completed") != 1 {
		t.Errorf("completion: known %v, records %d", c.History.Known[q.EventID], loreLiveCount(c.History, "quest_completed"))
	}
}

// Taking a contract records it.
func TestLoreContractTakenIsRecorded(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 5)
	for i := range c.QuestDefs {
		q := &c.QuestDefs[i]
		if !q.Contract {
			continue
		}
		if !c.AcceptQuest(q.ID) {
			t.Fatalf("could not accept contract %s", q.ID)
		}
		if loreLiveCount(c.History, "contract_taken") != 1 {
			t.Error("taking the contract was not recorded")
		}
		return
	}
	t.Fatal("no contract in the campaign")
}

// A ship scan that charts a site reads its story: the site's summary event
// becomes known.
func TestLoreScanRevealsTheSitesStory(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 5)
	for i := 0; i < 200; i++ {
		before := c.KnownEventCount()
		loc, ran := c.Scan(3)
		if !ran {
			t.Fatal("scan did not run")
		}
		if loc == nil {
			continue
		}
		if ev := c.storyEvent(loc); ev == "" || !c.History.Known[ev] || c.KnownEventCount() != before+1 {
			t.Errorf("scanned %s: story %q known %v", loc.ID, ev, c.History.Known[ev])
		}
		return
	}
	t.Fatal("200 scans charted nothing")
}
