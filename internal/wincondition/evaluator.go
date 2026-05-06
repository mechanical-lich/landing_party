package wincondition

import "github.com/mechanical-lich/mlge/ecs"

type EvalContext struct {
	Entities        []*ecs.Entity
	Flags           map[string]any
	SettlementPop   map[string]int
	StructuresBuilt map[string]int
	Day             int
}

type Evaluator struct {
	rules RuleSet
}

func New(rules RuleSet) *Evaluator {
	return &Evaluator{rules: rules}
}

func (e *Evaluator) EvalSettlementPopulation(settlementName string, ctx EvalContext) (*Rule, bool) {
	pop := ctx.SettlementPop[settlementName]
	for i := range e.rules.Rules {
		r := &e.rules.Rules[i]
		if r.Trigger != TriggerSettlementPopulation {
			continue
		}
		if r.Settlement != "" && r.Settlement != settlementName {
			continue
		}
		if !compareInt(pop, r.Op, r.Threshold) || !checkConditions(r.When, ctx) {
			continue
		}
		return r, true
	}
	return nil, false
}

func (e *Evaluator) EvalEntityEliminated(blueprint string, ctx EvalContext) (*Rule, bool) {
	for i := range e.rules.Rules {
		r := &e.rules.Rules[i]
		if r.Trigger != TriggerEntityEliminated || r.Blueprint != blueprint {
			continue
		}
		if !checkConditions(r.When, ctx) {
			continue
		}
		return r, true
	}
	return nil, false
}

func (e *Evaluator) EvalDaysSurvived(ctx EvalContext) (*Rule, bool) {
	for i := range e.rules.Rules {
		r := &e.rules.Rules[i]
		if r.Trigger != TriggerDaysSurvived {
			continue
		}
		if !compareInt(ctx.Day, r.Op, r.Threshold) || !checkConditions(r.When, ctx) {
			continue
		}
		return r, true
	}
	return nil, false
}

func (e *Evaluator) EvalStructureBuilt(structureType string, ctx EvalContext) (*Rule, bool) {
	for i := range e.rules.Rules {
		r := &e.rules.Rules[i]
		if r.Trigger != TriggerStructureBuilt || r.Structure != structureType {
			continue
		}
		if !checkConditions(r.When, ctx) {
			continue
		}
		return r, true
	}
	return nil, false
}

func (e *Evaluator) EvalColonistEliminated(ctx EvalContext) (*Rule, bool) {
	for i := range e.rules.Rules {
		r := &e.rules.Rules[i]
		if r.Trigger != TriggerColonistEliminated {
			continue
		}
		if !compareInt(ctx.SettlementPop["colony"], r.Op, r.Threshold) || !checkConditions(r.When, ctx) {
			continue
		}
		return r, true
	}
	return nil, false
}

func (e *Evaluator) EvalTechResearched(techKey string, ctx EvalContext) (*Rule, bool) {
	for i := range e.rules.Rules {
		r := &e.rules.Rules[i]
		if r.Trigger != TriggerTechResearched || r.TechKey != techKey {
			continue
		}
		if !checkConditions(r.When, ctx) {
			continue
		}
		return r, true
	}
	return nil, false
}

func checkConditions(conds []Condition, ctx EvalContext) bool {
	for _, c := range conds {
		if c.GameFlag != nil {
			if !isTruthy(ctx.Flags[*c.GameFlag]) {
				return false
			}
		}
		if c.EntityCount != nil {
			count := 0
			for _, e := range ctx.Entities {
				if e.Blueprint == c.EntityCount.Blueprint {
					count++
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
