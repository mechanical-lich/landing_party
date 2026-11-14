package game

import (
	"fmt"
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// The rival boxes (rival_selection_plan.md, step 3): under the Rival
// factions number, one row per rival in a second column, each choosing
// Random or a seedable doctrine and a group, plus the player's own group.
// Rows keep their picks when the number drops. All labels are drafts.

// rivalRow is one rival box.
type rivalRow struct {
	pick                      int // 0 Random, else rivalChoices[pick-1]
	group                     int
	prev, next, gMinus, gPlus *minui.Button
}

// rivalColumnX is the second column's left edge.
func rivalColumnX() int { return config.Global().ScreenWidth/2 + 200 }

// buildRivalRows makes factionsMax rows and the player's group stepper.
func (ts *TitleState) buildRivalRows() {
	ts.rivalChoices = campaign.RivalChoices()
	ts.playerGroup = campaign.PlayerGroup
	x0 := rivalColumnX()
	ts.playerGMinus = minui.NewButton("player_group_minus", "-")
	ts.playerGMinus.SetPosition(x0+252, 350)
	ts.playerGMinus.SetSize(28, 28)
	ts.playerGMinus.OnClick = func() { ts.playerGroup = max(ts.playerGroup-1, 1) }
	ts.playerGPlus = minui.NewButton("player_group_plus", "+")
	ts.playerGPlus.SetPosition(x0+312, 350)
	ts.playerGPlus.SetSize(28, 28)
	ts.playerGPlus.OnClick = func() { ts.playerGroup = min(ts.playerGroup+1, 9) }
	groups := campaign.RivalGroups(ts.factionsMax)
	ts.rivals = make([]*rivalRow, ts.factionsMax)
	for i := range ts.rivals {
		r := &rivalRow{group: groups[i]}
		y := 392 + 36*i
		r.prev = minui.NewButton(fmt.Sprintf("rival_prev_%d", i), "<")
		r.prev.SetPosition(x0+64, y)
		r.prev.SetSize(28, 28)
		r.prev.OnClick = func() { r.pick = (r.pick - 1 + len(ts.rivalChoices) + 1) % (len(ts.rivalChoices) + 1) }
		r.next = minui.NewButton(fmt.Sprintf("rival_next_%d", i), ">")
		r.next.SetPosition(x0+196, y)
		r.next.SetSize(28, 28)
		r.next.OnClick = func() { r.pick = (r.pick + 1) % (len(ts.rivalChoices) + 1) }
		r.gMinus = minui.NewButton(fmt.Sprintf("rival_group_minus_%d", i), "-")
		r.gMinus.SetPosition(x0+252, y)
		r.gMinus.SetSize(28, 28)
		r.gMinus.OnClick = func() { r.group = max(r.group-1, 1) }
		r.gPlus = minui.NewButton(fmt.Sprintf("rival_group_plus_%d", i), "+")
		r.gPlus.SetPosition(x0+312, y)
		r.gPlus.SetSize(28, 28)
		r.gPlus.OnClick = func() { r.group = min(r.group+1, 9) }
		ts.rivals[i] = r
	}
}

// rivalBoxes reports whether the boxes are in use (not under the count
// path's config switch).
func (ts *TitleState) rivalBoxes() bool { return !config.Global().RivalsByCount }

func (ts *TitleState) updateRivalRows() {
	if !ts.rivalBoxes() {
		return
	}
	ts.playerGMinus.Update()
	ts.playerGPlus.Update()
	for i := 0; i < ts.factions && i < len(ts.rivals); i++ {
		r := ts.rivals[i]
		r.prev.Update()
		r.next.Update()
		r.gMinus.Update()
		r.gPlus.Update()
	}
}

// rivalPickName is a box's shown choice: Random, or the doctrine's name as
// the rival is named everywhere else (the colony's rival view by its rival
// name, "Rival Colony").
func (ts *TitleState) rivalPickName(pick int) string {
	if pick <= 0 || pick > len(ts.rivalChoices) {
		return "Random" // DRAFT
	}
	return rivalChoiceName(ts.rivalChoices[pick-1])
}

// rivalChoiceName is a rival choice's name: its view's (the rival block's
// name), else its base doctrine's, else the key.
func rivalChoiceName(k string) string {
	fd := campaign.LoadFactionData()
	for _, key := range []string{k, campaign.BaseDoctrine(k)} {
		if d := fd.Doctrine(key); d != nil && d.Name != "" {
			return d.Name
		}
	}
	return k
}

func (ts *TitleState) drawRivalRows(screen *ebiten.Image) {
	if !ts.rivalBoxes() {
		return
	}
	x0 := rivalColumnX()
	lbl := color.RGBA{180, 210, 255, 255}
	val := color.RGBA{220, 230, 255, 255}
	mlge_text.Draw(screen, "Your group:", 14, x0, 358, lbl) // DRAFT
	ts.playerGMinus.Draw(screen)
	ts.playerGPlus.Draw(screen)
	mlge_text.Draw(screen, strconv.Itoa(ts.playerGroup), 16, x0+290, 354, val)
	for i := 0; i < ts.factions && i < len(ts.rivals); i++ {
		r := ts.rivals[i]
		y := 392 + 36*i
		mlge_text.Draw(screen, fmt.Sprintf("Rival %d:", i+1), 14, x0, y+6, lbl) // DRAFT
		r.prev.Draw(screen)
		r.next.Draw(screen)
		name := ts.rivalPickName(r.pick)
		nw, _ := mlge_text.Measure(name, 15)
		mlge_text.Draw(screen, name, 15, x0+144-int(nw)/2, y+4, val)
		mlge_text.Draw(screen, "grp", 12, x0+228, y+8, lbl) // DRAFT
		r.gMinus.Draw(screen)
		r.gPlus.Draw(screen)
		mlge_text.Draw(screen, strconv.Itoa(r.group), 16, x0+290, y+4, val)
	}
}

// rivalPicks is the boxes as GenOptions.Rivals (nil under the count path's
// switch, or with no rivals).
func (ts *TitleState) rivalPicks() []campaign.RivalPick {
	if !ts.rivalBoxes() || ts.factions <= 0 {
		return nil
	}
	out := make([]campaign.RivalPick, 0, ts.factions)
	for i := 0; i < ts.factions && i < len(ts.rivals); i++ {
		r := ts.rivals[i]
		p := campaign.RivalPick{Group: r.group}
		if r.pick > 0 && r.pick <= len(ts.rivalChoices) {
			p.Doctrine = ts.rivalChoices[r.pick-1]
		}
		out = append(out, p)
	}
	return out
}
