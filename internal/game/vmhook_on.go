//go:build ebitenginevmguest

package game

// VM play-testing hook, linked only into guest builds (-tags ebitenginevmguest).
// When LP_VMTEST_SOCKET names a unix socket, the game dials it and answers
// JSON-line requests from cmd/vmtest: a state summary, named UI targets and
// debug commands. Requests are served under a mutex that Game.Update and
// Game.Draw also hold, so they cost no ticks and never race the game.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
	"github.com/mechanical-lich/landing_party/internal/vmproto"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/state"
	"github.com/mechanical-lich/mlge/ui/minui"
)

var vm struct {
	mu     sync.Mutex
	active bool
	cur    state.StateInterface
}

func vmNoop() {}

// vmStart dials the sidecar socket when the host asked for one.
func vmStart(*Game) {
	path := os.Getenv("LP_VMTEST_SOCKET")
	if path == "" {
		return
	}
	vmStartServer(path)
}

// vmStartServer is the sidecar entry point. make vmtest-check looks for this
// symbol to prove release binaries do not contain the hook.
func vmStartServer(path string) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		log.Printf("vmtest: sidecar dial: %v", err)
		return
	}
	vm.active = true
	go vmServe(conn)
}

func vmServe(conn net.Conn) {
	r := bufio.NewReaderSize(conn, 1<<16)
	enc := json.NewEncoder(conn)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return
		}
		var req vmproto.Request
		var rep vmproto.Reply
		if err := json.Unmarshal(line, &req); err != nil {
			rep.Error = err.Error()
		} else {
			vm.mu.Lock()
			rep = vmHandle(req)
			vm.mu.Unlock()
		}
		if err := enc.Encode(rep); err != nil {
			return
		}
	}
}

// vmLock is taken by Game.Update and Game.Draw.
func vmLock() func() {
	if !vm.active {
		return vmNoop
	}
	vm.mu.Lock()
	return vm.mu.Unlock
}

// vmNote records the state running this tick; StateMachine has no getter.
func vmNote(s state.StateInterface) { vm.cur = s }

func vmHandle(req vmproto.Request) (rep vmproto.Reply) {
	defer func() {
		if r := recover(); r != nil {
			rep = vmproto.Reply{Error: fmt.Sprintf("guest panic in %s: %v", req.Op, r)}
		}
	}()
	switch req.Op {
	case vmproto.OpState:
		return vmproto.Reply{OK: true, State: vmState()}
	case vmproto.OpTarget:
		x, y, w, h, err := vmTarget(req.Name)
		if err != nil {
			return vmproto.Reply{Error: err.Error()}
		}
		return vmproto.Reply{OK: true, X: x, Y: y, W: w, H: h}
	case vmproto.OpTargets:
		return vmproto.Reply{OK: true, Targets: vmTargets()}
	case vmproto.OpDebug:
		out, err := vmDebug(req.Cmd)
		if err != nil {
			return vmproto.Reply{Error: err.Error()}
		}
		return vmproto.Reply{OK: true, Output: out}
	}
	return vmproto.Reply{Error: "unknown op " + req.Op}
}

// vmWorld returns the campaign's world manager behind the current state.
func vmWorld() *WorldManager {
	switch s := vm.cur.(type) {
	case *DashboardState:
		return s.wm
	case *MainState:
		return s.wm
	}
	return nil
}

// ---- state summary ----

var titleScreenNames = map[titleScreen]string{screenMain: "main", screenNewSettlement: "new", screenLoad: "load"}

func vmState() map[string]any {
	out := map[string]any{}
	var modals []string
	switch s := vm.cur.(type) {
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
		if s.storageInspector != nil && s.storageInspector.Visible {
			modals = append(modals, "inspector")
		}
		if p := s.starMapPanel(); p != nil {
			if p.scanOpen {
				modals = append(modals, "scan")
			}
			if loc := p.selectedLocation(); loc != nil {
				out["selected_location"] = map[string]any{"id": loc.ID, "name": loc.Name, "fuel_cost": s.campaign.FuelCost(loc.ID)}
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
	if wm := vmWorld(); wm != nil {
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
		"day": c.Day,
	}
	if loc != nil {
		camp["location_name"] = loc.Name
	}
	techs := append([]string(nil), c.KnownTechs...)
	sort.Strings(techs)
	camp["known_techs"] = vmList(techs)
	out["campaign"] = camp
	out["day"] = c.Day

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
func vmBridge() *tutorialBridge {
	wm := vmWorld()
	if wm == nil {
		return nil
	}
	b := &tutorialBridge{c: wm.Campaign, wm: wm}
	switch s := vm.cur.(type) {
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

func vmRectOf(el minui.Element) (float64, float64, float64, float64, bool) {
	if el == nil || !el.IsVisible() {
		return 0, 0, 0, 0, false
	}
	for p := el.GetParent(); p != nil; p = p.GetParent() {
		if !p.IsVisible() {
			return 0, 0, 0, 0, false
		}
	}
	x, y := el.GetAbsolutePosition()
	r := el.GetBounds()
	if r.Width <= 0 || r.Height <= 0 {
		return 0, 0, 0, 0, false
	}
	return float64(x), float64(y), float64(r.Width), float64(r.Height), true
}

// vmListRow is row n of a list box, assuming it is not scrolled.
func vmListRow(lb *minui.ListBox, n int) (float64, float64, float64, float64, bool) {
	if lb == nil || !lb.IsVisible() || n < 0 || n >= len(lb.Items) {
		return 0, 0, 0, 0, false
	}
	content := minui.GetContentBounds(lb.GetBounds(), lb.GetComputedStyle())
	const rowH = 20
	ax, ay := lb.GetAbsolutePosition()
	b := lb.GetBounds()
	x := content.X + ax - b.X
	y := content.Y + ay - b.Y + n*rowH
	if y+rowH > content.Y+ay-b.Y+content.Height {
		return 0, 0, 0, 0, false
	}
	return float64(x), float64(y), float64(content.Width), rowH, true
}

// vmTabRect mirrors minui's horizontal tab strip layout.
func vmTabRect(tb *minui.TabBar, id string) (float64, float64, float64, float64, bool) {
	if tb == nil || !tb.IsVisible() {
		return 0, 0, 0, 0, false
	}
	x, y := tb.GetAbsolutePosition()
	h := tb.GetBounds().Height
	st := tb.Style
	for _, t := range tb.Tabs {
		w := st.Pad * 2
		if t.Icon != nil {
			w += t.Icon.ScaledWidth() + 4
		}
		w += len(t.Label) * (st.FontSize*6/10 + 1)
		w = max(w, st.MinTabW)
		if t.ID == id {
			return float64(x), float64(y), float64(w), float64(h), true
		}
		x += w
	}
	return 0, 0, 0, 0, false
}

// vmTarget resolves a script target name to a rectangle (a ring becomes the
// square around its centre).
func vmTarget(name string) (x, y, w, h float64, err error) {
	kind, rest, _ := strings.Cut(name, ":")
	ok := false
	fail := func() (float64, float64, float64, float64, error) {
		return 0, 0, 0, 0, fmt.Errorf("target %q does not resolve on this screen", name)
	}
	fromHL := func(hl highlight, found bool) (float64, float64, float64, float64, bool) {
		if !found {
			return 0, 0, 0, 0, false
		}
		if hl.radius > 0 {
			return float64(hl.x - 1), float64(hl.y - 1), 2, 2, true
		}
		return float64(hl.x), float64(hl.y), float64(hl.w), float64(hl.h), true
	}
	switch s := vm.cur.(type) {
	case *TitleState:
		if kind == "title" {
			x, y, w, h, ok = vmTitleTarget(s, rest)
		}
	case *DashboardState:
		switch kind {
		case "tab":
			x, y, w, h, ok = vmTabRect(s.tabBar, rest)
		case "row":
			x, y, w, h, ok = vmDashRow(s, rest)
		case "gi":
			x, y, w, h, ok = vmGITarget(s, rest)
		}
	case *MainState:
		switch kind {
		case "hud":
			if s.guiManager != nil {
				ix, iy, iw, ih, found := s.guiManager.ElementBounds(rest)
				x, y, w, h, ok = float64(ix), float64(iy), float64(iw), float64(ih), found && iw > 0 && ih > 0
			}
		case "menu":
			// menu:<id> is a button in the HUD main menu (Esc): save, load, newgame, quit.
			x, y, w, h, ok = vmModalTarget(s, "mainMenu:"+rest)
		case "modal":
			x, y, w, h, ok = vmModalTarget(s, rest)
		case "tile":
			x, y, w, h, ok = vmTileTarget(s, rest)
		case "settle":
			// settle:dx,dy is the tile dx,dy from the colony's centre, on the
			// level the camera shows.
			if st := s.MainSettlement; st != nil {
				var dx, dy int
				if _, err := fmt.Sscanf(rest, "%d,%d", &dx, &dy); err == nil {
					x, y, w, h, ok = vmTileTarget(s, fmt.Sprintf("%d,%d,%d", st.CenterX+dx, st.CenterY+dy, s.camera.Z))
				}
			}
		case "btn":
			if rest == "dashboard" && s.dashboardBtn != nil {
				x, y, w, h, ok = vmRectOf(s.dashboardBtn)
			}
		}
	}
	if !ok {
		if b := vmBridge(); b != nil {
			switch kind {
			case "dash":
				x, y, w, h, ok = fromHL(b.dashHighlight(rest))
			case "star":
				x, y, w, h, ok = fromHL(b.starHighlight(rest))
			case "ship":
				x, y, w, h, ok = fromHL(b.shipHoldHighlight())
			case "inspector":
				if rest == "beam" || rest == "list" {
					x, y, w, h, ok = fromHL(b.inspectorHighlight(rest))
				} else {
					x, y, w, h, ok = vmInspectorTarget(b.openInspector(), rest)
				}
			case "region":
				x, y, w, h, ok = fromHL(b.regionHighlight(rest))
			case "entity":
				if bp, n, has := strings.Cut(rest, ":"); has {
					x, y, w, h, ok = vmNthEntity(b, bp, n)
				} else {
					x, y, w, h, ok = fromHL(b.entityHighlight(rest))
				}
			case "tutorial":
				x, y, w, h, ok = vmTutorialTarget(rest)
			case "popup":
				x, y, w, h, ok = vmPopupTarget(b.wm, rest)
			}
		}
	}
	if !ok {
		return fail()
	}
	return x, y, w, h, nil
}

func vmTitleTarget(s *TitleState, name string) (float64, float64, float64, float64, bool) {
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
					return vmListRow(s.saveListBox, i)
				}
			}
		}
	}
	if el == nil {
		return 0, 0, 0, 0, false
	}
	if t, ok := el.(*minui.Toggle); ok {
		// A toggle's bounds can be zero-sized; its box sits at its position.
		x, y := t.GetAbsolutePosition()
		r := t.GetBounds()
		return float64(x), float64(y), float64(max(r.Width, 20)), float64(max(r.Height, 20)), true
	}
	return vmRectOf(el)
}

// vmDashRow resolves row:locList:<location id>, row:shipList:<n>,
// row:planList:<n> and row:crew:<n> on the dashboard.
func vmDashRow(d *DashboardState, rest string) (float64, float64, float64, float64, bool) {
	list, key, _ := strings.Cut(rest, ":")
	n, nerr := strconv.Atoi(key)
	if p := d.starMapPanel(); p != nil && d.tabs[d.active].id == "starmap" {
		switch list {
		case "locList":
			if r, ok := p.rowRect(key); ok {
				return float64(r.X), float64(r.Y), float64(r.Width), float64(r.Height), true
			}
			return 0, 0, 0, 0, false
		case "shipList":
			if nerr == nil {
				return vmListRow(p.shipList, n)
			}
		case "planList":
			if nerr == nil {
				return vmListRow(p.planList, n)
			}
		}
	}
	return 0, 0, 0, 0, false
}

// vmGITarget resolves the Global Inventory tab's buttons: gi:beam_down, gi:beam_up.
func vmGITarget(d *DashboardState, name string) (float64, float64, float64, float64, bool) {
	if d.tabs[d.active].id != "inventory" {
		return 0, 0, 0, 0, false
	}
	p, ok := d.tabs[d.active].panel.(*globalInvPanel)
	if !ok {
		return 0, 0, 0, 0, false
	}
	switch name {
	case "beam_down":
		return vmRectOf(p.beamDownBtn)
	case "beam_up":
		return vmRectOf(p.beamUpBtn)
	}
	return 0, 0, 0, 0, false
}

// vmInspectorTarget resolves inspector:amount, :all, :close and :item:<blueprint>.
func vmInspectorTarget(insp *StorageInspectorModal, name string) (float64, float64, float64, float64, bool) {
	if insp == nil {
		return 0, 0, 0, 0, false
	}
	switch {
	case name == "amount":
		return vmRectOf(insp.amountInput)
	case name == "close":
		return vmRectOf(insp.modal.FindChildByID("si_close"))
	case name == "all":
		return vmRectOf(insp.modal.FindChildByID("si_all"))
	case strings.HasPrefix(name, "item:"):
		bp := strings.TrimPrefix(name, "item:")
		for i, b := range insp.invBPs {
			if b == bp {
				return vmListRow(insp.invList, i)
			}
		}
	}
	return 0, 0, 0, 0, false
}

func vmTutorialTarget(name string) (float64, float64, float64, float64, bool) {
	b := activeTutorial
	if b == nil || b.panel == nil {
		return 0, 0, 0, 0, false
	}
	p := b.panel
	switch name {
	case "skip":
		if !p.ending && !p.collapsed && !p.inspector {
			return vmRectOf(p.skipBtn)
		}
	case "manual":
		if step := b.dir.Current(); !p.ending && !p.collapsed && !p.inspector && step != nil && step.Topic != "" {
			return vmRectOf(p.manualBtn)
		}
	case "close":
		if p.ending && !p.collapsed {
			return vmRectOf(p.closeBtn)
		}
	case "collapse":
		if !p.inspector {
			return vmRectOf(p.collapseBtn)
		}
	}
	return 0, 0, 0, 0, false
}

func vmPopupTarget(wm *WorldManager, name string) (float64, float64, float64, float64, bool) {
	if wm == nil || wm.popup == nil || wm.ActiveIncident() == nil {
		return 0, 0, 0, 0, false
	}
	id := map[string]string{"pause": "incident_pause", "travel": "incident_travel", "ignore": "incident_ignore"}[name]
	if id == "" {
		return 0, 0, 0, 0, false
	}
	return vmRectOf(wm.popup.modal.FindChildByID(id))
}

// vmModalTarget resolves targets inside a HUD modal:
//
//	modal:<modal>:close                   the title-bar X
//	modal:<modal>:<element id>            any element
//	modal:<modal>:row:<list id>:<n>       row n of a list box
//	modal:<modal>:item:<list id>:<text>   first row whose text contains text
func vmModalTarget(s *MainState, spec string) (float64, float64, float64, float64, bool) {
	none := func() (float64, float64, float64, float64, bool) { return 0, 0, 0, 0, false }
	if s.guiManager == nil {
		return none()
	}
	modal, rest, _ := strings.Cut(spec, ":")
	m := s.guiManager.ModalElement(modal, "")
	if m == nil {
		return none()
	}
	switch {
	case rest == "close":
		mx, my := m.GetAbsolutePosition()
		return float64(mx + m.GetBounds().Width - 28), float64(my + 4), 24, 22, true
	case strings.HasPrefix(rest, "row:") || strings.HasPrefix(rest, "item:"):
		kind, r, _ := strings.Cut(rest, ":")
		listID, key, _ := strings.Cut(r, ":")
		lb, ok := s.guiManager.ModalElement(modal, listID).(*minui.ListBox)
		if !ok {
			return none()
		}
		if kind == "row" {
			n, err := strconv.Atoi(key)
			if err != nil {
				return none()
			}
			return vmListRow(lb, n)
		}
		for i, it := range lb.Items {
			if strings.Contains(strings.ToLower(it), strings.ToLower(key)) {
				return vmListRow(lb, i)
			}
		}
		return none()
	}
	return vmRectOf(s.guiManager.ModalElement(modal, rest))
}

// vmNthEntity resolves entity:<blueprint>:<n>, the nth living entity of that
// blueprint on screen at the camera's level, in level order (0-based).
func vmNthEntity(b *tutorialBridge, bp, nth string) (float64, float64, float64, float64, bool) {
	n, err := strconv.Atoi(nth)
	s := b.liveSite()
	if err != nil || s == nil || s.level == nil {
		return 0, 0, 0, 0, false
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
				return float64(hl.x), float64(hl.y), float64(hl.w), float64(hl.h), ok
			}
			n--
		}
	}
	return 0, 0, 0, 0, false
}

// vmTileTarget resolves tile:x,y,z to that tile's screen square.
func vmTileTarget(s *MainState, rest string) (float64, float64, float64, float64, bool) {
	parts := strings.Split(rest, ",")
	if len(parts) != 3 {
		return 0, 0, 0, 0, false
	}
	var v [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, 0, 0, 0, false
		}
		v[i] = n
	}
	if !s.camera.Contains(v[0], v[1], v[2]) || v[2] != s.camera.Z {
		return 0, 0, 0, 0, false
	}
	x, y := s.camera.WorldToScreenF(v[0], v[1])
	return float64(x), float64(y), float64(s.camera.TileW), float64(s.camera.TileH), true
}

// vmCandidates are names worth listing when they resolve.
func vmCandidates() []string {
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
	switch s := vm.cur.(type) {
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
	if b := vmBridge(); b != nil {
		if insp := b.openInspector(); insp != nil {
			for _, bp := range insp.invBPs {
				names = append(names, "inspector:item:"+bp)
			}
		}
	}
	return names
}

func vmTargets() []string {
	var out []string
	for _, n := range vmCandidates() {
		if _, _, _, _, err := vmTarget(n); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// ---- debug commands ----

// vmDebug runs "cheat <console command>" on the live level, or
// "incident <location id|ship>" to raise the away-damage popup for a level.
func vmDebug(cmd string) (string, error) {
	verb, rest, _ := strings.Cut(strings.TrimSpace(cmd), " ")
	rest = strings.TrimSpace(rest)
	switch verb {
	case "cheat":
		s, ok := vm.cur.(*MainState)
		if !ok || s.cheatModal == nil {
			return "", fmt.Errorf("cheat needs a site or the ship on screen")
		}
		before := len(message.MessageLog)
		s.cheatModal.runCommand(rest)
		return strings.Join(message.MessageLog[before:], "\n"), nil
	case "incident":
		wm := vmWorld()
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
