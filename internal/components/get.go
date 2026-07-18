package components

import "github.com/mechanical-lich/mlge/ecs"

// Get returns the *T component on e, or (nil, false) if it's absent — a checked
// replacement for the unchecked `e.GetComponent(ct).(*T)` assertion that panics
// the tick loop when the component is missing.
//
//	if pc, ok := components.Get[rlcomponents.PositionComponent](e); ok { ... }
//
// The component's type key is read from its GetType method. *T carries both
// value- and pointer-receiver methods, so it works whichever way GetType is
// declared across the codebase.
func Get[T any](e *ecs.Entity) (*T, bool) {
	if e == nil {
		return nil, false
	}
	var zero T
	typed, ok := any(&zero).(interface{ GetType() ecs.ComponentType })
	if !ok {
		return nil, false
	}
	ct := typed.GetType()
	if !e.HasComponent(ct) {
		return nil, false
	}
	c, ok := any(e.GetComponent(ct)).(*T)
	return c, ok
}
