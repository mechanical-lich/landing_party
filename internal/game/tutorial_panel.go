package game

import (
	"fmt"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/config"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// Panel geometry at the fixed 1152x800 layout. On a level it sits right of the
// sidebar, above the resource bar and left of the message log; on the
// dashboard it sits top right of the content area, over the star map canvas.
const (
	tutPanelSiteX = 220
	tutPanelSiteY = 620
	tutPanelSiteW = 508
	tutPanelDashW = 520
	tutPanelDashY = 120
	tutPanelH     = 140
	tutPanelBarH  = 26 // the title row alone, when collapsed
	// tutPanelCoveredY is where the title row goes while a storage inspector or
	// a HUD modal is open: below the lowest of them (inspector bottom 666,
	// colonist 680, research and crafting 650), above the resource bar (765)
	// and the dashboard's status lines.
	tutPanelCoveredY = 686
	// tutSkipArmFrames is how long "Really skip?" stays armed.
	tutSkipArmFrames = 180
)

var (
	tutPanelFill   = color.RGBA{10, 16, 30, 236}
	tutPanelStroke = color.RGBA{110, 170, 230, 255}
	tutTitleColor  = color.RGBA{200, 225, 255, 255}
	tutBodyColor   = color.RGBA{215, 225, 240, 255}
	tutNudgeColor  = color.RGBA{230, 190, 120, 255}
)

// tutorialPanel is the non-blocking prompt. It is hosted directly (not through a
// minui.GUI) so it lives across MainStates and on the dashboard, which has no
// GUI of its own; a minui.Modal would take exclusive input.
type tutorialPanel struct {
	b *tutorialBridge

	rect      minui.Rect // the full panel this frame
	collapsed bool       // [-] pressed: title row only
	covered   bool       // a storage inspector or HUD modal is open: title row only, moved

	collapseBtn *minui.Button
	manualBtn   *minui.Button
	skipBtn     *minui.Button
	closeBtn    *minui.Button
	skipArmed   int // frame the Skip confirmation expires, 0 when not armed

	// end panel state
	ending   bool
	endTitle string
	endText  string
	// offerToggle is "Offer the tutorial on new expeditions", shown on the end
	// panel. It writes the New Expedition toggle's default to config.local.json.
	offerToggle *minui.Toggle
}

func newTutorialPanel(b *tutorialBridge) *tutorialPanel {
	p := &tutorialPanel{b: b}
	p.collapseBtn = minui.NewButton("tut_collapse", "-")
	p.collapseBtn.SetSize(22, 18)
	// The default 8px padding plus the border leaves no room for the label in
	// a 22x18 button, so "-" / "+" was truncated to nothing.
	p.collapseBtn.GetStyle().Padding = minui.NewEdgeInsets(1)
	p.collapseBtn.OnClick = func() { p.collapsed = !p.collapsed }
	p.manualBtn = minui.NewButton("tut_manual", "Manual")
	p.manualBtn.SetSize(80, 24)
	p.manualBtn.OnClick = func() {
		if step := b.dir.Current(); step != nil {
			b.openManual(step.Topic)
		}
	}
	p.skipBtn = minui.NewButton("tut_skip", "Skip tutorial")
	p.skipBtn.SetSize(130, 24)
	p.skipBtn.OnClick = p.onSkip
	p.closeBtn = minui.NewButton("tut_close", "Close")
	p.closeBtn.SetSize(100, 24)
	p.closeBtn.OnClick = func() { b.close() }
	p.offerToggle = minui.NewToggle("tut_offer", "Offer the tutorial on new expeditions")
	p.offerToggle.On = !config.Global().TutorialOffDefault
	p.offerToggle.SetSize(330, 24)
	p.offerToggle.OnChange = func(on bool) {
		if err := config.SaveTutorialDefault(!on); err != nil {
			log.Printf("tutorial: save default: %v", err)
		}
	}
	return p
}

// onSkip takes two clicks: the first arms "Really skip?", the second skips.
func (p *tutorialPanel) onSkip() {
	if p.skipArmed > p.b.frame {
		p.skipArmed = 0
		p.skipBtn.Text = "Skip tutorial"
		p.b.skip()
		return
	}
	p.skipArmed = p.b.frame + tutSkipArmFrames
	p.skipBtn.Text = "Really skip?"
}

func (p *tutorialPanel) onStepChanged() {
	p.collapsed = false
}

// showEnd switches to the end panel: finished or skipped.
func (p *tutorialPanel) showEnd(skipped bool) {
	p.ending = true
	p.collapsed = false
	if skipped {
		p.endTitle = "Tutorial skipped"
		p.endText = "Travel is unlocked everywhere. The Field Manual in the Encyclopedia keeps every lesson for reference."
	} else {
		p.endTitle = "The galaxy is yours"
		p.endText = "The first landing is done and every destination is open. Quests and contracts carry on as they are. Remember that crew aboard the ship eat from its rations, and a site only feeds the people on it from its own storage. The Field Manual in the Encyclopedia keeps every lesson."
	}
}

// layout places the panel for this frame's host.
func (p *tutorialPanel) layout(ctx string) {
	cfg := config.Global()
	sw := cfg.ScreenWidth
	onDash := len(ctx) > 5 && ctx[:5] == "dash:"
	if onDash {
		p.rect = minui.Rect{X: sw - tutPanelDashW - 20, Y: tutPanelDashY, Width: tutPanelDashW, Height: tutPanelH}
	} else {
		p.rect = minui.Rect{X: tutPanelSiteX, Y: tutPanelSiteY, Width: tutPanelSiteW, Height: tutPanelH}
	}
	if p.covered {
		p.rect.Y = tutPanelCoveredY
		p.rect.Height = tutPanelBarH
	} else if p.collapsed {
		p.rect.Height = tutPanelBarH
	}
	r := p.rect
	p.collapseBtn.SetPosition(r.X+r.Width-26, r.Y+4)
	btnY := r.Y + r.Height - 30
	p.manualBtn.SetPosition(r.X+10, btnY)
	p.skipBtn.SetPosition(r.X+r.Width-140, btnY)
	p.closeBtn.SetPosition(r.X+r.Width-110, btnY)
	p.offerToggle.SetPosition(r.X+10, btnY)
}

// Update lays the panel out and runs its buttons. Buttons are skipped while an
// incident popup owns input, and while a storage inspector or a HUD modal
// (Esc menu, save, load, crafting, research, colonist) is open: the panel is
// then only a title row below the modal, so it covers none of its rows.
func (p *tutorialPanel) Update(ctx string, s *MainState, d *DashboardState) {
	p.covered = (s != nil && s.storageInspector != nil && s.storageInspector.Visible) ||
		(s != nil && s.guiManager != nil && s.guiManager.AnyModalOpen()) ||
		(d != nil && d.storageInspector != nil && d.storageInspector.Visible)
	p.layout(ctx)
	if p.skipArmed != 0 && p.b.frame > p.skipArmed {
		p.skipArmed = 0
		p.skipBtn.Text = "Skip tutorial"
	}
	if p.b.wm.ActiveIncident() != nil || p.covered {
		return
	}
	if p.ending {
		if !p.collapsed {
			p.offerToggle.Update()
			p.closeBtn.Update()
		}
		p.collapseBtn.Update()
		return
	}
	p.collapseBtn.Update()
	if p.collapsed {
		return
	}
	if step := p.b.dir.Current(); step != nil && step.Topic != "" {
		p.manualBtn.Update()
	}
	p.skipBtn.Update()
}

// Contains reports whether (x, y) is on the panel as laid out this frame.
func (p *tutorialPanel) Contains(x, y int) bool {
	r := p.rect
	return r.Width > 0 && x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// Draw renders the panel. Highlights are drawn by the bridge.
func (p *tutorialPanel) Draw(screen *ebiten.Image) {
	r := p.rect
	if r.Width == 0 {
		return
	}
	minui.DrawRect(screen, r, tutPanelFill)
	minui.DrawRectStroke(screen, r, 1, tutPanelStroke)

	title := p.titleText()
	mlge_text.Draw(screen, title, 14, r.X+10, r.Y+6, tutTitleColor)
	if !p.covered {
		p.collapseBtn.Text = "-"
		if p.collapsed {
			p.collapseBtn.Text = "+"
		}
		p.collapseBtn.Draw(screen)
	}
	if r.Height <= tutPanelBarH {
		return
	}

	bodyY := r.Y + 30
	bodyW := r.Width - 20
	if p.ending {
		drawWrapped(screen, p.endText, 12, r.X+10, bodyY, bodyW, 16, tutBodyColor)
		p.offerToggle.Draw(screen)
		p.closeBtn.Draw(screen)
		return
	}
	step := p.b.dir.Current()
	if step == nil {
		return
	}
	ctx := p.b.lastCtx
	if p.b.dir.InContext(ctx) && (step.At == "" || step.At == p.b.at()) {
		drawWrapped(screen, p.b.expand(step.Text), 12, r.X+10, bodyY, bodyW, 16, tutBodyColor)
	} else {
		drawWrapped(screen, p.b.nudgeText(ctx, p.b.at()), 13, r.X+10, bodyY+8, bodyW, 17, tutNudgeColor)
	}
	if step.Topic != "" {
		p.manualBtn.Draw(screen)
	}
	p.skipBtn.Draw(screen)
}

func (p *tutorialPanel) titleText() string {
	if p.ending {
		return "Tutorial  -  " + p.endTitle
	}
	step := p.b.dir.Current()
	if step == nil {
		return "Tutorial"
	}
	return fmt.Sprintf("Tutorial %d/%d  -  %s", p.b.dir.Index()+1, p.b.dir.Count(), p.b.expand(step.Title))
}
