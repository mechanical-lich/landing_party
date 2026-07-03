package game

import (
	"fmt"
	"image/color"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/buildinfo"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

type titleScreen int

const (
	screenMain titleScreen = iota
	screenNewSettlement
	screenLoad
)

type TitleState struct {
	screen titleScreen

	newBtn  *minui.Button
	loadBtn *minui.Button
	quitBtn *minui.Button

	nameInput     *minui.TextInput
	randomNameBtn *minui.Button

	seedInput     *minui.TextInput
	randomSeedBtn *minui.Button


	mapPicker *minui.SelectBox
	mapIDs    []string

	scenarioPicker *minui.SelectBox
	scenarioIDs    []string

	lightingPicker *minui.SelectBox
	lightingModes  []string // internal mode strings
	ambientLabel   *minui.Label
	ambientInput   *minui.TextInput

	generateBtn *minui.Button
	cancelBtn   *minui.Button

	// Load screen
	saveMetas     []SaveMeta
	campaignNames []string
	saveListBox   *minui.ListBox
	loadConfirm   *minui.Button
	loadCancel    *minui.Button

	errMsg string
	done   bool
	next   state.StateInterface
}

func NewTitleState() *TitleState {
	ts := &TitleState{screen: screenMain}
	generation.SetStructureRunner(func(level *world.Level, name string, x, y, w, h int) error {
		return RunStructureScript(level, name, x, y, w, h)
	})
	_ = scenario.Load("data/scenarios")
	_ = mapdef.Load("data/maps")
	_ = generation.LoadBiomes("data/biomes")
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

	ts.newBtn = minui.NewButton("title_new", "New Expedition")
	ts.newBtn.SetPosition(x, 320)
	ts.newBtn.SetSize(btnW, btnH)
	ts.newBtn.OnClick = func() {
		ts.errMsg = ""
		ts.reloadMapsAndScenarios()
		ts.screen = screenNewSettlement
	}

	ts.loadBtn = minui.NewButton("title_load", "Load Expedition")
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
	names := ListCampaigns()
	if len(names) == 0 {
		ts.errMsg = "No expeditions found."
		return
	}
	ts.campaignNames = names
	ts.saveListBox.SetItems(names)
	ts.saveListBox.SelectedIndex = 0
	ts.screen = screenLoad
}

func (ts *TitleState) confirmLoad() {
	idx := ts.saveListBox.SelectedIndex
	if idx < 0 || idx >= len(ts.campaignNames) {
		return
	}
	c, err := LoadCampaign(ts.campaignNames[idx])
	if err != nil {
		ts.errMsg = "Load failed: " + err.Error()
		ts.screen = screenMain
		return
	}
	c.BindQuests()
	wm := NewWorldManager(c)
	ts.next = NewDashboardState(c, wm)
	ts.done = true
}

func (ts *TitleState) buildNewSettlementScreen() {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2

	ts.nameInput = minui.NewTextInput("settlement_name", "")
	ts.nameInput.SetPosition(cx-150, 270)
	ts.nameInput.SetSize(220, 28)
	ts.randomNameBtn = minui.NewButton("random_name", "Random")
	ts.randomNameBtn.SetPosition(cx+80, 270)
	ts.randomNameBtn.SetSize(80, 28)
	ts.randomNameBtn.OnClick = func() { ts.nameInput.Text = lore.RandomSettlementName() }

	ts.seedInput = minui.NewTextInput("seed", "")
	ts.seedInput.SetPosition(cx-150, 312)
	ts.seedInput.SetSize(220, 28)
	ts.randomSeedBtn = minui.NewButton("random_seed", "Random")
	ts.randomSeedBtn.SetPosition(cx+80, 312)
	ts.randomSeedBtn.SetSize(80, 28)
	ts.randomSeedBtn.OnClick = func() {
		ts.seedInput.Text = strconv.FormatInt(rand.Int63(), 10)
	}

	ts.mapPicker = minui.NewSelectBox("map_picker", []string{"Random"})
	ts.mapPicker.SetPosition(cx-150, 400)
	ts.mapPicker.SetSize(300, 28)
	ts.mapPicker.OnSelect = func(idx int, _ string) {
		mapID := ""
		if idx > 0 && idx < len(ts.mapIDs) {
			mapID = ts.mapIDs[idx]
		}
		ts.refreshScenarioPicker(mapID)
	}

	ts.scenarioPicker = minui.NewSelectBox("scenario_picker", []string{"Random"})
	ts.scenarioPicker.SetPosition(cx-150, 444)
	ts.scenarioPicker.SetSize(300, 28)
	ts.scenarioPicker.SelectByIndex(0)

	// Lighting override
	ts.lightingModes = []string{"", world.LightModedayNight, world.LightModeFixed, world.LightModePitchDark}
	ts.lightingPicker = minui.NewSelectBox("lighting_picker", []string{"Scenario Default", "Day / Night Cycle", "Fixed Ambient", "Pitch Dark"})
	ts.lightingPicker.SetPosition(cx-150, 488)
	ts.lightingPicker.SetSize(300, 28)
	ts.lightingPicker.SelectByIndex(0)
	ts.lightingPicker.OnSelect = func(idx int, _ string) {
		isFixed := idx == 2
		ts.ambientLabel.SetVisible(isFixed)
		ts.ambientInput.SetVisible(isFixed)
	}

	ts.ambientLabel = minui.NewLabel("ambient_label", "Ambient (0-100):")
	ts.ambientLabel.SetPosition(cx-150, 530)
	ts.ambientLabel.SetSize(160, 18)
	ts.ambientLabel.SetVisible(false)
	ts.ambientInput = minui.NewTextInput("ambient_input", "50")
	ts.ambientInput.SetPosition(cx+10, 526)
	ts.ambientInput.SetSize(80, 28)
	ts.ambientInput.SetVisible(false)

	ts.generateBtn = minui.NewButton("generate", "Launch Expedition")
	ts.generateBtn.SetPosition(cx-80, 372)
	ts.generateBtn.SetSize(160, 36)
	ts.generateBtn.OnClick = func() { ts.startNewSettlement() }
	ts.cancelBtn = minui.NewButton("cancel", "Cancel")
	ts.cancelBtn.SetPosition(cx-80, 416)
	ts.cancelBtn.SetSize(160, 36)
	ts.cancelBtn.OnClick = func() { ts.screen = screenMain }

	ts.reloadMapsAndScenarios()
}

// reloadMapsAndScenarios re-reads the map/scenario data files and repopulates
// the map picker, then syncs the scenario picker to the current map.
func (ts *TitleState) reloadMapsAndScenarios() {
	_ = scenario.Load("data/scenarios")
	_ = mapdef.Load("data/maps")
	_ = generation.LoadBiomes("data/biomes")

	labels := []string{"Random"}
	ts.mapIDs = []string{""}
	for _, m := range mapdef.All() {
		labels = append(labels, m.Name)
		ts.mapIDs = append(ts.mapIDs, m.ID)
	}
	ts.mapPicker.SetItems(labels)
	ts.mapPicker.SelectByIndex(0)
	ts.refreshScenarioPicker("")
}

// refreshScenarioPicker rebuilds the scenario list to only those compatible
// with mapID (empty mapID = all scenarios).
func (ts *TitleState) refreshScenarioPicker(mapID string) {
	var scenarios []scenario.Scenario
	if mapID == "" {
		scenarios = scenario.AllEnabled()
	} else {
		scenarios = scenario.ForMap(mapID)
	}
	labels := []string{"Random"}
	ts.scenarioIDs = []string{""}
	for _, s := range scenarios {
		labels = append(labels, s.Name)
		ts.scenarioIDs = append(ts.scenarioIDs, s.ID)
	}
	ts.scenarioPicker.SetItems(labels)
	ts.scenarioPicker.SelectByIndex(0)
}

func (ts *TitleState) startNewSettlement() {
	name := ts.nameInput.Text
	if name == "" {
		name = lore.RandomSettlementName()
	}

	var seed int64
	if s := strings.TrimSpace(ts.seedInput.Text); s != "" {
		seed, _ = strconv.ParseInt(s, 10, 64)
	}
	if seed == 0 {
		seed = rand.Int63()
	}

	var mapID string
	if mi := ts.mapPicker.SelectedIndex; mi > 0 && mi < len(ts.mapIDs) {
		mapID = ts.mapIDs[mi]
	}
	var scenarioID string
	if idx := ts.scenarioPicker.SelectedIndex; idx > 0 && idx < len(ts.scenarioIDs) {
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

	_ = mapID
	_ = scenarioID
	_ = lightMode
	_ = ambient

	ms, err := StartNewExpedition(name, seed)
	if err != nil {
		ts.errMsg = "Generate failed: " + err.Error()
		return
	}
	ts.next = ms
	ts.done = true
}

// StartNewExpedition generates a fully procedural campaign (start system,
// nearby systems, a far-off visible Home) and opens the Star Map — the
// colonists begin aboard the ship in space.
func StartNewExpedition(name string, seed int64) (state.StateInterface, error) {
	c, err := campaign.GenerateCampaign(name, seed)
	if err != nil {
		return nil, fmt.Errorf("generate campaign: %w", err)
	}
	if c.CurrentLocationID == "" {
		return nil, fmt.Errorf("campaign has no starting location")
	}
	cfg := campaign.GenerationConfig()
	SeedNewCampaign(c)
	c.BindQuests()
	wm := NewWorldManager(c)
	wm.SeedShipCrew(cfg.StartColonists) // crew spawns aboard the ship
	wm.StockShipLocker(cfg.StartFuel)   // and the hold is stocked
	return NewDashboardState(c, wm), nil
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
		ts.seedInput.Update()
		ts.randomSeedBtn.Update()
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

	title := "Landing Party"
	mlge_text.Draw(screen, title, 48, cfg.ScreenWidth/2-50-len(title)*48*3/10/2, 160, color.RGBA{100, 200, 255, 255})
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
		lx := cfg.ScreenWidth/2 - 150
		lblCol := color.RGBA{180, 210, 255, 255}
		mlge_text.Draw(screen, "Expedition Name:", 14, lx, 253, lblCol)
		ts.nameInput.Draw(screen)
		ts.randomNameBtn.Draw(screen)
		mlge_text.Draw(screen, "Seed (blank = random):", 14, lx, 295, lblCol)
		ts.seedInput.Draw(screen)
		ts.randomSeedBtn.Draw(screen)
		ts.generateBtn.Draw(screen)
		ts.cancelBtn.Draw(screen)
	}

	if ts.errMsg != "" {
		mlge_text.Draw(screen, ts.errMsg, 13, cfg.ScreenWidth/2-200, cfg.ScreenHeight-60, color.RGBA{220, 80, 80, 255})
	}

	ver := buildinfo.Version
	mlge_text.Draw(screen, ver, 11, cfg.ScreenWidth-len(ver)*7-8, cfg.ScreenHeight-20, color.RGBA{80, 100, 120, 200})

	// Flush any overlays queued by widgets (e.g. open SelectBox dropdowns).
	minui.FlushOverlays(screen)
}

func (ts *TitleState) Done() bool { return ts.done }
