package game

import (
	"image/color"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/mlge/state"
	mlge_text "github.com/mechanical-lich/mlge/text"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// legendsPanel is the Legends tab: the chronicle of every history event the
// player knows (History.Known), oldest first, rendered from the same templates
// as the history dump. The list on the left filters it by site, civ or figure.
//
// It is its own tab rather than an Encyclopedia section: a chronicle with three
// filters needs the whole page, and the Encyclopedia tab is ungated in the
// dashboard, so Legends is too.
type legendsPanel struct {
	campaign *campaign.Campaign
	filters  *minui.ListBox // scrolls with the wheel past the panel, like the Encyclopedia's catalogue
	keys     []string       // parallel to the filter rows: "", "site:<id>", "civ:<id>", "fig:<id>"
	lines    []string       // the chronicle's events, for the state summary
	// text is the chronicle: the same scrolling text area as the HUD's message
	// log (wheel and scrollbar). It is rebuilt only when shown is stale, so a
	// rebuild opens at the newest events and the wheel stays where the
	// player put it otherwise.
	text  *minui.ScrollingTextArea
	shown string // filter key and known count the text was built for
}

func newLegendsPanel(c *campaign.Campaign) *legendsPanel {
	p := &legendsPanel{campaign: c}
	cfg := config.Global()
	p.filters = minui.NewListBox("dash_legends_filters", nil)
	listY := dashContentTop + 78
	p.filters.SetBounds(minui.Rect{X: 40, Y: listY, Width: 320, Height: cfg.ScreenHeight - listY - 40})
	p.filters.Layout()
	x, y := 400, dashContentTop+54
	p.text = minui.NewScrollingTextArea("dash_legends_chronicle", cfg.ScreenWidth-x-40, cfg.ScreenHeight-y-40)
	p.text.SetPosition(x, y)
	p.text.LineHeight = 20
	p.text.Layout()
	p.populate()
	return p
}

// Enter opens the tab at the newest events.
func (p *legendsPanel) Enter() {
	p.shown = ""
	p.populate()
}

func (p *legendsPanel) Update() {
	p.filters.Update()
	p.refreshText()
	p.text.Update()
}

// refreshText rebuilds the chronicle text when the filter or the known events
// changed, scrolled to the bottom (the newest).
func (p *legendsPanel) refreshText() {
	key := ""
	if i := p.filters.SelectedIndex; i >= 0 && i < len(p.keys) {
		key = p.keys[i]
	}
	sig := key + "|" + itoa(len(p.known())) + "|" + itoa(len(p.campaign.DatapadsRead))
	if sig == p.shown {
		return
	}
	p.shown = sig
	p.lines = p.chronicle()
	p.text.Clear()
	if len(p.lines) == 0 && key == datapadsKey {
		p.text.AddText("No datapads read yet.") // DRAFT text: the owner rewrites it
		return
	}
	if len(p.lines) == 0 {
		p.text.AddText("Nothing known yet. Datapads, scans and the places where things happened tell the story.")
		return
	}
	for i, line := range p.lines {
		if i > 0 {
			p.text.AddText(" ")
		}
		p.text.AddText(line)
	}
}

// visible reports which chronicle text lines are on screen and whether the
// view is at the top (oldest) and at the bottom (newest).
func (p *legendsPanel) visible() (lines []string, top, bottom bool) {
	t := p.text
	end := t.ScrollOffset + t.VisibleLines
	if end > len(t.Lines) {
		end = len(t.Lines)
	}
	for _, l := range t.Lines[t.ScrollOffset:end] {
		lines = append(lines, l.Text)
	}
	return lines, t.ScrollOffset == 0, end == len(t.Lines)
}

func (p *legendsPanel) Transition() state.StateInterface { return nil }

// known returns the known events, oldest first.
func (p *legendsPanel) known() []*history.Event {
	h := p.campaign.History
	if h == nil {
		return nil
	}
	var out []*history.Event
	for i := range h.Events {
		if eventShown(h, &h.Events[i]) {
			out = append(out, &h.Events[i])
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// siteName names an event's place: its site's location, else the place a live
// event recorded.
func (p *legendsPanel) siteName(ev *history.Event) string {
	h := p.campaign.History
	if s := h.Sites[ev.Site]; s != nil {
		if loc := p.campaign.Locations[s.Location]; loc != nil {
			return loc.Name
		}
	}
	return ev.Vars["place"]
}

// populate rebuilds the filter list from the known events, keeping the
// selection by key.
func (p *legendsPanel) populate() {
	prev := ""
	if i := p.filters.SelectedIndex; i >= 0 && i < len(p.keys) {
		prev = p.keys[i]
	}
	h := p.campaign.History
	items, keys := []string{"All events"}, []string{""}
	if len(p.campaign.DatapadsRead) > 0 {
		// Every datapad the crew picked up, word for word, to read again.
		items, keys = append(items, "Datapads read" /* DRAFT text */), append(keys, datapadsKey)
	}
	seen := map[string]bool{}
	var sites, civs, figs [][2]string
	for _, ev := range p.known() {
		if ev.Site != "" && !seen["site:"+ev.Site] {
			seen["site:"+ev.Site] = true
			sites = append(sites, [2]string{"site:" + ev.Site, "Site: " + p.siteName(ev)})
		}
		for _, c := range ev.Civs {
			if civ := h.Civs[c]; civ != nil && !seen["civ:"+c] {
				seen["civ:"+c] = true
				civs = append(civs, [2]string{"civ:" + c, "Civ: the " + civ.Name})
			}
		}
		for _, f := range ev.Figures {
			if fig := h.Figures[f]; fig != nil && !seen["fig:"+f] {
				seen["fig:"+f] = true
				figs = append(figs, [2]string{"fig:" + f, "Figure: " + fig.Name})
			}
		}
	}
	for _, group := range [][][2]string{sites, civs, figs} {
		for _, kv := range group {
			keys = append(keys, kv[0])
			items = append(items, kv[1])
		}
	}
	p.keys = keys
	p.filters.SetItems(items)
	p.filters.SelectedIndex = 0
	for i, k := range keys {
		if k == prev {
			p.filters.SelectedIndex = i
		}
	}
	p.refreshText()
}

// chronicle renders the known events the selected filter keeps.
func (p *legendsPanel) chronicle() []string {
	h := p.campaign.History
	lib, err := history.DefaultLibrary()
	if h == nil || err != nil {
		return nil
	}
	key := ""
	if i := p.filters.SelectedIndex; i >= 0 && i < len(p.keys) {
		key = p.keys[i]
	}
	if key == datapadsKey {
		return datapadsReadLines(p.campaign)
	}
	kind, id, _ := strings.Cut(key, ":")
	var out []string
	for _, ev := range p.known() {
		switch kind {
		case "site":
			if ev.Site != id {
				continue
			}
		case "civ":
			if !history.Has(ev.Civs, id) {
				continue
			}
		case "fig":
			if !history.Has(ev.Figures, id) {
				continue
			}
		}
		out = append(out, h.Render(lib, ev, p.siteName(ev)))
	}
	return out
}

func (p *legendsPanel) Draw(screen *ebiten.Image) {
	known := len(p.known())
	mlge_text.Draw(screen, "Known events: "+itoa(known), 14, 40, dashContentTop+20, color.RGBA{160, 185, 215, 255})
	mlge_text.Draw(screen, "Filter", 16, 40, dashContentTop+50, color.RGBA{200, 220, 255, 255})
	p.filters.Draw(screen)
	mlge_text.Draw(screen, "Chronicle", 16, 400, dashContentTop+24, color.RGBA{200, 220, 255, 255})
	p.text.Draw(screen)
}

// datapadsKey is the Legends filter that lists the datapads read.
const datapadsKey = "pads:"

// datapadsReadLines are the datapads the crew has read, oldest first, each
// with its star date and site.
func datapadsReadLines(c *campaign.Campaign) []string {
	var out []string
	for _, d := range c.DatapadsRead {
		where := ""
		if d.Site != "" {
			where = ", " + d.Site
		}
		out = append(out, "Star Date "+itoa(d.Day)+where+": "+d.Text)
	}
	return out
}
