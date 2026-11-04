// Package vmtest plays JSON-lines scripts against the game running as an
// Ebitengine VM guest. The host program is cmd/vmtest; this package holds the
// parts that need no GPU: the script parser, the step runner (against a small
// Session interface), the sandbox and guest process, frame writing and diffs.
package vmtest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// Header is the optional first line of a script.
type Header struct {
	Name string `json:"name"`
	// Seed is informational: scripts type it into the title's seed field.
	Seed int64 `json:"seed,omitempty"`
	// Config is written to the sandbox's data/config.local.json.
	Config map[string]any `json:"config,omitempty"`
	// FrameEvery renders a discarded frame every N ticks during waits; 0 means
	// only after input. The command-line flag overrides it.
	FrameEvery int `json:"frame_every,omitempty"`
}

// Step is one script line. Fields are shared across ops; Validate checks that
// each op has what it needs.
type Step struct {
	Op string `json:"op"`

	Ticks    int `json:"ticks,omitempty"`
	MaxTicks int `json:"max_ticks,omitempty"`

	Key  string   `json:"key,omitempty"`
	Mods []string `json:"mods,omitempty"`
	Hold int      `json:"hold,omitempty"`

	Target string    `json:"target,omitempty"`
	At     []float64 `json:"at,omitempty"`
	Button string    `json:"button,omitempty"`
	DX     float64   `json:"dx,omitempty"`
	DY     float64   `json:"dy,omitempty"`

	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"`
	Cmd  string `json:"cmd,omitempty"`

	Path     string `json:"path,omitempty"`
	Eq       any    `json:"eq,omitempty"`
	Ne       any    `json:"ne,omitempty"`
	Gte      any    `json:"gte,omitempty"`
	Lte      any    `json:"lte,omitempty"`
	Contains any    `json:"contains,omitempty"`
	// EqSaved and NeSaved compare path with the same path in a snapshot
	// stored by save_as.
	EqSaved string `json:"eq_saved,omitempty"`
	NeSaved string `json:"ne_saved,omitempty"`
	// SaveAs stores the whole state under a name (op save_as).
	SaveAs string `json:"save_as,omitempty"`

	// Line is the 1-based script line, for messages.
	Line int `json:"-"`
	// has records which comparison keys were present, since eq:false and
	// eq:0 are real values that omitempty cannot tell from absence.
	has map[string]bool
}

// Script is a parsed script file.
type Script struct {
	Header Header
	Steps  []Step
}

// ParseFile reads and validates a script.
func ParseFile(path string) (*Script, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads JSON lines. Blank lines and lines starting with # are skipped. A
// first object without "op" is the header.
func Parse(r io.Reader) (*Script, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	s := &Script{}
	line := 0
	sawStep := false
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		var keys map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &keys); err != nil {
			return nil, fmt.Errorf("line %d: %v", line, err)
		}
		if _, isStep := keys["op"]; !isStep {
			if sawStep || s.Header.Name != "" {
				return nil, fmt.Errorf("line %d: missing \"op\" (only the first line may be a header)", line)
			}
			dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&s.Header); err != nil {
				return nil, fmt.Errorf("line %d: header: %v", line, err)
			}
			if s.Header.Name == "" {
				return nil, fmt.Errorf("line %d: header needs a name", line)
			}
			continue
		}
		sawStep = true
		var st Step
		dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
		dec.DisallowUnknownFields()
		dec.UseNumber()
		if err := dec.Decode(&st); err != nil {
			return nil, fmt.Errorf("line %d: %v", line, err)
		}
		st.Line = line
		st.has = map[string]bool{}
		for k := range keys {
			st.has[k] = true
		}
		if err := st.Validate(); err != nil {
			return nil, fmt.Errorf("line %d: %v", line, err)
		}
		s.Steps = append(s.Steps, st)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return s, nil
}

var comparisons = []string{"eq", "ne", "gte", "lte", "contains", "eq_saved", "ne_saved"}

// Validate checks one step's fields.
func (st *Step) Validate() error {
	need := func(ok bool, msg string) error {
		if !ok {
			return fmt.Errorf("%s: %s", st.Op, msg)
		}
		return nil
	}
	switch st.Op {
	case "wait":
		return need(st.Ticks > 0, "ticks must be > 0")
	case "wait_until":
		if err := need(st.Path != "", "path required"); err != nil {
			return err
		}
		if err := need(st.MaxTicks > 0, "max_ticks must be > 0"); err != nil {
			return err
		}
		return st.needComparison()
	case "key":
		if _, err := ParseKey(st.Key); err != nil {
			return err
		}
		for _, m := range st.Mods {
			if _, err := ParseKey(m); err != nil {
				return err
			}
		}
		return need(st.Hold >= 0, "hold must be >= 0")
	case "click", "press", "release":
		if _, err := ParseButton(st.Button); err != nil {
			return err
		}
		if st.Op == "release" && st.Target == "" && st.At == nil {
			return nil // release where the cursor is
		}
		return st.needWhere()
	case "move":
		return st.needWhere()
	case "scroll":
		return need(st.DX != 0 || st.DY != 0, "dx or dy required")
	case "type":
		return need(st.Text != "", "text required")
	case "screenshot":
		if err := need(st.Name != "", "name required"); err != nil {
			return err
		}
		return need(safeName(st.Name), "name may only hold letters, digits, - and _")
	case "assert":
		if err := need(st.Path != "", "path required"); err != nil {
			return err
		}
		return st.needComparison()
	case "save_as":
		return need(st.Name != "", "name required")
	case "debug":
		return need(st.Cmd != "", "cmd required")
	case "relaunch":
		return nil
	case "expect_exit":
		if st.Key != "" {
			if _, err := ParseKey(st.Key); err != nil {
				return err
			}
		}
		return need(st.MaxTicks > 0, "max_ticks must be > 0")
	case "":
		return fmt.Errorf("empty op")
	}
	return fmt.Errorf("unknown op %q", st.Op)
}

func (st *Step) needWhere() error {
	if (st.Target == "") == (st.At == nil) {
		return fmt.Errorf("%s: exactly one of target or at", st.Op)
	}
	if st.At != nil && len(st.At) != 2 {
		return fmt.Errorf("%s: at must be [x, y]", st.Op)
	}
	return nil
}

func (st *Step) needComparison() error {
	n := 0
	for _, c := range comparisons {
		if st.has[c] {
			n++
		}
	}
	if n != 1 {
		return fmt.Errorf("%s: exactly one of eq, ne, gte, lte, contains, eq_saved, ne_saved", st.Op)
	}
	return nil
}

func safeName(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return s != ""
}

// ParseKey maps a key name to an ebiten.Key. It accepts ebiten's own names
// ("A", "Escape", "ShiftLeft", "Digit1") and a few aliases.
func ParseKey(name string) (ebiten.Key, error) {
	alias := map[string]string{"Shift": "ShiftLeft", "Ctrl": "ControlLeft", "Control": "ControlLeft",
		"Alt": "AltLeft", "Meta": "MetaLeft", "Esc": "Escape", "Return": "Enter"}
	if a, ok := alias[name]; ok {
		name = a
	}
	if len(name) == 1 && name[0] >= '0' && name[0] <= '9' {
		name = "Digit" + name
	}
	var k ebiten.Key
	if name == "" {
		return k, fmt.Errorf("key: name required")
	}
	if err := k.UnmarshalText([]byte(name)); err != nil {
		return k, fmt.Errorf("key: unknown key %q", name)
	}
	return k, nil
}

// ParseButton maps "", "left", "right", "middle" to a mouse button.
func ParseButton(name string) (ebiten.MouseButton, error) {
	switch name {
	case "", "left":
		return ebiten.MouseButtonLeft, nil
	case "right":
		return ebiten.MouseButtonRight, nil
	case "middle":
		return ebiten.MouseButtonMiddle, nil
	}
	return 0, fmt.Errorf("unknown button %q", name)
}
