// Package vmproto is the sidecar protocol between cmd/vmtest and the game
// running as a VM guest: one JSON object per line each way. It has no
// dependencies so the guest hook and the host share it.
package vmproto

// Request ops.
const (
	OpState   = "state"
	OpTarget  = "target"
	OpTargets = "targets"
	OpDebug   = "debug"
)

// Request is one host-to-guest line.
type Request struct {
	Op   string `json:"op"`
	Name string `json:"name,omitempty"`
	Cmd  string `json:"cmd,omitempty"`
}

// Reply is one guest-to-host line.
type Reply struct {
	OK      bool           `json:"ok"`
	Error   string         `json:"error,omitempty"`
	State   map[string]any `json:"state,omitempty"`
	X       float64        `json:"x,omitempty"`
	Y       float64        `json:"y,omitempty"`
	W       float64        `json:"w,omitempty"`
	H       float64        `json:"h,omitempty"`
	Targets []string       `json:"targets,omitempty"`
	Output  string         `json:"output,omitempty"`
}
