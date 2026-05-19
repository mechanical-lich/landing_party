package campaign

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/objective"
)

// Default template paths (data-driven; overridable in tests).
var (
	LocationTemplatePath = "data/location_templates.json"
	QuestTemplatePath    = "data/quest_templates.json"
)

type locArchetype struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Weight    int      `json:"weight"`
	StartOK   bool     `json:"start_ok"`
	Maps      []string `json:"maps"`
	Scenarios []string `json:"scenarios"`
	Tags      []string `json:"tags"`
	Summary   string   `json:"summary"`
}

type locTemplates struct {
	Archetypes []locArchetype `json:"archetypes"`
	Home         struct {
		Maps      []string `json:"maps"`
		Scenarios []string `json:"scenarios"`
		Summary   string   `json:"summary"`
	} `json:"home"`
}

type questTemplate struct {
	ID            string         `json:"id"`
	Weight        int            `json:"weight"`
	RequiresTag   string         `json:"requires_tag"`
	Name          string         `json:"name"`
	Desc          string         `json:"desc"`
	Trigger       string         `json:"trigger"`
	Resource      string         `json:"resource"`
	Blueprints    []string       `json:"blueprints"`
	Structure     string         `json:"structure"`
	TechPool      []string       `json:"tech_pool"`
	Min           int            `json:"min"`
	Max           int            `json:"max"`
	RewardFuelPer int            `json:"reward_fuel_per"`
	RewardFuelFlat int           `json:"reward_fuel_flat"`
	RewardRes     map[string]int `json:"reward_resources"`
	SpawnSystems  int            `json:"spawn_systems"`
	// TargetBlueprints / TargetNames drive a target_killed "bounty" quest: a
	// named entity (random blueprint + name from these pools) is spawned as a
	// location fixture and must be killed.
	TargetBlueprints []string `json:"target_blueprints"`
	TargetNames      []string `json:"target_names"`
	// FixtureStructure, when set, names a gen_structure script that builds a
	// lair around the quest's fixture entity on arrival (see QuestFixture).
	FixtureStructure string `json:"fixture_structure"`
	FixtureStructW   int    `json:"fixture_struct_w"`
	FixtureStructH   int    `json:"fixture_struct_h"`
	// SpawnArchetype, when set, makes this a "contract": instead of attaching
	// to an existing system's tags, generating the quest also charts a new
	// (hidden) location of that archetype and binds the quest to it. Accepting
	// the quest reveals it. Lets quests bring their own destination into being
	// (e.g. a distress call from a xeno-infested world).
	SpawnArchetype string `json:"spawn_archetype"`
}

type questTemplates struct {
	Templates []questTemplate `json:"templates"`
}

func loadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func loadGenTemplates() (*locTemplates, *questTemplates, error) {
	var lt locTemplates
	var qt questTemplates
	if err := loadJSON(LocationTemplatePath, &lt); err != nil {
		return nil, nil, fmt.Errorf("location templates: %w", err)
	}
	if err := loadJSON(QuestTemplatePath, &qt); err != nil {
		return nil, nil, fmt.Errorf("quest templates: %w", err)
	}
	if len(lt.Archetypes) == 0 || len(qt.Templates) == 0 {
		return nil, nil, fmt.Errorf("empty generation templates")
	}
	return &lt, &qt, nil
}

// GenerationConfig exposes the data-driven generation tuning (defaults if the
// file is absent) so callers can read start fuel/colonists/roster cap.
func GenerationConfig() GenConfig { return genConfig() }

// GenerateCampaign builds a fully procedural campaign: a generated start
// system, a far-off visible "Home" (reaching it wins), a couple of nearby
// systems, and their quests. Everything is derived from seed and persisted, so
// loading a save reproduces the run exactly.
func GenerateCampaign(name string, seed int64) (*Campaign, error) {
	lt, qt, err := loadGenTemplates()
	if err != nil {
		return nil, err
	}
	cfg := genConfig()
	c := &Campaign{
		Name:      name,
		Seed:      seed,
		Locations: make(map[string]*Location),
		Ship:      NewShipState(cfg.RosterCap),
	}
	rng := rand.New(rand.NewSource(seed))

	// Home: far away, visible from day one; distance drives its (huge) cost.
	hang := rng.Float64() * 2 * math.Pi
	home := &Location{
		ID: "home", Name: "Home", Kind: HomeKind,
		MapID: pick(rng, lt.Home.Maps), ScenarioID: pick(rng, lt.Home.Scenarios),
		Seed: seed + 1, Discovered: true,
		X: math.Cos(hang) * cfg.HomeRadius, Y: math.Sin(hang) * cfg.HomeRadius,
		Summary: lt.Home.Summary,
	}
	c.Locations[home.ID] = home

	// Start system at the origin.
	start := c.newSystem(rng, lt, qt, startArchetype(rng, lt), 0, 0, "start")
	start.Discovered = true
	c.CurrentLocationID = start.ID

	// Nearby systems, visible from the start so there's somewhere to go.
	nearby := cfg.NearbyMin
	if cfg.NearbyMax > cfg.NearbyMin {
		nearby += rng.Intn(cfg.NearbyMax - cfg.NearbyMin + 1)
	}
	for i := 0; i < nearby; i++ {
		ang := rng.Float64() * 2 * math.Pi
		r := cfg.NearbyRadiusMin + rng.Float64()*(cfg.NearbyRadiusMax-cfg.NearbyRadiusMin)
		near := c.newSystem(rng, lt, qt, weightedArchetype(rng, lt), math.Cos(ang)*r, math.Sin(ang)*r, "")
		near.Discovered = true
	}

	// One opening "contract" so a quest-defines-its-destination mission (e.g.
	// a distress call) is offered from the very start.
	if qd := c.makeContract(rng, lt, qt); qd != nil {
		c.QuestDefs = append(c.QuestDefs, *qd)
	}

	c.BindQuests()
	return c, nil
}

// Expand procedurally charts n new systems (with quests), marching the
// frontier toward Home. Deterministic via Seed+GenSeq. Returns the new
// systems (already Discovered) for the caller to announce.
func (c *Campaign) Expand(n int) []*Location {
	lt, qt, err := loadGenTemplates()
	if err != nil || n <= 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(c.Seed + int64(c.GenSeq+1)*1000003))
	var out []*Location
	for i := 0; i < n; i++ {
		x, y := c.placeTowardHome(rng)
		loc := c.newSystem(rng, lt, qt, weightedArchetype(rng, lt), x, y, "")
		loc.Discovered = true
		out = append(out, loc)
	}
	c.BindQuests()
	return out
}

// MaybeTravelQuests is rolled on every jump to a system: a 1-in-N chance
// (config: travel_quest_one_in) to generate 1..travel_quest_max new quests,
// each attached either to an existing discovered system or a freshly charted
// one. Keeps the run from stalling as long as the player keeps moving. Returns
// any newly charted systems and the number of quests added.
func (c *Campaign) MaybeTravelQuests() ([]*Location, int) {
	lt, qt, err := loadGenTemplates()
	if err != nil {
		return nil, 0
	}
	cfg := genConfig()
	c.TravelSeq++
	rng := rand.New(rand.NewSource(c.Seed + int64(c.TravelSeq)*2654435761 + int64(c.GenSeq)*131))
	if rng.Intn(cfg.TravelQuestOneIn) != 0 {
		return nil, 0
	}
	count := 1 + rng.Intn(cfg.TravelQuestMax)

	// Sorted by ID so selection (and thus the RNG draw sequence) is
	// deterministic — Go map iteration order is randomized.
	existing := []*Location{}
	for _, l := range c.Locations {
		if l.Discovered && l.Kind != HomeKind {
			existing = append(existing, l)
		}
	}
	sort.Slice(existing, func(i, j int) bool { return existing[i].ID < existing[j].ID })

	hasContracts := len(contractTemplates(qt)) > 0

	var newLocs []*Location
	added := 0
	for i := 0; i < count; i++ {
		// ~1/3 of rolled quests are "contracts" that chart their own hidden
		// destination (e.g. a distress call), independent of existing systems.
		if hasContracts && rng.Intn(cfg.ContractOneIn) == 0 {
			if qd := c.makeContract(rng, lt, qt); qd != nil {
				c.QuestDefs = append(c.QuestDefs, *qd)
				added++
				continue
			}
		}
		a := weightedArchetype(rng, lt)
		var target *Location
		if len(existing) == 0 || rng.Intn(cfg.NewSystemOneIn) == 0 {
			x, y := c.placeTowardHome(rng)
			target = c.newLocation(rng, a, x, y, "")
			target.Discovered = true
			c.addDatapadQuests(rng, qt, a, target)
			newLocs = append(newLocs, target)
			existing = append(existing, target)
		} else {
			target = existing[rng.Intn(len(existing))]
			// Roll a template against the target's own tag, not the random
			// archetype's, so the contract fits where it's posted.
			a = locArchetype{Tags: tagSlice(target.QuestTag)}
		}
		if qd := c.makeQuest(rng, qt, a, target); qd != nil {
			c.QuestDefs = append(c.QuestDefs, *qd)
			added++
		}
	}
	if added > 0 {
		c.BindQuests()
	}
	return newLocs, added
}

func tagSlice(t string) []string {
	if t == "" {
		return nil
	}
	return []string{t}
}

// placeTowardHome picks coordinates beyond the current explored frontier,
// biased toward Home so the run progresses without ever overshooting it.
func (c *Campaign) placeTowardHome(rng *rand.Rand) (float64, float64) {
	frontier := 0.0
	for _, loc := range c.Locations {
		if loc.Kind == HomeKind {
			continue
		}
		if d := math.Hypot(loc.X, loc.Y); d > frontier {
			frontier = d
		}
	}
	cfg := genConfig()
	home := c.HomeLocation()
	hang := 0.0
	if home != nil {
		hang = math.Atan2(home.Y, home.X)
	}
	r := frontier + cfg.ExpansionGapMin + rng.Float64()*cfg.ExpansionGapRand
	if cap := cfg.HomeRadius * cfg.HomeCapFrac; r > cap {
		r = cap
	}
	ang := hang + (rng.Float64()-0.5)*cfg.AngleJitter
	return math.Cos(ang) * r, math.Sin(ang) * r
}

// newLocation creates one location from an archetype (no quests) and appends
// it to the campaign.
func (c *Campaign) newLocation(rng *rand.Rand, a locArchetype, x, y float64, idHint string) *Location {
	c.GenSeq++
	id := fmt.Sprintf("sys_%d", c.GenSeq)
	if idHint != "" {
		id = idHint
	}
	loc := &Location{
		ID:         id,
		Name:       genName(rng),
		Kind:       a.Kind,
		MapID:      pick(rng, a.Maps),
		ScenarioID: pick(rng, a.Scenarios),
		Seed:       c.Seed + int64(c.GenSeq)*131,
		X:          x,
		Y:          y,
		Summary:    a.Summary,
		QuestTag:   firstTag(a.Tags),
	}
	c.Locations[id] = loc
	return loc
}

// newSystem creates one location from an archetype plus 1–2 quests, appends
// them to the campaign, and returns the location.
func (c *Campaign) newSystem(rng *rand.Rand, lt *locTemplates, qt *questTemplates, a locArchetype, x, y float64, idHint string) *Location {
	loc := c.newLocation(rng, a, x, y, idHint)
	nq := 1 + rng.Intn(2)
	for i := 0; i < nq; i++ {
		if qd := c.makeQuest(rng, qt, a, loc); qd != nil {
			c.QuestDefs = append(c.QuestDefs, *qd)
		}
	}
	c.addDatapadQuests(rng, qt, a, loc)
	return loc
}

// addDatapadQuests gives a location 1..datapad_quests_max hidden quests, each
// recoverable only by finding its datapad clue spawned at the location. They
// stay out of the quest log until the datapad is picked up.
func (c *Campaign) addDatapadQuests(rng *rand.Rand, qt *questTemplates, a locArchetype, loc *Location) {
	max := genConfig().DatapadQuestsMax
	if max <= 0 {
		return
	}
	n := 1 + rng.Intn(max)
	for i := 0; i < n; i++ {
		q := c.makeQuest(rng, qt, a, loc)
		if q == nil {
			continue
		}
		q.Delivery = "datapad"
		c.QuestDefs = append(c.QuestDefs, *q)
		loc.AddFixture(QuestFixture{
			QuestID:   q.ID,
			Blueprint: "datapad",
			Kind:      "datapad",
		})
	}
}

// makeQuest instantiates a concrete Quest from a tag-compatible, non-contract
// template, bound to the given (existing) location.
func (c *Campaign) makeQuest(rng *rand.Rand, qt *questTemplates, a locArchetype, loc *Location) *Quest {
	t := pickQuestTemplate(rng, qt, a.Tags)
	if t == nil {
		return nil
	}
	return c.buildQuestFromTemplate(rng, t, loc)
}

// makeContract rolls a "contract" quest: it charts a new hidden location of
// the template's spawn archetype and binds the quest there. Accepting the
// quest reveals the location (see Campaign.AcceptQuest).
func (c *Campaign) makeContract(rng *rand.Rand, lt *locTemplates, qt *questTemplates) *Quest {
	contracts := contractTemplates(qt)
	if len(contracts) == 0 {
		return nil
	}
	t := contracts[rng.Intn(len(contracts))]
	a, ok := archetypeByID(lt, t.SpawnArchetype)
	if !ok {
		return nil
	}
	x, y := c.placeTowardHome(rng)
	loc := c.newLocation(rng, a, x, y, "")
	loc.Discovered = false // hidden until the contract is accepted
	q := c.buildQuestFromTemplate(rng, t, loc)
	c.addDatapadQuests(rng, qt, a, loc)
	return q
}

// buildQuestFromTemplate constructs a concrete Quest from a chosen template,
// bound to loc.
func (c *Campaign) buildQuestFromTemplate(rng *rand.Rand, t *questTemplate, loc *Location) *Quest {
	c.GenSeq++
	q := &Quest{
		ID:       fmt.Sprintf("q_%d", c.GenSeq),
		Location: loc.ID,
		Reward:   QuestReward{SpawnSystems: t.SpawnSystems, Resources: t.RewardRes},
	}
	amount := 0
	if t.Max > t.Min {
		amount = t.Min + rng.Intn(t.Max-t.Min+1)
	} else {
		amount = t.Min
	}
	switch t.Trigger {
	case "resource_gathered":
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = replaceN(t.Desc, amount)
		q.Objective = objective.Rule{Trigger: objective.TriggerResourceGathered, Resource: t.Resource, Op: "gte", Threshold: amount}
		q.Reward.Fuel = t.RewardFuelFlat + t.RewardFuelPer*amount
	case "entity_killed":
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = t.Desc
		q.Objective = objective.Rule{Trigger: objective.TriggerEntityKilled, Blueprints: t.Blueprints, Op: "lte", Threshold: 0}
		q.Reward.Fuel = t.RewardFuelFlat
	case "tech_researched":
		tech := pick(rng, t.TechPool)
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = replaceTech(t.Desc, tech)
		q.Objective = objective.Rule{Trigger: objective.TriggerTechResearched, TechKey: tech}
		q.Reward.Fuel = t.RewardFuelFlat
	case "structure_built":
		q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
		q.Description = t.Desc
		q.Objective = objective.Rule{Trigger: objective.TriggerStructureBuilt, Structure: t.Structure}
		q.Reward.Fuel = t.RewardFuelFlat
	case "target_killed":
		q.Description = t.Desc
		q.Objective = objective.Rule{Trigger: objective.TriggerTargetKilled, Target: q.ID}
		q.Reward.Fuel = t.RewardFuelFlat
		if t.FixtureStructure != "" {
			// Structure-backed bounty: the generator script decides who the
			// target is and what it's called, then flags it via
			// mark_quest_target. The template carries no target info at all.
			q.TitlePrefix = t.Name
			q.Name = fmt.Sprintf("%s — %s", t.Name, loc.Name)
			loc.AddFixture(QuestFixture{
				QuestID:   q.ID,
				Kind:      "boss",
				Structure: t.FixtureStructure,
				StructW:   t.FixtureStructW,
				StructH:   t.FixtureStructH,
			})
		} else {
			// Legacy bounty: the template names a creature + bounty name and
			// the fixture spawns it in the open.
			blueprint := pick(rng, t.TargetBlueprints)
			if blueprint == "" {
				return nil
			}
			name := pick(rng, t.TargetNames)
			q.Name = fmt.Sprintf("%s: %s — %s", t.Name, name, loc.Name)
			loc.AddFixture(QuestFixture{
				QuestID:   q.ID,
				Blueprint: blueprint,
				Name:      name,
				Kind:      "boss",
			})
		}
	default:
		return nil
	}
	return q
}

// --- helpers ---

func contractTemplates(qt *questTemplates) []*questTemplate {
	var out []*questTemplate
	for i := range qt.Templates {
		if qt.Templates[i].SpawnArchetype != "" {
			out = append(out, &qt.Templates[i])
		}
	}
	return out
}

func archetypeByID(lt *locTemplates, id string) (locArchetype, bool) {
	for _, a := range lt.Archetypes {
		if a.ID == id {
			return a, true
		}
	}
	return locArchetype{}, false
}

func startArchetype(rng *rand.Rand, lt *locTemplates) locArchetype {
	var pool []locArchetype
	for _, a := range lt.Archetypes {
		if a.StartOK {
			pool = append(pool, a)
		}
	}
	if len(pool) == 0 {
		pool = lt.Archetypes
	}
	return pool[rng.Intn(len(pool))]
}

func weightedArchetype(rng *rand.Rand, lt *locTemplates) locArchetype {
	total := 0
	for _, a := range lt.Archetypes {
		w := a.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	r := rng.Intn(total)
	for _, a := range lt.Archetypes {
		w := a.Weight
		if w <= 0 {
			w = 1
		}
		if r < w {
			return a
		}
		r -= w
	}
	return lt.Archetypes[0]
}

func pickQuestTemplate(rng *rand.Rand, qt *questTemplates, tags []string) *questTemplate {
	var pool []*questTemplate
	for i := range qt.Templates {
		t := &qt.Templates[i]
		if t.SpawnArchetype != "" {
			continue // contracts bring their own location; not tag-rolled
		}
		if t.RequiresTag != "" && !hasTag(tags, t.RequiresTag) {
			continue
		}
		w := t.Weight
		if w <= 0 {
			w = 1
		}
		for j := 0; j < w; j++ {
			pool = append(pool, t)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[rng.Intn(len(pool))]
}

func genName(rng *rand.Rand) string {
	n := lore.RandomNameSeeded(rng, "location")
	if n == "" || n == "Unknown" {
		return fmt.Sprintf("System %d", rng.Intn(900)+100)
	}
	return n
}

func pick(rng *rand.Rand, s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[rng.Intn(len(s))]
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

func firstTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}

func replaceN(s string, n int) string  { return replaceAll(s, "%n", fmt.Sprintf("%d", n)) }
func replaceTech(s, t string) string   { return replaceAll(s, "%tech", t) }

func replaceAll(s, old, new string) string {
	out := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return out + s
		}
		out += s[:i] + new
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
