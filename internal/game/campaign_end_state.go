package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// CampaignEndState is the terminal screen: victory (reached Home) or defeat
// (total wipe). It returns to the title screen.
type CampaignEndState struct {
	victory bool
	detail  string
	backBtn *minui.Button
	done    bool
	next    state.StateInterface
}

var _ state.StateInterface = (*CampaignEndState)(nil)

func NewCampaignEndState(victory bool, detail string) *CampaignEndState {
	e := &CampaignEndState{victory: victory, detail: detail}
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	e.backBtn = minui.NewButton("end_back", "Return to Title")
	e.backBtn.SetPosition(cx-110, 460)
	e.backBtn.SetSize(220, 40)
	e.backBtn.OnClick = func() {
		e.next = NewTitleState()
		e.done = true
	}
	return e
}

func (e *CampaignEndState) Update() state.StateInterface {
	e.backBtn.Update()
	return e.next
}

func (e *CampaignEndState) Draw(screen *ebiten.Image) {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2

	title, tcol := "Expedition Lost", color.RGBA{220, 90, 80, 255}
	if e.victory {
		title, tcol = "You Made It Home", color.RGBA{120, 220, 150, 255}
	}
	screen.Fill(color.RGBA{6, 8, 16, 255})
	mlge_text.Draw(screen, title, 44, cx-len(title)*44*3/10/2, 220, tcol)
	if e.detail != "" {
		mlge_text.Draw(screen, e.detail, 16, cx-len(e.detail)*16*3/10/2, 300, color.RGBA{190, 205, 225, 255})
	}
	e.backBtn.Draw(screen)
	minui.FlushOverlays(screen)
}

func (e *CampaignEndState) Done() bool { return e.done }
