package game

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/mlge/ui/minui"
)

const (
	beamModalW       = 640
	beamModalRowH    = 36
	beamModalBodyTop = 60 // title bar (30) + column-header row (25) + gap (5)
	beamModalFooterH = 120 // status (20) + gap (8) + close button (30) + bottom padding (62)
)

// beamRow holds the live widgets for one resource type.
type beamRow struct {
	name     string
	shipLbl  *minui.Label
	siteLbl  *minui.Label
	qtyInput *minui.TextInput
}

// BeamResourcesModal is the Star Map overlay for transferring resources between
// the ship hold and the current site's storage locker.
type BeamResourcesModal struct {
	modal     *minui.Modal
	rows      []beamRow
	statusLbl *minui.Label
	wm        *WorldManager
	Visible   bool
}

func newBeamResourcesModal(wm *WorldManager) *BeamResourcesModal {
	return &BeamResourcesModal{wm: wm}
}

// Open (re)builds the modal with the current inventory snapshot and shows it.
func (b *BeamResourcesModal) Open(locName string) {
	b.rebuild(locName)
	b.Visible = true
}

func (b *BeamResourcesModal) rebuild(locName string) {
	wm := b.wm
	names := wm.AllResourceNames()

	numRows := len(names)
	emptyMsg := numRows == 0
	if emptyMsg {
		numRows = 1 // space for the "nothing here" label
	}
	modalH := beamModalBodyTop + numRows*beamModalRowH + beamModalFooterH

	cfg := config.Global()
	mx := (cfg.ScreenWidth - beamModalW) / 2
	my := (cfg.ScreenHeight - modalH) / 2

	title := "Beam Resources"
	if locName != "" {
		title += " — " + locName
	}
	b.modal = minui.NewModal("beam_res_modal", title, beamModalW, modalH)
	b.modal.SetPosition(mx, my)
	b.modal.Closeable = false

	// Column header labels (positions are relative to modal top-left corner).
	const (
		colName   = 8
		colShip   = 178
		colSite   = 268
		colQty    = 358
		colToShip = 438
		colToSite = 530
	)
	hdrY := 35
	for _, h := range []struct {
		x int
		t string
	}{
		{colName, "Resource"},
		{colShip, "On Ship"},
		{colSite, "On Site"},
		{colQty, "Amount"},
	} {
		lbl := minui.NewLabel("beam_hdr_"+h.t, h.t)
		lbl.SetPosition(h.x, hdrY)
		b.modal.AddChild(lbl)
	}

	b.rows = b.rows[:0]

	if emptyMsg {
		msg := minui.NewLabel("beam_empty", "No resources on ship or at this site.")
		msg.SetPosition(colName, beamModalBodyTop+8)
		b.modal.AddChild(msg)
	}

	colony := campaignColonyName(wm.Campaign)
	shipOwners := []string{campaign.ShipSettlementName}
	siteOwners := []string{colony}
	shipP := storage.ShipProvider{Ship: wm.Campaign.Ship}
	var siteP storage.Provider
	if wm.current != nil && wm.current.level != nil {
		siteP = storage.LevelProvider{Level: wm.current.level}
	}

	for i, name := range names {
		rowY := beamModalBodyTop + i*beamModalRowH
		btnY := rowY + (beamModalRowH-28)/2 // vertically centre 28-px-tall widgets in row

		shipCount := storage.CountResource(shipP, shipOwners, name)
		siteCount := 0
		if siteP != nil {
			siteCount = storage.CountResource(siteP, siteOwners, name)
		}

		nameLbl := minui.NewLabel(fmt.Sprintf("beam_name_%d", i), displayResourceName(name))
		nameLbl.SetPosition(colName, rowY+8)
		b.modal.AddChild(nameLbl)

		shipLbl := minui.NewLabel(fmt.Sprintf("beam_ship_%d", i), fmt.Sprintf("%d", shipCount))
		shipLbl.SetPosition(colShip, rowY+8)
		shipLbl.SetSize(80, 20)
		b.modal.AddChild(shipLbl)

		siteLbl := minui.NewLabel(fmt.Sprintf("beam_site_%d", i), fmt.Sprintf("%d", siteCount))
		siteLbl.SetPosition(colSite, rowY+8)
		siteLbl.SetSize(80, 20)
		b.modal.AddChild(siteLbl)

		qtyInput := minui.NewTextInput(fmt.Sprintf("beam_qty_%d", i), "")
		qtyInput.SetPosition(colQty, btnY)
		qtyInput.SetSize(70, 28)
		b.modal.AddChild(qtyInput)

		// Capture loop vars for closures.
		n := name
		qi := qtyInput

		toShipBtn := minui.NewButton(fmt.Sprintf("beam_to_ship_%d", i), "▲ To Ship")
		toShipBtn.SetPosition(colToShip, btnY)
		toShipBtn.SetSize(84, 28)
		toShipBtn.OnClick = func() {
			qty := parseBeamQty(qi.Text)
			if qty <= 0 {
				b.setStatus("Enter a positive whole number.")
				return
			}
			if err := wm.BeamResourceToShip(n, qty); err != nil {
				b.setStatus(err.Error())
			} else {
				b.setStatus(fmt.Sprintf("Beamed %d %s to ship.", qty, displayResourceName(n)))
				qi.SetText("")
			}
		}
		b.modal.AddChild(toShipBtn)

		toSiteBtn := minui.NewButton(fmt.Sprintf("beam_to_site_%d", i), "▼ To Site")
		toSiteBtn.SetPosition(colToSite, btnY)
		toSiteBtn.SetSize(84, 28)
		toSiteBtn.OnClick = func() {
			qty := parseBeamQty(qi.Text)
			if qty <= 0 {
				b.setStatus("Enter a positive whole number.")
				return
			}
			if err := wm.BeamResourceToSite(n, qty); err != nil {
				b.setStatus(err.Error())
			} else {
				b.setStatus(fmt.Sprintf("Beamed %d %s to site.", qty, displayResourceName(n)))
				qi.SetText("")
			}
		}
		b.modal.AddChild(toSiteBtn)

		b.rows = append(b.rows, beamRow{
			name:     name,
			shipLbl:  shipLbl,
			siteLbl:  siteLbl,
			qtyInput: qtyInput,
		})
	}

	// Status label and close button below the rows.
	statusY := beamModalBodyTop + numRows*beamModalRowH + 8
	b.statusLbl = minui.NewLabel("beam_status", "")
	b.statusLbl.SetPosition(colName, statusY)
	b.statusLbl.SetColor(color.RGBA{230, 160, 90, 255})
	b.modal.AddChild(b.statusLbl)

	closeBtn := minui.NewButton("beam_close", "Close")
	closeBtn.SetPosition((beamModalW-100)/2, statusY+28)
	closeBtn.SetSize(100, 30)
	closeBtn.OnClick = func() { b.Visible = false }
	b.modal.AddChild(closeBtn)
}

func (b *BeamResourcesModal) setStatus(msg string) {
	if b.statusLbl != nil {
		b.statusLbl.Text = msg
	}
}

// refreshCounts updates ship/site labels every frame so they stay live after
// each beam operation without requiring a full rebuild.
func (b *BeamResourcesModal) refreshCounts() {
	if b.wm == nil || b.wm.Campaign == nil {
		return
	}
	colony := campaignColonyName(b.wm.Campaign)
	shipOwners := []string{campaign.ShipSettlementName}
	siteOwners := []string{colony}
	shipP := storage.ShipProvider{Ship: b.wm.Campaign.Ship}
	var siteP storage.Provider
	if b.wm.current != nil && b.wm.current.level != nil {
		siteP = storage.LevelProvider{Level: b.wm.current.level}
	}
	for i := range b.rows {
		row := &b.rows[i]
		row.shipLbl.Text = fmt.Sprintf("%d", storage.CountResource(shipP, shipOwners, row.name))
		row.shipLbl.Layout()
		if siteP != nil {
			row.siteLbl.Text = fmt.Sprintf("%d", storage.CountResource(siteP, siteOwners, row.name))
		} else {
			row.siteLbl.Text = "—"
		}
		row.siteLbl.Layout()
	}
}

func (b *BeamResourcesModal) Update() {
	if !b.Visible {
		return
	}
	b.refreshCounts()
	b.modal.Update()
}

func (b *BeamResourcesModal) Draw(screen *ebiten.Image) {
	if !b.Visible {
		return
	}
	b.modal.Draw(screen)
}

// displayResourceName converts blueprint IDs like "metal_ore" to "Metal Ore".
func displayResourceName(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// parseBeamQty parses a positive integer from s, returning 0 on any failure.
func parseBeamQty(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || v <= 0 {
		return 0
	}
	return v
}
