//go:build !ebitenginevmguest

package game

import "github.com/mechanical-lich/mlge/state"

// The VM play-testing hook (vmhook_on.go) exists only in guest builds. These
// stubs keep normal builds free of it; the compiler drops the calls.

func vmStart(*Game)               {}
func vmLock() func()              { return vmNoop }
func vmNote(state.StateInterface) {}
func vmNoop()                     {}
