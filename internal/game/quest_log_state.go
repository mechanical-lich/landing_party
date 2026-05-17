package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// QuestLogState is the campaign quest log: available quests can be accepted,
// active quests show their objective, completed quests are listed. Reached
// from the Star Map; Back returns to it.
type QuestLogState struct {
	campaign *campaign.Campaign
	wm       *WorldManager

	list    *minui.ListBox
	quests  []*campaign.Quest
	accept  *minui.Button
	backBtn *minui.Button

	status string
	done   bool
	next   state.StateInterface
}

var _ state.StateInterface = (*QuestLogState)(nil)

func NewQuestLogState(c *campaign.Campaign, wm *WorldManager) *QuestLogState {
	q := &QuestLogState{campaign: c, wm: wm}
	cfg := config.Global()
	cx := cfg.ScreenWidth / 2

	q.list = minui.NewListBox("ql_list", nil)
	q.list.SetBounds(minui.Rect{X: cx - 300, Y: 180, Width: 600, Height: 320})
	q.list.Layout()

	q.accept = minui.NewButton("ql_accept", "Accept Quest")
	q.accept.SetPosition(cx-300, 620)
	q.accept.SetSize(200, 36)
	q.accept.OnClick = func() { q.acceptSelected() }

	q.backBtn = minui.NewButton("ql_back", "Back to Star Map")
	q.backBtn.SetPosition(cx+90, 620)
	q.backBtn.SetSize(210, 36)
	q.backBtn.OnClick = func() {
		q.next = NewOverworldState(q.campaign, q.wm)
		q.done = true
	}

	q.refresh()
	return q
}

func (q *QuestLogState) refresh() {
	q.quests = q.quests[:0]
	var labels []string
	add := func(qs []*campaign.Quest, tag string) {
		for _, def := range qs {
			q.quests = append(q.quests, def)
			labels = append(labels, fmt.Sprintf("[%s] %s", tag, def.Name))
		}
	}
	add(q.campaign.AvailableQuests(), "OFFER")
	add(q.campaign.ActiveQuests(), "ACTIVE")
	add(q.campaign.CompletedQuests(), "DONE")
	if len(labels) == 0 {
		labels = []string{"(no quests)"}
	}
	sel := q.list.SelectedIndex
	q.list.SetItems(labels)
	if sel >= 0 && sel < len(q.quests) {
		q.list.SelectedIndex = sel
	} else if len(q.quests) > 0 {
		q.list.SelectedIndex = 0
	}
}

func (q *QuestLogState) selected() *campaign.Quest {
	i := q.list.SelectedIndex
	if i < 0 || i >= len(q.quests) {
		return nil
	}
	return q.quests[i]
}

func (q *QuestLogState) acceptSelected() {
	def := q.selected()
	if def == nil {
		return
	}
	if q.campaign.AcceptQuest(def.ID) {
		q.status = "Accepted: " + def.Name
		if def.Location != "" {
			name := def.Location
			if loc := q.campaign.Locations[def.Location]; loc != nil {
				name = loc.Name
			}
			q.status += " — " + name + " revealed on the Star Map"
		}
		q.refresh()
	} else {
		q.status = "That quest can't be accepted."
	}
}

func (q *QuestLogState) Update() state.StateInterface {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		q.next = NewOverworldState(q.campaign, q.wm)
		q.done = true
		return q.next
	}
	q.list.Update()
	q.accept.Update()
	q.backBtn.Update()
	return q.next
}

func (q *QuestLogState) Draw(screen *ebiten.Image) {
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	cfg := config.Global()
	screen.Fill(color.RGBA{6, 8, 16, 255})
	cx := cfg.ScreenWidth / 2

	title := "Quest Log"
	mlge_text.Draw(screen, title, 40, cx-len(title)*40*3/10/2, 100, color.RGBA{120, 200, 255, 255})

	q.list.Draw(screen)

	if def := q.selected(); def != nil {
		y := 520
		mlge_text.Draw(screen, def.Name, 18, cx-300, y, color.RGBA{220, 230, 255, 255})
		mlge_text.Draw(screen, def.Description, 13, cx-300, y+26, color.RGBA{180, 200, 220, 255})
		if def.Location != "" {
			locName := def.Location
			if loc := q.campaign.Locations[def.Location]; loc != nil {
				locName = loc.Name
			}
			mlge_text.Draw(screen, "Location: "+locName, 13, cx-300, y+48, color.RGBA{170, 200, 240, 255})
		}
		reward := questRewardText(def)
		if reward != "" {
			mlge_text.Draw(screen, "Reward: "+reward, 13, cx-300, y+70, color.RGBA{150, 210, 160, 255})
		}
	}

	q.accept.Draw(screen)
	q.backBtn.Draw(screen)

	if q.status != "" {
		mlge_text.Draw(screen, q.status, 13, cx-300, cfg.ScreenHeight-40, color.RGBA{230, 160, 90, 255})
	}
	minui.FlushOverlays(screen)
}

func (q *QuestLogState) Done() bool { return q.done }

func questRewardText(def *campaign.Quest) string {
	parts := []string{}
	if def.Reward.Fuel > 0 {
		parts = append(parts, fmt.Sprintf("%d fuel", def.Reward.Fuel))
	}
	for bp, n := range def.Reward.Resources {
		parts = append(parts, fmt.Sprintf("%d %s", n, bp))
	}
	for _, id := range def.Reward.RevealLocations {
		parts = append(parts, "reveal "+id)
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
