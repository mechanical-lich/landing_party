package game

import (
	"image/color"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/lore"
	"github.com/mechanical-lich/scifi_settlements/internal/scenario"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

type titleScreen int

const (
	screenMain          titleScreen = iota
	screenNewSettlement
	screenLoad
)

type TitleState struct {
	screen titleScreen

	newBtn  *minui.Button
	loadBtn *minui.Button
	quitBtn *minui.Button

	nameInput      *minui.TextInput
	randomNameBtn  *minui.Button
	scenarioPicker *minui.SelectBox
	scenarioIDs    []string

	lightingPicker    *minui.SelectBox
	lightingModes     []string // internal mode strings
	ambientLabel      *minui.Label
	ambientInput      *minui.TextInput

	generateBtn *minui.Button
	cancelBtn   *minui.Button

	// Load screen
	saveMetas    []SaveMeta
	saveListBox  *minui.ListBox
	loadConfirm  *minui.Button
	loadCancel   *minui.Button

	errMsg string
	done   bool
	next   state.StateInterface
}

var _ state.StateInterface = (*TitleState)(nil)

func NewTitleState() *TitleState {
	ts := &TitleState{screen: screenMain}
	_ = scenario.Load("data/scenarios")
	ts.buildMainMenu()
	ts.buildNewSettlementScreen()
	ts.buildLoadScreen()
	return ts
}

func (ts *TitleState) buildMainMenu() {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	btnW, btnH := 220, 36
	x := cx - btnW/2

	ts.newBtn = minui.NewButton("title_new", "New Settlement")
	ts.newBtn.SetPosition(x, 320)
	ts.newBtn.SetSize(btnW, btnH)
	ts.newBtn.OnClick = func() {
		ts.errMsg = ""
		ts.buildScenarioPicker()
		ts.screen = screenNewSettlement
	}

	ts.loadBtn = minui.NewButton("title_load", "Load Save")
	ts.loadBtn.SetPosition(x, 320+btnH+12)
	ts.loadBtn.SetSize(btnW, btnH)
	ts.loadBtn.OnClick = func() {
		ts.errMsg = ""
		ts.openLoadScreen()
	}

	ts.quitBtn = minui.NewButton("title_quit", "Quit")
	ts.quitBtn.SetPosition(x, 320+(btnH+12)*2)
	ts.quitBtn.SetSize(btnW, btnH)
	ts.quitBtn.OnClick = func() { os.Exit(0) }
}

func (ts *TitleState) buildLoadScreen() {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2

	ts.saveListBox = minui.NewListBox("title_save_list", nil)
	ts.saveListBox.SetBounds(minui.Rect{X: cx - 200, Y: 300, Width: 400, Height: 220})

	ts.loadConfirm = minui.NewButton("title_load_confirm", "Load")
	ts.loadConfirm.SetPosition(cx-116, 534)
	ts.loadConfirm.SetSize(160, 36)
	ts.loadConfirm.OnClick = func() { ts.confirmLoad() }

	ts.loadCancel = minui.NewButton("title_load_cancel", "Cancel")
	ts.loadCancel.SetPosition(cx+52, 534)
	ts.loadCancel.SetSize(160, 36)
	ts.loadCancel.OnClick = func() { ts.screen = screenMain }
}

func (ts *TitleState) openLoadScreen() {
	metas, err := ListSaves()
	if err != nil || len(metas) == 0 {
		ts.errMsg = "No saves found."
		return
	}
	ts.saveMetas = metas
	labels := make([]string, len(metas))
	for i, m := range metas {
		labels[i] = m.Name + "  (" + m.SavedAt.Format("2006-01-02 15:04") + ")"
	}
	ts.saveListBox.SetItems(labels)
	ts.saveListBox.SelectedIndex = 0
	ts.screen = screenLoad
}

func (ts *TitleState) confirmLoad() {
	idx := ts.saveListBox.SelectedIndex
	if idx < 0 || idx >= len(ts.saveMetas) {
		return
	}
	ms, err := LoadSave(ts.saveMetas[idx].Name)
	if err != nil {
		ts.errMsg = "Load failed: " + err.Error()
		ts.screen = screenMain
		return
	}
	ts.next = ms
	ts.done = true
}

func (ts *TitleState) buildNewSettlementScreen() {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2

	ts.nameInput = minui.NewTextInput("settlement_name", "")
	ts.nameInput.SetPosition(cx-150, 300)
	ts.nameInput.SetSize(220, 28)

	ts.randomNameBtn = minui.NewButton("random_name", "Random")
	ts.randomNameBtn.SetPosition(cx+80, 300)
	ts.randomNameBtn.SetSize(80, 28)
	ts.randomNameBtn.OnClick = func() { ts.nameInput.Text = lore.RandomSettlementName() }

	ts.buildScenarioPicker()

	// Lighting override
	ts.lightingModes = []string{"", world.LightModedayNight, world.LightModeFixed, world.LightModePitchDark}
	ts.lightingPicker = minui.NewSelectBox("lighting_picker", []string{"Scenario Default", "Day / Night Cycle", "Fixed Ambient", "Pitch Dark"})
	ts.lightingPicker.SetPosition(cx-150, 410)
	ts.lightingPicker.SetSize(300, 28)
	ts.lightingPicker.SelectByIndex(0)
	ts.lightingPicker.OnSelect = func(idx int, _ string) {
		isFixed := idx == 2
		ts.ambientLabel.SetVisible(isFixed)
		ts.ambientInput.SetVisible(isFixed)
	}

	ts.ambientLabel = minui.NewLabel("ambient_label", "Ambient Level (0-100):")
	ts.ambientLabel.SetPosition(cx-150, 448)
	ts.ambientLabel.SetSize(220, 18)
	ts.ambientLabel.SetVisible(false)

	ts.ambientInput = minui.NewTextInput("ambient_input", "50")
	ts.ambientInput.SetPosition(cx-150, 468)
	ts.ambientInput.SetSize(100, 28)
	ts.ambientInput.SetVisible(false)

	ts.generateBtn = minui.NewButton("generate", "Generate")
	ts.generateBtn.SetPosition(cx-80, 510)
	ts.generateBtn.SetSize(160, 36)
	ts.generateBtn.OnClick = func() { ts.startNewSettlement() }

	ts.cancelBtn = minui.NewButton("cancel", "Cancel")
	ts.cancelBtn.SetPosition(cx-80, 556)
	ts.cancelBtn.SetSize(160, 36)
	ts.cancelBtn.OnClick = func() { ts.screen = screenMain }
}

func (ts *TitleState) buildScenarioPicker() {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	_ = scenario.Load("data/scenarios")
	scenarios := scenario.AllEnabled()
	labels := []string{"Random"}
	ts.scenarioIDs = []string{""}
	for _, s := range scenarios {
		labels = append(labels, s.Name)
		ts.scenarioIDs = append(ts.scenarioIDs, s.ID)
	}
	ts.scenarioPicker = minui.NewSelectBox("scenario_picker", labels)
	ts.scenarioPicker.SetPosition(cx-150, 355)
	ts.scenarioPicker.SetSize(300, 28)
	ts.scenarioPicker.SelectByIndex(0)
}

func (ts *TitleState) startNewSettlement() {
	name := ts.nameInput.Text
	if name == "" {
		name = lore.RandomSettlementName()
	}
	var scenarioID string
	idx := ts.scenarioPicker.SelectedIndex
	if idx > 0 && idx < len(ts.scenarioIDs) {
		scenarioID = ts.scenarioIDs[idx]
	}
	lightIdx := ts.lightingPicker.SelectedIndex
	lightMode := ""
	if lightIdx >= 0 && lightIdx < len(ts.lightingModes) {
		lightMode = ts.lightingModes[lightIdx]
	}
	ambient := 0
	if lightMode == world.LightModeFixed {
		ambient, _ = strconv.Atoi(ts.ambientInput.Text)
		if ambient < 0 {
			ambient = 0
		} else if ambient > 100 {
			ambient = 100
		}
	}
	cfg := SettlementConfig{Name: name, ScenarioID: scenarioID, LightingMode: lightMode, LightingAmbient: ambient}
	ms, err := NewMainState(cfg)
	if err != nil {
		ts.errMsg = "Generate failed: " + err.Error()
		return
	}
	ts.next = ms
	ts.done = true
}

func (ts *TitleState) Update() state.StateInterface {
	switch ts.screen {
	case screenMain:
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			os.Exit(0)
		}
		ts.newBtn.Update()
		ts.loadBtn.Update()
		ts.quitBtn.Update()
	case screenNewSettlement:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			ts.screen = screenMain
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			ts.startNewSettlement()
			return ts.next
		}
		ts.nameInput.Update()
		ts.randomNameBtn.Update()
		ts.scenarioPicker.Update()
		ts.lightingPicker.Update()
		if ts.ambientInput.IsVisible() {
			ts.ambientInput.Update()
		}
		ts.generateBtn.Update()
		ts.cancelBtn.Update()
	case screenLoad:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			ts.screen = screenMain
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			ts.confirmLoad()
		}
		ts.saveListBox.Update()
		ts.loadConfirm.Update()
		ts.loadCancel.Update()
	}
	return ts.next
}

func (ts *TitleState) Draw(screen *ebiten.Image) {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cfg := config.Global()
	screen.Fill(color.RGBA{8, 10, 18, 255})

	title := "Scifi Settlements"
	mlge_text.Draw(screen, title, 48, cfg.ScreenWidth/2-len(title)*48*3/10/2, 160, color.RGBA{100, 200, 255, 255})
	sub := "A Colony Sim"
	mlge_text.Draw(screen, sub, 18, cfg.ScreenWidth/2-len(sub)*18*3/10/2, 230, color.RGBA{70, 140, 180, 255})

	switch ts.screen {
	case screenMain:
		ts.newBtn.Draw(screen)
		ts.loadBtn.Draw(screen)
		ts.quitBtn.Draw(screen)
	case screenLoad:
		mlge_text.Draw(screen, "Load Save", 24, cfg.ScreenWidth/2-80, 260, color.RGBA{180, 210, 255, 255})
		ts.saveListBox.Draw(screen)
		ts.loadConfirm.Draw(screen)
		ts.loadCancel.Draw(screen)
	case screenNewSettlement:
		mlge_text.Draw(screen, "Colony Name:", 14, cfg.ScreenWidth/2-150, 283, color.RGBA{180, 210, 255, 255})
		ts.nameInput.Draw(screen)
		ts.randomNameBtn.Draw(screen)
		mlge_text.Draw(screen, "Scenario:", 14, cfg.ScreenWidth/2-150, 338, color.RGBA{180, 210, 255, 255})
		ts.scenarioPicker.Draw(screen)
		mlge_text.Draw(screen, "Lighting:", 14, cfg.ScreenWidth/2-150, 393, color.RGBA{180, 210, 255, 255})
		ts.lightingPicker.Draw(screen)
		if ts.ambientInput.IsVisible() {
			ts.ambientLabel.Draw(screen)
			ts.ambientInput.Draw(screen)
		}
		ts.generateBtn.Draw(screen)
		ts.cancelBtn.Draw(screen)
	}

	if ts.errMsg != "" {
		mlge_text.Draw(screen, ts.errMsg, 13, cfg.ScreenWidth/2-200, cfg.ScreenHeight-60, color.RGBA{220, 80, 80, 255})
	}
}

func (ts *TitleState) Done() bool { return ts.done }
