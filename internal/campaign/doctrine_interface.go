package campaign

// The doctrine's interface block (phase F step 2; playable_races_plan.md,
// docs/developer/factions.md "The interface block"): what the player's HUD,
// unit modal, crew panel and structure panels show for a race, as data over a
// fixed set of kinds implemented in Go. Every list or label a race needs is
// here; no race's string or list lives in Go. An absent block (or field) is
// the colony's interface as it was before doctrines.

// DoctrineInterface is a playable doctrine's interface.
type DoctrineInterface struct {
	// Resources is the HUD resource bar, in order: what each slot counts.
	Resources []InterfaceResource `json:"resources,omitempty"`
	// UnitLabel and UnitsLabel name one of the player's people and many
	// (the unit modal's title and a fallback name; the population tab and
	// hints).
	UnitLabel  string `json:"unit_label,omitempty"`
	UnitsLabel string `json:"units_label,omitempty"`
	// ResearchLabel titles the research modal ("Research", "Evolve").
	ResearchLabel string `json:"research_label,omitempty"`
	// Orders is the build menu's Orders category, in order (ids from the
	// order registry; label and description may override the defaults).
	Orders []InterfaceOrder `json:"orders,omitempty"`
	// UnitModal is the unit modal's sections, in order (section kinds).
	UnitModal []string `json:"unit_modal,omitempty"`
	// CrewGroup is the crew panel's grouping key: "" (none, the colony),
	// "type" or "type_tier".
	CrewGroup string `json:"crew_group,omitempty"`
	// Panels is a structure blueprint's panel: a click on one of the
	// player's opens it (step 3).
	Panels map[string]*InterfacePanel `json:"panels,omitempty"`
	// Groups names the director's rung groups the player may take over
	// (step 3), in panel order.
	Groups []InterfaceGroup `json:"groups,omitempty"`
}

// InterfaceResource is one resource bar slot: Source is a registry kind
// ("storage": the blueprint ID counted in the player's storage, "food":
// edible items, "larvae", "conversion"); ID names the slot and, for storage,
// the blueprint.
type InterfaceResource struct {
	ID     string `json:"id"`
	Source string `json:"source,omitempty"`
}

// InterfaceOrder is one Orders category entry.
type InterfaceOrder struct {
	ID          string `json:"id"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	// Roles are the unit roles the order sends (post: the guards).
	Roles []string `json:"roles,omitempty"`
}

// Order is the interface's order id, nil when it has none.
func (ui *DoctrineInterface) Order(id string) *InterfaceOrder {
	for i := range ui.Orders {
		if ui.Orders[i].ID == id {
			return &ui.Orders[i]
		}
	}
	return nil
}

// InterfacePanel is a structure's panel: a title and rows.
type InterfacePanel struct {
	Title string     `json:"title"`
	Rows  []PanelRow `json:"rows"`
}

// PanelRow is one row: Kind is a row kind ("readiness", "target_list",
// "action", "text"), Source a value source, Action an action, Label its
// words.
type PanelRow struct {
	Kind   string `json:"kind"`
	Source string `json:"source,omitempty"`
	Action string `json:"action,omitempty"`
	Label  string `json:"label,omitempty"`
}

// InterfaceGroup is a rung group the player may take over: its id (rungs
// carry it as "group"), its label.
type InterfaceGroup struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// UI is s's interface block, never nil (an absent block is an empty one:
// every reader falls back to the colony's defaults).
func (s *DoctrineSettlement) UI() *DoctrineInterface {
	if s == nil || s.Interface == nil {
		return &DoctrineInterface{}
	}
	return s.Interface
}
