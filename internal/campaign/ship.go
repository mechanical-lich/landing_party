package campaign

// ShipSettlementName is the synthetic settlement that owns everything in the
// ship's storage. The ship's hold is ordinary level storage owned by this name.
const ShipSettlementName = "ship"

// ShipState holds the campaign-level ship data that isn't on the ship level
// itself. The hold (storage) and crew now live as entities on the always-loaded
// ship level (see internal/ship and docs/developer/the_ship.md); only the
// roster cap remains here.
type ShipState struct {
	// RosterCap is the maximum number of colonists that may be aboard.
	RosterCap int `json:"roster_cap"`
}

// NewShipState returns ship state with the given roster cap.
func NewShipState(rosterCap int) *ShipState {
	return &ShipState{RosterCap: rosterCap}
}
