package game

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The doctrine's interface block on the game side (phase F step 2;
// docs/developer/factions.md, "The interface block"). Every list and label a
// race shows is data in its doctrine's settlement.interface; this file turns
// it into the HUD's spec and supplies the values its registries name. An
// absent block is the colony's interface as it was.

// interfaceSpec is the HUD's spec for st's interface.
func interfaceSpec(st *campaign.DoctrineSettlement) gui.InterfaceSpec {
	ui := st.UI()
	spec := gui.InterfaceSpec{UnitLabel: ui.UnitLabel, UnitsLabel: ui.UnitsLabel,
		ResearchLabel: ui.ResearchLabel, UnitModal: ui.UnitModal}
	for _, r := range ui.Resources {
		spec.Resources = append(spec.Resources, r.ID)
	}
	for _, o := range ui.Orders {
		spec.Orders = append(spec.Orders, gui.OrderSpec{ID: o.ID, Label: o.Label, Description: o.Description})
	}
	return spec
}

// unitsWord is the player's people in a hint ("colonists", "brood").
func (s *MainState) unitsWord() string {
	if w := s.doctrineSettlement().UI().UnitsLabel; w != "" {
		return strings.ToLower(w)
	}
	return "colonists"
}

// unitWord is one of the player's people, for a fallback name ("Colonist").
func (s *MainState) unitWord() string {
	if w := s.doctrineSettlement().UI().UnitLabel; w != "" {
		return w
	}
	return "Colonist"
}

// Resource bar sources: what a slot counts. storage: the slot's ID as a
// blueprint in storage; food: edible items in storage; larvae: the player
// settlement's larvae on the level; conversion: its converted share of the
// patch, in percent.
var resourceSources = map[string]bool{"storage": true, "food": true, "larvae": true, "conversion": true}

// colonyResourceSource is how the colony's (default) slots count: food by
// edibles, every other slot by blueprint.
func colonyResourceSource(id string) string {
	if id == "food" {
		return "food"
	}
	return "storage"
}

// resourceValues is the resource bar's values for the player's settlement on
// this level. The colony counts every storage on the level, as it always
// did; a race run counts only its own settlement's.
func (s *MainState) resourceValues() map[string]int {
	ui := s.doctrineSettlement().UI()
	slots := ui.Resources
	if len(slots) == 0 {
		for _, id := range []string{"metal_ore", "crystal", "food", "fuel", "biomass"} {
			slots = append(slots, campaign.InterfaceResource{ID: id, Source: colonyResourceSource(id)})
		}
	}
	owner := ""
	if spreadRun() {
		owner = s.colonyName()
	}
	stock, food := map[string]int{}, 0
	for _, entity := range s.level.Entities {
		if !entity.HasComponent(components.Storage) {
			continue
		}
		st := entity.GetComponent(components.Storage).(*components.StorageComponent)
		if owner != "" && st.OwnedBy != owner {
			continue
		}
		for _, item := range st.Items {
			if item.Blueprint != "" {
				if item.HasComponent(components.Material) {
					stock[item.Blueprint] += item.GetComponent(components.Material).(*components.MaterialComponent).Quantity
				} else {
					stock[item.Blueprint]++
				}
			}
			if item.HasComponent(rlcomponents.Food) && !item.HasComponent(components.Material) {
				food++
			}
		}
	}
	out := map[string]int{}
	for _, r := range slots {
		src := r.Source
		if src == "" {
			src = colonyResourceSource(r.ID)
		}
		switch src {
		case "storage":
			out[r.ID] = stock[r.ID]
		case "food":
			out[r.ID] = food
		case "larvae":
			out[r.ID] = len(hiveLarvae(s.level, s.colonyName()))
		case "conversion":
			out[r.ID] = s.conversionPercent()
		}
	}
	return out
}

// conversionPercent is the player's presence's converted share of its patch
// on this level, in percent (0 with none, or no conversion tile).
func (s *MainState) conversionPercent() int {
	if s.campaign == nil {
		return 0
	}
	loc := s.campaign.Locations[s.locationID]
	if loc == nil {
		return 0
	}
	p := loc.Presence(campaign.PlayerFaction)
	d := campaign.PlayerDoctrine(s.campaign)
	if p == nil || d == nil || d.Conversion.Tile == "" {
		return 0
	}
	cp := *p
	reconcileConversion(s.level, d, &cp)
	return int(cp.Conversion*100 + 0.5)
}

// unitTypeLine is the unit modal's type_tier line for e: its type's name
// (the doctrine's) and its print tier, "" for an untyped body.
func unitTypeLine(c *campaign.Campaign, e *ecs.Entity) string {
	k := components.KinOf(e)
	if k == nil || k.Type == "" {
		return ""
	}
	name := k.Type
	if d := campaign.PlayerDoctrine(c); d != nil && d.Types[k.Type] != nil && d.Types[k.Type].Name != "" {
		name = d.Types[k.Type].Name
	}
	if k.Tier != "" {
		return name + " · " + k.Tier
	}
	return name
}

// crewGroups are the crew panel's grouping keys.
var crewGroups = map[string]bool{"": true, "type": true, "type_tier": true}

// crewGroupKey is e's crew panel group under key ("" for none).
func crewGroupKey(c *campaign.Campaign, key string, e *ecs.Entity) string {
	k := components.KinOf(e)
	if key == "" || k == nil || k.Type == "" {
		return ""
	}
	name := k.Type
	if d := campaign.PlayerDoctrine(c); d != nil && d.Types[k.Type] != nil && d.Types[k.Type].Name != "" {
		name = d.Types[k.Type].Name
	}
	if key == "type_tier" && k.Tier != "" {
		name += " · " + k.Tier
	}
	return name
}

// validateInterfaces checks every doctrine's interface block against the
// registries: unknown resource sources, orders, unit modal sections, crew
// group keys, panel row kinds, value sources and actions fail loudly, as a
// missing blueprint does.
func validateInterfaces() error {
	fd := campaign.LoadFactionData()
	keys := make([]string, 0, len(fd.Doctrines))
	for k := range fd.Doctrines {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		d := fd.Doctrines[k]
		if d.Settlement == nil || d.Settlement.Interface == nil {
			continue
		}
		if err := validateInterface(d.Settlement.Interface); err != nil {
			return fmt.Errorf("%s: %s interface: %w", campaign.FactionsPath, k, err)
		}
		for _, key := range d.Settlement.Interface.HiddenResearch {
			if !d.Settlement.CanResearch(key) {
				return fmt.Errorf("%s: %s interface: hidden_research %q is not in its research", campaign.FactionsPath, k, key)
			}
		}
		if err := validateRungGroups(d.Settlement); err != nil {
			return fmt.Errorf("%s: %s director: %w", campaign.FactionsPath, k, err)
		}
	}
	return nil
}

// validateInterface checks one interface block.
func validateInterface(ui *campaign.DoctrineInterface) error {
	for _, r := range ui.Resources {
		if r.ID == "" || r.Source != "" && !resourceSources[r.Source] {
			return fmt.Errorf("resource %q: unknown source %q", r.ID, r.Source)
		}
	}
	for _, o := range ui.Orders {
		if !gui.ValidOrder(o.ID) {
			return fmt.Errorf("unknown order %q", o.ID)
		}
	}
	for _, sec := range ui.UnitModal {
		if !gui.ValidUnitModalSection(sec) {
			return fmt.Errorf("unknown unit modal section %q", sec)
		}
	}
	if !crewGroups[ui.CrewGroup] {
		return fmt.Errorf("unknown crew_group %q", ui.CrewGroup)
	}
	names := make([]string, 0, len(ui.Panels))
	for b := range ui.Panels {
		names = append(names, b)
	}
	sort.Strings(names)
	for _, b := range names {
		if err := validatePanel(ui.Panels[b]); err != nil {
			return fmt.Errorf("panel %q: %w", b, err)
		}
	}
	if ui.WorldPanel != nil {
		if err := validateWorldPanel(ui.WorldPanel); err != nil {
			return fmt.Errorf("world_panel: %w", err)
		}
	}
	if ui.Order("take_over") != nil && ui.WorldPanel == nil {
		return fmt.Errorf("order take_over: no world_panel to open")
	}
	return nil
}

// countsLine is counts by key as "drone 4, scout 2" ("-" for none).
func countsLine(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// validateRungGroups checks that every director rung's group is one the
// interface declares (a group the player could never toggle is a typo).
func validateRungGroups(st *campaign.DoctrineSettlement) error {
	if st.Director == nil {
		return nil
	}
	known := map[string]bool{}
	for _, g := range st.UI().Groups {
		known[g.ID] = true
	}
	for _, p := range st.Director.Priorities {
		if p.Group != "" && !known[p.Group] {
			return fmt.Errorf("rung %q: group %q is not in interface.groups", p.ID, p.Group)
		}
	}
	return nil
}
