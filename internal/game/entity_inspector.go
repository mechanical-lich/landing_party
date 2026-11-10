package game

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// entityInspector is the read-only inspector: what one entity on the level is
// (name, blueprint, owner, unit type, stats, task, needs, what it carries or
// holds, or what a datapad says) with no actions. Observer mode opens it for
// any click on an entity, where there is no player settlement for the colonist
// modal's equip and task actions to drive; a normal campaign opens it for a
// creature that is not the colony's own (the colonist modal, crafting,
// research and storage modals keep their entities). It refreshes while open,
// so health and task stay live.
type entityInspector struct {
	modal   *minui.Modal
	text    *minui.ScrollingTextArea
	Visible bool

	target *ecs.Entity
	// Title and Lines are what it shows, also the vmtest state's.
	Title string
	Lines []string
}

const (
	eiModalW   = 560
	eiTextH    = 440
	eiCloseW   = 100
	eiCloseH   = 28
	eiTitleBar = 30
	eiModalH   = 6 + eiTextH + 10 + eiCloseH + eiTitleBar + 12
)

func newEntityInspector() *entityInspector {
	ei := &entityInspector{}
	cfg := config.Global()
	ei.modal = minui.NewModal("ei_modal", "Inspect", eiModalW, eiModalH)
	my := (cfg.ScreenHeight - eiModalH) / 2
	if my < 10 {
		my = 10
	}
	ei.modal.SetPosition((cfg.ScreenWidth-eiModalW)/2, my)
	ei.modal.Closeable = false
	ei.text = minui.NewScrollingTextArea("ei_text", eiModalW-24, eiTextH)
	ei.text.SetPosition(12, 6)
	ei.text.Layout()
	ei.modal.AddChild(ei.text)
	closeBtn := minui.NewButton("ei_close", "Close")
	closeBtn.SetPosition((eiModalW-eiCloseW)/2, 6+eiTextH+10)
	closeBtn.SetSize(eiCloseW, eiCloseH)
	closeBtn.OnClick = func() { ei.Close() }
	ei.modal.AddChild(closeBtn)
	return ei
}

// Open shows e, read from the level and campaign as they stand.
func (ei *entityInspector) Open(c *campaign.Campaign, level *world.Level, e *ecs.Entity) {
	if e == nil {
		return
	}
	ei.target = e
	ei.fill(c, level, true)
	ei.Visible = true
}

// Close hides it.
func (ei *entityInspector) Close() {
	ei.Visible = false
	ei.target = nil
}

// Refresh re-reads the open entity, keeping the scroll where it is.
func (ei *entityInspector) Refresh(c *campaign.Campaign, level *world.Level) {
	if !ei.Visible || ei.target == nil {
		return
	}
	ei.fill(c, level, false)
}

func (ei *entityInspector) fill(c *campaign.Campaign, level *world.Level, top bool) {
	ei.Title, ei.Lines = inspectEntity(c, level, ei.target)
	ei.modal.Title = ei.Title
	off := ei.text.ScrollOffset
	ei.text.Clear()
	for _, l := range ei.Lines {
		ei.text.AddText(l)
	}
	if top {
		off = 0
	}
	ei.text.ScrollOffset = min(off, max(0, len(ei.text.Lines)-ei.text.VisibleLines))
}

func (ei *entityInspector) Update() {
	if ei.Visible {
		ei.modal.Update()
	}
}

func (ei *entityInspector) Draw(screen *ebiten.Image) {
	if ei.Visible {
		ei.modal.Draw(screen)
	}
}

// inspectEntity is what the inspector shows for e: a title and its lines,
// section headers among them. It reads and changes nothing (a datapad is not
// revealed, its quest not unlocked).
func inspectEntity(c *campaign.Campaign, level *world.Level, e *ecs.Entity) (string, []string) {
	title := entityDisplayLabel(e)
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	section := func(name string) { out = append(out, "", "— "+name+" —") }

	// Who it belongs to. A hidden-identity faction (the hive) is not named
	// outside an observer run: no blueprint ("hive_..."), faction,
	// settlement ("hive:<loc>"), owner, layer or presence.
	faction := ""
	if e.HasComponent(rlcomponents.Description) {
		faction = e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
	}
	hidden := hiddenFaction(faction)
	if k := components.KinOf(e); k != nil {
		hidden = hidden || hiddenFaction(settlementDoctrineKey(k.Settlement))
	}
	if !hidden {
		add("Blueprint: %s", e.Blueprint)
	}
	if pc, ok := entityPos(e); ok {
		add("At: %d,%d (z %d)", pc.GetX(), pc.GetY(), pc.GetZ())
	} else {
		add("Gone from the level")
	}
	if e.HasComponent(rlcomponents.Dead) {
		add("Dead")
	}

	if faction != "" && !hidden {
		add("Faction: %s", faction)
	}
	if e.HasComponent(components.Settlement) && !hidden {
		if n := e.GetComponent(components.Settlement).(*components.SettlementComponent).Name; n != "" {
			add("Settlement: %s", n)
		}
	}
	if e.HasComponent(components.Storage) && !hidden {
		if o := e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy; o != "" {
			add("Owner: %s", o)
		}
	}
	if k := components.KinOf(e); k != nil {
		kin := k.Morph
		if k.Type != "" {
			kin = strings.TrimSpace(kin + " " + k.Type)
		}
		if kin != "" {
			add("Unit: %s", kin)
		}
		if k.Tier != "" {
			add("Tier: %s", k.Tier)
		}
		if k.Settlement != "" && !e.HasComponent(components.Settlement) && !hidden {
			add("Laid by: %s", k.Settlement)
		}
		if p := findPresence(c, k.Presence); p != nil && !hidden {
			add("Presence: %s (%s, stage %d, %d bodies)", p.ID, p.Faction, p.Stage, p.Population)
		} else if k.Presence != "" && !hidden {
			add("Presence: %s", k.Presence)
		}
		if ups := typeUpgrades(k); len(ups) > 0 {
			add("Upgrades: %s", strings.Join(ups, ", "))
		}
	}
	if e.HasComponent(rlcomponents.Description) {
		if d := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).LongDescription; d != "" {
			out = append(out, d)
		}
	}

	inspectStats(e, add, section)
	inspectActivity(e, add, section)
	inspectCarried(e, add, section)
	inspectStructure(e, add, section)
	inspectDatapad(c, level, e, &out, section)
	return title, out
}

// inspectStats lists a body's health, ability scores, armour, speed, attack
// and work speed, those it has.
func inspectStats(e *ecs.Entity, add func(string, ...any), section func(string)) {
	hasHealth := e.HasComponent(rlcomponents.Health)
	if !hasHealth && !e.HasComponent(rlcomponents.Stats) {
		return
	}
	section("Stats")
	if hasHealth {
		hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		add("Health: %d / %d", hc.Health, hc.MaxHealth)
	}
	inv := inventoryOf(e)
	if e.HasComponent(rlcomponents.Stats) {
		st := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
		add("Str %d  Dex %d  Con %d  Int %d  Wis %d", st.Str, st.Dex, st.Con, st.Int, st.Wis)
		ac := st.AC
		if inv != nil {
			ac += inv.GetDefenseModifier()
		}
		add("AC: %d", ac)
		attack := st.BasicAttackDice
		if inv != nil {
			if w := weaponOf(inv.RightHand); w != nil && w.AttackDice != "" {
				attack = w.AttackDice
				if w.Ranged {
					attack += fmt.Sprintf(" ranged %d", w.Range)
				}
			}
		}
		if attack != "" {
			if st.BaseDamageType != "" {
				attack += " " + st.BaseDamageType
			}
			mod := 0
			if inv != nil {
				mod = inv.GetAttackModifier()
			}
			add("Attack: %s (%+d)", attack, mod)
		}
	}
	if e.HasComponent(rlcomponents.Initiative) {
		ic := e.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent)
		in := ic.DefaultValue
		if ic.OverrideValue > 0 {
			in = ic.OverrideValue
		}
		add("Initiative: %d (ticks a turn)", in)
	}
	if e.HasComponent(components.Worker) {
		w := e.GetComponent(components.Worker).(*components.WorkerComponent)
		m := w.WorkMult
		if m <= 0 {
			m = 1
		}
		add("Work speed: x%.2g", m)
	}
}

// inspectActivity lists a body's role, task or behaviour, and needs.
func inspectActivity(e *ecs.Entity, add func(string, ...any), section func(string)) {
	w, _ := e.GetComponent(components.Worker).(*components.WorkerComponent)
	ai, _ := e.GetComponent(components.FactionAI).(*components.FactionAIComponent)
	needs, _ := e.GetComponent(components.Needs).(*components.NeedsComponent)
	mem, _ := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	if w == nil && ai == nil && needs == nil && mem == nil {
		return
	}
	section("Activity")
	if mem != nil && mem.State != "" {
		add("State: %s", mem.State)
	}
	if w != nil {
		if w.Role != "" {
			add("Role: %s", w.Role)
		}
		if t := w.CurrentTask; t != nil && !t.Completed {
			action := string(t.Action)
			if action == "" {
				action = "move"
			}
			add("Task: %s at %d,%d (z %d)", action, t.X, t.Y, t.Z)
		} else {
			add("Task: none")
		}
	}
	if ai != nil && ai.BehaviorKey != "" {
		add("Behaviour: %s", ai.BehaviorKey)
	}
	if needs != nil {
		if needs.MaxHunger > 0 {
			status := "fed"
			if needs.IsStarving() {
				status = "starving"
			} else if needs.IsHungry() {
				status = "hungry"
			}
			add("Hunger: %d / %d (%s)", needs.Hunger, needs.MaxHunger, status)
		}
		if needs.MaxExhaustion > 0 {
			add("Exhaustion: %d / %d", needs.Exhaustion, needs.MaxExhaustion)
		}
	}
}

// inspectCarried lists what a body wears and carries.
func inspectCarried(e *ecs.Entity, add func(string, ...any), section func(string)) {
	inv := inventoryOf(e)
	if inv == nil {
		return
	}
	section("Equipment")
	worn := false
	for _, sl := range []struct {
		label string
		item  *ecs.Entity
	}{
		{"Right hand", inv.RightHand}, {"Left hand", inv.LeftHand}, {"Head", inv.Head},
		{"Torso", inv.Torso}, {"Legs", inv.Legs}, {"Feet", inv.Feet},
	} {
		if sl.item != nil {
			add("%s: %s", sl.label, itemLabel(sl.item))
			worn = true
		}
	}
	if !worn {
		add("(nothing equipped)")
	}
	section("Carrying")
	if len(inv.Bag) == 0 {
		add("(nothing)")
	}
	for _, it := range inv.Bag {
		add("%s", itemLabel(it))
	}
}

// inspectStructure lists a structure's contents and workings: a container's
// items by count, a station's or research building's kind, power.
func inspectStructure(e *ecs.Entity, add func(string, ...any), section func(string)) {
	if e.HasComponent(components.CraftingStation) {
		add("Crafting station: %s", e.GetComponent(components.CraftingStation).(*components.CraftingStationComponent).StationID)
	}
	if e.HasComponent(components.ResearchBuilding) {
		add("Research building")
	}
	if e.HasComponent(components.Bed) {
		add("Bed")
	}
	if e.HasComponent(components.Powered) {
		add("Unpowered (grid %s)", e.GetComponent(components.Powered).(*components.PoweredComponent).Grid)
	}
	if e.HasComponent(components.Repairable) {
		add("Broken: repairs into %s", e.GetComponent(components.Repairable).(*components.RepairableComponent).Into)
	}
	if e.HasComponent(components.Material) {
		add("Quantity: %d", e.GetComponent(components.Material).(*components.MaterialComponent).Quantity)
	}
	if e.HasComponent(rlcomponents.Item) {
		if d := e.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent).Description; d != "" {
			add("%s", d)
		}
	}
	if !e.HasComponent(components.Storage) {
		return
	}
	sc := e.GetComponent(components.Storage).(*components.StorageComponent)
	slots := fmt.Sprintf("%d slots", len(sc.Items))
	if sc.Capacity > 0 {
		slots = fmt.Sprintf("%d / %d slots", len(sc.Items), sc.Capacity)
	}
	section("Contents (" + slots + ")")
	counts := map[string]int{}
	names := map[string]string{}
	for _, it := range sc.Items {
		n := 1
		if it.HasComponent(components.Material) {
			n = it.GetComponent(components.Material).(*components.MaterialComponent).Quantity
		}
		counts[it.Blueprint] += n
		names[it.Blueprint] = entityDisplayLabel(it)
	}
	bps := make([]string, 0, len(counts))
	for bp := range counts {
		bps = append(bps, bp)
	}
	sort.Strings(bps)
	if len(bps) == 0 {
		add("(empty)")
	}
	for _, bp := range bps {
		add("%s x%d", names[bp], counts[bp])
	}
	if len(sc.AllowedTags) > 0 {
		add("Accepts: %s", strings.Join(sc.AllowedTags, ", "))
	}
}

// inspectDatapad adds a datapad's full text: what a colonist picking it up
// reads (datapadReading), or a log console's records.
func inspectDatapad(c *campaign.Campaign, level *world.Level, e *ecs.Entity, out *[]string, section func(string)) {
	dp, _ := e.GetComponent(components.Datapad).(*components.DatapadComponent)
	if dp == nil {
		return
	}
	if dp.Log {
		section("Station log")
		recs := stationLogText(c, level)
		if len(recs) == 0 {
			*out = append(*out, "(no records)")
		}
		for i, r := range recs {
			if i > 0 {
				*out = append(*out, "")
			}
			*out = append(*out, r)
		}
		return
	}
	section("Datapad")
	text, lead := datapadReading(c, dp)
	if text == "" && lead == "" {
		*out = append(*out, "(unreadable)")
	}
	if text != "" {
		*out = append(*out, text)
	}
	if lead != "" {
		*out = append(*out, "", "Lead: "+lead)
	}
}

// typeUpgrades are the names of the upgrades a typed body's presence knows
// for its type: the evolve entries of its type, and every flier's speed.
func typeUpgrades(k *components.KinComponent) []string {
	if k == nil || k.Type == "" {
		return nil
	}
	var out []string
	keys := append([]string(nil), techsOf(k.Presence)...)
	sort.Strings(keys)
	for _, key := range keys {
		r, ok := research.GetTech(key)
		if !ok || r.Evolve == nil {
			continue
		}
		if r.Evolve.Type == k.Type || (r.Evolve.FlierSpeed > 0 && k.Morph == "flier") {
			name := r.Name
			if name == "" {
				name = key
			}
			out = append(out, name)
		}
	}
	return out
}

// findPresence is the presence id anywhere in the campaign, or nil.
func findPresence(c *campaign.Campaign, id string) *campaign.FactionPresence {
	if c == nil || id == "" {
		return nil
	}
	for _, loc := range c.Locations {
		for i := range loc.Factions {
			if loc.Factions[i].ID == id {
				return &loc.Factions[i]
			}
		}
	}
	return nil
}

func inventoryOf(e *ecs.Entity) *rlcomponents.InventoryComponent {
	inv, _ := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	return inv
}

func weaponOf(item *ecs.Entity) *rlcomponents.WeaponComponent {
	if item == nil || !item.HasComponent(rlcomponents.Weapon) {
		return nil
	}
	switch w := item.GetComponent(rlcomponents.Weapon).(type) {
	case *rlcomponents.WeaponComponent:
		return w
	case rlcomponents.WeaponComponent:
		return &w
	}
	return nil
}

// itemLabel names an item, a stack with its count.
func itemLabel(it *ecs.Entity) string {
	name := entityDisplayLabel(it)
	if it.HasComponent(components.Material) {
		name = fmt.Sprintf("%s x%d", name, it.GetComponent(components.Material).(*components.MaterialComponent).Quantity)
	}
	return name
}

// settlementDoctrineKey is the doctrine key of a faction settlement's name
// ("hive" of "hive:<loc>"), "" for the player's.
func settlementDoctrineKey(name string) string {
	if i := strings.IndexByte(name, ':'); i > 0 {
		return name[:i]
	}
	return ""
}
