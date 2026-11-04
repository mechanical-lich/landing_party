package vmtest

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mechanical-lich/landing_party/internal/vmproto"
)

// Sidecar is the host end of the guest's sidecar socket. Requests cost no
// guest ticks: the guest answers them between ticks under its own lock.
type Sidecar struct {
	mu   sync.Mutex
	conn net.Conn
	r    *bufio.Reader
}

// NewSidecar wraps an accepted sidecar connection.
func NewSidecar(c net.Conn) *Sidecar {
	return &Sidecar{conn: c, r: bufio.NewReaderSize(c, 1<<20)}
}

func (s *Sidecar) call(req vmproto.Request) (*vmproto.Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(req)
	_ = s.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := s.conn.Write(append(b, '\n')); err != nil {
		return nil, fmt.Errorf("sidecar: %w", err)
	}
	line, err := s.r.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("sidecar: %w", err)
	}
	var rep vmproto.Reply
	if err := json.Unmarshal(line, &rep); err != nil {
		return nil, fmt.Errorf("sidecar: %w", err)
	}
	if !rep.OK {
		return &rep, errors.New(rep.Error)
	}
	return &rep, nil
}

// State returns the guest's state summary.
func (s *Sidecar) State() (map[string]any, error) {
	rep, err := s.call(vmproto.Request{Op: vmproto.OpState})
	if err != nil {
		return nil, err
	}
	return rep.State, nil
}

// Target resolves a named target to its centre point.
func (s *Sidecar) Target(name string) (float64, float64, error) {
	rep, err := s.call(vmproto.Request{Op: vmproto.OpTarget, Name: name})
	if err != nil {
		return 0, 0, fmt.Errorf("target %s: %w", name, err)
	}
	return rep.X + rep.W/2, rep.Y + rep.H/2, nil
}

// Targets lists the names resolvable now.
func (s *Sidecar) Targets() ([]string, error) {
	rep, err := s.call(vmproto.Request{Op: vmproto.OpTargets})
	if err != nil {
		return nil, err
	}
	return rep.Targets, nil
}

// Debug runs a guest debug command.
func (s *Sidecar) Debug(cmd string) error {
	_, err := s.call(vmproto.Request{Op: vmproto.OpDebug, Cmd: cmd})
	if err != nil {
		return fmt.Errorf("debug %q: %w", cmd, err)
	}
	return nil
}
