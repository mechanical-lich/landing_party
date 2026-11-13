package campaign

import (
	"math"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/research"
)

// Unit types (docs/brainstorm/hive_plan.md, "Types in data/factions.json"):
// the hive's colour blocks are specialisations, not research tiers. A
// doctrine's Types are keyed by type ("drone", "warrior"); the factory gives a
// spawned body its type's sprite block and stats (factory.ApplyType), and a
// presence's Units count its bodies by type (Tier 1 bookkeeping that the
// live hive is reconciled into). A stage's Mix names types for a doctrine
// with Types; a type locked by research gives its share to its morph's
// MorphFallback type.

// DoctrineType is one unit type of a doctrine with types (the hive).
type DoctrineType struct {
	Morph    string `json:"morph"`
	SpriteDX int    `json:"sprite_dx"`
	// Name is what the type is called once its morph link is known (phase
	// B+); SeenName before (the morph with its colour: "Red Xeno"). The
	// simulation reads neither.
	Name     string `json:"name"`
	SeenName string `json:"seen_name"`
	// Unlock is the research entry that unlocks the type ("" known from the
	// start).
	Unlock string `json:"unlock,omitempty"`
	// Cost is the biomass an egg of the type costs (the egg recipe's cost
	// mirrors it); HatchTurns the egg's hatch time.
	Cost       int `json:"cost,omitempty"`
	HatchTurns int `json:"hatch_turns,omitempty"`
	// Blueprint is the type's own body (the autonz: one silhouette per
	// type; the hive's come from the morph); PrintCost what its print
	// recipe costs (the content test holds the two equal).
	Blueprint string         `json:"blueprint,omitempty"`
	PrintCost map[string]int `json:"print_cost,omitempty"`
	// Absolute adult stats.
	Health int `json:"health,omitempty"`
	Str    int `json:"str,omitempty"`
	// Int is the body's Int (the greyish technicians research at Int 12+).
	Int int `json:"int,omitempty"`
	// Dig multiplies its dig work rate (the greys tunnel fast: their tech;
	// phase E4).
	Dig float64 `json:"dig,omitempty"`
	// Bore: a dig it finishes carries on this many tiles in all straight
	// through rock (the greys' tunnelling bore).
	Bore       int    `json:"bore,omitempty"`
	Dex        int    `json:"dex,omitempty"`
	AC         int    `json:"ac,omitempty"`
	Attack     string `json:"attack,omitempty"`
	Initiative int    `json:"initiative,omitempty"`
	// Skills and Equip are added to the adult; Role and Tasks set on a
	// worker (WorkerComponent.Role, AllowedTasks).
	Skills []string `json:"skills,omitempty"`
	Equip  []string `json:"equip,omitempty"`
	Role   string   `json:"role,omitempty"`
	Tasks  []string `json:"tasks,omitempty"`
	// Fallback is the type a locked type's share goes to when it is not its
	// morph's fallback (the autonz have no morphs: a seeker's share goes to
	// the enforcer, a harvester's to the wire placer; autonz_plan.md).
	Fallback string `json:"fallback,omitempty"`
	// Deploy: the unit can be replaced by a structure on a floor it stands
	// on (the relay drone into a relay pylon); nil for none.
	Deploy *DoctrineDeploy `json:"deploy,omitempty"`
}

// DoctrineDeploy is a type's deploy: after Ticks of work on a tile whose
// floor is one of Floor, the unit is replaced by the As structure.
type DoctrineDeploy struct {
	As    string   `json:"as"`
	Ticks int      `json:"ticks"`
	Floor []string `json:"floor,omitempty"`
}

// DoctrineTier is a print tier (the autonz' steel and bronze): a unit
// printed once its type's tier entry is known (TierOfType) is drawn
// SpriteDY lower on the sheet and has its health and str multiplied.
// Tiers[0] is the standard tier.
type DoctrineTier struct {
	ID       string  `json:"id"`
	SpriteDY int     `json:"sprite_dy,omitempty"`
	Health   float64 `json:"health,omitempty"`
	Str      float64 `json:"str,omitempty"`
}

// Tier is d's tier id, nil for none.
func (d *Doctrine) Tier(id string) *DoctrineTier {
	for i := range d.Tiers {
		if d.Tiers[i].ID == id {
			return &d.Tiers[i]
		}
	}
	return nil
}

// TierOfType is the tier a unit of type typ is printed at by a presence
// knowing techs: the last tier named by a known research entry's
// assimilate tier effect for typ, else d's first tier ("" when d has none).
func (d *Doctrine) TierOfType(typ string, techs []string) string {
	if d == nil || len(d.Tiers) == 0 {
		return ""
	}
	best := 0
	for _, k := range techs {
		r, ok := research.GetTech(k)
		if !ok || r.Assimilate == nil || r.Assimilate.Tier == nil || r.Assimilate.Tier.Type != typ {
			continue
		}
		for i := range d.Tiers {
			if d.Tiers[i].ID == r.Assimilate.Tier.Tier && i > best {
				best = i
			}
		}
	}
	return d.Tiers[best].ID
}

// HasTypes reports whether d's bodies are typed (the hive, the autonz).
func (d *Doctrine) HasTypes() bool { return d != nil && len(d.Types) > 0 }

// Broods reports whether d's typed bodies have morphs and life stages (the
// hive: eggs, larvae and the biomass tally), as the autonz' and greyish
// prints do not: a type names its morph. (The greyish have a morph for
// their names, but their types name none.)
func (d *Doctrine) Broods() bool {
	if !d.HasTypes() || len(d.Morphs) == 0 {
		return false
	}
	for _, t := range d.Types {
		if t.Morph != "" {
			return true
		}
	}
	return false
}

// Flies reports whether d's presences ride ships (the greyish, the mirror).
func (d *Doctrine) Flies() bool { return d != nil && d.Mobility.Kind == MobilityFleet }

// ShipSettles reports whether d is a typed settlement doctrine whose
// presence rides a ship (the greyish, phase E4): its settlement is the
// landed ship, with modules by hull tier (greyish_plan.md).
func (d *Doctrine) ShipSettles() bool {
	return d.HasTypes() && !d.Broods() && d.Settlement != nil && d.Flies()
}

// TypeKeys is d's type keys, sorted.
func (d *Doctrine) TypeKeys() []string {
	out := make([]string, 0, len(d.Types))
	for k := range d.Types {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// knownWithAliases is techs with every alias of a known tech added (an old
// save's tier_advanced is the warrior brood).
func (d *Doctrine) knownWithAliases(techs []string) map[string]bool {
	out := map[string]bool{}
	for _, t := range techs {
		out[t] = true
		if a, ok := d.TechAliases[t]; ok {
			out[a] = true
		}
	}
	return out
}

// TypesUnlocked is the set of d's types a presence knowing techs can raise:
// those with no Unlock and those whose Unlock (or an alias of a known tech)
// is known.
func (d *Doctrine) TypesUnlocked(techs []string) map[string]bool {
	known := d.knownWithAliases(techs)
	out := map[string]bool{}
	for k, t := range d.Types {
		if t.Unlock == "" || known[t.Unlock] {
			out[k] = true
		}
	}
	return out
}

// KnowsTech reports whether techs know tech, through an alias too.
func (d *Doctrine) KnowsTech(techs []string, tech string) bool {
	return d.knownWithAliases(techs)[tech]
}

// FallbackType is the type a locked type of morph gives its share to ("" for
// none): the morph's, else the doctrine's TypeFallback (the autonz).
func (d *Doctrine) FallbackType(morph string) string {
	if t, ok := d.MorphFallback[morph]; ok {
		return t
	}
	return d.TypeFallback
}

// fallbackOf is where locked type k's share goes: its own Fallback when
// that is open, else its morph's fallback.
func (d *Doctrine) fallbackOf(k string, open map[string]bool) string {
	if t := d.Types[k]; t != nil && t.Fallback != "" && open[t.Fallback] {
		return t.Fallback
	}
	return d.FallbackType(d.Types[k].Morph)
}

// EconomyTypes are the types a shot takes first and a starving presence
// loses last: the doctrine's EconomyTypeList when set (the autonz' wire
// placers, then relay drones), else the morphs' fallback types in morph
// order (the drone, then the scout).
func (d *Doctrine) EconomyTypes() []string {
	if len(d.EconomyTypeList) > 0 {
		return append([]string(nil), d.EconomyTypeList...)
	}
	morphs := make([]string, 0, len(d.MorphFallback))
	for m := range d.MorphFallback {
		morphs = append(morphs, m)
	}
	sort.Strings(morphs)
	out := make([]string, 0, len(morphs))
	for _, m := range morphs {
		if t := d.MorphFallback[m]; t != "" {
			out = append(out, t)
		}
	}
	return out
}

// counterpart is t's type of the other morph in the same colour block (the
// warrior's is the hunter), "" for none.
func (d *Doctrine) counterpart(t string) string {
	dt := d.Types[t]
	if dt == nil {
		return ""
	}
	for _, k := range d.TypeKeys() {
		o := d.Types[k]
		if k != t && o.SpriteDX == dt.SpriteDX && o.Morph != dt.Morph {
			return k
		}
	}
	return ""
}

// TypeMix is stage i's share of bodies per type on loc for a presence
// knowing techs: the stage's mix, each locked type's share given to its
// morph's fallback. On a site whose morph is not the default (a troid
// world) every fighting type swaps shares with its counterpart of the other
// morph (warrior and hunter, burrower and spitter), so its hive fights with
// the site's morph; the economy types (drone, scout) keep theirs, since the
// drones are the hive's workers. Empty shares are dropped.
func (d *Doctrine) TypeMix(i int, loc *Location, techs []string) map[string]float64 {
	if i < 0 || i >= len(d.Stages) {
		return nil
	}
	mix := map[string]float64{}
	for k, v := range d.Stages[i].Mix {
		if d.Types[k] != nil {
			mix[k] += v
		}
	}
	if site := d.MorphKeyAt(loc); site != "" && site != d.DefaultMorph {
		econ := map[string]bool{}
		for _, t := range d.EconomyTypes() {
			econ[t] = true
		}
		swapped := map[string]float64{}
		for k, v := range mix {
			if c := d.counterpart(k); c != "" && !econ[k] && !econ[c] {
				swapped[c] += v
			} else {
				swapped[k] += v
			}
		}
		mix = swapped
	}
	open := d.TypesUnlocked(techs)
	out := map[string]float64{}
	for k, v := range mix {
		if !open[k] {
			k = d.fallbackOf(k, open)
			if k == "" {
				continue
			}
		}
		out[k] += v
	}
	for k, v := range out {
		if v <= 0 {
			delete(out, k)
		}
	}
	return out
}

// UnitsTotal is the sum of p's Units.
func (p *FactionPresence) UnitsTotal() int {
	n := 0
	for _, v := range p.Units {
		n += v
	}
	return n
}

// mixShares is mix normalised to 1, nil when empty.
func mixShares(mix map[string]float64) map[string]float64 {
	total := 0.0
	for _, v := range mix {
		total += v
	}
	if total <= 0 {
		return nil
	}
	out := map[string]float64{}
	for k, v := range mix {
		out[k] = v / total
	}
	return out
}

// AddUnits adds n bodies to p's Units, each to the type most under its share
// of the stage's mix among unlocked types (ties by key). Deterministic.
func (d *Doctrine) AddUnits(p *FactionPresence, loc *Location, n int) {
	if !d.HasTypes() || n <= 0 {
		return
	}
	shares := mixShares(d.TypeMix(p.Stage, loc, p.KnownTechs))
	if len(shares) == 0 {
		shares = map[string]float64{d.FallbackType(d.DefaultMorph): 1}
	}
	keys := make([]string, 0, len(shares))
	for k := range shares {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if p.Units == nil {
		p.Units = map[string]int{}
	}
	for ; n > 0; n-- {
		total := float64(p.UnitsTotal() + 1)
		best, bestK := math.Inf(-1), ""
		for _, k := range keys {
			if def := shares[k]*total - float64(p.Units[k]); def > best {
				best, bestK = def, k
			}
		}
		p.Units[bestK]++
	}
}

// AddEconomyUnits adds n bodies of the default morph's fallback type (a
// landed brood is all drones).
func (d *Doctrine) AddEconomyUnits(p *FactionPresence, n int) {
	t := d.FallbackType(d.DefaultMorph)
	if !d.HasTypes() || n <= 0 || t == "" {
		return
	}
	if p.Units == nil {
		p.Units = map[string]int{}
	}
	p.Units[t] += n
}

// StarveUnits removes n bodies from p's Units: each from the type most over
// its share of the mix, the economy types (drones, then scouts) last.
func (d *Doctrine) StarveUnits(p *FactionPresence, loc *Location, n int) {
	if !d.HasTypes() {
		return
	}
	shares := mixShares(d.TypeMix(p.Stage, loc, p.KnownTechs))
	econ := d.EconomyTypes()
	isEcon := map[string]bool{}
	for _, t := range econ {
		isEcon[t] = true
	}
	for ; n > 0 && p.UnitsTotal() > 0; n-- {
		total := float64(p.UnitsTotal())
		best, bestK := math.Inf(-1), ""
		for _, k := range sortedUnitKeys(p.Units) {
			if p.Units[k] <= 0 || isEcon[k] {
				continue
			}
			if ex := float64(p.Units[k]) - shares[k]*total; ex > best {
				best, bestK = ex, k
			}
		}
		if bestK == "" {
			// Scouts before drones: the economy types in reverse.
			for i := len(econ) - 1; i >= 0; i-- {
				if p.Units[econ[i]] > 0 {
					bestK = econ[i]
					break
				}
			}
		}
		if bestK == "" {
			for _, k := range sortedUnitKeys(p.Units) {
				if p.Units[k] > 0 {
					bestK = k
					break
				}
			}
		}
		p.Units[bestK]--
		if p.Units[bestK] <= 0 {
			delete(p.Units, bestK)
		}
	}
}

// ShotUnits removes the n bodies a shot takes: drones first, then scouts,
// then by the starvation rule.
func (d *Doctrine) ShotUnits(p *FactionPresence, loc *Location, n int) {
	if !d.HasTypes() {
		return
	}
	for _, t := range d.EconomyTypes() {
		for n > 0 && p.Units[t] > 0 {
			p.Units[t]--
			n--
		}
		if p.Units[t] <= 0 {
			delete(p.Units, t)
		}
	}
	d.StarveUnits(p, loc, n)
}

// NormalizeUnits makes p's Units sum to its Population: bodies added by the
// growth rule, removed by the starvation rule. A doctrine without types
// keeps none.
func (d *Doctrine) NormalizeUnits(p *FactionPresence, loc *Location) {
	if !d.HasTypes() {
		p.Units = nil
		return
	}
	switch n := p.Population - p.UnitsTotal(); {
	case n > 0:
		d.AddUnits(p, loc, n)
	case n < 0:
		d.StarveUnits(p, loc, -n)
	}
	if len(p.Units) == 0 {
		p.Units = nil
	}
}

func sortedUnitKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Builds reports whether d's settlement builds its stage's structures with
// typed prints (the autonz): its presences keep Built.
func (d *Doctrine) Builds() bool {
	return d.HasTypes() && !d.Broods() && d.Settlement != nil && !d.Flies()
}

// StageBuilt is the structures a settlement of d at stage knowing techs
// stands: the stage's structures whose buildable's required tech is known
// (a hub without the foundry pattern has no foundry). nil for none.
func (d *Doctrine) StageBuilt(stage int, techs []string) map[string]int {
	if stage < 0 || stage >= len(d.Stages) {
		return nil
	}
	out := map[string]int{}
	all := construction.ListBuildables()
	for k, n := range d.Stages[stage].Structures {
		if b, ok := all[k]; ok && b.RequiredTech != "" && !d.KnowsTech(techs, b.RequiredTech) {
			continue
		}
		if n > 0 {
			out[k] = n
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
