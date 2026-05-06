package gui

type CursorModeType string

const (
	CursorModeDefault               CursorModeType = "default"
	CursorModeBuild                 CursorModeType = "build"
	CursorModeDig                   CursorModeType = "dig"
	CursorModeMine                  CursorModeType = "mine"
	CursorModeAttack                CursorModeType = "attack"
	CursorModeCancel                CursorModeType = "cancel"
	CursorModeSetSettlementLocation CursorModeType = "setSettlementLocation"
)
