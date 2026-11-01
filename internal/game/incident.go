package game

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/mlge/state"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// Incident is colonist damage on a level the player isn't on, waiting for an answer.
type Incident struct {
	LocationID string // "" for The Ship
	Where      string
	Victim     string
}

// Notification is presentation from a background level that would otherwise go to
// the live message log. Count is how many identical lines it stands for.
type Notification struct {
	LocationID string
	Where      string
	Text       string
	Count      int
	tick       int
}

const maxNotifications = 50

// incidentCooldown is how long a planet's colonists must go unhurt, in that planet's
// own ticks, before more damage counts as a new incident.
func incidentCooldown() int { return positiveOr(config.Global().IncidentCooldownTicks, 1800) }

// notificationMerge is how many ticks apart repeated lines still merge into one.
func notificationMerge() int { return positiveOr(config.Global().NotificationMergeTicks, 600) }

// locationLabel names a background level for the player.
func (wm *WorldManager) locationLabel(id string) string {
	if loc := wm.Campaign.Locations[id]; loc != nil {
		return loc.Name
	}
	return "The Ship"
}

// startsIncident records colonist damage on a level at the given tick and reports
// whether it begins a new incident rather than continuing the current one.
func (wm *WorldManager) startsIncident(id string, tick int) bool {
	last, seen := wm.lastDamage[id]
	wm.lastDamage[id] = tick
	return !seen || tick-last >= incidentCooldown()
}

// noteAwayDamage handles damage on a level ticking in the background: the first hit
// to a colonist in an incident raises the popup.
func (wm *WorldManager) noteAwayDamage(s *MainState, ev eventsystem.EntityDamagedEvent) {
	// Not IsColonyWorker: the blow may have killed them, and that counts most.
	if !ev.Victim.HasComponents(components.Worker, components.Settlement) ||
		ev.Victim.GetComponent(components.Settlement).(*components.SettlementComponent).Name != s.colonyName() {
		return
	}
	if !wm.startsIncident(s.locationID, s.tick) {
		return
	}
	victim := entityDisplayName(ev.Victim)
	where := wm.locationLabel(s.locationID)
	wm.incidents = append(wm.incidents, Incident{LocationID: s.locationID, Where: where, Victim: victim})
	wm.notify(s, fmt.Sprintf("%s came under attack", victim))
}

// ActiveIncident returns the incident awaiting an answer, or nil. While there is
// one the whole world clock is stopped.
func (wm *WorldManager) ActiveIncident() *Incident {
	if len(wm.incidents) == 0 {
		return nil
	}
	return &wm.incidents[0]
}

// IgnoreIncident dismisses the popup. The fight plays out, and further hits in it
// don't raise another one.
func (wm *WorldManager) IgnoreIncident() {
	if len(wm.incidents) > 0 {
		wm.incidents = wm.incidents[1:]
	}
}

// PauseIncident dismisses the popup and leaves the world clock stopped, so the
// player can look around before deciding.
func (wm *WorldManager) PauseIncident() {
	wm.IgnoreIncident()
	wm.paused = true
	wm.pauseReason = "incident"
}

// TravelToIncident goes to the incident's level at the usual fuel cost and returns it
// ready to switch to. On failure the popup stays up.
func (wm *WorldManager) TravelToIncident() (*MainState, error) {
	inc := wm.ActiveIncident()
	if inc == nil {
		return nil, fmt.Errorf("no incident")
	}
	var ms *MainState
	var err error
	if inc.LocationID == "" {
		ms, err = wm.EnterShip()
	} else {
		if err = wm.TravelPaying(inc.LocationID); err != nil {
			return nil, err
		}
		ms, err = wm.EnterCurrent()
	}
	if err != nil {
		return nil, err
	}
	wm.IgnoreIncident()
	return ms, nil
}

// TravelPaying travels to id and deducts the fuel, refusing if the ship is short.
func (wm *WorldManager) TravelPaying(id string) error {
	c := wm.Campaign
	moving := id != c.CurrentLocationID
	cost := c.FuelCost(id)
	hold := storage.ShipProvider{Level: wm.ShipLevel()}
	owners := []string{wm.shipSettlementName()}
	if have := storage.CountResource(hold, owners, "fuel"); moving && have < cost {
		return fmt.Errorf("not enough fuel: have %d, need %d", have, cost)
	}
	if err := wm.Travel(id); err != nil {
		return err
	}
	if moving && cost > 0 {
		storage.Deduct(hold, owners, map[string]int{"fuel": cost})
	}
	return nil
}

// notify records a line from a background level for the dashboard. Consecutive
// repeats from the same place in quick succession merge into one entry.
func (wm *WorldManager) notify(s *MainState, text string) {
	if n := len(wm.notifications); n > 0 {
		last := &wm.notifications[n-1]
		if last.LocationID == s.locationID && last.Text == text && s.tick-last.tick < notificationMerge() {
			last.Count++
			last.tick = s.tick
			return
		}
	}
	wm.notifications = append(wm.notifications, Notification{
		LocationID: s.locationID, Where: wm.locationLabel(s.locationID), Text: text, Count: 1, tick: s.tick,
	})
	if over := len(wm.notifications) - maxNotifications; over > 0 {
		wm.notifications = wm.notifications[over:]
	}
}

// RecentNotifications returns up to n of the newest notifications, oldest first.
func (wm *WorldManager) RecentNotifications(n int) []Notification {
	if len(wm.notifications) > n {
		return wm.notifications[len(wm.notifications)-n:]
	}
	return wm.notifications
}

// notificationLine formats one notification for display.
func notificationLine(n Notification) string {
	if n.Count > 1 {
		return fmt.Sprintf("%s: %s (x%d)", n.Where, n.Text, n.Count)
	}
	return fmt.Sprintf("%s: %s", n.Where, n.Text)
}

// incidentPopup is the Pause / Travel / Ignore modal over WorldManager's incident
// queue. Live levels and the dashboard both host it, since either may be up.
type incidentPopup struct {
	wm      *WorldManager
	modal   *minui.Modal
	label   *minui.Label
	errLine *minui.Label
	pending state.StateInterface
}

func newIncidentPopup(wm *WorldManager) *incidentPopup {
	cfg := config.Global()
	const w, h = 480, 160
	p := &incidentPopup{wm: wm, modal: minui.NewModal("incident_popup", "Colonist Under Attack", w, h)}
	p.modal.SetPosition((cfg.ScreenWidth-w)/2, (cfg.ScreenHeight-h)/2)
	p.modal.Closeable = false
	p.label = minui.NewLabel("incident_label", "")
	p.label.SetPosition(20, 14)
	p.modal.AddChild(p.label)
	p.errLine = minui.NewLabel("incident_error", "")
	p.errLine.SetPosition(20, 38)
	p.modal.AddChild(p.errLine)
	button := func(id, text string, x int, onClick func()) {
		b := minui.NewButton(id, text)
		b.SetPosition(x, 70)
		b.SetSize(130, 30)
		b.OnClick = onClick
		p.modal.AddChild(b)
	}
	button("incident_pause", "Pause", 20, wm.PauseIncident)
	button("incident_travel", "Travel", 175, func() {
		ms, err := wm.TravelToIncident()
		if err != nil {
			p.errLine.Text = err.Error()
			return
		}
		p.pending = ms
	})
	button("incident_ignore", "Ignore", 330, wm.IgnoreIncident)
	return p
}

// update runs the popup for one frame and returns the state to switch to, if the
// player chose Travel.
func (p *incidentPopup) update() state.StateInterface {
	inc := p.wm.ActiveIncident()
	if inc == nil {
		p.modal.SetVisible(false)
		p.errLine.Text = ""
		return nil
	}
	p.modal.SetVisible(true)
	p.label.Text = fmt.Sprintf("%s is under attack on %s.", inc.Victim, inc.Where)
	p.modal.Update()
	t := p.pending
	p.pending = nil
	return t
}

func (p *incidentPopup) draw(screen *ebiten.Image) {
	if p.wm.ActiveIncident() == nil {
		return
	}
	p.modal.Draw(screen)
}
