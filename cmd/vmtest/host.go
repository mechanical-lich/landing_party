package main

import (
	"errors"
	"fmt"
	"image"
	"io"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/vmhost"
	"github.com/mechanical-lich/landing_party/internal/vmtest"
)

type hostConfig struct {
	binary             string
	guestLog           io.Writer
	dir                *vmtest.Dir
	logicalW, logicalH int
	full               bool
}

// host is the ebiten.Game that owns the VM session, and the vmtest.Session the
// runner drives. The script runs on its own goroutine; anything that renders
// (frames, Close, SetOutsideScreen) is sent to the host goroutine and served
// one per host Update. Rendering many guest frames inside one host Update
// freezes capture silently after ~1,200 frames (see docs/brainstorm/vmtest_plan.md).
type host struct {
	cfg hostConfig

	guest *vmtest.Guest
	sess  *vmhost.GuestSession
	side  *vmtest.Sidecar

	screen *ebiten.Image
	scale  float64
	pix    []byte

	script     func()
	started    bool
	ops        chan func()
	opDone     chan struct{}
	scriptDone chan struct{}
	closed     bool

	hostUpdates int
	launchMS    []int64
	exited      bool  // the guest ended (expect_exit)
	exitErr     error // its process error
}

func newHost(cfg hostConfig) *host {
	return &host{
		cfg:        cfg,
		ops:        make(chan func()),
		opDone:     make(chan struct{}),
		scriptDone: make(chan struct{}),
	}
}

// launch starts a guest and opens a session on it, retrying once.
func (h *host) launch() error {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		t := time.Now()
		var g *vmtest.Guest
		g, err = vmtest.StartGuest(vmtest.SpawnOptions{
			Binary:   h.cfg.binary,
			Log:      h.cfg.guestLog,
			Env:      []string{"GODEBUG=randautoseed=0"},
			Endpoint: vmhost.EndpointURLFromAddr,
		})
		if err != nil {
			h.cfg.dir.Logf("launch attempt %d: %v", attempt+1, err)
			continue
		}
		var s *vmhost.GuestSession
		s, err = vmhost.NewGuestSession(g.VM, &vmhost.NewGuestSessionOptions{IdleTimeout: 60 * time.Second})
		if err != nil {
			g.Kill()
			h.cfg.dir.Logf("launch attempt %d: session: %v", attempt+1, err)
			continue
		}
		h.guest, h.sess = g, s
		h.side = nil
		if g.Side != nil {
			h.side = vmtest.NewSidecar(g.Side)
		}
		h.exited, h.exitErr = false, nil
		ms := time.Since(t).Milliseconds()
		h.launchMS = append(h.launchMS, ms)
		h.cfg.dir.Logf("guest connected in %d ms (sidecar: %v)", ms, g.Side != nil)
		return nil
	}
	return err
}

// Update serves at most one render request per call.
func (h *host) Update() error {
	h.hostUpdates++
	if !h.started {
		h.started = true
		h.scale = ebiten.Monitor().DeviceScaleFactor()
		w := int(float64(h.cfg.logicalW) * h.scale)
		hh := int(float64(h.cfg.logicalH) * h.scale)
		h.screen = ebiten.NewImage(w, hh)
		h.pix = make([]byte, 4*w*hh)
		h.cfg.dir.Logf("device scale %.2f, outside screen %dx%d, frames written at %s", h.scale, w, hh,
			map[bool]string{true: "physical size", false: fmt.Sprintf("%dx%d", h.cfg.logicalW, h.cfg.logicalH)}[h.cfg.full])
		// Must happen inside a host Update and before any tick.
		if err := h.sess.SetOutsideScreen(h.screen); err != nil {
			return err
		}
		go func() {
			defer close(h.scriptDone)
			defer func() {
				if r := recover(); r != nil {
					h.cfg.dir.Logf("script goroutine panic: %v", r)
				}
			}()
			h.script()
		}()
		return nil
	}
	select {
	case f := <-h.ops:
		f()
		h.opDone <- struct{}{}
	case <-h.scriptDone:
		if !h.closed && !h.exited {
			h.closed = true
			_ = h.sess.Close()
		}
		return ebiten.Termination
	default:
	}
	return nil
}

func (h *host) Draw(*ebiten.Image)          {}
func (h *host) Layout(w, hh int) (int, int) { return w, hh }
func (h *host) inFrame(f func())            { h.ops <- f; <-h.opDone }
func (h *host) sessErr(what string) error {
	err := h.sess.Err()
	if ex, perr := h.guest.Exited(); ex {
		return fmt.Errorf("%s: guest exited (%v), session: %v", what, perr, err)
	}
	return fmt.Errorf("%s: session: %v", what, err)
}

// finish reaps the guest after RunGame returns.
func (h *host) finish() error {
	if h.guest == nil {
		return nil
	}
	if h.exited {
		return h.exitErr
	}
	err, killed := h.guest.Wait(20 * time.Second)
	if killed {
		h.cfg.dir.Logf("guest killed after close timeout")
		return errors.New("killed after close timeout")
	}
	return err
}

// --- vmtest.Session ---

func (h *host) Ticks(n int) error {
	if h.exited {
		return errors.New("guest has exited")
	}
	h.sess.AdvanceTicks(n)
	if !h.sess.WaitTicks() {
		return h.sessErr("ticks")
	}
	return nil
}

func (h *host) PressKey(k ebiten.Key)             { h.sess.PressKey(k) }
func (h *host) ReleaseKey(k ebiten.Key)           { h.sess.ReleaseKey(k) }
func (h *host) MoveCursor(x, y float64)           { h.sess.MoveCursor(x, y) }
func (h *host) PressMouse(b ebiten.MouseButton)   { h.sess.PressMouseButton(b) }
func (h *host) ReleaseMouse(b ebiten.MouseButton) { h.sess.ReleaseMouseButton(b) }
func (h *host) Scroll(dx, dy float64)             { h.sess.ScrollWheel(dx, dy) }
func (h *host) TypeRune(r rune)                   { h.sess.TypeRune(r) }

func (h *host) Frame(keep bool) (image.Image, error) {
	if h.exited {
		return nil, errors.New("guest has exited")
	}
	var err error
	h.inFrame(func() {
		h.sess.AdvanceFrame()
		if !h.sess.WaitFrame() {
			err = h.sessErr("frame")
			return
		}
		if !h.sess.CompositeFrame() {
			err = h.sessErr("composite")
			return
		}
		if keep {
			h.screen.ReadPixels(h.pix)
		}
	})
	if err != nil || !keep {
		return nil, err
	}
	b := h.screen.Bounds()
	img := &image.RGBA{Pix: append([]byte(nil), h.pix...), Stride: 4 * b.Dx(), Rect: image.Rect(0, 0, b.Dx(), b.Dy())}
	if h.cfg.full {
		return img, nil
	}
	return vmtest.Downsample(img, h.cfg.logicalW, h.cfg.logicalH), nil
}

func (h *host) State() (map[string]any, error) {
	if h.side == nil || h.exited {
		return nil, vmtest.ErrNoSidecar
	}
	return h.side.State()
}

func (h *host) Target(name string) (float64, float64, error) {
	if h.side == nil || h.exited {
		return 0, 0, vmtest.ErrNoSidecar
	}
	return h.side.Target(name)
}

func (h *host) Targets() ([]string, error) {
	if h.side == nil || h.exited {
		return nil, vmtest.ErrNoSidecar
	}
	return h.side.Targets()
}

func (h *host) Debug(cmd string) error {
	if h.side == nil || h.exited {
		return vmtest.ErrNoSidecar
	}
	return h.side.Debug(cmd)
}

// Relaunch closes the session on the host goroutine, reaps the guest, and
// attaches a new guest in the same sandbox.
func (h *host) Relaunch() error {
	if !h.exited {
		h.inFrame(func() { _ = h.sess.Close() })
		err, killed := h.guest.Wait(20 * time.Second)
		h.cfg.dir.Logf("old guest exit: %v (killed=%v)", err, killed)
	}
	if err := h.launch(); err != nil {
		return err
	}
	var err error
	h.inFrame(func() { err = h.sess.SetOutsideScreen(h.screen) })
	return err
}

// ExpectExit advances ticks until the guest's session ends, then requires a
// zero exit status.
func (h *host) ExpectExit(maxTicks int) error {
	for done := 0; done < maxTicks; done += 5 {
		h.sess.AdvanceTicks(5)
		if !h.sess.WaitTicks() {
			err, killed := h.guest.Wait(20 * time.Second)
			h.exited, h.exitErr = true, err
			if killed {
				return errors.New("session ended but the guest did not exit")
			}
			if err != nil {
				return fmt.Errorf("guest exit: %v", err)
			}
			h.inFrame(func() { _ = h.sess.Close() })
			return nil
		}
	}
	return fmt.Errorf("guest still running after %d ticks", maxTicks)
}
