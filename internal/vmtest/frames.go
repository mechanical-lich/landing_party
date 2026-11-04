package vmtest

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Downsample box-filters src to w×h. When src already has that size it is
// returned unchanged. Used to turn the physical (device-pixel) frame into the
// logical size, so frame pixels equal click coordinates.
func Downsample(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == w && sh == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0 := y * sh / h
		y1 := max((y+1)*sh/h, y0+1)
		for x := 0; x < w; x++ {
			x0 := x * sw / w
			x1 := max((x+1)*sw/w, x0+1)
			var r, g, b, a, n uint32
			for yy := y0; yy < y1; yy++ {
				row := src.Pix[(yy)*src.Stride:]
				for xx := x0; xx < x1; xx++ {
					p := row[xx*4 : xx*4+4]
					r += uint32(p[0])
					g += uint32(p[1])
					b += uint32(p[2])
					a += uint32(p[3])
					n++
				}
			}
			o := dst.Pix[y*dst.Stride+x*4:]
			o[0], o[1], o[2], o[3] = uint8(r/n), uint8(g/n), uint8(b/n), uint8(a/n)
		}
	}
	return dst
}

// Dir is an Output writing to a run directory: NNNN_name.png,
// state_NNNN.json and log.txt.
type Dir struct {
	Path string

	mu  sync.Mutex
	log io.WriteCloser
	// Echo also receives log lines (stderr), or nil.
	Echo io.Writer
}

// NewDir creates (and empties of frames) a run directory.
func NewDir(path string) (*Dir, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	old, _ := filepath.Glob(filepath.Join(path, "*.png"))
	olds, _ := filepath.Glob(filepath.Join(path, "state_*.json"))
	for _, f := range append(old, olds...) {
		_ = os.Remove(f)
	}
	f, err := os.Create(filepath.Join(path, "log.txt"))
	if err != nil {
		return nil, err
	}
	return &Dir{Path: path, log: f}, nil
}

// WriteFrame writes NNNN_name.png and, if state is non-nil, state_NNNN.json.
func (d *Dir) WriteFrame(n int, name string, img image.Image, state map[string]any) error {
	if err := WritePNG(filepath.Join(d.Path, fmt.Sprintf("%04d_%s.png", n, name)), img); err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.Path, fmt.Sprintf("state_%04d.json", n)), b, 0o644)
}

// Logf appends a line to log.txt.
func (d *Dir) Logf(format string, args ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	line := fmt.Sprintf(format, args...) + "\n"
	io.WriteString(d.log, line)
	if d.Echo != nil {
		io.WriteString(d.Echo, line)
	}
}

// Close closes the log.
func (d *Dir) Close() error { return d.log.Close() }

// WritePNG encodes img to path.
func WritePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
