// Command vmtest plays a JSON-lines script against Landing Party running as an
// Ebitengine VM guest and writes frames, state snapshots and a log.
//
//	vmtest run [-label L] [-out DIR] [-frame-every N] [-full] SCRIPT
//
// SCRIPT is a path or a name under vmtest/scripts/. See docs/developer/vmtest.md.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/vmtest"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  vmtest run [flags] SCRIPT      play a script, write frames to vmtest/out/<script>/<label>/
  vmtest targets [flags] SCRIPT  play a script, then list every target name resolvable at its end
  vmtest diff [flags] SCRIPT A B compare two labelled runs of a script (or: diff DIR_A DIR_B)`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var code int
	switch os.Args[1] {
	case "run":
		code = cmdRun(os.Args[2:])
	case "targets":
		code = cmdTargets(os.Args[2:])
	case "diff":
		code = cmdDiff(os.Args[2:])
	default:
		usage()
	}
	os.Exit(code)
}

// runFlags are shared by run and targets.
type runFlags struct {
	label      string
	out        string
	frameEvery int
	full       bool
	repo       string
}

func (f *runFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.label, "label", "", "run label (default: short git hash)")
	fs.StringVar(&f.out, "out", "", "output directory (default vmtest/out/<script>/<label>)")
	fs.IntVar(&f.frameEvery, "frame-every", -1, "render a discarded frame every N ticks of a wait (1 = real-play pacing)")
	fs.BoolVar(&f.full, "full", false, "write frames at physical size instead of the logical screen size")
	fs.StringVar(&f.repo, "repo", ".", "repository root")
}

// scriptPath resolves a script name or path.
func scriptPath(repo, arg string) string {
	if strings.HasSuffix(arg, ".jsonl") || strings.ContainsRune(arg, filepath.Separator) {
		return arg
	}
	return filepath.Join(repo, "vmtest", "scripts", arg+".jsonl")
}

func gitLabel(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "run"
	}
	return strings.TrimSpace(string(out))
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var f runFlags
	f.register(fs)
	fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	res, err := play(f, fs.Arg(0), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vmtest:", err)
	}
	if res != nil {
		fmt.Printf("%s: %s  (%s)\n", res.Script, map[bool]string{true: "PASS", false: "FAIL"}[res.OK], res.Dir)
	}
	if err != nil || res == nil || !res.OK {
		return 1
	}
	return 0
}

func cmdTargets(args []string) int {
	fs := flag.NewFlagSet("targets", flag.ExitOnError)
	var f runFlags
	f.register(fs)
	fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	if f.label == "" {
		f.label = "targets"
	}
	var names []string
	res, err := play(f, fs.Arg(0), func(s vmtest.Session) error {
		var err error
		names, err = s.Targets()
		return err
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "vmtest:", err)
		return 1
	}
	if !res.OK {
		fmt.Fprintf(os.Stderr, "vmtest: script failed: %s\n", res.Error)
		return 1
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return 0
}

func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	threshold := fs.Float64("threshold", 0.2, "changed-pixel percentage that counts as different")
	tol := fs.Int("tol", 8, "per-channel tolerance")
	ignore := fs.String("ignore", strings.Join(vmtest.DefaultIgnoreKeys, ","), "state keys to ignore, comma separated")
	repo := fs.String("repo", ".", "repository root")
	fs.Parse(args)
	var a, b string
	switch fs.NArg() {
	case 2: // two directories
		a, b = fs.Arg(0), fs.Arg(1)
	case 3: // script and two labels
		name := fs.Arg(0)
		if sc, err := vmtest.ParseFile(scriptPath(*repo, name)); err == nil && sc.Header.Name != "" {
			name = sc.Header.Name
		}
		a = filepath.Join(*repo, "vmtest", "out", name, fs.Arg(1))
		b = filepath.Join(*repo, "vmtest", "out", name, fs.Arg(2))
	default:
		usage()
	}
	rep, err := vmtest.CompareRuns(a, b, vmtest.DiffOptions{
		Threshold: *threshold, Tolerance: *tol, DiffDir: b, IgnoreKeys: strings.Split(*ignore, ","),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "vmtest:", err)
		return 2
	}
	fmt.Printf("A: %s\nB: %s\n\n%s", a, b, rep.Format(*threshold))
	if rep.Failed() {
		fmt.Printf("\nDIFFERENT (diff_<frame>.png written to %s)\n", b)
		return 1
	}
	fmt.Println("\nSAME within threshold")
	return 0
}

// Result is written to result.json.
type Result struct {
	Script     string  `json:"script"`
	Label      string  `json:"label"`
	Dir        string  `json:"dir"`
	OK         bool    `json:"ok"`
	Error      string  `json:"error,omitempty"`
	FailedLine int     `json:"failed_line,omitempty"`
	Ticks      int     `json:"ticks"`
	Frames     int     `json:"frames_rendered"`
	ElapsedMS  int64   `json:"elapsed_ms"`
	TickMS     int64   `json:"tick_ms"`
	FrameMS    int64   `json:"frame_ms"`
	MSPerFrame float64 `json:"ms_per_frame"`
	HostFPS    float64 `json:"host_updates_per_s"`
	Launches   int     `json:"launches"`
	LaunchMS   []int64 `json:"launch_ms"`
	Scale      float64 `json:"device_scale"`
	GuestExit  string  `json:"guest_exit,omitempty"`
	GuestPanic bool    `json:"guest_panic,omitempty"`
}

// play runs one script; after, if non-nil, runs on the script goroutine once the
// script has passed, with the live session.
func play(f runFlags, arg string, after func(vmtest.Session) error) (*Result, error) {
	path := scriptPath(f.repo, arg)
	script, err := vmtest.ParseFile(path)
	if err != nil {
		return nil, err
	}
	name := script.Header.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	}
	if f.label == "" {
		f.label = gitLabel(f.repo)
	}
	if f.out == "" {
		f.out = filepath.Join(f.repo, "vmtest", "out", name, f.label)
	}
	dir, err := vmtest.NewDir(f.out)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	dir.Echo = os.Stderr
	res := &Result{Script: name, Label: f.label, Dir: f.out}

	guestLog, err := os.Create(filepath.Join(f.out, "guest.log"))
	if err != nil {
		return nil, err
	}
	defer guestLog.Close()

	sandbox, _ := filepath.Abs(filepath.Join(f.out, "sandbox"))
	if err := vmtest.MakeSandbox(f.repo, sandbox, script.Header.Config); err != nil {
		return nil, fmt.Errorf("sandbox: %w", err)
	}
	bin := filepath.Join(sandbox, "guest")
	t := time.Now()
	if err := vmtest.BuildGuest(f.repo, bin, guestLog); err != nil {
		return nil, fmt.Errorf("build guest (see %s): %w", guestLog.Name(), err)
	}
	dir.Logf("built guest in %d ms", time.Since(t).Milliseconds())

	frameEvery := script.Header.FrameEvery
	if f.frameEvery >= 0 {
		frameEvery = f.frameEvery
	}
	w, h := screenSize(f.repo, script.Header.Config)
	h2 := newHost(hostConfig{
		binary: bin, guestLog: guestLog, dir: dir, logicalW: w, logicalH: h, full: f.full,
	})
	runner := &vmtest.Runner{S: h2, Out: dir, FrameEvery: frameEvery}

	start := time.Now()
	if err := h2.launch(); err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	var runErr error
	h2.script = func() {
		runErr = runner.Run(script)
		if runErr == nil && after != nil {
			runErr = after(h2)
		}
	}
	ebiten.SetWindowSize(320, 240)
	ebiten.SetWindowTitle("vmtest host")
	ebiten.SetWindowVisible(false)
	ebiten.SetVsyncEnabled(false)
	ebiten.SetTPS(1000)
	if err := ebiten.RunGame(h2); err != nil && !errors.Is(err, ebiten.Termination) {
		runErr = errors.Join(runErr, err)
	}
	exitErr := h2.finish()

	res.Ticks = runner.Tick()
	res.Frames = runner.FramesRun
	res.ElapsedMS = time.Since(start).Milliseconds()
	res.TickMS = runner.TickTime.Milliseconds()
	res.FrameMS = runner.FrameTime.Milliseconds()
	if runner.FramesRun > 0 {
		res.MSPerFrame = float64(runner.FrameTime.Microseconds()) / 1000 / float64(runner.FramesRun)
	}
	if s := time.Since(start).Seconds(); s > 0 {
		res.HostFPS = float64(h2.hostUpdates) / s
	}
	res.Launches = len(h2.launchMS)
	res.LaunchMS = h2.launchMS
	res.Scale = h2.scale
	if exitErr != nil {
		res.GuestExit = exitErr.Error()
	}
	res.GuestPanic = guestPanicked(guestLog.Name())
	if res.GuestPanic && runErr == nil {
		runErr = errors.New("guest panicked (see guest.log)")
	}
	res.OK = runErr == nil
	if runErr != nil {
		res.Error = runErr.Error()
		var se *vmtest.StepError
		if errors.As(runErr, &se) {
			res.FailedLine = se.Step.Line
		}
	}
	dir.Logf("result: ok=%v ticks=%d frames=%d elapsed=%dms tick=%dms frame=%dms (%.1f ms/frame) host=%.0f updates/s",
		res.OK, res.Ticks, res.Frames, res.ElapsedMS, res.TickMS, res.FrameMS, res.MSPerFrame, res.HostFPS)
	b, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(filepath.Join(f.out, "result.json"), b, 0o644)
	return res, nil
}

func guestPanicked(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, "panic: ") || strings.Contains(s, "fatal error: ")
}

// screenSize reads the logical screen size from data/config.json, then the
// script's config overrides.
func screenSize(repo string, override map[string]any) (int, int) {
	var c struct {
		W int `json:"screenWidth"`
		H int `json:"screenHeight"`
	}
	if b, err := os.ReadFile(filepath.Join(repo, "data", "config.json")); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if v, ok := override["screenWidth"].(float64); ok {
		c.W = int(v)
	}
	if v, ok := override["screenHeight"].(float64); ok {
		c.H = int(v)
	}
	if c.W == 0 || c.H == 0 {
		return 1152, 800
	}
	return c.W, c.H
}

var _ io.Writer = (*os.File)(nil)
