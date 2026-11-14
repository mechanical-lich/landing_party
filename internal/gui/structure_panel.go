package gui

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/config"

	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// A structure's panel (phase F step 3; docs/developer/factions.md, "The
// interface block"): the doctrine declares a structure's title and rows, the
// game resolves each row's source into a PanelRowView, and this one modal
// draws any of them. Row kinds: text, readiness (status lines),
// target_list (a selectable list of sites), action (a button that runs on
// the selected target), group_toggles (the director's rung groups the
// player may take over).

// PanelView is a structure panel ready to draw.
type PanelView struct {
	Title string
	Rows  []PanelRowView
}

// PanelRowView is one row: Kind and what it shows.
type PanelRowView struct {
	Kind    string
	Label   string        // text, the action's button
	Lines   []string      // readiness
	Targets []PanelTarget // target_list
	Action  string        // action
	Toggles []PanelToggle // group_toggles
}

// PanelTarget is one selectable site.
type PanelTarget struct{ ID, Label string }

// PanelToggle is one rung group: On is "the player has taken it over".
type PanelToggle struct {
	ID, Label string
	On        bool
}

// StructurePanelActionEvent is an action row's button: Action on Target
// ("" when the panel has no list or nothing is selected).
type StructurePanelActionEvent struct{ Action, Target string }

// StructurePanelToggleEvent is a group toggle: the player takes Group over
// (On) or gives it back.
type StructurePanelToggleEvent struct {
	Group string
	On    bool
}

const (
	StructurePanelActionEventType event.EventType = "structure_panel_action"
	StructurePanelToggleEventType event.EventType = "structure_panel_toggle"
)

func (e StructurePanelActionEvent) GetType() event.EventType { return StructurePanelActionEventType }
func (e StructurePanelToggleEvent) GetType() event.EventType { return StructurePanelToggleEventType }

// OpenStructurePanel shows v (rebuilt in place when it is already up).
func (h *HUDScreen) OpenStructurePanel(v PanelView) {
	const w, ht = 460, 580
	if h.structurePanel == nil {
		sw, sh := config.Global().ScreenWidth, config.Global().ScreenHeight
		h.structurePanel = minui.NewModal("structurePanel", "", w, ht)
		h.structurePanel.SetPosition(sw/2-w/2, sh/2-ht/2)
		h.structurePanel.Closeable = true
		h.uiGUI.AddModal(h.structurePanel)
	}
	m := h.structurePanel
	m.Title = v.Title
	for _, c := range append([]minui.Element{}, m.GetChildren()...) {
		m.RemoveChild(c)
	}
	h.panelShown = h.panelShown[:0]
	y := 40
	var list *minui.ListBox
	var targets []PanelTarget
	for i, r := range v.Rows {
		h.panelShown = append(h.panelShown, r.Kind)
		switch r.Kind {
		case "text":
			l := minui.NewLabel(fmt.Sprintf("panel_text_%d", i), r.Label)
			l.SetPosition(14, y)
			m.AddChild(l)
			y += 22
		case "readiness":
			for j, line := range r.Lines {
				l := minui.NewLabel(fmt.Sprintf("panel_ready_%d_%d", i, j), line)
				l.SetPosition(14, y)
				m.AddChild(l)
				y += 20
			}
			y += 6
		case "target_list":
			targets = r.Targets
			items := make([]string, 0, len(targets))
			for _, t := range targets {
				items = append(items, t.Label)
			}
			if len(items) == 0 {
				items = []string{"(none in range)"} // DRAFT
			}
			list = minui.NewListBox(fmt.Sprintf("panelTargets_%d", i), items)
			list.SetBounds(minui.Rect{X: 14, Y: y, Width: w - 28, Height: 140})
			m.AddChild(list)
			y += 148
		case "action":
			act := r.Action
			b := minui.NewButton("panel_action_"+act, r.Label)
			b.SetBounds(minui.Rect{X: 14, Y: y, Width: 200, Height: 28})
			lst, tg := list, targets
			b.OnClick = func() {
				target := ""
				if lst != nil && lst.SelectedIndex >= 0 && lst.SelectedIndex < len(tg) {
					target = tg[lst.SelectedIndex].ID
				}
				event.GetQueuedInstance().QueueEvent(StructurePanelActionEvent{Action: act, Target: target})
			}
			m.AddChild(b)
			y += 36
		case "group_toggles", "world_toggle":
			for _, tg := range r.Toggles {
				group := tg.ID
				t := minui.NewToggle("panel_group_"+group, tg.Label)
				tw, th := w-28, 26
				t.GetStyle().Width = &tw
				t.GetStyle().Height = &th
				t.SetPosition(14, y)
				t.On = tg.On
				t.OnChange = func(on bool) {
					event.GetQueuedInstance().QueueEvent(StructurePanelToggleEvent{Group: group, On: on})
				}
				m.AddChild(t)
				y += 30
			}
		}
	}
	m.SetVisible(true)
}

// StructurePanelShown is the open panel's row kinds, in order (nil closed).
func (h *HUDScreen) StructurePanelShown() []string {
	if h.structurePanel == nil || !h.structurePanel.IsVisible() {
		return nil
	}
	return append([]string(nil), h.panelShown...)
}

// StructurePanelVisible reports whether a structure panel is up.
func (h *HUDScreen) StructurePanelVisible() bool {
	return h.structurePanel != nil && h.structurePanel.IsVisible()
}

func (gm *GUIManager) OpenStructurePanel(v PanelView) { gm.hud.OpenStructurePanel(v) }
func (gm *GUIManager) StructurePanelShown() []string  { return gm.hud.StructurePanelShown() }
func (gm *GUIManager) StructurePanelVisible() bool    { return gm.hud.StructurePanelVisible() }
func (gm *GUIManager) StructurePanelTitle() string {
	if gm.hud.structurePanel == nil {
		return ""
	}
	return gm.hud.structurePanel.Title
}
