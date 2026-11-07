package campaign

import (
	"math/rand"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/objective"
)

// History-backed quests are bound when a site is charted, from its history:
// find a named figure's log, recover a civ's relics, reach where the site's
// threat began. Each is about one history event with an origin; its objective
// is that event becoming known (its lore datapad picked up, or a colonist at
// its origin), and completing it reveals the event as every quest does.
//
// They draw from the site's own stream (its seed) and take IDs from the
// location, so binding them moves none of the campaign's own rolls: a seed's
// maps, positions and names stay as they were.

// historyQuestChance is the percent chance a charted site with something to
// tell gets one history quest.
const historyQuestChance = 60

// historyQuestSalt keeps the quest stream apart from the site's history stream.
const historyQuestSalt int64 = 0x48517565 // "HQue"

// KnownFlag is the evaluation flag a history quest waits on: set while the
// event is known.
func KnownFlag(eventID string) string { return "known:" + eventID }

// historyBinding is one way a template can bind on a site.
type historyBinding struct {
	event string
	fills map[string]string
}

// historyBindings lists every way template t binds on site, in event order.
func (c *Campaign) historyBindings(lib *history.Library, t *questTemplate, site *history.SiteHistory, loc *Location) []historyBinding {
	h := c.History
	var out []historyBinding
	for _, id := range site.Events {
		ev := h.Event(id)
		def := lib.Events[ev.Type]
		if ev == nil || def == nil || ev.Origin == nil {
			continue
		}
		civ := ""
		if len(ev.Civs) > 0 {
			if cv := h.Civs[ev.Civs[0]]; cv != nil {
				civ = cv.Name
			}
		}
		place := def.Place
		if place == "" {
			place = "where it happened"
		}
		fills := map[string]string{"site": loc.Name, "civ": civ, "place": place}
		switch t.History {
		case "figure_log":
			if len(ev.Figures) == 0 || civ == "" {
				continue
			}
			f := h.Figures[ev.Figures[0]]
			role := strings.ReplaceAll(f.Role, "_", " ")
			fills["figure"], fills["role"] = f.Name, role
			fills["Role"] = strings.ToUpper(role[:1]) + role[1:]
		case "civ_relic":
			if civ == "" || !history.Has(def.Effects.AddRemains, "relics") {
				continue
			}
		case "chapter_origin":
			if id != site.Chapter {
				continue
			}
			fills["omen"] = def.Omen
		default:
			continue
		}
		out = append(out, historyBinding{event: id, fills: fills})
	}
	return out
}

// addHistoryQuest gives a freshly charted site at most one history quest.
func (c *Campaign) addHistoryQuest(loc *Location) {
	site := c.SiteHistory(loc)
	if site == nil {
		return
	}
	lib, err := history.DefaultLibrary()
	if err != nil {
		return
	}
	_, qt, err := loadGenTemplates()
	if err != nil {
		return
	}
	type cand struct {
		t *questTemplate
		b []historyBinding
	}
	var cands []cand
	total := 0
	for i := range qt.Templates {
		t := &qt.Templates[i]
		if t.History == "" {
			continue
		}
		if b := c.historyBindings(lib, t, site, loc); len(b) > 0 {
			w := t.Weight
			if w <= 0 {
				w = 1
			}
			cands = append(cands, cand{t, b})
			total += w
		}
	}
	if len(cands) == 0 {
		return
	}
	rng := rand.New(rand.NewSource(loc.Seed ^ historyQuestSalt))
	if rng.Intn(100) >= historyQuestChance {
		return
	}
	r := rng.Intn(total)
	var pick cand
	for _, cd := range cands {
		w := cd.t.Weight
		if w <= 0 {
			w = 1
		}
		if r < w {
			pick = cd
			break
		}
		r -= w
	}
	b := pick.b[rng.Intn(len(pick.b))]
	q := Quest{
		ID:          "hq_" + loc.ID,
		Name:        fillHistory(pick.t.Name, b.fills) + " — " + loc.Name,
		Description: fillHistory(pick.t.Desc, b.fills),
		Location:    loc.ID,
		EventID:     b.event,
		Objective:   objective.Rule{Trigger: objective.TriggerFlagSet, Flag: KnownFlag(b.event)},
		Reward:      QuestReward{Fuel: pick.t.RewardFuelFlat, Resources: pick.t.RewardRes},
	}
	c.QuestDefs = append(c.QuestDefs, q)
}

// fillHistory replaces {key} with fills[key].
func fillHistory(s string, fills map[string]string) string {
	keys := make([]string, 0, len(fills))
	for k := range fills {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, "{"+k+"}", fills[k])
	}
	return strings.TrimSpace(strings.NewReplacer(pairs...).Replace(s))
}

// withKnownFlags adds a known:<event> flag for every known event to an
// evaluation context, so history quests see what the player has discovered.
func (c *Campaign) withKnownFlags(ctx objective.EvalContext) objective.EvalContext {
	if c.History == nil || len(c.History.Known) == 0 {
		return ctx
	}
	flags := make(map[string]any, len(ctx.Flags)+len(c.History.Known))
	for k, v := range ctx.Flags {
		flags[k] = v
	}
	for id := range c.History.Known {
		flags[KnownFlag(id)] = true
	}
	ctx.Flags = flags
	return ctx
}
