package gui

type CursorModeType string

const (
	CursorModeDefault               CursorModeType = "default"
	CursorModeBuild                 CursorModeType = "build"
	CursorModeDig                   CursorModeType = "dig"
	CursorModeMine                  CursorModeType = "mine"
	CursorModeAttack                CursorModeType = "attack"
	CursorModeSleep                 CursorModeType = "sleep"
	CursorModeCancel                CursorModeType = "cancel"
	CursorModeSetSettlementLocation CursorModeType = "setSettlementLocation"
	CursorModeFollow                CursorModeType = "follow"
	CursorModeRogue                 CursorModeType = "rogue"
	CursorModeRelocate              CursorModeType = "relocate"
	CursorModeStore                 CursorModeType = "store"
	CursorModeDrop                  CursorModeType = "drop"
	CursorModePickup                CursorModeType = "pickup"
)
