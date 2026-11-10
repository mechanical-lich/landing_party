package game

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
)

// The inspector reads a lore datapad where it lies with the words a colonist
// picking it up posts, and reading it changes nothing; picking it up then
// keeps those words for the Legends tab's "Datapads read", across a save.
func TestInspectDatapadReadsWhatPickingItUpPosts(t *testing.T) {
	loreLoadData(t)
	c, wm, err := newExpedition("Pads", 1512, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := wm.Travel("sys_10"); err != nil {
		t.Fatal(err)
	}
	ms := wm.Current()
	var pad *ecs.Entity
	for _, e := range ms.level.Entities {
		if dp, ok := e.GetComponent(components.Datapad).(*components.DatapadComponent); ok && dp.EventID != "" && dp.QuestID == "" && !dp.Log {
			pad = e
			break
		}
	}
	if pad == nil {
		t.Fatal("no lore datapad on sys_10")
	}
	dp := pad.GetComponent(components.Datapad).(*components.DatapadComponent)
	known := c.KnownEventCount()

	title, lines := inspectEntity(c, ms.level, pad)
	text := strings.Join(lines, "\n")
	want, _ := datapadReading(c, dp) // its own words, else its event's account
	if want == "" || !strings.Contains(text, "— Datapad —\n"+want) {
		t.Fatalf("inspector %q shows\n%s\nwant the datapad text %q", title, text, want)
	}
	if c.KnownEventCount() != known || c.History.Known[dp.EventID] {
		t.Error("inspecting a datapad made its event known")
	}

	pc := pad.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if _, err := spawnColonist(ms.level, ms.colonyName(), pc.GetX(), pc.GetY(), pc.GetZ()); err != nil {
		t.Fatal(err)
	}
	message.MessageLog = nil
	ms.collectDatapads()
	event.GetQueuedInstance().HandleQueue()
	if !c.History.Known[dp.EventID] {
		t.Error("picking the datapad up did not make its event known")
	}
	posted := false
	for _, l := range message.MessageLog {
		posted = posted || strings.Contains(l, "Recovered a datapad: "+want)
	}
	if !posted {
		t.Errorf("pickup posted %v, want %q", message.MessageLog, want)
	}
	if len(c.DatapadsRead) != 1 || c.DatapadsRead[0].Text != want || c.DatapadsRead[0].Site != c.Locations["sys_10"].Name {
		t.Fatalf("datapads read %+v, want one with %q", c.DatapadsRead, want)
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back campaign.Campaign
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if lines := datapadsReadLines(&back); len(lines) != 1 || !strings.Contains(lines[0], want) {
		t.Errorf("after a save the Datapads read list is %v", lines)
	}
}

// A container lists its contents by count, a creature its stats and needs.
func TestInspectStorageAndCreature(t *testing.T) {
	loreLoadData(t)
	locker, err := factory.Create("storage_locker", 3, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	sc := locker.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = "Rust Colony"
	for range 2 {
		stone, err := factory.Create("stone", 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		stone.GetComponent(components.Material).(*components.MaterialComponent).Quantity = 7
		sc.Items = append(sc.Items, stone)
	}
	_, lines := inspectEntity(nil, nil, locker)
	text := strings.Join(lines, "\n")
	for _, want := range []string{"Owner: Rust Colony", "Contents (2", "x14"} {
		if !strings.Contains(text, want) {
			t.Errorf("locker inspector lacks %q:\n%s", want, text)
		}
	}

	xeno, err := factory.Create("xeno_crew", 5, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	// An observer run names the hive; a normal campaign does not (below).
	observerRunActive = true
	_, lines = inspectEntity(nil, nil, xeno)
	observerRunActive = false
	text = strings.Join(lines, "\n")
	for _, want := range []string{"Blueprint: xeno_crew", "— Stats —", "Health:", "AC:", "Initiative:", "Hunger:"} {
		if !strings.Contains(text, want) {
			t.Errorf("xeno inspector lacks %q:\n%s", want, text)
		}
	}
}
