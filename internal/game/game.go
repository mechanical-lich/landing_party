package game

import (
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/input"
	"github.com/mechanical-lich/mlge/resource"
	"github.com/mechanical-lich/mlge/state"
)

type Game struct {
	title        string
	StateMachine state.StateMachine
	InputManager *input.InputManager
}

func NewGame(title string) (*Game, error) {
	g := &Game{title: title, InputManager: input.NewInputManager(event.GetQueuedInstance())}
	ebiten.SetWindowSize(config.Global().ScreenWidth, config.Global().ScreenHeight)
	ebiten.SetWindowTitle(title)
	ebiten.SetTPS(60)

	if err := resource.LoadAssetsFromJSON("data/assets.json"); err != nil {
		return nil, err
	}

	g.StateMachine.PushState(NewTitleState())
	return g, nil
}

func (g *Game) Run() error {
	return ebiten.RunGame(g)
}

func (g *Game) Update() error {
	g.InputManager.HandleInput()
	event.GetQueuedInstance().HandleQueue()
	g.StateMachine.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.StateMachine.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return config.Global().ScreenWidth, config.Global().ScreenHeight
}
