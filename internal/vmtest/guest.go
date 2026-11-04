package vmtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GuestTag is the build tag that turns the game into a VM guest and links the
// sidecar hook.
const GuestTag = "ebitenginevmguest"

// SidecarEnv names the sidecar socket for the guest.
const SidecarEnv = "LP_VMTEST_SOCKET"

// BuildGuest compiles ./cmd/game with the guest tag into bin.
func BuildGuest(repo, bin string, logw io.Writer) error {
	cmd := exec.Command("go", "build", "-tags", GuestTag, "-o", bin, "./cmd/game")
	cmd.Dir = repo
	cmd.Stdout = logw
	cmd.Stderr = logw
	return cmd.Run()
}

// MakeSandbox lays out a directory the guest can run in without touching the
// repo: assets and each child of data/ symlinked, except config.local.json,
// which is written from config, and an empty saves/. Existing contents of
// sandbox are removed first.
func MakeSandbox(repo, sandbox string, config map[string]any) error {
	repo, err := filepath.Abs(repo)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(sandbox); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(sandbox, "data"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(sandbox, "saves"), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(filepath.Join(repo, "assets"), filepath.Join(sandbox, "assets")); err != nil {
		return err
	}
	ents, err := os.ReadDir(filepath.Join(repo, "data"))
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.Name() == "config.local.json" {
			continue
		}
		if err := os.Symlink(filepath.Join(repo, "data", e.Name()), filepath.Join(sandbox, "data", e.Name())); err != nil {
			return err
		}
	}
	if config == nil {
		config = map[string]any{}
	}
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sandbox, "data", "config.local.json"), b, 0o644)
}

// Guest is one running guest process with its accepted connections.
type Guest struct {
	Cmd  *exec.Cmd
	VM   net.Conn
	Side net.Conn // nil if the guest never dialled the sidecar

	dir  string
	done chan struct{}
	err  error
}

// SpawnOptions configure StartGuest.
type SpawnOptions struct {
	Binary  string    // guest executable, inside the sandbox
	Log     io.Writer // guest stdout and stderr
	Env     []string  // extra environment
	Timeout time.Duration
	// Endpoint turns a listener address into the guest's endpoint URL
	// (vmhost.EndpointURLFromAddr), injected so this package needs no vmhost.
	Endpoint func(net.Addr) (string, error)
}

// StartGuest listens on the VM and sidecar sockets, starts the guest and
// accepts both. The sidecar is optional: if the guest has not dialled it by
// the time the VM connection arrives plus a short grace, Side stays nil.
func StartGuest(o SpawnOptions) (*Guest, error) {
	if o.Timeout == 0 {
		o.Timeout = 60 * time.Second
	}
	// Socket paths must stay short (104 bytes on macOS), so not in the sandbox.
	dir, err := os.MkdirTemp("", "lpvm")
	if err != nil {
		return nil, err
	}
	vmln, err := net.Listen("unix", filepath.Join(dir, "vm.sock"))
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	defer vmln.Close()
	sideln, err := net.Listen("unix", filepath.Join(dir, "side.sock"))
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	defer sideln.Close()
	ep, err := o.Endpoint(vmln.Addr())
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	cmd := exec.Command(o.Binary)
	cmd.Dir = filepath.Dir(o.Binary)
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EBITENGINE_VM_ENDPOINT=") || strings.HasPrefix(kv, SidecarEnv+"=") || strings.HasPrefix(kv, "GODEBUG=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "EBITENGINE_VM_ENDPOINT="+ep, SidecarEnv+"="+filepath.Join(dir, "side.sock"))
	cmd.Env = append(env, o.Env...)
	cmd.Stdout = o.Log
	cmd.Stderr = o.Log
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	g := &Guest{Cmd: cmd, dir: dir, done: make(chan struct{})}
	go func() {
		g.err = cmd.Wait()
		close(g.done)
	}()

	type res struct {
		c   net.Conn
		err error
	}
	accept := func(ln net.Listener) <-chan res {
		ch := make(chan res, 1)
		go func() {
			c, err := ln.Accept()
			ch <- res{c, err}
		}()
		return ch
	}
	vmch, sidech := accept(vmln), accept(sideln)
	deadline := time.After(o.Timeout)
	select {
	case r := <-vmch:
		if r.err != nil {
			g.Kill()
			return nil, r.err
		}
		g.VM = r.c
	case <-g.done:
		g.Close()
		return nil, fmt.Errorf("guest exited before connecting: %v", g.err)
	case <-deadline:
		g.Kill()
		return nil, errors.New("guest did not connect in time")
	}
	select {
	case r := <-sidech:
		if r.err == nil {
			g.Side = r.c
		}
	case <-time.After(2 * time.Second):
	}
	return g, nil
}

// Exited reports whether the process has ended, and its error.
func (g *Guest) Exited() (bool, error) {
	select {
	case <-g.done:
		return true, g.err
	default:
		return false, nil
	}
}

// Wait waits up to d for the process to end, killing it after. It returns the
// process error (nil for status 0) and whether it had to kill.
func (g *Guest) Wait(d time.Duration) (error, bool) {
	select {
	case <-g.done:
		g.Close()
		return g.err, false
	case <-time.After(d):
		g.Kill()
		return g.err, true
	}
}

// Kill ends the process and cleans up.
func (g *Guest) Kill() {
	if g.Cmd.Process != nil {
		_ = g.Cmd.Process.Kill()
	}
	<-g.done
	g.Close()
}

// Close releases connections and the socket directory (not the process).
func (g *Guest) Close() {
	if g.Side != nil {
		g.Side.Close()
	}
	os.RemoveAll(g.dir)
}
