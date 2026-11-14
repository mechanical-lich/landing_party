package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/ecs"
)

// Every doctrine's interface block names only registered kinds; an unknown
// section, order, resource source, crew group, panel row kind, source or
// action fails loudly.
func TestInterfaceBlocksValidate(t *testing.T) {
	useContentTables(t)
	if err := validateInterfaces(); err != nil {
		t.Fatal(err)
	}
	bad := []string{
		`{"unit_modal": ["identity", "horoscope"]}`,
		`{"orders": [{"id": "teleport"}]}`,
		`{"resources": [{"id": "mana", "source": "ley_lines"}]}`,
		`{"crew_group": "star_sign"}`,
		`{"panels": {"egg_launcher": {"title": "x", "rows": [{"kind": "hologram"}]}}}`,
		`{"panels": {"egg_launcher": {"title": "x", "rows": [{"kind": "readiness", "source": "weather"}]}}}`,
		`{"panels": {"egg_launcher": {"title": "x", "rows": [{"kind": "action", "action": "self_destruct"}]}}}`,
	}
	for _, js := range bad {
		var ui campaign.DoctrineInterface
		if err := json.Unmarshal([]byte(js), &ui); err != nil {
			t.Fatal(err)
		}
		if validateInterface(&ui) == nil {
			t.Errorf("%s validated", js)
		}
	}
}

// The colony declares its interface as it was before doctrines: its spec
// draws the same resource bar, orders, titles and unit modal as an empty
// one (the defaults).
func TestColonyInterfaceIsTheOldOne(t *testing.T) {
	c := newE2EColony(t)
	gm := c.ms.guiManager
	gm.SetInterface(gui.InterfaceSpec{})
	slots, orders := gm.ResourceSlots(), gm.OrderIDs()
	research, unit := gm.ResearchTitle(), gm.UnitModalTitle()
	gm.ShowUnitModal(c.colonist, nil, "")
	sections := gm.UnitModalShown()

	gm.SetInterface(interfaceSpec(colonySettlement(t)))
	gm.ShowUnitModal(c.colonist, nil, unitTypeLine(nil, c.colonist))
	if got := gm.ResourceSlots(); !reflect.DeepEqual(got, slots) {
		t.Errorf("resource slots %v, want %v", got, slots)
	}
	if got := gm.OrderIDs(); !reflect.DeepEqual(got, orders) {
		t.Errorf("orders %v, want %v", got, orders)
	}
	if gm.ResearchTitle() != research || gm.UnitModalTitle() != unit {
		t.Errorf("titles %q / %q, want %q / %q", gm.ResearchTitle(), gm.UnitModalTitle(), research, unit)
	}
	if got := gm.UnitModalShown(); !reflect.DeepEqual(got, sections) || !contains(sections, gui.SectionEquipment) {
		t.Errorf("unit modal %v, want %v (with equipment)", got, sections)
	}
}

// A hive run's HUD is the hive's: biomass and larvae, Lay Resin, Evolve, a
// unit modal with the type and no equipment, the crew panel grouped by type.
func TestHiveRunInterface(t *testing.T) {
	_, wm, ms := hiveRun(t, 7, 0)
	gm := ms.guiManager
	if got := gm.ResourceSlots(); !reflect.DeepEqual(got, []string{"biomass", "larvae"}) {
		t.Errorf("resource slots %v", got)
	}
	ms.refreshHUD()
	if gm.ResourceValue("biomass") <= 0 || gm.ResourceValue("larvae") <= 0 {
		t.Errorf("biomass %d, larvae %d at the start", gm.ResourceValue("biomass"), gm.ResourceValue("larvae"))
	}
	orders := gm.OrderIDs()
	if !contains(orders, "convert") || contains(orders, "sleep") {
		t.Errorf("orders %v: want convert, not sleep", orders)
	}
	if gm.ResearchTitle() != "Evolve" {
		t.Errorf("research title %q", gm.ResearchTitle())
	}
	var drone = settlementWorkers(ms.level, ms.colonyName())[0]
	ms.openColonistModal(drone)
	got := gm.UnitModalShown()
	if !contains(got, gui.SectionTypeTier) || contains(got, gui.SectionEquipment) {
		t.Errorf("unit modal %v: want type_tier, no equipment", got)
	}
	p := newCrewPanel(ms.campaign, wm)
	if len(p.crew) == 0 || p.crew[0].site == "" || !strings.Contains(p.list.Items[0], "  —  ") {
		t.Errorf("crew rows %v: want grouped rows of the brood", p.list.Items)
	}
}

// Lay Resin: the player's convert order is open to a doctrine with a
// conversion tile and still refused to the colony.
func TestConvertOrderByDoctrine(t *testing.T) {
	c := newE2EColony(t)
	if c.ms.convertTile() != "" || c.ms.queueConvert(3, 3, 0, "hive_resin") {
		t.Error("the colony may convert without terraforming")
	}
}

func TestConvertOrderForAHive(t *testing.T) {
	_, _, ms := hiveRun(t, 7, 0)
	st := ms.MainSettlement
	x, y, z := st.CenterX+4, st.CenterY+4, st.CenterZ
	if ms.convertTile() != "hive_resin" || !ms.queueConvert(x, y, z, "hive_resin") {
		t.Errorf("a hive player cannot lay resin (tile %q)", ms.convertTile())
	}
}

// A made-up fourth doctrine reusing existing section kinds renders its unit
// modal with no Go change: the sections it declares, in kind order, and its
// type line.
func TestFourthDoctrineUnitModal(t *testing.T) {
	c := newE2EColony(t)
	var st campaign.DoctrineSettlement
	fixture := `{"interface": {"resources": [{"id": "metal_ore", "source": "storage"}, {"id": "food", "source": "food"}],
		"unit_label": "Mycelid", "units_label": "Colony", "research_label": "Sporulate",
		"orders": [{"id": "default"}, {"id": "mine", "label": "Digest"}, {"id": "convert", "label": "Spread Mycelium"}],
		"unit_modal": ["identity", "type_tier", "needs", "inventory"], "crew_group": "type_tier"}}`
	if err := json.Unmarshal([]byte(fixture), &st); err != nil {
		t.Fatal(err)
	}
	if err := validateInterface(st.Interface); err != nil {
		t.Fatal(err)
	}
	gm := c.ms.guiManager
	gm.SetInterface(interfaceSpec(&st))
	gm.ShowUnitModal(c.colonist, nil, "Sporeling · elder")
	if got := gm.UnitModalShown(); !reflect.DeepEqual(got, []string{"identity", "type_tier", "needs", "inventory"}) {
		t.Errorf("sections %v", got)
	}
	if gm.UnitModalTitle() != "Mycelid" || gm.ResearchTitle() != "Sporulate" {
		t.Errorf("titles %q / %q", gm.UnitModalTitle(), gm.ResearchTitle())
	}
	if got := gm.OrderIDs(); !reflect.DeepEqual(got, []string{"default", "mine", "convert"}) {
		t.Errorf("orders %v", got)
	}
}

// A race run's techs are one pool: an evolve learned on one world reaches
// another world's presence, its settlement when loaded, and an egg in flight.
func TestRaceRunSharedTechs(t *testing.T) {
	c, wm, ms := hiveRun(t, 7, 0)
	st := ms.MainSettlement
	st.UnlockTech("hive_warrior_brood")
	var other *campaign.Location
	for _, l := range c.Locations {
		if l.ID != campaign.StartLocationID && l.HasSite() && len(l.Factions) == 0 {
			other = l
			break
		}
	}
	other.Factions = append(other.Factions, campaign.FactionPresence{ID: "fpx", Faction: campaign.PlayerFaction, Doctrine: "hive", Population: 3})
	c.Fleets = append(c.Fleets, campaign.Fleet{ID: "egg", Kind: campaign.FleetProjectile, Presence: campaign.FactionPresence{Faction: campaign.PlayerFaction, Doctrine: "hive"}})
	wm.syncPlayerTechs()
	if !containsStr(other.Presence(campaign.PlayerFaction).KnownTechs, "hive_warrior_brood") {
		t.Error("the other world's presence does not know the evolve")
	}
	if !containsStr(c.Fleets[len(c.Fleets)-1].Presence.KnownTechs, "hive_warrior_brood") || !containsStr(c.Player.KnownTechs, "hive_warrior_brood") {
		t.Error("the egg in flight or the pool lacks the evolve")
	}
	other.Factions[0].KnownTechs = append(other.Factions[0].KnownTechs, "hive_resin_flow_1")
	wm.syncPlayerTechs()
	if !settlement.Settlements[st.Name].HasTech("hive_resin_flow_1") {
		t.Error("the loaded world's settlement did not get the other's evolve")
	}
}

func contains(list []string, s string) bool { return containsStr(list, s) }

// The Brood Heart's panel lists the rung groups; taking every one over
// leaves the brood's director ordering nothing of them, while a world left
// alone keeps ordering.
func TestTakenOverGroupsSkipTheirRungs(t *testing.T) {
	_, wm, ms := hiveRun(t, 7, 0)
	heart := firstOf(ms.level.Entities, ms.level.StaticEntities, "hive_heart")
	if heart == nil {
		t.Fatal("no heart")
	}
	ms.openStructurePanel(heart)
	if got := ms.guiManager.StructurePanelShown(); !reflect.DeepEqual(got, []string{"world_toggle", "text", "group_toggles"}) || ms.guiManager.StructurePanelTitle() != "Brood Heart" {
		t.Fatalf("heart panel %q %v", ms.guiManager.StructurePanelTitle(), got)
	}
	for _, g := range ms.doctrineSettlement().UI().Groups {
		ms.panelToggle(g.ID, true)
	}
	name := ms.MainSettlement.Name
	before := map[string]int{}
	stepRun(wm, ms, 30) // the director's first plan
	dir := ms.factionDirectors[name]
	for k, v := range dir.Ordered {
		before[k] = v
	}
	stepRun(wm, ms, 1500)
	for k, v := range dir.Ordered {
		for _, pre := range []string{"grow:", "build:", "convert:", "research:", "mine:", "haul:"} {
			if strings.HasPrefix(k, pre) && v > before[k] {
				t.Errorf("taken over, the director still ordered %s (%d → %d)", k, before[k], v)
			}
		}
	}
	ms.panelToggle("breeding", false)
	stepRun(wm, ms, 1500)
	grew := 0
	for k, v := range dir.Ordered {
		if strings.HasPrefix(k, "grow:") {
			grew += v - before[k]
		}
	}
	if grew == 0 {
		t.Error("given breeding back, the director laid no egg")
	}
}

// The Spire's panel shows the launcher's readiness and the worlds in reach;
// Aim at takes the launch over and arms the target.
func TestSpirePanelAims(t *testing.T) {
	c, _, ms := hiveRun(t, 7, 0)
	st := ms.MainSettlement
	spire, err := factory.Create("egg_launcher", st.CenterX+3, st.CenterY, st.CenterZ)
	if err != nil {
		t.Fatal(err)
	}
	ms.level.AddEntity(spire)
	ms.openStructurePanel(spire)
	if got := ms.guiManager.StructurePanelShown(); !reflect.DeepEqual(got, []string{"readiness", "target_list", "action", "action"}) || ms.guiManager.StructurePanelTitle() != "Resin Spire" {
		t.Fatalf("spire panel %q %v", ms.guiManager.StructurePanelTitle(), got)
	}
	pr := ms.playerPresenceHere()
	targets := ms.panelTargets("launch_targets", pr)
	if len(targets) == 0 {
		t.Skip("no world in reach on this seed")
	}
	ms.panelAction("launch_at", targets[0].ID)
	if pr.LaunchTarget != targets[0].ID {
		t.Errorf("aim %q, want %s", pr.LaunchTarget, targets[0].ID)
	}
	_ = c
}

// A made-up fourth doctrine's panel, from existing row kinds, sources and
// actions, renders with no Go change.
func TestFourthDoctrinePanel(t *testing.T) {
	_, _, ms := hiveRun(t, 7, 0)
	var p campaign.InterfacePanel
	fixture := `{"title": "Spore Tower", "rows": [{"kind": "text", "label": "Release when ripe."},
		{"kind": "readiness", "source": "signal"}, {"kind": "target_list", "source": "signal_targets"},
		{"kind": "action", "action": "transmit_to", "label": "Release"}, {"kind": "group_toggles"}]}`
	if err := json.Unmarshal([]byte(fixture), &p); err != nil {
		t.Fatal(err)
	}
	if err := validatePanel(&p); err != nil {
		t.Fatal(err)
	}
	ms.guiManager.OpenStructurePanel(ms.panelView(&p))
	if got := ms.guiManager.StructurePanelShown(); !reflect.DeepEqual(got, []string{"text", "readiness", "target_list", "action", "group_toggles"}) || ms.guiManager.StructurePanelTitle() != "Spore Tower" {
		t.Errorf("panel %q %v", ms.guiManager.StructurePanelTitle(), got)
	}
}

func firstOf(a, b []*ecs.Entity, bp string) *ecs.Entity {
	for _, list := range [][]*ecs.Entity{a, b} {
		for _, e := range list {
			if e != nil && e.Blueprint == bp {
				return e
			}
		}
	}
	return nil
}

// "I run this world": one switch takes every group over, and the brood's
// director then orders nothing at all on that world (not a patrol, a guard
// post or an excavation either: every rung has a group); switched back, it
// takes up its rungs again.
func TestRunThisWorld(t *testing.T) {
	_, wm, ms := hiveRun(t, 7, 0)
	heart := firstOf(ms.level.Entities, ms.level.StaticEntities, "hive_heart")
	ms.openStructurePanel(heart)
	if got := ms.guiManager.StructurePanelShown(); len(got) == 0 || got[0] != "world_toggle" {
		t.Fatalf("heart panel rows %v", got)
	}
	d := campaign.PlayerDoctrine(ms.campaign)
	for _, p := range d.Settlement.Director.Priorities {
		if p.Group == "" {
			t.Errorf("rung %q has no group: the player could never take it over", p.ID)
		}
	}
	ms.panelToggle(worldToggle, true)
	pr := ms.playerPresenceHere()
	for _, g := range d.Settlement.UI().Groups {
		if !pr.Manual(g.ID) {
			t.Fatalf("group %s not taken over", g.ID)
		}
	}
	stepRun(wm, ms, 30)
	dir := ms.factionDirectors[ms.MainSettlement.Name]
	before := map[string]int{}
	for k, v := range dir.Ordered {
		before[k] = v
	}
	stepRun(wm, ms, 1500)
	for k, v := range dir.Ordered {
		if v > before[k] {
			t.Errorf("running the world by hand, the director still ordered %s (%d → %d)", k, before[k], v)
		}
	}
	ms.panelToggle(worldToggle, false)
	if len(pr.TakenOver) != 0 {
		t.Errorf("given back, still taken over: %v", pr.TakenOver)
	}
}

// Post Guard: the nearest idle unit with a guard role walks to the tile and
// holds there; with none, nothing goes.
func TestPostGuard(t *testing.T) {
	_, _, ms := hiveRun(t, 7, 0)
	st := ms.MainSettlement
	x, y, z := st.CenterX+5, st.CenterY, st.CenterZ
	if ms.postAt(x, y, z) {
		t.Fatal("posted with no warrior")
	}
	w, err := spawnFactionWorker(ms.level, "xeno_crew", st.Name, campaign.PlayerFaction, st.CenterX, st.CenterY+1, z)
	if err != nil {
		t.Fatal(err)
	}
	w.GetComponent(components.Worker).(*components.WorkerComponent).Role = "guard"
	if !ms.postAt(x, y, z) {
		t.Fatal("the warrior was not posted")
	}
	tk := w.GetComponent(components.Worker).(*components.WorkerComponent).CurrentTask
	if tk == nil || tk.Data != task_requests.HoldHere || tk.X != x || tk.Y != y {
		t.Errorf("warrior's task %+v, want a hold at %d,%d", tk, x, y)
	}
}

// Load the Spire: the launch's shot of the brood's drones and scouts walks to
// hold round the Spire (the launch taken over), never a warrior; the live
// launch counts them as it counts the launch rung's.
func TestLoadTheSpire(t *testing.T) {
	_, _, ms := hiveRun(t, 7, 0)
	st := ms.MainSettlement
	spire, err := factory.Create("egg_launcher", st.CenterX+3, st.CenterY, st.CenterZ)
	if err != nil {
		t.Fatal(err)
	}
	ms.level.AddEntity(spire)
	ms.openStructurePanel(spire)
	ms.panelAction("load_launcher", "")
	pr := ms.playerPresenceHere()
	d := campaign.PlayerDoctrine(ms.campaign)
	if !pr.Manual(campaign.GroupLaunch) {
		t.Error("loading did not take the launch over")
	}
	held := 0
	for _, e := range settlementWorkers(ms.level, st.Name) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && wc.CurrentTask.Data == task_requests.HoldHere {
			if wc.Role == "guard" {
				t.Error("a warrior went to load the Spire")
			}
			held++
		}
	}
	if held != d.Mobility.ShotPop {
		t.Errorf("%d holding by the Spire, want the shot of %d", held, d.Mobility.ShotPop)
	}
}

// The take_over order needs a world_panel to open, and a world panel holds
// no action on a structure (it stands for none).
func TestWorldPanelValidation(t *testing.T) {
	ui := &campaign.DoctrineInterface{Orders: []campaign.InterfaceOrder{{ID: "take_over"}}}
	if validateInterface(ui) == nil {
		t.Error("take_over with no world_panel passed")
	}
	ui.WorldPanel = &campaign.InterfacePanel{Rows: []campaign.PanelRow{{Kind: "world_toggle"}, {Kind: "group_toggles"}}}
	if err := validateInterface(ui); err != nil {
		t.Errorf("a world panel of toggles: %v", err)
	}
	ui.WorldPanel.Rows = append(ui.WorldPanel.Rows, campaign.PanelRow{Kind: "action", Action: "load_launcher"})
	if validateInterface(ui) == nil {
		t.Error("a world panel loading a launcher passed")
	}
}
