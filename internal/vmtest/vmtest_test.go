package vmtest

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestParseAcceptsAndRejects(t *testing.T) {
	good := `{"name":"x","seed":5,"config":{"a":true}}
# comment

{"op":"wait","ticks":3}
{"op":"click","target":"title:new"}
{"op":"click","at":[1,2],"button":"right"}
{"op":"key","key":"Escape","mods":["Shift"]}
{"op":"key","key":"1"}
{"op":"assert","path":"campaign.adrift","eq":false}
{"op":"wait_until","path":"state","eq":"site","max_ticks":60}
{"op":"screenshot","name":"a_b-1"}
{"op":"release"}
`
	s, err := Parse(strings.NewReader(good))
	if err != nil {
		t.Fatal(err)
	}
	if s.Header.Name != "x" || s.Header.Seed != 5 || len(s.Steps) != 9 {
		t.Fatalf("header %+v, %d steps", s.Header, len(s.Steps))
	}
	if s.Steps[0].Line != 4 {
		t.Errorf("line = %d, want 4", s.Steps[0].Line)
	}
	if !s.Steps[5].has["eq"] {
		t.Error("eq:false not recorded as present")
	}

	bad := []string{
		`{"op":"wait"}`,
		`{"op":"nope"}`,
		`{"op":"click"}`,
		`{"op":"click","target":"a","at":[1,2]}`,
		`{"op":"click","at":[1]}`,
		`{"op":"key","key":"NotAKey"}`,
		`{"op":"assert","path":"x"}`,
		`{"op":"assert","path":"x","eq":1,"ne":2}`,
		`{"op":"screenshot","name":"a b"}`,
		`{"op":"wait","ticks":1,"bogus":2}`,
		`{"op":"wait","ticks":1}` + "\n" + `{"name":"late"}`,
		`{"op":"wait_until","path":"x","eq":1}`,
		`not json`,
	}
	for _, b := range bad {
		if _, err := Parse(strings.NewReader(b)); err == nil {
			t.Errorf("accepted %q", b)
		}
	}
}

// fakeSession records calls; state is served from a map the test changes.
type fakeSession struct {
	calls  []string
	tick   int
	state  map[string]any
	onTick func(f *fakeSession)
	frames int
}

func (f *fakeSession) Ticks(n int) error {
	f.calls = append(f.calls, fmt.Sprintf("tick %d", n))
	for i := 0; i < n; i++ {
		f.tick++
		if f.onTick != nil {
			f.onTick(f)
		}
	}
	return nil
}
func (f *fakeSession) PressKey(k ebiten.Key)   { f.calls = append(f.calls, "press "+k.String()) }
func (f *fakeSession) ReleaseKey(k ebiten.Key) { f.calls = append(f.calls, "release "+k.String()) }
func (f *fakeSession) MoveCursor(x, y float64) {
	f.calls = append(f.calls, fmt.Sprintf("move %g,%g", x, y))
}
func (f *fakeSession) PressMouse(ebiten.MouseButton)   { f.calls = append(f.calls, "down") }
func (f *fakeSession) ReleaseMouse(ebiten.MouseButton) { f.calls = append(f.calls, "up") }
func (f *fakeSession) Scroll(dx, dy float64)           { f.calls = append(f.calls, "scroll") }
func (f *fakeSession) TypeRune(r rune)                 { f.calls = append(f.calls, "rune "+string(r)) }
func (f *fakeSession) Frame(keep bool) (image.Image, error) {
	f.frames++
	f.calls = append(f.calls, fmt.Sprintf("frame %v", keep))
	if !keep {
		return nil, nil
	}
	return image.NewRGBA(image.Rect(0, 0, 4, 4)), nil
}
func (f *fakeSession) State() (map[string]any, error) {
	if f.state == nil {
		return nil, ErrNoSidecar
	}
	return f.state, nil
}
func (f *fakeSession) Target(name string) (float64, float64, error) {
	if name == "ok" {
		return 10, 20, nil
	}
	return 0, 0, errors.New("unresolved")
}
func (f *fakeSession) Targets() ([]string, error) { return []string{"ok"}, nil }
func (f *fakeSession) Debug(string) error         { return nil }
func (f *fakeSession) Relaunch() error            { f.calls = append(f.calls, "relaunch"); return nil }
func (f *fakeSession) ExpectExit(int) error       { return nil }

type memOut struct {
	frames []string
	log    []string
}

func (m *memOut) WriteFrame(n int, name string, img image.Image, state map[string]any) error {
	m.frames = append(m.frames, fmt.Sprintf("%04d_%s", n, name))
	return nil
}
func (m *memOut) Logf(format string, args ...any) {
	m.log = append(m.log, fmt.Sprintf(format, args...))
}

func run(t *testing.T, f *fakeSession, script string) (*Runner, *memOut, error) {
	t.Helper()
	s, err := Parse(strings.NewReader(script))
	if err != nil {
		t.Fatal(err)
	}
	out := &memOut{}
	r := &Runner{S: f, Out: out}
	return r, out, r.Run(s)
}

func TestClickRecipe(t *testing.T) {
	f := &fakeSession{}
	r, _, err := run(t, f, `{"op":"click","target":"ok"}`)
	if err != nil {
		t.Fatal(err)
	}
	want := "move 10,20|tick 2|down|tick 1|up|tick 2|frame false"
	if got := strings.Join(f.calls, "|"); got != want {
		t.Errorf("calls %s\nwant  %s", got, want)
	}
	if r.Tick() != 5 {
		t.Errorf("tick = %d", r.Tick())
	}
}

func TestKeyWithMods(t *testing.T) {
	f := &fakeSession{}
	if _, _, err := run(t, f, `{"op":"key","key":"Escape","mods":["Shift"],"hold":3}`); err != nil {
		t.Fatal(err)
	}
	want := "press ShiftLeft|press Escape|tick 3|release Escape|release ShiftLeft|tick 2|frame false"
	if got := strings.Join(f.calls, "|"); got != want {
		t.Errorf("calls %s\nwant  %s", got, want)
	}
}

func TestUnresolvedTargetFailsWithFrame(t *testing.T) {
	f := &fakeSession{state: map[string]any{}}
	_, out, err := run(t, f, `{"op":"click","target":"gone"}`)
	var se *StepError
	if !errors.As(err, &se) || se.Step.Line != 1 {
		t.Fatalf("err = %v", err)
	}
	if len(out.frames) != 1 || out.frames[0] != "0001_FAIL" {
		t.Errorf("frames %v", out.frames)
	}
}

func TestWaitUntilAndTimeout(t *testing.T) {
	f := &fakeSession{state: map[string]any{"state": "dashboard"}}
	f.onTick = func(f *fakeSession) {
		if f.tick == 70 {
			f.state["state"] = "site"
		}
	}
	r, _, err := run(t, f, `{"op":"wait_until","path":"state","eq":"site","max_ticks":200}`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tick() != 90 {
		t.Errorf("tick = %d, want 90 (30-tick chunks)", r.Tick())
	}
	f2 := &fakeSession{state: map[string]any{"state": "dashboard"}}
	r2, _, err := run(t, f2, `{"op":"wait_until","path":"state","eq":"site","max_ticks":45}`)
	if err == nil || !strings.Contains(err.Error(), "after 45 ticks") {
		t.Fatalf("err = %v", err)
	}
	if r2.Tick() != 45 {
		t.Errorf("tick = %d", r2.Tick())
	}
}

func TestAssertOps(t *testing.T) {
	state := map[string]any{
		"n": 5.0, "s": "PAUSED - incident", "b": false,
		"list": []any{map[string]any{"name": "Ann"}}, "m": map[string]any{"fuel": 3.0},
	}
	cases := []struct {
		line string
		ok   bool
	}{
		{`{"op":"assert","path":"n","eq":5}`, true},
		{`{"op":"assert","path":"n","ne":5}`, false},
		{`{"op":"assert","path":"n","gte":5}`, true},
		{`{"op":"assert","path":"n","lte":4}`, false},
		{`{"op":"assert","path":"s","contains":"incident"}`, true},
		{`{"op":"assert","path":"b","eq":false}`, true},
		{`{"op":"assert","path":"missing","eq":false}`, false},
		{`{"op":"assert","path":"missing","eq":null}`, true},
		{`{"op":"assert","path":"list.0.name","eq":"Ann"}`, true},
		{`{"op":"assert","path":"list.#","eq":1}`, true},
		{`{"op":"assert","path":"m","contains":"fuel"}`, true},
		{`{"op":"assert","path":"m.fuel","eq":3}`, true},
	}
	for _, c := range cases {
		f := &fakeSession{state: state}
		_, _, err := run(t, f, c.line)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v", c.line, err)
		}
	}
}

func TestSaveAsEqSaved(t *testing.T) {
	f := &fakeSession{state: map[string]any{"day": 3.0}}
	s, _ := Parse(strings.NewReader(`{"op":"save_as","name":"before"}`))
	out := &memOut{}
	r := &Runner{S: f, Out: out}
	if err := r.Run(s); err != nil {
		t.Fatal(err)
	}
	f.state = map[string]any{"day": 3.0}
	s2, _ := Parse(strings.NewReader(`{"op":"assert","path":"day","eq_saved":"before"}`))
	if err := r.Run(s2); err != nil {
		t.Fatal(err)
	}
	f.state = map[string]any{"day": 4.0}
	if err := r.Run(s2); err == nil {
		t.Fatal("changed day passed")
	}
	s3, _ := Parse(strings.NewReader(`{"op":"assert","path":"day","ne_saved":"before"}`))
	if err := r.Run(s3); err != nil {
		t.Fatal(err)
	}
}

func TestFrameEvery(t *testing.T) {
	f := &fakeSession{}
	s, _ := Parse(strings.NewReader(`{"op":"wait","ticks":10}`))
	r := &Runner{S: f, Out: &memOut{}, FrameEvery: 4}
	if err := r.Run(s); err != nil {
		t.Fatal(err)
	}
	want := "tick 4|frame false|tick 4|frame false|tick 2|frame false"
	if got := strings.Join(f.calls, "|"); got != want {
		t.Errorf("calls %s", got)
	}
}

func TestScreenshotNumbering(t *testing.T) {
	f := &fakeSession{}
	_, out, err := run(t, f, `{"op":"screenshot","name":"a"}
{"op":"screenshot","name":"b"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(out.frames, ",") != "0001_a,0002_b" {
		t.Errorf("frames %v", out.frames)
	}
}

func TestDownsample(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			v := uint8(0)
			if x < 2 {
				v = 200
			}
			src.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	src.Set(0, 0, color.RGBA{100, 100, 100, 255})
	got := Downsample(src, 2, 1)
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 1 {
		t.Fatalf("size %v", got.Bounds())
	}
	if c := got.RGBAAt(0, 0); c.R != 175 || c.A != 255 {
		t.Errorf("left = %v, want 175", c)
	}
	if c := got.RGBAAt(1, 0); c.R != 0 {
		t.Errorf("right = %v", c)
	}
	if Downsample(src, 4, 2) != src {
		t.Error("same size should return src")
	}
}

func TestSandboxExcludesLocalConfig(t *testing.T) {
	repo := t.TempDir()
	for _, p := range []string{"assets/a.png", "data/config.json", "data/config.local.json", "data/maps/m.json"} {
		full := filepath.Join(repo, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(`{"owner":true}`), 0o644)
	}
	sb := filepath.Join(t.TempDir(), "sandbox")
	if err := MakeSandbox(repo, sb, map[string]any{"debugShowScenarioID": false}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(sb, "data", "config.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "owner") || !strings.Contains(string(b), "debugShowScenarioID") {
		t.Errorf("config.local.json = %s", b)
	}
	if fi, err := os.Lstat(filepath.Join(sb, "data", "config.local.json")); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Error("config.local.json must be a real file")
	}
	if _, err := os.Stat(filepath.Join(sb, "data", "maps", "m.json")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(sb, "assets", "a.png")); err != nil {
		t.Error(err)
	}
	if fi, err := os.Stat(filepath.Join(sb, "saves")); err != nil || !fi.IsDir() {
		t.Error("saves/ missing")
	}
}
