package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// crewPanel is a read-only crew inspector: a roster list on the left (everyone
// aboard the ship plus everyone planetside), and the selected colonist's
// location, vitals, stats, and equipment on the right.
type crewPanel struct {
	campaign *campaign.Campaign
	wm       *WorldManager
	list     *minui.ListBox
	crew     []crewMember
}

type crewMember struct {
	ent    *ecs.Entity
	onShip bool
}

func newCrewPanel(c *campaign.Campaign, wm *WorldManager) *crewPanel {
	p := &crewPanel{campaign: c, wm: wm}
	cfg := config.Global()
	p.list = minui.NewListBox("dash_crew_list", nil)
	p.list.SetBounds(minui.Rect{X: 20, Y: dashContentTop + 16, Width: 360, Height: cfg.ScreenHeight - dashContentTop - 56})
	p.list.Layout()
	p.refresh()
	return p
}

func (p *crewPanel) Enter() { p.refresh() }

func (p *crewPanel) refresh() {
	p.crew = p.crew[:0]
	var labels []string
	for _, e := range p.wm.ShipColonists() {
		p.crew = append(p.crew, crewMember{ent: e, onShip: true})
		labels = append(labels, entityDisplayName(e)+"  · Ship")
	}
	siteName := "Planetside"
	if loc := p.campaign.CurrentLocation(); loc != nil && p.wm.current != nil {
		siteName = loc.Name
	}
	for _, e := range p.wm.PlanetColonists() {
		p.crew = append(p.crew, crewMember{ent: e, onShip: false})
		labels = append(labels, entityDisplayName(e)+"  · "+siteName)
	}
	if len(labels) == 0 {
		labels = []string{"(no crew)"}
	}
	sel := p.list.SelectedIndex
	p.list.SetItems(labels)
	if sel >= 0 && sel < len(p.crew) {
		p.list.SelectedIndex = sel
	} else if len(p.crew) > 0 {
		p.list.SelectedIndex = 0
	}
}

func (p *crewPanel) selected() (crewMember, bool) {
	i := p.list.SelectedIndex
	if i < 0 || i >= len(p.crew) {
		return crewMember{}, false
	}
	return p.crew[i], true
}

func (p *crewPanel) Update()                          { p.list.Update() }
func (p *crewPanel) Transition() state.StateInterface { return nil }

func (p *crewPanel) Draw(screen *ebiten.Image) {
	p.list.Draw(screen)

	cm, ok := p.selected()
	if !ok {
		mlge_text.Draw(screen, "Select a crew member.", 14, 420, dashContentTop+40, dashBodyColor)
		return
	}
	e := cm.ent
	x := 420
	y := dashContentTop + 20
	hdr := color.RGBA{200, 220, 255, 255}

	mlge_text.Draw(screen, colonistName(e), 22, x, y, color.RGBA{230, 240, 255, 255})
	y += 34

	loc := "Aboard the Ship"
	if !cm.onShip {
		loc = "Planetside"
		if l := p.campaign.CurrentLocation(); l != nil && p.wm.current != nil {
			loc = "On " + l.Name
		}
	}
	mlge_text.Draw(screen, loc, 14, x, y, color.RGBA{170, 200, 240, 255})
	y += 36

	// Vitals.
	if e.HasComponent(rlcomponents.Health) {
		hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		mlge_text.Draw(screen, fmt.Sprintf("Health:      %d / %d", hc.Health, hc.MaxHealth), 14, x, y, dashBodyColor)
		y += 22
	}
	if e.HasComponent(components.Needs) {
		nc := e.GetComponent(components.Needs).(*components.NeedsComponent)
		mlge_text.Draw(screen, fmt.Sprintf("Hunger:      %d / %d", nc.Hunger, nc.MaxHunger), 14, x, y, dashBodyColor)
		y += 22
		mlge_text.Draw(screen, fmt.Sprintf("Exhaustion:  %d / %d", nc.Exhaustion, nc.MaxExhaustion), 14, x, y, dashBodyColor)
		y += 22
	}
	y += 12

	// Stats.
	mlge_text.Draw(screen, "Stats", 16, x, y, hdr)
	y += 24
	if e.HasComponent(components.StatProgression) {
		sp := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
		mlge_text.Draw(screen, fmt.Sprintf("STR %d      DEX %d      INT %d      CON %d",
			sp.Str.Level, sp.Dex.Level, sp.Int.Level, sp.Con.Level), 14, x, y, dashBodyColor)
		y += 26
	} else {
		mlge_text.Draw(screen, "(no stats)", 13, x, y, dashBodyColor)
		y += 26
	}
	y += 12

	// Equipment.
	mlge_text.Draw(screen, "Equipment", 16, x, y, hdr)
	y += 24
	shown := 0
	if e.HasComponent(rlcomponents.Inventory) {
		inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		slots := []struct {
			label string
			item  *ecs.Entity
		}{
			{"Right Hand", inv.RightHand},
			{"Left Hand", inv.LeftHand},
			{"Head", inv.Head},
			{"Torso", inv.Torso},
			{"Legs", inv.Legs},
			{"Feet", inv.Feet},
		}
		for _, s := range slots {
			if s.item == nil {
				continue
			}
			mlge_text.Draw(screen, fmt.Sprintf("%-12s %s", s.label, displayMaterialName(s.item.Blueprint)), 13, x, y, dashBodyColor)
			y += 22
			shown++
		}
	}
	if shown == 0 {
		mlge_text.Draw(screen, "(nothing equipped)", 13, x, y, color.RGBA{150, 170, 195, 255})
	}
}

// colonistName returns a colonist's individual name without the stat suffix
// that entityDisplayName adds for list rows.
func colonistName(e *ecs.Entity) string {
	if e != nil && e.HasComponent(rlcomponents.Description) {
		if dc, ok := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent); ok && dc.Name != "" {
			return dc.Name
		}
	}
	return "Colonist"
}
