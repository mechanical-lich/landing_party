package game

import (
	"fmt"
	"image/color"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	mlge_text "github.com/mechanical-lich/mlge/text"
)

// A spreading race's run (phase F; docs/brainstorm/playable_races_plan.md).
// There is no ship: the player's people are presences of faction
// campaign.PlayerFaction on the planets they hold, built, run, frozen and
// stepped exactly as a rival's (faction_settlement.go, faction_hive.go). The
// Star Map's View opens a held planet, and on it the cursor's order paths
// are bound to that presence's settlement ("player:<loc>"), so the build
// menu, the egg and evolve modals, the orders and the crew panel act there.
// The settlement director keeps running every rung (assist): the player's
// orders join its queue. The run ends by campaign.SpreadOutcome, once a day.

// playerSettlementName is the settlement of the player's presence on loc.
func playerSettlementName(locID string) string {
	return campaign.PlayerFaction + ":" + locID
}

// bindPlayerSettlement points ms's order paths at the player's presence's
// settlement on its level, in a spreading race's run (after the presences
// materialise). A level the player holds nothing on keeps its own, empty,
// colony settlement: the cursor there only selects and inspects.
func bindPlayerSettlement(c *campaign.Campaign, loc *campaign.Location, ms *MainState) {
	if ms == nil || loc == nil || !c.SpreadRun() {
		return
	}
	st := settlement.Settlements[playerSettlementName(loc.ID)]
	if st == nil {
		return
	}
	ms.MainSettlement = st
	ms.applyDoctrine()
	if ms.wm != nil {
		ms.wm.syncPlayerTechs()
	}
}

// ordersHere reports whether the cursor gives orders on this level: never in
// observer mode; in a spreading race's run only on a planet the player
// holds (its settlement is bound).
func (s *MainState) ordersHere() bool {
	if observerMode() {
		return false
	}
	if spreadRun() {
		return s.MainSettlement != nil && s.MainSettlement.Name == playerSettlementName(s.locationID)
	}
	return true
}

// checkSpreadEnd ends a spreading race's run once a day: lost with nothing
// left of the player's, won holding the race's win.sites worlds. The end
// screen is shown by whichever state is up (takeEnd).
func (wm *WorldManager) checkSpreadEnd() {
	c := wm.Campaign
	wm.dropDeadPlayerWorlds()
	switch c.SpreadOutcome() {
	case "won":
		c.Won = true
		wm.end = raceEndScreen(c, true)
	case "lost":
		c.Lost = true
		wm.end = raceEndScreen(c, false)
	}
}

// takeEnd hands over the end screen once (nil: the run goes on).
func (wm *WorldManager) takeEnd() *CampaignEndState {
	if wm == nil || wm.end == nil {
		return nil
	}
	e := wm.end
	wm.end = nil
	return e
}

// raceEndScreen is the end screen with the race's own words (draft, in the
// doctrine's race block).
func raceEndScreen(c *campaign.Campaign, won bool) *CampaignEndState {
	title, text := "Expedition Lost", ""
	if won {
		title = "Victory"
	}
	if d := campaign.PlayerDoctrine(c); d != nil && d.Race != nil {
		if won && d.Race.Win != nil {
			title, text = orStr(d.Race.Win.Title, title), d.Race.Win.Text
		}
		if !won && d.Race.Loss != nil {
			title, text = orStr(d.Race.Loss.Title, title), d.Race.Loss.Text
		}
	}
	e := NewCampaignEndState(won, text)
	e.title = title
	return e
}

func orStr(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

// raceLine is the dashboard header's line in a spreading race's run: how
// many worlds are the player's, against the win.
func raceLine(c *campaign.Campaign) string {
	held := len(c.PlayerSites())
	want := 0
	if d := campaign.PlayerDoctrine(c); d != nil && d.Race != nil && d.Race.Win != nil {
		want = d.Race.Win.Sites
	}
	eggs := 0
	for _, f := range c.Fleets {
		if f.Presence.Faction == campaign.PlayerFaction {
			eggs++
		}
	}
	line := fmt.Sprintf("Your worlds: %d / %d", held, want) // DRAFT
	if eggs > 0 {
		line += fmt.Sprintf("      in flight: %d", eggs) // DRAFT
	}
	return line
}

// drawPlayerSites is the Star Map's block where the ship's crew lists stand
// in a ship run: the player's worlds with their stage and population, and
// eggs or signals in flight.
func (p *starMapPanel) drawPlayerSites(screen *ebiten.Image, x, y int) {
	c := p.campaign
	lbl := color.RGBA{160, 185, 215, 255}
	mlge_text.Draw(screen, "Your worlds", 14, x, y, lbl) // DRAFT
	d := campaign.PlayerDoctrine(c)
	var lines []string
	for _, loc := range c.PlayerSites() {
		pr := loc.Presence(campaign.PlayerFaction)
		word := ""
		if d != nil {
			word = d.StageName(pr.Stage)
		}
		lines = append(lines, fmt.Sprintf("%s: %s, %d", loc.Name, word, pr.Population))
	}
	for _, loc := range c.Locations {
		if pr := loc.Presence(campaign.PlayerFaction); pr != nil && pr.Dormant {
			lines = append(lines, fmt.Sprintf("%s: dormant, wakes day %d", loc.Name, pr.WakeDay)) // DRAFT
		}
	}
	for _, f := range c.Fleets {
		if f.Presence.Faction == campaign.PlayerFaction {
			lines = append(lines, fmt.Sprintf("egg to %s, day %d", p.locName(f.ToID), f.ArriveDay)) // DRAFT
		}
	}
	if len(lines) == 0 {
		lines = []string{"Nothing left."} // DRAFT
	}
	for i, l := range lines {
		if i >= 7 {
			break
		}
		mlge_text.Draw(screen, l, 13, x, y+24+i*20, dashBodyColor)
	}
}

// syncPlayerTechs makes a race run's techs one pool across its worlds:
// each loaded world's live research joins its presence, the campaign pools
// them (Campaign.SyncPlayerTechs), and every loaded world's settlement then
// knows the pool, its living retyped for any upgrade it gained.
func (wm *WorldManager) syncPlayerTechs() {
	c := wm.Campaign
	if !c.SpreadRun() {
		return
	}
	type held struct {
		ms *MainState
		p  *campaign.FactionPresence
		st *settlement.Settlement
	}
	var worlds []held
	for id, ms := range wm.loaded {
		loc := c.Locations[id]
		if loc == nil || ms == nil || ms.level == nil {
			continue
		}
		p := loc.Presence(campaign.PlayerFaction)
		st := settlement.Settlements[playerSettlementName(id)]
		if p == nil || st == nil {
			continue
		}
		for _, t := range st.KnownTechs {
			if !containsStr(p.KnownTechs, t) {
				p.KnownTechs = append(p.KnownTechs, t)
			}
		}
		worlds = append(worlds, held{ms, p, st})
	}
	c.SyncPlayerTechs()
	for _, w := range worlds {
		for _, t := range c.Player.KnownTechs {
			if !w.st.HasTech(t) {
				w.st.UnlockTech(t)
				retypeSettlement(w.ms.level, w.st.Name, t)
			}
		}
		notePresenceTechs(w.p.ID, w.st.KnownTechs)
	}
}

// raceContactRange is how near a body of the player's must come to a
// neutral faction's worker for the two to have met (the hive's scouts
// record contact at 8).
const raceContactRange = 8

// raceContacts is a race run's daily contact check (the plan's open question
// 10): on every loaded world the player holds, a neutral faction (the rival
// colony) whose worker a body of the player's has come within
// raceContactRange of turns hostile, everywhere. The hostility marks are then
// synced (a ship landing on a held world turns it in the campaign step) and
// the faction's people on loaded levels take their hostile stance.
func (wm *WorldManager) raceContacts() {
	c := wm.Campaign
	if !c.SpreadRun() {
		return
	}
	turned := map[string]bool{}
	for id, ms := range wm.loaded {
		loc := c.Locations[id]
		if ms == nil || ms.level == nil || loc == nil || !c.HoldsSite(loc) {
			continue
		}
		mine := settlementWorkers(ms.level, playerSettlementName(id))
		if len(mine) == 0 {
			continue
		}
		ps, _ := settlementPresences(c, loc)
		for _, p := range ps {
			if p.Faction == campaign.PlayerFaction || c.Stance(p.Faction) != campaign.StanceNeutral {
				continue
			}
			if met(mine, settlementWorkers(ms.level, factionSettlementName(p, id))) && c.RaceContact(p.Faction, loc) {
				turned[p.Faction] = true
				message.PostMessage("Alert", fmt.Sprintf("Your brood has been seen on %s. They are hostile now.", loc.Name)) // DRAFT
			}
		}
	}
	syncFactionHostility(c)
	for f := range turned {
		d := campaign.LoadFactionData().Doctrine(c.DoctrineOf(f))
		for _, ms := range wm.loaded {
			if ms == nil || ms.level == nil || d == nil {
				continue
			}
			for _, e := range ms.level.Entities {
				if e != nil && e.HasComponent(rlcomponents.Description) &&
					e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction == f {
					seedHostility(e, d, true)
				}
			}
		}
	}
}

// met reports whether any of a is within raceContactRange of any of b.
func met(a, b []*ecs.Entity) bool {
	for _, x := range a {
		xp, ok := entityPos(x)
		if !ok {
			continue
		}
		for _, y := range b {
			yp, ok := entityPos(y)
			if ok && xp.GetZ() == yp.GetZ() && max(absI(xp.GetX()-yp.GetX()), absI(xp.GetY()-yp.GetY())) <= raceContactRange {
				return true
			}
		}
	}
	return false
}

// chartRaceRange charts the worlds in a race run's reach, once a day.
func (wm *WorldManager) chartRaceRange() {
	for _, loc := range wm.Campaign.ChartRaceReach() {
		message.PostMessage("Mission", "New world in reach: "+loc.Name) // DRAFT
	}
}

// broodLossThreshold is how many of a world's bodies must die in a day for
// the day's losses to make one Legends event.
const broodLossThreshold = 5

// noteBroodLoss counts a body of the player's lost on locID today.
func (wm *WorldManager) noteBroodLoss(locID string) {
	if wm.broodLosses == nil {
		wm.broodLosses = map[string]int{}
	}
	wm.broodLosses[locID]++
}

// recordBroodLosses writes the day's losses: one brood_losses event per
// world that lost broodLossThreshold or more, then forgets the day.
func (wm *WorldManager) recordBroodLosses() {
	c := wm.Campaign
	ids := make([]string, 0, len(wm.broodLosses))
	for id := range wm.broodLosses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if n := wm.broodLosses[id]; n >= broodLossThreshold {
			c.RecordLive("brood_losses", c.Locations[id], map[string]string{"count": fmt.Sprint(n)})
		}
	}
	wm.broodLosses = nil
}

// deployAt is the Deploy order (phase F step 5): the nearest idle unit of the
// player's whose type deploys (the autonz' relay drone) walks to (x, y, z)
// and folds into its structure, when the floor there takes it. It reports
// whether one was sent.
func (s *MainState) deployAt(x, y, z int) bool {
	d := campaign.PlayerDoctrine(s.campaign)
	if d == nil || s.level == nil {
		return false
	}
	var best *ecs.Entity
	var dep *campaign.DoctrineDeploy
	bestD := -1
	for _, e := range settlementWorkers(s.level, s.colonyName()) {
		k := components.KinOf(e)
		if k == nil || d.Types[k.Type] == nil || d.Types[k.Type].Deploy == nil {
			continue
		}
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
			continue
		}
		pc, ok := entityPos(e)
		if !ok {
			continue
		}
		if dd := max(absI(pc.GetX()-x), absI(pc.GetY()-y)); bestD < 0 || dd < bestD {
			best, bestD, dep = e, dd, d.Types[k.Type].Deploy
		}
	}
	if best == nil {
		message.PostMessage("Orders", "No idle unit to deploy.") // DRAFT
		return false
	}
	if orderDeploy(s.level, best, x, y, z, dep.As, dep.Floor, dep.Ticks) == nil {
		message.PostMessage("Orders", "It cannot deploy there.") // DRAFT
		return false
	}
	return true
}

// dropDeadPlayerWorlds removes the record of a player presence whose loaded
// world has nothing of it left (no worker, egg or larva): a wiped world still
// loaded would otherwise keep its record, and the run, alive until it froze.
func (wm *WorldManager) dropDeadPlayerWorlds() {
	c := wm.Campaign
	if !c.SpreadRun() {
		return
	}
	for id, ms := range wm.loaded {
		loc := c.Locations[id]
		if loc == nil || ms == nil || ms.level == nil {
			continue
		}
		p := loc.Presence(campaign.PlayerFaction)
		if p == nil || p.Dormant || !p.Materialized {
			continue
		}
		name := playerSettlementName(id)
		if len(settlementWorkers(ms.level, name)) > 0 || len(hiveBodies(ms.level, p.ID, name)) > 0 || len(hiveLarvae(ms.level, name)) > 0 {
			continue
		}
		out := loc.Factions[:0]
		for _, q := range loc.Factions {
			if q.Faction != campaign.PlayerFaction {
				out = append(out, q)
			}
		}
		loc.Factions = out
		message.PostMessage("Alert", "Nothing of yours is left on "+loc.Name+".") // DRAFT
	}
}

// postAt is the Post Guard order (the player in the director's guard rung's
// place): the nearest unit of the player's with one of the order's roles
// (data: interface.orders' post roles) that is idle or already holding a
// post walks to (x, y, z) and holds there. It reports whether one went.
func (s *MainState) postAt(x, y, z int) bool {
	o := s.doctrineSettlement().UI().Order("post")
	if o == nil || s.level == nil {
		return false
	}
	var best *ecs.Entity
	bestD := -1
	for _, e := range settlementWorkers(s.level, s.colonyName()) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if !containsStr(o.Roles, wc.Role) {
			continue
		}
		if t := wc.CurrentTask; t != nil && !t.Completed && t.Data != task_requests.HoldHere {
			continue // at work: only an idle guard, or one moved from its post
		}
		pc, ok := entityPos(e)
		if !ok {
			continue
		}
		if dd := max(absI(pc.GetX()-x), absI(pc.GetY()-y)); bestD < 0 || dd < bestD {
			best, bestD = e, dd
		}
	}
	if best == nil {
		message.PostMessage("Orders", "No idle guard to post.") // DRAFT
		return false
	}
	if t := best.GetComponent(components.Worker).(*components.WorkerComponent).CurrentTask; t != nil {
		t.Complete() // off its old post
	}
	return settlementOrders{}.Hold(best, x, y, z) != nil
}

// loadLauncher is the Load action (the player in the launch rung's place):
// the launcher's shot (its doctrine's shot_pop) of the player's units, the
// launch rung's roles first, else any but guards, nearest first, walks to the
// ring round the structure and holds there; the live launch or transmit
// takes them on a ready day, at the aim. It reports how many went.
func (s *MainState) loadLauncher(structure *ecs.Entity) int {
	d := campaign.PlayerDoctrine(s.campaign)
	sp, ok := entityPos(structure)
	if d == nil || !ok || s.level == nil {
		return 0
	}
	need := max(d.Mobility.ShotPop, 1)
	var roles []string
	if dir := d.Settlement.Director; dir != nil {
		for _, p := range dir.Priorities {
			if p.Launch != nil {
				roles = p.Launch.Roles
			}
		}
	}
	type cand struct {
		e    *ecs.Entity
		rank int
		dist int
	}
	var cands []cand
	for _, e := range settlementWorkers(s.level, s.colonyName()) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		pc, _ := entityPos(e)
		if wc.CurrentTask != nil && !wc.CurrentTask.Completed && wc.CurrentTask.Data == task_requests.HoldHere &&
			max(absI(pc.GetX()-sp.GetX()), absI(pc.GetY()-sp.GetY())) <= launchWalk(d) {
			need-- // already loaded
			continue
		}
		rank := len(roles) + 1
		for i, r := range roles {
			if wc.Role == r {
				rank = i
			}
		}
		if len(roles) > 0 && rank > len(roles) || len(roles) == 0 && wc.Role == "guard" {
			continue
		}
		cands = append(cands, cand{e, rank, max(absI(pc.GetX()-sp.GetX()), absI(pc.GetY()-sp.GetY()))})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		return cands[i].dist < cands[j].dist
	})
	sent, k := 0, 0
	walk := max(launchWalk(d), 1)
	for _, c := range cands {
		if need <= 0 {
			break
		}
		for ; k < 8*walk*walk; k++ {
			r := 1 + k/8
			if r > walk {
				break
			}
			dx := []int{-1, 0, 1, 1, 1, 0, -1, -1}[k%8] * r
			dy := []int{-1, -1, -1, 0, 1, 1, 1, 0}[k%8] * r
			x, y, z := sp.GetX()+dx, sp.GetY()+dy, sp.GetZ()
			if s.level.GetSolidEntityAt(x, y, z) != nil {
				continue
			}
			if t := c.e.GetComponent(components.Worker).(*components.WorkerComponent).CurrentTask; t != nil {
				t.Complete()
			}
			if (settlementOrders{}).Hold(c.e, x, y, z) != nil {
				sent++
				need--
				k++
				break
			}
		}
	}
	return sent
}

// handBack gives a race run's world back to its director when the player
// leaves it (the owner, 2026-11-12): every group taken over there is
// released. The aim stays (the launcher fires only where the player aimed
// it; the director never chooses a world itself, campaign.aimed).
func (wm *WorldManager) handBack(locID string) {
	c := wm.Campaign
	if locID == "" || !c.SpreadRun() {
		return
	}
	if loc := c.Locations[locID]; loc != nil {
		if p := loc.Presence(campaign.PlayerFaction); p != nil {
			p.TakenOver = nil
		}
	}
}

// aimNearest aims the player's presence on loc at the nearest world its
// launcher or signal reaches now (tests and the vmtest's "aim": a player
// who aims). It returns the world's ID, "" for none.
func aimNearest(c *campaign.Campaign, loc *campaign.Location, loaded func(string) bool) string {
	p := loc.Presence(campaign.PlayerFaction)
	d := campaign.PlayerDoctrine(c)
	if p == nil || d == nil {
		return ""
	}
	st := c.LaunchStatus(loc, p, loaded)
	if d.Mobility.Kind == campaign.MobilitySignal {
		st = c.SignalStatus(loc, p, loaded)
	}
	best, bestD := "", 0.0
	for _, t := range st.Targets {
		if dd := (t.X-loc.X)*(t.X-loc.X) + (t.Y-loc.Y)*(t.Y-loc.Y); best == "" || dd < bestD {
			best, bestD = t.ID, dd
		}
	}
	p.LaunchTarget = best
	return best
}
