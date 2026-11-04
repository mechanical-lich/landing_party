package vmtest

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DiffOptions tune CompareRuns.
type DiffOptions struct {
	// Threshold is the changed-pixel percentage above which a frame counts as
	// different (default 0.2).
	Threshold float64
	// Tolerance is the per-channel difference a pixel may have and still count
	// as unchanged (default 8).
	Tolerance int
	// DiffDir receives diff_<name>.png for frames over the threshold; "" skips.
	DiffDir string
	// IgnoreKeys are state keys left out of the state comparison, at any depth.
	IgnoreKeys []string
}

// FrameDiff is one paired frame.
type FrameDiff struct {
	Name    string // frame name without its number, plus #n for repeats
	A, B    string // file names, "" when missing on that side
	Changed float64
	Over    bool
	Note    string
}

// StateDiff is one differing state path.
type StateDiff struct {
	Frame string
	Path  string
	A, B  string
}

// DiffReport is the result of comparing two run directories.
type DiffReport struct {
	Frames []FrameDiff
	States []StateDiff
}

// Failed reports whether anything is over threshold or differs in state.
func (r *DiffReport) Failed() bool {
	for _, f := range r.Frames {
		if f.Over {
			return true
		}
	}
	return len(r.States) > 0
}

// DefaultIgnoreKeys are fields that differ between identical runs.
var DefaultIgnoreKeys = []string{"id", "stable_id", "created", "timestamp"}

var frameRe = regexp.MustCompile(`^(\d{4})_(.+)\.png$`)

type runFrame struct {
	file, num string
}

// listFrames maps "name" (and "name#2" for repeats) to its frame file.
func listFrames(dir string) (map[string]runFrame, []string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var files []string
	for _, e := range ents {
		if frameRe.MatchString(e.Name()) {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	out := map[string]runFrame{}
	var order []string
	seen := map[string]int{}
	for _, f := range files {
		m := frameRe.FindStringSubmatch(f)
		key := m[2]
		seen[key]++
		if seen[key] > 1 {
			key = fmt.Sprintf("%s#%d", key, seen[key])
		}
		out[key] = runFrame{file: f, num: m[1]}
		order = append(order, key)
	}
	return out, order, nil
}

// CompareRuns pairs frames by name between two run directories and compares
// pixels and state files.
func CompareRuns(dirA, dirB string, o DiffOptions) (*DiffReport, error) {
	if o.Threshold == 0 {
		o.Threshold = 0.2
	}
	if o.Tolerance == 0 {
		o.Tolerance = 8
	}
	if o.IgnoreKeys == nil {
		o.IgnoreKeys = DefaultIgnoreKeys
	}
	fa, orderA, err := listFrames(dirA)
	if err != nil {
		return nil, err
	}
	fb, orderB, err := listFrames(dirB)
	if err != nil {
		return nil, err
	}
	order := orderA
	for _, k := range orderB {
		if _, ok := fa[k]; !ok {
			order = append(order, k)
		}
	}
	rep := &DiffReport{}
	for _, name := range order {
		a, okA := fa[name]
		b, okB := fb[name]
		fd := FrameDiff{Name: name, A: a.file, B: b.file}
		if !okA || !okB {
			fd.Over, fd.Changed, fd.Note = true, 100, "missing in "+map[bool]string{true: "B", false: "A"}[okA]
			rep.Frames = append(rep.Frames, fd)
			continue
		}
		ia, err := readPNG(filepath.Join(dirA, a.file))
		if err != nil {
			return nil, err
		}
		ib, err := readPNG(filepath.Join(dirB, b.file))
		if err != nil {
			return nil, err
		}
		pct, mask := PixelDiff(ia, ib, o.Tolerance)
		fd.Changed = pct
		fd.Over = pct > o.Threshold
		if ia.Bounds() != ib.Bounds() {
			fd.Note = "size differs"
		}
		if fd.Over && o.DiffDir != "" && mask != nil {
			safe := strings.ReplaceAll(name, "#", "_")
			if err := WritePNG(filepath.Join(o.DiffDir, "diff_"+safe+".png"), DiffImage(ib, mask)); err != nil {
				return nil, err
			}
		}
		rep.Frames = append(rep.Frames, fd)

		sa, errA := readState(filepath.Join(dirA, "state_"+a.num+".json"))
		sb, errB := readState(filepath.Join(dirB, "state_"+b.num+".json"))
		if errA == nil && errB == nil {
			ignore := map[string]bool{}
			for _, k := range o.IgnoreKeys {
				ignore[k] = true
			}
			for _, d := range diffValues("", sa, sb, ignore) {
				d.Frame = name
				rep.States = append(rep.States, d)
			}
		}
	}
	return rep, nil
}

func readPNG(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba, nil
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	return rgba, nil
}

func readState(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v any
	return v, json.Unmarshal(b, &v)
}

// PixelDiff returns the percentage of pixels whose channels differ by more
// than tol, and a mask of them (nil when sizes differ, which counts as 100%).
func PixelDiff(a, b *image.RGBA, tol int) (float64, []bool) {
	if a.Bounds().Size() != b.Bounds().Size() {
		return 100, nil
	}
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	mask := make([]bool, w*h)
	changed := 0
	for y := 0; y < h; y++ {
		ra := a.Pix[y*a.Stride:]
		rb := b.Pix[y*b.Stride:]
		for x := 0; x < w; x++ {
			for c := 0; c < 4; c++ {
				d := int(ra[x*4+c]) - int(rb[x*4+c])
				if d > tol || d < -tol {
					mask[y*w+x] = true
					changed++
					break
				}
			}
		}
	}
	if w*h == 0 {
		return 0, mask
	}
	return float64(changed) * 100 / float64(w*h), mask
}

// DiffImage is b dimmed to grey with changed pixels in red.
func DiffImage(b *image.RGBA, mask []bool) *image.RGBA {
	w, h := b.Bounds().Dx(), b.Bounds().Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask[y*w+x] {
				out.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
				continue
			}
			p := b.Pix[y*b.Stride+x*4:]
			g := uint8((int(p[0]) + int(p[1]) + int(p[2])) / 3 / 3)
			out.SetRGBA(x, y, color.RGBA{g, g, g, 255})
		}
	}
	return out
}

func diffValues(path string, a, b any, ignore map[string]bool) []StateDiff {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			break
		}
		keys := map[string]bool{}
		for k := range av {
			keys[k] = true
		}
		for k := range bv {
			keys[k] = true
		}
		var names []string
		for k := range keys {
			if !ignore[k] {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		var out []StateDiff
		for _, k := range names {
			out = append(out, diffValues(joinPath(path, k), av[k], bv[k], ignore)...)
		}
		return out
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			break
		}
		var out []StateDiff
		for i := range av {
			out = append(out, diffValues(joinPath(path, fmt.Sprint(i)), av[i], bv[i], ignore)...)
		}
		return out
	}
	if Equal(a, b) {
		return nil
	}
	return []StateDiff{{Path: path, A: show(a), B: show(b)}}
}

func joinPath(p, k string) string {
	if p == "" {
		return k
	}
	return p + "." + k
}

// Format renders the report as a text table.
func (r *DiffReport) Format(threshold float64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-28s %9s  %s\n", "frame", "changed", "")
	for _, f := range r.Frames {
		mark := ""
		if f.Over {
			mark = fmt.Sprintf("OVER %.2f%%", threshold)
		}
		if f.Note != "" {
			mark = strings.TrimSpace(mark + " " + f.Note)
		}
		fmt.Fprintf(&sb, "%-28s %8.3f%%  %s\n", f.Name, f.Changed, mark)
	}
	if len(r.States) > 0 {
		fmt.Fprintf(&sb, "\nstate differences (%d):\n", len(r.States))
		for _, d := range r.States {
			fmt.Fprintf(&sb, "  %-20s %-36s %s -> %s\n", d.Frame, d.Path, d.A, d.B)
		}
	}
	return sb.String()
}
