package objective

import "github.com/mechanical-lich/mlge/ecs"

type EvalContext struct {
	Entities        []*ecs.Entity
	Flags           map[string]any
	SettlementPop   map[string]int
	StructuresBuilt map[string]int
	// ResourceCounts is the colony's stored quantity per resource blueprint
	// (e.g. "metal_ore", "crystal", "food").
	ResourceCounts map[string]int
	// EntityCounts is the live (non-dead) entity count per blueprint.
	EntityCounts map[string]int
	// KnownTechs is the list of tech keys the colony has researched.
	KnownTechs []string
	Day        int
}

type Evaluator struct {
	rules RuleSet
	// seenEntities records blueprints that have been observed alive at least
	// once, so "kill all of X" rules don't fire before X has ever existed.
	seenEntities map[string]bool
}

func New(rules RuleSet) *Evaluator {
	return &Evaluator{rules: rules, seenEntities: map[string]bool{}}
}

// Evaluate walks every rule and returns the first whose trigger condition is
// satisfied, so callers fire all scenario-defined win/lose rules from one place.
func (e *Evaluator) Evaluate(ctx EvalContext) (*Rule, bool) {
	if e.seenEntities == nil {
		e.seenEntities = map[string]bool{}
	}
	for bp, n := range ctx.EntityCounts {
		if n > 0 {
			e.seenEntities[bp] = true
		}
	}
	for i := range e.rules.Rules {
		r := &e.rules.Rules[i]
		var actual int
		switch r.Trigger {
		case TriggerDaysSurvived:
			actual = ctx.Day
		case TriggerColonistEliminated:
			actual = ctx.SettlementPop["colony"]
		case TriggerSettlementPopulation:
			actual = ctx.SettlementPop[r.Settlement]
		case TriggerStructureBuilt:
			// No op → "at least one built".
			if r.Op == "" {
				if ctx.StructuresBuilt[r.Structure] >= 1 && checkConditions(r.When, ctx) {
					return r, true
				}
				continue
			}
			actual = ctx.StructuresBuilt[r.Structure]
		case TriggerEntityEliminated:
			// Satisfied when none of the target blueprint(s) remain — but only
			// once at least one has existed, so it can't trip before any spawn.
			bps := ruleBlueprints(r)
			if e.anySeen(bps) && sumCounts(ctx.EntityCounts, bps) == 0 && checkConditions(r.When, ctx) {
				return r, true
			}
			continue
		case TriggerTechResearched:
			for _, k := range ctx.KnownTechs {
				if k == r.TechKey && checkConditions(r.When, ctx) {
					return r, true
				}
			}
			continue
		case TriggerEntityKilled:
			bps := ruleBlueprints(r)
			// "Kill all" (threshold 0) requires the entity to have existed.
			if r.Threshold == 0 && !e.anySeen(bps) {
				continue
			}
			actual = sumCounts(ctx.EntityCounts, bps)
		case TriggerResourceGathered:
			actual = ctx.ResourceCounts[r.Resource]
		default:
			continue
		}
		if compareInt(actual, r.Op, r.Threshold) && checkConditions(r.When, ctx) {
			return r, true
		}
	}
	return nil, false
}

// ruleBlueprints returns the blueprint set a rule targets: Blueprints if
// given, else the single Blueprint.
func ruleBlueprints(r *Rule) []string {
	if len(r.Blueprints) > 0 {
		return r.Blueprints
	}
	return []string{r.Blueprint}
}

func (e *Evaluator) anySeen(bps []string) bool {
	for _, bp := range bps {
		if e.seenEntities[bp] {
			return true
		}
	}
	return false
}

func sumCounts(counts map[string]int, bps []string) int {
	total := 0
	for _, bp := range bps {
		total += counts[bp]
	}
	return total
}

func checkConditions(conds []Condition, ctx EvalContext) bool {
	for _, c := range conds {
		if c.GameFlag != nil {
			if !isTruthy(ctx.Flags[*c.GameFlag]) {
				return false
			}
		}
		if c.EntityCount != nil {
			// Prefer the live (non-dead) tally so corpses don't keep a
			// "kill all" conjunction from ever completing; fall back to a
			// raw scan for callers that don't populate EntityCounts.
			count := 0
			if ctx.EntityCounts != nil {
				count = ctx.EntityCounts[c.EntityCount.Blueprint]
			} else {
				for _, e := range ctx.Entities {
					if e.Blueprint == c.EntityCount.Blueprint {
						count++
					}
				}
			}
			if !compareInt(count, c.EntityCount.Op, c.EntityCount.Value) {
				return false
			}
		}
	}
	return true
}

func compareInt(actual int, op string, threshold int) bool {
	switch op {
	case "eq", "":
		return actual == threshold
	case "gt":
		return actual > threshold
	case "lt":
		return actual < threshold
	case "gte":
		return actual >= threshold
	case "lte":
		return actual <= threshold
	}
	return false
}

func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case int:
		return t != 0
	case float64:
		return t != 0
	case string:
		return t != ""
	}
	return true
}
