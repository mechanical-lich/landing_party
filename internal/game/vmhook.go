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
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
	"github.com/mechanical-lich/landing_party/internal/world"
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
		out["title_fields"] = map[string]any{"name": s.nameInput.Text, "seed": s.seedInput.Text, "tutorial": s.tutorialToggle.IsOn(), "factions": s.factions}
		if s.rivalBoxes() {
			var names []any
			var groups []any
			for i := 0; i < s.factions && i < len(s.rivals); i++ {
				names = append(names, s.rivalPickName(s.rivals[i].pick))
				groups = append(groups, s.rivals[i].group)
			}
			tf := out["title_fields"].(map[string]any)
			tf["rivals"], tf["rival_groups"], tf["player_group"] = names, groups, s.playerGroup
		}
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
					"summary": loc.Summary, "history_summary": vmHistorySummary(s.campaign, loc), "tier": s.campaign.LocationTier(loc),
					"known_factions": len(loc.KnownPresences())}
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
		if s.level != nil {
			// What the view is centred on: its named room and station
			// module, and how many of each blueprint stand in view.
			st := s.camera.SidebarTiles()
			cx, cy := s.camera.X+st+(s.camera.ViewW()-st)/2, s.camera.Y+s.camera.ViewH()/2
			out["view_room"] = s.level.RoomAt(cx, cy, s.camera.Z)
			if m := s.level.ModuleAt(cx, cy, s.camera.Z); m != nil {
				out["view_module"] = m.ID
			} else {
				out["view_module"] = ""
			}
			inView := map[string]any{}
			for _, list := range [][]*ecs.Entity{s.level.Entities, s.level.StaticEntities} {
				for _, e := range list {
					if pc, ok := entityPos(e); ok && e.Blueprint != "" && s.camera.Contains(pc.GetX(), pc.GetY(), pc.GetZ()) {
						n, _ := inView[e.Blueprint].(int)
						inView[e.Blueprint] = n + 1
					}
				}
			}
			out["in_view"] = inView
		}
		if st := s.MainSettlement; st != nil {
			out["settlement"] = map[string]any{"name": st.Name, "x": st.CenterX, "y": st.CenterY, "z": st.CenterZ}
		}
		if f := vmFaction(s); f != nil {
			out["faction"] = f
		}
		if r := vmRival(s); r != nil {
			out["rival"] = r
		}
		if a := vmAutonz(s); a != nil {
			out["autonz"] = a
		}
		if s.guiManager != nil {
			// Phase F: the HUD the doctrine's interface block draws.
			res := map[string]any{}
			for _, id := range s.guiManager.ResourceSlots() {
				res[id] = s.guiManager.ResourceValue(id)
			}
			out["panel"] = map[string]any{"title": s.guiManager.StructurePanelTitle(), "rows": vmList(s.guiManager.StructurePanelShown())}
			if pr := s.playerPresenceHere(); pr != nil {
				out["taken_over"] = vmList(append([]string(nil), pr.TakenOver...))
				out["launch_target"] = pr.LaunchTarget
			}
			out["hud"] = map[string]any{"resources": res, "slots": vmList(s.guiManager.ResourceSlots()),
				"orders": vmList(s.guiManager.OrderIDs()), "unit_modal": vmList(s.guiManager.UnitModalShown()),
				"unit_title": s.guiManager.UnitModalTitle(), "research_title": s.guiManager.ResearchTitle(),
				"research_hull": vmList(s.guiManager.ResearchHull())}
		}
		if s.level != nil {
			out["settlement_stats"] = vmStats(s.settlementStats())
			out["observer"] = observerMode()
		}
		out["location"] = s.locationID
		if s.guiManager != nil {
			out["view_label"] = s.guiManager.ViewIndicator()
		}
		out["context"] = map[string]any{"title": s.contextTitle, "body": s.contextBody}
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
		if s.inspectorUp() {
			modals = append(modals, "entity_inspector")
			ei := s.inspector
			bp := ""
			if ei.target != nil {
				bp = ei.target.Blueprint
			}
			out["entity_inspector"] = map[string]any{"title": ei.Title, "blueprint": bp,
				"lines": vmList(ei.Lines), "text": strings.Join(ei.Lines, "\n")}
		}
		if s.guiManager != nil {
			for _, m := range []string{"mainMenu", "saveModal", "loadModal", "craftingModal", "researchModal", "colonistModal", "structurePanel"} {
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
		if wm.Campaign != nil && wm.Campaign.Encounter.Pending() {
			modals = append(modals, "encounter")
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
		// Rival factions: the New Expedition count and every presence (known
		// or not) as id -> population, for scripts that check seeding.
		"faction_count": c.FactionCount, "presences": vmPresences(c), "factions": vmFactionTable(c),
		"fleets": len(c.Fleets), "known_fleets": len(c.KnownFleets()),
		"ships": vmShips(c), "diplomacy": vmDiplomacy(c), "ship_fleets": vmFleets(c),
		"encounter": vmEncounter(c),
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
	techs := append([]string(nil), c.KnownTechs()...)
	sort.Strings(techs)
	camp["known_techs"] = vmList(techs)
	out["campaign"] = camp
	out["day"] = c.Day()
	// Observer runs: the expedition fleet (where it is, landed on a site
	// or not, crew, fuel, why it last lifted off), and the planet the viewer
	// is on.
	out["observer_run"] = c.Observer
	// Phase F: a spreading race's run, its race and the worlds it holds.
	out["spread_run"] = c.SpreadRun()
	out["race"] = campaign.PlayerDoctrineKey(c)
	var held []string
	for _, l := range c.PlayerSites() {
		held = append(held, l.ID)
	}
	out["player_sites"] = vmList(held)
	out["player_site_count"] = len(held)
	// What the Star Map shows of the factions (faction_view.go): every
	// presence and fleet in an observer run, the seen ones otherwise.
	shownP := 0
	for _, loc := range c.Locations {
		shownP += len(shownPresences(loc))
	}
	out["shown_presences"] = shownP
	out["shown_fleets"] = len(shownFleets(c))
	if f := c.Expedition(); f != nil {
		site := ""
		if f.Landed() && c.Locations[f.LandedID].HasSite() {
			site = f.LandedID
		}
		out["expedition"] = map[string]any{"landed": f.Landed(), "site": site, "to": f.ToID,
			"crew": f.Presence.Population, "fuel": f.Presence.Fuel, "last_move": f.LastMove}
	}
	out["viewing"] = wm.viewedID
	var loadedIDs []string
	for id := range wm.loaded {
		loadedIDs = append(loadedIDs, id)
	}
	sort.Strings(loadedIDs)
	out["loaded"] = vmList(loadedIDs)

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
			// panel_covered: the panel is down to its title row below an open
			// storage inspector or HUD modal.
			"panel_covered": b.panel != nil && b.panel.covered,
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
		// factions:site:<location id> is the Factions tab's first row that
		// selects that site.
		r.Handle("factions", func(rest string) (guest.Rect, bool) {
			id, ok := strings.CutPrefix(rest, "site:")
			if !ok || s.active < 0 || s.active >= len(s.tabs) {
				return guest.Rect{}, false
			}
			fp, isF := s.tabs[s.active].panel.(*factionsPanel)
			if !isF {
				return guest.Rect{}, false
			}
			return fp.rowRect(id)
		})
		// legends:chronicle is the chronicle text; legends:datapads the
		// filter row that lists the datapads read.
		r.Handle("legends", func(rest string) (guest.Rect, bool) {
			if s.active < 0 || s.active >= len(s.tabs) {
				return guest.Rect{}, false
			}
			lp, ok := s.tabs[s.active].panel.(*legendsPanel)
			if !ok {
				return guest.Rect{}, false
			}
			if rest == "datapads" {
				if i := slices.Index(lp.keys, datapadsKey); i >= 0 {
					return minuitarget.ListRow(lp.filters, i)
				}
				return guest.Rect{}, false
			}
			if rest != "chronicle" {
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
		// inspect:close is the entity inspector's Close button.
		r.Handle("inspect", func(rest string) (guest.Rect, bool) {
			if rest != "close" || !s.inspectorUp() {
				return guest.Rect{}, false
			}
			return minuitarget.Element(s.inspector.modal.FindChildByID("ei_close"))
		})
		// focused is the tile of the entity the last "focus" debug command
		// centred the camera on, where it stands now.
		r.Handle("focused", func(string) (guest.Rect, bool) {
			pc, ok := entityPos(vmFocused)
			if !ok {
				return guest.Rect{}, false
			}
			return vmTileTarget(s, fmt.Sprintf("%d,%d,%d", pc.GetX(), pc.GetY(), pc.GetZ()))
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
		if b.lastCtx == tutorial.ContextShip {
			return fromHL(b.entityHighlightOn(b.lastSite, rest)) // aboard (a grey run's vat)
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
		case "factions_minus":
			el = s.factionsMinus
		case "factions_plus":
			el = s.factionsPlus
		case "race_prev":
			el = s.racePrev
		case "race_next":
			el = s.raceNext
		case "launch":
			el = s.generateBtn
		case "cancel":
			el = s.cancelBtn
		case "player_group_minus":
			el = s.playerGMinus
		case "player_group_plus":
			el = s.playerGPlus
		default:
			// rival_prev_N, rival_next_N, rival_group_minus_N, rival_group_plus_N
			var kind string
			var n int
			for _, k := range []string{"rival_group_minus_", "rival_group_plus_", "rival_prev_", "rival_next_"} {
				if v, ok := strings.CutPrefix(name, k); ok {
					if i, err := strconv.Atoi(v); err == nil {
						kind, n = k, i
					}
					break
				}
			}
			if kind != "" && n >= 0 && n < len(s.rivals) {
				r := s.rivals[n]
				el = map[string]minui.Element{"rival_prev_": r.prev, "rival_next_": r.next,
					"rival_group_minus_": r.gMinus, "rival_group_plus_": r.gPlus}[kind]
			}
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
			if n, ok := strings.CutPrefix(key, "yours:"); ok {
				// row:locList:yours:<n>: the nth world the player holds
				// (a spreading race's run).
				i, err := strconv.Atoi(n)
				held := d.campaign.PlayerSites()
				if err != nil || i < 0 || i >= len(held) {
					return guest.Rect{}, false
				}
				key = held[i].ID
				for j, lid := range p.locIDs {
					if lid == key {
						p.locList.ScrollTo(j) // a row scrolled out of view does not resolve
						return minuitarget.ListRow(p.locList, j)
					}
				}
			}
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
		if !p.ending && !p.collapsed && !p.covered {
			return minuitarget.Element(p.skipBtn)
		}
	case "manual":
		if step := b.dir.Current(); !p.ending && !p.collapsed && !p.covered && step != nil && step.Topic != "" {
			return minuitarget.Element(p.manualBtn)
		}
	case "close":
		if p.ending && !p.collapsed {
			return minuitarget.Element(p.closeBtn)
		}
	case "collapse":
		if !p.covered {
			return minuitarget.Element(p.collapseBtn)
		}
	}
	return guest.Rect{}, false
}

func vmPopupTarget(wm *WorldManager, name string) (guest.Rect, bool) {
	// The Contact popup (phase D): popup:evade, popup:board, popup:parley.
	if wm != nil && wm.encPopup != nil && wm.Campaign != nil && wm.Campaign.Encounter.Pending() {
		id := map[string]string{"evade": "encounter_evade", "board": "encounter_board", "parley": "encounter_parley"}[name]
		if id == "" {
			return guest.Rect{}, false
		}
		return minuitarget.Element(wm.encPopup.modal.FindChildByID(id))
	}
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
		"title:tutorial", "title:factions_minus", "title:factions_plus", "title:launch", "title:cancel", "title:load_confirm", "title:load_cancel",
		"tab:starmap", "tab:crew", "tab:inventory", "tab:quests", "tab:encyclopedia",
		"dash:returnSiteBtn", "dash:returnShipBtn", "dash:saveBtn", "dash:locList", "dash:locateBtn",
		"dash:travelBtn", "dash:viewBtn", "dash:shipList", "dash:planList", "dash:beamDownBtn", "dash:beamUpBtn", "dash:scanBtn",
		"star:" + tutorial.AtTutorial, "star:" + tutorial.AtNext,
		"gi:beam_down", "gi:beam_up", "ship:hold", "btn:dashboard",
		"inspector:beam", "inspector:list", "inspector:amount", "inspector:all", "inspector:close",
		"menu:save", "menu:load", "menu:newgame", "menu:quit",
		"modal:colonistModal:close", "modal:craftingModal:close", "modal:researchModal:close",
		"inspect:close", "focused",
		"tutorial:skip", "tutorial:manual", "tutorial:close", "tutorial:collapse",
		"popup:pause", "popup:travel", "popup:ignore", "popup:evade", "popup:board", "popup:parley",
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
	case "focus":
		// "focus <spec>" centres the camera on the first living entity on
		// the level matching spec (a blueprint, kin:<unit type>, datapad,
		// storage or structure), for a click on target "focused".
		s, ok := cur.(*MainState)
		if !ok || s.level == nil || rest == "" {
			return "", fmt.Errorf("usage: focus <blueprint|kin:<type>|datapad|storage|structure> (needs a level on screen)")
		}
		e := vmFind(s.level, rest)
		pc, found := entityPos(e)
		if !found {
			return "", fmt.Errorf("nothing matches %q", rest)
		}
		vmFocused = e
		s.camera.CenterOn(pc.GetX(), pc.GetY(), pc.GetZ())
		return fmt.Sprintf("%s %d,%d,%d", e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ()), nil
	case "days":
		// "days <n>" moves the star date on n days and steps the factions
		// through them, as a jump would: a hive grows before the player
		// arrives.
		wm := vmWorld(cur)
		n, err := strconv.Atoi(rest)
		if wm == nil || err != nil || n < 0 {
			return "", fmt.Errorf("usage: days <n> (needs a campaign)")
		}
		wm.Campaign.AdvanceDays(n)
		wm.Campaign.StepFactions(wm.IsLoaded)
		return fmt.Sprintf("day %d", wm.Campaign.Day()), nil
	case "place":
		// "place <blueprint> <dx> <dy>": put a structure at the player
		// settlement's centre plus (dx, dy), as built (a script showing a
		// race's structure panel without waiting days for it).
		s, ok := cur.(*MainState)
		f := strings.Fields(rest)
		if !ok || s.level == nil || s.MainSettlement == nil || len(f) != 3 {
			return "", fmt.Errorf("usage: place <blueprint> <dx> <dy> (needs a site with a settlement)")
		}
		dx, e1 := strconv.Atoi(f[1])
		dy, e2 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil {
			return "", fmt.Errorf("usage: place <blueprint> <dx> <dy>")
		}
		st := s.MainSettlement
		e, err := factory.Create(f[0], st.CenterX+dx, st.CenterY+dy, st.CenterZ)
		if err != nil {
			return "", err
		}
		if e.HasComponent(components.Storage) {
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = st.Name // as a build gives it
		}
		s.level.AddEntity(e)
		return fmt.Sprintf("%s at %d,%d,%d", f[0], st.CenterX+dx, st.CenterY+dy, st.CenterZ), nil
	case "presence":
		// "presence <location id> <doctrine> <pop>": put a planet-carried
		// presence of doctrine on a frozen location (a script meeting a
		// rival on a race run's world; phase F). Built when the level is.
		wm := vmWorld(cur)
		f := strings.Fields(rest)
		if wm == nil || len(f) != 3 {
			return "", fmt.Errorf("usage: presence <location id> <doctrine> <pop> (needs a campaign)")
		}
		loc := wm.Campaign.Locations[f[0]]
		pop, err := strconv.Atoi(f[2])
		if loc == nil || err != nil || campaign.LoadFactionData().Doctrine(f[1]) == nil || wm.IsLoaded(f[0]) {
			return "", fmt.Errorf("presence: unknown or loaded location, unknown doctrine, or bad count")
		}
		fd := campaign.LoadFactionData()
		dk := fd.Resolve(f[1])      // "mirror" is the colony's rival view
		key := fd.FactionKeyFor(dk) // and its faction key stays "mirror"
		wm.Campaign.AddFactionRowFor(key, dk)
		setFactionDoctrines(wm.Campaign)
		wm.Campaign.FactionSeq++
		loc.Factions = append(loc.Factions, campaign.FactionPresence{ID: fmt.Sprintf("fp%d", wm.Campaign.FactionSeq),
			Faction: key, Doctrine: dk, Population: pop, Stockpile: map[string]int{"metal_ore": 30, "crystal": 15},
			Seed: wm.Campaign.Seed + int64(wm.Campaign.FactionSeq), LastShotDay: campaign.NeverFired, Known: true})
		return fmt.Sprintf("%s presence of %d on %s", f[1], pop, f[0]), nil
	case "aim":
		// "aim <location id>": aim the player's launcher or signal there at
		// the nearest world in its reach (a race run; phase F: the player's
		// spread fires only where the player aims it).
		wm := vmWorld(cur)
		if wm == nil || rest == "" {
			return "", fmt.Errorf("usage: aim <location id> (needs a campaign)")
		}
		loc := wm.Campaign.Locations[rest]
		if loc == nil || loc.Presence(campaign.PlayerFaction) == nil {
			return "", fmt.Errorf("nothing of the player's on %s", rest)
		}
		if t := aimNearest(wm.Campaign, loc, wm.IsLoaded); t != "" {
			return "aimed at " + t, nil
		}
		return "", fmt.Errorf("no world in reach of %s", rest)
	case "freeze":
		// "freeze <location id>": freeze a loaded planet the viewer is not
		// on, so Tier 1 (debug days) steps it (a race run's world stays
		// loaded while its brood lives there).
		wm := vmWorld(cur)
		if wm == nil || rest == "" {
			return "", fmt.Errorf("usage: freeze <location id> (needs a campaign)")
		}
		ms, loc := wm.loaded[rest], wm.Campaign.Locations[rest]
		if ms == nil || loc == nil || rest == wm.viewedID {
			return "", fmt.Errorf("%s is not loaded, or is on screen", rest)
		}
		ms.teardown()
		if err := wm.Freeze(loc, ms); err != nil {
			return "", err
		}
		return "frozen " + rest, nil
	case "stance":
		// "stance <faction> <neutral|hostile>" sets a faction's stance toward
		// the player (a script watching a hive build without its guards on
		// the landing party).
		wm := vmWorld(cur)
		f, st, ok := strings.Cut(rest, " ")
		if wm == nil || !ok || (st != campaign.StanceNeutral && st != campaign.StanceHostile) {
			return "", fmt.Errorf("usage: stance <faction> <neutral|hostile> (needs a campaign)")
		}
		wm.Campaign.SetStance(f, st, wm.Campaign.Locations[wm.currentID])
		components.SetFactionHostile(f, st == campaign.StanceHostile)
		return f + " " + st, nil
	case "faction":
		// "faction [walk]" frames the camera on the first faction presence
		// of the site on screen (its centre: the nest, the launcher); "walk"
		// also puts the first colonist beside it, so its field of view
		// reveals it.
		s, ok := cur.(*MainState)
		if !ok || s.level == nil || s.location() == nil || len(s.location().Factions) == 0 {
			return "", fmt.Errorf("faction needs a site with a presence on screen")
		}
		p := s.location().Factions[0]
		x, y, z, ok := generation.Origin(s.level, factionOriginKey(p.ID))
		if !ok {
			return "", fmt.Errorf("presence %s is not built on this level", p.ID)
		}
		s.camera.CenterOn(x, y, z)
		if rest == "walk" {
			for _, e := range s.level.Entities {
				if !components.IsColonyWorker(e, s.colonyName()) {
					continue
				}
				wx, wy, wz := freeLandingTile(s.level, x, y+8, z)
				if pc, ok := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent); ok {
					pc.SetPosition(wx, wy, wz)
				}
				break
			}
		}
		return fmt.Sprintf("%s %d,%d,%d", p.ID, x, y, z), nil
	case "tech":
		// "tech <location id> <faction> <tech>": the faction's presence there
		// knows the tech in its record (phase E3: its next spawns are at that
		// tier; the cheat console's "faction tech" does the same on a live
		// site).
		wm := vmWorld(cur)
		f := strings.Fields(rest)
		if wm == nil || len(f) != 3 {
			return "", fmt.Errorf("usage: tech <location id> <faction> <tech> (needs a campaign)")
		}
		loc := wm.Campaign.Locations[f[0]]
		if loc == nil || loc.Presence(f[1]) == nil {
			return "", fmt.Errorf("no %s presence on %s", f[1], f[0])
		}
		p := loc.Presence(f[1])
		if !containsStr(p.KnownTechs, f[2]) {
			p.KnownTechs = append(p.KnownTechs, f[2])
		}
		notePresenceTechs(p.ID, p.KnownTechs)
		return fmt.Sprintf("%s on %s knows %s", f[1], f[0], f[2]), nil
	case "ship":
		// "ship <fleet id> tier <n> | pop <n> | hold <n> | tech <tech> | leave": a
		// faction ship's hull tier, crew, a tech, or its stay ended so it
		// lifts off at the next day (phase E4: a warship to look at without
		// waiting on the hoard). Set before landing to see it materialise.
		wm := vmWorld(cur)
		f := strings.Fields(rest)
		if wm == nil || len(f) < 2 {
			return "", fmt.Errorf("usage: ship <fleet id> tier <n>|pop <n>|hold <n>|tech <tech>|leave (needs a campaign)")
		}
		var fl *campaign.Fleet
		for i := range wm.Campaign.Fleets {
			if wm.Campaign.Fleets[i].ID == f[0] {
				fl = &wm.Campaign.Fleets[i]
			}
		}
		if fl == nil || fl.Kind != campaign.FleetShip {
			return "", fmt.Errorf("no ship %s", f[0])
		}
		p := &fl.Presence
		switch {
		case f[1] == "leave":
			if d := campaign.LoadFactionData().Doctrine(p.Doctrine); d != nil {
				fl.LandedDay = wm.Campaign.FactionDay - history.StarDate(d.Mobility.StayDays)
			}
			return fmt.Sprintf("%s stay over", fl.ID), nil
		case len(f) == 3 && f[1] == "tier":
			n, err := strconv.Atoi(f[2])
			if err != nil {
				return "", err
			}
			fl.ShipTier = n
		case len(f) == 3 && f[1] == "pop":
			n, err := strconv.Atoi(f[2])
			if err != nil {
				return "", err
			}
			p.Population, p.Units = n, nil
			if d := campaign.LoadFactionData().Doctrine(p.Doctrine); d != nil && d.HasTypes() {
				d.AddUnits(p, wm.Campaign.Locations[fl.LandedID], n)
			}
		case len(f) == 3 && f[1] == "hold":
			// The hold emptied to n metal (so a trip's digging does not
			// fill it and send the ship home).
			n, err := strconv.Atoi(f[2])
			if err != nil {
				return "", err
			}
			p.Stockpile, p.YieldFrac = nil, nil
			if n > 0 {
				p.Stockpile = map[string]int{"metal_ore": n}
			}
		case len(f) == 3 && f[1] == "tech":
			if !containsStr(p.KnownTechs, f[2]) {
				p.KnownTechs = append(p.KnownTechs, f[2])
			}
			notePresenceTechs(p.ID, p.KnownTechs)
		default:
			return "", fmt.Errorf("usage: ship <fleet id> tier <n>|pop <n>|hold <n>|tech <tech>|leave")
		}
		return fmt.Sprintf("%s tier %d crew %d techs %v", fl.ID, fl.ShipTier, p.Population, p.KnownTechs), nil
	case "autonz":
		// "autonz [dx dy]" frames the camera on the autonz settlement of the
		// site on screen (phase E3), offset by dx, dy tiles.
		s, ok := cur.(*MainState)
		if !ok || s.level == nil || s.location() == nil {
			return "", fmt.Errorf("autonz needs a site on screen")
		}
		p := s.location().Presence("autonz")
		if p == nil {
			return "", fmt.Errorf("no autonz presence on this site")
		}
		x, y, z, ok := originOf(s.level, p.ID)
		if !ok {
			return "", fmt.Errorf("presence %s is not built on this level", p.ID)
		}
		var dx, dy int
		fmt.Sscanf(rest, "%d %d", &dx, &dy)
		s.camera.CenterOn(x+dx, y+dy, z)
		return fmt.Sprintf("%s %d,%d,%d", p.ID, x, y, z), nil
	case "rival":
		// "rival [walk]" frames the camera on the first faction settlement
		// on the site on screen (its camp); "walk" also puts the first
		// colonist south of it, so its field of view reveals the camp.
		s, ok := cur.(*MainState)
		if !ok || s.level == nil || s.location() == nil {
			return "", fmt.Errorf("rival needs a site on screen")
		}
		ps := rivalPresences(s.campaign, s.location())
		if len(ps) == 0 {
			return "", fmt.Errorf("no faction settlement on this site")
		}
		st := settlement.Settlements[factionSettlementName(ps[0], s.location().ID)]
		if st == nil {
			return "", fmt.Errorf("faction settlement not built on this level")
		}
		x, y, z := st.CenterX, st.CenterY, st.CenterZ
		s.camera.CenterOn(x, y, z)
		if rest == "walk" {
			for _, e := range s.level.Entities {
				if !components.IsColonyWorker(e, s.colonyName()) {
					continue
				}
				wx, wy, wz := freeLandingTile(s.level, x, y+8, z)
				if pc, ok := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent); ok {
					pc.SetPosition(wx, wy, wz)
				}
				break
			}
		}
		return fmt.Sprintf("%s %d,%d,%d", st.Name, x, y, z), nil
	case "boarding":
		// "boarding [walk]" frames the camera on a boarded ship's hull (the
		// encounter level's boarding_ship region); "walk" also puts the
		// first colonist beside it, so its field of view shows the hull and
		// the crew (phase D).
		s, ok := cur.(*MainState)
		if !ok || s.level == nil || s.location() == nil || s.location().Boarding == nil {
			return "", fmt.Errorf("boarding needs a boarded ship's deck on screen")
		}
		pts := s.level.Regions["boarding_ship"]
		if len(pts) == 0 {
			return "", fmt.Errorf("no boarding_ship region")
		}
		x, y, z := pts[0][0], pts[0][1], pts[0][2]
		s.camera.CenterOn(x-2, y, z)
		if rest == "walk" {
			for _, e := range s.level.Entities {
				if !components.IsColonyWorker(e, s.colonyName()) {
					continue
				}
				wx, wy, wz := freeLandingTile(s.level, x-6, y, z)
				if pc, ok := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent); ok {
					pc.SetPosition(wx, wy, wz)
				}
				break
			}
		}
		return fmt.Sprintf("boarding %d,%d,%d", x, y, z), nil
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
	case "module", "deck":
		// "module [walk] <id>" frames a station module (and walks the first
		// colonist into it); "deck <z>" frames deck z on the main stair column.
		s, ok := cur.(*MainState)
		if !ok || s.level == nil {
			return "", fmt.Errorf("%s needs a site on screen", verb)
		}
		var p [3]int
		walk := false
		if verb == "deck" {
			pts := s.level.Regions["station_deck:"+strings.TrimSpace(rest)]
			if len(pts) == 0 {
				return "", fmt.Errorf("this site has no deck %q", rest)
			}
			p = pts[0]
		} else {
			id := strings.TrimSpace(rest)
			if after, ok := strings.CutPrefix(id, "walk "); ok {
				walk, id = true, strings.TrimSpace(after)
			}
			m := s.level.ModuleByID(id)
			if m == nil {
				return "", fmt.Errorf("this site has no module %q", id)
			}
			p[0], p[1], p[2] = m.Center()
		}
		s.camera.CenterOn(p[0], p[1], p[2])
		if walk {
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

// vmFaction describes the first faction presence of the site on screen as
// built on its level: bodies by morph and life stage, its tier, whether its
// launcher stands. nil without one.
func vmFaction(s *MainState) map[string]any {
	loc := s.location()
	if loc == nil || len(loc.Factions) == 0 {
		return nil
	}
	p := loc.Factions[0]
	out := map[string]any{"id": p.ID, "stage": p.Stage, "population": p.Population,
		"launcher": presenceLauncher(s.level, p.ID) != nil}
	bodies := presenceBodies(s.level, p.ID)
	out["bodies"] = len(bodies)
	morphs, types := map[string]any{}, map[string]any{}
	for _, e := range bodies {
		k := components.KinOf(e)
		n, _ := morphs[k.Morph].(int)
		morphs[k.Morph] = n + 1
		if k.Type != "" {
			n, _ := types[k.Type].(int)
			types[k.Type] = n + 1
		}
	}
	out["morphs"] = morphs
	out["types"] = types
	// A typed settlement (the hive, phase E2): its bodies by type (eggs and
	// young as their type), larvae, resin share, pools' biomass, structures
	// and director orders.
	if d := campaign.LoadFactionData().Doctrine(p.Doctrine); d != nil && isHiveDoctrine(d) {
		name := factionSettlementName(&p, loc.ID)
		units, eggs := map[string]any{}, map[string]any{}
		for _, e := range hiveBodies(s.level, p.ID, name) {
			t := bodyType(d, e)
			n, _ := units[t].(int)
			units[t] = n + 1
			if hasSuffix(e.Blueprint, "_egg") {
				n, _ := eggs[t].(int)
				eggs[t] = n + 1
			}
		}
		out["units"], out["eggs"] = units, eggs
		out["bodies"] = len(hiveBodies(s.level, p.ID, name))
		out["launcher"] = hiveLauncher(s.level, d, &p) != nil
		out["larvae"] = len(hiveLarvae(s.level, name))
		q := p
		reconcileConversion(s.level, d, &q)
		out["resin"] = q.Conversion
		out["biomass"] = storage.Summarize(storage.LevelProvider{Level: s.level}, []string{name})["biomass"]
		if cx, cy, cz, ok := generation.Origin(s.level, factionOriginKey(p.ID)); ok {
			built := map[string]any{}
			for _, bp := range []string{"hive_heart", "biomass_pool", "egg_cluster", "evolution_chamber", "egg_launcher", "resin_bloom"} {
				built[bp] = len(hiveStructures(s.level, bp, cx, cy, cz, hiveRadius(d)+2))
			}
			out["built"] = built
		}
		if dir := s.factionDirectors[name]; dir != nil {
			out["ordered"] = dir.Ordered
		}
	}
	return out
}

// vmAutonz is the autonz settlement on the site on screen (phase E3): its
// stage, units by type and by tier ("enforcer bronze"), the structures
// standing, its converted share, techs and the director's orders. nil
// without one built.
func vmAutonz(s *MainState) map[string]any {
	loc := s.location()
	if loc == nil || s.level == nil {
		return nil
	}
	p := loc.Presence("autonz")
	if p == nil || !p.Materialized || p.Dormant {
		return nil
	}
	d := campaign.LoadFactionData().Doctrine(p.Doctrine)
	if d == nil || !isAutonzDoctrine(d) {
		return nil
	}
	name := factionSettlementName(p, loc.ID)
	units, tiers := map[string]any{}, map[string]any{}
	for _, e := range settlementWorkers(s.level, name) {
		k := components.KinOf(e)
		if k == nil {
			continue
		}
		n, _ := units[k.Type].(int)
		units[k.Type] = n + 1
		key := k.Type + " " + k.Tier
		m, _ := tiers[key].(int)
		tiers[key] = m + 1
	}
	out := map[string]any{"id": p.ID, "stage": p.Stage, "units": units, "tiers": tiers, "techs": p.KnownTechs}
	if st := settlement.Settlements[name]; st != nil {
		out["techs"] = st.KnownTechs
	}
	if cx, cy, cz, ok := originOf(s.level, p.ID); ok {
		built := map[string]any{}
		for k, v := range autonzBuilt(s.level, d, cx, cy, cz) {
			built[k] = v
		}
		out["built"] = built
		out["centre"] = fmt.Sprintf("%d,%d,%d", cx, cy, cz)
	}
	q := *p
	reconcileConversion(s.level, d, &q)
	out["conversion"] = q.Conversion
	if dir := s.factionDirectors[name]; dir != nil {
		out["ordered"] = dir.Ordered
	}
	return out
}

// vmRival is the first faction settlement on the site: its name, stance,
// workers, the director's orders and what stands within 12 of its centre.
func vmRival(s *MainState) map[string]any {
	loc := s.location()
	if loc == nil || s.level == nil {
		return nil
	}
	ps := rivalPresences(s.campaign, loc)
	if len(ps) == 0 {
		return nil
	}
	p := ps[0]
	name := factionSettlementName(p, loc.ID)
	out := map[string]any{"name": name, "faction": p.Faction, "stance": s.campaign.Stance(p.Faction),
		"workers": len(settlementWorkers(s.level, name)), "stage": p.Stage, "level_stage": p.LevelStage}
	// The converted share of its patch (phase E, the autonz), read live.
	if d := campaign.LoadFactionData().Doctrine(p.Doctrine); d != nil && d.Conversion.Tile != "" {
		if x, y, z, ok := generation.Origin(s.level, factionOriginKey(p.ID)); ok {
			out["conversion"] = generation.ConversionShare(s.level, x, y, z, d.Conversion.Radius, d.Conversion.Tile)
		}
	}
	st := settlement.Settlements[name]
	if st == nil {
		return out
	}
	built := map[string]any{}
	for _, list := range [][]*ecs.Entity{s.level.Entities, s.level.StaticEntities} {
		for _, e := range list {
			if pc, ok := entityPos(e); ok && e.Blueprint != "" && !e.HasComponent(rlcomponents.Item) && !e.HasComponent(components.Worker) &&
				absI(pc.GetX()-st.CenterX) <= 12 && absI(pc.GetY()-st.CenterY) <= 12 && pc.GetZ() == st.CenterZ {
				n, _ := built[e.Blueprint].(int)
				built[e.Blueprint] = n + 1
			}
		}
	}
	out["built"] = built
	ordered := map[string]any{}
	if d := s.factionDirectors[name]; d != nil {
		for k, v := range d.Ordered {
			ordered[k] = v
		}
	}
	out["ordered"] = ordered
	return out
}

// vmShips maps each faction ship's ID to the site it is landed on ("" in
// flight).
func vmShips(c *campaign.Campaign) map[string]any {
	out := map[string]any{}
	for _, f := range c.Fleets {
		if f.Kind == campaign.FleetShip {
			out[f.ID] = f.LandedID
		}
	}
	return out
}

// vmFleets is every ship (phase D): kind, doctrine, tier, where it is
// landed or bound, whether it is a mothership, returning, boarded or known.
func vmFleets(c *campaign.Campaign) map[string]any {
	out := map[string]any{}
	for _, f := range c.Fleets {
		if f.Kind != campaign.FleetShip {
			continue
		}
		out[f.ID] = map[string]any{"doctrine": f.Presence.Doctrine, "tier": f.ShipTier, "landed": f.LandedID,
			"to": f.ToID, "mothership": f.Mothership, "returning": f.Returning, "boarded": f.BoardedAt,
			"known": f.Known, "crew": f.Presence.Population}
	}
	return out
}

// vmDiplomacy is each settlement faction's stance toward the player.
func vmDiplomacy(c *campaign.Campaign) map[string]any {
	out := map[string]any{}
	fd := campaign.LoadFactionData()
	for key, d := range fd.Doctrines {
		if d.Settlement != nil && key != campaign.DefaultPlayerDoctrine {
			fk := fd.FactionKeyFor(key) // the rival colony's faction is "mirror"
			out[fk] = c.Stance(fk)
		}
	}
	for _, f := range c.Factions() { // and every faction instance (rival_selection_plan.md)
		if (c.Player == nil || f.Key != c.Player.Faction) && f.Key != campaign.ExpeditionFaction {
			if d := fd.Doctrine(f.Doctrine); d != nil && d.Settlement != nil {
				out[f.Key] = c.Stance(f.Key)
			}
		}
	}
	return out
}

// vmPresences counts presences per location ID (known or not).
func vmPresences(c *campaign.Campaign) map[string]any {
	out := map[string]any{}
	for id, loc := range c.Locations {
		if len(loc.Factions) > 0 {
			pop := 0
			for _, p := range loc.Factions {
				pop += p.Population
			}
			out[id] = pop
		}
	}
	return out
}

// vmStats is SettlementStats as the sidecar's JSON-shaped map.
func vmStats(st SettlementStats) map[string]any {
	built := map[string]any{}
	for k, v := range st.Built {
		built[k] = v
	}
	ordered := map[string]any{}
	for k, v := range st.Ordered {
		ordered[k] = v
	}
	return map[string]any{
		"alive": st.Alive, "dead": st.Dead, "beds": st.Beds, "lockers": st.Lockers,
		"food": st.Food, "build_queue": st.BuildQueue, "tasks": st.Tasks,
		"research": st.Research, "techs": st.Techs, "day": st.Day, "tick": st.Tick,
		"built": built, "ordered": ordered, "cancelled": st.Cancelled,
	}
}

// vmEncounter is the pending or last encounter (phase D), nil for none.
func vmEncounter(c *campaign.Campaign) any {
	e := c.Encounter
	if e == nil {
		return nil
	}
	return map[string]any{"fleet": e.FleetID, "doctrine": e.Doctrine, "at": e.At, "pending": e.Pending(),
		"answer": e.Answer, "outcome": e.Outcome, "fuel_cost": e.FuelCost, "boarding": e.BoardingID}
}

// rowRect is the screen rectangle of the first visible row selecting id,
// for vmtest targets.
func (p *factionsPanel) rowRect(id string) (guest.Rect, bool) {
	end := min(len(p.rows), p.scroll+p.visibleRows())
	for i := p.scroll; i < end; i++ {
		if p.rows[i].locID == id {
			y := factionsTop + (i-p.scroll)*factionsRowH
			return guest.Rect{X: 40, Y: float64(y), W: 400, H: factionsRowH}, true
		}
	}
	return guest.Rect{}, false
}

// vmFocused is the entity the last "focus" debug command found.
var vmFocused *ecs.Entity

// vmFind is the first living entity on level matching spec: a blueprint,
// kin:<unit type>, datapad, storage, or structure (a static entity with
// health or storage). Live entities first, then static ones.
func vmFind(level *world.Level, spec string) *ecs.Entity {
	match := func(e *ecs.Entity, static bool) bool {
		switch {
		case strings.HasPrefix(spec, "kin:"):
			k := components.KinOf(e)
			return k != nil && k.Type == strings.TrimPrefix(spec, "kin:")
		case spec == "datapad":
			return e.HasComponent(components.Datapad)
		case spec == "storage":
			return e.HasComponent(components.Storage)
		case spec == "structure":
			return static && (e.HasComponent(rlcomponents.Health) || e.HasComponent(components.Storage))
		}
		return e.Blueprint == spec
	}
	for i, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponent(rlcomponents.Dead) || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			if match(e, i == 1) {
				return e
			}
		}
	}
	return nil
}

// rivalPresences are loc's settlement presences other than a race run's own
// (the player's brood is no rival).
func rivalPresences(c *campaign.Campaign, loc *campaign.Location) []*campaign.FactionPresence {
	ps, _ := settlementPresences(c, loc)
	var out []*campaign.FactionPresence
	for _, p := range ps {
		if p.Faction != campaign.PlayerFaction {
			out = append(out, p)
		}
	}
	return out
}

// vmFactionTable is the campaign's faction instances by key: doctrine,
// group, slot (rival_selection_plan.md).
func vmFactionTable(c *campaign.Campaign) map[string]any {
	out := map[string]any{}
	for _, f := range c.Factions() {
		out[f.Key] = map[string]any{"doctrine": f.Doctrine, "group": f.Group, "slot": f.Slot, "name": f.Name}
	}
	return out
}
