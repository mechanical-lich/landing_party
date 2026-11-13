package game

import (
	"math"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
)

// The factory applies a faction's unit type at spawn from its doctrine's
// types (data/factions.json), with the upgrades its presence knows (the
// evolve entries of data/research.json); a creature's faction string is its
// doctrine key.
func init() { factory.TypeSource = doctrineType }

// presenceTechs is what each presence on a built level knows, by ID: set
// when it materialises and when its settlement researches, read when a body
// of it spawns. presenceSettlement is a settlement presence's settlement
// name, whose techs are the live ones.
var (
	presenceTechs      = map[string][]string{}
	presenceSettlement = map[string]string{}
)

// resetPresenceTechs forgets every presence's techs and settlement: a new
// campaign's presence IDs (fp1, ...) start again, so the last campaign's
// must not answer for them (review note: the maps were never cleared
// between campaigns in one process).
func resetPresenceTechs() {
	presenceTechs = map[string][]string{}
	presenceSettlement = map[string]string{}
}

// notePresenceTechs records what presence id knows, for its spawns.
func notePresenceTechs(id string, techs []string) {
	if id != "" {
		presenceTechs[id] = append([]string(nil), techs...)
	}
}

// techsOf is what presence knows: its settlement's techs, else the record's.
func techsOf(presence string) []string {
	if name, ok := presenceSettlement[presence]; ok {
		if st := settlement.Settlements[name]; st != nil {
			return st.KnownTechs
		}
	}
	return presenceTechs[presence]
}

func doctrineType(faction, typ, presence string) *factory.UnitType {
	d := campaign.LoadFactionData().Doctrine(faction)
	if d == nil || d.Types[typ] == nil {
		return nil
	}
	return upgradedType(d, typ, techsOf(presence))
}

// upgradedType is d's type typ with the upgrades known in techs applied: the
// type's evolve multipliers (health, str, initiative, work), every flier's
// flier speed, and the evolved frame once an upgrade marked evolved is known.
func upgradedType(d *campaign.Doctrine, typ string, techs []string) *factory.UnitType {
	t := d.Types[typ]
	hp, str, init, work, mine := 1.0, 1.0, 1.0, 1.0, 1.0
	ac := 0
	evolved := false
	keys := append([]string(nil), techs...)
	sort.Strings(keys)
	for _, k := range keys {
		r, ok := research.GetTech(k)
		if ok && r.Develop != nil {
			// The greys' develop upgrades (greyish_plan.md, "The tree").
			for _, u := range r.Develop.Upgrade {
				if u.Type != typ {
					continue
				}
				if u.Health > 0 {
					hp *= u.Health
				}
				if u.Str > 0 {
					str *= u.Str
				}
				if u.Initiative > 0 {
					init *= u.Initiative
				}
				ac += u.AC
			}
			if r.Develop.Salvage > 0 && hasTaskName(t.Tasks, "mine") {
				mine /= r.Develop.Salvage // salvage 0.75: mine work in three quarters of the time
			}
		}
		if !ok || r.Evolve == nil {
			continue
		}
		ev := r.Evolve
		if ev.FlierSpeed > 0 && t.Morph == "flier" {
			init *= ev.FlierSpeed
		}
		if ev.Type != typ {
			continue
		}
		if ev.Health > 0 {
			hp *= ev.Health
		}
		if ev.Str > 0 {
			str *= ev.Str
		}
		if ev.Initiative > 0 {
			init *= ev.Initiative
		}
		if ev.Work > 0 {
			work *= ev.Work
		}
		evolved = evolved || ev.Evolved
	}
	scale := func(v int, m float64) int {
		if v <= 0 {
			return v
		}
		return max(1, int(math.Round(float64(v)*m)))
	}
	// The print tier (the autonz' bronze): at spawn only, from what the
	// presence knows then.
	tierID, dy := d.TierOfType(typ, techs), 0
	if tr := d.Tier(tierID); tr != nil {
		dy = tr.SpriteDY
		if tr.Health > 0 {
			hp *= tr.Health
		}
		if tr.Str > 0 {
			str *= tr.Str
		}
	}
	ut := &factory.UnitType{
		ID: typ, Morph: t.Morph, SpriteDX: t.SpriteDX, SpriteDY: dy, Tier: tierID, SeenName: t.SeenName, HatchTurns: t.HatchTurns,
		Health: scale(t.Health, hp), Str: scale(t.Str, str), Dex: t.Dex, Int: t.Int, AC: t.AC + ac, Attack: t.Attack,
		Initiative: scale(t.Initiative, init),
		Role:       t.Role, Tasks: factory.TaskActions(t.Tasks), Skills: t.Skills, Equip: t.Equip,
		BodySkills: d.BodySkills, Evolved: evolved,
	}
	if work != 1 {
		ut.Work = work
	}
	if mine != 1 {
		ut.Mine = mine
	}
	ut.Dig, ut.Bore = t.Dig, t.Bore
	return ut
}

func hasTaskName(tasks []string, k string) bool {
	for _, v := range tasks {
		if v == k {
			return true
		}
	}
	return false
}

// retypeSettlement re-applies the type of every living adult of settlement
// name on level after it learned tech, when tech is an evolve upgrade (a
// type's multipliers or the fliers' speed): upgrades reach living units.
func retypeSettlement(level *world.Level, name, tech string) {
	r, ok := research.GetTech(tech)
	if level == nil || !ok {
		return
	}
	evolves := r.Evolve != nil && (r.Evolve.Type != "" || r.Evolve.FlierSpeed != 0)
	develops := r.Develop != nil && (len(r.Develop.Upgrade) > 0 || r.Develop.Salvage > 0)
	if !evolves && !develops {
		return
	}
	for _, e := range level.Entities {
		if e == nil || !components.IsColonyWorker(e, name) {
			continue
		}
		if k := components.KinOf(e); k != nil && k.Type != "" {
			factory.Retype(e)
		}
	}
}

// hiveAdults is the living typed adults of settlement name on level.
func hiveAdults(level *world.Level, name string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range settlementWorkers(level, name) {
		if k := components.KinOf(e); k != nil && k.Type != "" {
			out = append(out, e)
		}
	}
	return out
}
