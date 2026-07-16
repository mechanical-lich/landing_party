package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/audio"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// SettingsState is the options screen (its own state). V1 exposes the three
// audio volume sliders; it's opened from the title screen and pops back to it.
type SettingsState struct {
	done         bool
	sliders      []*volumeSlider
	friendlyFoot *minui.Toggle
	enemyFoot    *minui.Toggle
	backBtn      *minui.Button
}

// volumeSlider is a click/drag track for a 0..1 value. It applies its value to
// the audio bus live while dragging (immediate feedback) and reports a release
// so the caller can persist.
type volumeSlider struct {
	label    string
	value    float64
	apply    func(float64) // live bus update during drag
	x, y, w  int           // track geometry (top-left, width; fixed height)
	dragging bool
}

const volSliderHeight = 12

func (s *volumeSlider) update() (released bool) {
	mx, my := ebiten.CursorPosition()
	hit := mx >= s.x-10 && mx <= s.x+s.w+10 && my >= s.y-10 && my <= s.y+volSliderHeight+10
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && hit {
		s.dragging = true
	}
	if !s.dragging {
		return false
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		v := clampUnit(float64(mx-s.x) / float64(s.w))
		if v != s.value {
			s.value = v
			if s.apply != nil {
				s.apply(v)
			}
		}
		return false
	}
	s.dragging = false
	return true
}

func (s *volumeSlider) draw(screen *ebiten.Image) {
	labelCol := color.RGBA{180, 210, 255, 255}
	mlge_text.Draw(screen, s.label, 16, s.x-220, s.y-2, labelCol)

	track := minui.Rect{X: s.x, Y: s.y, Width: s.w, Height: volSliderHeight}
	minui.DrawRoundedRect(screen, track, volSliderHeight/2, color.RGBA{40, 48, 66, 255})
	fillW := int(float64(s.w) * s.value)
	if fillW > 0 {
		fill := minui.Rect{X: s.x, Y: s.y, Width: fillW, Height: volSliderHeight}
		minui.DrawRoundedRect(screen, fill, volSliderHeight/2, color.RGBA{90, 170, 230, 255})
	}
	handle := minui.Rect{X: s.x + fillW - 6, Y: s.y - 4, Width: 12, Height: volSliderHeight + 8}
	minui.DrawRoundedRect(screen, handle, 4, color.RGBA{200, 220, 255, 255})

	pct := fmt.Sprintf("%d%%", int(s.value*100+0.5))
	mlge_text.Draw(screen, pct, 16, s.x+s.w+20, s.y-2, labelCol)
}

func NewSettingsState() *SettingsState {
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2
	trackX, trackW := cx-40, 300
	startY, spacing := 320, 70

	ss := &SettingsState{}
	ss.sliders = []*volumeSlider{
		{label: "UI Volume", value: clampUnit(cfg.UIVolume), apply: audio.SetUIVolume, x: trackX, y: startY, w: trackW},
		{label: "Game Volume", value: clampUnit(cfg.GameVolume), apply: audio.SetGameVolume, x: trackX, y: startY + spacing, w: trackW},
		{label: "Music Volume", value: clampUnit(cfg.MusicVolume), apply: audio.SetMusicVolume, x: trackX, y: startY + spacing*2, w: trackW},
	}

	toggleY := startY + spacing*3
	ss.friendlyFoot = minui.NewToggle("settings_friendly_footsteps", "Friendly footstep sounds")
	ss.friendlyFoot.On = cfg.FriendlyFootsteps
	ss.friendlyFoot.SetPosition(trackX-220, toggleY)
	ss.friendlyFoot.OnChange = func(bool) { ss.commit() }

	ss.enemyFoot = minui.NewToggle("settings_enemy_footsteps", "Enemy footstep sounds")
	ss.enemyFoot.On = cfg.EnemyFootsteps
	ss.enemyFoot.SetPosition(trackX-220, toggleY+40)
	ss.enemyFoot.OnChange = func(bool) { ss.commit() }

	ss.backBtn = minui.NewButton("settings_back", "Back")
	ss.backBtn.SetSize(220, 36)
	ss.backBtn.SetPosition(cx-110, toggleY+100)
	ss.backBtn.OnClick = func() {
		ss.commit()
		ss.done = true
	}
	return ss
}

// commit writes the current audio settings to config.local.json, reloads the
// config, and re-applies them (per the settings flow).
func (ss *SettingsState) commit() {
	if err := config.SaveAudioSettings(
		ss.sliders[0].value, ss.sliders[1].value, ss.sliders[2].value,
		ss.friendlyFoot.On, ss.enemyFoot.On,
	); err != nil {
		return
	}
	cfg := config.Global()
	audio.ApplyVolumes(cfg.UIVolume, cfg.GameVolume, cfg.MusicVolume)
	audio.SetFootstepAudio(cfg.FriendlyFootsteps, cfg.EnemyFootsteps)
}

func (ss *SettingsState) Update() state.StateInterface {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		ss.commit()
		ss.done = true
		return nil
	}
	for _, s := range ss.sliders {
		if s.update() {
			ss.commit() // persist each adjustment on release
		}
	}
	ss.friendlyFoot.Update()
	ss.enemyFoot.Update()
	ss.backBtn.Update()
	return nil
}

func (ss *SettingsState) Draw(screen *ebiten.Image) {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cfg := config.Global()
	screen.Fill(color.RGBA{8, 10, 18, 255})

	title := "Settings"
	mlge_text.Draw(screen, title, 40, cfg.ScreenWidth/2-len(title)*40*3/10/2, 200, color.RGBA{100, 200, 255, 255})
	for _, s := range ss.sliders {
		s.draw(screen)
	}
	ss.friendlyFoot.Draw(screen)
	ss.enemyFoot.Draw(screen)
	ss.backBtn.Draw(screen)
	minui.FlushOverlays(screen)
}

func (ss *SettingsState) Done() bool { return ss.done }

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
