package history

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// FormatDate renders a star date for the chronicle: prehistory counts back from
// the campaign's start ("landfall"), play counts forward.
func FormatDate(d StarDate) string {
	switch {
	case d >= 0:
		return fmt.Sprintf("Star date %d", int(d))
	case d > -365:
		return fmt.Sprintf("%d days before landfall", int(-d))
	case d > -730:
		return "A year before landfall"
	default:
		return fmt.Sprintf("%d years before landfall", int(-d)/365)
	}
}

// Render fills an event's text template from its civs, figures and vars.
// siteName names the planet ("" renders as "this world").
func (h *History) Render(lib *Library, ev *Event, siteName string) string {
	def := lib.Events[ev.Type]
	if def == nil {
		return fmt.Sprintf("%s: %s", FormatDate(ev.Date), ev.Type)
	}
	if siteName == "" {
		siteName = ev.Vars["place"]
	}
	if siteName == "" {
		siteName = "this world"
	}
	fills := []string{"{date}", FormatDate(ev.Date), "{site}", siteName}
	civName := func(i int) string {
		if i < len(ev.Civs) {
			if c := h.Civs[ev.Civs[i]]; c != nil {
				return c.Name
			}
		}
		return "unknown"
	}
	fills = append(fills, "{civ}", civName(0), "{enemy}", civName(1))
	during := ""
	if ev.Type != WarEventType && len(ev.Civs) > 0 {
		if w := h.WarAt(ev.Civs[0], ev.Date); w != nil {
			during = fmt.Sprintf(", during the %s–%s war", h.civName0(w, 0), h.civName0(w, 1))
		}
	}
	fills = append(fills, "{during_war}", during)
	warEnd := ""
	if ev.Type == WarEventType {
		if end := WarEnd(ev); end == 0 {
			warEnd = "; it is still being fought"
		} else {
			warEnd = "; it ended " + strings.ToLower(FormatDate(end)[:1]) + FormatDate(end)[1:]
		}
	}
	fills = append(fills, "{war_end}", warEnd)
	for _, id := range ev.Figures {
		if f := h.Figures[id]; f != nil {
			fills = append(fills, "{"+f.Role+"}", f.Name)
		}
	}
	for k, v := range ev.Vars {
		fills = append(fills, "{"+k+"}", v)
	}
	return fixArticles(strings.NewReplacer(fills...).Replace(def.Text))
}

var articleBeforeVowel = regexp.MustCompile(`\b([Aa]) ([AEIOU])`)

// fixArticles turns "a Ixithvex" into "an Ixithvex" once names are filled in.
func fixArticles(s string) string {
	return articleBeforeVowel.ReplaceAllString(s, "${1}n $2")
}

func (h *History) civName0(ev *Event, i int) string {
	if i < len(ev.Civs) {
		if c := h.Civs[ev.Civs[i]]; c != nil {
			return c.Name
		}
	}
	return "unknown"
}

// Summary is a site's one line for the Star Map: its arc's summary, then its
// chapter's omen. "" if the arc has none.
func (h *History) Summary(lib *Library, site *SiteHistory) string {
	if site == nil {
		return ""
	}
	arc := lib.Arcs[site.Arc]
	if arc == nil || arc.Summary == "" {
		return ""
	}
	civ := "unknown"
	if c := h.Civs[site.Civ]; c != nil {
		civ = c.Name
	}
	out := fixArticles(strings.ReplaceAll(arc.Summary, "{civ}", civ))
	if site.Chapter != "" {
		if ev := h.Event(site.Chapter); ev != nil {
			if def := lib.Events[ev.Type]; def != nil && def.Omen != "" {
				out += " " + def.Omen
			}
		}
	}
	return out
}

// SiteEvidence is what a site's history adds to generation: the evidence
// features of its events (anchored ones keyed to their event's origin), the
// rolled origins by event ID, the chapter event's ID, and the events that leave
// a lore datapad (every event with an origin).
type SiteEvidence struct {
	Features []mapdef.FeatureBlock
	Origins  map[string][2]int
	Chapter  string
	Datapads []string
}

// Evidence collects a site's generation inputs, in event order.
func (h *History) Evidence(lib *Library, site *SiteHistory) SiteEvidence {
	var out SiteEvidence
	if site == nil {
		return out
	}
	out.Chapter = site.Chapter
	for _, id := range site.Events {
		ev := h.Event(id)
		def := lib.Events[ev.Type]
		if ev == nil || def == nil {
			continue
		}
		if ev.Origin != nil {
			if out.Origins == nil {
				out.Origins = map[string][2]int{}
			}
			out.Origins[ev.ID] = [2]int{ev.Origin.X, ev.Origin.Y}
			out.Datapads = append(out.Datapads, ev.ID)
		}
		for _, e := range def.Evidence {
			if e.Kind == "lore_datapad" {
				continue // every event with an origin leaves one (Datapads)
			}
			fb := e.FeatureBlock
			if fb.At == "origin" {
				if ev.Origin == nil {
					continue
				}
				fb.At = "origin:" + ev.ID
			}
			out.Features = append(out.Features, fb)
		}
	}
	return out
}

// LoreFlags are the site's history as setup-script flags: the chapter's event
// type and civ name, and what the history left (has_wreck, has_reactor,
// population). The resolved origin tile is added by the game once the map is
// built (origin_x, origin_y, origin_z). Empty strings and 0 for a site with no
// chapter or civ.
func (h *History) LoreFlags(lib *Library, site *SiteHistory) map[string]any {
	f := map[string]any{
		"origin_event": "",
		"origin_civ":   "",
		"has_wreck":    0.0,
		"has_reactor":  0.0,
		"population":   PopNever,
	}
	if site == nil {
		return f
	}
	f["population"] = site.State.Population
	if Has(site.State.Remains, "wrecks") {
		f["has_wreck"] = 1.0
	}
	if Has(site.State.Remains, "reactor") {
		f["has_reactor"] = 1.0
	}
	civ := site.Civ
	if site.Chapter != "" {
		if ev := h.Event(site.Chapter); ev != nil {
			f["origin_event"] = ev.Type
			if len(ev.Civs) > 0 {
				civ = ev.Civs[0]
			}
			if def := lib.Events[ev.Type]; def != nil {
				for k, v := range def.Flags {
					f[k] = v
				}
			}
		}
	}
	if c := h.Civs[civ]; c != nil {
		f["origin_civ"] = c.Name
	}
	return f
}
