package game

import (
	"fmt"
	"math"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
)

// Structure panels (phase F; docs/developer/factions.md, "The interface
// block"): a doctrine's interface.panels maps a structure blueprint to a
// panel of rows, each a row kind over a value source or an action from the
// registries below. The Resin Spire and the Signal Core are two data entries
// over this one implementation.

// panelRowKinds is every panel row kind: readiness (a source's status
// lines), target_list (a source's selectable sites), action (a button that
// runs an action on the selected target), text (the row's label).
var panelRowKinds = map[string]bool{"readiness": true, "target_list": true, "action": true, "text": true, "group_toggles": true, "world_toggle": true}

// panelSources is every value source a row may read: kind → the sources it
// accepts.
var panelSources = map[string]map[string]bool{
	"readiness":   {"launcher": true, "signal": true},
	"target_list": {"launch_targets": true, "signal_targets": true},
}

// panelActions is every action an action row may run.
var panelActions = map[string]bool{"launch_at": true, "transmit_to": true, "load_launcher": true}

// validateWorldPanel checks the world's panel: a panel's rows, none of
// them an action on the structure it stands for (it stands for none).
func validateWorldPanel(p *campaign.InterfacePanel) error {
	if err := validatePanel(p); err != nil {
		return err
	}
	for i, r := range p.Rows {
		if r.Kind == "action" && r.Action == "load_launcher" {
			return fmt.Errorf("row %d: %q needs a structure", i, r.Action)
		}
	}
	return nil
}

// validatePanel checks one panel's rows against the registries.
func validatePanel(p *campaign.InterfacePanel) error {
	if p == nil {
		return fmt.Errorf("empty")
	}
	for i, r := range p.Rows {
		if !panelRowKinds[r.Kind] {
			return fmt.Errorf("row %d: unknown kind %q", i, r.Kind)
		}
		if srcs, ok := panelSources[r.Kind]; ok && !srcs[r.Source] {
			return fmt.Errorf("row %d: unknown %s source %q", i, r.Kind, r.Source)
		}
		if r.Kind == "action" && !panelActions[r.Action] {
			return fmt.Errorf("row %d: unknown action %q", i, r.Action)
		}
	}
	return nil
}

// panelFor is the player's doctrine's panel for ent's blueprint, nil for
// none.
func (s *MainState) panelFor(ent *ecs.Entity) *campaign.InterfacePanel {
	if ent == nil {
		return nil
	}
	return s.doctrineSettlement().UI().Panels[ent.Blueprint]
}

// playerPresenceHere is the player's presence on this level's location (a
// race run), nil for none.
func (s *MainState) playerPresenceHere() *campaign.FactionPresence {
	if s.campaign == nil {
		return nil
	}
	if loc := s.campaign.Locations[s.locationID]; loc != nil {
		return loc.Presence(campaign.PlayerFaction)
	}
	return nil
}

// openStructurePanel opens ent's panel, its rows resolved from their
// sources (the registry above).
func (s *MainState) openStructurePanel(ent *ecs.Entity) {
	p := s.panelFor(ent)
	if p == nil || s.guiManager == nil {
		return
	}
	s.panelStructure, s.panelWorld = ent, false
	s.guiManager.OpenStructurePanel(s.panelView(p))
}

// openWorldPanel opens the world's own panel (interface.world_panel: the
// take_over order), on a world the player gives orders on.
func (s *MainState) openWorldPanel() {
	p := s.doctrineSettlement().UI().WorldPanel
	if p == nil || s.guiManager == nil {
		return
	}
	if !s.ordersHere() || s.playerPresenceHere() == nil {
		message.PostMessage("Orders", "Nothing of yours to run here.") // DRAFT
		return
	}
	s.panelStructure, s.panelWorld = nil, true
	s.guiManager.OpenStructurePanel(s.panelView(p))
}

// refreshStructurePanel redraws the open panel (after an action or a
// toggle).
func (s *MainState) refreshStructurePanel() {
	if s.guiManager == nil || !s.guiManager.StructurePanelVisible() {
		return
	}
	switch {
	case s.panelWorld:
		s.openWorldPanel()
	case s.panelStructure != nil:
		s.openStructurePanel(s.panelStructure)
	}
}

// panelView resolves p's rows.
func (s *MainState) panelView(p *campaign.InterfacePanel) gui.PanelView {
	v := gui.PanelView{Title: p.Title}
	pr := s.playerPresenceHere()
	for _, r := range p.Rows {
		row := gui.PanelRowView{Kind: r.Kind, Label: r.Label, Action: r.Action}
		switch r.Kind {
		case "readiness":
			row.Lines = s.readinessLines(r.Source, pr)
		case "target_list":
			row.Targets = s.panelTargets(r.Source, pr)
		case "world_toggle":
			// One switch for every group: the player runs this world.
			all := pr != nil && len(s.doctrineSettlement().UI().Groups) > 0
			for _, g := range s.doctrineSettlement().UI().Groups {
				all = all && pr.Manual(g.ID)
			}
			row.Toggles = []gui.PanelToggle{{ID: worldToggle, Label: r.Label, On: all}}
		case "group_toggles":
			for _, g := range s.doctrineSettlement().UI().Groups {
				row.Toggles = append(row.Toggles, gui.PanelToggle{ID: g.ID, Label: g.Label, On: pr != nil && pr.Manual(g.ID)})
			}
		}
		v.Rows = append(v.Rows, row)
	}
	return v
}

// spreadStatus is the player's presence here's launcher or signal status.
func (s *MainState) spreadStatus(source string, pr *campaign.FactionPresence) (campaign.SpreadStatus, bool) {
	if pr == nil || s.campaign == nil || s.wm == nil {
		return campaign.SpreadStatus{}, false
	}
	loc := s.campaign.Locations[s.locationID]
	switch source {
	case "launcher", "launch_targets":
		return s.campaign.LaunchStatus(loc, pr, s.wm.IsLoaded), true
	case "signal", "signal_targets":
		return s.campaign.SignalStatus(loc, pr, s.wm.IsLoaded), true
	}
	return campaign.SpreadStatus{}, false
}

// readinessLines are a launcher's or signal's status lines (DRAFT words,
// the same for every doctrine: the source is the mechanic, not the race).
func (s *MainState) readinessLines(source string, pr *campaign.FactionPresence) []string {
	st, ok := s.spreadStatus(source, pr)
	if !ok {
		return []string{"Nothing of yours here."} // DRAFT
	}
	yes := func(b bool) string {
		if b {
			return "yes"
		}
		return "not yet"
	}
	lines := []string{
		fmt.Sprintf("Numbers: %d / %d", st.Pop, st.Threshold),                     // DRAFT
		"Grown enough: " + yes(st.StageOK),                                        // DRAFT
		"Structure standing: " + yes(!st.StructureLost),                           // DRAFT
		fmt.Sprintf("Range: %.0f, %d worlds in reach", st.Range, len(st.Targets)), // DRAFT
	}
	if st.CooldownLeft > 0 {
		lines = append(lines, fmt.Sprintf("Recovering: %d days", st.CooldownLeft)) // DRAFT
	} else {
		lines = append(lines, "Recovered") // DRAFT
	}
	if pr.LaunchTarget != "" {
		lines = append(lines, "Aimed at "+s.locName(pr.LaunchTarget)) // DRAFT
	} else {
		lines = append(lines, "Aim: none (it will not fire until you aim it)") // DRAFT
	}
	return lines
}

// panelTargets are the sites a launcher or signal could found on now.
func (s *MainState) panelTargets(source string, pr *campaign.FactionPresence) []gui.PanelTarget {
	st, ok := s.spreadStatus(source, pr)
	if !ok || s.campaign == nil {
		return nil
	}
	loc := s.campaign.Locations[s.locationID]
	var out []gui.PanelTarget
	for _, t := range st.Targets {
		label := fmt.Sprintf("%s  (%.0f)", t.Name, math.Hypot(t.X-loc.X, t.Y-loc.Y))
		if pr.LaunchTarget == t.ID {
			label += "  · aimed" // DRAFT
		}
		out = append(out, gui.PanelTarget{ID: t.ID, Label: label})
	}
	return out
}

func (s *MainState) locName(id string) string {
	if s.campaign != nil {
		if l := s.campaign.Locations[id]; l != nil {
			return l.Name
		}
	}
	return id
}

// panelAction runs an action row's action.
func (s *MainState) panelAction(action, target string) {
	pr := s.playerPresenceHere()
	if pr == nil || !s.ordersHere() {
		return
	}
	switch action {
	case "launch_at", "transmit_to":
		if target == "" {
			message.PostMessage("Orders", "Choose a world first.") // DRAFT
			return
		}
		pr.LaunchTarget = target
		message.PostMessage("Orders", "Aimed at "+s.locName(target)+".") // DRAFT
	case "load_launcher":
		// Loading is the launch rung's walk by hand: it takes the launch over.
		pr.SetManual(campaign.GroupLaunch, true)
		n := s.loadLauncher(s.panelStructure)
		message.PostMessage("Orders", fmt.Sprintf("%d sent to load it.", n)) // DRAFT
	}
	s.refreshStructurePanel()
}

// panelToggle takes a rung group over on the player's presence here, or
// gives it back.
func (s *MainState) panelToggle(group string, on bool) {
	pr := s.playerPresenceHere()
	if pr == nil || !s.ordersHere() {
		return
	}
	if group == worldToggle {
		for _, g := range s.doctrineSettlement().UI().Groups {
			pr.SetManual(g.ID, on)
		}
		s.refreshStructurePanel()
		return
	}
	pr.SetManual(group, on)
	s.refreshStructurePanel()
}

// directorSkip is the rung groups the player took over on p. The launch
// rung (the walk to the Spire or Core) runs only toward the player's aim
// anyway (campaign.LaunchReady); taken over, the player loads it by hand.
func directorSkip(p *campaign.FactionPresence) map[string]bool {
	if p == nil || len(p.TakenOver) == 0 {
		return nil
	}
	m := map[string]bool{}
	for _, g := range p.TakenOver {
		m[g] = true
	}
	return m
}

// worldToggle is the world_toggle row's id: every group at once.
const worldToggle = "*"
