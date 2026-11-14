package game

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// hiveWord is the hidden faction's name in any case.
var hiveWord = regexp.MustCompile(`(?i)\bhive`)

// In a normal campaign the hive is named by its morph only: no player-facing
// string path says "hive" (review finding 6). It walks a campaign whose hive
// presences have reached the top stage and been seen, everything they did
// known, and scans the Factions tab, the Star Map tags and labels, every
// faction Legends line, the rooms of a top-stage hive's stamp, the HUD's
// faction line and the entity inspector on its structures and bodies.
func TestHiveNeverNamedInANormalCampaign(t *testing.T) {
	observerRunActive = false
	c, wm := expeditionWorld(t, 777)
	fd := campaign.LoadFactionData()
	hive := fd.Doctrine("hive")
	if hive == nil {
		t.Fatal("no hive doctrine")
	}
	top := len(hive.Stages) - 1
	hives := 0
	for _, loc := range c.Locations {
		for i := range loc.Factions {
			if p := &loc.Factions[i]; p.Doctrine == "hive" {
				p.Stage, p.Population, p.Known = top, 40, true
				hives++
			}
		}
	}
	if hives == 0 {
		t.Fatal("fixture: no hive presence on seed 777")
	}
	for day := 0; day < 120; day++ {
		c.StarDate++
		c.StepFactions(nil)
	}
	for _, loc := range c.Locations {
		c.RevealPresences(loc)
		for i := range loc.Factions {
			loc.Factions[i].Known = true
		}
	}
	var leaks []string
	check := func(where, s string) {
		if hiveWord.MatchString(s) {
			leaks = append(leaks, where+": "+s)
		}
	}

	// Stage names, the Factions tab and the Star Map.
	for i := range hive.Stages {
		check("stage word", stageWord(hive, i))
	}
	d := NewDashboardState(c, wm)
	fp := &factionsPanel{c: c, wm: wm, d: d}
	fp.rebuild()
	check("Factions tab", factionsText(fp))
	for _, loc := range c.Locations {
		check("Star Map tag", factionListTag(c, loc))
		check("Star Map label", strings.ToUpper(hive.MorphAt(loc).Tag+" "+stageWord(hive, top)))
		check("orbit alert", hive.OrbitAlert(loc))
	}

	// Legends: every faction event, as the player would read it.
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	h := c.History
	faction := 0
	for i := range h.Events {
		ev := &h.Events[i]
		if !strings.HasPrefix(ev.Type, "faction_") {
			continue
		}
		faction++
		check("Legends "+ev.Type, h.Render(lib, ev, ""))
	}
	if faction == 0 {
		t.Error("fixture: no faction events to read")
	}
	for _, typ := range []string{"faction_grew", "faction_launched"} {
		if def := lib.Events[typ]; def != nil {
			check("Legends template "+typ, strings.NewReplacer("{faction}", "Xeno", "{stage}", stageWord(hive, top), "{singular}", "xeno").Replace(def.Text))
		}
	}

	// A top-stage hive on a level: its rooms, the HUD and the inspector.
	ms, loc := hiveTestSite(t, 7, 30, top, 0)
	materialiseFactions(c, loc, ms.level, ms.scenario)
	seen := map[string]bool{}
	for _, r := range ms.level.Rooms {
		check("room", r.Name)
		seen[r.Name] = true
	}
	if len(seen) == 0 {
		t.Error("fixture: the hive's stamp named no rooms")
	}
	inspected := 0
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(rlcomponents.Description) ||
				e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction != "hive" {
				continue
			}
			inspected++
			check("HUD faction line", hudFactionLabel("hive"))
			title, lines := inspectEntity(c, ms.level, e)
			check("inspector title", title)
			check("inspector", strings.Join(lines, "\n"))
		}
	}
	if inspected == 0 {
		t.Error("fixture: no hive entity to inspect")
	}
	t.Logf("%d faction events, %d rooms, %d hive entities scanned", faction, len(seen), inspected)
	for _, l := range leaks {
		t.Error(l)
	}
}

// The Encyclopedia names a hive creature as the HUD and the inspector do:
// no "Faction: hive" attribute or summary, no "id: hive_..." footer, for
// every blueprint of the hive's (data/blueprints/entities/hive.json); an
// observer run names it.
func TestEncyclopediaNeverNamesTheHive(t *testing.T) {
	observerRunActive = false
	defer func() { observerRunActive = false }()
	raw, err := os.ReadFile("data/blueprints/entities/hive.json")
	if err != nil {
		t.Fatal(err)
	}
	var bps map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bps); err != nil {
		t.Fatal(err)
	}
	card := func(bp string) string {
		desc := factory.GetDescription(bp)
		var parts []string
		for _, a := range encyclopediaAttrs(desc) {
			if a[1] != "" {
				parts = append(parts, a[0]+": "+a[1])
			}
		}
		parts = append(parts, displayName(bp, desc), synthDescription(bp, desc))
		if id := encyclopediaID(bp, desc); id != "" {
			parts = append(parts, "id: "+id)
		}
		return strings.Join(parts, "\n")
	}
	n := 0
	for bp := range bps {
		desc := factory.GetDescription(bp)
		if desc == nil || desc.Faction != "hive" {
			continue
		}
		n++
		if s := card(bp); hiveWord.MatchString(s) {
			t.Errorf("%s card names the hive:\n%s", bp, s)
		}
	}
	if n == 0 {
		t.Fatal("fixture: no hive blueprint")
	}
	observerRunActive = true
	if s := card("hive_heart"); !strings.Contains(s, "Faction: hive") || !strings.Contains(s, "id: hive_heart") {
		t.Errorf("observer run: the card should name the hive:\n%s", s)
	}
}
