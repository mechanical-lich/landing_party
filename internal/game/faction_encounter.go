package game

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/state"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// Interception on the game side (phase D; docs/developer/factions.md,
// "Interception"). Travel looks for a ship the jump meets (the campaign's
// DetectInterception) before the jump's days pass; on arrival the encounter
// waits for an answer in a popup that stops the clock, like an incident.
// Boarding builds the ship's deck as an encounter location and takes the
// player into orbit of it; the fight is an ordinary landing.

// noteInterception records the encounter a jump met, if none is pending,
// and announces it. Observer mode answers Evade at once.
func (wm *WorldManager) noteInterception(e *campaign.Encounter) {
	c := wm.Campaign
	if e == nil || c.Encounter.Pending() {
		return
	}
	c.Encounter = e
	d := campaign.LoadFactionData().Doctrine(e.Doctrine)
	if d == nil {
		c.Encounter = nil
		return
	}
	message.PostMessage("Alert", encounterText(c, e))
	log.Printf("[faction] interception: %s (%s) near %s", e.FleetID, e.At, e.LocationID)
	if shiplessRun() {
		// An observer run has no player ship to meet one (only Travel
		// detects interceptions); kept as a guard: Evade at once, never a
		// popup addressed to a player who is not there.
		if _, err := wm.answerEncounter(campaign.AnswerEvade); err != nil {
			log.Printf("[faction] observer evade: %v", err)
			c.Encounter = nil
		}
		message.PostMessage("Observer", "Interception: evaded.")
	}
}

// encounterText is the popup's line: what was met, and where.
func encounterText(c *campaign.Campaign, e *campaign.Encounter) string {
	d := campaign.LoadFactionData().Doctrine(e.Doctrine)
	if d == nil {
		return "A ship."
	}
	what := shipWord(d, e)
	where := ""
	if loc := c.Locations[e.LocationID]; loc != nil {
		where = loc.Name
	}
	if e.At == "orbit" {
		return fmt.Sprintf("A %s %s waits in orbit of %s.", d.MorphAt(nil).Singular, what, where)
	}
	return fmt.Sprintf("A %s %s crosses your path on the way to %s.", d.MorphAt(nil).Singular, what, where)
}

// shipWord is the ship's class: its tier's name, "mothership", or "ship".
func shipWord(d *campaign.Doctrine, e *campaign.Encounter) string {
	switch {
	case e.Mothership:
		return "mothership"
	case d.TierName(e.Tier) != "":
		return d.TierName(e.Tier)
	}
	return "ship"
}

// AnswerEncounter answers the pending encounter: an evasion takes its fuel
// from the hold (what there is); a boarding (or a failed evasion) travels
// to the boarded ship's deck at no cost and returns its level to switch to.
func (wm *WorldManager) AnswerEncounter(answer string) (*MainState, error) {
	boarded, err := wm.answerEncounter(answer)
	if err != nil || !boarded {
		return nil, err
	}
	return wm.EnterCurrent()
}

// answerEncounter is AnswerEncounter without entering the boarded deck:
// it reports whether the ship is now in orbit of one (observer mode stays
// where the viewer is).
func (wm *WorldManager) answerEncounter(answer string) (bool, error) {
	c := wm.Campaign
	e, err := c.AnswerEncounter(answer)
	if err != nil {
		return false, err
	}
	if e.FuelCost > 0 {
		hold := storage.ShipProvider{Level: wm.ShipLevel()}
		owners := []string{wm.shipSettlementName()}
		have := storage.CountResource(hold, owners, "fuel")
		storage.Deduct(hold, owners, map[string]int{"fuel": min(have, e.FuelCost)})
	}
	switch e.Outcome {
	case "evaded":
		message.PostMessage("Star Map", fmt.Sprintf("Evaded, at %d fuel.", e.FuelCost))
		return false, nil
	case "parleyed":
		message.PostMessage("Star Map", "They let you pass.")
		return false, nil
	case "caught":
		message.PostMessage("Alert", "They caught you: prepare to board.")
	}
	if e.BoardingID == "" {
		return false, nil
	}
	if err := wm.Travel(e.BoardingID); err != nil {
		return false, err
	}
	return true, nil
}

// boardingCrew are the blueprints that fight aboard a boarded ship of
// doctrine d: its crew blueprints' fighting forms ("greenish_crew" fights
// as the scenario's "greenish"), or the doctrine's boarders.
func boardingCrew(d *campaign.Doctrine, n int) []string {
	var out []string
	if d.Settlement != nil {
		for _, bp := range d.Settlement.CrewBlueprints(n) {
			out = append(out, strings.TrimSuffix(bp, "_crew"))
		}
	}
	return out
}

// materialiseBoarding builds a boarded ship's deck, once: the ship's hull
// (crashed_ship by its class, docked: no furrow, with breaches to board
// through) beside the docking deck, its crew armed with their own AI and
// tagged as the fleet's, and its hold (a mothership's hoard) in a locker
// aboard as loot.
func materialiseBoarding(c *campaign.Campaign, loc *campaign.Location, level *world.Level) {
	b := loc.Boarding
	if b == nil || b.Built || level == nil {
		return
	}
	fd := campaign.LoadFactionData()
	d := fd.Doctrine(b.Doctrine)
	if d == nil {
		return
	}
	var f *campaign.Fleet
	for i := range c.Fleets {
		if c.Fleets[i].ID == b.FleetID {
			f = &c.Fleets[i]
		}
	}
	cx, cy, cz := level.GetWidth()/2+6, level.GetHeight()/2, level.SurfaceZ
	if pts := level.Regions["boarding_ship"]; len(pts) > 0 {
		cx, cy, cz = pts[0][0], pts[0][1], pts[0][2]
	}
	w, h := 29, 17
	params := map[string]any{"class": b.Class, "damage": "1", "crew": "dead", "frame": "station", "z": cz}
	if b.Class == "scout" {
		w, h = 15, 11
	}
	if d.Ships != nil && d.Ships.Marker == "saucer" && b.Class != "mothership" {
		params["class"], w, h = "saucer", 17, 17
	}
	if b.Class == "mothership" {
		w, h = 31, 21
		params["class"] = "warship" // a mothership's deck: the largest hull
	}
	stampAt(level, "crashed_ship", cx, cy, w, h, params, loc.Seed)
	presenceID := b.FleetID
	if f != nil {
		presenceID = f.Presence.ID
	}
	if isGreyishDoctrine(d) && f != nil {
		boardGreyCrew(level, d, f, b, presenceID, cx, cy, cz)
	}
	for i, bp := range boardingCrew(d, b.Crew) {
		if isGreyishDoctrine(d) && f != nil {
			break // typed above
		}
		x, y, z := freeLandingTile(level, cx+(i%3)-1, cy+(i/3)%3-1, cz)
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			log.Printf("[faction] boarding crew %s: %v", bp, err)
			continue
		}
		if k := components.KinOf(e); k != nil {
			k.Presence = presenceID
		} else {
			e.AddComponent(&components.KinComponent{Morph: "crew", Presence: presenceID})
		}
		armBoarder(e, d, b.Faction) // the ship's faction, never its doctrine (a rival colony's is "mirror")
		level.AddEntity(e)
	}
	if f != nil && len(f.Presence.Stockpile) > 0 {
		x, y, z := freeLandingTile(level, cx, cy, cz)
		if locker, err := factory.Create("storage_locker", x, y, z); err == nil {
			level.AddEntity(locker)
			sc := locker.GetComponent(components.Storage).(*components.StorageComponent)
			keys := make([]string, 0, len(f.Presence.Stockpile))
			for k := range f.Presence.Stockpile {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if it, err := factory.Create(k, 0, 0, 0); err == nil {
					if it.HasComponent(components.Material) {
						it.GetComponent(components.Material).(*components.MaterialComponent).Quantity = f.Presence.Stockpile[k]
					}
					sc.AddItem(it)
				}
			}
		}
	}
	b.Built = true
}

// armBoarder gives a boarded ship's crew member with no AI of its own (the
// mirror's colonists: their fighting form is the colonist blueprint, which
// the colonist AI drives only as a settlement's worker) the humanoid fighting
// script under the ship's faction, hostile: boarded, it fights back (phase
// E, closing D's gap). A crew member with its own script keeps it.
func armBoarder(e *ecs.Entity, d *campaign.Doctrine, faction string) {
	if e.HasComponent(components.ScriptedAI) {
		return
	}
	// A worker of no settlement stands idle: aboard it is a fighter.
	if e.HasComponent(components.Worker) && !e.HasComponent(components.Settlement) {
		e.RemoveComponent(components.Worker)
	}
	if e.HasComponent(rlcomponents.Description) {
		e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction = faction
	}
	e.AddComponent(&components.ScriptedAIComponent{Script: boarderScript, Vars: map[string]any{"ignored_factions": ""}})
	if !e.HasComponent(rlcomponents.AIMemory) {
		e.AddComponent(&rlcomponents.AIMemoryComponent{})
	}
	seedHostility(e, d, true)
	// Cornered aboard their own ship, they hunt the boarders (the script
	// hunts from 7), whatever the doctrine's own temper.
	if sc := e.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent); toFloat(sc.Vars["hostility"]) < boarderHostility {
		sc.Vars["hostility"] = float64(boarderHostility)
	}
}

const boarderHostility = 8

func toFloat(v any) float64 {
	f, _ := v.(float64)
	return f
}

// boarderScript is the fighting script a boarder without its own gets (the
// enforcer's and the scenario humanoids').
const boarderScript = "data/scripts/ai/faction_humanoid.basic"

// boardingAlive is the boarded ship's crew still alive on its deck.
func boardingAlive(c *campaign.Campaign, loc *campaign.Location, level *world.Level) int {
	b := loc.Boarding
	id := b.FleetID
	for i := range c.Fleets {
		if c.Fleets[i].ID == b.FleetID {
			id = c.Fleets[i].Presence.ID
		}
	}
	return len(presenceBodies(level, id))
}

// reconcileBoarding counts a boarded ship's crew back (on every write of
// its level and once a day): none left and the ship is taken.
func reconcileBoarding(c *campaign.Campaign, loc *campaign.Location, ms *MainState) bool {
	if loc.Boarding == nil || ms == nil || ms.level == nil {
		return false
	}
	return c.ReconcileBoarding(loc, boardingAlive(c, loc, ms.level))
}

// stepBoardings checks every loaded boarded deck once a day: a ship whose
// crew is all dead is taken (an alert on the deck in orbit).
func (wm *WorldManager) stepBoardings() {
	ids := make([]string, 0, len(wm.loaded))
	for id := range wm.loaded {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		loc := wm.Campaign.Locations[id]
		if loc == nil || loc.Boarding == nil {
			continue
		}
		if reconcileBoarding(wm.Campaign, loc, wm.loaded[id]) && id == wm.currentID {
			message.PostMessage("Alert", fmt.Sprintf("The %s is taken: its crew is dead and its hold is yours.", loc.Name))
		}
	}
}

// ---- the popup ----

// encounterPopup is the Evade / Board / Parley modal over the campaign's
// pending encounter. Hosted where the incident popup is.
type encounterPopup struct {
	wm      *WorldManager
	modal   *minui.Modal
	label   *minui.Label
	detail  *minui.Label
	errLine *minui.Label
	evade   *minui.Button
	parley  *minui.Button
	pending state.StateInterface
}

func newEncounterPopup(wm *WorldManager) *encounterPopup {
	cfg := config.Global()
	const w, h = 520, 180
	p := &encounterPopup{wm: wm, modal: minui.NewModal("encounter_popup", "Contact", w, h)}
	p.modal.SetPosition((cfg.ScreenWidth-w)/2, (cfg.ScreenHeight-h)/2)
	p.modal.Closeable = false
	p.label = minui.NewLabel("encounter_label", "")
	p.label.SetPosition(20, 14)
	p.modal.AddChild(p.label)
	p.detail = minui.NewLabel("encounter_detail", "")
	p.detail.SetPosition(20, 36)
	p.modal.AddChild(p.detail)
	p.errLine = minui.NewLabel("encounter_error", "")
	p.errLine.SetPosition(20, 58)
	p.modal.AddChild(p.errLine)
	button := func(id, text string, x int, onClick func()) *minui.Button {
		b := minui.NewButton(id, text)
		b.SetPosition(x, 90)
		b.SetSize(150, 30)
		b.OnClick = onClick
		p.modal.AddChild(b)
		return b
	}
	answer := func(a string) func() {
		return func() {
			ms, err := wm.AnswerEncounter(a)
			if err != nil {
				p.errLine.Text = err.Error()
				return
			}
			if ms != nil {
				p.pending = ms
			}
		}
	}
	p.evade = button("encounter_evade", "Evade", 20, answer(campaign.AnswerEvade))
	button("encounter_board", "Board", 185, answer(campaign.AnswerBoard))
	p.parley = button("encounter_parley", "Parley", 350, answer(campaign.AnswerParley))
	return p
}

func (p *encounterPopup) update() state.StateInterface {
	c := p.wm.Campaign
	e := c.Encounter
	if !e.Pending() {
		p.modal.SetVisible(false)
		p.errLine.Text = ""
		t := p.pending
		p.pending = nil
		return t
	}
	p.modal.SetVisible(true)
	p.label.Text = encounterText(c, e)
	p.evade.Text = fmt.Sprintf("Evade (%d fuel)", e.EvadeFuel())
	if c.CanParley(e) {
		p.parley.SetVisible(true)
		p.detail.Text = fmt.Sprintf("They are %s.", c.Stance(e.Faction))
	} else {
		p.parley.SetVisible(false)
		p.detail.Text = "They are hostile: they will not talk."
	}
	p.modal.Update()
	t := p.pending
	p.pending = nil
	return t
}

func (p *encounterPopup) draw(screen *ebiten.Image) {
	if !p.wm.Campaign.Encounter.Pending() {
		return
	}
	p.modal.Draw(screen)
}

