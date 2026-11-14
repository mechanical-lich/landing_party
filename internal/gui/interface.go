package gui

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// The player's doctrine's interface block (phase F step 2;
// docs/developer/factions.md, "The interface block"): the game hands the HUD
// an InterfaceSpec built from data, and the HUD draws it from fixed kinds
// implemented here. An empty spec is the colony's interface as it was.

// InterfaceSpec is what the HUD shows for the player's doctrine.
type InterfaceSpec struct {
	// Resources is the resource bar's slot IDs, in order.
	Resources []string
	// UnitLabel titles the unit modal; UnitsLabel heads the population tab
	// ("Colonists: n").
	UnitLabel, UnitsLabel string
	// ResearchLabel titles the research modal.
	ResearchLabel string
	// Orders is the Orders category, in order.
	Orders []OrderSpec
	// UnitModal is the unit modal's sections, in order.
	UnitModal []string
}

// OrderSpec is one Orders entry: an order ID from the registry, with the
// doctrine's label and description when it gives them.
type OrderSpec struct {
	ID, Label, Description string
}

// Unit modal section kinds. identity: the name and Take Control; type_tier:
// the unit's type and print tier (a line the game supplies); stats: attack,
// defence and stat levels; needs: hunger and exhaustion; equipment: the six
// slots and the storage column to equip from; task_filters: Self-Defend and
// the task toggles; inventory: the Carrying column and Pickup.
const (
	SectionIdentity    = "identity"
	SectionTypeTier    = "type_tier"
	SectionStats       = "stats"
	SectionNeeds       = "needs"
	SectionEquipment   = "equipment"
	SectionTaskFilters = "task_filters"
	SectionInventory   = "inventory"
)

// UnitModalSections is every section kind the unit modal draws.
var UnitModalSections = []string{SectionIdentity, SectionTypeTier, SectionStats, SectionNeeds,
	SectionEquipment, SectionTaskFilters, SectionInventory}

// defaultUnitModal is the colony's unit modal (the colonist modal as it was).
var defaultUnitModal = []string{SectionIdentity, SectionStats, SectionNeeds, SectionEquipment,
	SectionTaskFilters, SectionInventory}

// CursorModeConvert lays the doctrine's conversion tile (the hive's resin,
// the autonz' wire) where the player clicks.
const CursorModeConvert CursorModeType = "convert"

// CursorModeDeploy sends the nearest idle unit whose type deploys (the
// autonz' relay drone) to fold into its structure where the player clicks.
const CursorModeDeploy CursorModeType = "deploy"

// CursorModePost sends the nearest idle guard to hold the clicked tile
// (the player in the director's guard rung's place).
const CursorModePost CursorModeType = "post"

// CursorModeTakeOver is no cursor mode: the take_over order opens the
// world's panel (the doctrine's interface.world_panel, the take-over
// switches) and the cursor returns to the default.
const CursorModeTakeOver CursorModeType = "take_over"

// CursorModeExpandHull grows the clicked deck of a saucer to its next ring;
// CursorModeAddDeck adds a deck above its top one (phase F step 6d, the
// greys).
const (
	CursorModeExpandHull CursorModeType = "expand_hull"
	CursorModeAddDeck    CursorModeType = "add_deck"
)

// orderRegistry is every order an Orders entry may name, with its default
// words (the colony's). The words of a race's own order (convert) come from
// its doctrine; these defaults are only the fallback.
var orderRegistry = func() map[string]hudBuildOrderItem {
	m := map[string]hudBuildOrderItem{}
	for _, it := range buildOrderItems {
		m[it.id] = it
	}
	m["convert"] = hudBuildOrderItem{id: "convert", label: "Convert", description: "Click ground to convert it.", mode: CursorModeConvert}
	m["post"] = hudBuildOrderItem{id: "post", label: "Post Guard", description: "Click ground to post the nearest idle guard there.", mode: CursorModePost}
	m["deploy"] = hudBuildOrderItem{id: "deploy", label: "Deploy", description: "Click ground to deploy a unit there.", mode: CursorModeDeploy}
	m["take_over"] = hudBuildOrderItem{id: "take_over", label: "Take Over", description: "Choose what you run on this world yourself.", mode: CursorModeTakeOver} // DRAFT
	m["expand_hull"] = hudBuildOrderItem{id: "expand_hull", label: "Expand Hull", description: "Click a deck to grow its hull to the next ring.", mode: CursorModeExpandHull} // DRAFT
	m["add_deck"] = hudBuildOrderItem{id: "add_deck", label: "Add Deck", description: "Click to add a deck above the top one.", mode: CursorModeAddDeck}                      // DRAFT
	return m
}()

// ValidOrder reports whether id names an order in the registry.
func ValidOrder(id string) bool { _, ok := orderRegistry[id]; return ok }

// ValidUnitModalSection reports whether kind is a unit modal section kind.
func ValidUnitModalSection(kind string) bool {
	for _, k := range UnitModalSections {
		if k == kind {
			return true
		}
	}
	return false
}

// SetInterface applies the doctrine's interface to the HUD: the resource
// bar's slots, the labels, the Orders category and the unit modal's
// sections. Called by the game with the build menu.
func (h *HUDScreen) SetInterface(spec InterfaceSpec) {
	h.iface = spec
	if h.resourceBar != nil {
		ids := spec.Resources
		if len(ids) == 0 {
			ids = defaultResources
		}
		h.resourceBar.Clear()
		for _, id := range ids {
			h.resourceBar.AddResource(id, nil, 0)
		}
	}
	if h.researchModal != nil {
		h.researchModal.Title = orDefault(spec.ResearchLabel, "Research")
	}
	if h.colonistModal != nil {
		h.colonistModal.Title = orDefault(spec.UnitLabel, "Colonist")
	}
	h.rebuildBuildCategories()
}

// defaultResources is the colony's resource bar.
var defaultResources = []string{"metal_ore", "crystal", "food", "biomass"}

// orderItems is the Orders category's entries for the current interface.
func (h *HUDScreen) orderItems() []hudBuildOrderItem {
	if len(h.iface.Orders) == 0 {
		return buildOrderItems
	}
	var out []hudBuildOrderItem
	for _, o := range h.iface.Orders {
		it, ok := orderRegistry[o.ID]
		if !ok {
			continue // the content check refuses an unknown order
		}
		if o.Label != "" {
			it.label = o.Label
		}
		if o.Description != "" {
			it.description = o.Description
		}
		out = append(out, it)
	}
	return out
}

// unitSections is the unit modal's sections for the current interface.
func (h *HUDScreen) unitSections() map[string]bool {
	list := h.iface.UnitModal
	if len(list) == 0 {
		list = defaultUnitModal
	}
	m := map[string]bool{}
	for _, k := range list {
		m[k] = true
	}
	return m
}

// unitsLabel is the population tab's word for the player's people.
func (h *HUDScreen) unitsLabel() string { return orDefault(h.iface.UnitsLabel, "Colonists") }

func orDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

func setShown(e minui.Element, on bool) {
	if e != nil {
		e.SetVisible(on)
	}
}

// UnitModalShown is the sections the unit modal drew last, in kind order.
func (h *HUDScreen) UnitModalShown() []string { return append([]string(nil), h.unitShown...) }

// UnitModalShown is the HUD's.
func (gm *GUIManager) UnitModalShown() []string { return gm.hud.UnitModalShown() }

// ResourceSlots is the resource bar's slot IDs, in order.
func (gm *GUIManager) ResourceSlots() []string {
	var out []string
	if gm.hud.resourceBar != nil {
		for _, it := range gm.hud.resourceBar.Items {
			out = append(out, it.ID)
		}
	}
	return out
}

// ResourceValue is a resource bar slot's value.
func (gm *GUIManager) ResourceValue(id string) int {
	if gm.hud.resourceBar == nil {
		return 0
	}
	return gm.hud.resourceBar.GetResourceValue(id)
}

// OrderIDs is the Orders category's entries, in order.
func (gm *GUIManager) OrderIDs() []string {
	var out []string
	for _, it := range gm.hud.orderItems() {
		out = append(out, it.id)
	}
	return out
}

// ResearchTitle and UnitModalTitle are the modals' titles.
func (gm *GUIManager) ResearchTitle() string  { return gm.hud.researchModal.Title }
func (gm *GUIManager) UnitModalTitle() string { return gm.hud.colonistModal.Title }

// NeverTires reports a body whose exhaustion limit is out of reach (a hive
// drone, an autonz body: they do not sleep), whose exhaustion line is noise.
func NeverTires(nc *components.NeedsComponent) bool { return nc.MaxExhaustion >= 100000 }
