package game

import (
	"fmt"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"image/color"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

// The Factions tab (docs/developer/factions.md, "What the player sees"): one
// block per faction, what it holds at a glance. It exists in both modes
// with different fog: a normal campaign shows what the player has seen
// (the Known flags) under the hidden-identity rule, so a hive's morphs are
// separate blocks named by morph; an observer run shows every presence and
// fleet named by faction, the expedition included as just another
// colony-doctrine faction (faction_view.go). A site row selects the site on
// the Star Map (so View can open it); a fleet row selects the site it is at
// or bound for. Rebuilt on entry and once a star-date day.

type factionRow struct {
	text   string
	col    color.RGBA
	header bool
	locID  string // the site a click selects ("" none)
	detail bool   // a site's second line (a hive's units), not a site
}

type factionsPanel struct {
	c      *campaign.Campaign
	wm     *WorldManager
	d      *DashboardState
	rows   []factionRow
	day    int
	scroll int
	// pending is a transition a click asks for (none: clicks stay on the
	// dashboard).
	pending state.StateInterface
}

func newFactionsPanel(c *campaign.Campaign, wm *WorldManager, d *DashboardState) *factionsPanel {
	return &factionsPanel{c: c, wm: wm, d: d, day: -1}
}

const (
	factionsRowH = 18
	factionsTop  = dashContentTop + 14
)

func (p *factionsPanel) Enter() { p.rebuild() }

func (p *factionsPanel) Transition() state.StateInterface {
	t := p.pending
	p.pending = nil
	return t
}

// factionBlock gathers one faction's holdings.
type factionBlock struct {
	key, name string
	d         *campaign.Doctrine
	pop       int
	techs     map[string]bool
	sites     []factionRow
	fleets    []factionRow
}

// factionKey is how presences group: by faction in an observer run, by the
// morph the player would name it by otherwise (hidden identity).
func factionKey(d *campaign.Doctrine, p *campaign.FactionPresence, loc *campaign.Location) (key, name string) {
	if p.Faction == campaign.ExpeditionFaction {
		return p.Faction, "Expedition"
	}
	if observerRun() {
		return p.Faction, d.Name
	}
	m := d.MorphAt(loc)
	return p.Faction + "/" + m.Name, m.Name
}

// rebuild collects the blocks from the campaign under the viewer's fog.
func (p *factionsPanel) rebuild() {
	c := p.c
	p.day = c.Day()
	fd := campaign.LoadFactionData()
	blocks := map[string]*factionBlock{}
	get := func(d *campaign.Doctrine, pr *campaign.FactionPresence, loc *campaign.Location) *factionBlock {
		key, name := factionKey(d, pr, loc)
		b := blocks[key]
		if b == nil {
			b = &factionBlock{key: key, name: name, d: d, techs: map[string]bool{}}
			blocks[key] = b
		}
		for _, t := range pr.KnownTechs {
			b.techs[t] = true
		}
		return b
	}
	ids := make([]string, 0, len(c.Locations))
	for id := range c.Locations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		loc := c.Locations[id]
		for _, pr := range shownPresences(loc) {
			d := fd.Doctrine(pr.Doctrine)
			if d == nil {
				continue
			}
			b := get(d, pr, loc)
			b.pop += pr.Population
			text := fmt.Sprintf("  %s  ·  %s  ·  pop %d", loc.Name, stageWord(d, pr.Stage), pr.Population)
			if pr.Dormant {
				text = fmt.Sprintf("  %s  ·  signal, wakes day %d", loc.Name, int(pr.WakeDay))
			} else if pr.Conversion > 0 && d.Broods() {
				text += fmt.Sprintf("  ·  %.0f%% resin", 100*pr.Conversion)
			} else if pr.Conversion > 0 {
				text += fmt.Sprintf("  ·  %.0f%% converted", 100*pr.Conversion)
			}
			b.sites = append(b.sites, factionRow{text: text, col: factionBadgeColor(d), locID: loc.ID})
			if units := unitsLine(d, pr, loc); units != "" {
				b.sites = append(b.sites, factionRow{text: "      " + units, col: factionBadgeColor(d), locID: loc.ID, detail: true})
			}
		}
	}
	for _, f := range shownFleets(c) {
		d := fd.Doctrine(f.Presence.Doctrine)
		if d == nil {
			continue
		}
		b := get(d, &f.Presence, c.Locations[f.FromID])
		b.pop += f.Presence.Population
		name := func(id string) string {
			if l := c.Locations[id]; l != nil {
				return l.Name
			}
			return id
		}
		var text, at string
		switch {
		case f.Kind != campaign.FleetShip:
			text = fmt.Sprintf("  egg  ·  %d aboard  ·  to %s, arrives day %d", f.Presence.Population, name(f.ToID), int(f.ArriveDay))
			at = f.ToID
		default:
			kind := "ship"
			if f.Mothership {
				kind = "mothership"
			} else if t := d.TierName(f.ShipTier); t != "" {
				kind = t
			}
			text = fmt.Sprintf("  %s  ·  crew %d  ·  cargo %d", kind, f.Presence.Population, holdUnitsOf(f.Presence.Stockpile))
			switch {
			case f.Landed():
				text += "  ·  landed on " + name(f.LandedID)
				at = f.LandedID
			case f.Mothership || f.FromID == f.ToID:
				text += "  ·  in orbit of " + name(f.ToID)
				at = f.ToID
			default:
				text += fmt.Sprintf("  ·  heading %s, day %d", name(f.ToID), int(f.ArriveDay))
				at = f.ToID
			}
			if f.Presence.Fuel > 0 {
				text += fmt.Sprintf("  ·  fuel %d", f.Presence.Fuel)
			}
		}
		b.fleets = append(b.fleets, factionRow{text: text, col: factionBadgeColor(d), locID: at})
	}
	keys := make([]string, 0, len(blocks))
	for k := range blocks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	p.rows = p.rows[:0]
	for _, k := range keys {
		b := blocks[k]
		col := factionBadgeColor(b.d)
		sites := 0
		for _, r := range b.sites {
			if !r.detail {
				sites++
			}
		}
		head := fmt.Sprintf("%s  —  population %d  ·  techs %d  ·  sites %d  ·  fleets %d",
			strings.ToUpper(b.name), b.pop, len(b.techs), sites, len(b.fleets))
		p.rows = append(p.rows, factionRow{text: head, col: col, header: true})
		p.rows = append(p.rows, b.sites...)
		p.rows = append(p.rows, b.fleets...)
		p.rows = append(p.rows, factionRow{})
	}
	if len(p.rows) == 0 {
		msg := "No faction seen yet."
		if observerRun() {
			msg = "No factions in this campaign."
		}
		p.rows = append(p.rows, factionRow{text: msg, col: dashBodyColor})
	}
}

func holdUnitsOf(stock map[string]int) int {
	n := 0
	for _, v := range stock {
		n += v
	}
	return n
}

// visibleRows is how many rows fit.
func (p *factionsPanel) visibleRows() int {
	return (config.Global().ScreenHeight - factionsTop - 50) / factionsRowH
}

func (p *factionsPanel) Update() {
	if p.c.Day() != p.day {
		p.rebuild()
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		p.scroll -= int(wy * 3)
	}
	p.scroll = max(0, min(p.scroll, len(p.rows)-p.visibleRows()))
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		_, my := ebiten.CursorPosition()
		p.click(my)
	}
}

// click acts on the row under y: a site or fleet row selects its site on
// the Star Map tab.
func (p *factionsPanel) click(y int) {
	if y < factionsTop {
		return
	}
	i := p.scroll + (y-factionsTop)/factionsRowH
	if i < 0 || i >= len(p.rows) || p.rows[i].locID == "" {
		return
	}
	p.selectSite(p.rows[i].locID)
}

// selectSite switches to the Star Map with id selected and centred.
func (p *factionsPanel) selectSite(id string) {
	if p.d == nil {
		return
	}
	sm := p.d.starMapPanel()
	if sm == nil {
		return
	}
	p.d.selectTabByID("starmap")
	p.d.tabBar.SetActive("starmap")
	sm.refreshLocations()
	sm.selectByID(id)
	if loc := p.c.Locations[id]; loc != nil {
		sm.centerOn(loc)
	}
}

func (p *factionsPanel) Draw(screen *ebiten.Image) {
	x := 24
	title := "Factions: what the crew has seen"
	if observerRun() {
		title = "Factions: everything (observer run)"
	}
	mlge_text.Draw(screen, title, 14, x, dashContentTop-2, color.RGBA{160, 185, 215, 255})
	end := min(len(p.rows), p.scroll+p.visibleRows())
	for i := p.scroll; i < end; i++ {
		r := p.rows[i]
		y := factionsTop + (i-p.scroll)*factionsRowH
		if r.header {
			vector.DrawFilledRect(screen, float32(x), float32(y+3), 10, 10, r.col, false)
			mlge_text.Draw(screen, r.text, 14, x+16, y, r.col)
			continue
		}
		col := dashBodyColor
		if r.locID != "" {
			col = color.RGBA{200, 215, 235, 255}
		}
		mlge_text.Draw(screen, r.text, 13, x+16, y, col)
	}
}

// unitsLine is a typed presence's bodies by type for its Factions tab row:
// an observer run names every type (drones, warriors); a normal campaign
// only the site's morph's types, by the names they are seen by ("Red Xeno"),
// so the row never shows the other morph (hidden identity). "" for none.
func unitsLine(d *campaign.Doctrine, p *campaign.FactionPresence, loc *campaign.Location) string {
	if d.Builds() {
		return autonzUnitsLine(d, p)
	}
	if !d.Broods() || len(p.Units) == 0 {
		return ""
	}
	site := d.MorphKeyAt(loc)
	var parts []string
	for _, k := range d.TypeKeys() {
		n, t := p.Units[k], d.Types[k]
		if n <= 0 {
			continue
		}
		if observerRun() {
			parts = append(parts, fmt.Sprintf("%d %s", n, strings.ToLower(t.Name)))
		} else if t.Morph == site {
			parts = append(parts, fmt.Sprintf("%s %d", t.SeenName, n))
		}
	}
	if observerRun() && p.Larvae > 0 {
		parts = append(parts, fmt.Sprintf("%d larvae", p.Larvae))
	}
	return strings.Join(parts, ", ")
}

// autonzUnitsLine is an autonz presence's units by type and its structures
// for its Factions tab row (phase E3; no hidden identity, so both modes):
// "3 wire placers (bronze), 2 enforcers · cogitator, foundry, 1 relay
// pylon". A type marked bronze prints bronze now (steel ones printed before
// stay steel). "" for none.
func autonzUnitsLine(d *campaign.Doctrine, p *campaign.FactionPresence) string {
	var parts []string
	for _, k := range d.TypeKeys() {
		n := p.Units[k]
		if n <= 0 {
			continue
		}
		part := fmt.Sprintf("%d %s", n, strings.ToLower(d.Types[k].Name))
		if t := d.TierOfType(k, p.KnownTechs); len(d.Tiers) > 0 && t != d.Tiers[0].ID {
			part += " (" + t + ")"
		}
		parts = append(parts, part)
	}
	line := strings.Join(parts, ", ")
	var built []string
	for _, k := range autonzStructureKeys(d) {
		if n := p.Built[k]; n > 0 {
			name := strings.ToLower(construction.GetBuildable(k).Name)
			if name == "" {
				name = k
			}
			built = append(built, fmt.Sprintf("%d %s", n, name))
		}
	}
	if len(built) > 0 {
		if line != "" {
			line += "  ·  "
		}
		line += strings.Join(built, ", ")
	}
	return line
}
