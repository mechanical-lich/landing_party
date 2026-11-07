package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/history"
)

// Legends lists only known events, in the dump's words, and its filters keep
// one site's, civ's or figure's events.
func TestLoreLegendsListsKnownEvents(t *testing.T) {
	c, err := campaign.GenerateCampaign("Legends", 9)
	if err != nil {
		t.Fatal(err)
	}
	p := newLegendsPanel(c)
	if len(p.lines) != 0 {
		t.Fatalf("a new campaign shows %d events", len(p.lines))
	}
	lib, _ := history.DefaultLibrary()
	var sites []*history.SiteHistory
	for _, loc := range c.Locations {
		if s := c.SiteHistory(loc); s != nil && len(s.Events) > 0 {
			sites = append(sites, s)
		}
	}
	if len(sites) < 2 {
		t.Fatal("need two sites with history")
	}
	text, _ := c.RevealEvent(sites[0].Events[0])
	c.RevealEvent(sites[1].Events[0])
	p.Enter()
	if len(p.lines) != 2 || !strings.Contains(strings.Join(p.lines, "\n"), text) {
		t.Fatalf("legends shows %v, want the two known events incl. %q", p.lines, text)
	}
	for i, k := range p.keys {
		if k == "site:"+sites[0].ID {
			p.filters.SelectedIndex = i
		}
	}
	p.Update()
	if len(p.lines) != 1 || p.lines[0] != text {
		t.Errorf("filtered to %s: %v", sites[0].ID, p.lines)
	}
	ev := c.History.Event(sites[0].Events[0])
	if want := c.History.Render(lib, ev, c.Locations[sites[0].Location].Name); want != text {
		t.Errorf("legends text %q differs from the dump's %q", text, want)
	}
}
