package vmtest

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.SetRGBA(i%w, i/w, c)
	}
	return img
}

func TestPixelDiff(t *testing.T) {
	a := solid(10, 10, color.RGBA{100, 100, 100, 255})
	b := solid(10, 10, color.RGBA{105, 100, 100, 255}) // within tolerance
	if pct, _ := PixelDiff(a, b, 8); pct != 0 {
		t.Errorf("within tolerance: %v%%", pct)
	}
	b.SetRGBA(3, 3, color.RGBA{200, 100, 100, 255})
	pct, mask := PixelDiff(a, b, 8)
	if pct != 1 || !mask[33] {
		t.Errorf("one pixel: %v%%", pct)
	}
	if pct, _ := PixelDiff(a, solid(5, 5, color.RGBA{}), 8); pct != 100 {
		t.Errorf("size mismatch: %v%%", pct)
	}
}

func writeRun(t *testing.T, dir string, frames map[string]*image.RGBA, states map[string]string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	for name, img := range frames {
		if err := WritePNG(filepath.Join(dir, name), img); err != nil {
			t.Fatal(err)
		}
	}
	for name, s := range states {
		os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644)
	}
}

func TestCompareRuns(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	grey := solid(20, 10, color.RGBA{50, 50, 50, 255})
	changed := solid(20, 10, color.RGBA{50, 50, 50, 255})
	for x := 0; x < 20; x++ {
		changed.SetRGBA(x, 0, color.RGBA{255, 255, 255, 255}) // 10% of pixels
	}
	writeRun(t, a, map[string]*image.RGBA{"0001_title.png": grey, "0002_site.png": grey, "0003_only_a.png": grey},
		map[string]string{"state_0001.json": `{"state":"title","id":1}`, "state_0002.json": `{"fuel":5,"crew":[{"x":1}]}`})
	writeRun(t, b, map[string]*image.RGBA{"0001_title.png": grey, "0002_site.png": changed},
		map[string]string{"state_0001.json": `{"state":"title","id":2}`, "state_0002.json": `{"fuel":4,"crew":[{"x":1}]}`})
	rep, err := CompareRuns(a, b, DiffOptions{DiffDir: b})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Frames) != 3 {
		t.Fatalf("frames %+v", rep.Frames)
	}
	if rep.Frames[0].Over || rep.Frames[0].Changed != 0 {
		t.Errorf("title %+v", rep.Frames[0])
	}
	if !rep.Frames[1].Over || rep.Frames[1].Changed != 10 {
		t.Errorf("site %+v", rep.Frames[1])
	}
	if !rep.Frames[2].Over || rep.Frames[2].Note != "missing in B" {
		t.Errorf("only_a %+v", rep.Frames[2])
	}
	if _, err := os.Stat(filepath.Join(b, "diff_site.png")); err != nil {
		t.Error("no diff image:", err)
	}
	// "id" is ignored; fuel differs.
	if len(rep.States) != 1 || rep.States[0].Path != "fuel" || rep.States[0].Frame != "site" {
		t.Errorf("states %+v", rep.States)
	}
	if !rep.Failed() {
		t.Error("report should fail")
	}
}
