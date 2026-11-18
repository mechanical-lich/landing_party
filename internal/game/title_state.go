package game

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/audio"
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

	newBtn      *minui.Button
	loadBtn     *minui.Button
	settingsBtn *minui.Button
	quitBtn     *minui.Button

	openSettings bool

	nameInput     *minui.TextInput
	randomNameBtn *minui.Button

	seedInput     *minui.TextInput
	randomSeedBtn *minui.Button

	// tutorialToggle turns the guided first landing on or off for this
	// expedition. Its default comes from config (TutorialOffDefault).
	tutorialToggle *minui.Toggle

	// factions is the "Rival factions" stepper's value (campaign.GenOptions);
	// default and maximum come from generation.json.
	factions      int
	factionsMax   int
	factionsMinus *minui.Button
	factionsPlus  *minui.Button

	// sizes is the Galaxy size row's choices (generation.json galaxy.sizes)
	// and size the one chosen: GenOptions.Size.
	sizes              []campaign.GalaxySize
	size               int
	sizePrev, sizeNext *minui.Button

	// races is the playable doctrines (the colony first) and race the one
	// chosen (phase F): GenOptions.Doctrine.
	races    []string
	race     int
	racePrev *minui.Button
	raceNext *minui.Button

	// The rival boxes (title_rivals.go): one row per rival, the player's
	// group.
	rivalChoices              []string
	rivals                    []*rivalRow
	playerGroup               int
	playerGMinus, playerGPlus *minui.Button

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
	generation.SetStructureRunner(func(level *world.Level, name string, x, y, w, h int, params map[string]any, seed int64) error {
		return RunStructureScriptWithParams(level, name, x, y, w, h, params, seed)
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
		ts.screen = screenNewSettlement
	}

	ts.loadBtn = minui.NewButton("title_load", "Load Expedition")
	ts.loadBtn.SetPosition(x, 320+btnH+12)
	ts.loadBtn.SetSize(btnW, btnH)
	ts.loadBtn.OnClick = func() {
		ts.errMsg = ""
		ts.openLoadScreen()
	}

	ts.settingsBtn = minui.NewButton("title_settings", "Settings")
	ts.settingsBtn.SetPosition(x, 320+(btnH+12)*2)
	ts.settingsBtn.SetSize(btnW, btnH)
	ts.settingsBtn.OnClick = func() {
		ts.errMsg = ""
		ts.openSettings = true
	}

	ts.quitBtn = minui.NewButton("title_quit", "Quit")
	ts.quitBtn.SetPosition(x, 320+(btnH+12)*3)
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
	if err := wm.EnterStartingOrbit(); err != nil {
		log.Printf("EnterStartingOrbit: %v", err)
	}
	startTutorial(c, wm)
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

	ts.tutorialToggle = minui.NewToggle("tutorial_toggle", "Guided first landing (tutorial)")
	// Observer mode starts an observer run: no ship, nobody to teach, so
	// the toggle starts off (and StartNewExpedition keeps a run tutorial-free).
	ts.tutorialToggle.On = !cfg.TutorialOffDefault && !cfg.ObserverMode
	ts.tutorialToggle.SetPosition(cx-150, 354)

	// Rival factions: how many enemy presences are seeded (0 = none expand).
	ts.factions, ts.factionsMax = campaign.FactionCountRange()
	ts.factionsMinus = minui.NewButton("factions_minus", "-")
	ts.factionsMinus.SetPosition(cx+80, 392)
	ts.factionsMinus.SetSize(28, 28)
	ts.factionsMinus.OnClick = func() {
		if ts.factions > 0 {
			ts.factions--
		}
	}
	ts.factionsPlus = minui.NewButton("factions_plus", "+")
	ts.factionsPlus.SetPosition(cx+132, 392)
	ts.factionsPlus.SetSize(28, 28)
	ts.factionsPlus.OnClick = func() {
		if ts.factions < ts.factionsMax {
			ts.factions++
		}
	}

	// Galaxy size: a deliberate choice like Race; what is in it is rolled.
	var def string
	ts.sizes, def = campaign.GalaxySizes()
	for i, s := range ts.sizes {
		if s.ID == def {
			ts.size = i
		}
	}
	ts.sizePrev = minui.NewButton("size_prev", "<")
	ts.sizePrev.SetPosition(cx-20, 432)
	ts.sizePrev.SetSize(28, 28)
	ts.sizePrev.OnClick = func() { ts.stepSize(-1) }
	ts.sizeNext = minui.NewButton("size_next", ">")
	ts.sizeNext.SetPosition(cx+132, 432)
	ts.sizeNext.SetSize(28, 28)
	ts.sizeNext.OnClick = func() { ts.stepSize(1) }

	// Race: the playable doctrines, the colony first (phase F). An observer
	// run is always the colony's expedition.
	ts.races, ts.race = campaign.PlayableDoctrines(), 0
	ts.racePrev = minui.NewButton("race_prev", "<")
	ts.racePrev.SetPosition(cx-20, 472)
	ts.racePrev.SetSize(28, 28)
	ts.racePrev.OnClick = func() { ts.stepRace(-1) }
	ts.raceNext = minui.NewButton("race_next", ">")
	ts.raceNext.SetPosition(cx+132, 472)
	ts.raceNext.SetSize(28, 28)
	ts.raceNext.OnClick = func() { ts.stepRace(1) }

	ts.buildRivalRows()

	ts.generateBtn = minui.NewButton("generate", "Launch Expedition")
	ts.generateBtn.SetPosition(cx-80, 524)
	ts.generateBtn.SetSize(160, 36)
	ts.generateBtn.OnClick = func() { ts.startNewSettlement() }
	ts.cancelBtn = minui.NewButton("cancel", "Cancel")
	ts.cancelBtn.SetPosition(cx-80, 568)
	ts.cancelBtn.SetSize(160, 36)
	ts.cancelBtn.OnClick = func() { ts.screen = screenMain }
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

	ms, err := StartNewExpeditionSized(name, seed, ts.tutorialToggle.IsOn() && !ts.raceIsNotColony(), ts.factions, ts.raceKey(), ts.rivalPicks(), ts.playerGroup, ts.sizeID())
	if err != nil {
		ts.errMsg = "Generate failed: " + err.Error()
		return
	}
	ts.next = ms
	ts.done = true
}

// StartNewExpedition generates a galaxy and an expedition arrived at one of
// its landfalls (campaign.GenerateCampaignOpts) and opens the Star Map — the
// colonists begin aboard the ship in space at the rim. With tutorial set, the
// Training Ground is added beside the landfall and the hold carries only the
// tutorial's metal. factions is
// the number of rival presences seeded.
func StartNewExpedition(name string, seed int64, tutorial bool, factions int) (state.StateInterface, error) {
	return StartNewExpeditionAs(name, seed, tutorial, factions, "")
}

// StartNewExpeditionAs is StartNewExpedition as a chosen race (phase F;
// "" is the colony). A spreading race (the hive) has no ship and no
// tutorial: it opens viewing its first world.
func StartNewExpeditionAs(name string, seed int64, tutorial bool, factions int, race string) (state.StateInterface, error) {
	return StartNewExpeditionWith(name, seed, tutorial, factions, race, nil, 0)
}

// StartNewExpeditionWith is StartNewExpeditionAs with the rival boxes
// (rival_selection_plan.md step 3: nil is the Rival factions count alone)
// and the player's group.
func StartNewExpeditionWith(name string, seed int64, tutorial bool, factions int, race string, rivals []campaign.RivalPick, playerGroup int) (state.StateInterface, error) {
	return StartNewExpeditionSized(name, seed, tutorial, factions, race, rivals, playerGroup, "")
}

// StartNewExpeditionSized is StartNewExpeditionWith in a galaxy of size ("":
// the default; the New Expedition Galaxy size row).
func StartNewExpeditionSized(name string, seed int64, tutorial bool, factions int, race string, rivals []campaign.RivalPick, playerGroup int, size string) (state.StateInterface, error) {
	// Observer mode at New Expedition makes the whole run an observer run:
	// no player ship, the expedition a fleet (campaign/observer.go). An
	// observer run is always the colony's.
	observer := config.Global().ObserverMode
	if observer {
		tutorial = false // nobody to teach
		race = ""
	}
	c, wm, err := newExpeditionOpts(name, seed, tutorial, campaign.GenOptions{Factions: factions, Observer: observer, Doctrine: race,
		Rivals: rivals, PlayerGroup: playerGroup, Size: size})
	if err != nil {
		return nil, err
	}
	if c.SpreadRun() {
		if ms, err := wm.ViewLocation(c.StartID); err == nil {
			return ms, nil
		} else {
			log.Printf("race start view: %v", err)
		}
	}
	return NewDashboardState(c, wm), nil
}

// newExpedition is StartNewExpedition without the dashboard and with no rival
// factions, for headless tests.
func newExpedition(name string, seed int64, tutorial bool) (*campaign.Campaign, *WorldManager, error) {
	return newExpeditionOpts(name, seed, tutorial, campaign.GenOptions{})
}

func newExpeditionOpts(name string, seed int64, tutorial bool, opts campaign.GenOptions) (*campaign.Campaign, *WorldManager, error) {
	c, err := campaign.GenerateCampaignOpts(name, seed, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("generate campaign: %w", err)
	}
	if c.CurrentLocationID == "" {
		return nil, nil, fmt.Errorf("campaign has no starting location")
	}
	cfg := campaign.GenerationConfig()
	SeedNewCampaign(c)
	if c.SpreadRun() {
		// A spreading race (phase F): no ship crew, no hold, no tutorial,
		// no quests (they pay fuel into a hold the run has not got). The
		// player's first presence goes on the start system.
		if _, err := c.AddPlayerBrood(); err != nil {
			return nil, nil, err
		}
		return c, NewWorldManager(c), nil
	}
	var hold map[string]int
	if tutorial {
		campaign.AddTutorialLocation(c)
		hold = tutorialHoldStock()
	}
	c.BindQuests()
	if c.Observer {
		// A spectator knows the whole galaxy.
		for _, l := range c.Locations {
			l.Identified = true
		}
		// An observer run: the crew, hold and fuel are the expedition
		// fleet's, flown by the expedition table; the ship stays empty.
		if hold == nil {
			hold = defaultHoldStock()
		}
		if _, err := c.AddExpeditionFleet(cfg.StartColonists, hold, cfg.StartFuel); err != nil {
			return nil, nil, err
		}
		wm := NewWorldManager(c)
		return c, wm, nil
	}
	if st := campaign.PlayerDoctrine(c).ShipStart(); st != nil && st.Stockpile != nil && hold == nil {
		hold = st.Stockpile // a ship race's own hold (the greys)
	}
	wm := NewWorldManager(c)
	wm.SeedShipCrew(cfg.StartColonists)   // crew spawns aboard the ship
	wm.StockShipLocker(cfg.StartFuel, hold) // and the hold is stocked
	if err := wm.EnterStartingOrbit(); err != nil {
		log.Printf("EnterStartingOrbit: %v", err)
	}
	startTutorial(c, wm) // assigns nil with the toggle off
	return c, wm, nil
}

func (ts *TitleState) Update() state.StateInterface {
	audio.SetMusicState("menu")
	switch ts.screen {
	case screenMain:
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			os.Exit(0)
		}
		ts.newBtn.Update()
		ts.loadBtn.Update()
		ts.settingsBtn.Update()
		ts.quitBtn.Update()
		if ts.openSettings {
			ts.openSettings = false
			return NewSettingsState()
		}
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
		if !ts.raceIsNotColony() {
			ts.tutorialToggle.Update() // a spreading race has no tutorial
		}
		ts.factionsMinus.Update()
		ts.factionsPlus.Update()
		ts.sizePrev.Update()
		ts.sizeNext.Update()
		ts.updateRivalRows()
		if ts.raceChoice() {
			ts.racePrev.Update()
			ts.raceNext.Update()
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

	title := "Landing Party"
	mlge_text.Draw(screen, title, 48, cfg.ScreenWidth/2-50-len(title)*48*3/10/2, 160, color.RGBA{100, 200, 255, 255})
	sub := "A Colony Sim"
	mlge_text.Draw(screen, sub, 18, cfg.ScreenWidth/2-len(sub)*18*3/10/2, 230, color.RGBA{70, 140, 180, 255})

	switch ts.screen {
	case screenMain:
		ts.newBtn.Draw(screen)
		ts.loadBtn.Draw(screen)
		ts.settingsBtn.Draw(screen)
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
		if !ts.raceIsNotColony() {
			ts.tutorialToggle.Draw(screen)
		}
		mlge_text.Draw(screen, "Rival factions:", 14, lx, 398, lblCol)
		ts.factionsMinus.Draw(screen)
		ts.factionsPlus.Draw(screen)
		ts.drawRivalRows(screen)
		val := strconv.Itoa(ts.factions)
		vw, _ := mlge_text.Measure(val, 16)
		mlge_text.Draw(screen, val, 16, cfg.ScreenWidth/2+120-int(vw)/2, 396, color.RGBA{220, 230, 255, 255})
		mlge_text.Draw(screen, "Galaxy size:", 14, lx, 438, lblCol) // DRAFT
		ts.sizePrev.Draw(screen)
		ts.sizeNext.Draw(screen)
		if len(ts.sizes) > 0 {
			sz := ts.sizes[ts.size]
			name := fmt.Sprintf("%s (%d)", sz.NameDraft, sz.Systems)
			nw, _ := mlge_text.Measure(name, 16)
			mlge_text.Draw(screen, name, 16, cfg.ScreenWidth/2+74-int(nw)/2, 436, color.RGBA{220, 230, 255, 255})
		}
		if ts.raceChoice() {
			mlge_text.Draw(screen, "Race:", 14, lx, 478, lblCol) // DRAFT
			ts.racePrev.Draw(screen)
			ts.raceNext.Draw(screen)
			name := ts.raceName()
			nw, _ := mlge_text.Measure(name, 16)
			mlge_text.Draw(screen, name, 16, cfg.ScreenWidth/2+74-int(nw)/2, 476, color.RGBA{220, 230, 255, 255})
			if b := ts.raceBlurb(); b != "" {
				mlge_text.Draw(screen, b, 12, lx, 506, color.RGBA{150, 170, 195, 255})
			}
		}
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

// raceChoice reports whether the race row is offered: more than one
// playable doctrine, and not an observer run (always the colony's).
func (ts *TitleState) raceChoice() bool {
	return len(ts.races) > 1 && !config.Global().ObserverMode
}

// stepSize moves the Galaxy size choice by d, wrapping.
func (ts *TitleState) stepSize(d int) {
	if n := len(ts.sizes); n > 0 {
		ts.size = ((ts.size+d)%n + n) % n
	}
}

// sizeID is the chosen Galaxy size's id ("" with none).
func (ts *TitleState) sizeID() string {
	if ts.size < 0 || ts.size >= len(ts.sizes) {
		return ""
	}
	return ts.sizes[ts.size].ID
}

// stepRace moves the race choice by d, wrapping.
func (ts *TitleState) stepRace(d int) {
	if n := len(ts.races); n > 0 {
		ts.race = ((ts.race+d)%n + n) % n
	}
}

// raceKey is the chosen doctrine ("" for the colony or no choice).
func (ts *TitleState) raceKey() string {
	if !ts.raceChoice() || ts.race <= 0 || ts.race >= len(ts.races) {
		return ""
	}
	return ts.races[ts.race]
}

func (ts *TitleState) raceDoctrine() *campaign.Doctrine {
	if ts.race < 0 || ts.race >= len(ts.races) {
		return nil
	}
	return campaign.LoadFactionData().Doctrine(ts.races[ts.race])
}

// raceName is the chosen race's name on the row.
func (ts *TitleState) raceName() string {
	if d := ts.raceDoctrine(); d != nil {
		return d.Name
	}
	return "Colony"
}

// raceBlurb is the chosen race's one line under the row (draft).
func (ts *TitleState) raceBlurb() string {
	if d := ts.raceDoctrine(); d != nil && d.Race != nil {
		return d.Race.Blurb
	}
	return ""
}


// raceIsNotColony reports a race other than the colony picked: the tutorial
// is the colony's only (open question 16), for a ship race too.
func (ts *TitleState) raceIsNotColony() bool { return ts.raceKey() != "" }
