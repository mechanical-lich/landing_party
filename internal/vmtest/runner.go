package vmtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Session is what the runner drives: the VM guest plus the sidecar. The host
// program implements it over vmhost; tests use a fake.
type Session interface {
	// Ticks advances n guest ticks and waits for them.
	Ticks(n int) error
	PressKey(k ebiten.Key)
	ReleaseKey(k ebiten.Key)
	MoveCursor(x, y float64)
	PressMouse(b ebiten.MouseButton)
	ReleaseMouse(b ebiten.MouseButton)
	Scroll(dx, dy float64)
	TypeRune(r rune)
	// Frame renders one guest frame on the host goroutine and returns it at
	// logical size, or nil without error when keep is false.
	Frame(keep bool) (image.Image, error)
	// State returns the guest's state summary (sidecar).
	State() (map[string]any, error)
	// Target resolves a named UI target to a logical point (sidecar).
	Target(name string) (x, y float64, err error)
	// Targets lists every name resolvable now (sidecar).
	Targets() ([]string, error)
	// Debug runs a guest debug command (sidecar).
	Debug(cmd string) error
	// Relaunch closes the guest and starts a new one in the same sandbox.
	Relaunch() error
	// ExpectExit advances up to maxTicks and reports nil once the guest has
	// exited with status 0.
	ExpectExit(maxTicks int) error
}

// ErrNoSidecar is returned by sessions built without the sidecar.
var ErrNoSidecar = errors.New("sidecar not available")

// Output receives what a run produces.
type Output interface {
	// WriteFrame stores a numbered frame and its state; state may be nil.
	WriteFrame(n int, name string, img image.Image, state map[string]any) error
	Logf(format string, args ...any)
}

// Runner plays a script against a session.
type Runner struct {
	S   Session
	Out Output
	// FrameEvery renders a discarded frame every N ticks of a wait (0: none).
	FrameEvery int

	tick   int
	frames int
	saved  map[string]map[string]any
	// Timings for the report.
	TickTime  time.Duration
	FrameTime time.Duration
	FramesRun int
}

// Tick is the number of guest ticks this run has advanced (across relaunches).
func (r *Runner) Tick() int { return r.tick }

// StepError is a failed step.
type StepError struct {
	Step Step
	Err  error
}

func (e *StepError) Error() string {
	return fmt.Sprintf("line %d (%s): %v", e.Step.Line, e.Step.Op, e.Err)
}

func (e *StepError) Unwrap() error { return e.Err }

// Run plays every step, stopping at the first failure. On failure it takes a
// frame and state named FAIL (best effort) and returns a *StepError.
func (r *Runner) Run(s *Script) error {
	if r.saved == nil {
		r.saved = map[string]map[string]any{}
	}
	for _, st := range s.Steps {
		start := time.Now()
		if err := r.step(st); err != nil {
			r.Out.Logf("tick %6d  line %3d  %-11s FAIL %v", r.tick, st.Line, st.Op, err)
			r.fail()
			return &StepError{Step: st, Err: err}
		}
		r.Out.Logf("tick %6d  line %3d  %-11s ok   %s (%d ms)", r.tick, st.Line, st.Op, describe(st), time.Since(start).Milliseconds())
	}
	return nil
}

func (r *Runner) fail() {
	img, err := r.S.Frame(true)
	if err != nil {
		r.Out.Logf("FAIL frame: %v", err)
		return
	}
	state, _ := r.S.State()
	r.frames++
	if err := r.Out.WriteFrame(r.frames, "FAIL", img, state); err != nil {
		r.Out.Logf("FAIL frame: %v", err)
	}
	if names, err := r.S.Targets(); err == nil {
		r.Out.Logf("targets resolvable now: %s", strings.Join(names, " "))
	}
}

func describe(st Step) string {
	switch st.Op {
	case "wait":
		return fmt.Sprintf("%d ticks", st.Ticks)
	case "key":
		return st.Key
	case "click", "press", "release", "move":
		if st.Target != "" {
			return st.Target
		}
		return fmt.Sprint(st.At)
	case "type":
		return fmt.Sprintf("%q", st.Text)
	case "screenshot", "save_as":
		return st.Name
	case "assert", "wait_until":
		return st.Path
	case "debug":
		return st.Cmd
	}
	return ""
}

// ticks advances n ticks, rendering a discarded frame every FrameEvery.
func (r *Runner) ticks(n int) error {
	every := r.FrameEvery
	for n > 0 {
		chunk := n
		if every > 0 && chunk > every {
			chunk = every
		}
		t := time.Now()
		if err := r.S.Ticks(chunk); err != nil {
			return err
		}
		r.TickTime += time.Since(t)
		r.tick += chunk
		n -= chunk
		if every > 0 {
			if err := r.render(); err != nil {
				return err
			}
		}
	}
	return nil
}

// render draws one discarded frame, so the game's Draw-side state (the
// viewport FOV and audio read) follows the latest input.
func (r *Runner) render() error {
	t := time.Now()
	_, err := r.S.Frame(false)
	r.FrameTime += time.Since(t)
	r.FramesRun++
	return err
}

func (r *Runner) where(st Step) (float64, float64, error) {
	if st.At != nil {
		return st.At[0], st.At[1], nil
	}
	return r.S.Target(st.Target)
}

func (r *Runner) step(st Step) error {
	switch st.Op {
	case "wait":
		return r.ticks(st.Ticks)
	case "wait_until":
		for waited := 0; ; {
			ok, got, err := r.check(st)
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
			if waited >= st.MaxTicks {
				return fmt.Errorf("%s still %s after %d ticks (want %s)", st.Path, show(got), waited, wantText(st))
			}
			n := min(30, st.MaxTicks-waited)
			if err := r.ticks(n); err != nil {
				return err
			}
			waited += n
		}
	case "key":
		var mods []ebiten.Key
		for _, m := range st.Mods {
			k, _ := ParseKey(m)
			mods = append(mods, k)
			r.S.PressKey(k)
		}
		k, _ := ParseKey(st.Key)
		r.S.PressKey(k)
		hold := st.Hold
		if hold == 0 {
			hold = 1
		}
		if err := r.ticks(hold); err != nil {
			return err
		}
		r.S.ReleaseKey(k)
		for i := len(mods) - 1; i >= 0; i-- {
			r.S.ReleaseKey(mods[i])
		}
		if err := r.ticks(2); err != nil {
			return err
		}
		return r.render()
	case "click", "press", "release", "move":
		b, _ := ParseButton(st.Button)
		if st.Op != "release" || st.Target != "" || st.At != nil {
			x, y, err := r.where(st)
			if err != nil {
				return err
			}
			r.S.MoveCursor(x, y)
			if err := r.ticks(2); err != nil {
				return err
			}
		}
		if st.Op == "click" || st.Op == "press" {
			r.S.PressMouse(b)
			if err := r.ticks(1); err != nil {
				return err
			}
		}
		if st.Op == "click" || st.Op == "release" {
			r.S.ReleaseMouse(b)
			if err := r.ticks(2); err != nil {
				return err
			}
		}
		return r.render()
	case "scroll":
		r.S.Scroll(st.DX, st.DY)
		if err := r.ticks(2); err != nil {
			return err
		}
		return r.render()
	case "type":
		for _, ch := range st.Text {
			r.S.TypeRune(ch)
			if err := r.ticks(1); err != nil {
				return err
			}
		}
		if err := r.ticks(1); err != nil {
			return err
		}
		return r.render()
	case "screenshot":
		t := time.Now()
		img, err := r.S.Frame(true)
		r.FrameTime += time.Since(t)
		r.FramesRun++
		if err != nil {
			return err
		}
		state, err := r.S.State()
		if err != nil && !errors.Is(err, ErrNoSidecar) {
			return err
		}
		r.frames++
		return r.Out.WriteFrame(r.frames, st.Name, img, state)
	case "assert":
		ok, got, err := r.check(st)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s is %s, want %s", st.Path, show(got), wantText(st))
		}
		return nil
	case "save_as":
		state, err := r.S.State()
		if err != nil {
			return err
		}
		r.saved[st.Name] = state
		return nil
	case "debug":
		return r.S.Debug(st.Cmd)
	case "relaunch":
		return r.S.Relaunch()
	case "expect_exit":
		// The key (if any) is pressed and never released: the guest is
		// expected to be gone before a release could matter.
		if st.Key != "" {
			k, _ := ParseKey(st.Key)
			r.S.PressKey(k)
		}
		return r.S.ExpectExit(st.MaxTicks)
	}
	return fmt.Errorf("unknown op %q", st.Op)
}

// check evaluates a step's comparison against the current state.
func (r *Runner) check(st Step) (bool, any, error) {
	state, err := r.S.State()
	if err != nil {
		return false, nil, err
	}
	got, found := Lookup(state, st.Path)
	if st.has["eq_saved"] || st.has["ne_saved"] {
		name := st.EqSaved + st.NeSaved
		snap, ok := r.saved[name]
		if !ok {
			return false, nil, fmt.Errorf("no snapshot %q (use save_as first)", name)
		}
		want, wfound := Lookup(snap, st.Path)
		same := found == wfound && Equal(got, want)
		return same == st.has["eq_saved"], got, nil
	}
	if !found {
		got = nil
	}
	return Compare(st, got), got, nil
}

// Lookup walks a dotted path through maps and slices ("crew.0.name",
// "hold.fuel", "crew.#" for a length).
func Lookup(v any, path string) (any, bool) {
	if path == "" {
		return v, true
	}
	for _, part := range strings.Split(path, ".") {
		switch cur := v.(type) {
		case map[string]any:
			if part == "#" {
				return float64(len(cur)), true
			}
			next, ok := cur[part]
			if !ok {
				return nil, false
			}
			v = next
		case []any:
			if part == "#" {
				return float64(len(cur)), true
			}
			var i int
			if _, err := fmt.Sscanf(part, "%d", &i); err != nil || i < 0 || i >= len(cur) {
				return nil, false
			}
			v = cur[i]
		default:
			return nil, false
		}
	}
	return v, true
}

// Compare applies the step's comparison to got.
func Compare(st Step, got any) bool {
	switch {
	case st.has["eq"]:
		return Equal(got, st.Eq)
	case st.has["ne"]:
		return !Equal(got, st.Ne)
	case st.has["gte"]:
		g, ok1 := number(got)
		w, ok2 := number(st.Gte)
		return ok1 && ok2 && g >= w
	case st.has["lte"]:
		g, ok1 := number(got)
		w, ok2 := number(st.Lte)
		return ok1 && ok2 && g <= w
	case st.has["contains"]:
		switch c := got.(type) {
		case string:
			s, ok := st.Contains.(string)
			return ok && strings.Contains(c, s)
		case []any:
			for _, e := range c {
				if Equal(e, st.Contains) {
					return true
				}
			}
		case map[string]any:
			s, ok := st.Contains.(string)
			_, has := c[s]
			return ok && has
		}
		return false
	}
	return false
}

// Equal compares JSON-decoded values, treating all numbers alike.
func Equal(a, b any) bool {
	if x, ok := number(a); ok {
		y, ok := number(b)
		return ok && x == y
	}
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ja) == string(jb)
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

func wantText(st Step) string {
	for _, c := range comparisons {
		if st.has[c] {
			var v any
			switch c {
			case "eq":
				v = st.Eq
			case "ne":
				v = st.Ne
			case "gte":
				v = st.Gte
			case "lte":
				v = st.Lte
			case "contains":
				v = st.Contains
			case "eq_saved":
				return "same as snapshot " + st.EqSaved
			case "ne_saved":
				return "different from snapshot " + st.NeSaved
			}
			return c + " " + show(v)
		}
	}
	return "?"
}

// SortedKeys is a helper for logs.
func SortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// LogWriter adapts an io.Writer to the Logf half of Output.
type LogWriter struct{ W io.Writer }

func (l LogWriter) Logf(format string, args ...any) {
	fmt.Fprintf(l.W, format+"\n", args...)
}
