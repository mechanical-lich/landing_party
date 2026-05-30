package game

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// Layout constants for the storage inspector modal.
const (
	siModalW = 760
	siModalH = 520

	// Column-header label row (just under the title bar).
	siColHdrY = 36

	// Listbox row band.
	siListY = 60
	siListH = 320

	// Column positions: tag filter | inventory | allowed filter.
	siFilterColX = 12
	siFilterColW = 140
	siInvColX    = 160
	siInvColW    = 420
	siAllowColX  = 590
	siAllowColW  = 158

	// Action panel.
	siActionY    = siListY + siListH + 12
	siActionLblX = siInvColX
	siAmountX    = siInvColX + 70
	siAmountW    = 70
	siAllBtnX    = siInvColX + 146
	siAllBtnW    = 44
	siActionBtnX = siInvColX + 196
	siActionBtnW = 130
	siRelocateX  = siInvColX + 332
	siRelocateW  = 130
	siActionBtnH = 28

	// Status + close.
	siStatusY  = siActionY + 40
	siCloseBtnW = 100
	siCloseBtnH = 28
)

// StorageInspectorModal views the contents of one storage container and
// transfers materials to the opposite side:
//
//   - Source is the ship hold → action beams DOWN to the first colony-owned
//     locker on the current site.
//   - Source is a colony container → action beams UP to the ship hold.
//
// Layout is three list boxes side by side: display-tag filter (left),
// inventory (middle), and allowed/filter tags (right) — plus an action panel
// below for the currently-selected inventory item.
type StorageInspectorModal struct {
	modal   *minui.Modal
	wm      *WorldManager
	Visible bool

	source *ecs.Entity

	activeTagFilter string // "" = show all

	// Listboxes.
	filterList   *minui.ListBox
	filterValues []string // parallel to filterList items; "" = "All"

	invList *minui.ListBox
	invBPs  []string // parallel to invList items

	allowedList *minui.ListBox
	allowedTags []string // parallel to allowedList items (raw tag names)

	// Action panel.
	selectedBP   string
	selectedLbl  *minui.Label
	amountInput  *minui.TextInput
	actionBtn    *minui.Button
	relocateBtn  *minui.Button
	actionIsDown bool // true → "Beam Down", false → "Beam Up"

	statusLbl *minui.Label

	// OnBeginRelocate is set by MainState so the inspector can hand off to the
	// quantity modal + cursor-mode flow. Nil disables the Relocate button.
	OnBeginRelocate func(source *ecs.Entity, blueprint string, maxAvail int)
}

func newStorageInspectorModal(wm *WorldManager) *StorageInspectorModal {
	return &StorageInspectorModal{wm: wm}
}

// Open shows the inspector for the given source container. The beam direction
// is derived from the container's OwnedBy.
func (s *StorageInspectorModal) Open(source *ecs.Entity) {
	if source == nil || !source.HasComponent(components.Storage) {
		return
	}
	s.source = source
	s.activeTagFilter = ""
	s.selectedBP = ""
	s.build()
	s.Visible = true
}

func (s *StorageInspectorModal) sourceStorage() *components.StorageComponent {
	if s.source == nil || !s.source.HasComponent(components.Storage) {
		return nil
	}
	return s.source.GetComponent(components.Storage).(*components.StorageComponent)
}

// isShipSource reports whether the inspector is viewing the ship hold.
func (s *StorageInspectorModal) isShipSource() bool {
	sc := s.sourceStorage()
	return sc != nil && sc.OwnedBy == campaign.ShipSettlementName
}

// entityDisplayLabel returns a readable name for the container.
func entityDisplayLabel(e *ecs.Entity) string {
	if e == nil {
		return ""
	}
	if e.HasComponent(rlcomponents.Description) {
		dc := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		if dc.Name != "" {
			return dc.Name
		}
	}
	return displayMaterialName(e.Blueprint)
}

// collectSourceTags returns the sorted union of material tags in the source's
// inventory. Used to populate both filter listboxes.
func (s *StorageInspectorModal) collectSourceTags() []string {
	sc := s.sourceStorage()
	if sc == nil {
		return nil
	}
	tagSet := map[string]bool{}
	for _, item := range sc.Items {
		if item.HasComponent(components.Material) {
			mc := item.GetComponent(components.Material).(*components.MaterialComponent)
			for _, t := range mc.Tags {
				tagSet[t] = true
			}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

// allowedTagOptions returns the tags eligible to be in FilterTags. Currently
// that's AllowedTags if set, else the union of all material tags in the source.
func (s *StorageInspectorModal) allowedTagOptions() []string {
	sc := s.sourceStorage()
	if sc == nil {
		return nil
	}
	if len(sc.AllowedTags) > 0 {
		out := append([]string{}, sc.AllowedTags...)
		sort.Strings(out)
		return out
	}
	return s.collectSourceTags()
}

// build constructs the modal and all widgets fresh from the current source.
func (s *StorageInspectorModal) build() {
	cfg := config.Global()
	mx := (cfg.ScreenWidth - siModalW) / 2
	my := (cfg.ScreenHeight - siModalH) / 2
	if my < 10 {
		my = 10
	}

	title := "Storage Inspector — " + entityDisplayLabel(s.source)
	s.modal = minui.NewModal("si_modal", title, siModalW, siModalH)
	s.modal.SetPosition(mx, my)
	s.modal.Closeable = false

	ship := s.isShipSource()
	s.actionIsDown = ship

	// ── Column headers ────────────────────────────────────────────────────
	addHeader := func(text string, x int) {
		lbl := minui.NewLabel("si_hdr_"+text, text)
		lbl.SetPosition(x, siColHdrY)
		lbl.SetColor(color.RGBA{160, 185, 215, 255})
		s.modal.AddChild(lbl)
	}
	addHeader("Tag Filter", siFilterColX)
	invHdr := "Inventory"
	if sc := s.sourceStorage(); sc != nil && len(sc.AllowedTags) > 0 {
		invHdr = "Inventory  (filtered)"
	}
	addHeader(invHdr, siInvColX)
	addHeader("Allowed Tags", siAllowColX)

	// ── Tag filter listbox (left) ─────────────────────────────────────────
	s.filterList = minui.NewListBox("si_filter_list", nil)
	s.filterList.SetBounds(minui.Rect{X: siFilterColX, Y: siListY, Width: siFilterColW, Height: siListH})
	s.filterList.Layout()
	s.filterList.OnSelect = func(idx int, _ string) {
		if idx < 0 || idx >= len(s.filterValues) {
			return
		}
		s.activeTagFilter = s.filterValues[idx]
		s.refreshInventory()
	}
	s.modal.AddChild(s.filterList)

	// ── Inventory listbox (middle) ────────────────────────────────────────
	s.invList = minui.NewListBox("si_inv_list", nil)
	s.invList.SetBounds(minui.Rect{X: siInvColX, Y: siListY, Width: siInvColW, Height: siListH})
	s.invList.Layout()
	s.invList.OnSelect = func(idx int, _ string) {
		if idx < 0 || idx >= len(s.invBPs) {
			s.selectedBP = ""
			s.updateActionPanel()
			return
		}
		s.selectedBP = s.invBPs[idx]
		s.updateActionPanel()
	}
	s.modal.AddChild(s.invList)

	// ── Allowed-tags listbox (right) ──────────────────────────────────────
	s.allowedList = minui.NewListBox("si_allow_list", nil)
	s.allowedList.SetBounds(minui.Rect{X: siAllowColX, Y: siListY, Width: siAllowColW, Height: siListH})
	s.allowedList.Layout()
	s.allowedList.OnSelect = func(idx int, _ string) {
		if idx < 0 || idx >= len(s.allowedTags) {
			return
		}
		s.toggleFilterTag(s.allowedTags[idx])
		s.refreshAllowedList()
	}
	s.modal.AddChild(s.allowedList)

	// ── Action panel ──────────────────────────────────────────────────────
	s.selectedLbl = minui.NewLabel("si_selected", "Select a material to transfer.")
	s.selectedLbl.SetPosition(siActionLblX, siActionY)
	s.selectedLbl.SetSize(siInvColW, 20)
	s.modal.AddChild(s.selectedLbl)

	amtLbl := minui.NewLabel("si_amt_lbl", "Amount:")
	amtLbl.SetPosition(siActionLblX, siActionY+24)
	s.modal.AddChild(amtLbl)

	s.amountInput = minui.NewTextInput("si_amount", "")
	s.amountInput.SetPosition(siAmountX, siActionY+22)
	s.amountInput.SetSize(siAmountW, siActionBtnH)
	s.modal.AddChild(s.amountInput)

	allBtn := minui.NewButton("si_all", "All")
	allBtn.SetPosition(siAllBtnX, siActionY+22)
	allBtn.SetSize(siAllBtnW, siActionBtnH)
	allBtn.OnClick = func() {
		sc := s.sourceStorage()
		if sc == nil || s.selectedBP == "" {
			return
		}
		s.amountInput.SetText(strconv.Itoa(sc.CountResource(s.selectedBP)))
	}
	s.modal.AddChild(allBtn)

	actionLabel := "▲ Beam Up"
	if ship {
		actionLabel = "▼ Beam Down"
	}
	s.actionBtn = minui.NewButton("si_action", actionLabel)
	s.actionBtn.SetPosition(siActionBtnX, siActionY+22)
	s.actionBtn.SetSize(siActionBtnW, siActionBtnH)
	s.actionBtn.OnClick = s.performAction
	s.modal.AddChild(s.actionBtn)

	// Relocate is only meaningful for on-site containers — the ship hold can
	// only "beam down" to a site, which doesn't fit the in-level move flow.
	if !ship && s.OnBeginRelocate != nil {
		s.relocateBtn = minui.NewButton("si_relocate", "↔ Relocate")
		s.relocateBtn.SetPosition(siRelocateX, siActionY+22)
		s.relocateBtn.SetSize(siRelocateW, siActionBtnH)
		s.relocateBtn.OnClick = s.beginRelocate
		s.modal.AddChild(s.relocateBtn)
	}

	// ── Status + close ────────────────────────────────────────────────────
	s.statusLbl = minui.NewLabel("si_status", "")
	s.statusLbl.SetPosition(siFilterColX, siStatusY)
	s.statusLbl.SetSize(siModalW-24, 20)
	s.statusLbl.SetColor(color.RGBA{230, 160, 90, 255})
	s.modal.AddChild(s.statusLbl)

	closeBtn := minui.NewButton("si_close", "Close")
	closeBtn.SetPosition((siModalW-siCloseBtnW)/2, siModalH-siCloseBtnH-12)
	closeBtn.SetSize(siCloseBtnW, siCloseBtnH)
	closeBtn.OnClick = func() { s.Visible = false }
	s.modal.AddChild(closeBtn)

	// Populate listboxes.
	s.refreshFilterList()
	s.refreshAllowedList()
	s.refreshInventory()
	s.updateActionPanel()
}

// refreshFilterList rebuilds the left tag-filter listbox from current tags.
func (s *StorageInspectorModal) refreshFilterList() {
	tags := s.collectSourceTags()
	items := []string{"All"}
	values := []string{""}
	for _, t := range tags {
		items = append(items, capitalise(t))
		values = append(values, t)
	}
	s.filterValues = values
	s.filterList.SetItems(items)
	// Restore selection by value.
	for i, v := range values {
		if v == s.activeTagFilter {
			s.filterList.SelectedIndex = i
			break
		}
	}
}

// refreshAllowedList rebuilds the right listbox showing eligible tags with
// ✓-prefix for the ones currently in the container's FilterTags.
func (s *StorageInspectorModal) refreshAllowedList() {
	sc := s.sourceStorage()
	if sc == nil {
		s.allowedList.SetItems(nil)
		return
	}
	tags := s.allowedTagOptions()
	items := make([]string, 0, len(tags))
	for _, t := range tags {
		mark := "    "
		for _, ft := range sc.FilterTags {
			if ft == t {
				mark = "✓  "
				break
			}
		}
		items = append(items, mark+capitalise(t))
	}
	s.allowedTags = tags
	s.allowedList.SetItems(items)
	s.allowedList.SelectedIndex = -1
}

// refreshInventory rebuilds the middle listbox from source contents, applying
// the active tag filter. Preserves the selectedBP across rebuilds.
func (s *StorageInspectorModal) refreshInventory() {
	sc := s.sourceStorage()
	if sc == nil {
		s.invList.SetItems(nil)
		s.invBPs = nil
		return
	}
	type entry struct {
		bp   string
		name string
		qty  int
		tags []string
	}
	entries := []entry{}
	seen := map[string]bool{}
	for _, item := range sc.Items {
		if !item.HasComponent(components.Material) {
			continue
		}
		if seen[item.Blueprint] {
			continue
		}
		seen[item.Blueprint] = true
		mc := item.GetComponent(components.Material).(*components.MaterialComponent)
		if s.activeTagFilter != "" && !mc.HasTag(s.activeTagFilter) {
			continue
		}
		entries = append(entries, entry{
			bp:   item.Blueprint,
			name: displayMaterialName(item.Blueprint),
			qty:  sc.CountResource(item.Blueprint),
			tags: mc.Tags,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	items := make([]string, 0, len(entries))
	bps := make([]string, 0, len(entries))
	for _, e := range entries {
		items = append(items, fmt.Sprintf("%-20s × %d", e.name, e.qty))
		bps = append(bps, e.bp)
	}
	if len(items) == 0 {
		items = []string{"(nothing matches this filter)"}
		bps = []string{""}
	}
	s.invBPs = bps
	s.invList.SetItems(items)
	// Restore selection by blueprint.
	for i, bp := range bps {
		if bp != "" && bp == s.selectedBP {
			s.invList.SelectedIndex = i
			return
		}
	}
	s.selectedBP = ""
}

// updateActionPanel refreshes the selected-material label and enables/disables
// the action buttons.
func (s *StorageInspectorModal) updateActionPanel() {
	if s.selectedBP == "" {
		s.selectedLbl.Text = "Select a material to transfer."
		s.actionBtn.SetEnabled(false)
		if s.relocateBtn != nil {
			s.relocateBtn.SetEnabled(false)
		}
		return
	}
	sc := s.sourceStorage()
	qty := 0
	tagsStr := ""
	if sc != nil {
		qty = sc.CountResource(s.selectedBP)
	}
	if e, err := factory.Create(s.selectedBP, 0, 0, 0); err == nil && e.HasComponent(components.Material) {
		mc := e.GetComponent(components.Material).(*components.MaterialComponent)
		tagsStr = strings.Join(mc.Tags, ", ")
	}
	label := fmt.Sprintf("Selected: %s — %d available", displayMaterialName(s.selectedBP), qty)
	if tagsStr != "" {
		label += "  (" + tagsStr + ")"
	}
	s.selectedLbl.Text = label
	s.actionBtn.SetEnabled(true)
	if s.relocateBtn != nil {
		s.relocateBtn.SetEnabled(qty > 0)
	}
}

// beginRelocate hands off to MainState's relocate flow with the current
// selection and amount. Closes the inspector so the player can see the map
// and pick a destination tile / container.
func (s *StorageInspectorModal) beginRelocate() {
	if s.selectedBP == "" || s.OnBeginRelocate == nil {
		return
	}
	sc := s.sourceStorage()
	if sc == nil {
		return
	}
	avail := sc.CountResource(s.selectedBP)
	if avail <= 0 {
		s.setStatus("Nothing to relocate.")
		return
	}
	q := parseSIQty(s.amountInput.Text)
	if q <= 0 {
		s.setStatus("Enter an amount first (or click All).")
		return
	}
	if q > avail {
		s.setStatus(fmt.Sprintf("Only %d available.", avail))
		return
	}
	source, bp := s.source, s.selectedBP
	s.amountInput.SetText("")
	s.Visible = false
	s.OnBeginRelocate(source, bp, q)
}

// toggleFilterTag flips a tag's presence in the source's FilterTags.
func (s *StorageInspectorModal) toggleFilterTag(tag string) {
	sc := s.sourceStorage()
	if sc == nil {
		return
	}
	for i, t := range sc.FilterTags {
		if t == tag {
			sc.FilterTags = append(sc.FilterTags[:i], sc.FilterTags[i+1:]...)
			return
		}
	}
	sc.FilterTags = append(sc.FilterTags, tag)
	sort.Strings(sc.FilterTags)
}

// performAction beams the selected material in the source's direction.
func (s *StorageInspectorModal) performAction() {
	if s.selectedBP == "" {
		s.setStatus("Select a material first.")
		return
	}
	q := parseSIQty(s.amountInput.Text)
	if q <= 0 {
		s.setStatus("Enter a positive whole number.")
		return
	}
	bp := s.selectedBP
	var err error
	if s.actionIsDown {
		err = s.wm.BeamResourceDown(bp, q)
	} else {
		err = s.wm.BeamResourceUpFrom(s.source, bp, q)
	}
	if err != nil {
		s.setStatus(err.Error())
		return
	}
	if s.actionIsDown {
		s.setStatus(fmt.Sprintf("Beamed %d %s down.", q, displayMaterialName(bp)))
	} else {
		s.setStatus(fmt.Sprintf("Beamed %d %s up.", q, displayMaterialName(bp)))
	}
	s.amountInput.SetText("")
	// Refresh views: inventory contents and possibly tag options shrank.
	s.refreshFilterList()
	s.refreshInventory()
	s.updateActionPanel()
}

func (s *StorageInspectorModal) setStatus(msg string) {
	if s.statusLbl != nil {
		s.statusLbl.Text = msg
		s.statusLbl.Layout()
	}
}

func (s *StorageInspectorModal) Update() {
	if !s.Visible {
		return
	}
	s.modal.Update()
}

func (s *StorageInspectorModal) Draw(screen *ebiten.Image) {
	if !s.Visible {
		return
	}
	s.modal.Draw(screen)
}

// displayMaterialName converts blueprint IDs like "metal_ore" → "Metal Ore".
func displayMaterialName(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// capitalise upper-cases the first letter of a string.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseSIQty parses a positive integer from s, returning 0 on any failure.
func parseSIQty(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || v <= 0 {
		return 0
	}
	return v
}
