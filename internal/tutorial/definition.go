// Package tutorial is the guided first landing's director: a data-driven list
// of steps, each with a completion rule from internal/objective, advanced by
// polling snapshots of game state. It has no Ebiten or game dependency; the
// bridge in internal/game feeds it and draws its prompts.
package tutorial

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

// DefinitionPath is the step and topic data file, overridable in tests.
var DefinitionPath = "data/tutorial.json"

// Context values a step can be done in. "dash:<tab id>" names a dashboard tab.
const (
	ContextAny  = "any"
	ContextSite = "site"
	ContextShip = "ship"
)

// Location names used by Step.At and Step.Target.
const (
	AtTutorial = "tutorial"
	AtNext     = "next"
)

// Step is one prompt and the rule that completes it.
type Step struct {
	ID string `json:"id"`
	// At restricts evaluation to one location in orbit: "tutorial", "next" or
	// "" for anywhere. Observe stops at a step whose At does not match.
	At string `json:"at,omitempty"`
	// Context is where the prompt is live: site, ship, dash:<tab>, or any.
	// Elsewhere the panel shows a nudge.
	Context string `json:"context"`
	// Target is the destination ("tutorial" or "next") the ui.dest_selected
	// flag and the Star Map highlight refer to.
	Target string `json:"target,omitempty"`
	// UnlockNext marks the step that opens the second destination.
	UnlockNext bool   `json:"unlock_next,omitempty"`
	Title      string `json:"title"`
	Text       string `json:"text"`
	// Highlight lists targets; every visible one is drawn.
	Highlight []string `json:"highlight,omitempty"`
	// Topic is the Field Manual article the [Manual] button opens.
	Topic string `json:"topic,omitempty"`
	// Done is the completion rule.
	Done objective.Rule `json:"done"`
}

// Topic is one Field Manual article.
type Topic struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Definition is the whole data file.
type Definition struct {
	Steps  []Step  `json:"steps"`
	Topics []Topic `json:"topics"`
}

// Load reads a definition file.
func Load(path string) (Definition, error) {
	var d Definition
	raw, err := os.ReadFile(path)
	if err != nil {
		return d, fmt.Errorf("tutorial: %w", err)
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, fmt.Errorf("tutorial %s: %w", path, err)
	}
	if err := d.Validate(); err != nil {
		return d, fmt.Errorf("tutorial %s: %w", path, err)
	}
	return d, nil
}

// Validate checks the structural rules the director relies on: unique step
// IDs, known contexts, existing topics, and an op on every resource_gathered
// rule (a blank op means equal, which would miss a player who beamed down
// more than asked).
func (d Definition) Validate() error {
	if len(d.Steps) == 0 {
		return fmt.Errorf("no steps")
	}
	seen := map[string]bool{}
	for i, s := range d.Steps {
		if s.ID == "" {
			return fmt.Errorf("step %d has no id", i)
		}
		if seen[s.ID] {
			return fmt.Errorf("duplicate step id %q", s.ID)
		}
		seen[s.ID] = true
		switch {
		case s.Context == ContextAny, s.Context == ContextSite, s.Context == ContextShip,
			strings.HasPrefix(s.Context, "dash:"):
		default:
			return fmt.Errorf("step %s: unknown context %q", s.ID, s.Context)
		}
		if s.At != "" && s.At != AtTutorial && s.At != AtNext {
			return fmt.Errorf("step %s: unknown at %q", s.ID, s.At)
		}
		if s.Target != "" && s.Target != AtTutorial && s.Target != AtNext {
			return fmt.Errorf("step %s: unknown target %q", s.ID, s.Target)
		}
		if s.Topic != "" && d.Topic(s.Topic) == nil {
			return fmt.Errorf("step %s: unknown topic %q", s.ID, s.Topic)
		}
		if s.Done.Trigger == "" {
			return fmt.Errorf("step %s: no done rule", s.ID)
		}
		if s.Done.Trigger == objective.TriggerResourceGathered && s.Done.Op == "" {
			return fmt.Errorf("step %s: resource_gathered without an op", s.ID)
		}
		if s.Done.Trigger == objective.TriggerFlagSet && s.Done.Flag == "" {
			return fmt.Errorf("step %s: flag_set without a flag", s.ID)
		}
	}
	return nil
}

// Topic returns the Field Manual article with the given ID, or nil.
func (d Definition) Topic(id string) *Topic {
	for i := range d.Topics {
		if d.Topics[i].ID == id {
			return &d.Topics[i]
		}
	}
	return nil
}

// StepIndex returns the position of a step ID, or -1.
func (d Definition) StepIndex(id string) int {
	for i := range d.Steps {
		if d.Steps[i].ID == id {
			return i
		}
	}
	return -1
}
