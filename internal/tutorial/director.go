package tutorial

import "github.com/mechanical-lich/landing_party/internal/objective"

// Snapshot is what the director sees of the game each poll.
type Snapshot struct {
	// Context is where the player is: "site", "ship", "dash:starmap", ...
	Context string
	// At is "tutorial", "next" or "" for anywhere else, by the location in orbit.
	At string
	// Eval carries the simulation facts of the location in orbit plus the
	// bridge's ui.* flags.
	Eval objective.EvalContext
}

// Director advances through the steps. It takes and returns a step ID rather
// than campaign state so this package stays free of internal/campaign.
type Director struct {
	def     Definition
	idx     int
	eval    *objective.Evaluator
	skipped bool
	changed bool
}

// New starts a director at stepID. An unknown or empty ID starts at the first
// step; Observe then catches up past anything already satisfied.
func New(def Definition, stepID string) *Director {
	d := &Director{def: def}
	if i := def.StepIndex(stepID); i >= 0 {
		d.idx = i
	}
	d.resetEval()
	return d
}

func (d *Director) resetEval() {
	d.eval = nil
	if s := d.Current(); s != nil {
		d.eval = objective.New(objective.RuleSet{Rules: []objective.Rule{s.Done}})
	}
}

// Current is the live step, or nil when the tutorial is finished or skipped.
func (d *Director) Current() *Step {
	if d.skipped || d.idx < 0 || d.idx >= len(d.def.Steps) {
		return nil
	}
	return &d.def.Steps[d.idx]
}

// StepID is the current step's ID (the one Skip stopped on, after a skip), or
// the last step's once finished, so a save made after the handoff reloads as
// finished rather than restarting.
func (d *Director) StepID() string {
	if d.idx >= 0 && d.idx < len(d.def.Steps) {
		return d.def.Steps[d.idx].ID
	}
	if n := len(d.def.Steps); n > 0 {
		return d.def.Steps[n-1].ID
	}
	return ""
}

// Index is the current step's position (0-based); Count is the total.
func (d *Director) Index() int { return d.idx }
func (d *Director) Count() int { return len(d.def.Steps) }

// Done reports that every step completed. Skipped reports Skip was called.
func (d *Director) Done() bool    { return !d.skipped && d.idx >= len(d.def.Steps) }
func (d *Director) Skipped() bool { return d.skipped }

// Observe evaluates the current step against the snapshot and advances past
// every step the snapshot already satisfies, so doing things out of order
// just works. It stops at a step whose At does not match the snapshot.
func (d *Director) Observe(s Snapshot) {
	for {
		step := d.Current()
		if step == nil {
			return
		}
		if step.At != "" && step.At != s.At {
			return
		}
		if _, ok := d.eval.Evaluate(s.Eval); !ok {
			return
		}
		d.advance()
	}
}

func (d *Director) advance() {
	d.idx++
	d.changed = true
	d.resetEval()
}

// Skip ends the tutorial where it stands.
func (d *Director) Skip() {
	if d.Current() == nil {
		return
	}
	d.skipped = true
	d.changed = true
}

// InContext reports whether the current step is live in ctx.
func (d *Director) InContext(ctx string) bool {
	s := d.Current()
	if s == nil {
		return false
	}
	return s.Context == ContextAny || s.Context == ctx
}

// StepChanged reports, once, that the step moved since the last call, so the
// bridge can reset baselines and announce the new prompt.
func (d *Director) StepChanged() bool {
	c := d.changed
	d.changed = false
	return c
}

// NextUnlocked reports that the director is at or past the unlock_next step. A
// position test rather than an edge, so catching up past several steps in one
// Observe cannot miss the unlock.
func (d *Director) NextUnlocked() bool {
	for i := 0; i <= d.idx && i < len(d.def.Steps); i++ {
		if d.def.Steps[i].UnlockNext {
			return true
		}
	}
	return false
}
