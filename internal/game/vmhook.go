//go:build ebitenginevmguest

package game

// VM play-testing hook, linked only into guest builds (-tags ebitenginevmguest).
// Game.Run wraps the game with mlge's vmtest/guest, which serves this hook to
// the vmtest host over the sidecar socket: a state summary, named UI targets
// and debug commands, under a lock that the wrapped Update and Draw also take.
// The current screen is StateMachine.Current().

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/state"
	"github.com/mechanical-lich/mlge/ui/minui"
	"github.com/mechanical-lich/mlge/vmtest/guest"
	"github.com/mechanical-lich/mlge/vmtest/guest/minuitarget"
)

func init() { vmHook = func(g *Game) guest.Hook { return &lpHook{g: g} } }

// lpHook is Landing Party's guest.Hook.
type lpHook struct{ g *Game }

func (h *lpHook) cur() state.StateInterface             { return h.g.StateMachine.Current() }
func (h *lpHook) State() any                            { return vmState(h.cur()) }
func (h *lpHook) Target(name string) (guest.Rect, bool) { return vmTarget(h.cur(), name) }
func (h *lpHook) Candidates() []string                  { return vmCandidates(h.cur()) }
func (h *lpHook) Debug(cmd string) (string, error)      { return vmDebug(h.cur(), cmd) }

// vmWorld returns the campaign's world manager behind the current state.
func vmWorld(cur state.StateInterface) *WorldManager {
	switch s := cur.(type) {
	case *DashboardState:
		return s.wm
	case *MainState:
		return s.wm
	}
	return nil
}

// ---- state summary ----

var titleScreenNames = map[titleScreen]string{screenMain: "main", screenNewSettlement: "new", screenLoad: "load"}

func vmState(cur state.StateInterface) map[string]any {
	out := map[string]any{}
	var modals []string
	switch s := cur.(type) {
	case nil:
		out["state"] = "none"
	case *TitleState:
		out["state"] = "title"
		out["title_screen"] = titleScreenNames[s.screen]
		out["title_fields"] = map[string]any{"name": s.nameInput.Text, "seed": s.seedInput.Text, "tutorial": s.tutorialToggle.IsOn()}
		if s.errMsg != "" {
			out["title_error"] = s.errMsg
		}
	case *SettingsState:
		out["state"] = "settings"
	case *CampaignEndState:
		out["state"] = "end"
	case *DashboardState:
		out["state"] = "dashboard"
		if s.active >= 0 && s.active < len(s.tabs) {
			out["dash_tab"] = s.tabs[s.active].id
		}
		out["status"] = s.status
		if s.active >= 0 && s.active < len(s.tabs) {
			if lp, ok := s.tabs[s.active].panel.(*legendsPanel); ok {
				out["legends"] = vmList(lp.lines)
				out["legends_text"] = strings.Join(lp.lines, "\n")
				vis, top, bottom := lp.visible()
				out["legends_visible"] = strings.Join(vis, "\n")
				out["legends_at_top"], out["legends_at_bottom"] = top, bottom
			}
		}
		if s.storageInspector != nil && s.storageInspector.Visible {
			modals = append(modals, "inspector")
		}
		if p := s.starMapPanel(); p != nil {
			if p.scanOpen {
				modals = append(modals, "scan")
			}
			if loc := p.selectedLocation(); loc != nil {
				out["selected_location"] = map[string]any{"id": loc.ID, "name": loc.Name, "fuel_cost": s.campaign.FuelCost(loc.ID),
					"summary": loc.Summary, "history_summary": vmHistorySummary(s.campaign, loc), "tier": s.campaign.LocationTier(loc)}
			}
		}
		out["paused"] = s.wm.ClockStopped()
	case *MainState:
		if s.wm != nil && s == s.wm.shipLevel {
			out["state"] = "ship"
		} else {
			out["state"] = "site"
		}
		out["tick"] = s.tick
		if s.level != nil {
			out["hour"] = s.level.Hour
			out["level_day"] = s.level.Day
		}
		out["paused"] = s.clockStopped()
		out["pause_text"] = s.pauseIndicatorText()
		out["cursor_mode"] = string(s.CursorMode)
		out["build_mode"] = s.buildMode
		out["camera"] = map[string]any{"x": s.camera.X, "y": s.camera.Y, "z": s.camera.Z, "tile_w": s.camera.TileW}
		if st := s.MainSettlement; st != nil {
			out["settlement"] = map[string]any{"name": st.Name, "x": st.CenterX, "y": st.CenterY, "z": st.CenterZ}
		}
		if s.selectedEntity != nil {
			out["selected"] = map[string]any{"blueprint": s.selectedEntity.Blueprint, "name": entityDisplayName(s.selectedEntity)}
		}
		if s.mapModal != nil && s.mapModal.Visible {
			modals = append(modals, "map")
		}
		if s.cheatModal != nil && s.cheatModal.Visible {
			modals = append(modals, "cheat")
		}
		if s.storageInspector != nil && s.storageInspector.Visible {
			modals = append(modals, "inspector")
		}
		if s.guiManager != nil {
			for _, m := range []string{"mainMenu", "saveModal", "loadModal", "craftingModal", "researchModal", "colonistModal"} {
				if s.guiManager.ModalElement(m, "") != nil {
					modals = append(modals, m)
				}
			}
		}
	default:
		out["state"] = fmt.Sprintf("%T", s)
	}
	if wm := vmWorld(cur); wm != nil {
		if wm.ActiveIncident() != nil {
			modals = append(modals, "incident")
		}
		vmWorldState(out, wm)
	}
	sort.Strings(modals)
	out["modal"] = vmList(modals)
	n := len(message.MessageLog)
	out["messages"] = vmList(append([]string(nil), message.MessageLog[max(0, n-10):]...))
	return out
}

func vmList(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func vmWorldState(out map[string]any, wm *WorldManager) {
	c := wm.Campaign
	loc := c.CurrentLocation()
	camp := map[string]any{
		"name": c.Name, "seed": c.Seed, "location_id": c.CurrentLocationID,
		"adrift": c.IsAdrift(), "in_orbit": wm.Current() != nil, "won": c.Won, "lost": c.Lost,
		"day": c.Day(),
	}
	if loc != nil {
		camp["location_name"] = loc.Name
		camp["scenario"] = loc.ScenarioID
		chapter := ""
		if site := c.SiteHistory(loc); site != nil && site.Chapter != "" {
			if ev := c.History.Event(site.Chapter); ev != nil {
				chapter = ev.Type
			}
		}
		camp["chapter"] = chapter
		camp["tier"] = c.LocationTier(loc)
	}
	camp["known_events"] = c.KnownEventCount()
	techs := append([]string(nil), c.KnownTechs...)
	sort.Strings(techs)
	camp["known_techs"] = vmList(techs)
	out["campaign"] = camp
	out["day"] = c.Day()

	shipOwners := []string{wm.shipSettlementName()}
	ship := storage.ShipProvider{Level: wm.ShipLevel()}
	out["fuel"] = storage.CountResource(ship, shipOwners, "fuel")
	out["hold"] = vmStock(ship, shipOwners)
	if cur := wm.Current(); cur != nil && cur.level != nil {
		out["site_storage"] = vmStock(storage.LevelProvider{Level: cur.level}, []string{cur.colonyName()})
		out["site_structures"] = vmStructures(cur)
	}

	var crew []any
	addCrew := func(ms *MainState, where string) {
		if ms == nil || ms.level == nil {
			return
		}
		colony := ms.colonyName()
		if ms == wm.shipLevel {
			colony = wm.shipSettlementName()
		}
		var rows []map[string]any
		for _, e := range ms.level.Entities {
			if !components.IsColonyWorker(e, colony) {
				continue
			}
			row := map[string]any{"name": entityDisplayName(e), "where": where}
			if e.HasComponent(rlcomponents.Position) {
				pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				row["x"], row["y"], row["z"] = pc.GetX(), pc.GetY(), pc.GetZ()
			}
			if wc, ok := e.GetComponent(components.Worker).(*components.WorkerComponent); ok && wc.CurrentTask != nil {
				row["task"] = string(wc.CurrentTask.Action)
			}
			rows = append(rows, row)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i]["name"].(string) < rows[j]["name"].(string) })
		for _, r := range rows {
			crew = append(crew, r)
		}
	}
	addCrew(wm.shipLevel, "ship")
	ids := make([]string, 0, len(wm.loaded))
	for id := range wm.loaded {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	owed := map[string]any{}
	for _, id := range ids {
		addCrew(wm.loaded[id], id)
		owed[id] = wm.owed[id]
	}
	if crew == nil {
		crew = []any{}
	}
	out["crew"] = crew
	at := map[string]any{}
	for _, r := range crew {
		w := r.(map[string]any)["where"].(string)
		n, _ := at[w].(int)
		at[w] = n + 1
	}
	out["crew_at"] = at
	onSite, _ := at[c.CurrentLocationID].(int)
	if wm.Current() == nil {
		onSite = 0
	}
	out["crew_on_site"] = onSite
	out["loaded"] = vmList(ids)
	out["owed"] = owed

	if inc := wm.ActiveIncident(); inc != nil {
		out["incident"] = map[string]any{"location_id": inc.LocationID, "where": inc.Where, "victim": inc.Victim}
	} else {
		out["incident"] = nil
	}
	var notes []string
	for _, n := range wm.RecentNotifications(5) {
		notes = append(notes, notificationLine(n))
	}
	out["notifications"] = vmList(notes)

	if b := activeTutorial; b != nil && b.wm == wm {
		out["tutorial"] = map[string]any{
			"step": b.dir.StepID(), "index": b.dir.Index(), "count": b.dir.Count(),
			"done": b.dir.Done(), "skipped": b.dir.Skipped(), "ended": b.ended,
		}
	} else {
		out["tutorial"] = nil
	}
}

// vmStructures counts the buildable structures (entity buildables such as
// lockers, beds, benches, labs) standing on a level, by blueprint.
func vmStructures(ms *MainState) map[string]any {
	builds := construction.ListBuildables()
	out := map[string]any{}
	for _, ents := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range ents {
			if e == nil || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			if b, ok := builds[e.Blueprint]; ok && b.IsEntity {
				n, _ := out[e.Blueprint].(int)
				out[e.Blueprint] = n + 1
			}
		}
	}
	return out
}

// vmStock maps each resource in storage to its count, fuel included.
func vmStock(p storage.Provider, owners []string) map[string]any {
	names := storage.ListResources(p, owners)
	if !slices.Contains(names, "fuel") {
		names = append(names, "fuel")
	}
	out := map[string]any{}
	for name, n := range storage.CountAll(p, owners, names) {
		if n > 0 {
			out[name] = n
		}
	}
	return out
}

// ---- named targets ----

// vmBridge is a throwaway tutorialBridge for the current frame, so the
// tutorial's highlight resolvers serve as script targets with the tutorial off.
func vmBridge(cur state.StateInterface) *tutorialBridge {
	wm := vmWorld(cur)
	if wm == nil {
		return nil
	}
	b := &tutorialBridge{c: wm.Campaign, wm: wm}
	switch s := cur.(type) {
	case *DashboardState:
		b.lastDash = s
		b.lastCtx = "dash:"
		if s.active >= 0 && s.active < len(s.tabs) {
			b.lastCtx += s.tabs[s.active].id
		}
	case *MainState:
		b.lastSite = s
		b.lastCtx = tutorial.ContextSite
		if s == wm.shipLevel {
			b.lastCtx = tutorial.ContextShip
		}
	}
	return b
}

// vmTarget resolves a script target name to a rectangle (a ring becomes the
// square around its centre). Kinds that depend on the screen are tried
// first, then the tutorial's highlight resolvers.
func vmTarget(cur state.StateInterface, name string) (guest.Rect, bool) {
	var r guest.Router
	switch s := cur.(type) {
	case *TitleState:
		r.Handle("title", func(rest string) (guest.Rect, bool) { return vmTitleTarget(s, rest) })
	case *DashboardState:
		r.Handle("tab", func(rest string) (guest.Rect, bool) { return minuitarget.Tab(s.tabBar, rest) })
		r.Handle("row", func(rest string) (guest.Rect, bool) { return vmDashRow(s, rest) })
		r.Handle("gi", func(rest string) (guest.Rect, bool) { return vmGITarget(s, rest) })
		r.Handle("legends", func(rest string) (guest.Rect, bool) {
			if s.active < 0 || s.active >= len(s.tabs) || rest != "chronicle" {
				return guest.Rect{}, false
			}
			lp, ok := s.tabs[s.active].panel.(*legendsPanel)
			if !ok {
				return guest.Rect{}, false
			}
			b := lp.text.GetBounds()
			return guest.Rect{X: float64(b.X), Y: float64(b.Y), W: float64(b.Width), H: float64(b.Height)}, true
		})
	case *MainState:
		r.Handle("hud", func(rest string) (guest.Rect, bool) {
			if s.guiManager == nil {
				return guest.Rect{}, false
			}
			x, y, w, h, ok := s.guiManager.ElementBounds(rest)
			return guest.Rect{X: float64(x), Y: float64(y), W: float64(w), H: float64(h)}, ok
		})
		// menu:<id> is a button in the HUD main menu (Esc): save, load, newgame, quit.
		r.Handle("menu", func(rest string) (guest.Rect, bool) { return vmModalTarget(s, "mainMenu:"+rest) })
		r.Handle("modal", func(rest string) (guest.Rect, bool) { return vmModalTarget(s, rest) })
		r.Handle("tile", func(rest string) (guest.Rect, bool) { return vmTileTarget(s, rest) })
		// settle:dx,dy is the tile dx,dy from the colony's centre, on the
		// level the camera shows.
		r.Handle("settle", func(rest string) (guest.Rect, bool) {
			st := s.MainSettlement
			var dx, dy int
			if st == nil {
				return guest.Rect{}, false
			}
			if _, err := fmt.Sscanf(rest, "%d,%d", &dx, &dy); err != nil {
				return guest.Rect{}, false
			}
			return vmTileTarget(s, fmt.Sprintf("%d,%d,%d", st.CenterX+dx, st.CenterY+dy, s.camera.Z))
		})
		r.Handle("btn", func(rest string) (guest.Rect, bool) {
			if rest == "dashboard" && s.dashboardBtn != nil {
				return minuitarget.Element(s.dashboardBtn)
			}
			return guest.Rect{}, false
		})
	}
	if b := vmBridge(cur); b != nil {
		r.Fallback(func(name string) (guest.Rect, bool) { return vmBridgeTarget(b, name) })
	}
	return r.Target(name)
}

func fromHL(hl highlight, found bool) (guest.Rect, bool) {
	if !found {
		return guest.Rect{}, false
	}
	if hl.radius > 0 {
		return guest.Rect{X: float64(hl.x - 1), Y: float64(hl.y - 1), W: 2, H: 2}, true
	}
	return guest.Rect{X: float64(hl.x), Y: float64(hl.y), W: float64(hl.w), H: float64(hl.h)}, true
}

// vmBridgeTarget resolves the kinds backed by the tutorial's highlights.
func vmBridgeTarget(b *tutorialBridge, name string) (guest.Rect, bool) {
	kind, rest, _ := strings.Cut(name, ":")
	switch kind {
	case "dash":
		return fromHL(b.dashHighlight(rest))
	case "star":
		return fromHL(b.starHighlight(rest))
	case "ship":
		return fromHL(b.shipHoldHighlight())
	case "inspector":
		if rest == "beam" || rest == "list" {
			return fromHL(b.inspectorHighlight(rest))
		}
		return vmInspectorTarget(b.openInspector(), rest)
	case "region":
		return fromHL(b.regionHighlight(rest))
	case "entity":
		if bp, n, has := strings.Cut(rest, ":"); has {
			return vmNthEntity(b, bp, n)
		}
		return fromHL(b.entityHighlight(rest))
	case "tutorial":
		return vmTutorialTarget(rest)
	case "popup":
		return vmPopupTarget(b.wm, rest)
	}
	return guest.Rect{}, false
}

func vmTitleTarget(s *TitleState, name string) (guest.Rect, bool) {
	var el minui.Element
	switch s.screen {
	case screenMain:
		switch name {
		case "new":
			el = s.newBtn
		case "load":
			el = s.loadBtn
		case "settings":
			el = s.settingsBtn
		case "quit":
			el = s.quitBtn
		}
	case screenNewSettlement:
		switch name {
		case "name":
			el = s.nameInput
		case "seed":
			el = s.seedInput
		case "tutorial":
			el = s.tutorialToggle
		case "launch":
			el = s.generateBtn
		case "cancel":
			el = s.cancelBtn
		}
	case screenLoad:
		switch name {
		case "load_confirm":
			el = s.loadConfirm
		case "load_cancel":
			el = s.loadCancel
		}
		if strings.HasPrefix(name, "save:") {
			for i, n := range s.campaignNames {
				if n == strings.TrimPrefix(name, "save:") {
					return minuitarget.ListRow(s.saveListBox, i)
				}
			}
		}
	}
	if el == nil {
		return guest.Rect{}, false
	}
	if t, ok := el.(*minui.Toggle); ok {
		// A toggle's bounds can be zero-sized; its box sits at its position.
		x, y := t.GetAbsolutePosition()
		r := t.GetBounds()
		return guest.Rect{X: float64(x), Y: float64(y), W: float64(max(r.Width, 20)), H: float64(max(r.Height, 20))}, true
	}
	return minuitarget.Element(el)
}

// vmDashRow resolves row:locList:<location id>, row:shipList:<n>,
// row:planList:<n> and row:crew:<n> on the dashboard.
func vmDashRow(d *DashboardState, rest string) (guest.Rect, bool) {
	list, key, _ := strings.Cut(rest, ":")
	n, nerr := strconv.Atoi(key)
	if p := d.starMapPanel(); p != nil && d.tabs[d.active].id == "starmap" {
		switch list {
		case "locList":
			if r, ok := p.rowRect(key); ok {
				return minuitarget.FromRect(r), true
			}
			return guest.Rect{}, false
		case "shipList":
			if nerr == nil {
				return minuitarget.ListRow(p.shipList, n)
			}
		case "planList":
			if nerr == nil {
				return minuitarget.ListRow(p.planList, n)
			}
		}
	}
	return guest.Rect{}, false
}

// vmGITarget resolves the Global Inventory tab's buttons: gi:beam_down, gi:beam_up.
func vmGITarget(d *DashboardState, name string) (guest.Rect, bool) {
	if d.tabs[d.active].id != "inventory" {
		return guest.Rect{}, false
	}
	p, ok := d.tabs[d.active].panel.(*globalInvPanel)
	if !ok {
		return guest.Rect{}, false
	}
	switch name {
	case "beam_down":
		return minuitarget.Element(p.beamDownBtn)
	case "beam_up":
		return minuitarget.Element(p.beamUpBtn)
	}
	return guest.Rect{}, false
}

// vmInspectorTarget resolves inspector:amount, :all, :close and :item:<blueprint>.
func vmInspectorTarget(insp *StorageInspectorModal, name string) (guest.Rect, bool) {
	if insp == nil {
		return guest.Rect{}, false
	}
	switch {
	case name == "amount":
		return minuitarget.Element(insp.amountInput)
	case name == "close":
		return minuitarget.Element(insp.modal.FindChildByID("si_close"))
	case name == "all":
		return minuitarget.Element(insp.modal.FindChildByID("si_all"))
	case strings.HasPrefix(name, "item:"):
		bp := strings.TrimPrefix(name, "item:")
		for i, b := range insp.invBPs {
			if b == bp {
				return minuitarget.ListRow(insp.invList, i)
			}
		}
	}
	return guest.Rect{}, false
}

func vmTutorialTarget(name string) (guest.Rect, bool) {
	b := activeTutorial
	if b == nil || b.panel == nil {
		return guest.Rect{}, false
	}
	p := b.panel
	switch name {
	case "skip":
		if !p.ending && !p.collapsed && !p.inspector {
			return minuitarget.Element(p.skipBtn)
		}
	case "manual":
		if step := b.dir.Current(); !p.ending && !p.collapsed && !p.inspector && step != nil && step.Topic != "" {
			return minuitarget.Element(p.manualBtn)
		}
	case "close":
		if p.ending && !p.collapsed {
			return minuitarget.Element(p.closeBtn)
		}
	case "collapse":
		if !p.inspector {
			return minuitarget.Element(p.collapseBtn)
		}
	}
	return guest.Rect{}, false
}

func vmPopupTarget(wm *WorldManager, name string) (guest.Rect, bool) {
	if wm == nil || wm.popup == nil || wm.ActiveIncident() == nil {
		return guest.Rect{}, false
	}
	id := map[string]string{"pause": "incident_pause", "travel": "incident_travel", "ignore": "incident_ignore"}[name]
	if id == "" {
		return guest.Rect{}, false
	}
	return minuitarget.Element(wm.popup.modal.FindChildByID(id))
}

// vmModalTarget resolves modal:<modal>:<sub> inside a named HUD modal, with
// minuitarget.Modal's grammar: close, <element id>, row:<list id>:<n>,
// item:<list id>:<text>.
func vmModalTarget(s *MainState, spec string) (guest.Rect, bool) {
	if s.guiManager == nil {
		return guest.Rect{}, false
	}
	modal, rest, _ := strings.Cut(spec, ":")
	return minuitarget.Modal(s.guiManager.ModalElement(modal, ""), rest)
}

// vmNthEntity resolves entity:<blueprint>:<n>, the nth living entity of that
// blueprint on screen at the camera's level, in level order (0-based).
func vmNthEntity(b *tutorialBridge, bp, nth string) (guest.Rect, bool) {
	n, err := strconv.Atoi(nth)
	s := b.liveSite()
	if err != nil || s == nil || s.level == nil {
		return guest.Rect{}, false
	}
	for _, ents := range [][]*ecs.Entity{s.level.Entities, s.level.StaticEntities} {
		for _, e := range ents {
			if e == nil || e.Blueprint != bp || !e.HasComponent(rlcomponents.Position) || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if pc.GetZ() != s.camera.Z || !s.camera.Contains(pc.GetX(), pc.GetY(), pc.GetZ()) {
				continue
			}
			if n == 0 {
				hl, ok := b.tileHighlight(s, pc.GetX(), pc.GetY(), pc.GetZ(), 0)
				return guest.Rect{X: float64(hl.x), Y: float64(hl.y), W: float64(hl.w), H: float64(hl.h)}, ok
			}
			n--
		}
	}
	return guest.Rect{}, false
}

// vmTileTarget resolves tile:x,y,z to that tile's screen square.
func vmTileTarget(s *MainState, rest string) (guest.Rect, bool) {
	parts := strings.Split(rest, ",")
	if len(parts) != 3 {
		return guest.Rect{}, false
	}
	var v [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return guest.Rect{}, false
		}
		v[i] = n
	}
	if !s.camera.Contains(v[0], v[1], v[2]) || v[2] != s.camera.Z {
		return guest.Rect{}, false
	}
	x, y := s.camera.WorldToScreenF(v[0], v[1])
	return guest.Rect{X: float64(x), Y: float64(y), W: float64(s.camera.TileW), H: float64(s.camera.TileH)}, true
}

// vmCandidates are names worth listing when they resolve; the engine keeps
// those vmTarget resolves.
func vmCandidates(cur state.StateInterface) []string {
	names := []string{
		"title:new", "title:load", "title:settings", "title:quit", "title:name", "title:seed",
		"title:tutorial", "title:launch", "title:cancel", "title:load_confirm", "title:load_cancel",
		"tab:starmap", "tab:crew", "tab:inventory", "tab:quests", "tab:encyclopedia",
		"dash:returnSiteBtn", "dash:returnShipBtn", "dash:saveBtn", "dash:locList", "dash:locateBtn",
		"dash:travelBtn", "dash:shipList", "dash:planList", "dash:beamDownBtn", "dash:beamUpBtn", "dash:scanBtn",
		"star:" + tutorial.AtTutorial, "star:" + tutorial.AtNext,
		"gi:beam_down", "gi:beam_up", "ship:hold", "btn:dashboard",
		"inspector:beam", "inspector:list", "inspector:amount", "inspector:all", "inspector:close",
		"menu:save", "menu:load", "menu:newgame", "menu:quit",
		"modal:colonistModal:close", "modal:craftingModal:close", "modal:researchModal:close",
		"tutorial:skip", "tutorial:manual", "tutorial:close", "tutorial:collapse",
		"popup:pause", "popup:travel", "popup:ignore",
		"hud:cat_orders", "hud:cat_walls", "hud:cat_floors", "hud:cat_stairs", "hud:cat_structures", "hud:cat_doors",
		"hud:order_dig", "hud:order_mine", "hud:build_storage_locker", "hud:build_bed",
		"hud:build_basic_workbench", "hud:build_research_lab",
		"region:tut_outcrop", "region:tut_rations", "settle:0,0",
		"entity:colonist", "entity:storage_locker", "entity:basic_workbench", "entity:research_lab",
	}
	switch s := cur.(type) {
	case *DashboardState:
		if p := s.starMapPanel(); p != nil {
			for _, id := range p.locIDs {
				names = append(names, "row:locList:"+id)
			}
			for i := range p.shipList.Items {
				names = append(names, fmt.Sprintf("row:shipList:%d", i))
			}
			for i := range p.planList.Items {
				names = append(names, fmt.Sprintf("row:planList:%d", i))
			}
		}
	case *TitleState:
		for _, n := range s.campaignNames {
			names = append(names, "title:save:"+n)
		}
	}
	if b := vmBridge(cur); b != nil {
		if insp := b.openInspector(); insp != nil {
			for _, bp := range insp.invBPs {
				names = append(names, "inspector:item:"+bp)
			}
		}
	}
	return names
}

// ---- debug commands ----

// vmDebug runs "cheat <console command>" on the live level, or
// "incident <location id|ship>" to raise the away-damage popup for a level.
func vmDebug(cur state.StateInterface, cmd string) (string, error) {
	verb, rest, _ := strings.Cut(strings.TrimSpace(cmd), " ")
	rest = strings.TrimSpace(rest)
	switch verb {
	case "reveal":
		// "reveal all" marks every history event known, as if every datapad
		// had been read: a long chronicle for the Legends tab.
		wm := vmWorld(cur)
		if wm == nil || wm.Campaign.History == nil || rest != "all" {
			return "", fmt.Errorf("usage: reveal all (needs a campaign)")
		}
		h := wm.Campaign.History
		for _, e := range h.Events {
			h.MarkKnown(e.ID)
		}
		return fmt.Sprintf("%d events known", len(h.Known)), nil
	case "datapad":
		// Put the first colonist on the nearest lore datapad, as if it had
		// walked there; the next update picks it up.
		s, ok := cur.(*MainState)
		if !ok || s.level == nil {
			return "", fmt.Errorf("datapad needs a site on screen")
		}
		var who *ecs.Entity
		for _, e := range s.level.Entities {
			if components.IsColonyWorker(e, s.colonyName()) {
				who = e
				break
			}
		}
		if who == nil {
			return "", fmt.Errorf("no colonist on the site")
		}
		wpc := who.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		var best *rlcomponents.PositionComponent
		bestD := -1
		for _, e := range s.level.Entities {
			if e == nil || !e.HasComponent(components.Datapad) {
				continue
			}
			if dp := e.GetComponent(components.Datapad).(*components.DatapadComponent); dp.EventID == "" || dp.QuestID != "" {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			dx, dy := pc.GetX()-wpc.GetX(), pc.GetY()-wpc.GetY()
			if d := dx*dx + dy*dy; bestD < 0 || d < bestD {
				best, bestD = pc, d
			}
		}
		if best == nil {
			return "", fmt.Errorf("no lore datapad on the site")
		}
		wpc.SetPosition(best.GetX(), best.GetY(), best.GetZ())
		s.camera.CenterOn(best.GetX(), best.GetY(), best.GetZ())
		return fmt.Sprintf("%d,%d,%d", best.GetX(), best.GetY(), best.GetZ()), nil
	case "origin":
		// Frame the camera on the site chapter's resolved origin; "origin
		// walk" also moves the first colonist there to see it; "origin walk
		// <event type>" uses that event's origin instead.
		s, ok := cur.(*MainState)
		if !ok || s.level == nil {
			return "", fmt.Errorf("origin needs a site on screen")
		}
		pts := s.level.Regions[LoreOriginRegion]
		mode, evType, _ := strings.Cut(rest, " ")
		rest = mode
		// "origin walk <event type> <n>": the nth such event's origin
		// (1-based), for the two sides of a per-side event.
		nth := 1
		if t, n, ok := strings.Cut(evType, " "); ok {
			evType = t
			if v, err := strconv.Atoi(n); err == nil && v > 0 {
				nth = v
			}
		}
		if evType != "" {
			// "origin walk <event type>": that event's origin on this site
			// (a quiet site has no chapter origin).
			pts = nil
			if site := s.campaign.SiteHistory(s.location()); site != nil {
				for _, id := range site.Events {
					if ev := s.campaign.History.Event(id); ev != nil && ev.Type == evType {
						if x, y, z, ok := generation.Origin(s.level, id); ok {
							pts = append(pts, [3]int{x, y, z})
						}
					}
				}
			}
		}
		if len(pts) < nth {
			return "", fmt.Errorf("this site has no such history origin")
		}
		p := pts[nth-1]
		s.camera.CenterOn(p[0], p[1], p[2])
		if rest == "walk" {
			// Put the first colonist on the nearest free tile there, so the
			// game's own field of view reveals the origin.
			for _, e := range s.level.Entities {
				if !components.IsColonyWorker(e, s.colonyName()) {
					continue
				}
				x, y, z := freeLandingTile(s.level, p[0], p[1], p[2])
				if pc, ok := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent); ok {
					pc.SetPosition(x, y, z)
				}
				break
			}
		}
		return fmt.Sprintf("%d,%d,%d", p[0], p[1], p[2]), nil
	case "cheat":
		s, ok := cur.(*MainState)
		if !ok || s.cheatModal == nil {
			return "", fmt.Errorf("cheat needs a site or the ship on screen")
		}
		before := len(message.MessageLog)
		s.cheatModal.runCommand(rest)
		return strings.Join(message.MessageLog[before:], "\n"), nil
	case "incident":
		wm := vmWorld(cur)
		if wm == nil {
			return "", fmt.Errorf("no campaign")
		}
		var ms *MainState
		id := rest
		if rest == "ship" || rest == "" {
			ms, id = wm.shipLevel, ""
		} else {
			ms = wm.loaded[rest]
		}
		if ms == nil || ms.level == nil {
			return "", fmt.Errorf("level %q is not loaded", rest)
		}
		colony := ms.colonyName()
		if ms == wm.shipLevel {
			colony = wm.shipSettlementName()
		}
		var victim *ecs.Entity
		for _, e := range ms.level.Entities {
			if components.IsColonyWorker(e, colony) {
				victim = e
				break
			}
		}
		if victim == nil {
			return "", fmt.Errorf("no colonist on %q", rest)
		}
		name := entityDisplayName(victim)
		where := wm.locationLabel(id)
		wm.incidents = append(wm.incidents, Incident{LocationID: id, Where: where, Victim: name})
		wm.notify(ms, fmt.Sprintf("%s came under attack", name))
		return fmt.Sprintf("incident: %s on %s", name, where), nil
	}
	return "", fmt.Errorf("unknown debug command %q (cheat, incident)", verb)
}

// vmHistorySummary reports whether a location's Star Map summary is its
// history's summary line (not the archetype sentence).
func vmHistorySummary(c *campaign.Campaign, loc *campaign.Location) bool {
	site := c.SiteHistory(loc)
	if site == nil {
		return false
	}
	lib, err := history.DefaultLibrary()
	if err != nil {
		return false
	}
	return loc.Summary != "" && loc.Summary == c.History.Summary(lib, site)
}
